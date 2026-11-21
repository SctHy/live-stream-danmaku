package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/scthy/live-stream-danmaku/internal/api"
	"github.com/scthy/live-stream-danmaku/internal/infra"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"github.com/scthy/live-stream-danmaku/internal/repo"
	"github.com/scthy/live-stream-danmaku/internal/ws"
	"go.uber.org/zap"
)

func main() {
	logger.InitLogger("prod")
	defer logger.Sync()
	host := flag.String("host", "127.0.0.1", "listen address")
	port := flag.String("port", "8080", "HTTP port")
	capacity := flag.Int("send-queue", ws.SendChanSize, "maximum frames per connection")
	window := flag.Duration("batch-window", 5*time.Millisecond, "maximum hot-room coalescing delay")
	gap := flag.Duration("batch-arrival-gap", ws.DefaultBatchArrivalGap, "maximum interval between arrivals to start a batch")
	minViewers := flag.Int("batch-min-viewers", 128, "minimum batch-capable viewers")
	shards := flag.Int("room-shards", ws.RoomShards, "number of room workers")
	migrationTimeout := flag.Duration("migration-timeout", 30*time.Second, "maximum time for rolling handoff (zero closes immediately)")
	migrationBatch := flag.Int("migration-batch", 100, "maximum handoff invitations per 100ms")
	migrationLimit := flag.Int("migration-inflight", 64, "maximum outstanding handoffs (zero uses fixed rate)")
	flag.Parse()
	if *migrationTimeout < 0 || *migrationBatch < 1 || *migrationLimit < 0 || *shards < 1 || *shards > 256 || *capacity < 1 || *window < 0 || *minViewers < 1 || *gap < 0 {
		logger.Log.Fatal("invalid gateway configuration")
	}
	db := infra.InitDB()
	defer db.Close()
	m := ws.NewManager(*capacity, *shards)
	m.BatchWindow = *window
	m.BatchArrivalGap = *gap
	m.MinBatchViewers = *minViewers
	go m.Start()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	api.NewChatHandler(repo.NewMessageRepo(db), m).Routes(r)
	r.GET("/livez", func(c *gin.Context) { c.Status(200) })
	r.GET("/readyz", func(c *gin.Context) {
		if m.Retiring() {
			c.Status(503)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if err := m.RedisClient.Ping(ctx).Err(); err != nil {
			c.Status(503)
			return
		}
		if err := db.Ping(ctx); err != nil {
			c.Status(503)
			return
		}
		c.Status(200)
	})
	r.GET("/metrics", func(c *gin.Context) { c.JSON(200, m.Metrics()) })
	srv := http.Server{Addr: net.JoinHostPort(*host, *port), Handler: r, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("HTTP listener", zap.Error(err))
		}
	case <-ctx.Done():
		migration, finish := context.WithTimeout(context.Background(), *migrationTimeout)
		m.Migrate(migration, *migrationBatch, *migrationLimit, 100*time.Millisecond)
		finish()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
		if err := m.Drain(shutdown); err != nil {
			logger.Log.Warn("Drain timed out", zap.Error(err))
		}
	}
}
