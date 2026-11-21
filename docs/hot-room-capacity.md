# 千人和万人热门房间的端到端实验

## 发言重试与 Kafka 停机验收

使用独立的 Redis、Kafka、PostgreSQL、两个网关和一个消费者。停止 Kafka 后，通过正式会话入口在 A 发言，由近期接口独立确认发布，发送连接关闭且不读取回执。使用相同访客 cookie，通过 B 的 12 个连接重发相同请求。12 个成功回执均返回首次发布的事件 ID，近期查询只有一条。Kafka 停机时 Redis 待转发量为 1，恢复后归零，原媒体时间段回放只有一条且 ID 一致。独立 Chromium 测试在首个发送请求到达网关前断开连接，页面重连后点击重试，浏览器两次发出的请求 ID、文本及媒体时间完全一致，最终显示该消息。

复现后端验收时，让两个网关和消费者共享独立测试环境。停止该环境的 Kafka，设置两个网关地址并运行以下测试。看到等待归档日志后，在两分钟内恢复 Kafka。

```sh
DANMAKU_TEST_GATEWAY_A=http://127.0.0.1:38680 \
DANMAKU_TEST_GATEWAY_B=http://127.0.0.1:38681 \
go test -count=1 -v -run TestRetryAcrossGateways ./cmd/server
```

去重窗口为 10 分钟。待归档流独立于近期窗口，在 Kafka 确认前保留；重复转发由数据库事件 ID 唯一约束处理。该实验覆盖 Kafka 故障，Redis 数据丢失超出验证范围。
