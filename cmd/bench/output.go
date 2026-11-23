package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/scthy/maku/internal/eval"
)

func readLedger(path string) (r eval.Report, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return
}
func val(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%.6g", *v)
}
func evalDiff(args []string) error {
	f := flag.NewFlagSet("eval diff", flag.ContinueOnError)
	bp := f.String("baseline", "", "baseline ledger")
	cp := f.String("candidate", "", "candidate ledger")
	threshold := f.Float64("threshold", .15, "relative regression threshold")
	allow := f.Bool("allow-missing", false, "allow absent baseline items")
	j := f.Bool("json", false, "print JSON")
	out := f.String("out", "", "JSON output path")
	if err := f.Parse(args); err != nil {
		return err
	}
	b, err := readLedger(*bp)
	if err != nil {
		return err
	}
	c, err := readLedger(*cp)
	if err != nil {
		return err
	}
	d, err := eval.CompareReports(b, c, *threshold, *allow)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if *out != "" {
		if err = os.WriteFile(*out, data, 0644); err != nil {
			return err
		}
	}
	if *j {
		_, err = fmt.Fprintln(os.Stdout, string(data))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "Experiment\tLoad\tArm\tMetric\tBaseline\tCandidate\tStatus")
		for _, r := range d.Rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Experiment, r.Load, r.Arm, r.Metric, val(r.Baseline), val(r.Candidate), r.Status)
		}
		fmt.Fprintf(w, "Gate passed: %t\n", d.Passed)
		err = w.Flush()
	}
	if err != nil {
		return err
	}
	if !d.Passed {
		return errors.New("regression gate failed")
	}
	return nil
}

func evalReport(args []string) error {
	f := flag.NewFlagSet("eval report", flag.ContinueOnError)
	in := f.String("in", "", "input ledger")
	base := f.String("baseline", "", "baseline ledger")
	md := f.String("md", "", "Markdown path")
	html := f.String("html", "", "HTML path")
	if err := f.Parse(args); err != nil {
		return err
	}
	r, err := readLedger(*in)
	if err != nil {
		return err
	}
	var diff *eval.Diff
	if *base != "" {
		b, err := readLedger(*base)
		if err != nil {
			return err
		}
		d, err := eval.CompareReports(b, r, .15, false)
		if err != nil {
			return err
		}
		diff = &d
	}
	m, h, err := eval.Render(r, diff)
	if err != nil {
		return err
	}
	if *md == "" && *html == "" {
		_, err = fmt.Print(m)
		return err
	}
	if *md != "" {
		if err = os.WriteFile(*md, []byte(m), 0644); err != nil {
			return err
		}
	}
	if *html != "" {
		return os.WriteFile(*html, []byte(h), 0644)
	}
	return nil
}
