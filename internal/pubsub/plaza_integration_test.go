package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"sync/atomic"
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
	batch := []HeatBatch{{Minute: now.Unix() / 60, ID: rand.Text(), RoomFifths: map[string]uint64{slug: 15}}}
	for range 2 {
		if err := s.FlushHeat(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SnapshotStats(ctx, "test", map[string]int{room: 2, slug + "/two": 3}, map[string]uint64{room: 10}, time.Minute); err != nil {
		t.Fatal(err)
	}
	hook := new(countRoundTrips)
	s.r.AddHook(hook)
	activity, err := s.Activity(ctx)
	if hook.pipelines.Load() != 1 || hook.commands.Load() != 0 {
		t.Fatal("activity round trips", hook.pipelines.Load(), hook.commands.Load())
	}
	if err != nil || activity[slug] != (Activity{Online: 5, Heat: 17, BusiestSession: "two"}) {
		t.Fatal(activity[slug], err)
	}
}

func TestCachedSessionAndOBS(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	a, b := Open(addr), Open(addr)
	defer a.Close()
	defer b.Close()
	ctx := t.Context()
	uid := uint64(time.Now().UnixNano())
	token := fmt.Sprint(uid) + "." + rand.Text()
	profile := Login{UserID: uid, Nickname: "first"}
	if err := a.SaveLogin(ctx, token, profile, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*Store{a, b} {
		if n, e := s.SocketNickname(ctx, token); e != nil || n != "first" {
			t.Fatal(n, e)
		}
	}
	hook := new(countRoundTrips)
	a.r.AddHook(hook)
	if n, e := a.SocketNickname(ctx, token); e != nil || n != "first" || hook.commands.Load()+hook.pipelines.Load() != 0 {
		t.Fatal("session cache miss", n, e)
	}
	profile.Nickname = "new"
	if e := a.RefreshLogin(ctx, token, profile); e != nil {
		t.Fatal(e)
	}
	if n, e := b.SocketNickname(ctx, token); e != nil || n != "new" {
		t.Fatal(n, e)
	}
	if e := a.Logout(ctx, token); e != nil {
		t.Fatal(e)
	}
	if _, e := b.SocketNickname(ctx, token); e != redis.Nil {
		t.Fatal(e)
	}
	room := rand.Text()
	obs := OBSStream{ReplayID: 42}
	if err := a.SetOBS(ctx, room, obs); err != nil {
		t.Fatal(err)
	}
	if got, err := a.OBS(ctx, room); err != nil || got != obs {
		t.Fatal(got, err)
	}
	if err := a.r.Del(ctx, sessionPrefix(room)+"obs").Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := a.OBS(ctx, room); err != nil || got != obs {
		t.Fatal("cache miss", got, err)
	}
	if err := a.SetOBS(ctx, room, OBSStream{}); err != nil {
		t.Fatal(err)
	}
	if got, err := a.OBS(ctx, room); err != nil || got.ReplayID != 0 {
		t.Fatal("stale stream", got, err)
	}
}

type countRoundTrips struct{ commands, pipelines atomic.Int32 }

func (*countRoundTrips) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *countRoundTrips) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, c redis.Cmder) error { h.commands.Add(1); return next(ctx, c) }
}
func (h *countRoundTrips) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error { h.pipelines.Add(1); return next(ctx, cmds) }
}
