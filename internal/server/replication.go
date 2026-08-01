package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// NewReplicationManager 创建分发管理器
func NewReplicationManager() *ReplicationManager {
	return &ReplicationManager{
		nodes:  make(map[string]*Node),
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// UpdateNodes 更新节点列表
func (rm *ReplicationManager) UpdateNodes(nodes map[string]*Node) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.nodes = nodes
}

// ReplicateChunk 分发分片到副本节点
// 通过通知源节点，让源节点将分片推送到目标节点
func (rm *ReplicationManager) ReplicateChunk(chunkID, sourceNodeID string, targetNodeIDs []string) error {
	rm.mu.RLock()
	source, exists := rm.nodes[sourceNodeID]
	rm.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("源节点不存在: %s", sourceNodeID)
	}
	
	// 将节点ID转换为地址列表
	rm.mu.RLock()
	var targetAddrs []string
	for _, nodeID := range targetNodeIDs {
		if node, ok := rm.nodes[nodeID]; ok {
			targetAddrs = append(targetAddrs, fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
		}
	}
	rm.mu.RUnlock()
	
	if len(targetAddrs) == 0 {
		return fmt.Errorf("没有可用的目标节点")
	}
	
	// 构建分发请求 — 通知源节点将分片推送到目标节点
	reqBody := map[string]interface{}{
		"chunk_id":     chunkID,
		"target_nodes": targetAddrs,
	}
	
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}
	
	// 发送分发请求到源节点
	url := fmt.Sprintf("http://%s:%d/api/v1/replicate", source.IPAddress, source.Port)
	
	// 重试3次
	var lastErr error
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			log.Printf("重试分发请求: %s, 第%d次", chunkID, retry)
			time.Sleep(time.Duration(retry) * time.Second)
		}
		
		resp, err := rm.client.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			lastErr = fmt.Errorf("发送分发请求失败: %w", err)
			continue
		}
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("分发请求返回 %d: %s", resp.StatusCode, string(body))
			continue
		}
		
		// 检查响应
		var result struct {
			Success      bool `json:"success"`
			SuccessCount int  `json:"success_count"`
			TotalCount   int  `json:"total_count"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("解析响应失败: %w", err)
			continue
		}
		
		if !result.Success {
			lastErr = fmt.Errorf("分发失败: %d/%d成功", result.SuccessCount, result.TotalCount)
			continue
		}
		
		log.Printf("分片 %s 分发成功: %s -> %v (%d/%d)", chunkID, sourceNodeID, targetNodeIDs, result.SuccessCount, result.TotalCount)
		return nil
	}
	
	return fmt.Errorf("分发失败（已重试3次）: %w", lastErr)
}

// ReplicateWithRetry 带重试的分发（已内置在ReplicateChunk中）
func (rm *ReplicationManager) ReplicateWithRetry(chunkID, sourceNode string, targetNodes []string, maxRetries int) error {
	return rm.ReplicateChunk(chunkID, sourceNode, targetNodes)
}
