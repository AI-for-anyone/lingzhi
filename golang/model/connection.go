package model

type ConnectionState struct {
	UserId    string
	SessionId string
	RoleId    string
	DeviceId  string
	ClientIP  string

	ClientAbort      bool
	ClientListenMode string

	// vad相关变量
	ClientAudioBuffer       []byte
	ClientHaveVoice         bool
	ClientHaveVoiceLastTime float64
	ClientNoVoiceLastTime   float64
	ClientVoiceStop         bool

	// asr相关变量
	ASRAudio         [][]byte
	ASRServerReceive bool

	LLMFlag     bool
	IoTReceived bool
	IoTHandled  bool
	// IOT 相关
	Description interface{} `json:"description,omitempty"`
	States      interface{} `json:"states,omitempty"`

	// TTS相关
	StartSpeakTime int64   `json:"start_speak_time,omitempty"`
	TTSDuration    float64 `json:"tts_duration,omitempty"`
}

type CommandAudioParams struct {
	Format        string `json:"format,omitempty"`
	SampleRate    int    `json:"sample_rate,omitempty"`
	Channels      int    `json:"channels,omitempty"`
	FrameDuration int    `json:"frame_duration,omitempty"`
}

type ConnectionCommand struct {
	Type    string `json:"type"`
	Version int    `json:"version,omitempty"`
	Session string `json:"session,omitempty"`

	Transport   string             `json:"transport,omitempty"`
	AudioParams CommandAudioParams `json:"audio_params,omitempty"`

	State  string `json:"state,omitempty"`
	Mode   string `json:"mode,omitempty"` // auto/manual/realtime
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason,omitempty"`

	Description interface{} `json:"description,omitempty"`
	States      interface{} `json:"states,omitempty"`

	Emotion string `json:"emotion,omitempty"`
}
