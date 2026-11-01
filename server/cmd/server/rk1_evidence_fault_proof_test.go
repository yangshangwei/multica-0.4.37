//go:build agentintegration && !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The same wrapper finalization seam runs through the production host path.
// A valid fake Provider answer must remain an incomplete sample after any raw
// evidence I/O error, retain its one launch, and never start trial2.
func TestRK1EvidenceFaultProof(t *testing.T) {
	rk1RequireDatabase(t)
	env := rk1BuildEnv(t, "", rk1FakeVersion, "rk1-requested-model")
	for _, stage := range []string{"stdout_sync", "stdout_close", "launch_record"} {
		t.Run(stage, func(t *testing.T) {
			out := t.TempDir()
			if root := os.Getenv("RK1_EVIDENCE_FAULT_PROOF_OUT"); root != "" {
				out = filepath.Join(root, stage)
				if err := os.Mkdir(out, 0700); err != nil {
					t.Fatal("evidence-fault proof directory already exists or is unavailable")
				}
			}
			trials := rk1Series(t, env, out, rk1Scenarios["RK1-E0a"], 1, 2, rk1Fixed("evidence_"+stage))
			if len(trials) != 1 || trials[0].Status != "incomplete" || trials[0].Verdict != "measured:provider_evidence_publication_failed" {
				t.Fatalf("raw evidence fault did not stop the original trial: %+v", trials)
			}
			trial := trials[0]
			if len(trial.Runs) != 1 || trial.Runs[0].Launches != 1 || len(trial.Runs[0].ExecutionExitCodes) != 1 || trial.Runs[0].ExecutionExitCodes[0] == nil || *trial.Runs[0].ExecutionExitCodes[0] != 0 {
				t.Fatalf("evidence failure lost the one successful fake execution: %+v", trial.Runs)
			}
			launch := rk1ReadLaunch(filepath.Join(out, trial.Label, "provider", trial.Runs[0].RunID, "launch-1"))
			if !launch.ResultSeen || launch.ResultError {
				t.Fatal("the Provider's successful raw result was replaced by the evidence failure")
			}
			if _, err := os.Stat(filepath.Join(out, rk1Label("RK1-E0a", 2, ""))); !os.IsNotExist(err) {
				t.Fatalf("trial2 started after the raw evidence failure: %v", err)
			}
		})
	}
}
