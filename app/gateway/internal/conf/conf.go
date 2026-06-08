package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server   *Server             `yaml:"server"`
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

type JWT struct {
	Secret string `yaml:"secret"`
}

// Load 读取网关配置，并在缺省场景下补齐 P6 阶段约定的默认监听参数。
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
		bc.Server = &Server{HTTP: &Server_HTTP{Addr: ":8080"}}
	}
	if bc.Server.HTTP == nil {
		bc.Server.HTTP = &Server_HTTP{Addr: ":8080"}
	}
	if bc.Server.HTTP.Timeout == "" {
		bc.Server.HTTP.Timeout = "10s"
	}
	if bc.Services == nil {
		bc.Services = make(map[string]*Service)
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	return &bc, nil
}
