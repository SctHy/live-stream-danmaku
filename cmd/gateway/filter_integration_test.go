package main

import (
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

// Both gateways use a dictionary containing "bad" and the same archive stack.
func TestFilterBeforeJournalAndArchive(t *testing.T) {
	reject, mask := os.Getenv("DANMAKU_TEST_FILTER_REJECT"), os.Getenv("DANMAKU_TEST_FILTER_MASK")
	dsn := os.Getenv("DANMAKU_TEST_DATABASE_URL")
	if reject == "" || mask == "" || dsn == "" {
		t.Skip("requires reject/mask gateways, archiver and test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	room := "filter/" + rand.Text()[:20]
	path := "/api/rooms/" + room
	send := func(base, content, status string) {
		t.Helper()
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(base+"/api/session", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(resp.Cookies()) == 0 {
			t.Fatal("missing guest cookie")
		}
		header := http.Header{}
		cookie := resp.Cookies()[0]
		header.Set("Cookie", cookie.Name+"="+cookie.Value)
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, strings.Replace(base, "http", "ws", 1)+path+"/live", header)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var packet protocol.WsPacket
		if err := conn.ReadJSON(&packet); err != nil || packet.Type != protocol.TypeReady {
			t.Fatalf("ready: %v %v", packet, err)
		}
		data, _ := json.Marshal(protocol.DanmakuMessage{Content: content, ClientMsgID: rand.Text()})
		if err := conn.WriteJSON(protocol.WsPacket{Type: protocol.TypeDanmaku, Data: data}); err != nil {
			t.Fatal(err)
		}
		for {
			if err := conn.ReadJSON(&packet); err != nil {
				t.Fatal(err)
			}
			if packet.Type == protocol.TypeReceipt {
				var receipt protocol.Receipt
				if json.Unmarshal(packet.Data, &receipt) != nil || receipt.Status != status {
					t.Fatalf("want %s, got %s", status, packet.Data)
				}
				return
			}
		}
	}
	send(reject, "甲Ｂ aＤ乙", "filtered")
	send(mask, "甲Ｂ aＤ乙", "accepted")
	// A following record on the same Kafka key is a barrier for the database check.
	send(reject, "结束", "accepted")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(mask + path + "/recent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var recent struct{ Events []protocol.WsPacket }
	if err := json.NewDecoder(resp.Body).Decode(&recent); err != nil {
		t.Fatal(err)
	}
	if len(recent.Events) != 2 {
		t.Fatalf("unexpected stream: %+v", recent)
	}
	for i, want := range []string{"甲****乙", "结束"} {
		var d protocol.DanmakuMessage
		if json.Unmarshal(recent.Events[i].Data, &d) != nil || d.Content != want {
			t.Fatalf("stream entry %d: %s", i, recent.Events[i].Data)
		}
	}
	for {
		var total, masked, barrier int
		err := db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE content='甲****乙'),count(*) FILTER (WHERE content='结束') FROM danmaku_messages WHERE room_id=$1`, room).Scan(&total, &masked, &barrier)
		if err != nil {
			t.Fatal(err)
		}
		if barrier == 1 && masked >= 1 {
			if total != 2 || masked != 1 {
				t.Fatalf("unexpected archive: total=%d masked=%d", total, masked)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("missing masked message or archive barrier")
		case <-time.After(100 * time.Millisecond):
		}
	}
}
