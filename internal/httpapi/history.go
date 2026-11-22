package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

func (h *ChatHandler) recent(w http.ResponseWriter, r *http.Request) {
	out, err := h.sessions.Recover(r.Context(), roomID(r), r.URL.Query().Get("after"))
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, out)
}
func (h *ChatHandler) moderate(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("DANMAKU_MOD_TOKEN")
	room := roomID(r)
	if !sameOrigin(r) || key == "" || os.Getenv("DANMAKU_MOD_ROOM") != r.PathValue("room") || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) != 1 {
		w.WriteHeader(403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var in struct {
		Action  string
		EventID string `json:"event_id"`
		UserID  uint64 `json:"user_id"`
		Seconds int
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		w.WriteHeader(400)
		return
	}
	ctx := r.Context()
	var err error
	switch in.Action {
	case "mute":
		if in.UserID == 0 || in.Seconds < 1 || in.Seconds > 86400 {
			w.WriteHeader(400)
			return
		}
		err = h.sessions.Mute(ctx, room, in.UserID, time.Duration(in.Seconds)*time.Second)
	case "withdraw":
		if len(in.EventID) < 26 || len(in.EventID) > 80 {
			w.WriteHeader(400)
			return
		}
		err = h.history.Withdraw(ctx, room, in.EventID)
		if err == nil {
			data, _ := json.Marshal(protocol.Withdraw{EventID: in.EventID})
			packet, _ := json.Marshal(protocol.WsPacket{Type: protocol.TypeWithdraw, RoomID: room, Data: data})
			err = h.sessions.Journal(ctx, room, packet)
		}
	default:
		w.WriteHeader(400)
		return
	}
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.WriteHeader(204)
}
