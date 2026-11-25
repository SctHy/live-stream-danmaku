package eval

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

var panels = []struct {
	name, note string
	metrics    []string
}{
	{"fanout_sweep", "Batching can increase hot-room latency while reducing frames.", []string{"frames", "process_cpu_seconds_total", "hot_p50_us", "hot_p99_us"}},
	{"isolation_stalled", "", []string{"rss_peak_bytes", "cold_p99_us", ClientFull, Kicked}},
	{"migration_strategies", "Pacing handoffs spreads recovery requests and can extend drain time.", []string{"recent_peak_qps", "connect_peak_qps", "drain_ms"}},
	{"history_pagination", "", []string{"p50_us"}},
	{"pg_archive_write", "", []string{"rows_per_second"}},
	{"sensitive_filter_scaling", "", []string{"miss_us", "reload_p99_us"}},
	{"redis_lua_atomicity", "", []string{"p50_us", "qps", "race_duplicate_rate"}},
	{"live_source_singleflight", "", []string{"cold_upstream_calls", "warm_upstream_calls"}},
}

type table struct {
	Title   string
	Headers []string
	Rows    [][]string
}
type point struct {
	X, Y  float64
	Value string
}
type series struct {
	Name, Color, Points string
	Samples             []point
}
type tick struct {
	X     float64
	Label string
}
type chart struct {
	Title string
	Ticks []tick
	Lines []series
	Max   string
}
type document struct {
	Env        string
	Score      table
	Details    []table
	Charts     []chart
	Regression *table
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func value(p *float64) string {
	if p == nil {
		return "—"
	}
	return num(*p)
}
func makeChart(name, metric string, ex []Experiment) chart {
	c := chart{Title: name + " · " + metric}
	var loads []Experiment
	var arms []string
	var peak float64
	for _, e := range ex {
		if e.Name != name {
			continue
		}
		found := false
		for _, a := range e.Arms {
			if s, ok := a.Summary[metric]; ok {
				found = true
				peak = math.Max(peak, s.Median)
				if !slices.Contains(arms, a.Name) {
					arms = append(arms, a.Name)
				}
			}
		}
		if found {
			loads = append(loads, e)
		}
	}
	c.Max = num(peak)
	for i, e := range loads {
		x := float64(80 + i*420/max(1, len(loads)-1))
		if len(loads) == 1 {
			x = 290
		}
		c.Ticks = append(c.Ticks, tick{x, e.Load})
	}
	colors := []string{"#83e7c1", "#eaba78", "#90baff"}
	for i, arm := range arms {
		var points []string
		var samples []point
		for j, e := range loads {
			for _, a := range e.Arms {
				v, ok := a.Summary[metric]
				if a.Name != arm || !ok {
					continue
				}
				y := 112.
				if peak > 0 {
					y -= 90 * v.Median / peak
				}
				x := c.Ticks[j].X
				if len(loads) == 1 {
					x += float64(i)*48 - float64(len(arms)-1)*24
				}
				samples = append(samples, point{x, y, num(v.Median)})
				points = append(points, fmt.Sprintf("%.1f,%.1f", x, y))
			}
		}
		c.Lines = append(c.Lines, series{arm, colors[i%len(colors)], strings.Join(points, " "), samples})
	}
	return c
}
func buildDocument(r Report, d *Diff) (document, error) {
	if _, err := index(r); err != nil {
		return document{}, err
	}
	env := r.Env
	doc := document{
		Env: fmt.Sprintf("%s/%s · %s · CPUs %d · profile %s · repeats %d (+ warmup) · %s · git_sha %s · dirty %t",
			env.GOOS, env.GOARCH, env.GoVersion, env.NumCPU, env.Profile, env.Repeats,
			env.Timestamp.Format(time.RFC3339Nano), env.GitSHA, env.GitDirty),
		Score: table{Title: "Scorecard", Headers: []string{"Experiment / load", "Metric", "Baseline → treatment", "Medians", "Change (%)", "Δ 95% CI", "Trade-off", "Gate"}},
	}
	for _, p := range panels {
		for _, k := range p.metrics {
			c := makeChart(p.name, k, r.Experiments)
			if len(c.Lines) > 0 {
				doc.Charts = append(doc.Charts, c)
			}
		}
	}
	for _, e := range r.Experiments {
		title := strings.TrimSpace(e.Name + " " + e.Load)
		gate := "PASS"
		if !e.Passed {
			gate = "FAIL"
		}
		t := table{Title: title + " · " + gate, Headers: []string{"Arm", "Metric", "Median", "95% CI", "Runs"}}
		for i, a := range e.Arms {
			for _, k := range slices.Sorted(maps.Keys(a.Summary)) {
				v := a.Summary[k]
				t.Rows = append(t.Rows, []string{a.Name, k, num(v.Median), num(v.Low) + " … " + num(v.High), strconv.Itoa(v.Samples)})
				if i == 0 {
					continue
				}
				for _, p := range panels {
					if p.name != e.Name || !slices.Contains(p.metrics, k) {
						continue
					}
					delta, ok := a.Deltas[k]
					if !ok {
						continue
					}
					base := e.Arms[0]
					note := e.TradeOffNote
					if note == "" {
						note = p.note
					}
					doc.Score.Rows = append(doc.Score.Rows, []string{title, k, base.Name + " → " + a.Name, num(base.Summary[k].Median) + " → " + num(v.Median), value(delta.Percent), num(delta.Low) + " … " + num(delta.High), note, gate})
				}
			}
		}
		doc.Details = append(doc.Details, t)
	}
	if d != nil {
		t := table{Title: fmt.Sprintf("Regression · gate passed %t", d.Passed), Headers: []string{"Experiment", "Load", "Arm", "Metric", "Baseline", "Candidate", "Status"}}
		for _, row := range d.Rows {
			t.Rows = append(t.Rows, []string{row.Experiment, row.Load, row.Arm, row.Metric, value(row.Baseline), value(row.Candidate), row.Status})
		}
		doc.Regression = &t
	}
	return doc, nil
}
func (d document) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Maku Evaluation\n\n%s\n\n", d.Env)
	escape := strings.NewReplacer("|", "\\|", "\n", " ")
	write := func(t table) {
		fmt.Fprintf(&b, "## %s\n\n", t.Title)
		line := func(cells []string) {
			for _, v := range cells {
				fmt.Fprintf(&b, "| %s ", escape.Replace(v))
			}
			b.WriteString("|\n")
		}
		line(t.Headers)
		b.WriteString(strings.Repeat("| --- ", len(t.Headers)) + "|\n")
		for _, row := range t.Rows {
			line(row)
		}
		b.WriteString("\n")
	}
	write(d.Score)
	for _, t := range d.Details {
		write(t)
	}
	if d.Regression != nil {
		write(*d.Regression)
	}
	return b.String()
}

//go:embed report.html.tmpl
var pageSource string

var page = template.Must(template.New("page").Parse(pageSource))

func Render(r Report, d *Diff) (string, string, error) {
	doc, err := buildDocument(r, d)
	if err != nil {
		return "", "", err
	}
	var b bytes.Buffer
	if err := page.Execute(&b, doc); err != nil {
		return "", "", err
	}
	return doc.markdown(), b.String(), nil
}
