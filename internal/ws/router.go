package ws

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"

	"github.com/scthy/live-stream-danmaku/internal/infra"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"go.uber.org/zap"
)

func (m *Manager) reply(c *Client, id, status string) {
	m.receipt(c, id, status, "")
}
func (m *Manager) receipt(c *Client, id, status, eventID string) {
	if !c.Session {
		return
	}
	data, _ := json.Marshal(protocol.Receipt{ClientMsgID: id, Status: status, EventID: eventID})
	wire, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeReceipt, Data: data})
	m.safeSend(c, m.frame(wire, 1))
}

func (m *Manager) permitted(c *Client, action int, id string) bool {
	if !c.Session {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	status, err := infra.Policy(ctx, m.RedisClient, c.RoomID, c.UserID, action)
	if err != nil {
		status = "unavailable"
	}
	if status != "ok" {
		m.reply(c, id, status)
		return false
	}
	return true
}

func (m *Manager) dispatch(c *Client, packet protocol.WsPacket) {
	switch packet.Type {
	case protocol.TypeDanmaku:
		m.handleDanmaku(c, packet.Data)
	case protocol.TypeLike:
		m.handleLike(c, packet.Data)
	default:
		logger.Log.Warn("Unknown WebSocket message type", zap.Int("type", packet.Type))
	}
}

func (m *Manager) handleDanmaku(c *Client, data []byte) {
	var input protocol.DanmakuMessage
	if json.Unmarshal(data, &input) != nil || !input.FitsHistory() ||
		strings.TrimSpace(input.Content) == "" || len(input.ClientMsgID) > 80 ||
		(input.VideoOffsetMS != nil && (*input.VideoOffsetMS < 0 || *input.VideoOffsetMS >= 86400000)) {
		m.reply(c, input.ClientMsgID, "invalid")
		return
	}
	msg := protocol.DanmakuMessage{EventID: rand.Text(), RoomID: c.RoomID, UserID: c.UserID, Content: input.Content, SendTime: time.Now(), VideoOffsetMS: input.VideoOffsetMS, ClientMsgID: input.ClientMsgID}
	body, _ := json.Marshal(msg)
	packet := &protocol.WsPacket{Type: protocol.TypeDanmaku, RoomID: c.RoomID, Data: body}
	if c.Session {
		ctx, cancel := context.WithTimeout(context.Background(), infra.RedisCancelTimeout)
		defer cancel()
		wire, _ := json.Marshal(packet)
		status, eventID, err := infra.AcceptDanmaku(ctx, m.publisher, msg, wire)
		if err != nil {
			m.liveFailures.Add(1)
		}
		m.receipt(c, input.ClientMsgID, status, eventID)
		return
	}
	if m.enqueue(packet, c.done) {
		m.reply(c, input.ClientMsgID, "accepted")
	} else {
		m.reply(c, input.ClientMsgID, "overloaded")
	}
}

func (m *Manager) handleLike(c *Client, data []byte) {
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
