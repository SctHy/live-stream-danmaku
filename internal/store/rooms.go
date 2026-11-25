package store

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scthy/maku/internal/protocol"
)

type Room struct {
	Category      string    `json:"category"`
	Tags          []string  `json:"tags"`
	Slug          string    `json:"slug"`
	OwnerID       uint64    `json:"owner_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	ActiveSession string    `json:"active_session"`
	Bilibili      string    `json:"bilibili_room_id"`
	CreatedAt     time.Time `json:"created_at,omitzero"`
	UpdatedAt     time.Time `json:"updated_at,omitzero"`
	protocol.RoomControls
}

func LoadRoom(ctx context.Context, tx pgx.Tx, slug string, lock bool) (Room, error) {
	var v Room
	query := `SELECT slug,owner_id,title,description,active_session,bilibili_room_id,notice,status,slow_mode_seconds,mute_all,created_at,updated_at,category,tags FROM rooms WHERE slug=$1`
	if lock {
		query += " FOR UPDATE"
	}
	err := tx.QueryRow(ctx, query, slug).Scan(&v.Slug, &v.OwnerID, &v.Title, &v.Description, &v.ActiveSession, &v.Bilibili, &v.Notice, &v.Status, &v.Slow, &v.MuteAll, &v.CreatedAt, &v.UpdatedAt, &v.Category, &v.Tags)
	return v, err
}
func (v Room) Role(ctx context.Context, tx pgx.Tx, uid uint64) (string, error) {
	if uid == 0 {
		return "viewer", nil
	}
	if uid == v.OwnerID {
		return "owner", nil
	}
	var mod bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM room_moderators WHERE room_slug=$1 AND user_id=$2::text::numeric)", v.Slug, strconv.FormatUint(uid, 10)).Scan(&mod)
	if mod {
		return "moderator", err
	}
	return "viewer", err
}
func (v *Room) Save(ctx context.Context, tx pgx.Tx) error {
	return tx.QueryRow(ctx, `UPDATE rooms SET title=$2,description=$3,active_session=$4,bilibili_room_id=$5,notice=$6,status=$7,slow_mode_seconds=$8,mute_all=$9,category=$10,tags=$11,updated_at=now() WHERE slug=$1 RETURNING updated_at`, v.Slug, v.Title, v.Description, v.ActiveSession, v.Bilibili, v.Notice, v.Status, v.Slow, v.MuteAll, v.Category, v.Tags).Scan(&v.UpdatedAt)
}

type Audit struct {
	Session string    `json:"session,omitempty"`
	ID      int64     `json:"id"`
	Actor   uint64    `json:"actor_id"`
	Action  string    `json:"action"`
	Target  uint64    `json:"target_user_id"`
	Event   string    `json:"target_event_id"`
	Detail  string    `json:"detail"`
	Created time.Time `json:"created_at"`
}

func (a Audit) Write(ctx context.Context, tx pgx.Tx, room, session string) error {
	// Temporary rooms have no rooms row to lock. Serialize their audit sequence too.
	if session != "" {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 1605))", room); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO moderation_audits(room_id,actor_id,action,target_user_id,target_event_id,detail,session_slug,pending) VALUES($1,$2,$3,$4::text::numeric,$5,$6,$7,$7<>'')`, room, a.Actor, a.Action, strconv.FormatUint(a.Target, 10), a.Event, a.Detail, session)
	return err
}
func Audits(ctx context.Context, tx pgx.Tx, room string, before int64, limit int) ([]Audit, error) {
	rows, err := tx.Query(ctx, `SELECT session_slug,id,actor_id,action,target_user_id,target_event_id,detail,created_at FROM moderation_audits WHERE room_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT $3`, room, before, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Audit])
}
