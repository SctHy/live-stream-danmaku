package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/eval"
	"github.com/scthy/maku/internal/protocol"
)

func (l *lab) launch(bin, addr string, flags ...string) (*gatewayProcess, func(), error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	listener.Close()
	_, port, _ := net.SplitHostPort(addr)
	args := append([]string{"-port", port}, flags...)
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	g := &gatewayProcess{Process: cmd.Process, done: make(chan struct{})}
	done := g.done
	go func() { g.err = cmd.Wait(); g.exited = time.Now(); close(done) }()
	cleanup := func() { cmd.Process.Kill(); <-done }
	for range 150 {
		select {
		case <-done:
			return nil, nil, errors.New("test gateway exited")
		default:
		}
		resp, err := l.http.Get("http://" + addr + "/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return g, cleanup, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cleanup()
	return nil, nil, fmt.Errorf("gateway unavailable %s", addr)
}

type gatewayProcess struct {
	*os.Process
	done   chan struct{}
	exited time.Time
	err    error
}

type MigrationResult struct {
	Strategy string             `json:"strategy"`
	Attempts map[int]int        `json:"attempts_by_second"`
	Metrics  map[string]float64 `json:"metrics"`
	Passed   bool               `json:"passed"`
	Errors   []string           `json:"errors,omitempty"`
}

// Preserve the original flat migration output while also exposing run metrics.
func (r MigrationResult) MarshalJSON() ([]byte, error) {
	out := map[string]any{"strategy": r.Strategy, "attempts_by_second": r.Attempts, "metrics": r.Metrics, "passed": r.Passed}
	if len(r.Errors) > 0 {
		out["errors"] = r.Errors
	}
	for k, v := range r.Metrics {
		out[k] = v
	}
	return json.Marshal(out)
}
func peakQPS(times []int64) int {
	slices.Sort(times)
	left, peak := 0, 0
	for i, t := range times {
		for t-times[left] >= int64(100*time.Millisecond) {
			left++
		}
		peak = max(peak, i-left+1)
	}
	return peak * 10
}
func migration(args []string) {
	r, e := runMigration(args, false)
	if errors.Is(e, flag.ErrHelp) {
		return
	}
	if e == nil {
		e = json.NewEncoder(os.Stdout).Encode(r)
	}
	if e != nil {
		fatalf("%v", e)
	}
	if !r.Passed {
		fatalf("incomplete: %s", strings.Join(r.Errors, ", "))
	}
}
func runMigration(args []string, sampleMetrics bool) (result MigrationResult, err error) {
	flags := flag.NewFlagSet("migration", flag.ContinueOnError)
	flags.Bool("json", false, "emit structured JSON")
	events := flags.Int("events", 80, "number of sequential messages")
	bin := flags.String("gateway", "", "gateway executable")
	viewers := flags.Int("viewers", 10000, "even number of viewers")
	strategy := flags.String("strategy", "bounded", "crash, fixed or bounded")
	port := flags.Int("port", 38980, "first of two unused gateway ports")
	batch := flags.Int("batch", 100, "invitations per 100 ms")
	limit := flags.Int("inflight", 64, "outstanding invitations in bounded mode")
	if err = flags.Parse(args); err != nil {
		return
	}
	count := *events
	if count < 3 || *viewers < 2 || *viewers%2 != 0 || *batch < 1 || *limit < 1 {
		return result, errors.New("invalid load")
	}
	httpClient := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 128, MaxConnsPerHost: 512}}
	dialer := websocket.Dialer{Subprotocols: []string{protocol.BatchProtocol}, HandshakeTimeout: 10 * time.Second, NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := new(net.Dialer).DialContext(ctx, network, addr)
		if err == nil {
			c.(*net.TCPConn).SetLinger(0)
		}
		return c, err
	}}

	l := lab{http: httpClient, dialer: dialer, a: fmt.Sprintf("127.0.0.1:%d", *port), b: fmt.Sprintf("127.0.0.1:%d", *port+1), path: "/api/rooms/release/" + rand.Text()[:20], attempts: map[int]int{}}
	if *port == 0 {
		l.a, l.b, err = freeAddresses()
		if err != nil {
			return
		}
	}
	cap := *limit
	switch *strategy {
	case "crash", "fixed":
		cap = 0
	case "bounded":
	default:
		return result, errors.New("invalid strategy")
	}
	proc, endA, err := l.launch(*bin, l.a, "-migration-batch", strconv.Itoa(*batch), "-migration-inflight", strconv.Itoa(cap))
	if err != nil {
		return result, err
	}
	defer endA()
	_, endB, err := l.launch(*bin, l.b)
	if err != nil {
		return result, err
	}
	defer endB()
	defer httpClient.CloseIdleConnections()
	var before eval.Snapshot
	if sampleMetrics {
		before, err = eval.Scrape(httpClient, "http://"+l.b+"/metrics")
		if err != nil {
			return
		}
	}
	resp, err := l.http.Post("http://"+l.b+"/api/session", "", nil)
	if err != nil {
		return result, err
	}
	resp.Body.Close()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return result, fmt.Errorf("guest session returned HTTP %d without a cookie", resp.StatusCode)
	}
	cookie := cookies[0]
	l.cookie = http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}}
	sender, err := l.connect(l.b)
	if err != nil {
		return result, err
	}
	defer sender.Close()
	// Fixed-rate sequence covers startup, handoff and the final steady period.
	l.expectedMoves = int64(*viewers / 2)
	l.count = count
	l.sent = make([]atomic.Int64, count)
	sem := make(chan struct{}, 64)
	for i := range *viewers {
		sem <- struct{}{}
		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			addr := l.b
			onA := i%2 == 0
			if onA {
				addr = l.a
			}
			c, e := l.connect(addr)
			<-sem
			if e == nil {
				l.ready.Add(1)
				e = l.watch(c, onA)
			}
			if e != nil {
				l.mu.Lock()
				l.errors = append(l.errors, e.Error())
				l.mu.Unlock()
			}
		}()
	}
	for range 100 {
		if int(l.ready.Load()) == *viewers {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if int(l.ready.Load()) != *viewers {
		return result, fmt.Errorf("only %d viewers connected", l.ready.Load())
	}
	for n := range l.count {
		if n == 2 {
			l.start.Store(time.Now().UnixNano())
			sig := syscall.SIGTERM
			if *strategy == "crash" {
				sig = syscall.SIGKILL
			}
			if err := proc.Signal(sig); err != nil {
				return result, err
			}
		}
		l.sent[n].Store(time.Now().UnixNano())
		e := sender.WriteJSON(map[string]any{"type": protocol.TypeDanmaku, "data": map[string]any{"content": strconv.Itoa(n), "client_msg_id": strconv.Itoa(n)}})
		if e != nil {
			return result, e
		}
		time.Sleep(250 * time.Millisecond)
	}
	l.wg.Wait()
	peak := 0
	for _, n := range l.attempts {
		peak = max(peak, n)
	}
	m := map[string]float64{
		"last_handoff_ms":       float64(l.lastMoveMS),
		"viewers":               float64(*viewers),
		"events":                float64(count),
		"complete_viewers":      float64(*viewers - len(l.errors)),
		"invited":               float64(l.invited.Load()),
		"moved":                 float64(l.moved.Load()),
		"peak_attempts_per_sec": float64(peak),
		"connect_peak_qps":      float64(peakQPS(l.connects)),
		"recent_peak_qps":       float64(peakQPS(l.recents)),
	}
	select {
	case <-proc.done:
		if proc.err != nil && *strategy != "crash" {
			return result, proc.err
		}
		m["source_exit_ms"] = float64(proc.exited.Sub(time.Unix(0, l.start.Load())).Microseconds()) / 1000
	case <-time.After(40 * time.Second):
		return result, errors.New("source gateway drain timed out")
	}
	m["drain_ms"] = max(m["last_handoff_ms"], m["source_exit_ms"])
	for name, values := range map[string][]int64{"recovery": l.recovery, "resident_delivery": l.stableLatency, "migrating_delivery": l.movingLatency, "resident_during_handoff": l.during} {
		unit := "_us"
		if name == "recovery" {
			unit = "_ms"
		}
		m[name+"_samples"] = float64(len(values))
		for _, p := range []int{50, 95, 99} {
			m[fmt.Sprintf("%s_p%d%s", name, p, unit)] = float64(percentile(values, p))
		}
	}
	m["resident_delivery_p99_ms"] = float64(int64(m["resident_delivery_p99_us"]) / 1000)
	if sampleMetrics {
		after, e := eval.Scrape(httpClient, "http://"+l.b+"/metrics")
		if e != nil {
			return result, e
		}
		for k, v := range after.Changes(before) {
			m[k] = v
		}
	}
	return MigrationResult{
		Strategy: *strategy, Attempts: l.attempts, Metrics: m, Errors: l.errors,
		Passed: len(l.errors) == 0 && l.moved.Load() == l.expectedMoves,
	}, nil
}
func freeAddresses() (string, string, error) {
	a, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return "", "", e
	}
	defer a.Close()
	b, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return "", "", e
	}
	defer b.Close()
	return a.Addr().String(), b.Addr().String(), nil
}
