---
description: 前后端技术栈与后端类DDD分层架构介绍
title: 基本架构
---

# 基本架构

## 一、前后端技术栈

### 前端

| 技术 | 版本/说明 |
|------|-----------|
| 框架 | Vue 3.5（Composition API + `<script setup>`） |
| 构建工具 | Vite 8 |
| UI 组件库 | Element Plus 2.13 |
| 路由 | Vue Router 4 |
| HTTP 客户端 | Axios |
| 视频播放 | hls.js 1.6（支持 m3u8 流媒体播放） |
| 图标 | @element-plus/icons-vue |

### 后端

| 技术 | 版本/说明 |
|------|-----------|
| 语言 | Go |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | MySQL 8（utf8mb4） |
| 缓存 | Redis（含 RedisBloom Cuckoo Filter） |
| 消息队列 | Kafka（segmentio/kafka-go） |
| 对象存储 | MinIO |
| 搜索引擎 | Elasticsearch 8 |
| 视频处理 | FFmpeg / FFprobe |
| ID 混淆 | Hashids |
| 配置管理 | Viper |

---

## 二、后端分层架构

后端采用**四层架构**核心思想：**领域层不依赖基础设施层**，通过接口（端口）实现依赖倒置。

```
backend/
├── cmd/api/main.go          # 服务入口：依赖注入、路由注册、中间件挂载、后台任务启动
├── handler/                 # 用户接口层
│   └── middleware/          # 认证中间件、Beta Token 校验中间件
├── application/             # 应用层
├── domain/                  # 领域层
│   └── ports.go             # 领域端口契约（接口定义）
├── infra/                   # 基础设施层
│   ├── cache/               # Redis 缓存实现（Cuckoo Filter、计数器缓存、视频/用户缓存）
│   ├── elasticsearch/       # ES 客户端与索引管理
│   ├── hashids/             # Hashids 编解码
│   ├── media/               # 视频处理（转码器、封面生成器、视频分析器）
│   ├── messaging/           # Kafka 生产者与消费者
│   ├── minio/               # MinIO 服务封装
│   ├── persistence/         # DAO（GORM 数据访问）
│   ├── redis/               # Redis 客户端
│   ├── storage/             # VideoStorage 端口实现（基于 MinIO）
│   ├── tools/               # ffmpeg/ffprobe 命令行封装
│   └── workspace/           # 临时工作空间管理
├── config/                  # 基础设施配置加载（DB、Redis、MinIO、Kafka、ES、Viper）
└── pkg/                     # 公共工具包（统一响应、时间工具等）
```

---

## 三、各层级职责

### 1. 用户接口层（`handler/`）

- 接收 HTTP 请求，解析路由参数和 Query
- 参数校验（`ShouldBindJSON`、`ShouldBindQuery`）
- 调用应用层服务，将结果包装为统一 JSON 响应
- 挂载中间件（JWT 认证、Beta Token 全局校验）


### 2. 应用层（`application/`）

- **流程编排**：串联多个领域操作完成一个完整用例（如：创建视频 → 生成预签名 URL → 投递 Kafka 事件）
- **事务控制**：跨多个 DAO 操作的事务管理
- **DTO 转换**：将领域实体转为对外输出结构
- **跨领域协调**：如点赞时同时更新计数器缓存和发送 Kafka 通知


### 3. 领域层（`domain/`）

- **实体定义**：Video、User、Comment 等核心业务对象
- **值对象**：不可变的数据组合
- **状态机方法**：如 `video.PreparePublish()` 控制视频状态流转
- **领域端口契约**：`domain/ports.go` 中定义 `VideoStorage`、`VideoAnalyzer`、`CoverGenerator`、`SearchIndexer` 等接口

> 领域层**只定义契约，不依赖任何具体技术实现**（不 import MinIO、Redis、Kafka 等包）。

### 4. 基础设施层（`infra/`）

- 实现领域层定义的端口接口
- 封装所有技术细节：MySQL DAO、Redis 缓存、Kafka 消息、MinIO 存储、FFmpeg 转码
- 提供可替换的实现（如未来想把 MinIO 换成阿里云 OSS，只需新增一个 `infra/storage/oss_storage.go` 实现 `VideoStorage` 接口）

