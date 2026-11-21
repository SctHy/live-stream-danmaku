package ws

import (
	"context" // context is used to manage the lifecycle of the Redis client
	"encoding/json"
	"fmt"
	"hash/maphash"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time" //broadcast stats data timer

	"github.com/charlesAcmen/livestream-danmaku/internal/infra"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Job structure
type BroadcastJob struct {
	Clients []*Client // Immutable recipient snapshot.
	Packets [][]byte
}

// Manager is responsible for managing all websocket clients and the Redis connection
type Manager struct {
	// ServerID is a unique identifier for this running instance.
	// It is used to separate stats in Redis.
	ServerID string

	// Register channel: when a new client joins, send the client pointer to the channel
	Register chan *Client

	// Unregister channel: when a client disconnects, send the client pointer to the channel
	Unregister chan *Client

	publishQueues   []chan *model.WsPacket
	shardSeed       maphash.Seed
	sendCapacity    int
	BatchWindow     time.Duration
	MinBatchViewers int
	Prepared        bool
	dropped         atomic.Uint64
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

func NewManager(sendCapacity int) *Manager {
	// Generate a unique ID for this server instance.
	// In k8s, you might use os.Hostname().
	// Here we use Hostname + Random Int to ensure uniqueness during local restarts.
	hostname, _ := os.Hostname()
	serverID := fmt.Sprintf("%s-%d", hostname, rand.Intn(100000))

	rdb := infra.InitRedisClient()
	opts := *rdb.Options()
	opts.MaxRetries = -1
	m := &Manager{
		ServerID:       serverID,
		Register:       make(chan *Client),
		Unregister:     make(chan *Client),
		localLikes:     make(map[string]uint64),
		Rooms:          make(map[string]map[*Client]struct{}),
		cancelSub:      make(map[string]context.CancelFunc),
		snapshots:      make(map[string][]*Client),
		publishQueues:  make([]chan *model.WsPacket, RoomShards),
		deliveryQueues: make([]chan *BroadcastJob, RoomShards),
		sendCapacity:   sendCapacity,
		historyQueue:   make(chan []byte, infra.ProducerChanBufSize),
		publisher:      redis.NewClient(&opts),
		shardSeed:      maphash.MakeSeed(),
		RedisClient:    rdb,
	}
	for i := range RoomShards {
		m.publishQueues[i] = make(chan *model.WsPacket, BroadcastChanSize/RoomShards)
		m.deliveryQueues[i] = make(chan *BroadcastJob, (BroadcastJobChanSize+RoomShards-1)/RoomShards)
	}
	return m
}

// Creates the main loop for the manager.
func (m *Manager) Start() {
	logger.Log.Info("[MANAGER] Multi-room Manager started")
	m.StartWorkers()
	go infra.PublishHistory(m.historyQueue)
	statsTicker := time.NewTicker(BroadCastInterVal)
	defer statsTicker.Stop()

	for {
		select {
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
			go m.broadcastStats()
		}
	}
}

// StartWorkers should be called inside Manager.Start()
func (m *Manager) StartWorkers() {
	for i, queue := range m.publishQueues {
		go func() {
			for packet := range queue {
				m.handleBroadcast(packet)
			}
		}()
		go m.broadcastWorker(m.deliveryQueues[i])
	}
}

func (m *Manager) roomShard(room string) int {
	return int(maphash.String(m.shardSeed, room) % uint64(len(m.deliveryQueues)))
}

func (m *Manager) enqueue(packet *model.WsPacket) {
	m.publishQueues[m.roomShard(packet.RoomID)] <- packet
}

func (m *Manager) broadcastWorker(queue <-chan *BroadcastJob) {
	for job := range queue {
		m.deliver(job)
	}
}

func (m *Manager) handleRegister(client *Client) {
	//write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Initialize room map if it's the first client in this room on THIS server
	if _, ok := m.Rooms[client.RoomID]; !ok {
		m.Rooms[client.RoomID] = make(map[*Client]struct{})
		// 1. Create a cancelable context for this room's subscription
		// no timeout:cancel() is called when empty room,server closing or unsubscribe
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelSub[client.RoomID] = cancel
		// 2. Start a background goroutine to listen to THIS room's Redis channel
		// Pass context to subscriber
		go m.subscribeToRoom(ctx, client.RoomID)
		logger.Log.Info("[MANAGER] New room created on server", zap.String("room", client.RoomID))
	}

	//struct{} is type,second {} is initializing instance
	m.Rooms[client.RoomID][client] = struct{}{}
	delete(m.snapshots, client.RoomID)

	// Async Redis update: Online Count
	// because Incr involves network I/O,TCP round trip,queue in redis,
	// potentially blocking,timeout,shaking etc.
	// go infra.UpdateOnlineCount(m.RedisClient, client.RoomID, 1)
	// Reason: We use heartbeat in broadcastStats now.
	// logger.Log.Info("[MANAGER] Client registered",
	// 	zap.Uint64("uid", client.UserID),
	// 	zap.String("room", client.RoomID),
	// 	zap.Int("total", len(m.Rooms[client.RoomID])))
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
			// logger.Log.Info(
			// 	"[MANAGER] Client disconnected",
			// 	zap.Uint64("userID", client.UserID),
			// 	zap.Int("total", len(clients)),
			// )
			// Clean up room and STOP subscription if last client leaves
			if len(clients) == 0 {
				delete(m.Rooms, client.RoomID)
				if cancel, exists := m.cancelSub[client.RoomID]; exists {
					cancel() // Signals subscribeToRoom to exit
					delete(m.cancelSub, client.RoomID)
					logger.Log.Info("[MANAGER] Room subscription stopped", zap.String("room", client.RoomID))
				}
			}
			// Async Redis update: Decr Online Count
			// go infra.UpdateOnlineCount(m.RedisClient, client.RoomID, -1)
			// We use heartbeat in broadcastStats now.
		}
	}
}
func (m *Manager) handleBroadcast(packet *model.WsPacket) {
	// Serialize the envelope for distribution
	payload, err := json.Marshal(packet)
	if err != nil {
		logger.Log.Error("[MANAGER] Marshal failed", zap.Error(err))
		return
	}

	switch packet.Type {
	// Case A: Danmaku (User generated)
	// Send to Redis (so other servers see it).
	// Send to Kafka (for history storage).
	// Note: We do NOT call m.broadcastToLocalClients here.
	// Why? Because Redis Sub goroutine will receive it and call broadcastToLocalClients.
	// If we call it here, the local user will see the message twice!
	case model.TypeDanmaku:
		m.processDanmaku(packet.RoomID, payload)
	// Case B: Like (User generated)
	// Send to Redis (real-time).
	// Skip Kafka (usually likes are just counters, detailed logs might not be needed).
	case model.ActionLike:
	// that is not gonna happen cause like packet will not be sent in broadcast channel
	// m.processLike(packet)
	// Case A: Stats (Generated locally, sent locally)
	// Do NOT send to Redis (avoids broadcast storm).
	// Do NOT send to Kafka (no need to save transient stats).
	case model.TypeStats:
		m.broadcastToLocalClients(packet.RoomID, payload)
	default:
		logger.Log.Warn("[MANAGER] Unknown packet type", zap.Int("type", packet.Type))
	}
}

// processDanmaku handles user messages.
// Strategy: Persist to Kafka + Broadcast to All Servers (via Redis).
func (m *Manager) processDanmaku(roomID string, data []byte) {
	// Async IO to Redis and Kafka

	// 1. Persistence (Kafka)
	select {
	case m.historyQueue <- data:
	default:
		infra.HistoryQueueDrops.Add(1)
	}
	infra.PublishToRoom(m.publisher, roomID, data)
}

// processLike handles user likes.
// Strategy: Update State (Redis KV). Do NOT broadcast every single like to save bandwidth.
// The Ticker will pick up the new count later.
// func (m *Manager) processLike(packet *model.WsPacket) {
// 	// 1. Business Logic: Increment Redis Counter
// 	// We need to peek inside the Data to know the "Count"
// 	var cmd model.Like
// 	if err := json.Unmarshal(packet.Data, &cmd); err == nil {
// 		// Only update the counter in Redis.
// 		go infra.IncrRoomLikes(m.RedisClient, packet.RoomID, cmd.Count)
// 	}
// }

// broadcastStats fetches stats for ALL active rooms and sends updates locally.
func (m *Manager) broadcastStats() {
	// logger.Log.Info("[MANAGER]BroadcastStats called")

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

	// Decoupling: Calculate TTL here, not in infra
	// 3s interval -> 7s TTL (Interval * 2 + 1s)
	reportTTL := BroadCastInterVal*2 + time.Second

	for roomID, count := range localStats {
		// online, likes := infra.GetRoomStats(m.RedisClient, roomID)

		// STEP A: Report "I am alive and I have X users" to Redis
		// This overwrites any previous value for this server.
		// If this server crashes, this key will expire in 5 seconds.
		infra.UpdateServerOnline(m.RedisClient, roomID, m.ServerID, count, reportTTL)
		if likesDelta, ok := currentBatchLikes[roomID]; ok && likesDelta > 0 {
			go infra.IncrRoomLikes(m.RedisClient, roomID, likesDelta)
		}
		// STEP B: Fetch the global total (Sum of all servers)
		// We ignore likes for now as requested.
		totalOnline := infra.GetTotalOnline(m.RedisClient, roomID)
		totalLikes := infra.GetRoomLikes(m.RedisClient, roomID)
		stats := model.StatsData{
			Online: totalOnline,
			Likes:  totalLikes,
		}
		logger.Log.Info("[MANAGER] Room Stats",
			zap.String("roomID", roomID),
			zap.Uint64("likes", totalLikes),
			zap.Uint64("online", totalOnline),
		)
		dataBytes, _ := json.Marshal(stats)

		// Create packet
		packet := &model.WsPacket{
			Type:   model.TypeStats,
			RoomID: roomID,
			Data:   dataBytes,
		}

		payload, err := json.Marshal(packet)
		if err != nil {
			logger.Log.Error("[MANAGER] Marshal stats packet failed", zap.Error(err))
			continue
		}
		// Non-blocking send to self (avoid deadlock if channel is full)
		// select {
		// case m.Broadcast <- packet:
		// default:
		// 	logger.Log.Warn("[MANAGER] Broadcast channel full, skipping stats", zap.String("room", roomID))
		// }

		m.broadcastToLocalClients(roomID, payload)
	}
}

// subscribeToRoom listens to a specific Redis channel for a room
func (m *Manager) subscribeToRoom(ctx context.Context, roomID string) {
	// Channel name: e.g., "room:1001:pubsub"
	channelName := fmt.Sprintf(infra.KeyRoomPubSub, roomID)
	//context.Background(): a default context that never cancels, never expires, and has no values.
	//or context.WithTimeOut(...,3*time.Second)
	//here ctx is cancellable
	pubsub := m.RedisClient.Subscribe(ctx, channelName)
	defer pubsub.Close()
	// Go channel to receive Redis messages(read only)
	ch := pubsub.Channel()

	m.consumeRoom(ctx, roomID, ch)
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
	select {
	case m.deliveryQueues[m.roomShard(roomID)] <- job:
	default:
		m.dropped.Add(uint64(len(job.Packets)) * uint64(len(job.Clients)))
	}
}
