package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultChunkSize 默认分片大小 64MB
	DefaultChunkSize = 64 * 1024 * 1024
)

// UploadRequest 上传请求
type UploadRequest struct {
	FilePath string
	Replicas int
}

// UploadResult 上传结果
type UploadResult struct {
	Success  bool
	FileID   string
	Chunks   []string
	Error    error
}

// ChunkUploader 分片上传器
type ChunkUploader struct {
	config     *Config
	httpClient *http.Client
	chunkSize  int64
}

// NewChunkUploader 创建分片上传器
func NewChunkUploader(config *Config) *ChunkUploader {
	return &ChunkUploader{
		config:     config,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		chunkSize:  DefaultChunkSize,
	}
}

// Upload 上传文件
func (u *ChunkUploader) Upload(filePath string, replicas int) (*UploadResult, error) {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 生成文件ID
	fileID := fmt.Sprintf("%s_%d", filepath.Base(filePath), fileInfo.ModTime().UnixNano())

	// 计算分片数量
	chunkCount := (fileInfo.Size() + u.chunkSize - 1) / u.chunkSize

	fmt.Printf("开始上传: %s\n", filePath)
	fmt.Printf("文件大小: %d bytes\n", fileInfo.Size())
	fmt.Printf("分片数量: %d\n", chunkCount)
	fmt.Printf("副本数量: %d\n", replicas)

	// 逐个分片上传
	var chunkIDs []string
	for i := int64(0); i < chunkCount; i++ {
		// 计算分片偏移和大小
		offset := i * u.chunkSize
		size := u.chunkSize
		if offset+size > fileInfo.Size() {
			size = fileInfo.Size() - offset
		}

		// 读取分片数据
		chunkData := make([]byte, size)
		_, err := file.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("读取分片失败: %w", err)
		}

		// 计算分片hash
		hash := sha256.Sum256(chunkData)
		chunkHash := fmt.Sprintf("%x", hash)

		// 上传分片
		chunkID, err := u.uploadChunk(fileID, i, chunkData, chunkHash, replicas)
		if err != nil {
			return nil, fmt.Errorf("上传分片 %d 失败: %w", i, err)
		}

		chunkIDs = append(chunkIDs, chunkID)
		fmt.Printf("分片 %d/%d 上传成功\n", i+1, chunkCount)
	}

	return &UploadResult{
		Success: true,
		FileID:  fileID,
		Chunks:  chunkIDs,
	}, nil
}

// uploadChunk 上传单个分片
func (u *ChunkUploader) uploadChunk(fileID string, index int64, data []byte, hash string, replicas int) (string, error) {
	// 1. 向中心服务器请求写入
	writeReq := map[string]interface{}{
		"file_id":   fileID,
		"file_name": fmt.Sprintf("chunk_%d", index),
		"file_size": int64(len(data)),
		"replicas":  replicas,
	}

	reqBody, _ := json.Marshal(writeReq)
	resp, err := u.httpClient.Post(
		fmt.Sprintf("%s/api/v1/write", u.config.ServerAddr),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return "", fmt.Errorf("请求写入失败: %w", err)
	}
	defer resp.Body.Close()

	var writeResp struct {
		Success  bool     `json:"success"`
		ChunkIDs []string `json:"chunk_ids"`
		Nodes    []string `json:"nodes"`
		Error    string   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&writeResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if !writeResp.Success {
		return "", fmt.Errorf("写入请求失败: %s", writeResp.Error)
	}

	chunkID := writeResp.ChunkIDs[0]

	// 2. 向每个节点上传分片
	for _, nodeID := range writeResp.Nodes {
		// 获取节点地址
		nodeAddr, err := u.getNodeAddress(nodeID)
		if err != nil {
			return "", fmt.Errorf("获取节点地址失败: %w", err)
		}

		if err := u.uploadToNode(nodeAddr, chunkID, data); err != nil {
			return "", fmt.Errorf("上传到节点 %s 失败: %w", nodeID, err)
		}
	}

	// 3. 确认写入完成
	confirmReq := map[string]interface{}{
		"chunk_id": chunkID,
		"hash":     hash,
	}

	confirmBody, _ := json.Marshal(confirmReq)
	confirmResp, err := u.httpClient.Post(
		fmt.Sprintf("%s/api/v1/write/confirm", u.config.ServerAddr),
		"application/json",
		bytes.NewBuffer(confirmBody),
	)
	if err != nil {
		return "", fmt.Errorf("确认写入失败: %w", err)
	}
	defer confirmResp.Body.Close()

	// 检查响应状态码
	if confirmResp.StatusCode != http.StatusOK {
		// 读取错误响应
		body, _ := io.ReadAll(confirmResp.Body)
		return "", fmt.Errorf("确认写入失败: %s", string(body))
	}

	var confirmResult struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(confirmResp.Body).Decode(&confirmResult); err != nil {
		return "", fmt.Errorf("解析确认响应失败: %w", err)
	}

	if !confirmResult.Success {
		return "", fmt.Errorf("确认写入失败")
	}

	return chunkID, nil
}

// getNodeAddress 获取节点地址
func (u *ChunkUploader) getNodeAddress(nodeID string) (string, error) {
	resp, err := u.httpClient.Get(fmt.Sprintf("%s/api/v1/nodes/%s", u.config.ServerAddr, nodeID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var node struct {
		IPAddress string `json:"ip_address"`
		Port      int    `json:"port"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", node.IPAddress, node.Port), nil
}

// uploadToNode 上传分片到节点
func (u *ChunkUploader) uploadToNode(nodeAddr string, chunkID string, data []byte) error {
	url := fmt.Sprintf("http://%s/api/v1/upload?chunk_id=%s", nodeAddr, chunkID)

	resp, err := u.httpClient.Post(url, "application/octet-stream", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传失败: %s", resp.Status)
	}

	return nil
}
