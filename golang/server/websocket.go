package server

import (
	"fmt"
	"net/http"

	"lingzhi-server/config"
	"lingzhi-server/handle"
	"lingzhi-server/log"
	"lingzhi-server/utils"
)

// StartWebSocketServer 启动WebSocket服务器
// 参数:
//   - cfg: 服务器配置信息，包含WebSocket服务器的主机地址、端口和认证信息等
//
// 返回:
//   - error: 如果服务器启动失败，返回错误信息
func StartWebSocketServer(cfg *config.Config) error {
	// 获取本机IP地址，用于日志显示
	localIP := utils.GetLocalIP()

	// 设置WebSocket处理函数，将所有根路径的请求交给handleWebSocket处理
	// 当有新的WebSocket连接请求时，会调用这个处理函数
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handle.HandleWebSocket(w, r, cfg)
	})

	// 构建服务器地址字符串（格式：主机:端口）
	addr := fmt.Sprintf("%s:%d", cfg.WebSocket.Host, cfg.WebSocket.Port)
	// 记录服务器启动信息到日志
	log.Infof("正在启动WebSocket服务器，监听地址: %s (本机IP: %s)", addr, localIP)
	// 启动HTTP服务器，监听指定地址，这会阻塞当前goroutine直到服务器关闭
	return http.ListenAndServe(addr, nil)
}
