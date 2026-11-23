# 性能测试报告

测试日期：2026-11-22。测试环境：Apple M4、macOS arm64、Go 1.27.4。网关、归档服务和压测工具运行在宿主机；Redis 7.4.12、Kafka 3.9.1 KRaft、PostgreSQL 17.12 通过本项目 Docker Compose 运行在 Colima（4 vCPU、6 GiB 内存）。下表均为同机顺序执行的单轮实测结果。

## 单网关消息广播

热门房间模拟 160 位观众，以每秒 800 条弹幕的速度持续发送 4 秒；同时另开一个冷门房间，以每秒 5 条发送，用来观察高并发房间是否会影响普通房间。发送端使用正式访客会话，每个接收端都会逐条校验消息序号、重复数和游标。两种模式下均完整送达 512,000 次热门房间消息和 20 条冷门房间消息，错序、重复和丢失均为 0。延迟从发送时刻计到采样客户端读到数据帧的时刻。

| 发送模式 | 热门房间数据帧数 | 热门房间 P50 | 热门房间 P99 | 冷门房间 P99 |
|---|---:|---:|---:|---:|
| 单条发送 | 512,000 | 1.306 ms | 11.457 ms | 26.875 ms |
| 批量合并发送 | 128,160 | 4.032 ms | 10.728 ms | 6.611 ms |

开启批量合并后，热门房间的网络写帧次数减少了 74.97%（注：冷门房间每轮仅有 20 个延迟样本）。

## 房间隔离与双网关平滑迁移

- **冷热房间隔离**：热门房间 32 位发送者各自以每秒 5 条发送，测试客户端完整收到 640 条；同时独立冷门房间收齐 20 条，延迟 P50 为 6.791 ms，P99 为 10.096 ms。
- **双网关平滑迁移**：两台网关各接入 12 位观众，每批最多迁移 4 位，同时迁移人数上限为 2 位。向源网关发送 `SIGTERM` 后，12 位观众全部收到迁移通知并完成切换；24 位观众均按序收齐 80 条消息，最后一位在 606 ms 完成交接，单人断线补收耗时 P99 为 6 ms。

## 复现命令

先运行 `make up`，再在两个终端分别启动 `make run-archiver` 和 `make run-gateway`。以下命令使用默认端口和批量参数，`migration` 命令会自动拉起两台临时网关：

```sh
go run ./cmd/bench fanout -viewers 160 -rate 800 -seconds 4 -legacy
go run ./cmd/bench fanout -viewers 160 -rate 800 -seconds 4
go run ./cmd/bench rooms
go build -o /tmp/maku-gateway ./cmd/gateway
go run ./cmd/bench migration -gateway /tmp/maku-gateway -viewers 24 -batch 4 -inflight 2
```

## 敏感词过滤

测试环境：Apple M4、macOS arm64、Go 1.27.4。词库包含 5,000 个不同的五字中文词条，每条测试输入为 100 个汉字（命中组的敏感词放在文本末尾）。计时仅包含匹配或打码过程，词库在计时前已构建完毕。

| 操作 | 单次耗时 | 内存分配（B/次） | 分配次数/次 |
|---|---:|---:|---:|
| Contains（未命中） | 1.339 µs | 0 | 0 |
| Contains（命中） | 1.344 µs | 0 | 0 |
| Mask（未命中） | 1.288 µs | 0 | 0 |
| Mask（命中） | 3.553 µs | 2,528 | 4 |

```sh
go test -bench=. -benchmem ./internal/filter
```

## 缩小互斥锁范围（移除网络 I/O 外层锁）前后对比

对比基线：`c38d848`。环境：Apple M4、darwin/arm64、Go 1.27.4、Redis 7.4.12，每项跑 3 次取中位数。房间订阅测试使用单个房间和单个 Redis 分片，计时包含订阅、等待订阅确认和退订；本地广播测试包含一个分片工作协程和每位观众的一次 channel 收发。

| 操作 | 优化前（ns/op） | 优化后（ns/op） | 优化前 → 后（B/op） | 优化前 → 后（allocs/op） |
|---|---:|---:|---:|---:|
| Redis 房间订阅与退订 | 219,347 | 234,308 | 2,136 → 2,519 | 52 → 57 |
| 本地广播调度（100 人） | 67.44 | 66.86 | 104 → 104 | 2 → 2 |
| 本地广播调度（10,000 人） | 74.58 | 73.99 | 104 → 104 | 2 → 2 |
| 本地完整广播（10,000 人） | 244,783 | 236,425 | 176 → 176 | 4 → 4 |

如需复现基线，可将当前版本的 `BenchmarkRoomSubscription` 函数复制到基线的 `internal/pubsub/mux_test.go`，启动独立 Redis 7 实例后分别执行：

```sh
DANMAKU_TEST_REDIS_ADDR=127.0.0.1:6379 go test ./internal/pubsub -run '^$' -bench=. -benchmem -count=3
go test ./internal/hub -run '^$' -bench=. -benchmem -count=3
```

## PostgreSQL 弹幕批量归档

测试环境：Apple M4、本机 PostgreSQL 17，每批写入 128 条新弹幕，跑 3 轮、每轮 100 批，取中位数。

| 批写实现 | 耗时（ms/批） | 内存分配（B/批） | 分配次数/批 |
|---|---:|---:|---:|
| 优化前（6 个绑定数组） | 1.624 | 74,600 | 965 |
| 优化后（5 个绑定数组） | 1.588 | 71,236 | 961 |

设置 `DANMAKU_TEST_DATABASE_URL` 指向可创建临时 schema 的 PostgreSQL 17 数据库后运行：

```sh
go test ./internal/store -run '^$' -bench=. -benchmem -benchtime=100x -count=3
```

对比旧版本时，可将同一份 `internal/store/message_repo_bench_test.go` 放入优化前源码中运行（每轮自动隔离 schema）。
