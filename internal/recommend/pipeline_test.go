package recommend

import (
	"context"
	"slices"
	"testing"
	"time"
)

type extra struct{}

func (extra) Name() string                                                { return "custom" }
func (extra) Recall(_ context.Context, _ Request, s Snapshot) []Candidate { return s }
func (extra) Score(_ Request, items []Candidate) []ScoredCandidate {
	var out []ScoredCandidate
	for _, c := range items {
		out = append(out, ScoredCandidate{Candidate: c, Reason: "custom"})
	}
	return out
}
func (extra) Rerank(_ Request, s []ScoredCandidate) []ScoredCandidate { slices.Reverse(s); return s }

func TestCustomPipeline(t *testing.T) {
	r := Default()
	r.RegisterRecall(extra{})
	r.RegisterScorer(extra{})
	r.RegisterReranker(extra{})
	if err := r.RegisterStrategy(Strategy{Name: "test", Recalls: []string{"hot", "custom"}, Scorer: "custom", Reranker: "custom"}); err != nil {
		t.Fatal(err)
	}
	got := r.Run(t.Context(), "test", Request{}, Snapshot{{Slug: "a"}, {Slug: "b"}})
	if len(got) != 2 || got[0].Slug != "b" || got[1].Reason != "custom" {
		t.Fatal(got)
	}
}
func TestStrategies(t *testing.T) {
	q := Request{Now: time.Now(), Profile: Profile{Categories: map[string]float64{"coding": 8}}}
	items := Snapshot{{Slug: "hot", Status: "live", Heat: 100}, {Slug: "cold", Status: "live", Created: q.Now}, {Slug: "match", Status: "live", Category: "coding"}}
	r := Default()
	for _, tc := range []struct{ name, slug, reason string }{{"hot", "hot", "热门直播"}, {"personalized_v1", "match", "常看分区 · 编程"}, {"explore_v1", "cold", "新开播推荐"}} {
		got := r.Run(t.Context(), tc.name, q, items)
		if got[0].Slug != tc.slug || got[0].Reason != tc.reason {
			t.Fatal(tc, got)
		}
	}
}
func TestDiversityAndBucket(t *testing.T) {
	items := make([]ScoredCandidate, 6)
	for i := range items {
		items[i].Category = "chat"
	}
	items[4].Category = "music"
	items[5].Category = "gaming"
	got := reranker("diversity_v1").Rerank(Request{}, items)
	for i := 2; i < len(got); i++ {
		if got[i].Category == got[i-1].Category && got[i].Category == got[i-2].Category {
			t.Fatal(got)
		}
	}
	buckets := map[string]bool{}
	for uid := uint64(1); uid < 100; uid++ {
		first := Bucket(uid)
		if Bucket(uid) != first {
			t.Fatal(uid)
		}
		buckets[first] = true
	}
	if len(buckets) != 2 || Bucket(0) != "personalized_v1" {
		t.Fatal(buckets)
	}
}

func TestLiveStatusPrecedesFreshOffline(t *testing.T) {
	now := time.Now()
	items := Snapshot{{Slug: "off", Status: "offline", Created: now}, {Slug: "on", Status: "live"}}
	for _, strategy := range []string{"personalized_v1", "explore_v1", "hot"} {
		got := Default().Run(t.Context(), strategy, Request{Now: now, Profile: Profile{Recent: map[string]float64{"on": 1}}}, items)
		for _, c := range got {
			if c.Slug == "off" && c.Reason != "暂未开播" || c.Reason == "热门直播" {
				t.Fatal(strategy, c)
			}
		}
		if strategy != "hot" && got[0].Slug != "on" {
			t.Fatal(strategy, got)
		}
	}
}
