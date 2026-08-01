package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HandleChunkByPath 处理分片请求（根据URL路径中的chunkID）
// 支持: /api/v1/chunks/{chunkID} (GET/PUT/DELETE)
// 以及: /api/v1/chunks/upload?chunk_id=xxx 等旧路由
func (se *StorageEngine) HandleChunkByPath(w http.ResponseWriter, r *http.Request) {
	// 提取路径后缀
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chunks/")
	
	// 根据路径判断操作
	switch {
	case path == "upload":
		// 旧路由: /api/v1/chunks/upload?chunk_id=xxx
		se.handleUpload(w, r, r.URL.Query().Get("chunk_id"))
	case path == "download":
		// 旧路由: /api/v1/chunks/download?chunk_id=xxx
		se.handleDownload(w, r, r.URL.Query().Get("chunk_id"))
	case path == "delete":
		// 旧路由: /api/v1/chunks/delete?chunk_id=xxx
		se.handleDelete(w, r, r.URL.Query().Get("chunk_id"))
	case path != "" && path != "/":
		// 新路由: /api/v1/chunks/{chunkID}
		chunkID := path
		switch r.Method {
		case http.MethodGet:
			se.handleDownload(w, r, chunkID)
		case http.MethodPut:
			se.handleUpload(w, r, chunkID)
		case http.MethodDelete:
			se.handleDelete(w, r, chunkID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.Error(w, "Missing chunk_id", http.StatusBadRequest)
	}
}

// handleUpload 处理分片上传
func (se *StorageEngine) handleUpload(w http.ResponseWriter, r *http.Request, chunkID string) {
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

// handleDownload 处理分片下载
func (se *StorageEngine) handleDownload(w http.ResponseWriter, r *http.Request, chunkID string) {
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

// handleDelete 处理分片删除
func (se *StorageEngine) handleDelete(w http.ResponseWriter, r *http.Request, chunkID string) {
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

// HandleReplicate 处理分片分发（接收分片后转发到其他节点）
func (se *StorageEngine) HandleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req struct {
		ChunkID     string   `json:"chunk_id"`
		TargetNodes []string `json:"target_nodes"` // 格式: "ip:port"
		ServerAddr  string   `json:"server_addr"`  // 中心服务器地址，用于解析节点ID
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 读取本地分片数据
	chunkPath := filepath.Join(se.config.StoragePath, "chunks", req.ChunkID)
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		http.Error(w, "Read chunk failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 并行分发到目标节点
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, nodeAddr := range req.TargetNodes {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			if err := se.sendChunkToNode(addr, req.ChunkID, data); err != nil {
				log.Printf("分发到节点 %s 失败: %v", addr, err)
				return
			}
			mu.Lock()
			successCount++
			mu.Unlock()
		}(nodeAddr)
	}
	wg.Wait()

	log.Printf("分片 %s 分发完成: %d/%d 成功", req.ChunkID, successCount, len(req.TargetNodes))

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       successCount > 0,
		"success_count": successCount,
		"total_count":   len(req.TargetNodes),
	})
}

// sendChunkToNode 发送分片到目标节点
func (se *StorageEngine) sendChunkToNode(nodeAddr string, chunkID string, data []byte) error {
	client := &http.Client{Timeout: 60 * time.Second}
	
	// 重试3次
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			log.Printf("重试发送分片 %s 到 %s, 第%d次", chunkID, nodeAddr, retry+1)
			time.Sleep(time.Duration(retry) * time.Second)
		}
		
		url := fmt.Sprintf("http://%s/api/v1/chunks/%s", nodeAddr, chunkID)
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("发送到 %s 失败: %v", nodeAddr, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("发送到 %s 返回 %d: %s", nodeAddr, resp.StatusCode, string(body))
			continue
		}

		log.Printf("分片 %s 发送到 %s 成功", chunkID, nodeAddr)
		return nil
	}

	return fmt.Errorf("发送分片到 %s 失败（已重试3次）", nodeAddr)
}
