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
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

//go:embed all:web
var assets embed.FS

type HistoryStore interface {
	ListHistory(context.Context, string, uint64, int) ([]protocol.DanmakuMessage, bool, error)
	Withdraw(context.Context, string, string) error
}
type sessionStore interface {
	RoomRevision(context.Context) (string, error)
	BumpRoomRevision(context.Context, string) error
	Activity(context.Context) (map[string]pubsub.Activity, error)
	Contributors(context.Context, string, int) ([]pubsub.Contributor, error)
	Attendance(context.Context, string) (int64, int64, error)
	Gift(context.Context, string, []byte, int64, uint64, int64, string, time.Time) error
	Login(context.Context, string) (pubsub.Login, error)
	SaveLogin(context.Context, string, pubsub.Login, time.Duration) error
	Logout(context.Context, string) error
	SetControls(context.Context, string, protocol.RoomControls) error
	SetStaff(context.Context, string, uint64, bool) error
	Mutes(context.Context, string) ([]pubsub.MuteRecord, error)
	Guest(context.Context, string) (uint64, error)
	SaveGuest(context.Context, string, uint64, time.Duration) error
	Mute(context.Context, string, uint64, time.Duration) error
	Recover(context.Context, string, string) (pubsub.Recovery, error)
	Journal(context.Context, string, []byte) error
	JournalOnce(context.Context, string, []byte, int64) error
	RefreshLogin(context.Context, string, pubsub.Login) error
}
type ChatHandler struct {
	liveResolver *liveResolver
	history      HistoryStore
	manager      *hub.Hub
	sessions     sessionStore
	allowDev     bool
	plaza        plazaCache
}

func NewChatHandler(history HistoryStore, m *hub.Hub, sessions sessionStore, allowDev bool) *ChatHandler {
	return &ChatHandler{liveResolver: newLiveResolver("https://api.live.bilibili.com", nil), history: history, manager: m, sessions: sessions, allowDev: allowDev}
}

func (h *ChatHandler) Routes(mux *http.ServeMux) {
	web, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("GET /assets/protocol.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprintf(w, "export const danmaku=%d, stats=%d, like=%d, receipt=%d, withdraw=%d, ready=%d, migrate=%d, liveDanmaku=%d, roomState=%d, gift=%d;\n",
			protocol.TypeDanmaku, protocol.TypeStats, protocol.TypeLike, protocol.TypeReceipt,
			protocol.TypeWithdraw, protocol.TypeReady, protocol.TypeMigrate, protocol.TypeLiveDanmaku, protocol.TypeRoomState, protocol.TypeGift)
	})
	mux.Handle("GET /assets/", http.FileServerFS(web))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, assets, "web/index.html")
	})

	mux.HandleFunc("GET /api/live/playurl", h.liveResolver.serve)
	mux.HandleFunc("POST /api/session", h.session)
	h.businessRoutes(mux)
	for pattern, handler := range map[string]http.HandlerFunc{"GET /api/rooms/{room}/{session}/history": h.listHistory, "GET /api/rooms/{room}/{session}/live": h.live, "GET /api/rooms/{room}/{session}/recent": h.recent, "POST /api/rooms/{room}/{session}/moderate": h.moderate} {
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
				log.Error("HTTP panic", "panic", v, "method", r.Method, "path", r.URL.Path, "stack", string(debug.Stack()))
				http.Error(w, "internal error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func roomID(r *http.Request) string { return r.PathValue("room") + "/" + r.PathValue("session") }
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("Write JSON response", "error", err)
	}
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
func (h *ChatHandler) guest(r *http.Request) (uint64, error) {
	cookie, err := r.Cookie("danmaku_guest")
	if err != nil || len(cookie.Value) < 26 || len(cookie.Value) > 80 {
		return 0, redis.Nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	return h.sessions.Guest(ctx, cookie.Value)
}
func (h *ChatHandler) session(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		w.WriteHeader(403)
		return
	}
	login, err := h.loginIdentity(r)
	uid := login.UserID
	if err == redis.Nil {
		uid, err = h.ensureGuest(w, r)
	}
	if err != nil {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"user_id": uid})
}
func (h *ChatHandler) ensureGuest(w http.ResponseWriter, r *http.Request) (uint64, error) {
	uid, err := h.guest(r)
	if err == redis.Nil {
		n, e := rand.Int(rand.Reader, big.NewInt(1<<52))
		if e != nil {
			return 0, e
		}
		uid = n.Uint64() + 1
		token := rand.Text()
		ttl := 24 * time.Hour
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		err = h.sessions.SaveGuest(ctx, token, uid, ttl)
		if err == nil {
			http.SetCookie(w, &http.Cookie{Name: "danmaku_guest", Value: token, Path: "/", MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
		}
	}
	return uid, err
}
func (h *ChatHandler) live(w http.ResponseWriter, r *http.Request) {
	login, err := h.loginIdentity(r)
	uid := login.UserID
	if err == redis.Nil {
		uid, err = h.guest(r)
	}
	if err == redis.Nil && h.allowDev && loopback(r.RemoteAddr) && r.Header.Get("Origin") == "" {
		uid, err = strconv.ParseUint(r.URL.Query().Get("uid"), 10, 64)
	}
	if err != nil {
		w.WriteHeader(401)
		return
	}
	h.connect(w, r, uid, roomID(r), login.Nickname)
}

func (h *ChatHandler) connect(w http.ResponseWriter, r *http.Request, uid uint64, room, nickname string) {
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
	if nickname != "" {
		cookie, _ := r.Cookie(loginCookie)
		token := cookie.Value
		client.LoginNickname = func(ctx context.Context) (string, error) {
			v, err := h.sessions.Login(ctx, token)
			return v.Nickname, err
		}
	}

	if r.URL.Query().Get("migrate") == "1" {
		client.EnableMigration()
	}
	if !h.manager.Join(r.Context(), client) {
		client.Close()
		return
	}
	defer h.manager.Leave(client)
	defer client.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	ready := client.AwaitSubscription(ctx)
	cancel()
	if !ready {
		return
	}

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return
	}
	clock, err := json.Marshal(protocol.ClockReady{ServerTimeMS: time.Now().UnixMilli()})
	if err != nil {
		return
	}
	if err := conn.WriteJSON(protocol.WsPacket{Type: protocol.TypeReady, Data: clock}); err != nil {
		return
	}

	go client.WritePump()
	client.ReadPump(h.manager)
}
