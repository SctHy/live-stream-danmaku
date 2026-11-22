package livebridge

import (
	"bytes"
	"compress/zlib"
	"testing"
)

const example = `{"cmd":"DANMU_MSG","info":[[0,0,0,16711680],"hello",[123,"Alice"]]}`

func zipped(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return packet(2, 5, buf.Bytes())
}
func TestPackets(t *testing.T) {
	single := packet(0, 5, []byte(example))
	for _, tc := range []struct {
		wire  []byte
		count int
	}{{single, 1}, {zipped(t, append(bytes.Clone(single), single...)), 2}} {
		count := 0
		err := decode(tc.wire, func(op uint32, b []byte) error {
			if op != 5 || string(b) != example {
				t.Fatal(op, string(b))
			}
			count++
			return nil
		})
		if err != nil || count != tc.count {
			t.Fatal(err, count)
		}
	}
	invalid := bytes.Clone(single)
	invalid[5] = 17
	for _, b := range [][]byte{single[:12], single[:20], invalid, zipped(t, make([]byte, maxPayload+1))} {
		if decode(b, func(uint32, []byte) error { return nil }) == nil {
			t.Fatal("invalid frame accepted")
		}
	}
}
func TestParseMessage(t *testing.T) {
	m, ok := parseMessage([]byte(example))
	if !ok || m.Content != "hello" || m.Color != "#ff0000" || m.UserName != "Alice" || m.EventID == "" || m.SendTime.IsZero() {
		t.Fatal(m)
	}
	m, ok = parseMessage([]byte(`{"cmd":"DANMU_MSG:4:1","info":[[],"text"]}`))
	if !ok || m.UserName != "B站观众" || m.Color != "" {
		t.Fatal(m)
	}
	for _, b := range []string{`{"cmd":"LIKE","info":[[],"x"]}`, `{"cmd":"DANMU_MSG","info":[[],""]}`} {
		if _, ok := parseMessage([]byte(b)); ok {
			t.Fatal(b)
		}
	}
}
func TestWBI(t *testing.T) {
	key, err := mixin("https://i/a/7cd084941338484aae1ad9425b84077c.png", "https://i/a/4932caff0ff746eab6f01bf08b70ac45.png")
	if err != nil || key != "ea1db124af3c7062474693fa704f4ff8" {
		t.Fatal(key, err)
	}
	got := signed(map[string][]string{"foo": {"1!"}, "bar": {"514"}, "zab": {"1919810"}, "wts": {"1702204169"}}, key)
	if got != "bar=514&foo=1&wts=1702204169&zab=1919810&w_rid=a9fa27b9fef32a21b64b90169f6742e1" {
		t.Fatal(got)
	}
}
