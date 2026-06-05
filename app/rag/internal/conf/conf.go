package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Bootstrap Kratos 标准配置顶层结构
type Bootstrap struct {
	Server *Server `yaml:"server"`
	RAG    *RAG    `yaml:"rag"`
	MQ     *MQ     `yaml:"mq"`
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

// RAG 向量检索配置
type RAG struct {
	MilvusAddr     string `yaml:"milvus_addr"`
	CollectionName string `yaml:"collection_name"`
	ArkAPIKey      string `yaml:"ark_api_key"`
	ArkBaseURL     string `yaml:"ark_base_url"`
	EmbedModel     string `yaml:"embed_model"`
	TopK           int    `yaml:"top_k"`
}

// MQ RabbitMQ 消息队列配置
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
	if bc.RAG == nil {
		bc.RAG = &RAG{}
	}
	if bc.RAG.MilvusAddr == "" {
		bc.RAG.MilvusAddr = "localhost:19530"
	}
	if bc.RAG.CollectionName == "" {
		bc.RAG.CollectionName = "makejob_questions"
	}
	if bc.RAG.ArkBaseURL == "" {
		bc.RAG.ArkBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	if bc.RAG.EmbedModel == "" {
		bc.RAG.EmbedModel = "doubao-embedding-large-text-240915"
	}
	if bc.RAG.TopK == 0 {
		bc.RAG.TopK = 5
	}
	if bc.MQ == nil {
		bc.MQ = &MQ{}
	}
	if bc.MQ.Exchange == "" {
		bc.MQ.Exchange = "makejob.async"
	}
	return &bc, nil
}
