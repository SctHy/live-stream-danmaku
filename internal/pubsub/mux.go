package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

type subscription struct {
	ready   chan struct{}
	deliver func([]byte)
}

type shard struct {
	mu    sync.RWMutex
	conn  *redis.PubSub
	rooms map[string]*subscription
}

type Mux struct {
	seed    maphash.Seed
	shards  []shard
	workers sync.WaitGroup
}

func NewMux(ctx context.Context, r *redis.Client, count int) *Mux {
	m := &Mux{seed: maphash.MakeSeed(), shards: make([]shard, count)}
	for i := range m.shards {
		s := &m.shards[i]
		s.conn = r.Subscribe(ctx)
		s.rooms = make(map[string]*subscription)
		m.workers.Go(func() { s.receive() })
	}
	return m
}

func (m *Mux) shard(room string) *shard {
	return &m.shards[maphash.String(m.seed, room)%uint64(len(m.shards))]
}

func (m *Mux) Subscribe(ctx context.Context, room string, deliver func([]byte)) (<-chan struct{}, error) {
	s := m.shard(room)
	s.mu.Lock()
	defer s.mu.Unlock()
	channel := fmt.Sprintf("dm:{%s}:pubsub", room)
	ready := make(chan struct{})
	s.rooms[channel] = &subscription{ready, deliver}
	if err := s.conn.Subscribe(ctx, channel); err != nil {
		delete(s.rooms, channel)
		s.conn.Unsubscribe(ctx, channel)
		return nil, err
	}
	return ready, nil
}

func (m *Mux) Unsubscribe(ctx context.Context, room string) {
	s := m.shard(room)
	s.mu.Lock()
	defer s.mu.Unlock()
	channel := fmt.Sprintf("dm:{%s}:pubsub", room)
	delete(s.rooms, channel)
	s.conn.Unsubscribe(ctx, channel)
}

func (s *shard) receive() {
	for event := range s.conn.ChannelWithSubscriptions() {
		switch v := event.(type) {
		case *redis.Subscription:
			s.mu.Lock()
			if sub := s.rooms[v.Channel]; v.Kind == "subscribe" && sub != nil && sub.ready != nil {
				close(sub.ready)
				sub.ready = nil
			}
			s.mu.Unlock()
		case *redis.Message:
			s.mu.RLock()
			sub := s.rooms[v.Channel]
			s.mu.RUnlock()
			if sub == nil {
				continue
			}
			var envelope struct {
				Position string            `json:"pos"`
				Previous string            `json:"prev"`
				Packet   protocol.WsPacket `json:"packet"`
			}
			if json.Unmarshal([]byte(v.Payload), &envelope) != nil {
				continue
			}
			envelope.Packet.Position = envelope.Position
			envelope.Packet.Previous = envelope.Previous
			payload, err := json.Marshal(envelope.Packet)
			if err == nil {
				sub.deliver(payload)
			}
		}
	}
}

func (m *Mux) Close() {
	for i := range m.shards {
		m.shards[i].conn.Close()
	}
	m.workers.Wait()
}
