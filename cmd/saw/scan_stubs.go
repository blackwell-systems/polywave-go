package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runScanStubs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("scan-stubs: at least one file path required\nUsage: saw scan-stubs <file1> [file2 ...]")
	}

	result, err := protocol.ScanStubs(args)
	if err != nil {
		return fmt.Errorf("scan-stubs: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
