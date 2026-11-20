package ws

import (
	"fmt"
	"testing"
)

func BenchmarkBroadcastDispatch(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			members := make(map[*Client]struct{}, size)
			for i := 0; i < size; i++ {
				members[new(Client)] = struct{}{}
			}
			m := Manager{
				Rooms:            map[string]map[*Client]struct{}{"room": members},
				broadcastJobChan: make(chan *BroadcastJob, 1),
			}
			m.snapshots = make(map[string][]*Client)
			payload := []byte("benchmark")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.broadcastToLocalClients("room", payload)
				job := <-m.broadcastJobChan
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
		client := &Client{Send: make(chan []byte, 1)}
		members[client] = struct{}{}
		clients = append(clients, client)
	}
	m := Manager{
		Rooms:            map[string]map[*Client]struct{}{"room": members},
		broadcastJobChan: make(chan *BroadcastJob, 1),
	}
	m.snapshots = make(map[string][]*Client)
	go m.broadcastWorker()
	defer close(m.broadcastJobChan)
	payload := []byte("test")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.broadcastToLocalClients("room", payload)
		for _, client := range clients {
			<-client.Send
		}
	}
}
