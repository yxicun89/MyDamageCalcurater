// Command checkreadmodel validates a directory that holds the speed read model
// (speed-pokemon.json) with the same loader the service uses at startup, so a bad export
// fails before it is deployed (ADR-0603 §3).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: checkreadmodel <dir>")
		os.Exit(2)
	}
	summary, err := check(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read model check failed:", err)
		os.Exit(1)
	}
	fmt.Println(summary)
}
