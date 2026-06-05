package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server *Server `yaml:"server"`
	Data   *Data   `yaml:"data"`
	JWT    *JWT    `yaml:"jwt"`
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
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	return &bc, nil
}
