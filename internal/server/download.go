package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// DownloadRequest 下载请求
type DownloadRequest struct {
	FileID  string `json:"file_id"`
	Version int    `json:"version"` // 0表示最新版本
}

// DownloadResponse 下载响应
type DownloadResponse struct {
	Success     bool        `json:"success"`
	File        *FileInfo   `json:"file"`
	Version     *FileVersion `json:"version"`
	DownloadURL string      `json:"download_url"`
	Error       string      `json:"error,omitempty"`
}

// HandleDownload 处理文件下载
func (fm *FileManager) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL提取file_id和version
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	parts := strings.SplitN(path, "/", 2)
	fileID := parts[0]

	var version int
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &version)
	}

	// 获取文件信息
	file, exists := fm.files[fileID]
	if !exists {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}

	// 获取版本
	versions := fm.versions[fileID]
	if len(versions) == 0 {
		http.Error(w, `{"error":"No versions available"}`, http.StatusNotFound)
		return
	}

	var targetVersion *FileVersion
	if version == 0 {
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

	// 从存储节点下载数据
	data, err := fm.downloadFromNodes(targetVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回文件数据
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)

	log.Printf("文件下载成功: %s, 版本=%d, 大小=%d", fileID, targetVersion.Version, len(data))
}

// downloadFromNodes 从存储节点下载数据
func (fm *FileManager) downloadFromNodes(version *FileVersion) ([]byte, error) {
	if len(version.ChunkIDs) == 0 {
		return nil, fmt.Errorf("没有分片数据")
	}

	// 下载所有分片
	var allData []byte
	for _, chunkID := range version.ChunkIDs {
		// 从第一个可用节点下载
		var chunkData []byte
		var err error

		for _, nodeID := range version.NodeIDs {
			chunkData, err = fm.downloadChunkFromNode(nodeID, chunkID)
			if err == nil {
				break
			}
			log.Printf("从节点 %s 下载分片 %s 失败: %v", nodeID, chunkID, err)
		}

		if err != nil {
			return nil, fmt.Errorf("下载分片 %s 失败: %w", chunkID, err)
		}

		allData = append(allData, chunkData...)
	}

	return allData, nil
}

// downloadChunkFromNode 从指定节点下载分片
func (fm *FileManager) downloadChunkFromNode(nodeID, chunkID string) ([]byte, error) {
	// 获取节点信息
	node, exists := fm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 构建下载URL
	url := fmt.Sprintf("http://%s:%d/api/v1/chunks/%s", node.IPAddress, node.Port, chunkID)

	// 发送下载请求
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}

	// 读取数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取数据失败: %w", err)
	}

	log.Printf("分片 %s 从节点 %s 下载成功, 大小: %d", chunkID, nodeID, len(data))

	return data, nil
}
