package hub

import (
	"context"
)

// Start serializes membership changes. The room lock also protects snapshots
// taken concurrently by delivery, migration and shutdown.
func (m *Hub) handleRegister(client *Client) {
	room := client.RoomID
	m.mu.Lock()
	if m.Retiring() || (m.life != nil && m.life.Err() != nil) {
		m.mu.Unlock()
		client.Close()
		return
	}
	newRoom := m.rooms[room] == nil
	if newRoom {
		m.mu.Unlock()
		ctx, cancel := context.WithCancel(m.life)
		batch := &roomBatch{hub: m, room: room}
		ready, err := m.mux.Subscribe(ctx, room, func(data []byte) { batch.push(ctx, data) })
		if err != nil {
			cancel()
			client.Close()
			return
		}
		m.mu.Lock()
		if m.Retiring() || m.life.Err() != nil {
			m.mu.Unlock()
			cancel()
			m.mux.Unsubscribe(m.life, room)
			client.Close()
			return
		}
		m.rooms[room] = make(map[*Client]struct{})
		m.cancelSub[room] = cancel
		m.ready[room] = ready
		m.metrics.Membership(0, 1)
	}
	m.rooms[room][client] = struct{}{}
	delete(m.snapshots, room)
	client.subscription = m.ready[room]
	m.metrics.Membership(1, 0)
	m.mu.Unlock()
	if newRoom {
		m.log.Info("Room subscription started", "room", room)
	}
	if client.registered != nil {
		close(client.registered)
	}
}

func (m *Hub) handleUnregister(client *Client) {
	room := client.RoomID
	m.mu.Lock()
	clients := m.rooms[room]
	if _, exists := clients[client]; !exists {
		m.mu.Unlock()
		return
	}
	delete(clients, client)
	delete(m.snapshots, room)
	m.metrics.Membership(-1, 0)
	var cancel context.CancelFunc
	if len(clients) == 0 {
		cancel = m.cancelSub[room]
		delete(m.cancelSub, room)
		delete(m.ready, room)
		delete(m.rooms, room)
		m.metrics.Membership(0, -1)
	}
	m.mu.Unlock()
	client.Close()
	if cancel != nil {
		cancel()
		m.mux.Unsubscribe(m.life, room)
		m.log.Info("Room subscription stopped", "room", room)
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
	clients := m.rooms[roomID]
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
	case m.register <- c:
		return true
	case <-ctx.Done():
		return false
	case <-m.life.Done():
		return false
	}
}
func (m *Hub) Leave(c *Client) {
	select {
	case m.unregister <- c:
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
