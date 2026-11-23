package main

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scthy/maku/internal/eval"
)

type p1Suite struct {
	ctx      context.Context
	full     bool
	repeats  int
	selected map[string]bool
	report   eval.Report
}
type measurement = eval.RunSample

func newMeasurement() measurement { return measurement{Metrics: map[string]float64{}} }
func gate(s *measurement, name string, ok bool) {
	s.Invariants = append(s.Invariants, eval.InvariantResult{Name: name, Passed: ok})
}
func latency(s *measurement, name string, times []float64) {
	slices.Sort(times)
	for _, p := range []int{50, 95, 99, 999} {
		q := float64(p) / 100
		if p == 999 {
			q = .999
		}
		s.Metrics[fmt.Sprintf("%sp%d_us", name, p)] = times[int(math.Ceil(q*float64(len(times))))-1]
	}
	s.Metrics[name+"samples"] = float64(len(times))
	if len(times) < 1000 {
		s.LowSampleWarning = append(s.LowSampleWarning, name+"p999_us")
	}
	if len(times) < 100 {
		s.LowSampleWarning = append(s.LowSampleWarning, name+"p99_us")
	}
}
func micros(t time.Time) float64 { return float64(time.Since(t)) / float64(time.Microsecond) }
func burst(n int, fn func(int)) {
	var ready, done sync.WaitGroup
	ready.Add(n)
	done.Add(n)
	start := make(chan struct{})
	for i := range n {
		go func() { defer done.Done(); ready.Done(); <-start; fn(i) }()
	}
	ready.Wait()
	close(start)
	done.Wait()
}
func (s *p1Suite) run(name, load, note string, arms []string, fn func(string) (measurement, error)) error {
	ex := eval.Experiment{Name: name, Load: load, TradeOffNote: note}
	for _, a := range arms {
		ex.Arms = append(ex.Arms, eval.Arm{Name: a})
	}
	err := ex.Measure(s.repeats, func(a eval.Arm, round int) (measurement, error) {
		m, err := fn(a.Name)
		if round == 0 {
			for _, g := range m.Invariants {
				if !g.Passed {
					return m, fmt.Errorf("warmup invariant %s failed", g.Name)
				}
			}
		}
		return m, err
	})
	if err != nil {
		return err
	}
	for _, a := range ex.Arms {
		for _, k := range slices.Sorted(maps.Keys(a.Summary)) {
			delta := "-"
			if d := a.Deltas[k]; d.Percent != nil {
				delta = fmt.Sprintf("%+.2f%%", *d.Percent)
				if strings.HasSuffix(k, "_us") && *d.Multiple > 0 {
					delta += fmt.Sprintf(" (%.2fx speedup)", 1 / *d.Multiple)
				}
			}
			fmt.Printf("%s\t%s\t%s\t%s\t%.3f\t%s\n", name, load, a.Name, k, a.Summary[k].Median, delta)
		}
	}
	s.report.Experiments = append(s.report.Experiments, ex)
	return nil
}
