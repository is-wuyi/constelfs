package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// CreateVersionRequest 创建版本请求
type CreateVersionRequest struct {
	FileID   string   `json:"file_id"`
	FileName string   `json:"file_name"`
	FilePath string   `json:"file_path"`
	FileSize int64    `json:"file_size"`
	Hash     string   `json:"hash"`
	ChunkIDs []string `json:"chunk_ids"`
	NodeIDs  []string `json:"node_ids"`
}

// CreateVersionResponse 创建版本响应
type CreateVersionResponse struct {
	Success bool   `json:"success"`
	FileID  string `json:"file_id"`
	Version int    `json:"version"`
	Error   string `json:"error,omitempty"`
}

// HandleCreateVersion 处理创建版本请求
func (fm *FileManager) HandleCreateVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req CreateVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 获取或创建文件信息
	file, exists := fm.files[req.FileID]
	if !exists {
		// 创建新文件
		file = &FileInfo{
			FileID:        req.FileID,
			FileName:      req.FileName,
			FilePath:      req.FilePath,
			MaxVersions:   3, // 默认3个版本
			EncryptionKey: "",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		fm.files[req.FileID] = file
	}

	// 创建新版本
	newVersion, err := fm.versionManager.CreateNewVersion(file, req.ChunkIDs, req.NodeIDs, req.FileSize, req.Hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加版本到列表
	fm.versions[req.FileID] = append(fm.versions[req.FileID], newVersion)

	// 清理旧版本
	fm.versions[req.FileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[req.FileID])

	log.Printf("版本创建成功: 文件=%s, 版本=%d", req.FileID, newVersion.Version)

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateVersionResponse{
		Success: true,
		FileID:  req.FileID,
		Version: newVersion.Version,
	})
}
