package pubsub

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func minuteKey(t time.Time) string { return fmt.Sprintf("dm:{hot}:min:%d", t.Unix()/60) }

var heatIncrement = redis.NewScript(`
if #KEYS==2 and not redis.call('SET',KEYS[2],'1','NX','EX',900) then return 0 end
redis.call('ZINCRBY',KEYS[1],ARGV[2],ARGV[1])
return redis.call('EXPIRE',KEYS[1],900)`)

type HeatBatch struct {
	Minute     int64
	ID         string
	RoomFifths map[string]uint64 // one like is one fifth of a heat point
}

var flushHeat = redis.NewScript(`
if not redis.call('SET',KEYS[2],'1','EX',900,'NX') then return 0 end
for i=1,#ARGV,2 do redis.call('ZINCRBY',KEYS[1],ARGV[i+1],ARGV[i]) end
return redis.call('EXPIRE',KEYS[1],900)`)

// FlushHeat applies minute buckets in one pipeline. Stable batch IDs allow
// replay after an ambiguous network failure without duplicating scores.
func (s *Store) FlushHeat(ctx context.Context, batches []HeatBatch) error {
	p := s.r.Pipeline()
	for _, b := range batches {
		args := make([]any, 0, 2*len(b.RoomFifths))
		for room, count := range b.RoomFifths {
			args = append(args, room, float64(count)/5)
		}
		flushHeat.Eval(ctx, p, []string{minuteKey(time.Unix(b.Minute*60, 0)), "dm:{hot}:batch:" + b.ID}, args...)
	}
	_, err := p.Exec(ctx)
	return err
}

var onlineKeys = []string{"dm:{hot}:online", "dm:{hot}:sessions", "dm:{hot}:leases"}

const expireOnline = `
local now=redis.call('TIME')[1]
local function update(room,count)
 local slug=string.match(room,'^[^/]+')
 local old=redis.call('ZSCORE',KEYS[2],room) or 0
 local total=redis.call('ZINCRBY',KEYS[1],count-old,slug)
 if tonumber(total)<=0 then redis.call('ZREM',KEYS[1],slug) end
 if count==0 then redis.call('ZREM',KEYS[2],room); redis.call('ZREM',KEYS[3],room)
 else redis.call('ZADD',KEYS[2],count,room) end
end
for _,room in ipairs(redis.call('ZRANGEBYSCORE',KEYS[3],'-inf',now)) do update(room,0) end
`

var refreshOnline = redis.NewScript(expireOnline + `
for i=2,#ARGV,2 do
 update(ARGV[i],tonumber(ARGV[i+1]))
 redis.call('ZADD',KEYS[3],now+ARGV[1],ARGV[i])
end
return 1`)
var pruneOnline = redis.NewScript(expireOnline + "return 1")

type Activity struct {
	BusiestSession string  `json:"-"`
	Online         int64   `json:"online"`
	Heat           float64 `json:"heat"`
}

func (s *Store) Activity(ctx context.Context) (map[string]Activity, error) {
	p := s.r.Pipeline()
	pruneOnline.Eval(ctx, p, onlineKeys)
	keys := make([]string, 10)
	now := time.Now()
	for i := range keys {
		keys[i] = minuteKey(now.Add(-time.Duration(i) * time.Minute))
	}
	heat := p.ZUnionWithScores(ctx, redis.ZStore{Keys: keys})
	sessions := p.ZRevRangeWithScores(ctx, onlineKeys[1], 0, -1)
	if _, err := p.Exec(ctx); err != nil {
		return nil, err
	}
	out := make(map[string]Activity)
	for _, z := range heat.Val() {
		out[z.Member.(string)] = Activity{Heat: z.Score}
	}
	for _, z := range sessions.Val() {
		slug, session, _ := strings.Cut(z.Member.(string), "/")
		a := out[slug]
		if a.BusiestSession == "" {
			a.BusiestSession = session
		}
		a.Online += int64(z.Score)
		a.Heat += z.Score
		out[slug] = a
	}
	return out, nil
}
func (s *Store) Attendance(ctx context.Context, room string) (int64, int64, error) {
	p := s.r.Pipeline()
	key := fmt.Sprintf("dm:{%s}:", room)
	online := p.Get(ctx, key+"online")
	peak := p.ZScore(ctx, key+"peak_online", "peak")
	_, _ = p.Exec(ctx)
	n, e := online.Int64()
	if e != nil && e != redis.Nil {
		return 0, 0, e
	}
	v, e := peak.Result()
	if e == redis.Nil {
		e = nil
	}
	return n, int64(v), e
}

var creditGift = redis.NewScript(`
local last=redis.call('GET',KEYS[1])
if last and (#last>#ARGV[1] or (#last==#ARGV[1] and last>=ARGV[1])) then return 0 end
redis.call('ZINCRBY',KEYS[2],ARGV[3],ARGV[2])
redis.call('HSET',KEYS[3],ARGV[2],ARGV[4])
redis.call('SET',KEYS[1],ARGV[1])
return 1`)

// Each slot acknowledges independently. The ordered PostgreSQL dispatcher can
// replay any partially completed delivery before advancing to the next order.
func (s *Store) Gift(ctx context.Context, room string, wire []byte, id int64, uid uint64, cost int64, name string, created time.Time) error {
	prefix := fmt.Sprintf("dm:{%s}:", room)
	if err := creditGift.Run(ctx, s.r, []string{prefix + "last_credit", prefix + "contributors", prefix + "names"}, id, uid, cost, name).Err(); err != nil {
		return err
	}
	journal := sessionPrefix(room)
	err := journalOnce.Run(ctx, s.r, []string{journal + "recent", journal + "last_gift"}, wire, rand.Text(), id, fmt.Sprintf(KeyRoomPubSub, room)).Err()
	if err != nil || time.Since(created) >= 10*time.Minute {
		return err
	}
	slug, _, _ := strings.Cut(room, "/")
	return heatIncrement.Run(ctx, s.r, []string{minuteKey(created), fmt.Sprintf("dm:{hot}:gift:%s:%d", room, id)}, slug, float64(cost)/10).Err()
}

type Contributor struct {
	UserID   uint64 `json:"user_id"`
	Nickname string `json:"nickname"`
	Total    int64  `json:"total_cost"`
}

func (s *Store) Contributors(ctx context.Context, room string, limit int) ([]Contributor, error) {
	prefix := fmt.Sprintf("dm:{%s}:", room)
	zs, err := s.r.ZRevRangeWithScores(ctx, prefix+"contributors", 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(zs))
	for i, z := range zs {
		ids[i] = z.Member.(string)
	}
	out := make([]Contributor, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	names, err := s.r.HMGet(ctx, prefix+"names", ids...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		uid, e := strconv.ParseUint(id, 10, 64)
		if e != nil {
			return nil, e
		}
		name, _ := names[i].(string)
		out[i] = Contributor{uid, name, int64(zs[i].Score)}
	}
	return out, nil
}

// Gateway-local detail caches share this stamp so metadata edits are visible across gateways.
func (s *Store) RoomRevision(ctx context.Context) (string, error) {
	rev, err := s.r.Get(ctx, "dm:{hot}:room_revision").Result()
	if err == redis.Nil {
		return "", nil
	}
	return rev, err
}

var bumpRevision = redis.NewScript(`
local previous=redis.call('GET',KEYS[1]) or ''
local token=string.match(previous,'^[^:]+') or ''
return redis.call('SET',KEYS[1],ARGV[1]..':'..token..':'..ARGV[2])`)

func (s *Store) BumpRoomRevision(ctx context.Context, slug string) error {
	return bumpRevision.Run(ctx, s.r, []string{"dm:{hot}:room_revision"}, rand.Text(), slug).Err()
}

// One room cursor is sufficient because only its pending head can advance.
// Keep the last result beyond the combo TTL for retries after a gateway crash.
var countCombo = redis.NewScript(`
local last=redis.call('HGET',KEYS[1],'id')
if last==ARGV[1] then return redis.call('HGET',KEYS[1],'count') end
if last and (#last>#ARGV[1] or (#last==#ARGV[1] and last>ARGV[1])) then return 0 end
local count=redis.call('INCRBY',KEYS[2],ARGV[2])
redis.call('PEXPIRE',KEYS[2],5000)
redis.call('HSET',KEYS[1],'id',ARGV[1],'count',count)
return count`)

func (s *Store) Combo(ctx context.Context, room, key string, id int64, qty int) (int, error) {
	prefix := fmt.Sprintf("dm:{%s}:combo:", room)
	return countCombo.Run(ctx, s.r, []string{prefix + "cursor", prefix + "value:" + key}, id, qty).Int()
}
