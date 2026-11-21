// Exercises a rolling release on two disposable local gateways sharing Redis,
// PostgreSQL and Kafka. Run one strategy per invocation to isolate its results.
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	mathrand "math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

var httpClient = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 128, MaxConnsPerHost: 512}}
var dialer = websocket.Dialer{Subprotocols: []string{protocol.BatchProtocol}, HandshakeTimeout: 10 * time.Second, NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
	c, err := new(net.Dialer).DialContext(ctx, network, addr)
	if err == nil {
		c.(*net.TCPConn).SetLinger(0)
	}
	return c, err
}}

type lab struct {
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
	c, _, err := dialer.Dial("ws://"+addr+l.path+"/live?migrate=1", l.cookie)
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
				if p.Type != 101 {
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
		identity, err := httpClient.Do(req)
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
		resp, err := httpClient.Get("http://" + l.b + l.path + "/recent?after=" + cursor)
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

func launch(bin, addr string, flags ...string) (*os.Process, func()) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	listener.Close()
	_, port, _ := net.SplitHostPort(addr)
	args := append([]string{"-port", port}, flags...)
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	cleanup := func() { cmd.Process.Kill(); <-done }
	for range 150 {
		select {
		case <-done:
			panic("test gateway exited")
		default:
		}
		resp, err := httpClient.Get("http://" + addr + "/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return cmd.Process, cleanup
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cleanup()
	panic("gateway unavailable " + addr)
}
func p99(s []int64) int64 {
	if len(s) == 0 {
		return -1
	}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(math.Ceil(.99*float64(len(s))))-1]
}
func main() {
	bin := flag.String("gateway", "", "gateway executable")
	viewers := flag.Int("viewers", 10000, "even number of viewers")
	strategy := flag.String("strategy", "bounded", "crash, fixed or bounded")
	port := flag.Int("port", 38980, "first of two unused gateway ports")
	batch := flag.Int("batch", 100, "invitations per 100 ms")
	limit := flag.Int("inflight", 64, "outstanding invitations in bounded mode")
	flag.Parse()
	if *viewers < 2 || *viewers%2 != 0 || *batch < 1 || *limit < 1 {
		log.Fatal("invalid load")
	}
	l := lab{a: fmt.Sprintf("127.0.0.1:%d", *port), b: fmt.Sprintf("127.0.0.1:%d", *port+1), path: "/api/rooms/release/" + rand.Text()[:20], attempts: map[int]int{}}
	cap := *limit
	switch *strategy {
	case "crash", "fixed":
		cap = 0
	case "bounded":
	default:
		log.Fatal("invalid strategy")
	}
	proc, endA := launch(*bin, l.a, "-migration-batch", strconv.Itoa(*batch), "-migration-inflight", strconv.Itoa(cap))
	defer endA()
	_, endB := launch(*bin, l.b)
	defer endB()
	resp, err := httpClient.Post("http://"+l.b+"/api/session", "", nil)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	cookie := resp.Cookies()[0]
	l.cookie = http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}}
	sender, err := l.connect(l.b)
	if err != nil {
		panic(err)
	}
	defer sender.Close()
	// Fixed-rate sequence covers startup, handoff and the final steady period.
	l.count = 80
	sem := make(chan struct{}, 64)
	for i := range *viewers {
		sem <- struct{}{}
		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			addr := l.b
			onA := i%2 == 0
			if onA {
				addr = l.a
			}
			c, e := l.connect(addr)
			<-sem
			if e == nil {
				l.ready.Add(1)
				e = l.watch(c, onA)
			}
			if e != nil {
				l.mu.Lock()
				l.errors = append(l.errors, e.Error())
				l.mu.Unlock()
			}
		}()
	}
	for range 100 {
		if int(l.ready.Load()) == *viewers {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if int(l.ready.Load()) != *viewers {
		panic(fmt.Sprintf("only %d viewers connected", l.ready.Load()))
	}
	for n := range l.count {
		if n == 2 {
			l.start.Store(time.Now().UnixNano())
			sig := syscall.SIGTERM
			if *strategy == "crash" {
				sig = syscall.SIGKILL
			}
			if err := proc.Signal(sig); err != nil {
				panic(err)
			}
		}
		e := sender.WriteJSON(map[string]any{"type": 101, "data": map[string]any{"content": strconv.Itoa(n), "client_msg_id": strconv.Itoa(n)}})
		if e != nil {
			panic(e)
		}
		time.Sleep(250 * time.Millisecond)
	}
	l.wg.Wait()
	peak := 0
	for _, n := range l.attempts {
		peak = max(peak, n)
	}
	result := map[string]any{
		"strategy":                 *strategy,
		"last_handoff_ms":          l.lastMoveMS,
		"viewers":                  *viewers,
		"events":                   l.count,
		"complete_viewers":         *viewers - len(l.errors),
		"invited":                  l.invited.Load(),
		"moved":                    l.moved.Load(),
		"peak_attempts_per_sec":    peak,
		"attempts_by_second":       l.attempts,
		"recovery_p99_ms":          p99(l.recovery),
		"resident_delivery_p99_ms": p99(l.stableLatency),
	}
	data, _ := json.Marshal(result)
	fmt.Println(string(data))
	if len(l.errors) > 0 || int(l.moved.Load()) != *viewers/2 {
		panic("incomplete: " + strings.Join(l.errors[:min(3, len(l.errors))], ", "))
	}
}
