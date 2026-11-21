package hub

import (
	"context"
)

func (m *Hub) handleRegister(client *Client) {

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Retiring() || (m.life != nil && m.life.Err() != nil) {
		client.Close()
		return
	}

	if _, ok := m.Rooms[client.RoomID]; !ok {
		m.Rooms[client.RoomID] = make(map[*Client]struct{})

		ctx, cancel := context.WithCancel(m.life)
		m.cancelSub[client.RoomID] = cancel

		batch := &roomBatch{hub: m, room: client.RoomID}
		ready, err := m.mux.Subscribe(ctx, client.RoomID, func(data []byte) { batch.push(ctx, data) })
		if err != nil {
			cancel()
			delete(m.Rooms, client.RoomID)
			delete(m.cancelSub, client.RoomID)
			client.Close()
			return
		}
		m.ready[client.RoomID] = ready

		m.log.Info("Room subscription started", "room", client.RoomID)
	}

	m.Rooms[client.RoomID][client] = struct{}{}
	delete(m.snapshots, client.RoomID)
	client.subscription = m.ready[client.RoomID]
	if client.registered != nil {
		close(client.registered)
	}

}

func (m *Hub) handleUnregister(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, ok := m.Rooms[client.RoomID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			delete(m.snapshots, client.RoomID)

			client.Close()
			if len(clients) == 0 {
				delete(m.Rooms, client.RoomID)
				delete(m.ready, client.RoomID)
				if cancel, exists := m.cancelSub[client.RoomID]; exists {
					cancel()
					m.mux.Unsubscribe(m.life, client.RoomID)
					delete(m.cancelSub, client.RoomID)
					m.log.Info("Room subscription stopped", "room", client.RoomID)
				}
			}

		}
	}
}

func (m *Hub) roomSnapshot(roomID string) []*Client {
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

func (m *Hub) Join(ctx context.Context, c *Client) bool {
	select {
	case m.Register <- c:
		return true
	case <-ctx.Done():
		return false
	case <-m.life.Done():
		return false
	}
}
func (m *Hub) Leave(c *Client) {
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
