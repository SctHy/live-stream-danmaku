package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

// Run with seed addresses from a Redis Cluster, reachable at their advertised addresses.
func TestClusterPublishRecoverAndReplay(t *testing.T) {
	seeds := os.Getenv("DANMAKU_TEST_REDIS_CLUSTER_ADDRS")
	if seeds == "" {
		t.Skip("requires Redis Cluster")
	}
	store := Open(seeds)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	mux := store.Mux(ctx, 2)
	defer mux.Close()
	room := "cluster/" + rand.Text()
	messages := make(chan []byte, 2)
	ready, err := mux.Subscribe(ctx, room, func(payload []byte) {
		select {
		case messages <- payload:
		case <-ctx.Done():
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	msg := protocol.DanmakuMessage{RoomID: room, UserID: ^uint64(0), ClientMsgID: "retry", EventID: rand.Text(), Content: "cluster", SendTime: time.Now()}
	body, _ := json.Marshal(msg)
	packet, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeDanmaku, RoomID: room, Data: body})
	for _, want := range []string{"new", "accepted"} {
		status, id, err := store.AcceptDanmaku(ctx, msg, packet)
		if err != nil || status != want || id != msg.EventID {
			t.Fatal(status, id, err)
		}
	}
	select {
	case payload := <-messages:
		var p protocol.WsPacket
		var decoded protocol.DanmakuMessage
		if json.Unmarshal(payload, &p) != nil || json.Unmarshal(p.Data, &decoded) != nil || decoded.UserID != msg.UserID || p.Position == "" || p.Previous != "0-0" {
			t.Fatal(string(payload))
		}
		recent, err := store.Recover(ctx, room, "")
		if err != nil || len(recent.Events) != 1 || recent.Position != p.Position {
			t.Fatal(recent, err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
