package hub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

func (m *Hub) broadcastStats() {
	m.likesMu.Lock()
	currentBatchLikes := m.localLikes
	m.localLikes = make(map[string]uint64)
	m.likesMu.Unlock()
	m.mu.RLock()
	localStats := make(map[string]int)
	for roomID, clients := range m.rooms {
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
		ctx, cancel := context.WithTimeout(m.life, 2*time.Second)
		defer cancel()
		stats, err := m.store.SnapshotStats(ctx, m.serverID, chunk, currentBatchLikes, statsInterval*2+time.Second)
		if err != nil {
			m.likesMu.Lock()
			for room := range chunk {
				if n := currentBatchLikes[room]; n > 0 {
					m.localLikes[room] += n
				}
			}
			m.likesMu.Unlock()
			m.log.Warn("Room statistics unavailable", "error", err)
			return
		}
		for room, s := range stats {
			data, err := json.Marshal(s)
			if err != nil {
				m.log.Error("Encode WebSocket packet", "error", err)
				continue
			}
			wire, err := json.Marshal(protocol.WsPacket{Type: protocol.TypeStats, RoomID: room, Data: data})
			if err != nil {
				m.log.Error("Encode WebSocket packet", "error", err)
				continue
			}
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
