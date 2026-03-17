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
		newRunGatesCmd(),
		newCheckConflictsCmd(),
		newCheckDepsCmd(),
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
		newAnalyzeDepsCmd(),
		newDetectScaffoldsCmd(),
		newAnalyzeSuitabilityCmd(),
		newExtractCommandsCmd(),
		newFinalizeWaveCmd(),
		newFinalizeImplCmd(),
		newDiagnoseBuildFailureCmd(),
		newAssignAgentIDsCmd(),
		newRunScoutCmd(), // I3: Phase 5 integration
		newVerifyHookInstalledCmd(),
		newValidateIntegrationCmd(),
		newRetryCmd(),
		newBuildRetryContextCmd(),
		newResumeDetectCmd(),
		newDaemonCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
