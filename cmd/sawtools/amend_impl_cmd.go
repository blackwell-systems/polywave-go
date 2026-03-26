package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newAmendImplCmd() *cobra.Command {
	var (
		addWave       bool
		redirectAgent string
		waveNum       int
		newTask       string
		extendScope   bool
	)

	cmd := &cobra.Command{
		Use:   "amend-impl <manifest-path>",
		Short: "Amend a living IMPL doc: add a wave, redirect an agent, or extend scope",
		Long: `Mutates a living IMPL doc in one of three ways:

  --add-wave          Append a new empty wave skeleton to the manifest.
  --redirect-agent ID Re-queue an agent: update its task and clear its completion report.
  --extend-scope      Print instructions for re-engaging Scout with this IMPL as context.

Exactly one of --add-wave, --redirect-agent, or --extend-scope must be provided.
Output is always JSON.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Validate exactly one operation flag is set
			opCount := 0
			if addWave {
				opCount++
			}
			if redirectAgent != "" {
				opCount++
			}
			if extendScope {
				opCount++
			}
			if opCount == 0 {
				return fmt.Errorf("amend-impl: exactly one of --add-wave, --redirect-agent, or --extend-scope must be provided")
			}
			if opCount > 1 {
				return fmt.Errorf("amend-impl: only one of --add-wave, --redirect-agent, or --extend-scope may be provided at a time")
			}

			// --extend-scope: handled entirely in CLI layer
			if extendScope {
				out, _ := json.MarshalIndent(map[string]string{
					"operation":     "extend-scope",
					"manifest_path": manifestPath,
					"message":       "Re-engage Scout with --impl-context " + manifestPath,
				}, "", "  ")
				fmt.Println(string(out))
				return nil
			}

			// --redirect-agent: validate required companion flags
			if redirectAgent != "" {
				if waveNum == 0 {
					return fmt.Errorf("amend-impl: --wave is required with --redirect-agent")
				}
				// Read new task from stdin if not provided via flag
				if newTask == "" {
					fmt.Fprintln(os.Stderr, "Reading new task from stdin...")
					scanner := bufio.NewScanner(os.Stdin)
					var lines []string
					for scanner.Scan() {
						lines = append(lines, scanner.Text())
					}
					if err := scanner.Err(); err != nil {
						return fmt.Errorf("amend-impl: reading stdin: %w", err)
					}
					for _, line := range lines {
						if newTask != "" {
							newTask += "\n"
						}
						newTask += line
					}
				}
			}

			// Call protocol engine for --add-wave and --redirect-agent
			res := protocol.AmendImpl(protocol.AmendImplOpts{
				ManifestPath:  manifestPath,
				AddWave:       addWave,
				RedirectAgent: redirectAgent != "",
				AgentID:       redirectAgent,
				WaveNum:       waveNum,
				NewTask:       newTask,
			})
			if !res.IsSuccess() {
				op := "add-wave"
				if redirectAgent != "" {
					op = "redirect-agent"
				}
				msg := "amend-impl failed"
				if len(res.Errors) > 0 {
					msg = res.Errors[0].Message
				}
				errResp := map[string]interface{}{
					"success":   false,
					"operation": op,
					"error":     msg,
				}
				if len(res.Errors) > 0 && res.Errors[0].Code == "AMEND_BLOCKED" {
					out, _ := json.MarshalIndent(errResp, "", "  ")
					fmt.Println(string(out))
					return fmt.Errorf("amend-impl: blocked")
				}
				return fmt.Errorf("amend-impl: %s", msg)
			}
			data := res.GetData()

			out, _ := json.MarshalIndent(map[string]interface{}{
				"success":         true,
				"operation":       data.Operation,
				"new_wave_number": data.NewWaveNumber,
				"agent_id":        data.AgentID,
				"warnings":        data.Warnings,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVar(&addWave, "add-wave", false, "Append a new empty wave skeleton to the manifest")
	cmd.Flags().StringVar(&redirectAgent, "redirect-agent", "", "Agent ID to redirect (e.g. \"B\")")
	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number for --redirect-agent (required with --redirect-agent)")
	cmd.Flags().StringVar(&newTask, "new-task", "", "Replacement task text for --redirect-agent (reads from stdin if omitted)")
	cmd.Flags().BoolVar(&extendScope, "extend-scope", false, "Re-engage Scout with current IMPL as context")

	return cmd
}
