// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

type observer struct {
	conn                     *websocket.Conn
	received                 atomic.Int64
	frames                   int
	latency                  []int64
	seen                     []uint64
	unique, duplicates, gaps int
	firstDuplicate           int
	cursor                   string
	bytes                    int64
}

func dial(addr, room string, id int, batch, browser bool) *websocket.Conn {
	dialer := *websocket.DefaultDialer
	if batch {
		dialer.Subprotocols = []string{protocol.BatchProtocol}
	}
	path := fmt.Sprintf("/ws?room=%s&uid=%d", room, id)
	headers := http.Header{}
	if browser {
		response, err := http.Post("http://"+addr+"/api/session", "application/json", nil)
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		cookies := response.Cookies()
		if response.StatusCode != 200 || len(cookies) == 0 {
			log.Fatal("guest session unavailable")
		}
		headers.Set("Cookie", cookies[0].Name+"="+cookies[0].Value)
		path = "/api/rooms/" + room + "/load/live"
	}
	conn, _, err := dialer.Dial("ws://"+addr+path, headers)
	if err != nil {
		log.Fatal(err)
	}
	// Keep completed loopback runs from consuming the ephemeral port range in TIME_WAIT.
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		if err := tcp.SetLinger(0); err != nil {
			log.Fatal(err)
		}
	}
	if browser {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var ready protocol.WsPacket
		if err := conn.ReadJSON(&ready); err != nil || ready.Type != protocol.TypeReady {
			log.Fatalf("subscription failed: %v", err)
		}
		conn.SetReadDeadline(time.Time{})
	}
	return conn
}

// Each room has a unique run name. Sequence numbers are local to that room.
func (o *observer) record(seq int) {
	word, mask := seq/64, uint64(1)<<(seq%64)
	if o.seen[word]&mask != 0 {
		if o.duplicates == 0 {
			o.firstDuplicate = seq
		}
		o.duplicates++
		return
	}
	o.seen[word] |= mask
	o.unique++
}

func connect(addr, room string, id, expected int, batch, sample, browser bool, readers *sync.WaitGroup) *observer {
	conn := dial(addr, room, id, batch, browser)
	expectedRoom := room
	if browser {
		expectedRoom += "/load"
	}
	o := &observer{conn: conn, seen: make([]uint64, (expected+63)/64), cursor: "0-0", firstDuplicate: -1}
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			now := time.Now().UnixMicro()
			packets, err := protocol.DecodePackets(data)
			if err != nil {
				log.Fatal(err)
			}
			n := 0
			for _, p := range packets {
				if p.Type != protocol.TypeDanmaku {
					continue
				}
				if p.RoomID != expectedRoom {
					log.Fatalf("unexpected room %q for viewer in %q", p.RoomID, expectedRoom)
				}
				n++
				var d struct{ Content string }
				if err := json.Unmarshal(p.Data, &d); err != nil {
					log.Fatal(err)
				}
				prefix, stamp, ok := strings.Cut(d.Content, "/")
				seq, err := strconv.Atoi(prefix)
				if !ok || err != nil || seq < 0 || seq >= expected {
					log.Fatalf("unexpected sequence %q", d.Content)
				}
				o.record(seq)
				if browser {
					if p.Previous != o.cursor || p.Position == "" {
						o.gaps++
					}
					o.cursor = p.Position
				}
				if sample {
					sent, err := strconv.ParseInt(stamp, 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					o.latency = append(o.latency, now-sent)
				}
			}
			if n > 0 {
				o.bytes += int64(len(data))
				o.frames++
				o.received.Add(int64(n))
			}
		}
	}()
	return o
}

func send(writers []*observer, n, rate int) {
	start := time.Now()
	for i := 0; i < n; i++ {
		time.Sleep(time.Until(start.Add(time.Second * time.Duration(i) / time.Duration(rate))))
		data := fmt.Appendf(nil, `{"type":101,"data":{"content":"%d/%d"}}`, i, time.Now().UnixMicro())
		if err := writers[i%len(writers)].conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Fatal(err)
		}
	}
}

func percentile(samples []int64, percent int) int64 {
	if len(samples) == 0 {
		return -1
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[(percent*len(samples)-1)/100]
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "gateway address")
	viewers := flag.Int("viewers", 1000, "hot-room connections")
	rate := flag.Int("rate", 25, "hot-room messages per second")
	seconds := flag.Int("seconds", 10, "sending duration")
	legacy := flag.Bool("legacy", false, "use only legacy readers")
	peer := flag.String("peer", "", "second gateway for alternating readers")
	mixed := flag.Int("legacy-every", 0, "use the legacy frame protocol for every Nth reader")
	stalled := flag.Int("stalled", 0, "additional hot-room sockets that stop reading")
	rooms := flag.Int("idle-rooms", 0, "additional occupied rooms")
	churn := flag.Duration("churn", 0, "interval between hot-room join/leave cycles")
	browser := flag.Bool("browser", false, "use guest sessions and the recoverable browser protocol")
	peers := flag.String("peers", "", "comma-separated additional gateways, round robin")
	drain := flag.Duration("drain", 5*time.Second, "maximum time to receive outstanding events after sending")
	flag.Parse()
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
		log.Fatal("negative workload parameter")
	}
	if *viewers < 1 || *rate < 1 || *seconds < 1 || *drain <= 0 {
		log.Fatal("positive load parameters required")
	}
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	if *browser && *rate > *viewers*7 {
		log.Fatal("increase viewers for the configured per-guest send rate")
	}
	n := *rate * *seconds
	coldRate := 20
	if *browser {
		coldRate = 5
	}
	coldCount := coldRate * *seconds

	var readers sync.WaitGroup
	cold := connect(*addr, tag+"cold", 1, coldCount, !*legacy, true, *browser, &readers)
	hot := make([]*observer, *viewers)
	for i := range hot {
		host := hosts[i%len(hosts)]
		batch := !*legacy && (*mixed == 0 || (i+1)%*mixed != 0)
		hot[i] = connect(host, tag+"hot", i+2, n, batch, i < len(hosts), *browser, &readers)
	}
	idle := func(room string) *websocket.Conn {
		c := dial(*peer, tag+room, 1, false, *browser)
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
	writersForHot := hot[:1]
	if *browser {
		writersForHot = hot
	}
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
		log.Fatal("delivery verification failed")
	}
}
