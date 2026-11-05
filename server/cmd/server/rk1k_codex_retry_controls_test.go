//go:build agentintegration && !windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func rk1kRetryControl(t *testing.T) (rk1kCodexRegistration, rk1kCodexLedger, rk1kCodexStart) {
	t.Helper()
	dir := t.TempDir()
	config, executable, source := filepath.Join(dir, "config"), filepath.Join(dir, "fake-executable"), filepath.Join(dir, "source")
	for _, path := range []string{config, executable, source} {
		if err := os.WriteFile(path, []byte("inert test fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(path string) string {
		h, err := rk1kFileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	prior := rk1kCodexRegistration{SchemaVersion: 1, Authorized: true, Provider: "codex", Cap: 15, Executable: executable, ExecutableSHA256: hash(executable), Version: "0.166.0", Model: "test-model", ConfigPath: config, ConfigSHA256: hash(config), Ledger: filepath.Join(dir, "prior-ledger.json"), Output: filepath.Join(dir, "prior-output"), Database: "multica_rk1k_old", ExpiresAt: time.Now().Add(-time.Hour)}
	previousID := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
	old := rk1kCodexStart{Sequence: 1, Identity: previousID, ActualStarted: true, ReservedAt: "2026-11-05T05:40:21Z", StartedAt: "2026-11-05T05:40:22Z", Launch: filepath.Join(prior.Output, "E0b", "provider", previousID.key(), "launch-1")}
	priorLedger := rk1kCodexLedger{Cap: 15, Starts: []rk1kCodexStart{old}, Stopped: true}
	priorPath := filepath.Join(dir, "prior-registration.json")
	if err := rk1WriteJSON(prior.Ledger, priorLedger); err != nil {
		t.Fatal(err)
	}
	if err := rk1WriteJSON(priorPath, prior); err != nil {
		t.Fatal(err)
	}
	r := prior
	r.Mode = rk1kE0bRetryMode
	r.AttemptLimit = 1
	r.PriorActualStarts = 1
	r.PriorDebits = 1
	r.PriorLedger = prior.Ledger
	r.PriorLedgerSHA256 = hash(prior.Ledger)
	r.PriorRegistration = priorPath
	r.PriorRegistrationSHA256 = hash(priorPath)
	r.Ledger = filepath.Join(dir, "new-ledger.json")
	r.Output = filepath.Join(dir, "new-output")
	r.Database = "multica_rk1k_new"
	r.TestBinarySHA256 = hash(self)
	r.SourceFiles = map[string]string{source: hash(source)}
	r.ExpiresAt = time.Now().Add(time.Hour)
	ledger := rk1kCodexLedger{Cap: 15, Mode: rk1kE0bRetryMode, AttemptLimit: 1, PriorLedgerSHA256: r.PriorLedgerSHA256, PriorActualStarts: 1, PriorDebits: 1}
	if err := rk1WriteJSON(r.Ledger, ledger); err != nil {
		t.Fatal(err)
	}
	return r, priorLedger, old
}

func TestRK1KnowledgeCodexE0bRetryControls(t *testing.T) {
	t.Run("single_attempt_preserves_prior", func(t *testing.T) {
		r, _, _ := rk1kRetryControl(t)
		if err := rk1kValidateRetryHistory(&r); err != nil {
			t.Fatal(err)
		}
		id := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
		launch := filepath.Join(r.Output, "E0b", "provider", id.key(), "launch-1")
		slot, err := rk1kReserveCodexLedger(&r, id, launch)
		if err != nil {
			t.Fatal(err)
		}
		if err := slot.started(); err != nil {
			t.Fatal(err)
		}
		if err := slot.finish(true); err != nil {
			t.Fatal(err)
		}
		sample := rk1kSample{Identity: id, State: "finished", Verdict: "pass"}
		if err := rk1kAcceptCodexSample(&r, sample); err != nil {
			t.Fatal(err)
		}
		id.TaskID = uuid.NewString()
		if _, err := rk1kReserveCodexLedger(&r, id, filepath.Join(r.Output, "E0b", "provider", id.key(), "launch-1")); err == nil {
			t.Fatal("completed retry admitted a second process")
		}
		actual, err := rk1kFileHash(r.PriorLedger)
		if err != nil || actual != r.PriorLedgerSHA256 {
			t.Fatal("prior stopped ledger changed")
		}
	})
	for _, name := range []string{"missing", "corrupt", "drift", "spent_count", "debit_count", "prior_actual", "prior_sequence", "prior_not_stopped", "prior_complete", "prior_namespace", "prior_registration_drift", "prior_config", "new_ledger_namespace", "same_ledger", "wrong_surface", "attempt_limit"} {
		t.Run(name, func(t *testing.T) {
			r, prior, _ := rk1kRetryControl(t)
			switch name {
			case "missing":
				if err := os.Remove(r.PriorLedger); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(r.PriorLedger, []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
				r.PriorLedgerSHA256, _ = rk1kFileHash(r.PriorLedger)
			case "drift":
				prior.Stopped = false
				if err := rk1WriteJSON(r.PriorLedger, prior); err != nil {
					t.Fatal(err)
				}
			case "spent_count":
				r.PriorActualStarts = 0
			case "debit_count":
				r.PriorDebits = 0
			case "prior_actual":
				prior.Starts[0].ActualStarted = false
			case "prior_sequence":
				prior.Starts[0].Sequence = 2
			case "prior_not_stopped":
				prior.Stopped = false
			case "prior_complete":
				prior.Starts[0].Complete = true
			case "prior_namespace":
				prior.Starts[0].Launch = filepath.Join(t.TempDir(), "unrelated-launch")
			case "prior_registration_drift":
				if err := os.WriteFile(r.PriorRegistration, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "prior_config":
				r.ConfigSHA256 = "changed"
			case "new_ledger_namespace":
				if err := rk1WriteJSON(r.Ledger, rk1kCodexLedger{Cap: 15}); err != nil {
					t.Fatal(err)
				}
			case "same_ledger":
				r.Ledger = r.PriorLedger
			case "attempt_limit":
				r.AttemptLimit = 2
			}
			if name == "prior_actual" || name == "prior_sequence" || name == "prior_not_stopped" || name == "prior_complete" || name == "prior_namespace" {
				if err := rk1WriteJSON(r.PriorLedger, prior); err != nil {
					t.Fatal(err)
				}
				r.PriorLedgerSHA256, _ = rk1kFileHash(r.PriorLedger)
			}
			id := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
			launch := filepath.Join(r.Output, "E0b", "provider", id.key(), "launch-1")
			if name == "wrong_surface" {
				launch = filepath.Join(r.Output, "K1", "provider", id.key(), "launch-1")
			}
			if _, err := rk1kReserveCodexLedger(&r, id, launch); err == nil {
				t.Fatal("invalid retry history/namespace admitted a reservation")
			}
		})
	}
	t.Run("mode_separation_and_full_compatibility", func(t *testing.T) {
		r, _, _ := rk1kRetryControl(t)
		path := filepath.Join(filepath.Dir(r.Ledger), "retry-registration.json")
		if err := rk1WriteJSON(path, r); err != nil {
			t.Fatal(err)
		}
		digest, _ := rk1kFileHash(path)
		if _, err := rk1kLoadCodexRegistrationMode(path, digest, rk1kE0bRetryMode); err != nil {
			t.Fatal(err)
		}
		if _, err := rk1kLoadCodexRegistrationMode(path, digest, "full"); err == nil {
			t.Fatal("full entry accepted retry registration")
		}
		r.Mode = ""
		r.AttemptLimit = 0
		r.PriorLedger = ""
		r.PriorLedgerSHA256 = ""
		r.PriorActualStarts = 0
		r.PriorDebits = 0
		r.PriorRegistration = ""
		r.PriorRegistrationSHA256 = ""
		if err := rk1WriteJSON(path, r); err != nil {
			t.Fatal(err)
		}
		digest, _ = rk1kFileHash(path)
		if _, err := rk1kLoadCodexRegistrationMode(path, digest, "full"); err != nil {
			t.Fatal(err)
		}
		if _, err := rk1kLoadCodexRegistrationMode(path, digest, rk1kE0bRetryMode); err == nil {
			t.Fatal("retry entry accepted full registration")
		}
		r.PriorActualStarts = 1
		if err := rk1kValidateRetryHistory(&r); err == nil {
			t.Fatal("full mode accepted arbitrary prior budget override")
		}
	})
	t.Run("summary_is_e0b_only", func(t *testing.T) {
		id := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
		sample := rk1kSample{Label: "E0b", Surface: "chat", Identity: id, State: "finished", Verdict: "pass", Launches: []rk1kLaunchMeta{{PaidStarts: 1}}}
		summary, err := rk1kE0bOnlySummary(sample, true)
		if err != nil {
			t.Fatal(err)
		}
		if summary.Mode != rk1kE0bRetryMode || summary.PaidStarts != 1 || summary.FakeSeedStarts != 0 || summary.TotalActualStarts != 2 || summary.RemainingCap != 13 || len(summary.Samples) != 1 {
			t.Fatal("retry accounting changed")
		}
		raw, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		var restored rk1kE0bRetrySummary
		if json.Unmarshal(raw, &restored) != nil || restored.Samples[0].Label != "E0b" {
			t.Fatal("retry summary lost exact sample")
		}
		sample.Label = "K1"
		if _, err := rk1kE0bOnlySummary(sample, true); err == nil {
			t.Fatal("K1 sample entered E0b-only summary")
		}
	})
}
