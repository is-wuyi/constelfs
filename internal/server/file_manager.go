package server

import (
	"os"
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
	persist        *PersistenceManager
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

// SetPersistence 设置持久化管理器
func (fm *FileManager) SetPersistence(persist *PersistenceManager) {
	fm.persist = persist
}

// UpdateNodes 更新节点列表
func (fm *FileManager) UpdateNodes(nodes map[string]*Node) {
	fm.nodes = nodes
}

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

func (fm *FileManager) HandleFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	fileID := parts[0]
	
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		fm.getFile(w, r, fileID)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		fm.deleteFile(w, r, fileID)
	case len(parts) == 2 && parts[1] == "download" && r.Method == http.MethodGet:
		fm.downloadFile(w, r, fileID, 0)
	case len(parts) == 2 && parts[1] == "rollback" && r.Method == http.MethodPost:
		fm.rollbackFile(w, r, fileID)
	case len(parts) == 4 && parts[1] == "versions" && parts[3] == "download" && r.Method == http.MethodGet:
		version, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, "Invalid version", http.StatusBadRequest)
			return
		}
		fm.downloadFile(w, r, fileID, version)
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

func (fm *FileManager) HandleUpload(w http.ResponseWriter, r *http.Request) {
	fm.uploadFile(w, r)
}

func (fm *FileManager) HandleDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	fm.downloadFile(w, r, path, 0)
}

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

	newVersion, err := fm.versionManager.CreateNewVersion(file, req.ChunkIDs, req.NodeIDs, req.FileSize, req.Hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fm.versions[req.FileID] = append(fm.versions[req.FileID], newVersion)
	fm.versions[req.FileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[req.FileID])

	// 持久化
	if fm.persist != nil {
		fm.persist.SaveFile(file)
		fm.persist.SaveVersion(req.FileID, newVersion)
	}

	log.Printf("版本创建成功: 文件=%s, 版本=%d", req.FileID, newVersion.Version)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"version": newVersion,
	})
}

func (fm *FileManager) listFiles(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("dir")
	if dirPath == "" {
		dirPath = "/"
	}

	var files []*FileInfo
	for _, file := range fm.files {
		if dirPath == "/" {
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

func (fm *FileManager) uploadFile(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	
	var fileName, filePath string
	var tempFile *os.File
	var fileSize int64
	var err error
	
	// 创建临时文件用于流式处理
	tempFile, err = os.CreateTemp("", "constelfs-upload-*")
	if err != nil {
		http.Error(w, "Create temp file failed", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	var fileReader io.Reader
	
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.ParseMultipartForm(32 << 20) // 32MB 内存缓存，其余写入磁盘
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
		fileReader = file
	} else {
		fileName = r.Header.Get("X-File-Name")
		filePath = r.Header.Get("X-File-Path")
		if filePath == "" {
			filePath = "/" + fileName
		}
		fileReader = r.Body
	}
	
	if fileName == "" {
		fileName = "unnamed"
	}

	// 流式写入临时文件并计算hash
	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)
	
	fileSize, err = io.Copy(writer, fileReader)
	if err != nil {
		http.Error(w, "Read file failed", http.StatusInternalServerError)
		return
	}
	
	hashStr := fmt.Sprintf("%x", hasher.Sum(nil))
	
	// 回到文件开头
	tempFile.Seek(0, 0)

	replicas := 3
	if r := r.URL.Query().Get("replicas"); r != "" {
		if v, err := strconv.Atoi(r); err == nil && v > 0 {
			replicas = v
		}
	}

	fileID := fmt.Sprintf("%s_%d", fileName, time.Now().UnixNano())
	chunkSize := calculateChunkSize(fileSize)
	chunkCount := (fileSize + chunkSize - 1) / chunkSize
	
	var chunkIDs []string
	var allNodeIDs []string
	
	// 流式分片处理
	chunkBuf := make([]byte, chunkSize)
	for i := int64(0); i < chunkCount; i++ {
		n, readErr := io.ReadFull(tempFile, chunkBuf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			http.Error(w, fmt.Sprintf("Read chunk %d failed: %s", i, readErr.Error()), http.StatusInternalServerError)
			return
		}
		if n == 0 {
			break
		}
		chunkData := chunkBuf[:n]
		
		writeReq := &WriteRequest{
			FileID:   fileID,
			FileName: fmt.Sprintf("chunk_%d", i),
			FileSize: int64(n),
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
		
		if len(writeResp.NodeAddrs) > 0 {
			firstNodeAddr := writeResp.NodeAddrs[0]
			if err := fm.storage.UploadToNode(firstNodeAddr, chunkID, chunkData); err != nil {
				http.Error(w, fmt.Sprintf("Upload chunk %d to node failed: %s", i, err.Error()), http.StatusInternalServerError)
				return
			}
			
			chunkHash := sha256.Sum256(chunkData)
			if err := fm.storage.ConfirmWrite(chunkID, fmt.Sprintf("%x", chunkHash)); err != nil {
				http.Error(w, "Confirm write failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			
			for _, addr := range writeResp.NodeAddrs[1:] {
				allNodeIDs = append(allNodeIDs, addr)
			}
		}
		
		log.Printf("分片 %d/%d 上传完成", i+1, chunkCount)
	}

	// 创建版本
	file := fm.files[fileID]
	if file == nil {
		file = &FileInfo{
			FileID:      fileID,
			FileName:    fileName,
			FilePath:    filePath,
			Size:        fileSize,
			MaxVersions: 3,
			CreatedAt:   time.Now(),
		}
		fm.files[fileID] = file
	}

	newVersion, err := fm.versionManager.CreateNewVersion(file, chunkIDs, allNodeIDs, fileSize, hashStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fm.versions[fileID] = append(fm.versions[fileID], newVersion)
	fm.versions[fileID], _ = fm.versionManager.CleanupOldVersions(file, fm.versions[fileID])

	if fm.persist != nil {
		fm.persist.SaveFile(file)
		fm.persist.SaveVersion(fileID, newVersion)
		for _, chunkID := range chunkIDs {
			fm.persist.SaveChunk(&ChunkInfo{
				ChunkID:  chunkID,
				FileID:   fileID,
				Size:     chunkSize,
				NodeIDs:  allNodeIDs,
			})
		}
	}

	log.Printf("文件上传完成: %s, ID=%s, 大小=%d, 分片=%d", fileName, fileID, fileSize, len(chunkIDs))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"file_id":    fileID,
		"file_name":  fileName,
		"file_size":  fileSize,
		"chunk_count": len(chunkIDs),
		"version":    newVersion,
	})
}

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
		targetVersion = versions[len(versions)-1]
	} else {
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

	var fileData []byte
	for _, chunkID := range targetVersion.ChunkIDs {
		data, err := fm.storage.DownloadFromNode(chunkID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Download chunk %s failed: %s", chunkID, err.Error()), http.StatusInternalServerError)
			return
		}
		fileData = append(fileData, data...)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))
	w.Write(fileData)
}

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

	// 持久化
	if fm.persist != nil {
		fm.persist.SaveFile(file)
		fm.persist.SaveVersion(fileID, newVersion)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"new_version": newVersion,
	})
}

func (fm *FileManager) deleteFile(w http.ResponseWriter, r *http.Request, fileID string) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	versions := fm.versions[fileID]
	for _, version := range versions {
		for _, chunkID := range version.ChunkIDs {
			fm.storage.DeleteChunk(chunkID)
		}
	}

	delete(fm.files, fileID)
	delete(fm.versions, fileID)

	// 持久化删除
	if fm.persist != nil {
		fm.persist.DeleteFile(fileID)
	}

	log.Printf("删除文件: %s, 路径=%s", fileID, file.FilePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (fm *FileManager) deleteVersion(w http.ResponseWriter, r *http.Request, fileID string, version int) {
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	versions := fm.versions[fileID]
	for i, v := range versions {
		if v.Version == version {
			for _, chunkID := range v.ChunkIDs {
				fm.storage.DeleteChunk(chunkID)
			}
			
			fm.versions[fileID] = append(versions[:i], versions[i+1:]...)
			file.VersionCount--
			
			// 持久化
			if fm.persist != nil {
				fm.persist.SaveFile(file)
			}
			
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

func calculateChunkSize(fileSize int64) int64 {
	switch {
	case fileSize < 10*1024*1024:
		return fileSize
	case fileSize < 100*1024*1024:
		return 4 * 1024 * 1024
	case fileSize < 1024*1024*1024:
		return 16 * 1024 * 1024
	case fileSize < 10*1024*1024*1024:
		return 64 * 1024 * 1024
	default:
		return 128 * 1024 * 1024
	}
}
