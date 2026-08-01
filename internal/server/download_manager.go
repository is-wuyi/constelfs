package server

import (
	"encoding/json"
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

// DownloadSession 下载会话
type DownloadSession struct {
	SessionID string            `json:"session_id"`
	FileID    string            `json:"file_id"`
	Version   int               `json:"version"`
	Chunks    []ChunkDownloadInfo `json:"chunks"`
	Status    string            `json:"status"` // pending, downloading, completed, failed
	CreatedAt time.Time         `json:"created_at"`
}

// ChunkDownloadInfo 分片下载信息
type ChunkDownloadInfo struct {
	Index    int      `json:"index"`
	Size     int64    `json:"size"`
	Hash     string   `json:"hash"`
	Nodes    []string `json:"nodes"`
	Status   string   `json:"status"` // pending, downloading, completed, failed
	Data     []byte   `json:"-"`
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager() *DownloadManager {
	return &DownloadManager{
		nodes:  make(map[string]*Node),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// UpdateNodes 更新节点列表
func (dm *DownloadManager) UpdateNodes(nodes map[string]*Node) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.nodes = nodes
}

// CreateDownloadSession 创建下载会话
func (dm *DownloadManager) CreateDownloadSession(fileID string, version int, chunks []ChunkInfo) (*DownloadSession, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	sessionID := fmt.Sprintf("download_%d", time.Now().UnixNano())
	
	downloadChunks := make([]ChunkDownloadInfo, len(chunks))
	for i, chunk := range chunks {
		downloadChunks[i] = ChunkDownloadInfo{
			Index:  chunk.Index,
			Size:   chunk.Size,
			Hash:   chunk.Hash,
			Status: "pending",
		}
	}
	
	session := &DownloadSession{
		SessionID: sessionID,
		FileID:    fileID,
		Version:   version,
		Chunks:    downloadChunks,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	
	log.Printf("创建下载会话: %s, 文件: %s, 版本: %d", sessionID, fileID, version)
	
	return session, nil
}

// DownloadChunk 下载分片
func (dm *DownloadManager) DownloadChunk(chunkID string, nodeID string) ([]byte, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	node, exists := dm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}
	
	// 构建下载URL
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

// DownloadChunkWithFallback 带容错的下载
func (dm *DownloadManager) DownloadChunkWithFallback(chunkID string, nodeIDs []string) ([]byte, error) {
	var lastErr error
	
	for _, nodeID := range nodeIDs {
		data, err := dm.DownloadChunk(chunkID, nodeID)
		if err == nil {
			return data, nil
		}
		
		lastErr = err
		log.Printf("从节点 %s 下载失败: %v", nodeID, err)
	}
	
	return nil, fmt.Errorf("所有节点下载失败: %w", lastErr)
}

// AssembleFile 组装文件
func (dm *DownloadManager) AssembleFile(chunks []ChunkDownloadInfo) ([]byte, error) {
	// 计算总大小
	totalSize := int64(0)
	for _, chunk := range chunks {
		totalSize += chunk.Size
	}
	
	// 分配缓冲区
	fileData := make([]byte, 0, totalSize)
	
	// 按顺序组装分片
	for _, chunk := range chunks {
		if chunk.Data == nil {
			return nil, fmt.Errorf("分片 %d 数据为空", chunk.Index)
		}
		fileData = append(fileData, chunk.Data...)
	}
	
	log.Printf("文件组装完成, 大小: %d", len(fileData))
	
	return fileData, nil
}
