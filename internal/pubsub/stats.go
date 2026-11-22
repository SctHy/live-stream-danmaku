package pubsub

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

// SnapshotStats batches reads and writes for a bounded chunk of rooms. It removes
// acknowledged increments from likes, including when a later command fails.
func (s *Store) SnapshotStats(ctx context.Context, server string, rooms map[string]int, likes map[string]uint64, ttl time.Duration) (map[string]protocol.StatsData, error) {
	r := s.r
	p := r.Pipeline()
	optional := make(map[redis.Cmder]bool)
	increments := make(map[string]*redis.IntCmd)
	owners := make(map[string]*redis.StringSliceCmd)
	counts := make(map[string]*redis.StringCmd)
	tails := make(map[string]*redis.XMessageSliceCmd)
	for room, n := range rooms {
		key := fmt.Sprintf(KeyRoomLikes, room)
		if delta := likes[room]; delta > 0 {
			increments[room] = p.IncrBy(ctx, key, int64(delta))
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
		optional[counts[room]] = true
		if strings.Contains(room, "/") {
			tails[room] = p.XRevRangeN(ctx, "dm:{"+room+"}:recent", "+", "-", 1)
		}
	}
	cmds, execErr := p.Exec(ctx)
	// Remove acknowledged increments so callers only requeue unconfirmed likes.
	for room, cmd := range increments {
		if cmd.Err() == nil {
			delete(likes, room)
		}
	}
	for _, cmd := range cmds {
		if err := cmd.Err(); err != nil && !(optional[cmd] && err == redis.Nil) {
			return nil, err
		}
	}
	if err := execErr; err != nil && err != redis.Nil {
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
	type staleOwner struct {
		keys   []string
		server string
		cmd    *redis.Cmd
	}
	var stale []staleOwner
	out := make(map[string]protocol.StatsData, len(online))
	for room, cmd := range online {
		var stats protocol.StatsData
		if counts[room].Err() == nil {
			var err error
			stats.Likes, err = counts[room].Uint64()
			if err != nil {
				return nil, err
			}
		}
		for i, v := range cmd.Val() {
			if raw, ok := v.(string); ok {
				n, err := strconv.ParseUint(raw, 10, 64)
				if err != nil {
					return nil, err
				}
				stats.Online += n
			} else {
				id := owners[room].Val()[i]
				keys := []string{fmt.Sprintf(KeyRoomServerOnline, room, id), fmt.Sprintf(KeyServerOfRoom, room)}
				stale = append(stale, staleOwner{keys, id, s.cleanup.EvalSha(ctx, p, keys, id)})
			}
		}
		if tail := tails[room]; tail != nil && len(tail.Val()) > 0 {
			stats.Position = StreamPosition(tail.Val()[0])
		}
		out[room] = stats
	}
	if _, err := p.Exec(ctx); err != nil {
		if !redis.HasErrorPrefix(err, "NOSCRIPT") {
			return nil, err
		}
		// A cluster node may not have seen the script yet. Retry only its misses.
		for _, owner := range stale {
			switch err := owner.cmd.Err(); {
			case redis.HasErrorPrefix(err, "NOSCRIPT"):
				s.cleanup.Eval(ctx, p, owner.keys, owner.server)
			case err != nil:
				return nil, err
			}
		}
		if _, err := p.Exec(ctx); err != nil {
			return nil, err
		}
	}
	return out, nil
}

const removeExpiredServerLua = `
if redis.call('exists', KEYS[1]) == 0 then
 return redis.call('srem', KEYS[2], ARGV[1])
end
return 0`
