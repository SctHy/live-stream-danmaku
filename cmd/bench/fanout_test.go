package main

import "testing"

func TestDuplicateDoesNotMaskMissing(t *testing.T) {
	o := observer{seen: make([]uint64, 2)}
	for seq := range 100 {
		if seq == 10 {
			seq = 11
		}
		o.record(seq)
	}
	if o.unique != 99 || o.duplicates != 1 || o.firstDuplicate != 11 || o.seen[0]&(1<<10) != 0 {
		t.Fatalf("unique=%d duplicates=%d first duplicate=%d", o.unique, o.duplicates, o.firstDuplicate)
	}
}
