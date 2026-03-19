package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newRunGatesCmd() *cobra.Command {
	var waveNum int
	var noCache bool

	cmd := &cobra.Command{
		Use:   "run-gates <manifest-path>",
		Short: "Run quality gates from manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("run-gates: %w", err)
			}

			var results []protocol.GateResult
			if noCache {
				results, err = protocol.RunGates(m, waveNum, repoDir)
			} else {
				stateDir := filepath.Join(repoDir, ".saw-state")
				cache := gatecache.New(stateDir, gatecache.DefaultTTL)
				results, err = protocol.RunGatesWithCache(m, waveNum, repoDir, cache)
			}
			if err != nil {
				return fmt.Errorf("run-gates: %w", err)
			}

			out, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(out))

			for _, r := range results {
				if r.Required && !r.Passed {
					os.Exit(1)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable gate result caching")

	return cmd
}
