package hub

import (
	"strconv"
	"testing"
)

func BenchmarkConnectionAllocation(b *testing.B) {
	for _, capacity := range []int{128, SendChanSize, 4096} {
		b.Run(strconv.Itoa(capacity), func(b *testing.B) {
			m := Hub{sendCapacity: capacity}
			b.ReportAllocs()
			for b.Loop() {
				c := m.NewClient(1, "room", nil)
				if cap(c.send) != capacity {
					b.Fatal("unexpected queue capacity")
				}
			}
		})
	}
}
