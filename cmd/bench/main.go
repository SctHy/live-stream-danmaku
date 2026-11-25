package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "eval":
			if len(os.Args) < 3 {
				fatalf("usage: bench eval {p0|p1|all|report|diff}")
			}
			var err error
			switch os.Args[2] {
			case "report":
				err = evalReport(os.Args[3:])
			case "diff":
				err = evalDiff(os.Args[3:])
			case "all", "p0", "p1":
				err = evaluateSuite(os.Args[2], os.Args[3:])
			default:
				err = fmt.Errorf("unknown eval command %q", os.Args[2])
			}
			if err != nil {
				fatalf("%v", err)
			}
			return
		case "capacity", "soak":
			if err := loadBench(os.Args[1], os.Args[2:]); err != nil {
				fatalf("%v", err)
			}
			return
		case "fanout":
			fanout(os.Args[2:])
			return
		case "rooms":
			rooms(os.Args[2:])
			return
		case "migration":
			migration(os.Args[2:])
			return
		}
	}
	fmt.Fprintln(os.Stderr, "usage: bench {fanout|rooms|migration|capacity|soak|eval {p0|p1|all|diff|report}} [flags]")
	os.Exit(2)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
