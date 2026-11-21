package hub

import "sync/atomic"

type Metrics struct {
	dropped      atomic.Uint64
	shardDrops   atomic.Uint64
	clientDrops  atomic.Uint64
	waitNanos    atomic.Uint64
	measuredJobs atomic.Uint64
	liveFailures atomic.Uint64
}

func (m *Hub) Counters() map[string]uint64 {
	var failures uint64
	if m.producer != nil {
		failures = m.producer.Failures.Load()
	}
	return map[string]uint64{
		"shard_recipient_drops":        m.shardDrops.Load(),
		"client_recipient_drops":       m.clientDrops.Load(),
		"delivery_queue_wait_ns_total": m.waitNanos.Load(),
		"delivery_jobs":                m.measuredJobs.Load(),
		"journal_errors":               m.liveFailures.Load(),
		"history_publish_failures":     failures,
	}
}
