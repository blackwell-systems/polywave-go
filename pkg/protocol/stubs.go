package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// StubHit represents a single stub pattern match in source code.
type StubHit struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
	Context string `json:"context"`
}

// ScanStubsData contains all stub hits found during scanning.
type ScanStubsData struct {
	Hits []StubHit `json:"hits"`
}

// ScanStubs scans the provided files for common stub patterns.
// It looks for markers like TODO, FIXME, HACK, XXX, panic("not implemented"),
// and other placeholder indicators.
//
// Files that cannot be read are silently skipped (E20: informational only).
// Returns an empty Hits slice if no stubs are found or files list is empty.
func ScanStubs(files []string) result.Result[ScanStubsData] {
	data := ScanStubsData{
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
					data.Hits = append(data.Hits, hit)
					// Only record one hit per line (first pattern match wins)
					break
				}
			}
		}

		f.Close()
	}

	return result.NewSuccess(data)
}

// AppendStubData contains metadata about a completed stub report append operation.
type AppendStubData struct {
	ManifestPath string
	WaveKey      string
	Appended     bool
}

// AppendStubReport loads the manifest at manifestPath, stores result under
// waveKey (e.g. "wave1"), and saves the manifest back to disk.
func AppendStubReport(manifestPath, waveKey string, scanResult result.Result[ScanStubsData]) result.Result[AppendStubData] {
	manifest, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[AppendStubData]([]result.SAWError{
			result.NewFatal("STUB_APPEND_FAILED", fmt.Sprintf("AppendStubReport: failed to load manifest: %v", err)),
		})
	}
	if manifest.StubReports == nil {
		manifest.StubReports = make(map[string]*ScanStubsData)
	}
	scanData := scanResult.GetData()
	manifest.StubReports[waveKey] = &scanData
	if err := Save(manifest, manifestPath); err != nil {
		return result.NewFailure[AppendStubData]([]result.SAWError{
			result.NewFatal("STUB_APPEND_FAILED", fmt.Sprintf("AppendStubReport: failed to save manifest: %v", err)),
		})
	}
	return result.NewSuccess(AppendStubData{
		ManifestPath: manifestPath,
		WaveKey:      waveKey,
		Appended:     true,
	})
}
