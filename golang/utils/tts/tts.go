package tts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"lingzhi-server/config"
	"lingzhi-server/log"
	"net/http"
)

var client = &http.Client{}

// TTSRequest 表示发送到TTS服务的请求
type TTSRequest struct {
	Text   string                 `json:"text"`   // 要转换为语音的文本
	Config map[string]interface{} `json:"config"` // 配置参数
}

// TTSResponse 表示从TTS服务接收的响应
type TTSResponse struct {
	Status        string   `json:"status"`         // 状态，如 "success" 或 "error"
	AudioData     []string `json:"audio_data"`     // base64编码的音频数据列表
	Duration      float64  `json:"duration"`       // 音频持续时间（秒）
	Format        string   `json:"format"`         // 音频格式，如 "opus"
	FrameDuration int      `json:"frame_duration"` // 每帧持续时间（毫秒）
}

// ProcessTTS 处理文本到语音转换
// 参数:
//   - text: 要转换为语音的文本
//   - sessionId: 会话ID，用于跟踪请求
//   - cfg: 服务器配置
//
// 返回:
//   - [][]byte: Opus音频帧列表
//   - error: 处理过程中的错误
func ProcessTTS(text string, sessionId string, cfg *config.Config) ([][]byte, float64, error) {

	url := fmt.Sprintf("http://%s:%d/tts", cfg.PythonAPI.Host, cfg.PythonAPI.Port)

	// 调用TTS服务
	audioFrames, duration, err := callTTSService(url, text, sessionId)
	if err != nil {
		log.Errorf("TTS处理错误: %v", err)
		return nil, 0, err
	}

	return audioFrames, duration, nil
}

// callTTSService 调用TTS服务
// 参数:
//   - text: 要转换为语音的文本
//   - sessionId: 会话ID
//
// 返回:
//   - [][]byte: 解码后的Opus音频帧列表
//   - error: 处理过程中的错误
func callTTSService(url string, text string, sessionId string) ([][]byte, float64, error) {
	// 准备请求配置
	config := map[string]interface{}{
		"SessionId": sessionId,
	}

	// 创建请求体
	requestBody := TTSRequest{
		Text:   text,
		Config: config,
	}

	// 序列化请求体
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, 0, fmt.Errorf("序列化TTS请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("创建TTS HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("发送TTS请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取TTS响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("TTS服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var ttsResponse TTSResponse
	if err := json.Unmarshal(body, &ttsResponse); err != nil {
		return nil, 0, fmt.Errorf("解析TTS响应失败: %v", err)
	}

	// 检查响应状态
	if ttsResponse.Status != "success" {
		return nil, 0, fmt.Errorf("TTS服务返回错误状态: %s", ttsResponse.Status)
	}

	// 解码base64音频数据
	audioFrames := make([][]byte, len(ttsResponse.AudioData))
	for i, frameBase64 := range ttsResponse.AudioData {
		frameData, err := base64.StdEncoding.DecodeString(frameBase64)
		if err != nil {
			return nil, 0, fmt.Errorf("解码base64音频数据失败: %v", err)
		}

		// 直接存储原始Opus帧数据
		audioFrames[i] = frameData
	}

	return audioFrames, ttsResponse.Duration, nil
}
