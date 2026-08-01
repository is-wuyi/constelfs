package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Agent 存储节点代理
type Agent struct {
	config     *Config
	httpClient *http.Client
	storage    *StorageEngine
	speedTest  *SpeedTester
	stopCh     chan struct{}
}

// New 创建新的节点代理
func New(config *Config) (*Agent, error) {
	// 创建存储引擎
	storage := NewStorageEngine(config)

	// 初始化存储目录
	if err := storage.Init(); err != nil {
		return nil, fmt.Errorf("初始化存储失败: %w", err)
	}

	// 创建测速器
	speedTest := NewSpeedTester(config)

	return &Agent{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		storage:    storage,
		speedTest:  speedTest,
		stopCh:     make(chan struct{}),
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

	// 启动定时测速
	go a.speedTestLoop()

	// 启动HTTP服务
	go a.startHTTPServer()

	return nil
}

// Stop 停止节点代理
func (a *Agent) Stop() {
	close(a.stopCh)
}

// register 注册到中心服务器
func (a *Agent) register() error {
	totalDisk := a.getTotalDiskSpace()

	req := map[string]interface{}{
		"node_id":          a.config.NodeID,
		"ip_address":       a.config.AdvertiseIP,
		"port":             a.config.Port,
		"total_disk_space": totalDisk,
		"cpu_usage":        a.getCPUUsage(),
		"memory_usage":     a.getMemoryUsage(),
		"disk_usage":       a.getDiskUsage(),
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

// speedTestLoop 测速循环
func (a *Agent) speedTestLoop() {
	// 启动时测速一次
	a.runSpeedTest()

	// 定时测速
	ticker := time.NewTicker(2 * time.Hour) // 默认2小时测速一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.runSpeedTest()
		case <-a.stopCh:
			return
		}
	}
}

// runSpeedTest 运行测速
func (a *Agent) runSpeedTest() {
	result, err := a.speedTest.RunSpeedTest()
	if err != nil {
		log.Printf("测速失败: %v", err)
		return
	}

	// 上报测速结果到中心服务器
	if err := a.reportSpeedTest(result); err != nil {
		log.Printf("上报测速结果失败: %v", err)
	}
}

// reportSpeedTest 上报测速结果
func (a *Agent) reportSpeedTest(result *SpeedTestResult) error {
	data, _ := json.Marshal(result)
	url := fmt.Sprintf("%s/api/v1/nodes/%s/speedtest", a.config.ServerAddr, a.config.NodeID)

	resp, err := a.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上报测速结果失败: %s", resp.Status)
	}

	return nil
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() error {
	// 获取存储信息
	storageInfo := a.storage.GetStorageInfo()

	req := map[string]interface{}{
		"cpu_usage":    a.getCPUUsage(),
		"memory_usage": a.getMemoryUsage(),
		"disk_usage":   a.getDiskUsage(),
		"used_space":   storageInfo["total_size"],
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

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// 存储API
	mux.HandleFunc("/api/v1/upload", a.storage.HandleUpload)
	mux.HandleFunc("/api/v1/download", a.storage.HandleDownload)
	mux.HandleFunc("/api/v1/delete", a.storage.HandleDelete)

	// 存储信息
	mux.HandleFunc("/api/v1/storage", func(w http.ResponseWriter, r *http.Request) {
		info := a.storage.GetStorageInfo()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})

	addr := fmt.Sprintf(":%d", a.config.Port)
	log.Printf("节点HTTP服务启动在 %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("HTTP服务停止: %v", err)
	}
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
