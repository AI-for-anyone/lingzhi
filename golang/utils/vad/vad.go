package vad

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"lingzhi-server/log"
	"lingzhi-server/model"
	"net/http"
	"sync"
	"time"

	"gopkg.in/hraban/opus.v2"
)

// VADConfig 表示VAD配置参数
type VADConfig struct {
	EnergyThreshold    float64 // 能量阈值，超过此值认为有语音
	SilenceThresholdMs int64   // 静默阈值（毫秒），超过此值认为一句话结束
	SampleRate         int     // 采样率
	FrameSize          int     // 每帧采样点数
	VADServerURL       string  // VAD服务器URL
}

// DefaultVADConfig 返回默认VAD配置
func DefaultVADConfig() VADConfig {
	return VADConfig{
		EnergyThreshold:    0.01,                        // 能量阈值，根据实际情况调整
		SilenceThresholdMs: 500,                         // 静默阈值500毫秒
		SampleRate:         16000,                       // 采样率16kHz
		FrameSize:          512,                         // 每帧512个采样点
		VADServerURL:       "http://localhost:8001/vad", // VAD服务器URL
	}
}

// VADProcessor 处理VAD相关功能
type VADProcessor struct {
	config VADConfig
	client *http.Client
}

// 全局VAD处理器实例和互斥锁
var (
	vadProcessor *VADProcessor
	vadOnce      sync.Once
	vadMutex     sync.Mutex
	// opusDecoder  *opus.Decoder
)

// 获取VAD处理器单例
func getVADProcessor() *VADProcessor {
	vadOnce.Do(func() {
		config := DefaultVADConfig()
		vadProcessor = NewVADProcessor(config)
	})
	return vadProcessor
}

// NewVADProcessor 创建新的VAD处理器
func NewVADProcessor(config VADConfig) *VADProcessor {
	return &VADProcessor{
		config: config,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func Init() error {
	// 尝试创建解码器以验证 Opus 库是否可用
	_, err := opus.NewDecoder(16000, 1)
	if err != nil {
		return fmt.Errorf("初始化 Opus 解码器失败: %v", err)
	}
	return nil
}

// IsVAD 外部调用函数，检测是否有语音活动
// 参数:
//   - conn: WebSocket连接状态
//   - audioData: 音频数据
//
// 返回:
//   - bool: 是否检测到语音
func IsVAD(conn *model.ConnectionState, audioData []byte) bool {
	vadMutex.Lock()
	defer vadMutex.Unlock()

	processor := getVADProcessor()

	// 调用处理器的processVAD方法
	haveVoice, err := processor.processVAD(conn, audioData)
	if err != nil {
		log.Errorf("VAD处理错误: %v", err)
		return false
	}

	return haveVoice
}

// VADRequest 表示发送到VAD服务的请求
type VADRequest struct {
	AudioData string                 `json:"audio_data"` // base64编码的音频数据
	Config    map[string]interface{} `json:"config"`
}

// VADResponse 表示从VAD服务接收的响应
type VADResponse struct {
	Status string `json:"status"`
	Result bool   `json:"result"`
}

// 进行 Opus 解码
func decodeOpus(data []byte) ([]byte, error) {
	// 创建解码器，设置采样率为16kHz，通道数为1
	decoder, err := opus.NewDecoder(16000, 1)
	if err != nil {
		return nil, fmt.Errorf("创建Opus解码器失败: %v", err)
	}

	// 分配PCM缓冲区，Opus帧大小为960个采样点
	pcmBuffer := make([]int16, 960)

	// 解码Opus数据
	samplesDecoded, err := decoder.Decode(data, pcmBuffer)
	if err != nil {
		return nil, fmt.Errorf("Opus解码失败: %v", err)
	}

	// 将解码后的数据转换为字节数组
	pcmData := make([]byte, samplesDecoded*2) // 每个采样点2字节
	for i := 0; i < samplesDecoded; i++ {
		// 小端字节序
		pcmData[i*2] = byte(pcmBuffer[i] & 0xFF)
		pcmData[i*2+1] = byte((pcmBuffer[i] >> 8) & 0xFF)
	}

	// log.Debugf("Opus解码成功，解码前大小: %d字节，解码后大小: %d字节，采样点数: %d",
	// 	len(data), len(pcmData), samplesDecoded)

	return pcmData, nil
}

// processVAD 处理VAD逻辑
// 参数:
//   - conn: WebSocket连接状态
//   - audioData: 音频数据
//
// 返回:
//   - bool: 是否检测到语音
//   - error: 处理过程中的错误
func (v *VADProcessor) processVAD(conn *model.ConnectionState, audioData []byte) (bool, error) {
	// 将音频数据转换为PCM格式（如果需要）
	// 解码 Opus 数据为 PCM 格式
	pcmData, err := decodeOpus(audioData)
	if err != nil {
		log.Errorf("Opus解码错误: %v", err)
		// 如果解码失败，尝试直接使用原始数据
		pcmData = audioData
	}
	// 将新数据加入缓冲区
	conn.ClientAudioBuffer = append(conn.ClientAudioBuffer, pcmData...)

	// 处理缓冲区中的完整帧（每次处理指定采样点数）
	clientHaveVoice := false
	frameSize := v.config.FrameSize * 2 // 每个采样点2字节(int16)

	for len(conn.ClientAudioBuffer) >= frameSize {
		// 提取前N个采样点
		chunk := conn.ClientAudioBuffer[:frameSize]
		conn.ClientAudioBuffer = conn.ClientAudioBuffer[frameSize:]

		// 调用Python VAD服务进行语音检测
		vadResult, err := v.callVADService(chunk)
		if err != nil {
			log.Errorf("调用VAD服务错误: %v", err)
			vadResult = false
		}
		clientHaveVoice = vadResult

		// 如果之前有声音，但本次没有声音，且与上次有声音的时间差已经超过了静默阈值，则认为已经说完一句话
		if conn.ClientHaveVoice && !clientHaveVoice {
			stopDuration := time.Now().UnixMilli() - int64(conn.ClientHaveVoiceLastTime)
			if stopDuration >= v.config.SilenceThresholdMs {
				conn.ClientVoiceStop = true
				log.Debugf("检测到语音结束，静默持续时间: %d ms", stopDuration)
			}
		}

		if clientHaveVoice {
			conn.ClientHaveVoice = true
			conn.ClientHaveVoiceLastTime = float64(time.Now().UnixMilli())
			// log.Debugf("检测到语音活动")
		}
	}

	return clientHaveVoice, nil
}

// callVADService 调用VAD服务
// 参数:
//   - audioData: 音频数据
//
// 返回:
//   - bool: 是否检测到语音
//   - error: 处理过程中的错误
func (v *VADProcessor) callVADService(audioData []byte) (bool, error) {
	// 确保数据长度正确（Silero VAD 需要固定的帧大小）
	// 如果数据长度不足，填充零；如果超出，截断
	frameSize := v.config.FrameSize * 2 // 每个采样点2字节
	if len(audioData) > frameSize {
		audioData = audioData[:frameSize]
	} else if len(audioData) < frameSize {
		padding := make([]byte, frameSize-len(audioData))
		audioData = append(audioData, padding...)
	}

	// 准备请求数据
	reqData := VADRequest{
		// 对音频数据进行base64编码
		AudioData: base64.StdEncoding.EncodeToString(audioData),
		Config: map[string]interface{}{
			"sample_rate": v.config.SampleRate,
			"frame_size":  v.config.FrameSize,
		},
	}

	// 将请求数据转换为JSON
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return false, err
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", v.config.VADServerURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return false, err
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// 解析响应
	var vadResp VADResponse
	if err := json.Unmarshal(respBody, &vadResp); err != nil {
		return false, err
	}

	// 返回检测结果
	return vadResp.Result, nil
}

// bytesToInt16 将字节切片转换为int16切片
func bytesToInt16(bytes []byte) []int16 {
	// 计算int16切片的长度
	length := len(bytes) / 2

	// 创建一个空的int16切片
	int16Slice := make([]int16, length)

	// 转换字节为int16（假设小端字节序）
	for i := 0; i < length; i++ {
		// 从字节切片中读取两个字节并转换为int16
		int16Slice[i] = int16(bytes[i*2]) | int16(bytes[i*2+1])<<8
	}

	return int16Slice
}
