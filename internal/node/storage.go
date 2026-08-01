package node

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// StorageEngine 存储引擎
type StorageEngine struct {
	config *Config
	mu     sync.RWMutex
}

// NewStorageEngine 创建存储引擎
func NewStorageEngine(config *Config) *StorageEngine {
	return &StorageEngine{
		config: config,
	}
}

// Init 初始化存储目录
func (se *StorageEngine) Init() error {
	// 创建必要的目录
	dirs := []string{
		filepath.Join(se.config.StoragePath, "chunks"),
		filepath.Join(se.config.StoragePath, "temp"),
		filepath.Join(se.config.StoragePath, "meta"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	log.Printf("存储目录初始化完成: %s", se.config.StoragePath)
	return nil
}

// GetStorageInfo 获取存储信息
func (se *StorageEngine) GetStorageInfo() map[string]interface{} {
	se.mu.RLock()
	defer se.mu.RUnlock()

	// 统计分片数量
	chunksDir := filepath.Join(se.config.StoragePath, "chunks")
	chunkCount := 0
	totalSize := int64(0)

	entries, err := os.ReadDir(chunksDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				chunkCount++
				info, err := entry.Info()
				if err == nil {
					totalSize += info.Size()
				}
			}
		}
	}

	return map[string]interface{}{
		"chunk_count":  chunkCount,
		"total_size":   totalSize,
		"storage_path": se.config.StoragePath,
	}
}

// HandleUpload 处理上传请求（兼容旧接口）
func (se *StorageEngine) HandleUpload(w http.ResponseWriter, r *http.Request) {
	se.HandleChunkByPath(w, r)
}

// HandleDownload 处理下载请求（兼容旧接口）
func (se *StorageEngine) HandleDownload(w http.ResponseWriter, r *http.Request) {
	se.HandleChunkByPath(w, r)
}

// HandleDelete 处理删除请求（兼容旧接口）
func (se *StorageEngine) HandleDelete(w http.ResponseWriter, r *http.Request) {
	se.HandleChunkByPath(w, r)
}

// HandleChunk 处理分片请求（兼容旧接口）
func (se *StorageEngine) HandleChunk(w http.ResponseWriter, r *http.Request) {
	se.HandleChunkByPath(w, r)
}
