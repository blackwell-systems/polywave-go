package deps

import (
	"bufio"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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
func (p *CargoLockParser) Parse(filePath string) result.Result[[]PackageInfo] {
	file, err := os.Open(filePath)
	if err != nil {
		return result.NewFailure[[]PackageInfo]([]result.SAWError{
			result.NewFatal(result.CodeDepLockFileOpen, "failed to open Cargo.lock: "+err.Error()),
		})
	}
	defer file.Close()

	var packages []PackageInfo
	scanner := bufio.NewScanner(file)

	var currentPackage *PackageInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Start of a new package section
		if line == "[[package]]" {
			// Save previous package if exists and has a valid name
			if currentPackage != nil && currentPackage.Name != "" {
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
		return result.NewFailure[[]PackageInfo]([]result.SAWError{
			result.NewError(result.CodeDepLockFileParse, "error reading Cargo.lock: "+err.Error()),
		})
	}

	return result.NewSuccess(packages)
}

// Detect checks if this parser can handle the given file
func (p *CargoLockParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "Cargo.lock")
}

func init() {
	RegisterParser(&CargoLockParser{})
}
