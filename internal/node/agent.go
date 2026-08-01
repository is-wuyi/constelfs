package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
)

// Agent 存储节点代理
type Agent struct {
	config     *Config
	httpClient *http.Client
	storage    *StorageEngine
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

	return &Agent{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		storage:    storage,
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

	// 分片API - 支持路径方式和查询参数方式
	// 路径方式: PUT/GET/DELETE /api/v1/chunks/{chunkID}
	// 查询方式: PUT/GET/DELETE /api/v1/chunks/upload|download|delete?chunk_id=xxx
	mux.HandleFunc("/api/v1/chunks/", a.storage.HandleChunkByPath)

	// 分发API - 接收分片并转发到其他节点
	mux.HandleFunc("/api/v1/replicate", a.storage.HandleReplicate)

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
	// 简易实现：使用goroutine数量和GOMAXPROCS估算
	// 实际生产环境应使用 /proc/stat 或 gopsutil
	numCPU := float64(runtime.NumCPU())
	numGoroutines := float64(runtime.NumGoroutine())
	// 粗略估算：goroutine数/CPU数，最大100
	usage := numGoroutines / numCPU * 5
	if usage > 100 {
		usage = 100
	}
	return usage
}

// getMemoryUsage 获取内存使用率
func (a *Agent) getMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// 使用 Sys 内存占总系统内存的比例
	// 这是一个粗略估算
	if m.Sys > 0 {
		// 假设系统有至少1GB内存
		totalMem := float64(1024 * 1024 * 1024)
		usage := float64(m.Sys) / totalMem * 100
		if usage > 100 {
			usage = 100
		}
		return usage
	}
	return 0
}
