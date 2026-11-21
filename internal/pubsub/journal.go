package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/live-stream-danmaku/internal/protocol"
)

// Keep the recovery cursor and live publish in one Redis operation across gateways.
// Preserve the original JSON bytes, including integer user IDs.
const journalLua = `
local tail=redis.call('XREVRANGE',KEYS[1],'+','-','COUNT',1)
local prev='0-0'
local epoch=ARGV[2]
if #tail>0 then
 epoch=tail[1][2][6] or ARGV[2]
 prev=epoch..'@'..tail[1][1]
end
local id=epoch..'@'..redis.call('XADD',KEYS[1],'MAXLEN','~',256,'*','packet',ARGV[1],'prev',prev,'epoch',epoch)
redis.call('EXPIRE',KEYS[1],120)
local wire='{"pos":"'..id..'","prev":"'..prev..'","packet":'..ARGV[1]..'}'
redis.call('PUBLISH',KEYS[2],wire)
`

const appendEventLua = journalLua + `return id`

func (s *Store) Journal(ctx context.Context, room string, payload []byte) error {
	r := s.r
	return s.appendEvent.Run(ctx, r, []string{"dm:{" + room + "}:recent", fmt.Sprintf(KeyRoomPubSub, room)}, string(payload), rand.Text()).Err()
}

const acceptEventLua = `
local existing=redis.call('GET',KEYS[3])
if existing then return {'accepted',existing} end
if redis.call('EXISTS',KEYS[4])==1 then return {'muted',''} end
local n=redis.call('INCR',KEYS[5])
if n==1 then redis.call('PEXPIRE',KEYS[5],1000) end
if n>8 then return {'rate_limited',''} end
` + journalLua + `
redis.call('SET',KEYS[3],ARGV[3],'EX',600)
return {'new',ARGV[3]}
`

func (s *Store) AcceptDanmaku(ctx context.Context, msg protocol.DanmakuMessage, payload []byte) (string, string, error) {
	r := s.r
	prefix := fmt.Sprintf("dm:{%s}:", msg.RoomID)
	user := fmt.Sprint(msg.UserID)
	request := msg.ClientMsgID
	if request == "" {
		request = msg.EventID
	}
	keys := []string{
		"dm:{" + msg.RoomID + "}:recent",
		fmt.Sprintf(KeyRoomPubSub, msg.RoomID),
		prefix + "send:" + user + ":" + request,
		prefix + "mute:" + user,
		prefix + fmt.Sprintf("rate:%s:%d", user, protocol.TypeDanmaku),
	}
	result, err := s.acceptEvent.Run(ctx, r, keys, payload, rand.Text(), msg.EventID).StringSlice()
	if err != nil {
		return "unavailable", "", err
	}
	return result[0], result[1], nil
}

type Recovery struct {
	Events   []protocol.WsPacket `json:"events"`
	Position string              `json:"pos"`
	Reset    bool                `json:"reset"`
}

func (s *Store) Recover(ctx context.Context, room, after string) (Recovery, error) {
	r := s.r
	entries, err := r.XRange(ctx, "dm:{"+room+"}:recent", "-", "+").Result()
	out := Recovery{Events: []protocol.WsPacket{}, Position: "0-0", Reset: after != "" && after != "0-0"}
	if err != nil {
		return out, err
	}
	hidden := make(map[string]bool)
	for i, entry := range entries {
		if i == 0 && after == "0-0" && entry.Values["prev"] != "0-0" {
			out.Reset = true
		}
		var packet protocol.WsPacket
		raw, ok := entry.Values["packet"].(string)
		if !ok {
			return out, fmt.Errorf("missing journal payload")
		}
		if err := json.Unmarshal([]byte(raw), &packet); err != nil {
			return out, err
		}
		packet.Position = StreamPosition(entry)
		packet.Previous = out.Position
		out.Position = packet.Position
		out.Events = append(out.Events, packet)
		if packet.Type == protocol.TypeWithdraw {
			var d struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(packet.Data, &d); err != nil {
				return out, err
			}
			hidden[d.EventID] = true
		}
		if packet.Position == after {
			out.Events = out.Events[:0]
			out.Reset = false
		}
	}
	// A withdrawal can arrive before the archive writer flushes the original event.
	// Keep the original cursor recoverable while suppressing its withdrawn text.
	if len(hidden) > 0 {
		for i := range out.Events {
			p := &out.Events[i]
			if p.Type != protocol.TypeDanmaku {
				continue
			}
			var ref struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(p.Data, &ref); err != nil {
				return out, err
			}
			if hidden[ref.EventID] {
				p.Type = protocol.TypeWithdraw
				p.Data, _ = json.Marshal(ref)
			}
		}
	}
	return out, nil
}
func StreamPosition(entry redis.XMessage) string {
	epoch, _ := entry.Values["epoch"].(string)
	if epoch == "" {
		return entry.ID
	}
	return epoch + "@" + entry.ID
}

const allowActionLua = `
if redis.call('EXISTS',KEYS[1])==1 then return 'muted' end
local n=redis.call('INCR',KEYS[2])
if n==1 then redis.call('PEXPIRE',KEYS[2],ARGV[1]) end
if n>tonumber(ARGV[2]) then return 'rate_limited' end
return 'ok'`

func (s *Store) Policy(ctx context.Context, room string, uid uint64, action int) (string, error) {
	r := s.r
	prefix := fmt.Sprintf("dm:{%s}:", room)
	user := fmt.Sprint(uid)
	return s.allowAction.Run(ctx, r, []string{prefix + "mute:" + user, prefix + fmt.Sprintf("rate:%s:%d", user, action)}, time.Second.Milliseconds(), 8).Text()
}
