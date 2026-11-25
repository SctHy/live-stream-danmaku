package hub

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

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
	h := &Hub{life: t.Context(), log: slog.New(slog.DiscardHandler), localLikes: map[string]uint64{"written": 2, "pending": 3}}
	h.store = partialStatsStore{hub: h}
	h.broadcastStats()
	if len(h.localLikes) != 1 || h.localLikes["pending"] != 4 {
		t.Fatal(h.localLikes)
	}
}

type heatRecorder struct {
	roomStore
	batches [][]pubsub.HeatBatch
}

func (s *heatRecorder) FlushHeat(_ context.Context, b []pubsub.HeatBatch) error {
	s.batches = append(s.batches, slices.Clone(b))
	if len(s.batches) == 1 {
		return errors.New("lost acknowledgement")
	}
	return nil
}
func TestFlushHeatRetriesSameBatch(t *testing.T) {
	s := new(heatRecorder)
	minute := time.Now().Unix() / 60
	h := Hub{log: slog.New(slog.DiscardHandler), store: s, localHeat: map[int64]map[string]uint64{minute: {"room": 3}}}
	h.flushHeat(t.Context())
	h.localHeat[minute] = map[string]uint64{"room": 2}
	h.flushHeat(t.Context())
	if len(h.pendingHeat) != 0 || len(s.batches) != 2 || len(s.batches[1]) != 2 {
		t.Fatal(s.batches)
	}
	old, next := s.batches[1][0], s.batches[1][1]
	if old.ID != s.batches[0][0].ID || old.Rooms["room"] != 3 || next.ID == old.ID || next.Rooms["room"] != 2 {
		t.Fatal(s.batches)
	}
}
