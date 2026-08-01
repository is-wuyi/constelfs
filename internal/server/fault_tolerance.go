package server

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// FaultToleranceManager 容错管理器
type FaultToleranceManager struct {
	maxRetries    int
	retryInterval time.Duration
	mu            sync.RWMutex
}

// WriteResult 写入结果
type WriteResult struct {
	Success    bool     `json:"success"`
	ChunkID    string   `json:"chunk_id"`
	NodeID     string   `json:"node_id"`
	Error      string   `json:"error,omitempty"`
	RetryCount int      `json:"retry_count"`
}

// NewFaultToleranceManager 创建容错管理器
func NewFaultToleranceManager(maxRetries int, retryInterval time.Duration) *FaultToleranceManager {
	return &FaultToleranceManager{
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
	}
}

// WriteWithRetry 带重试的写入
func (ftm *FaultToleranceManager) WriteWithRetry(writeFunc func() error, description string) error {
	var lastErr error
	
	for retry := 0; retry <= ftm.maxRetries; retry++ {
		if retry > 0 {
			log.Printf("重试写入: %s, 第%d次", description, retry)
			time.Sleep(ftm.retryInterval * time.Duration(retry))
		}
		
		err := writeFunc()
		if err == nil {
			return nil
		}
		
		lastErr = err
		log.Printf("写入失败: %s, 错误: %v", description, err)
	}
	
	return fmt.Errorf("写入失败（已重试%d次）: %w", ftm.maxRetries, lastErr)
}

// WriteToMultipleNodes 写入到多个节点
func (ftm *FaultToleranceManager) WriteToMultipleNodes(
	chunkID string,
	data []byte,
	nodes []string,
	writeFunc func(nodeID string, data []byte) error,
) []WriteResult {
	results := make([]WriteResult, len(nodes))
	var wg sync.WaitGroup
	
	for i, nodeID := range nodes {
		wg.Add(1)
		go func(index int, node string) {
			defer wg.Done()
			
			err := ftm.WriteWithRetry(func() error {
				return writeFunc(node, data)
			}, fmt.Sprintf("chunk-%s-node-%s", chunkID, node))
			
			results[index] = WriteResult{
				Success: err == nil,
				ChunkID: chunkID,
				NodeID:  node,
			}
			
			if err != nil {
				results[index].Error = err.Error()
			}
		}(i, nodeID)
	}
	
	wg.Wait()
	
	return results
}

// CheckQuorum 检查Quorum
func (ftm *FaultToleranceManager) CheckQuorum(results []WriteResult, requiredSuccess int) bool {
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	
	return successCount >= requiredSuccess
}

// GetFailedNodes 获取失败的节点
func (ftm *FaultToleranceManager) GetFailedNodes(results []WriteResult) []string {
	var failedNodes []string
	for _, result := range results {
		if !result.Success {
			failedNodes = append(failedNodes, result.NodeID)
		}
	}
	return failedNodes
}

// CleanupTempFiles 清理临时文件
func (ftm *FaultToleranceManager) CleanupTempFiles(nodeID string, chunkIDs []string, deleteFunc func(nodeID, chunkID string) error) {
	for _, chunkID := range chunkIDs {
		err := deleteFunc(nodeID, chunkID)
		if err != nil {
			log.Printf("清理临时文件失败: node=%s, chunk=%s, error=%v", nodeID, chunkID, err)
		}
	}
}
