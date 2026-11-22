# AGENTS.md — Maku

本文件为在 **Maku** 仓库中协作的 AI 编程代理与开发者提供架构约定、工程约束与验证流程。修改代码前请先遵循以下规则。

## 1. 项目概览与模块边界

**Maku** 是一个基于 Go 与轻量前端构建的高性能分布式实时弹幕推送引擎与异步归档系统，核心拆分为两个常驻进程与一套压测工具：

| 路径 | 职责 |
|---|---|
| `cmd/gateway/` | 网关入口：HTTP/WebSocket 服务、健康检查（`/livez`、`/readyz`）、Prometheus `/metrics`、敏感词 `SIGHUP` 热更新、优雅停机与滚动迁移 |
| `cmd/archiver/` | 归档器入口：消费 Kafka 弹幕事件并按分区批量写入 PostgreSQL，可选独立暴露 `/metrics` |
| `cmd/bench/` | 压测与验收工具：`fanout`（扇出与自适应合批）、`rooms`（冷热房隔离）、`migration`（双网关滚动交接） |
| `internal/hub/` | 内存连接管理与广播核心：分片下行 worker（`RoomShards=32`）、成员不可变快照、自适应合批（`batcher.go`）、慢连接背压踢出、滚动迁移器（`migrator.go`） |
| `internal/pubsub/` | Redis 交互层：Lua 脚本原子化校验/去重/限流/写恢复流/发布（`journal.go`）、共享订阅多路复用（`mux.go`）、在线人数与点赞统计（`stats.go`） |
| `internal/mq/` | Kafka 生产与消费层：有界缓冲区异步投递（`producer.go`）、分区批量消费与带抖动指数退避重试（`consumer.go`、`retry.go`） |
| `internal/store/` | PostgreSQL 存储层：内嵌 SQL 迁移（`migrations/*.sql` + `pg_advisory_xact_lock`）、`UNNEST` 批量幂等写入与撤回记录（`message_repo.go`） |
| `internal/httpapi/` | HTTP/WebSocket 路由、`embed.FS` 内嵌前端资源、访客 HttpOnly Cookie 会话、B 站直播源解析（`live_source.go`）、房管接口（`history.go`） |
| `internal/filter/` | 基于 Aho-Corasick 自动机的敏感词过滤引擎，支持 `reject` / `mask` 模式与原子热重载 |
| `internal/protocol/` | WebSocket 信封（`WsPacket`）、消息体（`DanmakuMessage`）及 `danmaku.batch.v1` 子协议常量 |
| `internal/metrics/` | 网关与归档器的 Prometheus 指标定义（严格控制标签基数） |
| `frontend/` | Vite + TypeScript/React 源码（构建产物直接输出到 `internal/httpapi/web/`） |
| `scripts/` | Node.js 端到端验证脚本（`check-migration.cjs`、`check-recovery.cjs`） |

## 2. 核心设计约束（修改代码时不可破坏）

1. **克制依赖与标准库优先**
   - 保持现有 5 个核心直接依赖（`gorilla/websocket`、`go-redis/v9`、`franz-go`、`pgx/v5`、`prometheus/client_golang`）。
   - HTTP 路由使用标准库 `net/http.ServeMux`，日志使用 `log/slog`，不要引入重型 Web 框架或 ORM。
2. **协议与前端同步**
   - `internal/protocol/message.go` 中的消息类型常量会由 `/assets/protocol.js` 动态导出给前端；新增或修改消息类型时，需同步检查 `internal/httpapi/chat_handler.go` 与 `frontend/src/room.js`。
   - 修改 `frontend/` 下的任何文件后，必须执行 `cd frontend && npm ci && npm run build` 更新 `internal/httpapi/web/` 内嵌构建产物。
3. **并发、幂等与精度边界**
   - `user_id` 在 Go 侧为 `uint64`，PostgreSQL 侧为 `NUMERIC(20,0)`；广播信封需保留原始 JSON，避免 64 位整数在跨端序列化时丢失精度。
   - 数据库迁移文件位于 `internal/store/migrations/`，按字典序递增编号，仅追加新迁移脚本，禁止修改已发布的旧迁移文件。
   - 保持 Kafka 投递与 PostgreSQL `UNNEST` 归档的幂等语义（基于唯一 `event_id` 去重），处理完批次后再提交消费位点。
4. **可观测性与安全性**
   - Prometheus 指标标签只允许使用预定义的低基数枚举值（如模式、拒绝原因），禁止把 `room_id`、`user_id`、`event_id` 放入标签。
   - 房管密钥校验必须使用 `crypto/subtle.ConstantTimeCompare`，并保持 `sameOrigin` 校验。
   - 优雅停机流程必须维持：`SIGTERM` -> `/readyz` 返回 503 -> 分批发送迁移邀请 -> 关闭 HTTP 与连接 -> `Drain` 刷新 Kafka 缓冲区。

## 3. 代码规范与风格要求

- **Go 代码**：
  - 提交前必须运行 `gofmt` 与 `go vet ./...`。
  - 避免忽略关键 I/O、编码或关闭操作的错误返回值；测试文件遵循 `*_test.go`（纯单测）与 `*_integration_test.go`（依赖外部中间件）分离。
- **前端代码**：
  - 优先使用 TypeScript 严格模式（`tsconfig.json` 中 `strict: true`），避免继续扩大无类型 `.js` 文件的逻辑体积。
  - 保持暗色极简视觉风格（深蓝灰基底 + 低饱和薄荷绿主色），新增控件需兼顾键盘焦点（`:focus-visible`）、响应式布局与 `prefers-reduced-motion`。
- **文档与基准数据**：
  - 任何性能数据或架构描述必须与 `docs/benchmarks.md` 和实际代码保持一致，不得编造未实测的数据。

## 4. 常用开发与验证命令

### 启动基础设施与服务

```sh
make up                  # 启动 Redis、PostgreSQL、Kafka
make up-monitoring       # 额外启动 Prometheus（默认 127.0.0.1:9090）
make run-archiver        # 启动归档器
make run-gateway         # 启动网关（默认 127.0.0.1:8080）
```

### 测试与静态检查

```sh
gofmt -l .
go vet ./...
make test                # 等价于 go test -race ./...
```

### 集成测试环境变量

```sh
# 启用 Redis 与 PostgreSQL 集成测试
DANMAKU_TEST_REDIS_ADDR=127.0.0.1:6379
DANMAKU_TEST_DATABASE_URL=postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable

# 启用双网关滚动迁移与房管集成测试
DANMAKU_TEST_GATEWAY_A=http://127.0.0.1:8080
DANMAKU_TEST_GATEWAY_B=http://127.0.0.1:8081
DANMAKU_MOD_ROOM=retry
DANMAKU_MOD_TOKEN=<your-token>
```

### 前端构建与压测验证

```sh
cd frontend && npm ci && npm run build
make bench-fanout ARGS='-viewers 160 -rate 800 -seconds 4'
go run ./cmd/bench rooms
go build -o /tmp/maku-gateway ./cmd/gateway
go run ./cmd/bench migration -gateway /tmp/maku-gateway -viewers 24
```
