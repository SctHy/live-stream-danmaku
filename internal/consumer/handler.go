package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/charlesAcmen/livestream-danmaku/internal/model"
	"go.uber.org/zap"
)

const FlushInterval = 2 * time.Second

type batchStore interface {
	SaveBatch(context.Context, []*model.DanmakuMessage) error
}

type DanmakuHandler struct {
	repo      batchStore
	batchSize int
}

func NewDanmakuHandler(r batchStore, size int) *DanmakuHandler {
	return &DanmakuHandler{repo: r, batchSize: size}
}

func (*DanmakuHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (*DanmakuHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// Each claim owns one partition. Only advance its offset after all prior
// valid records in this partition are durable.
func (h *DanmakuHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	batch := make([]*model.DanmakuMessage, 0, h.batchSize)
	var last *sarama.ConsumerMessage
	flush := func() error {
		if last == nil {
			return nil
		}
		for len(batch) > 0 {
			err := h.repo.SaveBatch(ctx, batch)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logger.Log.Warn("History batch pending retry", zap.Error(err), zap.Int32("partition", last.Partition), zap.Int64("last_offset", last.Offset))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
		session.MarkMessage(last, "")
		batch = batch[:0]
		last = nil
		return nil
	}
	tick := time.NewTicker(FlushInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil // unmarked records replay after rebalance
		case <-tick.C:
			if err := flush(); err != nil {
				return err
			}
		case msg, ok := <-claim.Messages():
			if !ok {
				return flush()
			}
			last = msg
			var p model.WsPacket
			var d model.DanmakuMessage
			if err := json.Unmarshal(msg.Value, &p); err != nil || p.Type != model.TypeDanmaku || json.Unmarshal(p.Data, &d) != nil || !d.FitsHistory() {
				logger.Log.Warn("Skipping invalid history record", zap.Int32("partition", msg.Partition), zap.Int64("offset", msg.Offset))
				continue
			}
			if d.EventID == "" {
				d.EventID = fmt.Sprintf("kafka:%s:%d:%d", msg.Topic, msg.Partition, msg.Offset)
			}
			batch = append(batch, &d)
			if len(batch) >= h.batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}
