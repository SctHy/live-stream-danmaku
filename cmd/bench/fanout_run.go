// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"flag"
	"fmt"

	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func fanout(args []string) {
	flags := flag.NewFlagSet("fanout", flag.ExitOnError)
	addr := flags.String("addr", "127.0.0.1:8080", "gateway address")
	viewers := flags.Int("viewers", 1000, "hot-room connections")
	rate := flags.Int("rate", 25, "hot-room messages per second")
	seconds := flags.Int("seconds", 10, "sending duration")
	legacy := flags.Bool("legacy", false, "use only legacy readers")
	peer := flags.String("peer", "", "second gateway for alternating readers")
	mixed := flags.Int("legacy-every", 0, "use the legacy frame protocol for every Nth reader")
	stalled := flags.Int("stalled", 0, "additional hot-room sockets that stop reading")
	rooms := flags.Int("idle-rooms", 0, "additional occupied rooms")
	churn := flags.Duration("churn", 0, "interval between hot-room join/leave cycles")
	peers := flags.String("peers", "", "comma-separated additional gateways, round robin")
	drain := flags.Duration("drain", 5*time.Second, "maximum time to receive outstanding events after sending")
	flags.Parse(args)
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
	if *mixed < 0 || *stalled < 0 || *rooms < 0 || *churn < 0 {
		fatalf("negative workload parameter")
	}
	if *viewers < 1 || *rate < 1 || *seconds < 1 || *drain <= 0 {
		fatalf("positive load parameters required")
	}
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	if *rate > *viewers*7 {
		fatalf("increase viewers for the configured per-guest send rate")
	}
	n := *rate * *seconds
	coldRate := 5
	coldCount := coldRate * *seconds

	var readers sync.WaitGroup
	cold := connect(*addr, tag+"cold", coldCount, !*legacy, true, &readers)
	hot := make([]*observer, *viewers)
	for i := range hot {
		host := hosts[i%len(hosts)]
		batch := !*legacy && (*mixed == 0 || (i+1)%*mixed != 0)
		hot[i] = connect(host, tag+"hot", n, batch, i < len(hosts), &readers)
	}
	idle := func(room string) *websocket.Conn {
		c := dial(*peer, tag+room, false)
		if tcp, ok := c.UnderlyingConn().(*net.TCPConn); ok {
			tcp.SetReadBuffer(1024)
		}
		return c
	}
	for range *stalled {
		defer idle("hot").Close()
	}
	for i := range *rooms {
		defer idle(fmt.Sprintf("idle%d", i)).Close()
	}
	stopChurn := make(chan struct{})
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
					idle("hot").Close()
				}
			}
		}()
	}
	defer func() { close(stopChurn); churnWG.Wait() }()
	time.Sleep(300 * time.Millisecond)
	expected := n * *viewers
	var writers sync.WaitGroup
	writers.Add(1)
	go func() { defer writers.Done(); send([]*observer{cold}, coldCount, coldRate) }()
	writersForHot := hot
	sendStart := time.Now()
	send(writersForHot, n, *rate)
	sendElapsed := time.Since(sendStart)
	writers.Wait()
	deadline := time.Now().Add(*drain)
	for time.Now().Before(deadline) {
		complete := cold.received.Load() == int64(coldCount)
		for _, o := range hot {
			complete = complete && o.received.Load() == int64(n)
		}
		if complete {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cold.conn.Close()
	for _, o := range hot {
		o.conn.Close()
	}
	readers.Wait()
	total, frames := int64(0), 0
	var payloadBytes int64
	missing, duplicate, gaps, affected, worst := 0, 0, 0, 0, 0
	for i, o := range hot {
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
				fmt.Printf("viewer=%d missing=%d first_missing=%d duplicates=%d first_duplicate=%d cursor_gaps=%d\n", i, lost, firstMissing, o.duplicates, o.firstDuplicate, o.gaps)
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
		fmt.Printf("gateway=%s samples=%d p50_us=%d p99_us=%d\n", host, len(samples), percentile(samples, 50), percentile(samples, 99))
	}
	fmt.Printf("viewers=%d target_rate=%d achieved_rate=%.2f seconds=%d received=%d expected=%d frames=%d payload_bytes=%d missing=%d duplicates=%d cursor_gaps=%d affected=%d worst_missing=%d hot_p99_us=%d cold_p99_us=%d cold_received=%d cold_unique=%d\n", *viewers, *rate, float64(n)/(sendElapsed.Seconds()+1/float64(*rate)), *seconds, total, expected, frames, payloadBytes, missing, duplicate, gaps, affected, worst, percentile(hot[0].latency, 99), percentile(cold.latency, 99), cold.received.Load(), cold.unique)
	if affected > 0 || cold.unique != coldCount || cold.duplicates > 0 || cold.gaps > 0 {
		fatalf("delivery verification failed")
	}
}
