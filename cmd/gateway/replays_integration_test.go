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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/mq"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

// Use a dedicated test broker: this exercises the production Archiver consumer group.
func TestOBSReplayLifecycle(t *testing.T) {
	brokers := os.Getenv("DANMAKU_TEST_KAFKA_BROKERS")
	if brokers == "" {
		t.Skip("requires dedicated Kafka broker, PostgreSQL and Redis")
	}
	dir := t.TempDir()
	t.Setenv("REPLAY_STORAGE_DIR", dir)
	t.Setenv("STREAM_WEBHOOK_SECRET", "test-webhook")
	accountServer(t, func(t *testing.T, ctx context.Context, h *gatewayHistory, pool *historyPool, rs *pubsub.Store, mux *http.ServeMux, handler *httpapi.ChatHandler, server *httptest.Server) {
		repo := store.NewMessageRepo(pool.db)
		consumer, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mq.Consume(consumer, mq.Brokers(brokers), repo, slog.Default(), nil) }()
		defer func() { stop(); <-done }()
		var cookie *http.Cookie
		call := func(method, path, body string, want int) map[string]any {
			t.Helper()
			r := httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("X-Webhook-Secret", "test-webhook")
			if cookie != nil {
				r.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("%s %s: %d (want %d): %s", method, path, w.Code, want, w.Body)
			}
			if cookies := w.Result().Cookies(); len(cookies) > 0 {
				cookie = cookies[0]
			}
			var obj map[string]any
			if w.Body.Len() > 0 && json.Unmarshal(w.Body.Bytes(), &obj) != nil {
				t.Fatal("invalid JSON")
			}
			return obj
		}
		room := "obs" + rand.Text()[:12]
		base := "/api/rooms/" + room
		call("POST", "/api/auth/register", `{"username":"owner","password":"password"}`, 200)
		call("POST", "/api/rooms", fmt.Sprintf(`{"slug":%q,"title":"Replay"}`, room), 201)
		owner := cookie
		cookie = nil
		call("GET", base+"/stream-key", "", 403)
		cookie = owner
		key := call("GET", base+"/stream-key", "", 200)["stream_key"].(string)
		if len(key) < 32 {
			t.Fatal("short key")
		}
		if _, leak := call("GET", base, "", 200)["stream_key"]; leak {
			t.Fatal("public stream key")
		}
		auth := func(key string, want int) {
			call("POST", "/internal/stream/auth", fmt.Sprintf(`{"action":"publish","path":"live/%s","query":"key=%s","protocol":"rtmp"}`, room, key), want)
		}
		auth(key, 200)
		newKey := call("POST", base+"/stream-key/rotate", "", 200)["stream_key"].(string)
		auth(key, 403)
		auth(newKey, 200)
		hook := fmt.Sprintf(`{"room_slug":%q,"source_id":"connection-1"}`, room)
		active := call("POST", "/internal/stream/ready", hook, 200)
		if call("POST", "/internal/stream/ready", hook, 200)["replay_id"] != active["replay_id"] {
			t.Fatal("duplicate replay")
		}
		session := active["session_id"].(string)
		id := fmt.Sprint(active["replay_id"])
		path := base + "/replays/" + id
		if call("GET", base+"/stream", "", 200)["session"] != session {
			t.Fatal("stream lookup")
		}
		ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+base+"/"+session+"/live", http.Header{"Cookie": []string{cookie.String()}})
		if err != nil {
			t.Fatal(err)
		}
		defer ws.Close()
		if err = ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var pkt protocol.WsPacket
		if err = ws.ReadJSON(&pkt); err != nil || pkt.Type != protocol.TypeReady {
			t.Fatal(err)
		}
		h.current.Store(nil)
		if err = ws.WriteJSON(map[string]any{"type": 101, "data": map[string]string{"content": "archived"}}); err != nil {
			t.Fatal(err)
		}
		for pkt.Type != protocol.TypeDanmaku {
			if err = ws.ReadJSON(&pkt); err != nil {
				t.Fatal(err)
			}
		}
		var msg protocol.DanmakuMessage
		if err = json.Unmarshal(pkt.Data, &msg); err != nil || msg.VideoOffsetMS == nil {
			t.Fatal("missing live offset", err)
		}
		h.current.Store(pool)
		for deadline := time.Now().Add(15 * time.Second); ; {
			var exists bool
			err = pool.db.QueryRow(ctx, "SELECT EXISTS(SELECT FROM danmaku_messages WHERE event_id=$1 AND video_offset_ms=$2)", msg.EventID, *msg.VideoOffsetMS).Scan(&exists)
			if err != nil {
				t.Fatal(err)
			}
			if exists {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Archiver did not persist offset")
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err = os.WriteFile(filepath.Join(dir, "video.mp4"), []byte("0123456789"), 0600); err != nil {
			t.Fatal(err)
		}
		end := func(file string, want int) {
			call("POST", "/internal/stream/ended", fmt.Sprintf(`{"room_slug":%q,"replay_id":%s,"duration_ms":120000,"video_file":%q}`, room, id, file), want)
		}
		end("../video.mp4", 400)
		end("video.mp4", 200)
		end("video.mp4", 200)
		if call("GET", path, "", 200)["chunk_ms"] != float64(60000) {
			t.Fatal("chunk size")
		}
		r := httptest.NewRequest("GET", "/api/replays/"+id+"/video", nil)
		r.Header.Set("Range", "bytes=2-4")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 206 || w.Body.String() != "234" {
			t.Fatal("range", w.Code, w.Body)
		}
		timeline := path + "/danmaku"
		cookie = nil
		call("POST", timeline, `{"content":"anonymous","video_offset_ms":0}`, 401)
		cookie = owner
		call("POST", timeline, `{"content":"replay-blocked-word","video_offset_ms":0}`, 400)
		call("POST", timeline, `{"content":"late","video_offset_ms":125001}`, 400)
		call("GET", timeline+"?to_ms=300001", "", 400)
		call("GET", timeline+"?after_id=2", "", 400)
		cross := httptest.NewRequest("DELETE", path, nil)
		cross.Header.Set("Origin", "https://example.invalid")
		cross.AddCookie(owner)
		refused := httptest.NewRecorder()
		mux.ServeHTTP(refused, cross)
		if refused.Code != 403 {
			t.Fatal("cross-origin deletion", refused.Code)
		}
		if stream := call("GET", base+"/stream", "", 200); stream["live"] != false {
			t.Fatal("ended stream still active")
		}
		for range 12 {
			call("POST", timeline, `{"content":"same millisecond","video_offset_ms":60000}`, 200)
		}
		if err = repo.Withdraw(ctx, room+"/"+session, msg.EventID); err != nil {
			t.Fatal(err)
		}
		if len(call("GET", timeline, "", 200)["items"].([]any)) != 0 {
			t.Fatal("withdrawn visible")
		}
		seen := map[float64]bool{}
		cursor := ""
		for {
			p := call("GET", timeline+"?from_ms=60000&limit=3"+cursor, "", 200)
			for _, raw := range p["items"].([]any) {
				id := raw.(map[string]any)["id"].(float64)
				if seen[id] {
					t.Fatal("duplicate cursor item")
				}
				seen[id] = true
			}
			if p["has_more"] != true {
				break
			}
			cursor = fmt.Sprintf("&after_offset_ms=%.0f&after_id=%.0f", p["next_offset_ms"], p["next_after_id"])
		}
		if len(seen) != 12 || call("GET", path, "", 200)["danmaku_count"] != float64(12) {
			t.Fatal("missing comments")
		}
		_, err = pool.db.Exec(ctx, `INSERT INTO danmaku_messages(room_id,content,user_id,send_time) SELECT room_id,'legacy',1,date_trunc('milliseconds',started_at)+interval '1 millisecond' FROM room_replays WHERE id=$1`, int64(active["replay_id"].(float64)))
		if err != nil {
			t.Fatal(err)
		}
		legacy := call("GET", timeline, "", 200)["items"].([]any)
		if len(legacy) != 1 || legacy[0].(map[string]any)["content"] != "legacy" {
			t.Fatal("legacy timeline", legacy)
		}
		if listing := call("GET", base+"/replays?limit=1", "", 200); len(listing["items"].([]any)) != 1 {
			t.Fatal(listing)
		}
		_, err = pool.db.Exec(ctx, `INSERT INTO danmaku_messages(room_id,content,user_id,event_id,send_time,video_offset_ms)
        SELECT $1,'dense',1,'dense-'||n,now(),120000 FROM generate_series(1,2001) n`, room+"/"+session)
		if err != nil {
			t.Fatal(err)
		}
		dense := call("GET", timeline+"?from_ms=120000&limit=9999", "", 200)
		if len(dense["items"].([]any)) != 2000 || dense["has_more"] != true {
			t.Fatal("dense chunk limit")
		}
		tail := call("GET", timeline+fmt.Sprintf("?from_ms=120000&after_offset_ms=120000&after_id=%.0f", dense["next_after_id"]), "", 200)
		if len(tail["items"].([]any)) != 1 || tail["has_more"] != false {
			t.Fatal("dense chunk tail")
		}
		// A delayed remux from the previous publisher must leave the newer
		// session live, even when both callbacks land on the same gateway.
		older := call("POST", "/internal/stream/ready", fmt.Sprintf(`{"room_slug":%q,"source_id":"older"}`, room), 200)
		newer := call("POST", "/internal/stream/ready", fmt.Sprintf(`{"room_slug":%q,"source_id":"newer"}`, room), 200)
		call("POST", "/internal/stream/ended", fmt.Sprintf(`{"room_slug":%q,"replay_id":%.0f}`, room, older["replay_id"]), 200)
		if call("GET", base+"/stream", "", 200)["replay_id"] != newer["replay_id"] {
			t.Fatal("stale ended callback replaced active session")
		}
		stopped := call("POST", "/internal/stream/ended", fmt.Sprintf(`{"room_slug":%q}`, room), 200)
		if stopped["status"] != "failed" || stopped["replay_id"] != newer["replay_id"] {
			t.Fatal("missing recording", stopped)
		}
		call("DELETE", path, "", 204)
		if _, err = os.Stat(filepath.Join(dir, "video.mp4")); !os.IsNotExist(err) {
			t.Fatal("video still present", err)
		}
	})
}
