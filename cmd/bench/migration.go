// Exercises a rolling release on two disposable local gateways sharing Redis,
// PostgreSQL and Kafka. Run one strategy per invocation to isolate its results.
package main

import (
	"encoding/json"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

type lab struct {
	http                    *http.Client
	dialer                  websocket.Dialer
	a, b, path              string
	cookie                  http.Header
	count                   int
	start                   atomic.Int64
	lastMoveMS              int64
	wg                      sync.WaitGroup
	moved, invited, ready   atomic.Int64
	mu                      sync.Mutex
	errors                  []string
	attempts                map[int]int
	stableLatency, recovery []int64
}

func (l *lab) connect(addr string) (*websocket.Conn, error) {
	c, _, err := l.dialer.Dial("ws://"+addr+l.path+"/live?migrate=1", l.cookie)
	if err != nil {
		return nil, err
	}
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	var p protocol.WsPacket
	if err = c.ReadJSON(&p); err != nil || p.Type != protocol.TypeReady {
		c.Close()
		return nil, fmt.Errorf("welcome %v: %v", p.Type, err)
	}
	c.SetReadDeadline(time.Time{})
	return c, nil
}

type viewer struct {
	cursor string
	next   int
}

func (v *viewer) apply(p protocol.WsPacket) (time.Duration, error) {
	if p.Previous != v.cursor && !(v.cursor == "" && p.Previous == "0-0") {
		return 0, fmt.Errorf("cursor gap at %d", v.next)
	}
	var d protocol.DanmakuMessage
	if err := json.Unmarshal(p.Data, &d); err != nil {
		return 0, err
	}
	if d.Content != strconv.Itoa(v.next) {
		return 0, fmt.Errorf("want %d got %s", v.next, d.Content)
	}
	v.next++
	v.cursor = p.Position
	return time.Since(d.SendTime), nil
}
func (l *lab) watch(c *websocket.Conn, onA bool) error {
	defer func() { c.Close() }()
	var v viewer
	// The new subscription may buffer events also present in the snapshot.
	var overlap map[string]bool
	for v.next < l.count {
		c.SetReadDeadline(time.Now().Add(45 * time.Second))
		_, frame, err := c.ReadMessage()
		migrate := err != nil
		if err == nil {
			packets, e := protocol.DecodePackets(frame)
			if e != nil {
				return e
			}
			for _, p := range packets {
				if p.Type == protocol.TypeMigrate {
					migrate = true
					l.invited.Add(1)
					continue
				}
				if p.Type != protocol.TypeDanmaku {
					continue
				}
				if overlap[p.Position] {
					delete(overlap, p.Position)
					continue
				}
				latency, e := v.apply(p)
				if e != nil {
					return e
				}
				if !onA && v.next > 2 {
					l.mu.Lock()
					l.stableLatency = append(l.stableLatency, latency.Milliseconds())
					l.mu.Unlock()
				}
			}
		}
		if !migrate {
			continue
		}
		if !onA || overlap != nil {
			return fmt.Errorf("unexpected disconnect: %v", err)
		}
		began := time.Now()
		replacement, events, e := l.replace(v.cursor, err != nil)
		if e != nil {
			return e
		}
		overlap = map[string]bool{}
		for _, p := range events {
			if _, e = v.apply(p); e != nil {
				replacement.Close()
				return e
			}
			overlap[p.Position] = true
		}
		c.Close()
		c = replacement
		l.moved.Add(1)
		l.mu.Lock()
		l.lastMoveMS = max(l.lastMoveMS, time.Since(time.Unix(0, l.start.Load())).Milliseconds())
		l.recovery = append(l.recovery, time.Since(began).Milliseconds())
		l.mu.Unlock()
	}
	return nil
}

// Recheck the guest identity and fetch the missing suffix through the same
// HTTP endpoints as the browser. A failed attempt keeps its original cursor.
func (l *lab) replace(cursor string, backoff bool) (*websocket.Conn, []protocol.WsPacket, error) {
	began := time.Now()
	for attempt := 0; time.Since(began) < 30*time.Second; attempt++ {
		if backoff || attempt > 0 {
			delay := min(8*time.Second, 300*time.Millisecond<<min(attempt, 5))
			time.Sleep(delay/2 + mathrand.N(delay))
		}
		bucket := int(time.Since(time.Unix(0, l.start.Load())) / time.Second)
		l.mu.Lock()
		l.attempts[bucket]++
		l.mu.Unlock()
		req, err := http.NewRequest("POST", "http://"+l.b+"/api/session", nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header = l.cookie.Clone()
		identity, err := l.http.Do(req)
		if err != nil {
			continue
		}
		var session struct {
			UserID uint64 `json:"user_id"`
		}
		err = json.NewDecoder(identity.Body).Decode(&session)
		identity.Body.Close()
		if err != nil || session.UserID == 0 {
			continue
		}
		c, err := l.connect(l.b)
		if err != nil {
			continue
		}
		resp, err := l.http.Get("http://" + l.b + l.path + "/recent?after=" + cursor)
		if err != nil {
			c.Close()
			continue
		}
		var snapshot struct {
			Reset  bool
			Events []protocol.WsPacket
		}
		err = json.NewDecoder(resp.Body).Decode(&snapshot)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			c.Close()
			continue
		}
		if snapshot.Reset {
			c.Close()
			return nil, nil, fmt.Errorf("recovery window exceeded")
		}
		return c, snapshot.Events, nil
	}
	return nil, nil, fmt.Errorf("handoff attempts timed out")
}
