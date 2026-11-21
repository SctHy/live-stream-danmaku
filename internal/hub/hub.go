package hub

import (
	"context" // context is used to manage the lifecycle of the Redis client
	"fmt"
	"github.com/scthy/live-stream-danmaku/internal/pubsub"
	"hash/maphash"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time" //broadcast stats data timer

	"github.com/scthy/live-stream-danmaku/internal/infra"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"go.uber.org/zap"
)

type BroadcastJob struct {
	queued  time.Time
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

type Hub struct {
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

	historyDone     chan struct{}
	started         chan struct{}
	sendCapacity    int
	BatchWindow     time.Duration
	MinBatchViewers int
	BatchArrivalGap time.Duration
	Metrics
	statsRunning atomic.Bool
	ready        map[string]<-chan struct{}
	historyQueue chan []byte
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

func New(store *pubsub.Store, sendCapacity, shards, muxShards int) *Hub {

	hostname, _ := os.Hostname()
	serverID := fmt.Sprintf("%s-%d", hostname, rand.Intn(100000))

	life, cancel := context.WithCancel(context.Background())
	m := &Hub{
		life: life, stop: cancel,
		ServerID:       serverID,
		started:        make(chan struct{}),
		historyDone:    make(chan struct{}),
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
		historyQueue:   make(chan []byte, infra.ProducerChanBufSize),
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
	logger.Log.Info("[MANAGER] Multi-room Hub started")
	m.StartWorkers()
	go func() { defer close(m.historyDone); infra.PublishHistory(m.historyQueue) }()
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
				logger.Log.Warn("Live delivery dropped", zap.Uint64("recipient_messages", n))
			}
			infra.ReportDeliveryFailures()
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
		close(m.historyQueue)
		m.ingress.Unlock()
		m.stop()
		m.CloseClients()
		m.mux.Close()
	})
	select {
	case <-m.historyDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
