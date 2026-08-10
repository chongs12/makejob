package conf

import (
	"makejob/pkg/config"
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server    *Server    `yaml:"server"`
	Data      *Data      `yaml:"data"`
	MQ        *MQ        `yaml:"mq"`
	JWT       *JWT       `yaml:"jwt"`
	Telemetry *Telemetry `yaml:"telemetry"`
}

type Server struct {
	HTTP *Server_HTTP `yaml:"http"`
	GRPC *Server_GRPC `yaml:"grpc"`
}
type Server_HTTP struct {
	Addr string `yaml:"addr"`
}
type Server_GRPC struct {
	Addr    string `yaml:"addr"`
	Timeout string `yaml:"timeout"`
}

type Data struct {
	Database *Data_Database `yaml:"database"`
}
type Data_Database struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

// MQ RabbitMQ 消息队列配置
type MQ struct {
	URL      string `yaml:"url"`
	Exchange string `yaml:"exchange"`
}

type JWT struct {
	Secret string `yaml:"secret"`
}

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
	if bc.Server.GRPC.Timeout == "" {
		bc.Server.GRPC.Timeout = "10s"
	}
	if bc.Data == nil {
		bc.Data = &Data{}
	}
	if bc.Data.Database == nil {
		bc.Data.Database = &Data_Database{}
	}
	if bc.MQ == nil {
		bc.MQ = &MQ{}
	}
	if bc.MQ.Exchange == "" {
		bc.MQ.Exchange = "makejob.events"
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	if bc.Telemetry == nil {
		bc.Telemetry = &Telemetry{}
	}
	config.ApplyEnvOverrides(&bc)
	return &bc, nil
}

// Telemetry 可观测性配置（OTel tracing/metrics/pprof），对应 config.yaml 的 telemetry 段
type Telemetry struct {
	OTLPEndpoint string  `yaml:"otlp_endpoint"`
	ServiceName  string  `yaml:"service_name"`
	SampleRatio  float64 `yaml:"sample_ratio"`
	HTTPPort     int     `yaml:"http_port"`
}
