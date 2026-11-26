package hub

import (
	"context"
	"crypto/rand"
	"errors"
	"hash/maphash"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

type broadcastJob struct {
	queued  time.Time
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

type roomStore interface {
	AcceptDanmakuPacket(context.Context, protocol.DanmakuMessage, []byte) (string, string, []byte, error)
	Policy(context.Context, string, uint64, int) (string, error)
	SnapshotStats(context.Context, string, map[string]int, map[string]uint64, time.Duration) (map[string]protocol.StatsData, error)
}

type roomSubscriptions interface {
	Subscribe(context.Context, string, func([]byte)) (<-chan struct{}, error)
	Unsubscribe(context.Context, string)
	Close()
}

type LiveBridge interface {
	Subscribe(string, uint64, func(protocol.LiveDanmaku))
	Unsubscribe(string)
	Close()
}

type SessionClock interface {
	OffsetMS(ctx context.Context, roomID string, at time.Time) (*int64, error)
}

type Hub struct {
	clock      SessionClock
	bus        *events.Bus
	liveBridge LiveBridge
	log        *slog.Logger
	producer   *mq.Producer
	life       context.Context
	stop       context.CancelFunc
	drainErr   error
	drained    chan struct{}
	retiring   atomic.Bool

	disableSlowKick bool

	serverID string

	register chan *Client

	unregister chan *Client

	mux        roomSubscriptions
	shardSeed  maphash.Seed
	ingress    sync.Mutex
	publishing sync.WaitGroup
	draining   bool

	started         chan struct{}
	sendCapacity    int
	batchWindow     time.Duration
	minBatchViewers int
	batchArrivalGap time.Duration
	metrics         *metrics.GatewayMetrics
	filter          *filter.Filter
	maskWords       bool
	statsRunning    atomic.Bool
	ready           map[string]<-chan struct{}
	store           roomStore

	rooms map[string]map[*Client]struct{}

	snapshots map[string][]*Client

	cancelSub map[string]context.CancelFunc

	mu sync.RWMutex // Protects the rooms map

	deliveryQueues []chan *broadcastJob

	likesMu      sync.Mutex
	localLikes   map[string]uint64
	compatHeat   *pubsub.HeatWriter
	statsStopped chan struct{}
}

const (
	statsInterval = 3 * time.Second

	broadcastJobChanSize = 1000
	RoomShards           = 32
)

type Config struct {
	Bus                    *events.Bus
	SessionClock           SessionClock
	DisableSlowKick        bool
	LiveBridge             LiveBridge
	Logger                 *slog.Logger
	Producer               *mq.Producer
	Store                  *pubsub.Store
	Metrics                *metrics.GatewayMetrics
	Filter                 *filter.Filter
	MaskWords              bool
	SendCapacity           int
	RoomShards             int
	PubSubShards           int
	BatchWindow            time.Duration
	MinBatchViewers        int
	BatchArrivalGap        time.Duration
	DisableArrivalBatching bool // Preserves the CLI's explicit zero arrival gap.
}

func New(cfg Config) *Hub {
	shards := cfg.RoomShards
	if shards <= 0 {
		shards = RoomShards
	}
	if cfg.SendCapacity <= 0 {
		cfg.SendCapacity = SendChanSize
	}
	if cfg.PubSubShards <= 0 {
		cfg.PubSubShards = 8
	}
	if cfg.BatchArrivalGap == 0 && !cfg.DisableArrivalBatching {
		cfg.BatchArrivalGap = DefaultBatchArrivalGap
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.SessionClock == nil && cfg.Store != nil {
		cfg.SessionClock = cfg.Store
	}
	serverID := rand.Text()

	life, cancel := context.WithCancel(context.Background())
	m := &Hub{
		clock:          cfg.SessionClock,
		liveBridge:     cfg.LiveBridge,
		log:            cfg.Logger,
		producer:       cfg.Producer,
		metrics:        cfg.Metrics,
		filter:         cfg.Filter,
		maskWords:      cfg.MaskWords,
		life:           life,
		stop:           cancel,
		serverID:       serverID,
		started:        make(chan struct{}),
		drained:        make(chan struct{}),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		localLikes:     make(map[string]uint64),
		statsStopped:   make(chan struct{}),
		rooms:          make(map[string]map[*Client]struct{}),
		ready:          make(map[string]<-chan struct{}),
		cancelSub:      make(map[string]context.CancelFunc),
		snapshots:      make(map[string][]*Client),
		mux:            cfg.Store.Mux(life, cfg.PubSubShards),
		deliveryQueues: make([]chan *broadcastJob, shards),
		sendCapacity:   cfg.SendCapacity,
		store:          cfg.Store,
		shardSeed:      maphash.MakeSeed(),

		disableSlowKick: cfg.DisableSlowKick,
		batchArrivalGap: cfg.BatchArrivalGap,
		batchWindow:     cfg.BatchWindow,
		minBatchViewers: cfg.MinBatchViewers,
	}
	m.bus = cfg.Bus
	if m.bus == nil {
		m.bus = new(events.Bus)
		m.compatHeat = pubsub.SubscribeHeat(m.bus, cfg.Store, cfg.Logger)
	}
	m.bus.Likes.Subscribe(func(_ context.Context, e events.LikeRecorded) { m.like(e.Room, e.Count) })
	if cfg.Producer != nil {
		cfg.Producer.Subscribe(m.bus)
	}
	for i := range shards {

		m.deliveryQueues[i] = make(chan *broadcastJob, (broadcastJobChanSize+shards-1)/shards)
	}
	return m
}

func (m *Hub) Start() {
	m.log.Info("Hub started")
	defer close(m.statsStopped)
	var stats sync.WaitGroup
	defer stats.Wait()
	if m.compatHeat != nil {
		stats.Go(func() { m.drainErr = m.compatHeat.Run(m.life) })
	}
	m.startWorkers()
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
				stats.Go(func() { defer m.statsRunning.Store(false); m.broadcastStats() })
			}
		}
	}
}

func (m *Hub) closeClients() {
	m.mu.RLock()
	var clients []*Client
	for _, room := range m.rooms {
		for c := range room {
			clients = append(clients, c)
		}
	}
	m.mu.RUnlock()
	for _, c := range clients {
		c.Close()
	}
}

func (m *Hub) Drain(ctx context.Context) error {
	<-m.started
	m.ingress.Lock()
	if m.draining {
		m.ingress.Unlock()
		<-m.drained
		return m.drainErr
	}
	m.draining = true
	m.ingress.Unlock()
	m.publishing.Wait()
	m.stop()
	<-m.statsStopped
	// Ingress and the statistics worker have stopped. Persist remaining likes
	// without refreshing the retiring gateway's online leases.
	m.likesMu.Lock()
	likes := m.localLikes
	m.localLikes = nil
	m.likesMu.Unlock()
	if len(likes) > 0 {
		rooms := make(map[string]int, len(likes))
		for room := range likes {
			rooms[room] = 0
		}
		_, err := m.store.SnapshotStats(ctx, m.serverID, rooms, likes, statsInterval*2+time.Second)
		m.drainErr = errors.Join(m.drainErr, err)
	}
	m.closeClients()
	if m.liveBridge != nil {
		m.liveBridge.Close()
	}
	m.mux.Close()
	if m.producer != nil {
		m.drainErr = errors.Join(m.drainErr, m.producer.Drain(ctx))
	}
	close(m.drained)
	return m.drainErr
}
