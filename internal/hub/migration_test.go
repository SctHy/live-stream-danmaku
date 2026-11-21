package hub

import (
	"context"
	"testing"
	"time"

	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

func TestMigrationWaitsForOldConnectionOrDeadline(t *testing.T) {
	first := &Client{migration: make(chan *outboundFrame, 1), done: make(chan struct{})}
	second := &Client{migration: make(chan *outboundFrame, 1), done: make(chan struct{})}
	m := Hub{Rooms: map[string]map[*Client]struct{}{"room": {first: {}, second: {}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { m.Migrate(ctx, 2, 1, time.Millisecond); close(done) }()
	var notice *outboundFrame
	select {
	case notice = <-first.migration:
	case notice = <-second.migration:
		first, second = second, first
	case <-ctx.Done():
		t.Fatal("missing invitation")
	}
	packets, err := protocol.DecodePackets(notice.payload)
	if err != nil || len(packets) != 1 || packets[0].Type != protocol.TypeMigrate || string(packets[0].Data) != "{}" {
		t.Fatalf("unexpected notice: %s", notice.payload)
	}
	if !m.Retiring() {
		t.Fatal("gateway still ready")
	}
	late := &Client{done: make(chan struct{})}
	m.handleRegister(late)
	select {
	case <-late.done:
	default:
		t.Fatal("late registration admitted")
	}
	select {
	case <-second.migration:
		t.Fatal("in-flight limit exceeded")
	case <-time.After(10 * time.Millisecond):
	}
	first.Close()
	select {
	case next := <-second.migration:
		if next != notice {
			t.Fatal("migration frame was not shared")
		}
	case <-ctx.Done():
		t.Fatal("slot was not reused")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deadline ignored")
	}
}
