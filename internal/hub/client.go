package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/metrics"
	"github.com/scthy/maku/internal/protocol"
)

const (
	SendChanSize = 512
	PongWait     = 60 * time.Second
	PingPeriod   = 30 * time.Second
	WriteWait    = 5 * time.Second
)

type Client struct {
	metrics          *metrics.GatewayMetrics
	migration        chan struct{}
	registered       chan struct{}
	subscription     <-chan struct{}
	consecutiveDrops atomic.Uint32
	stats            chan *outboundFrame
	UserID           uint64 // client id

	Interest      func(weight int)
	LoginNickname func(context.Context) (string, error)
	nickname      string
	RoomID        string

	Socket       *websocket.Conn // the websocket connection that the server holds for each client
	batch        bool
	pendingBytes atomic.Int64
	send         chan *outboundFrame // a channel to send messages to the client
	// done is used to signal the goroutines to stop
	done      chan struct{}
	closeOnce sync.Once
}

func (m *Hub) NewClient(uid uint64, room string, conn *websocket.Conn) *Client {
	return &Client{
		metrics: m.metrics, registered: make(chan struct{}),
		stats:  make(chan *outboundFrame, 1),
		batch:  conn != nil && conn.Subprotocol() == protocol.BatchProtocol,
		UserID: uid,

		RoomID: room,
		Socket: conn,
		send:   make(chan *outboundFrame, m.sendCapacity),
		done:   make(chan struct{}),
	}
}

// EnableMigration is negotiated before the client joins the room.
func (c *Client) EnableMigration() { c.migration = make(chan struct{}, 1) }

func (c *Client) ReadPump(manager *Hub) {
	c.Socket.SetReadLimit(8192)
	if err := c.Socket.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return
	}
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
			manager.metrics.Reject("invalid")
			manager.log.Error("Invalid WebSocket JSON", "uid", c.UserID)
			continue
		}

		manager.dispatch(c, packet)
	}
}

func (c *Client) WritePump() {
	defer c.Close()

	ping := time.NewTicker(PingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-c.done:
			return
		default:
		}
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
		case message = <-c.send:
			chat = true
		case <-c.migration:
			if err := c.Socket.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return
			}
			if err := c.Socket.WriteJSON(protocol.WsPacket{Type: protocol.TypeMigrate, Data: json.RawMessage("{}")}); err != nil {
				return
			}
			continue
		case message = <-c.stats:
		}
		if err := c.Socket.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
			return
		}
		err := c.Socket.WriteMessage(websocket.TextMessage, message.payload)
		if chat {
			c.pendingBytes.Add(-int64(len(message.payload)))
		}
		if err != nil {
			return
		}
		if message.fanout {
			c.metrics.Fanout(message.count)
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
func (c *Client) Close()     { c.close(false) }
func (c *Client) closeSlow() { c.close(true) }
func (c *Client) close(slow bool) {
	c.closeOnce.Do(func() {
		if slow {
			c.metrics.Kick()
		}
		close(c.done)
		if c.Socket != nil {
			if err := c.Socket.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				slog.Warn("Close WebSocket", "error", err)
			}
		}
	})
}
