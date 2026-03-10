package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runVerifyBuild(args []string) error {
	fs := flag.NewFlagSet("verify-build", flag.ContinueOnError)
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("verify-build: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("verify-build: manifest path required\nUsage: saw verify-build <manifest-path> [--repo-dir <path>]")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.VerifyBuild(manifestPath, *repoDir)
	if err != nil {
		return fmt.Errorf("verify-build: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	if !result.TestPassed || !result.LintPassed {
		os.Exit(1)
	}
	return nil
}
