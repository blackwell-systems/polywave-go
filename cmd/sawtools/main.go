package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print sawtools version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sawtools %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}

func main() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(
		newVersionCmd(),
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
		newCheckCompletionCmd(),
		newMarkCompleteCmd(),
		newSetImplStateCmd(),
		newRunGatesCmd(),
		newRunReviewCmd(),
		newCheckConflictsCmd(),
		newPredictConflictsCmd(),
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
		newDetectSharedTypesCmd(),
		newDetectWiringCmd(),
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
		newRunIntegrationAgentCmd(), // E26: Integration agent workflow
		newRunIntegrationWaveCmd(),  // E27: Planned integration waves
		newVerifyHookInstalledCmd(),
		newValidateIntegrationCmd(),
		newRetryCmd(),
		newResolveImplCmd(),
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
		newInitCmd(),           // zero-config project initialization
		newMetricsCmd(),
		newQueryCmd(),
		newCleanupStaleCmd(),
		newProgramExecuteCmd(),
		newCreateProgramCmd(),
		newCreateProgramWorktreesCmd(),
		newCheckIMPLConflictsCmd(),
		newPrepareTierCmd(),
		newFinalizeTierCmd(),
		newCheckProgramConflictsCmd(),
		// C7: Previously unregistered commands
		newPreWaveGateCmd(),
		newQueueCmd(),
		newUpdateProgramImplCmd(),
		newUpdateProgramStateCmd(),
		newCloseImplCmd(),
		newPreCommitCheckCmd(),
		newInstallHooksCmd(),
		newSetInjectionMethodCmd(),
		newPreWaveValidateCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
