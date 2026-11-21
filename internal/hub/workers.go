package hub

import (
	"hash/maphash"
	"time"
)

func (m *Hub) StartWorkers() {
	for _, q := range m.deliveryQueues {
		go m.broadcastWorker(q)
	}
}

func (m *Hub) roomShard(room string) int {
	return int(maphash.String(m.shardSeed, room) % uint64(len(m.deliveryQueues)))
}

func (m *Hub) broadcastWorker(queue <-chan *BroadcastJob) {
	var stopped <-chan struct{}
	if m.life != nil {
		stopped = m.life.Done()
	}
	for {
		select {
		case <-stopped:
			return
		case job, ok := <-queue:
			if !ok {
				return
			}
			m.waitNanos.Add(uint64(time.Since(job.queued)))
			m.measuredJobs.Add(1)
			m.deliver(job)
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

func (m *Hub) queueJob(roomID string, job *BroadcastJob) {
	job.queued = time.Now()
	select {
	case m.deliveryQueues[m.roomShard(roomID)] <- job:
	default:
		m.shardDrops.Add(uint64(len(job.Packets)) * uint64(len(job.Clients)))
		m.dropped.Add(uint64(len(job.Packets)) * uint64(len(job.Clients)))
	}
}
