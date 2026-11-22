本文件是本仓库的硬性工程规范。任何改动（新功能、重构、修 Bug）都必须严格遵守；若需求与下列约束冲突，先提出权衡，禁止擅自突破。

## 1. 技术栈与依赖白名单

- **中间件栈固定**：Redis 7、Apache Kafka（KRaft）、PostgreSQL 17。不引入其他中间件。
- **Go 直接依赖白名单**：
  - 标准库：`net/http`（Go 1.22+ `ServeMux`）、`log/slog`、`embed`、`sync/atomic`
  - 第三方：`github.com/gorilla/websocket`、`github.com/redis/go-redis/v9`、`github.com/twmb/franz-go`、`github.com/jackc/pgx/v5`、`github.com/prometheus/client_golang`
- **禁止引入**：`gin`、`zap`、`sarama`、`viper`、任何 ORM、任何依赖注入框架、gRPC/Protobuf、重型规则/正则库。新增第三方依赖前必须先征得确认。

## 2. 架构不变量（严禁破坏）

1. **单一主链路**：所有弹幕上行必须经过同一条校验 → 过滤 → Redis 原子发布 → Kafka 异步归档流水线；禁止为 CLI/压测维护第二套并行内核。
2. **零包级可变全局变量**：禁止全局 logger、全局 registry、全局 Prometheus `DefaultRegisterer` 或全局计数器。所有依赖（`*slog.Logger`、store、metrics、filter）通过构造函数显式注入。
3. **Redis Cluster 兼容与高精度安全**：
   - 所有单房间 Key 必须使用统一 Hash Tag `dm:{<room>}:*`，禁止在多 Key Lua 脚本中混入全局 Key。
   - 禁止在 Lua 中用 `string.sub` 拼接 JSON，也禁止用 `cjson` 编解码 64 位 `user_id`（避免浮点精度丢失）；保持外层信封透传原始 JSON。
4. **连接与并发红线**：
   - Redis 订阅保持 $O(1)$ 固定分片多路复用（`internal/pubsub/mux.go`），禁止按房间新建独立连接。
   - **严禁在持有 `sync.Mutex` / `sync.RWMutex` 期间执行网络 I/O**（Redis/Kafka/PG/Socket 读写）。
   - 单连接保持帧数 + 字节双上限背压、连续 8 次入队失败踢出及 Ping/Pong 保活。
5. **归档语义**：`cmd/archiver` 只做纯粹的 Kafka → PostgreSQL 分区批量幂等写（`UNNEST ... ON CONFLICT DO NOTHING`），事务成功后手动提交位点；瞬态错误指数退避，SQLSTATE `22`/`23` 毒丸逐条隔离，绝不阻塞分区。
6. **低基数可观测性**：严禁把 `room_id`、`session`、`user_id` 作为 Prometheus Label。

## 3. 反过度工程化规则（Anti-Slop）

- **做加法前先做减法**：替换旧实现时必须同步删除旧代码、旧 flag、旧表和旧测试，禁止以“向后兼容”名义保留死分支。
- **禁止伪抽象**：
  - 不建空壳 `service` 层；
  - 不为单一实现定义 `interface`（仅在单测需要 mock 的最小边界定义）；
  - 不为 2–3 个枚举分支引入全局注册表或策略工厂，直接用 `switch`；
  - 不保留实测无效或默认关闭的实验性开关（如已删除的 `PreparedMessage`）。
- **控制文件体量**：单个 Go 源文件尽量控制在 200 行以内，拒绝超过 300 行的上帝对象。
- **不污染仓库**：不提交大体积二进制（视频/图片）、不把个人本机工具链写进仓库脚本、不为每次小改新建零散的 `.md` 报告或独立压测目录（压测统一收口在 `cmd/bench`）。

## 4. 目录归属

- `internal/protocol`：协议常量（`101`–`107`）与编解码，禁止在其他包散落魔法数字。
- `internal/hub`：房间、连接、合批、迁移、上行路由。
- `internal/pubsub`：Redis 复用订阅、单房间恢复流、统计聚合。
- `internal/mq`：`franz-go` 生产者、分区消费者、退避与毒丸隔离。
- `internal/store`：`pgxpool`、内嵌 SQL 迁移、回放与撤回查询。
- `internal/filter`：零依赖 AC 自动机敏感词过滤与无锁热更新。
- `internal/metrics`：独立 Prometheus Registry 与指标定义。
- `internal/httpapi`：标准库 HTTP 路由、访客会话、轻量 Demo 静态页。

## 5. 交付前强制验收

每次修改完成前必须全部运行并通过，禁止跳过：

```sh
go mod tidy
go vet ./...
go test -race ./...
```

若改动涉及热路径（广播合批、Pub/Sub、敏感词过滤、批量落库），必须附带可复现的 `go test -bench=. -benchmem` 对比结果并同步更新 `docs/benchmarks.md` 与 `README.md`。
