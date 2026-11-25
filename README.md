# Maku

Maku 是一个分布式实时弹幕系统，拆分为两个独立服务：**Gateway（网关）** 负责 WebSocket 实时广播和断线补发，**Archiver（归档服务）** 负责从 Kafka 消费弹幕并批量写入 PostgreSQL。这样可以把“在线观众收到弹幕的速度”和“历史数据写数据库的耗时”拆开，互相不拖慢。

HTTP 路由和日志直接使用 Go 标准库 `net/http.ServeMux` 和 `log/slog`。整个 Go 项目只引入 5 个核心第三方库：`gorilla/websocket`、`go-redis/v9`、`franz-go`、`pgx/v5`、`prometheus/client_golang`。

## 系统架构

```mermaid
flowchart LR
    B[浏览器] <-->|实时推送 · WebSocket| G[Gateway 网关]
    G -->|发送弹幕| L[Redis Lua 脚本]
    L -->|写入最近消息队列| S[(Redis Stream)]
    L -->|发布跨节点通知| R[Redis Pub/Sub]
    R -->|共享连接订阅| G
    B -->|断线补发 · recent + 消息游标| G
    G -->|读取近期弹幕| S
    G -->|异步投递新弹幕| K[Kafka]
    K -->|按分区批量消费| A[Archiver 归档服务]
    A -->|UNNEST 批量写入| P[(PostgreSQL)]
    G -->|查询历史弹幕 · 游标分页| P
    B -->|获取直播流地址| G
    G -->|请求直播源| U[B 站直播 API]
    W[B 站弹幕 WebSocket] -->|同直播间共享连接| LB[internal/livebridge]
    LB -->|转发实时弹幕 · 108| G
    B -->|FLV / HLS 播放| CDN[视频 CDN]
```

## 核心设计说明

1. **一次 Redis 脚本调用搞定防重、限流和排序。** 房间名和场次拼成逻辑房间 ID，相关的 Redis 键统一带 `{room}` 标签，兼容 Redis Cluster 同槽执行。Lua 脚本先按访客和 `client_msg_id` 去重（10 分钟内重复提交直接返回原来的 `event_id`）；通过禁言和限流检查后，在同一次执行里把弹幕写入 Redis Stream（供断线补发）并发布到 Pub/Sub（通知其他网关广播）。广播消息直接嵌入原始 JSON 文本，避免 Lua 重新编解码导致 64 位整数（`uint64`）精度丢失。
2. **平时单条直发，高并发时自动批量合并发送。** 默认用 8 条共享 Redis 订阅连接复用所有活跃房间，32 个后台工作协程按房间分片处理，广播时使用只读的成员快照避免长时间加锁。只有当本机同一房间支持批量协议（`danmaku.batch.v1`）的观众达到 128 人，且相邻弹幕间隔 ≤ 2 ms 时，才会开启 5 ms 的合并窗口（每批最多 32 条或 32 KiB），普通客户端仍按单条接收。每个连接有独立的发送队列（默认最多 512 帧 / 256 KiB），连续 8 次写不进去就会主动断开慢连接，防止拖垮整个房间。
3. **同一套消息位置标记（游标）解决断线重连和不停服发布。** 浏览器会比对收到的 `prev` 和本地记录的游标，一旦发现漏消息，就带上 `epoch@stream-id` 调用 `recent` 接口补发（Redis Stream 保留约 256 条，120 秒无新消息后过期；游标太旧时直接拉最新快照重置）。网关收到 `SIGTERM` 停机信号后，会先把 `/readyz` 改为返回 503 让入口不再分配新连接，再分批通知在线客户端迁移（限制同时迁移人数）。客户端自动建立新连接并补收漏掉的消息，接管完成后关闭旧连接；最后网关关闭 HTTP 服务并把 Kafka 缓冲区里的消息全部发完再退出。
4. **可安全重试的批量写库与高效历史分页。** Kafka 生产端采用 `LeaderAck`。Archiver 按分区消费后，通过 PostgreSQL 的 `UNNEST` 数组参数批量插入数据库，靠 `ON CONFLICT (event_id) DO NOTHING` 忽略重复消息。遇到网络抖动等临时错误会按随机退避自动重试；遇到数据格式或约束错误（SQLSTATE 22/23）则降级成逐条写入，把无法处理的坏数据记入死信日志和监控指标。等整批写入成功后再提交消费位点，停机时会额外等待最多 3 秒完成最后一次提交。历史弹幕查询走 `(room_id, id DESC)` 索引按 `before_id` 向下翻页，并用 `NOT EXISTS` 过滤已撤回的弹幕，避免传统 `OFFSET` 翻页越往后越慢。
5. **高性能敏感词过滤与防重复请求外部接口。** 敏感词过滤使用 Aho-Corasick 多模式匹配算法，并通过 `atomic.Pointer` 无锁读取词库；收到 `SIGHUP` 信号时在后台构建好新词库再一次性替换，未命中敏感词时做到 0 内存分配。解析 B 站直播流地址使用独立 HTTP 客户端（6 秒超时），同一房间的并发请求会自动合并成一次外部调用，避免瞬间把上游接口打爆；结果缓存上限 512 项（开播缓存 15 秒，未开播缓存 5 秒），视频流本身由浏览器直连 CDN。
6. **区分实时回执、历史归档和计数统计。** 客户端收到 `accepted` 表示弹幕已写入 Redis 并开始广播；写入 Kafka 属于异步步骤，极端情况下（进程突然崩溃或发送缓冲区满）可能丢失少量历史记录。在线人数和点赞数先在网关内存里汇总，再定期同步到 Redis 合并，因此页面展示的是最终一致的统计值。
7. **按直播间共享 B 站实时弹幕连接。** 网关通过 `internal/livebridge` 缓存 B 站签名密钥和 `buvid3`，同一数字直播间只维持一条上游 WebSocket 连接，所有观众共享，最后一位观众离开后自动断开。解析时限制 zlib 解压后的最大体积防内存撑爆；B 站弹幕以消息类型 `108` 实时下发，仅用于页面展示，不计入本地恢复游标，也不写入 Kafka 和 PostgreSQL。

## 性能测试结果

测试环境为 darwin/arm64、go1.27.4，Go 可见 10 个逻辑 CPU。profile 为 `full`，每个 arm 预热后保留 5 轮，表中为各轮中位数。

| 实验与负载 | 对照组 → 处理组 | 指标 | 对照中位数 | 处理中位数 | Gate |
| --- | --- | --- | ---: | ---: | --- |
| 自适应批合并 high | legacy → adaptive_batch | frames | 313600 | 103712 | PASS |
| 慢连接隔离  | baseline_single_shard_unbounded → protected_default | rss_peak_bytes | 70189056 | 48103424 | PASS |
| 双网关迁移  | crash → bounded | recent_peak_qps | 300 | 80 | PASS |
| 迁移排空  | crash → bounded | drain_ms | 454 | 1905.344 | PASS |
| 深分页 page_3600 | offset_baseline → cursor_before_id_prod | p50_us | 162105.792 | 552.792 | PASS |
| 归档写入 batch_128 | row_by_row_tx → unnest_prod | rows_per_second | 4489.995356916848 | 80562.93349781599 | PASS |
| 敏感词扫描 words_20000 | naive_contains → ac_prod | miss_us | 137.23083 | 1.31166 | PASS |
| Redis 去重 2ms | multi_cmd_baseline → lua_evalsha_prod | race_duplicate_rate | 0.21875 | 0 | PASS |
| 直播源请求合并 burst_120 | direct_no_coalesce → prod_singleflight_cache | cold_upstream_calls | 120 | 1 | PASS |

```sh
export REDIS_ADDR=127.0.0.1:26379
export DATABASE_URL='postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable'
make eval-all LEDGER=evaluation/candidate.json && make eval-diff
```

[评估口径与复现说明](docs/benchmarks.md) · [完整实测报告](evaluation/report.md) · [离线可视化](evaluation/report.html)

## HTTP 与 WebSocket 接口

房间名和场次支持 1–24 位英文字母、数字、下划线或短横线。客户端需先调用 `/api/session` 获取访客会话 Cookie，再建立 WebSocket 连接。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/session` | 创建或复用访客身份，返回 `user_id`，并通过 HttpOnly Cookie 保存会话 |
| GET / WS | `/api/rooms/{room}/{session}/live` | WebSocket 实时通道：发送弹幕与点赞，接收广播、统计和发送回执；支持 `?migrate=1` 迁移与 `danmaku.batch.v1` 批量协议 |
| GET | `/api/rooms/{room}/{session}/recent` | 传入 `after`（消息游标），补发断线期间漏掉的弹幕，返回 `events`、`pos`、`reset` |
| GET | `/api/rooms/{room}/{session}/history` | 按 ID 倒序查历史弹幕，`limit` 默认 50（上限 100），翻页传 `before_id=next_before_id` |
| POST | `/api/rooms/{room}/{session}/moderate` | 房管操作（校验同源、房间权限和 Bearer 密钥）：按 `event_id` 撤回弹幕，或按 `user_id` 禁言指定秒数 |
| GET | `/api/live/playurl?room_id=6` | 查询 B 站数字直播间开播状态与可用播放地址 |
| GET | `/livez` | 进程存活检查（Gateway 与 Archiver 均提供） |
| GET | `/readyz` | 服务就绪检查（Gateway 检查停机状态与 Redis；Gateway 在 PostgreSQL 故障期间继续提供实时服务，历史查询和撤回返回 503，数据库恢复后自动重试；Archiver 检查 PostgreSQL；未就绪返回 503） |
| GET | `/metrics` | Prometheus 监控指标（Gateway 复用主端口；Archiver 需通过 `-metrics-addr` 显式开启） |

## 项目结构

```text
maku/
├── cmd/
│   ├── gateway/        # 实时弹幕网关启动入口
│   ├── archiver/       # Kafka -> PostgreSQL 异步归档启动入口
│   └── bench/          # 广播压测、冷热房间隔离、双网关迁移测试工具
├── internal/
│   ├── filter/         # Aho-Corasick 敏感词过滤与词库热更新
│   ├── httpapi/        # HTTP/WebSocket 路由、访客会话、直播源解析与内嵌前端静态文件
│   ├── hub/            # 房间连接管理、分片广播、批量合并发送、慢连接保护与平滑迁移
│   ├── livebridge/     # B 站实时弹幕鉴权、共享连接与二进制帧解析
│   ├── metrics/        # Prometheus 监控指标定义
│   ├── mq/             # Kafka 生产者与批量消费者（含退避重试）
│   ├── protocol/       # WebSocket 消息格式与子协议常量
│   ├── pubsub/         # Redis Lua 脚本、共享订阅管理与在线人数/点赞统计
│   └── store/          # PostgreSQL 连接池、内嵌 SQL 迁移与 UNNEST 批量写库
├── frontend/           # 纯 TypeScript 前端（不依赖 React/Vue，仅引入 mpegts.js / hls.js）
├── Dockerfile          # 构建网关与归档服务镜像
├── deploy/             # Nginx 负载均衡与 Prometheus 采集配置
├── docs/               # 完整性能测试报告（benchmarks.md）
└── scripts/            # 浏览器级断线补发与网关迁移验证脚本
```

## 快速开始

需要本地安装 Go 1.27.4、Docker Compose 和 Make。仓库已内嵌编译好的前端静态文件，启动依赖后直接运行两个 Go 进程即可体验：

```sh
make up
make run-archiver    # 终端 1：启动归档服务
```

```sh
make run-gateway     # 终端 2：启动弹幕网关
```

浏览器打开 `http://127.0.0.1:8080/`，用两个标签页进入同一个房间和场次即可实时互发弹幕。输入数字房间号（如 `6`）会自动拉取 B 站直播画面与实时弹幕，可通过播放器下方的「B 站弹幕」开关控制显隐。如果修改了前端源码，重新构建并重启网关即可生效：

```sh
(cd frontend && npm ci && npm run build)
```

### Nginx 双网关集群

若已启动本地网关与归档进程，先将它们停止，再运行 `make up-cluster`。浏览器通过 `http://127.0.0.1:8080/` 进入房间。Nginx 对 `/metrics` 返回 403。要查看指标，可直接访问本机端口上的 `/metrics`，其中 `8081`、`8082` 分别对应 gateway-a、gateway-b，`9091` 对应归档服务。

在集群中启用 Prometheus 时，先按 `deploy/prometheus.yml` 中的注释将采集目标改为 `gateway-a:8080`、`gateway-b:8080` 和 `archiver:9091`，再运行 `make up-monitoring`。

运行 `docker compose logs -f nginx`，多次刷新页面，通过日志中的 `upstream=` 查看各请求命中的网关。要观察连接迁移，可让多个标签页进入同一房间，在另一终端运行 `docker compose stop gateway-a`，观察原先连到 gateway-a 的客户端重连并继续收发弹幕。运行 `docker compose start gateway-a` 恢复双网关。

## 常用配置

| 分类 | 配置项 | 默认值与说明 |
| --- | --- | --- |
| 中间件 | `REDIS_ADDR` / `KAFKA_BROKERS` | `127.0.0.1:6379` / `127.0.0.1:9092`（多个地址用逗号分隔） |
| 数据库 | `DATABASE_URL` | 本机 PostgreSQL 5432 端口的 `danmaku_db`，默认账号密码与 Compose 一致 |
| 房管 | `DANMAKU_MOD_ROOM` / `DANMAKU_MOD_TOKEN` | 同时配置房间名和密钥后可调用房管接口 |
| 网关监听 | `-host` / `-port` | `127.0.0.1` / `8080` |
| B 站弹幕 | `-bilibili-danmaku` | 默认 `true`，数字房间自动接入 B 站实时弹幕 |
| 广播参数 | `-pubsub-shards` / `-room-shards` / `-send-queue` | 8 条订阅连接 / 32 个广播分片 / 每连接队列上限 512 帧 |
| 批量发送 | `-batch-min-viewers` / `-batch-arrival-gap` / `-batch-window` | 128 人 / 2 ms / 5 ms |
| 平滑迁移 | `-migration-batch` / `-migration-inflight` / `-migration-timeout` | 每 100 ms 最多邀请 100 人 / 同时迁移上限 64 人 / 超时 30 秒 |
| 敏感词 | `-sensitive-words` / `-filter-mode` | 默认不开启；词库文件每行一词，支持 `reject`（拦截，默认）和 `mask`（打码） |
| 本地测试 | `-allow-dev-guest` | 默认关闭；开启后允许本机无 Origin 的压测客户端用 `uid` 直接入房 |
| 归档监控 | `-metrics-addr` | 默认关闭；示例：`make run-archiver ARGS='-metrics-addr 127.0.0.1:9091'` |
| 日志格式 | `-log-format` | 支持 `json`（默认）和 `text` |

## 监控与测试

网关暴露活跃连接数、活跃房间数、单条与批量发送帧数、分片排队耗时、队列丢弃数、慢连接断开数、拦截数及 Kafka 投递失败数；归档服务暴露写入行数、批写耗时与死信数。所有指标标签均使用固定枚举值，避免标签数量无限增长。

```sh
make up-monitoring
curl -fsS http://127.0.0.1:8080/metrics
```

Prometheus 默认监听 `127.0.0.1:9090`（可用 `PROMETHEUS_PORT` 修改），配置文件见 [deploy/prometheus.yml](deploy/prometheus.yml)。

```sh
gofmt -l .
go vet ./...
make test            # go test -race ./...

# 启动中间件与服务后运行压测（migration 会自动拉起两台临时网关）
make bench-fanout ARGS='-viewers 160 -rate 800 -seconds 4'
go run ./cmd/bench rooms
go build -o /tmp/maku-gateway ./cmd/gateway
go run ./cmd/bench migration -gateway /tmp/maku-gateway -viewers 24 -batch 4 -inflight 2
```

设置 `DANMAKU_TEST_REDIS_ADDR` 和 `DANMAKU_TEST_DATABASE_URL` 可开启 Redis/PostgreSQL 集成测试；跨网关重试测试需配置 `DANMAKU_TEST_GATEWAY_A`、`DANMAKU_TEST_GATEWAY_B` 并运行 Archiver（设置 `DANMAKU_MOD_ROOM=retry` 和对应密钥可一并测试房管分支）。浏览器端到端验证脚本见 [check-migration.cjs](scripts/check-migration.cjs) 和 [check-recovery.cjs](scripts/check-recovery.cjs)。
