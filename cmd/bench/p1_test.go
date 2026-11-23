package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestRequestDelayPerRESPFrame(t *testing.T) {
	const command = "*2\r\n$4\r\nECHO\r\n$4\r\na\r\nb\r\n"
	input := strings.Repeat(command, 3)
	var out bytes.Buffer
	start := time.Now()
	err := forwardRequests(&out, iotest.OneByteReader(strings.NewReader(input)), time.Millisecond)
	if !errors.Is(err, io.EOF) || out.String() != input || time.Since(start) < 3*time.Millisecond {
		t.Fatalf("forwarded %q, error %v", out.String(), err)
	}
	for _, bad := range []string{"PING\r\n", "*-1\r\n", "*1\r\n$3\r\nxx"} {
		if err := forwardRequests(io.Discard, strings.NewReader(bad), 0); err == nil {
			t.Fatalf("accepted malformed frame %q", bad)
		}
	}
}
func TestP1LatencyRanks(t *testing.T) {
	m := newMeasurement()
	latency(&m, "x_", []float64{8, 3, 2, 1})
	if m.Metrics["x_p50_us"] != 2 || m.Metrics["x_p99_us"] != 8 || !slices.Contains(m.LowSampleWarning, "x_p99_us") {
		t.Fatal(m)
	}
}
func TestP1GateAndWarmup(t *testing.T) {
	s := p1Suite{repeats: 2}
	i := 0
	err := s.run("gate", "test", "", []string{"arm"}, func(string) (measurement, error) {
		i++
		m := newMeasurement()
		m.Metrics["counter"] = float64(i)
		gate(&m, "expected", i != 2)
		return m, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	e := s.report.Experiments[0]
	if e.Passed || e.Arms[0].Summary["counter"].Median != 2.5 || len(e.Arms[0].Runs) != 2 {
		t.Fatal(e)
	}
}
func TestP1LocalExperiments(t *testing.T) {
	s := p1Suite{ctx: context.Background(), repeats: 2}
	if err := s.filters(); err != nil {
		t.Fatal(err)
	}
	if err := s.live(); err != nil {
		t.Fatal(err)
	}
	for _, ex := range s.report.Experiments {
		if !ex.Passed {
			t.Fatalf("%s/%s failed", ex.Name, ex.Load)
		}
	}
}
