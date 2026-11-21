package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ArchiveMetrics struct {
	reg                 *prometheus.Registry
	written, deadletter prometheus.Counter
	duration            prometheus.Histogram
}

func NewArchiveMetrics() *ArchiveMetrics {
	m := &ArchiveMetrics{
		reg:        prometheus.NewRegistry(),
		written:    prometheus.NewCounter(prometheus.CounterOpts{Name: "danmaku_archive_written_total", Help: "Rows newly inserted into PostgreSQL."}),
		deadletter: prometheus.NewCounter(prometheus.CounterOpts{Name: "danmaku_archive_deadletter_total", Help: "History records isolated due to invalid data."}),
		duration:   prometheus.NewHistogram(prometheus.HistogramOpts{Name: "danmaku_archive_batch_duration_seconds", Help: "Duration of individual database batch write attempts."}),
	}
	m.reg.MustRegister(m.written, m.deadletter, m.duration)
	return m
}
func (m *ArchiveMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
func (m *ArchiveMetrics) Batch(rows int64, elapsed time.Duration) {
	if m != nil {
		m.written.Add(float64(rows))
		m.duration.Observe(elapsed.Seconds())
	}
}
func (m *ArchiveMetrics) DeadLetter() {
	if m != nil {
		m.deadletter.Inc()
	}
}
