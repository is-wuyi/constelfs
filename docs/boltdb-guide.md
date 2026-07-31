# BoltDB (bbolt) 使用指南

## 1. 为什么选择BoltDB

### 1.1 优势
- **Go原生**: 纯Go实现，无CGO依赖，编译简单
- **高性能**: 使用B+树存储，读写性能优秀
- **ACID事务**: 支持完整的事务语义
- **单文件**: 整个数据库存储在一个文件中，易于备份
- **嵌入式**: 无需单独部署，开箱即用
- **并发安全**: 支持多个读事务和一个写事务并发

### 1.2 适用场景
- ✅ 中小型应用（百万级数据）
- ✅ 嵌入式系统
- ✅ 配置存储
- ✅ 元数据管理
- ❌ 超大规模数据（十亿级）
- ❌ 需要复杂SQL查询

## 2. 安装与配置

### 2.1 安装
```bash
# 使用官方维护的bbolt版本
go get go.etcd.io/bbolt
```

### 2.2 基本配置
```go
package main

import (
    "log"
    "time"
    bolt "go.etcd.io/bbolt"
)

func main() {
    // 打开数据库
    db, err := bolt.Open("constelfs.db", 0600, &bolt.Options{
        Timeout: 1 * time.Second,  // 超时时间
    })
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 数据库已就绪
    log.Println("数据库已打开")
}
```

## 3. 核心操作

### 3.1 创建Bucket
```go
// 创建Bucket（类似SQL中的表）
err := db.Update(func(tx *bolt.Tx) error {
    _, err := tx.CreateBucketIfNotExists([]byte("Users"))
    return err
})
```

### 3.2 写入数据
```go
// 写入单条数据
err := db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    return b.Put([]byte("user123"), []byte("用户数据"))
})
```

### 3.3 读取数据
```go
// 读取单条数据
err := db.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    data := b.Get([]byte("user123"))
    if data == nil {
        return fmt.Errorf("用户不存在")
    }
    log.Printf("用户数据: %s", data)
    return nil
})
```

### 3.4 删除数据
```go
// 删除数据
err := db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    return b.Delete([]byte("user123"))
})
```

### 3.5 遍历数据
```go
// 遍历所有数据
err := db.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    return b.ForEach(func(k, v []byte) error {
        log.Printf("Key: %s, Value: %s", k, v)
        return nil
    })
})
```

## 4. ConstelFS中的使用示例

### 4.1 用户管理
```go
// 存储用户信息
type User struct {
    ID           int64     `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"`
    QuotaBytes   int64     `json:"quota_bytes"`
    UsedBytes    int64     `json:"used_bytes"`
    CreatedAt    time.Time `json:"created_at"`
}

// 创建用户
func CreateUser(db *bolt.DB, user *User) error {
    return db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Users"))
        
        // 序列化用户信息
        data, err := json.Marshal(user)
        if err != nil {
            return err
        }
        
        // 存储用户信息
        return b.Put([]byte(user.Username), data)
    })
}

// 获取用户
func GetUser(db *bolt.DB, username string) (*User, error) {
    var user User
    err := db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Users"))
        data := b.Get([]byte(username))
        if data == nil {
            return fmt.Errorf("用户 %s 不存在", username)
        }
        return json.Unmarshal(data, &user)
    })
    return &user, err
}
```

### 4.2 节点管理
```go
// 存储节点信息
type Node struct {
    NodeID       string    `json:"node_id"`
    IPAddress    string    `json:"ip_address"`
    Port         int       `json:"port"`
    Status       string    `json:"status"`
    CPUUsage     float64   `json:"cpu_usage"`
    MemoryUsage  float64   `json:"memory_usage"`
    DiskUsage    float64   `json:"disk_usage"`
    TotalSpace   int64     `json:"total_space"`
    UsedSpace    int64     `json:"used_space"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
}

// 更新节点心跳
func UpdateNodeHeartbeat(db *bolt.DB, nodeID string, node *Node) error {
    return db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Nodes"))
        data, err := json.Marshal(node)
        if err != nil {
            return err
        }
        return b.Put([]byte(nodeID), data)
    })
}

// 获取所有在线节点
func GetOnlineNodes(db *bolt.DB) ([]*Node, error) {
    var nodes []*Node
    err := db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Nodes"))
        return b.ForEach(func(k, v []byte) error {
            var node Node
            if err := json.Unmarshal(v, &node); err != nil {
                return err
            }
            if node.Status == "online" {
                nodes = append(nodes, &node)
            }
            return nil
        })
    })
    return nodes, err
}
```

## 5. 最佳实践

### 5.1 事务使用
```go
// ✅ 正确：使用事务
err := db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    // 多个操作在同一个事务中
    b.Put([]byte("user1"), []byte("data1"))
    b.Put([]byte("user2"), []byte("data2"))
    return nil
})

// ❌ 错误：不使用事务
b.Put([]byte("user1"), []byte("data1"))  // 这样写会报错
```

### 5.2 错误处理
```go
// 始终检查错误
err := db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    if b == nil {
        return fmt.Errorf("Bucket不存在")
    }
    return b.Put([]byte("user1"), []byte("data1"))
})
if err != nil {
    log.Printf("操作失败: %v", err)
}
```

### 5.3 性能优化
```go
// 批量操作时使用批量写入
err := db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("Users"))
    for i := 0; i < 1000; i++ {
        key := fmt.Sprintf("user_%d", i)
        value := fmt.Sprintf("data_%d", i)
        if err := b.Put([]byte(key), []byte(value)); err != nil {
            return err
        }
    }
    return nil
})
```

### 5.4 备份策略
```go
// 定期备份数据库
func BackupDB(db *bolt.DB, backupPath string) error {
    return db.View(func(tx *bolt.Tx) error {
        return tx.CopyFile(backupPath, 0600)
    })
}
```

## 6. 监控与维护

### 6.1 数据库统计
```go
// 获取数据库统计信息
func GetDBStats(db *bolt.DB) bolt.Stats {
    return db.Stats()
}

// 使用示例
stats := GetDBStats(db)
log.Printf("事务统计: %d 次打开, %d 次回滚", 
    stats.TxN, stats.TxStats.Rollback)
```

### 6.2 空间回收
```go
// BoltDB会自动回收已删除数据的空间
// 但如果需要手动压缩，可以使用以下方法
func CompactDB(db *bolt.DB, newPath string) error {
    return db.View(func(tx *bolt.Tx) error {
        return tx.CopyFile(newPath, 0600)
    })
}
```

## 7. 常见问题

### 7.1 数据库锁定
```go
// 如果遇到数据库锁定，设置超时时间
db, err := bolt.Open("constelfs.db", 0600, &bolt.Options{
    Timeout: 5 * time.Second,
})
```

### 7.2 并发访问
```go
// BoltDB支持多个读事务并发，但写事务是串行的
// 对于高并发写入场景，考虑批量写入或使用连接池
```

### 7.3 数据迁移
```go
// 从旧版本迁移数据
func MigrateData(db *bolt.DB) error {
    return db.Update(func(tx *bolt.Tx) error {
        // 创建新Bucket
        newBucket, err := tx.CreateBucketIfNotExists([]byte("NewUsers"))
        if err != nil {
            return err
        }
        
        // 读取旧数据
        oldBucket := tx.Bucket([]byte("Users"))
        if oldBucket == nil {
            return nil
        }
        
        // 迁移数据
        return oldBucket.ForEach(func(k, v []byte) error {
            return newBucket.Put(k, v)
        })
    })
}
```

## 8. 替代方案

如果BoltDB不能满足需求，可以考虑：

1. **BadgerDB**: 另一个高性能的Go KV存储
2. **LevelDB**: Google的KV存储，Go有实现
3. **SQLite**: 嵌入式SQL数据库，功能更强大

---

**文档版本**: v1.0  
**创建日期**: 2026-07-31
