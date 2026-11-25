package pubsub

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Copy only expiring session data. Likes and membership still use their old
// keys. RESTORE preserves stream IDs and the original absolute expiry.
var readLegacy = redis.NewScript(`
local expires=redis.call('PEXPIRETIME',KEYS[1])
if expires<0 then return {} end
return {redis.call('DUMP',KEYS[1]),tostring(expires)}`)
var restoreLegacy = redis.NewScript(`
if redis.call('EXISTS',KEYS[2])==1 then return 0 end
local now=redis.call('TIME')
if tonumber(ARGV[2])<=now[1]*1000+math.floor(now[2]/1000) then return 0 end
if redis.call('EXISTS',KEYS[1])==0 then
 redis.call('RESTORE',KEYS[1],ARGV[2],ARGV[1],'ABSTTL')
 if ARGV[3]~='' then
  redis.call('ZADD',KEYS[3],ARGV[2],ARGV[3])
  redis.call('EXPIRE',KEYS[3],86401)
 end
end
redis.call('SET',KEYS[2],'1','PXAT',ARGV[2])
return 1`)

// UpgradeSessionKeys runs before accepting traffic on a gateway. The marker
// makes later starts constant-time. Per-key markers allow concurrent starters.
func (s *Store) UpgradeSessionKeys(ctx context.Context) error {
	const marker = "dm:migrations:room-slots-v1"
	done, err := s.r.Exists(ctx, marker).Result()
	if err != nil || done != 0 {
		return err
	}
	scan := func(ctx context.Context, node *redis.Client) error {
		iter := node.Scan(ctx, 0, "dm:{*/*}:*", 256).Iterator()
		for iter.Next(ctx) {
			old := iter.Val()
			room, suffix, ok := strings.Cut(strings.TrimPrefix(old, "dm:{"), "}:")
			if !ok {
				continue
			}
			kind, value, _ := strings.Cut(suffix, ":")
			switch kind {
			case "recent", "mute", "send", "rate":
			default:
				continue
			}
			vals, err := readLegacy.Run(ctx, s.r, []string{old}).StringSlice()
			if err != nil {
				return err
			}
			if len(vals) == 0 {
				continue
			}
			prefix := sessionPrefix(room)
			uid := ""
			if kind == "mute" {
				uid = value
			}
			err = restoreLegacy.Run(ctx, s.r, []string{prefix + suffix, prefix + "upgraded:" + suffix, prefix + "mutes"}, vals[0], vals[1], uid).Err()
			if err != nil {
				return err
			}
		}
		return iter.Err()
	}
	switch client := s.r.(type) {
	case *redis.ClusterClient:
		err = client.ForEachMaster(ctx, scan)
	case *redis.Client:
		err = scan(ctx, client)
	}
	if err != nil {
		return err
	}
	return s.r.Set(ctx, marker, 1, 0).Err()
}
