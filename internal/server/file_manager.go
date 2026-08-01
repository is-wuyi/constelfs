package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FileManager 文件管理器
type FileManager struct {
	versionManager *VersionManager
	storage        *StorageManager
	files          map[string]*FileInfo
	versions       map[string][]*FileVersion
}

// NewFileManager 创建文件管理器
func NewFileManager(versionManager *VersionManager, storage *StorageManager) *FileManager {
	return &FileManager{
		versionManager: versionManager,
		storage:        storage,
		files:          make(map[string]*FileInfo),
		versions:       make(map[string][]*FileVersion),
	}
}

// HandleFiles 处理文件列表请求
func (fm *FileManager) HandleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		fm.listFiles(w, r)
	case http.MethodPost:
		fm.uploadFile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleFile 处理单个文件请求
func (fm *FileManager) HandleFile(w http.ResponseWriter, r *http.Request) {
	// 从URL提取file_id
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	parts := strings.SplitN(path, "/", 2)
	fileID := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		fm.getFile(w, r, fileID)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		fm.deleteFile(w, r, fileID)
	case len(parts) == 2 && parts[1] == "download" && r.Method == http.MethodGet:
		fm.downloadFile(w, r, fileID, 0) // 0 表示最新版本
	case len(parts) == 2 && parts[1] == "rollback" && r.Method == http.MethodPost:
		fm.rollbackFile(w, r, fileID)
	default:
		// 检查是否是版本下载请求
		if len(parts) == 3 && parts[1] == "versions" && strings.HasSuffix(parts[2], "/download") {
			versionStr := strings.TrimSuffix(parts[2], "/download")
			var version int
			fmt.Sscanf(versionStr, "%d", &version)
			fm.downloadFile(w, r, fileID, version)
			return
		}
		// 检查是否是版本删除请求
		if len(parts) == 3 && parts[1] == "versions" && r.Method == http.MethodDelete {
			var version int
			fmt.Sscanf(parts[2], "%d", &version)
			fm.deleteVersion(w, r, fileID, version)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// listFiles 列出文件
func (fm *FileManager) listFiles(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("dir")
	if dirPath == "" {
		dirPath = "/"
	}

	// 获取指定目录下的文件
	var files []*FileInfo
	for _, file := range fm.files {
		if strings.HasPrefix(file.FilePath, dirPath) {
			files = append(files, file)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"total": len(files),
		"dir":   dirPath,
	})
}

// getFile 获取文件详情
func (fm *FileManager) getFile(w http.ResponseWriter, r *http.Request, fileID string) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 获取版本列表
	versions := fm.versions[fileID]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":     file,
		"versions": versions,
	})
}

// uploadFile 上传文件
func (fm *FileManager) uploadFile(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现文件上传
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// downloadFile 下载文件
func (fm *FileManager) downloadFile(w http.ResponseWriter, r *http.Request, fileID string, version int) {
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

	// TODO: 实现文件下载
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":    file,
		"version": targetVersion,
	})
}

// rollbackFile 回滚文件
func (fm *FileManager) rollbackFile(w http.ResponseWriter, r *http.Request, fileID string) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 查找目标版本
	versions := fm.versions[fileID]
	var targetVersion *FileVersion
	for _, v := range versions {
		if v.Version == req.Version {
			targetVersion = v
			break
		}
	}

	if targetVersion == nil {
		http.Error(w, `{"error":"Version not found"}`, http.StatusNotFound)
		return
	}

	// 回滚
	newVersion, err := fm.versionManager.RollbackToVersion(file, targetVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加新版本
	fm.versions[fileID] = append(fm.versions[fileID], newVersion)

	// 清理旧版本
	fm.versions[fileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[fileID])

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"new_version": newVersion,
	})
}

// deleteFile 删除文件
func (fm *FileManager) deleteFile(w http.ResponseWriter, r *http.Request, fileID string) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 删除所有版本
	versions := fm.versions[fileID]
	for _, version := range versions {
		fm.versionManager.DeleteVersion(version)
	}

	// 删除文件记录
	delete(fm.files, fileID)
	delete(fm.versions, fileID)

	log.Printf("删除文件: %s, 路径=%s", fileID, file.FilePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// deleteVersion 删除版本
func (fm *FileManager) deleteVersion(w http.ResponseWriter, r *http.Request, fileID string, version int) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 查找并删除版本
	versions := fm.versions[fileID]
	for i, v := range versions {
		if v.Version == version {
			// 删除版本
			fm.versionManager.DeleteVersion(v)
			fm.versions[fileID] = append(versions[:i], versions[i+1:]...)
			file.VersionCount--
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
