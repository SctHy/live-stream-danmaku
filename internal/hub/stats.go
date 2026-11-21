package hub

import (
	"context" // context is used to manage the lifecycle of the Redis client
	"encoding/json"
	"time" //broadcast stats data timer

	"github.com/scthy/live-stream-danmaku/internal/logger"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"go.uber.org/zap"
)

func (m *Hub) broadcastStats() {

	m.likesMu.Lock()
	currentBatchLikes := m.localLikes
	m.localLikes = make(map[string]uint64)
	m.likesMu.Unlock()

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
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stats, err := m.store.SnapshotStats(ctx, m.ServerID, chunk, currentBatchLikes, BroadCastInterVal*2+time.Second)
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
