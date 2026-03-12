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
		newJournalInitCmd(),
		newPrepareAgentCmd(),
		newJournalContextCmd(),
		newAnalyzeDepsCmd(),
		newDetectScaffoldsCmd(),
		newAnalyzeSuitabilityCmd(),
		newExtractCommandsCmd(),
		newFinalizeWaveCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
