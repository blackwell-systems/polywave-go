package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runUpdateContext(args []string) error {
	fs := flag.NewFlagSet("update-context", flag.ContinueOnError)
	projectRoot := fs.String("project-root", ".", "Project root directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("update-context: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("update-context: manifest path required\nUsage: saw update-context <manifest-path> [--project-root <path>]")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.UpdateContext(manifestPath, *projectRoot)
	if err != nil {
		return fmt.Errorf("update-context: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
