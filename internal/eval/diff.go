package eval

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
)

type Comparison struct {
	Experiment string   `json:"experiment"`
	Load       string   `json:"load"`
	Arm        string   `json:"arm,omitempty"`
	Metric     string   `json:"metric,omitempty"`
	Direction  string   `json:"direction,omitempty"`
	Baseline   *float64 `json:"baseline,omitempty"`
	Candidate  *float64 `json:"candidate,omitempty"`
	Status     string   `json:"status"`
}
type Diff struct {
	Passed    bool         `json:"passed"`
	Threshold float64      `json:"threshold"`
	Rows      []Comparison `json:"rows"`
}

func direction(k string) string {
	switch k {
	case "qps", "rows_per_second", "rows_per_sec", "achieved_rate":
		return "higher_is_better"
	case "frames", "process_cpu_seconds_total", "rss_peak_bytes", "go_memstats_alloc_bytes_total",
		"peak_attempts_per_sec", "connect_peak_qps", "recent_peak_qps", "go_gc_duration_seconds_sum",
		"danmaku_delivery_queue_wait_seconds_sum", "process_resident_memory_bytes",
		"duplicates", "cold_duplicates", "duplicate_rows", "race_duplicate_rate", "extra_broadcasts",
		"missing", "cold_missing", "allocs_per_batch", "alloc_bytes_per_batch", "miss_allocs_per_op":
		return "lower_is_better"
	}
	if strings.HasSuffix(k, "_us") || strings.HasSuffix(k, "_ms") || strings.HasSuffix(k, "_upstream_calls") {
		return "lower_is_better"
	}
	return ""
}

type experimentKey struct{ name, load string }

func index(r Report) (map[experimentKey]Experiment, error) {
	m := map[experimentKey]Experiment{}
	if r.Experiments == nil {
		return nil, fmt.Errorf("empty ledger")
	}
	for _, e := range r.Experiments {
		k := experimentKey{e.Name, e.Load}
		if _, ok := m[k]; ok || e.Name == "" {
			return nil, fmt.Errorf("ambiguous experiment %q/%q", e.Name, e.Load)
		}
		seen := map[string]bool{}
		for _, a := range e.Arms {
			if a.Name == "" || seen[a.Name] {
				return nil, fmt.Errorf("ambiguous arm %q in %s", a.Name, e.Name)
			}
			seen[a.Name] = true
		}
		m[k] = e
	}
	return m, nil
}
func CompareReports(b, c Report, threshold float64, allowMissing bool) (Diff, error) {
	d := Diff{Passed: true, Threshold: threshold}
	if threshold < 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return d, fmt.Errorf("invalid threshold")
	}
	if _, err := index(b); err != nil {
		return d, err
	}
	ci, err := index(c)
	if err != nil {
		return d, err
	}
	add := func(row Comparison) {
		d.Rows = append(d.Rows, row)
		if row.Status == "REGRESSED" || row.Status == "INVARIANT_FAIL" || (row.Status == "MISSING" && !allowMissing) {
			d.Passed = false
		}
	}
	for _, e := range c.Experiments {
		if !e.Passed {
			add(Comparison{Experiment: e.Name, Load: e.Load, Status: "INVARIANT_FAIL"})
		}
	}
	for _, e := range b.Experiments {
		row := Comparison{Experiment: e.Name, Load: e.Load, Status: "MISSING"}
		other, ok := ci[experimentKey{e.Name, e.Load}]
		if !ok {
			add(row)
			continue
		}
		arms := map[string]Arm{}
		for _, a := range other.Arms {
			arms[a.Name] = a
		}
		for _, a := range e.Arms {
			row := row
			row.Arm = a.Name
			ca, ok := arms[a.Name]
			if !ok {
				add(row)
				continue
			}
			for _, k := range slices.Sorted(maps.Keys(a.Summary)) {
				r := row
				r.Direction = direction(k)
				if r.Direction == "" {
					continue
				}
				r.Metric = k
				x := a.Summary[k].Median
				r.Baseline = &x
				s, ok := ca.Summary[k]
				if !ok {
					add(r)
					continue
				}
				y := s.Median
				r.Candidate = &y
				change := y - x
				if r.Direction == "higher_is_better" {
					change = -change
				}
				limit := math.Abs(x) * threshold
				switch {
				case change > limit:
					r.Status = "REGRESSED"
				case change < -limit:
					r.Status = "IMPROVED"
				default:
					r.Status = "NEUTRAL"
				}
				add(r)
			}
		}
	}
	return d, nil
}
