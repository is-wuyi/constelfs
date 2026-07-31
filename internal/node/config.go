package node

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 节点配置
type Config struct {
	NodeID            string `yaml:"node_id"`
	ServerAddr        string `yaml:"server_addr"`
	AdvertiseIP       string `yaml:"advertise_ip"`
	Port              int    `yaml:"port"`
	StoragePath       string `yaml:"storage_path"`
	HeartbeatInterval int    `yaml:"heartbeat_interval"`
	LogLevel          string `yaml:"log_level"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		NodeID:            hostname,
		ServerAddr:        "http://193.134.209.37:8080",
		AdvertiseIP:       "0.0.0.0",
		Port:              8081,
		StoragePath:       "/data/constelfs",
		HeartbeatInterval: 30,
		LogLevel:          "info",
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
