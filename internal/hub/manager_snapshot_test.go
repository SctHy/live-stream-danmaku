package hub

import (
	"hash/maphash"
	"testing"
)

func TestQueuedBroadcastKeepsOriginalMembers(t *testing.T) {
	first := &Client{RoomID: "room"}
	next := &Client{RoomID: "room"}
	m := Hub{
		Rooms:          map[string]map[*Client]struct{}{"room": {first: {}}},
		snapshots:      make(map[string][]*Client),
		shardSeed:      maphash.MakeSeed(),
		deliveryQueues: []chan *BroadcastJob{make(chan *BroadcastJob, 1)},
	}
	m.broadcastToLocalClients("room", nil)
	queued := <-m.deliveryQueues[0]
	m.handleRegister(next)
	if members := m.roomSnapshot("room"); len(members) != 2 {
		t.Fatalf("after join: %d members", len(members))
	}
	// Simulate an already closed transport; this test exercises membership changes.
	first.once.Do(func() {})
	m.handleUnregister(first)
	if members := m.roomSnapshot("room"); len(members) != 1 || members[0] != next {
		t.Fatal("unexpected recipients after leave")
	}
	if len(queued.Clients) != 1 || queued.Clients[0] != first {
		t.Fatal("queued recipients changed")
	}
	next.once.Do(func() {})
	m.handleUnregister(next)
	if members := m.roomSnapshot("room"); len(members) != 0 {
		t.Fatal("empty room still has recipients")
	}
}
