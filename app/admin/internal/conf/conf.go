package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Bootstrap Kratos 标准配置顶层结构
type Bootstrap struct {
	Server            *Server            `yaml:"server"`
	Data              *Data              `yaml:"data"`
	JWT               *JWT               `yaml:"jwt"`
	MQ                *MQ                `yaml:"mq"`
	DependentServices *DependentServices `yaml:"dependent_services"`
	Telemetry  *Telemetry  `yaml:"telemetry"`
}

// DependentServices 下游微服务地址配置
type DependentServices struct {
	UserAddr      string `yaml:"user_addr"`
	QuestionAddr  string `yaml:"question_addr"`
	InterviewAddr string `yaml:"interview_addr"`
	AIGatewayAddr string `yaml:"ai_gateway_addr"`
	RAGAddr       string `yaml:"rag_addr"`
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

// MQ 描述 Admin 服务使用的 RabbitMQ 连接与交换机配置。
type MQ struct {
	URL      string `yaml:"url"`
	Exchange string `yaml:"exchange"`
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

	// 设置默认值
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
	if bc.MQ == nil {
		bc.MQ = &MQ{}
	}
	if bc.MQ.Exchange == "" {
		bc.MQ.Exchange = "makejob.async"
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
