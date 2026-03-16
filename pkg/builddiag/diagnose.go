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
	catalogs[strings.ToLower(language)] = patterns
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
