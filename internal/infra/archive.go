package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ForwardArchive deletes entries only after Kafka confirms the whole batch.
// Concurrent relays and a crash between confirmation and deletion can replay
// the same event. The history consumer's unique event_id absorbs duplicates.
func ForwardArchive(ctx context.Context) {
	r := InitRedisClient(-1)
	defer r.Close()
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Compression = sarama.CompressionSnappy
	var producer sarama.SyncProducer
	defer func() {
		if producer != nil {
			producer.Close()
		}
	}()
	for ctx.Err() == nil {
		var err error
		if producer == nil {
			producer, err = sarama.NewSyncProducer(KafkaBrokers(), config)
		}
		if err == nil {
			err = forwardArchiveBatch(ctx, r, producer)
		}
		if err != nil && err != redis.Nil && ctx.Err() == nil {
			logger.Log.Warn("Archive transfer pending", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func forwardArchiveBatch(ctx context.Context, r *redis.Client, producer sarama.SyncProducer) error {
	streams, err := r.XRead(ctx, &redis.XReadArgs{Streams: []string{archiveStream, "0-0"}, Count: 256, Block: time.Second}).Result()
	if err != nil {
		return err
	}
	entries := streams[0].Messages
	messages := make([]*sarama.ProducerMessage, 0, len(entries))
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		payload, ok := entry.Values["packet"].(string)
		if !ok {
			return fmt.Errorf("missing archive payload at %s", entry.ID)
		}
		messages = append(messages, &sarama.ProducerMessage{Topic: DanmakuSaveTopic, Value: sarama.StringEncoder(payload)})
		ids = append(ids, entry.ID)
	}
	if err := producer.SendMessages(messages); err != nil {
		return err
	}
	return r.XDel(ctx, archiveStream, ids...).Err()
}
