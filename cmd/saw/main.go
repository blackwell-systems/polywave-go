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
	case "create-worktrees":
		if err := runCreateWorktrees(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "verify-commits":
		if err := runVerifyCommits(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "scan-stubs":
		if err := runScanStubs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "merge-agents":
		if err := runMergeAgents(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "cleanup":
		if err := runCleanup(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "verify-build":
		if err := runVerifyBuild(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "update-status":
		if err := runUpdateStatus(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "update-context":
		if err := runUpdateContext(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "list-impls":
		if err := runListIMPLs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "run-wave":
		if err := runRunWave(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
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
	fmt.Fprintln(w, "  create-worktrees   Create git worktrees for all agents in a wave")
	fmt.Fprintln(w, "  verify-commits     Verify each agent branch has commits (I5 trip wire)")
	fmt.Fprintln(w, "  scan-stubs         Scan files for stub/TODO patterns (E20)")
	fmt.Fprintln(w, "  merge-agents       Merge all agent branches for a wave")
	fmt.Fprintln(w, "  cleanup            Remove worktrees and branches after merge")
	fmt.Fprintln(w, "  verify-build       Run test and lint commands from manifest")
	fmt.Fprintln(w, "  update-status      Update agent status in manifest")
	fmt.Fprintln(w, "  update-context     Update project CONTEXT.md (E18)")
	fmt.Fprintln(w, "  list-impls         List all IMPL manifests in a directory")
	fmt.Fprintln(w, "  run-wave           Execute full wave lifecycle (create, verify, merge, build, cleanup)")
}
