package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileChunkerGetChunkSize(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int64
		expected int64
	}{
		{"小于10MB", 5 * 1024 * 1024, 5 * 1024 * 1024},
		{"10MB", 10 * 1024 * 1024, 4 * 1024 * 1024},
		{"50MB", 50 * 1024 * 1024, 4 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024, 16 * 1024 * 1024},
		{"500MB", 500 * 1024 * 1024, 16 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024, 64 * 1024 * 1024},
		{"5GB", 5 * 1024 * 1024 * 1024, 64 * 1024 * 1024},
		{"10GB", 10 * 1024 * 1024 * 1024, 128 * 1024 * 1024},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := &FileChunker{
				FileSize: tt.fileSize,
			}
			got := chunker.GetChunkSize()
			if got != tt.expected {
				t.Errorf("GetChunkSize() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

func TestFileChunkerGetChunkCount(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int64
		expected int64
	}{
		{"小于10MB", 5 * 1024 * 1024, 1},
		{"10MB", 10 * 1024 * 1024, 3},
		{"100MB", 100 * 1024 * 1024, 7},
		{"1GB", 1024 * 1024 * 1024, 16},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := &FileChunker{
				FileSize: tt.fileSize,
			}
			got := chunker.GetChunkCount()
			if got != tt.expected {
				t.Errorf("GetChunkCount() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

func TestFileChunkerChunkFile(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	
	// 写入测试数据
	testData := make([]byte, 1024*1024) // 1MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	err := os.WriteFile(filePath, testData, 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	
	// 创建分片器
	chunker, err := NewFileChunker(filePath)
	if err != nil {
		t.Fatalf("创建分片器失败: %v", err)
	}
	
	// 验证文件大小
	if chunker.FileSize != int64(len(testData)) {
		t.Errorf("文件大小不匹配: 期望 %d, 实际 %d", len(testData), chunker.FileSize)
	}
	
	// 获取分片信息
	chunks, err := chunker.ChunkFile()
	if err != nil {
		t.Fatalf("分片失败: %v", err)
	}
	
	// 验证分片数量
	expectedChunks := int64(1) // 1MB文件应该只有1个分片
	if int64(len(chunks)) != expectedChunks {
		t.Errorf("分片数量不匹配: 期望 %d, 实际 %d", expectedChunks, len(chunks))
	}
	
	// 验证分片大小
	if chunks[0].Size != int64(len(testData)) {
		t.Errorf("分片大小不匹配: 期望 %d, 实际 %d", len(testData), chunks[0].Size)
	}
	
	// 验证分片hash
	if chunks[0].Hash == "" {
		t.Error("分片hash不能为空")
	}
}

func TestFileChunkerReadChunk(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	
	// 写入测试数据
	testData := []byte("Hello, ConstelFS! This is a test file.")
	
	err := os.WriteFile(filePath, testData, 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	
	// 创建分片器
	chunker, err := NewFileChunker(filePath)
	if err != nil {
		t.Fatalf("创建分片器失败: %v", err)
	}
	
	// 读取分片
	data, err := chunker.ReadChunk(0)
	if err != nil {
		t.Fatalf("读取分片失败: %v", err)
	}
	
	// 验证分片内容
	if string(data) != string(testData) {
		t.Errorf("分片内容不匹配: 期望 %s, 实际 %s", string(testData), string(data))
	}
}
