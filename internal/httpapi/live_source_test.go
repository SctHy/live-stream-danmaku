package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLiveSource(t *testing.T) {
	var calls atomic.Int32
	codec := `"codec":[{"base_url":"/live","url_info":[{"host":"https://cdn.example","extra":"?x"}]}]`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Query().Get("room_id") {
		case "1":
			fmt.Fprintf(w, `{"code":0,"data":{"live_status":1,"playurl_info":{"playurl":{"stream":[{"format":[{"format_name":"flv",%s},{"format_name":"ts",%s}]}]}}}}`, codec, codec)
		case "2":
			fmt.Fprint(w, `{"code":0,"data":{"live_status":0}}`)
		default:
			w.WriteHeader(503)
		}
	}))
	defer upstream.Close()
	s := newLiveResolver(upstream.URL, upstream.Client())
	for _, room := range []string{"1", "2"} {
		for range 2 {
			w := httptest.NewRecorder()
			s.serve(w, httptest.NewRequest("GET", "/?room_id="+room, nil))
			var result liveResult
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Live != (room == "1") {
				t.Fatal(room, w.Body.String())
			}
			if result.Live && (result.Sources["flv"] != "https://cdn.example/live?x" || result.Sources["hls"] != "https://cdn.example/live?x") {
				t.Fatal(result)
			}
		}
	}
	if calls.Load() != 2 {
		t.Fatal(calls.Load())
	}
	for range 2 {
		w := httptest.NewRecorder()
		s.serve(w, httptest.NewRequest("GET", "/?room_id=3", nil))
		if w.Code != 502 {
			t.Fatal(w.Code)
		}
	}
	if calls.Load() != 4 {
		t.Fatal("error was cached")
	}
	for room, ttl := range map[uint64]time.Duration{1: 15 * time.Second, 2: 5 * time.Second} {
		entry := s.cache[room]
		remaining := time.Until(entry.expires)
		if remaining <= 0 || remaining > ttl || (room == 1 && remaining <= 5*time.Second) {
			t.Fatalf("room %d TTL %s", room, remaining)
		}
		entry.expires = time.Now()
		s.cache[room] = entry
		if _, err := s.resolve(t.Context(), room); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 6 {
		t.Fatal("expired entry served")
	}

}
func TestLiveConcurrentMiss(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(30 * time.Millisecond)
		fmt.Fprint(w, `{"code":0}`)
	}))
	defer upstream.Close()
	s := newLiveResolver(upstream.URL, upstream.Client())
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			<-start
			if _, err := s.resolve(t.Context(), 1); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}
