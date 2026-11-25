package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/recommend"
	"github.com/scthy/maku/internal/store"
)

type profileStore interface {
	Preferences(context.Context, uint64) (recommend.Profile, error)
	RecordInterests(context.Context, []pubsub.InterestDelta) error
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
	p, err := h.readProfile(r.Context(), s, uid)
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

const interestLimit = 4096

type interestBuffer struct {
	mu      sync.Mutex
	users   map[uint64]map[string]pubsub.InterestDelta
	size    int
	writing map[uint64]map[string]pubsub.InterestDelta
	// Serializes detached batches, so a reader also waits for in-flight writes.
	flush chan struct{}
}

func newInterestBuffer() interestBuffer {
	return interestBuffer{users: make(map[uint64]map[string]pubsub.InterestDelta), flush: make(chan struct{}, 1)}
}

func (h *ChatHandler) recordInterest(uid uint64, room string, weight int) {
	if _, ok := h.sessions.(profileStore); !ok || uid == 0 {
		return
	}
	b := &h.interests
	b.mu.Lock()
	defer b.mu.Unlock()
	rooms := b.users[uid]
	d, exists := rooms[room]
	if !exists {
		if b.size >= interestLimit {
			return
		}
		if rooms == nil {
			rooms = make(map[string]pubsub.InterestDelta)
			b.users[uid] = rooms
		}
		d.UserID = uid
		d.Room = room
		b.size++
	}
	d.Weight += weight
	d.At = time.Now()
	rooms[room] = d
}

func (h *ChatHandler) readProfile(ctx context.Context, s profileStore, uid uint64) (recommend.Profile, error) {
	b := &h.interests
	b.mu.Lock()
	pending := len(b.users[uid]) > 0 || len(b.writing[uid]) > 0
	b.mu.Unlock()
	if pending {
		if err := h.flushInterests(ctx, s, uid); err != nil {
			return recommend.Profile{}, err
		}
	}
	return s.Preferences(ctx, uid)
}
func (h *ChatHandler) flushInterests(ctx context.Context, s profileStore, uid uint64) error {
	b := &h.interests
	select {
	case b.flush <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-b.flush }()
	b.mu.Lock()
	var users map[uint64]map[string]pubsub.InterestDelta
	if uid == 0 {
		users = b.users
		b.users = make(map[uint64]map[string]pubsub.InterestDelta)
		b.size = 0
	} else if rooms := b.users[uid]; rooms != nil {
		users = map[uint64]map[string]pubsub.InterestDelta{uid: rooms}
		delete(b.users, uid)
		b.size -= len(rooms)
	}
	b.writing = users
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.writing = nil; b.mu.Unlock() }()
	if len(users) == 0 {
		return nil
	}
	var batch []pubsub.InterestDelta
	metadata := make(map[string]store.Room)
	for _, rooms := range users {
		for room, d := range rooms {
			metadata[room] = store.Room{}
			batch = append(batch, d)
		}
	}
	now := time.Now()
	h.plaza.mu.Lock()
	for slug := range metadata {
		if cached := h.plaza.details[slug]; now.Before(cached.until) {
			metadata[slug] = cached.value
		}
	}
	h.plaza.mu.Unlock()
	for i, d := range batch {
		meta := metadata[d.Room]
		batch[i].Category = meta.Category
		batch[i].Tags = meta.Tags
	}
	return s.RecordInterests(ctx, batch)
}

// RunInterestWriter drains pending interactions when its serving lifecycle ends.
func (h *ChatHandler) RunInterestWriter(ctx context.Context) {
	s, ok := h.sessions.(profileStore)
	if !ok {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
		}
		stopping := ctx.Err() != nil
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		err := h.flushInterests(work, s, 0)
		cancel()
		if err != nil {
			slog.Warn("Flush interest profiles", "error", err)
		}
		if stopping {
			return
		}
	}
}
