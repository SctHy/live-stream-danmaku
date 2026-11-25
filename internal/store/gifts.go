package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GiftOrder struct {
	ID            int64      `json:"id"`
	ClientOrderID string     `json:"client_order_id"`
	RoomID        string     `json:"room_id"`
	RoomSlug      string     `json:"room_slug"`
	SenderID      uint64     `json:"sender_id"`
	SenderName    string     `json:"sender_name"`
	GiftCode      string     `json:"gift_code"`
	GiftName      string     `json:"gift_name"`
	Quantity      int        `json:"quantity"`
	UnitPrice     int64      `json:"unit_price"`
	TotalCost     int64      `json:"total_cost"`
	Message       string     `json:"message"`
	PinnedUntil   *time.Time `json:"pinned_until"`
	CreatedAt     time.Time  `json:"created_at"`
}

const giftColumns = "id,client_order_id,room_id,room_slug,sender_id,sender_name,gift_code,gift_name,quantity,unit_price,total_cost,message,pinned_until,created_at"

var ErrInsufficientBalance = errors.New("insufficient_balance")

func Checkin(ctx context.Context, db *pgxpool.Pool, uid uint64) (claimed bool, balance int64, err error) {
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, "INSERT INTO daily_checkins(user_id,claim_date,amount) VALUES($1,CURRENT_DATE,500) ON CONFLICT DO NOTHING", uid)
		if e != nil {
			return e
		}
		claimed = tag.RowsAffected() == 1
		if claimed {
			return tx.QueryRow(ctx, "UPDATE users SET balance=balance+500 WHERE id=$1 RETURNING balance", uid).Scan(&balance)
		}
		return tx.QueryRow(ctx, "SELECT balance FROM users WHERE id=$1", uid).Scan(&balance)
	})
	return
}

func BuyGift(ctx context.Context, db *pgxpool.Pool, g GiftOrder, pin int) (order GiftOrder, balance int64, duplicate bool, err error) {
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// Serialize purchases for this wallet before inspecting its idempotency key.
		if e := tx.QueryRow(ctx, "SELECT balance,nickname FROM users WHERE id=$1 FOR UPDATE", g.SenderID).Scan(&balance, &g.SenderName); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, "SELECT "+giftColumns+" FROM gift_orders WHERE sender_id=$1 AND client_order_id=$2", g.SenderID, g.ClientOrderID)
		if e != nil {
			return e
		}
		order, e = pgx.CollectOneRow(rows, pgx.RowToStructByPos[GiftOrder])
		if e == nil {
			duplicate = true
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		e = tx.QueryRow(ctx, "UPDATE users SET balance=balance-$1 WHERE id=$2 AND balance >= $1 RETURNING balance", g.TotalCost, g.SenderID).Scan(&balance)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrInsufficientBalance
		}
		if e != nil {
			return e
		}
		// Order allocation and delivery share the lock so a later cursor cannot overtake a pending insert.
		if e = lockGiftRoom(ctx, tx, g.RoomID); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO gift_orders(client_order_id,room_id,room_slug,sender_id,sender_name,gift_code,gift_name,quantity,unit_price,total_cost,message,pinned_until)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now()+NULLIF($12::int,0)*interval '1 second') RETURNING id,created_at,pinned_until`, g.ClientOrderID, g.RoomID, g.RoomSlug, g.SenderID, g.SenderName, g.GiftCode, g.GiftName, g.Quantity, g.UnitPrice, g.TotalCost, g.Message, pin).Scan(&g.ID, &g.CreatedAt, &g.PinnedUntil)
		order = g
		return e
	})
	return
}
func lockGiftRoom(ctx context.Context, tx pgx.Tx, room string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,1606))", room)
	return err
}

func DeliverGifts(ctx context.Context, db *pgxpool.Pool, room string, send func(GiftOrder) error) error {
	for {
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			if err := lockGiftRoom(ctx, tx, room); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, "SELECT "+giftColumns+" FROM gift_orders WHERE room_id=$1 AND pending ORDER BY id LIMIT 1", room)
			if err != nil {
				return err
			}
			g, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[GiftOrder])
			if err != nil {
				return err
			}
			if err = send(g); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "UPDATE gift_orders SET pending=false WHERE id=$1", g.ID)
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
func RecentGifts(ctx context.Context, db *pgxpool.Pool, room string, limit int, active bool) ([]GiftOrder, error) {
	query := "SELECT " + giftColumns + " FROM gift_orders WHERE room_id=$1"
	args := []any{room}
	if active {
		query += " AND pinned_until>now() ORDER BY pinned_until DESC"
	} else {
		query += " ORDER BY id DESC LIMIT $2"
		args = append(args, limit)
	}
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[GiftOrder])
}
func WalletOrders(ctx context.Context, db *pgxpool.Pool, uid uint64) ([]GiftOrder, error) {
	rows, err := db.Query(ctx, "SELECT "+giftColumns+" FROM gift_orders WHERE sender_id=$1 ORDER BY id DESC LIMIT 20", uid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[GiftOrder])
}
