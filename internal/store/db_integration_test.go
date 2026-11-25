package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/protocol"
)

// Every run gets its own schema, including its version ledger.
func TestPostgresMigrationAndArchive(t *testing.T) {
	dsn := os.Getenv("DANMAKU_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DANMAKU_TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			schema := pgx.Identifier{fmt.Sprintf("dmtest_%d", time.Now().UnixNano())}.Sanitize()
			if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
				t.Fatal(err)
			}
			defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
			cfg, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConnConfig.RuntimeParams["search_path"] = schema
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			if legacy {
				_, err = pool.Exec(ctx, `CREATE TABLE danmaku_messages(id bigserial PRIMARY KEY,room_id varchar(50) NOT NULL,user_id numeric(20,0) NOT NULL,content varchar(500) NOT NULL,send_time timestamptz(3) NOT NULL);
INSERT INTO danmaku_messages(room_id,user_id,content,send_time) VALUES ('x',1,'legacy','2000-01-01')`)
				if err != nil {
					t.Fatal(err)
				}
			}
			var wg sync.WaitGroup
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := migrate(ctx, pool); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if t.Failed() {
				t.FailNow()
			}
			r := NewMessageRepo(pool)
			batch := []*protocol.DanmakuMessage{{EventID: "same", RoomID: "x", Content: "now", UserID: ^uint64(0), SendTime: time.Now()}}
			for range 2 {
				if _, err := r.SaveBatch(ctx, batch); err != nil {
					t.Fatal(err)
				}
			}
			var archived, preserved int
			err = pool.QueryRow(ctx, `SELECT
             count(*) FILTER (WHERE event_id='same' AND user_id=18446744073709551615 AND content='now'),
             count(*) FILTER (WHERE event_id IS NULL AND content='legacy')
             FROM danmaku_messages WHERE room_id='x'`).Scan(&archived, &preserved)
			if err != nil || archived != 1 || (legacy && preserved != 1) || (!legacy && preserved != 0) {
				t.Fatalf("migration and archive: archived=%d preserved=%d err=%v", archived, preserved, err)
			}
			if err := r.Withdraw(ctx, "x", "same"); err != nil {
				t.Fatal(err)
			}
			// Insert a newer visible message after the withdrawal, then verify both cursor pages.
			for _, id := range []string{"older", "newer"} {
				_, err := r.SaveBatch(ctx, []*protocol.DanmakuMessage{{EventID: id, RoomID: "x", Nickname: "测试昵称", Content: id, UserID: ^uint64(0), SendTime: time.Now()}})
				if err != nil {
					t.Fatal(err)
				}
			}
			first, more, err := r.ListHistory(ctx, "x", ^uint64(0), 1)
			if err != nil || !more || len(first) != 1 || first[0].EventID != "newer" || first[0].UserID != ^uint64(0) || first[0].Nickname != "测试昵称" {
				t.Fatal(first, more, err)
			}
			rest, more, err := r.ListHistory(ctx, "x", first[0].ID, 100)
			if err != nil || more || len(rest) != 1+preserved || rest[0].EventID != "older" || rest[0].ID >= first[0].ID {
				t.Fatal(rest, more, err)
			}
			if legacy && (rest[1].EventID != "" || rest[1].Content != "legacy") {
				t.Fatal(rest)
			}
			var recorded bool
			if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM danmaku_withdrawals WHERE room_id='x' AND event_id='same')").Scan(&recorded); err != nil || !recorded {
				t.Fatalf("withdrawal recorded=%v err=%v", recorded, err)
			}
			var versions int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM danmaku_schema_migrations").Scan(&versions); err != nil || versions != 6 {
				t.Fatalf("ledger: %d %v", versions, err)
			}
		})
	}
}
