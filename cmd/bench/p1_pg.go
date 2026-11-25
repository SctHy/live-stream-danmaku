package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/store"
)

func (s *p1Suite) postgres(dsn string) (result error) {
	admin, err := pgx.Connect(s.ctx, dsn)
	if err != nil {
		return err
	}
	defer admin.Close(context.Background())
	schema := pgx.Identifier{"p1_" + strings.ToLower(rand.Text())}.Sanitize()
	if _, err = admin.Exec(s.ctx, "CREATE SCHEMA "+schema); err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		result = errors.Join(result, err)
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(s.ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = store.Migrate(s.ctx, pool); err != nil {
		return err
	}
	if s.selected == nil || s.selected["history_pagination"] {
		if err = s.history(pool); err != nil {
			return err
		}
	}
	if s.selected == nil || s.selected["pg_archive_write"] {
		return s.archive(pool)
	}
	return nil
}

const visibleSQL = " FROM danmaku_messages m WHERE m.room_id='history' AND NOT EXISTS (SELECT 1 FROM danmaku_withdrawals w WHERE w.room_id=m.room_id AND w.event_id=m.event_id) ORDER BY m.id DESC"

func (s *p1Suite) history(pool *pgxpool.Pool) error {
	n := 100000
	pages := []int{1, 200, 1000, 1800}
	if s.full {
		n = 200000
		pages = []int{1, 200, 1000, 2000, 3600}
	}
	now := time.Now()
	_, err := pool.CopyFrom(s.ctx, pgx.Identifier{"danmaku_messages"}, []string{"id", "event_id", "room_id", "user_id", "content", "send_time"}, pgx.CopyFromSlice(n, func(i int) ([]any, error) {
		return []any{i + 1, strconv.Itoa(i + 1), "history", int64(1), "弹幕", now}, nil
	}))
	if err != nil {
		return err
	}
	_, err = pool.Exec(s.ctx, "INSERT INTO danmaku_withdrawals SELECT room_id,event_id FROM danmaku_messages WHERE id%20=0; ANALYZE danmaku_messages; ANALYZE danmaku_withdrawals")
	if err != nil {
		return err
	}
	visible := make([]int, 0, n)
	for i := n; i > 0; i-- {
		if i%20 != 0 {
			visible = append(visible, i)
		}
	}
	repo := store.NewMessageRepo(pool)
	for _, page := range pages {
		offset := (page - 1) * 50
		var before uint64
		if offset > 0 {
			before = uint64(visible[offset-1])
		}
		want := make([]string, 50)
		for i := range want {
			want[i] = strconv.Itoa(visible[offset+i])
		}
		err = s.run("history_pagination", fmt.Sprintf("page_%d", page), "Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps.", []string{"offset_baseline", "cursor_before_id_prod"}, func(arm string) (measurement, error) {
			m := newMeasurement()
			ok := true
			var times []float64
			for range 6 {
				start := time.Now()
				var msgs []protocol.DanmakuMessage
				if arm == "cursor_before_id_prod" {
					msgs, _, err = repo.ListHistory(s.ctx, "history", before, 50)
				} else {
					var rows pgx.Rows
					rows, err = pool.Query(s.ctx, "SELECT m.id,m.event_id,m.room_id,m.user_id::text,m.content,m.send_time"+visibleSQL+" LIMIT 50 OFFSET $1", offset)
					if err == nil {
						msgs, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (protocol.DanmakuMessage, error) {
							var msg protocol.DanmakuMessage
							var uid string
							e := r.Scan(&msg.ID, &msg.EventID, &msg.RoomID, &uid, &msg.Content, &msg.SendTime)
							if e == nil {
								msg.UserID, e = strconv.ParseUint(uid, 10, 64)
							}
							return msg, e
						})
					}
				}
				times = append(times, micros(start))
				if err != nil {
					return m, err
				}
				ids := make([]string, len(msgs))
				for i, msg := range msgs {
					ids[i] = msg.EventID
				}
				ok = ok && slices.Equal(ids, want)
			}
			latency(&m, "", times)
			gate(&m, "ordered_visible_event_ids", ok)
			return m, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

const insertPrefix = "INSERT INTO danmaku_messages(event_id,room_id,user_id,content,send_time) VALUES "
const conflict = " ON CONFLICT(event_id) DO NOTHING"

func archiveWrite(ctx context.Context, p *pgxpool.Pool, repo *store.MessageRepo, arm string, msgs []*protocol.DanmakuMessage) (int64, error) {
	if arm == "unnest_prod" {
		return repo.SaveBatch(ctx, msgs)
	}
	argsFor := func(m *protocol.DanmakuMessage) []any {
		return []any{m.EventID, m.RoomID, strconv.FormatUint(m.UserID, 10), m.Content, m.SendTime}
	}
	if arm == "row_by_row_tx" {
		var count int64
		err := pgx.BeginFunc(ctx, p, func(tx pgx.Tx) error {
			for _, m := range msgs {
				tag, e := tx.Exec(ctx, insertPrefix+"($1,$2,$3::text::numeric,$4,$5)"+conflict, argsFor(m)...)
				if e != nil {
					return e
				}
				count += tag.RowsAffected()
			}
			return nil
		})
		return count, err
	}
	var groups []string
	var args []any
	for _, m := range msgs {
		i := len(args)
		groups = append(groups, fmt.Sprintf("($%d,$%d,$%d::text::numeric,$%d,$%d)", i+1, i+2, i+3, i+4, i+5))
		args = append(args, argsFor(m)...)
	}
	tag, err := p.Exec(ctx, insertPrefix+strings.Join(groups, ",")+conflict, args...)
	return tag.RowsAffected(), err
}
func (s *p1Suite) archive(pool *pgxpool.Pool) error {
	batches := 30
	if s.full {
		batches = 100
	}
	data := make([][]*protocol.DanmakuMessage, batches)
	unique := map[int]bool{}
	for b := range data {
		for j := range 128 {
			i := b*128 + j
			if i > 0 && i%20 == 0 {
				i--
			}
			unique[i] = true
			data[b] = append(data[b], &protocol.DanmakuMessage{RoomID: "archive", UserID: 1, EventID: strconv.Itoa(i), Content: "弹幕", SendTime: time.Now()})
		}
	}
	repo := store.NewMessageRepo(pool)
	return s.run("pg_archive_write", "batch_128", "Production UNNEST binds nine arrays per batch. The comparison inserts supply five fields per row.", []string{"row_by_row_tx", "multivalue_insert", "unnest_prod"}, func(arm string) (measurement, error) {
		m := newMeasurement()
		_, err := pool.Exec(s.ctx, "TRUNCATE danmaku_messages")
		if err != nil {
			return m, err
		}
		times := make([]float64, 0, batches)
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		start := time.Now()
		var inserted int64
		for _, batch := range data {
			t := time.Now()
			n, err := archiveWrite(s.ctx, pool, repo, arm, batch)
			if err != nil {
				return m, err
			}
			times = append(times, micros(t))
			inserted += n
		}
		elapsed := time.Since(start)
		runtime.ReadMemStats(&b)
		m.Metrics["rows_per_second"] = float64(batches*128) / elapsed.Seconds()
		m.Metrics["alloc_bytes_per_batch"] = float64(b.TotalAlloc-a.TotalAlloc) / float64(batches)
		m.Metrics["allocs_per_batch"] = float64(b.Mallocs-a.Mallocs) / float64(batches)
		latency(&m, "batch_", times)
		var count, distinct int
		err = pool.QueryRow(s.ctx, "SELECT count(*),count(DISTINCT event_id) FROM danmaku_messages").Scan(&count, &distinct)
		m.Metrics["duplicate_rows"] = float64(count - distinct)
		gate(&m, "unique_persisted_rows", count == len(unique) && distinct == count && int(inserted) == count)
		return m, err
	})
}
