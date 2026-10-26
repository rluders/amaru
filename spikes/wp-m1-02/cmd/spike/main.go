// Command spike runs the WP-M1-02 experiments. See ../../README.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: spike bundle|pg|platform ...")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "bundle":
		err = cmdBundle(os.Args[2:])
	case "pg":
		err = cmdPG(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "spike:", err)
		os.Exit(1)
	}
}
