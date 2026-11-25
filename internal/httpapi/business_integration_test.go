package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

type plazaQueryCounts struct{ index, details atomic.Int32 }

func (c *plazaQueryCounts) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(d.SQL, "SELECT slug,owner_id,title") && strings.Contains(d.SQL, "WHERE slug=ANY($1)") {
		c.details.Add(1)
	}
	if strings.HasPrefix(d.SQL, "SELECT slug,status,created_at") {
		c.index.Add(1)
	}
	return ctx
}
func (*plazaQueryCounts) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type businessTestDB struct {
	*store.MessageRepo
	pool *pgxpool.Pool
}

func (db *businessTestDB) BusinessDB() *pgxpool.Pool { return db.pool }
func TestBusinessRoutesIntegration(t *testing.T) {
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
	schema := pgx.Identifier{"gifts_" + rand.Text()}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	counts := new(plazaQueryCounts)
	cfg.ConnConfig.Tracer = counts
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = store.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	slug := rand.Text()[:16]
	room := slug + "/live"
	uid := ^uint64(0)
	_, e = db.Exec(ctx, "INSERT INTO users(id,username,nickname,password_hash) VALUES($1,'owner','owner',''),(2,'mod','mod','')", uid)
	if e != nil {
		t.Fatal(e)
	}
	redis := pubsub.Open(addr)
	defer redis.Close()
	token := fmt.Sprint(uid) + "." + rand.Text()
	modToken := "2." + rand.Text()
	for k, v := range map[string]uint64{token: uid, modToken: 2} {
		if e := redis.SaveLogin(ctx, k, pubsub.Login{UserID: v}, time.Minute); e != nil {
			t.Fatal(e)
		}
	}
	history := &businessTestDB{store.NewMessageRepo(db), db}
	h := NewChatHandler(history, nil, redis, false)
	mux := http.NewServeMux()
	h.Routes(mux)
	call := func(method, path, body, login string) (int, map[string]any) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if login != "" {
			r.AddCookie(&http.Cookie{Name: loginCookie, Value: login})
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var obj map[string]any
		if w.Body.Len() > 0 {
			if e := json.Unmarshal(w.Body.Bytes(), &obj); e != nil {
				t.Error(e)
			}
		}
		return w.Code, obj
	}
	expect := func(method, path, body, login string, status int) map[string]any {
		t.Helper()
		n, o := call(method, path, body, login)
		if n != status {
			t.Fatalf("%s %s: status %d %v", method, path, n, o)
		}
		return o
	}
	base := "/api/rooms/" + room
	expect("POST", "/api/rooms", fmt.Sprintf(`{"slug":%q,"title":"old"}`, slug), token, 201)
	var wg sync.WaitGroup
	var claims, orders, spent atomic.Int32
	for range 12 {
		wg.Go(func() {
			n, o := call("POST", "/api/wallet/checkin", "", token)
			if n != 200 {
				t.Error(n, o)
			}
			if o["claimed"] == true {
				claims.Add(1)
			}
		})
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatal(claims.Load())
	}
	send := func(id int) (int, map[string]any) {
		return call("POST", base+"/gifts", fmt.Sprintf(`{"client_order_id":"gift-order-%016d","gift_code":"sc_super","quantity":1,"message":"hello"}`, id), token)
	}
	for range 16 {
		wg.Go(func() {
			n, o := send(0)
			if n != 200 {
				t.Error(n, o)
			}
			if o["duplicate"] == false {
				orders.Add(1)
			}
		})
	}
	wg.Wait()
	if orders.Load() != 1 {
		t.Fatal(orders.Load())
	}
	for i := 1; i <= 12; i++ {
		wg.Go(func() {
			n, o := send(i)
			if n == 200 {
				spent.Add(1)
			} else if n != 409 || o["error"] != "insufficient_balance" {
				t.Error(n, o)
			}
		})
	}
	wg.Wait()
	if spent.Load() != 4 {
		t.Fatal(spent.Load())
	}
	listing := expect("GET", base+"/gifts?limit=2", "", "", 200)
	if len(listing["gifts"].([]any)) != 2 || len(listing["active_superchats"].([]any)) != 5 {
		t.Fatal(listing)
	}
	wallet := expect("GET", "/api/wallet", "", token, 200)
	if wallet["balance"] != float64(0) || len(wallet["orders"].([]any)) != 5 {
		t.Fatal(wallet)
	}
	journal, e := redis.Recover(ctx, room, "0-0")
	if e != nil || len(journal.Events) != 5 {
		t.Fatal(journal, e)
	}
	for _, p := range journal.Events {
		if p.Type != protocol.TypeGift {
			t.Fatal(p)
		}
	}
	if _, e = redis.SnapshotStats(ctx, "a", map[string]int{room: 3}, map[string]uint64{}, time.Minute); e != nil {
		t.Fatal(e)
	}
	for i, body := range []string{`%_\`, `%_\`, "abc", `%_\`, `%_\`} {
		sender := uid
		if i == 4 {
			sender = 2
		}
		_, e := history.SaveBatch(ctx, []*protocol.DanmakuMessage{{RoomID: room, UserID: sender, Content: body, EventID: fmt.Sprint(i), SendTime: time.Now()}})
		if e != nil {
			t.Fatal(e)
		}
	}
	if e = history.Withdraw(ctx, room, "3"); e != nil {
		t.Fatal(e)
	}
	path := base + "/history?limit=1&q=" + url.QueryEscape(`%_\`) + "&user_id=" + fmt.Sprint(uid)
	first := expect("GET", path, "", "", 200)
	if first["has_more"] != true {
		t.Fatal(first)
	}
	second := expect("GET", path+"&before_id="+fmt.Sprint(first["next_before_id"]), "", "", 200)
	if second["has_more"] != false || second["messages"].([]any)[0].(map[string]any)["event_id"] != "0" {
		t.Fatal(second)
	}
	expect("GET", base+"/analytics", "", "", 403)
	expect("GET", base+"/analytics", "", modToken, 403)
	if _, e = db.Exec(ctx, "INSERT INTO room_moderators VALUES($1,2,$2,now())", slug, uid); e != nil {
		t.Fatal(e)
	}
	for _, login := range []string{modToken, token} {
		o := expect("GET", base+"/analytics", "", login, 200)
		for key, n := range map[string]int{"danmaku_count": 4, "withdrawn_count": 1, "active_senders": 2, "peak_online": 3, "online": 3, "gift_order_count": 5, "gift_total_cost": 1500, "moderation_count": 1} {
			if o[key] != float64(n) {
				t.Fatal(key, o)
			}
		}
	}
	rooms := func() map[string]any {
		obj := expect("GET", "/api/rooms?sort=hot", "", "", 200)
		for _, v := range obj["rooms"].([]any) {
			item := v.(map[string]any)
			if item["slug"] == slug {
				return item
			}
		}
		t.Fatal(obj)
		return nil
	}
	for range 24 {
		wg.Go(func() {
			n, _ := call("GET", "/api/rooms?sort=hot", "", "")
			if n != 200 {
				t.Error(n)
			}
		})
	}
	wg.Wait()
	if counts.details.Load() != 1 || counts.index.Load() != 1 {
		t.Fatalf("concurrent index/detail queries = %d/%d", counts.index.Load(), counts.details.Load())
	}
	if rooms()["title"] != "old" {
		t.Fatal("missing metadata")
	}
	peer := http.NewServeMux()
	NewChatHandler(history, nil, redis, false).Routes(peer)
	primary := mux
	mux = peer
	expect("PATCH", "/api/rooms/"+slug, `{"title":"new"}`, token, 200)
	mux = primary
	if rooms()["title"] != "new" {
		t.Fatal("stale metadata")
	}
	history.pool = nil
	if o := rooms(); o["degraded"] != true || o["online"] != float64(3) {
		t.Fatal(o)
	}
	expect("GET", base+"/analytics", "", token, 503)
	board := expect("GET", base+"/leaderboard", "", "", 200)
	if board["contributors"].([]any)[0].(map[string]any)["total_cost"] != float64(1500) {
		t.Fatal(board)
	}
}
