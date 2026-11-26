package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

// The real hub waits for a Redis subscription acknowledgement before sending
// the WebSocket ready frame. Model just that exchange and the HTTP commands.
func testRedis(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	workers.Go(func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			workers.Go(func() { serveTestRedis(c) })
		}
	})
	t.Cleanup(func() { l.Close(); workers.Wait() })
	return l.Addr().String()
}

func serveTestRedis(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		var count int
		if _, err := fmt.Fscanf(r, "*%d\r\n", &count); err != nil {
			return
		}
		args := make([]string, count)
		for i := range args {
			var n int
			if _, err := fmt.Fscanf(r, "$%d\r\n", &n); err != nil {
				return
			}
			data := make([]byte, n+2)
			if _, err := io.ReadFull(r, data); err != nil {
				return
			}
			args[i] = string(data[:n])
		}
		var reply strings.Builder
		switch cmd := strings.ToLower(args[0]); cmd {
		case "subscribe", "unsubscribe":
			for i, ch := range args[1:] {
				fmt.Fprintf(&reply, "*3\r\n+%s\r\n$%d\r\n%s\r\n:%d\r\n", cmd, len(ch), ch, i+1)
			}
		case "evalsha":
			// The mute script updates the expiring key and the room index together.
			reply.WriteString(":1\r\n")
		case "xrange":
			reply.WriteString("*0\r\n")
		case "set", "client":
			reply.WriteString("+OK\r\n")
		default:
			reply.WriteString("-ERR unsupported command\r\n")
		}
		if _, err := io.WriteString(c, reply.String()); err != nil {
			return
		}
	}
}

func TestRealtimeDuringHistoryOutage(t *testing.T) {
	sessions := pubsub.Open(testRedis(t))
	defer sessions.Close()
	producer, err := mq.NewProducer(mq.Brokers(""), slog.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m := hub.New(hub.Config{Producer: producer, Store: sessions, SessionClock: sessions, PubSubShards: 1, RoomShards: 1})
	go m.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := m.Drain(ctx); err != nil {
			t.Error(err)
		}
	}()
	mux := http.NewServeMux()
	httpapi.NewChatHandler(&gatewayHistory{}, m, sessions, true).Routes(mux)
	checkDegradedRoutes(t, mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/rooms/r/s/live?uid=2", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var packet protocol.WsPacket
	if err := conn.ReadJSON(&packet); err != nil || packet.Type != protocol.TypeReady {
		t.Fatalf("ready packet: %v, error: %v", packet, err)
	}
}

func checkDegradedRoutes(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	t.Setenv("DANMAKU_MOD_ROOM", "r")
	t.Setenv("DANMAKU_MOD_TOKEN", "test")
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/wallet", "", 503},
		{"POST", "/api/wallet/recharges", "{}", 503},
		{"POST", "/api/wallet/redeem", "{}", 503},
		{"POST", "/api/rooms/r/s/gifts", "{}", 503},
		{"GET", "/api/rooms/r/replays", "", 503},
		{"GET", "/api/rooms/r/stream-key", "", 503},
		{"GET", "/api/rooms/r/s/history", "", 503},
		{"POST", "/api/rooms/r/s/moderate", `{"action":"withdraw","event_id":"12345678901234567890123456"}`, 503},
		{"GET", "/api/rooms/r/s/recent", "", 200},
		{"POST", "/api/session", "", 200},
		{"POST", "/api/rooms/r/s/moderate", `{"action":"mute","user_id":1,"seconds":1}`, 204},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer test")
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
}
