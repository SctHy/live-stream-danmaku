# Maku 性能评估

基线采于 2026-11-25T12:00:15.613601Z，`git_sha: f29748d9ed7acba06cfe1b4942daceabb9f572f9`，`dirty: false`。环境为 darwin/arm64、go1.27.4，Go 可见 10 个逻辑 CPU。`full` 模式下每组预热后测量 5 轮，表中为中位数。

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

分片对照仅改变广播分片数（1 → 32），未引入停读连接。冷房 P99 差值的 95% 置信区间为 −1857 至 710 µs，未测出明确的延迟改善。踢除对照固定默认分片并加入 4 个停读连接，仅切换踢除策略。启用后踢除了这 4 个连接，发送队列满导致的丢弃从 11992 降至 32。

[原始账本](../evaluation/baseline.json) · [完整 Scorecard 与实验明细](../evaluation/report.md) · [离线图表](../evaluation/report.html)

## 阶梯容量压测

热房固定发送 240 条/秒，冷房 10 条/秒，每档持续 5 秒。下表的投递数按热房观众累加。[原始容量报告](../evaluation/capacity.json)

| 热房观众 | 热房接收数 | 热房 P99（µs） | 冷房 P99（µs） | 网关峰值 RSS（字节） |
| ---: | ---: | ---: | ---: | ---: |
| 128 | 153600 | 3380 | 5846 | 44695552 |
| 256 | 307200 | 3848 | 3561 | 51363840 |
| 512 | 614400 | 4535 | 2214 | 60243968 |
| 1024 | 1228800 | 11611 | 21886 | 80183296 |

四档的热房与冷房均无漏收、重复或游标缺口。512 到 1024 人之间出现延迟拐点，热房 P99 从 4535 升至 11611 µs，冷房从 2214 升至 21886 µs。每档冷房仅有 50 个延迟样本。压测客户端和网关共用单机，通过 loopback 通信，1024 人是本次最高测试档位，生产容量仍需在目标网络和硬件上测量。

## 3 分钟稳态与慢连接持续压测

128 名热房观众持续接收，热房每秒发送 240 条，冷房每秒 10 条。同时每 200 ms 模拟观众进出，每 15 秒引入 2 个停读连接。表中丢弃和踢除为累计计数。[原始稳态报告](../evaluation/soak.json)

| 采样时点 | 网关 RSS（字节） | goroutine 数 | 热房 P99（µs，本区间） | client_full 丢弃 | 踢除慢连接 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 首个区间（约 15 秒） | 48398336 | 332 | 3989 | 16 | 2 |
| 中点（约 90 秒） | 98238464 | 333 | 3964 | 96 | 12 |
| 结束（约 180 秒） | 102612992 | 332 | 4049 | 192 | 24 |

热房正常观众收齐 5529600 条投递，冷房收齐 1800 条，均无重复或游标缺口。RSS 前半程增长较快，后半程趋于收敛，末两次采样均为 102612992 字节。12 次采样的 goroutine 数在 332–333 之间。上述趋势仅覆盖本次 3 分钟单机运行。

在 Redis 就绪后运行以下命令，工具会构建并启动临时网关。可通过 `-duration` 延长稳态测试。

```sh
make bench-capacity ARGS='-out /tmp/capacity.json'
make bench-soak ARGS='-out /tmp/soak.json'
make bench-soak ARGS='-duration 30m -out /tmp/soak-30m.json'
```

## 复现与回归

在仓库根目录、真实 Redis 和可创建临时 schema 的 PostgreSQL 就绪后运行。下列命令保留正式基线，将本次全量采样写入候选账本，执行门禁。

```sh
export REDIS_ADDR=127.0.0.1:${REDIS_PORT:-6379}
export DATABASE_URL='postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable'
make eval-all LEDGER=evaluation/candidate.json && make eval-diff
```

`make eval-quick` 用于短测；与正式基线比较时请使用相同 profile 和负载。可用 `make eval-p0`、`make eval-p1` 分开执行。上述 eval 采样入口都接受 `-profile`、`-repeats`、`-only`、`-out`，网关实验支持 `-gateway`。例如：

```sh
go run ./cmd/bench eval all -only history_pagination,pg_archive_write -out /tmp/pg.json
go run ./cmd/bench eval report -in evaluation/candidate.json -baseline evaluation/baseline.json -html /tmp/candidate.html -md /tmp/candidate.md
go run ./cmd/bench eval diff -baseline evaluation/baseline.json -candidate evaluation/candidate.json -threshold 0.15 -out /tmp/diff.json
```

## 如何读数

各轮交替运行对照组和处理组，首轮预热不计入统计。Scorecard 的百分比表示处理中位数相对首个 arm 的变化；置信区间使用独立重采样估计中位数差，单位与指标一致。基线中位数为零时比例留空。延迟先在轮内计算分位数，再跨轮汇总。低样本标记见各轮的 `low_sample_warning`。

扇出检查热门和冷门观众有序、完整、无重复地收到消息；两组隔离实验均加入空闲房间及反复进出房的观众，踢除对照另加入停读连接，RSS 取轮内采样峰值。迁移实验用发起建连和补发请求的滑动窗口峰值衡量尖峰，同时记录源网关退出与最后交接完成的排空时间。

PG 深分页比较相同可见事件和顺序；游标组使用已知上一页边界。归档吞吐按尝试写入的行数计算，并单独验证冲突去重后的持久化行数。敏感词实验分别测量命中、未命中和词库重载。Redis 对照跨命令竞态与 Lua 原子操作，延迟代理作用于发往 Redis 的请求。直播源使用本地模拟上游进行冷、热两次并发请求，校验内容和实际调用数。

回归比较按实验、负载、arm、指标精确匹配。方向已登记的指标按相对阈值判定；吞吐下降或开销上升超过阈值时退出非零。相同值记为 NEUTRAL，零基线上的不利变化直接记为 REGRESSED。缺项默认失败，部分套件可显式选择 `-allow-missing`。候选实验的不变式失败始终阻断。
