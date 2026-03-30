package main

import (
	"encoding/json"
	"fmt"

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
				res := protocol.RunGatesWithCache(m, waveNum, repoDir, nil, nil)
				if !res.IsSuccess() {
					return fmt.Errorf("run-gates: %v", res.Errors)
				}
				results = res.GetData().Gates
			} else {
				stateDir := protocol.SAWStateDir(repoDir)
				cache := gatecache.New(stateDir, gatecache.DefaultTTL)
				res := protocol.RunGatesWithCache(m, waveNum, repoDir, cache, nil)
				if !res.IsSuccess() {
					return fmt.Errorf("run-gates: %v", res.Errors)
				}
				results = res.GetData().Gates
			}

			out, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(out))

			for _, r := range results {
				if r.Required && !r.Passed {
					return fmt.Errorf("run-gates: required gate failed")
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable gate result caching")

	return cmd
}
