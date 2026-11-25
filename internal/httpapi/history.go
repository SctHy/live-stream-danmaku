package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/store"
)

func (h *ChatHandler) recent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	out, err := h.sessions.Recover(ctx, roomID(r), r.URL.Query().Get("after"))
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, out)
}
func bearer(r *http.Request) bool {
	key := os.Getenv("DANMAKU_MOD_TOKEN")
	return key != "" && os.Getenv("DANMAKU_MOD_ROOM") == r.PathValue("room") && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) == 1
}
func (h *ChatHandler) moderate(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		w.WriteHeader(403)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	legacy := bearer(r)
	if !legacy {
		if _, err := r.Cookie(loginCookie); err != nil {
			w.WriteHeader(403)
			return
		}
	}
	var in struct {
		Action  string
		EventID string `json:"event_id"`
		UserID  uint64 `json:"user_id"`
		Seconds *int
		Enabled *bool
		Notice  *string
	}
	if !decode(w, r, &in) {
		return
	}
	room := roomID(r)
	a := roomAccess{role: "owner"}
	// The original bearer can mute during a database outage and operate in temporary rooms.
	if !legacy || h.db() != nil {
		// Leave time for the legacy Redis mute even when a stale PG pool cannot connect.
		lookup, cancelLookup := context.WithTimeout(ctx, time.Second)
		access, e := h.loadAccess(r.WithContext(lookup), legacy)
		// pgx only binds Begin to lookup. Queries and commit use the request context.
		cancelLookup()
		if e == nil {
			a = access
			defer a.tx.Rollback(ctx)
		} else if !legacy || (in.Action != "mute" && in.Action != "unmute") {
			dbError(w, e)
			return
		}
	}
	if a.role == "viewer" {
		w.WriteHeader(403)
		return
	}
	audit := store.Audit{Actor: a.actor, Action: in.Action, Target: in.UserID, Event: in.EventID}
	var err error
	switch in.Action {
	case "mute", "unmute":
		var ttl time.Duration
		if in.UserID == 0 {
			w.WriteHeader(400)
			return
		}
		if in.Action == "mute" {
			if in.Seconds == nil || *in.Seconds < 1 || *in.Seconds > 86400 {
				w.WriteHeader(400)
				return
			}
			ttl = time.Duration(*in.Seconds) * time.Second
			audit.Detail = strconv.Itoa(*in.Seconds)
		}
		if a.role == "moderator" {
			role, e := a.room.Role(ctx, a.tx, in.UserID)
			if e != nil {
				dbError(w, e)
				return
			}
			if role != "viewer" {
				w.WriteHeader(403)
				return
			}
		}
		if a.tx == nil {
			err = h.sessions.Mute(ctx, room, in.UserID, ttl)
		}
	case "withdraw":
		if len(in.EventID) < 26 || len(in.EventID) > 80 || strings.ContainsRune(in.EventID, 0) {
			w.WriteHeader(400)
			return
		}
		if a.tx != nil {
			_, err = a.tx.Exec(ctx, "INSERT INTO danmaku_withdrawals(room_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING", room, in.EventID)
		} else if h.history != nil {
			err = h.history.Withdraw(ctx, room, in.EventID)
		} else {
			w.WriteHeader(503)
			return
		}
		if err == nil && a.tx == nil {
			data, e := json.Marshal(protocol.Withdraw{EventID: in.EventID})
			if e != nil {
				dbError(w, e)
				return
			}
			packet, e := json.Marshal(protocol.WsPacket{Type: protocol.TypeWithdraw, RoomID: room, Data: data})
			if e != nil {
				dbError(w, e)
				return
			}
			err = h.sessions.Journal(ctx, room, packet)
		}
	case "mute_all", "slow_mode", "notice":
		if a.tx == nil {
			w.WriteHeader(503)
			return
		}
		if a.room.OwnerID == 0 {
			w.WriteHeader(404)
			return
		}
		switch in.Action {
		case "mute_all":
			if in.Enabled == nil {
				w.WriteHeader(400)
				return
			}
			a.room.MuteAll = *in.Enabled
		case "slow_mode":
			if in.Seconds == nil || *in.Seconds < 0 || *in.Seconds > 300 {
				w.WriteHeader(400)
				return
			}
			a.room.Slow = *in.Seconds
		case "notice":
			if in.Notice == nil || !validText(*in.Notice, 0, 1000) {
				w.WriteHeader(400)
				return
			}
			a.room.Notice = *in.Notice
		}
		err = a.room.Save(ctx, a.tx)
		if err == nil {
			var data []byte
			data, err = json.Marshal(a.room.RoomControls)
			audit.Detail = string(data)
		}
	default:
		w.WriteHeader(400)
		return
	}
	if err == nil && a.tx != nil {
		err = h.commitAudit(ctx, a.tx, audit, r.PathValue("room"), r.PathValue("session"))
	}
	if err != nil {
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *ChatHandler) listHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	var before uint64
	limit := 50
	if q.Has("before_id") {
		before, err = strconv.ParseUint(q.Get("before_id"), 10, 64)
		if err != nil {
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
	msgs, more, err := h.history.ListHistory(r.Context(), roomID(r), before, limit)
	if err != nil {
		w.WriteHeader(503)
		return
	}
	if msgs == nil {
		msgs = []protocol.DanmakuMessage{}
	}
	var next uint64
	if more && len(msgs) > 0 {
		next = msgs[len(msgs)-1].ID
	}
	writeJSON(w, map[string]any{"messages": msgs, "has_more": more, "next_before_id": next})
}
