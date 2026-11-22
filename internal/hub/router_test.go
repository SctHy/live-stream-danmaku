package hub

import (
	"context"
	"encoding/json"
	"testing"

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

func (s *captureStore) AcceptDanmaku(_ context.Context, msg protocol.DanmakuMessage, wire []byte) (string, string, error) {
	s.msg = msg
	return "accepted", msg.EventID, nil
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
