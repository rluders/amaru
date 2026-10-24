// Command doccheck validates the repository's Markdown documentation offline:
// relative links and anchors, code fences, the ADR index and the Skill layout.
//
// Usage: doccheck [root]   (root defaults to the current directory)
//
// It exits 1 when it finds problems and 2 on an I/O error.
package main

import (
	"fmt"
	"os"
)

func main() {
	root := "."
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: doccheck [root]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	problems, err := Check(root, DefaultLayout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "doccheck:", err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "doccheck: %d problem(s)\n", len(problems))
		os.Exit(1)
	}
	fmt.Println("doccheck: ok")
}
