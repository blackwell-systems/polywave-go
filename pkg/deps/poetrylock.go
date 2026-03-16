package deps

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PoetryLockParser parses Python poetry.lock files (TOML format)
type PoetryLockParser struct{}

// Parse reads a poetry.lock file and extracts package metadata
func (p *PoetryLockParser) Parse(filePath string) ([]PackageInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open poetry.lock: %w", err)
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
		if line == "" || (strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[[")) {
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
				currentPackage.Name = strings.Trim(value, `"`)
			}

			// Parse version field: version = "2.28.1"
			if strings.HasPrefix(line, "version = ") {
				value := strings.TrimPrefix(line, "version = ")
				currentPackage.Version = strings.Trim(value, `"`)
			}

			// Parse source field (if present): source = "https://..."
			if strings.HasPrefix(line, "source = ") {
				value := strings.TrimPrefix(line, "source = ")
				currentPackage.Source = strings.Trim(value, `"`)
			}
		}
	}

	// Don't forget the last package
	if currentPackage != nil && currentPackage.Name != "" {
		packages = append(packages, *currentPackage)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading poetry.lock: %w", err)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages found in poetry.lock (possible malformed TOML)")
	}

	return packages, nil
}

// Detect checks if this parser can handle the given file
func (p *PoetryLockParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "poetry.lock")
}

func init() {
	RegisterParser(&PoetryLockParser{})
}
