package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Bootstrap Kratos 标准配置顶层结构
type Bootstrap struct {
	Server             *Server             `yaml:"server"`
	Data               *Data               `yaml:"data"`
	JWT                *JWT                `yaml:"jwt"`
	Volcengine         *Volcengine         `yaml:"volcengine"`
	DependentServices  *DependentServices  `yaml:"dependent_services"`
	Telemetry  *Telemetry  `yaml:"telemetry"`
}

type Server struct {
	HTTP *Server_HTTP `yaml:"http"`
	GRPC *Server_GRPC `yaml:"grpc"`
}
type Server_HTTP struct {
	Addr    string `yaml:"addr"`
	Timeout string `yaml:"timeout"`
}
type Server_GRPC struct {
	Addr    string `yaml:"addr"`
	Timeout string `yaml:"timeout"`
}

type Data struct {
	Database *Data_Database `yaml:"database"`
	Redis    *Data_Redis    `yaml:"redis"`
}
type Data_Database struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}
type Data_Redis struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWT struct {
	Secret        string `yaml:"secret"`
	ExpireHours   int    `yaml:"expire_hours"`
	ServiceSecret string `yaml:"service_secret"`
}

// Volcengine 火山引擎语音服务配置（对齐单体 volcengine.realtime 段）
type Volcengine struct {
	// 基础连接
	AppID  string `yaml:"app_id"`
	Token  string `yaml:"token"`
	WSUrl  string `yaml:"ws_url"`

	// 实时对话高级配置（对齐单体 VolcRealtimeDialogConfig）
	Enabled         bool   `yaml:"enabled"`
	BaseURL         string `yaml:"base_url"`
	AccessToken     string `yaml:"access_token"`
	AppKey          string `yaml:"app_key"`
	ResourceID      string `yaml:"resource_id"`
	Speaker         string `yaml:"speaker"`
	InputMode       string `yaml:"input_mode"`
	AudioFormat     string `yaml:"audio_format"`
	SampleRate      int    `yaml:"sample_rate"`
	TTSFormat       string `yaml:"tts_format"`
	TTSSampleRate   int    `yaml:"tts_sample_rate"`
	BotName         string `yaml:"bot_name"`
	SystemRole      string `yaml:"system_role"`
	SpeakingStyle   string `yaml:"speaking_style"`
	CharacterPrompt string `yaml:"character_prompt"`
	LocationCity    string `yaml:"location_city"`
	RecvTimeout     int    `yaml:"recv_timeout"`
}

// DependentServices 下游依赖服务地址配置
type DependentServices struct {
	InterviewAddr string `yaml:"interview_addr"`
	RAGAddr       string `yaml:"rag_addr"`
}

// Load 从 YAML 文件加载配置
func Load(path string) (*Bootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bc Bootstrap
	if err := yaml.Unmarshal(data, &bc); err != nil {
		return nil, err
	}
	if bc.Server == nil {
		bc.Server = &Server{}
	}
	if bc.Server.HTTP == nil {
		bc.Server.HTTP = &Server_HTTP{}
	}
	if bc.Server.GRPC == nil {
		bc.Server.GRPC = &Server_GRPC{}
	}
	if bc.Data == nil {
		bc.Data = &Data{}
	}
	if bc.Data.Database == nil {
		bc.Data.Database = &Data_Database{}
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	if bc.Volcengine == nil {
		bc.Volcengine = &Volcengine{}
	}
	if bc.DependentServices == nil {
		bc.DependentServices = &DependentServices{}
	}
	if bc.Server.HTTP.Timeout == "" {
		bc.Server.HTTP.Timeout = "10s"
	}
	if bc.Server.GRPC.Timeout == "" {
		bc.Server.GRPC.Timeout = "10s"
	}
	if bc.Data.Database.Driver == "" {
		bc.Data.Database.Driver = "postgres"
	}
	if bc.JWT.ExpireHours == 0 {
		bc.JWT.ExpireHours = 168
	}
	if bc.Telemetry == nil {
		bc.Telemetry = &Telemetry{}
	}
	return &bc, nil
}

// Telemetry 可观测性配置（OTel tracing/metrics/pprof），对应 config.yaml 的 telemetry 段
type Telemetry struct {
	OTLPEndpoint string  `yaml:"otlp_endpoint"`
	ServiceName  string  `yaml:"service_name"`
	SampleRatio  float64 `yaml:"sample_ratio"`
	HTTPPort     int     `yaml:"http_port"`
}
