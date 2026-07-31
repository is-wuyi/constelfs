package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// FileInfo 文件信息
type FileInfo struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	DirPath           string    `json:"dir_path"`
	FileName          string    `json:"file_name"`
	FilePath          string    `json:"file_path"`
	FileSize          int64     `json:"file_size"`
	IsDirectory       bool      `json:"is_directory"`
	ReplicationFactor int       `json:"replication_factor"`
	ErasureCoded      bool      `json:"erasure_coded"`
	CreatedAt         string    `json:"created_at"`
	UpdatedAt         string    `json:"updated_at"`
}

// handleFiles 处理文件列表请求
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listFiles(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFile 处理单个文件请求
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	_ = path

	switch r.Method {
	case http.MethodGet:
		// 获取文件信息
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	case http.MethodDelete:
		// 删除文件
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listFiles 获取文件列表
func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	// 从数据库获取文件列表
	dirPath := r.URL.Query().Get("dir")
	if dirPath == "" {
		dirPath = "/"
	}

	// TODO: 从数据库查询文件
	files := []*FileInfo{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"total": len(files),
		"dir":   dirPath,
	})
}
