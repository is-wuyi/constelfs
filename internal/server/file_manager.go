package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FileManager 文件管理器
type FileManager struct {
	versionManager *VersionManager
	storage        *StorageManager
	scheduler      *Scheduler
	nodes          map[string]*Node
	files          map[string]*FileInfo
	versions       map[string][]*FileVersion
}

// NewFileManager 创建文件管理器
func NewFileManager(versionManager *VersionManager, storage *StorageManager, scheduler *Scheduler) *FileManager {
	return &FileManager{
		versionManager: versionManager,
		storage:        storage,
		scheduler:      scheduler,
		nodes:          make(map[string]*Node),
		files:          make(map[string]*FileInfo),
		versions:       make(map[string][]*FileVersion),
	}
}

// UpdateNodes 更新节点列表
func (fm *FileManager) UpdateNodes(nodes map[string]*Node) {
	fm.nodes = nodes
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
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	fileID := parts[0]
	
	switch {
	// GET /api/v1/files/:id - 获取文件详情
	case len(parts) == 1 && r.Method == http.MethodGet:
		fm.getFile(w, r, fileID)
		
	// DELETE /api/v1/files/:id - 删除文件
	case len(parts) == 1 && r.Method == http.MethodDelete:
		fm.deleteFile(w, r, fileID)
		
	// GET /api/v1/files/:id/download - 下载最新版本
	case len(parts) == 2 && parts[1] == "download" && r.Method == http.MethodGet:
		fm.downloadFile(w, r, fileID, 0)
		
	// POST /api/v1/files/:id/rollback - 版本回滚
	case len(parts) == 2 && parts[1] == "rollback" && r.Method == http.MethodPost:
		fm.rollbackFile(w, r, fileID)
		
	// GET /api/v1/files/:id/versions/:v/download - 下载指定版本
	case len(parts) == 4 && parts[1] == "versions" && parts[3] == "download" && r.Method == http.MethodGet:
		version, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, "Invalid version", http.StatusBadRequest)
			return
		}
		fm.downloadFile(w, r, fileID, version)
		
	// DELETE /api/v1/files/:id/versions/:v - 删除指定版本
	case len(parts) == 3 && parts[1] == "versions" && r.Method == http.MethodDelete:
		version, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, "Invalid version", http.StatusBadRequest)
			return
		}
		fm.deleteVersion(w, r, fileID, version)
		
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// HandleUpload 处理上传（兼容旧路由）
func (fm *FileManager) HandleUpload(w http.ResponseWriter, r *http.Request) {
	fm.uploadFile(w, r)
}

// HandleDownload 处理下载（兼容旧路由）
func (fm *FileManager) HandleDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	fm.downloadFile(w, r, path, 0)
}

// HandleCreateVersion 处理版本创建
func (fm *FileManager) HandleCreateVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID   string   `json:"file_id"`
		FileName string   `json:"file_name"`
		FilePath string   `json:"file_path"`
		FileSize int64    `json:"file_size"`
		Hash     string   `json:"hash"`
		ChunkIDs []string `json:"chunk_ids"`
		NodeIDs  []string `json:"node_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 查找或创建文件记录
	file, exists := fm.files[req.FileID]
	if !exists {
		file = &FileInfo{
			FileID:      req.FileID,
			FileName:    req.FileName,
			FilePath:    req.FilePath,
			MaxVersions: 3,
			CreatedAt:   time.Now(),
		}
		fm.files[req.FileID] = file
	}

	// 创建新版本
	newVersion, err := fm.versionManager.CreateNewVersion(file, req.ChunkIDs, req.NodeIDs, req.FileSize, req.Hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加版本
	fm.versions[req.FileID] = append(fm.versions[req.FileID], newVersion)

	// 清理旧版本
	fm.versions[req.FileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[req.FileID])

	log.Printf("版本创建成功: 文件=%s, 版本=%d", req.FileID, newVersion.Version)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"version": newVersion,
	})
}

// listFiles 列出文件
func (fm *FileManager) listFiles(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("dir")
	if dirPath == "" {
		dirPath = "/"
	}

	var files []*FileInfo
	for _, file := range fm.files {
		// 简单的目录匹配
		if dirPath == "/" {
			// 根目录下显示所有文件
			files = append(files, file)
		} else if strings.HasPrefix(file.FilePath, dirPath) {
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

	versions := fm.versions[fileID]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file":     file,
		"versions": versions,
	})
}

// uploadFile 上传文件（端到端流程）
func (fm *FileManager) uploadFile(w http.ResponseWriter, r *http.Request) {
	// 解析multipart或raw body
	contentType := r.Header.Get("Content-Type")
	
	var fileData []byte
	var fileName, filePath string
	var err error
	
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// multipart上传
		r.ParseMultipartForm(64 << 20) // 64MB max memory
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Parse form failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		
		fileName = header.Filename
		filePath = r.FormValue("path")
		if filePath == "" {
			filePath = "/" + fileName
		}
		
		fileData, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, "Read file failed", http.StatusInternalServerError)
			return
		}
	} else {
		// Raw body上传 — 需要从header获取文件名
		fileName = r.Header.Get("X-File-Name")
		filePath = r.Header.Get("X-File-Path")
		if filePath == "" {
			filePath = "/" + fileName
		}
		
		fileData, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Read body failed", http.StatusInternalServerError)
			return
		}
	}
	
	if fileName == "" {
		fileName = "unnamed"
	}

	replicas := 3
	if r := r.URL.Query().Get("replicas"); r != "" {
		if v, err := strconv.Atoi(r); err == nil && v > 0 {
			replicas = v
		}
	}

	// 生成文件ID
	fileID := fmt.Sprintf("%s_%d", fileName, time.Now().UnixNano())
	
	// 计算整体hash
	hash := sha256.Sum256(fileData)
	hashStr := fmt.Sprintf("%x", hash)

	// 分片
	chunkSize := calculateChunkSize(int64(len(fileData)))
	chunkCount := (int64(len(fileData)) + chunkSize - 1) / chunkSize
	
	var chunkIDs []string
	var allNodeIDs []string
	
	for i := int64(0); i < chunkCount; i++ {
		offset := i * chunkSize
		end := offset + chunkSize
		if end > int64(len(fileData)) {
			end = int64(len(fileData))
		}
		chunkData := fileData[offset:end]
		
		// 向中心服务器请求写入（选择节点）
		writeReq := &WriteRequest{
			FileID:   fileID,
			FileName: fmt.Sprintf("chunk_%d", i),
			FileSize: int64(len(chunkData)),
			Replicas: replicas,
		}
		
		writeResp, err := fm.storage.PrepareWrite(&Server{nodes: fm.nodes, scheduler: fm.scheduler, storage: fm.storage}, writeReq)
		if err != nil {
			http.Error(w, "Prepare write failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !writeResp.Success {
			http.Error(w, "Prepare write failed: "+writeResp.Error, http.StatusInternalServerError)
			return
		}
		
		chunkID := writeResp.ChunkID
		chunkIDs = append(chunkIDs, chunkID)
		
		// 上传分片到第一个节点
		if len(writeResp.NodeAddrs) > 0 {
			firstNodeAddr := writeResp.NodeAddrs[0]
			if err := fm.storage.UploadToNode(firstNodeAddr, chunkID, chunkData); err != nil {
				http.Error(w, fmt.Sprintf("Upload chunk %d to node failed: %s", i, err.Error()), http.StatusInternalServerError)
				return
			}
			
			// 确认写入（会触发异步副本分发）
			chunkHash := sha256.Sum256(chunkData)
			if err := fm.storage.ConfirmWrite(chunkID, fmt.Sprintf("%x", chunkHash)); err != nil {
				log.Printf("确认写入失败: %v", err)
			}
		}
		
		if len(writeResp.Nodes) > 0 {
			allNodeIDs = append(allNodeIDs, writeResp.Nodes[0])
		}
		
		log.Printf("分片 %d/%d 上传完成: %s", i+1, chunkCount, chunkID)
	}
	
	// 创建文件记录
	file := &FileInfo{
		FileID:        fileID,
		FileName:      fileName,
		FilePath:      filePath,
		MaxVersions:   3,
		Size:          int64(len(fileData)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	fm.files[fileID] = file
	
	// 创建版本
	newVersion, err := fm.versionManager.CreateNewVersion(file, chunkIDs, allNodeIDs, int64(len(fileData)), hashStr)
	if err != nil {
		log.Printf("创建版本失败: %v", err)
	} else {
		fm.versions[fileID] = append(fm.versions[fileID], newVersion)
	}

	log.Printf("文件上传完成: %s, ID=%s, 大小=%d, 分片=%d", fileName, fileID, len(fileData), chunkCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"file_id":   fileID,
		"file_name": fileName,
		"size":      len(fileData),
		"chunks":    chunkCount,
		"hash":      hashStr,
	})
}

// downloadFile 下载文件（端到端流程）
func (fm *FileManager) downloadFile(w http.ResponseWriter, r *http.Request, fileID string, version int) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	versions := fm.versions[fileID]
	if len(versions) == 0 {
		http.Error(w, `{"error":"No versions available"}`, http.StatusNotFound)
		return
	}

	var targetVersion *FileVersion
	if version == 0 || version == file.LatestVersion {
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

	// 从各分片节点下载数据并组装
	var fileData []byte
	for _, chunkID := range targetVersion.ChunkIDs {
		data, err := fm.storage.DownloadFromNode(chunkID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Download chunk %s failed: %s", chunkID, err.Error()), http.StatusInternalServerError)
			return
		}
		fileData = append(fileData, data...)
	}

	// 返回文件数据
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))
	w.Write(fileData)
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

	newVersion, err := fm.versionManager.RollbackToVersion(file, targetVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fm.versions[fileID] = append(fm.versions[fileID], newVersion)
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

	// 删除所有版本的分片
	versions := fm.versions[fileID]
	for _, version := range versions {
		for _, chunkID := range version.ChunkIDs {
			fm.storage.DeleteChunk(chunkID)
		}
	}

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

	versions := fm.versions[fileID]
	for i, v := range versions {
		if v.Version == version {
			// 删除该版本的分片
			for _, chunkID := range v.ChunkIDs {
				fm.storage.DeleteChunk(chunkID)
			}
			
			fm.versions[fileID] = append(versions[:i], versions[i+1:]...)
			file.VersionCount--
			
			log.Printf("删除版本: 文件=%s, 版本=%d", fileID, version)
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
			})
			return
		}
	}

	http.Error(w, `{"error":"Version not found"}`, http.StatusNotFound)
}

// calculateChunkSize 根据文件大小计算分片大小
func calculateChunkSize(fileSize int64) int64 {
	switch {
	case fileSize < 10*1024*1024:     // < 10MB: 不切片
		return fileSize
	case fileSize < 100*1024*1024:    // < 100MB: 4MB
		return 4 * 1024 * 1024
	case fileSize < 1024*1024*1024:   // < 1GB: 16MB
		return 16 * 1024 * 1024
	case fileSize < 10*1024*1024*1024: // < 10GB: 64MB
		return 64 * 1024 * 1024
	default:                           // >= 10GB: 128MB
		return 128 * 1024 * 1024
	}
}
