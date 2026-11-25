package pubsub

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	KeyRoomLikes        = "dm:{%s}:likes"
	KeyServerOfRoom     = "dm:{%s}:servers"
	KeyRoomServerOnline = "dm:{%s}:online:%s"
	KeyRoomPubSub       = "dm:{%s}:pubsub"
)

type Store struct {
	r                                              redis.UniversalClient
	appendEvent, acceptEvent, allowAction, cleanup *redis.Script
}

func Open(addr string) *Store {
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	return newStore(redis.NewUniversalClient(&redis.UniversalOptions{Addrs: strings.Split(addr, ","), ContextTimeoutEnabled: true, MaxRetries: -1}))
}
func (s *Store) Close() error                   { return s.r.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.r.Ping(ctx).Err() }
func (s *Store) Guest(ctx context.Context, token string) (uint64, error) {
	return s.r.Get(ctx, "dm:guest:"+token).Uint64()
}
func (s *Store) SaveGuest(ctx context.Context, token string, uid uint64, ttl time.Duration) error {
	return s.r.Set(ctx, "dm:guest:"+token, uid, ttl).Err()
}
func (s *Store) Mux(ctx context.Context, n int) *Mux { return NewMux(ctx, s.r, n) }

func newStore(r redis.UniversalClient) *Store {
	return &Store{r: r,
		appendEvent: redis.NewScript(appendEventLua), acceptEvent: redis.NewScript(acceptEventLua),
		allowAction: redis.NewScript(allowActionLua), cleanup: redis.NewScript(removeExpiredServerLua),
	}
}
