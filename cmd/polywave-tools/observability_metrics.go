package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	obs "github.com/blackwell-systems/polywave-go/pkg/observability"
	"github.com/blackwell-systems/polywave-go/pkg/observability/sqlite"
	"github.com/spf13/cobra"
)

// newMetricsCmd returns a cobra.Command that shows cost and performance metrics
// for an IMPL, using the observability store.
func newMetricsCmd() *cobra.Command {
	var (
		programSlug string
		breakdown   bool
		storeDSN    string
	)

	cmd := &cobra.Command{
		Use:   "metrics <impl-slug>",
		Short: "Show metrics for an IMPL (cost, duration, success rate)",
		Long:  "Show metrics for an IMPL from the observability store (cost, duration, success rate).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implSlug := args[0]
			ctx := context.Background()

			store, err := openStore(storeDSN)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			// If --program is set, show program-level summary instead.
			if programSlug != "" {
				return showProgramSummary(ctx, store, programSlug)
			}

			metricsRes := obs.GetIMPLMetrics(ctx, store, implSlug)
			if metricsRes.IsFatal() {
				return fmt.Errorf("get metrics: %s", metricsRes.Errors[0].Message)
			}
			metrics := metricsRes.GetData()

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "IMPL\t%s\n", implSlug)
			fmt.Fprintf(w, "Total Cost\t$%.4f\n", metrics.TotalCost)
			fmt.Fprintf(w, "Total Duration\t%.1f min\n", metrics.TotalDurationMin)
			fmt.Fprintf(w, "Success Rate\t%.1f%%\n", metrics.SuccessRate*100)
			fmt.Fprintf(w, "Agents Failed\t%d\n", metrics.AgentsFailed)
			fmt.Fprintf(w, "Waves Completed\t%d\n", metrics.WavesCompleted)
			w.Flush()

			if breakdown {
				bdRes := obs.GetCostBreakdown(ctx, store, implSlug)
				if bdRes.IsFatal() {
					return fmt.Errorf("get cost breakdown: %s", bdRes.Errors[0].Message)
				}
				bd := bdRes.GetData().PerAgent
				if len(bd) > 0 {
					fmt.Println()
					fmt.Println("Cost Breakdown by Agent:")
					bw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
					fmt.Fprintf(bw, "AGENT\tCOST\n")
					// Sort agents for deterministic output.
					agents := make([]string, 0, len(bd))
					for a := range bd {
						agents = append(agents, a)
					}
					sort.Strings(agents)
					for _, a := range agents {
						fmt.Fprintf(bw, "%s\t$%.4f\n", a, bd[a])
					}
					bw.Flush()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&programSlug, "program", "", "Show program-level summary instead of IMPL metrics")
	cmd.Flags().BoolVar(&breakdown, "breakdown", false, "Show per-agent cost breakdown")
	cmd.Flags().StringVar(&storeDSN, "store", "", "Store DSN (default: ~/.saw/observability.db)")

	return cmd
}

func showProgramSummary(ctx context.Context, store obs.Store, programSlug string) error {
	summaryRes := obs.GetProgramSummary(ctx, store, programSlug)
	if summaryRes.IsFatal() {
		return fmt.Errorf("get program summary: %s", summaryRes.Errors[0].Message)
	}
	summary := summaryRes.GetData()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Program\t%s\n", programSlug)
	fmt.Fprintf(w, "Total Cost\t$%.4f\n", summary.TotalCost)
	fmt.Fprintf(w, "IMPL Count\t%d\n", summary.IMPLCount)
	fmt.Fprintf(w, "Total Duration\t%.1f min\n", summary.TotalDurationMin)
	fmt.Fprintf(w, "Success Rate\t%.1f%%\n", summary.OverallSuccessRate*100)
	w.Flush()

	return nil
}

// openStore creates an observability.Store from the given DSN.
// If dsn is empty, defaults to ~/.saw/observability.db (SQLite).
// For now, returns an in-memory stub since concrete store implementations
// (SQLite, PostgreSQL) are provided by other agents/packages.
func openStore(dsn string) (obs.Store, error) {
	if dsn == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		dsn = home + "/.saw/observability.db"
	}

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("PostgreSQL store not yet implemented; use SQLite file path")
	}

	// Ensure parent directory exists.
	dir := dsn[:strings.LastIndex(dsn, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create store directory: %w", err)
		}
	}

	return sqlite.Open(dsn)
}
