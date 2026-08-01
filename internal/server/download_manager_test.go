package server

import (
	"testing"
)

func TestDownloadManagerCreateSession(t *testing.T) {
	dm := NewDownloadManager()
	
	// 创建测试分片
	chunks := []ChunkInfo{
		{Index: 0, Size: 1024, Hash: "hash-001"},
		{Index: 1, Size: 2048, Hash: "hash-002"},
		{Index: 2, Size: 512, Hash: "hash-003"},
	}
	
	// 创建下载会话
	session, err := dm.CreateDownloadSession("test-file", 1, chunks)
	if err != nil {
		t.Fatalf("创建下载会话失败: %v", err)
	}
	
	// 验证会话信息
	if session.FileID != "test-file" {
		t.Errorf("文件ID不匹配: 期望 %s, 实际 %s", "test-file", session.FileID)
	}
	
	if session.Version != 1 {
		t.Errorf("版本不匹配: 期望 %d, 实际 %d", 1, session.Version)
	}
	
	if len(session.Chunks) != 3 {
		t.Errorf("分片数量不匹配: 期望 %d, 实际 %d", 3, len(session.Chunks))
	}
	
	if session.Status != "pending" {
		t.Errorf("会话状态不匹配: 期望 %s, 实际 %s", "pending", session.Status)
	}
	
	// 验证分片信息
	for i, chunk := range session.Chunks {
		if chunk.Index != i {
			t.Errorf("分片索引不匹配: 期望 %d, 实际 %d", i, chunk.Index)
		}
		if chunk.Status != "pending" {
			t.Errorf("分片状态不匹配: 期望 %s, 实际 %s", "pending", chunk.Status)
		}
	}
}

func TestDownloadManagerUpdateNodes(t *testing.T) {
	dm := NewDownloadManager()
	
	// 创建测试节点
	nodes := map[string]*Node{
		"node-1": {
			NodeID:    "node-1",
			IPAddress: "192.168.1.1",
			Port:      8081,
			Status:    NodeStatusOnline,
		},
		"node-2": {
			NodeID:    "node-2",
			IPAddress: "192.168.1.2",
			Port:      8081,
			Status:    NodeStatusOnline,
		},
	}
	
	// 更新节点
	dm.UpdateNodes(nodes)
	
	// 验证节点已更新
	dm.mu.RLock()
	if len(dm.nodes) != 2 {
		t.Errorf("节点数量不匹配: 期望 %d, 实际 %d", 2, len(dm.nodes))
	}
	
	if _, exists := dm.nodes["node-1"]; !exists {
		t.Error("节点1不存在")
	}
	
	if _, exists := dm.nodes["node-2"]; !exists {
		t.Error("节点2不存在")
	}
	dm.mu.RUnlock()
}

func TestDownloadManagerAssembleFile(t *testing.T) {
	dm := NewDownloadManager()
	
	// 创建测试分片数据
	chunks := []ChunkDownloadInfo{
		{Index: 0, Size: 5, Data: []byte("Hello")},
		{Index: 1, Size: 6, Data: []byte(", World")},
		{Index: 2, Size: 1, Data: []byte("!")},
	}
	
	// 组装文件
	fileData, err := dm.AssembleFile(chunks)
	if err != nil {
		t.Fatalf("组装文件失败: %v", err)
	}
	
	// 验证文件内容
	expected := "Hello, World!"
	if string(fileData) != expected {
		t.Errorf("文件内容不匹配: 期望 %s, 实际 %s", expected, string(fileData))
	}
	
	// 验证文件大小
	expectedSize := int64(12)
	if int64(len(fileData)) != expectedSize {
		t.Errorf("文件大小不匹配: 期望 %d, 实际 %d", expectedSize, len(fileData))
	}
}

func TestDownloadManagerAssembleFileWithNilData(t *testing.T) {
	dm := NewDownloadManager()
	
	// 创建测试分片数据（包含空数据）
	chunks := []ChunkDownloadInfo{
		{Index: 0, Size: 5, Data: []byte("Hello")},
		{Index: 1, Size: 6, Data: nil}, // 空数据
		{Index: 2, Size: 1, Data: []byte("!")},
	}
	
	// 组装文件（应该失败）
	_, err := dm.AssembleFile(chunks)
	if err == nil {
		t.Error("应该返回错误")
	}
}
