package hub

import (
	"context"
	"crypto/rand"
	"hash/maphash"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/pubsub"
)

type BroadcastJob struct {
	queued  time.Time
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

type Hub struct {
	log       *slog.Logger
	producer  Publisher
	life      context.Context
	stop      context.CancelFunc
	drainErr  error
	drainOnce sync.Once
	retiring  atomic.Bool

	ServerID string

	register   chan *Client
	unregister chan *Client
	mux        *pubsub.Mux
	shardSeed  maphash.Seed
	ingress    sync.RWMutex
	draining   bool

	started         chan struct{}
	sendCapacity    int
	BatchWindow     time.Duration
	MinBatchViewers int
	BatchArrivalGap time.Duration
	metrics         *metrics.GatewayMetrics
	filter          WordFilter
	maskWords       bool
	statsRunning    atomic.Bool
	ready           map[string]<-chan struct{}
	store           *pubsub.Store

	rooms map[string]map[*Client]struct{}

	snapshots map[string][]*Client

	cancelSub map[string]context.CancelFunc

	mu sync.RWMutex // Protects the rooms map

	deliveryQueues []chan *BroadcastJob

	localLikes map[string]uint64
	likesMu    sync.Mutex
}

const (
	statsInterval = 3 * time.Second

	BroadcastJobChanSize = 1000
	RoomShards           = 32
)

type Publisher interface {
	Publish(string, []byte)
	Drain(context.Context) error
}

type WordFilter interface {
	Contains(string) bool
	Mask(string, rune) (string, bool)
}

type Config struct {
	Log                             *slog.Logger
	Producer                        Publisher
	Store                           *pubsub.Store
	Metrics                         *metrics.GatewayMetrics
	Filter                          WordFilter
	MaskWords                       bool
	SendCapacity, Shards, MuxShards int
}

func New(cfg Config) *Hub {
	serverID := rand.Text()

	life, cancel := context.WithCancel(context.Background())
	m := &Hub{
		log:            cfg.Log,
		producer:       cfg.Producer,
		metrics:        cfg.Metrics,
		filter:         cfg.Filter,
		maskWords:      cfg.MaskWords,
		life:           life,
		stop:           cancel,
		ServerID:       serverID,
		started:        make(chan struct{}),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		localLikes:     make(map[string]uint64),
		rooms:          make(map[string]map[*Client]struct{}),
		ready:          make(map[string]<-chan struct{}),
		cancelSub:      make(map[string]context.CancelFunc),
		snapshots:      make(map[string][]*Client),
		mux:            cfg.Store.Mux(life, cfg.MuxShards),
		deliveryQueues: make([]chan *BroadcastJob, cfg.Shards),
		sendCapacity:   cfg.SendCapacity,
		store:          cfg.Store,
		shardSeed:      maphash.MakeSeed(),

		BatchArrivalGap: DefaultBatchArrivalGap,
	}
	for i := range cfg.Shards {
		m.deliveryQueues[i] = make(chan *BroadcastJob, (BroadcastJobChanSize+cfg.Shards-1)/cfg.Shards)
	}
	return m
}

func (m *Hub) Start() {
	m.log.Info("Hub started")
	m.StartWorkers()
	close(m.started)
	statsTicker := time.NewTicker(statsInterval)
	defer statsTicker.Stop()

	for {
		select {
		case <-m.life.Done():
			return
		case client := <-m.register:
			m.handleRegister(client)
		case client := <-m.unregister:
			m.handleUnregister(client)
		case <-statsTicker.C:
			if m.statsRunning.CompareAndSwap(false, true) {
				go func() { defer m.statsRunning.Store(false); m.broadcastStats() }()
			}
		}
	}
}

func (m *Hub) CloseClients() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, room := range m.rooms {
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
		m.drainErr = m.producer.Drain(ctx)
	})
	return m.drainErr
}
