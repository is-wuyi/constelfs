# ConstelFS 设计文档

## 1. 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   客户端        │    │   中心服务器    │    │   存储节点集群  │
│  (CLI/Web)      │◄──►│  (元数据管理)   │◄──►│  (数据存储)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

**数据流**：
- 客户端 → 存储节点（直接传输数据）
- 客户端 ↔ 中心服务器（元数据操作）
- 存储节点 ↔ 存储节点（分片分发）

## 2. 分片上传流程

**策略**：客户端上传到接收节点，接收节点分发到副本节点

```
文件: video.mp4 (40MB)
分片: chunk_001 (10MB), chunk_002 (10MB), chunk_003 (10MB), chunk_004 (10MB)
副本数: 3

流程:
1. 客户端请求写入 → 中心服务器
2. 中心服务器返回: 接收节点=A, 副本节点=[B, C]
3. 客户端上传 chunk_001 → Node A
4. Node A 分发 chunk_001 → Node B, Node C
5. 客户端继续上传 chunk_002 → Node B
6. ...
```

**分发策略**：智能选择（根据带宽）
- 并行分发：接收节点上行带宽大时
- 逐个分发：接收节点上行带宽小时
- 链式复制：节点间带宽大时

**分发失败处理**：
- 重试3次
- 失败后换节点
- 保证N份副本数

## 3. 分片大小策略

动态分片，根据文件大小调整：

| 文件大小 | 分片大小 |
|----------|----------|
| < 10MB   | 不切片   |
| < 100MB  | 4MB      |
| < 1GB    | 16MB     |
| < 10GB   | 64MB     |
| >= 10GB  | 128MB    |

## 4. 版本控制

**版本号**：递增整数（1, 2, 3...）

**版本限制**：默认3个版本，超过删除最旧版本

**版本回滚**：创建新版本，内容与目标版本相同

**版本删除**：立即删除分片数据

**分片存储**：每个版本独立存储分片

**SMB等协议**：只显示最新版本

**Web界面**：
- 显示最新版本详细信息
- 可以查看历史版本列表
- 可以下载任何历史版本
- 可以回滚到历史版本

## 5. 文件目录结构

使用路径字段实现目录结构：

```go
type FileInfo struct {
    FileID        string    `json:"file_id"`
    FileName      string    `json:"file_name"`
    FilePath      string    `json:"file_path"`      // 虚拟路径，如 /documents/report.pdf
    IsDirectory   bool      `json:"is_directory"`
    LatestVersion int       `json:"latest_version"`
    VersionCount  int       `json:"version_count"`
    MaxVersions   int       `json:"max_versions"`   // 默认3
    // ...
}
```

## 6. 数据加密

**加密方式**：客户端加密后上传，中心服务器存储密钥

**加密流程**：
1. 客户端生成随机密钥 (AES-256)
2. 用密钥加密文件数据
3. 上传加密后的数据到存储节点
4. 将密钥上传到中心服务器

**密钥管理**：
- 中心服务器存储密钥
- Web端可以获取密钥解密查看
- 存储节点只存储加密后的数据，无法解密

## 7. 带宽测量

**测速工具**：speedtest-cli（通过测速脚本自动安装）

**测速频率**：默认2小时一测，可手动触发

**测速数据**：存储在中心服务器，用于节点选择算法

**客户端测速**：启动时测速，用于计算并行下载数量

## 8. 节点选择算法

**评分因素**：

| 因素 | 权重 | 说明 |
|------|------|------|
| 上行带宽 | 30% | 家宽通常上行低，是瓶颈 |
| 空间利用率 | 25% | 已用/预分配，越低越好 |
| 下行带宽 | 20% | 家宽通常下行高 |
| 在线率 | 15% | 历史在线时间比例 |
| CPU/内存 | 10% | 系统负载 |

**权重配置**：Web管理界面可配置

## 9. 心跳机制

**心跳间隔**：30秒

**心跳超时**：90秒（3次心跳间隔）

**超时处理**：标记节点为offline

## 10. 节点配置流程

```
1. 存储节点启动 → 自动注册到中心服务器
2. 管理员在Web界面配置节点：
   - 存储路径（如 /volume1/constelfs/data）
   - 预分配空间（如 50GB）
3. 节点状态：registered → configured → online
```

## 11. API设计

### 节点管理
```
GET    /api/v1/nodes                    # 获取节点列表
POST   /api/v1/nodes                    # 注册节点
GET    /api/v1/nodes/:id                # 获取节点详情
DELETE /api/v1/nodes/:id                # 删除节点
POST   /api/v1/nodes/:id/configure      # 配置节点
POST   /api/v1/nodes/:id/heartbeat      # 节点心跳
```

### 文件管理
```
POST   /api/v1/files                    # 上传文件
GET    /api/v1/files/:id                # 获取文件详情
GET    /api/v1/files/:id/download       # 下载最新版本
GET    /api/v1/files/:id/versions/:v/download  # 下载指定版本
POST   /api/v1/files/:id/rollback       # 版本回滚
DELETE /api/v1/files/:id                # 删除文件
DELETE /api/v1/files/:id/versions/:v    # 删除指定版本
```

### 写入管理
```
POST   /api/v1/write                    # 请求写入
POST   /api/v1/write/confirm            # 确认写入完成
```

### 健康检查
```
GET    /api/v1/health                   # 健康检查
```

## 12. 数据库设计

### 中心服务器数据库

```go
// 文件信息
type FileInfo struct {
    FileID          string    `json:"file_id"`
    FileName        string    `json:"file_name"`
    FilePath        string    `json:"file_path"`
    IsDirectory     bool      `json:"is_directory"`
    LatestVersion   int       `json:"latest_version"`
    VersionCount    int       `json:"version_count"`
    MaxVersions     int       `json:"max_versions"`
    EncryptionKey   string    `json:"encryption_key"`
    Size            int64     `json:"size"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// 文件版本
type FileVersion struct {
    VersionID   string    `json:"version_id"`
    FileID      string    `json:"file_id"`
    Version     int       `json:"version"`
    Size        int64     `json:"size"`
    Hash        string    `json:"hash"`
    ChunkIDs    []string  `json:"chunk_ids"`
    NodeIDs     []string  `json:"node_ids"`
    CreatedAt   time.Time `json:"created_at"`
}

// 节点信息
type NodeInfo struct {
    NodeID          string    `json:"node_id"`
    IPAddress       string    `json:"ip_address"`
    Port            int       `json:"port"`
    Status          string    `json:"status"`
    StoragePath     string    `json:"storage_path"`
    TotalDiskSpace  int64     `json:"total_disk_space"`
    AllocatedSpace  int64     `json:"allocated_space"`
    UsedSpace       int64     `json:"used_space"`
    UploadSpeed     float64   `json:"upload_speed"`
    DownloadSpeed   float64   `json:"download_speed"`
    LastHeartbeat   time.Time `json:"last_heartbeat"`
    LastSpeedTest   time.Time `json:"last_speed_test"`
    CreatedAt       time.Time `json:"created_at"`
}

// 测速结果
type SpeedTestResult struct {
    ID            int64     `json:"id"`
    NodeID        string    `json:"node_id"`
    UploadSpeed   float64   `json:"upload_speed"`
    DownloadSpeed float64   `json:"download_speed"`
    Latency       int64     `json:"latency"`
    TestTime      time.Time `json:"test_time"`
}
```

### 存储节点数据库

```go
// 本地分片信息
type LocalChunkInfo struct {
    ChunkID   string    `json:"chunk_id"`
    FileID    string    `json:"file_id"`
    Version   int       `json:"version"`
    Index     int       `json:"index"`
    Size      int64     `json:"size"`
    Hash      string    `json:"hash"`
    FilePath  string    `json:"file_path"`
    CreatedAt time.Time `json:"created_at"`
}

// 分发任务
type ReplicationTask struct {
    TaskID      string    `json:"task_id"`
    ChunkID     string    `json:"chunk_id"`
    TargetNode  string    `json:"target_node"`
    Status      string    `json:"status"`
    RetryCount  int       `json:"retry_count"`
    CreatedAt   time.Time `json:"created_at"`
    CompletedAt time.Time `json:"completed_at"`
}
```

## 13. 技术栈

| 组件 | 技术 |
|------|------|
| 后端语言 | Go 1.21 |
| 数据库 | BoltDB (bbolt) |
| API | RESTful + gRPC |
| Web前端 | Vue3 + Element Plus |
| 测速工具 | speedtest-cli |
| 加密算法 | AES-256 |

## 14. 测试环境

| 角色 | 地址 | 凭据 |
|------|------|------|
| 中心服务器 | 193.134.209.37:78 | root / 0044039Bb* |
| NAS-001 | 27119.et.net | jimo / 12345678 |
| NAS-002 | 27233.et.net | jimo / 12345678 |
| NAS-003 | 27348.et.net | jimo / 12345678 |

---

**最后更新**: 2026-08-01
