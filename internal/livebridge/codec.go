// Package livebridge receives public chat events from Bilibili live rooms.
package livebridge

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/scthy/maku/internal/protocol"
)

const maxPayload = 4 << 20

func packet(version uint16, op uint32, body []byte) []byte {
	b := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	binary.BigEndian.PutUint16(b[4:], 16)
	binary.BigEndian.PutUint16(b[6:], version)
	binary.BigEndian.PutUint32(b[8:], op)
	binary.BigEndian.PutUint32(b[12:], 1)
	copy(b[16:], body)
	return b
}

type visitPacket func(uint32, []byte) error

func decode(data []byte, visit visitPacket) error {
	budget := maxPayload
	return packets(data, &budget, true, visit)
}
func packets(data []byte, budget *int, outer bool, visit visitPacket) error {
	if len(data) == 0 {
		return io.ErrUnexpectedEOF
	}
	for len(data) > 0 {
		if len(data) < 16 {
			return io.ErrUnexpectedEOF
		}
		size := uint64(binary.BigEndian.Uint32(data))
		if binary.BigEndian.Uint16(data[4:]) != 16 || size < 16 || size > uint64(len(data)) {
			return errors.New("invalid live packet length")
		}
		version := binary.BigEndian.Uint16(data[6:])
		op := binary.BigEndian.Uint32(data[8:])
		body := data[16:size]
		data = data[size:]
		switch {
		case version == 2 && op == 5:
			if !outer {
				return errors.New("nested compressed packet")
			}
			r, err := zlib.NewReader(bytes.NewReader(body))
			if err != nil {
				return err
			}
			plain, err := io.ReadAll(io.LimitReader(r, int64(*budget)+1))
			closeErr := r.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			*budget -= len(plain)
			if *budget < 0 {
				return errors.New("live packet exceeds decompression limit")
			}
			if err := packets(plain, budget, false, visit); err != nil {
				return err
			}
		case version <= 1:
			if err := visit(op, body); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseMessage(body []byte) (protocol.LiveDanmaku, bool) {
	var event struct {
		Cmd  string
		Info []json.RawMessage
	}
	var msg protocol.LiveDanmaku
	if json.Unmarshal(body, &event) != nil || (event.Cmd != "DANMU_MSG" && !strings.HasPrefix(event.Cmd, "DANMU_MSG:")) || len(event.Info) < 2 || json.Unmarshal(event.Info[1], &msg.Content) != nil || strings.TrimSpace(msg.Content) == "" {
		return msg, false
	}
	var user []json.RawMessage
	if len(event.Info) > 2 && json.Unmarshal(event.Info[2], &user) == nil && len(user) > 1 {
		if err := json.Unmarshal(user[1], &msg.UserName); err != nil {
			msg.UserName = ""
		}
	}
	if msg.UserName == "" {
		msg.UserName = "B站观众"
	}
	var style []json.RawMessage
	var rgb int
	if json.Unmarshal(event.Info[0], &style) == nil && len(style) > 3 && json.Unmarshal(style[3], &rgb) == nil && rgb > 0 && rgb <= 0xffffff {
		msg.Color = fmt.Sprintf("#%06x", rgb)
	}
	msg.EventID = "bili:" + rand.Text()
	msg.SendTime = time.Now().UTC()
	return msg, true
}

func mixin(img, sub string) (string, error) {
	var key string
	for _, raw := range []string{img, sub} {
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		name := path.Base(u.Path)
		stem := strings.TrimSuffix(name, path.Ext(name))
		if len(stem) != 32 {
			return "", errors.New("invalid WBI image key")
		}
		key += stem
	}
	// Only the first 32 entries of the permutation contribute to the key.
	order := [...]int{46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13}
	var out strings.Builder
	for _, i := range order {
		out.WriteByte(key[i])
	}
	return out.String(), nil
}
func signed(query url.Values, key string) string {
	clean := make(url.Values, len(query))
	for k, values := range query {
		for _, v := range values {
			clean.Add(k, strings.Map(func(r rune) rune {
				if strings.ContainsRune("!'()*", r) {
					return -1
				}
				return r
			}, v))
		}
	}
	encoded := strings.ReplaceAll(clean.Encode(), "+", "%20")
	sum := md5.Sum([]byte(encoded + key))
	return encoded + "&w_rid=" + hex.EncodeToString(sum[:])
}
