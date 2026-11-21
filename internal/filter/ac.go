// Package filter matches dictionary words across case, width and whitespace variations.
package filter

import (
	"os"
	"strings"
	"sync/atomic"
	"unicode"
)

type node struct {
	next           map[rune]int
	fail, matchLen int
}
type matcher struct{ nodes []node }
type Filter struct{ cur atomic.Pointer[matcher] }

func New(words []string) *Filter {
	f := &Filter{}
	f.Reload(words)
	return f
}

func LoadFile(path string) (*Filter, error) {
	f := &Filter{}
	if err := f.ReloadFile(path); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Filter) ReloadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var words []string
	for line := range strings.SplitSeq(string(data), "\n") {
		word := strings.TrimSpace(line)
		if word != "" && !strings.HasPrefix(word, "#") {
			words = append(words, word)
		}
	}
	f.Reload(words)
	return nil
}

func normalizeRune(r rune) rune {
	switch {
	case r == 0x3000:
		return ' '
	case r >= 0xff01 && r <= 0xff5e:
		r -= 0xfee0
	}
	return unicode.ToLower(r)
}

// Reload publishes a fully built trie. Readers keep their snapshot through the whole scan.
func (f *Filter) Reload(words []string) {
	m := &matcher{nodes: []node{{}}}
	for _, word := range words {
		state, length := 0, 0
		for _, r := range word {
			r = normalizeRune(r)
			if unicode.IsSpace(r) {
				continue
			}
			next := m.nodes[state].next[r]
			if next == 0 {
				if m.nodes[state].next == nil {
					m.nodes[state].next = make(map[rune]int)
				}
				next = len(m.nodes)
				m.nodes[state].next[r] = next
				m.nodes = append(m.nodes, node{})
			}
			state = next
			length++
		}
		m.nodes[state].matchLen = max(length, m.nodes[state].matchLen)
	}
	var queue []int
	for _, child := range m.nodes[0].next {
		queue = append(queue, child)
	}
	for i := 0; i < len(queue); i++ {
		parent := m.nodes[queue[i]]
		for r, child := range parent.next {
			fail := m.step(parent.fail, r)
			m.nodes[child].fail = fail
			// All suffix matches end at the same rune, so the longest covers them all.
			m.nodes[child].matchLen = max(m.nodes[child].matchLen, m.nodes[fail].matchLen)
			queue = append(queue, child)
		}
	}
	f.cur.Store(m)
}

func (m *matcher) step(state int, r rune) int {
	for {
		if child := m.nodes[state].next[r]; child != 0 {
			return child
		}
		if state == 0 {
			return 0
		}
		state = m.nodes[state].fail
	}
}

func (m *matcher) contains(text string) bool {
	if m == nil || len(m.nodes) == 1 {
		return false
	}
	state := 0
	for _, r := range text {
		r = normalizeRune(r)
		if unicode.IsSpace(r) {
			continue
		}
		state = m.step(state, r)
		if m.nodes[state].matchLen > 0 {
			return true
		}
	}
	return false
}
func (f *Filter) Contains(text string) bool { return f.cur.Load().contains(text) }

func (f *Filter) Mask(text string, repl rune) (string, bool) {
	m := f.cur.Load()
	if !m.contains(text) {
		return text, false
	}
	original := []rune(text)
	origIdx := make([]int, 0, len(original))
	// Difference array merges overlapping hits in linear time, including interior whitespace.
	coverage := make([]int, len(original)+1)
	state := 0
	for i, r := range original {
		r = normalizeRune(r)
		if unicode.IsSpace(r) {
			continue
		}
		origIdx = append(origIdx, i)
		state = m.step(state, r)
		if n := m.nodes[state].matchLen; n != 0 {
			coverage[origIdx[len(origIdx)-n]]++
			coverage[i+1]--
		}
	}
	active := 0
	for i := range original {
		active += coverage[i]
		if active > 0 {
			original[i] = repl
		}
	}
	return string(original), true
}
