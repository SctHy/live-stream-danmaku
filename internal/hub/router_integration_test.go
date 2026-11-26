package hub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/scthy/maku/internal/events"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

type lostReplyStore struct {
	*pubsub.Store
	attempts int
	original []byte
	id       string
}

func (s *lostReplyStore) AcceptDanmakuPacket(ctx context.Context, msg protocol.DanmakuMessage, wire []byte) (string, string, []byte, error) {
	s.attempts++
	if s.attempts == 2 {
		return "unavailable", "", nil, context.DeadlineExceeded
	}
	status, id, original, err := s.Store.AcceptDanmakuPacket(ctx, msg, wire)
	if err == nil && status == "new" {
		s.original = bytes.Clone(wire)
		s.id = id
		return "unavailable", "", nil, context.DeadlineExceeded
	}
	return status, id, original, err
}
func TestRetryArchivesOriginalAfterLostReply(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := &lostReplyStore{Store: pubsub.Open(addr)}
	defer s.Close()
	bus := new(events.Bus)
	var archived []byte
	bus.Danmaku.Subscribe(func(_ context.Context, e events.DanmakuAccepted) {
		if !e.Duplicate || e.EventID != s.id {
			t.Fatal(e)
		}
		archived = e.Wire()
	})
	h := Hub{life: t.Context(), store: s, bus: bus, log: slog.Default(), sendCapacity: 1}
	c := h.NewClient(^uint64(0), "retry-"+rand.Text(), nil)
	for _, want := range []string{"unavailable", "accepted"} {
		h.handleDanmaku(c, []byte(`{"content":"hello","client_msg_id":"same"}`))
		packets, err := protocol.DecodePackets((<-c.send).payload)
		var receipt protocol.Receipt
		if err != nil || len(packets) != 1 || json.Unmarshal(packets[0].Data, &receipt) != nil || receipt.Status != want || want == "accepted" && receipt.EventID != s.id {
			t.Fatal(receipt, err)
		}
	}
	if !bytes.Equal(archived, s.original) || !bytes.Contains(archived, []byte("18446744073709551615")) {
		t.Fatal(string(archived))
	}
}
