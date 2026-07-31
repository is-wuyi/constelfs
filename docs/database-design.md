# 数据库设计补充说明

## 1. 节点空间管理设计

### 1.1 三个空间字段的含义

```go
type Node struct {
    TotalDiskSpace int64 `json:"total_disk_space"`   // 节点总磁盘空间
    AllocatedSpace int64 `json:"allocated_space"`     // 预分配上限
    UsedSpace      int64 `json:"used_space"`          // 集群实际已占用
}
```

| 字段 | 含义 | 设置方式 | 用途 |
|------|------|----------|------|
| `total_disk_space` | 节点总磁盘空间 | 自动检测 | 计算磁盘使用率 |
| `allocated_space` | 预分配上限 | 管理员设置 | 限制集群最大使用量 |
| `used_space` | 集群实际占用 | 系统自动统计 | 监控当前使用情况 |

### 1.2 使用率计算

```go
// 计算磁盘使用率（硬件层面）
diskUsageRate := float64(node.UsedSpace) / float64(node.TotalDiskSpace)

// 计算集群配额使用率
clusterUsageRate := float64(node.UsedSpace) / float64(node.AllocatedSpace)

// 检查是否还有可用空间
canWrite := node.UsedSpace < node.AllocatedSpace
```

### 1.3 Web界面展示

```
┌─────────────────────────────────────────────────────────┐
│ 节点: NAS-001 (192.168.1.100)                           │
├─────────────────────────────────────────────────────────┤
│ 磁盘空间: ████████░░░░ 800GB / 1TB (80%)                │
│ 集群配额: ██████░░░░░░ 600GB / 800GB (75%)              │
│ 剩余可用: 200GB                                         │
└─────────────────────────────────────────────────────────┘
```

### 1.4 典型使用场景

**场景1：限制集群使用量**
```
管理员设置：allocated_space = 800GB
集群实际使用：used_space = 600GB
结果：还可以写入 200GB 数据
```

**场景2：防止磁盘写满**
```
节点总磁盘：total_disk_space = 1TB
集群已使用：used_space = 900GB
磁盘使用率：90%（需要告警）
```

## 2. 文件路径设计优化

### 2.1 路径拆分设计

```go
type File struct {
    DirPath  string `json:"dir_path"`   // 目录路径，如 "/documents"
    FileName string `json:"file_name"`  // 文件名，如 "report.pdf"
    FilePath string `json:"file_path"`  // 完整路径（自动生成）
}
```

### 2.2 为什么这样设计？

**问题**：如果只用 `file_path` 存储完整路径
- 查询某个目录下的文件：需要字符串匹配，性能差
- 防止重名：需要复杂的唯一性检查

**解决方案**：拆分为 `dir_path` + `file_name`
- 查询某个目录：`WHERE dir_path = '/documents'`
- 防止重名：`UNIQUE(user_id, dir_path, file_name)`

### 2.3 查询示例

```go
// 查询某个目录下的所有文件
func ListFiles(db *bolt.DB, userID int64, dirPath string) ([]*File, error) {
    var files []*File
    err := db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        return b.ForEach(func(k, v []byte) error {
            var file File
            if err := json.Unmarshal(v, &file); err != nil {
                return err
            }
            // 匹配用户ID和目录路径
            if file.UserID == userID && file.DirPath == dirPath {
                files = append(files, &file)
            }
            return nil
        })
    })
    return files, err
}

// 递归查询某个目录下的所有文件（包括子目录）
func ListFilesRecursive(db *bolt.DB, userID int64, dirPath string) ([]*File, error) {
    var files []*File
    err := db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        return b.ForEach(func(k, v []byte) error {
            var file File
            if err := json.Unmarshal(v, &file); err != nil {
                return err
            }
            // 匹配用户ID，且路径以dirPath开头
            if file.UserID == userID && strings.HasPrefix(file.DirPath, dirPath) {
                files = append(files, &file)
            }
            return nil
        })
    })
    return files, err
}
```

### 2.4 索引设计

```go
// 创建索引桶，加速查询
// 索引1：按用户ID查询
// Key: user_id (int64的字节表示)
// Value: file_path列表的JSON编码

// 索引2：按目录路径查询
// Key: "user_id:dir_path" (string)
// Value: file_name列表的JSON编码
```

## 3. 分片Hash存储设计

### 3.1 Hash存储的优势

```go
type Chunk struct {
    ChunkHash string `json:"chunk_hash"`  // SHA-256 Hash
}
```

**优势**：
1. **天然防冲突**：相同内容的分片 Hash 相同，不会冲突
2. **支持去重**：相同内容只存储一份，节省空间
3. **完整性校验**：Hash 可用于验证数据完整性
4. **便于迁移**：Hash 作为文件名，迁移时不会冲突

### 3.2 存储路径设计

```
/storage/node-001/
├── chunks/
│   ├── a1/
│   │   ├── b2/
│   │   │   ├── a1b2c3d4e5f6...  # 分片文件
│   │   │   └── ...
│   │   └── ...
│   └── ...
└── meta/
    └── chunk_index.db
```

**路径生成规则**：
```go
// 使用Hash前两位作为目录，防止单目录文件过多
func ChunkPath(hash string) string {
    return fmt.Sprintf("/chunks/%s/%s/%s", 
        hash[:2], hash[2:4], hash)
}

// 示例
// Hash: a1b2c3d4e5f6...
// 路径: /chunks/a1/b2/a1b2c3d4e5f6...
```

### 3.3 去重实现

```go
// 存储分片时，检查是否已存在
func StoreChunk(db *bolt.DB, chunkData []byte) (string, error) {
    // 计算Hash
    hash := sha256.Sum256(chunkData)
    hashStr := hex.EncodeToString(hash[:])
    
    // 检查是否已存在
    exists, err := ChunkExists(db, hashStr)
    if err != nil {
        return "", err
    }
    
    if !exists {
        // 存储分片文件
        chunkPath := ChunkPath(hashStr)
        if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
            return "", err
        }
        
        // 存储元数据
        if err := StoreChunkMetadata(db, hashStr, chunkData); err != nil {
            return "", err
        }
    }
    
    return hashStr, nil
}
```

### 3.4 完整性验证

```go
// 验证分片完整性
func VerifyChunk(chunkPath string, expectedHash string) (bool, error) {
    data, err := os.ReadFile(chunkPath)
    if err != nil {
        return false, err
    }
    
    hash := sha256.Sum256(data)
    actualHash := hex.EncodeToString(hash[:])
    
    return actualHash == expectedHash, nil
}
```

## 4. 完整的数据库Schema

```go
// 用户桶
// Key: username (string)
type User struct {
    ID           int64     `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"`
    QuotaBytes   int64     `json:"quota_bytes"`
    UsedBytes    int64     `json:"used_bytes"`
    CreatedAt    time.Time `json:"created_at"`
}

// 节点桶
// Key: node_id (string)
type Node struct {
    ID             int64     `json:"id"`
    NodeID         string    `json:"node_id"`
    IPAddress      string    `json:"ip_address"`
    Port           int       `json:"port"`
    Status         string    `json:"status"`
    CPUUsage       float64   `json:"cpu_usage"`
    MemoryUsage    float64   `json:"memory_usage"`
    DiskUsage      float64   `json:"disk_usage"`
    TotalDiskSpace int64     `json:"total_disk_space"`
    AllocatedSpace int64     `json:"allocated_space"`
    UsedSpace      int64     `json:"used_space"`
    LastHeartbeat  time.Time `json:"last_heartbeat"`
    CreatedAt      time.Time `json:"created_at"`
}

// 文件桶
// Key: "user_id:dir_path:file_name" (string)
type File struct {
    ID                int64     `json:"id"`
    UserID            int64     `json:"user_id"`
    DirPath           string    `json:"dir_path"`
    FileName          string    `json:"file_name"`
    FilePath          string    `json:"file_path"`
    FileSize          int64     `json:"file_size"`
    IsDirectory       bool      `json:"is_directory"`
    ReplicationFactor int       `json:"replication_factor"`
    ErasureCoded      bool      `json:"erasure_coded"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

// 分片桶
// Key: chunk_hash (string)
type Chunk struct {
    ID         int64     `json:"id"`
    FileID     int64     `json:"file_id"`
    ChunkIndex int       `json:"chunk_index"`
    ChunkSize  int64     `json:"chunk_size"`
    ChunkHash  string    `json:"chunk_hash"`
    Checksum   string    `json:"checksum"`
    CreatedAt  time.Time `json:"created_at"`
}

// 副本桶
// Key: "chunk_hash:node_id" (string)
type Replica struct {
    ID        int64     `json:"id"`
    ChunkHash string    `json:"chunk_hash"`
    NodeID    string    `json:"node_id"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}
```

---

**文档版本**: v1.0  
**创建日期**: 2026-07-31
