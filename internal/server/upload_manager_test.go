package server

import (
	"testing"
)

func TestNewUploadManager(t *testing.T) {
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	encMgr := NewEncryptionManager()
	
	uploadMgr := NewUploadManager(scheduler, storage, encMgr)
	
	if uploadMgr == nil {
		t.Fatal("上传管理器不应为nil")
	}
	
	if uploadMgr.scheduler != scheduler {
		t.Error("调度器不匹配")
	}
	
	if uploadMgr.storage != storage {
		t.Error("存储管理器不匹配")
	}
	
	if uploadMgr.encMgr != encMgr {
		t.Error("加密管理器不匹配")
	}
}
