package infra

import (
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"go.uber.org/zap"
)

const (
	DanmakuSaveTopic    = "danmaku_save_topic" // Topic name for danmaku saving
	ProducerChanBufSize = 16384
)

var HistoryQueueDrops atomic.Uint64
var kafkaFailures, redisFailures atomic.Uint64

func ReportDeliveryFailures() {
	q, k, r := HistoryQueueDrops.Swap(0), kafkaFailures.Swap(0), redisFailures.Swap(0)
	if q+k+r > 0 {
		logger.Log.Warn("Delivery failures since last report", zap.Uint64("history_queue_full", q), zap.Uint64("history_broker_failed", k), zap.Uint64("live_publish_failed", r))
	}
}
func KafkaBrokers() []string {
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		return strings.Split(v, ",")
	}
	return []string{"127.0.0.1:9092"}
}

func InitKafkaProducer(brokers []string) (sarama.AsyncProducer, error) {
	// Init Kafka Producer
	// Configure Sarama settings
	config := sarama.NewConfig()
	config.ChannelBufferSize = ProducerChanBufSize // Allow more buffering in memory
	// We must wait for the acknowledgment from Kafka to ensure data is safe.
	//   - NoResponse
	//   - WaitForLocal: Leader returns OK after receiving
	//   - WaitForAll: all follower synced
	// For Danmaku, speed > absolute consistency.
	config.Producer.RequiredAcks = sarama.WaitForLocal

	// Snappy is the standard for Kafka logging (Google developed, fast/low CPU).
	// This drastically reduces network bandwidth usage.
	config.Producer.Compression = sarama.CompressionSnappy

	// Improve Batching.
	// Sarama will wait up to 10ms or until batch size is reached.
	// This reduces the number of requests sent to the broker.
	// This prevents sending too many tiny packets.

	//(Producer.Flush.MaxMessages must be >= Producer.Flush.Messages when set)
	config.Producer.Flush.Messages = 50
	config.Producer.Flush.MaxMessages = 200
	config.Producer.Flush.Frequency = 50 * time.Millisecond
	config.Producer.Flush.Bytes = 8 * 1024 // 8KB
	// We need to return success info to avoid errors in SyncProducer.
	// config.Producer.Return.Successes = true

	// performance optimization:no return for successful msgs(reduce channel overhead)
	config.Producer.Return.Successes = false
	//return msgs with error,otherwise have no ability to monitor KAFKA
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 3
	// multiple partitions in one topic of Kafka
	// Use Random partitioner to distribute messages evenly in all partitions
	config.Producer.Partitioner = sarama.NewRandomPartitioner

	// Connect to Kafka (running on localhost:9092 (in .yaml) via Docker)1

	// producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	//listen from KAFKA producer error channel
	//if not doing so,errors will fill in channel till blocking producer
	go func() {
		for err := range producer.Errors() {
			kafkaFailures.Add(1)
			logger.Log.Error("[KAFKA INFRA] Kafka Async Write Error",
				zap.Error(err.Err),
				zap.String("topic", err.Msg.Topic),
			)
		}
	}()
	return producer, nil
}

// InitKafkaConsumerGroup initializes the consumer group with high-throughput settings.
func InitKafkaConsumerGroup(brokers []string, groupID string) sarama.ConsumerGroup {
	config := sarama.NewConfig()
	// OffsetOldest: If the group is new, start from the very beginning.
	// This ensures we don't miss messages sent while the consumer was down.
	// OffsetNewest: discard history
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Performance: Fetch Settings (Critical for Batch Processing)
	// Default is 1MB. Increase to 5MB to reduce network round-trips.
	// We want to fetch a large chunk of data to feed our batch handler.
	config.Consumer.Fetch.Default = 5 * 1024 * 1024
	// Limit error logging for debugging
	config.Consumer.Return.Errors = true

	// Connect to Brokers
	group, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		logger.Log.Panic("[KAFKA INFRA] Failed to create consumer group", zap.Error(err))
	}

	// Monitor Errors in background
	// It is safer to handle errors here (logging) than leaving the channel full blocking the consumer.
	go func() {
		for err := range group.Errors() {
			logger.Log.Error("[KAFKA INFRA] Consumer Group Error", zap.Error(err))
		}
	}()

	return group
}

// PublishHistory runs independently from live Redis delivery. During startup
// outages the bounded queue absorbs messages until the broker becomes available.
func PublishHistory(queue <-chan []byte) {
	brokers := KafkaBrokers()
	for {
		producer, err := InitKafkaProducer(brokers)
		if err != nil {
			logger.Log.Warn("History broker unavailable", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		for payload := range queue {
			producer.Input() <- &sarama.ProducerMessage{Topic: DanmakuSaveTopic, Value: sarama.ByteEncoder(payload)}
		}
		producer.AsyncClose()
		return
	}
}
