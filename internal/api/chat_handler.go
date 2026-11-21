package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/infra"
	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"github.com/charlesAcmen/livestream-danmaku/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

//go:embed web/*
var assets embed.FS
var slug = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,24}$`)
var upgrader = websocket.Upgrader{Subprotocols: []string{model.BatchProtocol}}

type historyStore interface {
	Playback(ctx context.Context, room string, from, until, after int64) ([]model.DanmakuMessage, error)
	Withdraw(ctx context.Context, room, eventID string) error
}

type ChatHandler struct {
	history historyStore
	manager *ws.Manager
}

func NewChatHandler(history historyStore, m *ws.Manager) *ChatHandler {
	return &ChatHandler{history, m}
}

func (h *ChatHandler) Routes(r *gin.Engine) {
	web, _ := fs.Sub(assets, "web")
	r.StaticFS("/assets", http.FS(web))
	index, err := fs.ReadFile(web, "index.html")
	if err != nil {
		panic(err)
	}
	r.GET("/", func(c *gin.Context) { c.Data(200, "text/html; charset=utf-8", index) })
	r.GET("/ws", h.ConnectWebSocket)
	r.POST("/api/session", h.session)
	room := r.Group("/api/rooms/:room/:session", func(c *gin.Context) {
		a, b := c.Param("room"), c.Param("session")
		if !slug.MatchString(a) || !slug.MatchString(b) {
			c.AbortWithStatus(400)
			return
		}
		c.Set("room", a+"/"+b)
	})
	room.GET("/live", h.live)
	room.GET("/recent", h.recent)
	room.GET("/playback", h.playback)
	room.POST("/moderate", h.moderate)
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
func (h *ChatHandler) identity(c *gin.Context) (uint64, error) {
	token, err := c.Cookie("danmaku_guest")
	if err != nil || len(token) < 26 || len(token) > 80 {
		return 0, redis.Nil
	}
	return h.manager.RedisClient.Get(c.Request.Context(), "dm:guest:"+token).Uint64()
}
func (h *ChatHandler) session(c *gin.Context) {
	if !sameOrigin(c.Request) {
		c.Status(403)
		return
	}
	uid, err := h.identity(c)
	if err == redis.Nil {
		n, e := rand.Int(rand.Reader, big.NewInt(1<<52))
		if e != nil {
			c.Status(503)
			return
		}
		uid = n.Uint64() + 1
		token := rand.Text()
		ttl := 24 * time.Hour
		err = h.manager.RedisClient.Set(c.Request.Context(), "dm:guest:"+token, uid, ttl).Err()
		if err == nil {
			http.SetCookie(c.Writer, &http.Cookie{Name: "danmaku_guest", Value: token, Path: "/", MaxAge: int(ttl.Seconds()), HttpOnly: true, Secure: c.Request.TLS != nil, SameSite: http.SameSiteStrictMode})
		}
	}
	if err != nil {
		c.Status(503)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"user_id": uid})
}
func (h *ChatHandler) live(c *gin.Context) {
	uid, err := h.identity(c)
	if err != nil {
		c.Status(401)
		return
	}
	h.connect(c, uid, c.GetString("room"), true)
}

// The original CLI and load generator use a loopback-only, separate room namespace.
func (h *ChatHandler) ConnectWebSocket(c *gin.Context) {
	room := c.Query("room")
	uid, err := strconv.ParseUint(c.Query("uid"), 10, 64)
	if !loopback(c.Request.RemoteAddr) || c.GetHeader("Origin") != "" {
		c.Status(403)
		return
	}
	if err != nil || room == "" || strings.ContainsAny(room, "/\x00") || len(room) > 50 {
		c.Status(400)
		return
	}
	h.connect(c, uid, room, false)
}
func (h *ChatHandler) connect(c *gin.Context, uid uint64, room string, session bool) {
	if h.manager.Retiring() {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := h.manager.NewClient(uid, c.Query("name"), room, conn)
	client.Session = session
	if session && c.Query("migrate") == "1" {
		client.EnableMigration()
	}
	if !h.manager.Join(c.Request.Context(), client) {
		client.Close()
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
	ready := client.AwaitSubscription(ctx)
	cancel()
	if !ready {
		client.Close()
		h.manager.Leave(client)
		return
	}
	if session {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		clock, _ := json.Marshal(map[string]int64{"server_time_ms": time.Now().UnixMilli()})
		if err := conn.WriteJSON(model.WsPacket{Type: 106, Data: clock}); err != nil {
			client.Close()
			h.manager.Leave(client)
			return
		}
	}
	go client.WritePump()
	client.ReadPump(h.manager)
}
func (h *ChatHandler) recent(c *gin.Context) {
	out, err := infra.Recover(c.Request.Context(), h.manager.RedisClient, c.GetString("room"), c.Query("after"))
	if err != nil {
		c.Status(503)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, out)
}
func (h *ChatHandler) playback(c *gin.Context) {
	from, e1 := strconv.ParseInt(c.DefaultQuery("from", "0"), 10, 64)
	until, e2 := strconv.ParseInt(c.DefaultQuery("until", "86400000"), 10, 64)
	after, e3 := strconv.ParseInt(c.DefaultQuery("after", "0"), 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || from < 0 || until > 86400000 || from >= until || after < 0 {
		c.Status(400)
		return
	}
	rows, err := h.history.Playback(c.Request.Context(), c.GetString("room"), from, until, after)
	if err != nil {
		c.Status(503)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"events": rows})
}
func (h *ChatHandler) moderate(c *gin.Context) {
	key := os.Getenv("DANMAKU_MOD_TOKEN")
	room := c.GetString("room")
	if !sameOrigin(c.Request) || key == "" || os.Getenv("DANMAKU_MOD_ROOM") != c.Param("room") || subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+key)) != 1 {
		c.Status(403)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	var in struct {
		Action  string
		EventID string `json:"event_id"`
		UserID  uint64 `json:"user_id"`
		Seconds int
	}
	if c.ShouldBindJSON(&in) != nil {
		c.Status(400)
		return
	}
	ctx := c.Request.Context()
	var err error
	switch in.Action {
	case "mute":
		if in.UserID == 0 || in.Seconds < 1 || in.Seconds > 86400 {
			c.Status(400)
			return
		}
		err = h.manager.RedisClient.Set(ctx, "dm:mute:"+room+":"+strconv.FormatUint(in.UserID, 10), 1, time.Duration(in.Seconds)*time.Second).Err()
	case "withdraw":
		if len(in.EventID) < 26 || len(in.EventID) > 80 {
			c.Status(400)
			return
		}
		err = h.history.Withdraw(ctx, room, in.EventID)
		if err == nil {
			data, _ := json.Marshal(map[string]string{"event_id": in.EventID})
			packet, _ := json.Marshal(model.WsPacket{Type: 105, RoomID: room, Data: data})
			err = infra.Journal(ctx, h.manager.RedisClient, room, packet)
		}
	default:
		c.Status(400)
		return
	}
	if err != nil {
		c.Status(503)
		return
	}
	c.Status(204)
}
