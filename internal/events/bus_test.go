package events

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanoutPreservesBytes(t *testing.T) {
	var bus Bus
	wire := []byte(`{"user_id":18446744073709551615}`)
	e := Accepted("a/1", math.MaxUint64, time.Now(), "id", wire)
	if &wire[0] != &e.Wire()[0] {
		t.Fatal("wire copied")
	}
	bus.Danmaku.Subscribe(func(_ context.Context, event DanmakuAccepted) { panic("projection failed") })
	var count atomic.Int64
	bus.Danmaku.Subscribe(func(_ context.Context, event DanmakuAccepted) {
		if string(event.Wire()) != `{"user_id":18446744073709551615}` || event.UserID != math.MaxUint64 {
			t.Error("altered event")
		}
		count.Add(1)
	})
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			for range 4 {
				bus.Danmaku.Publish(t.Context(), e)
			}
		})
	}
	wg.Wait()
	bus.Close()
	bus.Danmaku.Publish(t.Context(), e)
	if count.Load() != 48 {
		t.Fatal(count.Load())
	}
}
func TestCloseWaitsForPublication(t *testing.T) {
	var bus Bus
	started, release, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	bus.Gifts.Subscribe(func(context.Context, GiftOrdered) { close(started); <-release })
	go bus.Gifts.Publish(t.Context(), GiftOrdered{})
	<-started
	go func() { bus.Close(); close(closed) }()
	for bus.Gifts.done.Load() == nil {
		time.Sleep(time.Millisecond)
	}
	select {
	case <-closed:
		t.Fatal("closed during callback")
	default:
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not drain")
	}
}

func BenchmarkArchivePublication(b *testing.B) {
	var bus Bus
	var archived []byte
	bus.Danmaku.Subscribe(func(_ context.Context, e DanmakuAccepted) { archived = e.Wire() })
	wire := make([]byte, 1024)
	ctx := b.Context()
	for b.Loop() {
		bus.Danmaku.Publish(ctx, Accepted("room", 1, time.Time{}, "id", wire))
	}
	if len(archived) != len(wire) {
		b.Fatal(len(archived))
	}
}
