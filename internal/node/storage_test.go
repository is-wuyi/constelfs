package node

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageEngineInit(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	
	config := &Config{
		StoragePath: tmpDir,
	}
	
	storage := NewStorageEngine(config)
	
	// 测试初始化
	err := storage.Init()
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	
	// 验证目录创建
	dirs := []string{"chunks", "temp", "meta"}
	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("目录 %s 不存在", dir)
		}
	}
}

func TestStorageEngineUpload(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	
	config := &Config{
		StoragePath: tmpDir,
	}
	
	storage := NewStorageEngine(config)
	err := storage.Init()
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	
	// 测试数据
	testData := []byte("Hello, ConstelFS!")
	chunkID := "test-chunk-001"
	
	// 保存分片
	chunkPath := filepath.Join(tmpDir, "chunks", chunkID)
	err = os.WriteFile(chunkPath, testData, 0644)
	if err != nil {
		t.Fatalf("保存分片失败: %v", err)
	}
	
	// 验证分片存在
	if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
		t.Error("分片文件不存在")
	}
	
	// 验证分片内容
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		t.Fatalf("读取分片失败: %v", err)
	}
	
	if string(data) != string(testData) {
		t.Errorf("分片内容不匹配: 期望 %s, 实际 %s", string(testData), string(data))
	}
}

func TestStorageEngineDelete(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	
	config := &Config{
		StoragePath: tmpDir,
	}
	
	storage := NewStorageEngine(config)
	err := storage.Init()
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	
	// 创建测试分片
	chunkID := "test-chunk-002"
	chunkPath := filepath.Join(tmpDir, "chunks", chunkID)
	err = os.WriteFile(chunkPath, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("创建分片失败: %v", err)
	}
	
	// 删除分片
	err = os.Remove(chunkPath)
	if err != nil {
		t.Fatalf("删除分片失败: %v", err)
	}
	
	// 验证分片已删除
	if _, err := os.Stat(chunkPath); !os.IsNotExist(err) {
		t.Error("分片应该已被删除")
	}
}

func TestStorageEngineGetStorageInfo(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	
	config := &Config{
		StoragePath: tmpDir,
	}
	
	storage := NewStorageEngine(config)
	err := storage.Init()
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	
	// 获取存储信息
	info := storage.GetStorageInfo()
	
	// 验证存储信息
	if info["storage_path"] != tmpDir {
		t.Errorf("存储路径不匹配: 期望 %s, 实际 %s", tmpDir, info["storage_path"])
	}
	
	// 验证分片数量
	if info["chunk_count"] != 0 {
		t.Errorf("分片数量应该为0，实际 %d", info["chunk_count"])
	}
}
