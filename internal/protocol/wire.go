package protocol

import (
	"bytes"
	"encoding/json"
)

const BatchProtocol = "danmaku.batch.v1"

// DecodePackets accepts both legacy single packets and negotiated arrays.
func DecodePackets(data []byte) ([]WsPacket, error) {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '[' {
		var packets []WsPacket
		err := json.Unmarshal(data, &packets)
		return packets, err
	}
	var packet WsPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, err
	}
	return []WsPacket{packet}, nil
}
