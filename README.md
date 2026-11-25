# Maku

Maku 是一个分布式实时弹幕系统。观众可以从直播广场进入房间，发送弹幕、签到送礼或发布醒目留言。主播可以创建房间、分配房管、检索历史并查看场次数据。Gateway 提供实时广播、断线补发和业务 API，Archiver 独立消费 Kafka 消息并批量归档到 PostgreSQL。

HTTP 路由和日志直接使用 Go 标准库 `net/http.ServeMux` 和 `log/slog`。整个 Go 项目只引入 5 个核心第三方库：`gorilla/websocket`、`go-redis/v9`、`franz-go`、`pgx/v5`、`prometheus/client_golang`。

## 来源与改造范围

Maku 基于 charlesAcmen 的 `live-stream-danmaku`，沿用 MIT 许可证，保留上游版权及 SctHy 的修改署名。项目扩展了带游标、回执和批量帧的 WebSocket 协议，通过共享订阅、分片广播和有界发送队列隔离慢连接，并支持停机迁移。归档端在数据持久化后提交 Kafka 位点，网关将实时服务与 PostgreSQL 可用性解耦。前端使用纯 TypeScript 实现直播播放和断线补发，评估体系覆盖对照实验、容量与稳态压测及回归门禁。[许可证](LICENSE)

## 系统架构

```mermaid
flowchart LR
    B[浏览器] <-->|实时推送 · WebSocket| G[Gateway 网关]
    G -->|按 Redis 活跃场次写入 video_offset_ms| L[Redis Lua 脚本]
    L -->|写入最近消息队列| S[(Redis Stream)]
    L -->|发布跨节点通知| R[Redis Pub/Sub]
    R -->|共享连接订阅| G
    B -->|断线补发 · recent + 消息游标| G
    G -->|读取近期弹幕| S
    G -->|异步投递新弹幕| K[Kafka]
    K -->|按分区批量消费| A[Archiver 归档服务]
    A -->|UNNEST 批量写入| P[(PostgreSQL)]
    G -->|账号与房间权限 · 钱包与订单 · 历史检索| P
    G -->|广场分钟桶 · 场次贡献榜| Z[(Redis ZSET)]
    B -->|获取直播流地址| G
    G -->|请求直播源| U[B 站直播 API]
    W[B 站弹幕 WebSocket] -->|同直播间共享连接| LB[internal/livebridge]
    LB -->|转发实时弹幕 · 108| G
    OBS[OBS / ffmpeg] -->|RTMP| M[MediaMTX]
    M -->|HLS| B
    M -->|录制并封装 faststart MP4| V[(回放文件)]
    M -->|开播与停播回调| G
    V -->|HTTP Range 经网关| B
    P -->|分片弹幕时间轴经网关| B
    B -->|FLV / HLS 播放| CDN[视频 CDN]
```

## 核心设计说明

1. **一次 Redis 脚本调用搞定防重、限流和排序。** 房间名和场次拼成逻辑房间 ID，相关的 Redis 键统一带 `{room}` 标签，兼容 Redis Cluster 同槽执行。Lua 脚本先按访客和 `client_msg_id` 去重（10 分钟内重复提交直接返回原来的 `event_id`）；通过禁言和限流检查后，在同一次执行里把弹幕写入 Redis Stream（供断线补发）并发布到 Pub/Sub（通知其他网关广播）。广播消息直接嵌入原始 JSON 文本，避免 Lua 重新编解码导致 64 位整数（`uint64`）精度丢失。
2. **平时单条直发，高并发时自动批量合并发送。** 默认用 8 条共享 Redis 订阅连接复用所有活跃房间，32 个后台工作协程按房间分片处理，广播时使用只读的成员快照避免长时间加锁。只有当本机同一房间支持批量协议（`danmaku.batch.v1`）的观众达到 128 人，且相邻弹幕间隔 ≤ 2 ms 时，才会开启 5 ms 的合并窗口（每批最多 32 条或 32 KiB），普通客户端仍按单条接收。每个连接有独立的发送队列（默认最多 512 帧 / 256 KiB），连续 8 次写不进去就会主动断开慢连接，防止拖垮整个房间。
3. **同一套消息位置标记（游标）解决断线重连和不停服发布。** 浏览器会比对收到的 `prev` 和本地记录的游标，一旦发现漏消息，就带上 `epoch@stream-id` 调用 `recent` 接口补发（Redis Stream 保留约 256 条，120 秒无新消息后过期；游标太旧时直接拉最新快照重置）。网关收到 `SIGTERM` 停机信号后，会先把 `/readyz` 改为返回 503 让入口不再分配新连接，再分批通知在线客户端迁移（限制同时迁移人数）。客户端自动建立新连接并补收漏掉的消息，接管完成后关闭旧连接；最后网关关闭 HTTP 服务并在退出前限时刷新 Kafka 缓冲区。
4. **可安全重试的批量写库与高效历史分页。** Kafka 生产端采用 `LeaderAck`。Archiver 按分区消费后，通过 PostgreSQL 的 `UNNEST` 数组参数批量插入数据库，靠 `ON CONFLICT (event_id) DO NOTHING` 忽略重复消息。遇到网络抖动等临时错误会按随机退避自动重试；遇到数据格式或约束错误（SQLSTATE 22/23）则降级成逐条写入，把无法处理的坏数据记入死信日志和监控指标。等整批写入成功后再提交消费位点，停机时会额外等待最多 3 秒完成最后一次提交。历史弹幕查询走 `(room_id, id DESC)` 索引按 `before_id` 向下翻页，并用 `NOT EXISTS` 过滤已撤回的弹幕，避免传统 `OFFSET` 翻页越往后越慢。
5. **高性能敏感词过滤与防重复请求外部接口。** 敏感词过滤使用 Aho-Corasick 多模式匹配算法，并通过 `atomic.Pointer` 无锁读取词库；收到 `SIGHUP` 信号时在后台构建好新词库再一次性替换，未命中敏感词时做到 0 内存分配。解析 B 站直播流地址使用独立 HTTP 客户端（6 秒超时），同一房间的并发请求会自动合并成一次外部调用，避免瞬间把上游接口打爆；结果缓存上限 512 项（开播缓存 15 秒，未开播缓存 5 秒），视频流本身由浏览器直连 CDN。
6. **区分实时回执、历史归档和计数统计。** 新弹幕的 `accepted` 表示 Redis 已写入恢复流并发布广播，重复提交返回原事件 ID。此回执只确认实时接收，Kafka 异步投递在队列满、投递失败或进程崩溃时可能丢失，历史记录因此可能缺项。在线人数定期汇总，点赞数为近似最终一致统计，网关崩溃可能丢失未同步的增量，Redis 已执行但应答丢失时重试可能重复计数。
7. **按直播间共享 B 站实时弹幕连接。** 网关通过 `internal/livebridge` 缓存 B 站签名密钥和 `buvid3`，同一数字直播间只维持一条上游 WebSocket 连接，所有观众共享，最后一位观众离开后自动断开。解析时限制 zlib 解压后的最大体积防内存撑爆；B 站弹幕以消息类型 `108` 实时下发，仅用于页面展示，不计入本地恢复游标，也不写入 Kafka 和 PostgreSQL。

8. **账号与多房间授权。** Redis 保存登录会话，PostgreSQL 按房间区分房主、房管和观众。房主任免房管，房主与房管可撤回弹幕、禁言和更新公告。Redis Lua 在接收弹幕时统一执行全员禁言和慢速模式，跨网关生效。
9. **直播广场。** Redis 按分钟写入热度 ZSET，合并最近 10 个分钟桶生成滚动热榜。网关分别缓存排序索引与房间详情，只批量补取缺失的详情，用 singleflight 合并同键并发回源。
10. **虚拟积分与礼物。** 每日签到可领取积分。礼物与醒目留言在同一个 PostgreSQL 事务内创建订单、校验余额并条件扣款，以 `(sender_id, client_order_id)` 唯一约束防止重试重复扣款。订单投递后更新场次贡献榜，醒目留言按礼物档位限时置顶。
11. **场次看板与历史检索。** 看板汇总当前及峰值在线、已归档弹幕、发言人数、撤回、管控和礼物数据。历史查询可组合关键词、发言人和 `before_id` 游标，翻页时继续过滤撤回记录。
12. **OBS 直播与全量弹幕回放。** MediaMTX 接收 RTMP、提供 HLS 并录制 fMP4 分段，停播后合并为 faststart MP4。开播时网关将活跃场次写入 Redis，弹幕热路径按场次起点注入 `video_offset_ms`，无需查询 PostgreSQL。Archiver 批量归档后，回放页用 HTTP Range（206）读取视频，按 60 秒分片、用偏移量与记录 ID 的复合游标取完每片的可见弹幕，过滤已撤回记录。弹幕跟随视频时钟暂停、变速和跳转，点击时间轴上的弹幕可定位视频，登录后可在当前播放位置补发弹幕。

**PostgreSQL 离线时**，只要 Redis 正常且网关未停机，实时弹幕、断线补发和 `/readyz` 仍可用。广场回退到 Redis 中的活跃房间，已有登录会话可读取身份（余额缺省），礼物目录和已有贡献榜仍可读取。注册、登录、修改资料、房间资料与授权、钱包签到与礼物订单、历史检索、审计和场次看板等依赖数据库的读写返回 503。配置的旧版房管密钥仍可禁言和解禁单个用户。已缓存的 OBS 直播源仍可读取；推流密钥、开播鉴权及依赖场次记录的回放查询、播放与补发返回 503。数据库重连成功后，网关自动恢复业务。

## 可插拔扩展架构

- **礼物与账本**　`internal/commerce` 通过 `CatalogProvider` 提供目录，按商品的 `effect_type` 从注册表选择 `GiftEffect`。新增商品可直接配置目录；新增效果可实现 `Prepare` 并在启动时注册。签到、充值、兑换和送礼都在事务内更新余额并追加 `wallet_ledger`，以用户、业务类型和业务标识保证重复请求只记一次账。
- **支付渠道**　`PaymentProvider` 封装建单和回调验签。内置 `sandbox` 支持模拟支付或关单，`voucher` 支持兑换码入账。接入新支付渠道时，实现 `Name`、`Create`、`VerifyWebhook` 并加入渠道注册表，即可复用充值单状态流转和账本结算。
- **广场推荐**　`internal/recommend` 依次执行 Recall → Filter → Scorer → Reranker。召回阶段选出候选房间，过滤阶段按直播状态和分区筛选，评分阶段计算顺序与推荐理由，重排阶段可调整分区多样性。新增策略时注册需要的组件，再用 `RegisterStrategy` 按名称组合。前端读取策略列表，默认按用户分桶，也可手动指定策略。

## 性能测试结果

以下是核心引擎提交 `f29748d` 的实测数据，未覆盖后续账号、广场、礼物和看板接口。基线采于 2026-11-25T12:00:15.613601Z，`git_sha: f29748d9ed7acba06cfe1b4942daceabb9f572f9`，`dirty: false`。环境为 darwin/arm64、go1.27.4，Go 可见 10 个逻辑 CPU。`full` 模式下每组预热后测量 5 轮，表中为中位数。

| 实验与负载 | 对照组 → 处理组 | 指标 | 对照中位数 | 处理中位数 | Gate |
| --- | --- | --- | ---: | ---: | --- |
| 自适应批合并 high | legacy → adaptive_batch | `frames` | 313600 | 99680 | PASS |
| 热房延迟 high | legacy → adaptive_batch | `hot_p99_us` | 4385 | 9984 | PASS |
| 分片隔离 | single_shard → default_shards | `cold_p99_us` | 3741 | 3646 | PASS |
| 分片隔离 | single_shard → default_shards | `danmaku_dropped_messages_total{reason="client_full"}` | 0 | 0 | PASS |
| 分片隔离 | single_shard → default_shards | `danmaku_kicked_slow_clients_total` | 0 | 0 | PASS |
| 慢连接踢除 | kick_disabled → kick_enabled | `cold_p99_us` | 3413 | 3325 | PASS |
| 慢连接踢除 | kick_disabled → kick_enabled | `danmaku_dropped_messages_total{reason="client_full"}` | 11992 | 32 | PASS |
| 慢连接踢除 | kick_disabled → kick_enabled | `danmaku_kicked_slow_clients_total` | 0 | 4 | PASS |
| 迁移补发峰值 | crash → bounded | `recent_peak_qps` | 340 | 80 | PASS |
| 迁移排空 | crash → bounded | `drain_ms` | 453 | 1907.667 | PASS |
| 深分页 page_3600 | offset_baseline → cursor_before_id_prod | `p50_us` | 155210.083 | 383.959 | PASS |
| 归档写入 batch_128 | row_by_row_tx → multivalue_insert | `rows_per_second` | 6291.176410054454 | 93155.55362516547 | PASS |
| 归档写入 batch_128 | row_by_row_tx → unnest_prod | `rows_per_second` | 6291.176410054454 | 103448.51943428337 | PASS |
| 敏感词扫描 words_20000 | naive_contains → ac_prod | `miss_us` | 137.0575 | 1.47125 | PASS |
| Redis 去重 2ms | multi_cmd_baseline → lua_evalsha_prod | `race_duplicate_rate` | 0.21875 | 0 | PASS |
| 直播源请求合并 burst_120 | direct_no_coalesce → prod_singleflight_cache | `cold_upstream_calls` | 120 | 1 | PASS |

每批 128 行时，多值 INSERT 和 UNNEST 的吞吐分别约为逐行写入的 14.81 倍和 16.44 倍。主要收益来自批量化，两种批写方式之间相差约 11%。UNNEST 的工程价值是每批固定使用 5 个绑定参数，多值 INSERT 的参数数目随行数增长。

单机 loopback 上，512 到 1024 名热房观众之间出现延迟拐点，热房 P99 从 4535 升至 11611 µs，各档均无漏收。另一次 3 分钟持续压测中，热房正常观众收齐 5529600 条投递，累计踢除 24 个慢连接，末两次采样 RSS 持平，goroutine 全程采样为 332–333 个。[阶梯容量报告](evaluation/capacity.json) · [稳态报告](evaluation/soak.json)

```sh
export REDIS_ADDR=127.0.0.1:${REDIS_PORT:-6379}
export DATABASE_URL='postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable'
make eval-all LEDGER=evaluation/candidate.json && make eval-diff
```

[评估口径与复现说明](docs/benchmarks.md) · [完整实测报告](evaluation/report.md) · [离线可视化](evaluation/report.html)

## HTTP 与 WebSocket 接口

房间名和场次支持 1–24 位英文字母、数字、下划线或短横线。客户端需先调用 `/api/session` 获取访客会话 Cookie，再建立 WebSocket 连接。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/rooms/{room}/stream-key` | 房主读取 OBS 服务器和推流码 |
| POST | `/api/rooms/{room}/stream-key/rotate` | 房主轮换密钥，下次推流使用新码 |
| GET | `/api/rooms/{room}/stream` | 返回直播源模式、直播状态与可用播放地址 |
| GET | `/api/rooms/{room}/replays` | 本房间录制列表，按 `before_id` 翻页 |
| GET / DELETE | `/api/rooms/{room}/replays/{replay_id}` | 查询详情；房主可删除回放 |
| GET | `/api/replays/{replay_id}/video` | 读取 MP4，支持 HTTP Range |
| GET / POST | `/api/rooms/{room}/replays/{replay_id}/danmaku` | 按 `[from_ms,to_ms)` 读取，分页续传 `after_offset_ms` 和 `after_id`；登录后提交 `content` 和 `video_offset_ms` |
| POST | `/internal/stream/*` | MediaMTX 内部鉴权及开停播回调，Nginx 对外返回 403 |
| POST | `/api/session` | 创建或复用访客身份，返回 `user_id`，并通过 HttpOnly Cookie 保存会话 |
| POST | `/api/auth/register`、`/api/auth/login`、`/api/auth/logout` | 注册、登录或退出账号，登录态由 Cookie 保存 |
| GET / PATCH | `/api/auth/me` | 读取当前身份，或修改已登录账号的昵称 |
| GET | `/api/rooms` | 广场排序 `sort=hot/online/latest/recommend`，支持 `status=all/live`、`category` 分区及 `strategy` 策略；返回生效策略和各房间的推荐理由，`limit` 默认 20，上限 50 |
| POST | `/api/rooms` | 登录后提交 `slug`、`title` 建房，可附简介、B 站房号、分区 `category` 及最多五个标签 `tags` |
| GET / PATCH | `/api/rooms/{room}` | 查询资料与当前角色；房主可更新资料、当前场次、直播状态和管控设置 |
| GET / POST | `/api/rooms/{room}/moderators` | 房主或房管查看名单；房主按用户名或用户 ID 任命 |
| DELETE | `/api/rooms/{room}/moderators/{user_id}` | 房主解除任命 |
| GET | `/api/rooms/{room}/{session}/mutes` | 查看本场次禁言名单 |
| GET | `/api/rooms/{room}/{session}/audits` | 按房间查询审计记录，支持 `before_id`、`limit` |
| GET | `/api/plaza/strategies` | 查询合法分区及已注册的推荐策略 |
| GET | `/api/me/preferences` | 查看当前账号或访客的分区、标签及近期房间偏好 |
| GET | `/api/wallet/recharge-packages` | 查询充值档位、价格（分）、到账积分及可用渠道 |
| POST | `/api/wallet/recharges` | 提交 `client_order_id`、`package_code` 和可选 `provider`（默认 sandbox） |
| GET | `/api/wallet/recharges/{order_no}` | 查询本人充值单状态 |
| POST | `/api/wallet/recharges/{order_no}/simulate` | 模拟回调，传 `action=pay` 或 `close` |
| POST | `/api/payments/webhook/{provider}` | 渠道外部回调；sandbox 用原始请求体的 HMAC-SHA256 十六进制摘要校验 `X-Payment-Signature` |
| POST | `/api/wallet/redeem` | 传 `code` 兑换积分，重复领取返回 `duplicate=true` |
| GET | `/api/gifts/catalog` | 礼物编码、价格、效果类型与醒目留言置顶时长 |
| GET | `/api/wallet` | 查询余额、今日签到状态、近期礼物单、充值单和资金流水 |
| POST | `/api/wallet/checkin` | 每日签到，重复领取不增加余额 |
| GET / POST | `/api/rooms/{room}/{session}/gifts` | 按 `limit` 查近期礼物与有效置顶；下单传 `client_order_id`、`gift_code`、`quantity` 和可选留言 `message`（醒目留言必填） |
| GET | `/api/rooms/{room}/{session}/leaderboard` | 查询本场贡献榜前 20 名 |
| GET | `/api/rooms/{room}/{session}/analytics` | 房主或房管查看场次看板 |
| GET / WS | `/api/rooms/{room}/{session}/live` | WebSocket 实时通道：发送弹幕与点赞，接收弹幕、礼物、房间管控变更、统计和发送回执；支持 `?migrate=1` 迁移与 `danmaku.batch.v1` 批量协议 |
| GET | `/api/rooms/{room}/{session}/recent` | 传入 `after`（消息游标），补发断线期间漏掉的弹幕，返回 `events`、`pos`、`reset` |
| GET | `/api/rooms/{room}/{session}/history` | 按关键词 `q` 和发言人 `user_id` 可选过滤，按 ID 倒序查历史弹幕，`limit` 默认 50（上限 100），翻页传 `before_id=next_before_id` |
| POST | `/api/rooms/{room}/{session}/moderate` | 同源请求，按房间角色或配置的 Bearer 密钥授权。`action` 支持 `withdraw`（event_id）、`mute/unmute`（user_id，禁言另传 seconds）、`mute_all`（enabled）、`slow_mode`（seconds，0 关闭）、`notice`（notice） |
| GET | `/api/live/playurl?room_id=6` | 查询 B 站数字直播间开播状态与可用播放地址 |
| GET | `/livez` | 进程存活检查（Gateway 与 Archiver 均提供） |
| GET | `/readyz` | 服务就绪检查（Gateway 只检查停机状态与 Redis。PostgreSQL 离线时 Gateway 仍可启动并提供实时服务，历史查询和撤回返回 503，后台重连成功后自动恢复。Archiver 仍检查 PostgreSQL；未就绪返回 503） |
| GET | `/metrics` | Prometheus 监控指标（Gateway 复用主端口；Archiver 需通过 `-metrics-addr` 显式开启） |

## 项目结构

```text
maku/
├── cmd/
│   ├── gateway/        # 实时弹幕网关启动入口
│   ├── archiver/       # Kafka -> PostgreSQL 异步归档启动入口
│   └── bench/          # 广播压测、冷热房间隔离、双网关迁移、阶梯容量与稳态压测工具
├── internal/
│   ├── commerce/       # 礼物目录、效果与支付渠道注册表
│   ├── recommend/      # 召回、过滤、评分、重排及策略注册
│   ├── filter/         # Aho-Corasick 敏感词过滤与词库热更新
│   ├── httpapi/        # HTTP/WebSocket、账号权限、广场缓存、礼物与看板、OBS 回调、回放视频与弹幕 API、内嵌前端
│   ├── hub/            # 房间连接管理、分片广播、批量合并发送、慢连接保护与平滑迁移
│   ├── livebridge/     # B 站实时弹幕鉴权、共享连接与二进制帧解析
│   ├── metrics/        # Prometheus 监控指标定义
│   ├── mq/             # Kafka 生产者与批量消费者（含退避重试）
│   ├── protocol/       # WebSocket 消息格式与子协议常量
│   ├── pubsub/         # Redis Lua 管控、共享订阅、热度分钟桶与场次贡献榜
│   └── store/          # PostgreSQL 迁移、归档与检索、房间授权、钱包及订单事务
├── frontend/           # 直播与回放播放器，纯 TypeScript（不依赖 React/Vue，仅引入 mpegts.js / hls.js）
├── Dockerfile          # 构建网关与归档服务镜像
├── deploy/             # Nginx 负载均衡、MediaMTX 配置及录制钩子、Prometheus 采集配置
├── docs/               # 完整性能测试报告（benchmarks.md）
└── scripts/            # 浏览器级断线补发、网关迁移与回放验证脚本
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

浏览器打开 `http://127.0.0.1:8080/`，用两个标签页进入同一个房间和场次即可实时互发弹幕。输入数字房间号（如 `6`）会自动拉取 B 站直播画面与实时弹幕，可通过播放器下方的「B 站弹幕」开关控制显隐。

1. 打开「直播广场」，按热度、在线人数或最新排序。注册并登录后，展开「创建直播间」，填写房间标识和标题，选择「创建并进入」。
2. 打开「账号与钱包」签到领取积分，在房间里送礼或选择醒目留言并填写文字。可在贡献榜核对本场累计贡献。
3. 房主打开「房管与看板」，按用户名任命房管，切换全员禁言、慢速模式或更新公告。在「禁言与审计」查看操作记录，在「场次看板」查看在线、发言和礼物统计。历史面板支持按关键词及发言人筛选。

如果修改了前端源码，重新构建并重启网关即可生效：

```sh
(cd frontend && npm ci && npm run build)
```

### Nginx 双网关集群

若已启动本地网关与归档进程，先将它们停止，再运行 `make up-cluster`。浏览器通过 `http://127.0.0.1:8080/` 进入房间。Nginx 对 `/metrics` 返回 403。要查看指标，可直接访问本机端口上的 `/metrics`，其中 `8081`、`8082` 分别对应 gateway-a、gateway-b，`9091` 对应归档服务。

在集群中启用 Prometheus 时，先按 `deploy/prometheus.yml` 中的注释将采集目标改为 `gateway-a:8080`、`gateway-b:8080` 和 `archiver:9091`，再运行 `make up-monitoring`。

运行 `docker compose logs -f nginx`，多次刷新页面，通过日志中的 `upstream=` 查看各请求命中的网关。要观察连接迁移，可让多个标签页进入同一房间，在另一终端运行 `docker compose stop gateway-a`，观察原先连到 gateway-a 的客户端重连并继续收发弹幕。运行 `docker compose start gateway-a` 恢复双网关。

### OBS 推流与直播回放

1. 停止占用相同端口的本地服务，在同一终端运行以下命令。媒体环境包含双网关、归档服务、Nginx 与 MediaMTX。再次部署时沿用同一回调密钥。

   ```sh
   export STREAM_WEBHOOK_SECRET="$(openssl rand -hex 32)"
   make up-media   # docker compose --profile media --profile cluster up -d --build
   ```

2. 打开 `http://127.0.0.1:8080/`，登录并创建房间。在「房管与看板」复制服务器地址和推流码，分别填入 OBS「设置 → 直播 → 自定义」的服务器和串流密钥，开始推流。
3. 也可用带 libx264 的 ffmpeg 代替 OBS。将下方房间名和完整推流码换成自己的值，命令会发送带时间码的测试画面和音频，默认 30 秒后自动停播。`RTMP_URL` 可覆盖服务器地址。

   ```sh
   make demo-obs-stream ROOM=myroom STREAM_KEY='myroom?key=复制的密钥' SECONDS=60
   ```

4. 在直播间观看 HLS 并发几条弹幕。停止推流，待录制处理完成后，在房间打开「直播回放」并选择本场录制，可点弹幕跳转或在播放处补发。体验结束后运行 `make down` 停止全部服务，录制保留在命名卷中。

## 常用配置

| 分类 | 配置项 | 默认值与说明 |
| --- | --- | --- |
| 礼物目录 | `GIFT_CATALOG_FILE` | 可选 JSON 礼物数组文件路径，留空加载内置目录 |
| 支付回调 | `PAYMENT_WEBHOOK_SECRET` | sandbox 回调签名密钥，留空时每进程随机生成；多网关验签应配置相同密钥 |
| 兑换渠道 | `VOUCHER_CODES` | 逗号分隔的 `CODE=积分`，省略面额时为 600，默认 `MAKU2026=600` |
| OBS 推流 | `OBS_RTMP_URL` | 房主看到的服务器地址，默认 `rtmp://127.0.0.1:1935/live` |
| OBS 播放 | `OBS_HLS_BASE_URL` | HLS 根路径，默认 `/hls`，由 Nginx 转发 |
| 录制存储 | `REPLAY_STORAGE_DIR` | 本机默认 `./data/replays`，Compose 使用共享卷 `/replays` |
| 媒体回调 | `STREAM_WEBHOOK_SECRET` | 媒体环境必须设置，网关和 MediaMTX 共用；本机留空时开停播回调仅允许回环地址 |
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

GitHub Actions 在 PR 或推送到 main 时检查 Go 格式，运行 vet、竞态测试并构建三个入口，使用 Redis 和 PostgreSQL 对全部 Go 包运行集成测试。独立前端任务通过 Node 24 LTS 安装锁定依赖、执行 TypeScript 类型检查与构建，并检查内嵌目录的变更及新增文件。[CI 配置](.github/workflows/ci.yml)

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
make bench-capacity
make bench-soak
go build -o /tmp/maku-gateway ./cmd/gateway
go run ./cmd/bench migration -gateway /tmp/maku-gateway -viewers 24 -batch 4 -inflight 2
```

设置 `DANMAKU_TEST_REDIS_ADDR` 和 `DANMAKU_TEST_DATABASE_URL` 可开启 Redis/PostgreSQL 集成测试；跨网关重试测试需配置 `DANMAKU_TEST_GATEWAY_A`、`DANMAKU_TEST_GATEWAY_B` 并运行 Archiver（设置 `DANMAKU_MOD_ROOM=retry` 和对应密钥可一并测试房管分支）。浏览器端到端验证脚本见 [check-migration.cjs](scripts/check-migration.cjs) 和 [check-recovery.cjs](scripts/check-recovery.cjs)。

OBS 归档集成测试还需设置 `DANMAKU_TEST_KAFKA_BROKERS`，指向没有其他 Archiver 消费的专用 Kafka。浏览器回放验证可在安装 Playwright 和 Chromium 后运行以下命令。使用专用网关直连端口；预先在其录制目录放置至少 125 秒的测试 MP4，并沿用该网关的回调密钥。[回放验证脚本](scripts/check-replay.cjs)

```sh
GATEWAY_ORIGIN=http://127.0.0.1:8081 REPLAY_FIXTURE=test.mp4 node scripts/check-replay.cjs
```
