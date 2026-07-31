package client

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 客户端配置
type Config struct {
	ServerAddr string `yaml:"server_addr"`
	LogLevel   string `yaml:"log_level"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerAddr: "http://193.134.209.37:8080",
		LogLevel:   "info",
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
