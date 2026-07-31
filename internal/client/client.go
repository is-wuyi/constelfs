package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Client ConstelFS客户端
type Client struct {
	config     *Config
	httpClient *http.Client
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
	return &Client{
		config:     config,
		httpClient: &http.Client{},
	}, nil
}

// Upload 上传文件
func (c *Client) Upload(localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// TODO: 实现文件上传逻辑
	// 1. 获取文件信息
	// 2. 向中心服务器请求上传节点
	// 3. 分片上传到存储节点

	return fmt.Errorf("上传功能尚未实现")
}

// Download 下载文件
func (c *Client) Download(remotePath, localPath string) error {
	// TODO: 实现文件下载逻辑
	// 1. 向中心服务器查询文件位置
	// 2. 从存储节点下载分片
	// 3. 组装并保存到本地

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
func (c *Client) Delete(remotePath string) error {
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
