package httpapi

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCaptureReservation(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"sessions", "live"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	id := "12345678-1234-1234-1234-123456789abc"
	for range 2 {
		if err := reserveCapture(root, "room", id); err != nil {
			t.Fatal(err)
		}
	}
	if err := reserveCapture(root, "room", id[:35]+"d"); !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	if err := reserveCapture(root, "room", "../other"); !errors.Is(err, os.ErrInvalid) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "live", "room", "room"))
	if err != nil || string(b) != "room" {
		t.Fatal(string(b), err)
	}
}
func TestVideoRootConfinement(t *testing.T) {
	root := t.TempDir()
	t.Setenv("REPLAY_STORAGE_DIR", root)
	outside := filepath.Join(t.TempDir(), "elsewhere.mp4")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.mp4")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, "../elsewhere.mp4", "nested/../movie.mp4", "link.mp4"} {
		if f, _, err := openVideo(path); err == nil {
			f.Close()
			t.Fatal(path)
		}
	}
}
func TestWebhookAuthorization(t *testing.T) {
	h := new(ChatHandler)
	for _, tc := range []struct {
		secret, header, addr string
		want                 int
	}{
		{"", "", "198.51.100.3:2000", 403}, {"", "", "127.0.0.1:8000", 503}, {"key", "bad", "127.0.0.1:8000", 403}, {"key", "key", "198.51.100.3:2000", 503},
	} {
		t.Setenv("STREAM_WEBHOOK_SECRET", tc.secret)
		r := httptest.NewRequest("POST", "/internal/stream/ready", strings.NewReader(`{"room_slug":"demo"}`))
		r.RemoteAddr = tc.addr
		r.Header.Set("X-Webhook-Secret", tc.header)
		w := httptest.NewRecorder()
		h.streamHook(w, r)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code)
		}
	}
}
