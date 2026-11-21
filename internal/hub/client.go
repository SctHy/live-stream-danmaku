package hub

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

const (
	SendChanSize = 512
	PongWait     = 60 * time.Second
	PingPeriod   = 30 * time.Second
	WriteWait    = 5 * time.Second
)

type Client struct {
	migration        chan *outboundFrame
	registered       chan struct{}
	subscription     <-chan struct{}
	consecutiveDrops atomic.Uint32
	stats            chan *outboundFrame
	UserID           uint64 // client id

	RoomID string

	Socket       *websocket.Conn // the websocket connection that the server holds for each client
	batch        bool
	pendingBytes atomic.Int64
	Send         chan *outboundFrame // a channel to send messages to the client
	// done is used to signal the goroutines to stop
	done chan struct{}
	// once ensures that the Close operations are only performed once
	once sync.Once
}

func (m *Hub) NewClient(uid uint64, room string, conn *websocket.Conn) *Client {
	return &Client{
		registered: make(chan struct{}),
		stats:      make(chan *outboundFrame, 1),
		batch:      conn != nil && conn.Subprotocol() == protocol.BatchProtocol,
		UserID:     uid,

		RoomID: room,
		Socket: conn,
		Send:   make(chan *outboundFrame, m.sendCapacity),
		done:   make(chan struct{}),
	}
}

// EnableMigration is negotiated before the client joins the room.
func (c *Client) EnableMigration() { c.migration = make(chan *outboundFrame, 1) }

func (c *Client) ReadPump(manager *Hub) {
	defer func() {

		manager.Leave(c)
		c.Socket.Close()
	}()

	c.Socket.SetReadLimit(8192)
	c.Socket.SetReadDeadline(time.Now().Add(PongWait))
	c.Socket.SetPongHandler(func(string) error { return c.Socket.SetReadDeadline(time.Now().Add(PongWait)) })
	for {

		_, messageBytes, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,         // 1001: browser navigation leave/refreshing
				websocket.CloseAbnormalClosure) { //1006:connection closed abnormally
				//log when error does not belong to reasons mentioned above
				manager.log.Error("WebSocket read failed",
					"uid", c.UserID,
					"error", err,
				)
			}
			break // if read error, break the loop
		}

		var packet protocol.WsPacket
		if err := json.Unmarshal(messageBytes, &packet); err != nil {
			manager.log.Error("Invalid WebSocket JSON", "uid", c.UserID)
			continue
		}

		manager.dispatch(c, packet)
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Socket.Close()
	}()

	ping := time.NewTicker(PingPeriod)
	defer ping.Stop()
	for {
		var message *outboundFrame
		var chat bool
		select {
		case <-c.done:
			return
		case <-ping.C:
			if err := c.Socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(WriteWait)); err != nil {
				return
			}
			continue
		case message = <-c.Send:
			chat = true
		case message = <-c.migration:
		case message = <-c.stats:
		}
		c.Socket.SetWriteDeadline(time.Now().Add(WriteWait))
		err := c.Socket.WriteMessage(websocket.TextMessage, message.payload)
		if chat {
			c.pendingBytes.Add(-int64(len(message.payload)))
		}
		if err != nil {
			return
		}
	}
}

// A single stats writer replaces an outdated snapshot without delaying chat delivery.
func (c *Client) setStats(frame *outboundFrame) {
	select {
	case c.stats <- frame:
		return
	default:
	}
	select {
	case <-c.stats:
	default:
	}
	select {
	case c.stats <- frame:
	default:
	}
}

// Close gracefully closes the client connection and signals goroutines to exit.
// It is idempotent thanks to sync.Once.
func (c *Client) Close() {
	c.once.Do(func() {

		close(c.done)

		if c.Socket != nil {
			c.Socket.Close()
		}

	})
}
