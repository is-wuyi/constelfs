package server

import (
	"math"
	"sort"
	"sync"
	"time"
)

// NodeScore 节点评分
type NodeScore struct {
	Node  *Node
	Score float64
}

// Scheduler 节点调度器
type Scheduler struct {
	mu sync.RWMutex
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// SelectNodes 智能选择N个节点
func (s *Scheduler) SelectNodes(nodes map[string]*Node, n int, excludeNodes []string) []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 获取所有在线且已配置的节点
	var availableNodes []*Node
	excludeMap := make(map[string]bool)
	for _, id := range excludeNodes {
		excludeMap[id] = true
	}

	for _, node := range nodes {
		if node.Status == NodeStatusOnline && 
		   node.StoragePath != "" && 
		   !excludeMap[node.NodeID] {
			availableNodes = append(availableNodes, node)
		}
	}

	// 如果可用节点不足，返回所有可用节点
	if len(availableNodes) < n {
		return availableNodes
	}

	// 计算每个节点的评分
	var scores []NodeScore
	for _, node := range availableNodes {
		score := s.ScoreNode(node)
		scores = append(scores, NodeScore{Node: node, Score: score})
	}

	// 按评分排序（从高到低）
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// 选择评分最高的N个节点
	var selected []*Node
	for i := 0; i < n && i < len(scores); i++ {
		selected = append(selected, scores[i].Node)
	}

	return selected
}

// ScoreNode 计算节点评分
func (s *Scheduler) ScoreNode(node *Node) float64 {
	score := 0.0

	// 1. CPU使用率（权重40%）- 使用率越低越好
	cpuScore := (100 - node.CPUUsage) * 0.4
	score += cpuScore

	// 2. 可用空间（权重30%）- 空间越大越好
	if node.AllocatedSpace > 0 {
		availableSpace := node.AllocatedSpace - node.UsedSpace
		spaceRatio := float64(availableSpace) / float64(node.AllocatedSpace)
		spaceScore := spaceRatio * 30
		score += spaceScore
	}

	// 3. 内存使用率（权重20%）- 使用率越低越好
	memoryScore := (100 - node.MemoryUsage) * 0.2
	score += memoryScore

	// 4. 响应时间（权重10%）- 基于最后心跳时间
	// 心跳越近，说明节点越健康
	timeSinceHeartbeat := time.Since(node.LastHeartbeat).Seconds()
	if timeSinceHeartbeat < 30 {
		latencyScore := (30 - timeSinceHeartbeat) / 30 * 10
		score += latencyScore
	}

	return math.Round(score*100) / 100
}

// SelectNewNodes 选择新的节点（排除指定节点）
func (s *Scheduler) SelectNewNodes(nodes map[string]*Node, n int, excludeNodes []string) []*Node {
	return s.SelectNodes(nodes, n, excludeNodes)
}

// GetNodeHealth 获取节点健康度
func (s *Scheduler) GetNodeHealth(node *Node) string {
	// 检查心跳时间
	timeSinceHeartbeat := time.Since(node.LastHeartbeat).Seconds()
	if timeSinceHeartbeat > 90 {
		return "offline"
	}

	// 检查CPU使用率
	if node.CPUUsage > 90 {
		return "warning"
	}

	// 检查内存使用率
	if node.MemoryUsage > 90 {
		return "warning"
	}

	// 检查磁盘使用率
	if node.DiskUsage > 90 {
		return "warning"
	}

	return "healthy"
}
