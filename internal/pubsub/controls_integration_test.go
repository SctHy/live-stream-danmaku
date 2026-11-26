package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

func TestRoomPolicyAndRetries(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := Open(addr)
	defer s.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	slug := "policy-" + rand.Text()[:12]
	defer s.r.Del(ctx, sessionPrefix(slug)+"controls", sessionPrefix(slug)+"gate", sessionPrefix(slug)+"staff")
	msg := protocol.DanmakuMessage{UserID: 7, RoomID: slug + "/first"}
	send := func(id, want string) {
		t.Helper()
		msg.EventID = id
		status, _, err := s.AcceptDanmaku(ctx, msg, []byte(`{"type":101,"data":{}}`))
		if err != nil || status != want {
			t.Fatalf("got %s (%v), want %s", status, err, want)
		}
	}
	for range 8 {
		send(rand.Text(), "new")
	}
	send(rand.Text(), "rate_limited")
	msg.UserID = 8
	if e := s.SetStaff(ctx, slug, msg.UserID, true); e != nil {
		t.Fatal(e)
	}
	if e := s.SetControls(ctx, slug, protocol.RoomControls{Slow: 30}); e != nil {
		t.Fatal(e)
	}
	// Anonymous connections using a staff member's numeric ID have no exemption.
	send("once", "new")
	msg.RoomID = slug + "/second"
	send("twice", "slow_mode")
	msg.Nickname = "staff"
	send("exempt", "new")
	if e := s.SetControls(ctx, slug, protocol.RoomControls{MuteAll: true}); e != nil {
		t.Fatal(e)
	}
	msg.Nickname = ""
	send("exempt", "accepted")
	send("blocked", "room_muted")
	if e := s.Mute(ctx, msg.RoomID, msg.UserID, time.Minute); e != nil {
		t.Fatal(e)
	}
	msg.Nickname = "staff"
	send("individual", "muted")
}

func TestGuardedAppendAndRecoveryFilters(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := Open(addr)
	defer s.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	room := "guards-" + rand.Text() + "/session"
	prefix := sessionPrefix(room)
	g := GuardSpec{
		StreamKey: prefix + "recent", DedupKey: prefix + "request", DenyKey: prefix + "deny", RateKey: prefix + "rate",
		GateKey: prefix + "gate", ExemptSetKey: prefix + "staff", LastAdmittedKey: prefix + "last",
		Channel: fmt.Sprintf(KeyRoomPubSub, room), Member: "7", GateWindowSeconds: 30, RateWindowMS: 1000, RateLimit: 2, DedupSeconds: 60,
	}
	if err := s.r.Set(ctx, g.GateKey, 31, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	body := `{"event_id":"a","user_id":18446744073709551615}`
	payload := []byte(fmt.Sprintf(`{"type":%d,"data":%s}`, protocol.TypeDanmaku, body))
	for _, want := range []string{"new", "duplicate"} {
		status, id, err := s.AppendWithGuards(ctx, g, "a", payload)
		if err != nil || status != want || id != "a" {
			t.Fatal(status, id, err)
		}
	}
	if err := s.Journal(ctx, room, []byte(fmt.Sprintf(`{"type":%d,"data":{"event_id":"a"}}`, protocol.TypeWithdraw))); err != nil {
		t.Fatal(err)
	}
	raw, err := s.RecoverWithFilter(ctx, room, "0-0", nil)
	if err != nil || raw.Reset || len(raw.Events) != 2 || string(raw.Events[0].Data) != body {
		t.Fatal(raw, err)
	}
	filtered, err := s.Recover(ctx, room, "0-0")
	if err != nil || len(filtered.Events) != 2 || filtered.Events[0].Type != protocol.TypeWithdraw || filtered.Position != raw.Position || filtered.Events[0].Position != raw.Events[0].Position {
		t.Fatal(filtered, err)
	}
}
