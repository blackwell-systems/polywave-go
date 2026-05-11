package deps

import (
	"bufio"
	"os"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// GoSumParser implements LockFileParser for go.sum files
type GoSumParser struct{}

// Parse reads a go.sum file and returns package metadata
func (p *GoSumParser) Parse(filePath string) result.Result[[]PackageInfo] {
	file, err := os.Open(filePath)
	if err != nil {
		return result.NewFailure[[]PackageInfo]([]result.PolywaveError{result.NewFatal(result.CodeDepLockFileOpen, "failed to open go.sum: "+err.Error())})
	}
	defer file.Close()

	// Track unique packages (go.sum has duplicate entries with /go.mod suffix)
	uniquePackages := make(map[string]PackageInfo)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse line: "github.com/foo/bar v1.2.3 h1:hash"
		// or: "github.com/foo/bar v1.2.3/go.mod h1:hash"
		fields := strings.Fields(line)

		// Need at least 3 fields: module, version, hash
		if len(fields) < 3 {
			continue
		}

		moduleName := fields[0]
		version := fields[1]
		hash := fields[2]

		// Validate hash format (should start with h1: or h2:)
		if !strings.HasPrefix(hash, "h1:") && !strings.HasPrefix(hash, "h2:") {
			continue
		}

		// Skip /go.mod lines (duplicates)
		if strings.HasSuffix(version, "/go.mod") {
			continue
		}

		// Use module name as key to deduplicate
		uniquePackages[moduleName] = PackageInfo{
			Name:    moduleName,
			Version: version,
			Source:  moduleName, // go.sum doesn't include registry URL, use module path
		}
	}

	if err := scanner.Err(); err != nil {
		return result.NewFailure[[]PackageInfo]([]result.PolywaveError{result.NewError(result.CodeDepLockFileParse, "error reading go.sum: "+err.Error())})
	}

	// Convert map to slice
	packages := make([]PackageInfo, 0, len(uniquePackages))
	for _, pkg := range uniquePackages {
		packages = append(packages, pkg)
	}

	return result.NewSuccess(packages)
}

// Detect checks if this parser can handle the given file
func (p *GoSumParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "go.sum")
}

func init() {
	RegisterParser(&GoSumParser{})
}
