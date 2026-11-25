// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

var sessionClient = &http.Client{Timeout: 10 * time.Second}

type observer struct {
	windows                  []deliveryWindow
	err                      error
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

func dial(addr, room string, batch bool) (*websocket.Conn, error) {
	dialer := *websocket.DefaultDialer
	if batch {
		dialer.Subprotocols = []string{protocol.BatchProtocol}
	}
	path := "/api/rooms/" + room + "/load/live"
	headers := http.Header{}

	response, err := sessionClient.Post("http://"+addr+"/api/session", "application/json", nil)
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	cookies := response.Cookies()
	if response.StatusCode != 200 || len(cookies) == 0 {
		return nil, fmt.Errorf("guest session unavailable: %s", response.Status)
	}
	headers.Set("Cookie", cookies[0].Name+"="+cookies[0].Value)

	conn, _, err := dialer.Dial("ws://"+addr+path, headers)
	if err != nil {
		return nil, err
	}
	// Keep completed loopback runs from consuming the ephemeral port range in TIME_WAIT.
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		if err := tcp.SetLinger(0); err != nil {
			conn.Close()
			return nil, err
		}
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ready protocol.WsPacket
	if err := conn.ReadJSON(&ready); err != nil || ready.Type != protocol.TypeReady {
		conn.Close()
		return nil, fmt.Errorf("subscription failed: %v", err)
	}
	conn.SetReadDeadline(time.Time{})

	return conn, nil
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

func connect(addr, room string, expected, windowSize int, batch, sample bool, readers *sync.WaitGroup) (*observer, error) {
	conn, err := dial(addr, room, batch)
	if err != nil {
		return nil, err
	}
	expectedRoom := room + "/load"
	o := &observer{conn: conn, seen: make([]uint64, (expected+63)/64), cursor: "0-0", firstDuplicate: -1}
	if windowSize > 0 {
		o.windows = make([]deliveryWindow, (expected+windowSize-1)/windowSize)
	}
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
				o.err = err
				return
			}
			n := 0
			for _, p := range packets {
				if p.Type != protocol.TypeDanmaku {
					continue
				}
				if p.RoomID != expectedRoom {
					o.err = fmt.Errorf("unexpected room %q for viewer in %q", p.RoomID, expectedRoom)
					return
				}
				n++
				var d struct{ Content string }
				if err := json.Unmarshal(p.Data, &d); err != nil {
					o.err = err
					return
				}
				prefix, stamp, ok := strings.Cut(d.Content, "/")
				seq, err := strconv.Atoi(prefix)
				if !ok || err != nil || seq < 0 || seq >= expected {
					o.err = fmt.Errorf("unexpected sequence %q", d.Content)
					return
				}
				var w *deliveryWindow
				if windowSize > 0 {
					w = &o.windows[seq/windowSize]
				}
				previousUnique := o.unique
				o.record(seq)
				if w != nil {
					if o.unique > previousUnique {
						w.unique++
					} else {
						w.duplicates++
					}
				}

				if p.Previous != o.cursor || p.Position == "" {
					o.gaps++
					if w != nil {
						w.gaps++
					}
				}
				o.cursor = p.Position

				if sample {
					sent, err := strconv.ParseInt(strings.SplitN(stamp, "/", 2)[0], 10, 64)
					if err != nil {
						o.err = err
						return
					}
					o.latency = append(o.latency, now-sent)
					if w != nil {
						w.latency = append(w.latency, now-sent)
					}
				}
			}
			if n > 0 {
				o.bytes += int64(len(data))
				o.frames++
				o.received.Add(int64(n))
			}
		}
	}()
	return o, nil
}

func send(writers []*observer, n, rate int, padding string) error {
	start := time.Now()
	for i := 0; i < n; i++ {
		time.Sleep(time.Until(start.Add(time.Second * time.Duration(i) / time.Duration(rate))))
		data := fmt.Appendf(nil, `{"type":%d,"data":{"content":"%d/%d%s"}}`, protocol.TypeDanmaku, i, time.Now().UnixMicro(), padding)
		if err := writers[i%len(writers)].conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
	}
	return nil
}

func percentile(samples []int64, percent int) int64 {
	if len(samples) == 0 {
		return -1
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[(percent*len(samples)-1)/100]
}
