package deps

import (
	"bufio"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// PoetryLockParser parses Python poetry.lock files (TOML format)
type PoetryLockParser struct{}

// Parse reads a poetry.lock file and extracts package metadata
func (p *PoetryLockParser) Parse(filePath string) result.Result[[]PackageInfo] {
	file, err := os.Open(filePath)
	if err != nil {
		return result.NewFailure[[]PackageInfo]([]result.SAWError{result.NewFatal(result.CodeDepLockFileOpen, "failed to open poetry.lock: "+err.Error())})
	}
	defer file.Close()

	var packages []PackageInfo
	var currentPackage *PackageInfo
	inPackageSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Start of a new [[package]] section
		if line == "[[package]]" {
			// Save previous package if exists
			if currentPackage != nil && currentPackage.Name != "" {
				packages = append(packages, *currentPackage)
			}
			// Start new package
			currentPackage = &PackageInfo{}
			inPackageSection = true
			continue
		}

		// End of package section (empty line or start of new section)
		if line == "" || strings.HasPrefix(line, "[[") || (strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[[") && inPackageSection) {
			if currentPackage != nil && currentPackage.Name != "" {
				packages = append(packages, *currentPackage)
				currentPackage = nil
			}
			inPackageSection = false
			continue
		}

		// Parse package fields
		if inPackageSection && currentPackage != nil {
			// Parse name field: name = "requests"
			if strings.HasPrefix(line, "name = ") {
				value := strings.TrimPrefix(line, "name = ")
				currentPackage.Name = unquoteTOML(value)
			}

			// Parse version field: version = "2.28.1"
			if strings.HasPrefix(line, "version = ") {
				value := strings.TrimPrefix(line, "version = ")
				currentPackage.Version = unquoteTOML(value)
			}

			// Parse source field (if present): source = "https://..."
			if strings.HasPrefix(line, "source = ") {
				value := strings.TrimPrefix(line, "source = ")
				currentPackage.Source = unquoteTOML(value)
			}
		}
	}

	// Don't forget the last package
	if currentPackage != nil && currentPackage.Name != "" {
		packages = append(packages, *currentPackage)
	}

	if err := scanner.Err(); err != nil {
		return result.NewFailure[[]PackageInfo]([]result.SAWError{result.NewError(result.CodeDepLockFileParse, "error reading poetry.lock: "+err.Error())})
	}

	// An empty poetry.lock is valid (project with no declared dependencies).
	// Return success with an empty slice, consistent with CargoLockParser.
	return result.NewSuccess(packages)
}

// Detect checks if this parser can handle the given file
func (p *PoetryLockParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "poetry.lock")
}

func init() {
	RegisterParser(&PoetryLockParser{})
}
