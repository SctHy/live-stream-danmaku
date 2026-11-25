package main

import (
	"slices"
	"testing"
)

func TestLoadSteps(t *testing.T) {
	got, err := parseSteps("12, 24")
	if err != nil || !slices.Equal(got, []int{12, 24}) {
		t.Fatal(got, err)
	}
	for _, raw := range []string{"", "-1", "1,1", "2,1", "2,x"} {
		if _, err := parseSteps(raw); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestPartialSoakWindow(t *testing.T) {
	s := soakMonitor{interval: 3}
	s.delivery([]*observer{{windows: []deliveryWindow{{unique: 6}, {unique: 1, duplicates: 1, gaps: 1, latency: []int64{100}}}}}, 2, 4, "hot")
	last := s.windows[1]
	if s.windows[0]["hot_missing"] != 0 || last["hot_missing"] != 1 || last["hot_duplicates"] != 1 || last["hot_cursor_gaps"] != 1 || last["hot_p99_us"] != 100 {
		t.Fatal(s.windows)
	}
}
func TestLeakAssessment(t *testing.T) {
	sample := func(n float64) map[string]float64 { return map[string]float64{"go_goroutines": n} }
	if leakAssessment(sample(10), sample(15), sample(24)) == leakAssessment(sample(10), sample(15), sample(23)) {
		t.Fatal("growth thresholds ignored")
	}
}
