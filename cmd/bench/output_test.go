package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scthy/maku/internal/eval"
)

func TestAllSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := evaluateSuite("all", []string{"-only", "unknown", "-out", path}); err == nil {
		t.Fatal("accepted unknown experiment")
	}
	if err := evaluateSuite("all", []string{"-only", "live_source_singleflight", "-repeats", "1", "-gateway", "absent", "-out", path}); err != nil {
		t.Fatal(err)
	}
	r, err := readLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Experiments) != 1 || !r.Experiments[0].Passed || r.Env.Repeats != 1 || len(r.Experiments[0].Arms[0].Runs) != 1 {
		t.Fatal("wrong selection or repeat count")
	}
}
func TestDiffAndReportCommands(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, r eval.Report) string {
		t.Helper()
		p := filepath.Join(dir, name)
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	fixture := func() eval.Report {
		return eval.Report{Experiments: []eval.Experiment{{Name: "fanout_sweep", Load: "high", Passed: true, Arms: []eval.Arm{{Name: "baseline", Summary: map[string]eval.MetricSummary{"frames": {Median: 100}, "qps": {Median: 100}}}}}}}
	}
	bp := write("base.json", fixture())
	cp := filepath.Join(dir, "candidate.json")
	for _, tc := range []struct {
		name        string
		edit        func(*eval.Experiment)
		allow, fail bool
	}{
		{"equal", func(*eval.Experiment) {}, false, false},
		{"boundary", func(e *eval.Experiment) { e.Arms[0].Summary["frames"] = eval.MetricSummary{Median: 115} }, false, false},
		{"regressed", func(e *eval.Experiment) { e.Arms[0].Summary["frames"] = eval.MetricSummary{Median: 116} }, false, true},
		{"throughput", func(e *eval.Experiment) { e.Arms[0].Summary["qps"] = eval.MetricSummary{Median: 80} }, false, true},
		{"gate", func(e *eval.Experiment) { e.Passed = false }, true, true},
		{"missing arm", func(e *eval.Experiment) { e.Arms[0].Name = "other" }, false, true},
		{"missing load", func(e *eval.Experiment) { e.Load = "low" }, false, true},
		{"allowed", func(e *eval.Experiment) { e.Load = "low" }, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixture()
			tc.edit(&r.Experiments[0])
			write("candidate.json", r)
			out := filepath.Join(dir, "diff.json")
			args := []string{"-baseline", bp, "-candidate", cp, "-out", out}
			if tc.allow {
				args = append(args, "-allow-missing")
			}
			if err := evalDiff(args); (err != nil) != tc.fail {
				t.Fatal(err)
			}
			var d eval.Diff
			b, err := os.ReadFile(out)
			if err != nil || json.Unmarshal(b, &d) != nil || d.Passed == tc.fail {
				t.Fatal("invalid diff")
			}
		})
	}
	md, html := filepath.Join(dir, "report.md"), filepath.Join(dir, "report.html")
	if err := evalReport([]string{"-in", bp, "-baseline", bp, "-md", md, "-html", html}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{md, html} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.Contains(s, "Regression") || !strings.Contains(s, "Scorecard") || !strings.Contains(s, "frames") || strings.Contains(s, "ZgotmplZ") {
			t.Fatal("invalid report", p)
		}
		if p == html && !strings.Contains(s, "<svg") {
			t.Fatal("no chart")
		}
	}
}
