package pubsub

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestBoundedInterestProfile(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := Open(addr)
	defer s.Close()
	ctx := t.Context()
	at := time.Now()
	uid := uint64(at.UnixNano())
	defer s.r.Del(ctx, profileKeys(uid)...)
	for i := 0; i < 40; i++ {
		room := fmt.Sprintf("room-%02d", i)
		if err := s.RecordInterest(ctx, uid, room, "music", []string{room}, i+1, at.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RecordInterest(ctx, uid, "unknown", "", nil, 1, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preferences(ctx, uid)
	if err != nil || len(p.Tags) != 32 || len(p.Recent) != 30 || p.Tags["room-07"] != 0 || p.Tags["room-08"] != 9 || p.Recent["room-10"] != 0 || p.Recent["unknown"] == 0 || p.Categories["music"] != 820 {
		t.Fatal(p, err)
	}
	for _, key := range profileKeys(uid) {
		ttl, err := s.r.TTL(ctx, key).Result()
		if err != nil || ttl < 7*24*time.Hour-time.Minute || ttl > 7*24*time.Hour {
			t.Fatal(key, ttl, err)
		}
	}
}
