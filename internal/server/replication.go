package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ReplicationManager 分发管理器
type ReplicationManager struct {
	nodes    map[string]*Node
	mu       sync.RWMutex
	client   *http.Client
}

// ReplicationTask 分发任务
type ReplicationTask struct {
	TaskID      string    `json:"task_id"`
	ChunkID     string    `json:"chunk_id"`
	SourceNode  string    `json:"source_node"`
	TargetNodes []string  `json:"target_nodes"`
	Status      string    `json:"status"` // pending, in_progress, completed, failed
	RetryCount  int       `json:"retry_count"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// ReplicationRequest 分发请求
type ReplicationRequest struct {
	ChunkID     string   `json:"chunk_id"`
	TargetNodes []string `json:"target_nodes"`
}

// ReplicationResponse 分发响应
type ReplicationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// NewReplicationManager 创建分发管理器
func NewReplicationManager() *ReplicationManager {
	return &ReplicationManager{
		nodes:  make(map[string]*Node),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// UpdateNodes 更新节点列表
func (rm *ReplicationManager) UpdateNodes(nodes map[string]*Node) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.nodes = nodes
}

// ReplicateChunk 分发分片到副本节点
func (rm *ReplicationManager) ReplicateChunk(chunkID, sourceNode string, targetNodes []string) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	// 获取源节点信息
	source, exists := rm.nodes[sourceNode]
	if !exists {
		return fmt.Errorf("源节点不存在: %s", sourceNode)
	}
	
	// 构建分发请求
	req := ReplicationRequest{
		ChunkID:     chunkID,
		TargetNodes: targetNodes,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}
	
	// 发送分发请求到源节点
	url := fmt.Sprintf("http://%s:%d/api/v1/replicate", source.IPAddress, source.Port)
	resp, err := rm.client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("发送分发请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("分发请求失败: %s", resp.Status)
	}
	
	// 解析响应
	var replicationResp ReplicationResponse
	if err := json.NewDecoder(resp.Body).Decode(&replicationResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	
	if !replicationResp.Success {
		return fmt.Errorf("分发失败: %s", replicationResp.Error)
	}
	
	log.Printf("分片 %s 分发成功: %s -> %v", chunkID, sourceNode, targetNodes)
	
	return nil
}

// ReplicateWithRetry 带重试的分发
func (rm *ReplicationManager) ReplicateWithRetry(chunkID, sourceNode string, targetNodes []string, maxRetries int) error {
	var lastErr error
	
	for retry := 0; retry <= maxRetries; retry++ {
		if retry > 0 {
			log.Printf("重试分发: %s, 第%d次", chunkID, retry)
			time.Sleep(time.Duration(retry) * time.Second)
		}
		
		err := rm.ReplicateChunk(chunkID, sourceNode, targetNodes)
		if err == nil {
			return nil
		}
		
		lastErr = err
		log.Printf("分发失败: %v", err)
	}
	
	return fmt.Errorf("分发失败（已重试%d次）: %w", maxRetries, lastErr)
}
