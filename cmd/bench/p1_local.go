package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scthy/maku/internal/filter"
	"github.com/scthy/maku/internal/httpapi"
)

func dictionary(n int) []string {
	rng := rand.New(rand.NewSource(42))
	seen := map[string]bool{}
	words := make([]string, 0, n)
	for len(words) < n {
		var w [5]rune
		for i := range w {
			w[i] = '一' + rune(rng.Intn(256))
		}
		word := string(w[:])
		if !seen[word] {
			seen[word] = true
			words = append(words, word)
		}
	}
	return words
}
func (s *p1Suite) filters() error {
	for _, n := range []int{100, 1000, 5000, 20000} {
		words := dictionary(n)
		arms := []string{"naive_contains", "merged_regexp", "ac_prod"}
		note := "AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs."
		if n > 1000 {
			arms = []string{"naive_contains", "ac_prod"}
		}
		err := s.run("sensitive_filter_scaling", fmt.Sprintf("words_%d", n), note, arms, func(arm string) (measurement, error) {
			m := newMeasurement()
			match := func(text string) bool {
				for _, word := range words {
					if strings.Contains(text, word) {
						return true
					}
				}
				return false
			}
			switch arm {
			case "merged_regexp":
				re, e := regexp.Compile(strings.Join(words, "|"))
				if e != nil {
					return m, e
				}
				match = re.MatchString
			case "ac_prod":
				match = filter.New(words).Contains
			}
			miss := strings.Repeat("晴", 100)
			hit := strings.Repeat("晴", 95) + words[n-1]
			ok := true
			rng := rand.New(rand.NewSource(1))
			for i := range 100 {
				p := rng.Intn(96)
				word := "清晨天气好"
				if i%2 == 0 {
					word = words[rng.Intn(n)]
				}
				ok = ok && match(strings.Repeat("晴", p)+word+strings.Repeat("晴", 95-p)) == (i%2 == 0)
			}
			for _, v := range []struct {
				name, text string
				want       bool
			}{{"miss", miss, false}, {"hit", hit, true}} {
				t := time.Now()
				for range 100 {
					if match(v.text) != v.want {
						ok = false
					}
				}
				m.Metrics[v.name+"_us"] = micros(t) / 100
			}
			m.Metrics["miss_allocs_per_op"] = testing.AllocsPerRun(100, func() { match(miss) })
			gate(&m, "match_agreement", ok)
			if arm == "ac_prod" {
				gate(&m, "no_miss_allocations", m.Metrics["miss_allocs_per_op"] == 0)
			}
			return m, nil
		})
		if err != nil {
			return err
		}
	}
	return s.run("sensitive_filter_scaling", "reload_5000", "Atomic reload temporarily retains old and new tries.", []string{"rwmutex_rebuild_baseline", "atomic_pointer_prod"}, reloadMeasure)
}
func reloadMeasure(arm string) (measurement, error) {
	words := dictionary(5000)
	f := filter.New(words)
	var mu sync.RWMutex
	locked := arm == "rwmutex_rebuild_baseline"
	const readers = 8
	times := make([][]float64, readers)
	valid := make([]bool, readers)
	var ready, done sync.WaitGroup
	ready.Add(readers)
	done.Add(readers)
	stop := make(chan struct{})
	start := make(chan struct{})
	query := strings.Repeat("晴", 95) + words[0]
	for i := range readers {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			valid[i] = true
			for {
				select {
				case <-stop:
					return
				default:
				}
				t := time.Now()
				if locked {
					mu.RLock()
				}
				hit := f.Contains(query)
				if locked {
					mu.RUnlock()
				}
				times[i] = append(times[i], micros(t))
				valid[i] = valid[i] && hit
			}
		}()
	}
	ready.Wait()
	close(start)
	for range 20 {
		if locked {
			mu.Lock()
		}
		f.Reload(words)
		if locked {
			mu.Unlock()
		}
	}
	close(stop)
	done.Wait()
	var combined []float64
	ok := true
	for i, v := range times {
		ok = ok && len(v) > 0 && valid[i]
		combined = append(combined, v...)
	}
	m := newMeasurement()
	gate(&m, "reload_match_consistency", ok)
	if len(combined) > 0 {
		latency(&m, "reload_", combined)
	}
	return m, nil
}
func (s *p1Suite) live() error {
	n := 64
	if s.full {
		n = 120
	}
	return s.run("live_source_singleflight", fmt.Sprintf("burst_%d", n), "Live results may remain cached for 15 seconds.", []string{"direct_no_coalesce", "prod_singleflight_cache"}, func(arm string) (measurement, error) {
		var calls atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			time.Sleep(80 * time.Millisecond)
			io.WriteString(w, `{"code":0,"data":{"live_status":1,"playurl_info":{"playurl":{"stream":[{"format":[{"format_name":"ts","codec":[{"base_url":"/stream","url_info":[{"host":"https://example.invalid"}]}]}]}]}}}}`)
		}))
		defer server.Close()
		cached := arm == "prod_singleflight_cache"
		resolve := httpapi.LiveSourceLookup(server.URL, server.Client(), cached)
		m := newMeasurement()
		for _, phase := range []string{"cold", "warm"} {
			before := calls.Load()
			times := make([]float64, n)
			valid := make([]bool, n)
			burst(n, func(i int) {
				t := time.Now()
				v, e := resolve(s.ctx, 1)
				times[i] = micros(t)
				valid[i] = e == nil && v.Live && len(v.Sources) == 1 && v.Sources["hls"] == "https://example.invalid/stream"
			})
			ok := true
			for _, v := range valid {
				ok = ok && v
			}
			gate(&m, phase+"_valid_results", ok)
			latency(&m, phase+"_", times)
			count := calls.Load() - before
			m.Metrics[phase+"_upstream_calls"] = float64(count)
			want := n
			if cached {
				want = 1
				if phase == "warm" {
					want = 0
				}
			}
			gate(&m, phase+"_upstream_count", count == int64(want))
		}
		return m, nil
	})
}
