package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runUpdateStatus(args []string) error {
	fs := flag.NewFlagSet("update-status", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	agentID := fs.String("agent", "", "Agent ID (required)")
	status := fs.String("status", "", "Status: complete|partial|blocked (required)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("update-status: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("update-status: manifest path required\nUsage: saw update-status <manifest-path> --wave <N> --agent <ID> --status <complete|partial|blocked>")
	}
	if *waveNum == 0 {
		return fmt.Errorf("update-status: --wave is required")
	}
	if *agentID == "" {
		return fmt.Errorf("update-status: --agent is required")
	}
	if *status == "" {
		return fmt.Errorf("update-status: --status is required")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.UpdateStatus(manifestPath, *waveNum, *agentID, *status)
	if err != nil {
		return fmt.Errorf("update-status: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
