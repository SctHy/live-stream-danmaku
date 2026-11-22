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

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

type Msg struct {
	Type int `json:"type"`
	Data struct {
		Content string `json:"content"`
	} `json:"data"`
}

func rooms(args []string) {
	flags := flag.NewFlagSet("rooms", flag.ExitOnError)
	addr := flags.String("addr", "127.0.0.1:8080", "WebSocket server address")
	flags.Parse(args)
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	var conns []*websocket.Conn
	var done sync.WaitGroup
	var hot, cold atomic.Int64
	var sample []int64
	join := func(room string, observe bool) *websocket.Conn {
		c := dial(*addr, tag+room, false)
		conns = append(conns, c)
		done.Add(1)
		go func() {
			defer done.Done()
			for {
				_, v, e := c.ReadMessage()
				if e != nil {
					return
				}
				if !observe {
					continue
				}
				var m Msg
				if json.Unmarshal(v, &m) != nil || m.Type != protocol.TypeDanmaku {
					continue
				}
				if room == "cold" {
					t, _ := strconv.ParseInt(m.Data.Content, 10, 64)
					sample = append(sample, time.Now().UnixMicro()-t)
					cold.Add(1)
				} else {
					hot.Add(1)
				}
			}
		}()
		return c
	}
	c := join("cold", true)
	var pubs []*websocket.Conn
	for i := 0; i < 32; i++ {
		pubs = append(pubs, join("hot", i == 0))
	}
	time.Sleep(350 * time.Millisecond)
	var w sync.WaitGroup
	for i := 0; i < len(pubs); i++ {
		w.Add(1)
		go func(i int) {
			defer w.Done()
			t := time.Now()
			for j := 0; j < 20; j++ {
				e := pubs[i].WriteMessage(1, fmt.Appendf(nil, `{"type":%d,"data":{"content":"hot"}}`, protocol.TypeDanmaku))
				if e != nil {
					panic(e)
				}
				time.Sleep(time.Until(t.Add(time.Duration(j+1) * 200 * time.Millisecond)))
			}
		}(i)
	}
	t := time.Now()
	for i := 0; i < 20; i++ {
		if err := c.WriteMessage(1, []byte(fmt.Sprintf(`{"type":%d,"data":{"content":"%d"}}`, protocol.TypeDanmaku, time.Now().UnixMicro()))); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Until(t.Add(time.Duration(i+1) * 200 * time.Millisecond)))
	}
	w.Wait()
	deadline := time.Now().Add(8 * time.Second)
	for (hot.Load() < 640 || cold.Load() < 20) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	for _, c := range conns {
		c.Close()
	}
	done.Wait()
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	if len(sample) != 20 || hot.Load() != 640 {
		log.Fatal("incomplete room delivery", len(sample), hot.Load())
	}
	fmt.Printf("cold=%d/20 hot=%d/640 P50=%dus P99=%dus\n", len(sample), hot.Load(), sample[len(sample)/2], sample[(len(sample)*99-1)/100])
}
