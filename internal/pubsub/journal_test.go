package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

func TestJournalRecoveryAndWithdrawal(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set DANMAKU_TEST_REDIS_ADDR for journal integration")
	}
	r := redis.NewClient(&redis.Options{Addr: addr})
	defer r.Close()
	store := newStore(r)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	room := "check/" + rand.Text()
	key := sessionPrefix(room) + "recent"
	defer r.Del(ctx, key)
	publish := func(kind int, id string) {
		t.Helper()
		data, _ := json.Marshal(map[string]string{"content": "hello", "event_id": id})
		packet, _ := json.Marshal(protocol.WsPacket{Type: kind, Data: data})
		if err := store.Journal(ctx, room, packet); err != nil {
			t.Fatal(err)
		}
	}
	recoverFrom := func(pos string) Recovery {
		t.Helper()
		res, err := store.Recover(ctx, room, pos)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	publish(protocol.TypeDanmaku, "first")
	first := recoverFrom("").Position
	publish(protocol.TypeDanmaku, "second")
	publish(protocol.TypeWithdraw, "first")
	all := recoverFrom("")
	if len(all.Events) != 3 || all.Events[0].Type != protocol.TypeWithdraw || string(all.Events[0].Data) != `{"event_id":"first"}` {
		t.Fatal(all)
	}
	missed := recoverFrom(first)
	if missed.Reset || len(missed.Events) != 2 || missed.Events[0].Previous != first {
		t.Fatal(missed)
	}
	for range 512 {
		publish(protocol.TypeDanmaku, "next")
	}
	trimmed := recoverFrom(first)
	if !trimmed.Reset || len(trimmed.Events) < 256 || len(trimmed.Events) >= 512 {
		t.Fatal("trim did not signal reset")
	}
	if err := r.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	publish(protocol.TypeDanmaku, "new stream")
	fresh := recoverFrom(trimmed.Position)
	if !fresh.Reset || len(fresh.Events) != 1 {
		t.Fatal(fresh)
	}
}
