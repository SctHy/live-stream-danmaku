package httpapi

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/recommend"
)

type profileSpy struct {
	sessionStore
	writes  chan []pubsub.InterestDelta
	release chan struct{}
}

func (s *profileSpy) RecordInterests(ctx context.Context, batch []pubsub.InterestDelta) error {
	s.writes <- batch
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (*profileSpy) Preferences(context.Context, uint64) (recommend.Profile, error) {
	return recommend.Profile{}, nil
}

func TestInterestReadAndDrain(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s := &profileSpy{writes: make(chan []pubsub.InterestDelta, 2), release: make(chan struct{})}
	h := NewChatHandler(nil, nil, s, false)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 1000 {
				h.recordInterest(7, "hot", 2)
			}
		})
	}
	wg.Wait()
	h.recordInterest(8, "cold", 1)
	done := make(chan error, 1)
	go func() { _, err := h.readProfile(ctx, s, 7); done <- err }()
	select {
	case batch := <-s.writes:
		if len(batch) != 1 || batch[0].Weight != 32000 || batch[0].UserID != 7 {
			t.Fatal(batch)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// A blocked Redis pipeline must not stall ingress or create more writers.
	for i := 0; i < interestLimit; i++ {
		h.recordInterest(8, fmt.Sprint(i), 1)
	}
	h.recordInterest(8, "cold", 3)
	if h.interests.size != interestLimit || h.interests.users[8]["cold"].Weight != 4 {
		t.Fatal("capacity or accumulation")
	}
	close(s.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	life, stop := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() { defer close(stopped); h.RunInterestWriter(life) }()
	stop()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if got := len(<-s.writes); got != interestLimit || h.interests.size != 0 {
		t.Fatalf("drained %d", got)
	}
}
