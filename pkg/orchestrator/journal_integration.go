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
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// loggerFrom returns the provided logger, or slog.Default() if nil.
func loggerFrom(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

// PrepareAgentContext loads journal history and generates context.md for agent recovery.
// Returns success with empty ContextMD if no journal exists (first launch).
// Returns failure if journal exists but cannot be read (differentiate from "no journal").
func PrepareAgentContext(opts PrepareAgentContextOpts) result.Result[PrepareContextData] {
	projectRoot := opts.ProjectRoot
	waveNum := opts.WaveNum
	agentID := opts.AgentID
	maxEntries := opts.MaxEntries
	logger := opts.Logger

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
		return result.NewSuccess(PrepareContextData{ContextMD: ""})
	} else if err != nil {
		// Journal exists but cannot be accessed - return error
		return result.NewFailure[PrepareContextData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to access journal at %s: %s", journalPath, err.Error())),
		})
	}

	// Read all entries from index.jsonl
	entries, loadErr := loadJournalEntries(journalPath, log)
	if loadErr != nil {
		return result.NewFailure[PrepareContextData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to load journal entries: %s", loadErr.Error())),
		})
	}

	// If journal is empty, return empty string (equivalent to no journal)
	if len(entries) == 0 {
		return result.NewSuccess(PrepareContextData{ContextMD: ""})
	}

	// Generate context markdown using journal.GenerateContext
	contextMd, genErr := journal.GenerateContext(entries, maxEntries)
	if genErr != nil {
		return result.NewFailure[PrepareContextData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to generate context: %s", genErr.Error())),
		})
	}

	return result.NewSuccess(PrepareContextData{ContextMD: contextMd})
}

// WriteJournalEntry appends a tool use/result entry to the agent's journal.
// Called by orchestrator after each agent tool invocation.
// Creates journal directory structure on first write.
func WriteJournalEntry(projectRoot string, waveNum int, agentID string, entry journal.ToolEntry) result.Result[WriteJournalData] {
	// Construct journal directory: .saw-state/wave{N}/agent-{ID}/
	journalDir := protocol.SAWStateAgentDir(projectRoot, waveNum, agentID)

	// Create directory structure if needed
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return result.NewFailure[WriteJournalData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to create journal directory: %s", err.Error())),
		})
	}

	// Append to index.jsonl
	indexPath := filepath.Join(journalDir, "index.jsonl")
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return result.NewFailure[WriteJournalData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to open index.jsonl: %s", err.Error())),
		})
	}
	defer f.Close()

	// Write entry as JSON line
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(entry); err != nil {
		return result.NewFailure[WriteJournalData]([]result.SAWError{
			result.NewFatal(result.CodeJournalInitFail,
				fmt.Sprintf("failed to encode journal entry: %s", err.Error())),
		})
	}

	return result.NewSuccess(WriteJournalData{JournalPath: indexPath})
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
