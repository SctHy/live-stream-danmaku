package mq

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/metrics"
	"github.com/twmb/franz-go/pkg/kgo"
)

const topic = "danmaku_save_topic"

type Producer struct {
	client  *kgo.Client
	log     *slog.Logger
	metrics *metrics.GatewayMetrics
}

func NewProducer(brokers []string, log *slog.Logger, metrics *metrics.GatewayMetrics) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		// Leader acknowledgements reduce latency and require disabling Kafka idempotence.
		// PostgreSQL deduplicates archive retries with ON CONFLICT (event_id) DO NOTHING.
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
	return &Producer{client: cl, log: log, metrics: metrics}, nil
}
func (p *Producer) Subscribe(bus *events.Bus) {
	bus.Danmaku.Subscribe(func(_ context.Context, e events.DanmakuAccepted) { p.Publish(e.Room, e.Wire()) })
}
func (p *Producer) Publish(room string, data []byte) {
	p.client.TryProduce(context.Background(), &kgo.Record{Topic: topic, Key: []byte(room), Value: data}, func(_ *kgo.Record, err error) {
		if err != nil {
			p.metrics.KafkaFailure()
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
