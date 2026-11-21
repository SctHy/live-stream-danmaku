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

	"github.com/scthy/live-stream-danmaku/internal/metrics"
	"github.com/scthy/live-stream-danmaku/internal/mq"
	"github.com/scthy/live-stream-danmaku/internal/store"
)

func main() {
	format := flag.String("log-format", "json", "text or json")
	addr := flag.String("metrics-addr", "", "optional Prometheus listen address")
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
	db, err := store.Open()
	if err != nil {
		log.Error("PostgreSQL startup", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var telemetry *metrics.ArchiveMetrics
	if *addr != "" {
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			log.Error("Metrics listener", "error", err)
			os.Exit(1)
		}
		telemetry = metrics.NewArchiveMetrics()
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", telemetry.Handler())
		srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		defer srv.Close()
		go func() {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("Metrics listener", "error", err)
				stop()
			}
		}()
	}
	err = mq.Consume(ctx, mq.Brokers(os.Getenv("KAFKA_BROKERS")), store.NewMessageRepo(db), log, telemetry)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Archive stopped", "error", err)
		os.Exit(1)
	}
}
