package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/eval"
)

type deliveryWindow struct {
	unique, duplicates, gaps int
	latency                  []int64
}

type soakMonitor struct {
	interval, stalled int
	client            *http.Client
	url               string
	resources         []eval.Snapshot
	windows           []map[string]float64
}

func (s *soakMonitor) monitor(stop <-chan struct{}, seconds int, dial func() (*websocket.Conn, error)) error {
	var wave []*websocket.Conn
	closeWave := func() {
		for _, c := range wave {
			c.Close()
		}
		wave = nil
	}
	defer closeWave()
	start := time.Now()
	for offset := 0; offset < seconds; offset += s.interval {
		closeWave()
		for range s.stalled {
			c, err := dial()
			if err != nil {
				return err
			}
			wave = append(wave, c)
		}
		end := min(seconds, offset+s.interval)
		select {
		case <-stop:
			return nil
		case <-time.After(time.Until(start.Add(time.Duration(end) * time.Second))):
		}
		snap, err := eval.Scrape(s.client, s.url)
		if err != nil {
			return err
		}
		snap["elapsed_seconds"] = time.Since(start).Seconds()
		s.resources = append(s.resources, snap)
	}
	return nil
}

// Attribute late arrivals to the interval of their published sequence, after drain.
func (s *soakMonitor) delivery(viewers []*observer, rate, seconds int, room string) {
	for i := range viewers[0].windows {
		if len(s.windows) <= i {
			s.windows = append(s.windows, map[string]float64{})
		}
		m := s.windows[i]
		var lat []int64
		for _, o := range viewers {
			w := o.windows[i]
			m[room+"_missing"] += float64(rate*min(s.interval, seconds-i*s.interval) - w.unique)
			m[room+"_duplicates"] += float64(w.duplicates)
			m[room+"_cursor_gaps"] += float64(w.gaps)
			lat = append(lat, w.latency...)
		}
		m[room+"_p99_us"] = float64(percentile(lat, 99))
	}
}

func parseSteps(raw string) ([]int, error) {
	var steps []int
	for _, s := range strings.Split(raw, ",") {
		n, e := strconv.Atoi(strings.TrimSpace(s))
		if e != nil || n < 1 || len(steps) > 0 && n <= steps[len(steps)-1] {
			return nil, fmt.Errorf("expected positive increasing viewer counts: %q", raw)
		}
		steps = append(steps, n)
	}
	return steps, nil
}

type loadReport struct {
	Mode     string                        `json:"mode"`
	Runs     []map[string]float64          `json:"runs"`
	Config   []string                      `json:"fanout_args,omitempty"`
	Interval int                           `json:"interval_seconds,omitempty"`
	Stalled  int                           `json:"stalled_per_interval,omitempty"`
	Phases   map[string]map[string]float64 `json:"phases,omitempty"`
	Total    map[string]float64            `json:"total,omitempty"`
	Leak     string                        `json:"leak_assessment,omitempty"`
	Passed   bool                          `json:"passed"`
}

func loadBench(mode string, args []string) error {
	f := flag.NewFlagSet(mode, flag.ContinueOnError)
	gateway := f.String("gateway", "", "gateway executable (default: build locally)")
	out := f.String("out", "", "save JSON report")
	rate := f.Int("rate", 240, "hot-room messages per second")
	var steps string
	var viewers, stalled int
	var dur, interval, churn time.Duration
	if mode == "capacity" {
		f.StringVar(&steps, "steps", "128,256,512,1024", "increasing hot-room viewer counts")
		f.DurationVar(&dur, "duration", 5*time.Second, "sending duration per step")
	} else {
		f.IntVar(&viewers, "viewers", 128, "persistent hot-room viewers")
		f.IntVar(&stalled, "stalled", 2, "new stalled sockets per interval")
		f.DurationVar(&churn, "churn", 200*time.Millisecond, "viewer join/leave interval")
		f.DurationVar(&dur, "duration", 3*time.Minute, "continuous sending duration")
		f.DurationVar(&interval, "interval", 15*time.Second, "resource sampling and stalled socket replacement interval")
	}
	if err := f.Parse(args); err != nil {
		return err
	}
	if *rate <= 0 || dur < time.Second || dur%time.Second != 0 {
		return errors.New("positive rate and whole-second duration required")
	}
	counts := []int{viewers}
	var err error
	if mode == "capacity" {
		counts, err = parseSteps(steps)
		if err != nil {
			return err
		}
	} else if viewers < 1 || stalled < 1 || churn <= 0 || interval < time.Second || interval%time.Second != 0 || dur/interval < 3 {
		return errors.New("soak needs positive viewers, stalled and churn, and at least three whole-second intervals")
	}
	if *rate > counts[0]*7 {
		return errors.New("increase viewers for the configured per-guest send rate")
	}
	if *gateway == "" {
		dir, err := os.MkdirTemp("", "maku-load-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		*gateway = filepath.Join(dir, "gateway")
		cmd := exec.Command("go", "build", "-o", *gateway, "./cmd/gateway")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	report := loadReport{Mode: mode, Passed: true}
	for _, n := range counts {
		args := []string{"-viewers", strconv.Itoa(n), "-rate", strconv.Itoa(*rate), "-seconds", strconv.Itoa(int(dur / time.Second)), "-cold-rate", "10", "-sample-viewers", "4"}
		if mode == "capacity" {
			r, err := measure(*gateway, mode, eval.Arm{Args: args})
			if err != nil {
				return err
			}
			report.Runs = append(report.Runs, r.Metrics)
			report.Passed = report.Passed && r.Invariants[0].Passed
			fmt.Fprintf(os.Stderr, "viewers=%d rate=%.1f hot_p99_us=%.0f cpu_seconds=%.3f delivery=%t\n", n, r.Metrics["achieved_rate"], r.Metrics["hot_p99_us"], r.Metrics["process_cpu_seconds_total"], r.Invariants[0].Passed)
		} else {
			args = append(args, "-churn", churn.String(), "-padding", "400")
			report.Config = args
			if err := runSoak(*gateway, args, int(interval/time.Second), stalled, &report); err != nil {
				return err
			}
		}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if *out != "" {
		err = os.WriteFile(*out, data, 0644)
	} else {
		_, err = os.Stdout.Write(data)
	}
	if err != nil {
		return err
	}
	if !report.Passed {
		return errors.New("workload gate failed")
	}
	return nil
}

func runSoak(bin string, args []string, interval, stalled int, r *loadReport) error {
	addr, _, err := freeAddresses()
	if err != nil {
		return err
	}
	l := lab{http: &http.Client{Timeout: 3 * time.Second}}
	_, end, err := l.launch(bin, addr)
	if err != nil {
		return err
	}
	defer end()
	s := soakMonitor{interval: interval, stalled: stalled, client: l.http, url: "http://" + addr + "/metrics"}
	fan, _, err := observedFanout(append([]string{"-addr", addr}, args...), io.Discard, &s)
	if err != nil {
		return err
	}
	r.Total = fan.Metrics
	r.Passed = fan.Passed
	for i, m := range s.windows {
		for k, v := range s.resources[i] {
			m[k] = v
		}
		r.Runs = append(r.Runs, m)
	}
	r.Interval, r.Stalled = interval, stalled
	start, mid, endRun := r.Runs[0], r.Runs[(len(r.Runs)-1)/2], r.Runs[len(r.Runs)-1]
	r.Phases = map[string]map[string]float64{"start": start, "middle": mid, "end": endRun}
	r.Leak = leakAssessment(start, mid, endRun)
	r.Passed = r.Passed && endRun[eval.ClientFull] > 0 && endRun[eval.Kicked] > 0
	return nil
}

func leakAssessment(first, middle, last map[string]float64) string {
	// Compare equal-load samples. RSS includes the runtime heap and producer buffers.
	growing := func(k string, tolerance float64) bool { return middle[k] > first[k] && last[k] > middle[k]+tolerance }
	if growing("process_resident_memory_bytes", 32<<20) || growing("go_goroutines", 8) {
		return "sustained growth observed; investigate retention"
	}
	return "no sustained growth beyond 32 MiB RSS or 8 goroutines from midpoint to end"
}
