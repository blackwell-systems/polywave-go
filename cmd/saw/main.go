package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(
		newCreateWorktreesCmd(),
		newVerifyCommitsCmd(),
		newScanStubsCmd(),
		newMergeAgentsCmd(),
		newCleanupCmd(),
		newVerifyBuildCmd(),
		newUpdateStatusCmd(),
		newUpdateContextCmd(),
		newListIMPLsCmd(),
		newRunWaveCmd(),
		newVerifyIsolationCmd(),
		newValidateCmd(),
		newExtractContextCmd(),
		newSetCompletionCmd(),
		newMarkCompleteCmd(),
		newSetImplStateCmd(),
		newRunGatesCmd(),
		newRunReviewCmd(),
		newCheckConflictsCmd(),
		newCheckDepsCmd(),
		newCheckTypeCollisionsCmd(),
		newValidateScaffoldsCmd(),
		newValidateScaffoldCmd(),
		newFreezeCheckCmd(),
		newUpdateAgentPromptCmd(),
		newSolveCmd(),
		newDebugJournalCmd(),
		newDetectCascadesCmd(),
		newJournalInitCmd(),
		newPrepareAgentCmd(),
		newPrepareWaveCmd(),
		newJournalContextCmd(),
		newAmendImplCmd(),
		newAnalyzeDepsCmd(),
		newDetectScaffoldsCmd(),
		newAnalyzeSuitabilityCmd(),
		newExtractCommandsCmd(),
		newFinalizeWaveCmd(),
		newFinalizeImplCmd(),
		newDiagnoseBuildFailureCmd(),
		newAssignAgentIDsCmd(),
		newInterviewCmd(),
		newRunScoutCmd(),       // I3: Phase 5 integration
		newRunCriticCmd(),      // E37: Pre-wave brief review
		newSetCriticReviewCmd(), // E37: Used by critic agents to write results
		newVerifyHookInstalledCmd(),
		newValidateIntegrationCmd(),
		newRetryCmd(),
		newBuildRetryContextCmd(),
		newResumeDetectCmd(),
		newDaemonCmd(),
		newValidateProgramCmd(),
		newImportImplsCmd(),
		newListProgramsCmd(),
		newPopulateIntegrationChecklistCmd(), // M5: integration checklist
		newTierGateCmd(),
		newFreezeContractsCmd(),
		newProgramStatusCmd(),
		newProgramReplanCmd(),
		newMarkProgramCompleteCmd(),
		newVerifyInstallCmd(),
		newMetricsCmd(),
		newQueryCmd(),
		newCleanupStaleCmd(),
		newProgramExecuteCmd(),
		newCreateProgramCmd(),
		newCheckIMPLConflictsCmd(),
		newFinalizeTierCmd(),
		newCheckProgramConflictsCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
