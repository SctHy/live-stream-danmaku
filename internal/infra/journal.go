package infra

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
local id=epoch..'@'..redis.call('XADD',KEYS[1],'MAXLEN',256,'*','packet',ARGV[1],'prev',prev,'epoch',epoch)
redis.call('EXPIRE',KEYS[1],120)
local wire='{"pos":"'..id..'","prev":"'..prev..'",'..string.sub(ARGV[1],2)
redis.call('PUBLISH',KEYS[2],wire)
`

var appendEvent = redis.NewScript(journalLua + `return id`)

func Journal(ctx context.Context, r *redis.Client, room string, payload []byte) error {
	return appendEvent.Run(ctx, r, []string{"dm:recent:" + room, fmt.Sprintf(KeyRoomPubSub, room)}, string(payload), rand.Text()).Err()
}

// Session sends append an archive obligation in the same operation as the live
// event. Reject new messages before publishing when the archive backlog is full.
const archiveStream = "dm:archive:pending"

var acceptEvent = redis.NewScript(`
local existing=redis.call('GET',KEYS[3])
if existing then return {'accepted',existing} end
if redis.call('EXISTS',KEYS[4])==1 then return {'muted',''} end
if redis.call('XLEN',KEYS[6])>=65536 then return {'overloaded',''} end
local n=redis.call('INCR',KEYS[5])
if n==1 then redis.call('PEXPIRE',KEYS[5],1000) end
if n>8 then return {'rate_limited',''} end
` + journalLua + `
redis.call('XADD',KEYS[6],'*','packet',ARGV[1])
redis.call('SET',KEYS[3],ARGV[3],'EX',600)
return {'accepted',ARGV[3]}
`)

func AcceptDanmaku(ctx context.Context, r *redis.Client, msg protocol.DanmakuMessage, payload []byte) (string, string, error) {
	user := fmt.Sprintf("%s:%d", msg.RoomID, msg.UserID)
	request := msg.ClientMsgID
	if request == "" {
		request = msg.EventID
	}
	keys := []string{
		"dm:recent:" + msg.RoomID,
		fmt.Sprintf(KeyRoomPubSub, msg.RoomID),
		"dm:send:" + user + ":" + request,
		"dm:mute:" + user,
		fmt.Sprintf("dm:rate:%s:%d", user, protocol.TypeDanmaku),
		archiveStream,
	}
	result, err := acceptEvent.Run(ctx, r, keys, payload, rand.Text(), msg.EventID).StringSlice()
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

func Recover(ctx context.Context, r *redis.Client, room, after string) (Recovery, error) {
	entries, err := r.XRange(ctx, "dm:recent:"+room, "-", "+").Result()
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

var allowAction = redis.NewScript(`
if redis.call('EXISTS',KEYS[1])==1 then return 'muted' end
local n=redis.call('INCR',KEYS[2])
if n==1 then redis.call('PEXPIRE',KEYS[2],ARGV[1]) end
if n>tonumber(ARGV[2]) then return 'rate_limited' end
return 'ok'`)

func Policy(ctx context.Context, r *redis.Client, room string, uid uint64, action int) (string, error) {
	user := fmt.Sprintf("%s:%d", room, uid)
	return allowAction.Run(ctx, r, []string{"dm:mute:" + user, fmt.Sprintf("dm:rate:%s:%d", user, action)}, time.Second.Milliseconds(), 8).Text()
}
