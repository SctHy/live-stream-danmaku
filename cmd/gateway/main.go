package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/livebridge"
	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

func main() { os.Exit(run()) }
func run() (code int) {
	format := flag.String("log-format", "json", "text or json")
	host := flag.String("host", "127.0.0.1", "listen address")
	port := flag.String("port", "8080", "HTTP port")
	disableKick := flag.Bool("bench-disable-slow-kick", false, "benchmark control: disable consecutive-drop eviction")
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
	wordFile := flag.String("sensitive-words", "", "dictionary file, one word per line")
	mode := flag.String("filter-mode", "reject", "reject or mask matching messages")
	bridgeEnabled := flag.Bool("bilibili-danmaku", true, "bridge real-time danmaku for numeric Bilibili rooms")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	switch *format {
	case "text":
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	case "json":
	default:
		flag.Usage()
		return 2
	}
	if *muxShards < 1 || *migrationTimeout < 0 || *migrationBatch < 1 || *migrationLimit < 0 || *shards < 1 || *shards > 256 || *capacity < 1 || *window < 0 || *minViewers < 1 || *gap < 0 {
		log.Error("invalid gateway configuration")
		return 2
	}
	if *mode != "reject" && *mode != "mask" {
		log.Error("invalid filter mode", "mode", *mode)
		return 2
	}
	var words *filter.Filter
	if *wordFile != "" {
		var err error
		words, err = filter.LoadFile(*wordFile)
		if err != nil {
			log.Error("Load sensitive words", "error", err)
			return 1
		}
	}
	telemetry := metrics.NewGatewayMetrics()
	db, err := store.Open()
	if err != nil {
		log.Error("PostgreSQL startup", "error", err)
		return 1
	}
	defer db.Close()
	sessions := pubsub.Open(os.Getenv("REDIS_ADDR"))
	defer sessions.Close()
	producer, err := mq.NewProducer(mq.Brokers(os.Getenv("KAFKA_BROKERS")), log, telemetry)
	if err != nil {
		log.Error("Kafka startup", "error", err)
		return 1
	}
	var bridge hub.LiveBridge
	if *bridgeEnabled {
		bridge = livebridge.New(livebridge.Config{Logger: log})
	}
	m := hub.New(hub.Config{
		LiveBridge:             bridge,
		DisableSlowKick:        *disableKick,
		Logger:                 log,
		Producer:               producer,
		Store:                  sessions,
		Metrics:                telemetry,
		Filter:                 words,
		MaskWords:              *mode == "mask",
		SendCapacity:           *capacity,
		RoomShards:             *shards,
		PubSubShards:           *muxShards,
		BatchWindow:            *window,
		BatchArrivalGap:        *gap,
		DisableArrivalBatching: *gap == 0,
		MinBatchViewers:        *minViewers,
	})
	go m.Start()
	r := http.NewServeMux()
	httpapi.NewChatHandler(store.NewMessageRepo(db), m, sessions, *allowDev).Routes(r)
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
	r.Handle("GET /metrics", telemetry.Handler())
	srv := http.Server{Addr: net.JoinHostPort(*host, *port), Handler: httpapi.Recover(log, r), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if words != nil {
		reload := make(chan os.Signal, 1)
		signal.Notify(reload, syscall.SIGHUP)
		defer signal.Stop(reload)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-reload:
					if err := words.ReloadFile(*wordFile); err != nil {
						log.Error("Reload sensitive words", "error", err)
					}
				}
			}
		}()
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP listener", "error", err)
			code = 1
		}
	case <-ctx.Done():
		migration, finish := context.WithTimeout(context.Background(), *migrationTimeout)
		m.Migrate(migration, *migrationBatch, *migrationLimit, 100*time.Millisecond)
		finish()
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Error("HTTP shutdown", "error", err)
		code = 1
	}
	if err := m.Drain(shutdown); err != nil {
		log.Error("Drain timed out", "error", err)
		return 1
	}
	return code
}
