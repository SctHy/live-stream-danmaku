package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestStatistics(t *testing.T) {
	values := []float64{8, 1, 2, 5}
	s := Summarize(values)
	if s.Mean != 4 || s.Median != 3.5 || s.P50 != 2 || s.P95 != 8 || s.P99 != 8 || s.IQR != 4 || s.Samples != 4 || s.Low > s.Median || s.High < s.Median || values[0] != 8 {
		t.Fatal(s, values)
	}
	for _, mean := range []bool{false, true} {
		a := Compare([]float64{2, 2}, []float64{3, 3}, mean)
		if a.Low != 1 || a.High != 1 || *a.Percent != 50 || *a.Multiple != 1.5 {
			t.Fatal(a)
		}
		d := Compare(values, []float64{3, 4, 12}, mean)
		if !reflect.DeepEqual(d, Compare(values, []float64{3, 4, 12}, mean)) {
			t.Fatal("unstable bootstrap")
		}
	}
	if Compare([]float64{0}, values, false).Percent != nil {
		t.Fatal("relative change at zero")
	}
}
func TestReportGate(t *testing.T) {
	arm := func(v float64, passed bool) Arm {
		return Arm{Runs: []RunSample{{Metrics: map[string]float64{"drain_ms": v}, Invariants: []InvariantResult{{Name: "delivery", Passed: passed}}}}}
	}
	ex := Experiment{Arms: []Arm{arm(2, true), arm(3, false)}}
	ex.Finish()
	if ex.Passed || !ex.Arms[1].Deltas["drain_ms"].TradeOff {
		t.Fatal(ex)
	}
	r := Report{Experiments: []Experiment{ex}}
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var decoded Report
	if e = json.Unmarshal(b, &decoded); e != nil || !reflect.DeepEqual(decoded.Experiments[0].Arms[1], ex.Arms[1]) {
		t.Fatal(e, string(b))
	}
	ex.Arms[1].Runs[0].Invariants[0].Passed = true
	ex.Finish()
	if !ex.Passed {
		t.Fatal("valid gate failed")
	}
}
func TestMetricsRound(t *testing.T) {
	var body strings.Builder
	body.WriteString("# TYPE unrelated histogram\nunrelated_bucket{bound=\"with spaces\"} 2\n")
	for _, k := range keys {
		fmt.Fprintf(&body, "%s 1.2e2\n", k)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body.String())
	}))
	defer server.Close()
	before, e := Scrape(server.Client(), server.URL+"/metrics")
	if e != nil {
		t.Fatal(e)
	}
	after := Snapshot{}
	for k, v := range before {
		after[k] = v + 3
	}
	after["go_goroutines"] = 100
	d := after.Changes(before)
	if d["go_goroutines"] != -20 || d[keys[0]] != 3 {
		t.Fatal(d)
	}
	if _, e = Scrape(server.Client(), server.URL); e == nil {
		t.Fatal("expected HTTP error")
	}
	for _, txt := range []string{"", strings.Replace(body.String(), "1.2e2", "NaN", 1)} {
		if _, e = Parse(strings.NewReader(txt)); e == nil {
			t.Fatal("accepted incomplete or invalid scrape")
		}
	}
}

func TestWarmupDiscarded(t *testing.T) {
	e := Experiment{Arms: []Arm{{Name: "a"}, {Name: "b"}}}
	var order []string
	err := e.Measure(2, func(a Arm, round int) (RunSample, error) {
		order = append(order, a.Name)
		return RunSample{Metrics: map[string]float64{"value": float64(round)}, Invariants: []InvariantResult{{Passed: round != 0}}}, nil
	})
	if err != nil || !e.Passed || strings.Join(order, "") != "ababab" || e.Arms[0].Summary["value"].Median != 1.5 || e.Arms[1].Summary["value"].Samples != 2 {
		t.Fatal(e, err, order)
	}
}

func TestSlowKickGate(t *testing.T) {
	for _, tc := range []struct {
		arm          string
		drops, kicks float64
		pass         bool
	}{
		{"kick_enabled", 8, 1, true}, {"kick_enabled", 0, 1, false}, {"kick_enabled", 8, 0, false}, {"kick_disabled", 8, 0, true}, {"kick_disabled", 8, 1, false},
	} {
		e := Experiment{Name: "isolation_stalled", Load: "slow_kick", Arms: []Arm{{Name: tc.arm}}}
		err := e.Measure(1, func(Arm, int) (RunSample, error) {
			return RunSample{Metrics: map[string]float64{ClientFull: tc.drops, Kicked: tc.kicks}, Invariants: []InvariantResult{{Name: "delivery", Passed: true}}}, nil
		})
		if err != nil || e.Passed != tc.pass {
			t.Fatal(tc, e.Passed, err)
		}
	}
}
