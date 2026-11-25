package pubsub

import (
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

func TestActivityWindowAndSessionTotals(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := Open(addr)
	defer s.Close()
	ctx := t.Context()
	slug := rand.Text()[:16]
	room := slug + "/one"
	now := time.Now()
	for offset, weight := range map[int]float64{3: 4, 8: 5, 12: 100} {
		key := minuteKey(now.Add(-time.Duration(offset) * time.Minute))
		if err := s.r.ZAdd(ctx, key, redis.Z{Member: slug, Score: weight}).Err(); err != nil {
			t.Fatal(err)
		}
		defer s.r.ZRem(ctx, key, slug)
	}
	msg := protocol.DanmakuMessage{EventID: rand.Text(), RoomID: room, UserID: 1}
	for _, want := range []string{"new", "accepted"} {
		status, _, err := s.AcceptDanmaku(ctx, msg, []byte(`{"type":101}`))
		if err != nil || status != want {
			t.Fatal(status, err)
		}
	}
	if _, err := s.SnapshotStats(ctx, "test", map[string]int{room: 2, slug + "/two": 3}, map[string]uint64{room: 10}, time.Minute); err != nil {
		t.Fatal(err)
	}
	activity, err := s.Activity(ctx)
	if err != nil || activity[slug] != (Activity{Online: 5, Heat: 17, BusiestSession: "two"}) {
		t.Fatal(activity[slug], err)
	}
}
