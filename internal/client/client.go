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
}

// FileInfo 文件信息
type FileInfo struct {
	Name string
	Mode string
	Size int64
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
	return c, nil
}

// Upload 上传文件
func (c *Client) Upload(filePath string, replicas int) (*UploadResult, error) {
	return c.uploader.Upload(filePath, replicas)
}

// Download 下载文件
func (c *Client) Download(fileID, localPath string) error {
	// TODO: 实现文件下载逻辑
	return fmt.Errorf("下载功能尚未实现")
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
		Files []struct {
			FileName string `json:"file_name"`
			FileSize int64  `json:"file_size"`
			IsDir    bool   `json:"is_directory"`
		} `json:"files"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	files := make([]*FileInfo, 0, len(result.Files))
	for _, f := range result.Files {
		mode := "-rw-r--r--"
		if f.IsDir {
			mode = "drwxr-xr-x"
		}
		files = append(files, &FileInfo{
			Name: f.FileName,
			Mode: mode,
			Size: f.FileSize,
		})
	}

	return files, nil
}

// Delete 删除文件
func (c *Client) Delete(fileID string) error {
	// TODO: 实现文件删除逻辑
	return fmt.Errorf("删除功能尚未实现")
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
