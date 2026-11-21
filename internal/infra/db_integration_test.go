package infra

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"github.com/charlesAcmen/livestream-danmaku/internal/repo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Every run gets its own schema, including its version ledger.
func TestPostgresMigrationAndReplay(t *testing.T) {
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
			r := repo.NewMessageRepo(pool)
			batch := []*model.DanmakuMessage{{EventID: "same", RoomID: "x", Content: "now", UserID: ^uint64(0), SendTime: time.Now()}}
			for range 2 {
				if err := r.SaveBatch(ctx, batch); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := r.GetPlayBackDanmaku("x", time.Time{}, 100)
			if err != nil {
				t.Fatal(err)
			}
			expected := 1
			if legacy {
				expected = 2
			}
			if len(rows) != expected || rows[len(rows)-1].UserID != ^uint64(0) {
				t.Fatalf("rows %v", rows)
			}
			if legacy && (rows[0].Content != "legacy" || rows[0].EventID != "") {
				t.Fatalf("legacy record changed: %+v", rows[0])
			}
			rows, err = r.GetPlayBackDanmaku("x", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), 1)
			if err != nil || len(rows) != 1 || rows[0].Content != "now" {
				t.Fatalf("playback %v %v", rows, err)
			}
			var versions int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM danmaku_schema_migrations").Scan(&versions); err != nil || versions != 1 {
				t.Fatalf("ledger: %d %v", versions, err)
			}
		})
	}
}
