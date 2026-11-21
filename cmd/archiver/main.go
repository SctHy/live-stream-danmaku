package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/scthy/live-stream-danmaku/internal/consumer"
	"github.com/scthy/live-stream-danmaku/internal/infra"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"github.com/scthy/live-stream-danmaku/internal/repo"
)

const (
	KafkaTopic   = "danmaku_save_topic"
	KafkaGroupID = "danmaku_group_v1" // consumer group ID
	BatchSize    = 1000
)

func main() {
	// 1. init Logger
	logger.InitLogger("dev")
	defer logger.Sync()

	logger.Log.Info("[KAFKA CONSUMER]Starting Kafka Consumer...")

	// 2. init db
	db := infra.InitDB()
	defer db.Close()
	logger.Log.Info("[KAFKA CONSUMER]Database initialized")

	messageRepo := repo.NewMessageRepo(db)
	handler := consumer.NewDanmakuHandler(messageRepo, BatchSize)

	brokers := infra.KafkaBrokers()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	group, err := infra.InitKafkaConsumerGroup(brokers, KafkaGroupID)
	for err != nil {
		logger.Log.Warn("History broker unavailable", zap.Error(err))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		group, err = infra.InitKafkaConsumerGroup(brokers, KafkaGroupID)
	}
	defer group.Close()
	// 7. start consuming
	logger.Log.Info("[KAFKA CONSUMER]Ready to process messages")

	for {
		// Consume is a blocking call.
		// It returns when:
		// A) A rebalance occurs (new member joined/left) -> Loop continues
		// B) Context is canceled -> Loop breaks
		err := group.Consume(ctx, []string{KafkaTopic}, handler)
		if err != nil {
			logger.Log.Error("[KAFKA CONSUMER]Consume loop error", zap.Error(err))
			time.Sleep(time.Second) // retry after a bit
		}
		if ctx.Err() != nil {
			break
		}
	}

	logger.Log.Info("[KAFKA CONSUMER]Consumer exited")
}
