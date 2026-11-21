package filter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMask(t *testing.T) {
	f := New([]string{"ab", "abc", "bcde", "Ａ 坏😊", "he", "sheep", " ", ""})
	for _, tc := range []struct {
		text, want string
		hit        bool
	}{
		{"!ＡbＣ dＥ?", "!******?", true}, {"汉A　坏😊好", "汉****好", true}, {" abc ab ", " *** ** ", true},
		{"she!", "s**!", true}, {"a\tb", "***", true}, {"", "", false}, {"安😊全", "安😊全", false},
	} {
		got, hit := f.Mask(tc.text, '*')
		if got != tc.want || hit != tc.hit || f.Contains(tc.text) != tc.hit {
			t.Errorf("Mask(%q) = %q, %v", tc.text, got, hit)
		}
	}
	if got := testing.AllocsPerRun(100, func() { f.Mask("安😊全", '*') }); got != 0 {
		t.Fatalf("unmatched allocations: %v", got)
	}
	f.Reload(nil)
	if f.Contains("abc") {
		t.Fatal("empty dictionary matched")
	}
}
func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words")
	if err := os.WriteFile(path, []byte("  # comment\r\n\n Ａb \r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := LoadFile(path)
	if err != nil || !f.Contains("ab") || f.Contains("comment") {
		t.Fatal(err)
	}
	if err := f.ReloadFile(path + ".missing"); err == nil || !f.Contains("ab") {
		t.Fatal("failed reload changed dictionary")
	}
}
func TestConcurrentReload(t *testing.T) {
	f := New([]string{"abc"})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 500 {
				f.Contains("abcd")
				s, _ := f.Mask("abcd", '*')
				if s != "***d" && s != "a***" {
					t.Errorf("mixed snapshot: %s", s)
					return
				}
			}
		})
	}
	for range 500 {
		f.Reload([]string{"bcd"})
		f.Reload([]string{"abc"})
	}
	wg.Wait()
}
func BenchmarkFilter5000Words(b *testing.B) {
	words := make([]string, 5000)
	for i := range words {
		words[i] = fmt.Sprintf("测试词%c%c", rune(0x4e00+i/100), rune(0x4e00+i%100))
	}
	f := New(words)
	for _, tc := range []struct{ name, text string }{{"miss", strings.Repeat("晴", 100)}, {"hit", strings.Repeat("晴", 95) + words[4182]}} {
		b.Run("Contains/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				f.Contains(tc.text)
			}
		})
		b.Run("Mask/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				f.Mask(tc.text, '*')
			}
		})
	}
}
