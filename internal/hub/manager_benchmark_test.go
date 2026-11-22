package hub

import (
	"fmt"
	"hash/maphash"
	"testing"
)

func BenchmarkBroadcastDispatch(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			members := make(map[*Client]struct{}, size)
			for i := 0; i < size; i++ {
				members[new(Client)] = struct{}{}
			}
			m := Hub{
				rooms:          map[string]map[*Client]struct{}{"room": members},
				shardSeed:      maphash.MakeSeed(),
				deliveryQueues: []chan *BroadcastJob{make(chan *BroadcastJob, 1)},
			}
			m.snapshots = make(map[string][]*Client)
			payload := []byte("benchmark")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.broadcastToLocalClients("room", payload)
				job := <-m.deliveryQueues[0]
				if len(job.Clients) != size {
					b.Fatal("recipient count")
				}
			}
		})
	}
}

// BenchmarkRoomFanout includes the worker and one channel send and receive per viewer.
func BenchmarkRoomFanout(b *testing.B) {
	const size = 10000
	members := make(map[*Client]struct{}, size)
	clients := make([]*Client, 0, size)
	for i := 0; i < size; i++ {
		client := &Client{Send: make(chan *outboundFrame, 1)}
		members[client] = struct{}{}
		clients = append(clients, client)
	}
	m := Hub{
		rooms:          map[string]map[*Client]struct{}{"room": members},
		shardSeed:      maphash.MakeSeed(),
		deliveryQueues: []chan *BroadcastJob{make(chan *BroadcastJob, 1)},
	}
	m.snapshots = make(map[string][]*Client)
	go m.broadcastWorker(m.deliveryQueues[0])
	defer close(m.deliveryQueues[0])
	payload := []byte("test")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.broadcastToLocalClients("room", payload)
		for _, client := range clients {
			f := <-client.Send
			client.pendingBytes.Add(-int64(len(f.payload)))
		}
	}
}

func (m *Hub) broadcastToLocalClients(roomID string, payload []byte) {
	clients := m.roomSnapshot(roomID)
	if len(clients) == 0 {
		return
	}
	m.queueJob(roomID, &BroadcastJob{Clients: clients, Packets: [][]byte{payload}})
}
