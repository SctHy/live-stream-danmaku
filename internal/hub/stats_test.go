package hub

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

type partialStatsStore struct {
	roomStore
	hub *Hub
}

func (s partialStatsStore) SnapshotStats(ctx context.Context, _ string, _ map[string]int, likes map[string]uint64, _ time.Duration) (map[string]protocol.StatsData, error) {
	// Simulate one acknowledged increment and an incoming like during the flush.
	delete(likes, "written")
	s.hub.likesMu.Lock()
	s.hub.localLikes["pending"]++
	s.hub.likesMu.Unlock()
	return nil, errors.New("partial pipeline")
}

func TestStatsRequeuesUnconfirmedLikes(t *testing.T) {
	h := &Hub{life: t.Context(), log: slog.New(slog.DiscardHandler)}
	h.localLikes = map[string]uint64{"written": 2, "pending": 3}
	h.store = partialStatsStore{hub: h}
	h.broadcastStats()
	if len(h.localLikes) != 1 || h.localLikes["pending"] != 4 {
		t.Fatal(h.localLikes)
	}
}

type finalLikesStore struct {
	roomStore
	flushing, resume chan struct{}
	count            uint64
}

func (s *finalLikesStore) SnapshotStats(_ context.Context, _ string, _ map[string]int, likes map[string]uint64, _ time.Duration) (map[string]protocol.StatsData, error) {
	close(s.flushing)
	<-s.resume
	s.count = likes["r"]
	return nil, nil
}
func TestDrainFreezesSharedBusLikes(t *testing.T) {
	s := &finalLikesStore{flushing: make(chan struct{}), resume: make(chan struct{})}
	life, stop := context.WithCancel(t.Context())
	h := &Hub{life: life, stop: stop, store: s, mux: &pubsub.Mux{}, started: make(chan struct{}), drained: make(chan struct{}), statsStopped: make(chan struct{}), localLikes: map[string]uint64{}, log: slog.Default()}
	var bus events.Bus
	bus.Likes.Subscribe(func(_ context.Context, e events.LikeRecorded) { h.like(e.Room, e.Count) })
	send := func() { bus.Likes.Publish(t.Context(), events.LikeRecorded{Room: "r", Count: 1}) }
	go h.Start()
	send()
	done := make(chan error, 1)
	go func() { done <- h.Drain(t.Context()) }()
	<-s.flushing
	send()
	close(s.resume)
	if err := <-done; err != nil || s.count != 1 {
		t.Fatal(s.count, err)
	}
}
