package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// PackageLockParser parses npm package-lock.json files (v7+)
type PackageLockParser struct{}

// packageLockFile represents the structure of package-lock.json (npm v7+)
type packageLockFile struct {
	LockfileVersion int                           `json:"lockfileVersion"`
	Packages        map[string]packageLockPackage `json:"packages"`
}

// packageLockPackage represents a package entry in package-lock.json
type packageLockPackage struct {
	Version  string `json:"version"`
	Resolved string `json:"resolved"`
}

// Parse reads package-lock.json and returns package metadata
func (p *PackageLockParser) Parse(filePath string) result.Result[[]PackageInfo] {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return result.NewFailure[[]PackageInfo]([]result.PolywaveError{result.NewFatal(result.CodeDepLockFileOpen, "failed to read package-lock.json: "+err.Error())})
	}

	// Parse JSON
	var lockFile packageLockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return result.NewFailure[[]PackageInfo]([]result.PolywaveError{result.NewFatal(result.CodeDepLockFileParse, "failed to parse package-lock.json: "+err.Error())})
	}

	// Check lockfile version (support v7+ which is lockfileVersion >= 2)
	if lockFile.LockfileVersion < 2 {
		return result.NewFailure[[]PackageInfo]([]result.PolywaveError{result.NewError(result.CodeDepUnsupportedVersion, fmt.Sprintf("unsupported lockfile version %d (requires >= 2)", lockFile.LockfileVersion))})
	}

	// Extract packages
	var packages []PackageInfo
	for path, pkg := range lockFile.Packages {
		// Skip root package (empty string key)
		if path == "" {
			continue
		}

		// Strip "node_modules/" prefix if present
		name := path
		if strings.HasPrefix(name, "node_modules/") {
			name = strings.TrimPrefix(name, "node_modules/")
		}

		// Only include packages that have a version
		// (some entries may be workspaces without versions)
		if pkg.Version == "" {
			continue
		}

		packages = append(packages, PackageInfo{
			Name:    name,
			Version: pkg.Version,
			Source:  pkg.Resolved,
		})
	}

	return result.NewSuccess(packages)
}

// Detect checks if this parser can handle the given file
func (p *PackageLockParser) Detect(filePath string) bool {
	return strings.HasSuffix(filePath, "package-lock.json")
}

func init() {
	RegisterParser(&PackageLockParser{})
}
