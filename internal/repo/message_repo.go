// Repository/DAO,Data access object
package repo

import (
	"context"
	"strconv"
	"time" //IsZero()

	"github.com/charlesAcmen/livestream-danmaku/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MessageRepo handles database operations for danmaku messages.
type MessageRepo struct {
	db *pgxpool.Pool
}

// NewMessageRepo creates a new repository instance.
func NewMessageRepo(db *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{db: db}
}

// SaveBatch inserts one partition batch atomically. The PostgreSQL driver encodes
// arrays as five bind parameters regardless of batch size.
func (r *MessageRepo) SaveBatch(ctx context.Context, msgs []*model.DanmakuMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]string, len(msgs))
	rooms := make([]string, len(msgs))
	users := make([]string, len(msgs))
	contents := make([]string, len(msgs))
	times := make([]time.Time, len(msgs))
	offsets := make([]*int64, len(msgs))
	for i, msg := range msgs {
		offsets[i] = msg.VideoOffsetMS
		ids[i] = msg.EventID
		rooms[i] = msg.RoomID
		// NUMERIC(20,0) also accepts user IDs above PostgreSQL's signed BIGINT range.
		users[i] = strconv.FormatUint(msg.UserID, 10)
		contents[i] = msg.Content
		times[i] = msg.SendTime
	}
	_, err := r.db.Exec(ctx, `INSERT INTO danmaku_messages (event_id, room_id, user_id, content, send_time, video_offset_ms)
 SELECT id, room, uid::numeric, body, sent, media_time
 FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[], $6::bigint[])
 AS batch(id, room, uid, body, sent, media_time)
 ON CONFLICT (event_id) DO NOTHING`, ids, rooms, users, contents, times, offsets)
	return err
}

// GetPlayBackDanmaku fetches messages for video playback
// (VOD video on demand mode).
// It retrieves messages that happened AFTER specific timestamp.
// queryTime: The current timestamp of the video player progress.
func (r *MessageRepo) GetPlayBackDanmaku(roomID string, queryTime time.Time, limit int) ([]*model.DanmakuMessage, error) {
	ctx := context.Background()
	query := `SELECT id,coalesce(event_id,''),room_id,user_id::text,content,send_time FROM danmaku_messages WHERE room_id=$1 AND NOT EXISTS (SELECT 1 FROM danmaku_withdrawals w WHERE w.room_id=danmaku_messages.room_id AND w.event_id=danmaku_messages.event_id)`
	if limit < 0 {
		limit = -1
	}
	args := []any{roomID, limit}
	if !queryTime.IsZero() {
		query += " AND send_time >= $3"
		args = append(args, queryTime)
	}
	rows, err := r.db.Query(ctx, query+" ORDER BY send_time,id LIMIT NULLIF($2::int,-1)", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs := make([]*model.DanmakuMessage, 0)
	for rows.Next() {
		var d model.DanmakuMessage
		var uid string
		if err := rows.Scan(&d.ID, &d.EventID, &d.RoomID, &uid, &d.Content, &d.SendTime); err != nil {
			return nil, err
		}
		d.UserID, err = strconv.ParseUint(uid, 10, 64)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, &d)
	}
	return msgs, rows.Err()
}

// Playback pages through a half-open media interval. The row id breaks ties at the same millisecond.
func (r *MessageRepo) Playback(ctx context.Context, room string, from, until, after int64) ([]model.DanmakuMessage, error) {
	rows, err := r.db.Query(ctx, `SELECT id,event_id,content,video_offset_ms,user_id::text,send_time FROM danmaku_messages m
 WHERE room_id=$1 AND (video_offset_ms,id)>($2,$3) AND video_offset_ms<$4
 AND NOT EXISTS (SELECT 1 FROM danmaku_withdrawals w WHERE w.room_id=m.room_id AND w.event_id=m.event_id)
 ORDER BY video_offset_ms,id LIMIT 100`, room, from, after, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.DanmakuMessage, 0)
	for rows.Next() {
		m := model.DanmakuMessage{RoomID: room}
		var uid string
		if err := rows.Scan(&m.ID, &m.EventID, &m.Content, &m.VideoOffsetMS, &uid, &m.SendTime); err != nil {
			return nil, err
		}
		m.UserID, err = strconv.ParseUint(uid, 10, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MessageRepo) Withdraw(ctx context.Context, room, id string) error {
	_, err := r.db.Exec(ctx, "INSERT INTO danmaku_withdrawals(room_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING", room, id)
	return err
}
