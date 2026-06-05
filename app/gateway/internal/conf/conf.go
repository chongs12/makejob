package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server   *Server             `yaml:"server"`
	Data     *Data               `yaml:"data"`
	Services map[string]*Service `yaml:"services"`
	JWT      *JWT                `yaml:"jwt"`
}

type Server struct {
	HTTP *Server_HTTP `yaml:"http"`
}
type Server_HTTP struct {
	Addr    string `yaml:"addr"`
	Timeout string `yaml:"timeout"`
}

type Service struct {
	Addr string `yaml:"addr"`
}

// Data 表示 gateway 复用 backend bridge 所需的数据源配置。
type Data struct {
	Database *Database `yaml:"database"`
}

// Database 表示 gateway 使用的数据库配置。
type Database struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
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
		bc.Server = &Server{HTTP: &Server_HTTP{Addr: ":8082"}}
	}
	if bc.Server.HTTP == nil {
		bc.Server.HTTP = &Server_HTTP{Addr: ":8082"}
	}
	if bc.Server.HTTP.Timeout == "" {
		bc.Server.HTTP.Timeout = "10s"
	}
	if bc.Services == nil {
		bc.Services = make(map[string]*Service)
	}
	if bc.Data == nil {
		bc.Data = &Data{}
	}
	if bc.Data.Database == nil {
		bc.Data.Database = &Database{}
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	return &bc, nil
}
