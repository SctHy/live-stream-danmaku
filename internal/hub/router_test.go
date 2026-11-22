package hub

import (
	"testing"

	"github.com/scthy/maku/internal/protocol"
)

func TestInvalidInputReturnsReceipt(t *testing.T) {
	m := Hub{sendCapacity: 1}
	c := m.NewClient(42, "room", nil)
	m.dispatch(c, protocol.WsPacket{Type: protocol.TypeDanmaku})
	f := <-c.Send
	packets, err := protocol.DecodePackets(f.payload)
	if err != nil || packets[0].Type != protocol.TypeReceipt {
		t.Fatal(string(f.payload), err)
	}
}
