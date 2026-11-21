package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
)

type batchStore interface {
	SaveBatch(context.Context, []*protocol.DanmakuMessage) error
}

func Consume(ctx context.Context, brokers []string, store batchStore, log *slog.Logger) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumerGroup("danmaku_group_v1"), kgo.DisableAutoCommit(), kgo.AllowAutoTopicCreation(), kgo.BlockRebalanceOnPoll(), kgo.OnPartitionsRevoked(func(_ context.Context, _ *kgo.Client, partitions map[string][]int32) {
		log.Debug("Partitions released", "partitions", partitions)
	}))
	if err != nil {
		return err
	}
	defer cl.CloseAllowingRebalance()
	for ctx.Err() == nil {
		fetch := cl.PollRecords(ctx, 1000)
		for _, e := range fetch.Errors() {
			if ctx.Err() == nil {
				log.Warn("Kafka fetch", "error", e.Err, "partition", e.Partition)
			}
		}
		fetch.EachPartition(func(p kgo.FetchTopicPartition) {
			if err == nil && len(p.Records) > 0 {
				err = archivePartition(ctx, store, log, p.Records, cl.CommitRecords)
			}
		})
		cl.AllowRebalance()
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
func archivePartition(ctx context.Context, store batchStore, log *slog.Logger, records []*kgo.Record, commit func(context.Context, ...*kgo.Record) error) error {
	batch := make([]*protocol.DanmakuMessage, 0, len(records))
	for _, r := range records {
		var p protocol.WsPacket
		var d protocol.DanmakuMessage
		if json.Unmarshal(r.Value, &p) != nil || p.Type != protocol.TypeDanmaku || json.Unmarshal(p.Data, &d) != nil || !d.FitsHistory() {
			log.Error("Invalid history record", "topic", r.Topic, "partition", r.Partition, "offset", r.Offset)
			continue
		}
		if d.EventID == "" {
			d.EventID = fmt.Sprintf("kafka:%s:%d:%d", r.Topic, r.Partition, r.Offset)
		}
		batch = append(batch, &d)
	}
	if err := persist(ctx, store, batch, log); err != nil {
		return err
	}
	return commit(ctx, records[len(records)-1])
}
