package protocol

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	TypeDanmaku  = 101
	TypeStats    = 102
	TypeLike     = 103
	TypeReceipt  = 104
	TypeWithdraw = 105
	TypeReady    = 106
	TypeMigrate  = 107
)

// WsPacket carries a typed payload and optional recovery cursors.
type WsPacket struct {
	Position string          `json:"pos,omitempty"`
	Previous string          `json:"prev,omitempty"`
	Type     int             `json:"type"`
	RoomID   string          `json:"room_id,omitempty"`
	Data     json.RawMessage `json:"data"`
}

// DanmakuMessage is a persisted event and the WebSocket chat payload.
type DanmakuMessage struct {
	ClientMsgID string    `json:"client_msg_id,omitempty"`
	EventID     string    `json:"event_id,omitempty"`
	ID          uint64    `json:"id"`
	RoomID      string    `json:"room_id"`
	UserID      uint64    `json:"user_id"`
	Content     string    `json:"content"`
	SendTime    time.Time `json:"send_time"`
}

// FitsHistory checks the PostgreSQL text limits before publishing or batching.
func (d DanmakuMessage) FitsHistory() bool {
	return utf8.RuneCountInString(d.RoomID) <= 50 && utf8.RuneCountInString(d.Content) <= 500 &&
		!strings.ContainsRune(d.RoomID, 0) && !strings.ContainsRune(d.Content, 0) && !strings.ContainsRune(d.EventID, 0)
}

type StatsData struct {
	Position string `json:"pos,omitempty"`
	Online   uint64 `json:"online"`
	Likes    uint64 `json:"likes"`
}

type Like struct {
	Count uint64 `json:"count"`
}

type Receipt struct {
	ClientMsgID string `json:"client_msg_id"`
	Status      string `json:"status"`
	EventID     string `json:"event_id"`
}
type Withdraw struct {
	EventID string `json:"event_id"`
}
type ClockReady struct {
	ServerTimeMS int64 `json:"server_time_ms"`
}
