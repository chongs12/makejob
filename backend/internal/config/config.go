// Package config 提供应用程序配置管理功能
// 使用Viper库从YAML配置文件加载配置，支持环境变量覆盖
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// Config 应用程序全局配置结构体
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Casbin   CasbinConfig   `mapstructure:"casbin"`
}

// ServerConfig 服务器相关配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug 或 release
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// DSN 生成PostgreSQL连接字符串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// RedisConfig Redis连接配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr 返回Redis服务器地址
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig JWT令牌配置
type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	Expire        int    `mapstructure:"expire"`         // 访问令牌过期时间（小时）
	RefreshExpire int    `mapstructure:"refresh_expire"` // 刷新令牌过期时间（小时）
}

// CasbinConfig Casbin RBAC权限配置
type CasbinConfig struct {
	ModelPath string `mapstructure:"model_path"`
}

var (
	instance *Config
	once     sync.Once
)

// Load 从配置文件加载配置
// configPath 为配置文件路径，如果为空则使用默认路径 "config.yaml"
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "config.yaml"
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 支持环境变量覆盖配置
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// GetConfig 获取全局配置实例（单例模式）
// 首次调用时会从默认路径加载配置
func GetConfig() *Config {
	once.Do(func() {
		cfg, err := Load("")
		if err != nil {
			// 如果加载失败，使用默认配置
			cfg = &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "debug",
				},
				Database: DatabaseConfig{
					Host:    "localhost",
					Port:    5432,
					User:    "postgres",
					DBName:  "makejob",
					SSLMode: "disable",
				},
				Redis: RedisConfig{
					Host: "localhost",
					Port: 6379,
					DB:   0,
				},
				JWT: JWTConfig{
					Secret:        "makejob-secret-key",
					Expire:        24,
					RefreshExpire: 168,
				},
				Casbin: CasbinConfig{
					ModelPath: "config/rbac_model.conf",
				},
			}
		}
		instance = cfg
	})
	return instance
}

// SetConfig 设置全局配置实例（主要用于测试）
func SetConfig(cfg *Config) {
	instance = cfg
}
