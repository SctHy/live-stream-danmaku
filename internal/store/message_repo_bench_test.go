package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

func BenchmarkSaveBatch(b *testing.B) {
	dsn := os.Getenv("DANMAKU_TEST_DATABASE_URL")
	if dsn == "" {
		b.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		b.Fatal(err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer admin.Close()
	schema := pgx.Identifier{fmt.Sprintf("dmbench_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		b.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg = cfg.Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()
	if err := migrate(ctx, pool); err != nil {
		b.Fatal(err)
	}
	repo := NewMessageRepo(pool)
	batch := make([]*protocol.DanmakuMessage, 128)
	for i := range batch {
		batch[i] = &protocol.DanmakuMessage{RoomID: "room", UserID: uint64(i + 1), Content: "benchmark", SendTime: time.Now()}
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i, msg := range batch {
			msg.EventID = strconv.Itoa(n*len(batch) + i)
		}
		if _, err := repo.SaveBatch(ctx, batch); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
