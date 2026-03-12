package commands

import "errors"

// LanguageDefaults is a fallback for when no CI/build system configs are found.
// This is a stub - actual implementation is owned by Agent E.
// TODO(Agent E): Implement language detection and default commands.
var LanguageDefaults = func(repoRoot string) (*CommandSet, error) {
	return nil, errors.New("LanguageDefaults not yet implemented (Agent E)")
}
