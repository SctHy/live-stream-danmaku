package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
)

// Sessions of a room share a cluster slot but have independent recovery streams.
func sessionPrefix(room string) string {
	slug, session, ok := strings.Cut(room, "/")
	prefix := "dm:{" + slug + "}:"
	if ok {
		return prefix + "session:" + session + ":"
	}
	return prefix
}

type Login struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	IsGuest  bool   `json:"is_guest"`
}

// The numeric prefix routes both keys to one slot. Only the random token authenticates.
func loginKeys(token string) (session, profile string, ok bool) {
	raw, secret, cut := strings.Cut(token, ".")
	uid, err := strconv.ParseUint(raw, 10, 64)
	if !cut || err != nil || uid == 0 || len(secret) < 26 || len(secret) > 80 {
		return "", "", false
	}
	prefix := "dm:login:{" + strconv.FormatUint(uid, 10) + "}:"
	return prefix + "token:" + secret, prefix + "nickname", true
}
func (s *Store) Login(ctx context.Context, token string) (Login, error) {
	var v Login
	key, profile, ok := loginKeys(token)
	if !ok {
		return v, redis.Nil
	}
	values, err := s.r.MGet(ctx, key, profile).Result()
	if err != nil {
		return v, err
	}
	raw, ok := values[0].(string)
	if !ok {
		return v, redis.Nil
	}
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return v, err
	}
	if name, ok := values[1].(string); ok {
		v.Nickname = name
	}
	return v, nil
}
func (s *Store) SaveLogin(ctx context.Context, token string, v Login, ttl time.Duration) error {
	key, _, ok := loginKeys(token)
	if !ok {
		return redis.Nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.r.Set(ctx, key, raw, ttl).Err()
}
func (s *Store) RefreshLogin(ctx context.Context, token string, v Login) error {
	_, profile, ok := loginKeys(token)
	if !ok {
		return redis.Nil
	}
	// All tokens issued before this rename expire within their original lifetime.
	return s.changeLogin(ctx, profile, profile, "rename", v.Nickname)
}
func (s *Store) Logout(ctx context.Context, token string) error {
	key, profile, ok := loginKeys(token)
	if !ok {
		return nil
	}
	return s.changeLogin(ctx, key, profile, "logout", "")
}

// Keep metadata and the single value read by the publish hot path in sync.
var saveControls = redis.NewScript(`
redis.call('HSET',KEYS[1],'notice',ARGV[1],'status',ARGV[2],'slow',ARGV[3],'mute_all',ARGV[4])
if ARGV[4]=='1' then redis.call('SET',KEYS[2],-1)
elseif ARGV[3]~='0' then redis.call('SET',KEYS[2],ARGV[3])
else redis.call('DEL',KEYS[2]) end
return 1`)

func (s *Store) SetControls(ctx context.Context, slug string, v protocol.RoomControls) error {
	p := sessionPrefix(slug)
	return saveControls.Run(ctx, s.r, []string{p + "controls", p + "gate"}, v.Notice, v.Status, v.Slow, v.MuteAll).Err()
}
func (s *Store) SetStaff(ctx context.Context, slug string, uid uint64, grant bool) error {
	key := sessionPrefix(slug) + "staff"
	if grant {
		return s.r.SAdd(ctx, key, uid).Err()
	}
	return s.r.SRem(ctx, key, uid).Err()
}

var setMute = redis.NewScript(`
if tonumber(ARGV[2])==0 then
 redis.call('DEL',KEYS[1])
 redis.call('ZREM',KEYS[2],ARGV[1])
else
 local now=redis.call('TIME')
 redis.call('SET',KEYS[1],'1','EX',ARGV[2])
 redis.call('ZADD',KEYS[2],(now[1]+ARGV[2])*1000+math.floor(now[2]/1000),ARGV[1])
 redis.call('EXPIRE',KEYS[2],86401)
end
return 1`)

func (s *Store) Mute(ctx context.Context, room string, uid uint64, ttl time.Duration) error {
	p := sessionPrefix(room)
	return setMute.Run(ctx, s.r, []string{p + "mute:" + fmt.Sprint(uid), p + "mutes"}, uid, int64(ttl/time.Second)).Err()
}

var listMutes = redis.NewScript(`
local now=redis.call('TIME')
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',now[1]*1000+math.floor(now[2]/1000))
return redis.call('ZRANGE',KEYS[1],0,-1,'WITHSCORES')`)

type MuteRecord struct {
	UserID    uint64 `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"`
}

func (s *Store) Mutes(ctx context.Context, room string) ([]MuteRecord, error) {
	vals, err := listMutes.Run(ctx, s.r, []string{sessionPrefix(room) + "mutes"}).StringSlice()
	out := make([]MuteRecord, 0, len(vals)/2)
	for i := 0; i < len(vals); i += 2 {
		uid, e := strconv.ParseUint(vals[i], 10, 64)
		if e != nil {
			return nil, e
		}
		until, e := strconv.ParseInt(vals[i+1], 10, 64)
		if e != nil {
			return nil, e
		}
		out = append(out, MuteRecord{uid, (until + 999) / 1000})
	}
	return out, err
}

// Cache leases share the account's slot. A completed profile edit or logout
// fences readers on other gateways even if a peer is unreachable.
const nicknameTTL = 2 * time.Second

var socketLogin = redis.NewScript(`
local ttl=redis.call('PTTL',KEYS[1])
if ttl==-2 then return false end
if ttl~=-1 and ttl<tonumber(ARGV[2]) then return {'','','short'} end
local reader=redis.call('GET',KEYS[3])
redis.call('SET',KEYS[3],(reader and reader~=ARGV[1]) and '*' or ARGV[1],'PX',ARGV[2])
return {redis.call('GET',KEYS[1]),redis.call('GET',KEYS[2]) or '', 'ok'}`)
var shortLogin = errors.New("session near expiration")

func (s *Store) SocketNickname(ctx context.Context, token string) (string, error) {
	key, profile, ok := loginKeys(token)
	if !ok {
		return "", redis.Nil
	}
	name, err := s.names.Load(key, nicknameTTL, func() (string, error) {
		values, err := socketLogin.Run(ctx, s.r, []string{key, profile, profile + ":readers"}, s.peer, nicknameTTL.Milliseconds()).StringSlice()
		if err != nil {
			return "", err
		}
		if values[2] == "short" {
			return "", shortLogin
		}
		if values[1] != "" {
			return values[1], nil
		}
		var login Login
		err = json.Unmarshal([]byte(values[0]), &login)
		return login.Nickname, err
	})
	if errors.Is(err, shortLogin) {
		v, err := s.Login(ctx, token)
		return v.Nickname, err
	}
	return name, err
}

var editLogin = redis.NewScript(`
if ARGV[1]=='rename' then redis.call('SET',KEYS[1],ARGV[2],'EX',604800)
else redis.call('DEL',KEYS[1]) end
if redis.call('GET',KEYS[2])~=ARGV[3] then return redis.call('PTTL',KEYS[2]) end
return 0`)

func (s *Store) changeLogin(ctx context.Context, key, profile, action, value string) error {
	wait, err := editLogin.Run(ctx, s.r, []string{key, profile + ":readers"}, action, value, s.peer).Int64()
	if action == "logout" {
		s.names.Invalidate(key)
	} else {
		prefix := strings.TrimSuffix(profile, "nickname") + "token:"
		s.names.InvalidateMatching(func(key string) bool { return strings.HasPrefix(key, prefix) })
	}
	if err != nil || wait <= 0 {
		return err
	}
	timer := time.NewTimer(time.Duration(wait+1) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
