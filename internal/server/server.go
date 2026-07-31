package server

import (
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
	config *Config
	db     *bolt.DB
	grpc   *grpc.Server
	mu     sync.RWMutex

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

	s := &Server{
		config: config,
		db:     db,
		nodes:  make(map[string]*Node),
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
	mux.HandleFunc("/api/v1/files", s.handleFiles)
	mux.HandleFunc("/api/v1/files/", s.handleFile)
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
