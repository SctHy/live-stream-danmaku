package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerEntry struct {
	ID        int64           `json:"id"`
	UserID    uint64          `json:"user_id"`
	BizType   string          `json:"biz_type"`
	BizID     string          `json:"biz_id"`
	Delta     int64           `json:"delta"`
	Before    int64           `json:"balance_before"`
	After     int64           `json:"balance_after"`
	Meta      json.RawMessage `json:"meta"`
	CreatedAt time.Time       `json:"created_at"`
}

const ledgerColumns = "id,user_id,biz_type,biz_id,delta,balance_before,balance_after,meta,created_at"

func ApplyLedger(ctx context.Context, tx pgx.Tx, uid uint64, kind, id string, delta int64, meta json.RawMessage) (entry LedgerEntry, balance int64, duplicate bool, err error) {
	err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id=$1 FOR UPDATE", uid).Scan(&balance)
	if err != nil {
		return
	}
	rows, err := tx.Query(ctx, "SELECT "+ledgerColumns+" FROM wallet_ledger WHERE user_id=$1 AND biz_type=$2 AND biz_id=$3", uid, kind, id)
	if err != nil {
		return
	}
	entry, err = pgx.CollectOneRow(rows, pgx.RowToStructByPos[LedgerEntry])
	if err == nil {
		return entry, balance, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if delta < -balance {
		return entry, balance, false, ErrInsufficientBalance
	}
	if delta == 0 || delta > math.MaxInt64-balance {
		return entry, balance, false, errors.New("invalid wallet delta")
	}
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	rows, err = tx.Query(ctx, "INSERT INTO wallet_ledger(user_id,biz_type,biz_id,delta,balance_before,balance_after,meta) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING "+ledgerColumns, uid, kind, id, delta, balance, balance+delta, meta)
	if err != nil {
		return
	}
	entry, err = pgx.CollectOneRow(rows, pgx.RowToStructByPos[LedgerEntry])
	if err != nil {
		return
	}
	_, err = tx.Exec(ctx, "UPDATE users SET balance=$2 WHERE id=$1", uid, entry.After)
	return entry, entry.After, false, err
}

func walletLedger(ctx context.Context, db querier, uid uint64) ([]LedgerEntry, error) {
	rows, err := db.Query(ctx, "SELECT "+ledgerColumns+" FROM wallet_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT 30", uid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[LedgerEntry])
}

type WalletSnapshot struct {
	Earnings  int64         `json:"earnings"`
	Balance   int64         `json:"balance"`
	Checked   bool          `json:"checked_in_today"`
	Orders    []GiftOrder   `json:"orders"`
	Ledger    []LedgerEntry `json:"ledger"`
	Recharges []Recharge    `json:"recharges"`
}

func ReadWallet(ctx context.Context, db *pgxpool.Pool, uid uint64) (wallet WalletSnapshot, err error) {
	err = pgx.BeginTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, "SELECT earnings,balance,EXISTS(SELECT FROM daily_checkins WHERE user_id=$1 AND claim_date=CURRENT_DATE) FROM users WHERE id=$1", uid).Scan(&wallet.Earnings, &wallet.Balance, &wallet.Checked)
		if err != nil {
			return err
		}
		wallet.Orders, err = WalletOrders(ctx, tx, uid)
		if err != nil {
			return err
		}
		wallet.Ledger, err = walletLedger(ctx, tx, uid)
		if err != nil {
			return err
		}
		wallet.Recharges, err = walletRecharges(ctx, tx, uid)
		return err
	})
	return
}
