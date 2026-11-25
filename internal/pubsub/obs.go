package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type OBSStream struct {
	ReplayID int64  `json:"replay_id"`
	RoomID   string `json:"room_id"`
	Session  string `json:"session_id"`
	Started  int64  `json:"started_at_ms"`
	HLS      string `json:"hls_url"`
}

func (s *Store) OBS(ctx context.Context, room string) (OBSStream, error) {
	return s.obs.Load(room, 2*time.Second, func() (OBSStream, error) {
		var v OBSStream
		raw, err := s.r.Get(ctx, sessionPrefix(room)+"obs").Bytes()
		if errors.Is(err, redis.Nil) {
			return v, nil
		}
		if err == nil {
			err = json.Unmarshal(raw, &v)
		}
		return v, err
	})
}
func (s *Store) InvalidateOBS(room string) { s.obs.Invalidate(room) }
func (s *Store) SetOBS(ctx context.Context, room string, v OBSStream) error {
	defer s.obs.Invalidate(room)
	key := sessionPrefix(room) + "obs"
	if v.ReplayID == 0 {
		return s.r.Del(ctx, key).Err()
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.r.Set(ctx, key, raw, 0).Err()
}
