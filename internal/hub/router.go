package hub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

func (m *Hub) reply(c *Client, id, status string) {
	m.receipt(c, id, status, "")
}
func (m *Hub) receipt(c *Client, id, status, eventID string) {
	if status != "accepted" {
		m.metrics.Reject(status)
	}
	data, _ := json.Marshal(protocol.Receipt{ClientMsgID: id, Status: status, EventID: eventID})
	wire, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeReceipt, Data: data})
	m.safeSend(c, m.frame(wire, 1))
}

func (m *Hub) permitted(c *Client, action int, id string) bool {
	ctx, cancel := context.WithTimeout(m.life, 500*time.Millisecond)
	defer cancel()
	status, err := m.store.Policy(ctx, c.RoomID, c.UserID, action)
	if err != nil {
		status = "unavailable"
	}
	if status != "ok" {
		m.reply(c, id, status)
		return false
	}
	return true
}

func (m *Hub) dispatch(c *Client, packet protocol.WsPacket) {
	switch packet.Type {
	case protocol.TypeDanmaku:
		m.handleDanmaku(c, packet.Data)
	case protocol.TypeLike:
		m.handleLike(c, packet.Data)
	default:
		m.metrics.Reject("invalid")
		m.log.Warn("Unknown WebSocket message type", "type", packet.Type)
	}
}

func (m *Hub) handleDanmaku(c *Client, data []byte) {
	var input protocol.DanmakuMessage
	if json.Unmarshal(data, &input) != nil || !input.FitsHistory() ||
		strings.TrimSpace(input.Content) == "" || len(input.ClientMsgID) > 80 {
		m.reply(c, input.ClientMsgID, "invalid")
		return
	}
	if m.filter != nil {
		if m.maskWords {
			input.Content, _ = m.filter.Mask(input.Content, '*')
		} else if m.filter.Contains(input.Content) {
			m.reply(c, input.ClientMsgID, "filtered")
			return
		}
	}
	msg := protocol.DanmakuMessage{EventID: rand.Text(), RoomID: c.RoomID, UserID: c.UserID, Content: input.Content, SendTime: time.Now(), ClientMsgID: input.ClientMsgID}
	body, _ := json.Marshal(msg)
	packet := &protocol.WsPacket{Type: protocol.TypeDanmaku, RoomID: c.RoomID, Data: body}
	m.ingress.RLock()
	defer m.ingress.RUnlock()
	if m.draining {
		m.reply(c, input.ClientMsgID, "unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(m.life, 500*time.Millisecond)
	defer cancel()
	wire, _ := json.Marshal(packet)
	status, eventID, err := m.store.AcceptDanmaku(ctx, msg, wire)
	if err != nil {
		m.log.Warn("Redis publication failed", "error", err)
	}
	if status == "new" {
		m.producer.Publish(c.RoomID, wire)
		status = "accepted"
	}
	m.receipt(c, input.ClientMsgID, status, eventID)

}

func (m *Hub) handleLike(c *Client, data []byte) {
	var like protocol.Like
	if json.Unmarshal(data, &like) != nil || like.Count == 0 || like.Count > 20 {
		m.reply(c, "", "invalid")
		return
	}
	if !m.permitted(c, protocol.TypeLike, "") {
		return
	}
	m.likesMu.Lock()
	m.localLikes[c.RoomID] += like.Count
	m.likesMu.Unlock()
}
