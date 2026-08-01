package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// UploadRequest 上传请求
type UploadRequest struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
	Replicas    int    `json:"replicas"`
	MaxVersions int    `json:"max_versions"`
}

// UploadResponse 上传响应
type UploadResponse struct {
	Success   bool     `json:"success"`
	FileID    string   `json:"file_id"`
	Version   int      `json:"version"`
	ChunkIDs  []string `json:"chunk_ids"`
	NodeIDs   []string `json:"node_ids"`
	Error     string   `json:"error,omitempty"`
}

// HandleUpload 处理文件上传
func (fm *FileManager) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 默认值
	if req.Replicas == 0 {
		req.Replicas = 3
	}
	if req.MaxVersions == 0 {
		req.MaxVersions = 3
	}

	// 读取文件数据
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read file failed", http.StatusInternalServerError)
		return
	}

	// 计算文件hash
	hash := sha256.Sum256(data)
	hashStr := fmt.Sprintf("%x", hash)

	// 获取或创建文件信息
	file, exists := fm.files[req.FileID]
	if !exists {
		// 创建新文件
		file = &FileInfo{
			FileID:        req.FileID,
			FileName:      req.FileName,
			FilePath:      req.FilePath,
			MaxVersions:   req.MaxVersions,
			EncryptionKey: "", // TODO: 处理加密密钥
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		fm.files[req.FileID] = file
	}

	// 上传分片到存储节点
	chunkIDs, nodeIDs, err := fm.uploadChunks(req.FileID, data, req.Replicas)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 创建新版本
	newVersion, err := fm.versionManager.CreateNewVersion(file, chunkIDs, nodeIDs, int64(len(data)), hashStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加版本到列表
	fm.versions[req.FileID] = append(fm.versions[req.FileID], newVersion)

	// 清理旧版本
	fm.versions[req.FileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[req.FileID])

	log.Printf("文件上传成功: %s, 版本=%d, 大小=%d", req.FileID, newVersion.Version, len(data))

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		FileID:   req.FileID,
		Version:  newVersion.Version,
		ChunkIDs: chunkIDs,
		NodeIDs:  nodeIDs,
	})
}

// uploadChunks 上传分片到存储节点
func (fm *FileManager) uploadChunks(fileID string, data []byte, replicas int) ([]string, []string, error) {
	// TODO: 实现分片上传逻辑
	// 1. 将数据分片
	// 2. 选择存储节点
	// 3. 上传分片到接收节点
	// 4. 接收节点分发到副本节点

	// 临时实现：返回空列表
	return []string{}, []string{}, nil
}
