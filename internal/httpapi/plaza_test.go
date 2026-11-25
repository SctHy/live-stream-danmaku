package httpapi

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

func TestDetailBatchCoalescesAndInvalidates(t *testing.T) {
	var c plazaCache
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	load := func(keys []string) (map[string]store.Room, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return map[string]store.Room{"r": {Slug: "r", Title: "title"}}, nil
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			rooms, e := c.getDetails(t.Context(), []rank{{Slug: "r"}}, load)
			if e != nil || rooms["r"].Title != "title" {
				t.Error(rooms, e)
			}
		})
	}
	<-entered
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	c.invalidate("r")
	_, e := c.getDetails(t.Context(), []rank{{Slug: "r"}}, load)
	if e != nil || calls.Load() != 2 {
		t.Fatal(calls.Load(), e)
	}
}

type testActivity struct {
	sessionStore
	calls int
}

func (s *testActivity) Activity(context.Context) (map[string]pubsub.Activity, error) {
	s.calls++
	return map[string]pubsub.Activity{"popular": {Heat: 18, Online: 2}, "crowded": {Heat: 10, Online: 8}}, nil
}
func TestOfflineIndexSortAndReuse(t *testing.T) {
	sessions := new(testActivity)
	h := NewChatHandler(nil, nil, sessions, false)
	for _, tc := range []struct{ sort, first string }{{"hot", "popular"}, {"online", "crowded"}, {"hot", "popular"}} {
		idx, e := h.rankings(t.Context(), nil, tc.sort, "all")
		if e != nil || !idx.Degraded || len(idx.Ranks) != 2 || idx.Ranks[0].Slug != tc.first {
			t.Fatal(tc, idx, e)
		}
	}
	if sessions.calls != 2 {
		t.Fatal(sessions.calls)
	}
}
