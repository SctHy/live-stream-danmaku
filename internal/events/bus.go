// Package events defines the gateway's version 1 in-process domain events.
package events

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type schemaV1 struct{}

func (schemaV1) SchemaVersion() int { return 1 }

// Accepted takes ownership of the encoded packet. Subscribers may retain Wire()
// for asynchronous delivery and must treat its contents as read-only.
type DanmakuAccepted struct {
	schemaV1
	Room      string
	UserID    uint64
	At        time.Time
	EventID   string
	Duplicate bool
	wire      []byte
}

func Accepted(room string, uid uint64, at time.Time, id string, wire []byte) DanmakuAccepted {
	return DanmakuAccepted{Room: room, UserID: uid, At: at, EventID: id, wire: wire}
}
func (e DanmakuAccepted) Wire() []byte { return e.wire }

type LikeRecorded struct {
	schemaV1
	Room          string
	At            time.Time
	UserID, Count uint64
}
type DanmakuWithdrawn struct {
	schemaV1
	Room, EventID string
}
type RoomStateChanged struct {
	schemaV1
	Room     string
	Snapshot string // optional committed room JSON
	Started  time.Time
}
type OBSStreamChanged struct {
	schemaV1
	Room     string
	Live     bool
	ReplayID int64
}
type GiftOrdered struct {
	schemaV1
	Room    string
	UserID  uint64
	OrderID int64
}
type RoomViewed struct {
	schemaV1
	Room   string
	UserID uint64
}
type ReplayCommented struct {
	schemaV1
	Room   string
	UserID uint64
}
type ReplayRemoved struct {
	schemaV1
	ID int64
}

type handler[T any] func(context.Context, T)

// Topic publishes synchronously against an immutable subscriber snapshot.
// Register short callbacks that hand off expensive work to their own buffers.
type Topic[T any] struct {
	subscribers atomic.Pointer[[]handler[T]]
	active      atomic.Int64
	done        atomic.Pointer[chan struct{}]
	once        sync.Once
}

func (t *Topic[T]) Subscribe(fn func(context.Context, T)) {
	for {
		old := t.subscribers.Load()
		var callbacks []handler[T]
		if old != nil {
			callbacks = slices.Clone(*old)
		}
		callbacks = append(callbacks, fn)
		if t.subscribers.CompareAndSwap(old, &callbacks) {
			return
		}
	}
}
func (t *Topic[T]) Publish(ctx context.Context, e T) {
	if t.done.Load() != nil {
		return
	}
	t.active.Add(1)
	defer func() {
		if t.active.Add(-1) == 0 {
			if done := t.done.Load(); done != nil && t.active.Load() == 0 {
				t.once.Do(func() { close(*done) })
			}
		}
	}()
	if t.done.Load() != nil {
		return
	}
	if handlers := t.subscribers.Load(); handlers != nil {
		for _, fn := range *handlers {
			deliver(ctx, fn, e)
		}
	}
}

// A projection failure must not suppress later subscribers or a sender receipt.
func deliver[T any](ctx context.Context, fn handler[T], e T) {
	defer func() {
		if failure := recover(); failure != nil {
			slog.ErrorContext(ctx, "Domain event subscriber panicked", "error", failure)
		}
	}()
	fn(ctx, e)
}
func (t *Topic[T]) close() {
	ch := make(chan struct{})
	if !t.done.CompareAndSwap(nil, &ch) {
		ch = *t.done.Load()
	}
	if t.active.Load() == 0 {
		t.once.Do(func() { close(ch) })
	}
	<-ch
}

type Bus struct {
	Danmaku        Topic[DanmakuAccepted]
	Likes          Topic[LikeRecorded]
	Withdrawals    Topic[DanmakuWithdrawn]
	Rooms          Topic[RoomStateChanged]
	OBS            Topic[OBSStreamChanged]
	Gifts          Topic[GiftOrdered]
	Views          Topic[RoomViewed]
	ReplayComments Topic[ReplayCommented]
	ReplayRemovals Topic[ReplayRemoved]
}

// Close drains active callbacks. Stop ingress before closing all topics.
func (b *Bus) Close() {
	b.Danmaku.close()
	b.Likes.close()
	b.Withdrawals.close()
	b.Rooms.close()
	b.OBS.close()
	b.Gifts.close()
	b.Views.close()
	b.ReplayComments.close()
	b.ReplayRemovals.close()
}
