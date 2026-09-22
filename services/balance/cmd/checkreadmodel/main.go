// Command checkreadmodel validates a directory of balance read models (pokemon-types.json,
// moves.json, abilities.json) with the same loaders the service uses at startup, so a bad
// export fails before it is deployed (ADR-0403).
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
