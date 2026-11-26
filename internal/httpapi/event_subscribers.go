package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/store"
)

func (h *ChatHandler) SubscribeInterests() {
	b := h.bus
	b.Danmaku.Subscribe(func(_ context.Context, e events.DanmakuAccepted) {
		if e.Duplicate {
			return
		}
		room, _, _ := strings.Cut(e.Room, "/")
		h.recordInterest(e.UserID, room, 2)
	})
	b.Likes.Subscribe(func(_ context.Context, e events.LikeRecorded) {
		room, _, _ := strings.Cut(e.Room, "/")
		h.recordInterest(e.UserID, room, 1)
	})
	b.Gifts.Subscribe(func(_ context.Context, e events.GiftOrdered) { h.recordInterest(e.UserID, e.Room, 5) })
	b.Views.Subscribe(func(_ context.Context, e events.RoomViewed) { h.recordInterest(e.UserID, e.Room, 1) })
	b.ReplayComments.Subscribe(func(_ context.Context, e events.ReplayCommented) { h.recordInterest(e.UserID, e.Room, 2) })
}
func (h *ChatHandler) SubscribeCaches() {
	b := h.bus
	b.Rooms.Subscribe(func(ctx context.Context, e events.RoomStateChanged) {
		var snapshot store.Room
		if e.Snapshot != "" {
			if err := json.Unmarshal([]byte(e.Snapshot), &snapshot); err != nil {
				slog.WarnContext(ctx, "Decode room snapshot", "error", err)
			}
		}
		h.invalidateRoom(ctx, e.Room, snapshot, e.Started)
	})
	b.OBS.Subscribe(func(ctx context.Context, e events.OBSStreamChanged) {
		h.invalidateRoom(ctx, e.Room, store.Room{}, time.Time{})
	})
	if cache, ok := h.sessions.(interface{ InvalidateOBS(string) }); ok {
		b.Rooms.Subscribe(func(_ context.Context, e events.RoomStateChanged) { cache.InvalidateOBS(e.Room) })
		b.OBS.Subscribe(func(_ context.Context, e events.OBSStreamChanged) { cache.InvalidateOBS(e.Room) })
	}
	b.ReplayRemovals.Subscribe(func(_ context.Context, e events.ReplayRemoved) { h.videos.Invalidate(e.ID) })
	b.OBS.Subscribe(func(_ context.Context, e events.OBSStreamChanged) { h.videos.Invalidate(e.ReplayID) })
}
