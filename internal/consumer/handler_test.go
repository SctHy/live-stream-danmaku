package consumer

import (
	"context"
	"errors"
	"github.com/IBM/sarama"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
	"testing"
)

type testClaim struct {
	sarama.ConsumerGroupClaim
	records chan *sarama.ConsumerMessage
}

func (c testClaim) Messages() <-chan *sarama.ConsumerMessage { return c.records }

type testSession struct {
	sarama.ConsumerGroupSession
	ctx    context.Context
	marked []int64
}

func (s *testSession) Context() context.Context { return s.ctx }
func (s *testSession) MarkMessage(m *sarama.ConsumerMessage, _ string) {
	s.marked = append(s.marked, m.Offset)
}

type writeBatch func(context.Context, []*protocol.DanmakuMessage) error

func (f writeBatch) SaveBatch(ctx context.Context, b []*protocol.DanmakuMessage) error {
	return f(ctx, b)
}

func TestClaimReplaysUnconfirmedBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &testSession{ctx: ctx}
	claim := func() testClaim {
		records := make(chan *sarama.ConsumerMessage, 2)
		records <- &sarama.ConsumerMessage{Topic: "history", Partition: 1, Offset: 10, Value: []byte(`{"type":101,"data":{"content":"legacy"}}`)}
		records <- &sarama.ConsumerMessage{Topic: "history", Partition: 1, Offset: 11, Value: []byte(`{"type":101,"data":{"event_id":"stable"}}`)}
		close(records)
		return testClaim{records: records}
	}
	store := writeBatch(func(context.Context, []*protocol.DanmakuMessage) error {
		cancel()
		return errors.New("connection lost before acknowledgement")
	})
	err := NewDanmakuHandler(store, 2).ConsumeClaim(session, claim())
	if !errors.Is(err, context.Canceled) || len(session.marked) != 0 {
		t.Fatalf("err=%v marked=%v", err, session.marked)
	}
	session.ctx = context.Background()
	store = func(_ context.Context, batch []*protocol.DanmakuMessage) error {
		if len(batch) != 2 || batch[0].EventID != "kafka:history:1:10" || batch[1].EventID != "stable" {
			t.Fatalf("unexpected replay: %v", batch)
		}
		if len(session.marked) != 0 {
			t.Fatal("offset advanced before transaction")
		}
		return nil
	}
	if err := NewDanmakuHandler(store, 2).ConsumeClaim(session, claim()); err != nil {
		t.Fatal(err)
	}
	if len(session.marked) != 1 || session.marked[0] != 11 {
		t.Fatal(session.marked)
	}
}
