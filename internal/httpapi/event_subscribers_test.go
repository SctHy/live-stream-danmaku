package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/scthy/maku/internal/events"
)

func TestOptionalInterestSubscriber(t *testing.T) {
	for _, enable := range []bool{false, true} {
		var bus events.Bus
		h := NewChatHandlerWithBus(nil, nil, new(profileSpy), false, &bus)
		if enable {
			h.SubscribeInterests()
		}
		var archived []byte
		bus.Danmaku.Subscribe(func(_ context.Context, e events.DanmakuAccepted) { archived = e.Wire() })
		const id = uint64(1<<63 + 9)
		event := events.Accepted("music/live", id, time.Now(), "1", []byte("original packet"))
		bus.Danmaku.Publish(t.Context(), event)
		event.Duplicate = true
		bus.Danmaku.Publish(t.Context(), event)
		bus.Likes.Publish(t.Context(), events.LikeRecorded{Room: "music/live", UserID: id, Count: 8})
		weight := h.interests.users[id]["music"].Weight
		want := 0
		if enable {
			want = 3
		}
		if string(archived) != "original packet" || weight != want {
			t.Fatalf("interest enabled %v, archive %q, weight %d", enable, archived, weight)
		}
	}
}
