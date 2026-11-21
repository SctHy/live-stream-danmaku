package ws

import (
	"encoding/json"
	"hash/maphash"
	"testing"

	"github.com/charlesAcmen/livestream-danmaku/internal/model"
)

func TestDispatchWithoutRegistration(t *testing.T) {
	queue := make(chan *model.WsPacket, 1)
	m := Manager{
		shardSeed:      maphash.MakeSeed(),
		deliveryQueues: make([]chan *BroadcastJob, 1),
		publishQueues:  []chan *model.WsPacket{queue},
		localLikes:     make(map[string]uint64),
	}
	c := &Client{RoomID: "test", UserID: 42}
	m.dispatch(c, model.WsPacket{Type: model.TypeDanmaku, Data: json.RawMessage(`{"content":"hello"}`)})
	select {
	case p := <-queue:
		var msg model.DanmakuMessage
		if err := json.Unmarshal(p.Data, &msg); err != nil || msg.Content != "hello" || msg.RoomID != c.RoomID || msg.UserID != c.UserID || msg.EventID == "" {
			t.Fatalf("unexpected broadcast %s (%v)", p.Data, err)
		}
	default:
		t.Fatal("message did not enter the publish queue")
	}
	m.dispatch(c, model.WsPacket{Type: model.ActionLike, Data: json.RawMessage(`{"count":2}`)})
	if m.localLikes[c.RoomID] != 2 {
		t.Fatal("like did not reach the room counter")
	}
}
