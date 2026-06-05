package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Bootstrap Kratos 标准配置顶层结构
type Bootstrap struct {
	Server   *Server   `yaml:"server"`
	Data     *Data     `yaml:"data"`
	AI       *AI       `yaml:"ai"`
	Industry *Industry `yaml:"industry"`
	Archive  *Archive  `yaml:"archive"`
	MQ       *MQ       `yaml:"mq"`
	JWT      *JWT      `yaml:"jwt"`
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

type AI struct {
	ServiceAddr string `yaml:"service_addr"`
}

type Industry struct {
	ServiceAddr string `yaml:"service_addr"`
}

type Archive struct {
	ServiceAddr string `yaml:"service_addr"`
}

type MQ struct {
	URL      string `yaml:"url"`
	Exchange string `yaml:"exchange"`
}

type JWT struct {
	Secret        string `yaml:"secret"`
	ExpireHours   int    `yaml:"expire_hours"`
	ServiceSecret string `yaml:"service_secret"`
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

	// 设置默认值（带 nil 安全检查）
	if bc.Server == nil {
		bc.Server = &Server{}
	}
	if bc.Server.HTTP == nil {
		bc.Server.HTTP = &Server_HTTP{}
	}
	if bc.Server.GRPC == nil {
		bc.Server.GRPC = &Server_GRPC{}
	}
	if bc.Server.HTTP.Timeout == "" {
		bc.Server.HTTP.Timeout = "10s"
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
	if bc.Data.Database.Driver == "" {
		bc.Data.Database.Driver = "postgres"
	}
	if bc.AI == nil {
		bc.AI = &AI{}
	}
	if bc.Industry == nil {
		bc.Industry = &Industry{}
	}
	if bc.Archive == nil {
		bc.Archive = &Archive{}
	}
	if bc.MQ == nil {
		bc.MQ = &MQ{}
	}
	if bc.MQ.Exchange == "" {
		bc.MQ.Exchange = "makejob.async"
	}
	if bc.JWT == nil {
		bc.JWT = &JWT{}
	}
	if bc.JWT.ExpireHours == 0 {
		bc.JWT.ExpireHours = 168
	}

	return &bc, nil
}
