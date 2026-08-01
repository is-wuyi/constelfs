package server

import (
	"testing"
)

func TestCalculateChunkSize(t *testing.T) {
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
			got := calculateChunkSize(tt.fileSize)
			if got != tt.expected {
				t.Errorf("calculateChunkSize(%d) = %d, 期望 %d", tt.fileSize, got, tt.expected)
			}
		})
	}
}

func TestUploadManagerCreateSession(t *testing.T) {
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	encMgr := NewEncryptionManager()
	uploadMgr := NewUploadManager(scheduler, storage, encMgr)
	
	// 创建上传会话
	session, err := uploadMgr.CreateUploadSession(
		"test-file-001",
		"test.txt",
		"/test.txt",
		100*1024*1024, // 100MB
		3,
		3,
	)
	
	if err != nil {
		t.Fatalf("创建上传会话失败: %v", err)
	}
	
	// 验证会话信息
	if session.FileID != "test-file-001" {
		t.Errorf("文件ID不匹配: 期望 %s, 实际 %s", "test-file-001", session.FileID)
	}
	
	if session.FileName != "test.txt" {
		t.Errorf("文件名不匹配: 期望 %s, 实际 %s", "test.txt", session.FileName)
	}
	
	if session.FileSize != 100*1024*1024 {
		t.Errorf("文件大小不匹配: 期望 %d, 实际 %d", 100*1024*1024, session.FileSize)
	}
	
	if session.Replicas != 3 {
		t.Errorf("副本数不匹配: 期望 %d, 实际 %d", 3, session.Replicas)
	}
	
	// 验证分片数量
	expectedChunks := 7 // 100MB / 16MB = 6.25, 向上取整 = 7
	if len(session.Chunks) != expectedChunks {
		t.Errorf("分片数量不匹配: 期望 %d, 实际 %d", expectedChunks, len(session.Chunks))
	}
	
	// 验证会话状态
	if session.Status != "pending" {
		t.Errorf("会话状态不匹配: 期望 %s, 实际 %s", "pending", session.Status)
	}
}

func TestUploadManagerGetSession(t *testing.T) {
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	encMgr := NewEncryptionManager()
	uploadMgr := NewUploadManager(scheduler, storage, encMgr)
	
	// 创建上传会话
	session, _ := uploadMgr.CreateUploadSession(
		"test-file-002",
		"test2.txt",
		"/test2.txt",
		50*1024*1024,
		3,
		3,
	)
	
	// 获取上传会话
	got, err := uploadMgr.GetUploadSession(session.SessionID)
	if err != nil {
		t.Fatalf("获取上传会话失败: %v", err)
	}
	
	if got.SessionID != session.SessionID {
		t.Errorf("会话ID不匹配: 期望 %s, 实际 %s", session.SessionID, got.SessionID)
	}
	
	// 测试不存在的会话
	_, err = uploadMgr.GetUploadSession("non-existent")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestUploadManagerConfirmChunk(t *testing.T) {
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	encMgr := NewEncryptionManager()
	uploadMgr := NewUploadManager(scheduler, storage, encMgr)
	
	// 创建上传会话
	session, _ := uploadMgr.CreateUploadSession(
		"test-file-003",
		"test3.txt",
		"/test3.txt",
		10*1024*1024,
		3,
		3,
	)
	
	// 确认分片上传
	err := uploadMgr.ConfirmChunkUpload(session.SessionID, 0, "test-hash-001")
	if err != nil {
		t.Fatalf("确认分片上传失败: %v", err)
	}
	
	// 验证分片状态
	got, _ := uploadMgr.GetUploadSession(session.SessionID)
	if got.Chunks[0].Status != "completed" {
		t.Errorf("分片状态不匹配: 期望 %s, 实际 %s", "completed", got.Chunks[0].Status)
	}
	
	if got.Chunks[0].Hash != "test-hash-001" {
		t.Errorf("分片hash不匹配: 期望 %s, 实际 %s", "test-hash-001", got.Chunks[0].Hash)
	}
}
