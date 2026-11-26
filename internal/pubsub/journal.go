package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"slices"
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

var journalOnce = redis.NewScript(`local last=redis.call('GET',KEYS[2])
if last and (#last>#ARGV[3] or (#last==#ARGV[3] and last>=ARGV[3])) then return 0 end
` + journalLua + `redis.call('SET',KEYS[2],ARGV[3])
return 1`)

// The room dispatcher processes audits in order. A repeated delivery must keep its cursor.
func (s *Store) JournalOnce(ctx context.Context, room string, payload []byte, auditID int64) error {
	prefix := sessionPrefix(room)
	return journalOnce.Run(ctx, s.r, []string{prefix + "recent", prefix + "last_control"}, payload, rand.Text(), auditID, fmt.Sprintf(KeyRoomPubSub, room)).Err()
}

const guardedAppendLua = `
local state=redis.call('MGET',KEYS[2],KEYS[3],KEYS[5])
if state[1] then
 -- Older gateways store only the ID. Recover their packet while it is retained.
 local packet=redis.call('GET',KEYS[8])
 if packet then return {'duplicate',state[1],packet} end
 for _,row in ipairs(redis.call('XREVRANGE',KEYS[1],'+','-')) do
  local raw=row[2][2]
  local ok,msg=pcall(cjson.decode,raw)
  if ok and type(msg)=='table' and type(msg.data)=='table' and msg.data.event_id==state[1] and msg.type==101 then
   return {'duplicate',state[1],raw}
  end
 end
 return {'duplicate',state[1]}
end
if state[2] then return {'denied',''} end
local slow=0
if state[3] and not (ARGV[4]=='1' and redis.call('SISMEMBER',KEYS[6],ARGV[5])==1) then
 slow=tonumber(state[3])
 if slow<0 then return {'gate_closed',''} end
 -- A fixed lifetime lets a changed policy take effect on the next send.
 local ttl=redis.call('PTTL',KEYS[7])
 if ttl>0 and ttl>(tonumber(ARGV[6])-slow)*1000 then return {'too_soon',''} end
end
local n=redis.call('INCR',KEYS[4])
if n==1 then redis.call('PEXPIRE',KEYS[4],ARGV[7]) end
if n>tonumber(ARGV[8]) then return {'rate_limited',''} end
if slow>0 then redis.call('SET',KEYS[7],'1','EX',ARGV[6]) end
` + journalLua + `
redis.call('SET',KEYS[2],ARGV[3],'EX',ARGV[9])
redis.call('SET',KEYS[8],ARGV[1],'EX',ARGV[9])
return {'new',ARGV[3]}
`

// GuardSpec names co-located keys for one ordered stream and its admission policy.
// A negative gate value closes admission. Positive values set a per-member
// interval in seconds, bounded by GateWindowSeconds.
type GuardSpec struct {
	StreamKey, DedupKey, DenyKey, RateKey                    string
	GateKey, ExemptSetKey, LastAdmittedKey                   string
	Channel, Member                                          string
	CheckExemption                                           bool
	GateWindowSeconds, RateWindowMS, RateLimit, DedupSeconds int
}

func (s *Store) AppendWithGuards(ctx context.Context, g GuardSpec, eventID string, payload []byte) (string, string, error) {
	status, id, _, err := s.appendWithGuards(ctx, g, eventID, payload)
	return status, id, err
}

func (s *Store) appendWithGuards(ctx context.Context, g GuardSpec, eventID string, payload []byte) (string, string, []byte, error) {
	if strings.TrimSpace(g.Channel) == "" || strings.TrimSpace(eventID) == "" || !json.Valid(payload) {
		return "unavailable", "", nil, fmt.Errorf("invalid channel, event ID or packet")
	}
	// Reject invalid expirations before the script can publish a partial append.
	if g.DedupSeconds <= 0 || g.GateWindowSeconds <= 0 || g.RateWindowMS <= 0 || g.RateLimit <= 0 {
		return "unavailable", "", nil, fmt.Errorf("guard windows and rate limit must be positive")
	}
	keys := []string{g.StreamKey, g.DedupKey, g.DenyKey, g.RateKey, g.GateKey, g.ExemptSetKey, g.LastAdmittedKey, "packet:" + g.DedupKey}
	for i, key := range keys {
		if strings.TrimSpace(key) == "" || slices.Contains(keys[:i], key) {
			return "unavailable", "", nil, fmt.Errorf("guard keys must be nonempty and distinct")
		}
	}
	result, err := s.guardedAppend.Run(ctx, s.r, keys, payload, rand.Text(), eventID, g.CheckExemption, g.Member,
		g.GateWindowSeconds, g.RateWindowMS, g.RateLimit, g.DedupSeconds, g.Channel).StringSlice()
	if err != nil {
		return "unavailable", "", nil, err
	}
	if len(result) == 3 {
		return result[0], result[1], []byte(result[2]), nil
	}
	return result[0], result[1], nil, nil
}

func (s *Store) AcceptDanmaku(ctx context.Context, msg protocol.DanmakuMessage, payload []byte) (string, string, error) {
	status, id, _, err := s.AcceptDanmakuPacket(ctx, msg, payload)
	return status, id, err
}

func (s *Store) AcceptDanmakuPacket(ctx context.Context, msg protocol.DanmakuMessage, payload []byte) (string, string, []byte, error) {
	prefix := sessionPrefix(msg.RoomID)
	user := fmt.Sprint(msg.UserID)
	request := msg.ClientMsgID
	if request == "" {
		request = msg.EventID
	}
	slug, _, _ := strings.Cut(msg.RoomID, "/")
	roomPrefix := sessionPrefix(slug)
	g := GuardSpec{
		StreamKey: prefix + "recent", DedupKey: prefix + "send:" + user + ":" + request, DenyKey: prefix + "mute:" + user,
		RateKey: prefix + fmt.Sprintf("rate:%s:%d", user, protocol.TypeDanmaku),
		GateKey: roomPrefix + "gate", ExemptSetKey: roomPrefix + "staff", LastAdmittedKey: roomPrefix + "slow:" + user,
		Channel: fmt.Sprintf(KeyRoomPubSub, msg.RoomID), Member: user, CheckExemption: msg.Nickname != "",
		GateWindowSeconds: 300, RateWindowMS: 1000, RateLimit: 8, DedupSeconds: 600,
	}
	status, id, original, err := s.appendWithGuards(ctx, g, msg.EventID, payload)
	switch status {
	case "duplicate":
		status = "accepted"
	case "denied":
		status = "muted"
	case "gate_closed":
		status = "room_muted"
	case "too_soon":
		status = "slow_mode"
	}
	return status, id, original, err
}

type Recovery struct {
	Events   []protocol.WsPacket `json:"events"`
	Position string              `json:"pos"`
	Reset    bool                `json:"reset"`
}

func (s *Store) Recover(ctx context.Context, room, after string) (Recovery, error) {
	return s.RecoverWithFilter(ctx, room, after, WithdrawFilter)
}

// RecoveryFilter inspects the whole retained window and may rewrite packets
// from start onward. Cursor positions and ordering must be preserved.
type RecoveryFilter func(events []protocol.WsPacket, start int) error

func (s *Store) RecoverWithFilter(ctx context.Context, room, after string, filter RecoveryFilter) (Recovery, error) {
	r := s.r
	entries, err := r.XRange(ctx, sessionPrefix(room)+"recent", "-", "+").Result()
	out := Recovery{Events: []protocol.WsPacket{}, Position: "0-0", Reset: after != "" && after != "0-0"}
	if err != nil {
		return out, err
	}
	start := 0
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
		if packet.Position == after {
			start = len(out.Events)
			out.Reset = false
		}
	}
	if filter != nil {
		if err := filter(out.Events, start); err != nil {
			return out, err
		}
	}
	out.Events = out.Events[start:]
	return out, nil
}

func WithdrawFilter(events []protocol.WsPacket, start int) error {
	hidden := make(map[string]bool)
	for _, packet := range events {
		if packet.Type == protocol.TypeWithdraw {
			var d struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(packet.Data, &d); err != nil {
				return err
			}
			hidden[d.EventID] = true
		}
	}
	// A withdrawal can arrive before the archive writer flushes the original event.
	// Keep the original cursor recoverable while suppressing its withdrawn text.
	if len(hidden) > 0 {
		for i := range events[start:] {
			p := &events[start+i]
			if p.Type != protocol.TypeDanmaku {
				continue
			}
			var ref struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(p.Data, &ref); err != nil {
				return err
			}
			if hidden[ref.EventID] {
				p.Type = protocol.TypeWithdraw
				data, err := json.Marshal(ref)
				if err != nil {
					return err
				}
				p.Data = data
			}
		}
	}
	return nil
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
