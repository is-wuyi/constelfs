package server

import (
	"testing"
	"time"
)

func TestScoreNode(t *testing.T) {
	scheduler := NewScheduler()

	// 测试健康节点
	node := &Node{
		CPUUsage:       20,
		MemoryUsage:    30,
		DiskUsage:      40,
		AllocatedSpace: 100 * 1024 * 1024 * 1024, // 100GB
		UsedSpace:      20 * 1024 * 1024 * 1024,   // 20GB
		OnlineRate:     1.0,                         // 在线率100%
		LastHeartbeat:  time.Now(),
	}

	score := scheduler.ScoreNode(node)
	if score <= 0 {
		t.Errorf("健康节点评分应大于0，实际 %f", score)
	}

	// 测试高负载节点
	highLoadNode := &Node{
		CPUUsage:       90,
		MemoryUsage:    85,
		DiskUsage:      80,
		AllocatedSpace: 100 * 1024 * 1024 * 1024,
		UsedSpace:      90 * 1024 * 1024 * 1024,
		OnlineRate:     1.0,
		LastHeartbeat:  time.Now(),
	}

	highLoadScore := scheduler.ScoreNode(highLoadNode)
	if highLoadScore >= score {
		t.Errorf("高负载节点评分应低于健康节点: 高负载=%f, 健康=%f", highLoadScore, score)
	}

	// 测试心跳超时节点
	timeoutNode := &Node{
		CPUUsage:       20,
		MemoryUsage:    30,
		DiskUsage:      40,
		AllocatedSpace: 100 * 1024 * 1024 * 1024,
		UsedSpace:      20 * 1024 * 1024 * 1024,
		OnlineRate:     0.0, // 在线率0%
		LastHeartbeat:  time.Now().Add(-2 * time.Minute),
	}

	timeoutScore := scheduler.ScoreNode(timeoutNode)
	if timeoutScore >= score {
		t.Errorf("超时节点评分应低于健康节点: 超时=%f, 健康=%f", timeoutScore, score)
	}
}

func TestSelectNodes(t *testing.T) {
	scheduler := NewScheduler()

	// 创建测试节点
	nodes := map[string]*Node{
		"node-1": {
			NodeID:         "node-1",
			Status:         NodeStatusOnline,
			StoragePath:    "/data",
			CPUUsage:       20,
			MemoryUsage:    30,
			AllocatedSpace: 100 * 1024 * 1024 * 1024,
			UsedSpace:      20 * 1024 * 1024 * 1024,
			OnlineRate:     1.0,
			LastHeartbeat:  time.Now(),
		},
		"node-2": {
			NodeID:         "node-2",
			Status:         NodeStatusOnline,
			StoragePath:    "/data",
			CPUUsage:       50,
			MemoryUsage:    60,
			AllocatedSpace: 100 * 1024 * 1024 * 1024,
			UsedSpace:      50 * 1024 * 1024 * 1024,
			OnlineRate:     1.0,
			LastHeartbeat:  time.Now(),
		},
		"node-3": {
			NodeID:         "node-3",
			Status:         NodeStatusOffline,
			StoragePath:    "/data",
			CPUUsage:       10,
			MemoryUsage:    20,
			AllocatedSpace: 100 * 1024 * 1024 * 1024,
			UsedSpace:      10 * 1024 * 1024 * 1024,
			OnlineRate:     1.0,
			LastHeartbeat:  time.Now(),
		},
	}

	// 测试选择2个节点
	selected := scheduler.SelectNodes(nodes, 2, nil)
	if len(selected) != 2 {
		t.Errorf("应选择2个节点，实际 %d", len(selected))
	}

	// 验证选择的是在线节点
	for _, node := range selected {
		if node.Status != NodeStatusOnline {
			t.Errorf("应选择在线节点，实际 %s", node.Status)
		}
	}

	// 测试排除节点
	selected = scheduler.SelectNodes(nodes, 2, []string{"node-1"})
	if len(selected) != 1 {
		t.Errorf("排除node-1后应选择1个节点，实际 %d", len(selected))
	}
	if selected[0].NodeID != "node-2" {
		t.Errorf("应选择node-2，实际 %s", selected[0].NodeID)
	}

	// 测试可用节点不足
	nodes["node-4"] = &Node{
		NodeID:         "node-4",
		Status:         NodeStatusRegistered, // 未配置
		StoragePath:    "",
		LastHeartbeat:  time.Now(),
	}

	selected = scheduler.SelectNodes(nodes, 3, nil)
	if len(selected) != 2 {
		t.Errorf("可用节点不足时应返回所有可用节点，实际 %d", len(selected))
	}
}

func TestGetNodeHealth(t *testing.T) {
	scheduler := NewScheduler()

	// 测试健康节点
	healthyNode := &Node{
		CPUUsage:     20,
		MemoryUsage:  30,
		DiskUsage:    40,
		LastHeartbeat: time.Now(),
	}
	if scheduler.GetNodeHealth(healthyNode) != "healthy" {
		t.Errorf("应返回healthy，实际 %s", scheduler.GetNodeHealth(healthyNode))
	}

	// 测试离线节点
	offlineNode := &Node{
		CPUUsage:     20,
		MemoryUsage:  30,
		DiskUsage:    40,
		LastHeartbeat: time.Now().Add(-2 * time.Minute),
	}
	if scheduler.GetNodeHealth(offlineNode) != "offline" {
		t.Errorf("应返回offline，实际 %s", scheduler.GetNodeHealth(offlineNode))
	}

	// 测试高负载节点
	warningNode := &Node{
		CPUUsage:     95,
		MemoryUsage:  30,
		DiskUsage:    40,
		LastHeartbeat: time.Now(),
	}
	if scheduler.GetNodeHealth(warningNode) != "warning" {
		t.Errorf("应返回warning，实际 %s", scheduler.GetNodeHealth(warningNode))
	}
}
