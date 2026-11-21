// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

type observer struct {
	conn                     *websocket.Conn
	received                 atomic.Int64
	frames                   int
	latency                  []int64
	seen                     []uint64
	unique, duplicates, gaps int
	firstDuplicate           int
	cursor                   string
	bytes                    int64
}

func dial(addr, room string, batch bool) *websocket.Conn {
	dialer := *websocket.DefaultDialer
	if batch {
		dialer.Subprotocols = []string{protocol.BatchProtocol}
	}
	path := "/api/rooms/" + room + "/load/live"
	headers := http.Header{}

	response, err := http.Post("http://"+addr+"/api/session", "application/json", nil)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	cookies := response.Cookies()
	if response.StatusCode != 200 || len(cookies) == 0 {
		log.Fatal("guest session unavailable")
	}
	headers.Set("Cookie", cookies[0].Name+"="+cookies[0].Value)

	conn, _, err := dialer.Dial("ws://"+addr+path, headers)
	if err != nil {
		log.Fatal(err)
	}
	// Keep completed loopback runs from consuming the ephemeral port range in TIME_WAIT.
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		if err := tcp.SetLinger(0); err != nil {
			log.Fatal(err)
		}
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ready protocol.WsPacket
	if err := conn.ReadJSON(&ready); err != nil || ready.Type != protocol.TypeReady {
		log.Fatalf("subscription failed: %v", err)
	}
	conn.SetReadDeadline(time.Time{})

	return conn
}

// Each room has a unique run name. Sequence numbers are local to that room.
func (o *observer) record(seq int) {
	word, mask := seq/64, uint64(1)<<(seq%64)
	if o.seen[word]&mask != 0 {
		if o.duplicates == 0 {
			o.firstDuplicate = seq
		}
		o.duplicates++
		return
	}
	o.seen[word] |= mask
	o.unique++
}

func connect(addr, room string, expected int, batch, sample bool, readers *sync.WaitGroup) *observer {
	conn := dial(addr, room, batch)
	expectedRoom := room + "/load"
	o := &observer{conn: conn, seen: make([]uint64, (expected+63)/64), cursor: "0-0", firstDuplicate: -1}
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			now := time.Now().UnixMicro()
			packets, err := protocol.DecodePackets(data)
			if err != nil {
				log.Fatal(err)
			}
			n := 0
			for _, p := range packets {
				if p.Type != protocol.TypeDanmaku {
					continue
				}
				if p.RoomID != expectedRoom {
					log.Fatalf("unexpected room %q for viewer in %q", p.RoomID, expectedRoom)
				}
				n++
				var d struct{ Content string }
				if err := json.Unmarshal(p.Data, &d); err != nil {
					log.Fatal(err)
				}
				prefix, stamp, ok := strings.Cut(d.Content, "/")
				seq, err := strconv.Atoi(prefix)
				if !ok || err != nil || seq < 0 || seq >= expected {
					log.Fatalf("unexpected sequence %q", d.Content)
				}
				o.record(seq)

				if p.Previous != o.cursor || p.Position == "" {
					o.gaps++
				}
				o.cursor = p.Position

				if sample {
					sent, err := strconv.ParseInt(stamp, 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					o.latency = append(o.latency, now-sent)
				}
			}
			if n > 0 {
				o.bytes += int64(len(data))
				o.frames++
				o.received.Add(int64(n))
			}
		}
	}()
	return o
}

func send(writers []*observer, n, rate int) {
	start := time.Now()
	for i := 0; i < n; i++ {
		time.Sleep(time.Until(start.Add(time.Second * time.Duration(i) / time.Duration(rate))))
		data := fmt.Appendf(nil, `{"type":%d,"data":{"content":"%d/%d"}}`, protocol.TypeDanmaku, i, time.Now().UnixMicro())
		if err := writers[i%len(writers)].conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Fatal(err)
		}
	}
}

func percentile(samples []int64, percent int) int64 {
	if len(samples) == 0 {
		return -1
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[(percent*len(samples)-1)/100]
}
