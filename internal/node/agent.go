package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Agent 存储节点代理
type Agent struct {
	config     *Config
	httpClient *http.Client
	stopCh     chan struct{}
}

// New 创建新的节点代理
func New(config *Config) (*Agent, error) {
	return &Agent{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
	}, nil
}

// Start 启动节点代理
func (a *Agent) Start() error {
	// 注册到中心服务器
	if err := a.register(); err != nil {
		return fmt.Errorf("注册失败: %w", err)
	}

	// 启动心跳
	go a.heartbeatLoop()

	// 启动HTTP服务（用于接收中心服务器的指令）
	go a.startHTTPServer()

	return nil
}

// Stop 停止节点代理
func (a *Agent) Stop() {
	close(a.stopCh)
}

// register 注册到中心服务器
func (a *Agent) register() error {
	hostname, _ := os.Hostname()

	// 获取系统信息
	totalDisk := a.getTotalDiskSpace()

	req := map[string]interface{}{
		"node_id":          a.config.NodeID,
		"ip_address":       a.config.AdvertiseIP,
		"port":             a.config.Port,
		"total_disk_space": totalDisk,
		"cpu_usage":        0,
		"memory_usage":     0,
		"disk_usage":       0,
	}

	data, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/api/v1/nodes", a.config.ServerAddr)

	resp, err := a.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("注册失败: %s", resp.Status)
	}

	log.Printf("注册成功: %s", a.config.NodeID)
	return nil
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("心跳失败: %v", err)
			}
		case <-a.stopCh:
			return
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() error {
	req := map[string]interface{}{
		"cpu_usage":    a.getCPUUsage(),
		"memory_usage": a.getMemoryUsage(),
		"disk_usage":   a.getDiskUsage(),
		"used_space":   a.getUsedSpace(),
		"status":       "online",
	}

	data, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", a.config.ServerAddr, a.config.NodeID)

	resp, err := a.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// startHTTPServer 启动HTTP服务
func (a *Agent) startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	addr := fmt.Sprintf(":%d", a.config.Port)
	log.Printf("节点HTTP服务启动在 %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("HTTP服务停止: %v", err)
	}
}

// getTotalDiskSpace 获取总磁盘空间
func (a *Agent) getTotalDiskSpace() int64 {
	// TODO: 获取实际磁盘空间
	return 1024 * 1024 * 1024 * 100 // 100GB
}

// getCPUUsage 获取CPU使用率
func (a *Agent) getCPUUsage() float64 {
	// TODO: 获取实际CPU使用率
	return 0
}

// getMemoryUsage 获取内存使用率
func (a *Agent) getMemoryUsage() float64 {
	// TODO: 获取实际内存使用率
	return 0
}

// getDiskUsage 获取磁盘使用率
func (a *Agent) getDiskUsage() float64 {
	// TODO: 获取实际磁盘使用率
	return 0
}

// getUsedSpace 获取已使用空间
func (a *Agent) getUsedSpace() int64 {
	// TODO: 获取实际已使用空间
	return 0
}
