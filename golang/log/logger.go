package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	// Debug 调试级别日志记录器
	Debug *log.Logger
	// Info 信息级别日志记录器
	Info *log.Logger
	// Warn 警告级别日志记录器
	Warn *log.Logger
	// Error 错误级别日志记录器
	Error *log.Logger
	// Fatal 致命错误级别日志记录器
	Fatal *log.Logger
)

// LogConfig 包含日志系统的配置信息
type LogConfig struct {
	// LogLevel 是最低输出的日志级别
	LogLevel string `yaml:"log_level"`
	// LogFile 是日志文件的路径
	LogFile string `yaml:"log_file"`
	// EnableConsole 决定是否同时将日志输出到控制台
	EnableConsole bool `yaml:"enable_console"`
	// EnableJSON 决定日志是否使用JSON格式
	EnableJSON bool `yaml:"enable_json"`
}

// LogLevel 表示日志级别
type LogLevel int

const (
	// DebugLevel 调试级别，最详细的日志信息
	DebugLevel LogLevel = iota
	// InfoLevel 信息级别，常规操作信息
	InfoLevel
	// WarnLevel 警告级别，需要注意但不是错误的情况
	WarnLevel
	// ErrorLevel 错误级别，操作失败但程序可以继续运行
	ErrorLevel
	// FatalLevel 致命错误级别，会导致程序退出的严重错误
	FatalLevel
)

// 日志级别名称映射表，用于将字符串日志级别转换为LogLevel枚举
var levelNames = map[string]LogLevel{
	"debug": DebugLevel,
	"info":  InfoLevel,
	"warn":  WarnLevel,
	"error": ErrorLevel,
	"fatal": FatalLevel,
}

// Init 根据给定的配置初始化日志系统
// 参数：
//   - config：日志配置信息，包含日志级别、文件路径等
// 返回：
//   - error：如果初始化失败，返回错误信息
func Init(config *LogConfig) error {
	// 解析日志级别，如果配置的日志级别无效，默认使用InfoLevel
	level, ok := levelNames[config.LogLevel]
	if !ok {
		level = InfoLevel
	}

	// 配置日志输出目标
	var output io.Writer

	// 如果配置了日志文件，添加文件输出
	if config.LogFile != "" {
		// 创建日志目录（如果不存在）
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("创建日志目录失败：%v", err)
		}

		// 打开日志文件，如果不存在则创建，以追加模式写入
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("打开日志文件失败：%v", err)
		}

		// 如果同时启用了控制台输出，使用多重输出
		if config.EnableConsole {
			output = io.MultiWriter(file, os.Stdout)
		} else {
			output = file
		}
	} else if config.EnableConsole {
		// 如果没有配置日志文件但启用了控制台输出，仅输出到控制台
		output = os.Stdout
	} else {
		// 如果既没有配置日志文件也没有启用控制台输出，丢弃所有日志
		output = io.Discard
	}

	// 创建带有适当标志的日志记录器
	// 日志格式：日期 时间 微秒 文件名：行号
	flags := log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile

	// 根据配置的日志级别初始化各级别的日志记录器
	// 如果当前级别低于配置的级别，将输出重定向到io.Discard（丢弃）
	if level <= DebugLevel {
		Debug = log.New(output, "\033[36mDEBUG：\033[0m", flags)
	} else {
		Debug = log.New(io.Discard, "", 0)
	}

	if level <= InfoLevel {
		Info = log.New(output, "\033[32mINFO：\033[0m", flags)
	} else {
		Info = log.New(io.Discard, "", 0)
	}

	if level <= WarnLevel {
		Warn = log.New(output, "\033[33mWARN：\033[0m", flags)
	} else {
		Warn = log.New(io.Discard, "", 0)
	}

	if level <= ErrorLevel {
		Error = log.New(output, "\033[31mERROR：\033[0m", flags)
	} else {
		Error = log.New(io.Discard, "", 0)
	}

	if level <= FatalLevel {
		Fatal = log.New(output, "\033[35mFATAL：\033[0m", flags)
	} else {
		Fatal = log.New(io.Discard, "", 0)
	}

	// 记录日志系统初始化完成的信息
	Info.Printf("日志系统已初始化，级别：%s", config.LogLevel)
	return nil
}

// Debugf 以调试级别记录格式化的消息
// 参数：
//   - format：格式化字符串
//   - args：格式化参数
func Debugf(format string, args ...interface{}) {
	// 使用Output方法而不是Printf，可以指定调用深度
	// 2表示跳过两层调用栈：Debugf函数本身和Output函数
	Debug.Output(2, fmt.Sprintf(format, args...))
}

// Infof 以信息级别记录格式化的消息
// 参数：
//   - format：格式化字符串
//   - args：格式化参数
func Infof(format string, args ...interface{}) {
	Info.Output(2, fmt.Sprintf(format, args...))
}

// Warnf 以警告级别记录格式化的消息
// 参数：
//   - format：格式化字符串
//   - args：格式化参数
func Warnf(format string, args ...interface{}) {
	Warn.Output(2, fmt.Sprintf(format, args...))
}

// Errorf 以错误级别记录格式化的消息
// 参数：
//   - format：格式化字符串
//   - args：格式化参数
func Errorf(format string, args ...interface{}) {
	Error.Output(2, fmt.Sprintf(format, args...))
}

// Fatalf 以致命错误级别记录格式化的消息，然后退出程序
// 参数：
//   - format：格式化字符串
//   - args：格式化参数
func Fatalf(format string, args ...interface{}) {
	Fatal.Output(2, fmt.Sprintf(format, args...))
	os.Exit(1)
}
