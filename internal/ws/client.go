package ws

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/charlesAcmen/livestream-danmaku/internal/model" //database structures
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	SendChanSize = 512
)

type Client struct {
	Session          bool
	migration        chan struct{}
	registered       chan struct{}
	subscription     <-chan struct{}
	consecutiveDrops atomic.Uint32
	stats            chan *outboundFrame
	UserID           uint64 // client id
	Username         string
	RoomID           string

	Socket       *websocket.Conn // the websocket connection that the server holds for each client
	batch        bool
	pendingBytes atomic.Int64
	Send         chan *outboundFrame // a channel to send messages to the client
	// done is used to signal the goroutines to stop
	done chan struct{}
	// once ensures that the Close operations are only performed once
	once sync.Once
}

func (m *Manager) NewClient(uid uint64, name, room string, conn *websocket.Conn) *Client {
	return &Client{
		registered: make(chan struct{}),
		stats:      make(chan *outboundFrame, 1),
		batch:      conn != nil && conn.Subprotocol() == model.BatchProtocol,
		UserID:     uid,
		Username:   name,
		RoomID:     room,
		Socket:     conn,
		Send:       make(chan *outboundFrame, m.sendCapacity),
		done:       make(chan struct{}),
	}
}

// EnableMigration is negotiated before the client joins the room.
func (c *Client) EnableMigration() { c.migration = make(chan struct{}, 1) }

// ReadPump: clients read from websocket to receive message, then send to manager to broadcast
func (c *Client) ReadPump(manager *Manager) {
	defer func() {
		// if read failed (client disconnected), unregister the client
		manager.Leave(c)
		c.Socket.Close()
	}()

	c.Socket.SetReadLimit(8192)
	for {
		// 1. read message from WebSocket
		_, messageBytes, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,         // 1001: browser navigation leave/refreshing
				websocket.CloseAbnormalClosure) { //1006:connection closed abnormally
				//log when error does not belong to reasons mentioned above
				logger.Log.Error("[CLIENT] Read error",
					zap.Uint64("uid", c.UserID),
					zap.Error(err),
				)
			}
			break // if read error, break the loop
		}

		// 2. Parse the Envelope (WsPacket)
		var packet model.WsPacket
		if err := json.Unmarshal(messageBytes, &packet); err != nil {
			logger.Log.Error("[CLIENT]Invalid JSON format", zap.Uint64("uid", c.UserID))
			continue
		}

		// 3. Dispatch to Handler
		// We pass 'manager' because handlers need to Broadcast or AddLike.
		manager.dispatch(c, packet)
	}
}

// WritePump: listen from Send channel, once there is a message, write to WebSocket
// i.e pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	defer func() {
		c.Socket.Close()
	}()

	for {
		var message *outboundFrame
		var chat bool
		select {
		case <-c.done:
			return
		case message = <-c.Send:
			chat = true
		case <-c.migration:
			message = migrationNotice
		case message = <-c.stats:
		}
		c.Socket.SetWriteDeadline(time.Now().Add(5 * time.Second))
		var err error
		if message.prepared != nil {
			err = c.Socket.WritePreparedMessage(message.prepared)
		} else {
			err = c.Socket.WriteMessage(websocket.TextMessage, message.payload)
		}
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
		// 1. Close the done channel to signal WritePump and other observers
		close(c.done)
		// 2. Close the physical connection
		if c.Socket != nil {
			c.Socket.Close()
		}
		// Note: We DO NOT close c.Send here to prevent panic in workers
		// logger.Log.Info("[CLIENT] Connection closed safely", zap.Uint64("uid", c.UserID))
	})
}
