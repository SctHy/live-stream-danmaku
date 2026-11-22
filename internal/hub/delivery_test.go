package hub

import (
	"context"
	"hash/maphash"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/metrics"
)

func TestMixedFanoutAndBudget(t *testing.T) {
	m := Hub{metrics: metrics.NewGatewayMetrics()}
	legacy := &Client{send: make(chan *outboundFrame, 3)}
	a := &Client{batch: true, send: make(chan *outboundFrame, 1)}
	b := &Client{batch: true, send: make(chan *outboundFrame, 1)}
	job := &broadcastJob{Clients: []*Client{legacy, a, b}, Packets: [][]byte{[]byte(`{"n":1}`), []byte(`{"n":2}`)}}
	m.deliver(job)
	frame := <-a.send
	if frame != <-b.send || string(frame.payload) != `[{"n":1},{"n":2}]` || frame.count != 2 {
		t.Fatal("batch not shared or out of order")
	}
	for _, p := range job.Packets {
		if f := <-legacy.send; string(f.payload) != string(p) || f.count != 1 {
			t.Fatal("legacy packet changed")
		}
	}
	a.pendingBytes.Store(maxQueuedBytes - 1)
	m.safeSend(a, frame)
	if !strings.Contains(scrape(m.metrics), `danmaku_dropped_messages_total{reason="client_full"} 2`) || len(a.send) != 0 || a.pendingBytes.Load() != maxQueuedBytes-1 {
		t.Fatal("byte limit or drop count")
	}
}

func TestRoomBatchDrainsInOrder(t *testing.T) {
	c := &Client{batch: true}
	m := Hub{rooms: map[string]map[*Client]struct{}{"r": {c: {}}}, snapshots: make(map[string][]*Client), batchWindow: time.Hour, batchArrivalGap: time.Second, shardSeed: maphash.MakeSeed(), deliveryQueues: []chan *broadcastJob{make(chan *broadcastJob, 100)}}
	in := make(chan []byte, 80)
	for i := range 80 {
		in <- []byte(string(rune('A' + i)))
	}
	close(in)
	b := roomBatch{hub: &m, room: "r"}
	for payload := range in {
		b.push(context.Background(), payload)
	}
	b.mu.Lock()
	b.flush()
	b.mu.Unlock()
	total := 0
	for len(m.deliveryQueues[0]) > 0 {
		job := <-m.deliveryQueues[0]
		if len(job.Packets) > maxBatchMessages || total == 0 && len(job.Packets) != 1 {
			t.Fatal("burst boundaries")
		}
		for _, p := range job.Packets {
			if string(p) != string(rune('A'+total)) {
				t.Fatal("order")
			}
			total++
		}
	}
	if total != 80 {
		t.Fatal(total)
	}
}

func TestArrivalGapAndLocalViewerThreshold(t *testing.T) {
	for _, tc := range []struct {
		name              string
		gap               time.Duration
		minimum, wantJobs int
	}{
		{"disabled", 0, 1, 3},
		{"eligible", time.Hour, 1, 2},
		{"below-local-minimum", time.Hour, 2, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{batch: true}
			jobs := make(chan *broadcastJob, 3)
			m := Hub{
				rooms:           map[string]map[*Client]struct{}{"r": {c: {}}},
				snapshots:       make(map[string][]*Client),
				deliveryQueues:  []chan *broadcastJob{jobs},
				shardSeed:       maphash.MakeSeed(),
				batchWindow:     time.Hour,
				batchArrivalGap: tc.gap,
				minBatchViewers: tc.minimum,
			}
			messages := make(chan []byte, 3)
			for _, p := range []string{"a", "b", "c"} {
				messages <- []byte(p)
			}
			close(messages)
			b := roomBatch{hub: &m, room: "r"}
			for payload := range messages {
				b.push(context.Background(), payload)
			}
			b.mu.Lock()
			b.flush()
			b.mu.Unlock()
			if len(jobs) != tc.wantJobs {
				t.Fatalf("got %d jobs, want %d", len(jobs), tc.wantJobs)
			}
			if first := <-jobs; len(first.Packets) != 1 || string(first.Packets[0]) != "a" {
				t.Fatal("first event was not dispatched immediately")
			}
		})
	}
}

func TestStalledViewerAndLatestStats(t *testing.T) {
	m := Hub{sendCapacity: 1, metrics: metrics.NewGatewayMetrics()}
	stalled := m.NewClient(1, "room", nil)
	frame := m.frame([]byte("hello"), 1)
	m.safeSend(stalled, frame)
	for range 8 {
		m.safeSend(stalled, frame)
	}
	select {
	case <-stalled.done:
	default:
		t.Fatal("stalled viewer still connected")
	}
	out := scrape(m.metrics)
	for _, sample := range []string{`danmaku_dropped_messages_total{reason="client_full"} 8`, `danmaku_dropped_messages_total{reason="shard_full"} 0`, "danmaku_kicked_slow_clients_total 1"} {
		if !strings.Contains(out, sample) {
			t.Fatal(sample)
		}
	}
	healthy := m.NewClient(2, "room", nil)
	healthy.setStats(frame)
	latest := m.frame([]byte("new"), 1)
	healthy.setStats(latest)
	m.safeSend(healthy, frame)
	if <-healthy.stats != latest || <-healthy.send != frame {
		t.Fatal("stats displaced chat")
	}
}

func scrape(m *metrics.GatewayMetrics) string {
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	return w.Body.String()
}

// Hold the underlying close open so concurrent callers must wait for the socket.
type gatedConn struct {
	net.Conn
	entered, release, closed chan struct{}
}

func (g *gatedConn) Close() error {
	close(g.entered)
	<-g.release
	err := g.Conn.Close()
	close(g.closed)
	return err
}
func TestConcurrentClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()
		ws.ReadMessage()
	}))
	defer server.Close()
	gate := &gatedConn{entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		var err error
		gate.Conn, err = (&net.Dialer{}).DialContext(ctx, network, addr)
		return gate, err
	}}
	ws, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := (&Hub{}).NewClient(1, "r", ws)
	var callers sync.WaitGroup
	callers.Go(c.Close)
	<-gate.entered
	starts := make(chan struct{}, 32)
	for range cap(starts) {
		callers.Go(func() {
			starts <- struct{}{}
			c.Close()
			select {
			case <-gate.closed:
			default:
				t.Error("Close returned while socket was closing")
			}
			select {
			case <-c.done:
			default:
				t.Error("missing shutdown signal")
			}
		})
	}
	for range cap(starts) {
		<-starts
	}
	close(gate.release)
	callers.Wait()
}
func TestBatchTimerFlush(t *testing.T) {
	c := &Client{batch: true}
	jobs := make(chan *broadcastJob, 2)
	m := Hub{snapshots: map[string][]*Client{"r": {c}}, deliveryQueues: []chan *broadcastJob{jobs}, shardSeed: maphash.MakeSeed(), batchArrivalGap: time.Hour, batchWindow: time.Millisecond}
	b := roomBatch{hub: &m, room: "r"}
	b.push(t.Context(), []byte("first"))
	b.push(t.Context(), []byte("second"))
	for _, want := range []string{"first", "second"} {
		select {
		case job := <-jobs:
			if len(job.Packets) != 1 || string(job.Packets[0]) != want {
				t.Fatal(job)
			}
		case <-time.After(time.Second):
			t.Fatal("pending batch did not flush")
		}
	}
}
