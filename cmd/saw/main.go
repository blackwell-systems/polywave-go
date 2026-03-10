package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}
	switch os.Args[1] {
	// Wave 2 commands will be wired here by Agent U (Wave 3)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(1)
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "Usage: saw <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  (commands will be wired in Wave 3)")
}
