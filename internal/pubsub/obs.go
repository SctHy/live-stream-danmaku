package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// OffsetMS resolves the media clock only for sessions backed by an OBS stream.
func (s *Store) OffsetMS(ctx context.Context, roomID string, at time.Time) (*int64, error) {
	room, session, _ := strings.Cut(roomID, "/")
	if !strings.HasPrefix(session, "obs-") {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	active, err := s.OBS(ctx, room)
	if err != nil || active.RoomID != roomID {
		return nil, err
	}
	offset := max(int64(0), at.UnixMilli()-active.Started)
	return &offset, nil
}
