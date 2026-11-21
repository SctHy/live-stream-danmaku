package main

import (
	"context"
	"errors"
	"flag"
	"github.com/scthy/live-stream-danmaku/internal/pubsub"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"encoding/json"
	"github.com/scthy/live-stream-danmaku/internal/httpapi"
	"github.com/scthy/live-stream-danmaku/internal/hub"
	"github.com/scthy/live-stream-danmaku/internal/mq"
	"github.com/scthy/live-stream-danmaku/internal/store"
	"log/slog"
)

func main() {
	format := flag.String("log-format", "json", "text or json")
	host := flag.String("host", "127.0.0.1", "listen address")
	port := flag.String("port", "8080", "HTTP port")
	capacity := flag.Int("send-queue", hub.SendChanSize, "maximum frames per connection")
	window := flag.Duration("batch-window", 5*time.Millisecond, "maximum hot-room coalescing delay")
	gap := flag.Duration("batch-arrival-gap", hub.DefaultBatchArrivalGap, "maximum interval between arrivals to start a batch")
	minViewers := flag.Int("batch-min-viewers", 128, "minimum batch-capable viewers")
	shards := flag.Int("room-shards", hub.RoomShards, "number of room workers")
	migrationTimeout := flag.Duration("migration-timeout", 30*time.Second, "maximum time for rolling handoff (zero closes immediately)")
	migrationBatch := flag.Int("migration-batch", 100, "maximum handoff invitations per 100ms")
	migrationLimit := flag.Int("migration-inflight", 64, "maximum outstanding handoffs (zero uses fixed rate)")
	allowDev := flag.Bool("allow-dev-guest", false, "allow loopback benchmark identities")
	muxShards := flag.Int("pubsub-shards", 8, "Redis subscription connections")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	switch *format {
	case "text":
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	case "json":
	default:
		flag.Usage()
		os.Exit(2)
	}
	if *muxShards < 1 || *migrationTimeout < 0 || *migrationBatch < 1 || *migrationLimit < 0 || *shards < 1 || *shards > 256 || *capacity < 1 || *window < 0 || *minViewers < 1 || *gap < 0 {
		log.Error("invalid gateway configuration")
		os.Exit(2)
	}
	db, err := store.Open()
	if err != nil {
		log.Error("PostgreSQL startup", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	sessions := pubsub.Open(os.Getenv("REDIS_ADDR"))
	defer sessions.Close()
	producer, err := mq.NewProducer(mq.Brokers(os.Getenv("KAFKA_BROKERS")), log)
	if err != nil {
		log.Error("Kafka startup", "error", err)
		os.Exit(1)
	}
	m := hub.New(log, producer, sessions, *capacity, *shards, *muxShards)
	m.BatchWindow = *window
	m.BatchArrivalGap = *gap
	m.MinBatchViewers = *minViewers
	go m.Start()
	r := http.NewServeMux()
	httpapi.NewChatHandler(log, store.NewMessageRepo(db), m, sessions, *allowDev).Routes(r)
	r.HandleFunc("GET /livez", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })
	r.HandleFunc("GET /readyz", func(w http.ResponseWriter, req *http.Request) {
		if m.Retiring() {
			w.WriteHeader(503)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), time.Second)
		defer cancel()
		if err := sessions.Ping(ctx); err != nil {
			w.WriteHeader(503)
			return
		}
		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	r.HandleFunc("GET /metrics", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.Counters())
	})
	srv := http.Server{Addr: net.JoinHostPort(*host, *port), Handler: httpapi.Recover(log, r), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP listener", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		migration, finish := context.WithTimeout(context.Background(), *migrationTimeout)
		m.Migrate(migration, *migrationBatch, *migrationLimit, 100*time.Millisecond)
		finish()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
		if err := m.Drain(shutdown); err != nil {
			log.Error("Drain timed out", "error", err)
			os.Exit(1)
		}
	}
}
