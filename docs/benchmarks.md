# Maku 性能评估

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

[原始账本](../evaluation/baseline.json) · [完整 Scorecard 与实验明细](../evaluation/report.md) · [离线图表](../evaluation/report.html)

## 复现与回归

在仓库根目录、真实 Redis 和可创建临时 schema 的 PostgreSQL 就绪后运行。下列命令保留正式基线，将本次全量采样写入候选账本，执行门禁。

```sh
export REDIS_ADDR=127.0.0.1:26379
export DATABASE_URL='postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable'
make eval-all LEDGER=evaluation/candidate.json && make eval-diff
```

`make eval-quick` 用于短测；与正式基线比较时请使用相同 profile 和负载。可用 `make eval-p0`、`make eval-p1` 分开执行。所有采样入口都接受 `-profile`、`-repeats`、`-only`、`-out`，网关实验支持 `-gateway`。例如：

```sh
go run ./cmd/bench eval all -only history_pagination,pg_archive_write -out /tmp/pg.json
go run ./cmd/bench eval report -in evaluation/candidate.json -baseline evaluation/baseline.json -html /tmp/candidate.html -md /tmp/candidate.md
go run ./cmd/bench eval diff -baseline evaluation/baseline.json -candidate evaluation/candidate.json -threshold 0.15 -out /tmp/diff.json
```

## 如何读数

各轮交替运行对照组和处理组，首轮预热不计入统计。Scorecard 的百分比表示处理中位数相对首个 arm 的变化；置信区间使用独立重采样估计中位数差，单位与指标一致。基线中位数为零时比例留空。延迟先在轮内计算分位数，再跨轮汇总。低样本标记见各轮的 `low_sample_warning`。

扇出检查热门和冷门观众有序、完整、无重复地收到消息；隔离实验加入停读连接、空闲房间及反复进出房的观众，RSS 取轮内采样峰值。迁移实验用发起建连和补发请求的滑动窗口峰值衡量尖峰，同时记录源网关退出与最后交接完成的排空时间。

PG 深分页比较相同可见事件和顺序；游标组使用已知上一页边界。归档吞吐按尝试写入的行数计算，并单独验证冲突去重后的持久化行数。敏感词实验分别测量命中、未命中和词库重载。Redis 对照跨命令竞态与 Lua 原子操作，延迟代理作用于发往 Redis 的请求。直播源使用本地模拟上游进行冷、热两次并发请求，校验内容和实际调用数。

回归比较按实验、负载、arm、指标精确匹配。方向已登记的指标按相对阈值判定；吞吐下降或开销上升超过阈值时退出非零。相同值记为 NEUTRAL，零基线上的不利变化直接记为 REGRESSED。缺项默认失败，部分套件可显式选择 `-allow-missing`。候选实验的不变式失败始终阻断。
