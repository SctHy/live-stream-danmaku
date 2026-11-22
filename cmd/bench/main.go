package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
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
	fmt.Fprintln(os.Stderr, "usage: bench {fanout|rooms|migration} [flags]")
	os.Exit(2)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
