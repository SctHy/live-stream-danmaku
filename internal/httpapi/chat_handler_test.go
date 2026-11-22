package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

type fakeStore struct {
	sessionStore
	list                        func(string, uint64, int) ([]protocol.DanmakuMessage, bool, error)
	muted, withdrawn, journaled bool
}

func (f *fakeStore) ListHistory(_ context.Context, room string, b uint64, l int) ([]protocol.DanmakuMessage, bool, error) {
	return f.list(room, b, l)
}
func (f *fakeStore) Withdraw(_ context.Context, room, id string) error {
	f.withdrawn = room == "r/s" && id == strings.Repeat("e", 26)
	return nil
}
func (f *fakeStore) Mute(_ context.Context, room string, id uint64, ttl time.Duration) error {
	f.muted = room == "r/s" && id == 7 && ttl == time.Minute
	return nil
}
func (f *fakeStore) Journal(_ context.Context, room string, data []byte) error {
	var p protocol.WsPacket
	var d protocol.Withdraw
	f.journaled = json.Unmarshal(data, &p) == nil && json.Unmarshal(p.Data, &d) == nil && p.Type == protocol.TypeWithdraw && d.EventID == strings.Repeat("e", 26) && p.RoomID == room
	return nil
}
func routes(f *fakeStore) *http.ServeMux {
	mux := http.NewServeMux()
	NewChatHandler(f, nil, f, false).Routes(mux)
	return mux
}
func TestHistoryRoute(t *testing.T) {
	f := &fakeStore{}
	mux := routes(f)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	base := "/api/rooms/r/s/history"
	for _, path := range []string{"/api/rooms/r/bad!/history", "/api/rooms/bad!/s/history", base + "?limit=0", base + "?limit=101", base + "?limit=a", base + "?before_id=", base + "?before_id=-1", base + "?before_id=18446744073709551616"} {
		if w := get(path); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	f.list = func(r string, b uint64, l int) ([]protocol.DanmakuMessage, bool, error) {
		if r != "r/s" || b != ^uint64(0) || l != 2 {
			t.Fatal(r, b, l)
		}
		return []protocol.DanmakuMessage{{ID: 9}, {ID: 7}}, true, nil
	}
	w := get(base + "?before_id=18446744073709551615&limit=2")
	var page struct {
		Messages []protocol.DanmakuMessage
		More     bool   `json:"has_more"`
		Next     uint64 `json:"next_before_id"`
	}
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(w.Body.Bytes(), &page) != nil || !page.More || page.Next != 7 || len(page.Messages) != 2 {
		t.Fatal(w.Body.String())
	}
	f.list = func(r string, b uint64, l int) ([]protocol.DanmakuMessage, bool, error) {
		if b != 0 || l != 50 {
			t.Fatal(b, l)
		}
		return nil, false, nil
	}
	w = get(base)
	if !strings.Contains(w.Body.String(), `"messages":[]`) || !strings.Contains(w.Body.String(), `"next_before_id":0`) {
		t.Fatal(w.Body.String())
	}
	f.list = func(string, uint64, int) ([]protocol.DanmakuMessage, bool, error) {
		return nil, false, errors.New("offline")
	}
	if w := get(base); w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestModerateRoute(t *testing.T) {
	t.Setenv("DANMAKU_MOD_ROOM", "r")
	t.Setenv("DANMAKU_MOD_TOKEN", "secret")
	f := &fakeStore{}
	mux := routes(f)
	mute := `{"action":"mute","user_id":7,"seconds":60}`
	for _, tc := range []struct {
		body, token, origin string
		code                int
	}{
		{mute, "bad", "", 403}, {mute, "secret", "https://other", 403}, {"{", "secret", "", 400}, {`{"action":"unknown"}`, "secret", "", 400}, {`{"action":"mute","user_id":1}`, "secret", "", 400}, {`{"action":"withdraw","event_id":"x"}`, "secret", "", 400}, {mute, "secret", "http://example.com", 204}, {`{"action":"withdraw","event_id":"` + strings.Repeat("e", 26) + `"}`, "secret", "", 204},
	} {
		r := httptest.NewRequest("POST", "/api/rooms/r/s/moderate", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatal(tc, w.Code)
		}
	}
	if !f.muted || !f.withdrawn || !f.journaled {
		t.Fatal("moderation arguments or journal mismatch")
	}
}
func TestRecoverAndOrigin(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	r := httptest.NewRequest("GET", "/boom", nil)
	for origin, want := range map[string]bool{"": true, "http://EXAMPLE.com": true, "file://example.com": false, "https://other": false, "%": false} {
		r.Header.Set("Origin", origin)
		if sameOrigin(r) != want {
			t.Fatal(origin)
		}
	}
	w := httptest.NewRecorder()
	Recover(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") })).ServeHTTP(w, r)
	raw, _ := io.ReadAll(&logs)
	if w.Code != 500 || !bytes.Contains(raw, []byte(`"path":"/boom"`)) || !bytes.Contains(raw, []byte(`"method":"GET"`)) || !bytes.Contains(raw, []byte("goroutine")) {
		t.Fatal(w.Code, string(raw))
	}
}

func TestLiveDanmakuProtocolExport(t *testing.T) {
	w := httptest.NewRecorder()
	routes(&fakeStore{}).ServeHTTP(w, httptest.NewRequest("GET", "/assets/protocol.js", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("protocol Cache-Control = %q", got)
	}
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "liveDanmaku=108") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestIndexCacheControl(t *testing.T) {
	w := httptest.NewRecorder()
	routes(&fakeStore{}).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if got := w.Header().Get("Cache-Control"); w.Code != http.StatusOK || got != "no-cache" {
		t.Fatalf("index status = %d, Cache-Control = %q", w.Code, got)
	}
}
