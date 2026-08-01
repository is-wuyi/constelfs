package server

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ChunkInfo 分片信息
type ChunkInfo struct {
	ChunkID   string    `json:"chunk_id"`
	FileID    string    `json:"file_id"`
	Index     int       `json:"index"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	Replicas  []string  `json:"replicas"`  // 存储该分片的节点列表
	Status    string    `json:"status"`    // pending, writing, success, failed
	CreatedAt time.Time `json:"created_at"`
}

// WriteRequest 写入请求
type WriteRequest struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Replicas int    `json:"replicas"` // 副本数，默认3
}

// WriteResponse 写入响应
type WriteResponse struct {
	Success  bool     `json:"success"`
	ChunkIDs []string `json:"chunk_ids,omitempty"`
	Nodes    []string `json:"nodes,omitempty"`   // 写入的节点列表
	Error    string   `json:"error,omitempty"`
}

// StorageManager 存储管理器
type StorageManager struct {
	scheduler *Scheduler
	chunks    map[string]*ChunkInfo
	mu        sync.RWMutex
}

// NewStorageManager 创建存储管理器
func NewStorageManager(scheduler *Scheduler) *StorageManager {
	return &StorageManager{
		scheduler: scheduler,
		chunks:    make(map[string]*ChunkInfo),
	}
}

// PrepareWrite 准备写入
func (sm *StorageManager) PrepareWrite(server *Server, req *WriteRequest) (*WriteResponse, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 智能选择节点
	nodes := sm.scheduler.SelectNodes(server.nodes, req.Replicas, nil)
	if len(nodes) < req.Replicas {
		return &WriteResponse{
			Success: false,
			Error:   fmt.Sprintf("可用节点不足: 需要%d个，只有%d个", req.Replicas, len(nodes)),
		}, nil
	}

	// 生成分片ID
	chunkID := fmt.Sprintf("%s_%d", req.FileID, time.Now().UnixNano())
	
	// 记录分片信息
	chunk := &ChunkInfo{
		ChunkID:   chunkID,
		FileID:    req.FileID,
		Size:      req.FileSize,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	
	// 记录副本位置
	for _, node := range nodes {
		chunk.Replicas = append(chunk.Replicas, node.NodeID)
	}
	
	sm.chunks[chunkID] = chunk

	// 返回节点列表
	var nodeIDs []string
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
	}

	return &WriteResponse{
		Success:  true,
		ChunkIDs: []string{chunkID},
		Nodes:    nodeIDs,
	}, nil
}

// ConfirmWrite 确认写入完成
func (sm *StorageManager) ConfirmWrite(chunkID string, hash string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	chunk, exists := sm.chunks[chunkID]
	if !exists {
		return fmt.Errorf("分片不存在: %s", chunkID)
	}

	// 记录hash并标记为成功
	chunk.Status = "success"
	chunk.Hash = hash
	log.Printf("分片 %s 写入成功, hash: %s", chunkID, hash)

	return nil
}

// GetChunkStatus 获取分片状态
func (sm *StorageManager) GetChunkStatus(chunkID string) (*ChunkInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	chunk, exists := sm.chunks[chunkID]
	if !exists {
		return nil, fmt.Errorf("分片不存在: %s", chunkID)
	}

	return chunk, nil
}

// CleanupFailedChunks 清理失败的分片
func (sm *StorageManager) CleanupFailedChunks() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for chunkID, chunk := range sm.chunks {
		if chunk.Status == "failed" || 
		   (chunk.Status == "pending" && time.Since(chunk.CreatedAt) > 24*time.Hour) {
			delete(sm.chunks, chunkID)
			log.Printf("清理分片: %s", chunkID)
		}
	}
}

// GetChunksByFileID 获取文件的所有分片
func (sm *StorageManager) GetChunksByFileID(fileID string) []*ChunkInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var chunks []*ChunkInfo
	for _, chunk := range sm.chunks {
		if chunk.FileID == fileID {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// DeleteChunk 删除分片
func (sm *StorageManager) DeleteChunk(chunkID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	chunk, exists := sm.chunks[chunkID]
	if !exists {
		return fmt.Errorf("分片不存在: %s", chunkID)
	}

	// 删除分片记录
	delete(sm.chunks, chunkID)

	log.Printf("删除分片: %s, 节点=%v", chunkID, chunk.Replicas)

	return nil
}
