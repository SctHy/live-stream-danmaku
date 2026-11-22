package hub

import (
	"context"
	"encoding/json"
	"hash/maphash"
	"log/slog"
	"testing"

	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/protocol"
)

type localSubscriptions struct{}

func (localSubscriptions) Subscribe(context.Context, string, func([]byte)) (<-chan struct{}, error) {
	ready := make(chan struct{})
	close(ready)
	return ready, nil
}
func (localSubscriptions) Unsubscribe(context.Context, string) {}
func (localSubscriptions) Close()                              {}

type recordingBridge struct {
	id                      uint64
	room                    string
	deliver                 func(protocol.LiveDanmaku)
	subscriptions, removals int
}

func (b *recordingBridge) Subscribe(room string, id uint64, deliver func(protocol.LiveDanmaku)) {
	b.room, b.id, b.deliver = room, id, deliver
	b.subscriptions++
}
func (b *recordingBridge) Unsubscribe(room string) { b.room = room; b.removals++ }
func (b *recordingBridge) Close()                  {}

func TestLiveRoomMembershipAndBroadcast(t *testing.T) {
	for _, mode := range []string{"reject", "mask"} {
		t.Run(mode, func(t *testing.T) {
			bridge := new(recordingBridge)
			jobs := make(chan *broadcastJob, 4)
			h := Hub{life: t.Context(), log: slog.Default(), mux: localSubscriptions{}, liveBridge: bridge, filter: filter.New([]string{"bad"}), maskWords: mode == "mask", rooms: make(map[string]map[*Client]struct{}), ready: make(map[string]<-chan struct{}), cancelSub: make(map[string]context.CancelFunc), snapshots: make(map[string][]*Client), shardSeed: maphash.MakeSeed(), deliveryQueues: []chan *broadcastJob{jobs}, sendCapacity: 4}
			regular := h.NewClient(1, "roll/123", nil)
			h.handleRegister(regular)
			h.handleUnregister(regular)
			if bridge.subscriptions != 0 || bridge.removals != 0 {
				t.Fatal("non-numeric room used bridge")
			}
			room := "6/screening-01"
			a, b := h.NewClient(1, room, nil), h.NewClient(2, room, nil)
			h.handleRegister(a)
			h.handleRegister(b)
			if bridge.id != 6 || bridge.room != room || bridge.subscriptions != 1 {
				t.Fatal("expected one subscription for numeric room")
			}
			bridge.deliver(protocol.LiveDanmaku{Content: "bad!"})
			want := "***!"
			if mode == "reject" {
				if len(jobs) != 0 {
					t.Fatal("rejected event queued")
				}
				bridge.deliver(protocol.LiveDanmaku{Content: "ok"})
				want = "ok"
			}
			if len(jobs) != 1 {
				t.Fatal("expected one broadcast")
			}
			h.deliver(<-jobs)
			for _, c := range []*Client{a, b} {
				if len(c.send) != 1 {
					t.Fatal("missing broadcast")
				}
				wire := (<-c.send).payload
				var p protocol.WsPacket
				var msg protocol.LiveDanmaku
				if json.Unmarshal(wire, &p) != nil || p.Type != 108 || p.Position != "" || p.Previous != "" || p.RoomID != room || json.Unmarshal(p.Data, &msg) != nil || msg.RoomID != room || msg.Source != "bilibili" || msg.Content != want {
					t.Fatal(string(wire))
				}
			}
			h.handleUnregister(a)
			if bridge.removals != 0 {
				t.Fatal("early unsubscribe")
			}
			h.handleUnregister(b)
			if bridge.room != room || bridge.removals != 1 {
				t.Fatal("missing unsubscribe")
			}
		})
	}
}
