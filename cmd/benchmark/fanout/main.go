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
	"sync"
	"sync/atomic"
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"github.com/gorilla/websocket"
)

type observer struct {
	conn     *websocket.Conn
	received atomic.Int64
	frames   int
	latency  []int64
}

func dial(addr, room string, id int, batch, browser bool) *websocket.Conn {
	dialer := *websocket.DefaultDialer
	if batch {
		dialer.Subprotocols = []string{model.BatchProtocol}
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
	if browser {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var ready model.WsPacket
		if err := conn.ReadJSON(&ready); err != nil || ready.Type != 106 {
			log.Fatalf("subscription failed: %v", err)
		}
		conn.SetReadDeadline(time.Time{})
	}
	return conn
}
func connect(addr, room string, id int, batch, sample, browser bool, readers *sync.WaitGroup) *observer {
	conn := dial(addr, room, id, batch, browser)
	o := &observer{conn: conn}
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			now := time.Now().UnixMicro()
			packets, err := model.DecodePackets(data)
			if err != nil {
				log.Fatal(err)
			}
			n := 0
			for _, p := range packets {
				if p.Type != model.TypeDanmaku {
					continue
				}
				n++
				if sample {
					var d struct{ Content string }
					if err := json.Unmarshal(p.Data, &d); err != nil {
						log.Fatal(err)
					}
					sent, err := strconv.ParseInt(d.Content, 10, 64)
					if err != nil {
						log.Fatal(err)
					}
					o.latency = append(o.latency, now-sent)
				}
			}
			if n > 0 {
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
		data := fmt.Appendf(nil, `{"type":101,"data":{"content":"%d"}}`, time.Now().UnixMicro())
		if err := writers[i%len(writers)].conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Fatal(err)
		}
	}
}

func p99(samples []int64) int64 {
	if len(samples) == 0 {
		return -1
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[(99*len(samples)-1)/100]
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "gateway address")
	viewers := flag.Int("viewers", 64, "hot-room connections")
	rate := flag.Int("rate", 1000, "hot-room messages per second")
	seconds := flag.Int("seconds", 3, "sending duration")
	legacy := flag.Bool("legacy", false, "use only legacy readers")
	peer := flag.String("peer", "", "second gateway for alternating readers")
	mixed := flag.Int("legacy-every", 0, "use the legacy frame protocol for every Nth reader")
	stalled := flag.Int("stalled", 0, "additional hot-room sockets that stop reading")
	rooms := flag.Int("idle-rooms", 0, "additional occupied rooms")
	churn := flag.Duration("churn", 0, "interval between hot-room join/leave cycles")
	browser := flag.Bool("browser", false, "use guest sessions and the recoverable browser protocol")
	flag.Parse()
	if *peer == "" {
		*peer = *addr
	}
	if *mixed < 0 || *stalled < 0 || *rooms < 0 || *churn < 0 {
		log.Fatal("negative workload parameter")
	}
	if *viewers < 1 || *rate < 1 || *seconds < 1 {
		log.Fatal("positive load parameters required")
	}
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	if *browser && *rate > *viewers*7 {
		log.Fatal("increase viewers for the configured per-guest send rate")
	}
	var readers sync.WaitGroup
	cold := connect(*addr, tag+"cold", 1, !*legacy, true, *browser, &readers)
	hot := make([]*observer, *viewers)
	for i := range hot {
		host := *addr
		if i%2 == 1 {
			host = *peer
		}
		batch := !*legacy && (*mixed == 0 || (i+1)%*mixed != 0)
		hot[i] = connect(host, tag+"hot", i+2, batch, i == 0, *browser, &readers)
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
	n := *rate * *seconds
	coldRate := 20
	if *browser {
		coldRate = 5
	}
	coldCount := coldRate * *seconds
	expected := n * *viewers
	var writers sync.WaitGroup
	writers.Add(1)
	go func() { defer writers.Done(); send([]*observer{cold}, coldCount, coldRate) }()
	writersForHot := hot[:1]
	if *browser {
		writersForHot = hot
	}
	send(writersForHot, n, *rate)
	writers.Wait()
	deadline := time.Now().Add(2 * time.Second)
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
	for _, o := range hot {
		total += o.received.Load()
		frames += o.frames
	}
	fmt.Printf("received=%d expected=%d frames=%d hot_p99_us=%d cold_p99_us=%d cold_received=%d\n", total, expected, frames, p99(hot[0].latency), p99(cold.latency), cold.received.Load())
	if total != int64(expected) || cold.received.Load() != int64(coldCount) {
		log.Fatal("incomplete delivery")
	}
}
