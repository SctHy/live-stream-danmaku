package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/protocol"
	"github.com/scthy/maku/internal/pubsub"
)

// Forward one complete RESP command per delay, independent of TCP fragmentation.
func forwardRequests(dst io.Writer, src io.Reader, delay time.Duration) error {
	r := bufio.NewReader(src)
	w := bufio.NewWriter(dst)
	header := func(kind byte) (int, error) {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		if len(line) < 4 || line[0] != kind || !strings.HasSuffix(line, "\r\n") {
			return 0, errors.New("invalid RESP header")
		}
		n, err := strconv.Atoi(line[1 : len(line)-2])
		if err != nil || n < 0 {
			return 0, errors.New("invalid RESP length")
		}
		_, err = w.WriteString(line)
		return n, err
	}
	for {
		n, e := header('*')
		if e != nil {
			return e
		}
		time.Sleep(delay)
		for range n {
			size, e := header('$')
			if e != nil {
				return e
			}
			if _, e = io.CopyN(w, r, int64(size)+2); e != nil {
				return e
			}
		}
		if e = w.Flush(); e != nil {
			return e
		}
	}
}
func delayProxy(addr string, delay time.Duration) (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			client, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer client.Close()
				up, err := net.DialTimeout("tcp", addr, 2*time.Second)
				if err != nil {
					return
				}
				defer up.Close()
				done := make(chan struct{})
				go func() { io.Copy(client, up); client.Close(); close(done) }()
				forwardRequests(up, client, delay)
				up.Close()
				<-done
			}()
		}
	}()
	return ln.Addr().String(), func() { ln.Close(); wg.Wait() }, nil
}
func multiAccept(ctx context.Context, r *redis.Client, m protocol.DanmakuMessage, payload string) (string, error) {
	prefix := "dm:{" + m.RoomID + "}:"
	user := strconv.FormatUint(m.UserID, 10)
	key := prefix + "send:" + user + ":" + m.ClientMsgID
	_, err := r.Get(ctx, key).Result()
	if err == nil {
		return "accepted", nil
	}
	if err != redis.Nil {
		return "", err
	}
	muted, err := r.Exists(ctx, prefix+"mute:"+user).Result()
	if err != nil || muted != 0 {
		return "muted", err
	}
	rate := fmt.Sprintf("%srate:%s:%d", prefix, user, protocol.TypeDanmaku)
	n, err := r.Incr(ctx, rate).Result()
	if err != nil {
		return "", err
	}
	if n == 1 {
		if err = r.PExpire(ctx, rate, time.Second).Err(); err != nil {
			return "", err
		}
	}
	if n > 8 {
		return "rate_limited", nil
	}
	stream := prefix + "recent"
	tail, err := r.XRevRangeN(ctx, stream, "+", "-", 1).Result()
	if err != nil {
		return "", err
	}
	epoch, prev := rand.Text(), "0-0"
	if len(tail) > 0 {
		epoch = tail[0].Values["epoch"].(string)
		prev = pubsub.StreamPosition(tail[0])
	}
	id, err := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, MaxLen: 256, Approx: true, Values: []any{"packet", payload, "prev", prev, "epoch", epoch}}).Result()
	if err != nil {
		return "", err
	}
	if err = r.Expire(ctx, stream, 120*time.Second).Err(); err != nil {
		return "", err
	}
	wire := fmt.Sprintf(`{"pos":%q,"prev":%q,"packet":%s}`, epoch+"@"+id, prev, payload)
	if err = r.Publish(ctx, prefix+"pubsub", wire).Err(); err != nil {
		return "", err
	}
	return "new", r.Set(ctx, key, m.EventID, 600*time.Second).Err()
}
func (s *p1Suite) redisSuite(addr string) error {
	for _, delay := range []time.Duration{0, 2 * time.Millisecond} {
		if err := s.redisLoad(addr, delay); err != nil {
			return err
		}
	}
	return nil
}
func (s *p1Suite) redisLoad(addr string, delay time.Duration) (result error) {
	proxy, stop, err := delayProxy(addr, delay)
	if err != nil {
		return err
	}
	defer stop()
	client := redis.NewClient(&redis.Options{Addr: proxy, MaxRetries: -1})
	defer client.Close()
	prod := pubsub.Open(proxy)
	defer prod.Close()
	control := redis.NewClient(&redis.Options{Addr: addr})
	defer control.Close()
	prefix := "p1-" + rand.Text()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		var cursor uint64
		var keys []string
		for {
			found, next, e := control.Scan(ctx, cursor, "dm:{"+prefix+"-*}:*", 500).Result()
			if e != nil {
				result = errors.Join(result, e)
				return
			}
			keys = append(keys, found...)
			cursor = next
			if next == 0 {
				break
			}
		}
		if len(keys) > 0 {
			result = errors.Join(result, control.Del(ctx, keys...).Err())
		}
	}()
	return s.run("redis_lua_atomicity", delay.String(), "Lua serializes policy and publication on the Redis server within one round trip.", []string{"multi_cmd_baseline", "lua_evalsha_prod"}, func(arm string) (measurement, error) {
		m := newMeasurement()
		room := prefix + "-" + rand.Text()
		accept := func(room string, user uint64) (string, error) {
			msg := protocol.DanmakuMessage{RoomID: room, UserID: user, ClientMsgID: "request", EventID: rand.Text()}
			const payload = `{"type":1,"data":{}}`
			if arm == "multi_cmd_baseline" {
				return multiAccept(s.ctx, client, msg, payload)
			}
			state, _, e := prod.AcceptDanmaku(s.ctx, msg, []byte(payload))
			return state, e
		}
		_, err := accept(room, 9999)
		if err != nil {
			return m, err
		}
		var times []float64
		start := time.Now()
		ok := true
		for i := range 100 {
			t := time.Now()
			state, e := accept(room, uint64(i+1))
			times = append(times, micros(t))
			if e != nil {
				return m, e
			}
			ok = ok && state == "new"
		}
		m.Metrics["qps"] = 100 / time.Since(start).Seconds()
		latency(&m, "", times)
		gate(&m, "fresh_accepted", ok)
		var writes, broadcasts int64
		const trials = 5
		for range trials {
			trialRoom := prefix + "-" + rand.Text()
			channel := fmt.Sprintf(pubsub.KeyRoomPubSub, trialRoom)
			count, published, err := raceTrial(s.ctx, control, trialRoom, channel, func() error { _, e := accept(trialRoom, 1); return e })
			if err != nil {
				return m, err
			}
			writes += count
			broadcasts += published
			ok = ok && count >= 1 && published == count
		}
		extra := writes - trials
		m.Metrics["stream_writes"] = float64(writes)
		m.Metrics["extra_broadcasts"] = float64(broadcasts - trials)
		m.Metrics["race_duplicate_rate"] = float64(extra) / (32 * trials)
		gate(&m, "broadcasts_match_stream", ok)
		if arm == "lua_evalsha_prod" {
			gate(&m, "one_write_per_request", extra == 0)
		}
		return m, nil
	})
}
func raceTrial(ctx context.Context, c *redis.Client, room, channel string, accept func() error) (int64, int64, error) {
	sub := c.Subscribe(ctx, channel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		return 0, 0, err
	}
	errs := make([]error, 32)
	burst(32, func(i int) { errs[i] = accept() })
	if e := errors.Join(errs...); e != nil {
		return 0, 0, e
	}
	if err := c.Publish(ctx, channel, "end").Err(); err != nil {
		return 0, 0, err
	}
	var broadcasts int64
	for {
		msg, err := sub.ReceiveTimeout(ctx, 5*time.Second)
		if err != nil {
			return 0, 0, err
		}
		if m, ok := msg.(*redis.Message); ok {
			if m.Payload == "end" {
				break
			}
			broadcasts++
		}
	}
	n, err := c.XLen(ctx, "dm:{"+room+"}:recent").Result()
	return n, broadcasts, err
}
