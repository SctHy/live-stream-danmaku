package hub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"

	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

func (m *Hub) reply(c *Client, id, status string) {
	m.receipt(c, id, status, "")
}
func (m *Hub) receipt(c *Client, id, status, eventID string) {
	if status != "accepted" {
		m.metrics.Reject(status)
	}
	data, err := json.Marshal(protocol.Receipt{ClientMsgID: id, Status: status, EventID: eventID})
	if err != nil {
		m.log.Error("Encode WebSocket packet", "error", err)
		return
	}
	wire, err := json.Marshal(protocol.WsPacket{Type: protocol.TypeReceipt, Data: data})
	if err != nil {
		m.log.Error("Encode WebSocket packet", "error", err)
		return
	}
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
	if c.LoginNickname != nil {
		ctx, cancel := context.WithTimeout(m.life, 500*time.Millisecond)
		name, err := c.LoginNickname(ctx)
		cancel()
		if err != nil {
			c.Close()
			return
		}
		c.nickname = name
	}
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
	var allowed bool
	input.Content, allowed = m.FilterMessage(input.Content)
	if !allowed {
		m.reply(c, input.ClientMsgID, "filtered")
		return
	}
	msg := protocol.DanmakuMessage{Color: input.Color, Mode: input.Mode, Nickname: c.nickname, EventID: rand.Text(), RoomID: c.RoomID, UserID: c.UserID, Content: input.Content, SendTime: time.Now(), ClientMsgID: input.ClientMsgID}
	room, session, _ := strings.Cut(c.RoomID, "/")
	if strings.HasPrefix(session, "obs-") {
		if obs, ok := m.store.(interface {
			OBS(context.Context, string) (pubsub.OBSStream, error)
		}); ok {
			ctx, cancel := context.WithTimeout(m.life, 500*time.Millisecond)
			active, err := obs.OBS(ctx, room)
			cancel()
			if err != nil {
				m.reply(c, input.ClientMsgID, "unavailable")
				return
			}
			if active.RoomID == c.RoomID {
				offset := max(int64(0), msg.SendTime.UnixMilli()-active.Started)
				msg.VideoOffsetMS = &offset
			}
		}
	}
	body, err := json.Marshal(msg)
	if err != nil {
		m.log.Error("Encode WebSocket packet", "error", err)
		return
	}
	packet := &protocol.WsPacket{Type: protocol.TypeDanmaku, RoomID: c.RoomID, Data: body}
	m.ingress.Lock()
	if m.draining {
		m.ingress.Unlock()
		m.reply(c, input.ClientMsgID, "unavailable")
		return
	}

	m.publishing.Add(1)
	m.ingress.Unlock()
	defer m.publishing.Done()
	ctx, cancel := context.WithTimeout(m.life, 500*time.Millisecond)
	defer cancel()
	wire, err := json.Marshal(packet)
	if err != nil {
		m.log.Error("Encode WebSocket packet", "error", err)
		return
	}
	status, eventID, err := m.store.AcceptDanmaku(ctx, msg, wire)
	if err != nil {
		m.log.Warn("Redis publication failed", "error", err)
	}
	if status == "new" {
		m.likesMu.Lock()
		minute := msg.SendTime.Unix() / 60
		if m.localHeat[minute] == nil {
			m.localHeat[minute] = make(map[string]uint64)
		}
		m.localHeat[minute][room]++
		m.likesMu.Unlock()
		m.producer.Publish(c.RoomID, wire)
		if c.Interest != nil {
			c.Interest(2)
		}
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
	if c.Interest != nil {
		c.Interest(1)
	}
}

func (m *Hub) FilterMessage(content string) (string, bool) {
	if m == nil || m.filter == nil {
		return content, true
	}
	if m.maskWords {
		masked, _ := m.filter.Mask(content, '*')
		return masked, true
	}
	return content, !m.filter.Contains(content)
}
