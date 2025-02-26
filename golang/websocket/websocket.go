package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lingzhi-server/config"
	"lingzhi-server/log"
	"lingzhi-server/model"
	"lingzhi-server/utils/asr"
	"lingzhi-server/utils/llm"
	"lingzhi-server/utils/tts"
	"lingzhi-server/utils/vad"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// APIResponse 表示来自Python API的响应
type APIResponse struct {
	Status string `json:"status"`           // 响应状态，如"success"或"error"
	Result string `json:"result"`           // 响应结果，通常是处理后的数据
	Error  string `json:"detail,omitempty"` // 错误详情，仅在出错时存在
}

// ResponseMessage 表示要发送的响应消息
type ResponseMessage struct {
	MessageType int    // WebSocket消息类型
	Data        []byte // 消息数据
}

// WebSocketConnection 表示一个WebSocket连接
type WebSocketConnection struct {
	conn            *websocket.Conn       // WebSocket连接对象
	config          *config.Config        // 服务器配置
	responseChan    chan ResponseMessage  // 响应消息通道
	llmChan         chan string           // llm聊天消息通道
	ttsChan         chan string           // tts聊天消息通道
	dialogueStore   []model.Dialogue      // 对话存储
	ctx             context.Context       // 上下文，用于控制goroutine生命周期
	cancelFunc      context.CancelFunc    // 取消函数，用于关闭上下文
	connectionState model.ConnectionState // 状态信息
}

// NewWebSocketConnection 创建一个新的WebSocket连接处理器
// 参数:
//   - conn: WebSocket连接对象
//   - header: HTTP请求头
//   - cfg: 服务器配置
//
// 返回:
//   - *WebSocketConnection: 新创建的WebSocket连接处理器
func NewWebSocketConnection(conn *websocket.Conn, r *http.Request, cfg *config.Config) *WebSocketConnection {
	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建WebSocketConnection实例
	wsConn := &WebSocketConnection{
		conn:         conn,
		config:       cfg,
		responseChan: make(chan ResponseMessage, 10), // 缓冲区大小为10的通道
		llmChan:      make(chan string, 10),          // 缓冲区大小为10的通道
		ttsChan:      make(chan string, 10),          // 缓冲区大小为10的通道
		ctx:          ctx,
		cancelFunc:   cancel,
		connectionState: model.ConnectionState{
			DeviceId:         r.Header.Get("device-id"),
			ClientIP:         r.RemoteAddr,
			SessionId:        uuid.NewString(),
			ClientListenMode: "auto",
			ASRServerReceive: true,
		},
	}

	log.Debugf("wsConn.connectionState: %+v", wsConn.connectionState)

	// 启动响应处理协程
	go wsConn.handleResponses()

	// 启动LLM处理协程
	go wsConn.handleChat()

	// 启动TTS处理协程
	go wsConn.handleTTS()

	return wsConn
}

// 客户端开始说话
func (wsc *WebSocketConnection) startSpeak() error {
	// 发送TTS响应到客户端
	ttsStartResponse := model.ConnectionCommand{
		Type:  "tts",
		State: "start",
	}

	// 将响应消息编码为JSON
	res, err := json.Marshal(&ttsStartResponse)
	if err != nil {
		log.Errorf("JSON编码错误: %v", err)
		return err
	}

	// 发送TTS开始消息
	wsc.sendResponse(websocket.TextMessage, res)

	return nil
}

// 客户端说话结束
func (wsc *WebSocketConnection) endSpeak(sleepuTime int64) error {
	// 等待毫秒
	time.Sleep(time.Duration(sleepuTime) * time.Millisecond)

	// 发送TTS响应到客户端
	ttsStartResponse := model.ConnectionCommand{
		Type:  "tts",
		State: "stop",
	}

	// 将响应消息编码为JSON
	res, err := json.Marshal(&ttsStartResponse)
	if err != nil {
		log.Errorf("JSON编码错误: %v", err)
		return err
	}

	// 发送TTS结束消息
	wsc.sendResponse(websocket.TextMessage, res)

	return nil
}

// 客户端一句话开始
func (wsc *WebSocketConnection) startSentenceSpeak(text string) error {
	// 发送TTS响应到客户端
	ttsStartResponse := model.ConnectionCommand{
		Type:  "tts",
		State: "sentence_start",
		Text:  text,
	}

	// 将响应消息编码为JSON
	res, err := json.Marshal(&ttsStartResponse)
	if err != nil {
		log.Errorf("JSON编码错误: %v", err)
		return err
	}

	// 发送TTS开始消息
	wsc.sendResponse(websocket.TextMessage, res)

	return nil
}

// TTS 语音合成，通过访问api /tts实现
func (wsc *WebSocketConnection) TTS(text string) ([][]byte, error) {

	// 调用TTS处理函数
	audioFrames, duration, err := tts.ProcessTTS(text, wsc.connectionState.SessionId, wsc.config)
	if err != nil {
		log.Errorf("TTS处理错误: %v", err)
		return nil, err
	}
	wsc.connectionState.TTSDuration += duration * 1000

	return audioFrames, nil
}

// handleTTS 处理TTS的协程
func (wsc *WebSocketConnection) handleTTS() {
	defer log.Debugf("TTS处理协程已退出")

	for {
		select {
		case <-wsc.ctx.Done():
			// 上下文被取消，退出协程
			return
		case text, ok := <-wsc.ttsChan:
			// 通道关闭
			if !ok {
				log.Errorf("TTS通道关闭")
				return
			}

			log.Debugf("收到TTS消息: %s", text)

			// 记录当前时间，精确到毫秒
			if wsc.connectionState.StartSpeakTime == 0 {
				wsc.connectionState.StartSpeakTime = time.Now().UnixMilli()
			}

			// 如果客户端已经请求中止，不继续处理
			if wsc.connectionState.ClientAbort {
				log.Debugf("客户端请求中止，跳过TTS处理")
				continue
			}

			// 调用TTS函数处理文本
			audioFrames, err := wsc.TTS(text)
			if err != nil {
				log.Errorf("TTS处理错误: %v", err)
				continue
			}

			// 发送每一帧音频数据
			for _, frame := range audioFrames {
				// 为每一帧添加长度前缀（2字节，小端序）
				// frameLength := len(frame)
				// lengthBytes := []byte{byte(frameLength & 0xFF), byte((frameLength >> 8) & 0xFF)}

				// // 合并长度前缀和帧数据
				// frameWithLength := append(lengthBytes, frame...)

				// 发送帧数据
				wsc.sendResponse(websocket.BinaryMessage, frame)
			}

			if !wsc.connectionState.LLMFlag && len(wsc.ttsChan) == 0 {

				err := wsc.endSpeak(int64(wsc.connectionState.TTSDuration - float64(time.Now().UnixMilli()-wsc.connectionState.StartSpeakTime)))
				if err != nil {
					log.Errorf("结束说话失败: %v", err)
				}

				wsc.connectionState.TTSDuration = 0
				wsc.connectionState.StartSpeakTime = 0
			}
		}
	}
}

// handleChat 处理LLM聊天的协程
func (wsc *WebSocketConnection) handleChat() {
	defer log.Debugf("LLM处理协程已退出")

	// 监听聊天通道和上下文取消信号
	for {
		select {
		case <-wsc.ctx.Done():
			// 上下文被取消，退出协程
			return
		case text, ok := <-wsc.llmChan:
			// 通道关闭
			if !ok {
				log.Errorf("LLM通道关闭")
				return
			}

			wsc.dialogueStore = append(wsc.dialogueStore, model.Dialogue{Role: "user", Content: text})

			err := wsc.startSpeak()
			if err != nil {
				log.Errorf("开始说话失败:%v", err)
				continue
			}

			err = llm.ProcessLLM(wsc.dialogueStore, wsc.config, wsc.ttsChan, &wsc.connectionState, &wsc.connectionState.LLMFlag, &wsc.dialogueStore)
			if err != nil {
				log.Errorf("LLM处理错误:%v", err)
			}

		}
	}
}

// handleResponses 回复响应消息的协程
// 从responseChan通道读取消息并发送到WebSocket连接
func (wsc *WebSocketConnection) handleResponses() {
	defer log.Debugf("响应处理协程已退出")

	for {
		select {
		case <-wsc.ctx.Done():
			// 上下文被取消，退出协程
			log.Debugf("响应处理协程已退出")
			return

		case response, ok := <-wsc.responseChan:
			// 通道关闭
			if !ok {
				log.Debugf("响应通道已关闭")
				return
			}

			// 发送消息到WebSocket连接
			err := wsc.conn.WriteMessage(response.MessageType, response.Data)
			if err != nil {
				log.Errorf("写入消息错误: %v", err)
				// 发生错误时取消上下文，触发连接关闭
				wsc.cancelFunc()
				return
			}
		}
	}
}

// sendResponse 发送响应消息
// 参数:
//   - messageType: WebSocket消息类型
//   - data: 消息数据
func (wsc *WebSocketConnection) sendResponse(messageType int, data []byte) {
	select {
	case <-wsc.ctx.Done():
		// 上下文已取消，不发送消息
		return
	case wsc.responseChan <- ResponseMessage{MessageType: messageType, Data: data}:
		// 消息已发送到通道
	}
}

// llm
func (wsc *WebSocketConnection) send2LLM(text string) {
	select {
	case <-wsc.ctx.Done():
		// 上下文已取消，不发送消息
		return
	case wsc.llmChan <- text:
		// 消息已发送到通道
	}
}

// handleAudioMessage
func (wsc *WebSocketConnection) handleAudioMessage(data []byte) error {
	if !wsc.connectionState.ASRServerReceive {
		log.Debugf("前期数据处理中，暂停接收")
		return nil
	}

	var (
		HavaVoice bool
	)

	if wsc.connectionState.ClientListenMode == "auto" {
		HavaVoice = vad.IsVAD(&wsc.connectionState, data)
		// log.Debugf("检测语音活动，结果: %v", HavaVoice)
	} else {
		HavaVoice = wsc.connectionState.ClientHaveVoice
	}

	// 如果本次没有声音，本段也没声音，就把声音丢弃了
	if !HavaVoice && !wsc.connectionState.ClientHaveVoice {
		// log.Debugf("本次没有声音，本段也没声音，就把声音丢弃了")
		if wsc.connectionState.ClientNoVoiceLastTime == 0 {
			wsc.connectionState.ClientNoVoiceLastTime = float64(time.Now().Unix()) * 1000
		} else {
			NoVoiceDuration := float64(time.Now().Unix())*1000 - wsc.connectionState.ClientNoVoiceLastTime
			if NoVoiceDuration >= 1000*wsc.config.WebSocket.CloseConnectionTimeout {
				wsc.connectionState.ClientAbort = false
				wsc.connectionState.ASRServerReceive = false

				// 回复再见，TODO
			}
		}
		// 清除asr_audio缓存
		wsc.connectionState.ASRAudio = [][]byte{}
		return nil
	}
	wsc.connectionState.ClientNoVoiceLastTime = 0
	wsc.connectionState.ASRAudio = append(wsc.connectionState.ASRAudio, data)

	// 有声音, 且停止了，发送给ASR服务器
	if wsc.connectionState.ClientVoiceStop {
		log.Debugf("有声音，且停止了，发送给ASR服务器,len(wsc.connectionState.ASRAudio): %d", len(wsc.connectionState.ASRAudio))
		wsc.connectionState.ClientAbort = false
		wsc.connectionState.ASRServerReceive = false

		// 音频太短了，无法识别
		if len(wsc.connectionState.ASRAudio) < 3 {
			wsc.connectionState.ASRServerReceive = true
		} else {
			// 发送给ASR服务器
			log.Debugf("发送给ASR服务器")

			text, err := asr.ProcessASR(&wsc.connectionState)
			if err != nil {
				log.Errorf("ASR处理错误: %v", err)
				return err
			}

			log.Debugf("ASR识别结果: %s", text)
			wsc.send2LLM(text)

			wsc.connectionState.ASRServerReceive = true
		}

		wsc.connectionState.ASRAudio = [][]byte{}
		wsc.connectionState.ClientAudioBuffer = []byte{}
		wsc.connectionState.ClientHaveVoice = false
		wsc.connectionState.ClientHaveVoiceLastTime = 0
		wsc.connectionState.ClientVoiceStop = false
		log.Debugf("VAD states reset")
	}

	return nil
}

// handleTextMessage 处理WebSocket消息
// 参数:
//   - messageType: WebSocket消息类型，1为文本消息，2为二进制消息
//   - data: 消息数据
func (wsc *WebSocketConnection) handleTextMessage(data []byte) error {
	// json 解码
	var msg_json model.ConnectionCommand
	if err := json.Unmarshal(data, &msg_json); err != nil {
		return err
	}

	switch msg_json.Type {
	case "hello":
		// 新建连接
		var helloResponse model.ConnectionCommand = model.ConnectionCommand{
			Type:      "hello",
			Transport: "websocket",
			AudioParams: model.CommandAudioParams{
				SampleRate: wsc.config.WebSocket.SampleRate,
			},
		}
		res, err := json.Marshal(&helloResponse)
		if err != nil {
			log.Errorf("JSON编码错误: %v", err)
			return err
		}
		wsc.sendResponse(websocket.TextMessage, res)
	case "iot":
		// 处理iot消息
		wsc.connectionState.States = msg_json.States
		wsc.connectionState.Description = msg_json.Description
		log.Debugf("wsConn.connectionState.States: %+v", wsc.connectionState.States)
		log.Debugf("wsConn.connectionState.Description: %+v", wsc.connectionState.Description)
	case "abort":
		// 中止消息
		log.Infof("DeviceId(%s) 中止消息", wsc.connectionState.DeviceId)

		// 设置成打断状态，会自动打断llm、tts任务
		wsc.connectionState.ClientAbort = true

		// 打断屏显任务

		// 打断客户端说话状态
		res, err := json.Marshal(&model.ConnectionCommand{
			Type:    "tts",
			State:   "stop",
			Session: wsc.connectionState.SessionId,
		})
		if err != nil {
			log.Errorf("JSON编码错误: %v", err)
			return err
		}
		wsc.sendResponse(websocket.TextMessage, res)
	case "listen":
		if msg_json.Mode != "" {
			wsc.connectionState.ClientListenMode = msg_json.Mode
		}

		switch msg_json.State {
		case "start":
			wsc.connectionState.ClientHaveVoice = true
			wsc.connectionState.ClientVoiceStop = false
		case "stop":
			wsc.connectionState.ClientHaveVoice = true
			wsc.connectionState.ClientVoiceStop = true
		case "detect":
			// TODO,没太理解这里
			wsc.connectionState.ClientHaveVoice = false
			// wsc.connectionState.ASRServerReceive = false
			wsc.connectionState.ASRAudio = [][]byte{}
		}
	default:
		return fmt.Errorf("未知消息类型:%s", msg_json.Type)
	}

	return nil
}

// processMessage 根据消息类型处理WebSocket消息
// 参数:
//   - messageType: WebSocket消息类型
//   - data: 消息数据
//
// 返回:
//   - *APIResponse: 处理结果
//   - error: 如果处理失败，返回错误信息
func (wsc *WebSocketConnection) processMessage(messageType int, data []byte) error {
	switch messageType {
	case websocket.TextMessage:
		// 处理文本消息，例如JSON命令
		log.Debugf("处理文本消息: %s", string(data))
		return wsc.handleTextMessage(data)

	case websocket.BinaryMessage:
		// 处理二进制消息，例如音频数据
		// log.Debugf("处理二进制消息，大小: %d字节", len(data))
		return wsc.handleAudioMessage(data)

	case websocket.PingMessage:
		// 处理Ping消息，自动回复Pong
		log.Debugf("收到Ping消息")
		wsc.sendResponse(websocket.PongMessage, nil)
		return nil

	case websocket.PongMessage:
		// 处理Pong消息，通常不需要响应
		log.Debugf("收到Pong消息")
		return nil

	case websocket.CloseMessage:
		// 处理关闭消息
		log.Debugf("收到关闭消息")
		return fmt.Errorf("客户端请求关闭连接")

	default:
		// 处理未知类型的消息
		return fmt.Errorf("未知的消息类型: %d", messageType)
	}
}

// HandleConnection 处理WebSocket连接的主循环
// 负责认证、接收消息、处理消息和发送响应
func (wsc *WebSocketConnection) HandleConnection() {
	// 确保连接在函数返回时关闭
	defer func() {
		wsc.conn.Close()
		// 取消上下文，通知所有协程退出
		wsc.cancelFunc()
		// 关闭响应通道
		close(wsc.responseChan)
		log.Infof("WebSocket连接已关闭")
	}()

	// 如果启用了认证，进行认证检查
	if wsc.config.WebSocket.Auth.Enabled {
		// 读取认证消息
		_, msg, err := wsc.conn.ReadMessage()
		if err != nil {
			log.Errorf("读取认证消息失败: %v", err)
			return
		}

		// 验证令牌
		token := string(msg)
		authenticated := false
		for _, validToken := range wsc.config.WebSocket.Auth.Tokens {
			if validToken.Token == token {
				authenticated = true
				log.Infof("设备已认证: %s", validToken.Name)
				break
			}
		}

		// 如果认证失败，关闭连接
		if !authenticated {
			log.Warnf("连接认证失败")
			return
		}
	}

	log.Debugf("WebSocket连接已建立")

	// 主消息循环
	for {
		select {
		case <-wsc.ctx.Done():
			// 上下文被取消，退出循环
			return
		default:
			// 读取客户端发送的消息
			messageType, message, err := wsc.conn.ReadMessage()
			if err != nil {
				log.Errorf("读取消息错误: %v", err)
				return
			}

			// log.Debugf("收到类型为%d的消息，大小为%d字节", messageType, len(message))
			// 按照不同的messageType进行处理
			err = wsc.processMessage(messageType, message)
			if err != nil {
				log.Errorf("处理消息错误: %v", err)
				if messageType == websocket.CloseMessage {
					return // 如果是关闭消息，退出循环
				}
				continue
			}

			// // 如果没有响应或响应为空，继续下一个消息
			// if response == nil || response.Result == "" {
			// 	continue
			// }

			// // 将处理结果发送回客户端
			// log.Debugf("发送响应，大小为%d字节，内容为:%s", len(response.Result), response.Result)
			// wsc.sendResponse(messageType, []byte(response.Result))
		}
	}
}
