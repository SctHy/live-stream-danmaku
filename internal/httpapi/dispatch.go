package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/store"
)

// Pending audits are the durable handoff between PostgreSQL and Redis. Requests
// flush synchronously; the worker also delivers after a crash or Redis outage.
func (h *ChatHandler) RetryModeration(ctx context.Context) {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		db := h.db()
		if db == nil {
			continue
		}
		op, cancel := context.WithTimeout(ctx, 3*time.Second)
		rows, err := db.Query(op, "SELECT DISTINCT room_id FROM moderation_audits WHERE pending ORDER BY room_id")
		var rooms []string
		if err == nil {
			rooms, err = pgx.CollectRows(rows, pgx.RowTo[string])
		}
		cancel()
		for _, room := range rooms {
			op, cancel = context.WithTimeout(ctx, 3*time.Second)
			e := h.flush(op, db, room)
			cancel()
			if e != nil {
				err = e
			}
			if ctx.Err() != nil {
				return
			}
		}
		op, cancel = context.WithTimeout(ctx, 3*time.Second)
		giftRows, giftErr := db.Query(op, "SELECT DISTINCT room_id FROM gift_orders WHERE pending")
		var giftRooms []string
		if giftErr == nil {
			giftRooms, giftErr = pgx.CollectRows(giftRows, pgx.RowTo[string])
		}
		cancel()
		for _, room := range giftRooms {
			op, cancel = context.WithTimeout(ctx, 3*time.Second)
			giftErr = h.flushGifts(op, db, room)
			cancel()
			if giftErr != nil {
				break
			}
		}
		if giftErr != nil && ctx.Err() == nil {
			slog.Warn("Retry gift delivery", "error", giftErr)
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("Retry moderation delivery", "error", err)
		}
	}
}

type pendingAudit struct {
	ID              int64
	Session, Action string
	Target          uint64
	Event, Detail   string
	Created         time.Time
}

func (h *ChatHandler) flush(ctx context.Context, db *pgxpool.Pool, room string) error {
	for {
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			// A single dispatcher owns this room until the Redis acknowledgement is recorded.
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 1605))", room); err != nil {
				return err
			}
			var a pendingAudit
			err := tx.QueryRow(ctx, `SELECT id,session_slug,action,target_user_id,target_event_id,detail,created_at FROM moderation_audits WHERE room_id=$1 AND pending ORDER BY id LIMIT 1`, room).Scan(&a.ID, &a.Session, &a.Action, &a.Target, &a.Event, &a.Detail, &a.Created)
			if err != nil {
				return err
			}
			if err = h.deliver(ctx, room, a); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "UPDATE moderation_audits SET pending=false WHERE id=$1", a.ID)
			return err
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
func (h *ChatHandler) deliver(ctx context.Context, slug string, a pendingAudit) error {
	room := slug + "/" + a.Session
	var data any
	kind := protocol.TypeRoomState
	switch a.Action {
	case "obs_state":
		v, err := h.applyOBSState(ctx, slug, a.Detail)
		if err != nil {
			return err
		}
		data = v
	case "create_room":
		if err := h.sessions.SetControls(ctx, slug, protocol.RoomControls{Status: "live"}); err != nil {
			return err
		}
		return h.sessions.SetStaff(ctx, slug, a.Target, true)
	case "grant_moderator", "revoke_moderator":
		return h.sessions.SetStaff(ctx, slug, a.Target, a.Action == "grant_moderator")
	case "mute", "unmute":
		var ttl time.Duration
		if a.Action == "mute" {
			seconds, err := strconv.Atoi(a.Detail)
			if err != nil {
				return err
			}
			ttl = max(0, time.Until(a.Created.Add(time.Duration(seconds)*time.Second)).Round(time.Second))
		}
		return h.sessions.Mute(ctx, room, a.Target, ttl)
	case "withdraw":
		kind = protocol.TypeWithdraw
		data = protocol.Withdraw{EventID: a.Event}
	case "room_update", "mute_all", "notice", "slow_mode":
		var controls protocol.RoomControls
		if err := json.Unmarshal([]byte(a.Detail), &controls); err != nil {
			return err
		}
		if err := h.sessions.SetControls(ctx, slug, controls); err != nil {
			return err
		}
		data = controls
	default:
		return fmt.Errorf("unknown pending moderation action %q", a.Action)
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	wire, err := json.Marshal(protocol.WsPacket{Type: kind, RoomID: room, Data: body})
	if err != nil {
		return err
	}
	return h.sessions.JournalOnce(ctx, room, wire, a.ID)
}
func (h *ChatHandler) commitAudit(ctx context.Context, tx pgx.Tx, v store.Audit, room, session string) error {
	if err := v.Write(ctx, tx, room, session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	switch v.Action {
	case "withdraw":
		h.bus.Withdrawals.Publish(ctx, events.DanmakuWithdrawn{Room: room + "/" + session, EventID: v.Event})
	case "notice", "slow_mode", "mute_all":
		h.bus.Rooms.Publish(ctx, events.RoomStateChanged{Room: room})
	}
	if session == "" {
		return nil
	}
	db := h.db()
	if db == nil {
		return pgx.ErrTxClosed
	}
	return h.flush(ctx, db, room)
}
