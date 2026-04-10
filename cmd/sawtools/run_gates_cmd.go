package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("run-gates: %w", err)
			}

			var results []protocol.GateResult
			if noCache {
				res := protocol.RunGatesWithCache(cmd.Context(), m, waveNum, repoDir, manifestPath, nil, nil)
				if !res.IsSuccess() {
					return fmt.Errorf("run-gates: %w", errors.Join(result.ToErrors(res.Errors)...))
				}
				results = res.GetData().Gates
			} else {
				stateDir := protocol.SAWStateDir(repoDir)
				cache := gatecache.New(cmd.Context(), stateDir, gatecache.DefaultTTL)
				res := protocol.RunGatesWithCache(cmd.Context(), m, waveNum, repoDir, manifestPath, cache, nil)
				if !res.IsSuccess() {
					return fmt.Errorf("run-gates: %w", errors.Join(result.ToErrors(res.Errors)...))
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
