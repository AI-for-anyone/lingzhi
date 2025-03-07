# Lingzhi - 智能语音对话系统

本项目为开源智能硬件项目 [xiaozhi-esp32](https://github.com/78/xiaozhi-esp32)提供golang+python后端服务框架，适用于高并发场景。
根据 [小智通信协议](https://ccnphfhqs21z.feishu.cn/wiki/M0XiwldO9iJwHikpXD5cEx71nKh) 实现。
Lingzhi 是一个完整的智能语音对话系统，结合了语音活动检测(VAD)、语音识别(ASR)、大语言模型(LLM)和语音合成(TTS)功能，提供流畅的语音交互体验。系统采用 Go 语言和 Python 混合架构，通过 WebSocket 实现实时音频流处理。

## 🌟 主要特点

- **混合架构设计**：Go 语言实现高性能 WebSocket 服务器，Python 实现核心 AI 处理功能
- **完整语音处理流水线**：VAD → ASR → LLM → TTS
- **多种模型支持**：
  - **VAD**：基于 Silero VAD
  - **ASR**：支持 FunASR、火山引擎等
  - **LLM**：支持 Ollama、OpenAI、ChatGLM、Gemini、Dify 等多种大语言模型
  - **TTS**：支持 Edge TTS、火山引擎、CosyVoice 等多种语音合成服务
- **实时流式处理**：支持音频数据的流式处理和响应
- **高度可配置**：通过 YAML 配置文件灵活配置各组件参数
- **跨平台支持**：支持 Linux、macOS 和 Windows

## 📋 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  客户端设备      │    │  Go WebSocket   │    │  Python API     │
│  (App/物联网)    │◄──►│  服务器         │◄──►│  服务           │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                      │
                                                      ▼
                                        ┌─────────────────────────┐
                                        │  AI 处理模块            │
                                        │  - VAD (语音活动检测)   │
                                        │  - ASR (语音识别)       │
                                        │  - LLM (大语言模型)     │
                                        │  - TTS (语音合成)       │
                                        └─────────────────────────┘
```

### VAD 模块

我们实现了一个基于能量检测的语音活动检测（VAD）系统，主要特点：

- 使用纯 Go 实现，不依赖外部 C/C++ 库
- 基于音频能量计算进行语音检测
- 使用单例模式管理 VAD 处理器
- 线程安全设计

## 🔧 安装与配置

### 系统要求

- Go 1.18+
- Conda 

### 依赖安装

1. 安装系统依赖：

```bash
sudo apt-get install pkg-config libopus-dev libopusfile-dev gcc
```

2. 安装 Python 环境：

```bash
# 使用 Conda 创建环境
cd python
conda env create -f environment.yml
```

3. 安装ASR模型
默认使用SenseVoiceSmall模型，进行语音转文字。因为模型较大，需要独立下载，下载后把model.pt 文件放在python/model/SenseVoiceSmall 目录下。
huggingface 搜索SenseVoiceSmall，下载model.pt文件。

4. 编译 Go 服务器：

```bash
cd golang
go mod tidy
go build -o lingzhi-server
```

### 配置

1. 复制示例配置文件：

```bash
cp config.yaml.example config.yaml
cp go_conf.yaml.example go_conf.yaml
```

2. 编辑 `config.yaml` 文件，配置各模块参数：
   - 服务器地址和端口
   - VAD、ASR、LLM、TTS 模块选择和参数
   - 认证信息（可选）

## 🚀 使用方法

### 启动服务

1. 启动 Python API 服务：

```bash
# 使用 Conda 环境
conda activate lingzhi && python python/api.py

2. 启动 Go WebSocket 服务器：

```bash
cd golang && ./lingzhi-server --config ../go_conf.yaml
```

### 客户端连接

客户端可以通过 WebSocket 连接到服务器：

```
ws://服务器IP:端口
```

## 📝 API 文档

### WebSocket 消息格式

#### 客户端到服务器

```json
{
  "type": "audio",
  "data": "base64编码的音频数据"
}
```

#### 服务器到客户端

```json
{
  "type": "tts",
  "state": "start|speaking|end",
  "text": "文本内容"
}
```

## 🔄 处理流程

1. 客户端通过 WebSocket 发送音频数据
2. VAD 模块检测是否包含语音
3. 如果检测到语音，ASR 模块将语音转换为文本
4. LLM 模块处理文本并生成响应
5. TTS 模块将响应转换为语音
6. 服务器将语音数据发送回客户端

## 🛠️ 自定义与扩展

### 添加新的 LLM 模型

在 `python/core/providers/llm/` 目录下创建新的适配器，并在配置文件中添加相应配置。

### 添加新的 TTS 引擎

在 `python/core/providers/tts/` 目录下创建新的适配器，并在配置文件中添加相应配置。

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎贡献代码、报告问题或提出改进建议！请遵循以下步骤：

1. Fork 本仓库
2. 创建您的特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启一个 Pull Request

## 📞 联系方式

如有任何问题或建议，请通过 Issues 与我们联系。
本人每个人都能享受AI时代的到来
深圳如果有志同道合并且想干点事情的朋友，可以加微信: hust_sai47

---

**Lingzhi** - 让语音交互更智能、更自然！
