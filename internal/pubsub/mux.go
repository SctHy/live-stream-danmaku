package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"hash/maphash"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

type subscription struct {
	ack     string
	ready   chan struct{}
	deliver func([]byte)
}

type shard struct {
	life     context.Context
	commands chan func()
	done     chan struct{}
	conn     *redis.PubSub
	rooms    map[string]*subscription
	pending  map[string]*subscription
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
		s.life = ctx
		s.conn = r.Subscribe(ctx)
		s.rooms = make(map[string]*subscription)
		s.pending = make(map[string]*subscription)
		s.commands = make(chan func())
		s.done = make(chan struct{})
		events := s.conn.ChannelWithSubscriptions()
		m.workers.Go(func() { s.receive(events) })
	}
	return m
}

func (m *Mux) shard(room string) *shard {
	return &m.shards[maphash.String(m.seed, room)%uint64(len(m.shards))]
}

func (m *Mux) Subscribe(ctx context.Context, room string, deliver func([]byte)) (<-chan struct{}, error) {
	s := m.shard(room)
	channel := fmt.Sprintf(KeyRoomPubSub, room)
	ready := make(chan struct{})
	ack := channel + ":ready:" + rand.Text()
	sub := &subscription{ack, ready, deliver}
	result := make(chan error, 1)
	select {
	case s.commands <- func() {
		op, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := s.conn.Subscribe(op, channel, ack)
		cancel()
		if err == nil {
			s.rooms[channel] = sub
			s.pending[ack] = sub
		} else {
			s.unsubscribe(ctx, channel, ack)
		}
		result <- err
	}:
	case <-s.done:
		return nil, redis.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := <-result; err != nil {
		return nil, err
	}
	return ready, nil
}

func (m *Mux) Unsubscribe(ctx context.Context, room string) {
	s := m.shard(room)
	channel := fmt.Sprintf(KeyRoomPubSub, room)
	done := make(chan struct{})
	select {
	case s.commands <- func() {
		defer close(done)
		sub := s.rooms[channel]
		if sub == nil {
			return
		}
		delete(s.rooms, channel)
		channels := []string{channel}
		if sub.ready != nil {
			delete(s.pending, sub.ack)
			channels = append(channels, sub.ack)
		}
		s.unsubscribe(ctx, channels...)
	}:
	case <-s.done:
		return
	case <-ctx.Done():
		return
	}
	select {
	case <-done:
	case <-s.done:
	case <-ctx.Done():
	}
}

func (s *shard) unsubscribe(parent context.Context, channels ...string) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	if err := s.conn.Unsubscribe(ctx, channels...); err != nil && !errors.Is(err, redis.ErrClosed) {
		slog.Warn("Redis unsubscribe", "error", err)
	}
}

func (s *shard) receive(events <-chan any) {
	defer close(s.done)
	for {
		select {
		case <-s.life.Done():
			return
		case command := <-s.commands:
			command()
		case event, ok := <-events:
			if !ok {
				return
			}
			switch v := event.(type) {
			case *redis.Subscription:
				if v.Kind != "subscribe" {
					continue
				}

				if sub := s.pending[v.Channel]; sub != nil {
					close(sub.ready)
					sub.ready = nil
					delete(s.pending, v.Channel)
					s.unsubscribe(s.life, v.Channel)
				}
			case *redis.Message:
				sub := s.rooms[v.Channel]
				if sub == nil || sub.ready != nil {
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
}

func (m *Mux) Close() {
	for i := range m.shards {
		if err := m.shards[i].conn.Close(); err != nil && !errors.Is(err, redis.ErrClosed) {
			slog.Warn("Close Redis subscription", "error", err)
		}
	}
	m.workers.Wait()
}
