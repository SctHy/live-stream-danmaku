package hub

import "github.com/scthy/maku/internal/events"

func (m *Hub) Events() *events.Bus { return m.bus }
func (m *Hub) like(roomID string, count uint64) {
	m.likesMu.Lock()
	if m.localLikes != nil {
		m.localLikes[roomID] += count
	}
	m.likesMu.Unlock()
}
