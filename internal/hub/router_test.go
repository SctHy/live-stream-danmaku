package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/protocol"
)

func TestInvalidInputReturnsReceipt(t *testing.T) {
	m := Hub{sendCapacity: 1}
	c := m.NewClient(42, "room", nil)
	m.dispatch(c, protocol.WsPacket{Type: protocol.TypeDanmaku})
	f := <-c.send
	packets, err := protocol.DecodePackets(f.payload)
	if err != nil || packets[0].Type != protocol.TypeReceipt {
		t.Fatal(string(f.payload), err)
	}
}

type captureStore struct {
	roomStore
	msg protocol.DanmakuMessage
}

func (s *captureStore) AcceptDanmakuPacket(_ context.Context, msg protocol.DanmakuMessage, wire []byte) (string, string, []byte, error) {
	s.msg = msg
	return "accepted", msg.EventID, nil, nil
}
func TestFilterAndDraining(t *testing.T) {
	for _, tc := range []struct {
		name            string
		mask, drain     bool
		status, content string
	}{
		{"reject", false, false, "filtered", ""}, {"mask", true, false, "accepted", "**!"}, {"draining", true, true, "unavailable", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &captureStore{}
			h := Hub{life: t.Context(), store: store, filter: filter.New([]string{"禁词"}), maskWords: tc.mask, draining: tc.drain, sendCapacity: 1}
			c := h.NewClient(7, "r", nil)
			h.handleDanmaku(c, []byte(`{"content":"禁词!"}`))
			packets, err := protocol.DecodePackets((<-c.send).payload)
			if err != nil || len(packets) != 1 {
				t.Fatal(packets, err)
			}
			var receipt protocol.Receipt
			if json.Unmarshal(packets[0].Data, &receipt) != nil || receipt.Status != tc.status || store.msg.Content != tc.content {
				t.Fatal(receipt, store.msg)
			}
		})
	}
}

type clockFunc func(context.Context, string, time.Time) (*int64, error)

func (f clockFunc) OffsetMS(ctx context.Context, room string, at time.Time) (*int64, error) {
	return f(ctx, room, at)
}

func TestSessionClockOnAdmission(t *testing.T) {
	for _, fail := range []bool{false, true} {
		store := &captureStore{}
		offset := int64(321)
		var calledAt time.Time
		clock := clockFunc(func(_ context.Context, room string, at time.Time) (*int64, error) {
			if room != "room/session" {
				t.Fatal(room)
			}
			calledAt = at
			if fail {
				return &offset, errors.New("clock offline")
			}
			return &offset, nil
		})
		h := Hub{life: t.Context(), clock: clock, log: slog.Default(), store: store, sendCapacity: 1}
		c := h.NewClient(7, "room/session", nil)
		h.handleDanmaku(c, []byte(`{"content":"hello"}`))
		if calledAt.IsZero() || calledAt != store.msg.SendTime || store.msg.Content != "hello" {
			t.Fatal(store.msg)
		}
		got := store.msg.VideoOffsetMS
		if fail && got != nil || !fail && (got == nil || *got != offset) {
			t.Fatal(got)
		}
		packets, err := protocol.DecodePackets((<-c.send).payload)
		var receipt protocol.Receipt
		if err != nil || len(packets) != 1 || json.Unmarshal(packets[0].Data, &receipt) != nil || receipt.Status != "accepted" {
			t.Fatal(receipt, err)
		}
	}
}
