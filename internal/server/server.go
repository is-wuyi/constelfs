package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
)

// Server 中心服务器
type Server struct {
	config    *Config
	db        *bolt.DB
	grpc      *grpc.Server
	scheduler *Scheduler
	storage   *StorageManager
	fileMgr   *FileManager
	mu        sync.RWMutex

	// 节点管理
	nodes map[string]*Node
}

// New 创建新的服务器
func New(config *Config) (*Server, error) {
	// 打开数据库
	db, err := bolt.Open(config.DatabasePath, 0600, &bolt.Options{
		Timeout: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 初始化数据库Bucket
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := []string{"Nodes", "Files", "Chunks", "Replicas", "Users"}
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("创建Bucket %s 失败: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 创建调度器、存储管理器、版本管理器、文件管理器
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	versionMgr := NewVersionManager(storage)
	fileMgr := NewFileManager(versionMgr, storage, scheduler)

	s := &Server{
		config:    config,
		db:        db,
		scheduler: scheduler,
		storage:   storage,
		fileMgr:   fileMgr,
		nodes:     make(map[string]*Node),
	}

	// 启动节点状态检查器
	s.StartNodeChecker()

	return s, nil
}

// Router 返回HTTP路由器
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// API路由
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/", s.handleNode)
	mux.HandleFunc("/api/v1/files", s.fileMgr.HandleFiles)
	mux.HandleFunc("/api/v1/files/", s.fileMgr.HandleFile)
	mux.HandleFunc("/api/v1/upload", s.fileMgr.HandleUpload)
	mux.HandleFunc("/api/v1/download/", s.fileMgr.HandleDownload)
	mux.HandleFunc("/api/v1/write", s.handleWrite)
	mux.HandleFunc("/api/v1/write/confirm", s.handleConfirmWrite)
	mux.HandleFunc("/api/v1/version/create", s.fileMgr.HandleCreateVersion)
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// 静态文件（Web管理界面）
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return mux
}

// StartGRPC 启动gRPC服务
func (s *Server) StartGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	s.grpc = grpc.NewServer()
	// 注册gRPC服务
	// pb.RegisterNodeServiceServer(s.grpc, s)
	// pb.RegisterStorageServiceServer(s.grpc, s)

	log.Printf("gRPC服务启动在 %s", addr)
	return s.grpc.Serve(lis)
}

// Shutdown 关闭服务器
func (s *Server) Shutdown() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
	}
	if s.db != nil {
		s.db.Close()
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

// handleWrite 处理写入请求
func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 默认3副本
	if req.Replicas == 0 {
		req.Replicas = 3
	}

	// 准备写入
	resp, err := s.storage.PrepareWrite(s, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleConfirmWrite 确认写入完成
func (s *Server) handleConfirmWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChunkID string `json:"chunk_id"`
		Hash    string `json:"hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.storage.ConfirmWrite(req.ChunkID, req.Hash); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
