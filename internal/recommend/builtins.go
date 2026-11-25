package recommend

import (
	"context"
	"math"
	"sort"
)

func Default() *Registry {
	r := &Registry{map[string]RecallSource{}, map[string]Filter{}, map[string]Scorer{}, map[string]Reranker{}, map[string]Strategy{}}
	for _, n := range []string{"hot", "online", "affinity", "fresh_explore"} {
		r.RegisterRecall(recall(n))
	}
	for _, n := range []string{"status", "category"} {
		r.RegisterFilter(filter(n))
	}
	for _, n := range []string{"passthrough", "diversity_v1"} {
		r.RegisterReranker(reranker(n))
	}
	for n, label := range map[string]string{"hot": "热门", "online": "在线人数", "latest": "最新", "personalized_v1": "个性推荐", "explore_v1": "发现新房间"} {
		r.RegisterScorer(scorer(n))
		p := Strategy{Name: n, Label: label, Recalls: []string{"hot"}, Filters: []string{"status", "category"}, Scorer: n, Reranker: "passthrough"}
		if n == "online" {
			p.Recalls = []string{"online"}
		}
		if n == "personalized_v1" || n == "explore_v1" {
			p.Recalls = []string{"hot", "online", "affinity", "fresh_explore"}
			p.Reranker = "diversity_v1"
			p.RecallLimit = 50
		}
		if err := r.RegisterStrategy(p); err != nil {
			panic(err)
		}
	}
	return r
}

type filter string

func (f filter) Name() string { return string(f) }
func (f filter) Allow(q Request, c Candidate) bool {
	if f == "status" {
		return q.Status != "live" || c.Status == "live"
	}
	return q.Category == "" || q.Category == c.Category
}
func age(q Request, c Candidate) float64 {
	t := c.Created
	if c.Started.After(t) {
		t = c.Started
	}
	return max(0, q.Now.Sub(t).Hours())
}
func affinity(q Request, c Candidate) (float64, string) {
	cat := math.Log1p(q.Profile.Categories[c.Category])
	tag := 0.
	reason := ""
	for _, t := range c.Tags {
		v := math.Log1p(q.Profile.Tags[t])
		if v > tag {
			tag = v
			reason = "标签匹配 · " + t
		}
	}
	if cat > 0 && cat >= tag {
		reason = "常看分区 · " + CategoryName(c.Category)
	}
	return cat + tag, reason
}

type recall string

func (p recall) Name() string { return string(p) }
func (p recall) Recall(ctx context.Context, q Request, s Snapshot) []Candidate {
	var out []Candidate
	for _, c := range s {
		if ctx.Err() != nil {
			break
		}
		switch p {
		case "affinity":
			if a, _ := affinity(q, c); a == 0 {
				continue
			}
		case "fresh_explore":
			if age(q, c) > 24 {
				continue
			}
		}
		out = append(out, c)
	}
	// Rank the hot and online sources independently. Eligible taxonomy/status are
	// pushed down before the top-K cutoff, and also enforced by pipeline filters.
	if q.recallLimit > 0 && (p == "hot" || p == "online") {
		eligible := out[:0]
		for _, c := range out {
			if filter("category").Allow(q, c) && filter("status").Allow(q, c) {
				eligible = append(eligible, c)
			}
		}
		out = eligible
		sort.Slice(out, func(i, j int) bool {
			a, b := out[i], out[j]
			if p == "hot" && a.Heat != b.Heat {
				return a.Heat > b.Heat
			}
			if p == "online" && a.Online != b.Online {
				return a.Online > b.Online
			}
			return a.Slug < b.Slug
		})
		out = out[:min(q.recallLimit, len(out))]
	}
	return out
}

type scorer string

func (p scorer) Name() string { return string(p) }
func (p scorer) Score(q Request, items []Candidate) []ScoredCandidate {
	out := make([]ScoredCandidate, 0, len(items))
	for _, c := range items {
		v := ScoredCandidate{Candidate: c, Score: c.Heat, Reason: "热门直播"}
		switch p {
		case "online":
			v.Score = float64(c.Online)
			v.Reason = "人气直播"
		case "latest":
			v.Reason = "最近创建"
		case "explore_v1", "personalized_v1":
			a, reason := affinity(q, c)
			hours := age(q, c)
			fresh := math.Exp(-hours / 24)
			if hours <= 24 {
				fresh += 1
			}
			weight := 1.
			if p == "explore_v1" {
				weight = 6
			}
			v.Score = math.Log1p(c.Heat) + math.Log1p(float64(c.Online)) + 3*a + weight*fresh
			if q.Profile.Recent[c.Slug] > 0 {
				v.Score -= 0.5
			}
			if reason != "" {
				v.Reason = reason
			} else if hours <= 24 {
				v.Reason = "新开播推荐"
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if p == "latest" && !a.Created.Equal(b.Created) {
			return a.Created.After(b.Created)
		}
		if p != "latest" && a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Slug < b.Slug
	})
	return out
}

type reranker string

func (p reranker) Name() string { return string(p) }
func (p reranker) Rerank(q Request, in []ScoredCandidate) []ScoredCandidate {
	if p == "passthrough" {
		return in
	}
	limit := len(in)
	if q.Limit > 0 {
		limit = min(limit, q.Limit)
	}
	for i := 2; i < limit; i++ {
		cat := in[i].Category
		if cat != in[i-1].Category || cat != in[i-2].Category {
			continue
		}
		j := i + 1
		for j < len(in) && in[j].Category == cat {
			j++
		}
		if j == len(in) {
			break
		}
		pick := in[j]
		copy(in[i+1:j+1], in[i:j])
		in[i] = pick
	}
	return in
}
