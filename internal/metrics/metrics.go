// Package metrics owns the per-gateway Prometheus registry.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type GatewayMetrics struct {
	reg                   *prometheus.Registry
	connections, rooms    prometheus.Gauge
	single, batch         prometheus.Counter
	clientFull, shardFull prometheus.Counter
	kicked, kafkaFailures prometheus.Counter
	rejected              map[string]prometheus.Counter
	wait                  prometheus.Histogram
}

func NewGatewayMetrics() *GatewayMetrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	gauge := func(name, help string) prometheus.Gauge {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "danmaku", Name: name, Help: help})
		reg.MustRegister(g)
		return g
	}
	counter := func(name, help string) prometheus.Counter {
		c := prometheus.NewCounter(prometheus.CounterOpts{Namespace: "danmaku", Name: name, Help: help})
		reg.MustRegister(c)
		return c
	}
	vector := func(name, help, label string) *prometheus.CounterVec {
		v := prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "danmaku", Name: name, Help: help}, []string{label})
		reg.MustRegister(v)
		return v
	}
	frames := vector("fanout_frames_total", "Room broadcast frames successfully written to viewers.", "mode")
	drops := vector("dropped_messages_total", "Recipient messages dropped due to full delivery queues.", "reason")
	rejects := vector("rejected_total", "Rejected upstream actions.", "reason")
	m := &GatewayMetrics{
		reg:           reg,
		connections:   gauge("active_connections", "WebSocket connections registered on this gateway."),
		rooms:         gauge("active_rooms", "Rooms with local viewers."),
		single:        frames.WithLabelValues("single"),
		batch:         frames.WithLabelValues("batch"),
		clientFull:    drops.WithLabelValues("client_full"),
		shardFull:     drops.WithLabelValues("shard_full"),
		kicked:        counter("kicked_slow_clients_total", "Connections closed after consecutive enqueue failures."),
		kafkaFailures: counter("kafka_publish_failures_total", "Failed asynchronous Kafka publications."),
		rejected:      make(map[string]prometheus.Counter),
		wait: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "danmaku",
			Name:      "delivery_queue_wait_seconds",
			Help:      "Time a broadcast job spent in the shard queue.",
			Buckets:   []float64{.0005, .001, .002, .005, .01, .025, .05, .1, .25},
		}),
	}
	for _, reason := range []string{"invalid", "muted", "rate_limited", "room_muted", "slow_mode", "filtered", "unavailable"} {
		m.rejected[reason] = rejects.WithLabelValues(reason)
	}
	reg.MustRegister(m.wait)
	return m
}
func (m *GatewayMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
func (m *GatewayMetrics) Membership(connections, rooms int) {
	if m != nil {
		m.connections.Add(float64(connections))
		m.rooms.Add(float64(rooms))
	}
}
func (m *GatewayMetrics) Fanout(count int) {
	if m != nil {
		if count > 1 {
			m.batch.Inc()
		} else {
			m.single.Inc()
		}
	}
}
func (m *GatewayMetrics) QueueWait(d time.Duration) {
	if m != nil {
		m.wait.Observe(d.Seconds())
	}
}
func (m *GatewayMetrics) ClientDrop(n int) {
	if m != nil {
		m.clientFull.Add(float64(n))
	}
}
func (m *GatewayMetrics) ShardDrop(n int) {
	if m != nil {
		m.shardFull.Add(float64(n))
	}
}
func (m *GatewayMetrics) Kick() {
	if m != nil {
		m.kicked.Inc()
	}
}
func (m *GatewayMetrics) Reject(reason string) {
	if m != nil {
		if c := m.rejected[reason]; c != nil {
			c.Inc()
		}
	}
}
func (m *GatewayMetrics) KafkaFailure() {
	if m != nil {
		m.kafkaFailures.Inc()
	}
}
