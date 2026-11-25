package eval

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

const (
	ClientFull = `danmaku_dropped_messages_total{reason="client_full"}`
	Kicked     = "danmaku_kicked_slow_clients_total"
)

var keys = []string{
	`danmaku_fanout_frames_total{mode="single"}`,
	`danmaku_fanout_frames_total{mode="batch"}`,
	ClientFull,
	`danmaku_dropped_messages_total{reason="shard_full"}`,
	Kicked,
	"danmaku_delivery_queue_wait_seconds_sum",
	"danmaku_delivery_queue_wait_seconds_count",
	"process_cpu_seconds_total",
	"process_resident_memory_bytes",
	"go_memstats_alloc_bytes_total",
	"go_gc_duration_seconds_sum",
	"go_goroutines",
}

type Snapshot map[string]float64

func Parse(r io.Reader) (Snapshot, error) {
	m := Snapshot{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 2 {
			continue
		}
		for _, k := range keys {
			if f[0] == k {
				v, e := strconv.ParseFloat(f[1], 64)
				if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, fmt.Errorf("invalid %s", k)
				}
				m[k] = v
				break
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing metric %s", k)
		}
	}
	return m, nil
}
func Scrape(c *http.Client, url string) (Snapshot, error) {
	r, e := c.Get(url)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("metrics: %s", r.Status)
	}
	return Parse(r.Body)
}
func (s Snapshot) Changes(before Snapshot) map[string]float64 {
	m := map[string]float64{}
	for k, v := range s {
		m[k] = v - before[k]
	}
	return m
}
