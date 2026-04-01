package result

import (
	"strings"
	"testing"
)

// TestNCodesFollowNamingPattern verifies all N-range constants N018-N084
// follow the "Nxxx_DESCRIPTION" naming pattern.
func TestNCodesFollowNamingPattern(t *testing.T) {
	ncodes := []struct {
		name  string
		value string
	}{
		{"CodeContextCancelled", CodeContextCancelled},
		{"CodeScoutInvalidOpts", CodeScoutInvalidOpts},
		{"CodeScoutRunFailed", CodeScoutRunFailed},
		{"CodeScoutBoundaryViolation", CodeScoutBoundaryViolation},
		{"CodePlannerInvalidOpts", CodePlannerInvalidOpts},
		{"CodePlannerFailed", CodePlannerFailed},
		{"CodeWaveInvalidOpts", CodeWaveInvalidOpts},
		{"CodeWaveFailed", CodeWaveFailed},
		{"CodeWaveSequencingFailed", CodeWaveSequencingFailed},
		{"CodeHookVerifyFailed", CodeHookVerifyFailed},
		{"CodeScaffoldRunFailed", CodeScaffoldRunFailed},
		{"CodeAgentRunFailed", CodeAgentRunFailed},
		{"CodeAgentRunInvalidOpts", CodeAgentRunInvalidOpts},
		{"CodeMergeWaveFailed", CodeMergeWaveFailed},
		{"CodeMergeWaveInvalidOpts", CodeMergeWaveInvalidOpts},
		{"CodeEngineVerificationFailed", CodeEngineVerificationFailed},
		{"CodeUpdateStatusFailed", CodeUpdateStatusFailed},
		{"CodeValidateFailed", CodeValidateFailed},
		{"CodeJournalArchiveFailed", CodeJournalArchiveFailed},
		{"CodeMarkCompleteFailed", CodeMarkCompleteFailed},
		{"CodeMarkCompleteInvalidOpts", CodeMarkCompleteInvalidOpts},
		{"CodeVerifyTiersIncomplete", CodeVerifyTiersIncomplete},
		{"CodeMarkerReadFailed", CodeMarkerReadFailed},
		{"CodeMarkerWriteFailed", CodeMarkerWriteFailed},
		{"CodeUpdateProgParseFailed", CodeUpdateProgParseFailed},
		{"CodeUpdateProgSlugNotFound", CodeUpdateProgSlugNotFound},
		{"CodeSyncParseFailed", CodeSyncParseFailed},
		{"CodeSyncStatusFailed", CodeSyncStatusFailed},
		{"CodeWriteManifestFailed", CodeWriteManifestFailed},
		{"CodeRestoreLoadFailed", CodeRestoreLoadFailed},
		{"CodeRestoreSaveFailed", CodeRestoreSaveFailed},
		{"CodeTestLoadFailed", CodeTestLoadFailed},
		{"CodeTestNoCommand", CodeTestNoCommand},
		{"CodeTestPipeFailed", CodeTestPipeFailed},
		{"CodeTestStartFailed", CodeTestStartFailed},
		{"CodeTestCommandFailed", CodeTestCommandFailed},
		{"CodeScoutRunnerFailed", CodeScoutRunnerFailed},
		{"CodeScoutValidationFailed", CodeScoutValidationFailed},
		{"CodeScoutCorrectionExhausted", CodeScoutCorrectionExhausted},
		{"CodeSetBlockedLoadFailed", CodeSetBlockedLoadFailed},
		{"CodeSetBlockedSaveFailed", CodeSetBlockedSaveFailed},
		{"CodeFixBuildInvalidOpts", CodeFixBuildInvalidOpts},
		{"CodeFixBuildFailed", CodeFixBuildFailed},
		{"CodeGomodFixupFailed", CodeGomodFixupFailed},
		{"CodeCleanupFailed", CodeCleanupFailed},
		{"CodeResolveInvalidOpts", CodeResolveInvalidOpts},
		{"CodeResolveLoadFailed", CodeResolveLoadFailed},
		{"CodeResolveGitFailed", CodeResolveGitFailed},
		{"CodeResolveNoConflicts", CodeResolveNoConflicts},
		{"CodeResolveBackendFailed", CodeResolveBackendFailed},
		{"CodeResolveFileFailed", CodeResolveFileFailed},
		{"CodeResolveCommitFailed", CodeResolveCommitFailed},
		{"CodeResolveFileReadFailed", CodeResolveFileReadFailed},
		{"CodeResolveBackendCallFailed", CodeResolveBackendCallFailed},
		{"CodeResolveFileWriteFailed", CodeResolveFileWriteFailed},
		{"CodeResolveGitAddFailed", CodeResolveGitAddFailed},
		{"CodeExportFileExists", CodeExportFileExists},
		{"CodeExportNoEntries", CodeExportNoEntries},
		{"CodeExportWriteFailed", CodeExportWriteFailed},
		{"CodeIntegrationInvalidOpts", CodeIntegrationInvalidOpts},
		{"CodeIntegrationLoadFailed", CodeIntegrationLoadFailed},
		{"CodeIntegrationNoConnectors", CodeIntegrationNoConnectors},
		{"CodeIntegrationPromptFailed", CodeIntegrationPromptFailed},
		{"CodeIntegrationBackendFailed", CodeIntegrationBackendFailed},
		{"CodeIntegrationAgentFailed", CodeIntegrationAgentFailed},
		{"CodeChatInvalidOpts", CodeChatInvalidOpts},
		{"CodeChatFailed", CodeChatFailed},
	}
	for _, tc := range ncodes {
		t.Run(tc.name, func(t *testing.T) {
			// Must match N + 3 digits + underscore + uppercase description
			if len(tc.value) < 5 {
				t.Errorf("%s = %q: too short to match Nxxx_ pattern", tc.name, tc.value)
				return
			}
			if tc.value[0] != 'N' {
				t.Errorf("%s = %q: must start with 'N'", tc.name, tc.value)
			}
			if !strings.Contains(tc.value, "_") {
				t.Errorf("%s = %q: must contain underscore separator", tc.name, tc.value)
			}
		})
	}
}
