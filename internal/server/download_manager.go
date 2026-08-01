package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// DownloadManager 下载管理器
type DownloadManager struct {
	nodes    map[string]*Node
	mu       sync.RWMutex
	client   *http.Client
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager() *DownloadManager {
	return &DownloadManager{
		nodes:  make(map[string]*Node),
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// UpdateNodes 更新节点列表
func (dm *DownloadManager) UpdateNodes(nodes map[string]*Node) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.nodes = nodes
}

// DownloadChunk 从指定节点下载分片
func (dm *DownloadManager) DownloadChunk(chunkID string, nodeID string) ([]byte, error) {
	dm.mu.RLock()
	node, exists := dm.nodes[nodeID]
	dm.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}
	
	// 构建下载URL — 使用路径方式匹配节点Agent的路由
	url := fmt.Sprintf("http://%s:%d/api/v1/chunks/%s", node.IPAddress, node.Port, chunkID)
	
	// 发送下载请求
	resp, err := dm.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}
	
	// 读取数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取数据失败: %w", err)
	}
	
	log.Printf("下载分片成功: %s, 节点: %s, 大小: %d", chunkID, nodeID, len(data))
	
	return data, nil
}

// DownloadChunkWithFallback 从多个节点尝试下载（带容错）
func (dm *DownloadManager) DownloadChunkWithFallback(chunkID string, nodeIDs []string) ([]byte, string, error) {
	var lastErr error
	
	for _, nodeID := range nodeIDs {
		data, err := dm.DownloadChunk(chunkID, nodeID)
		if err == nil {
			return data, nodeID, nil
		}
		
		lastErr = err
		log.Printf("从节点 %s 下载分片 %s 失败: %v", nodeID, chunkID, err)
	}
	
	return nil, "", fmt.Errorf("所有节点下载失败: %w", lastErr)
}

// getNodeAddress 获取节点地址
func (dm *DownloadManager) getNodeAddress(nodeID string) (string, int, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	node, exists := dm.nodes[nodeID]
	if !exists {
		return "", 0, fmt.Errorf("节点不存在: %s", nodeID)
	}
	
	return node.IPAddress, node.Port, nil
}
