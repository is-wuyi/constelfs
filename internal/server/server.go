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
	persist   *PersistenceManager
	grpc      *grpc.Server
	scheduler *Scheduler
	storage   *StorageManager
	fileMgr   *FileManager
	mu        sync.RWMutex

	nodes map[string]*Node
}

// New 创建新的服务器
func New(config *Config) (*Server, error) {
	db, err := bolt.Open(config.DatabasePath, 0600, &bolt.Options{
		Timeout: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		buckets := []string{"Nodes", "Files", "Chunks", "Replicas", "Users", "SpeedTests"}
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

	persist := NewPersistenceManager(db)
	scheduler := NewScheduler()
	storage := NewStorageManager(scheduler)
	versionMgr := NewVersionManager(storage)
	fileMgr := NewFileManager(versionMgr, storage, scheduler)

	// 设置持久化管理器
	storage.SetPersistence(persist)
	fileMgr.SetPersistence(persist)

	s := &Server{
		config:    config,
		db:        db,
		persist:   persist,
		scheduler: scheduler,
		storage:   storage,
		fileMgr:   fileMgr,
		nodes:     make(map[string]*Node),
	}

	if err := s.loadData(); err != nil {
		log.Printf("加载历史数据失败: %v", err)
	}

	s.StartNodeChecker()

	return s, nil
}

// loadData 从数据库加载数据
func (s *Server) loadData() error {
	nodes, err := s.persist.LoadNodes()
	if err != nil {
		return fmt.Errorf("加载节点失败: %w", err)
	}
	s.nodes = nodes
	log.Printf("加载了 %d 个节点", len(nodes))

	files, err := s.persist.LoadFiles()
	if err != nil {
		return fmt.Errorf("加载文件失败: %w", err)
	}
	s.fileMgr.files = files
	log.Printf("加载了 %d 个文件", len(files))

	versions, err := s.persist.LoadVersions()
	if err != nil {
		return fmt.Errorf("加载版本失败: %w", err)
	}
	s.fileMgr.versions = versions
	loadedVersions := 0
	for _, v := range versions {
		loadedVersions += len(v)
	}
	log.Printf("加载了 %d 个版本", loadedVersions)

	chunks, err := s.persist.LoadChunks()
	if err != nil {
		return fmt.Errorf("加载分片失败: %w", err)
	}
	s.storage.chunks = chunks
	log.Printf("加载了 %d 个分片", len(chunks))

	s.syncNodes()

	return nil
}

func (s *Server) syncNodes() {
	s.fileMgr.UpdateNodes(s.nodes)
	s.storage.UpdateNodes(s.nodes)
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

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

	mux.Handle("/", http.FileServer(http.Dir("web")))

	return mux
}

func (s *Server) StartGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	s.grpc = grpc.NewServer()

	log.Printf("gRPC服务启动在 %s", addr)
	return s.grpc.Serve(lis)
}

func (s *Server) Shutdown() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
	}
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

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

	if req.Replicas == 0 {
		req.Replicas = 3
	}

	resp, err := s.storage.PrepareWrite(s, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

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
