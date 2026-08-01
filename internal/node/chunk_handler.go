package node

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// HandleChunkUpload 处理分片上传
func (se *StorageEngine) HandleChunkUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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

// HandleChunkDownload 处理分片下载
func (se *StorageEngine) HandleChunkDownload(w http.ResponseWriter, r *http.Request) {
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

// HandleChunkDelete 处理分片删除
func (se *StorageEngine) HandleChunkDelete(w http.ResponseWriter, r *http.Request) {
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

// HandleReplicate 处理分片分发
func (se *StorageEngine) HandleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req struct {
		ChunkID     string   `json:"chunk_id"`
		TargetNodes []string `json:"target_nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 读取分片数据
	chunkPath := filepath.Join(se.config.StoragePath, "chunks", req.ChunkID)
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		http.Error(w, "Read chunk failed", http.StatusInternalServerError)
		return
	}

	// 分发到目标节点
	successCount := 0
	for _, nodeID := range req.TargetNodes {
		if err := se.sendToNode(nodeID, req.ChunkID, data); err != nil {
			log.Printf("分发到节点 %s 失败: %v", nodeID, err)
			continue
		}
		successCount++
	}

	log.Printf("分片 %s 分发完成: %d/%d 成功", req.ChunkID, successCount, len(req.TargetNodes))

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        successCount > 0,
		"success_count":  successCount,
		"total_count":    len(req.TargetNodes),
	})
}

// sendToNode 发送分片到目标节点
func (se *StorageEngine) sendToNode(nodeID string, chunkID string, data []byte) error {
	// TODO: 获取节点地址
	// 这里简化处理，直接返回成功
	// 实际实现需要：
	// 1. 从中心服务器获取节点地址
	// 2. 发送HTTP请求到目标节点
	// 3. 上传分片数据

	log.Printf("发送分片 %s 到节点 %s", chunkID, nodeID)
	return nil
}
