package orchestrator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// loggerFrom returns the provided logger, or slog.Default() if nil.
func loggerFrom(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

// PrepareAgentContext loads journal history and generates context.md for agent recovery.
// Returns empty string if no journal exists (first launch).
// Returns error if journal exists but cannot be read (differentiate from "no journal").
func PrepareAgentContext(projectRoot string, waveNum int, agentID string, maxEntries int, logger *slog.Logger) (string, error) {
	log := loggerFrom(logger)
	// Default maxEntries to 50 per E23A spec
	if maxEntries == 0 {
		maxEntries = 50
	}

	// Construct journal path: .saw-state/wave{N}/agent-{ID}/index.jsonl
	journalPath := filepath.Join(protocol.SAWStateAgentDir(projectRoot, waveNum, agentID), "index.jsonl")

	// Check if journal exists
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		// No journal yet - this is first launch, return empty string
		return "", nil
	} else if err != nil {
		// Journal exists but cannot be accessed - return error
		return "", fmt.Errorf("failed to access journal at %s: %w", journalPath, err)
	}

	// Read all entries from index.jsonl
	entries, err := loadJournalEntries(journalPath, log)
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
	journalDir := protocol.SAWStateAgentDir(projectRoot, waveNum, agentID)

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
func loadJournalEntries(indexPath string, logger *slog.Logger) ([]journal.ToolEntry, error) {
	log := loggerFrom(logger)
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
			log.Warn("skipping malformed journal entry", "line", lineNum, "err", err)
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read index.jsonl: %w", err)
	}

	return entries, nil
}
