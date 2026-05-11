package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	obs "github.com/blackwell-systems/polywave-go/pkg/observability"
	"github.com/spf13/cobra"
)

// newQueryCmd returns a cobra.Command that queries observability events
// with various filters and output formats.
func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query observability data",
	}

	cmd.AddCommand(newQueryEventsCmd())

	return cmd
}

// newQueryEventsCmd returns the "query events" subcommand.
func newQueryEventsCmd() *cobra.Command {
	var (
		eventType   string
		implSlug    string
		programSlug string
		agentID     string
		since       string
		format      string
		limit       int
		storeDSN    string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Query observability events with filters",
		Long:  "Query observability events by type, IMPL, agent, and time range.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			store, err := openStore(storeDSN)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			filters := obs.QueryFilters{
				Limit: limit,
			}

			if eventType != "" {
				filters.EventTypes = []string{eventType}
			}
			if implSlug != "" {
				filters.IMPLSlugs = []string{implSlug}
			}
			if programSlug != "" {
				filters.ProgramSlugs = []string{programSlug}
			}
			if agentID != "" {
				filters.AgentIDs = []string{agentID}
			}
			if since != "" {
				d, err := parseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				t := time.Now().Add(-d)
				filters.StartTime = &t
			}

			events, err := store.QueryEvents(ctx, filters)
			if err != nil {
				return fmt.Errorf("query events: %w", err)
			}

			switch format {
			case "json":
				return outputJSON(events)
			case "csv":
				return outputCSV(events)
			default:
				return outputTable(events)
			}
		},
	}

	cmd.Flags().StringVar(&eventType, "type", "", "Event type filter (cost, agent_performance, activity)")
	cmd.Flags().StringVar(&implSlug, "impl", "", "IMPL slug filter")
	cmd.Flags().StringVar(&programSlug, "program", "", "Program slug filter")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID filter")
	cmd.Flags().StringVar(&since, "since", "", "Time range (e.g., 24h, 7d, 30d)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, csv)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results to return")
	cmd.Flags().StringVar(&storeDSN, "store", "", "Store DSN (default: ~/.saw/observability.db)")

	return cmd
}

// parseDuration parses human-friendly duration strings like "24h", "7d", "30d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("parse days: %w", err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func outputTable(events []obs.Event) error {
	if len(events) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tTYPE\tTIMESTAMP\tDETAILS\n")

	for _, e := range events {
		details := eventDetails(e)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			truncate(e.EventID(), 12),
			e.EventType(),
			e.Timestamp().Format(time.RFC3339),
			details,
		)
	}
	return w.Flush()
}

func outputJSON(events []obs.Event) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(events)
}

func outputCSV(events []obs.Event) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.Write([]string{"id", "type", "timestamp", "details"}); err != nil {
		return err
	}
	for _, e := range events {
		if err := w.Write([]string{
			e.EventID(),
			e.EventType(),
			e.Timestamp().Format(time.RFC3339),
			eventDetails(e),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// eventDetails returns a one-line summary of an event's key fields.
func eventDetails(e obs.Event) string {
	switch ev := e.(type) {
	case *obs.CostEvent:
		return fmt.Sprintf("agent=%s impl=%s cost=$%.4f model=%s", ev.AgentID, ev.IMPLSlug, ev.CostUSD, ev.Model)
	case *obs.AgentPerformanceEvent:
		return fmt.Sprintf("agent=%s impl=%s status=%s duration=%ds", ev.AgentID, ev.IMPLSlug, ev.Status, ev.DurationSeconds)
	case *obs.ActivityEvent:
		return fmt.Sprintf("activity=%s impl=%s wave=%d", ev.ActivityType, ev.IMPLSlug, ev.WaveNumber)
	default:
		return ""
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
