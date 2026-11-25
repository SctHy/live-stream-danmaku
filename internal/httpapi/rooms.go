package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/recommend"
	"github.com/scthy/maku/internal/store"
)

type roomView struct {
	store.Room
	Role string `json:"role"`
}

type roomAccess struct {
	tx    pgx.Tx
	room  store.Room
	role  string
	actor uint64
}

func (h *ChatHandler) access(w http.ResponseWriter, r *http.Request, legacy bool) (roomAccess, bool) {
	a, err := h.loadAccess(r, legacy)
	if err != nil {
		dbError(w, err)
	}
	return a, err == nil
}

var errBusinessOffline = errors.New("business database unavailable")

func (h *ChatHandler) loadAccess(r *http.Request, legacy bool) (roomAccess, error) {
	ctx := r.Context()
	var a roomAccess
	if !legacy {
		login, err := h.loginIdentity(r)
		if err != nil && err != redis.Nil {
			return a, err
		}
		a.actor = login.UserID
	}
	db := h.db()
	if db == nil {
		return a, errBusinessOffline
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return a, err
	}
	a.tx = tx
	name := r.PathValue("room")
	a.room, err = store.LoadRoom(ctx, tx, name, r.Method != "GET")
	if errors.Is(err, pgx.ErrNoRows) {
		a.room = store.Room{Slug: name, Title: name, CoverTheme: "zinc", Category: "chat", Tags: []string{}, ActiveSession: "live", RoomControls: protocol.RoomControls{Status: "live"}}
		err = nil
	}
	if err == nil {
		a.role, err = a.room.Role(ctx, tx, a.actor)
	}
	if err != nil {
		tx.Rollback(ctx)
		return a, err
	}
	if legacy {
		a.role = "owner"
	}
	return a, nil
}

var numericRoom = regexp.MustCompile(`^[0-9]{1,20}$`)

func normalizeTags(tags []string) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		if !validText(tag, 1, 16) || len(out) == 5 {
			return nil, false
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out, true
}
func validRoom(v store.Room) bool {
	switch v.CoverTheme {
	case "zinc", "emerald", "ocean", "amber", "violet", "rose":
	default:
		return false
	}
	return recommend.CategoryName(v.Category) != "" && slug.MatchString(v.Slug) && validText(v.Title, 1, 100) && validText(v.Description, 0, 1000) && validText(v.Notice, 0, 1000) && slug.MatchString(v.ActiveSession) && (v.Bilibili == "" || numericRoom.MatchString(v.Bilibili)) && (v.Status == "live" || v.Status == "offline") && v.Slow >= 0 && v.Slow <= 300
}
func (h *ChatHandler) rooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	generation := h.plaza.epoch()
	if r.Method == "POST" {
		login, ok := h.requireLogin(w, r)
		if !ok {
			return
		}
		var in struct {
			Slug, Title, Description string
			CoverTheme               *string `json:"cover_theme"`
			Category                 *string
			Tags                     []string
			Bilibili                 string `json:"bilibili_room_id"`
		}
		if !decode(w, r, &in) {
			return
		}
		v := store.Room{Slug: in.Slug, Title: in.Title, Description: in.Description, Bilibili: in.Bilibili, OwnerID: login.UserID, ActiveSession: "live", RoomControls: protocol.RoomControls{Status: "live"}}
		v.CoverTheme = "zinc"
		if in.CoverTheme != nil {
			v.CoverTheme = *in.CoverTheme
		}
		v.Category = "chat"
		if in.Category != nil {
			v.Category = *in.Category
		}
		v.Tags, ok = normalizeTags(in.Tags)
		if !ok || !validRoom(v) {
			w.WriteHeader(400)
			return
		}
		db := h.db()
		if db == nil {
			w.WriteHeader(503)
			return
		}
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			err := tx.QueryRow(ctx, "INSERT INTO rooms(slug,owner_id,title,description,bilibili_room_id,category,tags,cover_theme) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at,updated_at", v.Slug, v.OwnerID, v.Title, v.Description, v.Bilibili, v.Category, v.Tags, v.CoverTheme).Scan(&v.CreatedAt, &v.UpdatedAt)
			if err != nil {
				return err
			}
			return (store.Audit{Action: "create_room", Actor: v.OwnerID, Target: v.OwnerID}).Write(ctx, tx, v.Slug, v.ActiveSession)
		})
		if err == nil {
			h.invalidateRoom(ctx, v.Slug)
			h.plaza.remember(generation+1, v)
			err = h.flush(ctx, db, v.Slug)
		}
		if err != nil {
			dbError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		writeJSON(w, roomView{v, "owner"})
		return
	}
	a, ok := h.access(w, r, false)
	if !ok {
		return
	}
	defer a.tx.Rollback(ctx)
	v := a.room
	if r.Method == "GET" {
		h.plaza.remember(generation, v)
		var count int64
		var following bool
		if err := a.tx.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE user_id=$2)>0 FROM room_follows WHERE room_slug=$1", v.Slug, a.actor).Scan(&count, &following); err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, struct {
			roomView
			Count     int64 `json:"follower_count"`
			Following bool  `json:"following"`
		}{roomView{v, a.role}, count, following})
		return
	}
	if a.role != "owner" {
		w.WriteHeader(403)
		return
	}
	var fields map[string]json.RawMessage
	if !decode(w, r, &fields) {
		return
	}
	dest := map[string]any{"cover_theme": &v.CoverTheme, "category": &v.Category, "tags": &v.Tags, "title": &v.Title, "description": &v.Description, "bilibili_room_id": &v.Bilibili, "active_session": &v.ActiveSession, "slow_mode_seconds": &v.Slow, "mute_all": &v.MuteAll, "status": &v.Status, "notice": &v.Notice}
	for k, raw := range fields {
		p, ok := dest[k]
		if !ok || string(raw) == "null" || json.Unmarshal(raw, p) != nil {
			w.WriteHeader(400)
			return
		}
	}
	v.Tags, ok = normalizeTags(v.Tags)
	if !ok || len(fields) == 0 || !validRoom(v) {
		w.WriteHeader(400)
		return
	}
	err := v.Save(ctx, a.tx)
	if err == nil && v.RoomControls != a.room.RoomControls {
		detail, e := json.Marshal(v.RoomControls)
		if e != nil {
			dbError(w, e)
			return
		}
		err = h.commitAudit(ctx, a.tx, store.Audit{Actor: a.actor, Action: "room_update", Detail: string(detail)}, v.Slug, v.ActiveSession)
	} else if err == nil {
		err = a.tx.Commit(ctx)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	h.invalidateRoom(ctx, v.Slug)
	h.plaza.remember(generation+1, v)
	writeJSON(w, roomView{v, "owner"})
}

type moderator struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func (h *ChatHandler) moderators(w http.ResponseWriter, r *http.Request) {
	a, ok := h.access(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	defer a.tx.Rollback(ctx)
	if a.role == "viewer" || (r.Method != "GET" && a.role != "owner") {
		w.WriteHeader(403)
		return
	}
	audit := store.Audit{Actor: a.actor, Action: "list_moderators"}
	session := ""
	var out []moderator
	var err error
	if r.Method == "GET" {
		rows, e := a.tx.Query(ctx, `SELECT u.id::bigint,u.username,u.nickname FROM room_moderators m JOIN users u ON u.id=m.user_id WHERE room_slug=$1 ORDER BY u.username`, a.room.Slug)
		if e != nil {
			dbError(w, e)
			return
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[moderator])
	} else {
		grant := r.Method == "POST"
		var id uint64
		if grant {
			var in struct {
				Username string
				UserID   uint64 `json:"user_id"`
			}
			if !decode(w, r, &in) {
				return
			}
			if (in.Username == "") == (in.UserID == 0) {
				w.WriteHeader(400)
				return
			}
			err = a.tx.QueryRow(ctx, "SELECT id::bigint FROM users WHERE username=$1 OR id=$2::text::numeric", in.Username, strconv.FormatUint(in.UserID, 10)).Scan(&id)
		} else {
			id, err = strconv.ParseUint(r.PathValue("user_id"), 10, 64)
			if err != nil || id == 0 {
				w.WriteHeader(400)
				return
			}
		}
		if err != nil {
			dbError(w, err)
			return
		}
		if id == a.room.OwnerID {
			w.WriteHeader(400)
			return
		}
		audit.Target = id
		if grant {
			audit.Action = "grant_moderator"
			_, err = a.tx.Exec(ctx, "INSERT INTO room_moderators(room_slug,user_id,granted_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", a.room.Slug, id, a.actor)
		} else {
			audit.Action = "revoke_moderator"
			_, err = a.tx.Exec(ctx, "DELETE FROM room_moderators WHERE room_slug=$1 AND user_id=$2::text::numeric", a.room.Slug, strconv.FormatUint(id, 10))
		}
		if err == nil {
			session = a.room.ActiveSession
		}
	}
	if err == nil {
		err = h.commitAudit(ctx, a.tx, audit, a.room.Slug, session)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if r.Method == "GET" {
		writeJSON(w, map[string]any{"moderators": out})
		return
	}
	w.WriteHeader(204)
}
func (h *ChatHandler) moderationList(w http.ResponseWriter, r *http.Request) {
	legacy := bearer(r)
	a, ok := h.access(w, r, legacy)
	if !ok {
		return
	}
	ctx := r.Context()
	defer a.tx.Rollback(ctx)
	if a.role == "viewer" {
		w.WriteHeader(403)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/mutes") {
		v, e := h.sessions.Mutes(ctx, roomID(r))
		if e != nil {
			dbError(w, e)
			return
		}
		writeJSON(w, map[string]any{"mutes": v})
		return
	}
	q := r.URL.Query()
	var before int64
	limit := 50
	var err error
	if q.Has("before_id") {
		before, err = strconv.ParseInt(q.Get("before_id"), 10, 64)
		if err != nil || before < 0 {
			w.WriteHeader(400)
			return
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			w.WriteHeader(400)
			return
		}
	}
	v, e := store.Audits(ctx, a.tx, a.room.Slug, before, limit+1)
	if e != nil {
		dbError(w, e)
		return
	}
	more := len(v) > limit
	var next int64
	if more {
		v = v[:limit]
		next = v[limit-1].ID
	}
	writeJSON(w, map[string]any{"audits": v, "has_more": more, "next_before_id": next})
}
