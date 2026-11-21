package hub

import (
	"context" // context is used to manage the lifecycle of the Redis client
	"fmt"
	"github.com/scthy/live-stream-danmaku/internal/mq"
	"github.com/scthy/live-stream-danmaku/internal/pubsub"
	"hash/maphash"
	"log/slog"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time" //broadcast stats data timer
)

type BroadcastJob struct {
	queued  time.Time
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

type Hub struct {
	log       *slog.Logger
	producer  *mq.Producer
	life      context.Context
	stop      context.CancelFunc
	drainOnce sync.Once
	retiring  atomic.Bool

	ServerID string

	Register chan *Client

	Unregister chan *Client

	mux       *pubsub.Mux
	shardSeed maphash.Seed
	ingress   sync.RWMutex
	draining  bool

	started         chan struct{}
	sendCapacity    int
	BatchWindow     time.Duration
	MinBatchViewers int
	BatchArrivalGap time.Duration
	Metrics
	statsRunning atomic.Bool
	ready        map[string]<-chan struct{}
	store        *pubsub.Store

	Rooms map[string]map[*Client]struct{}

	snapshots map[string][]*Client

	cancelSub map[string]context.CancelFunc

	mu sync.RWMutex // Protects the Rooms map

	deliveryQueues []chan *BroadcastJob

	localLikes map[string]uint64
	likesMu    sync.Mutex
}

const (
	BroadCastInterVal    = 3 * time.Second
	BroadcastChanSize    = 1024
	BroadcastJobChanSize = 1000
	RoomShards           = 32
)

func New(log *slog.Logger, producer *mq.Producer, store *pubsub.Store, sendCapacity, shards, muxShards int) *Hub {

	hostname, _ := os.Hostname()
	serverID := fmt.Sprintf("%s-%d", hostname, rand.Intn(100000))

	life, cancel := context.WithCancel(context.Background())
	m := &Hub{
		log: log, producer: producer, life: life, stop: cancel,
		ServerID:       serverID,
		started:        make(chan struct{}),
		Register:       make(chan *Client),
		Unregister:     make(chan *Client),
		localLikes:     make(map[string]uint64),
		Rooms:          make(map[string]map[*Client]struct{}),
		ready:          make(map[string]<-chan struct{}),
		cancelSub:      make(map[string]context.CancelFunc),
		snapshots:      make(map[string][]*Client),
		mux:            store.Mux(life, muxShards),
		deliveryQueues: make([]chan *BroadcastJob, shards),
		sendCapacity:   sendCapacity,
		store:          store,
		shardSeed:      maphash.MakeSeed(),

		BatchArrivalGap: DefaultBatchArrivalGap,
	}
	for i := range shards {

		m.deliveryQueues[i] = make(chan *BroadcastJob, (BroadcastJobChanSize+shards-1)/shards)
	}
	return m
}

func (m *Hub) Start() {
	m.log.Info("[MANAGER] Multi-room Hub started")
	m.StartWorkers()
	close(m.started)
	statsTicker := time.NewTicker(BroadCastInterVal)
	defer statsTicker.Stop()

	for {
		select {
		case <-m.life.Done():
			return
		case client := <-m.Register:
			m.handleRegister(client)
		case client := <-m.Unregister:
			m.handleUnregister(client)
		case <-statsTicker.C:

			if n := m.dropped.Swap(0); n > 0 {
				m.log.Warn("Live delivery dropped", "recipient_messages", n)
			}
			if m.statsRunning.CompareAndSwap(false, true) {
				go func() { defer m.statsRunning.Store(false); m.broadcastStats() }()
			}
		}
	}
}

func (m *Hub) CloseClients() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, room := range m.Rooms {
		for c := range room {
			c.Close()
		}
	}
}

func (m *Hub) Drain(ctx context.Context) error {
	<-m.started
	m.drainOnce.Do(func() {
		m.ingress.Lock()
		m.draining = true
		m.ingress.Unlock()
		m.stop()
		m.CloseClients()
		m.mux.Close()
	})
	return m.producer.Drain(ctx)
}
