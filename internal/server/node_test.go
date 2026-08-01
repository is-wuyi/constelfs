package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestServer() *Server {
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	versionMgr := NewVersionManager(storage)
	fileMgr := NewFileManager(versionMgr, storage, scheduler)
	
	return &Server{
		nodes:     make(map[string]*Node),
		scheduler: scheduler,
		storage:   storage,
		fileMgr:   fileMgr,
	}
}

func TestRegisterNode(t *testing.T) {
	s := setupTestServer()

	// 测试注册新节点
	body := `{"node_id":"test-001","ip_address":"192.168.1.1","port":8081,"total_disk_space":107374182400}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.registerNode(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["success"] != true {
		t.Error("注册应返回 success=true")
	}
	if resp["node_id"] != "test-001" {
		t.Errorf("期望 node_id=test-001，实际 %s", resp["node_id"])
	}

	// 验证节点已添加
	if _, exists := s.nodes["test-001"]; !exists {
		t.Error("节点应已添加到 map")
	}
}

func TestRegisterDuplicateNode(t *testing.T) {
	s := setupTestServer()

	// 先注册一个节点
	s.nodes["test-001"] = &Node{
		NodeID:    "test-001",
		IPAddress: "192.168.1.1",
		Status:    NodeStatusRegistered,
	}

	// 再次注册相同节点（应更新）
	body := `{"node_id":"test-001","ip_address":"192.168.1.2","port":8082,"total_disk_space":214748364800}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.registerNode(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证IP已更新
	if s.nodes["test-001"].IPAddress != "192.168.1.2" {
		t.Errorf("期望 IP=192.168.1.2，实际 %s", s.nodes["test-001"].IPAddress)
	}
}

func TestConfigureNode(t *testing.T) {
	s := setupTestServer()

	// 先注册节点
	s.nodes["test-001"] = &Node{
		NodeID:         "test-001",
		TotalDiskSpace: 107374182400,
		Status:         NodeStatusRegistered,
	}

	// 配置节点
	body := `{"storage_path":"/data/constelfs","allocated_space":53687091200}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/test-001/configure", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.configureNode(w, req, "test-001")

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证配置已更新
	node := s.nodes["test-001"]
	if node.Status != NodeStatusConfigured {
		t.Errorf("期望状态=configured，实际 %s", node.Status)
	}
	if node.StoragePath != "/data/constelfs" {
		t.Errorf("期望路径=/data/constelfs，实际 %s", node.StoragePath)
	}
	if node.AllocatedSpace != 53687091200 {
		t.Errorf("期望分配空间=53687091200，实际 %d", node.AllocatedSpace)
	}
}

func TestConfigureNodeExceedSpace(t *testing.T) {
	s := setupTestServer()

	// 先注册节点
	s.nodes["test-001"] = &Node{
		NodeID:         "test-001",
		TotalDiskSpace: 107374182400, // 100GB
		Status:         NodeStatusRegistered,
	}

	// 尝试分配超过总空间（应失败）
	body := `{"storage_path":"/data","allocated_space":214748364800}` // 200GB
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/test-001/configure", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.configureNode(w, req, "test-001")

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际 %d", w.Code)
	}
}

func TestHeartbeatNode(t *testing.T) {
	s := setupTestServer()

	// 先注册并配置节点
	s.nodes["test-001"] = &Node{
		NodeID:         "test-001",
		Status:         NodeStatusConfigured,
		LastHeartbeat:  time.Now().Add(-1 * time.Minute),
	}

	// 发送心跳
	body := `{"cpu_usage":15.5,"memory_usage":45.2,"disk_usage":30.1,"used_space":10737418240,"status":"online"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes/test-001/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.heartbeatNode(w, req, "test-001")

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证状态变为online
	node := s.nodes["test-001"]
	if node.Status != NodeStatusOnline {
		t.Errorf("期望状态=online，实际 %s", node.Status)
	}
	if node.CPUUsage != 15.5 {
		t.Errorf("期望CPU=15.5，实际 %f", node.CPUUsage)
	}
}

func TestDeleteOnlineNode(t *testing.T) {
	s := setupTestServer()

	// 创建在线节点
	s.nodes["test-001"] = &Node{
		NodeID: "test-001",
		Status: NodeStatusOnline,
	}

	// 尝试删除在线节点（应失败）
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/test-001", nil)
	w := httptest.NewRecorder()

	s.deleteNode(w, req, "test-001")

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际 %d", w.Code)
	}

	// 验证节点仍存在
	if _, exists := s.nodes["test-001"]; !exists {
		t.Error("在线节点不应被删除")
	}
}

func TestDeleteOfflineNode(t *testing.T) {
	s := setupTestServer()

	// 创建离线节点
	s.nodes["test-001"] = &Node{
		NodeID: "test-001",
		Status: NodeStatusOffline,
	}

	// 删除离线节点（应成功）
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/test-001", nil)
	w := httptest.NewRecorder()

	s.deleteNode(w, req, "test-001")

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证节点已删除
	if _, exists := s.nodes["test-001"]; exists {
		t.Error("离线节点应被删除")
	}
}

func TestDeleteNonexistentNode(t *testing.T) {
	s := setupTestServer()

	// 删除不存在的节点（应失败）
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/nonexistent", nil)
	w := httptest.NewRecorder()

	s.deleteNode(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404，实际 %d", w.Code)
	}
}

func TestCheckNodeStatusTimeout(t *testing.T) {
	s := setupTestServer()

	// 创建一个心跳超时的节点
	s.nodes["test-timeout"] = &Node{
		NodeID:        "test-timeout",
		Status:        NodeStatusOnline,
		LastHeartbeat: time.Now().Add(-2 * time.Minute), // 2分钟前，超过90秒超时
	}

	// 检查状态
	s.checkNodeStatus()

	// 验证状态变为offline
	if s.nodes["test-timeout"].Status != NodeStatusOffline {
		t.Errorf("期望状态=offline，实际 %s", s.nodes["test-timeout"].Status)
	}
}

func TestCheckNodeStatusNotTimeout(t *testing.T) {
	s := setupTestServer()

	// 创建一个心跳未超时的节点
	s.nodes["test-active"] = &Node{
		NodeID:        "test-active",
		Status:        NodeStatusOnline,
		LastHeartbeat: time.Now().Add(-30 * time.Second), // 30秒前，未超时
	}

	// 检查状态
	s.checkNodeStatus()

	// 验证状态仍为online
	if s.nodes["test-active"].Status != NodeStatusOnline {
		t.Errorf("期望状态=online，实际 %s", s.nodes["test-active"].Status)
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "ok" {
		t.Errorf("期望 status=ok，实际 %s", resp["status"])
	}
}
