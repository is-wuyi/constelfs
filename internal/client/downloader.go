package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ChunkDownloader struct {
	config     *Config
	httpClient *http.Client
}

func NewChunkDownloader(config *Config) *ChunkDownloader {
	return &ChunkDownloader{
		config:     config,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (d *ChunkDownloader) Download(fileID, localPath string) error {
	// 获取文件信息
	_, err := d.getFileInfo(fileID)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	fmt.Printf("开始下载: %s\n", fileID)

	// 获取加密密钥（如果有）
	encryptionKey, _ := d.getEncryptionKey(fileID)
	if encryptionKey != "" {
		fmt.Println("文件已加密，将自动解密")
	}

	// 获取最新版本信息
	versions, err := d.getVersions(fileID)
	if err != nil {
		return fmt.Errorf("获取版本信息失败: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("没有可用版本")
	}

	latestVersion := versions[len(versions)-1]
	fmt.Printf("版本: %d\n", latestVersion.Version)
	fmt.Printf("分片数: %d\n", len(latestVersion.ChunkIDs))

	// 下载所有分片
	var fileData []byte
	for i, chunkID := range latestVersion.ChunkIDs {
		chunkData, err := d.downloadChunk(chunkID)
		if err != nil {
			return fmt.Errorf("下载分片 %d 失败: %w", i, err)
		}

		// 解密分片数据（如果有加密密钥）
		if encryptionKey != "" {
			chunkData, err = decryptData(chunkData, encryptionKey)
			if err != nil {
				return fmt.Errorf("解密分片 %d 失败: %w", i, err)
			}
		}

		fileData = append(fileData, chunkData...)
		fmt.Printf("分片 %d/%d 下载成功\n", i+1, len(latestVersion.ChunkIDs))
	}

	// 保存文件
	if err := os.WriteFile(localPath, fileData, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	fmt.Printf("下载完成: %s\n", localPath)
	return nil
}

func (d *ChunkDownloader) getFileInfo(fileID string) (*FileInfo, error) {
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/files/%s", d.config.ServerAddr, fileID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取文件信息失败: %s", resp.Status)
	}

	var result struct {
		File     *FileInfo      `json:"file"`
		Versions []FileVersion  `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.File, nil
}

func (d *ChunkDownloader) getVersions(fileID string) ([]FileVersion, error) {
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/files/%s", d.config.ServerAddr, fileID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取版本信息失败: %s", resp.Status)
	}

	var result struct {
		File     *FileInfo      `json:"file"`
		Versions []FileVersion  `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Versions, nil
}

func (d *ChunkDownloader) getEncryptionKey(fileID string) (string, error) {
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/encryption/key/%s", d.config.ServerAddr, fileID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取加密密钥失败: %s", resp.Status)
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Key, nil
}

func (d *ChunkDownloader) downloadChunk(chunkID string) ([]byte, error) {
	// 从服务器获取分片信息（包含副本节点）
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/chunks/%s/status", d.config.ServerAddr, chunkID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取分片信息失败: %s", resp.Status)
	}

	var chunkInfo struct {
		ChunkID  string   `json:"chunk_id"`
		Replicas []string `json:"replicas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chunkInfo); err != nil {
		return nil, err
	}

	// 从第一个可用节点下载
	for _, nodeID := range chunkInfo.Replicas {
		nodeAddr, err := d.getNodeAddress(nodeID)
		if err != nil {
			continue
		}

		data, err := d.downloadFromNode(nodeAddr, chunkID)
		if err != nil {
			continue
		}

		return data, nil
	}

	return nil, fmt.Errorf("所有节点下载失败")
}

func (d *ChunkDownloader) downloadFromNode(nodeAddr, chunkID string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/api/v1/chunks/%s", nodeAddr, chunkID)

	resp, err := d.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (d *ChunkDownloader) getNodeAddress(nodeID string) (string, error) {
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/nodes/%s", d.config.ServerAddr, nodeID))
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

// DownloadVersion 下载指定版本
func (d *ChunkDownloader) DownloadVersion(fileID string, version int, localPath string) error {
	// 获取加密密钥
	encryptionKey, _ := d.getEncryptionKey(fileID)

	// 获取版本信息
	versions, err := d.getVersions(fileID)
	if err != nil {
		return fmt.Errorf("获取版本信息失败: %w", err)
	}

	// 查找指定版本
	var targetVersion *FileVersion
	for _, v := range versions {
		if v.Version == version {
			targetVersion = &v
			break
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("版本 %d 不存在", version)
	}

	fmt.Printf("下载版本: %d\n", targetVersion.Version)

	// 下载所有分片
	var fileData []byte
	for i, chunkID := range targetVersion.ChunkIDs {
		chunkData, err := d.downloadChunk(chunkID)
		if err != nil {
			return fmt.Errorf("下载分片 %d 失败: %w", i, err)
		}

		// 解密分片数据
		if encryptionKey != "" {
			chunkData, err = decryptData(chunkData, encryptionKey)
			if err != nil {
				return fmt.Errorf("解密分片 %d 失败: %w", i, err)
			}
		}

		fileData = append(fileData, chunkData...)
	}

	// 保存文件
	if err := os.WriteFile(localPath, fileData, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	fmt.Printf("下载完成: %s\n", localPath)
	return nil
}
