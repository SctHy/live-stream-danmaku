package pubsub

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

func TestUpgradeLegacySession(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	// Use a separate logical DB so the one-time migration marker is isolated.
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 14})
	defer client.Close()
	s := newStore(client)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	room := "upgrade/" + rand.Text()
	old := "dm:{" + room + "}:"
	const marker = "dm:migrations:room-slots-v1"
	// Leave an existing deployment's marker alone.
	if n, err := client.Exists(ctx, marker).Result(); err != nil || n != 0 {
		t.Skip("migration already applied in test DB")
	}
	defer client.Del(ctx, marker)
	for k, v := range map[string]string{"mute:12": "1", "send:7:retry": "prior"} {
		if err := client.Set(ctx, old+k, v, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
	}
	tail, err := client.XAdd(ctx, &redis.XAddArgs{Stream: old + "recent", Values: []string{"packet", `{"type":101,"data":{}}`, "prev", "0-0", "epoch", "old"}}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Expire(ctx, old+"recent", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err := s.UpgradeSessionKeys(ctx); err != nil {
		t.Fatal(err)
	}
	res, e := s.Recover(ctx, room, "")
	if e != nil || res.Position != "old@"+tail || len(res.Events) != 1 {
		t.Fatal(res, e)
	}
	mutes, e := s.Mutes(ctx, room)
	if e != nil || len(mutes) != 1 || mutes[0].UserID != 12 {
		t.Fatal(mutes, e)
	}
	for _, tc := range []struct {
		uid  uint64
		want string
	}{{7, "accepted"}, {12, "muted"}} {
		msg := protocol.DanmakuMessage{UserID: tc.uid, RoomID: room, ClientMsgID: "retry"}
		status, _, err := s.AcceptDanmaku(ctx, msg, []byte("{}"))
		if err != nil || status != tc.want {
			t.Fatal(status, err)
		}
	}
}
