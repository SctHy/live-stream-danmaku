package httpapi

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scthy/maku/internal/protocol"
)

func replayRoot() string { return setting("REPLAY_STORAGE_DIR", "./data/replays") }
func videoName(path string) (string, error) {
	if strings.Contains(path, "\\") || strings.Contains("/"+path+"/", "/../") {
		return "", fs.ErrInvalid
	}
	if filepath.IsAbs(path) {
		root, err := filepath.Abs(replayRoot())
		if err != nil {
			return "", err
		}
		path, err = filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
	}
	if !fs.ValidPath(path) || !filepath.IsLocal(path) || !strings.EqualFold(filepath.Ext(path), ".mp4") {
		return "", fs.ErrInvalid
	}
	return path, nil
}
func openVideo(name string) (*os.File, fs.FileInfo, error) {
	path, err := videoName(name)
	if err != nil {
		return nil, nil, err
	}
	root, err := os.OpenRoot(replayRoot())
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	f, err := root.Open(path)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err == nil && !st.Mode().IsRegular() {
		err = fs.ErrInvalid
	}
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, st, nil
}

type replayFile struct {
	path string
	info fs.FileInfo
}

func (h *ChatHandler) video(w http.ResponseWriter, r *http.Request) {
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	id, ok := replayID(w, r)
	if !ok {
		return
	}
	for range 2 {
		var f *os.File
		var st fs.FileInfo
		cached, err := h.videos.Load(id, 30*time.Second, func() (replayFile, error) {
			var v replayFile
			if err := db.QueryRow(r.Context(), "SELECT video_path FROM room_replays WHERE id=$1 AND status='ready'", id).Scan(&v.path); err != nil {
				return v, err
			}
			var err error
			f, st, err = openVideo(v.path)
			if err != nil {
				return v, pgx.ErrNoRows
			}
			v.info = st
			return v, nil
		})
		if err != nil {
			dbError(w, err)
			return
		}
		if f == nil {
			f, st, err = openVideo(cached.path)
			// A delete on a peer may have released this path for a different replay.
			if err != nil || !os.SameFile(st, cached.info) || st.Size() != cached.info.Size() || !st.ModTime().Equal(cached.info.ModTime()) {
				if f != nil {
					f.Close()
				}
				h.videos.Invalidate(id)
				continue
			}
		}
		defer f.Close()
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
		return
	}
	w.WriteHeader(404)
}
func replayID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		w.WriteHeader(400)
		return 0, false
	}
	return id, true
}
func number(r *http.Request, key string, fallback int64) (int64, bool) {
	q := r.URL.Query()
	if !q.Has(key) {
		return fallback, true
	}
	v, err := strconv.ParseInt(q.Get(key), 10, 64)
	return v, err == nil && v >= 0
}

type replayView struct {
	ID       int64      `json:"id"`
	Session  string     `json:"session_id"`
	Title    string     `json:"title"`
	Status   string     `json:"status"`
	Started  time.Time  `json:"started_at"`
	Ended    *time.Time `json:"ended_at"`
	Duration int64      `json:"duration_ms"`
	Count    int64      `json:"danmaku_count"`
	Video    string     `json:"video_url"`
	Chunk    int        `json:"chunk_ms,omitempty"`
}

const visibleMessage = `NOT EXISTS (SELECT FROM danmaku_withdrawals w WHERE w.room_id=m.room_id AND w.event_id=m.event_id)`
const replaySelect = `SELECT p.id,p.session_id,p.title,p.status,p.started_at,p.ended_at,p.duration_ms,
 CASE WHEN p.status='ready' THEN coalesce(p.visible_count,ready_replay_count(p.id)) ELSE (SELECT count(*) FROM danmaku_messages m WHERE m.room_id=p.room_id AND (m.video_offset_ms IS NOT NULL OR m.send_time BETWEEN p.started_at AND coalesce(p.ended_at,now())) AND ` + visibleMessage + `) END FROM room_replays p WHERE p.room_slug=$1`

func scanReplay(row pgx.CollectableRow) (replayView, error) {
	var v replayView
	err := row.Scan(&v.ID, &v.Session, &v.Title, &v.Status, &v.Started, &v.Ended, &v.Duration, &v.Count)
	if v.Status == "ready" {
		v.Video = fmt.Sprintf("/api/replays/%d/video", v.ID)
	}
	return v, err
}
func (h *ChatHandler) replays(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("session") != "" && r.PathValue("session") != "replays" {
		w.WriteHeader(404)
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	ctx := r.Context()
	room := r.PathValue("room")
	if r.PathValue("id") != "" {
		id, ok := replayID(w, r)
		if !ok {
			return
		}
		if r.Method == "DELETE" {
			h.deleteReplay(w, r, id)
			return
		}
		rows, err := db.Query(ctx, replaySelect+" AND p.id=$2", room, id)
		if err != nil {
			dbError(w, err)
			return
		}
		v, err := pgx.CollectOneRow(rows, scanReplay)
		if err != nil {
			dbError(w, err)
			return
		}
		if uid, err := h.viewer(r); err == nil {
			h.recordInterest(uid, room, 1)
		}
		v.Chunk = 60000
		writeJSON(w, v)
		return
	}
	before, ok := number(r, "before_id", 0)
	limit, valid := number(r, "limit", 20)
	if !ok || !valid || limit == 0 {
		w.WriteHeader(400)
		return
	}
	limit = min(limit, 50)
	rows, err := db.Query(ctx, replaySelect+" AND ($2::bigint=0 OR p.id<$2) ORDER BY p.id DESC LIMIT $3", room, before, limit+1)
	if err != nil {
		dbError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, scanReplay)
	if err != nil {
		dbError(w, err)
		return
	}
	more := int64(len(items)) > limit
	var next int64
	if more {
		items = items[:limit]
		next = items[limit-1].ID
	}
	writeJSON(w, map[string]any{"items": items, "has_more": more, "next_before_id": next})
}
func (h *ChatHandler) deleteReplay(w http.ResponseWriter, r *http.Request, id int64) {
	a, ok := h.access(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	defer a.tx.Rollback(ctx)
	if a.role != "owner" {
		w.WriteHeader(403)
		return
	}
	var path, status string
	err := a.tx.QueryRow(ctx, "SELECT video_path,status FROM room_replays WHERE room_slug=$1 AND id=$2 FOR UPDATE", a.room.Slug, id).Scan(&path, &status)
	if err != nil {
		dbError(w, err)
		return
	}
	if status == "live" || status == "processing" {
		w.WriteHeader(409)
		return
	}
	if path != "" {
		name, err := videoName(path)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		root, err := os.OpenRoot(replayRoot())
		if err != nil {
			dbError(w, err)
			return
		}
		defer root.Close()
		// Persist the unavailable state before unlinking. An interrupted delete
		// can be retried without advertising a missing MP4 as ready.
		if _, err = a.tx.Exec(ctx, "UPDATE room_replays SET status='failed' WHERE id=$1", id); err == nil {
			err = a.tx.Commit(ctx)
		}
		if err != nil {
			dbError(w, err)
			return
		}
		h.videos.Invalidate(id)
		db := h.db()
		if db == nil {
			w.WriteHeader(503)
			return
		}
		err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			// Serialize unlink with concurrent retries. A retry that finds the
			// record gone must not touch a filename reused by a later replay.
			tag, err := tx.Exec(ctx, "DELETE FROM room_replays WHERE id=$1 AND status='failed' AND video_path=$2", id, path)
			if err != nil || tag.RowsAffected() == 0 {
				return err
			}
			err = root.Remove(name)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		})
	} else {
		_, err = a.tx.Exec(ctx, "DELETE FROM room_replays WHERE id=$1", id)
		if err == nil {
			err = a.tx.Commit(ctx)
		}
	}
	if err != nil {
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}

type timelineItem struct {
	Color   string    `json:"color"`
	Mode    string    `json:"mode"`
	ID      int64     `json:"id"`
	Event   string    `json:"event_id"`
	User    string    `json:"user_id"`
	Name    string    `json:"username"`
	Content string    `json:"content"`
	Offset  int64     `json:"video_offset_ms"`
	Created time.Time `json:"created_at"`
}

func scanItem(row pgx.Row) (timelineItem, error) {
	var v timelineItem
	err := row.Scan(&v.ID, &v.Event, &v.User, &v.Name, &v.Content, &v.Offset, &v.Created, &v.Color, &v.Mode)
	return v, err
}
func (h *ChatHandler) timeline(w http.ResponseWriter, r *http.Request) {
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	id, ok := replayID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	slug := r.PathValue("room")
	if r.Method == "POST" {
		login, ok := h.requireLogin(w, r)
		if !ok {
			return
		}
		var in struct {
			Color   string `json:"color"`
			Mode    string `json:"mode"`
			Content string `json:"content"`
			Offset  *int64 `json:"video_offset_ms"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !protocol.ValidStyle(in.Color, in.Mode) || in.Offset == nil || *in.Offset < 0 || !validText(in.Content, 1, 500) {
			w.WriteHeader(400)
			return
		}
		content, ok := h.manager.FilterMessage(in.Content)
		if !ok {
			w.WriteHeader(400)
			return
		}
		v, err := scanItem(db.QueryRow(ctx, `INSERT INTO danmaku_messages(room_id,event_id,user_id,nickname,content,video_offset_ms,send_time,color,mode)
 SELECT p.room_id,$3,$4::text::numeric,u.nickname,$5,$6,now(),coalesce(nullif($7,''),'#ffffff'),coalesce(nullif($8,''),'scroll') FROM room_replays p JOIN users u ON u.id=$4::text::numeric
 WHERE p.room_slug=$1 AND p.id=$2 AND p.status='ready' AND p.duration_ms >= $6::bigint-5000 FOR UPDATE OF p
 RETURNING id,event_id,user_id::text,nickname,content,video_offset_ms,send_time,color,mode`, slug, id, rand.Text(), strconv.FormatUint(login.UserID, 10), content, *in.Offset, in.Color, in.Mode))
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(400)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		h.recordInterest(login.UserID, slug, 2)
		writeJSON(w, v)
		return
	}
	from, validFrom := number(r, "from_ms", 0)
	to, validTo := number(r, "to_ms", from+60000)
	offset, validOffset := number(r, "after_offset_ms", -1)
	after, validID := number(r, "after_id", 0)
	limit, validLimit := number(r, "limit", 500)
	if !validFrom || !validTo || !validOffset || !validID || !validLimit || limit == 0 || to <= from || to-from > 300000 || from > math.MaxInt64-60000 || r.URL.Query().Has("after_offset_ms") != r.URL.Query().Has("after_id") {
		w.WriteHeader(400)
		return
	}
	limit = min(limit, 2000)
	var room string
	var start, end time.Time
	err := db.QueryRow(ctx, "SELECT room_id,started_at,coalesce(ended_at,now()) FROM room_replays WHERE room_slug=$1 AND id=$2", slug, id).Scan(&room, &start, &end)
	if err != nil {
		dbError(w, err)
		return
	}
	// Legacy messages already lie inside the session interval. Clamp large
	// caller offsets before converting milliseconds to a Go duration.
	boundary := func(ms int64) time.Time {
		if ms > end.Sub(start).Milliseconds() {
			return end.Add(time.Millisecond)
		}
		return start.Add(time.Duration(ms) * time.Millisecond)
	}
	rows, err := db.Query(ctx, `SELECT id,coalesce(event_id,''),user_id::text,nickname,content,off,send_time,color,mode FROM (
 SELECT id,event_id,user_id,nickname,content,video_offset_ms AS off,send_time,color,mode FROM danmaku_messages m WHERE room_id=$1 AND video_offset_ms >= $4 AND video_offset_ms < $5 AND (video_offset_ms,id)>($6,$7) AND `+visibleMessage+`
 UNION ALL SELECT id,event_id,user_id,nickname,content,floor(extract(epoch FROM (send_time-$2::timestamptz))*1000)::bigint,send_time,color,mode FROM danmaku_messages m WHERE room_id=$1 AND video_offset_ms IS NULL AND send_time BETWEEN $2 AND $3 AND send_time >= $9 AND send_time < $10 AND `+visibleMessage+`
 ) timeline WHERE off >= $4 AND off < $5 AND (off,id)>($6,$7) ORDER BY off,id LIMIT $8`, room, start, end, from, to, offset, after, limit+1, boundary(from), boundary(to))
	if err != nil {
		dbError(w, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (timelineItem, error) { return scanItem(row) })
	if err != nil {
		dbError(w, err)
		return
	}
	more := int64(len(items)) > limit
	var last timelineItem
	if more {
		items = items[:limit]
		last = items[limit-1]
	}
	writeJSON(w, map[string]any{"items": items, "has_more": more, "next_offset_ms": last.Offset, "next_after_id": last.ID})
}

func (h *ChatHandler) density(w http.ResponseWriter, r *http.Request) {
	id, ok := replayID(w, r)
	if !ok {
		return
	}
	n, ok := number(r, "buckets", 60)
	if !ok || n < 10 || n > 120 {
		w.WriteHeader(400)
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	var duration, width int64
	var counts []int64
	err := db.QueryRow(r.Context(), `WITH replay AS (
 SELECT *,greatest(1,ceil(duration_ms::numeric/$3)::bigint) width FROM room_replays WHERE room_slug=$1 AND id=$2
 ), totals AS (
 SELECT least($3::bigint-1,coalesce(m.video_offset_ms,floor(extract(epoch FROM (m.send_time-p.started_at))*1000)::bigint)/p.width) bucket,count(*) n
 FROM replay p JOIN danmaku_messages m ON m.room_id=p.room_id
 WHERE (m.video_offset_ms IS NOT NULL OR m.send_time BETWEEN p.started_at AND coalesce(p.ended_at,now())) AND `+visibleMessage+` GROUP BY 1
 ) SELECT duration_ms,width,ARRAY(SELECT coalesce(n,0) FROM generate_series(0,$3::bigint-1) i LEFT JOIN totals ON bucket=i ORDER BY i) FROM replay`, r.PathValue("room"), id, n).Scan(&duration, &width, &counts)
	if err != nil {
		dbError(w, err)
		return
	}
	var peak int64
	for _, count := range counts {
		peak = max(peak, count)
	}
	writeJSON(w, map[string]any{"duration_ms": duration, "bucket_ms": width, "buckets": counts, "max_count": peak})
}
