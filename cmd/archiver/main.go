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

	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/store"
)

func main() { os.Exit(run()) }
func run() (code int) {
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
		return 2
	}
	db, err := store.Open()
	if err != nil {
		log.Error("PostgreSQL startup", "error", err)
		return 1
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var telemetry *metrics.ArchiveMetrics
	if *addr != "" {
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			log.Error("Metrics listener", "error", err)
			return 1
		}
		telemetry = metrics.NewArchiveMetrics()
		mux := adminHandler(telemetry.Handler(), db.Ping)
		srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		serveDone := make(chan error, 1)
		defer func() {
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdown); err != nil {
				log.Error("HTTP shutdown", "error", err)
				code = 1
			}
			if err := <-serveDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("Metrics listener", "error", err)
				code = 1
			}
		}()
		go func() {
			err := srv.Serve(ln)
			serveDone <- err
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop()
			}
		}()
	}
	err = mq.Consume(ctx, mq.Brokers(os.Getenv("KAFKA_BROKERS")), store.NewMessageRepo(db), log, telemetry)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Archive stopped", "error", err)
		return 1
	}
	return 0
}
func adminHandler(metrics http.Handler, ping func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metrics)
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if ping(ctx) != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	return mux
}
