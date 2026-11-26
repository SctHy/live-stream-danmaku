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

func TestWithdrawFilterUsesWithdrawalsBeforeCursor(t *testing.T) {
	events := []protocol.WsPacket{
		{Type: protocol.TypeWithdraw, Data: json.RawMessage(`{"event_id":"a"}`), Position: "first"},
		{Type: protocol.TypeDanmaku, Data: json.RawMessage(`{"event_id":"a","content":"hidden"}`), Previous: "first", Position: "second"},
	}
	if err := WithdrawFilter(events, 1); err != nil {
		t.Fatal(err)
	}
	if p := events[1]; p.Type != protocol.TypeWithdraw || string(p.Data) != `{"event_id":"a"}` || p.Previous != "first" || p.Position != "second" {
		t.Fatal(p)
	}
}

func TestGuardedAppendRejectsInvalidWindow(t *testing.T) {
	s := &Store{}
	status, _, err := s.AppendWithGuards(t.Context(), GuardSpec{GateWindowSeconds: 30, RateWindowMS: 500, RateLimit: 1}, "id", nil)
	if err == nil || status != "unavailable" {
		t.Fatal(status, err)
	}
}

func TestGuardedAppendRejectsInvalidInput(t *testing.T) {
	valid := GuardSpec{DenyKey: "deny", RateKey: "rate", GateKey: "gate", ExemptSetKey: "staff", LastAdmittedKey: "last", StreamKey: "stream", Channel: "channel", DedupKey: "request", GateWindowSeconds: 1, RateWindowMS: 10, RateLimit: 1, DedupSeconds: 1}
	for _, field := range []string{"stream", "channel", "dedup", "event", "empty", "json", "window", "collision"} {
		g, id, payload := valid, "event", []byte("{}")
		switch field {
		case "stream":
			g.StreamKey = ""
		case "channel":
			g.Channel = " "
		case "dedup":
			g.DedupKey = ""
		case "event":
			id = ""
		case "empty":
			payload = nil
		case "json":
			payload = []byte("{")
		case "collision":
			g.DedupKey = g.StreamKey
		case "window":
			g.DedupSeconds = 0
		}
		if _, _, err := new(Store).AppendWithGuards(t.Context(), g, id, payload); err == nil {
			t.Fatal(field)
		}
	}
}
