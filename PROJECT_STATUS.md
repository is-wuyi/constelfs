# ConstelFS 项目状态

## ✅ 已完成

### 1. 项目初始化
- [x] GitHub仓库创建: https://github.com/is-wuyi/constelfs
- [x] 项目目录结构
- [x] Go模块初始化
- [x] GitHub Actions CI配置

### 2. 核心代码框架
- [x] 中心服务器 (cmd/server, internal/server)
- [x] 存储节点代理 (cmd/node, internal/node)
- [x] 客户端 (cmd/client, internal/client)

### 3. 基础功能
- [x] 节点注册API
- [x] 节点心跳API
- [x] 节点配置API
- [x] 健康检查API
- [x] 文件列表API (占位)

### 4. 文档
- [x] README.md
- [x] 开发计划 (docs/development-plan.md)
- [x] 数据库设计 (docs/database-design.md)
- [x] 节点配置流程 (docs/node-configuration.md)
- [x] BoltDB使用指南 (docs/boltdb-guide.md)

## 🚧 进行中

### 1. 测试环境
- [ ] 中心服务器部署
- [ ] 存储节点部署
- [ ] 集成测试

## 📋 待完成

### 1. 核心存储功能
- [ ] 文件分片
- [ ] 多副本存储
- [ ] 纠删码支持
- [ ] 数据加密

### 2. 客户端功能
- [ ] 文件上传
- [ ] 文件下载
- [ ] 文件删除
- [ ] 并行传输

### 3. Web管理界面
- [ ] 节点管理
- [ ] 文件管理
- [ ] 监控告警

### 4. 协议转出
- [ ] SMB协议
- [ ] FUSE挂载
- [ ] WebDAV协议

## 🖥️ 测试环境

### 中心服务器
- **地址**: 193.134.209.37:78 (SSH), :8080 (HTTP)
- **系统**: Rocky Linux 9.7
- **用户**: root / 0044039Bb*

### 存储节点
| 节点 | 地址 | 系统 |
|------|------|------|
| NAS-001 | 27119.et.net | Synology DSM |
| NAS-002 | 27233.et.net | Synology DSM |
| NAS-003 | 27348.et.net | Synology DSM |

- **用户**: jimo / 12345678

## 📦 技术栈

| 组件 | 技术 |
|------|------|
| 后端语言 | Go 1.21 |
| 数据库 | BoltDB (bbolt) |
| API | RESTful + gRPC |
| 前端 | Vue3 + Element Plus |
| CI/CD | GitHub Actions |

## 🔗 相关链接

- **GitHub**: https://github.com/is-wuyi/constelfs
- **许可证**: GPL v3

---

**最后更新**: 2026-07-31
