package orchestrator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
)

// PrepareAgentContext loads journal history and generates context.md for agent recovery.
// Returns empty string if no journal exists (first launch).
// Returns error if journal exists but cannot be read (differentiate from "no journal").
func PrepareAgentContext(projectRoot string, waveNum int, agentID string, maxEntries int) (string, error) {
	// Default maxEntries to 50 per E23A spec
	if maxEntries == 0 {
		maxEntries = 50
	}

	// Construct journal path: .saw-state/wave{N}/agent-{ID}/index.jsonl
	journalPath := filepath.Join(projectRoot, ".saw-state", fmt.Sprintf("wave%d", waveNum), agentID, "index.jsonl")

	// Check if journal exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		// No journal yet - this is first launch, return empty string
		return "", nil
	} else if err != nil {
		// Journal exists but cannot be accessed - return error
		return "", fmt.Errorf("failed to access journal at %s: %w", journalPath, err)
	}

	// Read all entries from index.jsonl
	entries, err := loadJournalEntries(journalPath)
	if err != nil {
		return "", fmt.Errorf("failed to load journal entries: %w", err)
	}

	// If journal is empty, return empty string (equivalent to no journal)
	if len(entries) == 0 {
		return "", nil
	}

	// Generate context markdown using journal.GenerateContext
	contextMd, err := journal.GenerateContext(entries, maxEntries)
	if err != nil {
		return "", fmt.Errorf("failed to generate context: %w", err)
	}

	return contextMd, nil
}

// WriteJournalEntry appends a tool use/result entry to the agent's journal.
// Called by orchestrator after each agent tool invocation.
// Creates journal directory structure on first write.
func WriteJournalEntry(projectRoot string, waveNum int, agentID string, entry journal.ToolEntry) error {
	// Construct journal directory: .saw-state/wave{N}/agent-{ID}/
	journalDir := filepath.Join(projectRoot, ".saw-state", fmt.Sprintf("wave%d", waveNum), agentID)

	// Create directory structure if needed
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return fmt.Errorf("failed to create journal directory: %w", err)
	}

	// Append to index.jsonl
	indexPath := filepath.Join(journalDir, "index.jsonl")
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open index.jsonl: %w", err)
	}
	defer f.Close()

	// Write entry as JSON line
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode journal entry: %w", err)
	}

	return nil
}

// loadJournalEntries reads all ToolEntry records from index.jsonl file.
// Returns empty slice if file is empty or contains no valid entries.
func loadJournalEntries(indexPath string) ([]journal.ToolEntry, error) {
	f, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index.jsonl: %w", err)
	}
	defer f.Close()

	var entries []journal.ToolEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line size

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		var entry journal.ToolEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Skip malformed lines with warning (don't fail entire load)
			fmt.Fprintf(os.Stderr, "Warning: skipping malformed journal entry at line %d: %v\n", lineNum, err)
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read index.jsonl: %w", err)
	}

	return entries, nil
}
