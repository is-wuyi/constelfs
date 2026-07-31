# 存储节点配置流程

## 1. 概述

存储节点的配置是一个**两阶段过程**：
1. **自动注册**：节点部署后自动连接中心服务器
2. **手动配置**：管理员在Web界面配置存储路径和空间

## 2. 完整流程

### 2.1 流程图

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  存储节点    │     │  中心服务器  │     │  Web管理界面 │
│  (NAS设备)   │     │              │     │  (管理员)    │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ 1. 启动并连接      │                    │
       │───────────────────►│                    │
       │                    │                    │
       │ 2. 发送节点信息    │                    │
       │    (IP、CPU、内存、│                    │
       │     磁盘信息)      │                    │
       │───────────────────►│                    │
       │                    │                    │
       │ 3. 注册节点        │                    │
       │    状态: registered │                    │
       │                    │                    │
       │                    │ 4. 推送新节点通知  │
       │                    │───────────────────►│
       │                    │                    │
       │                    │ 5. 管理员查看节点  │
       │                    │◄───────────────────│
       │                    │                    │
       │                    │ 6. 配置存储路径    │
       │                    │    和预分配空间    │
       │                    │◄───────────────────│
       │                    │                    │
       │ 7. 推送配置        │                    │
       │◄───────────────────│                    │
       │                    │                    │
       │ 8. 应用配置        │                    │
       │    创建存储目录    │                    │
       │───────────────────►│                    │
       │                    │                    │
       │ 9. 配置完成        │                    │
       │    状态: configured│                    │
       │───────────────────►│                    │
       │                    │                    │
       │                    │ 10. 更新界面状态   │
       │                    │───────────────────►│
       │                    │                    │
       │ 11. 开始心跳       │                    │
       │    状态: online    │                    │
       │───────────────────►│                    │
       │                    │                    │
```

### 2.2 详细步骤说明

#### 步骤1-2：节点启动并发送信息
```go
// 存储节点启动时
func (n *Node) Start() error {
    // 1. 检测本机信息
    info := n.detectSystemInfo()
    
    // 2. 连接中心服务器
    conn, err := n.connectToMaster(info)
    if err != nil {
        return err
    }
    
    // 3. 发送节点信息
    return n.registerNode(conn, info)
}

// 检测系统信息
func (n *Node) detectSystemInfo() *NodeInfo {
    return &NodeInfo{
        NodeID:      n.generateNodeID(),
        IPAddress:   n.getIPAddress(),
        Port:        n.config.Port,
        CPUUsage:    n.getCPUUsage(),
        MemoryUsage: n.getMemoryUsage(),
        Disks:       n.getDiskInfo(),  // 所有可用磁盘
    }
}
```

#### 步骤3：中心服务器注册节点
```go
// 中心服务器收到节点注册请求
func (s *Server) RegisterNode(info *NodeInfo) error {
    // 1. 检查节点是否已注册
    existingNode, err := s.db.GetNode(info.NodeID)
    if err == nil {
        // 已存在，更新信息
        existingNode.IPAddress = info.IPAddress
        existingNode.CPUUsage = info.CPUUsage
        existingNode.MemoryUsage = info.MemoryUsage
        return s.db.UpdateNode(existingNode)
    }
    
    // 2. 创建新节点
    node := &Node{
        NodeID:         info.NodeID,
        IPAddress:      info.IPAddress,
        Port:           info.Port,
        Status:         "registered",  // 初始状态：已注册
        TotalDiskSpace: info.TotalDiskSpace,
        CPUUsage:       info.CPUUsage,
        MemoryUsage:    info.MemoryUsage,
        DiskUsage:      info.DiskUsage,
        CreatedAt:      time.Now(),
    }
    
    return s.db.CreateNode(node)
}
```

#### 步骤4-6：管理员配置节点
```go
// Web管理界面 - 配置节点
type NodeConfigRequest struct {
    NodeID         string `json:"node_id"`
    StoragePath    string `json:"storage_path"`     // 存储路径
    AllocatedSpace int64  `json:"allocated_space"`  // 预分配空间
}

// API处理器
func (s *Server) ConfigureNode(w http.ResponseWriter, r *http.Request) {
    var req NodeConfigRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 1. 获取节点信息
    node, err := s.db.GetNode(req.NodeID)
    if err != nil {
        http.Error(w, "节点不存在", http.StatusNotFound)
        return
    }
    
    // 2. 验证存储路径
    if err := s.validateStoragePath(node, req.StoragePath); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 3. 验证预分配空间
    if err := s.validateAllocatedSpace(node, req.AllocatedSpace); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 4. 更新节点配置
    node.StoragePath = req.StoragePath
    node.AllocatedSpace = req.AllocatedSpace
    node.Status = "configured"
    node.ConfiguredAt = time.Now()
    
    if err := s.db.UpdateNode(node); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 5. 通知节点应用配置
    s.notifyNodeConfig(node)
    
    // 6. 返回成功
    json.NewEncoder(w).Encode(node)
}

// 验证存储路径
func (s *Server) validateStoragePath(node *Node, path string) error {
    // 发送请求到节点，验证路径是否存在且可写
    resp, err := s.sendToNode(node.NodeID, &ValidatePathRequest{
        Path: path,
    })
    if err != nil {
        return err
    }
    
    if !resp.Valid {
        return fmt.Errorf("存储路径无效: %s", resp.Reason)
    }
    
    return nil
}

// 验证预分配空间
func (s *Server) validateAllocatedSpace(node *Node, allocatedSpace int64) error {
    // 检查是否超过磁盘总空间
    if allocatedSpace > node.TotalDiskSpace {
        return fmt.Errorf("预分配空间超过磁盘总空间")
    }
    
    // 检查是否合理（至少1GB）
    if allocatedSpace < 1024*1024*1024 {
        return fmt.Errorf("预分配空间至少需要1GB")
    }
    
    return nil
}
```

#### 步骤7-9：节点应用配置
```go
// 存储节点收到配置
func (n *Node) ApplyConfig(config *NodeConfig) error {
    // 1. 创建存储目录
    if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
        return err
    }
    
    // 2. 检查目录权限
    if err := n.checkDirectoryPermissions(config.StoragePath); err != nil {
        return err
    }
    
    // 3. 初始化存储结构
    if err := n.initStorageStructure(config.StoragePath); err != nil {
        return err
    }
    
    // 4. 更新本地配置
    n.config.StoragePath = config.StoragePath
    n.config.AllocatedSpace = config.AllocatedSpace
    n.status = "configured"
    
    // 5. 通知中心服务器配置完成
    return n.notifyConfigComplete()
}

// 初始化存储结构
func (n *Node) initStorageStructure(basePath string) error {
    // 创建必要的目录
    dirs := []string{
        "chunks",      // 分片存储
        "meta",        // 元数据
        "temp",        // 临时文件
        "trash",       // 回收站
    }
    
    for _, dir := range dirs {
        path := filepath.Join(basePath, dir)
        if err := os.MkdirAll(path, 0755); err != nil {
            return err
        }
    }
    
    return nil
}
```

#### 步骤10-11：节点开始工作
```go
// 节点开始心跳
func (n *Node) StartHeartbeat() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // 1. 收集系统信息
        info := n.collectSystemInfo()
        
        // 2. 发送心跳
        err := n.sendHeartbeat(info)
        if err != nil {
            n.status = "offline"
            log.Printf("心跳发送失败: %v", err)
            continue
        }
        
        // 3. 更新状态
        n.status = "online"
    }
}

// 收集系统信息
func (n *Node) collectSystemInfo() *HeartbeatInfo {
    return &HeartbeatInfo{
        NodeID:      n.nodeID,
        CPUUsage:    n.getCPUUsage(),
        MemoryUsage: n.getMemoryUsage(),
        DiskUsage:   n.getDiskUsage(),
        UsedSpace:   n.getUsedSpace(),
        Status:      n.status,
    }
}
```

## 3. API设计

### 3.1 节点注册API
```
POST /api/v1/nodes/register
Content-Type: application/json

Request:
{
    "node_id": "node-abc123",
    "ip_address": "192.168.1.102",
    "port": 8080,
    "total_disk_space": 2147483648000,
    "cpu_usage": 15.5,
    "memory_usage": 45.2,
    "disk_usage": 23.1
}

Response:
{
    "success": true,
    "node_id": "node-abc123",
    "status": "registered"
}
```

### 3.2 节点配置API
```
POST /api/v1/nodes/{node_id}/configure
Content-Type: application/json

Request:
{
    "storage_path": "/mnt/disk2/constelfs",
    "allocated_space": 1610612736000  // 1.5TB
}

Response:
{
    "success": true,
    "node": {
        "node_id": "node-abc123",
        "status": "configured",
        "storage_path": "/mnt/disk2/constelfs",
        "allocated_space": 1610612736000,
        "configured_at": "2026-07-31T12:00:00Z"
    }
}
```

### 3.3 获取未配置节点API
```
GET /api/v1/nodes?status=registered

Response:
{
    "nodes": [
        {
            "node_id": "node-abc123",
            "ip_address": "192.168.1.102",
            "status": "registered",
            "cpu_usage": 15.5,
            "memory_usage": 45.2,
            "disks": [
                {"path": "/mnt/disk1", "total": 1073741824000, "available": 858993459200},
                {"path": "/mnt/disk2", "total": 2147483648000, "available": 1610612736000}
            ]
        }
    ]
}
```

### 3.4 节点心跳API
```
POST /api/v1/nodes/{node_id}/heartbeat
Content-Type: application/json

Request:
{
    "cpu_usage": 15.5,
    "memory_usage": 45.2,
    "disk_usage": 23.1,
    "used_space": 322122547200,
    "status": "online"
}

Response:
{
    "success": true,
    "commands": []  // 中心服务器可以下发指令
}
```

## 4. Web管理界面设计

### 4.1 未配置节点卡片
```html
<div class="node-card unconfigured">
    <div class="node-header">
        <span class="node-name">NAS-003</span>
        <span class="node-ip">192.168.1.102</span>
        <span class="node-status">未配置</span>
    </div>
    <div class="node-metrics">
        <div class="metric">
            <span class="label">CPU</span>
            <span class="value">15%</span>
        </div>
        <div class="metric">
            <span class="label">内存</span>
            <span class="value">45%</span>
        </div>
    </div>
    <div class="node-disks">
        <h4>可用磁盘</h4>
        <div class="disk">
            <span>/mnt/disk1</span>
            <span>1TB可用</span>
            <button>选择</button>
        </div>
        <div class="disk">
            <span>/mnt/disk2</span>
            <span>2TB可用</span>
            <button>选择</button>
        </div>
    </div>
    <div class="node-actions">
        <button class="btn-primary">配置存储</button>
        <button class="btn-secondary">忽略</button>
    </div>
</div>
```

### 4.2 配置对话框
```html
<div class="config-dialog">
    <h3>配置存储节点 - NAS-003</h3>
    
    <div class="form-group">
        <label>存储路径</label>
        <select>
            <option>/mnt/disk1/constelfs</option>
            <option>/mnt/disk2/constelfs</option>
        </select>
    </div>
    
    <div class="form-group">
        <label>预分配空间</label>
        <input type="number" placeholder="1.5">
        <span>TB</span>
        <span class="hint">最大: 2TB</span>
    </div>
    
    <div class="form-group">
        <label>存储目录预览</label>
        <code>/mnt/disk2/constelfs/</code>
    </div>
    
    <div class="dialog-actions">
        <button class="btn-secondary">取消</button>
        <button class="btn-primary">确认配置</button>
    </div>
</div>
```

## 5. 配置验证

### 5.1 路径验证
```go
// 验证存储路径
func (n *Node) ValidateStoragePath(path string) error {
    // 1. 检查路径是否存在
    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        // 尝试创建目录
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("无法创建目录: %v", err)
        }
    } else if err != nil {
        return fmt.Errorf("路径检查失败: %v", err)
    }
    
    // 2. 检查是否是目录
    if !info.IsDir() {
        return fmt.Errorf("路径不是目录")
    }
    
    // 3. 检查写权限
    testFile := filepath.Join(path, ".test_write")
    if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
        return fmt.Errorf("目录没有写权限")
    }
    os.Remove(testFile)
    
    // 4. 检查是否为空目录
    entries, err := os.ReadDir(path)
    if err != nil {
        return fmt.Errorf("无法读取目录: %v", err)
    }
    if len(entries) > 0 {
        return fmt.Errorf("目录不为空，可能包含其他数据")
    }
    
    return nil
}
```

### 5.2 空间验证
```go
// 验证预分配空间
func (n *Node) ValidateAllocatedSpace(allocatedSpace int64) error {
    // 1. 检查是否超过磁盘总空间
    if allocatedSpace > n.TotalDiskSpace {
        return fmt.Errorf("预分配空间 (%.2fGB) 超过磁盘总空间 (%.2fGB)", 
            float64(allocatedSpace)/1024/1024/1024,
            float64(n.TotalDiskSpace)/1024/1024/1024)
    }
    
    // 2. 检查是否合理（至少1GB，最多90%磁盘空间）
    minSpace := int64(1024 * 1024 * 1024)  // 1GB
    maxSpace := int64(float64(n.TotalDiskSpace) * 0.9)  // 90%
    
    if allocatedSpace < minSpace {
        return fmt.Errorf("预分配空间至少需要1GB")
    }
    if allocatedSpace > maxSpace {
        return fmt.Errorf("预分配空间不能超过磁盘总空间的90%%")
    }
    
    return nil
}
```

## 6. 错误处理

### 6.1 常见错误
| 错误 | 原因 | 解决方案 |
|------|------|----------|
| 路径不存在 | 存储路径无效 | 检查路径是否正确 |
| 权限不足 | 目录没有写权限 | 修改目录权限 |
| 空间不足 | 预分配空间超过磁盘容量 | 减少预分配空间 |
| 目录不为空 | 存储目录包含其他数据 | 清空目录或选择其他路径 |

### 6.2 错误响应格式
```json
{
    "success": false,
    "error": {
        "code": "INVALID_STORAGE_PATH",
        "message": "存储路径无效: /mnt/disk1/constelfs",
        "details": "目录没有写权限"
    }
}
```

---

**文档版本**: v1.0  
**创建日期**: 2026-07-31

## 7. 存储路径设计理念

### 7.1 设计原则

ConstelFS **不关心**存储路径的底层实现，只关心：
- 路径是否存在
- 是否有写权限
- 可用空间大小

这意味着用户可以自由选择：

| 方案 | 存储路径 | 底层实现 | 适用场景 |
|------|----------|----------|----------|
| **直接目录** | `/mnt/data/constelfs` | 真实目录 | 通用场景 |
| **Loop设备** | `/volume1/constelfs/data` | Loop设备挂载 | 隐藏存储（Synology等NAS） |

### 7.2 Loop设备隐藏方案（ConstelFS测试/部署使用）

```bash
# 1. 创建稀疏文件（不立即占用空间）
dd if=/dev/zero of=/volume1/constelfs-storage/constelfs.img bs=1M seek=204800 count=0

# 2. 格式化为ext4
mkfs.ext4 -F /volume1/constelfs-storage/constelfs.img

# 3. 创建挂载点
mkdir -p /volume1/constelfs-storage/data

# 4. 挂载loop设备
mount -o loop /volume1/constelfs-storage/constelfs.img /volume1/constelfs-storage/data

# 5. 在ConstelFS中配置存储路径
# 存储路径 = /volume1/constelfs-storage/data
```

### 7.3 优势

- **开源通用**：任何人都能使用，不依赖特定NAS
- **灵活配置**：用户自主决定是否隐藏存储
- **ConstelFS无感知**：只关心路径，不关心底层实现

---

**文档版本**: v1.1  
**更新日期**: 2026-07-31
