package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/protocol"
)

type testHistory struct{ online atomic.Bool }

func (p *testHistory) Withdraw(context.Context, string, string) error {
	if !p.online.Load() {
		return errHistoryUnavailable
	}
	return nil
}
func (p *testHistory) ListHistory(ctx context.Context, room string, _ uint64, _ int) ([]protocol.DanmakuMessage, bool, error) {
	return nil, false, p.Withdraw(ctx, room, "")
}

func TestHistoryRecovery(t *testing.T) {
	var db testHistory
	var attempts, opened, closed atomic.Int32
	h := startHistory(t.Context(), func(ctx context.Context) (*historyPool, error) {
		attempts.Add(1)
		if !db.online.Load() {
			return nil, errHistoryUnavailable
		}
		opened.Add(1)
		return &historyPool{HistoryStore: &db, close: func() { closed.Add(1) }}, nil
	}, time.Millisecond, slog.Default())
	defer h.Close()
	mux := http.NewServeMux()
	httpapi.NewChatHandler(h, nil, nil, false).Routes(mux)
	status := func() int {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/rooms/r/s/history", nil))
		return w.Code
	}
	await := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatal("history state transition timed out")
			}
			time.Sleep(time.Millisecond)
		}
	}
	await(func() bool { return attempts.Load() > 1 })
	for cycle := range 2 {
		if got := status(); got != 503 {
			t.Fatalf("offline history: %d", got)
		}
		db.online.Store(true)
		var readers sync.WaitGroup
		for range 20 {
			readers.Go(func() {
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					switch got := status(); got {
					case 200:
						return
					case 503:
						time.Sleep(time.Millisecond)
					default:
						t.Errorf("history: %d", got)
						return
					}
				}
				t.Error("reader did not recover")
			})
		}
		readers.Wait()
		if got := opened.Load(); got != int32(cycle+1) {
			t.Fatalf("opened %d pools", got)
		}
		db.online.Store(false)
	}
	h.Close()
	if opened.Load() != closed.Load() || status() != 503 {
		t.Fatal("shutdown left a pool available or unclosed")
	}
}

func TestCancelDuringHistoryOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	h := startHistory(ctx, func(op context.Context) (*historyPool, error) {
		if _, ok := op.Deadline(); !ok {
			t.Error("open must be bounded")
		}
		close(started)
		<-op.Done()
		return nil, op.Err()
	}, time.Hour, slog.Default())
	<-started
	cancel()
	select {
	case <-h.done:
	case <-time.After(time.Second):
		t.Fatal("worker survived cancellation")
	}
	h.Close()
}

func TestReadiness(t *testing.T) {
	for _, tc := range []struct {
		retiring, redisDown bool
		want                int
	}{{false, false, 200}, {false, true, 503}, {true, false, 503}} {
		ping := func(ctx context.Context) error {
			if tc.retiring {
				t.Fatal("ping while retiring")
			}
			if tc.redisDown {
				return errors.New("offline")
			}
			return nil
		}
		w := httptest.NewRecorder()
		readiness(func() bool { return tc.retiring }, ping)(w, httptest.NewRequest("GET", "/readyz", nil))
		if w.Code != tc.want {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
	}
}
