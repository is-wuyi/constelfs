package server

import (
	"encoding/json"
	"log"
	"time"

	bolt "go.etcd.io/bbolt"
)

// PersistenceManager 持久化管理器
type PersistenceManager struct {
	db *bolt.DB
}

// NewPersistenceManager 创建持久化管理器
func NewPersistenceManager(db *bolt.DB) *PersistenceManager {
	return &PersistenceManager{db: db}
}

// SaveNode 保存节点信息
func (pm *PersistenceManager) SaveNode(node *Node) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Nodes"))
		data, err := json.Marshal(node)
		if err != nil {
			return err
		}
		return b.Put([]byte(node.NodeID), data)
	})
}

// LoadNodes 加载所有节点
func (pm *PersistenceManager) LoadNodes() (map[string]*Node, error) {
	nodes := make(map[string]*Node)
	
	err := pm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Nodes"))
		return b.ForEach(func(k, v []byte) error {
			var node Node
			if err := json.Unmarshal(v, &node); err != nil {
				log.Printf("加载节点 %s 失败: %v", string(k), err)
				return nil // 跳过损坏的数据
			}
			// 服务器重启后，所有节点标记为离线
			// 等待心跳恢复在线状态
			if node.Status == NodeStatusOnline {
				node.Status = NodeStatusOffline
			}
			nodes[node.NodeID] = &node
			return nil
		})
	})
	
	return nodes, err
}

// DeleteNode 删除节点
func (pm *PersistenceManager) DeleteNode(nodeID string) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Nodes"))
		return b.Delete([]byte(nodeID))
	})
}

// SaveFile 保存文件信息
func (pm *PersistenceManager) SaveFile(file *FileInfo) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		data, err := json.Marshal(file)
		if err != nil {
			return err
		}
		return b.Put([]byte(file.FileID), data)
	})
}

// LoadFiles 加载所有文件
func (pm *PersistenceManager) LoadFiles() (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)
	
	err := pm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		return b.ForEach(func(k, v []byte) error {
			var file FileInfo
			if err := json.Unmarshal(v, &file); err != nil {
				log.Printf("加载文件 %s 失败: %v", string(k), err)
				return nil
			}
			files[file.FileID] = &file
			return nil
		})
	})
	
	return files, err
}

// DeleteFile 删除文件
func (pm *PersistenceManager) DeleteFile(fileID string) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		return b.Delete([]byte(fileID))
	})
}

// SaveVersion 保存版本信息
func (pm *PersistenceManager) SaveVersion(fileID string, version *FileVersion) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Replicas"))
		// 使用 fileID_version 作为key
		key := fileID + "_" + version.VersionID
		data, err := json.Marshal(version)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// LoadVersions 加载所有版本（按文件ID分组）
func (pm *PersistenceManager) LoadVersions() (map[string][]*FileVersion, error) {
	versions := make(map[string][]*FileVersion)
	
	err := pm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Replicas"))
		return b.ForEach(func(k, v []byte) error {
			var version FileVersion
			if err := json.Unmarshal(v, &version); err != nil {
				log.Printf("加载版本 %s 失败: %v", string(k), err)
				return nil
			}
			versions[version.FileID] = append(versions[version.FileID], &version)
			return nil
		})
	})
	
	return versions, err
}

// SaveChunk 保存分片信息
func (pm *PersistenceManager) SaveChunk(chunk *ChunkInfo) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Chunks"))
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		return b.Put([]byte(chunk.ChunkID), data)
	})
}

// LoadChunks 加载所有分片
func (pm *PersistenceManager) LoadChunks() (map[string]*ChunkInfo, error) {
	chunks := make(map[string]*ChunkInfo)
	
	err := pm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Chunks"))
		return b.ForEach(func(k, v []byte) error {
			var chunk ChunkInfo
			if err := json.Unmarshal(v, &chunk); err != nil {
				log.Printf("加载分片 %s 失败: %v", string(k), err)
				return nil
			}
			chunks[chunk.ChunkID] = &chunk
			return nil
		})
	})
	
	return chunks, err
}

// DeleteChunk 删除分片
func (pm *PersistenceManager) DeleteChunk(chunkID string) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Chunks"))
		return b.Delete([]byte(chunkID))
	})
}

// DeleteChunksByFileID 删除文件的所有分片
func (pm *PersistenceManager) DeleteChunksByFileID(fileID string) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Chunks"))
		var toDelete [][]byte
		
		b.ForEach(func(k, v []byte) error {
			var chunk ChunkInfo
			if err := json.Unmarshal(v, &chunk); err == nil {
				if chunk.FileID == fileID {
					toDelete = append(toDelete, k)
				}
			}
			return nil
		})
		
		for _, key := range toDelete {
			if err := b.Delete(key); err != nil {
				log.Printf("删除分片 %s 失败: %v", string(key), err)
			}
		}
		
		return nil
	})
}

// SaveSpeedTestResult 保存测速结果
func (pm *PersistenceManager) SaveSpeedTestResult(nodeID string, result *SpeedTestResult) error {
	return pm.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("SpeedTests"))
		if err != nil {
			return err
		}
		key := nodeID + "_" + result.TestTime.Format(time.RFC3339)
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// GetLatestSpeedTest 获取最新测速结果
func (pm *PersistenceManager) GetLatestSpeedTest(nodeID string) (*SpeedTestResult, error) {
	var latest *SpeedTestResult
	
	err := pm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("SpeedTests"))
		if b == nil {
			return nil
		}
		
		prefix := []byte(nodeID + "_")
		c := b.Cursor()
		
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var result SpeedTestResult
			if err := json.Unmarshal(v, &result); err == nil {
				if latest == nil || result.TestTime.After(latest.TestTime) {
					latest = &result
				}
			}
		}
		
		return nil
	})
	
	return latest, err
}
