package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/scthy/live-stream-danmaku/internal/mq"
	"github.com/scthy/live-stream-danmaku/internal/store"
)

func main() {
	format := flag.String("log-format", "json", "text or json")
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
	err = mq.Consume(ctx, mq.Brokers(os.Getenv("KAFKA_BROKERS")), store.NewMessageRepo(db), log)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Archive stopped", "error", err)
		os.Exit(1)
	}
}
