package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Msg struct {
	Type int `json:"type"`
	Data struct {
		Content string `json:"content"`
	} `json:"data"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "WebSocket server address")
	flag.Parse()
	tag := strconv.FormatInt(time.Now().UnixNano(), 36)
	var conns []*websocket.Conn
	var done sync.WaitGroup
	var hot, cold atomic.Int64
	var sample []int64
	dial := func(room string, n int, observe bool) *websocket.Conn {
		c, _, e := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/ws?room=%s%s&uid=%d", *addr, tag, room, n), nil)
		if e != nil {
			panic(e)
		}
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
				if json.Unmarshal(v, &m) != nil || m.Type != 101 {
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
	c := dial("cold", 1, true)
	var pubs []*websocket.Conn
	for i := 0; i < 12; i++ {
		pubs = append(pubs, dial("hot", i+2, i == 0))
	}
	time.Sleep(350 * time.Millisecond)
	var w sync.WaitGroup
	for i := 0; i < 4; i++ {
		w.Add(1)
		go func(i int) {
			defer w.Done()
			t := time.Now()
			for j := 0; j < 3000; j++ {
				e := pubs[i].WriteMessage(1, []byte(`{"type":101,"data":{"content":"hot"}}`))
				if e != nil {
					panic(e)
				}
				if j%10 == 9 {
					time.Sleep(time.Until(t.Add(time.Duration(j+1) * time.Millisecond)))
				}
			}
		}(i)
	}
	t := time.Now()
	for i := 0; i < 300; i++ {
		if err := c.WriteMessage(1, []byte(fmt.Sprintf(`{"type":101,"data":{"content":"%d"}}`, time.Now().UnixMicro()))); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Until(t.Add(time.Duration(i+1) * 10 * time.Millisecond)))
	}
	w.Wait()
	deadline := time.Now().Add(8 * time.Second)
	for (hot.Load() < 12000 || cold.Load() < 300) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	for _, c := range conns {
		c.Close()
	}
	done.Wait()
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	if len(sample) == 0 {
		log.Fatal("no cold-room replies")
	}
	fmt.Printf("cold=%d/300 hot=%d/12000 P50=%dus P99=%dus\n", len(sample), hot.Load(), sample[len(sample)/2], sample[(len(sample)*99-1)/100])
}
