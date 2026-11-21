package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGatewayExposition(t *testing.T) {
	m := NewGatewayMetrics()
	other := NewGatewayMetrics()
	m.Membership(3, 2)
	m.Membership(-1, -1)
	m.Fanout(1)
	m.Fanout(5)
	m.QueueWait(2 * time.Millisecond)
	m.Kick()
	m.ShardDrop(6)
	m.ClientDrop(2)
	m.KafkaFailure()
	m.Reject("filtered")
	m.Reject("unknown-room")
	scrape := func(m *GatewayMetrics) string {
		w := httptest.NewRecorder()
		m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
			t.Fatal(w.Result())
		}
		return w.Body.String()
	}
	out := scrape(m)
	if strings.Count(out, "# TYPE ") != 8 || strings.Contains(out, "unknown-room") {
		t.Fatal(out)
	}
	for _, sample := range []string{"danmaku_active_connections 2", "danmaku_active_rooms 1", `danmaku_fanout_frames_total{mode="single"} 1`, `danmaku_fanout_frames_total{mode="batch"} 1`, `danmaku_delivery_queue_wait_seconds_bucket{le="0.002"} 1`, `danmaku_dropped_messages_total{reason="shard_full"} 6`, `danmaku_dropped_messages_total{reason="client_full"} 2`, "danmaku_kicked_slow_clients_total 1", `danmaku_rejected_total{reason="filtered"} 1`, "danmaku_kafka_publish_failures_total 1"} {
		if !strings.Contains(out, "\n"+sample+"\n") {
			t.Errorf("missing sample %q", sample)
		}
	}
	if !strings.Contains(scrape(other), "\ndanmaku_active_connections 0\n") {
		t.Fatal("shared registry")
	}
}
