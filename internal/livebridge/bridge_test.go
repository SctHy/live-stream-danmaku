package livebridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for upstream")
	}
	var zero T
	return zero
}

func TestSharedConnectionReconnectAndRelease(t *testing.T) {
	var connections, credentials atomic.Int32
	var reject atomic.Bool
	reject.Store(true)
	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	host := strings.Split(strings.TrimPrefix(srv.URL, "https://"), ":")
	mux.HandleFunc("/xlive/web-room/v2/index/getRoomPlayInfo", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("room_id") != "6" {
			t.Error("wrong short id")
		}
		fmt.Fprint(w, `{"data":{"room_id":7734200}}`)
	})
	mux.HandleFunc("/x/frontend/finger/spi", func(w http.ResponseWriter, r *http.Request) {
		credentials.Add(1)
		fmt.Fprint(w, `{"data":{"b_3":"visitor"}}`)
	})
	mux.HandleFunc("/x/web-interface/nav", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "buvid3=visitor" {
			t.Error("missing cookie")
		}
		fmt.Fprint(w, `{"code":-101,"data":{"wbi_img":{"img_url":"https://i/7cd084941338484aae1ad9425b84077c.png","sub_url":"https://i/4932caff0ff746eab6f01bf08b70ac45.png"}}}`)
	})
	mux.HandleFunc("/xlive/web-room/v1/index/getDanmuInfo", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		q.Del("w_rid")
		if q.Get("id") != "7734200" || q.Get("type") != "0" || q.Get("web_location") != "444.8" || r.URL.RawQuery != signed(q, "ea1db124af3c7062474693fa704f4ff8") || r.Header.Get("Referer") != origin+"/" || r.Header.Get("Cookie") != "buvid3=visitor" {
			t.Error("invalid signed request")
		}
		if reject.Swap(false) {
			fmt.Fprint(w, `{"code":-352}`)
			return
		}
		fmt.Fprintf(w, `{"data":{"token":"token","host_list":[{"host":"%s","wss_port":%s}]}}`, host[0], host[1])
	})
	compressed := zipped(t, packet(0, 5, []byte(example)))
	drop := make(chan struct{})
	released := make(chan struct{}, 4)
	mux.HandleFunc("/sub", func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == origin }}
		conn, err := u.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		n := connections.Add(1)
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		_, auth, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		err = decode(auth, func(op uint32, b []byte) error {
			var a struct {
				RoomID, Protover, Type int
				Key, Buvid, Platform   string
			}
			if json.Unmarshal(b, &a) != nil || op != 7 || a.RoomID != 7734200 || a.Protover != 2 || a.Key != "token" || a.Buvid != "visitor" || a.Platform != "web" || a.Type != 2 {
				t.Errorf("bad auth %s", b)
			}
			return nil
		})
		if err != nil {
			t.Error(err)
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, packet(1, 8, []byte(`{"code":0}`))); err != nil {
			t.Error(err)
			return
		}
		typ, heartbeat, err := conn.ReadMessage()
		if err != nil || typ != websocket.BinaryMessage || !bytes.Equal(heartbeat, packet(1, 2, nil)) {
			t.Errorf("initial heartbeat: type=%d data=%x err=%v", typ, heartbeat, err)
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, compressed); err != nil {
			t.Error(err)
			return
		}
		if n == 1 {
			select {
			case <-drop:
			case <-time.After(6 * time.Second):
			}
			return
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			released <- struct{}{}
		}
	})
	dialer := websocket.Dialer{TLSClientConfig: srv.Client().Transport.(*http.Transport).TLSClientConfig}
	m := New(Config{LiveBase: srv.URL, APIBase: srv.URL, HTTPClient: srv.Client(), Dialer: &dialer})
	defer m.Close()
	a, b := make(chan protocol.LiveDanmaku, 4), make(chan protocol.LiveDanmaku, 4)
	m.Subscribe("6/a", 6, func(msg protocol.LiveDanmaku) { a <- msg })
	m.Subscribe("6/b", 6, func(msg protocol.LiveDanmaku) { b <- msg })
	first, other := receive(t, a), receive(t, b)
	if first.EventID != other.EventID || first.Content != "hello" || connections.Load() != 1 {
		t.Fatal(first, other, connections.Load())
	}
	m.Unsubscribe("6/a")
	close(drop)
	next := receive(t, b)
	if next.EventID == first.EventID || connections.Load() != 2 || credentials.Load() != 2 {
		t.Fatal(next, connections.Load(), credentials.Load())
	}
	select {
	case <-a:
		t.Fatal("delivered to removed room")
	default:
	}
	m.Unsubscribe("6/b")
	receive(t, released)
}
