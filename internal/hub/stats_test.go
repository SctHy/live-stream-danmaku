package hub

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/scthy/maku/internal/protocol"
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
