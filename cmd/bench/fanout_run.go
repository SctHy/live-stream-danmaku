// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type FanoutResult struct {
	Metrics map[string]float64 `json:"metrics"`
	Passed  bool               `json:"passed"`
}

func fanout(args []string) {
	r, j, e := runFanout(args, os.Stdout)
	if errors.Is(e, flag.ErrHelp) {
		return
	}
	if j && e == nil {
		e = json.NewEncoder(os.Stdout).Encode(r)
	}
	if e != nil {
		fatalf("%v", e)
	}
	if !r.Passed {
		fatalf("delivery verification failed")
	}
}
func runFanout(args []string, log io.Writer) (result FanoutResult, jsonMode bool, err error) {
	return observedFanout(args, log, nil)
}

func observedFanout(args []string, log io.Writer, soak *soakMonitor) (result FanoutResult, jsonMode bool, err error) {
	flags := flag.NewFlagSet("fanout", flag.ContinueOnError)
	addr := flags.String("addr", "127.0.0.1:8080", "gateway address")
	viewers := flags.Int("viewers", 1000, "hot-room connections")
	rate := flags.Int("rate", 25, "hot-room messages per second")
	seconds := flags.Int("seconds", 10, "sending duration")
	padding := flags.Int("padding", 0, "extra hot-message CJK characters (0-450)")
	legacy := flags.Bool("legacy", false, "use only legacy readers")
	peer := flags.String("peer", "", "second gateway for alternating readers")
	mixed := flags.Int("legacy-every", 0, "use the legacy frame protocol for every Nth reader")
	stalled := flags.Int("stalled", 0, "additional hot-room sockets that stop reading")
	rooms := flags.Int("idle-rooms", 0, "additional occupied rooms")
	churn := flags.Duration("churn", 0, "interval between hot-room join/leave cycles")
	peers := flags.String("peers", "", "comma-separated additional gateways, round robin")
	drain := flags.Duration("drain", 5*time.Second, "maximum time to receive outstanding events after sending")
	coldRate := flags.Int("cold-rate", 5, "cold-room messages per second")
	sample := flags.Int("sample-viewers", 1, "hot viewers sampled (at least one per gateway)")
	jsonFlag := flags.Bool("json", false, "emit JSON")
	if err = flags.Parse(args); err != nil {
		return
	}
	jsonMode = *jsonFlag
	if jsonMode {
		log = io.Discard
	}
	hosts := []string{*addr}
	if *peer != "" {
		hosts = append(hosts, *peer)
	}
	if *peers != "" {
		hosts = append(hosts, strings.Split(*peers, ",")...)
	}
	if *peer == "" {
		*peer = *addr
	}
	if *padding < 0 || *padding > 450 {
		return result, jsonMode, fmt.Errorf("invalid padding")
	}
	if *mixed < 0 || *stalled < 0 || *rooms < 0 || *churn < 0 {
		return result, jsonMode, fmt.Errorf("negative workload parameter")
	}
	if *coldRate < 1 || *sample < 1 || *viewers < 1 || *rate < 1 || *seconds < 1 || *drain <= 0 {
		return result, jsonMode, fmt.Errorf("positive load parameters required")
	}
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	if *rate > *viewers*7 {
		return result, jsonMode, fmt.Errorf("increase viewers for the configured per-guest send rate")
	}
	n := *rate * *seconds

	coldCount := *coldRate * *seconds

	var interval int
	if soak != nil {
		interval = soak.interval
	}
	var readers sync.WaitGroup
	cold, err := connect(*addr, tag+"cold", coldCount, interval**coldRate, !*legacy, true, &readers)
	if err != nil {
		return
	}
	var sockets []*websocket.Conn
	sockets = append(sockets, cold.conn)
	defer func() {
		for _, c := range sockets {
			c.Close()
		}
		readers.Wait()
	}()
	coldWriters := []*observer{cold}
	for i := 1; i < (*coldRate+4)/5; i++ {
		o, e := connect(*addr, tag+"cold", coldCount, interval**coldRate, !*legacy, false, &readers)
		if e != nil {
			return result, jsonMode, e
		}
		sockets = append(sockets, o.conn)
		coldWriters = append(coldWriters, o)
	}
	hot := make([]*observer, *viewers)
	for i := range hot {
		host := hosts[i%len(hosts)]
		batch := !*legacy && (*mixed == 0 || (i+1)%*mixed != 0)
		hot[i], err = connect(host, tag+"hot", n, interval**rate, batch, i < max(*sample, len(hosts)), &readers)
		if err != nil {
			return
		}
		sockets = append(sockets, hot[i].conn)
	}
	idle := func(room string) (*websocket.Conn, error) {
		c, e := dial(*peer, tag+room, false)
		if e != nil {
			return nil, e
		}
		if tcp, ok := c.UnderlyingConn().(*net.TCPConn); ok {
			if e := tcp.SetReadBuffer(1024); e != nil {
				c.Close()
				return nil, e
			}
		}
		return c, nil
	}
	for range *stalled {
		c, e := idle("hot")
		if e != nil {
			return result, jsonMode, e
		}
		defer c.Close()
	}
	for i := range *rooms {
		c, e := idle(fmt.Sprintf("idle%d", i))
		if e != nil {
			return result, jsonMode, e
		}
		defer c.Close()
	}
	stopChurn := make(chan struct{})
	var churnErr error
	var churnWG sync.WaitGroup
	if *churn > 0 {
		churnWG.Add(1)
		go func() {
			defer churnWG.Done()
			ticker := time.NewTicker(*churn)
			defer ticker.Stop()
			for {
				select {
				case <-stopChurn:
					return
				case <-ticker.C:
					c, e := idle("hot")
					if e != nil {
						churnErr = e
						return
					}
					c.Close()
				}
			}
		}()
	}
	defer func() {
		close(stopChurn)
		churnWG.Wait()
		if err == nil {
			err = churnErr
		}
	}()
	time.Sleep(300 * time.Millisecond)
	expected := n * *viewers
	var monitorDone chan error
	if soak != nil {
		stop := make(chan struct{})
		monitorDone = make(chan error, 1)
		go func() {
			monitorDone <- soak.monitor(stop, *seconds, func() (*websocket.Conn, error) { return idle("hot") })
		}()
		defer func() {
			close(stop)
			if monitorDone != nil {
				err = errors.Join(err, <-monitorDone)
			}
		}()
	}
	var writers sync.WaitGroup
	writers.Add(1)
	var coldErr error
	go func() { defer writers.Done(); coldErr = send(coldWriters, coldCount, *coldRate, "") }()
	writersForHot := hot
	sendStart := time.Now()
	suffix := ""
	if *padding > 0 {
		suffix = "/" + strings.Repeat("测", *padding)
	}
	err = send(writersForHot, n, *rate, suffix)
	sendElapsed := time.Since(sendStart)
	writers.Wait()
	if err != nil {
		return
	}
	if coldErr != nil {
		return result, jsonMode, coldErr
	}
	if monitorDone != nil {
		e := <-monitorDone
		monitorDone = nil
		if e != nil {
			return result, jsonMode, e
		}
	}
	deadline := time.Now().Add(*drain)
	for time.Now().Before(deadline) {
		complete := true
		for _, o := range coldWriters {
			complete = complete && o.received.Load() == int64(coldCount)
		}
		for _, o := range hot {
			complete = complete && o.received.Load() == int64(n)
		}
		if complete {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, o := range coldWriters {
		o.conn.Close()
	}
	for _, o := range hot {
		o.conn.Close()
	}
	readers.Wait()
	if soak != nil {
		soak.delivery(hot, *rate, *seconds, "hot")
		soak.delivery(coldWriters, *coldRate, *seconds, "cold")
	}
	total, frames := int64(0), 0
	var payloadBytes int64
	missing, duplicate, gaps, affected, worst := 0, 0, 0, 0, 0
	for i, o := range hot {
		if o.err != nil {
			return result, jsonMode, o.err
		}
		total += o.received.Load()
		frames += o.frames
		payloadBytes += o.bytes
		lost := n - o.unique
		if lost > 0 || o.duplicates > 0 || o.gaps > 0 {
			if affected < 3 {
				firstMissing := -1
				for seq := 0; seq < n; seq++ {
					if o.seen[seq/64]&(uint64(1)<<(seq%64)) == 0 {
						firstMissing = seq
						break
					}
				}
				fmt.Fprintf(log, "viewer=%d missing=%d first_missing=%d duplicates=%d first_duplicate=%d cursor_gaps=%d\n", i, lost, firstMissing, o.duplicates, o.firstDuplicate, o.gaps)
			}
			affected++
		}
		worst = max(worst, lost)
		missing += lost
		duplicate += o.duplicates
		gaps += o.gaps
	}
	for i, host := range hosts[:min(len(hosts), len(hot))] {
		samples := hot[i].latency
		fmt.Fprintf(log, "gateway=%s samples=%d p50_us=%d p99_us=%d\n", host, len(samples), percentile(samples, 50), percentile(samples, 99))
	}
	var latency []int64
	for _, o := range hot {
		latency = append(latency, o.latency...)
	}
	summaryLatency := hot[0].latency
	if *sample > 1 {
		summaryLatency = latency
	}
	fmt.Fprintf(log, "viewers=%d target_rate=%d achieved_rate=%.2f seconds=%d received=%d expected=%d frames=%d payload_bytes=%d missing=%d duplicates=%d cursor_gaps=%d affected=%d worst_missing=%d hot_p99_us=%d cold_p99_us=%d cold_received=%d cold_unique=%d\n", *viewers, *rate, float64(n)/(sendElapsed.Seconds()+1/float64(*rate)), *seconds, total, expected, frames, payloadBytes, missing, duplicate, gaps, affected, worst, percentile(summaryLatency, 99), percentile(cold.latency, 99), cold.received.Load(), cold.unique)
	coldMissing, coldDuplicates, coldGaps := 0, 0, 0
	for _, o := range coldWriters {
		if o.err != nil {
			return result, jsonMode, o.err
		}
		coldMissing += coldCount - o.unique
		coldDuplicates += o.duplicates
		coldGaps += o.gaps
	}
	result.Metrics = map[string]float64{
		"viewers":         float64(*viewers),
		"seconds":         float64(*seconds),
		"target_rate":     float64(*rate),
		"achieved_rate":   float64(n) / (sendElapsed.Seconds() + 1/float64(*rate)),
		"cold_rate":       float64(*coldRate),
		"cold_received":   float64(cold.received.Load()),
		"cold_unique":     float64(cold.unique),
		"affected":        float64(affected),
		"worst_missing":   float64(worst),
		"hot_samples":     float64(len(latency)),
		"cold_samples":    float64(len(cold.latency)),
		"hot_p50_us":      float64(percentile(latency, 50)),
		"hot_p99_us":      float64(percentile(latency, 99)),
		"cold_p50_us":     float64(percentile(cold.latency, 50)),
		"cold_p99_us":     float64(percentile(cold.latency, 99)),
		"frames":          float64(frames),
		"received":        float64(total),
		"expected":        float64(expected),
		"payload_bytes":   float64(payloadBytes),
		"missing":         float64(missing),
		"duplicates":      float64(duplicate),
		"cursor_gaps":     float64(gaps),
		"cold_missing":    float64(coldMissing),
		"cold_duplicates": float64(coldDuplicates),
		"cold_gaps":       float64(coldGaps),
	}
	result.Passed = affected == 0 && coldMissing == 0 && coldDuplicates == 0 && coldGaps == 0
	return result, jsonMode, cold.err
}
