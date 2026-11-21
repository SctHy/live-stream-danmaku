package ws

import (
	"context"
	"testing"
	"time"
)

func TestMigrationWaitsForOldConnectionOrDeadline(t *testing.T) {
	first := &Client{migration: make(chan struct{}, 1), done: make(chan struct{})}
	second := &Client{migration: make(chan struct{}, 1), done: make(chan struct{})}
	m := Manager{Rooms: map[string]map[*Client]struct{}{"room": {first: {}, second: {}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { m.Migrate(ctx, 2, 1, time.Millisecond); close(done) }()
	select {
	case <-first.migration:
	case <-second.migration:
		first, second = second, first
	case <-ctx.Done():
		t.Fatal("missing invitation")
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
	case <-second.migration:
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
