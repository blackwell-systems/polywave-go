// Package types defines shared protocol types used across the engine.
//
// # Core Types
//
// IMPLDoc — Parsed IMPL document structure:
//
//	type IMPLDoc struct {
//	    Title              string
//	    Verdict            string // "SUITABLE" | "NOT SUITABLE"
//	    FileOwnership      []FileOwnershipInfo
//	    InterfaceContracts string
//	    Waves              []Wave
//	    QualityGates       []QualityGate
//	    // ...
//	}
//
// Wave — A set of agents that execute in parallel:
//
//	type Wave struct {
//	    Number int
//	    Agents []Agent
//	}
//
// Agent — An agent specification with task and owned files:
//
//	type Agent struct {
//	    Letter       string // "A", "B", "A2", etc.
//	    Task         string
//	    Dependencies []string
//	    Files        []string
//	    Model        string // Optional per-agent model override
//	    // ...
//	}
//
// CompletionReport — Agent completion report (parsed from YAML):
//
//	type CompletionReport struct {
//	    Status      string // "complete" | "partial" | "blocked"
//	    FailureType string // "transient" | "fixable" | "needs_replan" | "escalate" | "timeout"
//	    Files       []string
//	    Deviations  string
//	    Repo        string // Optional, for cross-repo mode
//	}
//
// # File Ownership
//
// FileOwnershipInfo — A row from the file ownership table:
//
//	type FileOwnershipInfo struct {
//	    Wave         int
//	    Agent        string
//	    File         string
//	    Lines        string
//	    Dependencies string
//	    Repo         string // Optional, for cross-repo mode
//	}
//
// # Quality Gates
//
// QualityGate — A custom verification gate from the IMPL doc:
//
//	type QualityGate struct {
//	    Name     string
//	    Command  string
//	    Required bool
//	}
//
// # Status Constants
//
// Agent completion status:
//   - StatusComplete — "complete"
//   - StatusPartial — "partial"
//   - StatusBlocked — "blocked"
//
// Failure types (E19 routing):
//   - FailureTypeTransient — "transient"
//   - FailureTypeFixable — "fixable"
//   - FailureTypeNeedsReplan — "needs_replan"
//   - FailureTypeEscalate — "escalate"
//   - FailureTypeTimeout — "timeout"
//
// See docs/protocol-parsing.md for how these types are populated from IMPL docs.
package types
