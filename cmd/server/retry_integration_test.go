package main

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

// Point both URLs at gateways sharing a dedicated stack. Kafka can be stopped
// before the test and restarted while the test waits for playback.
func TestRetryAcrossGateways(t *testing.T) {
	a, b := os.Getenv("DANMAKU_TEST_GATEWAY_A"), os.Getenv("DANMAKU_TEST_GATEWAY_B")
	if a == "" || b == "" {
		t.Skip("requires two running test gateways")
	}
	path := "/api/rooms/retry/" + rand.Text()[:24]
	response, err := http.Post(a+"/api/session", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cookies := response.Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing guest cookie")
	}
	dial := func(base string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(base, "http", "ws", 1)+path+"/live", http.Header{"Cookie": []string{cookies[0].Name + "=" + cookies[0].Value}})
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		var clock protocol.WsPacket
		if err := conn.ReadJSON(&clock); err != nil || clock.Type != protocol.TypeReady {
			t.Fatalf("clock %v: %v", clock, err)
		}
		return conn
	}
	req := map[string]any{"type": 101, "data": map[string]any{"content": "retry", "video_offset_ms": 42, "client_msg_id": "retry-1"}}
	first := dial(a)
	if err := first.WriteJSON(req); err != nil {
		t.Fatal(err)
	}
	// Observe acceptance independently; the sender never reads its receipt.
	read := func(suffix string) []json.RawMessage {
		t.Helper()
		resp, err := http.Get(b + path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result struct{ Events []json.RawMessage }
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result.Events
	}
	until := time.Now().Add(4 * time.Second)
	for len(read("/recent")) == 0 && time.Now().Before(until) {
		time.Sleep(20 * time.Millisecond)
	}
	first.Close()
	events := read("/recent")
	if len(events) != 1 {
		t.Fatalf("first event count %d", len(events))
	}
	var original struct{ Data protocol.DanmakuMessage }
	if err := json.Unmarshal(events[0], &original); err != nil || original.Data.EventID == "" {
		t.Fatal("missing event id", err)
	}
	// Independent connections submit the same request before reading responses.
	clients := make([]*websocket.Conn, 12)
	for i := range clients {
		clients[i] = dial(b)
		defer clients[i].Close()
	}
	for _, c := range clients {
		if err := c.WriteJSON(req); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range clients {
		for {
			var packet protocol.WsPacket
			if err := c.ReadJSON(&packet); err != nil {
				t.Fatal(err)
			}
			if packet.Type != protocol.TypeReceipt {
				continue
			}
			var receipt struct {
				Status  string
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(packet.Data, &receipt); err != nil || receipt.Status != "accepted" || receipt.EventID != original.Data.EventID {
				t.Fatal(receipt, err)
			}
			break
		}
	}
	if n := len(read("/recent")); n != 1 {
		t.Fatalf("recent events %d", n)
	}
	t.Log("12 retries returned original event; waiting for archive", original.Data.EventID)
	until = time.Now().Add(2 * time.Minute)
	for time.Now().Before(until) {
		rows := read("/playback?from=0&until=100")
		if len(rows) > 0 {
			var row protocol.DanmakuMessage
			if len(rows) != 1 || json.Unmarshal(rows[0], &row) != nil || row.EventID != original.Data.EventID {
				t.Fatal("unexpected archive", rows)
			}
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("archive never recovered")
}
