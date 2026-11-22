package protocol

import (
	"strings"
	"testing"
)

func TestDecodePackets(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		types []int
	}{
		{` {"type":101} `, []int{101}}, {` [{"type":102},{"type":103}] `, []int{102, 103}}, {" \t", nil}, {`[invalid`, nil},
	} {
		got, err := DecodePackets([]byte(tc.raw))
		if (err != nil) != (tc.types == nil) || len(got) != len(tc.types) {
			t.Fatalf("%q: %v %v", tc.raw, got, err)
		}
		for i, v := range tc.types {
			if got[i].Type != v {
				t.Fatal(got)
			}
		}
	}
}
func TestFitsHistory(t *testing.T) {
	base := DanmakuMessage{RoomID: strings.Repeat("房", 50), Content: strings.Repeat("字", 500)}
	if !base.FitsHistory() {
		t.Fatal("valid rune boundary")
	}
	for _, change := range []func(*DanmakuMessage){func(d *DanmakuMessage) { d.RoomID += "房" }, func(d *DanmakuMessage) { d.Content += "字" }, func(d *DanmakuMessage) { d.EventID = "a\x00" }, func(d *DanmakuMessage) { d.RoomID = "\x00" }, func(d *DanmakuMessage) { d.Content = "\x00" }} {
		d := base
		change(&d)
		if d.FitsHistory() {
			t.Fatal("accepted invalid field")
		}
	}
}
