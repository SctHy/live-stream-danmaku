# 🎬 Live-Stream-Danmaku — 高性能直播弹幕后端系统

> Bilibili / YouTube 级直播弹幕系统，Go 语言从零搭建，单机支撑 **6万+ 长连接**、**150万 QPS 下行广播**。

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27.4-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Redis-Pub%2FSub-DC382D?style=flat&logo=redis&logoColor=white" alt="Redis">
  <img src="https://img.shields.io/badge/Kafka-Async%20Producer-231F20?style=flat&logo=apachekafka&logoColor=white" alt="Kafka">
  <img src="https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License">
</p>

---

## 在 Mac 上打开弹幕放映室

```sh
./scripts/dev-mac.sh --open
```

脚本复用项目所在的 Docker context 和实际映射端口，运行原生 Go 网关与消费者，在 Ardia 打开本机页面。支持按房间和场次直播、发送回执、断线补发、视频时间轴回放、禁言和撤回。静态页面和示例视频随网关分发。配置、协议与双网关实测见[macOS 和 Ardia 使用说明](docs/macos-browser.md)。

## ❓ 项目背景

直播弹幕系统看似简单，实则暗藏多个技术难点：

1. **高并发连接** — 几十万人同时在线，服务器如何抗住？
2. **广播风暴** — 1 条弹幕如何实时发给同一直播间的几万用户？
3. **削峰填谷** — 瞬时百万条弹幕，PostgreSQL 怎么撑住不崩？
4. **数据一致性** — 分布式环境下，在线人数、点赞数如何实时且准确？

本项目是对上述问题的完整工程实践。

---

## 📊 性能数据

下表为旧版在本地 WSL2 环境的历史记录。本次 KRaft 与广播、落库改造的独立对照见[实测报告](docs/kraft-benchmarks.md)。

| 指标 | 数据 |
|------|------|
| ⚡ 连接能力 | 稳定支撑 **6万+** 静态长连接（受限于本地端口/内存） |
| 🚀 吞吐极限 | 峰值下行广播达 **150万 QPS** |
| ⬆️ 上行处理 | 单机稳定处理 **2.5万 QPS** 上行请求（写入 Redis + Kafka） |
| 🛡️ 交付保障 | 极限负载下，周期丢包率 **< 1%**，系统稳定无 Error |
| 💾 落盘效率 | Kafka 消费者批量写入，**500万条积压弹幕 2 分钟内全部落盘**，平均 40k QPS |

另外实现了直播房间在线人数 + 点赞数定时广播。

---

## 🏗️ 系统架构

```
┌─────────────┐     WebSocket      ┌──────────────────────────────────────────┐
│   Client     │ ◄──────────────► │         Go Server (Gin + Gorilla WS)      │
│  (浏览器/APP) │                   │                                          │
└─────────────┘                   │  ┌─────────┐  ┌──────────┐  ┌─────────┐ │
                                  │  │ Handler  │─►│ Service  │─►│  Repo   │ │
                                  │  └────┬────┘  └──────────┘  └────┬────┘ │
                                  │       │                          │       │
                                  │       ▼                          ▼       │
                                  │  ┌─────────────────────────────────┐    │
                                  │  │        WebSocket Manager         │    │
                                  │  │  ┌───────────────────────────┐  │    │
                                  │  │  │  Register / Unregister    │  │    │
                                  │  │  │  Per-shard publish queue  │  │    │
                                  │  │  │  Rooms Map (RoomID→Clients)│ │    │
                                  │  │  │  Room-sharded workers     │  │    │
                                  │  │  │  Cached room snapshots    │  │    │
                                  │  │  └───────────────────────────┘  │    │
                                  │  └──────────┬──────────┬────────────┘    │
                                  └─────────────┼──────────┼─────────────────┘
                                                │          │
                              ┌─────────────────▼──┐  ┌────▼──────────────┐
                              │   Redis (Pub/Sub)   │  │  Kafka (Async)    │
                              │   ┌──────────────┐  │  │  ┌─────────────┐ │
                              │   │ room:{id}:   │  │  │  │ danmaku_    │ │
                              │   │   pubsub     │  │  │  │ save_topic  │ │
                              │   ├──────────────┤  │  │  └──────┬──────┘ │
                              │   │ room:{id}:   │  │  └─────────┼────────┘
                              │   │   likes      │  │            │
                              │   ├──────────────┤  │   ┌────────▼────────┐
                              │   │ room:{id}:   │  │   │ Kafka Consumer  │
                              │   │   online:{s} │  │   │ Group (Batch)   │
                              │   └──────────────┘  │   └────────┬────────┘
                              └────────────────────┘            │
                                                          ┌─────▼─────┐
                                                          │  PG 17     │
                                                          │  (pgxpool) │
                                                          └───────────┘
```

---

## 💡 技术选型与设计

### 1. 🚀 Server 层

- **Go + Gin + Gorilla WebSocket**：协程处理长 WebSocket 连接，内存占用极低，天然适合直播间用户连接
- **分层架构**：Controller → Service → Repo，依赖注入，职责清晰
- **策略模式**：`Dispatcher` + `ActionRegistry` 路由不同消息类型（弹幕 / 点赞 / 统计）

### 2. 📡 分布式消息 — Redis

- **Redis Pub/Sub**：实现多台服务器之间的弹幕实时广播，秒级送达
- **按直播间房号区分频道**：`room:{roomID}:pubsub`，天然隔离
- **Redis 原子计数**：`INCRBY` 处理点赞的超高频读写
- **心跳式在线人数**：每 3 秒各 Server 上报本机在线数（带 TTL），聚合计算全局在线人数，Server 宕机自动过期

### 3. 📈 削峰填谷 — Kafka

- **Async Producer**：独立的有界历史队列，满时记录漏存数量并继续实时发布
- **Snappy 压缩 + 批量 Flush**：减少网络请求次数，提升吞吐
- **Consumer Group + 批量落盘**：每个 Kafka 分区独立攒批，使用 PostgreSQL UNNEST 批量写入；事务成功后标记该批次的消费位点

Kafka 地址可通过 `KAFKA_BROKERS` 配置，多个地址用逗号分隔。实时策略允许已显示弹幕缺少历史记录。网关会在后台连接 Kafka，启动期间暂存最多 16,384 条待提交历史消息；进程退出会丢失内存中的待提交记录。队列满、Sarama 写入失败和 Redis 发布失败分别按 3 秒周期统计。Sarama 对可重试写错误最多重试 3 次；Redis 弹幕发布仅尝试一次，以避免超时结果不明时重复广播。Redis 发布失败时已排队的历史写入仍会继续。用 Kafka 消费组的 lag 观察尚未入库的积压。

### 4. 🛡️ 广播优化

客户端可协商 WebSocket 子协议 `danmaku.batch.v1`，收包时兼容单个对象和对象数组；仓库自带命令行客户端已支持。旧协议逐条投递。房间达到 128 位已协商观众（可调 `-batch-min-viewers`）后，间隔不超过 2 ms 的连续到达触发合批，默认最多等待 5 ms、32 条或 32 KiB，先到即发。稀疏消息直接发送。服务端参数 `-batch-window 0` 关闭合批，`-prepared=true` 开启跨连接共享预制帧。实测与故障语义见[广播与存储验证](docs/fanout-benchmarks.md)。


- **Sharding Workers**：32 个房间哈希分片，各自串行处理发布和下行分发；同分片房间共享队列
- **缓存房间成员快照**：成员变更后重建，广播间复用
- **非阻塞 safeSend**：每连接默认最多缓冲 512 个弹幕帧，待写数据上限 256 KiB，超过任一上限按包数记录丢弃。单次 socket 写超时为 5 秒，连续 8 次入队失败会断开该连接。统计帧单独保留最新值
- **读写锁分离**：`RLock` 广播遍历 + `Lock` 注册/注销，广播不阻塞连接管理

---

## 📁 项目结构

```
.
├── cmd/
│   ├── server/          # 弹幕 WebSocket 服务器入口
│   ├── consumer/        # Kafka 消费者落盘服务入口
│   ├── client/          # 交互式 CLI 客户端
│   └── benchmark/       # 压测工具（WebSocket + Kafka Producer）
│       └── kafka/       # Kafka 纯写入吞吐压测
├── internal/
│   ├── api/             # Handler 层 (Gin Controller + WebSocket Upgrade)
│   ├── service/         # Service 层 (业务逻辑)
│   ├── repo/            # Repo 层 (pgxpool 与参数化 SQL)
│   ├── model/           # 数据模型 (WsPacket / DanmakuMessage / StatsData / Like)
│   ├── ws/              # WebSocket 核心 (Manager / Client / Dispatcher / Router)
│   ├── consumer/        # Kafka 消费者 Handler (批量落盘 + 重试)
│   ├── infra/           # 基础设施 (Redis / Kafka / PostgreSQL 初始化)
│   └── logger/          # Zap 日志 (dev 彩色 / prod JSON)
├── docker-compose.yaml  # 一键启动 Redis + PostgreSQL + Kafka (KRaft)
├── go.mod
└── go.sum
```

---

## 🔌 WebSocket 协议

所有通信均使用 `WsPacket` 信封格式：

```json
{
  "type": 101,
  "room_id": "room_0",
  "data": { ... }
}
```

| type | 常量 | 方向 | data 内容 |
|------|------|------|-----------|
| 101 | `TypeDanmaku` | C→S / S→C | `{"room_id":"...", "user_id":123, "content":"hello", "send_time":"..."}` |
| 102 | `TypeStats` | S→C | `{"online":50000, "likes":12000}` |
| 103 | `ActionLike` | C→S | `{"count":1}` |

---

## 🚀 快速开始

### 前置依赖

- Go 1.27.4+
- Docker & Docker Compose

### 1. 启动基础设施

```bash
docker compose up -d
```

将启动 Redis (6379)、PostgreSQL (5432)、单节点 Kafka KRaft (9092)。宿主机应用连接 127.0.0.1:9092，同一 Compose 网络内连接 kafka:29092。Kafka 数据和消费组位点保存在 kafka_data 命名卷。旧 ZooKeeper 部署如有待保留的日志或位点，应先按 [Kafka 元数据迁移说明](https://kafka.apache.org/39/operations/kraft/) 完成迁移；新卷启动的是空集群。

如果本机 6379 已占用，可以用 `REDIS_PORT=26379 docker compose up -d`，并在运行弹幕服务时设置 `REDIS_ADDR=127.0.0.1:26379`。

PostgreSQL 使用命名卷 `postgres_data` 保存历史记录。默认连接为 `postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable`。连接其他实例时，请为服务端和消费者设置同一个 `DATABASE_URL`。两个进程启动时通过 pgxpool 在事务锁内执行尚未登记的版本化 SQL，版本记录保存在 `danmaku_schema_migrations`。基线迁移可接管既有表，历史记录允许 event_id 为空。

可用以下命令进入数据库练习 SQL：

```bash
docker compose exec postgres psql -U danmaku -d danmaku_db
```

### 2. 启动弹幕服务器

```bash
go run cmd/server/main.go -port 8080
```

### 3. 启动 Kafka 消费者（落盘服务）

```bash
go run cmd/consumer/main.go
```

### 4. 连接客户端

```bash
# 交互式 CLI 客户端
go run cmd/client/main.go -uid 1001 -room 1001 -port 8080

# 输入文字发送弹幕，输入 1 发送点赞
```

### 5. 压测

```bash
# WebSocket 压测（默认 30000 连接）
go run cmd/benchmark/main.go -c 60000 -r 1s

# Kafka 纯写入吞吐压测
go run cmd/benchmark/kafka/kafka.go
```

---

## 🔧 关键配置参数

### Manager 常量 (`internal/ws/manager.go`)

| 参数 | 值 | 说明 |
|------|----|------|
| `BroadCastInterVal` | 3s | 统计广播间隔 |
| `BroadcastChanSize` | 1024 | 发布缓冲总预算，各分片均分 |
| `BroadcastJobChanSize` | 1000 | 下行任务缓冲总预算，均分后向上取整 |
| `RoomShards` | 32 | 发布和分发的房间分片数 |

### Client 常量 (`internal/ws/client.go`)

| 参数 | 值 | 说明 |
|------|----|------|
| `SendChanSize` | 512 | 单客户端发送缓冲，可用服务端参数 `-send-queue 128` 等覆盖 |

### Kafka Producer 常量 (`internal/infra/kafka.go`)

| 参数 | 值 | 说明 |
|------|----|------|
| `ProducerChanBufSize` | 16384 | Sarama Channel 缓冲 |
| `Flush.Messages` | 50 | 批量消息数阈值 |
| `Flush.Frequency` | 50ms | 批量等待时间阈值 |
| `Compression` | Snappy | 压缩算法 |

### Consumer 常量 (`internal/consumer/handler.go`)

| 参数 | 值 | 说明 |
|------|----|------|
| `FlushInterval` | 2s | DB 刷盘间隔 |
| 写入重试间隔 | 1s | 当前分区等待数据库恢复；消费会话结束时释放分区 |

---

## 🧩 核心流程

### 弹幕发送流程

```
Client ──WS──► Handler ──► Router (TypeDanmaku)
                                │
                    ┌───────────┼───────────┐
                    ▼                       ▼
            Kafka Async Producer      Redis Pub/Sub
            (持久化缓冲)              (跨服务器广播)
                    │                       │
                    ▼                       ▼
            Kafka Cluster          其他 Server 的
                                       subscribeToRoom
                                           │
                                           ▼
                                  broadcastToLocalClients
                                           │
                                           ▼
                                  房间分片 Worker ──► Client.Send
```

### 在线人数 / 点赞数广播流程

```
Manager (每3秒 Ticker)
    │
    ├─ 1. 本地点赞批量聚合 → Redis INCRBY
    ├─ 2. 上报本机在线数 (SETEX + TTL) → Redis Pipeline
    ├─ 3. 聚合全局在线数 (SMembers + MGet)
    ├─ 4. 获取全局点赞数 (GET)
    │
    ▼
broadcastToLocalClients (TypeStats)
    │
    ▼
所有本房间客户端收到 {"online": N, "likes": M}
```

---

## 📌 技术亮点

- **信封协议设计**：`WsPacket` + `json.RawMessage` 延迟解析，路由层无需反序列化内部数据
- **历史幂等**：服务端为弹幕生成 event_id，旧格式消息使用 Kafka 主题、分区和位点生成幂等键。
- **消费恢复**：会话结束后从已提交位点继续，重复批次通过 PostgreSQL 唯一索引去重。
- **Kafka 背压**：数据库持续写入失败时暂停该分区推进，待修复后重试。
- **Server 心跳**：Redis Key 带 TTL，Server 宕机后自动清理，在线数自动修正

---

## 🛣️ 未来规划

- [ ] JWT 鉴权接入
- [ ] 配置文件化（Viper / YAML）
- [ ] Kubernetes 部署 + HPA 弹性伸缩
- [ ] 弹幕敏感词过滤
- [ ] WebSocket 压缩扩展 (permessage-deflate)
- [ ] gRPC 内部通信
- [ ] Prometheus + Grafana 监控

---

## 👤 作者

**charlesAcmen**

- 📕 小红书：[charlesAcmen](https://www.xiaohongshu.com/user/profile/60b9ea030000000001006c68)
- 🐙 GitHub：[charlesAcmen](https://github.com/charlesAcmen)

---

## 📄 开源协议

本项目基于 [MIT License](LICENSE) 开源，欢迎 Star、Fork 和 PR！
