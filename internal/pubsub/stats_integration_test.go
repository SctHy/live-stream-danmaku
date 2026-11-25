package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSnapshotStatsPipelineErrors(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := Open(addr)
	defer s.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	room := "stats/" + rand.Text()
	recent := sessionPrefix(room) + "recent"
	likesKey := fmt.Sprintf(KeyRoomLikes, room)
	registry := fmt.Sprintf(KeyServerOfRoom, room)
	online := fmt.Sprintf(KeyRoomServerOnline, room, "active")
	defer s.r.Del(ctx, recent, likesKey, registry, online)
	if err := s.r.Set(ctx, recent, "wrong type", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	rooms := map[string]int{room: 1}
	// GET sees a missing likes key before XREVRANGE receives WRONGTYPE.
	likes := map[string]uint64{}
	checkFailure := func() {
		t.Helper()
		_, err := s.SnapshotStats(ctx, "active", rooms, likes, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "WRONGTYPE") {
			t.Fatalf("expected stream error: %v", err)
		}
	}
	checkFailure()
	likes[room] = 3
	checkFailure()
	if len(likes) != 0 {
		t.Fatal("acknowledged delta still pending", likes)
	}
	if err := s.r.Del(ctx, recent).Err(); err != nil {
		t.Fatal(err)
	}
	if err := s.r.SAdd(ctx, registry, "expired").Err(); err != nil {
		t.Fatal(err)
	}
	stats, err := s.SnapshotStats(ctx, "active", rooms, likes, time.Minute)
	if err != nil || stats[room].Likes != 3 || stats[room].Online != 1 {
		t.Fatal(stats, err)
	}
	if n, err := s.r.SCard(ctx, registry).Result(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
