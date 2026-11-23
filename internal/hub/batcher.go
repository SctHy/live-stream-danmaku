package hub

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"
)

const (
	maxBatchMessages       = 32
	maxBatchBytes          = 32 << 10
	maxQueuedBytes         = 256 << 10
	DefaultBatchArrivalGap = 2 * time.Millisecond
)

type outboundFrame struct {
	payload []byte
	count   int
	fanout  bool
}

func (m *Hub) frame(payload []byte, count int) *outboundFrame {
	return &outboundFrame{payload: payload, count: count}
}

func (m *Hub) deliver(job *broadcastJob) {
	var batch *outboundFrame
	var singles []*outboundFrame
	for _, c := range job.Clients {
		if c.batch && len(job.Packets) > 1 {
			if batch == nil {
				payload := append([]byte{'['}, bytes.Join(job.Packets, []byte{','})...)
				batch = &outboundFrame{payload: append(payload, ']'), count: len(job.Packets), fanout: true}
			}
			m.safeSend(c, batch)
			continue
		}
		if singles == nil {
			singles = make([]*outboundFrame, len(job.Packets))
			for i, p := range job.Packets {
				singles[i] = &outboundFrame{payload: p, count: 1, fanout: true}
			}
		}
		for _, f := range singles {
			m.safeSend(c, f)
		}
	}
}

func (m *Hub) safeSend(c *Client, f *outboundFrame) {
	size := int64(len(f.payload))
	if c.pendingBytes.Add(size) <= maxQueuedBytes {
		select {
		case c.send <- f:
			c.consecutiveDrops.Store(0)
			return
		default:
		}
	}
	c.pendingBytes.Add(-size)
	m.metrics.ClientDrop(f.count)
	if c.consecutiveDrops.Add(1) == 8 && !m.disableSlowKick && c.done != nil {
		c.closeSlow()
	}
}

// The Redis shard calls push in wire order. Only a pending burst owns a timer.
type roomBatch struct {
	hub               *Hub
	room              string
	mu                sync.Mutex
	previous, expires time.Time
	pending           *broadcastJob
	timer             *time.Timer
	size              int
}

func (b *roomBatch) flush() {
	if b.pending != nil {
		b.hub.queueJob(b.room, b.pending)
		b.pending = nil
	}
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	b.size = 0
}

func (b *roomBatch) push(ctx context.Context, payload []byte) {
	members := b.hub.roomSnapshot(b.room)
	if ctx.Err() != nil || len(members) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if !members[len(members)-1].batch {
		split := sort.Search(len(members), func(i int) bool { return !members[i].batch })
		b.hub.queueJob(b.room, &broadcastJob{Clients: members[split:], Packets: [][]byte{payload}})
		members = members[:split]
		if len(members) == 0 {
			b.flush()
			return
		}
	}
	if b.pending != nil && (!now.Before(b.expires) || &b.pending.Clients[0] != &members[0] || b.size+len(payload)+1 > maxBatchBytes) {
		b.flush()
	}
	hot := now.Sub(b.previous) <= b.hub.batchArrivalGap
	b.previous = now
	if b.pending == nil {
		job := &broadcastJob{Clients: members, Packets: [][]byte{payload}}
		if !hot || len(members) < b.hub.minBatchViewers || b.hub.batchWindow == 0 || len(payload)+2 >= maxBatchBytes {
			b.hub.queueJob(b.room, job)
			return
		}
		b.pending = job
		b.size = len(payload) + 2
		b.expires = now.Add(b.hub.batchWindow)
		b.timer = time.AfterFunc(b.hub.batchWindow, func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.pending == job {
				b.flush()
			}
		})
		return
	}
	b.pending.Packets = append(b.pending.Packets, payload)
	b.size += len(payload) + 1
	if len(b.pending.Packets) >= maxBatchMessages || b.size >= maxBatchBytes {
		b.flush()
	}
}
