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

// ChunkInfo 分片信息
type ChunkInfo struct {
	ChunkID   string    `json:"chunk_id"`
	FileID    string    `json:"file_id"`
	Index     int       `json:"index"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	Replicas  []string  `json:"replicas"`  // 存储该分片的节点列表 (nodeID)
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
	Success    bool     `json:"success"`
	ChunkID    string   `json:"chunk_id,omitempty"`
	ChunkIDs   []string `json:"chunk_ids,omitempty"`
	Nodes      []string `json:"nodes,omitempty"`   // 写入的节点列表 (nodeID)
	NodeAddrs  []string `json:"node_addrs,omitempty"` // 节点地址列表
	Error      string   `json:"error,omitempty"`
}

// StorageManager 存储管理器
type StorageManager struct {
	scheduler        *Scheduler
	replicationMgr   *ReplicationManager
	downloadMgr      *DownloadManager
	chunks           map[string]*ChunkInfo
	mu               sync.RWMutex
}

// NewStorageManager 创建存储管理器
func NewStorageManager(scheduler *Scheduler) *StorageManager {
	return &StorageManager{
		scheduler:      scheduler,
		replicationMgr: NewReplicationManager(),
		downloadMgr:    NewDownloadManager(),
		chunks:         make(map[string]*ChunkInfo),
	}
}

// UpdateNodes 更新节点列表
func (sm *StorageManager) UpdateNodes(nodes map[string]*Node) {
	sm.replicationMgr.UpdateNodes(nodes)
	sm.downloadMgr.UpdateNodes(nodes)
}

// PrepareWrite 准备写入 — 选择节点并返回节点列表
func (sm *StorageManager) PrepareWrite(server *Server, req *WriteRequest) (*WriteResponse, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 默认3副本
	if req.Replicas == 0 {
		req.Replicas = 3
	}

	// 智能选择节点
	nodes := sm.scheduler.SelectNodes(server.nodes, req.Replicas, nil)
	if len(nodes) == 0 {
		return &WriteResponse{
			Success: false,
			Error:   "没有可用节点",
		}, nil
	}

	// 生成分片ID
	chunkID := fmt.Sprintf("%s_chunk_%d", req.FileID, time.Now().UnixNano())
	
	// 记录分片信息
	chunk := &ChunkInfo{
		ChunkID:   chunkID,
		FileID:    req.FileID,
		Size:      req.FileSize,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	
	// 记录副本位置
	var nodeIDs []string
	var nodeAddrs []string
	for _, node := range nodes {
		chunk.Replicas = append(chunk.Replicas, node.NodeID)
		nodeIDs = append(nodeIDs, node.NodeID)
		nodeAddrs = append(nodeAddrs, fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
	}
	
	sm.chunks[chunkID] = chunk

	return &WriteResponse{
		Success:   true,
		ChunkID:   chunkID,
		ChunkIDs:  []string{chunkID},
		Nodes:     nodeIDs,
		NodeAddrs: nodeAddrs,
	}, nil
}

// ConfirmWrite 确认写入完成并触发副本分发
func (sm *StorageManager) ConfirmWrite(chunkID string, hash string) error {
	sm.mu.Lock()
	chunk, exists := sm.chunks[chunkID]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("分片不存在: %s", chunkID)
	}

	// 记录hash并标记为成功
	chunk.Status = "success"
	chunk.Hash = hash
	
	// 获取需要分发的目标节点（除第一个节点外的所有副本节点）
	var targetNodeIDs []string
	sourceNodeID := ""
	if len(chunk.Replicas) > 0 {
		sourceNodeID = chunk.Replicas[0]
		targetNodeIDs = chunk.Replicas[1:]
	}
	sm.mu.Unlock()

	log.Printf("分片 %s 写入成功, hash: %s, 副本节点: %v", chunkID, hash, chunk.Replicas)

	// 异步触发副本分发
	if len(targetNodeIDs) > 0 && sourceNodeID != "" {
		go func() {
			if err := sm.replicationMgr.ReplicateChunk(chunkID, sourceNodeID, targetNodeIDs); err != nil {
				log.Printf("分片 %s 副本分发失败: %v", chunkID, err)
				// 分发失败不阻塞主流程，但标记为降级
				sm.mu.Lock()
				if c, ok := sm.chunks[chunkID]; ok {
					c.Status = "degraded"
				}
				sm.mu.Unlock()
			} else {
				log.Printf("分片 %s 副本分发完成", chunkID)
			}
		}()
	}

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

// DeleteChunk 删除分片（从所有副本节点）
func (sm *StorageManager) DeleteChunk(chunkID string) error {
	sm.mu.Lock()
	chunk, exists := sm.chunks[chunkID]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("分片不存在: %s", chunkID)
	}
	
	// 获取副本节点列表
	replicas := make([]string, len(chunk.Replicas))
	copy(replicas, chunk.Replicas)
	
	delete(sm.chunks, chunkID)
	sm.mu.Unlock()

	// 从所有副本节点删除分片
	for _, nodeID := range replicas {
		sm.downloadMgr.mu.RLock()
		node, ok := sm.downloadMgr.nodes[nodeID]
		sm.downloadMgr.mu.RUnlock()
		if !ok {
			continue
		}
		
		url := fmt.Sprintf("http://%s:%d/api/v1/chunks/%s", node.IPAddress, node.Port, chunkID)
		req, _ := http.NewRequest(http.MethodDelete, url, nil)
		resp, err := sm.downloadMgr.client.Do(req)
		if err != nil {
			log.Printf("从节点 %s 删除分片 %s 失败: %v", nodeID, chunkID, err)
			continue
		}
		resp.Body.Close()
		log.Printf("从节点 %s 删除分片 %s 成功", nodeID, chunkID)
	}

	return nil
}

// UploadToNode 上传分片数据到指定节点
func (sm *StorageManager) UploadToNode(nodeAddr string, chunkID string, data []byte) error {
	client := &http.Client{Timeout: 120 * time.Second}
	
	url := fmt.Sprintf("http://%s/api/v1/chunks/%s", nodeAddr, chunkID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传返回 %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应获取hash
	var result struct {
		Success bool   `json:"success"`
		Hash    string `json:"hash"`
		Size    int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// 忽略解析错误，上传成功即可
		return nil
	}

	return nil
}

// DownloadFromNode 从指定节点下载分片数据
func (sm *StorageManager) DownloadFromNode(chunkID string) ([]byte, error) {
	chunk, err := sm.GetChunkStatus(chunkID)
	if err != nil {
		return nil, err
	}
	
	if len(chunk.Replicas) == 0 {
		return nil, fmt.Errorf("分片 %s 没有副本", chunkID)
	}
	
	data, _, err := sm.downloadMgr.DownloadChunkWithFallback(chunkID, chunk.Replicas)
	return data, err
}
