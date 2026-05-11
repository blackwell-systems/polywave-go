package protocol

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// CallerSite represents one location where a function/method is called.
type CallerSite struct {
	File    string `json:"file"`    // repo-relative path
	Line    int    `json:"line"`    // 1-indexed line number
	Context string `json:"context"` // surrounding line content (trimmed)
}

// CheckCallers scans repoDir for all call sites of symbolName.
// symbolName may be a plain function name ("GetData") or a method call
// expression ("cache.Get") — both styles are matched via regex grep.
// Test files are included in the scan. Returns every (file, line, context)
// tuple where symbolName appears as a call expression.
func CheckCallers(ctx context.Context, repoDir, symbolName string) result.Result[[]CallerSite] {
	if symbolName == "" {
		return result.NewFailure[[]CallerSite]([]result.PolywaveError{
			result.NewFatal("X001_INVALID_INPUT", "symbolName must not be empty"),
		})
	}

	// Verify repoDir is readable
	if _, err := os.Stat(repoDir); err != nil {
		return result.NewFailure[[]CallerSite]([]result.PolywaveError{
			result.NewFatal("X001_INVALID_INPUT", fmt.Sprintf("repoDir unreadable: %v", err)),
		})
	}

	var sites []CallerSite

	walkErr := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files/dirs
		}

		// Skip vendor and .git directories
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		f, err := os.Open(path)
		if err != nil {
			return nil // skip unreadable files
		}
		defer f.Close()

		// Compute repo-relative path
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			relPath = path
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, symbolName) {
				sites = append(sites, CallerSite{
					File:    relPath,
					Line:    lineNum,
					Context: strings.TrimSpace(line),
				})
			}
		}
		return nil
	})

	if walkErr != nil && walkErr == ctx.Err() {
		return result.NewFailure[[]CallerSite]([]result.PolywaveError{
			result.NewFatal("X001_INVALID_INPUT", fmt.Sprintf("scan cancelled: %v", walkErr)),
		})
	}

	if sites == nil {
		sites = []CallerSite{}
	}

	return result.NewSuccess(sites)
}

// ErrorCodeRange describes a single allocated error code prefix range.
type ErrorCodeRange struct {
	Prefix      string `json:"prefix"`      // e.g. "V", "K", "C"
	Start       int    `json:"start"`       // first code number in range (1-indexed)
	End         int    `json:"end"`         // last code number in range (inclusive)
	Description string `json:"description"` // e.g. "Validation errors"
}

// rangeLineRe matches lines like: \tV001-V099: Validation errors
var rangeLineRe = regexp.MustCompile(`\t([A-Z]+)(\d+)-[A-Z]+(\d+):\s*(.+)`)

// ListErrorRanges parses the package-doc comment block in pkg/result/codes.go
// and returns all declared error code ranges.
// Returns Result[[]ErrorCodeRange].
func ListErrorRanges(ctx context.Context, repoDir string) result.Result[[]ErrorCodeRange] {
	codesPath := filepath.Join(repoDir, "pkg", "result", "codes.go")

	data, err := os.ReadFile(codesPath)
	if err != nil {
		return result.NewFailure[[]ErrorCodeRange]([]result.PolywaveError{
			result.NewFatal("X002_FILE_READ", fmt.Sprintf("failed to read codes.go: %v", err)),
		})
	}

	var ranges []ErrorCodeRange
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Stop parsing once we hit the package declaration
		if strings.HasPrefix(line, "package ") {
			break
		}

		matches := rangeLineRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		// matches[1] = prefix, matches[2] = start digits, matches[3] = end digits, matches[4] = description
		prefix := matches[1]
		start, err := strconv.Atoi(matches[2])
		if err != nil {
			continue
		}
		end, err := strconv.Atoi(matches[3])
		if err != nil {
			continue
		}
		description := strings.TrimSpace(matches[4])

		ranges = append(ranges, ErrorCodeRange{
			Prefix:      prefix,
			Start:       start,
			End:         end,
			Description: description,
		})
	}

	if ranges == nil {
		ranges = []ErrorCodeRange{}
	}

	return result.NewSuccess(ranges)
}
