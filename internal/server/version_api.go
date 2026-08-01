package server

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

// HandleCreateVersion 处理创建版本请求 — 已移至 file_manager.go
