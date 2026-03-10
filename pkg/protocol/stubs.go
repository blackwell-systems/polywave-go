package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StubHit represents a single stub pattern match in source code.
type StubHit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
	Context string `json:"context"`
}

// ScanStubsResult contains all stub hits found during scanning.
type ScanStubsResult struct {
	Hits []StubHit `json:"hits"`
}

// ScanStubs scans the provided files for common stub patterns.
// It looks for markers like TODO, FIXME, HACK, XXX, panic("not implemented"),
// and other placeholder indicators.
//
// Files that cannot be read are silently skipped (E20: informational only).
// Returns an empty Hits slice if no stubs are found or files list is empty.
func ScanStubs(files []string) (*ScanStubsResult, error) {
	result := &ScanStubsResult{
		Hits: []StubHit{},
	}

	// Stub patterns to search for (case-insensitive where applicable)
	patterns := []string{
		"TODO",
		"FIXME",
		"HACK",
		"XXX",
		`panic("not implemented")`,
		`panic("TODO")`,
		`panic("todo")`,
		"// stub",
		"// placeholder",
		"unimplemented!()",
		"todo!()",
	}

	for _, file := range files {
		// Open the file
		f, err := os.Open(file)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			lineLower := strings.ToLower(line)

			// Check each pattern
			for _, pattern := range patterns {
				patternLower := strings.ToLower(pattern)
				if strings.Contains(lineLower, patternLower) {
					hit := StubHit{
						File:    file,
						Line:    lineNum,
						Pattern: pattern,
						Context: strings.TrimSpace(line),
					}
					result.Hits = append(result.Hits, hit)
					// Only record one hit per line (first pattern match wins)
					break
				}
			}
		}

		f.Close()
	}

	return result, nil
}

// AppendStubReport loads the manifest at manifestPath, stores result under
// waveKey (e.g. "wave1"), and saves the manifest back to disk.
func AppendStubReport(manifestPath, waveKey string, result *ScanStubsResult) error {
	manifest, err := Load(manifestPath)
	if err != nil {
		return fmt.Errorf("AppendStubReport: failed to load manifest: %w", err)
	}
	if manifest.StubReports == nil {
		manifest.StubReports = make(map[string]*ScanStubsResult)
	}
	manifest.StubReports[waveKey] = result
	if err := Save(manifest, manifestPath); err != nil {
		return fmt.Errorf("AppendStubReport: failed to save manifest: %w", err)
	}
	return nil
}
