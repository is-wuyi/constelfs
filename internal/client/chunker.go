package client

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

const (
	// 分片大小常量
	ChunkSize10MB  = 10 * 1024 * 1024
	ChunkSize100MB = 100 * 1024 * 1024
	ChunkSize1GB   = 1024 * 1024 * 1024
	ChunkSize10GB  = 10 * 1024 * 1024 * 1024
)

// ChunkInfo 分片信息
type ChunkInfo struct {
	Index    int    `json:"index"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
	Offset   int64  `json:"offset"`
}

// FileChunker 文件分片器
type FileChunker struct {
	FilePath string
	FileSize int64
}

// NewFileChunker 创建文件分片器
func NewFileChunker(filePath string) (*FileChunker, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return &FileChunker{
		FilePath: filePath,
		FileSize: fileInfo.Size(),
	}, nil
}

// GetChunkSize 根据文件大小获取分片大小
func (fc *FileChunker) GetChunkSize() int64 {
	switch {
	case fc.FileSize < ChunkSize10MB:
		return fc.FileSize // 不切片
	case fc.FileSize < ChunkSize100MB:
		return 4 * 1024 * 1024 // 4MB
	case fc.FileSize < ChunkSize1GB:
		return 16 * 1024 * 1024 // 16MB
	case fc.FileSize < ChunkSize10GB:
		return 64 * 1024 * 1024 // 64MB
	default:
		return 128 * 1024 * 1024 // 128MB
	}
}

// GetChunkCount 获取分片数量
func (fc *FileChunker) GetChunkCount() int64 {
	chunkSize := fc.GetChunkSize()
	if chunkSize == 0 {
		return 0
	}
	return (fc.FileSize + chunkSize - 1) / chunkSize
}

// ChunkFile 将文件分片
func (fc *FileChunker) ChunkFile() ([]ChunkInfo, error) {
	file, err := os.Open(fc.FilePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	chunkSize := fc.GetChunkSize()
	chunkCount := fc.GetChunkCount()
	chunks := make([]ChunkInfo, 0, chunkCount)

	var offset int64
	for i := int64(0); i < chunkCount; i++ {
		size := chunkSize
		if offset+size > fc.FileSize {
			size = fc.FileSize - offset
		}

		// 读取分片数据
		data := make([]byte, size)
		_, err := io.ReadFull(file, data)
		if err != nil {
			return nil, fmt.Errorf("读取分片失败: %w", err)
		}

		// 计算分片hash
		hash := sha256.Sum256(data)
		hashStr := fmt.Sprintf("%x", hash)

		chunks = append(chunks, ChunkInfo{
			Index:  int(i),
			Size:   size,
			Hash:   hashStr,
			Offset: offset,
		})

		offset += size
	}

	return chunks, nil
}

// ReadChunk 读取指定分片的数据
func (fc *FileChunker) ReadChunk(chunkIndex int) ([]byte, error) {
	file, err := os.Open(fc.FilePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	chunkSize := fc.GetChunkSize()
	offset := int64(chunkIndex) * chunkSize
	
	size := chunkSize
	if offset+size > fc.FileSize {
		size = fc.FileSize - offset
	}

	data := make([]byte, size)
	_, err = file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("读取分片失败: %w", err)
	}

	return data, nil
}
