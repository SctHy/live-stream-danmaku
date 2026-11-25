package pubsub

import (
	"context"
	"crypto/rand"
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
