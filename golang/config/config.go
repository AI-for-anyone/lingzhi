package config

import (
	"fmt"
	"os"

	"lingzhi-server/log"

	"gopkg.in/yaml.v3"
)

// Config 表示服务器的完整配置
type Config struct {
	WebSocket  WebSocketConfig `yaml:"websocket"`  // WebSocket服务器配置
	HTTP       HTTPConfig      `yaml:"http"`       // HTTP服务器配置
	PythonAPI  PythonAPIConfig `yaml:"python_api"` // Python API配置
	Log        log.LogConfig   `yaml:"log"`        // 日志配置
	ConfigPath string          `yaml:"-"`          // 配置文件路径，不存储在YAML中
	LLM        LLMConfig       `yaml:"llm"`        // LLM配置
}

// LLMConfig 表示LLM服务器的配置
type LLMConfig struct {
	URL          string `yaml:"url"`           // LLM服务器URL
	Timeout      int    `yaml:"timeout"`       // LLM服务器超时时间
	SystemPrompt string `yaml:"system_prompt"` // 系统提示
}

// WebSocketConfig 表示WebSocket服务器的配置
type WebSocketConfig struct {
	Host string `yaml:"host"` // 服务器主机地址，如"0.0.0.0"表示所有网络接口
	Port int    `yaml:"port"` // 服务器端口
	Auth struct {
		Enabled bool `yaml:"enabled"` // 是否启用认证
		Tokens  []struct {
			Token string `yaml:"token"` // 认证令牌
			Name  string `yaml:"name"`  // 设备名称
		} `yaml:"tokens"` // 有效的认证令牌列表
		AllowedDevices []string `yaml:"allowed_devices,omitempty"` // 允许连接的设备列表（可选）
	} `yaml:"auth"` // 认证配置

	SampleRate             int     `yaml:"sample_rate"`              // 采样率
	CloseConnectionTimeout float64 `yaml:"close_connection_timeout"` // 关闭连接时长
}

// HTTPConfig 表示HTTP服务器的配置
type HTTPConfig struct {
	IP   string `yaml:"ip"`   // 服务器IP地址
	Port int    `yaml:"port"` // 服务器端口
}

// PythonAPIConfig 表示Python API的配置
type PythonAPIConfig struct {
	Host    string `yaml:"host"`    // API主机地址
	Port    int    `yaml:"port"`    // API端口
	Timeout int    `yaml:"timeout"` // API超时时间
}

// LoadConfig 从YAML文件加载配置
// 参数:
//   - configPath: 配置文件路径
//
// 返回:
//   - *Config: 加载的配置对象
//   - error: 如果加载失败，返回错误信息
func LoadConfig(configPath string) (*Config, error) {
	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML内容到Config结构体
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 存储配置文件路径
	cfg.ConfigPath = configPath

	// 设置日志配置的默认值（如果未指定）
	if cfg.Log.LogLevel == "" {
		cfg.Log.LogLevel = "info" // 默认日志级别为info
	}
	if cfg.Log.LogFile == "" {
		cfg.Log.LogFile = "logs/server.log" // 默认日志文件路径
	}
	if !cfg.Log.EnableConsole {
		cfg.Log.EnableConsole = true // 默认启用控制台输出
	}

	return &cfg, nil
}

// GetRawConfig 从YAML文件读取原始配置
// 参数:
//   - configPath: 配置文件路径
//
// 返回:
//   - map[string]interface{}: 原始配置映射
//   - error: 如果读取失败，返回错误信息
func GetRawConfig(configPath string) (map[string]interface{}, error) {
	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML内容到通用映射
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return rawConfig, nil
}
