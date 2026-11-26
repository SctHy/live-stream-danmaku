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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/scthy/maku/internal/commerce"
	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/livebridge"
	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/recommend"
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
	plugins, err := commerce.LoadPlugins(os.Getenv("GIFT_CATALOG_FILE"), os.Getenv("PAYMENT_WEBHOOK_SECRET"), os.Getenv("VOUCHER_CODES"))
	if err != nil {
		log.Error("Invalid commerce configuration", "error", err)
		return 2
	}
	telemetry := metrics.NewGatewayMetrics()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	history := startHistory(ctx, openHistory, 5*time.Second, log)
	defer history.Close()
	sessions := pubsub.Open(os.Getenv("REDIS_ADDR"))
	defer sessions.Close()
	producer, err := mq.NewProducer(mq.Brokers(os.Getenv("KAFKA_BROKERS")), log, telemetry)
	if err != nil {
		log.Error("Kafka startup", "error", err)
		return 1
	}
	defer func() {
		op, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := producer.Drain(op); err != nil {
			log.Error("Drain Kafka", "error", err)
			code = 1
		}
	}()
	bus := new(events.Bus)
	producer.Subscribe(bus)
	var bridge hub.LiveBridge
	if *bridgeEnabled {
		bridge = livebridge.New(livebridge.Config{Logger: log})
	}
	m := hub.New(hub.Config{
		LiveBridge:             bridge,
		DisableSlowKick:        *disableKick,
		Logger:                 log,
		Bus:                    bus,
		Store:                  sessions,
		SessionClock:           sessions,
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

	heat := pubsub.SubscribeHeat(bus, sessions, log)
	r := http.NewServeMux()
	handler := httpapi.NewChatHandlerWithBus(history, m, sessions, *allowDev, bus)
	handler.SubscribeInterests()
	handler.SubscribeCaches()
	// Keep collecting through migration and drain before closing Redis.
	profileCtx, stopProfiles := context.WithCancel(context.WithoutCancel(ctx))
	profilesDone := make(chan struct{})
	go func() { defer close(profilesDone); handler.RunInterestWriter(profileCtx) }()
	heatDone := make(chan error, 1)
	go func() { heatDone <- heat.Run(profileCtx) }()
	defer func() {
		stopProfiles()
		<-profilesDone
		if err := <-heatDone; err != nil {
			log.Error("Drain heat", "error", err)
			code = 1
		}
	}()
	defer bus.Close()
	handler.SetRecommendations(recommend.Default())
	handler.SetCommerce(plugins)
	go m.Start()
	handler.Routes(r)
	deliveryCtx, endDelivery := context.WithCancel(ctx)
	deliveryDone := make(chan struct{})
	var keysReady atomic.Bool
	go func() {
		defer close(deliveryDone)
		for deliveryCtx.Err() == nil {
			op, cancel := context.WithTimeout(deliveryCtx, time.Minute)
			err := sessions.UpgradeSessionKeys(op)
			cancel()
			if err == nil {
				keysReady.Store(true)
				handler.RetryModeration(deliveryCtx)
				return
			}
			log.Warn("Upgrade Redis session keys", "error", err)
			select {
			case <-deliveryCtx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	defer func() { endDelivery(); <-deliveryDone }()
	r.HandleFunc("GET /livez", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })
	r.HandleFunc("GET /readyz", readiness(func() bool { return m.Retiring() || !keysReady.Load() }, sessions.Ping))
	r.Handle("GET /metrics", telemetry.Handler())
	requestCtx, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRequests()
	var requests sync.WaitGroup
	var admission sync.Mutex
	accepting := true
	srv := http.Server{BaseContext: func(net.Listener) context.Context { return requestCtx }, Addr: net.JoinHostPort(*host, *port), Handler: httpapi.Recover(log, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		admission.Lock()
		if !accepting {
			admission.Unlock()
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		requests.Add(1)
		admission.Unlock()
		defer requests.Done()
		if !keysReady.Load() && strings.HasPrefix(req.URL.Path, "/api/rooms/") && !strings.HasSuffix(req.URL.Path, "/history") {
			w.WriteHeader(503)
			return
		}
		r.ServeHTTP(w, req)
	})), ReadHeaderTimeout: 5 * time.Second}
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
	admission.Lock()
	accepting = false
	admission.Unlock()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Error("HTTP shutdown", "error", err)
		code = 1
	}
	cancelRequests()
	if shutdown.Err() != nil {
		if err := srv.Close(); err != nil {
			log.Error("Close HTTP", "error", err)
			code = 1
		}
	}
	drain, endDrain := context.WithTimeout(context.Background(), 5*time.Second)
	defer endDrain()
	if err := m.Drain(drain); err != nil {
		log.Error("Drain timed out", "error", err)
		code = 1
	}
	requests.Wait()
	return code
}

func readiness(retiring func() bool, ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if retiring() {
			w.WriteHeader(503)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}
}
