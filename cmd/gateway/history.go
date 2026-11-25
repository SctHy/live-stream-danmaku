package main

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/scthy/maku/internal/httpapi"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/store"
)

var errHistoryUnavailable = errors.New("history database unavailable")

type historyPool struct {
	httpapi.HistoryStore
	close  func()
	failed chan struct{}
}

func openHistory(ctx context.Context) (*historyPool, error) {
	db, err := store.OpenContext(ctx)
	if err != nil {
		return nil, err
	}
	return &historyPool{HistoryStore: store.NewMessageRepo(db), close: db.Close}, nil
}

type gatewayHistory struct {
	current atomic.Pointer[historyPool]
	cancel  context.CancelFunc
	done    chan struct{}
}

// A single worker owns opening, migrating and closing pools. Requests only
// see a migrated pool and can invalidate only the generation they used.
func startHistory(parent context.Context, open func(context.Context) (*historyPool, error), retry time.Duration, log *slog.Logger) *gatewayHistory {
	ctx, cancel := context.WithCancel(parent)
	h := &gatewayHistory{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(h.done)
		for ctx.Err() == nil {
			attempt, finish := context.WithTimeout(ctx, time.Minute)
			p, err := open(attempt)
			finish()
			if err == nil {
				p.failed = make(chan struct{})
				h.current.Store(p)
				select {
				case <-ctx.Done():
				case <-p.failed:
				}
				h.current.Store(nil)
				p.close()
			} else if ctx.Err() == nil {
				log.Warn("Open history database", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retry):
			}
		}
	}()
	return h
}

func (h *gatewayHistory) Close() { h.cancel(); <-h.done }

func (h *gatewayHistory) invalidate(p *historyPool, ctx context.Context, err error) {
	if err != nil && ctx.Err() == nil && h.current.CompareAndSwap(p, nil) {
		close(p.failed)
	}
}

func (h *gatewayHistory) ListHistory(ctx context.Context, room string, before uint64, limit int) ([]protocol.DanmakuMessage, bool, error) {
	p := h.current.Load()
	if p == nil {
		return nil, false, errHistoryUnavailable
	}
	op, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	msgs, more, err := p.ListHistory(op, room, before, limit)
	h.invalidate(p, ctx, err)
	return msgs, more, err
}

func (h *gatewayHistory) Withdraw(ctx context.Context, room, id string) error {
	p := h.current.Load()
	if p == nil {
		return errHistoryUnavailable
	}
	op, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := p.Withdraw(op, room, id)
	h.invalidate(p, ctx, err)
	return err
}
