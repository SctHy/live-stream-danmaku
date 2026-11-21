package mq

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"github.com/twmb/franz-go/pkg/kgo"
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
	records := []*kgo.Record{
		historyRecord(t, 10, protocol.DanmakuMessage{Content: "legacy"}),
		historyRecord(t, 11, protocol.DanmakuMessage{EventID: "stable"}),
	}

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
	records := []*kgo.Record{
		historyRecord(t, 10, protocol.DanmakuMessage{EventID: "first"}),
		historyRecord(t, 11, protocol.DanmakuMessage{EventID: "bad"}),
		historyRecord(t, 12, protocol.DanmakuMessage{EventID: "last"}),
	}
	var offset int64
	err := archivePartition(context.Background(), store, slog.Default(), records, func(_ context.Context, rs ...*kgo.Record) error { offset = rs[0].Offset; return nil })
	if err != nil || offset != 12 || strings.Join(saved, ",") != "first,last" {
		t.Fatal(err, offset, saved)
	}
}
func historyRecord(t *testing.T, offset int64, msg protocol.DanmakuMessage) *kgo.Record {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := json.Marshal(protocol.WsPacket{Type: protocol.TypeDanmaku, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return &kgo.Record{Topic: "history", Partition: 1, Offset: offset, Value: packet}
}
