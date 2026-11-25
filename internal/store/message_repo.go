package store

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/protocol"
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
func (r *MessageRepo) SaveBatch(ctx context.Context, msgs []*protocol.DanmakuMessage) (int64, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	ids := make([]string, len(msgs))
	rooms := make([]string, len(msgs))
	users := make([]string, len(msgs))
	contents := make([]string, len(msgs))
	names := make([]string, len(msgs))
	times := make([]time.Time, len(msgs))
	for i, msg := range msgs {
		ids[i] = msg.EventID
		rooms[i] = msg.RoomID
		// NUMERIC(20,0) also accepts user IDs above PostgreSQL's signed BIGINT range.
		users[i] = strconv.FormatUint(msg.UserID, 10)
		contents[i] = msg.Content
		names[i] = msg.Nickname
		times[i] = msg.SendTime
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO danmaku_messages (event_id, room_id, user_id, content, send_time,nickname)
 SELECT id, room, uid::numeric, body, sent,nickname
 FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[],$6::text[])
 AS batch(id, room, uid, body, sent,nickname)
 ON CONFLICT (event_id) DO NOTHING`, ids, rooms, users, contents, times, names)
	return tag.RowsAffected(), err
}

func (r *MessageRepo) Withdraw(ctx context.Context, room, id string) error {
	_, err := r.db.Exec(ctx, "INSERT INTO danmaku_withdrawals(room_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING", room, id)
	return err
}

func (r *MessageRepo) ListHistory(ctx context.Context, room string, beforeID uint64, limit int) ([]protocol.DanmakuMessage, bool, error) {
	if limit < 1 {
		limit = 50
	}
	limit = min(limit, 100)
	// Every BIGSERIAL ID is below a cursor outside the signed range.
	if beforeID > math.MaxInt64 {
		beforeID = 0
	}
	rows, err := r.db.Query(ctx, `SELECT m.id,coalesce(m.event_id,''),m.room_id,m.user_id::text,m.content,m.send_time,m.nickname
 FROM danmaku_messages m WHERE m.room_id=$1 AND ($2::bigint=0 OR m.id<$2)
 AND NOT EXISTS (SELECT 1 FROM danmaku_withdrawals w WHERE w.room_id=m.room_id AND w.event_id=m.event_id)
 ORDER BY m.id DESC LIMIT $3`, room, int64(beforeID), limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]protocol.DanmakuMessage, 0, limit+1)
	for rows.Next() {
		var m protocol.DanmakuMessage
		var uid string
		if err := rows.Scan(&m.ID, &m.EventID, &m.RoomID, &uid, &m.Content, &m.SendTime, &m.Nickname); err != nil {
			return nil, false, err
		}
		m.UserID, err = strconv.ParseUint(uid, 10, 64)
		if err != nil {
			return nil, false, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return out[:min(limit, len(out))], len(out) > limit, nil
}
