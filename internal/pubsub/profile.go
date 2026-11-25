package pubsub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/recommend"
)

func profileKeys(uid uint64) []string {
	prefix := fmt.Sprintf("dm:{u:%d}:", uid)
	return []string{prefix + "pref:cat", prefix + "pref:tag", prefix + "recent_rooms"}
}

var updateProfile = redis.NewScript(`
if ARGV[3]~='' then redis.call('ZINCRBY',KEYS[1],ARGV[2],ARGV[3]) end
for i=5,#ARGV do redis.call('ZINCRBY',KEYS[2],ARGV[2],ARGV[i]) end
redis.call('ZREMRANGEBYRANK',KEYS[2],0,-33)
redis.call('ZADD',KEYS[3],'GT',ARGV[4],ARGV[1])
redis.call('ZREMRANGEBYRANK',KEYS[3],0,-31)
for _,key in ipairs(KEYS) do redis.call('EXPIRE',key,604800) end
return 1`)

type InterestDelta struct {
	UserID         uint64
	Room, Category string
	Tags           []string
	Weight         int
	At             time.Time
}

func (d InterestDelta) args() []any {
	args := []any{d.Room, d.Weight, d.Category, d.At.UnixMilli()}
	for _, tag := range d.Tags {
		args = append(args, tag)
	}
	return args
}

// Each script operates on a single user's hash slot. Retry only commands
// Redis explicitly rejected with NOSCRIPT, never ambiguous increments.
func (s *Store) RecordInterests(ctx context.Context, batch []InterestDelta) error {
	p := s.r.Pipeline()
	cmds := make([]*redis.Cmd, len(batch))
	for i, d := range batch {
		cmds[i] = updateProfile.EvalSha(ctx, p, profileKeys(d.UserID), d.args()...)
	}
	if _, err := p.Exec(ctx); err == nil {
		return nil
	}
	var firstErr error
	retry := s.r.Pipeline()
	for i, cmd := range cmds {
		err := cmd.Err()
		if redis.HasErrorPrefix(err, "NOSCRIPT") {
			d := batch[i]
			updateProfile.Eval(ctx, retry, profileKeys(d.UserID), d.args()...)
		} else if firstErr == nil {
			firstErr = err
		}
	}
	_, err := retry.Exec(ctx)
	return errors.Join(firstErr, err)
}
func (s *Store) Preferences(ctx context.Context, uid uint64) (recommend.Profile, error) {
	keys := profileKeys(uid)
	p := s.r.Pipeline()
	read := func(k string) *redis.ZSliceCmd { return p.ZRevRangeWithScores(ctx, k, 0, -1) }
	cats, tags, recent := read(keys[0]), read(keys[1]), read(keys[2])
	if _, err := p.Exec(ctx); err != nil {
		return recommend.Profile{}, err
	}
	scores := func(cmd *redis.ZSliceCmd) map[string]float64 {
		out := make(map[string]float64, len(cmd.Val()))
		for _, z := range cmd.Val() {
			out[z.Member.(string)] = z.Score
		}
		return out
	}
	return recommend.Profile{Categories: scores(cats), Tags: scores(tags), Recent: scores(recent)}, nil
}
