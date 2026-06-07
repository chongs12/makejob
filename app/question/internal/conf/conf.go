package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server *Server `yaml:"server"`
	Data   *Data   `yaml:"data"`
	AI     *AI     `yaml:"ai"`
	JWT    *JWT    `yaml:"jwt"`
	MQ     *MQ     `yaml:"mq"`
}

type MQ struct {
	URL string `yaml:"url"`
}

type Server struct {
	HTTP *Server_HTTP `yaml:"http"`
	GRPC *Server_GRPC `yaml:"grpc"`
}
type Server_HTTP struct{ Addr string `yaml:"addr"` }
type Server_GRPC struct{ Addr string `yaml:"addr"` }

type Data struct {
	Database *Data_Database `yaml:"database"`
}
type Data_Database struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

type AI struct {
	ServiceAddr    string `yaml:"service_addr"`
	CodeRunnerAddr string `yaml:"coderunner_addr"`
	AIGatewayAddr  string `yaml:"ai_gateway_addr"`
	RAGAddr        string `yaml:"rag_addr"`
}
type JWT struct{ Secret string `yaml:"secret"` }

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
	if bc.AI == nil {
		bc.AI = &AI{}
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	if bc.MQ == nil {
		bc.MQ = &MQ{URL: "amqp://guest:guest@localhost:5672/"}
	}
	return &bc, nil
}
