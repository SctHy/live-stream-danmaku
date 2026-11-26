package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/hub"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

func accountServer(t *testing.T, run func(*testing.T, context.Context, *gatewayHistory, *historyPool, *pubsub.Store, *http.ServeMux, *httpapi.ChatHandler, *httptest.Server), observe ...func(*events.Bus)) {
	t.Helper()
	dsn, addr := os.Getenv("DANMAKU_TEST_DATABASE_URL"), os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if dsn == "" || addr == "" {
		t.Skip("requires PostgreSQL and Redis")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := pgx.Identifier{"phase_a_" + rand.Text()}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = store.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	h := new(gatewayHistory)
	pool := &historyPool{db: db, HistoryStore: store.NewMessageRepo(db)}
	h.current.Store(pool)
	rs := pubsub.Open(addr)
	defer rs.Close()
	producer, e := mq.NewProducer(mq.Brokers(os.Getenv("DANMAKU_TEST_KAFKA_BROKERS")), slog.Default(), nil)
	if e != nil {
		t.Fatal(e)
	}
	m := hub.New(hub.Config{Filter: filter.New([]string{"replay-blocked-word"}), Producer: producer, Store: rs, RoomShards: 1, PubSubShards: 1})
	for _, subscribe := range observe {
		subscribe(m.Events())
	}
	go m.Start()
	defer func() { drain, stop := context.WithTimeout(t.Context(), time.Second); defer stop(); m.Drain(drain) }()
	mux := http.NewServeMux()
	handler := httpapi.NewChatHandler(h, m, rs, false)
	handler.Routes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	run(t, ctx, h, pool, rs, mux, handler, server)
}
func TestAccountRoomLifecycle(t *testing.T) {
	accountServer(t, checkAccountRoomLifecycle)
}

func checkAccountRoomLifecycle(t *testing.T, ctx context.Context, h *gatewayHistory, pool *historyPool, rs *pubsub.Store, mux *http.ServeMux, handler *httpapi.ChatHandler, server *httptest.Server) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	type response struct {
		body   map[string]json.RawMessage
		cookie *http.Cookie
	}
	call := func(method, path, body string, c *http.Cookie, want int) response {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, want, w.Body)
		}
		var v response
		if w.Body.Len() > 0 {
			if e := json.Unmarshal(w.Body.Bytes(), &v.body); e != nil {
				t.Fatal(e)
			}
		}
		cookies := w.Result().Cookies()
		if len(cookies) > 0 {
			v.cookie = cookies[0]
		}
		return v
	}
	reg := func(name string) response {
		return call("POST", "/api/auth/register", fmt.Sprintf(`{"username":%q,"password":"secret123"}`, name), nil, 200)
	}
	owner, mod, viewer := reg("owner"), reg("mod"), reg("viewer")
	if owner.cookie == nil || !owner.cookie.HttpOnly || owner.cookie.SameSite != http.SameSiteStrictMode || string(owner.body["balance"]) != "1000" {
		t.Fatal("register session")
	}
	call("POST", "/api/auth/login", `{"username":"owner","password":"invalid"}`, nil, 401)
	owner.cookie = call("POST", "/api/auth/login", `{"username":"owner","password":"secret123"}`, nil, 200).cookie
	call("PATCH", "/api/auth/me", `{"nickname":"测试房主"}`, owner.cookie, 200)
	slug := "room-" + rand.Text()[:14]
	base := "/api/rooms/" + slug
	session := base + "/live"
	call("POST", "/api/rooms", fmt.Sprintf(`{"slug":%q,"title":"直播间"}`, slug), owner.cookie, 201)
	call("PATCH", base, `{"mute_all":true,"notice":"公告"}`, viewer.cookie, 403)
	// A failed Redis update must leave committed settings available for automatic delivery.
	control := redis.NewClient(&redis.Options{Addr: addr})
	defer control.Close()
	key := "dm:{" + slug + "}:controls"
	if err := control.Set(ctx, key, "wrong type", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	call("PATCH", base, `{"notice":"重试"}`, owner.cookie, 503)
	if err := control.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	workerCtx, stopWorker := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); handler.RetryModeration(workerCtx) }()
	defer func() { stopWorker(); <-done }()
	for start := time.Now(); ; {
		if notice, _ := control.HGet(ctx, key, "notice").Result(); notice == "重试" {
			break
		}
		if time.Since(start) > 6*time.Second {
			t.Fatal("pending control was not reapplied")
		}
		time.Sleep(50 * time.Millisecond)
	}
	call("PATCH", base, `{"mute_all":true,"notice":"公告"}`, owner.cookie, 200)
	call("POST", base+"/moderators", `{"username":"mod"}`, owner.cookie, 204)
	if got := string(call("GET", base, "", mod.cookie, 200).body["role"]); got != `"moderator"` {
		t.Fatal(got)
	}
	call("GET", base+"/moderators", "", mod.cookie, 200)
	moderate := func(c *http.Cookie, body string, status int) {
		t.Helper()
		call("POST", session+"/moderate", body, c, status)
	}
	moderate(mod.cookie, `{"action":"mute","seconds":30,"user_id":`+string(owner.body["user_id"])+"}", 403)
	moderate(mod.cookie, `{"action":"mute","seconds":30,"user_id":`+string(mod.body["user_id"])+"}", 403)
	call("POST", "/api/rooms/other/live/moderate", `{"action":"mute","seconds":60,"user_id":7}`, mod.cookie, 403)
	const target = "18446744073709551615"
	moderate(mod.cookie, `{"action":"mute","seconds":60,"user_id":`+target+"}", 204)
	if !strings.Contains(string(call("GET", session+"/mutes", "", mod.cookie, 200).body["mutes"]), target) {
		t.Fatal("missing mute")
	}
	moderate(mod.cookie, `{"action":"unmute","user_id":`+target+"}", 204)
	if got := string(call("GET", session+"/mutes", "", mod.cookie, 200).body["mutes"]); got != "[]" {
		t.Fatal(got)
	}
	page := call("GET", session+"/audits?limit=1", "", owner.cookie, 200)
	if string(page.body["has_more"]) != "true" || !strings.Contains(string(page.body["audits"]), target) {
		t.Fatal("audit page", page.body)
	}
	call("GET", session+"/audits?before_id="+string(page.body["next_before_id"]), "", owner.cookie, 200)
	accept := func(user response, want string) {
		t.Helper()
		uid, _ := strconv.ParseUint(string(user.body["user_id"]), 10, 64)
		msg := protocol.DanmakuMessage{EventID: rand.Text(), Nickname: "account", RoomID: slug + "/other", UserID: uid}
		status, _, e := rs.AcceptDanmaku(ctx, msg, []byte(`{"type":101,"data":{}}`))
		if e != nil || status != want {
			t.Fatal(status, e, want)
		}
	}
	accept(mod, "new")
	accept(viewer, "room_muted")
	call("DELETE", base+"/moderators/"+string(mod.body["user_id"]), "", owner.cookie, 204)
	accept(mod, "room_muted")
	moderate(owner.cookie, `{"action":"mute_all","enabled":false}`, 204)
	moderate(owner.cookie, `{"action":"slow_mode","seconds":3}`, 204)
	accept(viewer, "new")
	accept(viewer, "slow_mode")
	h.current.Store(nil)
	me := call("GET", "/api/auth/me", "", owner.cookie, 200)
	if string(me.body["balance"]) != "null" || string(me.body["authenticated"]) != "true" {
		t.Fatal("offline identity")
	}
	call("PATCH", "/api/auth/me", `{"nickname":"offline"}`, owner.cookie, 503)
	call("GET", base, "", owner.cookie, 503)
	header := http.Header{"Cookie": []string{owner.cookie.Name + "=" + owner.cookie.Value}}
	ws, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+session+"/live", header)
	if e != nil {
		t.Fatal(e)
	}
	defer ws.Close()
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var packet protocol.WsPacket
	if e = ws.ReadJSON(&packet); e != nil || packet.Type != protocol.TypeReady {
		t.Fatal("ready", packet, e)
	}
	if e = ws.WriteJSON(map[string]any{"type": 101, "data": map[string]string{"content": "hello", "nickname": "spoof"}}); e != nil {
		t.Fatal(e)
	}
	for packet.Type != protocol.TypeDanmaku {
		if e = ws.ReadJSON(&packet); e != nil {
			t.Fatal(e)
		}
	}
	var msg protocol.DanmakuMessage
	if e = json.Unmarshal(packet.Data, &msg); e != nil || msg.Nickname != "测试房主" {
		t.Fatal(msg, e)
	}
	call("GET", session+"/recent", "", owner.cookie, 200)
	if events := string(call("GET", session+"/recent", "", owner.cookie, 200).body["events"]); !strings.Contains(events, `"type":109`) {
		t.Fatal("room state missing from recovery")
	}
	h.current.Store(pool)
	call("GET", base, "", owner.cookie, 200)
	call("POST", "/api/auth/logout", "", owner.cookie, 204)
	if err := ws.WriteJSON(map[string]any{"type": 101, "data": map[string]string{"content": "logged out"}}); err != nil {
		t.Fatal(err)
	}
	for {
		if e = ws.ReadJSON(&packet); e != nil {
			break
		}
		if packet.Type == protocol.TypeDanmaku {
			t.Fatal("logged-out socket published")
		}
	}
	if got := string(call("GET", "/api/auth/me", "", owner.cookie, 200).body["authenticated"]); got != "false" {
		t.Fatal(got)
	}
}
