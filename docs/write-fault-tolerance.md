# 写入容错方案设计

## 概述

ConstelFS 采用 **Quorum + 临时文件 + 动态替换 + 断点续传** 的组合策略来保证写入的可靠性。

## 核心机制

### 1. 副本策略

- **N副本**：默认3副本，可配置（2-5）
- **必须全部成功**：所有副本都写入成功才算写入成功
- **失败重试**：单个副本失败后重试，最多3次
- **动态替换**：重试失败后换节点重新写入

### 2. 写入流程

```
客户端请求写入
    │
    ▼
中心服务器选择N个节点（智能选择）
    │
    ▼
客户端并行写入N个节点（临时文件 .tmp）
    │
    ▼
检查所有节点写入结果
    │
    ├─ 全部成功 → 重命名临时文件为正式文件 → 返回成功
    │
    └─ 有失败 → 重试失败的分片（最多3次）
                    │
                    ├─ 重试成功 → 继续
                    │
                    └─ 重试失败 → 换节点重试
                                    │
                                    ├─ 成功 → 继续
                                    │
                                    └─ 失败 → 返回错误
```

### 3. 重试策略

| 失败原因 | 处理方式 | 说明 |
|----------|----------|------|
| 网络超时 | 重试同一节点 | 可能是临时网络问题 |
| 节点离线 | 换节点 | 节点不可用 |
| 磁盘满 | 换节点 | 空间不足 |
| 写入错误 | 重试同一节点 | 可能是临时IO问题 |
| 校验失败 | 换节点 | 数据损坏 |

**重试配置**：
- 最大重试次数：3次
- 重试间隔：指数退避（1s → 2s → 4s）
- 单次写入超时：30秒

### 4. 断点续传

文件被切分为多个分片（chunk），每个分片独立写入和校验。

```
文件: video.mp4 (1GB)
分片: chunk_001 (64MB), chunk_002 (64MB), ..., chunk_016 (64MB)

写入状态:
- chunk_001: ✅ 成功
- chunk_002: ✅ 成功
- chunk_003: ❌ 失败
- chunk_004: ✅ 成功
...

断点续传: 只重传 chunk_003
```

**实现方式**：
- 记录每个分片的写入状态（成功/失败/进行中）
- 只重传失败的分片
- 支持断点续传（客户端可以暂停和恢复）

### 5. 智能节点选择

节点选择算法综合考虑以下因素：

| 因素 | 权重 | 说明 |
|------|------|------|
| CPU使用率 | 40% | 使用率越低越好 |
| 可用空间 | 30% | 空间越大越好 |
| 在线率 | 20% | 历史在线率越高越好 |
| 响应速度 | 10% | 延迟越低越好 |

**选择流程**：
1. 获取所有在线节点
2. 计算每个节点的评分
3. 选择评分最高的N个节点
4. 如果节点数不足，返回错误

## 错误处理

### 写入失败处理

1. **单个分片失败**：
   - 重试该分片（最多3次）
   - 重试失败则换节点

2. **所有重试都失败**：
   - 清理已写入的临时文件
   - 返回错误给客户端

3. **节点离线**：
   - 标记节点为offline
   - 选择新节点继续写入

### 临时文件清理

- 写入成功：重命名为正式文件
- 写入失败：删除临时文件
- 定期清理：清理孤立的临时文件（超过24小时）

## 代码示例

```go
// 写入请求
type WriteRequest struct {
    FileID    string
    Data      []byte
    Replicas  int  // 副本数，默认3
}

// 写入结果
type WriteResult struct {
    Success   bool
    ChunkIDs  []string
    Errors    []error
}

// 写入流程
func (s *Server) HandleWrite(req *WriteRequest) (*WriteResult, error) {
    // 1. 智能选择N个节点
    nodes := s.SelectNodes(req.Replicas)
    if len(nodes) < req.Replicas {
        return nil, fmt.Errorf("可用节点不足")
    }
    
    // 2. 并行写入临时文件
    results := s.WriteToNodes(nodes, req.Data, ".tmp")
    
    // 3. 检查结果
    failedNodes := s.GetFailedNodes(results)
    
    // 4. 重试失败的分片
    for retry := 0; retry < 3; retry++ {
        if len(failedNodes) == 0 {
            break
        }
        
        // 等待重试间隔
        time.Sleep(time.Duration(retry+1) * time.Second)
        
        // 重试失败的分片
        retryResults := s.RetryFailedChunks(failedNodes, req.Data)
        failedNodes = s.GetFailedNodes(retryResults)
    }
    
    // 5. 如果还有失败，换节点重试
    if len(failedNodes) > 0 {
        newNodes := s.SelectNewNodes(failedNodes, len(failedNodes))
        retryResults := s.WriteToNodes(newNodes, req.Data, ".tmp")
        failedNodes = s.GetFailedNodes(retryResults)
    }
    
    // 6. 检查是否全部成功
    if len(failedNodes) > 0 {
        // 清理临时文件
        s.CleanupTempFiles(nodes)
        return nil, fmt.Errorf("写入失败: %d个分片无法写入", len(failedNodes))
    }
    
    // 7. 全部成功，重命名临时文件
    s.RenameChunks(nodes, ".tmp", "")
    
    return &WriteResult{Success: true}, nil
}
```

---

**文档版本**: v1.0  
**创建日期**: 2026-07-31
