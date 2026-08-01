package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// HandleEncryption 处理加密相关API
func (s *Server) HandleEncryption(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/encryption/")
	parts := strings.Split(path, "/")

	switch {
	// POST /api/v1/encryption/key - 保存加密密钥
	case len(parts) == 1 && parts[0] == "key" && r.Method == http.MethodPost:
		s.saveEncryptionKey(w, r)
	// GET /api/v1/encryption/key/:fileID - 获取加密密钥
	case len(parts) == 2 && parts[0] == "key" && r.Method == http.MethodGet:
		s.getEncryptionKey(w, r, parts[1])
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (s *Server) saveEncryptionKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FileID string `json:"file_id"`
		Key    string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if s.persist != nil {
		err := s.db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("EncryptionKeys"))
			if err != nil {
				return err
			}
			return b.Put([]byte(req.FileID), []byte(req.Key))
		})
		if err != nil {
			http.Error(w, "保存密钥失败", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("保存加密密钥: %s", req.FileID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) getEncryptionKey(w http.ResponseWriter, r *http.Request, fileID string) {
	if s.persist == nil {
		http.Error(w, "持久化未启用", http.StatusInternalServerError)
		return
	}

	var key string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("EncryptionKeys"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(fileID))
		if v != nil {
			key = string(v)
		}
		return nil
	})
	if err != nil {
		http.Error(w, "获取密钥失败", http.StatusInternalServerError)
		return
	}

	if key == "" {
		http.Error(w, "密钥不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key": key,
	})
}
