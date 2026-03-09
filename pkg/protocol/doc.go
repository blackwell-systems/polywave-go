// Package protocol provides IMPL document parsing, validation, and extraction.
//
// # IMPL Document Format
//
// IMPL docs are markdown with structured sections:
//
//	# IMPL: Add User Authentication
//
//	**verdict:** SUITABLE
//
//	## File Ownership
//
//	| Wave | Agent | File | Lines | Dependencies |
//	|------|-------|------|-------|--------------|
//	| 1 | A | src/auth.go | 50 | crypto/bcrypt |
//	| 1 | B | src/middleware.go | 30 | src/auth.go |
//
//	## Wave 1
//
//	### Agent A — Authentication Module
//
//	**task:** Implement user authentication logic
//
//	### Agent A Completion Report
//
//	```yaml
//	status: complete
//	files:
//	  - src/auth.go
//	```
//
// See the scout-and-wave protocol spec for full format definition:
// https://github.com/blackwell-systems/scout-and-wave/tree/main/protocol
//
// # Parsing
//
// ParseIMPLDoc parses markdown IMPL docs into types.IMPLDoc:
//
//	doc, err := protocol.ParseIMPLDoc("/path/to/IMPL-feature.md")
//	if err != nil {
//	    log.Fatalf("Parse failed: %v", err)
//	}
//
//	fmt.Printf("Verdict: %s\n", doc.Verdict)
//	fmt.Printf("Waves: %d\n", len(doc.Waves))
//	for _, wave := range doc.Waves {
//	    fmt.Printf("Wave %d has %d agents\n", wave.Number, len(wave.Agents))
//	}
//
// # Validation
//
// ValidateInvariants checks protocol invariants (I1–I6):
//
//	violations := protocol.ValidateInvariants(doc)
//	if len(violations) > 0 {
//	    for _, v := range violations {
//	        fmt.Println("Invariant violation:", v)
//	    }
//	}
//
// # Per-Agent Context Extraction (E23)
//
// ExtractAgentContext trims the IMPL doc to only sections relevant to a single agent:
//
//	agentContext, err := protocol.ExtractAgentContext(doc, waveNum, agentLetter)
//	payload := protocol.FormatAgentContextPayload(agentContext)
//	// payload contains: agent prompt, interface contracts, file ownership, scaffolds, quality gates
//	// Other agents' prompts are omitted — reduces context waste
//
// # Parser Architecture
//
// The parser uses a line-by-line state machine with section header detection:
//
//   - ## Wave N → stateWave
//   - ### Agent X → stateAgent
//   - ## File Ownership → stateFileOwnership
//   - YAML fence → parse completion report
//   - Go/Rust/etc fence → parse scaffold
//
// See docs/protocol-parsing.md for parser internals and architecture.
package protocol
