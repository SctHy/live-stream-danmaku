package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

func TestFanoutJSONRound(t *testing.T) {
	// One viewer per room lets the mock echo each publication with a real cursor chain.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "guest", Value: "test"})
	})
	mux.HandleFunc("/api/rooms/", func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{}
		c, e := u.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		if e = c.WriteJSON(protocol.WsPacket{Type: protocol.TypeReady}); e != nil {
			return
		}
		prev := "0-0"
		for n := 1; ; n++ {
			var packet protocol.WsPacket
			if e := c.ReadJSON(&packet); e != nil {
				return
			}
			packet.RoomID = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/rooms/"), "/live")
			packet.Previous = prev
			prev = strconv.Itoa(n) + "-0"
			packet.Position = prev
			if e = c.WriteJSON(packet); e != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	var out bytes.Buffer
	r, j, e := runFanout([]string{"-addr", strings.TrimPrefix(srv.URL, "http://"), "-json", "-viewers", "1", "-rate", "1", "-cold-rate", "1", "-seconds", "1"}, &out)
	if e != nil || !j || out.Len() != 0 || !r.Passed || r.Metrics["hot_samples"] != 1 || r.Metrics["cold_samples"] != 1 {
		t.Fatal(e, r, out.String())
	}
	if _, e = json.Marshal(r); e != nil {
		t.Fatal(e)
	}
}
func TestPeakSlidingWindow(t *testing.T) {
	ms := int64(time.Millisecond)
	if q := peakQPS([]int64{100 * ms, 190 * ms, 99 * ms, 199 * ms}); q != 30 {
		t.Fatal(q)
	}
}
func TestInvalidInputs(t *testing.T) {
	if _, _, e := runFanout([]string{"-sample-viewers", "0"}, io.Discard); e == nil {
		t.Fatal("invalid sampling")
	}
	if _, e := runMigration([]string{"-viewers", "3"}, false); e == nil {
		t.Fatal("invalid migration")
	}
	for _, args := range [][]string{{"-profile", "unknown"}, {"-repeats", "0"}} {
		if e := evaluate(args); e == nil {
			t.Fatal(args)
		}
	}
}

func TestMigrationRejectsInvalidSequence(t *testing.T) {
	valid := protocol.WsPacket{Position: "1-0", Previous: "0-0", Data: json.RawMessage(`{"content":"0"}`)}
	var v viewer
	if err := v.apply(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []protocol.WsPacket{valid, {Position: "2-0", Previous: "1-0", Data: json.RawMessage(`{"content":"2"}`)}, {Position: "2-0", Previous: "0-0", Data: json.RawMessage(`{"content":"1"}`)}} {
		if err := v.apply(bad); err == nil || v.next != 1 || v.cursor != "1-0" {
			t.Fatal("accepted invalid delivery", bad, v)
		}
	}
}
