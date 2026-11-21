// Measures per-danmaku latency, frame count, and delivery to every viewer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
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

func connect(addr, room string, id int, batch, sample bool, readers *sync.WaitGroup) *observer {
	dialer := *websocket.DefaultDialer
	if batch {
		dialer.Subprotocols = []string{model.BatchProtocol}
	}
	conn, _, err := dialer.Dial(fmt.Sprintf("ws://%s/ws?room=%s&uid=%d", addr, room, id), nil)
	if err != nil {
		log.Fatal(err)
	}
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

func send(c *websocket.Conn, n, rate int) {
	start := time.Now()
	for i := 0; i < n; i++ {
		time.Sleep(time.Until(start.Add(time.Second * time.Duration(i) / time.Duration(rate))))
		data := fmt.Appendf(nil, `{"type":101,"data":{"content":"%d"}}`, time.Now().UnixMicro())
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
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
	flag.Parse()
	if *viewers < 1 || *rate < 1 || *seconds < 1 {
		log.Fatal("positive load parameters required")
	}
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	var readers sync.WaitGroup
	cold := connect(*addr, tag+"cold", 1, !*legacy, true, &readers)
	hot := make([]*observer, *viewers)
	for i := range hot {
		hot[i] = connect(*addr, tag+"hot", i+2, !*legacy, i == 0, &readers)
	}
	time.Sleep(300 * time.Millisecond)
	n := *rate * *seconds
	coldCount := 20 * *seconds
	expected := n * *viewers
	var writers sync.WaitGroup
	writers.Add(1)
	go func() { defer writers.Done(); send(cold.conn, coldCount, 20) }()
	send(hot[0].conn, n, *rate)
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
