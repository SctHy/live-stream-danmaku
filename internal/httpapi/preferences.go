package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/recommend"
	"github.com/scthy/maku/internal/store"
)

type profileStore interface {
	Preferences(context.Context, uint64) (recommend.Profile, error)
	RecordInterest(context.Context, uint64, string, string, []string, int, time.Time) error
}

func (h *ChatHandler) viewer(r *http.Request) (uint64, error) {
	login, err := h.loginIdentity(r)
	if err == redis.Nil {
		return h.guest(r)
	}
	return login.UserID, err
}
func (h *ChatHandler) preferences(w http.ResponseWriter, r *http.Request) {
	uid, err := h.viewer(r)
	if err == redis.Nil {
		uid, err = h.ensureGuest(w, r)
	}
	if err != nil {
		dbError(w, err)
		return
	}
	s, ok := h.sessions.(profileStore)
	if !ok {
		w.WriteHeader(503)
		return
	}
	p, err := s.Preferences(r.Context(), uid)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, p)
}
func (h *ChatHandler) strategies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"categories": recommend.Categories, "strategies": h.recommendations.Strategies()})
}
func (c *plazaCache) knownRoom(slug string) store.Room {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.details[slug]
	if time.Now().Before(v.until) {
		return v.value
	}
	return store.Room{}
}
func (h *ChatHandler) recordInterest(ctx context.Context, uid uint64, room string, weight int) {
	h.scheduleInterest(ctx, room, weight, func(context.Context) uint64 { return uid })
}
func (h *ChatHandler) scheduleInterest(ctx context.Context, room string, weight int, identity func(context.Context) uint64) {
	s, ok := h.sessions.(profileStore)
	if !ok {
		return
	}
	select {
	case h.profileSlots <- struct{}{}:
	default:
		return
	}
	at := time.Now()
	var meta store.Room
	if h.db() != nil {
		meta = h.plaza.knownRoom(room)
	}
	go func() {
		defer func() { <-h.profileSlots }()
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
		defer cancel()
		uid := identity(work)
		if uid == 0 {
			return
		}
		_ = s.RecordInterest(work, uid, room, meta.Category, meta.Tags, weight, at)
	}()
}
