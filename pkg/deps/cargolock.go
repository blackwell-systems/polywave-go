package deps

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// CargoLockParser implements LockFileParser for Rust Cargo.lock files
type CargoLockParser struct{}

// Parse reads a Cargo.lock file and extracts package information
// Cargo.lock format (TOML):
//
//	[[package]]
//	name = "serde"
//	version = "1.0.136"
//	source = "registry+https://github.com/rust-lang/crates.io-index"
func (p *CargoLockParser) Parse(filePath string) ([]PackageInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Cargo.lock: %w", err)
	}
	defer file.Close()

	var packages []PackageInfo
	scanner := bufio.NewScanner(file)

	var currentPackage *PackageInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Start of a new package section
		if line == "[[package]]" {
			// Save previous package if exists
			if currentPackage != nil {
				packages = append(packages, *currentPackage)
			}
			// Start new package
			currentPackage = &PackageInfo{}
			continue
		}

		// Parse package fields
		if currentPackage != nil {
			if strings.HasPrefix(line, "name = ") {
				currentPackage.Name = unquoteTOML(strings.TrimPrefix(line, "name = "))
			} else if strings.HasPrefix(line, "version = ") {
				currentPackage.Version = unquoteTOML(strings.TrimPrefix(line, "version = "))
			} else if strings.HasPrefix(line, "source = ") {
				currentPackage.Source = unquoteTOML(strings.TrimPrefix(line, "source = "))
			}
		}
	}

	// Don't forget the last package
	if currentPackage != nil && currentPackage.Name != "" {
		packages = append(packages, *currentPackage)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Cargo.lock: %w", err)
	}

	return packages, nil
}

// Detect checks if this parser can handle the given file
func (p *CargoLockParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "Cargo.lock")
}

// unquoteTOML removes quotes from TOML string values
// Input: "value" or 'value'
// Output: value
func unquoteTOML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func init() {
	RegisterParser(&CargoLockParser{})
}
