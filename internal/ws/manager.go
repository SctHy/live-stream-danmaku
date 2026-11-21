package ws

import (
	"context" // context is used to manage the lifecycle of the Redis client
	"encoding/json"
	"fmt"
	"github.com/scthy/live-stream-danmaku/internal/pubsub"
	"hash/maphash"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time" //broadcast stats data timer

	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/infra"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"go.uber.org/zap"
)

// Job structure
type BroadcastJob struct {
	queued  time.Time
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

// Manager is responsible for managing all websocket clients and the Redis connection
type Manager struct {
	life      context.Context
	stop      context.CancelFunc
	drainOnce sync.Once
	retiring  atomic.Bool

	// ServerID is a unique identifier for this running instance.
	// It is used to separate stats in Redis.
	ServerID string

	// Register channel: when a new client joins, send the client pointer to the channel
	Register chan *Client

	// Unregister channel: when a client disconnects, send the client pointer to the channel
	Unregister chan *Client

	mux             *pubsub.Mux
	shardSeed       maphash.Seed
	ingress         sync.RWMutex
	draining        bool
	publishWorkers  sync.WaitGroup
	historyDone     chan struct{}
	started         chan struct{}
	sendCapacity    int
	BatchWindow     time.Duration
	MinBatchViewers int
	BatchArrivalGap time.Duration
	dropped         atomic.Uint64
	shardDrops      atomic.Uint64
	clientDrops     atomic.Uint64
	rejected        atomic.Uint64
	waitNanos       atomic.Uint64
	measuredJobs    atomic.Uint64
	liveFailures    atomic.Uint64
	statsRunning    atomic.Bool
	ready           map[string]<-chan struct{}
	historyQueue    chan []byte
	publisher       *redis.Client

	// Rooms: Map of RoomID -> Set of Clients
	Rooms map[string]map[*Client]struct{}
	// Immutable snapshots guarded by mu. Membership changes invalidate the room entry.
	snapshots map[string][]*Client

	//Store cancel functions to stop Redis subscriptions
	cancelSub map[string]context.CancelFunc

	mu sync.RWMutex // Protects the Rooms map

	//Read and write mutex lock
	//RLock() when multiple routines read room map to broadcast simultaneously,
	//non blocking

	// Channel to distribute broadcast tasks to background workers
	deliveryQueues []chan *BroadcastJob

	// Map of RoomID -> Count of likes
	localLikes map[string]uint64
	likesMu    sync.Mutex

	// RedisClient: the connection to the Redis server.
	RedisClient *redis.Client
}

const (
	BroadCastInterVal    = 3 * time.Second
	BroadcastChanSize    = 1024
	BroadcastJobChanSize = 1000
	RoomShards           = 32
)

func NewManager(sendCapacity, shards, muxShards int) *Manager {
	// Generate a unique ID for this server instance.
	// In k8s, you might use os.Hostname().
	// Here we use Hostname + Random Int to ensure uniqueness during local restarts.
	hostname, _ := os.Hostname()
	serverID := fmt.Sprintf("%s-%d", hostname, rand.Intn(100000))

	rdb := infra.InitRedisClient(0)

	life, cancel := context.WithCancel(context.Background())
	m := &Manager{
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
		mux:            pubsub.NewMux(life, rdb, muxShards),
		deliveryQueues: make([]chan *BroadcastJob, shards),
		sendCapacity:   sendCapacity,
		historyQueue:   make(chan []byte, infra.ProducerChanBufSize),
		publisher:      infra.InitRedisClient(-1),
		shardSeed:      maphash.MakeSeed(),
		RedisClient:    rdb,

		BatchArrivalGap: DefaultBatchArrivalGap,
	}
	for i := range shards {

		m.deliveryQueues[i] = make(chan *BroadcastJob, (BroadcastJobChanSize+shards-1)/shards)
	}
	return m
}

// Creates the main loop for the manager.
func (m *Manager) Start() {
	logger.Log.Info("[MANAGER] Multi-room Manager started")
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
			// Do NOT call m.broadcastStats() directly here.
			// broadcastStats involves Redis I/O and loops through all rooms.
			// If run synchronously, it blocks the Register/Unregister channels,
			// causing a bottleneck (Head-of-Line Blocking).
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

// StartWorkers should be called inside Manager.Start()
func (m *Manager) StartWorkers() {
	for _, q := range m.deliveryQueues {
		go m.broadcastWorker(q)
	}
}

func (m *Manager) roomShard(room string) int {
	return int(maphash.String(m.shardSeed, room) % uint64(len(m.deliveryQueues)))
}

func (m *Manager) broadcastWorker(queue <-chan *BroadcastJob) {
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

func (m *Manager) handleRegister(client *Client) {
	//write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Retiring() || (m.life != nil && m.life.Err() != nil) {
		client.Close()
		return
	}
	// Only occupied rooms hold a Redis subscription.
	if _, ok := m.Rooms[client.RoomID]; !ok {
		m.Rooms[client.RoomID] = make(map[*Client]struct{})
		// 1. Create a cancelable context for this room's subscription
		// no timeout:cancel() is called when empty room,server closing or unsubscribe
		ctx, cancel := context.WithCancel(m.life)
		m.cancelSub[client.RoomID] = cancel
		// 2. Start a background goroutine to listen to THIS room's Redis channel
		// Pass context to subscriber
		messages := make(chan *redis.Message, 256)
		ready, err := m.mux.Subscribe(ctx, client.RoomID, func(data []byte) {
			select {
			case messages <- &redis.Message{Payload: string(data)}:
			case <-ctx.Done():
			}
		})
		if err != nil {
			cancel()
			delete(m.Rooms, client.RoomID)
			delete(m.cancelSub, client.RoomID)
			client.Close()
			return
		}
		m.ready[client.RoomID] = ready
		go m.consumeRoom(ctx, client.RoomID, messages)
		logger.Log.Info("[MANAGER] New room created on server", zap.String("room", client.RoomID))
	}

	//struct{} is type,second {} is initializing instance
	m.Rooms[client.RoomID][client] = struct{}{}
	delete(m.snapshots, client.RoomID)
	client.subscription = m.ready[client.RoomID]
	if client.registered != nil {
		close(client.registered)
	}

}

func (m *Manager) handleUnregister(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, ok := m.Rooms[client.RoomID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			delete(m.snapshots, client.RoomID)
			// Do NOT close(client.Send) here if workers are still using it.
			// close(client.Send)
			client.Close()
			if len(clients) == 0 {
				delete(m.Rooms, client.RoomID)
				delete(m.ready, client.RoomID)
				if cancel, exists := m.cancelSub[client.RoomID]; exists {
					cancel()
					m.mux.Unsubscribe(m.life, client.RoomID)
					delete(m.cancelSub, client.RoomID)
					logger.Log.Info("[MANAGER] Room subscription stopped", zap.String("room", client.RoomID))
				}
			}

		}
	}
}

// broadcastStats fetches stats for ALL active rooms and sends updates locally.
func (m *Manager) broadcastStats() {

	m.likesMu.Lock()
	currentBatchLikes := m.localLikes
	m.localLikes = make(map[string]uint64)
	m.likesMu.Unlock()

	// 1. Iterate over all active rooms on this server
	// We need to fetch and broadcast stats for EACH room separately.
	m.mu.RLock()
	localStats := make(map[string]int)
	for roomID, clients := range m.Rooms {
		localStats[roomID] = len(clients)
	}

	m.mu.RUnlock()

	for room := range currentBatchLikes {
		if _, ok := localStats[room]; !ok {
			localStats[room] = 0
		}
	}
	chunk := make(map[string]int, 128)
	flush := func() {
		ctx, cancel := context.WithTimeout(context.Background(), infra.RedisCancelTimeout)
		defer cancel()
		stats, err := infra.SnapshotStats(ctx, m.publisher, m.ServerID, chunk, currentBatchLikes, BroadCastInterVal*2+time.Second)
		if err != nil {
			logger.Log.Warn("Room statistics unavailable", zap.Error(err))
			return
		}
		for room, s := range stats {
			data, _ := json.Marshal(s)
			wire, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeStats, RoomID: room, Data: data})
			frame := m.frame(wire, 1)
			for _, client := range m.roomSnapshot(room) {
				client.setStats(frame)
			}
		}
	}
	for room, count := range localStats {
		chunk[room] = count
		if len(chunk) == 128 {
			flush()
			clear(chunk)
		}
	}
	if len(chunk) > 0 {
		flush()
	}
}

// roomSnapshot reuses a read-only recipient list until the room membership changes.
// Jobs already queued can finish using the previous snapshot.
func (m *Manager) roomSnapshot(roomID string) []*Client {
	m.mu.RLock()
	snapshot := m.snapshots[roomID]
	m.mu.RUnlock()
	if snapshot != nil {
		return snapshot
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if snapshot = m.snapshots[roomID]; snapshot != nil {
		return snapshot
	}
	clients := m.Rooms[roomID]
	if len(clients) == 0 {
		return nil
	}
	// Group protocol versions once per membership change. Legacy receivers can
	// then bypass the batch timer even in a mixed room.
	snapshot = make([]*Client, len(clients))
	front, back := 0, len(snapshot)-1
	for client := range clients {
		if client.batch {
			snapshot[front] = client
			front++
		} else {
			snapshot[back] = client
			back--
		}
	}
	m.snapshots[roomID] = snapshot
	return snapshot
}

func (m *Manager) broadcastToLocalClients(roomID string, payload []byte) {
	clients := m.roomSnapshot(roomID)
	if len(clients) == 0 {
		return
	}
	m.queueJob(roomID, &BroadcastJob{Clients: clients, Packets: [][]byte{payload}})
}

func (m *Manager) queueJob(roomID string, job *BroadcastJob) {
	job.queued = time.Now()
	select {
	case m.deliveryQueues[m.roomShard(roomID)] <- job:
	default:
		m.shardDrops.Add(uint64(len(job.Packets)) * uint64(len(job.Clients)))
		m.dropped.Add(uint64(len(job.Packets)) * uint64(len(job.Clients)))
	}
}

func (m *Manager) Metrics() map[string]uint64 {
	return map[string]uint64{
		"shard_recipient_drops":        m.shardDrops.Load(),
		"client_recipient_drops":       m.clientDrops.Load(),
		"publish_queue_rejected":       m.rejected.Load(),
		"delivery_queue_wait_ns_total": m.waitNanos.Load(),
		"delivery_jobs":                m.measuredJobs.Load(),
		"journal_errors":               m.liveFailures.Load(),
		"history_queue_entries":        uint64(len(m.historyQueue)),
	}
}
func (m *Manager) Join(ctx context.Context, c *Client) bool {
	select {
	case m.Register <- c:
		return true
	case <-ctx.Done():
		return false
	case <-m.life.Done():
		return false
	}
}
func (m *Manager) Leave(c *Client) {
	select {
	case m.Unregister <- c:
	case <-m.life.Done():
		c.Close()
	}
}
func (c *Client) AwaitSubscription(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-c.done:
		return false
	case <-c.registered:
	}
	select {
	case <-ctx.Done():
		return false
	case <-c.done:
		return false
	case <-c.subscription:
		return true
	}
}
func (m *Manager) CloseClients() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, room := range m.Rooms {
		for c := range room {
			c.Close()
		}
	}
}

// Drain stops accepting publications and lets already accepted history reach the producer.
func (m *Manager) Drain(ctx context.Context) error {
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
