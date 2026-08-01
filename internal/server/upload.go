package server

import (
	"bytes"
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
	// 1. 将数据分片
	chunker := &FileChunker{FileSize: int64(len(data))}
	chunkSize := chunker.GetChunkSize()
	chunkCount := chunker.GetChunkCount()

	chunkIDs := make([]string, 0, chunkCount)
	nodeIDs := make([]string, 0, replicas)

	// 2. 选择存储节点
	nodes := fm.scheduler.SelectNodes(nil, replicas, nil)
	if len(nodes) < replicas {
		return nil, nil, fmt.Errorf("可用节点不足: 需要%d, 可用%d", replicas, len(nodes))
	}

	// 记录节点ID
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
	}

	// 3. 上传分片到接收节点（第一个节点）
	for i := int64(0); i < chunkCount; i++ {
		offset := i * chunkSize
		size := chunkSize
		if offset+size > int64(len(data)) {
			size = int64(len(data)) - offset
		}

		chunkData := data[offset : offset+size]
		chunkID := fmt.Sprintf("%s_chunk_%d", fileID, i)

		// 上传到第一个节点
		receiverNode := nodes[0]
		err := fm.uploadToNode(receiverNode, chunkID, chunkData)
		if err != nil {
			return nil, nil, fmt.Errorf("上传分片到节点 %s 失败: %w", receiverNode.NodeID, err)
		}

		// 4. 接收节点分发到副本节点
		if len(nodes) > 1 {
			err = fm.replicateToNodes(chunkID, chunkData, nodes[1:])
			if err != nil {
				log.Printf("分发分片 %s 失败: %v", chunkID, err)
				// 继续处理其他分片
			}
		}

		chunkIDs = append(chunkIDs, chunkID)
	}

	return chunkIDs, nodeIDs, nil
}

// uploadToNode 上传分片到指定节点
func (fm *FileManager) uploadToNode(node *Node, chunkID string, data []byte) error {
	url := fmt.Sprintf("http://%s:%d/api/v1/chunks/%s", node.IPAddress, node.Port, chunkID)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传失败: %s", resp.Status)
	}

	log.Printf("分片 %s 上传到节点 %s 成功", chunkID, node.NodeID)
	return nil
}

// replicateToNodes 分发分片到多个节点
func (fm *FileManager) replicateToNodes(chunkID string, data []byte, nodes []*Node) error {
	for _, node := range nodes {
		err := fm.uploadToNode(node, chunkID, data)
		if err != nil {
			log.Printf("分发分片 %s 到节点 %s 失败: %v", chunkID, node.NodeID, err)
			// 继续分发到其他节点
			continue
		}
	}
	return nil
}
