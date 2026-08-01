package client

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultChunkSize = 64 * 1024 * 1024
)

type UploadResult struct {
	Success      bool
	FileID       string
	Chunks       []string
	EncryptionKey string
	Error        error
}

type ChunkUploader struct {
	config     *Config
	httpClient *http.Client
	chunkSize  int64
	encrypt    bool
}

func NewChunkUploader(config *Config) *ChunkUploader {
	return &ChunkUploader{
		config:     config,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		chunkSize:  DefaultChunkSize,
		encrypt:    config.Encrypt,
	}
}

// generateKey 生成AES-256密钥
func generateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// encryptData 加密数据
func encryptData(data []byte, keyStr string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decryptData 解密数据
func decryptData(ciphertext []byte, keyStr string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (u *ChunkUploader) Upload(filePath string, replicas int) (*UploadResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileID := fmt.Sprintf("%s_%d", filepath.Base(filePath), fileInfo.ModTime().UnixNano())

	// 生成加密密钥（如果启用加密）
	var encryptionKey string
	if u.encrypt {
		encryptionKey, err = generateKey()
		if err != nil {
			return nil, fmt.Errorf("生成加密密钥失败: %w", err)
		}
	}

	chunkCount := (fileInfo.Size() + u.chunkSize - 1) / u.chunkSize

	fmt.Printf("开始上传: %s\n", filePath)
	fmt.Printf("文件大小: %d bytes\n", fileInfo.Size())
	fmt.Printf("分片数量: %d\n", chunkCount)
	fmt.Printf("副本数量: %d\n", replicas)
	fmt.Printf("加密: %v\n", u.encrypt)

	var chunkIDs []string
	var chunkHash string
	for i := int64(0); i < chunkCount; i++ {
		offset := i * u.chunkSize
		size := u.chunkSize
		if offset+size > fileInfo.Size() {
			size = fileInfo.Size() - offset
		}

		chunkData := make([]byte, size)
		_, err := file.ReadAt(chunkData, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("读取分片失败: %w", err)
		}

		// 加密分片数据（如果启用加密）
		uploadData := chunkData
		if u.encrypt && encryptionKey != "" {
			uploadData, err = encryptData(chunkData, encryptionKey)
			if err != nil {
				return nil, fmt.Errorf("加密分片失败: %w", err)
			}
		}

		hash := sha256.Sum256(chunkData) // 使用原始数据的hash
		chunkHash = fmt.Sprintf("%x", hash)

		chunkID, err := u.uploadChunk(fileID, i, uploadData, chunkHash, replicas)
		if err != nil {
			return nil, fmt.Errorf("上传分片 %d 失败: %w", i, err)
		}

		chunkIDs = append(chunkIDs, chunkID)
		fmt.Printf("分片 %d/%d 上传成功\n", i+1, chunkCount)
	}

	if err := u.createVersion(fileID, filepath.Base(filePath), filePath, fileInfo.Size(), chunkHash, chunkIDs, replicas, encryptionKey); err != nil {
		return nil, fmt.Errorf("创建版本失败: %w", err)
	}

	return &UploadResult{
		Success:       true,
		FileID:        fileID,
		Chunks:        chunkIDs,
		EncryptionKey: encryptionKey,
	}, nil
}

func (u *ChunkUploader) createVersion(fileID, fileName, filePath string, fileSize int64, hash string, chunkIDs []string, replicas int, encryptionKey string) error {
	req := map[string]interface{}{
		"file_id":   fileID,
		"file_name": fileName,
		"file_path": filePath,
		"file_size": fileSize,
		"hash":      hash,
		"chunk_ids": chunkIDs,
		"node_ids":  []string{},
	}

	reqBody, _ := json.Marshal(req)
	resp, err := u.httpClient.Post(
		fmt.Sprintf("%s/api/v1/version/create", u.config.ServerAddr),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("请求创建版本失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("创建版本失败: %s", result.Error)
	}

	// 保存加密密钥到服务器
	if encryptionKey != "" {
		if err := u.saveEncryptionKey(fileID, encryptionKey); err != nil {
			fmt.Printf("警告: 保存加密密钥失败: %v\n", err)
		}
	}

	return nil
}

func (u *ChunkUploader) saveEncryptionKey(fileID, key string) error {
	req := map[string]interface{}{
		"file_id": fileID,
		"key":     key,
	}

	reqBody, _ := json.Marshal(req)
	resp, err := u.httpClient.Post(
		fmt.Sprintf("%s/api/v1/encryption/key", u.config.ServerAddr),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("保存密钥失败: %s", resp.Status)
	}

	return nil
}

func (u *ChunkUploader) uploadChunk(fileID string, index int64, data []byte, hash string, replicas int) (string, error) {
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
		Success   bool     `json:"success"`
		ChunkID   string   `json:"chunk_id"`
		ChunkIDs  []string `json:"chunk_ids"`
		Nodes     []string `json:"nodes"`
		NodeAddrs []string `json:"node_addrs"`
		Error     string   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&writeResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if !writeResp.Success {
		return "", fmt.Errorf("写入请求失败: %s", writeResp.Error)
	}

	chunkID := writeResp.ChunkID
	if len(writeResp.ChunkIDs) > 0 {
		chunkID = writeResp.ChunkIDs[0]
	}

	for _, nodeAddr := range writeResp.NodeAddrs {
		if err := u.uploadToNode(nodeAddr, chunkID, data); err != nil {
			return "", fmt.Errorf("上传到节点 %s 失败: %w", nodeAddr, err)
		}
	}

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

func (u *ChunkUploader) uploadToNode(nodeAddr string, chunkID string, data []byte) error {
	url := fmt.Sprintf("http://%s/api/v1/chunks/%s", nodeAddr, chunkID)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传失败: %s", resp.Status)
	}

	return nil
}
