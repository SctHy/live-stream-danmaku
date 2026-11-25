package store

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Recharge struct {
	OrderNo  string     `json:"order_no"`
	ClientID string     `json:"client_order_id"`
	UserID   uint64     `json:"user_id"`
	Provider string     `json:"provider"`
	Package  string     `json:"package_code"`
	Amount   int64      `json:"amount_cents"`
	Coins    int64      `json:"coins"`
	Status   string     `json:"status"`
	Trade    *string    `json:"provider_trade_no"`
	Expires  time.Time  `json:"expires_at"`
	Paid     *time.Time `json:"paid_at"`
	Created  time.Time  `json:"created_at"`
	Updated  time.Time  `json:"updated_at"`
}

const rechargeCols = "order_no,client_order_id,user_id,provider,package_code,amount_cents,coins,CASE WHEN status='pending' AND expires_at<=clock_timestamp() THEN 'expired' ELSE status END,provider_trade_no,expires_at,paid_at,created_at,updated_at"

var ErrRechargeConflict = errors.New("conflicting recharge")

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func oneRecharge(ctx context.Context, q querier, sql string, args ...any) (Recharge, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return Recharge{}, err
	}
	return pgx.CollectOneRow(rows, pgx.RowToStructByPos[Recharge])
}
func CreateRecharge(ctx context.Context, db *pgxpool.Pool, r Recharge) (Recharge, error) {
	saved, err := oneRecharge(ctx, db, "INSERT INTO recharge_orders(order_no,client_order_id,user_id,provider,package_code,amount_cents,coins,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '15 min') ON CONFLICT(user_id,client_order_id) DO UPDATE SET client_order_id=excluded.client_order_id RETURNING "+rechargeCols, rand.Text(), r.ClientID, r.UserID, r.Provider, r.Package, r.Amount, r.Coins)
	if err == nil && (saved.Package != r.Package || saved.Provider != r.Provider) {
		err = ErrRechargeConflict
	}
	return saved, err
}
func UserRecharge(ctx context.Context, db *pgxpool.Pool, uid uint64, no string) (Recharge, error) {
	return oneRecharge(ctx, db, "SELECT "+rechargeCols+" FROM recharge_orders WHERE user_id=$1 AND order_no=$2", uid, no)
}
func walletRecharges(ctx context.Context, db querier, uid uint64) ([]Recharge, error) {
	rows, err := db.Query(ctx, "SELECT "+rechargeCols+" FROM recharge_orders WHERE user_id=$1 ORDER BY id DESC LIMIT 20", uid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Recharge])
}
func SettleRecharge(ctx context.Context, db *pgxpool.Pool, provider, no, action, trade string) (order Recharge, err error) {
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var e error
		order, e = oneRecharge(ctx, tx, "SELECT "+rechargeCols+" FROM recharge_orders WHERE provider=$1 AND order_no=$2 FOR UPDATE", provider, no)
		if e != nil {
			return e
		}
		switch action {
		case "close":
			if order.Status == "closed" {
				return nil
			}
		case "pay":
			if trade == "" {
				return ErrRechargeConflict
			}
			if order.Status == "paid" && order.Trade != nil && *order.Trade == trade {
				return nil
			}
		default:
			return ErrRechargeConflict
		}
		if order.Status != "pending" {
			return ErrRechargeConflict
		}
		status := "closed"
		var tradeNo *string
		if action == "pay" {
			status = "paid"
			tradeNo = &trade
		}
		order, e = oneRecharge(ctx, tx, "UPDATE recharge_orders SET status=$2,provider_trade_no=$3,paid_at=CASE WHEN $2='paid' THEN now() END,updated_at=now() WHERE order_no=$1 AND expires_at>clock_timestamp() RETURNING "+rechargeCols, no, status, tradeNo)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrRechargeConflict
		}
		if e != nil {
			return e
		}
		if status == "paid" {
			_, _, _, e = ApplyLedger(ctx, tx, order.UserID, "recharge", no, order.Coins, nil)
		}
		return e
	})
	return
}
