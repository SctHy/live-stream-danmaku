package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

// Both gateways and the archiver must use the same dedicated stack.
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
	req := map[string]any{"type": protocol.TypeDanmaku, "data": map[string]any{"content": "retry", "client_msg_id": "retry-1"}}
	observer := dial(b)
	defer observer.Close()
	first := dial(a)
	if err := first.WriteJSON(req); err != nil {
		t.Fatal(err)
	}
	var live protocol.WsPacket
	for live.Type != protocol.TypeDanmaku {
		if err := observer.ReadJSON(&live); err != nil {
			t.Fatal(err)
		}
	}
	if live.Position == "" || live.Previous != "0-0" {
		t.Fatalf("bad cross-gateway cursor %+v", live)
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
	if len(read("/recent?after="+live.Position)) != 0 {
		t.Fatal("recovery repeated acknowledged event")
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
	if dsn := os.Getenv("DANMAKU_TEST_DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		db, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		for {
			var count int
			if err := db.QueryRow(ctx, "SELECT count(*) FROM danmaku_messages WHERE event_id=$1", original.Data.EventID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == 1 {
				break
			}
			if count > 1 {
				t.Fatalf("duplicate archive: %d", count)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
	if token := os.Getenv("DANMAKU_MOD_TOKEN"); token != "" && os.Getenv("DANMAKU_MOD_ROOM") == "retry" {
		moderate := func(body any) {
			data, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", a+path+"/moderate", bytes.NewReader(data))
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 204 {
				t.Fatal(resp.Status)
			}
		}
		moderate(map[string]any{"action": "mute", "user_id": original.Data.UserID, "seconds": 10})
		muted := dial(b)
		defer muted.Close()
		req["data"] = map[string]any{"content": "blocked", "client_msg_id": "muted-test"}
		if err := muted.WriteJSON(req); err != nil {
			t.Fatal(err)
		}
		for {
			var p protocol.WsPacket
			if err := muted.ReadJSON(&p); err != nil {
				t.Fatal(err)
			}
			if p.Type == protocol.TypeReceipt {
				var receipt protocol.Receipt
				if json.Unmarshal(p.Data, &receipt) != nil || receipt.Status != "muted" {
					t.Fatal(string(p.Data))
				}
				break
			}
		}
		moderate(map[string]any{"action": "withdraw", "event_id": original.Data.EventID})
		suffix := read("/recent?after=" + live.Position)
		var withdrawal protocol.WsPacket
		if len(suffix) != 1 || json.Unmarshal(suffix[0], &withdrawal) != nil || withdrawal.Type != protocol.TypeWithdraw || withdrawal.Previous != live.Position {
			t.Fatal("withdrawal missing in recovery")
		}
	}
}
