# Agent A Brief - Wave 1

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-h7-build-failure-diagnosis.yaml

## Files Owned

- `pkg/builddiag/types.go`
- `pkg/builddiag/types_test.go`
- `pkg/builddiag/diagnose.go`
- `pkg/builddiag/diagnose_test.go`


## Task

## What to Implement

Create the core types and diagnosis engine for build failure pattern matching. This includes the `ErrorPattern` struct, `Diagnosis` result type, and the main `DiagnoseError` function that routes to language-specific pattern catalogs.

## Interfaces to Implement

```go
// pkg/builddiag/types.go
package builddiag

// ErrorPattern represents a recognized build error with fix recommendation
type ErrorPattern struct {
  Name        string  // e.g., "missing_import", "type_mismatch"
  Regex       string  // Regex pattern to match error message
  Fix         string  // Command or action to fix
  Rationale   string  // Why this fix works
  AutoFixable bool    // Whether fix can be automated
  Confidence  float64 // Match confidence (0.0-1.0)
}

// Diagnosis is the result of pattern matching
type Diagnosis struct {
  Pattern     string  `yaml:"diagnosis"`
  Confidence  float64 `yaml:"confidence"`
  Fix         string  `yaml:"fix"`
  Rationale   string  `yaml:"rationale"`
  AutoFixable bool    `yaml:"auto_fixable"`
}
```

```go
// pkg/builddiag/diagnose.go
package builddiag

import (
  "fmt"
  "regexp"
  "strings"
)

// Language-specific pattern catalogs (registered by language files)
var catalogs = make(map[string][]ErrorPattern)

// RegisterPatterns adds patterns for a language to the catalog
func RegisterPatterns(language string, patterns []ErrorPattern) {
  catalogs[language] = patterns
}

// DiagnoseError matches error log against language patterns
func DiagnoseError(errorLog string, language string) (*Diagnosis, error) {
  patterns, ok := catalogs[strings.ToLower(language)]
  if !ok {
    return nil, fmt.Errorf("unsupported language: %s", language)
  }

  // Try each pattern in order (highest confidence first)
  for _, pattern := range patterns {
    matched, err := regexp.MatchString(pattern.Regex, errorLog)
    if err != nil {
      continue // Skip invalid regex
    }

    if matched {
      return &Diagnosis{
        Pattern:     pattern.Name,
        Confidence:  pattern.Confidence,
        Fix:         pattern.Fix,
        Rationale:   pattern.Rationale,
        AutoFixable: pattern.AutoFixable,
      }, nil
    }
  }

  // No pattern matched
  return &Diagnosis{
    Pattern:     "unknown",
    Confidence:  0.0,
    Fix:         "Manual investigation required",
    Rationale:   "No known pattern matched this error",
    AutoFixable: false,
  }, nil
}

// SupportedLanguages returns list of languages with pattern catalogs
func SupportedLanguages() []string {
  langs := make([]string, 0, len(catalogs))
  for lang := range catalogs {
    langs = append(langs, lang)
  }
  return langs
}
```

## Interfaces to Call

None - this is the foundation layer.

## Tests to Write

1. `TestRegisterPatterns` - pattern registration works
2. `TestDiagnoseError_KnownPattern` - matches known pattern
3. `TestDiagnoseError_NoMatch` - returns "unknown" for unmatched error
4. `TestDiagnoseError_UnsupportedLanguage` - returns error for invalid language
5. `TestDiagnoseError_InvalidRegex` - skips patterns with invalid regex
6. `TestSupportedLanguages` - returns registered languages

## Verification Gate

```bash
go build ./pkg/builddiag/...
go vet ./pkg/builddiag/...
go test ./pkg/builddiag/... -v
```

## Constraints

- Use global `catalogs` map (simple, init-time registration)
- Try patterns in order (assume higher-confidence patterns first)
- Return "unknown" diagnosis (not error) when no pattern matches
- Skip patterns with invalid regex (log but don't fail)
- Language names are case-insensitive



## Interface Contracts

### ErrorPattern

Represents a recognized build error pattern with its fix recommendation.
Each language catalog defines a list of these patterns.


```
type ErrorPattern struct {
  Name        string  // Human-readable name (e.g., "missing_import")
  Regex       string  // Regex pattern to match error message
  Fix         string  // Command or action to fix (e.g., "go mod tidy")
  Rationale   string  // Why this fix works
  AutoFixable bool    // Whether the fix can be automated
  Confidence  float64 // Match confidence (0.0-1.0)
}

```

### DiagnoseError

Main entrypoint for build failure diagnosis.
Matches error log against language-specific pattern catalogs.


```
func DiagnoseError(errorLog string, language string) (*Diagnosis, error)

type Diagnosis struct {
  Pattern     string  `yaml:"diagnosis"`
  Confidence  float64 `yaml:"confidence"`
  Fix         string  `yaml:"fix"`
  Rationale   string  `yaml:"rationale"`
  AutoFixable bool    `yaml:"auto_fixable"`
}

```



## Quality Gates

Level: standard

- **build**: `go build ./pkg/builddiag/... ./cmd/saw/...` (required: true)
- **lint**: `go vet ./pkg/builddiag/... ./cmd/saw/...` (required: false)
  Check for common Go mistakes
- **test**: `go test ./pkg/builddiag/... ./cmd/saw/...` (required: true)

