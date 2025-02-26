package handle

import (
	"net/http"

	"lingzhi-server/config"
	"lingzhi-server/log"
	ws "lingzhi-server/websocket"

	"github.com/gorilla/websocket"
)

// upgrader 用于将HTTP连接升级为WebSocket连接
var upgrader = websocket.Upgrader{
	// 允许所有来源的跨域请求
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境中应该限制
	},
}

// HandleWebSocket 将HTTP连接升级为WebSocket并创建新的WebSocketConnection
// 参数:
//   - w: HTTP响应写入器
//   - r: HTTP请求
//   - cfg: 服务器配置
func HandleWebSocket(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 将HTTP连接升级为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("升级连接失败: %v", err)
		return
	}

	log.Infof("新的WebSocket连接来自 %s", r.RemoteAddr)

	// 为此连接创建一个新的WebSocket连接处理器
	wsConn := ws.NewWebSocketConnection(conn, r, cfg)

	// 在新的goroutine中处理连接
	go wsConn.HandleConnection()
}
