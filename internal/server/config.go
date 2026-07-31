package server

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 服务器配置
type Config struct {
	HTTPPort     int    `yaml:"http_port"`
	GRPCPort     int    `yaml:"grpc_port"`
	DatabasePath string `yaml:"database_path"`
	LogLevel     string `yaml:"log_level"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		HTTPPort:     8080,
		GRPCPort:     9090,
		DatabasePath: "data/constelfs.db",
		LogLevel:     "info",
	}
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
