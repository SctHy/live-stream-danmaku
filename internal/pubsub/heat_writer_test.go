package pubsub

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/scthy/maku/internal/events"
)

type heatRecorder struct {
	batches [][]HeatBatch
}

func (s *heatRecorder) FlushHeat(_ context.Context, b []HeatBatch) error {
	s.batches = append(s.batches, slices.Clone(b))
	if len(s.batches) == 1 {
		return errors.New("lost acknowledgement")
	}
	return nil
}
func TestFlushHeatRetriesSameBatch(t *testing.T) {
	s := new(heatRecorder)
	minute := time.Now().Unix() / 60
	h := HeatWriter{log: slog.New(slog.DiscardHandler), store: s}
	h.localHeat = map[int64]map[string]uint64{minute: {"room": 3}}
	h.flushHeat(t.Context())
	h.localHeat[minute] = map[string]uint64{"room": 2}
	h.flushHeat(t.Context())
	if len(h.pendingHeat) != 0 || len(s.batches) != 2 || len(s.batches[1]) != 2 {
		t.Fatal(s.batches)
	}
	old, next := s.batches[1][0], s.batches[1][1]
	if old.ID != s.batches[0][0].ID || old.RoomFifths["room"] != 3 || next.ID == old.ID || next.RoomFifths["room"] != 2 {
		t.Fatal(s.batches)
	}
}

func TestHeatWithOptionalArchive(t *testing.T) {
	for _, archive := range []bool{false, true} {
		var bus events.Bus
		store := new(heatRecorder)
		heat := SubscribeHeat(&bus, store, slog.New(slog.DiscardHandler))
		var archived int
		if archive {
			bus.Danmaku.Subscribe(func(context.Context, events.DanmakuAccepted) { archived++ })
		}
		at := time.Now()
		event := events.Accepted("music/live", 1, at, "id", nil)
		bus.Danmaku.Publish(t.Context(), event)
		event.Duplicate = true
		bus.Danmaku.Publish(t.Context(), event)
		bus.Likes.Publish(t.Context(), events.LikeRecorded{Room: "music/live", At: at, Count: 2})
		heat.flushHeat(t.Context())
		if len(store.batches) != 1 || store.batches[0][0].RoomFifths["music"] != 7 || archive != (archived == 2) {
			t.Fatal(store.batches, archived)
		}
	}
}

type unavailableHeat struct{}

func (unavailableHeat) FlushHeat(context.Context, []HeatBatch) error { return errors.New("offline") }
func TestFinalHeatRetry(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		s := new(heatRecorder)
		h := SubscribeHeat(new(events.Bus), s, slog.New(slog.DiscardHandler))
		if persistent {
			h.store = unavailableHeat{}
		}
		h.increment("room", time.Now(), 5)
		ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
		err := h.drain(ctx)
		cancel()
		if persistent {
			if !errors.Is(err, context.DeadlineExceeded) || len(h.pendingHeat) != 1 {
				t.Fatal(err, h.pendingHeat)
			}
		} else if err != nil || len(s.batches) != 2 || s.batches[0][0].ID != s.batches[1][0].ID {
			t.Fatal(err, s.batches)
		}
	}
}
