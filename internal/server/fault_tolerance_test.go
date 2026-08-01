package server

import (
	"fmt"
	"testing"
	"time"
)

func TestFaultToleranceManagerWriteWithRetry(t *testing.T) {
	ftm := NewFaultToleranceManager(3, 10*time.Millisecond)
	
	// 测试成功写入
	callCount := 0
	err := ftm.WriteWithRetry(func() error {
		callCount++
		return nil
	}, "test-write")
	
	if err != nil {
		t.Fatalf("写入应该成功: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("应该只调用一次，实际调用 %d 次", callCount)
	}
	
	// 测试失败后重试成功
	callCount = 0
	err = ftm.WriteWithRetry(func() error {
		callCount++
		if callCount < 3 {
			return fmt.Errorf("模拟失败")
		}
		return nil
	}, "test-retry")
	
	if err != nil {
		t.Fatalf("重试后应该成功: %v", err)
	}
	
	if callCount != 3 {
		t.Errorf("应该调用3次，实际调用 %d 次", callCount)
	}
	
	// 测试全部失败
	callCount = 0
	err = ftm.WriteWithRetry(func() error {
		callCount++
		return fmt.Errorf("模拟失败")
	}, "test-fail")
	
	if err == nil {
		t.Error("应该返回错误")
	}
	
	if callCount != 4 { // 1次初始 + 3次重试
		t.Errorf("应该调用4次，实际调用 %d 次", callCount)
	}
}

func TestFaultToleranceManagerWriteToMultipleNodes(t *testing.T) {
	ftm := NewFaultToleranceManager(2, 10*time.Millisecond)
	
	// 测试数据
	chunkID := "test-chunk"
	data := []byte("test data")
	nodes := []string{"node-1", "node-2", "node-3"}
	
	// 模拟写入函数
	writeFunc := func(nodeID string, data []byte) error {
		if nodeID == "node-2" {
			return fmt.Errorf("node-2失败")
		}
		return nil
	}
	
	// 写入到多个节点
	results := ftm.WriteToMultipleNodes(chunkID, data, nodes, writeFunc)
	
	// 验证结果
	if len(results) != 3 {
		t.Fatalf("结果数量不匹配: 期望 %d, 实际 %d", 3, len(results))
	}
	
	// 验证node-1成功
	if !results[0].Success {
		t.Error("node-1应该成功")
	}
	
	// 验证node-2失败
	if results[1].Success {
		t.Error("node-2应该失败")
	}
	
	// 验证node-3成功
	if !results[2].Success {
		t.Error("node-3应该成功")
	}
}

func TestFaultToleranceManagerCheckQuorum(t *testing.T) {
	ftm := NewFaultToleranceManager(3, 10*time.Millisecond)
	
	// 测试全部成功
	results := []WriteResult{
		{Success: true},
		{Success: true},
		{Success: true},
	}
	
	if !ftm.CheckQuorum(results, 2) {
		t.Error("应该满足Quorum")
	}
	
	// 测试部分成功
	results = []WriteResult{
		{Success: true},
		{Success: false},
		{Success: true},
	}
	
	if !ftm.CheckQuorum(results, 2) {
		t.Error("应该满足Quorum")
	}
	
	// 测试不满足Quorum
	results = []WriteResult{
		{Success: true},
		{Success: false},
		{Success: false},
	}
	
	if ftm.CheckQuorum(results, 2) {
		t.Error("不应该满足Quorum")
	}
}

func TestFaultToleranceManagerGetFailedNodes(t *testing.T) {
	ftm := NewFaultToleranceManager(3, 10*time.Millisecond)
	
	results := []WriteResult{
		{Success: true, NodeID: "node-1"},
		{Success: false, NodeID: "node-2"},
		{Success: true, NodeID: "node-3"},
		{Success: false, NodeID: "node-4"},
	}
	
	failedNodes := ftm.GetFailedNodes(results)
	
	if len(failedNodes) != 2 {
		t.Errorf("失败节点数量不匹配: 期望 %d, 实际 %d", 2, len(failedNodes))
	}
	
	if failedNodes[0] != "node-2" {
		t.Errorf("第一个失败节点不匹配: 期望 %s, 实际 %s", "node-2", failedNodes[0])
	}
	
	if failedNodes[1] != "node-4" {
		t.Errorf("第二个失败节点不匹配: 期望 %s, 实际 %s", "node-4", failedNodes[1])
	}
}
