// Package recommend assembles room recommendation strategies at gateway startup.
package recommend

import (
	"context"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"strconv"
	"time"
)

type Category struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var Categories = []Category{{"chat", "闲聊"}, {"gaming", "游戏"}, {"coding", "编程"}, {"music", "音乐"}, {"outdoor", "户外"}}

func CategoryName(code string) string {
	for _, v := range Categories {
		if v.Code == code {
			return v.Name
		}
	}
	return ""
}

type Candidate struct {
	Slug, Category, Status string
	Tags                   []string
	Created, Started       time.Time
	Heat                   float64
	Online                 int64
}
type Snapshot []Candidate
type Profile struct {
	Categories map[string]float64 `json:"categories"`
	Tags       map[string]float64 `json:"tags"`
	Recent     map[string]float64 `json:"recent_rooms"`
}
type Request struct {
	Limit            int
	recallLimit      int
	Profile          Profile
	Status, Category string
	Now              time.Time
}
type ScoredCandidate struct {
	Candidate
	Score  float64
	Reason string
}
type RecallSource interface {
	Name() string
	Recall(context.Context, Request, Snapshot) []Candidate
}
type Filter interface {
	Name() string
	Allow(Request, Candidate) bool
}
type Scorer interface {
	Name() string
	Score(Request, []Candidate) []ScoredCandidate
}
type Reranker interface {
	Name() string
	Rerank(Request, []ScoredCandidate) []ScoredCandidate
}
type Strategy struct {
	RecallLimit int      `json:"recall_limit,omitempty"`
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Recalls     []string `json:"recalls"`
	Filters     []string `json:"filters"`
	Scorer      string   `json:"scorer"`
	Reranker    string   `json:"reranker"`
}
type Registry struct {
	recalls    map[string]RecallSource
	filters    map[string]Filter
	scorers    map[string]Scorer
	rerankers  map[string]Reranker
	strategies map[string]Strategy
}

func (r *Registry) RegisterRecall(p RecallSource) { r.recalls[p.Name()] = p }
func (r *Registry) RegisterFilter(p Filter)       { r.filters[p.Name()] = p }
func (r *Registry) RegisterScorer(p Scorer)       { r.scorers[p.Name()] = p }
func (r *Registry) RegisterReranker(p Reranker)   { r.rerankers[p.Name()] = p }
func (r *Registry) RegisterStrategy(s Strategy) error {
	if s.Name == "" || len(s.Recalls) == 0 || r.scorers[s.Scorer] == nil || r.rerankers[s.Reranker] == nil {
		return fmt.Errorf("invalid strategy %q", s.Name)
	}
	for _, n := range s.Recalls {
		if r.recalls[n] == nil {
			return fmt.Errorf("unknown recall %q", n)
		}
	}
	for _, n := range s.Filters {
		if r.filters[n] == nil {
			return fmt.Errorf("unknown filter %q", n)
		}
	}
	r.strategies[s.Name] = s
	return nil
}
func (r *Registry) Strategies() []Strategy {
	var out []Strategy
	for _, n := range slices.Sorted(maps.Keys(r.strategies)) {
		out = append(out, r.strategies[n])
	}
	return out
}
func (r *Registry) Has(n string) bool { _, ok := r.strategies[n]; return ok }
func Bucket(uid uint64) string {
	h := fnv.New32a()
	h.Write([]byte(strconv.FormatUint(uid, 10)))
	if uid != 0 && h.Sum32()%5 == 0 {
		return "explore_v1"
	}
	return "personalized_v1"
}
func (r *Registry) Run(ctx context.Context, name string, q Request, s Snapshot) []ScoredCandidate {
	p := r.strategies[name]
	q.recallLimit = p.RecallLimit
	seen := map[string]bool{}
	var items []Candidate
	for _, n := range p.Recalls {
	next:
		for _, c := range r.recalls[n].Recall(ctx, q, s) {
			if seen[c.Slug] {
				continue
			}
			seen[c.Slug] = true
			for _, f := range p.Filters {
				if !r.filters[f].Allow(q, c) {
					continue next
				}
			}
			items = append(items, c)
		}
	}
	return r.rerankers[p.Reranker].Rerank(q, r.scorers[p.Scorer].Score(q, items))
}
