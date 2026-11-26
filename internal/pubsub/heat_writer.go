package pubsub

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scthy/maku/internal/events"
)

type heatStore interface {
	FlushHeat(context.Context, []HeatBatch) error
}
type HeatWriter struct {
	store       heatStore
	log         *slog.Logger
	mu          sync.Mutex
	localHeat   map[int64]map[string]uint64
	pendingHeat []HeatBatch // owned by Run
}

func SubscribeHeat(bus *events.Bus, store heatStore, log *slog.Logger) *HeatWriter {
	h := &HeatWriter{store: store, log: log, localHeat: make(map[int64]map[string]uint64)}
	bus.Danmaku.Subscribe(func(_ context.Context, e events.DanmakuAccepted) {
		if !e.Duplicate {
			h.increment(e.Room, e.At, 5)
		}
	})
	bus.Likes.Subscribe(func(_ context.Context, e events.LikeRecorded) { h.increment(e.Room, e.At, e.Count) })
	return h
}
func (h *HeatWriter) Run(ctx context.Context) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
		}
		stopping := ctx.Err() != nil
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if stopping {
			err := h.drain(work)
			cancel()
			return err
		}
		h.flushHeat(work)
		cancel()
	}
}
func (m *HeatWriter) increment(roomID string, at time.Time, fifths uint64) {
	room, _, _ := strings.Cut(roomID, "/")
	minute := at.Unix() / 60
	m.mu.Lock()
	if m.localHeat[minute] == nil {
		m.localHeat[minute] = make(map[string]uint64)
	}
	m.localHeat[minute][room] += fifths
	m.mu.Unlock()
}
func (h *HeatWriter) drain(ctx context.Context) error {
	err := h.flushHeat(ctx)
	for delay := 25 * time.Millisecond; err != nil; delay = min(2*delay, 250*time.Millisecond) {
		select {
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		case <-time.After(delay):
		}
		err = h.flushPending(ctx)
	}
	return nil
}
func (m *HeatWriter) flushHeat(ctx context.Context) error {
	m.mu.Lock()
	heat := m.localHeat
	m.localHeat = make(map[int64]map[string]uint64)
	m.mu.Unlock()
	for minute, rooms := range heat {
		m.pendingHeat = append(m.pendingHeat, HeatBatch{Minute: minute, RoomFifths: rooms, ID: rand.Text()})
	}
	cutoff := time.Now().Unix()/60 - 9
	m.pendingHeat = slices.DeleteFunc(m.pendingHeat, func(b HeatBatch) bool { return b.Minute < cutoff })
	return m.flushPending(ctx)
}
func (m *HeatWriter) flushPending(ctx context.Context) error {
	if len(m.pendingHeat) == 0 {
		return nil
	}
	if err := m.store.FlushHeat(ctx, m.pendingHeat); err != nil {
		m.log.Warn("Flush room heat", "error", err)
		return err
	}
	m.pendingHeat = nil
	return nil
}
