// Package journal implements an external log observer for tool execution tracking
// in Claude Code sessions, enabling context recovery after compaction.
//
// # Architecture: External Observer Pattern
//
// The journal system operates as an external observer that tails Claude Code's
// session logs (~/.claude/projects/<project>/*.jsonl) without requiring any
// modifications to the Claude Code backend or agent prompt. This architectural
// choice provides several key benefits:
//
//   - Zero backend coupling: Works with any Claude Code version
//   - Retroactive enablement: Can observe existing sessions
//   - Cross-agent visibility: One observer can track multiple agents
//   - Post-mortem debugging: Logs persist after agent termination
//
// # Session Log Format
//
// Claude Code writes structured JSONL logs where each line is a JSON object
// containing conversation turns, tool executions, and results. The journal
// observer incrementally parses these logs to extract tool_use and tool_result
// blocks, building an indexed history of agent actions.
//
// # File Structure
//
// For each agent, the observer maintains:
//
//	.polywave-state/wave{N}/agent-{ID}/
//	├── cursor.json              # SessionCursor: tracks read position
//	├── index.jsonl              # ToolEntry per line (append-only)
//	├── recent.json              # Last 30 entries (JSON array)
//	└── tool-results/
//	    ├── toolu_abc123.txt     # Full output for tool use abc123
//	    └── toolu_def456.txt
//
// # Usage
//
// Create an observer for an agent and sync periodically:
//
//	observer, err := journal.NewObserver("/path/to/repo", "agent-A")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	result, err := observer.Sync()
//	if err != nil {
//	    log.Printf("sync failed: %v", err)
//	}
//	fmt.Printf("Synced %d tool uses, %d results\n",
//	    result.NewToolUses, result.NewToolResults)
//
// # Compaction Safety
//
// When agents hit context limits and compact their conversation history, they
// lose visibility into their execution history. The journal provides recovery:
//
//  1. Agent prompt can query journal for "what did I just do?"
//  2. Context generator summarizes recent tool executions
//  3. Checkpoint manager creates named snapshots at milestones
//  4. Archive policy compresses old journals after wave completion
//
// # Persistence Guarantees
//
// The cursor is persisted after each sync, ensuring that even if the observer
// process crashes, the next sync will resume from the last committed position.
// The index.jsonl file is append-only, preventing data loss from concurrent
// writers or partial writes.
package journal
