package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusRegistered  NodeStatus = "registered"
	NodeStatusConfigured  NodeStatus = "configured"
	NodeStatusOnline      NodeStatus = "online"
	NodeStatusOffline     NodeStatus = "offline"
	NodeStatusMaintenance NodeStatus = "maintenance"
)

const HeartbeatTimeout = 90

// Node 存储节点信息
type Node struct {
	ID             int64      `json:"id"`
	NodeID         string     `json:"node_id"`
	IPAddress      string     `json:"ip_address"`
	Port           int        `json:"port"`
	Status         NodeStatus `json:"status"`
	StoragePath    string     `json:"storage_path"`
	TotalDiskSpace int64      `json:"total_disk_space"`
	AllocatedSpace int64      `json:"allocated_space"`
	UsedSpace      int64      `json:"used_space"`
	CPUUsage       float64    `json:"cpu_usage"`
	MemoryUsage    float64    `json:"memory_usage"`
	DiskUsage      float64    `json:"disk_usage"`
	UploadSpeed    float64    `json:"upload_speed"`
	DownloadSpeed  float64    `json:"download_speed"`
	OnlineRate     float64    `json:"online_rate"`
	LastHeartbeat  time.Time  `json:"last_heartbeat"`
	LastSpeedTest  time.Time  `json:"last_speed_test"`
	ConfiguredAt   time.Time  `json:"configured_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type RegisterRequest struct {
	NodeID         string  `json:"node_id"`
	IPAddress      string  `json:"ip_address"`
	Port           int     `json:"port"`
	TotalDiskSpace int64   `json:"total_disk_space"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	DiskUsage      float64 `json:"disk_usage"`
}

type ConfigureRequest struct {
	StoragePath    string `json:"storage_path"`
	AllocatedSpace int64  `json:"allocated_space"`
}

type HeartbeatRequest struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	UsedSpace   int64   `json:"used_space"`
	Status      string  `json:"status"`
}

type SpeedTestResult struct {
	UploadSpeed   float64   `json:"upload_speed"`
	DownloadSpeed float64   `json:"download_speed"`
	Latency       int64     `json:"latency"`
	TestTime      time.Time `json:"test_time"`
}

// StartNodeChecker 启动节点状态检查器
func (s *Server) StartNodeChecker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.checkNodeStatus()
		}
	}()
	log.Println("节点状态检查器已启动")
}

// checkNodeStatus 检查节点状态
func (s *Server) checkNodeStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	changed := false
	for _, node := range s.nodes {
		if node.Status == NodeStatusConfigured || node.Status == NodeStatusOnline {
			elapsed := now.Sub(node.LastHeartbeat).Seconds()
			if elapsed > HeartbeatTimeout {
				if node.Status != NodeStatusOffline {
					log.Printf("节点 %s 心跳超时 (%.0f秒)，标记为离线", node.NodeID, elapsed)
					node.Status = NodeStatusOffline
					changed = true
					if s.persist != nil {
						if err := s.persist.SaveNode(node); err != nil {
							log.Printf("持久化节点 %s 失败: %v", node.NodeID, err)
						}
					}
				}
			}
		}
	}
	if changed {
		s.syncNodes()
	}
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listNodes(w, r)
	case http.MethodPost:
		s.registerNode(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/")
	parts := strings.SplitN(path, "/", 2)
	nodeID := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.getNode(w, r, nodeID)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		s.deleteNode(w, r, nodeID)
	case len(parts) == 2 && parts[1] == "configure" && r.Method == http.MethodPost:
		s.configureNode(w, r, nodeID)
	case len(parts) == 2 && parts[1] == "heartbeat" && r.Method == http.MethodPost:
		s.heartbeatNode(w, r, nodeID)
	case len(parts) == 2 && parts[1] == "speedtest" && r.Method == http.MethodPost:
		s.speedTestNode(w, r, nodeID)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (s *Server) getNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		http.Error(w, `{"error":"Node not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		http.Error(w, `{"error":"Node not found"}`, http.StatusNotFound)
		return
	}

	if node.Status == NodeStatusOnline {
		http.Error(w, `{"error":"Cannot delete online node, please stop it first"}`, http.StatusBadRequest)
		return
	}

	delete(s.nodes, nodeID)
	
	if s.persist != nil {
		if err := s.persist.DeleteNode(nodeID); err != nil {
			log.Printf("持久化删除节点 %s 失败: %v", nodeID, err)
		}
	}
	
	s.syncNodes()
	log.Printf("节点已删除: %s", nodeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Node %s deleted", nodeID),
	})
}

func (s *Server) registerNode(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[req.NodeID]; exists {
		s.nodes[req.NodeID].IPAddress = req.IPAddress
		s.nodes[req.NodeID].Port = req.Port
		s.nodes[req.NodeID].TotalDiskSpace = req.TotalDiskSpace
		s.nodes[req.NodeID].CPUUsage = req.CPUUsage
		s.nodes[req.NodeID].MemoryUsage = req.MemoryUsage
		s.nodes[req.NodeID].DiskUsage = req.DiskUsage
		s.nodes[req.NodeID].LastHeartbeat = time.Now()
	} else {
		s.nodes[req.NodeID] = &Node{
			NodeID:         req.NodeID,
			IPAddress:      req.IPAddress,
			Port:           req.Port,
			Status:         NodeStatusRegistered,
			TotalDiskSpace: req.TotalDiskSpace,
			CPUUsage:       req.CPUUsage,
			MemoryUsage:    req.MemoryUsage,
			DiskUsage:      req.DiskUsage,
			OnlineRate:     1.0,
			LastHeartbeat:  time.Now(),
			CreatedAt:      time.Now(),
		}
	}

	if s.persist != nil {
		if err := s.persist.SaveNode(s.nodes[req.NodeID]); err != nil {
			log.Printf("持久化节点 %s 失败: %v", req.NodeID, err)
		}
	}

	s.syncNodes()
	log.Printf("节点注册: %s (%s)", req.NodeID, req.IPAddress)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"node_id": req.NodeID,
		"status":  NodeStatusRegistered,
	})
}

func (s *Server) configureNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	var req ConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	if req.AllocatedSpace > node.TotalDiskSpace {
		http.Error(w, "Allocated space exceeds total disk space", http.StatusBadRequest)
		return
	}

	node.StoragePath = req.StoragePath
	node.AllocatedSpace = req.AllocatedSpace
	node.Status = NodeStatusConfigured
	node.ConfiguredAt = time.Now()

	if s.persist != nil {
		if err := s.persist.SaveNode(node); err != nil {
			log.Printf("持久化节点 %s 失败: %v", nodeID, err)
		}
	}

	s.syncNodes()
	log.Printf("节点配置: %s, 路径: %s, 空间: %dGB", nodeID, req.StoragePath, req.AllocatedSpace/1024/1024/1024)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"node":    node,
	})
}

func (s *Server) heartbeatNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	node.CPUUsage = req.CPUUsage
	node.MemoryUsage = req.MemoryUsage
	node.DiskUsage = req.DiskUsage
	node.UsedSpace = req.UsedSpace
	node.LastHeartbeat = time.Now()

	if node.Status == NodeStatusConfigured || node.Status == NodeStatusOffline {
		node.Status = NodeStatusOnline
		log.Printf("节点 %s 上线", nodeID)
	}

	if s.persist != nil {
		if err := s.persist.SaveNode(node); err != nil {
			log.Printf("持久化节点 %s 失败: %v", nodeID, err)
		}
	}

	s.syncNodes()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"commands": []string{},
	})
}

func (s *Server) speedTestNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	var result SpeedTestResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	node.UploadSpeed = result.UploadSpeed
	node.DownloadSpeed = result.DownloadSpeed
	node.LastSpeedTest = time.Now()

	if s.persist != nil {
		if err := s.persist.SaveNode(node); err != nil {
			log.Printf("持久化节点 %s 失败: %v", nodeID, err)
		}
		if err := s.persist.SaveSpeedTestResult(nodeID, &result); err != nil {
			log.Printf("持久化测速结果 %s 失败: %v", nodeID, err)
		}
	}

	s.syncNodes()
	log.Printf("节点 %s 测速完成: 上传=%.2f Mbps, 下载=%.2f Mbps", 
		nodeID, result.UploadSpeed, result.DownloadSpeed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
