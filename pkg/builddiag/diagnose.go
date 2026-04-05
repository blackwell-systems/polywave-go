package builddiag

import (
	"regexp"
	"sort"
	"strings"
)

// Language-specific pattern catalogs (registered by language files)
var catalogs = make(map[string][]ErrorPattern)

// RegisterPatterns replaces the pattern catalog for a language
func RegisterPatterns(language string, patterns []ErrorPattern) {
	catalogs[strings.ToLower(language)] = patterns
}

// DiagnoseError matches error log against language patterns.
// Returns nil for unsupported languages. For supported languages with no
// matching pattern, returns a Diagnosis with Pattern: "unknown" and Confidence: 0.0.
func DiagnoseError(errorLog string, language string) *Diagnosis {
	patterns, ok := catalogs[strings.ToLower(language)]
	if !ok {
		return nil
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
			}
		}
	}

	// No pattern matched
	return &Diagnosis{
		Pattern:     "unknown",
		Confidence:  0.0,
		Fix:         "Manual investigation required",
		Rationale:   "No known pattern matched this error",
		AutoFixable: false,
	}
}

// SupportedLanguages returns a sorted slice of registered language names.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(catalogs))
	for lang := range catalogs {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}
