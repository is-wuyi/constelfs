package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client ConstelFS客户端
type Client struct {
	config     *Config
	httpClient *http.Client
	uploader   *ChunkUploader
	downloader *ChunkDownloader
}

// FileInfo 文件信息
type FileInfo struct {
	FileID        string `json:"file_id"`
	FileName      string `json:"file_name"`
	FilePath      string `json:"file_path"`
	FileSize      int64  `json:"file_size"`
	IsDirectory   bool   `json:"is_directory"`
	LatestVersion int    `json:"latest_version"`
	VersionCount  int    `json:"version_count"`
	MaxVersions   int    `json:"max_versions"`
	EncryptionKey string `json:"encryption_key"`
}

// FileVersion 文件版本
type FileVersion struct {
	VersionID string   `json:"version_id"`
	FileID    string   `json:"file_id"`
	Version   int      `json:"version"`
	Size      int64    `json:"size"`
	Hash      string   `json:"hash"`
	ChunkIDs  []string `json:"chunk_ids"`
	NodeIDs   []string `json:"node_ids"`
	CreatedAt string   `json:"created_at"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID    string `json:"node_id"`
	IPAddress string `json:"ip_address"`
	Status    string `json:"status"`
}

// New 创建新的客户端
func New(config *Config) (*Client, error) {
	c := &Client{
		config:     config,
		httpClient: &http.Client{},
	}
	c.uploader = NewChunkUploader(config)
	c.downloader = NewChunkDownloader(config)
	return c, nil
}

// Upload 上传文件
func (c *Client) Upload(filePath string, replicas int) (*UploadResult, error) {
	return c.uploader.Upload(filePath, replicas)
}

// Download 下载文件
func (c *Client) Download(fileID, localPath string) error {
	return c.downloader.Download(fileID, localPath)
}

// List 列出文件
func (c *Client) List(dirPath string) ([]*FileInfo, error) {
	url := fmt.Sprintf("%s/api/v1/files?dir=%s", c.config.ServerAddr, dirPath)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Files []FileInfo `json:"files"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	files := make([]*FileInfo, 0, len(result.Files))
	for i := range result.Files {
		files = append(files, &result.Files[i])
	}

	return files, nil
}

// Delete 删除文件
func (c *Client) Delete(fileID string) error {
	url := fmt.Sprintf("%s/api/v1/files/%s", c.config.ServerAddr, fileID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("删除失败: %s", resp.Status)
	}

	return nil
}

// ListNodes 列出节点
func (c *Client) ListNodes() ([]*NodeInfo, error) {
	url := fmt.Sprintf("%s/api/v1/nodes", c.config.ServerAddr)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []NodeInfo `json:"nodes"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	nodes := make([]*NodeInfo, 0, len(result.Nodes))
	for i := range result.Nodes {
		nodes = append(nodes, &result.Nodes[i])
	}

	return nodes, nil
}
