// Package eval records independent benchmark runs and compares their distributions.
package eval

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"
)

type Report struct {
	Env         EnvMetadata  `json:"env"`
	Experiments []Experiment `json:"experiments"`
}
type EnvMetadata struct {
	GitSHA    string    `json:"git_sha"`
	GitDirty  bool      `json:"git_dirty"`
	GoVersion string    `json:"go_version"`
	GOOS      string    `json:"goos"`
	GOARCH    string    `json:"goarch"`
	NumCPU    int       `json:"num_cpu"`
	Timestamp time.Time `json:"timestamp"`
	Profile   string    `json:"profile"`
	Repeats   int       `json:"repeats"`
}
type Experiment struct {
	TradeOffNote string `json:"trade_off_note,omitempty"`
	Name         string `json:"name"`
	Load         string `json:"load,omitempty"`
	Arms         []Arm  `json:"arms"`
	Passed       bool   `json:"passed"`
}
type Arm struct {
	Name        string                   `json:"name"`
	Args        []string                 `json:"args"`
	GatewayArgs []string                 `json:"gateway_args,omitempty"`
	Runs        []RunSample              `json:"runs"`
	Summary     map[string]MetricSummary `json:"summary"`
	Deltas      map[string]Delta         `json:"deltas,omitempty"`
}
type RunSample struct {
	Metrics          map[string]float64 `json:"metrics"`
	Invariants       []InvariantResult  `json:"invariants"`
	LowSampleWarning []string           `json:"low_sample_warning,omitempty"`
}
type InvariantResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}
type MetricSummary struct {
	Median  float64 `json:"median"`
	Mean    float64 `json:"mean"`
	P50     float64 `json:"p50"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
	IQR     float64 `json:"iqr"`
	Low     float64 `json:"ci95_low"`
	High    float64 `json:"ci95_high"`
	Samples int     `json:"samples"`
}
type Delta struct {
	Percent    *float64 `json:"percent"`
	Multiple   *float64 `json:"multiple"`
	Difference float64  `json:"difference"`
	Low        float64  `json:"ci95_low"`
	High       float64  `json:"ci95_high"`
	TradeOff   bool     `json:"trade_off"`
}

// rank expects a nonempty sorted sample.
func rank(x []float64, p float64) float64 { return x[int(math.Ceil(float64(len(x))*p))-1] }
func statistic(x []float64, mean bool) float64 {
	if mean {
		var s float64
		for _, v := range x {
			s += v
		}
		return s / float64(len(x))
	}
	slices.Sort(x)
	return (x[(len(x)-1)/2] + x[len(x)/2]) / 2
}

// Compare independently resamples runs. CI endpoints are absolute differences in the metric's units.
func Compare(base, treatment []float64, mean bool) Delta {
	if len(treatment) == 0 {
		return Delta{}
	}
	rng := rand.New(rand.NewSource(1))
	draw := func(x []float64) float64 {
		if len(x) == 0 {
			return 0
		}
		y := make([]float64, len(x))
		for i := range y {
			y[i] = x[rng.Intn(len(x))]
		}
		return statistic(y, mean)
	}
	d := make([]float64, 1000)
	for i := range d {
		d[i] = draw(treatment) - draw(base)
	}
	slices.Sort(d)
	var b float64
	if len(base) > 0 {
		b = statistic(slices.Clone(base), mean)
	}
	t := statistic(slices.Clone(treatment), mean)
	out := Delta{Difference: t - b, Low: rank(d, .025), High: rank(d, .975)}
	if b != 0 {
		r := t / b
		p := 100 * (r - 1)
		out.Multiple = &r
		out.Percent = &p
	}
	return out
}
func Summarize(values []float64) MetricSummary {
	if len(values) == 0 {
		return MetricSummary{}
	}
	x := slices.Clone(values)
	slices.Sort(x)
	ci := Compare(nil, x, false)
	return MetricSummary{
		Median: statistic(x, false), Mean: statistic(x, true),
		P50: rank(x, .5), P95: rank(x, .95), P99: rank(x, .99),
		IQR: rank(x, .75) - rank(x, .25), Low: ci.Low, High: ci.High, Samples: len(x),
	}
}
func (a Arm) Values(key string) []float64 {
	var x []float64
	for _, r := range a.Runs {
		if v, ok := r.Metrics[key]; ok {
			x = append(x, v)
		}
	}
	return x
}

// Measure alternates arms within each round and discards the first round as warmup.
func (e *Experiment) Measure(repeats int, run func(Arm, int) (RunSample, error)) error {
	if repeats < 1 {
		return fmt.Errorf("repeats must be positive")
	}
	for round := 0; round <= repeats; round++ {
		for i := range e.Arms {
			a := &e.Arms[i]
			sample, err := run(*a, round)
			if err != nil {
				return fmt.Errorf("%s/%s round %d: %w", e.Name, a.Name, round, err)
			}
			if e.Name == "isolation_stalled" && e.Load == "slow_kick" {
				drops := sample.Metrics[ClientFull]
				kicks := sample.Metrics[Kicked]
				wantKick := a.Name == "kick_enabled"
				sample.Invariants = append(sample.Invariants, InvariantResult{
					Name:   "stalled_backpressure_and_kick_policy",
					Passed: drops > 0 && ((wantKick && kicks > 0) || (!wantKick && kicks == 0)),
					Detail: fmt.Sprintf("client_full=%g kicked=%g", drops, kicks),
				})
			}
			if round > 0 {
				a.Runs = append(a.Runs, sample)
			}
		}
	}
	e.Finish()
	return nil
}

func (e *Experiment) Finish() {
	e.Passed = len(e.Arms) > 0
	for i := range e.Arms {
		a := &e.Arms[i]
		a.Summary = map[string]MetricSummary{}
		values := map[string][]float64{}
		if len(a.Runs) == 0 {
			e.Passed = false
		}
		for _, r := range a.Runs {
			if len(r.Invariants) == 0 {
				e.Passed = false
			}
			for _, g := range r.Invariants {
				e.Passed = e.Passed && g.Passed
			}
			for k, v := range r.Metrics {
				values[k] = append(values[k], v)
			}
		}
		a.Deltas = map[string]Delta{}
		for k, v := range values {
			a.Summary[k] = Summarize(v)
			if i == 0 {
				continue
			}
			base := e.Arms[0].Values(k)
			if len(base) == 0 {
				continue
			}
			d := Compare(base, v, false)
			d.TradeOff = (k == "drain_ms" || k == "last_handoff_ms" || k == "hot_p50_us") && d.Difference > 0
			a.Deltas[k] = d
		}
	}
}
