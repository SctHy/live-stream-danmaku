package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

func (l *lab) launch(bin, addr string, flags ...string) (*os.Process, func(), error) {
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
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
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
				return cmd.Process, cleanup, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cleanup()
	return nil, nil, fmt.Errorf("gateway unavailable %s", addr)
}
func p99(s []int64) int64 {
	if len(s) == 0 {
		return -1
	}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(math.Ceil(.99*float64(len(s))))-1]
}
func migration(args []string) {
	if err := runMigration(args); err != nil {
		fatalf("%v", err)
	}
}
func runMigration(args []string) error {
	flags := flag.NewFlagSet("migration", flag.ExitOnError)
	bin := flags.String("gateway", "", "gateway executable")
	viewers := flags.Int("viewers", 10000, "even number of viewers")
	strategy := flags.String("strategy", "bounded", "crash, fixed or bounded")
	port := flags.Int("port", 38980, "first of two unused gateway ports")
	batch := flags.Int("batch", 100, "invitations per 100 ms")
	limit := flags.Int("inflight", 64, "outstanding invitations in bounded mode")
	flags.Parse(args)
	if *viewers < 2 || *viewers%2 != 0 || *batch < 1 || *limit < 1 {
		return errors.New("invalid load")
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
	cap := *limit
	switch *strategy {
	case "crash", "fixed":
		cap = 0
	case "bounded":
	default:
		return errors.New("invalid strategy")
	}
	proc, endA, err := l.launch(*bin, l.a, "-migration-batch", strconv.Itoa(*batch), "-migration-inflight", strconv.Itoa(cap))
	if err != nil {
		return err
	}
	defer endA()
	_, endB, err := l.launch(*bin, l.b)
	if err != nil {
		return err
	}
	defer endB()
	resp, err := l.http.Post("http://"+l.b+"/api/session", "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return fmt.Errorf("guest session returned HTTP %d without a cookie", resp.StatusCode)
	}
	cookie := cookies[0]
	l.cookie = http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}}
	sender, err := l.connect(l.b)
	if err != nil {
		return err
	}
	defer sender.Close()
	// Fixed-rate sequence covers startup, handoff and the final steady period.
	l.count = 80
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
		return fmt.Errorf("only %d viewers connected", l.ready.Load())
	}
	for n := range l.count {
		if n == 2 {
			l.start.Store(time.Now().UnixNano())
			sig := syscall.SIGTERM
			if *strategy == "crash" {
				sig = syscall.SIGKILL
			}
			if err := proc.Signal(sig); err != nil {
				return err
			}
		}
		e := sender.WriteJSON(map[string]any{"type": protocol.TypeDanmaku, "data": map[string]any{"content": strconv.Itoa(n), "client_msg_id": strconv.Itoa(n)}})
		if e != nil {
			return e
		}
		time.Sleep(250 * time.Millisecond)
	}
	l.wg.Wait()
	peak := 0
	for _, n := range l.attempts {
		peak = max(peak, n)
	}
	result := map[string]any{
		"strategy":                 *strategy,
		"last_handoff_ms":          l.lastMoveMS,
		"viewers":                  *viewers,
		"events":                   l.count,
		"complete_viewers":         *viewers - len(l.errors),
		"invited":                  l.invited.Load(),
		"moved":                    l.moved.Load(),
		"peak_attempts_per_sec":    peak,
		"attempts_by_second":       l.attempts,
		"recovery_p99_ms":          p99(l.recovery),
		"resident_delivery_p99_ms": p99(l.stableLatency),
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return err
	}
	if len(l.errors) > 0 || int(l.moved.Load()) != *viewers/2 {
		return fmt.Errorf("incomplete: %s", strings.Join(l.errors[:min(3, len(l.errors))], ", "))
	}
	return nil
}
