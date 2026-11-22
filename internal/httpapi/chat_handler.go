package httpapi

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"

	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
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
	for pattern, handler := range map[string]http.HandlerFunc{"GET /api/rooms/{room}/{session}/live": h.live, "GET /api/rooms/{room}/{session}/recent": h.recent, "POST /api/rooms/{room}/{session}/moderate": h.moderate} {
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
