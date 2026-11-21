package infra

import (
	"context"
	"fmt"
	"os"

	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Redis Key Templates (Centralized management to avoid typos)
const (
	// Ensure Redis is running on localhost:6379 via Docker.
	RedisAddr           = "127.0.0.1:6379"
	KeyRoomPubSub       = "room:%s:pubsub"    // Channel for Redis Pub/Sub
	KeyRoomLikes        = "room:%s:likes"     // Counter for likes
	KeyRoomOnline       = "room:%s:online:*"  // Counter for online users of a room
	KeyRoomServerOnline = "room:%s:online:%s" // Counter for online users of a server
	KeyServerOfRoom     = "room:%s:servers"   // server lists for a specific room
	RedisCancelTimeout  = 2 * time.Second
)

func InitRedisClient(retries int) *redis.Client {
	// Initialize Redis client.
	// Lazy loading: only create Redis client when first time trying to contact
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = RedisAddr
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: addr, MaxRetries: retries,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return rdb
}

// PublishToRoom sends a message to a specific room's Redis channel.
func PublishToRoom(rdb *redis.Client, roomID string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), RedisCancelTimeout)
	defer cancel()

	channel := fmt.Sprintf(KeyRoomPubSub, roomID)

	err := rdb.Publish(ctx, channel, payload).Err()
	if err != nil {
		redisFailures.Add(1)
		logger.Log.Error("[REDIS INFRA] Publish failed",
			zap.String("room", roomID),
			zap.Error(err),
		)
	}
}
