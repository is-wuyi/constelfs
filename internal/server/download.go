package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// DownloadRequest 下载请求
type DownloadRequest struct {
	FileID  string `json:"file_id"`
	Version int    `json:"version"` // 0表示最新版本
}

// DownloadResponse 下载响应
type DownloadResponse struct {
	Success     bool        `json:"success"`
	File        *FileInfo   `json:"file"`
	Version     *FileVersion `json:"version"`
	DownloadURL string      `json:"download_url"`
	Error       string      `json:"error,omitempty"`
}

// HandleDownload 处理文件下载
func (fm *FileManager) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL提取file_id和version
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	parts := strings.SplitN(path, "/", 2)
	fileID := parts[0]

	var version int
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &version)
	}

	// 获取文件信息
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 获取版本
	versions := fm.versions[fileID]
	if len(versions) == 0 {
		http.Error(w, `{"error":"No versions available"}`, http.StatusNotFound)
		return
	}

	var targetVersion *FileVersion
	if version == 0 {
		// 最新版本
		targetVersion = versions[len(versions)-1]
	} else {
		// 指定版本
		for _, v := range versions {
			if v.Version == version {
				targetVersion = v
				break
			}
		}
	}

	if targetVersion == nil {
		http.Error(w, `{"error":"Version not found"}`, http.StatusNotFound)
		return
	}

	// 生成下载URL
	downloadURL := fm.generateDownloadURL(fileID, targetVersion)

	log.Printf("文件下载: %s, 版本=%d, 节点=%v", fileID, targetVersion.Version, targetVersion.NodeIDs)

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DownloadResponse{
		Success:     true,
		File:        file,
		Version:     targetVersion,
		DownloadURL: downloadURL,
	})
}

// generateDownloadURL 生成下载URL
func (fm *FileManager) generateDownloadURL(fileID string, version *FileVersion) string {
	// TODO: 生成实际的下载URL
	// 这里应该根据节点地址和分片信息生成下载URL
	return fmt.Sprintf("/api/v1/files/%s/versions/%d/download", fileID, version.Version)
}

// HandleDirectDownload 处理直接下载
func (fm *FileManager) HandleDirectDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL提取file_id和version
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 || parts[1] != "versions" || !strings.HasSuffix(parts[2], "/download") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fileID := parts[0]
	versionStr := strings.TrimSuffix(parts[2], "/download")
	var version int
	fmt.Sscanf(versionStr, "%d", &version)

	// 获取文件信息
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 获取版本
	versions := fm.versions[fileID]
	var targetVersion *FileVersion
	for _, v := range versions {
		if v.Version == version {
			targetVersion = v
			break
		}
	}

	if targetVersion == nil {
		http.Error(w, `{"error":"Version not found"}`, http.StatusNotFound)
		return
	}

	// TODO: 实现实际的文件下载
	// 这里应该从存储节点读取分片数据并返回给客户端

	log.Printf("直接下载: %s, 版本=%d, 大小=%d", fileID, version, targetVersion.Size)

	// 临时实现：返回文件信息
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":    file,
		"version": targetVersion,
		"message": "Download not implemented yet",
	})
}
