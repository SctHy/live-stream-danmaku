package hub

import (
	"context"
	"encoding/json"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"time"
)

// Migrate marks the gateway unready before snapshotting membership. A notified
// browser releases its old socket after syncing on the replacement gateway.
func (m *Hub) Migrate(ctx context.Context, batch, limit int, interval time.Duration) {
	m.mu.Lock()
	if m.retiring.Swap(true) {
		m.mu.Unlock()
		return
	}
	var waiting []*Client
	for _, room := range m.Rooms {
		for c := range room {
			if c.migration != nil {
				waiting = append(waiting, c)
			}
		}
	}
	m.mu.Unlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var active []*Client
	for len(waiting) > 0 || len(active) > 0 {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		remaining := active[:0]
		for _, c := range active {
			select {
			case <-c.done:
			default:
				remaining = append(remaining, c)
			}
		}
		active = remaining
		count := min(batch, len(waiting))
		if limit > 0 {
			count = min(count, limit-len(active))
		}
		for _, c := range waiting[:count] {
			c.migration <- struct{}{}
			active = append(active, c)
		}
		waiting = waiting[count:]
	}
}

func migrationNotice() *outboundFrame {
	wire, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeMigrate, Data: json.RawMessage(`{}`)})
	return &outboundFrame{payload: wire, count: 1}
}

func (m *Hub) Retiring() bool { return m.retiring.Load() }
