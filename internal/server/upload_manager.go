package server

import (
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
	Index      int      `json:"index"`
	Size       int64    `json:"size"`
	Hash       string   `json:"hash"`
	Nodes      []string `json:"nodes"`
	Status     string   `json:"status"` // pending, uploading, completed, failed
	RetryCount int      `json:"retry_count"`
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
