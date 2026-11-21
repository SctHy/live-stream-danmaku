package mq

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
	"strings"
	"testing"
)

type writeBatch func(context.Context, []*protocol.DanmakuMessage) error

func (f writeBatch) SaveBatch(ctx context.Context, b []*protocol.DanmakuMessage) error {
	return f(ctx, b)
}

func TestClaimReplaysUnconfirmedBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var marked []int64
	commit := func(ctx context.Context, rs ...*kgo.Record) error { marked = append(marked, rs[0].Offset); return nil }
	records := []*kgo.Record{{Topic: "history", Partition: 1, Offset: 10, Value: []byte(`{"type":101,"data":{"content":"legacy"}}`)}, {Topic: "history", Partition: 1, Offset: 11, Value: []byte(`{"type":101,"data":{"event_id":"stable"}}`)}}
	store := writeBatch(func(context.Context, []*protocol.DanmakuMessage) error {
		cancel()
		return errors.New("connection lost before acknowledgement")
	})
	err := archivePartition(ctx, store, slog.Default(), records, commit)
	if !errors.Is(err, context.Canceled) || len(marked) != 0 {
		t.Fatalf("err=%v marked=%v", err, marked)
	}
	ctx = context.Background()
	store = func(_ context.Context, batch []*protocol.DanmakuMessage) error {
		if len(batch) != 2 || batch[0].EventID != "kafka:history:1:10" || batch[1].EventID != "stable" {
			t.Fatalf("unexpected replay: %v", batch)
		}
		if len(marked) != 0 {
			t.Fatal("offset advanced before transaction")
		}
		return nil
	}
	if err := archivePartition(ctx, store, slog.Default(), records, commit); err != nil {
		t.Fatal(err)
	}
	if len(marked) != 1 || marked[0] != 11 {
		t.Fatal(marked)
	}
}

func TestConstraintFailureIsolated(t *testing.T) {
	var saved []string
	store := writeBatch(func(ctx context.Context, b []*protocol.DanmakuMessage) error {
		for _, r := range b {
			if r.EventID == "bad" {
				return &pgconn.PgError{Code: "23514"}
			}
		}
		saved = append(saved, b[0].EventID)
		return nil
	})
	if err := persist(context.Background(), store, []*protocol.DanmakuMessage{{EventID: "first"}, {EventID: "bad"}, {EventID: "last"}}, slog.Default()); err != nil || strings.Join(saved, ",") != "first,last" {
		t.Fatal(err, saved)
	}
}
