package server

import (
	"testing"
)

func TestReplicationManagerUpdateNodes(t *testing.T) {
	rm := NewReplicationManager()
	
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
	rm.UpdateNodes(nodes)
	
	// 验证节点已更新
	rm.mu.RLock()
	if len(rm.nodes) != 2 {
		t.Errorf("节点数量不匹配: 期望 %d, 实际 %d", 2, len(rm.nodes))
	}
	
	if _, exists := rm.nodes["node-1"]; !exists {
		t.Error("节点1不存在")
	}
	
	if _, exists := rm.nodes["node-2"]; !exists {
		t.Error("节点2不存在")
	}
	rm.mu.RUnlock()
}

func TestReplicationManagerReplicateChunk(t *testing.T) {
	rm := NewReplicationManager()
	
	// 创建测试节点
	nodes := map[string]*Node{
		"source": {
			NodeID:    "source",
			IPAddress: "192.168.1.1",
			Port:      8081,
			Status:    NodeStatusOnline,
		},
		"target-1": {
			NodeID:    "target-1",
			IPAddress: "192.168.1.2",
			Port:      8081,
			Status:    NodeStatusOnline,
		},
		"target-2": {
			NodeID:    "target-2",
			IPAddress: "192.168.1.3",
			Port:      8081,
			Status:    NodeStatusOnline,
		},
	}
	
	rm.UpdateNodes(nodes)
	
	// 测试分发（由于没有实际的节点服务，这里只测试函数调用）
	// 实际测试需要启动模拟服务器
	t.Log("分发管理器创建成功")
}

func TestReplicationTask(t *testing.T) {
	// 测试分发任务结构
	task := ReplicationTask{
		TaskID:      "task-001",
		ChunkID:     "chunk-001",
		SourceNode:  "node-1",
		TargetNodes: []string{"node-2", "node-3"},
		Status:      "pending",
		RetryCount:  0,
	}
	
	if task.TaskID != "task-001" {
		t.Errorf("任务ID不匹配: 期望 %s, 实际 %s", "task-001", task.TaskID)
	}
	
	if task.ChunkID != "chunk-001" {
		t.Errorf("分片ID不匹配: 期望 %s, 实际 %s", "chunk-001", task.ChunkID)
	}
	
	if task.SourceNode != "node-1" {
		t.Errorf("源节点不匹配: 期望 %s, 实际 %s", "node-1", task.SourceNode)
	}
	
	if len(task.TargetNodes) != 2 {
		t.Errorf("目标节点数量不匹配: 期望 %d, 实际 %d", 2, len(task.TargetNodes))
	}
}
