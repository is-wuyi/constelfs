# ConstelFS (星群存储) 开发计划

## 1. 项目概述

### 1.1 项目名称
ConstelFS (星群存储)

### 1.2 项目寓意
源自Constellation（星座/星群），寓意散落在各地的不同Linux节点，就像一颗颗散落的星星，聚在一起组成星座。

### 1.3 项目目标
开发一个分布式文件存储系统，聚合全国各地的Linux NAS闲置存储资源，提供高可用、高性能的文件存储服务。

### 1.4 核心特性
- **分布式存储**: 多节点协同工作，数据自动分布
- **高可用**: 支持节点动态上下线，数据自动恢复
- **高性能**: 并行读写，智能调度
- **易管理**: Web管理界面，可视化监控
- **多协议支持**: SMB、FUSE、WebDAV协议转出

## 2. 架构设计

### 2.1 整体架构
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   客户端        │    │   中心服务器    │    │   存储节点集群  │
│  (SMB/FUSE/     │◄──►│  (元数据管理)   │◄──►│  (数据存储)     │
│   WebDAV)       │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 2.2 控制流与数据流分离
- **控制流**: 客户端 ↔ 中心服务器（轻量级，元数据操作）
- **数据流**: 客户端 ↔ 存储节点（重量级，文件读写）

### 2.3 节点角色
1. **中心服务器**: 元数据管理、节点调度、用户认证
2. **存储节点代理**: 数据存储、健康监控、数据传输
3. **客户端**: 协议转出、文件切片、并行传输

## 3. 技术栈选择

### 3.1 后端技术
| 组件 | 技术选择 | 理由 |
|------|----------|------|
| **编程语言** | Go | 高并发性能好，跨平台编译简单，学习曲线平缓 |
| **数据库** | BoltDB (bbolt) | Go原生KV存储，高性能，单文件，无需部署 |
| **API协议** | RESTful + gRPC | Web管理用RESTful，节点间通信用gRPC |
| **配置管理** | YAML配置文件 | 简单直接，易于管理 |

### 3.2 前端技术
| 组件 | 技术选择 | 理由 |
|------|----------|------|
| **前端框架** | Vue3 | 学习曲线平缓，中文文档丰富 |
| **UI组件库** | Element Plus | 丰富的管理界面组件，适合快速开发 |
| **构建工具** | Vite | 快速开发服务器，热更新 |

### 3.3 运维技术
| 组件 | 技术选择 | 理由 |
|------|----------|------|
| **版本控制** | Git + GitHub | 标准版本控制系统，工具支持好 |
| **CI/CD** | GitHub Actions | 渐进式CI/CD，与GitHub集成好 |
| **监控** | Prometheus + Grafana | 轻量级，易于部署，可视化好 |
| **日志** | 结构化日志 + ELK（可选） | 初期简单日志，后期可扩展 |

## 4. 开发阶段

### 4.1 第一阶段：核心存储功能（4-6周）
**目标**: 实现基本的分布式存储功能

**主要任务**:
1. **项目初始化**
   - 创建项目结构
   - 配置Go模块
   - 设置CI/CD流水线

2. **中心服务器核心**
   - 元数据管理（文件、分片、副本信息）
   - 节点管理（注册、心跳、状态监控）
   - 用户认证（预共享密钥）

3. **存储节点代理**
   - 数据存储（分片存储、校验和验证）
   - 健康监控（CPU、内存、磁盘使用率）
   - 心跳上报

4. **基础通信**
   - RESTful API（Web管理）
   - gRPC（节点间通信）
   - 节点自动发现

**交付物**:
- 可运行的中心服务器
- 可运行的存储节点代理
- 基础API文档

### 4.2 第二阶段：Web管理界面（3-4周）
**目标**: 实现可视化的管理界面

**主要任务**:
1. **节点控制台**
   - 节点列表展示（IP、CPU、内存、磁盘）
   - 节点状态监控（在线/离线）
   - 手动管控（标记离线、触发数据恢复）

2. **文件管理**
   - 文件浏览器（目录结构、文件列表）
   - 文件元数据查看（分片信息、副本位置）
   - 文件操作（上传、下载、删除）

3. **容量分析**
   - 集群总空间、已用空间
   - 副本占用空间、纠删码节省空间
   - 冷热数据分析

4. **用户管理**
   - 用户列表、空间配额
   - 权限控制（读写/只读）

5. **告警通知**
   - 节点掉线告警
   - 磁盘空间不足告警
   - 数据恢复失败告警

**交付物**:
- 完整的Web管理界面
- 用户手册

### 4.3 第三阶段：客户端开发（4-5周）
**目标**: 实现协议转出功能

**主要任务**:
1. **命令行客户端**
   - 基础文件操作（上传、下载、列表）
   - 配置管理（服务器地址、认证信息）

2. **FUSE挂载**
   - 实现FUSE文件系统接口
   - 支持挂载为本地磁盘

3. **SMB协议转出**
   - 集成Samba或实现SMB服务器
   - 支持Windows文件共享

4. **WebDAV协议**
   - 实现WebDAV服务器
   - 支持WebDAV客户端访问

5. **并行传输优化**
   - 文件分片并行上传/下载
   - 多节点并行读取加速

**交付物**:
- 命令行客户端工具
- FUSE挂载功能
- SMB/WebDAV协议支持

### 4.4 第四阶段：高级功能（3-4周）
**目标**: 实现数据安全和性能优化

**主要任务**:
1. **数据加密**
   - 传输加密（TLS）
   - 存储加密（AES-256）

2. **纠删码支持**
   - 实现Reed-Solomon纠删码
   - 支持副本转纠删码
   - 可配置纠删码参数

3. **智能调度**
   - 节点健康度评估
   - 智能副本放置策略
   - 负载均衡算法

4. **数据迁移**
   - 可配置迁移策略
   - 自动/手动触发迁移
   - 迁移进度监控

**交付物**:
- 完整的数据安全功能
- 智能调度系统
- 数据迁移功能

### 4.5 第五阶段：测试与优化（2-3周）
**目标**: 系统测试和性能优化

**主要任务**:
1. **单元测试**
   - 核心模块单元测试
   - 覆盖率目标：80%+

2. **集成测试**
   - 组件间集成测试
   - 端到端功能测试

3. **性能测试**
   - 读写性能测试
   - 并发性能测试
   - 大规模节点测试

4. **文档完善**
   - API文档完善
   - 部署指南
   - 用户手册

**交付物**:
- 完整的测试套件
- 性能测试报告
- 完整的项目文档

## 5. 模块设计

### 5.1 中心服务器模块
```
server/
├── api/           # RESTful API处理器
├── auth/          # 用户认证
├── metadata/      # 元数据管理
├── scheduler/     # 节点调度
├── config/        # 配置管理
└── main.go        # 入口文件
```

### 5.2 存储节点模块
```
node/
├── storage/       # 数据存储引擎
├── health/        # 健康监控
├── transport/     # 数据传输
├── config/        # 配置管理
└── main.go        # 入口文件
```

### 5.3 客户端模块
```
client/
├── cli/           # 命令行接口
├── fuse/          # FUSE文件系统
├── smb/           # SMB协议
├── webdav/        # WebDAV协议
└── main.go        # 入口文件
```

### 5.4 公共模块
```
common/
├── protocol/      # 通信协议定义
├── utils/         # 工具函数
├── crypto/        # 加密算法
└── erasure/       # 纠删码算法
```

## 6. 数据库设计 (BoltDB KV存储)

### 6.1 存储桶(Bucket)设计
```go
// BoltDB使用Bucket来组织数据，类似SQL中的表

// 用户信息桶
// Key: username (string)
// Value: JSON编码的User结构体
type User struct {
    ID           int64     `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"`
    QuotaBytes   int64     `json:"quota_bytes"`
    UsedBytes    int64     `json:"used_bytes"`
    CreatedAt    time.Time `json:"created_at"`
}

// 节点信息桶
// Key: node_id (string)
// Value: JSON编码的Node结构体
type Node struct {
    ID              int64     `json:"id"`
    NodeID          string    `json:"node_id"`
    IPAddress       string    `json:"ip_address"`
    Port            int       `json:"port"`
    Status          string    `json:"status"`            // registered, configured, online, offline, maintenance
    StoragePath     string    `json:"storage_path"`      // 存储路径（管理员配置）
    TotalDiskSpace  int64     `json:"total_disk_space"`  // 节点总磁盘空间
    AllocatedSpace  int64     `json:"allocated_space"`   // 预分配上限（管理员设置）
    UsedSpace       int64     `json:"used_space"`        // 集群实际已占用空间
    CPUUsage        float64   `json:"cpu_usage"`
    MemoryUsage     float64   `json:"memory_usage"`
    DiskUsage       float64   `json:"disk_usage"`
    LastHeartbeat   time.Time `json:"last_heartbeat"`
    ConfiguredAt    time.Time `json:"configured_at"`     // 配置时间
    CreatedAt       time.Time `json:"created_at"`
}

// 文件信息桶
// Key: "user_id:dir_path:file_name" (string, 如 "1:/documents:report.pdf")
// Value: JSON编码的File结构体
type File struct {
    ID                int64     `json:"id"`
    UserID            int64     `json:"user_id"`
    DirPath           string    `json:"dir_path"`            // 目录路径，如 "/documents"
    FileName          string    `json:"file_name"`           // 文件名，如 "report.pdf"
    FilePath          string    `json:"file_path"`           // 完整路径（自动生成）
    FileSize          int64     `json:"file_size"`
    IsDirectory       bool      `json:"is_directory"`
    ReplicationFactor int       `json:"replication_factor"`
    ErasureCoded      bool      `json:"erasure_coded"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

// 分片信息桶
// Key: chunk_hash (string, SHA-256 Hash)
// Value: JSON编码的Chunk结构体
type Chunk struct {
    ID         int64     `json:"id"`
    FileID     int64     `json:"file_id"`
    ChunkIndex int       `json:"chunk_index"`
    ChunkSize  int64     `json:"chunk_size"`
    ChunkHash  string    `json:"chunk_hash"`   // SHA-256 Hash，用作存储文件名
    Checksum   string    `json:"checksum"`      // 校验和（可选，用于完整性验证）
    CreatedAt  time.Time `json:"created_at"`
}

// 副本信息桶
// Key: "chunk_id:node_id" (string, 如 "456:node1")
// Value: JSON编码的Replica结构体
type Replica struct {
    ID        int64     `json:"id"`
    ChunkID   int64     `json:"chunk_id"`
    NodeID    string    `json:"node_id"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}
```

### 6.2 索引设计
```go
// 为了支持高效查询，需要维护额外的索引桶

// 用户ID索引
// Key: user_id (int64的字节表示)
// Value: username (string)

// 文件ID索引
// Key: file_id (int64的字节表示)
// Value: file_path (string)

// 节点状态索引
// Key: status (string)
// Value: node_id列表的JSON编码
```

## 7. API设计

### 7.1 RESTful API（Web管理）
```
# 节点管理
GET    /api/v1/nodes              # 获取节点列表
GET    /api/v1/nodes/:id          # 获取节点详情
POST   /api/v1/nodes/:id/offline  # 标记节点离线
POST   /api/v1/nodes/:id/recover  # 触发数据恢复

# 文件管理
GET    /api/v1/files              # 获取文件列表
GET    /api/v1/files/:id          # 获取文件详情
POST   /api/v1/files/upload       # 上传文件
DELETE /api/v1/files/:id          # 删除文件

# 用户管理
GET    /api/v1/users              # 获取用户列表
POST   /api/v1/users              # 创建用户
PUT    /api/v1/users/:id/quota    # 设置用户配额

# 监控数据
GET    /api/v1/metrics/cluster    # 集群指标
GET    /api/v1/metrics/nodes      # 节点指标
GET    /api/v1/alerts             # 告警信息
```

### 7.2 gRPC API（节点间通信）
```protobuf
syntax = "proto3";

service NodeService {
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    rpc RegisterNode(RegisterNodeRequest) returns (RegisterNodeResponse);
    rpc GetNodeStatus(GetNodeStatusRequest) returns (GetNodeStatusResponse);
}

service StorageService {
    rpc WriteChunk(WriteChunkRequest) returns (WriteChunkResponse);
    rpc ReadChunk(ReadChunkRequest) returns (ReadChunkResponse);
    rpc DeleteChunk(DeleteChunkRequest) returns (DeleteChunkResponse);
    rpc ReplicateChunk(ReplicateChunkRequest) returns (ReplicateChunkResponse);
}

service MetadataService {
    rpc GetFileMetadata(GetFileMetadataRequest) returns (GetFileMetadataResponse);
    rpc UpdateFileMetadata(UpdateFileMetadataRequest) returns (UpdateFileMetadataResponse);
    rpc GetChunkLocations(GetChunkLocationsRequest) returns (GetChunkLocationsResponse);
}
```

## 8. 测试策略

### 8.1 单元测试
- **覆盖率目标**: 80%+
- **测试框架**: Go标准testing包
- **测试内容**: 核心算法、数据结构、工具函数

### 8.2 集成测试
- **测试范围**: 组件间交互
- **测试环境**: Docker容器化
- **测试内容**: API接口、数据流、错误处理

### 8.3 端到端测试
- **测试场景**: 完整用户流程
- **测试工具**: 自动化测试脚本
- **测试内容**: 文件上传下载、节点故障恢复、数据迁移

### 8.4 性能测试
- **测试指标**: 吞吐量、延迟、并发数
- **测试工具**: 自定义基准测试
- **测试内容**: 读写性能、并发性能、大规模测试

## 9. 部署方案

### 9.1 开发环境
- **本地开发**: 单机多节点模拟
- **容器化**: Docker Compose
- **配置管理**: 环境变量 + 配置文件

### 9.2 测试环境
- **节点数量**: 3-5个节点
- **部署方式**: 原生部署
- **监控**: Prometheus + Grafana

### 9.3 生产环境
- **节点数量**: 25个节点左右
- **部署方式**: 原生部署（后续支持Docker）
- **高可用**: 中心服务器主备
- **备份**: 定期配置备份

## 10. 风险评估与应对

### 10.1 技术风险
| 风险 | 影响 | 应对措施 |
|------|------|----------|
| 网络不稳定 | 高 | 实现重试机制、断点续传 |
| 数据一致性 | 高 | 实现强一致性协议、原子提交 |
| 性能瓶颈 | 中 | 智能调度、并行传输优化 |
| 存储节点故障 | 中 | 多副本、自动数据恢复 |

### 10.2 进度风险
| 风险 | 影响 | 应对措施 |
|------|------|----------|
| 技术难点 | 中 | 提前原型验证、分阶段实现 |
| 需求变更 | 低 | 需求冻结、变更控制 |
| 资源不足 | 低 | 优先级排序、核心功能优先 |

## 11. 项目时间线

```
第1-6周:  核心存储功能开发
第7-10周: Web管理界面开发
第11-15周: 客户端开发
第16-19周: 高级功能开发
第20-22周: 测试与优化
第23周:   文档完善与发布准备
```

**总开发时间**: 约23周（5-6个月）

## 12. 后续扩展

### 12.1 功能扩展
- **快照功能**: 文件版本控制、快照恢复
- **多租户**: 完整的多租户支持
- **API扩展**: S3兼容API
- **移动端**: 移动客户端应用

### 12.2 性能扩展
- **分布式元数据**: 元数据分片存储
- **缓存层**: Redis缓存热点数据
- **CDN集成**: 静态资源CDN加速

### 12.3 运维扩展
- **容器化部署**: Docker + Kubernetes
- **自动化运维**: Ansible/Terraform
- **告警推送**: 钉钉/飞书/Telegram集成

---

**文档版本**: v1.0  
**创建日期**: 2026-07-31  
**最后更新**: 2026-07-31
