package hub

import (
	"context"
	"hash/maphash"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scthy/maku/internal/metrics"
)

func TestMixedFanoutAndBudget(t *testing.T) {
	m := Hub{metrics: metrics.NewGatewayMetrics()}
	legacy := &Client{Send: make(chan *outboundFrame, 3)}
	a := &Client{batch: true, Send: make(chan *outboundFrame, 1)}
	b := &Client{batch: true, Send: make(chan *outboundFrame, 1)}
	job := &BroadcastJob{Clients: []*Client{legacy, a, b}, Packets: [][]byte{[]byte(`{"n":1}`), []byte(`{"n":2}`)}}
	m.deliver(job)
	frame := <-a.Send
	if frame != <-b.Send || string(frame.payload) != `[{"n":1},{"n":2}]` || frame.count != 2 {
		t.Fatal("batch not shared or out of order")
	}
	for _, p := range job.Packets {
		if f := <-legacy.Send; string(f.payload) != string(p) || f.count != 1 {
			t.Fatal("legacy packet changed")
		}
	}
	a.pendingBytes.Store(maxQueuedBytes - 1)
	m.safeSend(a, frame)
	if !strings.Contains(scrape(m.metrics), `danmaku_dropped_messages_total{reason="client_full"} 2`) || len(a.Send) != 0 || a.pendingBytes.Load() != maxQueuedBytes-1 {
		t.Fatal("byte limit or drop count")
	}
}

func TestRoomBatchDrainsInOrder(t *testing.T) {
	c := &Client{batch: true}
	m := Hub{rooms: map[string]map[*Client]struct{}{"r": {c: {}}}, snapshots: make(map[string][]*Client), BatchWindow: time.Hour, BatchArrivalGap: time.Second, shardSeed: maphash.MakeSeed(), deliveryQueues: []chan *BroadcastJob{make(chan *BroadcastJob, 100)}}
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
			jobs := make(chan *BroadcastJob, 3)
			m := Hub{
				rooms:           map[string]map[*Client]struct{}{"r": {c: {}}},
				snapshots:       make(map[string][]*Client),
				deliveryQueues:  []chan *BroadcastJob{jobs},
				shardSeed:       maphash.MakeSeed(),
				BatchWindow:     time.Hour,
				BatchArrivalGap: tc.gap,
				MinBatchViewers: tc.minimum,
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
	if <-healthy.stats != latest || <-healthy.Send != frame {
		t.Fatal("stats displaced chat")
	}
}

func scrape(m *metrics.GatewayMetrics) string {
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	return w.Body.String()
}
