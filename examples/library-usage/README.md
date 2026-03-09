# Library Usage Example

This example shows how to use the Scout-and-Wave engine as a library in your Go application.

## Overview

The example demonstrates:
1. Running Scout to generate an IMPL doc
2. Parsing the IMPL doc
3. Running Wave 1 with SSE event monitoring
4. Merging and verifying

## Files

- `main.go` — Complete example using the engine
- `go.mod` — Dependencies

## Implementation

### 1. Import Engine

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)
```

### 2. Run Scout

```go
func main() {
    ctx := context.Background()

    // Run Scout
    scoutOpts := engine.RunScoutOpts{
        Prompt:     "Add user authentication to the API",
        RepoPath:   "/path/to/repo",
        ScoutModel: "claude-sonnet-4-6",
    }

    implPath, err := engine.RunScout(ctx, scoutOpts)
    if err != nil {
        log.Fatalf("Scout failed: %v", err)
    }

    fmt.Printf("IMPL doc generated: %s\n", implPath)
}
```

### 3. Parse IMPL Doc

```go
// Parse IMPL doc
doc, err := protocol.ParseIMPLDoc(implPath)
if err != nil {
    log.Fatalf("Parse failed: %v", err)
}

fmt.Printf("Verdict: %s\n", doc.Verdict)
fmt.Printf("Waves: %d\n", len(doc.Waves))
fmt.Printf("Total agents: %d\n", countAgents(doc))
```

### 4. Run Wave with SSE Monitoring

```go
// Create orchestrator
orch := orchestrator.New(orchestrator.Config{
    RepoPath: "/path/to/repo",
    IMPLPath: implPath,
    BackendConfig: orchestrator.BackendConfig{
        Kind:      "api",
        APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
        WaveModel: "claude-sonnet-4-6",
    },
})

// Subscribe to SSE events
eventChan := orch.SSEBroker.Subscribe("session-1")
go monitorEvents(eventChan)

// Run Wave 1
err = orch.StartWave(ctx, 1)
if err != nil {
    log.Fatalf("Wave 1 failed: %v", err)
}

fmt.Println("Wave 1 completed successfully")
```

### 5. Monitor Events

```go
func monitorEvents(eventChan <-chan orchestrator.Event) {
    for event := range eventChan {
        switch event.Type {
        case "agent_started":
            agentID := event.Data["agent_id"].(string)
            fmt.Printf("Agent %s started\n", agentID)

        case "agent_output":
            agentID := event.Data["agent_id"].(string)
            chunk := event.Data["chunk"].(string)
            fmt.Printf("[%s] %s", agentID, chunk)

        case "agent_completed":
            agentID := event.Data["agent_id"].(string)
            status := event.Data["status"].(string)
            fmt.Printf("Agent %s completed with status: %s\n", agentID, status)

        case "wave_completed":
            waveNum := event.Data["wave_number"].(int)
            fmt.Printf("Wave %d completed\n", waveNum)
        }
    }
}
```

### 6. Merge and Verify

```go
// Merge wave branches
err = orch.MergeWave(ctx, 1)
if err != nil {
    log.Fatalf("Merge failed: %v", err)
}

// Run verification
err = orch.RunVerification(ctx)
if err != nil {
    log.Fatalf("Verification failed: %v", err)
}

fmt.Println("Wave 1 merged and verified successfully")
```

### 7. Multi-Wave Example

```go
// Run all waves in sequence
for waveNum := 1; waveNum <= len(doc.Waves); waveNum++ {
    fmt.Printf("Starting Wave %d...\n", waveNum)

    err = orch.StartWave(ctx, waveNum)
    if err != nil {
        log.Fatalf("Wave %d failed: %v", waveNum, err)
    }

    err = orch.MergeWave(ctx, waveNum)
    if err != nil {
        log.Fatalf("Wave %d merge failed: %v", waveNum, err)
    }

    err = orch.RunVerification(ctx)
    if err != nil {
        log.Fatalf("Wave %d verification failed: %v", waveNum, err)
    }

    fmt.Printf("Wave %d complete\n", waveNum)
}
```

## Running

```bash
export ANTHROPIC_API_KEY="your-api-key"
go run main.go
```

## Configuration

### Using Different Backends

```go
// Bedrock backend
config := orchestrator.BackendConfig{
    Kind:      "bedrock",
    WaveModel: "bedrock:claude-sonnet-4-5",
}

// OpenAI backend
config := orchestrator.BackendConfig{
    Kind:       "openai",
    OpenAIKey:  os.Getenv("OPENAI_API_KEY"),
    WaveModel:  "openai:gpt-4o",
}

// Ollama (local)
config := orchestrator.BackendConfig{
    Kind:      "ollama",
    WaveModel: "ollama:qwen2.5-coder:32b",
}
```

### Per-Agent Model Routing

```go
// Agents can specify models in their IMPL doc sections:
// **model:** claude-opus-4-6

// The orchestrator automatically creates per-agent backends
```

## TODO

- [ ] Implement complete `main.go` example
- [ ] Add error handling and retry logic
- [ ] Add configuration examples for all backends
- [ ] Add multi-wave orchestration example
- [ ] Test with real IMPL docs
