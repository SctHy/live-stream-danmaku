package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/scthy/maku/internal/eval"
)

func suite(profile string) []eval.Experiment {
	var experiments []eval.Experiment
	for i, load := range []string{"low", "medium", "high"} {
		viewers := []int{32, 96, 160}[i]
		rate := []int{100, 400, 600}[i]
		if profile == "full" && i == 2 {
			viewers = 224
			rate = 700
		}
		e := eval.Experiment{Name: "fanout_sweep", Load: load}
		for _, name := range []string{"legacy", "adaptive_batch", "mixed_25pct_legacy"} {
			a := fanArm(name, viewers, rate)
			switch name {
			case "legacy":
				a.Args = append(a.Args, "-legacy")
			case "mixed_25pct_legacy":
				a.Args = append(a.Args, "-legacy-every", "4")
			}
			e.Arms = append(e.Arms, a)
		}
		experiments = append(experiments, e)
	}
	iso := eval.Experiment{Name: "isolation_stalled"}
	for _, name := range []string{"baseline_single_shard_unbounded", "protected_default"} {
		a := fanArm(name, 160, 800)
		a.Args = append(a.Args, "-stalled", "12", "-idle-rooms", "32", "-churn", "50ms")
		if name != "protected_default" {
			a.GatewayArgs = []string{"-room-shards", "1", "-send-queue", "8192", "-bench-disable-slow-kick"}
		}
		iso.Arms = append(iso.Arms, a)
	}
	experiments = append(experiments, iso)
	migration := eval.Experiment{Name: "migration_strategies"}
	viewers, events := "120", "24"
	if profile == "full" {
		viewers, events = "144", "24"
	}
	for _, strategy := range []string{"crash", "fixed", "bounded"} {
		migration.Arms = append(migration.Arms, eval.Arm{Name: strategy, Args: []string{"-events", events, "-strategy", strategy, "-viewers", viewers, "-batch", "24", "-inflight", "4", "-port", "0"}})
	}
	return append(experiments, migration)
}
func fanArm(name string, viewers, rate int) eval.Arm {
	return eval.Arm{Name: name, Args: []string{"-viewers", strconv.Itoa(viewers), "-rate", strconv.Itoa(rate), "-seconds", "2", "-cold-rate", "100", "-sample-viewers", "8"}}
}
func measure(bin, experiment string, a eval.Arm) (sample eval.RunSample, err error) {
	var passed bool
	var detail string
	if experiment == "migration_strategies" {
		r, e := runMigration(append([]string{"-gateway", bin}, a.Args...), true)
		if e != nil {
			return sample, e
		}
		sample.Metrics = r.Metrics
		passed = r.Passed
		detail = strings.Join(r.Errors, ", ")
	} else {
		addr, _, e := freeAddresses()
		if e != nil {
			return sample, e
		}
		l := lab{http: &http.Client{Timeout: 3 * time.Second}}
		_, end, e := l.launch(bin, addr, a.GatewayArgs...)
		if e != nil {
			return sample, e
		}
		defer end()
		url := "http://" + addr + "/metrics"
		before, e := eval.Scrape(l.http, url)
		if e != nil {
			return sample, e
		}
		stop := make(chan struct{})
		done := make(chan struct{})
		peak := before["process_resident_memory_bytes"]
		var pollErr error
		go func() {
			defer close(done)
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					s, e := eval.Scrape(l.http, url)
					if e != nil {
						pollErr = e
						return
					}
					peak = max(peak, s["process_resident_memory_bytes"])
				}
			}
		}()
		r, _, e := runFanout(append([]string{"-addr", addr}, a.Args...), io.Discard)
		close(stop)
		<-done
		if e != nil {
			return sample, e
		}
		if pollErr != nil {
			return sample, pollErr
		}
		after, e := eval.Scrape(l.http, url)
		if e != nil {
			return sample, e
		}
		sample.Metrics = r.Metrics
		passed = r.Passed
		for k, v := range after.Changes(before) {
			sample.Metrics[k] = v
		}
		sample.Metrics["rss_peak_bytes"] = max(peak, after["process_resident_memory_bytes"])
	}
	for k, n := range sample.Metrics {
		if strings.HasSuffix(k, "_samples") && n < 100 {
			sample.LowSampleWarning = append(sample.LowSampleWarning, k)
		}
	}
	slices.Sort(sample.LowSampleWarning)
	sample.Invariants = []eval.InvariantResult{{Name: "complete_ordered_unique_delivery", Passed: passed, Detail: detail}}
	return sample, nil
}
func evaluate(args []string) error {
	return evaluateSuite("p0", args)
}
func evaluateSuite(mode string, args []string) error {
	f := flag.NewFlagSet("eval "+mode, flag.ContinueOnError)
	profile := f.String("profile", "quick", "quick or full")
	repeats := f.Int("repeats", 0, "measured runs per arm")
	bin := f.String("gateway", "", "gateway executable")
	only := f.String("only", "", "comma-separated experiment names")
	dsn := f.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")
	addr := f.String("redis-addr", os.Getenv("REDIS_ADDR"), "Redis host:port")
	out := f.String("out", mode+".json", "JSON ledger")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *profile != "quick" && *profile != "full" {
		return errors.New("invalid profile")
	}
	specified := false
	f.Visit(func(x *flag.Flag) {
		if x.Name == "repeats" {
			specified = true
		}
	})
	if !specified {
		*repeats = 3
		if *profile == "full" {
			*repeats = 5
		}
	}
	if *repeats < 1 {
		return errors.New("repeats must be positive")
	}
	names := map[string]bool{}
	p0 := suite(*profile)
	if mode != "p1" {
		for _, e := range p0 {
			names[e.Name] = true
		}
	}
	if mode != "p0" {
		for _, n := range []string{"history_pagination", "pg_archive_write", "redis_lua_atomicity", "sensitive_filter_scaling", "live_source_singleflight"} {
			names[n] = true
		}
	}
	if *only != "" {
		selection := map[string]bool{}
		for _, n := range strings.Split(*only, ",") {
			n = strings.TrimSpace(n)
			if !names[n] {
				return fmt.Errorf("unknown experiment %q", n)
			}
			selection[n] = true
		}
		names = selection
	}
	if *bin == "" && (names["fanout_sweep"] || names["isolation_stalled"] || names["migration_strategies"]) {
		dir, e := os.MkdirTemp("", "maku-eval-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(dir)
		*bin = filepath.Join(dir, "gateway")
		c := exec.Command("go", "build", "-o", *bin, "./cmd/gateway")
		c.Stderr = os.Stderr
		if e = c.Run(); e != nil {
			return e
		}
	}
	sha, e := exec.Command("git", "rev-parse", "HEAD").Output()
	if e != nil {
		return e
	}
	status, e := exec.Command("git", "status", "--porcelain").Output()
	if e != nil {
		return e
	}
	report := eval.Report{Env: eval.EnvMetadata{GitSHA: strings.TrimSpace(string(sha)), GitDirty: len(status) > 0, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, NumCPU: runtime.NumCPU(), Timestamp: time.Now().UTC(), Profile: *profile, Repeats: *repeats}}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "experiment\tload\tarm\tmetric\tmedian\tvs_baseline\ttrade_off")
	for _, ex := range p0 {
		if !names[ex.Name] {
			continue
		}
		err := ex.Measure(*repeats, func(a eval.Arm, round int) (eval.RunSample, error) {
			fmt.Fprintf(os.Stderr, "%s %s %s round %d/%d\n", ex.Name, ex.Load, a.Name, round, *repeats)
			return measure(*bin, ex.Name, a)
		})
		if err != nil {
			return err
		}
		for _, a := range ex.Arms {
			for _, k := range []string{"frames", "process_cpu_seconds_total", "hot_p50_us", "hot_p99_us", "cold_p99_us", "go_memstats_alloc_bytes_total", "missing", "danmaku_kicked_slow_clients_total", "rss_peak_bytes", "connect_peak_qps", "recent_peak_qps", "resident_during_handoff_p99_us", "recovery_p99_ms", "drain_ms", "last_handoff_ms"} {
				if s, ok := a.Summary[k]; ok {
					change := "—"
					if d := a.Deltas[k]; d.Percent != nil {
						change = fmt.Sprintf("%+.1f%%", *d.Percent)
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.3f\t%s\t%t\n", ex.Name, ex.Load, a.Name, k, s.Median, change, a.Deltas[k].TradeOff)
				}
			}
		}
		if err := w.Flush(); err != nil {
			return err
		}
		report.Experiments = append(report.Experiments, ex)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	s := p1Suite{ctx: ctx, full: *profile == "full", repeats: *repeats, selected: names, report: report}
	if names["history_pagination"] || names["pg_archive_write"] {
		if err := s.postgres(*dsn); err != nil {
			return err
		}
	}
	if names["redis_lua_atomicity"] {
		if *addr == "" {
			*addr = "127.0.0.1:6379"
		}
		if err := s.redisSuite(*addr); err != nil {
			return err
		}
	}
	if names["sensitive_filter_scaling"] {
		if err := s.filters(); err != nil {
			return err
		}
	}
	if names["live_source_singleflight"] {
		if err := s.live(); err != nil {
			return err
		}
	}
	report = s.report

	data, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(*out, data, 0644); e != nil {
		return e
	}
	for _, ex := range report.Experiments {
		if !ex.Passed {
			return fmt.Errorf("%s/%s invariant failed; see %s", ex.Name, ex.Load, *out)
		}
	}
	return nil
}
