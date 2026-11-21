package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

type subscription struct {
	ack     string
	ready   chan struct{}
	deliver func([]byte)
}

type shard struct {
	mu      sync.RWMutex
	conn    *redis.PubSub
	rooms   map[string]*subscription
	pending map[string]*subscription
}

type Mux struct {
	seed    maphash.Seed
	shards  []shard
	workers sync.WaitGroup
}

func NewMux(ctx context.Context, r redis.UniversalClient, count int) *Mux {
	m := &Mux{seed: maphash.MakeSeed(), shards: make([]shard, count)}
	for i := range m.shards {
		s := &m.shards[i]
		s.conn = r.Subscribe(ctx)
		s.rooms = make(map[string]*subscription)
		s.pending = make(map[string]*subscription)
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
	// A unique acknowledgement distinguishes rapid leave/rejoin generations.
	ack := channel + ":ready:" + rand.Text()
	sub := &subscription{ack, ready, deliver}
	s.rooms[channel] = sub
	s.pending[ack] = sub
	if err := s.conn.Subscribe(ctx, channel, ack); err != nil {
		delete(s.rooms, channel)
		delete(s.pending, ack)
		s.conn.Unsubscribe(ctx, channel, ack)
		return nil, err
	}
	return ready, nil
}

func (m *Mux) Unsubscribe(ctx context.Context, room string) {
	s := m.shard(room)
	s.mu.Lock()
	defer s.mu.Unlock()
	channel := fmt.Sprintf("dm:{%s}:pubsub", room)
	sub := s.rooms[channel]
	if sub == nil {
		return
	}
	delete(s.rooms, channel)
	if sub.ready != nil {
		delete(s.pending, sub.ack)
		s.conn.Unsubscribe(ctx, channel, sub.ack)
	} else {
		s.conn.Unsubscribe(ctx, channel)
	}
}

func (s *shard) receive() {
	for event := range s.conn.ChannelWithSubscriptions() {
		switch v := event.(type) {
		case *redis.Subscription:
			s.mu.Lock()
			if sub := s.pending[v.Channel]; v.Kind == "subscribe" && sub != nil {
				close(sub.ready)
				sub.ready = nil
				delete(s.pending, v.Channel)
				s.conn.Unsubscribe(context.Background(), v.Channel)
			}
			s.mu.Unlock()
		case *redis.Message:
			s.mu.RLock()
			sub := s.rooms[v.Channel]
			if sub != nil && sub.ready != nil {
				sub = nil
			}
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
