package mq

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const topic = "danmaku_save_topic"

type Producer struct {
	client   *kgo.Client
	log      *slog.Logger
	Failures atomic.Uint64
}

func NewProducer(brokers []string, log *slog.Logger) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.DisableIdempotentWrite(),
		kgo.RequiredAcks(kgo.LeaderAck()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		kgo.ProducerLinger(25*time.Millisecond),
		kgo.MaxBufferedRecords(16384),
		kgo.MaxBufferedBytes(32<<20),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{client: cl, log: log}, nil
}
func (p *Producer) Publish(room string, data []byte) {
	p.client.TryProduce(context.Background(), &kgo.Record{Topic: topic, Key: []byte(room), Value: data}, func(_ *kgo.Record, err error) {
		if err != nil {
			p.Failures.Add(1)
			p.log.Error("History publish failed", "error", err)
		}
	})
}
func (p *Producer) Drain(ctx context.Context) error {
	err := p.client.Flush(ctx)
	if err != nil {
		p.client.UnsafeAbortBufferedRecords()
	}
	p.client.Close()
	return err
}

func Brokers(raw string) []string {
	if raw == "" {
		return []string{"127.0.0.1:9092"}
	}
	return strings.Split(raw, ",")
}
