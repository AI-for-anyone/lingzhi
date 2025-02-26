package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"lingzhi-server/config"
	"lingzhi-server/log"
	"lingzhi-server/model"
	"net/http"
	"strings"
	"time"
)

// LLMResponse 表示从 LLM 服务接收到的响应
type LLMResponse struct {
	Status  string `json:"status"`
	Chunk   string `json:"chunk,omitempty"`
	Message string `json:"message,omitempty"`
}

// LLMRequest 表示发送给 LLM 服务的请求
type LLMRequest struct {
	Dialogue []model.Dialogue       `json:"dialogue"`
	Config   map[string]interface{} `json:"config"`
}

// ProcessLLM 向 LLM 服务发送对话请求并处理流式响应
// 参数:
//   - dialogueStore: 对话历史
//   - cfg: 服务器配置
//   - chatChan: 用于发送 LLM 响应的通道
//   - connectionState: 连接状态
//
// 返回:
//   - error: 如果处理失败，返回错误信息
func ProcessLLM(dialogueStore []model.Dialogue, cfg *config.Config, ttsChan chan string, connectionState *model.ConnectionState, llmFlag *bool, dialogues *[]model.Dialogue) error {
	// 创建 LLM 请求
	llmConfig := make(map[string]interface{})
	llmConfig["SessionId"] = connectionState.SessionId

	// 如果有系统提示，可以在这里添加
	if cfg.LLM.SystemPrompt != "" {
		llmConfig["system_prompt"] = cfg.LLM.SystemPrompt
	}

	// 创建请求体
	requestBody := LLMRequest{
		Dialogue: dialogueStore,
		Config:   llmConfig,
	}

	// 将请求体转换为 JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("JSON 编码错误: %v", err)
		return err
	}

	// 创建 HTTP 请求
	llmURL := fmt.Sprintf("%s/llm", cfg.LLM.URL)
	req, err := http.NewRequest("POST", llmURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("创建 HTTP 请求错误: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{
		Timeout: time.Duration(cfg.LLM.Timeout) * time.Second,
	}

	log.Debugf("发送 LLM 请求: %s", requestBody.Dialogue)

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送 HTTP 请求错误: %v", err)
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Errorf("LLM 服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return fmt.Errorf("LLM 服务返回错误状态码: %d", resp.StatusCode)
	}

	// 处理流式响应
	reader := bufio.NewReader(resp.Body)
	fullResponse := ""

	*llmFlag = true
	defer func() {
		*llmFlag = false
	}()

	for {
		// 检查是否需要中止处理
		if connectionState.ClientAbort {
			log.Debugf("LLM 处理被中止")
			return nil
		}

		// 读取一行响应
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Errorf("读取响应错误: %v", err)
			return err
		}

		// 跳过空行
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析 JSON 响应
		var llmResponse LLMResponse
		if err := json.Unmarshal([]byte(line), &llmResponse); err != nil {
			log.Errorf("解析 JSON 响应错误: %v, 响应: %s", err, line)
			continue
		}

		// 处理不同类型的响应
		switch llmResponse.Status {
		case "streaming":
			// 处理流式响应
			if llmResponse.Chunk != "" {
				log.Debugf("收到 LLM 流式响应: %s", llmResponse.Chunk)
				fullResponse += llmResponse.Chunk

				// 发送响应到通道
				select {
				case ttsChan <- llmResponse.Chunk:
					// 成功发送到通道
				default:
					// 通道已满或关闭，记录警告
					log.Warnf("无法发送 LLM 响应到通道")
				}
			}
		case "warning":
			// 处理警告
			log.Warnf("LLM 服务警告: %s", llmResponse.Message)
		case "complete":
			// 处理完成信号
			log.Debugf("LLM 处理完成，完整响应: %s", llmResponse.Message)
			*dialogues = append(*dialogues, model.Dialogue{
				Role:    "assistant",
				Content: llmResponse.Message,
			})
			return nil
		default:
			// 处理未知状态
			log.Warnf("未知的 LLM 响应状态: %s", llmResponse.Status)
		}
	}

	// 如果没有收到完成信号但已经读取完所有响应，标记为完成
	connectionState.LLMFlag = true
	return nil
}
