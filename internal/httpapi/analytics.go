package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/scthy/maku/internal/pubsub"
)

type messageTotals struct{ count, senders, withdrawals, audits int64 }
type messageStatsReader interface {
	MessageStats(context.Context, string, string, string) (messageTotals, error)
}
type giftStatsReader interface {
	GiftStats(context.Context, string) (orders, cost int64, err error)
}
type attendanceReader interface {
	Attendance(context.Context, string) (int64, int64, error)
}
type contributionReader interface {
	Contributors(context.Context, string, int) ([]pubsub.Contributor, error)
}

type statsQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
type messageStats struct{ q statsQuery }

func (s messageStats) MessageStats(ctx context.Context, room, slug, session string) (messageTotals, error) {
	var v messageTotals
	err := s.q.QueryRow(ctx, `SELECT count(*) FILTER(WHERE w.event_id IS NULL),count(DISTINCT m.user_id) FILTER(WHERE w.event_id IS NULL),
 (SELECT count(*) FROM danmaku_withdrawals WHERE room_id=$1),
 (SELECT count(*) FROM moderation_audits WHERE room_id=$2 AND session_slug=$3)
 FROM danmaku_messages m LEFT JOIN danmaku_withdrawals w ON m.room_id=w.room_id AND m.event_id=w.event_id WHERE m.room_id=$1`, room, slug, session).Scan(&v.count, &v.senders, &v.withdrawals, &v.audits)
	return v, err
}

type giftStats struct{ q statsQuery }

func (s giftStats) GiftStats(ctx context.Context, room string) (orders, cost int64, err error) {
	err = s.q.QueryRow(ctx, `SELECT count(*),coalesce(sum(total_cost),0) FROM gift_orders WHERE room_id=$1`, room).Scan(&orders, &cost)
	return
}

type analyticsSources struct {
	messages      messageStatsReader
	gifts         giftStatsReader
	attendance    attendanceReader
	contributions contributionReader
}

func (s analyticsSources) totals(ctx context.Context, slug, session string) (map[string]any, error) {
	room := slug + "/" + session
	v, err := s.messages.MessageStats(ctx, room, slug, session)
	if err != nil {
		return nil, err
	}
	orders, cost, err := s.gifts.GiftStats(ctx, room)
	if err != nil {
		return nil, err
	}
	online, peak, err := s.attendance.Attendance(ctx, room)
	if err != nil {
		return nil, err
	}
	top, err := s.contributions.Contributors(ctx, room, 5)
	if err != nil {
		return nil, err
	}
	return map[string]any{"online": online, "peak_online": peak, "danmaku_count": v.count, "active_senders": v.senders, "withdrawn_count": v.withdrawals, "moderation_count": v.audits, "gift_order_count": orders, "gift_total_cost": cost, "top_contributors": top}, nil
}
func (h *ChatHandler) analytics(w http.ResponseWriter, r *http.Request) {
	a, err := h.loadAccessTx(r, bearer(r), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		dbError(w, err)
		return
	}
	ctx := r.Context()
	defer a.tx.Rollback(ctx)
	if a.role == "viewer" {
		w.WriteHeader(403)
		return
	}
	sources := analyticsSources{messages: messageStats{a.tx}, gifts: giftStats{a.tx}, attendance: h.sessions, contributions: h.sessions}
	totals, err := sources.totals(ctx, a.room.Slug, r.PathValue("session"))
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, totals)
}
