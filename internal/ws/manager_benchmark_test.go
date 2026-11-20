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
			m := Manager{Rooms: map[string]map[*Client]struct{}{"room": members}, broadcastJobChan: make(chan *BroadcastJob, 1)}
			m.clientPool.New = func() any { return make([]*Client, 0, InitClientPoolCap) }
			payload := []byte("benchmark")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.broadcastToLocalClients("room", payload)
				job := <-m.broadcastJobChan
				if len(job.Clients) != size {
					b.Fatal("recipient count")
				}
				m.clientPool.Put(job.Clients)
			}
		})
	}
}
