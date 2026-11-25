package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/scthy/maku/internal/commerce"
	"github.com/scthy/maku/internal/store"
)

func (h *ChatHandler) packages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"packages": commerce.Packages(), "providers": h.plugins.Providers.Names()})
}
func (h *ChatHandler) recharge(w http.ResponseWriter, r *http.Request) {
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
	if no := r.PathValue("order_no"); no != "" {
		order, err := store.UserRecharge(ctx, db, login.UserID, no)
		if err != nil {
			dbError(w, err)
			return
		}
		if r.Method == "GET" {
			writeJSON(w, map[string]any{"order": order})
			return
		}
		sandbox, ok := h.plugins.Providers[order.Provider].(*commerce.Sandbox)
		if !ok {
			w.WriteHeader(400)
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.Action != "pay" && input.Action != "close" {
			w.WriteHeader(400)
			return
		}
		body, err := json.Marshal(commerce.WebhookEvent{OrderNo: no, Trade: "sandbox-" + no, Action: input.Action})
		if err != nil {
			dbError(w, err)
			return
		}
		h.processWebhook(w, r, sandbox, sandbox.Sign(body), body)
		return
	}
	var input struct {
		ClientID string `json:"client_order_id"`
		Package  string `json:"package_code"`
		Provider string `json:"provider"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Provider == "" {
		input.Provider = "sandbox"
	}
	provider := h.plugins.Providers[input.Provider]
	if !orderID.MatchString(input.ClientID) || provider == nil || input.Provider == "voucher" {
		w.WriteHeader(400)
		return
	}
	for _, p := range commerce.Packages() {
		if input.Package != p.Code {
			continue
		}
		order, err := store.CreateRecharge(ctx, db, store.Recharge{ClientID: input.ClientID, UserID: login.UserID, Provider: provider.Name(), Package: p.Code, Amount: p.Amount, Coins: p.Coins})
		if err != nil {
			paymentError(w, err)
			return
		}
		checkout, err := provider.Create(ctx, order)
		if err != nil {
			dbError(w, err)
			return
		}
		writeJSON(w, map[string]any{"order": order, "checkout": checkout})
		return
	}
	w.WriteHeader(400)
}
func paymentError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrRechargeConflict) {
		w.WriteHeader(409)
		return
	}
	dbError(w, err)
}
func (h *ChatHandler) webhook(w http.ResponseWriter, r *http.Request) {
	if h.db() == nil {
		w.WriteHeader(503)
		return
	}
	provider := h.plugins.Providers[r.PathValue("provider")]
	if provider == nil {
		w.WriteHeader(404)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	h.processWebhook(w, r, provider, r.Header, body)
}
func (h *ChatHandler) processWebhook(w http.ResponseWriter, r *http.Request, p commerce.PaymentProvider, headers http.Header, body []byte) {
	e, err := p.VerifyWebhook(headers, body)
	if err != nil {
		w.WriteHeader(403)
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	order, err := store.SettleRecharge(r.Context(), db, p.Name(), e.OrderNo, e.Action, e.Trade)
	if err != nil {
		paymentError(w, err)
		return
	}
	writeJSON(w, map[string]any{"order": order})
}
func (h *ChatHandler) redeem(w http.ResponseWriter, r *http.Request) {
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	p, ok := h.plugins.Providers["voucher"].(interface{ Redeem(string) (int64, bool) })
	if !ok {
		w.WriteHeader(503)
		return
	}
	coins, ok := p.Redeem(in.Code)
	if !ok {
		w.WriteHeader(400)
		return
	}
	ctx := r.Context()
	var balance int64
	var duplicate bool
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var err error
		_, balance, duplicate, err = store.ApplyLedger(ctx, tx, login.UserID, "voucher", in.Code, coins, nil)
		return err
	})
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, map[string]any{"balance": balance, "duplicate": duplicate})
}
