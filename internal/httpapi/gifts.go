package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/commerce"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/store"
)

func (h *ChatHandler) giftCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"gifts": h.plugins.Catalog.List()})
}
func (h *ChatHandler) wallet(w http.ResponseWriter, r *http.Request) {
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if r.Method == "POST" {
		claimed, balance, err := store.Checkin(ctx, db, login.UserID)
		if err != nil {
			dbError(w, err)
			return
		}
		amount := 0
		if claimed {
			amount = 500
		}
		writeJSON(w, map[string]any{"claimed": claimed, "amount": amount, "balance": balance})
		return
	}
	snapshot, err := store.ReadWallet(ctx, db, login.UserID)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, snapshot)
}

var orderID = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,64}$`)

func listLimit(r *http.Request, fallback, maximum int) (int, bool) {
	if !r.URL.Query().Has("limit") {
		return fallback, true
	}
	n, e := strconv.Atoi(r.URL.Query().Get("limit"))
	return n, e == nil && n >= 1 && n <= maximum
}
func (h *ChatHandler) gifts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	room := roomID(r)
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	if r.Method == "GET" {
		limit, ok := listLimit(r, 30, 50)
		if !ok {
			w.WriteHeader(400)
			return
		}
		orders, err := store.RecentGifts(ctx, db, room, limit, false)
		if err != nil {
			dbError(w, err)
			return
		}
		pins, err := store.RecentGifts(ctx, db, room, 0, true)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, map[string]any{"gifts": orders, "active_superchats": pins})
		return
	}
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}
	var in struct {
		ID       string `json:"client_order_id"`
		Code     string `json:"gift_code"`
		Quantity int
		Message  string
	}
	if !decode(w, r, &in) {
		return
	}
	item, ok := h.plugins.Catalog.Get(in.Code)
	if !ok || !orderID.MatchString(in.ID) {
		w.WriteHeader(400)
		return
	}
	effect := h.plugins.Effects[item.EffectType]
	if effect == nil {
		w.WriteHeader(503)
		return
	}
	key, err := effect.Prepare(item, commerce.GiftRequest{SenderID: login.UserID, Quantity: in.Quantity, Message: in.Message})
	if err != nil {
		w.WriteHeader(400)
		return
	}
	message, allowed := h.manager.FilterMessage(in.Message)
	if !allowed {
		w.WriteHeader(400)
		return
	}
	g, balance, duplicate, err := store.BuyGift(ctx, db, store.GiftOrder{EffectType: item.EffectType, ComboKey: key, ClientOrderID: in.ID, RoomID: room, RoomSlug: r.PathValue("room"), SenderID: login.UserID, GiftCode: item.Code, GiftName: item.Name, Quantity: in.Quantity, UnitPrice: item.Price, TotalCost: item.Price * int64(in.Quantity), Message: message}, item.Pin)
	if errors.Is(err, store.ErrInsufficientBalance) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "insufficient_balance"})
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if !duplicate {
		h.recordInterest(ctx, login.UserID, r.PathValue("room"), 5)
	}
	if err = h.flushGifts(ctx, db, g.RoomID); err != nil {
		dbError(w, err)
		return
	}
	if g.ComboKey != "" {
		if err = db.QueryRow(ctx, "SELECT combo_count FROM gift_orders WHERE id=$1", g.ID).Scan(&g.ComboCount); err != nil {
			dbError(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{"order": g, "balance": balance, "duplicate": duplicate})
}
func (h *ChatHandler) flushGifts(ctx context.Context, db *pgxpool.Pool, room string) error {
	return store.DeliverGifts(ctx, db, room, func(g store.GiftOrder) error {
		if g.ComboKey != "" {
			count, err := h.sessions.Combo(ctx, room, g.ComboKey, g.ID, g.Quantity)
			if err != nil {
				return err
			}
			// A newer order has already acknowledged this head.
			if count == 0 {
				return nil
			}
			g.ComboCount = count
			if _, err = db.Exec(ctx, "UPDATE gift_orders SET combo_count=$2 WHERE id=$1 AND pending", g.ID, count); err != nil {
				return err
			}
		}
		data, err := json.Marshal(g)
		if err != nil {
			return err
		}
		wire, err := json.Marshal(protocol.WsPacket{RoomID: room, Type: protocol.TypeGift, Data: data})
		if err != nil {
			return err
		}
		return h.sessions.Gift(ctx, room, wire, g.ID, g.SenderID, g.TotalCost, g.SenderName, g.CreatedAt)
	})
}
func (h *ChatHandler) leaderboard(w http.ResponseWriter, r *http.Request) {
	top, err := h.sessions.Contributors(r.Context(), roomID(r), 20)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, map[string]any{"contributors": top})
}
func (h *ChatHandler) analytics(w http.ResponseWriter, r *http.Request) {
	a, ok := h.access(w, r, bearer(r))
	if !ok {
		return
	}
	ctx := r.Context()
	defer a.tx.Rollback(ctx)
	if a.role == "viewer" {
		w.WriteHeader(403)
		return
	}
	room := roomID(r)
	var count, senders, withdrawals, audits, orders, cost int64
	err := a.tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE w.event_id IS NULL),count(DISTINCT m.user_id) FILTER(WHERE w.event_id IS NULL),
 (SELECT count(*) FROM danmaku_withdrawals WHERE room_id=$1),
 (SELECT count(*) FROM moderation_audits WHERE room_id=$2 AND session_slug=$3),
 (SELECT count(*) FROM gift_orders WHERE room_id=$1),(SELECT coalesce(sum(total_cost),0) FROM gift_orders WHERE room_id=$1)
 FROM danmaku_messages m LEFT JOIN danmaku_withdrawals w ON m.room_id=w.room_id AND m.event_id=w.event_id WHERE m.room_id=$1`, room, a.room.Slug, r.PathValue("session")).Scan(&count, &senders, &withdrawals, &audits, &orders, &cost)
	if err != nil {
		dbError(w, err)
		return
	}
	online, peak, err := h.sessions.Attendance(ctx, room)
	if err != nil {
		dbError(w, err)
		return
	}
	top, err := h.sessions.Contributors(ctx, room, 5)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, map[string]any{"online": online, "peak_online": peak, "danmaku_count": count, "active_senders": senders, "withdrawn_count": withdrawals, "moderation_count": audits, "gift_order_count": orders, "gift_total_cost": cost, "top_contributors": top})
}
