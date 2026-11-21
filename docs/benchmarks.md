# 重构后压测

2026-11-22，Apple M4，macOS arm64，Go 1.27.4。网关、归档器和压测客户端运行在宿主机；Redis 7.4.12、Kafka 3.9.1 KRaft、PostgreSQL 17.12 使用本项目 Compose，Colima 分配 4 vCPU、6 GiB 内存。下表为同机顺序执行的单轮短测。

## 单网关扇出

160 个热房观众，每秒 800 条，持续 4 秒。每轮另有一个独立冷房，以每秒 5 条发送。发送者使用正式访客会话。每个接收端核对序号、重复和游标，两个模式均收齐 512,000 次热房投递及 20 条冷房消息，三项校验计数均为零。延迟取发送时间到采样客户端读帧的时间。

| 帧协议 | 热房数据帧 | 热房 P50 | 热房 P99 | 冷房 P99 |
|---|---:|---:|---:|---:|
| 单条帧 | 512,000 | 1.306 ms | 11.457 ms | 26.875 ms |
| 自适应合批 | 128,160 | 4.032 ms | 10.728 ms | 6.611 ms |

这轮合批少写了 74.97% 的热房数据帧。冷房每轮只有 20 个延迟样本。

## 房间隔离与滚动交接

- 32 个热房发送者各以每秒 5 条发送，热房探针收齐 640 条。独立冷房收齐 20 条，P50 为 6.791 ms，P99 为 10.096 ms。
- 两台网关各接入 12 位观众，每批最多邀请 4 位，在途上限为 2。源网关收到 SIGTERM 后，12 位观众全部收到交接邀请并迁移。24 位观众均按序收齐 80 条，最后一位在 606 ms 完成交接，单人恢复 P99 为 6 ms。

## 复现

先执行 `make up`，在两个终端分别运行 `make run-archiver` 和 `make run-gateway`。以下命令使用默认端口和默认合批参数，迁移命令自行启动两台临时网关。

```sh
go run ./cmd/bench fanout -viewers 160 -rate 800 -seconds 4 -legacy
go run ./cmd/bench fanout -viewers 160 -rate 800 -seconds 4
go run ./cmd/bench rooms
go build -o /tmp/danmaku-gateway ./cmd/gateway
go run ./cmd/bench migration -gateway /tmp/danmaku-gateway -viewers 24 -batch 4 -inflight 2
```
