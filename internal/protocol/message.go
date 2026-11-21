package protocol

import (
	"encoding/json" //Delayed Parsing
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

// WsPacket is the standard wrapper for ALL websocket communications.
// Every message sent or received must follow this structure.
// It contains metadata required for ROUTING without parsing the inner payload.
type WsPacket struct {
	// Type tells the receiver what 'Data' contains.
	Position string `json:"pos,omitempty"`
	Previous string `json:"prev,omitempty"`
	Type     int    `json:"type"`
	RoomID   string `json:"room_id,omitempty"` // Routing key
	// type RawMessage []byte,rather than map[int]interface{} using Assert
	Data json.RawMessage `json:"data"`
}

// DanmakuMessage is a persisted event and the WebSocket chat payload.
type DanmakuMessage struct {
	VideoOffsetMS *int64    `json:"video_offset_ms,omitempty"`
	ClientMsgID   string    `json:"client_msg_id,omitempty"`
	EventID       string    `json:"event_id,omitempty"`
	ID            uint64    `json:"id"`
	RoomID        string    `json:"room_id"`
	UserID        uint64    `json:"user_id"`
	Content       string    `json:"content"`
	SendTime      time.Time `json:"send_time"`
}

// FitsHistory checks the PostgreSQL text limits before publishing or batching.
func (d DanmakuMessage) FitsHistory() bool {
	return utf8.RuneCountInString(d.RoomID) <= 50 && utf8.RuneCountInString(d.Content) <= 500 &&
		!strings.ContainsRune(d.RoomID, 0) && !strings.ContainsRune(d.Content, 0) && !strings.ContainsRune(d.EventID, 0)
}

// StatsData: The content for TypeStats
type StatsData struct {
	Position string `json:"pos,omitempty"`
	Online   uint64 `json:"online"`
	Likes    uint64 `json:"likes"`
}

// CmdLike: The content for TypeLike (Client -> Server)
// Currently empty, but extensible (e.g., send 10 likes at once).
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
