package ws

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/infra"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"go.uber.org/zap"
)

func (m *Manager) reply(c *Client, id, status string) {
	m.receipt(c, id, status, "")
}
func (m *Manager) receipt(c *Client, id, status, eventID string) {
	if !c.Session {
		return
	}
	data, _ := json.Marshal(map[string]string{"client_msg_id": id, "status": status, "event_id": eventID})
	wire, _ := json.Marshal(model.WsPacket{Type: 104, Data: data})
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

func (m *Manager) dispatch(c *Client, packet model.WsPacket) {
	switch packet.Type {
	case model.TypeDanmaku:
		m.handleDanmaku(c, packet.Data)
	case model.ActionLike:
		m.handleLike(c, packet.Data)
	default:
		logger.Log.Warn("Unknown WebSocket message type", zap.Int("type", packet.Type))
	}
}

func (m *Manager) handleDanmaku(c *Client, data []byte) {
	var input model.DanmakuMessage
	if json.Unmarshal(data, &input) != nil || !input.FitsHistory() ||
		strings.TrimSpace(input.Content) == "" || len(input.ClientMsgID) > 80 ||
		(input.VideoOffsetMS != nil && (*input.VideoOffsetMS < 0 || *input.VideoOffsetMS >= 86400000)) {
		m.reply(c, input.ClientMsgID, "invalid")
		return
	}
	msg := model.DanmakuMessage{EventID: rand.Text(), RoomID: c.RoomID, UserID: c.UserID, Content: input.Content, SendTime: time.Now(), VideoOffsetMS: input.VideoOffsetMS, ClientMsgID: input.ClientMsgID}
	body, _ := json.Marshal(msg)
	packet := &model.WsPacket{Type: model.TypeDanmaku, RoomID: c.RoomID, Data: body}
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
	var like model.Like
	if json.Unmarshal(data, &like) != nil || like.Count == 0 || like.Count > 20 {
		m.reply(c, "", "invalid")
		return
	}
	if !m.permitted(c, model.ActionLike, "") {
		return
	}
	m.likesMu.Lock()
	m.localLikes[c.RoomID] += like.Count
	m.likesMu.Unlock()
}
