package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

func TestMigrationWaitsForOldConnectionOrDeadline(t *testing.T) {
	first := &Client{migration: make(chan struct{}, 1), done: make(chan struct{})}
	second := &Client{migration: make(chan struct{}, 1), done: make(chan struct{})}
	m := Hub{rooms: map[string]map[*Client]struct{}{"room": {first: {}, second: {}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { m.Migrate(ctx, 2, 1, time.Millisecond); close(done) }()

	select {
	case <-first.migration:
	case <-second.migration:
		first, second = second, first
	case <-ctx.Done():
		t.Fatal("missing invitation")
	}
	if !m.Retiring() {
		t.Fatal("gateway still ready")
	}
	late := &Client{done: make(chan struct{})}
	m.handleRegister(late)
	select {
	case <-late.done:
	default:
		t.Fatal("late registration admitted")
	}
	select {
	case <-second.migration:
		t.Fatal("in-flight limit exceeded")
	case <-time.After(10 * time.Millisecond):
	}
	first.Close()
	select {
	case <-second.migration:
	case <-ctx.Done():
		t.Fatal("slot was not reused")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deadline ignored")
	}
}

func TestWritePumpSendsMigrationNotice(t *testing.T) {
	clients := make(chan *Client, 1)
	pumpDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var upgrader websocket.Upgrader
		socket, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		client := (&Hub{}).NewClient(1, "room", socket)
		client.EnableMigration()
		client.migration <- struct{}{}
		clients <- client
		client.WritePump()
		close(pumpDone)
	}))
	defer server.Close()
	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	client := <-clients
	defer func() {
		client.Close()
		select {
		case <-pumpDone:
		case <-time.After(3 * time.Second):
			t.Error("write pump did not exit")
		}
	}()
	if err := socket.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var packet protocol.WsPacket
	if err := socket.ReadJSON(&packet); err != nil {
		t.Fatal(err)
	}
	if packet.Type != protocol.TypeMigrate || string(packet.Data) != "{}" {
		t.Fatalf("unexpected migration notice: %+v", packet)
	}
}
