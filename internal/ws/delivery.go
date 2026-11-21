package ws

import (
	"bytes"
	"context"
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	maxBatchMessages       = 32
	maxBatchBytes          = 32 << 10
	maxQueuedBytes         = 256 << 10
	DefaultBatchArrivalGap = 2 * time.Millisecond
)

type outboundFrame struct {
	payload  []byte
	prepared *websocket.PreparedMessage
	count    int
}

func (m *Manager) frame(payload []byte, count int) *outboundFrame {
	f := &outboundFrame{payload: payload, count: count}
	if m.Prepared {
		// Gorilla builds the uncompressed server representation entirely in memory.
		p, err := websocket.NewPreparedMessage(websocket.TextMessage, payload)
		if err == nil {
			f.prepared = p
		}
	}
	return f
}

func (m *Manager) deliver(job *BroadcastJob) {
	var batch *outboundFrame
	var singles []*outboundFrame
	for _, c := range job.Clients {
		if c.batch && len(job.Packets) > 1 {
			if batch == nil {
				payload := append([]byte{'['}, bytes.Join(job.Packets, []byte{','})...)
				batch = m.frame(append(payload, ']'), len(job.Packets))
			}
			m.safeSend(c, batch)
			continue
		}
		if singles == nil {
			singles = make([]*outboundFrame, len(job.Packets))
			for i, p := range job.Packets {
				singles[i] = m.frame(p, 1)
			}
		}
		for _, f := range singles {
			m.safeSend(c, f)
		}
	}
}

func (m *Manager) safeSend(c *Client, f *outboundFrame) {
	size := int64(len(f.payload))
	if c.pendingBytes.Add(size) <= maxQueuedBytes {
		select {
		case c.Send <- f:
			c.consecutiveDrops.Store(0)
			return
		default:
		}
	}
	c.pendingBytes.Add(-size)
	m.dropped.Add(uint64(f.count))
	m.clientDrops.Add(uint64(f.count))
	if c.consecutiveDrops.Add(1) == 8 && c.done != nil {
		c.Close()
	}
}

// Only consecutive arrivals in a busy room start a batch. An idle room sends
// its first packet immediately. Each job retains its original member snapshot.
func (m *Manager) consumeRoom(ctx context.Context, room string, ch <-chan *redis.Message) {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var deadline <-chan time.Time
	var previous, expires time.Time
	var pending *BroadcastJob
	size := 0
	flush := func() {
		if pending != nil {
			m.queueJob(room, pending)
			pending = nil
		}
		deadline = nil
		timer.Stop()
		size = 0
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			flush()
		case msg, ok := <-ch:
			if !ok {
				flush()
				return
			}
			now := time.Now()
			members := m.roomSnapshot(room)
			if len(members) == 0 {
				continue
			}
			payload := []byte(msg.Payload)
			if !members[len(members)-1].batch {
				split := sort.Search(len(members), func(i int) bool { return !members[i].batch })
				m.queueJob(room, &BroadcastJob{Clients: members[split:], Packets: [][]byte{payload}})
				members = members[:split]
				if len(members) == 0 {
					flush()
					continue
				}
			}
			if pending != nil && (!now.Before(expires) || &pending.Clients[0] != &members[0] || size+len(payload)+1 > maxBatchBytes) {
				flush()
			}
			hot := m.BatchArrivalGap > 0 && now.Sub(previous) <= m.BatchArrivalGap
			previous = now
			if pending == nil {
				pending = &BroadcastJob{Clients: members}
				if !hot || len(members) < m.MinBatchViewers || m.BatchWindow == 0 || len(payload)+2 >= maxBatchBytes {
					pending.Packets = [][]byte{payload}
					flush()
					continue
				}
				expires = now.Add(m.BatchWindow)
				deadline = timer.C
				timer.Reset(m.BatchWindow)
				size = 1
			}
			pending.Packets = append(pending.Packets, payload)
			size += len(payload) + 1
			if len(pending.Packets) >= maxBatchMessages || size >= maxBatchBytes {
				flush()
			}
		}
	}
}
