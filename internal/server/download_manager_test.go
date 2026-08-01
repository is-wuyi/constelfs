package server

import (
	"testing"
)

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

func TestDownloadChunkNodeNotFound(t *testing.T) {
	dm := NewDownloadManager()
	
	// 尝试从不存在的节点下载
	_, err := dm.DownloadChunk("test-chunk", "non-existent-node")
	if err == nil {
		t.Error("应该返回错误")
	}
}
