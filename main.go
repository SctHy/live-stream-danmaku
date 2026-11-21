package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println("Go version:", runtime.Version())
	fmt.Println("NumCPU:", runtime.NumCPU())
	fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))
}
