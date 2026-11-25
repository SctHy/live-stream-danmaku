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
	var path string
	err := db.QueryRow(r.Context(), "SELECT video_path FROM room_replays WHERE id=$1 AND status='ready'", id).Scan(&path)
	if err != nil {
		dbError(w, err)
		return
	}
	f, st, err := openVideo(path)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
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
 (SELECT count(*) FROM danmaku_messages m WHERE m.room_id=p.room_id AND (m.video_offset_ms IS NOT NULL OR m.send_time BETWEEN p.started_at AND coalesce(p.ended_at,now())) AND ` + visibleMessage + `) FROM room_replays p WHERE p.room_slug=$1`

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
	err := row.Scan(&v.ID, &v.Event, &v.User, &v.Name, &v.Content, &v.Offset, &v.Created)
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
			Content string `json:"content"`
			Offset  *int64 `json:"video_offset_ms"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Offset == nil || *in.Offset < 0 || !validText(in.Content, 1, 500) {
			w.WriteHeader(400)
			return
		}
		content, ok := h.manager.FilterMessage(in.Content)
		if !ok {
			w.WriteHeader(400)
			return
		}
		v, err := scanItem(db.QueryRow(ctx, `INSERT INTO danmaku_messages(room_id,event_id,user_id,nickname,content,video_offset_ms,send_time)
 SELECT p.room_id,$3,$4::text::numeric,u.nickname,$5,$6,now() FROM room_replays p JOIN users u ON u.id=$4::text::numeric
 WHERE p.room_slug=$1 AND p.id=$2 AND p.status='ready' AND p.duration_ms >= $6::bigint-5000 FOR SHARE OF p
 RETURNING id,event_id,user_id::text,nickname,content,video_offset_ms,send_time`, slug, id, rand.Text(), strconv.FormatUint(login.UserID, 10), content, *in.Offset))
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(400)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
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
	rows, err := db.Query(ctx, `SELECT id,coalesce(event_id,''),user_id::text,nickname,content,off,send_time FROM (
 SELECT id,event_id,user_id,nickname,content,video_offset_ms AS off,send_time FROM danmaku_messages m WHERE room_id=$1 AND video_offset_ms IS NOT NULL AND `+visibleMessage+`
 UNION ALL SELECT id,event_id,user_id,nickname,content,floor(extract(epoch FROM (send_time-$2::timestamptz))*1000)::bigint,send_time FROM danmaku_messages m WHERE room_id=$1 AND video_offset_ms IS NULL AND send_time BETWEEN $2 AND $3 AND `+visibleMessage+`
 ) timeline WHERE off >= $4 AND off < $5 AND (off,id)>($6,$7) ORDER BY off,id LIMIT $8`, room, start, end, from, to, offset, after, limit+1)
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
