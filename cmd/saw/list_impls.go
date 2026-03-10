package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// runListIMPLs scans a directory for IMPL manifest files and returns summaries.
// Command: saw list-impls [--dir <path>]
// Outputs JSON result with IMPL summaries (path, feature_slug, verdict, current_wave, total_waves).
// Always exits 0 (empty list is valid).
func runListIMPLs(args []string) error {
	fs := flag.NewFlagSet("list-impls", flag.ContinueOnError)
	dir := fs.String("dir", "docs/IMPL", "Directory to scan for IMPL manifests")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("list-impls: %w", err)
	}

	result, err := protocol.ListIMPLs(*dir)
	if err != nil {
		return fmt.Errorf("list-impls: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
