package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"fmt"
	"encoding/json"
	"io/fs"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/hub"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"github.com/scthy/live-stream-danmaku/internal/pubsub"
	"github.com/scthy/live-stream-danmaku/internal/store"
)

//go:embed all:web
var assets embed.FS

type ChatHandler struct {
	history  *store.MessageRepo
	manager  *hub.Hub
	sessions *pubsub.Store
	allowDev bool
}

func NewChatHandler(history *store.MessageRepo, m *hub.Hub, sessions *pubsub.Store, allowDev bool) *ChatHandler {
	return &ChatHandler{history, m, sessions, allowDev}
}

func (h *ChatHandler) Routes(mux *http.ServeMux) {
	web, _ := fs.Sub(assets, "web")
	mux.HandleFunc("GET /assets/protocol.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprintf(w, "export const danmaku=%d, stats=%d, like=%d, receipt=%d, withdraw=%d, ready=%d, migrate=%d;\n",
			protocol.TypeDanmaku, protocol.TypeStats, protocol.TypeLike, protocol.TypeReceipt,
			protocol.TypeWithdraw, protocol.TypeReady, protocol.TypeMigrate)
	})
	mux.Handle("GET /assets/", http.FileServerFS(web))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, assets, "web/index.html") })

	mux.HandleFunc("GET /api/live/playurl", liveSource)
	mux.HandleFunc("POST /api/session", h.session)
	slug := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,24}$`)
	for pattern, handler := range map[string]http.HandlerFunc{"GET /api/rooms/{room}/{session}/live": h.live, "GET /api/rooms/{room}/{session}/recent": h.recent, "GET /api/rooms/{room}/{session}/playback": h.playback, "POST /api/rooms/{room}/{session}/moderate": h.moderate} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if !slug.MatchString(r.PathValue("room")) || !slug.MatchString(r.PathValue("session")) {
				w.WriteHeader(400)
				return
			}
			handler(w, r)
		})
	}
}
func Recover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error("HTTP panic", "panic", v)
				http.Error(w, "internal error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func roomID(r *http.Request) string { return r.PathValue("room") + "/" + r.PathValue("session") }
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func query(r *http.Request, key, fallback string) string {
	if r.URL.Query().Has(key) {
		return r.URL.Query().Get(key)
	}
	return fallback
}
func loopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	return err == nil && net.ParseIP(host).IsLoopback()
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host) && (u.Scheme == "http" || u.Scheme == "https")
}
func (h *ChatHandler) identity(r *http.Request) (uint64, error) {
	cookie, err := r.Cookie("danmaku_guest")
	if err != nil || len(cookie.Value) < 26 || len(cookie.Value) > 80 {
		return 0, redis.Nil
	}
	return h.sessions.Guest(r.Context(), cookie.Value)
}
func (h *ChatHandler) session(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		w.WriteHeader(403)
		return
	}
	uid, err := h.identity(r)
	if err == redis.Nil {
		n, e := rand.Int(rand.Reader, big.NewInt(1<<52))
		if e != nil {
			w.WriteHeader(503)
			return
		}
		uid = n.Uint64() + 1
		token := rand.Text()
		ttl := 24 * time.Hour
		err = h.sessions.SaveGuest(r.Context(), token, uid, ttl)
		if err == nil {
			http.SetCookie(w, &http.Cookie{Name: "danmaku_guest", Value: token, Path: "/", MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
		}
	}
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"user_id": uid})
}
func (h *ChatHandler) live(w http.ResponseWriter, r *http.Request) {
	uid, err := h.identity(r)
	if err == redis.Nil && h.allowDev && loopback(r.RemoteAddr) && r.Header.Get("Origin") == "" {
		uid, err = strconv.ParseUint(r.URL.Query().Get("uid"), 10, 64)
	}
	if err != nil {
		w.WriteHeader(401)
		return
	}
	h.connect(w, r, uid, roomID(r))
}

func (h *ChatHandler) connect(w http.ResponseWriter, r *http.Request, uid uint64, room string) {
	if h.manager.Retiring() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	upgrader := websocket.Upgrader{Subprotocols: []string{protocol.BatchProtocol}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := h.manager.NewClient(uid, room, conn)

	if r.URL.Query().Get("migrate") == "1" {
		client.EnableMigration()
	}
	if !h.manager.Join(r.Context(), client) {
		client.Close()
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	ready := client.AwaitSubscription(ctx)
	cancel()
	if !ready {
		client.Close()
		h.manager.Leave(client)
		return
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	clock, _ := json.Marshal(protocol.ClockReady{ServerTimeMS: time.Now().UnixMilli()})
	if err := conn.WriteJSON(protocol.WsPacket{Type: protocol.TypeReady, Data: clock}); err != nil {
		client.Close()
		h.manager.Leave(client)
		return
	}

	go client.WritePump()
	client.ReadPump(h.manager)
}
func (h *ChatHandler) recent(w http.ResponseWriter, r *http.Request) {
	out, err := h.sessions.Recover(r.Context(), roomID(r), r.URL.Query().Get("after"))
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, out)
}
func (h *ChatHandler) playback(w http.ResponseWriter, r *http.Request) {
	from, e1 := strconv.ParseInt(query(r, "from", "0"), 10, 64)
	until, e2 := strconv.ParseInt(query(r, "until", "86400000"), 10, 64)
	after, e3 := strconv.ParseInt(query(r, "after", "0"), 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || from < 0 || until > 86400000 || from >= until || after < 0 {
		w.WriteHeader(400)
		return
	}
	rows, err := h.history.Playback(r.Context(), roomID(r), from, until, after)
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"events": rows})
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
