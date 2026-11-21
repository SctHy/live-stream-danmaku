package infra

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

// SnapshotStats batches network round trips across rooms. Callers bound each chunk.
func SnapshotStats(ctx context.Context, r *redis.Client, server string, rooms map[string]int, likes map[string]uint64, ttl time.Duration) (map[string]protocol.StatsData, error) {
	p := r.Pipeline()
	owners := make(map[string]*redis.StringSliceCmd)
	counts := make(map[string]*redis.StringCmd)
	tails := make(map[string]*redis.XMessageSliceCmd)
	for room, n := range rooms {
		key := fmt.Sprintf(KeyRoomLikes, room)
		if delta := likes[room]; delta > 0 {
			p.IncrBy(ctx, key, int64(delta))
		}
		if n == 0 {
			continue
		}
		registry := fmt.Sprintf(KeyServerOfRoom, room)
		p.Set(ctx, fmt.Sprintf(KeyRoomServerOnline, room, server), n, ttl)
		p.SAdd(ctx, registry, server)
		p.Expire(ctx, registry, ttl*2)
		owners[room] = p.SMembers(ctx, registry)
		counts[room] = p.Get(ctx, key)
		if strings.Contains(room, "/") {
			tails[room] = p.XRevRangeN(ctx, "dm:recent:"+room, "+", "-", 1)
		}
	}
	if _, err := p.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	online := make(map[string]*redis.SliceCmd)
	for room, cmd := range owners {
		keys := make([]string, 0, len(cmd.Val()))
		for _, id := range cmd.Val() {
			keys = append(keys, fmt.Sprintf(KeyRoomServerOnline, room, id))
		}
		if len(keys) > 0 {
			online[room] = p.MGet(ctx, keys...)
		}
	}
	if _, err := p.Exec(ctx); err != nil {
		return nil, err
	}
	out := make(map[string]protocol.StatsData, len(online))
	for room, cmd := range online {
		var s protocol.StatsData
		s.Likes, _ = counts[room].Uint64()
		for i, v := range cmd.Val() {
			if raw, ok := v.(string); ok {
				n, _ := strconv.ParseUint(raw, 10, 64)
				s.Online += n
			} else {
				id := owners[room].Val()[i]
				p.Eval(ctx, "if redis.call('exists',KEYS[1])==0 then return redis.call('srem',KEYS[2],ARGV[1]) end", []string{fmt.Sprintf(KeyRoomServerOnline, room, id), fmt.Sprintf(KeyServerOfRoom, room)}, id)
			}
		}
		if tail := tails[room]; tail != nil && len(tail.Val()) > 0 {
			s.Position = StreamPosition(tail.Val()[0])
		}
		out[room] = s
	}
	_, err := p.Exec(ctx)
	return out, err
}
