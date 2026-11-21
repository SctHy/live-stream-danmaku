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
	for i, msg := range msgs {
		ids[i] = msg.EventID
		rooms[i] = msg.RoomID
		// NUMERIC(20,0) also accepts user IDs above PostgreSQL's signed BIGINT range.
		users[i] = strconv.FormatUint(msg.UserID, 10)
		contents[i] = msg.Content
		times[i] = msg.SendTime
	}
	_, err := r.db.Exec(ctx, `INSERT INTO danmaku_messages (event_id, room_id, user_id, content, send_time)
 SELECT id, room, uid::numeric, body, sent
 FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[])
 AS batch(id, room, uid, body, sent)
 ON CONFLICT (event_id) DO NOTHING`, ids, rooms, users, contents, times)
	return err
}

// GetPlayBackDanmaku fetches messages for video playback
// (VOD video on demand mode).
// It retrieves messages that happened AFTER specific timestamp.
// queryTime: The current timestamp of the video player progress.
func (r *MessageRepo) GetPlayBackDanmaku(roomID string, queryTime time.Time, limit int) ([]*model.DanmakuMessage, error) {
	ctx := context.Background()
	query := `SELECT id,coalesce(event_id,''),room_id,user_id::text,content,send_time FROM danmaku_messages WHERE room_id=$1`
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
