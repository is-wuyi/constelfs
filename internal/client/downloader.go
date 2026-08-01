package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ChunkDownloader 分片下载器
type ChunkDownloader struct {
	config     *Config
	httpClient *http.Client
}

// NewChunkDownloader 创建分片下载器
func NewChunkDownloader(config *Config) *ChunkDownloader {
	return &ChunkDownloader{
		config:     config,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Download 下载文件
func (d *ChunkDownloader) Download(fileID string, outputPath string) error {
	// 1. 从中心服务器获取文件信息
	fileInfo, versions, err := d.getFileInfo(fileID)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("没有可用的版本")
	}

	// 2. 获取最新版本
	latestVersion := versions[len(versions)-1]

	fmt.Printf("开始下载: %s\n", fileInfo.FileName)
	fmt.Printf("文件大小: %d bytes\n", latestVersion.Size)
	fmt.Printf("分片数量: %d\n", len(latestVersion.ChunkIDs))

	// 3. 下载所有分片
	var allData []byte
	for i, chunkID := range latestVersion.ChunkIDs {
		// 获取分片所在的节点
		nodeID := latestVersion.NodeIDs[i%len(latestVersion.NodeIDs)]

		// 从节点下载分片
		chunkData, err := d.downloadChunk(nodeID, chunkID)
		if err != nil {
			return fmt.Errorf("下载分片 %s 失败: %w", chunkID, err)
		}

		allData = append(allData, chunkData...)
		fmt.Printf("分片 %d/%d 下载成功\n", i+1, len(latestVersion.ChunkIDs))
	}

	// 4. 保存到文件
	if err := os.WriteFile(outputPath, allData, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	fmt.Printf("下载完成: %s\n", outputPath)
	return nil
}

// getFileInfo 获取文件信息
func (d *ChunkDownloader) getFileInfo(fileID string) (*FileInfo, []*FileVersion, error) {
	resp, err := d.httpClient.Get(fmt.Sprintf("%s/api/v1/files/%s", d.config.ServerAddr, fileID))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var result struct {
		File     FileInfo       `json:"file"`
		Versions []*FileVersion `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	return &result.File, result.Versions, nil
}

// downloadChunk 下载分片
func (d *ChunkDownloader) downloadChunk(nodeID string, chunkID string) ([]byte, error) {
	// 获取节点地址
	nodeAddr, err := d.getNodeAddress(nodeID)
	if err != nil {
		return nil, fmt.Errorf("获取节点地址失败: %w", err)
	}

	// 下载分片
	url := fmt.Sprintf("http://%s/api/v1/chunks/download?chunk_id=%s", nodeAddr, chunkID)
	resp, err := d.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}

	// 读取数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// getNodeAddress 获取节点地址
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
