package server

import (
	"crypto/sha256"
	"fmt"
	"log"
	"sync"
	"time"
)

// UploadManager 上传管理器
type UploadManager struct {
	scheduler    *Scheduler
	storage      *StorageManager
	encMgr       *EncryptionManager
	uploads      map[string]*UploadSession
	mu           sync.RWMutex
}

// UploadSession 上传会话
type UploadSession struct {
	SessionID    string            `json:"session_id"`
	FileID       string            `json:"file_id"`
	FileName     string            `json:"file_name"`
	FilePath     string            `json:"file_path"`
	FileSize     int64             `json:"file_size"`
	Replicas     int               `json:"replicas"`
	MaxVersions  int               `json:"max_versions"`
	Chunks       []ChunkUploadInfo `json:"chunks"`
	Status       string            `json:"status"` // pending, uploading, completed, failed
	CreatedAt    time.Time         `json:"created_at"`
	CompletedAt  time.Time         `json:"completed_at"`
}

// ChunkUploadInfo 分片上传信息
type ChunkUploadInfo struct {
	Index    int      `json:"index"`
	Size     int64    `json:"size"`
	Hash     string   `json:"hash"`
	Nodes    []string `json:"nodes"`
	Status   string   `json:"status"` // pending, uploading, completed, failed
	RetryCount int    `json:"retry_count"`
}

// NewUploadManager 创建上传管理器
func NewUploadManager(scheduler *Scheduler, storage *StorageManager, encMgr *EncryptionManager) *UploadManager {
	return &UploadManager{
		scheduler: scheduler,
		storage:   storage,
		encMgr:    encMgr,
		uploads:   make(map[string]*UploadSession),
	}
}

// CreateUploadSession 创建上传会话
func (um *UploadManager) CreateUploadSession(fileID, fileName, filePath string, fileSize int64, replicas, maxVersions int) (*UploadSession, error) {
	um.mu.Lock()
	defer um.mu.Unlock()
	
	// 生成会话ID
	sessionID := fmt.Sprintf("upload_%d", time.Now().UnixNano())
	
	// 计算分片信息
	chunkSize := calculateChunkSize(fileSize)
	chunkCount := (fileSize + chunkSize - 1) / chunkSize
	
	chunks := make([]ChunkUploadInfo, chunkCount)
	for i := int64(0); i < chunkCount; i++ {
		size := chunkSize
		if (i+1)*chunkSize > fileSize {
			size = fileSize - i*chunkSize
		}
		chunks[i] = ChunkUploadInfo{
			Index:  int(i),
			Size:   size,
			Status: "pending",
		}
	}
	
	session := &UploadSession{
		SessionID:   sessionID,
		FileID:      fileID,
		FileName:    fileName,
		FilePath:    filePath,
		FileSize:    fileSize,
		Replicas:    replicas,
		MaxVersions: maxVersions,
		Chunks:      chunks,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	
	um.uploads[sessionID] = session
	
	log.Printf("创建上传会话: %s, 文件: %s, 分片数: %d", sessionID, fileName, chunkCount)
	
	return session, nil
}

// GetUploadSession 获取上传会话
func (um *UploadManager) GetUploadSession(sessionID string) (*UploadSession, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()
	
	session, exists := um.uploads[sessionID]
	if !exists {
		return nil, fmt.Errorf("上传会话不存在: %s", sessionID)
	}
	
	return session, nil
}

// SelectNodesForChunk 为分片选择节点
func (um *UploadManager) SelectNodesForChunk(sessionID string, chunkIndex int) ([]string, error) {
	um.mu.Lock()
	defer um.mu.Unlock()
	
	session, exists := um.uploads[sessionID]
	if !exists {
		return nil, fmt.Errorf("上传会话不存在: %s", sessionID)
	}
	
	if chunkIndex >= len(session.Chunks) {
		return nil, fmt.Errorf("分片索引越界: %d", chunkIndex)
	}
	
	// 选择节点
	nodes := um.scheduler.SelectNodes(nil, session.Replicas, nil)
	if len(nodes) < session.Replicas {
		return nil, fmt.Errorf("可用节点不足: 需要%d, 可用%d", session.Replicas, len(nodes))
	}
	
	// 提取节点ID
	nodeIDs := make([]string, len(nodes))
	for i, node := range nodes {
		nodeIDs[i] = node.NodeID
	}
	
	// 更新分片信息
	session.Chunks[chunkIndex].Nodes = nodeIDs
	session.Chunks[chunkIndex].Status = "uploading"
	session.Status = "uploading"
	
	return nodeIDs, nil
}

// ConfirmChunkUpload 确认分片上传完成
func (um *UploadManager) ConfirmChunkUpload(sessionID string, chunkIndex int, hash string) error {
	um.mu.Lock()
	defer um.mu.Unlock()
	
	session, exists := um.uploads[sessionID]
	if !exists {
		return fmt.Errorf("上传会话不存在: %s", sessionID)
	}
	
	if chunkIndex >= len(session.Chunks) {
		return fmt.Errorf("分片索引越界: %d", chunkIndex)
	}
	
	// 验证hash
	expectedHash := session.Chunks[chunkIndex].Hash
	if expectedHash != "" && expectedHash != hash {
		return fmt.Errorf("分片hash不匹配: 期望 %s, 实际 %s", expectedHash, hash)
	}
	
	// 更新分片状态
	session.Chunks[chunkIndex].Status = "completed"
	session.Chunks[chunkIndex].Hash = hash
	
	// 检查是否所有分片都完成
	allCompleted := true
	for _, chunk := range session.Chunks {
		if chunk.Status != "completed" {
			allCompleted = false
			break
		}
	}
	
	if allCompleted {
		session.Status = "completed"
		session.CompletedAt = time.Now()
		log.Printf("上传会话完成: %s", sessionID)
	}
	
	return nil
}

// calculateChunkSize 计算分片大小
func calculateChunkSize(fileSize int64) int64 {
	const (
		ChunkSize10MB  = 10 * 1024 * 1024
		ChunkSize100MB = 100 * 1024 * 1024
		ChunkSize1GB   = 1024 * 1024 * 1024
		ChunkSize10GB  = 10 * 1024 * 1024 * 1024
	)
	
	switch {
	case fileSize < ChunkSize10MB:
		return fileSize
	case fileSize < ChunkSize100MB:
		return 4 * 1024 * 1024
	case fileSize < ChunkSize1GB:
		return 16 * 1024 * 1024
	case fileSize < ChunkSize10GB:
		return 64 * 1024 * 1024
	default:
		return 128 * 1024 * 1024
	}
}
