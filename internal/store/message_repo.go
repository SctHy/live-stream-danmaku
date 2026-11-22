package store

import (
	"context"
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
	times := make([]time.Time, len(msgs))
	for i, msg := range msgs {
		ids[i] = msg.EventID
		rooms[i] = msg.RoomID
		// NUMERIC(20,0) also accepts user IDs above PostgreSQL's signed BIGINT range.
		users[i] = strconv.FormatUint(msg.UserID, 10)
		contents[i] = msg.Content
		times[i] = msg.SendTime
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO danmaku_messages (event_id, room_id, user_id, content, send_time)
 SELECT id, room, uid::numeric, body, sent
 FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[])
 AS batch(id, room, uid, body, sent)
 ON CONFLICT (event_id) DO NOTHING`, ids, rooms, users, contents, times)
	return tag.RowsAffected(), err
}

func (r *MessageRepo) Withdraw(ctx context.Context, room, id string) error {
	_, err := r.db.Exec(ctx, "INSERT INTO danmaku_withdrawals(room_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING", room, id)
	return err
}
