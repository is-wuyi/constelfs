package webdav

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebDAVServer WebDAV服务器
type WebDAVServer struct {
	serverAddr string
	listenAddr string
	files      map[string]*FileInfo
}

type FileInfo struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
	Content []byte
}

// NewWebDAVServer 创建新的WebDAV服务器
func NewWebDAVServer(serverAddr, listenAddr string) *WebDAVServer {
	return &WebDAVServer{
		serverAddr: serverAddr,
		listenAddr: listenAddr,
		files:      make(map[string]*FileInfo),
	}
}

// Start 启动WebDAV服务器
func (s *WebDAVServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	log.Printf("WebDAV服务器启动在 %s", s.listenAddr)
	return http.ListenAndServe(s.listenAddr, mux)
}

// handleRequest 处理WebDAV请求
func (s *WebDAVServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "."
	}

	switch r.Method {
	case "GET":
		s.handleGet(w, r, path)
	case "PUT":
		s.handlePut(w, r, path)
	case "DELETE":
		s.handleDelete(w, r, path)
	case "MKCOL":
		s.handleMkcol(w, r, path)
	case "PROPFIND":
		s.handlePropfind(w, r, path)
	case "OPTIONS":
		s.handleOptions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet 处理GET请求
func (s *WebDAVServer) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	file, exists := s.files[path]
	if !exists {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if file.IsDir {
		http.Error(w, "Is a directory", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(file.Content)))
	w.Write(file.Content)
}

// handlePut 处理PUT请求
func (s *WebDAVServer) handlePut(w http.ResponseWriter, r *http.Request, path string) {
	// 读取请求体
	content := make([]byte, r.ContentLength)
	r.Body.Read(content)

	// 创建或更新文件
	s.files[path] = &FileInfo{
		Name:    path,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
		IsDir:   false,
		Content: content,
	}

	w.WriteHeader(http.StatusCreated)
}

// handleDelete 处理DELETE请求
func (s *WebDAVServer) handleDelete(w http.ResponseWriter, r *http.Request, path string) {
	_, exists := s.files[path]
	if !exists {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	delete(s.files, path)
	w.WriteHeader(http.StatusNoContent)
}

// handleMkcol 处理MKCOL请求
func (s *WebDAVServer) handleMkcol(w http.ResponseWriter, r *http.Request, path string) {
	_, exists := s.files[path]
	if exists {
		http.Error(w, "Already exists", http.StatusMethodNotAllowed)
		return
	}

	s.files[path] = &FileInfo{
		Name:    path,
		Size:    0,
		Mode:    os.ModeDir | 0755,
		ModTime: time.Now(),
		IsDir:   true,
	}

	w.WriteHeader(http.StatusCreated)
}

// handlePropfind 处理PROPFIND请求
func (s *WebDAVServer) handlePropfind(w http.ResponseWriter, r *http.Request, path string) {
	file, exists := s.files[path]
	if !exists {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)

	// 生成PROPFIND响应
	response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/` + path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>` + getResourceType(file.IsDir) + `</D:resourcetype>
        <D:getcontentlength>` + fmt.Sprintf("%d", file.Size) + `</D:getcontentlength>
        <D:getlastmodified>` + file.ModTime.Format(time.RFC1123) + `</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

	w.Write([]byte(response))
}

// handleOptions 处理OPTIONS请求
func (s *WebDAVServer) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, PUT, DELETE, MKCOL, PROPFIND, OPTIONS")
	w.Header().Set("DAV", "1, 2")
	w.WriteHeader(http.StatusOK)
}

func getResourceType(isDir bool) string {
	if isDir {
		return "<D:collection/>"
	}
	return ""
}
