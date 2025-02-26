package main

import (
	"flag"
	"fmt"
	"os"

	"lingzhi-server/config"
	"lingzhi-server/log"
	"lingzhi-server/server"
	"lingzhi-server/utils"
)

func main() {
	// 定义命令行参数，用于指定配置文件路径
	// 默认配置文件为当前目录下的config.yaml
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	// 解析命令行参数
	flag.Parse()

	// 加载配置文件
	// 如果配置文件加载失败，打印错误信息并退出程序
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	// 根据配置文件中的日志配置初始化日志系统
	if err := log.Init(&cfg.Log); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 记录服务器启动信息
	log.Infof("正在启动lingzhi-server...")
	log.Infof("已加载配置文件: %s", *configPath)

	// 初始化Python API
	// 连接到Python API服务并发送配置信息
	if err := server.InitializePythonAPI(cfg); err != nil {
		log.Fatalf("初始化Python API失败: %v", err)
	}

	// 初始化util库
	if err := utils.Init(); err != nil {
		log.Fatalf("初始化util库失败: %v", err)
	}

	// 启动WebSocket服务器
	// 这会阻塞当前goroutine直到服务器关闭
	if err := server.StartWebSocketServer(cfg); err != nil {
		log.Fatalf("WebSocket服务器错误: %v", err)
	}
}
