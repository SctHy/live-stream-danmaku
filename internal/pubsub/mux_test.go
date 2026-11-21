package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHundredRoomsShareEightConnections(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	name := "mux-" + rand.Text()
	r := redis.NewClient(&redis.Options{Addr: addr, ClientName: name})
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m := NewMux(ctx, r, 8)
	defer m.Close()
	for i := range 100 {
		ready, err := m.Subscribe(ctx, fmt.Sprint(i), func([]byte) {})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	clients, err := r.Do(ctx, "CLIENT", "LIST", "TYPE", "PUBSUB").Text()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(clients, "name="+name+" "); n != 8 {
		t.Fatalf("100 rooms used %d connections", n)
	}
}
