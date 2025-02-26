package server

import (
	"fmt"
	"net/http"
	"time"

	"lingzhi-server/config"
	"lingzhi-server/log"
)

// InitializePythonAPI 初始化Python API连接并发送配置信息
// 参数:
//   - cfg: 服务器配置信息，包含Python API的主机地址、端口等
//
// 返回:
//   - error: 如果初始化失败，返回错误信息
func InitializePythonAPI(cfg *config.Config) error {
	// 构建Python API的基础URL
	pythonAPI := fmt.Sprintf("http://%s:%d", cfg.PythonAPI.Host, cfg.PythonAPI.Port)
	// 等待Python API服务就绪
	log.Infof("等待Python API就绪，地址: %s...", pythonAPI)
	ready := false
	// 尝试连接Python API，最多等待30秒
	for i := 0; i < 30; i++ { // 最多等待30秒
		// 发送健康检查请求，检查Python API是否可用
		if resp, err := http.Get(pythonAPI + "/health"); err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			ready = true
			break
		}
		// 每次尝试间隔1秒
		time.Sleep(time.Second)
	}

	// 如果30秒后仍未就绪，返回错误
	if !ready {
		return fmt.Errorf("python API 不可用，地址: %s", pythonAPI)
	}
	log.Infof("Python API 已就绪")

	// // 读取原始配置信息，用于初始化Python API
	// rawConfig, err := config.GetRawConfig(cfg.ConfigPath)
	// if err != nil {
	// 	return fmt.Errorf("读取Python API配置失败: %v", err)
	// }

	// // 将配置信息转换为JSON格式
	// configData, err := json.Marshal(rawConfig)
	// if err != nil {
	// 	return fmt.Errorf("序列化Python API配置失败: %v", err)
	// }

	// // 发送配置信息到Python API的初始化端点
	// log.Debugf("正在发送配置信息到Python API")
	// resp, err := http.Post(pythonAPI+"/init", "application/json", bytes.NewReader(configData))
	// if err != nil || resp.StatusCode != http.StatusOK {
	// 	return fmt.Errorf("初始化Python API失败: %v", err)
	// }
	// defer resp.Body.Close()

	// 初始化成功
	log.Infof("Python API 初始化成功")
	return nil
}
