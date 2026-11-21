package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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
		ready, err := m.Subscribe(ctx, fmt.Sprintf("%s/%d", name, i), func([]byte) {})
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

func TestShardReconnectRestoresRoom(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	name := "reconnect-" + rand.Text()
	r := redis.NewClient(&redis.Options{Addr: addr, ClientName: name})
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mux := NewMux(ctx, r, 1)
	defer mux.Close()
	delivered := make(chan struct{}, 1)
	ready, err := mux.Subscribe(ctx, name, func([]byte) {
		select {
		case delivered <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	clients, err := r.Do(ctx, "CLIENT", "LIST", "TYPE", "PUBSUB").Text()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, line := range strings.Split(clients, "\n") {
		if strings.Contains(line, "name="+name+" ") {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "id=") {
					id = strings.TrimPrefix(field, "id=")
				}
			}
		}
	}
	if id == "" {
		t.Fatal("missing test subscription")
	}
	if err := r.Do(ctx, "CLIENT", "KILL", "ID", id).Err(); err != nil {
		t.Fatal(err)
	}
	for {
		err := r.Publish(ctx, fmt.Sprintf(KeyRoomPubSub, name), `{"pos":"new","prev":"old","packet":{"type":102,"data":{}}}`).Err()
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-delivered:
			return
		case <-ctx.Done():
			t.Fatal("lost subscription after reconnect")
		case <-time.After(25 * time.Millisecond):
		}
	}
}
