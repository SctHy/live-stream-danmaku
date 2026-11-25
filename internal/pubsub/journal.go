package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
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
redis.call('PUBLISH',ARGV[#ARGV],wire)
`

const appendEventLua = journalLua + `return id`

func (s *Store) Journal(ctx context.Context, room string, payload []byte) error {
	r := s.r
	return s.appendEvent.Run(ctx, r, []string{sessionPrefix(room) + "recent"}, string(payload), rand.Text(), fmt.Sprintf(KeyRoomPubSub, room)).Err()
}

var journalOnce = redis.NewScript(`if redis.call('GET',KEYS[2])==ARGV[3] then return 0 end
` + journalLua + `redis.call('SET',KEYS[2],ARGV[3])
return 1`)

// The room dispatcher processes audits in order. A repeated delivery must keep its cursor.
func (s *Store) JournalOnce(ctx context.Context, room string, payload []byte, auditID int64) error {
	prefix := sessionPrefix(room)
	return journalOnce.Run(ctx, s.r, []string{prefix + "recent", prefix + "last_control"}, payload, rand.Text(), auditID, fmt.Sprintf(KeyRoomPubSub, room)).Err()
}

const acceptEventLua = `
local state=redis.call('MGET',KEYS[2],KEYS[3],KEYS[5])
if state[1] then return {'accepted',state[1]} end
if state[2] then return {'muted',''} end
local slow=0
if state[3] and not (ARGV[4]=='1' and redis.call('SISMEMBER',KEYS[6],ARGV[5])==1) then
 slow=tonumber(state[3])
 if slow<0 then return {'room_muted',''} end
 -- A fixed 300s lifetime lets a changed policy take effect on the next send.
 if redis.call('PTTL',KEYS[7])>(300-slow)*1000 then return {'slow_mode',''} end
end
local n=redis.call('INCR',KEYS[4])
if n==1 then redis.call('PEXPIRE',KEYS[4],1000) end
if n>8 then return {'rate_limited',''} end
if slow>0 then redis.call('SET',KEYS[7],'1','EX',300) end
` + journalLua + `
redis.call('SET',KEYS[2],ARGV[3],'EX',600)
return {'new',ARGV[3]}
`

func (s *Store) AcceptDanmaku(ctx context.Context, msg protocol.DanmakuMessage, payload []byte) (string, string, error) {
	r := s.r
	prefix := sessionPrefix(msg.RoomID)
	user := fmt.Sprint(msg.UserID)
	request := msg.ClientMsgID
	if request == "" {
		request = msg.EventID
	}
	slug, _, _ := strings.Cut(msg.RoomID, "/")
	roomPrefix := sessionPrefix(slug)
	keys := []string{
		prefix + "recent",
		prefix + "send:" + user + ":" + request,
		prefix + "mute:" + user,
		prefix + fmt.Sprintf("rate:%s:%d", user, protocol.TypeDanmaku),
		roomPrefix + "gate", roomPrefix + "staff", roomPrefix + "slow:" + user,
	}
	result, err := s.acceptEvent.Run(ctx, r, keys, payload, rand.Text(), msg.EventID, msg.Nickname != "", user, fmt.Sprintf(KeyRoomPubSub, msg.RoomID)).StringSlice()
	if err != nil {
		return "unavailable", "", err
	}
	if result[0] == "new" {
		if err := s.addHeat(ctx, slug, 1); err != nil {
			slog.Warn("Record danmaku heat", "error", err)
		}
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
	entries, err := r.XRange(ctx, sessionPrefix(room)+"recent", "-", "+").Result()
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
				p.Data, err = json.Marshal(ref)
				if err != nil {
					return out, err
				}
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
	prefix := sessionPrefix(room)
	user := fmt.Sprint(uid)
	return s.allowAction.Run(ctx, r, []string{prefix + "mute:" + user, prefix + fmt.Sprintf("rate:%s:%d", user, action)}, time.Second.Milliseconds(), 8).Text()
}
