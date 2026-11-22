package mq

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/protocol"
)

func persist(ctx context.Context, store batchStore, batch []*protocol.DanmakuMessage, log *slog.Logger, metrics *metrics.ArchiveMetrics) error {
	delay := 200 * time.Millisecond
	for len(batch) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		written, err := store.SaveBatch(ctx, batch)
		metrics.Batch(written, time.Since(start))
		if err == nil {
			return nil
		}
		var pg *pgconn.PgError
		if errors.As(err, &pg) && len(pg.Code) >= 2 && (pg.Code[:2] == "22" || pg.Code[:2] == "23") {
			if len(batch) == 1 {
				metrics.DeadLetter()
				log.Error("Dead-letter history record", "event_id", batch[0].EventID, "sqlstate", pg.Code, "error", err)
				return nil
			}
			for i := range batch {
				if err := persist(ctx, store, batch[i:i+1], log, metrics); err != nil {
					return err
				}
			}
			return nil
		}
		log.Warn("Retrying history batch", "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay/2 + rand.N(delay/2)):
		}
		delay = min(2*delay, 5*time.Second)
	}
	return nil
}
