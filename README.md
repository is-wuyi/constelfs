# ConstelFS (星群存储)

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

## 简介

ConstelFS 是一个分布式文件存储系统，旨在聚合全国各地的 Linux NAS 闲置存储资源，提供高可用、高性能的文件存储服务。

**名称寓意**：源自 Constellation（星座/星群），寓意散落在各地的不同 Linux 节点，就像一颗颗散落的星星，聚在一起组成星座。

## 特性

- 🔄 **分布式存储**：多节点协同工作，数据自动分布
- 🛡️ **高可用**：支持节点动态上下线，数据自动恢复
- ⚡ **高性能**：并行读写，智能调度
- 🖥️ **易管理**：Web 管理界面，可视化监控
- 📁 **多协议支持**：SMB、FUSE、WebDAV 协议转出
- 🔒 **数据安全**：传输加密 + 存储加密
- 📊 **灵活配置**：可配置副本数（2-5 副本），支持纠删码

## 架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   客户端        │    │   中心服务器    │    │   存储节点集群  │
│  (SMB/FUSE/     │◄──►│  (元数据管理)   │◄──►│  (数据存储)     │
│   WebDAV)       │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 技术栈

| 组件 | 技术选择 |
|------|----------|
| 编程语言 | Go |
| 数据库 | BoltDB (bbolt) |
| API | RESTful + gRPC |
| 前端 | Vue3 + Element Plus |
| 监控 | Prometheus + Grafana |

## 快速开始

### 前置要求

- Go 1.21+
- Linux/macOS/Windows

### 安装

```bash
# 克隆仓库
git clone https://github.com/is-wuyi/constelfs.git
cd constelfs

# 编译
go build -o constelfs-server ./cmd/server
go build -o constelfs-client ./cmd/client
go build -o constelfs-node ./cmd/node
```

### 运行

```bash
# 启动中心服务器
./constelfs-server --config config/server.yaml

# 启动存储节点
./constelfs-node --config config/node.yaml

# 使用客户端
./constelfs-client upload /path/to/file
```

## 项目结构

```
constelfs/
├── cmd/                    # 可执行文件入口
│   ├── server/            # 中心服务器
│   ├── client/            # 客户端
│   └── node/              # 存储节点代理
├── internal/              # 内部包
│   ├── server/            # 服务器核心逻辑
│   ├── client/            # 客户端核心逻辑
│   ├── node/              # 存储节点核心逻辑
│   ├── common/            # 公共组件
│   └── protocol/          # 通信协议定义
├── api/                   # API 定义
├── web/                   # Web 前端
├── docs/                  # 文档
├── scripts/               # 脚本
└── test/                  # 测试
```

## 文档

- [开发计划](docs/development-plan.md)
- [数据库设计](docs/database-design.md)
- [节点配置流程](docs/node-configuration.md)
- [API 文档](docs/api.md)

## 贡献

欢迎贡献！请阅读 [贡献指南](CONTRIBUTING.md)。

## 许可证

本项目采用 [GNU General Public License v3.0](LICENSE)。

## 致谢

- [SeaweedFS](https://github.com/seaweedfs/seaweedfs) - 架构参考
- [Garage](https://github.com/deuxfleurs-org/garage) - 设计参考
