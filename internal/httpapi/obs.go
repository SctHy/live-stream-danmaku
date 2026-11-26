package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

type obsStore interface {
	OBS(context.Context, string) (pubsub.OBSStream, error)
	SetOBS(context.Context, string, pubsub.OBSStream) error
}

func setting(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func hlsURL(room string) string {
	return strings.TrimRight(setting("OBS_HLS_BASE_URL", "/hls"), "/") + "/live/" + room + "/index.m3u8"
}
func (h *ChatHandler) streamKey(w http.ResponseWriter, r *http.Request) {
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
	var key string
	err := a.tx.QueryRow(ctx, `UPDATE rooms SET stream_key=CASE WHEN stream_key='' OR $2 THEN $3 ELSE stream_key END WHERE slug=$1 RETURNING stream_key`, a.room.Slug, r.Method == "POST", rand.Text()+rand.Text()).Scan(&key)
	if err == nil {
		err = a.tx.Commit(ctx)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	server := strings.TrimRight(setting("OBS_RTMP_URL", "rtmp://127.0.0.1:1935/live"), "/")
	suffix := a.room.Slug + "?key=" + key
	writeJSON(w, map[string]string{"obs_server": server, "obs_stream_key": suffix, "stream_key": key, "rtmp_url": server + "/" + suffix, "hls_url": hlsURL(a.room.Slug)})
}

var publisherID = regexp.MustCompile(`^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)

// MediaMTX expands %path, but does not expand the publisher ID in recordPath.
// Bind the path before acknowledging publish so even the first native segment
// belongs to the right session. Competing admissions cannot replace this link.
func reserveCapture(root, room, id string) error {
	if !publisherID.MatchString(id) {
		return os.ErrInvalid
	}
	target := filepath.Join("..", "sessions", id)
	link := filepath.Join(root, "live", room)
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, os.ErrExist) {
			if current, e := os.Readlink(link); e == nil && current == target {
				return nil
			}
		}
		return err
	}
	session := filepath.Join(root, "sessions", id)
	err := os.Mkdir(session, 0755)
	if err == nil {
		err = os.WriteFile(filepath.Join(session, "room"), []byte(room), 0644)
	}
	if err != nil {
		return errors.Join(err, os.Remove(link))
	}
	return nil
}

func (h *ChatHandler) streamAuth(w http.ResponseWriter, r *http.Request) {
	var in struct{ Action, Path, Query, ID string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in) != nil {
		w.WriteHeader(400)
		return
	}
	room, ok := strings.CutPrefix(in.Path, "live/")
	if !ok || !slug.MatchString(room) {
		w.WriteHeader(403)
		return
	}
	if in.Action == "read" {
		return
	}
	q, err := url.ParseQuery(in.Query)
	if in.Action != "publish" || err != nil || q.Get("key") == "" {
		w.WriteHeader(403)
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	var key string
	err = db.QueryRow(r.Context(), "SELECT stream_key FROM rooms WHERE slug=$1", room).Scan(&key)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("key")), []byte(key)) != 1 {
		w.WriteHeader(403)
		return
	}
	if root := os.Getenv("OBS_CAPTURE_DIR"); root != "" {
		if err := reserveCapture(root, room, in.ID); err != nil {
			if errors.Is(err, os.ErrExist) {
				w.WriteHeader(409)
			} else if errors.Is(err, os.ErrInvalid) {
				w.WriteHeader(400)
			} else {
				dbError(w, err)
			}
		}
	}
}

type obsEvent struct {
	protocol.RoomControls
	Stream pubsub.OBSStream `json:"stream"`
}

type streamNotice struct {
	Room     string `json:"room_slug"`
	ReplayID int64  `json:"replay_id"`
	Source   string `json:"source_id"`
	File     string `json:"video_file"`
	Duration *int64 `json:"duration_ms"`
}

type endedReplay struct {
	ID       int64  `json:"replay_id"`
	Duration int64  `json:"duration_ms"`
	Status   string `json:"status"`
}

func (h *ChatHandler) streamHook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("STREAM_WEBHOOK_SECRET")
	if (secret == "" && !loopback(r.RemoteAddr)) || (secret != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Webhook-Secret")), []byte(secret)) != 1) {
		w.WriteHeader(403)
		return
	}
	var in streamNotice
	if !decode(w, r, &in) {
		return
	}
	if !slug.MatchString(in.Room) || in.ReplayID < 0 || (in.Duration != nil && *in.Duration < 0) {
		w.WriteHeader(400)
		return
	}
	ready := strings.HasSuffix(r.URL.Path, "/ready")
	if !ready && in.File != "" {
		path, err := videoName(in.File)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		f, _, err := openVideo(path)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		if err = f.Close(); err != nil {
			dbError(w, err)
			return
		}
		in.File = path
	}
	ctx := r.Context()
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	var active pubsub.OBSStream
	var ended endedReplay
	var stateChanged bool
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		room, err := store.LoadRoom(ctx, tx, in.Room, true)
		if err != nil {
			return err
		}
		old := room.ActiveSession
		if ready {
			changed, err := in.start(ctx, tx, &room, &active)
			if err != nil || !changed {
				return err
			}
		} else {
			changed, err := in.end(ctx, tx, &room, &ended)
			if err != nil || !changed {
				return err
			}
		}
		stateChanged = true
		_, err = tx.Exec(ctx, "UPDATE rooms SET active_session=$2,status=$3,stream_mode='obs',updated_at=now() WHERE slug=$1", in.Room, room.ActiveSession, room.Status)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(obsEvent{room.RoomControls, active})
		if err != nil {
			return err
		}
		audit := store.Audit{Action: "obs_state", Detail: string(raw)}
		if old != room.ActiveSession {
			if err = audit.Write(ctx, tx, in.Room, old); err != nil {
				return err
			}
		}
		return audit.Write(ctx, tx, in.Room, room.ActiveSession)
	})
	if err == nil {
		if stateChanged {
			id := active.ReplayID
			if !ready {
				id = ended.ID
			}
			h.bus.OBS.Publish(ctx, events.OBSStreamChanged{Room: in.Room, Live: ready, ReplayID: id})
		}
		err = h.flush(ctx, db, in.Room)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if ready {
		writeJSON(w, active)
	} else {
		writeJSON(w, ended)
	}
}

func (in streamNotice) start(ctx context.Context, tx pgx.Tx, room *store.Room, active *pubsub.OBSStream) (bool, error) {
	source := in.Source
	if source == "" {
		source = rand.Text()
	}
	hash := sha256.Sum256([]byte(source))
	active.Session = "obs-" + hex.EncodeToString(hash[:10])
	active.RoomID = in.Room + "/" + active.Session
	var started time.Time
	var status string
	err := tx.QueryRow(ctx, `INSERT INTO room_replays(room_slug,session_id,room_id,title) VALUES($1,$2,$3,$4) ON CONFLICT(room_id) DO UPDATE SET room_id=excluded.room_id RETURNING id,started_at,status`, in.Room, active.Session, active.RoomID, room.Title).Scan(&active.ReplayID, &started, &status)
	if err != nil {
		return false, err
	}
	active.Started = started.UnixMilli()
	active.HLS = hlsURL(in.Room)
	if room.ActiveSession == active.Session || status != "live" {
		return false, nil
	}
	_, err = tx.Exec(ctx, "UPDATE room_replays SET status='processing',ended_at=now() WHERE room_slug=$1 AND id<>$2 AND status='live'", in.Room, active.ReplayID)
	if err != nil {
		return false, err
	}
	room.ActiveSession = active.Session
	room.Status = "live"
	return true, nil
}
func (in streamNotice) end(ctx context.Context, tx pgx.Tx, room *store.Room, ended *endedReplay) (bool, error) {
	var session string
	ended.Status = "failed"
	if in.File != "" {
		ended.Status = "ready"
	}
	err := tx.QueryRow(ctx, `UPDATE room_replays SET status=$3, ended_at=coalesce(ended_at,now()), video_path=$5,
 duration_ms=coalesce($4::bigint,greatest(0,floor(extract(epoch FROM (coalesce(ended_at,now())-started_at))*1000)::bigint))
 WHERE id=(SELECT id FROM room_replays WHERE room_slug=$1 AND (($2::bigint=0 AND status='live') OR id=$2)
           ORDER BY id DESC LIMIT 1)
 AND (status IN ('live','processing') OR (status='failed' AND video_path=''))
 RETURNING session_id,id,duration_ms`, in.Room, in.ReplayID, ended.Status, in.Duration, in.File).Scan(&session, &ended.ID, &ended.Duration)
	if errors.Is(err, pgx.ErrNoRows) && in.ReplayID > 0 {
		return false, tx.QueryRow(ctx, "SELECT session_id,id,duration_ms FROM room_replays WHERE id=$1 AND room_slug=$2 AND status=$3 AND video_path=$4", in.ReplayID, in.Room, ended.Status, in.File).Scan(&session, &ended.ID, &ended.Duration)
	}
	if err != nil || session != room.ActiveSession {
		return false, err
	}
	room.Status = "offline"
	return true, nil
}
func (h *ChatHandler) stream(w http.ResponseWriter, r *http.Request) {
	room := r.PathValue("room")
	ctx := r.Context()
	var active pubsub.OBSStream
	if cache, ok := h.sessions.(obsStore); ok {
		active, _ = cache.OBS(ctx, room)
	}
	bili := room
	if active.ReplayID == 0 {
		if db := h.db(); db != nil {
			var bound string
			var id *int64
			var started *time.Time
			err := db.QueryRow(ctx, `SELECT r.bilibili_room_id,r.active_session,p.id,p.started_at FROM rooms r LEFT JOIN room_replays p ON p.room_slug=r.slug AND p.session_id=r.active_session AND p.status='live' WHERE r.slug=$1`, room).Scan(&bound, &active.Session, &id, &started)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				dbError(w, err)
				return
			}
			if bound != "" {
				bili = bound
			}
			if id != nil {
				active.ReplayID = *id
				active.Started = started.UnixMilli()
				active.HLS = hlsURL(room)
			}
		}
	}
	if active.ReplayID != 0 {
		writeJSON(w, map[string]any{"mode": "obs", "live": true, "session": active.Session, "replay_id": active.ReplayID, "started_at_ms": active.Started, "sources": map[string]string{"hls": active.HLS}})
		return
	}
	mode := "none"
	result := liveResult{Sources: map[string]string{}}
	if id, err := strconv.ParseUint(bili, 10, 63); err == nil && id > 0 {
		mode = "bilibili"
		result, err = h.liveResolver.resolve(ctx, id)
		if err != nil {
			http.Error(w, "直播源暂不可用", 502)
			return
		}
	}
	writeJSON(w, map[string]any{"mode": mode, "live": result.Live, "sources": result.Sources})
}

func (h *ChatHandler) applyOBSState(ctx context.Context, slug, detail string) (obsEvent, error) {
	var v obsEvent
	if err := json.Unmarshal([]byte(detail), &v); err != nil {
		return v, err
	}
	cache, ok := h.sessions.(obsStore)
	if !ok {
		return v, errors.New("OBS store unavailable")
	}
	if err := cache.SetOBS(ctx, slug, v.Stream); err != nil {
		return v, err
	}
	if err := h.sessions.SetControls(ctx, slug, v.RoomControls); err != nil {
		return v, err
	}
	return v, nil
}
