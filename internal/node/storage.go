package node

import (
	"crypto/sha256"
	"fmt"
	"io"
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

// HandleUpload 处理上传请求
func (se *StorageEngine) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取分片ID
	chunkID := r.URL.Query().Get("chunk_id")
	if chunkID == "" {
		http.Error(w, "Missing chunk_id", http.StatusBadRequest)
		return
	}

	// 读取数据
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read body failed", http.StatusInternalServerError)
		return
	}

	// 计算hash
	hash := sha256.Sum256(data)
	hashStr := fmt.Sprintf("%x", hash)

	// 保存到临时文件
	tempPath := filepath.Join(se.config.StoragePath, "temp", chunkID+".tmp")
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		http.Error(w, "Write file failed", http.StatusInternalServerError)
		return
	}

	// 移动到正式目录
	chunkPath := filepath.Join(se.config.StoragePath, "chunks", chunkID)
	if err := os.Rename(tempPath, chunkPath); err != nil {
		http.Error(w, "Move file failed", http.StatusInternalServerError)
		return
	}

	log.Printf("分片 %s 上传成功, hash: %s, size: %d", chunkID, hashStr, len(data))

	// 返回成功
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true,"hash":"%s","size":%d}`, hashStr, len(data))
}

// HandleDownload 处理下载请求
func (se *StorageEngine) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取分片ID
	chunkID := r.URL.Query().Get("chunk_id")
	if chunkID == "" {
		http.Error(w, "Missing chunk_id", http.StatusBadRequest)
		return
	}

	// 读取分片文件
	chunkPath := filepath.Join(se.config.StoragePath, "chunks", chunkID)
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Chunk not found", http.StatusNotFound)
		} else {
			http.Error(w, "Read file failed", http.StatusInternalServerError)
		}
		return
	}

	// 计算hash
	hash := sha256.Sum256(data)
	hashStr := fmt.Sprintf("%x", hash)

	// 返回数据
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Chunk-Hash", hashStr)
	w.Header().Set("X-Chunk-Size", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// HandleDelete 处理删除请求
func (se *StorageEngine) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取分片ID
	chunkID := r.URL.Query().Get("chunk_id")
	if chunkID == "" {
		http.Error(w, "Missing chunk_id", http.StatusBadRequest)
		return
	}

	// 删除分片文件
	chunkPath := filepath.Join(se.config.StoragePath, "chunks", chunkID)
	if err := os.Remove(chunkPath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Chunk not found", http.StatusNotFound)
		} else {
			http.Error(w, "Delete file failed", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("分片 %s 已删除", chunkID)

	// 返回成功
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true}`)
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
		"chunk_count": chunkCount,
		"total_size":  totalSize,
		"storage_path": se.config.StoragePath,
	}
}
