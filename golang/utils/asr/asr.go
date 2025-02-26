package asr

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
)

// ASRConfig 表示ASR配置参数
type ASRConfig struct {
	SampleRate   int    // 采样率
	ChannelCount int    // 通道数
	Language     string // 语言，例如 "zh-CN"
	ASRServerURL string // ASR服务器URL
	Timeout      int    // 请求超时时间（秒）
	MaxAudioSize int    // 最大音频大小（字节）
}

// DefaultASRConfig 返回默认ASR配置
func DefaultASRConfig() ASRConfig {
	return ASRConfig{
		SampleRate:   16000,                       // 采样率16kHz
		ChannelCount: 1,                           // 单通道
		Language:     "zh-CN",                     // 默认中文
		ASRServerURL: "http://localhost:8001/asr", // ASR服务器URL
		Timeout:      10,                          // 10秒超时
		MaxAudioSize: 1024 * 1024 * 2,             // 最大2MB
	}
}

// ASRProcessor 处理ASR相关功能
type ASRProcessor struct {
	config ASRConfig
	client *http.Client
}

// 全局ASR处理器实例和互斥锁
var (
	asrProcessor *ASRProcessor
	asrOnce      sync.Once
	asrMutex     sync.Mutex
)

// 获取ASR处理器单例
func getASRProcessor() *ASRProcessor {
	asrOnce.Do(func() {
		config := DefaultASRConfig()
		asrProcessor = NewASRProcessor(config)
	})
	return asrProcessor
}

// NewASRProcessor 创建新的ASR处理器
func NewASRProcessor(config ASRConfig) *ASRProcessor {
	return &ASRProcessor{
		config: config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Init 初始化ASR模块
func Init() error {
	// 初始化ASR处理器
	_ = getASRProcessor()
	return nil
}

// ASRRequest 表示发送到ASR服务的请求
type ASRRequest struct {
	AudioData []string               `json:"audio_data"` // base64编码的音频数据
	Config    map[string]interface{} `json:"config"`
}

// ASRResponse 表示从ASR服务接收的响应
type ASRResponse struct {
	Status string `json:"status"`
	Text   string `json:"text"`
}

// ProcessASR 处理语音识别
// 参数:
//   - conn: WebSocket连接状态
//
// 返回:
//   - string: 识别结果文本
//   - error: 处理过程中的错误
func ProcessASR(conn *model.ConnectionState) (string, error) {
	asrMutex.Lock()
	defer asrMutex.Unlock()

	processor := getASRProcessor()

	// 检查是否有音频数据
	if len(conn.ASRAudio) == 0 {
		return "", fmt.Errorf("没有可用的音频数据进行识别")
	}

	// 检查音频大小
	length := 0
	for _, chunk := range conn.ASRAudio {
		length += len(chunk)
	}

	if length > processor.config.MaxAudioSize {
		return "", fmt.Errorf("音频数据超过最大限制 (%d > %d 字节)",
			length, processor.config.MaxAudioSize)
	}

	// 调用ASR服务
	text, err := processor.callASRService(conn)
	if err != nil {
		log.Errorf("ASR处理错误: %v", err)
		return "", err
	}

	// 清空音频缓冲区
	conn.ASRAudio = nil

	return text, nil
}

// AddAudioData 添加音频数据到ASR处理队列
// 参数:
//   - conn: WebSocket连接状态
//   - audioData: 音频数据
func AddAudioData(conn *model.ConnectionState, audioData []byte) {
	asrMutex.Lock()
	defer asrMutex.Unlock()

	// 将音频数据添加到队列
	conn.ASRAudio = append(conn.ASRAudio, audioData)
}

// callASRService 调用ASR服务
// 参数:
//   - audioData: 音频数据
//
// 返回:
//   - string: 识别结果文本
//   - error: 处理过程中的错误
func (a *ASRProcessor) callASRService(conn *model.ConnectionState) (string, error) {
	// 准备请求数据
	var base64Audios []string
	for _, chunk := range conn.ASRAudio {
		base64Audios = append(base64Audios, base64.StdEncoding.EncodeToString(chunk))
	}

	// 构建请求配置
	config := map[string]interface{}{
		"SessionId":     conn.SessionId,
		"channel_count": a.config.ChannelCount,
		"language":      a.config.Language,
	}

	// 创建请求体
	requestBody := ASRRequest{
		AudioData: base64Audios,
		Config:    config,
	}

	// 序列化请求体
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化ASR请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", a.config.ASRServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建ASR HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送ASR请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取ASR响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ASR服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var asrResponse ASRResponse
	err = json.Unmarshal(body, &asrResponse)
	if err != nil {
		return "", fmt.Errorf("解析ASR响应失败: %v", err)
	}

	// 检查响应状态
	if asrResponse.Status != "success" {
		return "", fmt.Errorf("ASR服务返回错误状态: %s", asrResponse.Status)
	}

	// log.Debugf("ASR识别成功，结果: %s", asrResponse.Text)
	return asrResponse.Text, nil
}
