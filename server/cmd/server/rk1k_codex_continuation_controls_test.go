//go:build agentintegration && !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type rk1kContinuationControl struct {
	current, retry, original    rk1kCodexRegistration
	retryLedger, originalLedger rk1kCodexLedger
}

func rk1kNewContinuationControl(t *testing.T) *rk1kContinuationControl {
	t.Helper()
	retry, originalLedger, _ := rk1kRetryControl(t)
	id := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
	retryLedger := rk1kCodexLedger{Cap: 15, Mode: rk1kE0bRetryMode, AttemptLimit: 1, PriorActualStarts: 1, PriorDebits: 1, PriorLedgerSHA256: retry.PriorLedgerSHA256, Stopped: true, Starts: []rk1kCodexStart{{Sequence: 1, Identity: id, Launch: filepath.Join(retry.Output, "E0b", "provider", id.key(), "launch-1"), ReservedAt: "2026-11-05T06:08:42Z", StartedAt: "2026-11-05T06:08:43Z", ActualStarted: true}}}
	var original rk1kCodexRegistration
	if err := rk1kReadBoundJSON(retry.PriorRegistration, retry.PriorRegistrationSHA256, &original); err != nil {
		t.Fatal(err)
	}
	retry.ExpiresAt = time.Now().Add(-time.Hour)
	current := retry
	current.Mode = rk1kFullContinuationMode
	current.Cap = 17
	current.AttemptLimit = 15
	current.PriorActualStarts = 2
	current.PriorDebits = 2
	current.PriorRegistration = filepath.Join(filepath.Dir(retry.Ledger), "retry-registration.json")
	current.PriorLedger = retry.Ledger
	current.Ledger = filepath.Join(filepath.Dir(retry.Ledger), "continuation-ledger.json")
	current.Output = filepath.Join(filepath.Dir(retry.Ledger), "continuation-output")
	current.Database = "multica_rk1k_continuation"
	current.ExpiresAt = time.Now().Add(time.Hour)
	control := &rk1kContinuationControl{current: current, retry: retry, original: original, retryLedger: retryLedger, originalLedger: originalLedger}
	control.publishHistory(t)
	ledger := rk1kCodexLedger{Cap: 17, Mode: rk1kFullContinuationMode, AttemptLimit: 15, PriorActualStarts: 2, PriorDebits: 2, PriorLedgerSHA256: control.current.PriorLedgerSHA256}
	if err := rk1WriteJSON(control.current.Ledger, ledger); err != nil {
		t.Fatal(err)
	}
	return control
}

func (c *rk1kContinuationControl) publishHistory(t *testing.T) {
	t.Helper()
	write := func(path string, v any) string {
		if err := rk1WriteJSON(path, v); err != nil {
			t.Fatal(err)
		}
		h, err := rk1kFileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	c.retry.PriorRegistrationSHA256 = write(c.retry.PriorRegistration, c.original)
	c.retry.PriorLedgerSHA256 = write(c.retry.PriorLedger, c.originalLedger)
	c.retryLedger.PriorLedgerSHA256 = c.retry.PriorLedgerSHA256
	c.current.PriorRegistrationSHA256 = write(c.current.PriorRegistration, c.retry)
	c.current.PriorLedgerSHA256 = write(c.current.PriorLedger, c.retryLedger)
}

func rk1kContinuationIdentity() rk1kIdentity {
	return rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
}

func TestRK1KnowledgeCodexContinuationControls(t *testing.T) {
	t.Run("two_prior_plus_fifteen_only", func(t *testing.T) {
		c := rk1kNewContinuationControl(t)
		r := &c.current
		if err := rk1kValidateContinuationHistory(r); err != nil {
			t.Fatal(err)
		}
		old1, old2 := c.retry.PriorLedgerSHA256, r.PriorLedgerSHA256
		for i := 0; i < 15; i++ {
			id := rk1kContinuationIdentity()
			slot, err := rk1kReserveCodexLedger(r, id, filepath.Join(r.Output, "producer", "provider", id.key(), "launch-1"))
			if err != nil {
				t.Fatalf("new start%d: %v", i+1, err)
			}
			if err := slot.started(); err != nil {
				t.Fatal(err)
			}
			if err := slot.finish(true); err != nil {
				t.Fatal(err)
			}
			if err := rk1kAcceptCodexSample(r, rk1kSample{Identity: id, State: "finished", Verdict: "pass"}); err != nil {
				t.Fatal(err)
			}
		}
		id := rk1kContinuationIdentity()
		if _, err := rk1kReserveCodexLedger(r, id, filepath.Join(r.Output, "provider", id.key(), "launch-1")); err == nil {
			t.Fatal("sixteenth new execution admitted after complete predecessor")
		}
		got1, _ := rk1kFileHash(c.retry.PriorLedger)
		got2, _ := rk1kFileHash(r.PriorLedger)
		if got1 != old1 || got2 != old2 {
			t.Fatal("historical stopped ledgers changed")
		}
	})
	cases := []string{"missing_retry", "missing_original", "retry_drift", "original_drift", "corrupt_retry", "corrupt_original", "cycle", "duplicate_identity", "aliased_ledger", "forged_retry_actual", "forged_original_actual", "prior_actual_count", "prior_debit_count", "retry_prior_count", "retry_not_stopped", "original_not_stopped", "third_old_start", "retry_complete", "retry_namespace", "account_drift", "model_drift", "current_cap", "attempt_limit", "current_ledger_mode", "current_ledger_prior", "incomplete_current", "current_output_escape"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			c := rk1kNewContinuationControl(t)
			r := &c.current
			switch name {
			case "forged_retry_actual":
				c.retryLedger.Starts[0].ActualStarted = false
			case "forged_original_actual":
				c.originalLedger.Starts[0].ActualStarted = false
			case "duplicate_identity":
				c.retryLedger.Starts[0].Identity = c.originalLedger.Starts[0].Identity
				c.retryLedger.Starts[0].Launch = filepath.Join(c.retry.Output, "E0b", "provider", c.retryLedger.Starts[0].Identity.key(), "launch-1")
			case "retry_prior_count":
				c.retry.PriorDebits = 2
			case "retry_not_stopped":
				c.retryLedger.Stopped = false
			case "original_not_stopped":
				c.originalLedger.Stopped = false
			case "third_old_start":
				c.retryLedger.Starts = append(c.retryLedger.Starts, c.retryLedger.Starts[0])
				c.retryLedger.Starts[1].Sequence = 2
			case "retry_complete":
				c.retryLedger.Starts[0].Complete = true
			case "retry_namespace":
				c.retryLedger.Starts[0].Launch = filepath.Join(c.original.Output, "E0b", "provider", c.retryLedger.Starts[0].Identity.key(), "launch-1")
			}
			c.publishHistory(t)
			switch name {
			case "missing_retry":
				if err := os.Remove(r.PriorLedger); err != nil {
					t.Fatal(err)
				}
			case "missing_original":
				if err := os.Remove(c.retry.PriorLedger); err != nil {
					t.Fatal(err)
				}
			case "retry_drift":
				if err := os.WriteFile(r.PriorLedger, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "original_drift":
				if err := os.WriteFile(c.retry.PriorLedger, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt_retry":
				if err := os.WriteFile(r.PriorLedger, []byte("bad JSON"), 0600); err != nil {
					t.Fatal(err)
				}
				r.PriorLedgerSHA256, _ = rk1kFileHash(r.PriorLedger)
			case "corrupt_original":
				if err := os.WriteFile(c.retry.PriorLedger, []byte("bad JSON"), 0600); err != nil {
					t.Fatal(err)
				}
				c.retry.PriorLedgerSHA256, _ = rk1kFileHash(c.retry.PriorLedger)
				if err := rk1WriteJSON(r.PriorRegistration, c.retry); err != nil {
					t.Fatal(err)
				}
				r.PriorRegistrationSHA256, _ = rk1kFileHash(r.PriorRegistration)
			case "cycle":
				c.retry.PriorRegistration = r.PriorRegistration
				c.retry.PriorRegistrationSHA256 = r.PriorRegistrationSHA256
				if err := rk1WriteJSON(r.PriorRegistration, c.retry); err != nil {
					t.Fatal(err)
				}
				r.PriorRegistrationSHA256, _ = rk1kFileHash(r.PriorRegistration)
			case "aliased_ledger":
				if err := os.Remove(r.Ledger); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(r.PriorLedger, r.Ledger); err != nil {
					t.Fatal(err)
				}
			case "prior_actual_count":
				r.PriorActualStarts = 3
			case "prior_debit_count":
				r.PriorDebits = 1
			case "account_drift":
				r.ConfigSHA256 = "different-account"
			case "model_drift":
				r.Model = "different-model"
			case "current_cap":
				r.Cap = 18
			case "attempt_limit":
				r.AttemptLimit = 16
			}
			// History mutations must fail in the history validator itself, not
			// incidentally because republishing changed the current ledger hash.
			if name != "current_ledger_mode" && name != "current_ledger_prior" && name != "incomplete_current" && name != "current_output_escape" {
				if err := rk1kValidateContinuationHistory(r); err == nil {
					t.Fatal("invalid continuation history was accepted")
				}
				return
			}
			id := rk1kContinuationIdentity()
			launch := filepath.Join(r.Output, "provider", id.key(), "launch-1")
			if name == "current_ledger_mode" || name == "current_ledger_prior" || name == "incomplete_current" {
				l := rk1kCodexLedger{Cap: 17, Mode: rk1kFullContinuationMode, AttemptLimit: 15, PriorActualStarts: 2, PriorDebits: 2, PriorLedgerSHA256: r.PriorLedgerSHA256}
				if name == "current_ledger_mode" {
					l.Mode = "full"
				}
				if name == "current_ledger_prior" {
					l.PriorDebits = 1
				}
				if name == "incomplete_current" {
					l.Starts = []rk1kCodexStart{{Sequence: 1, Identity: rk1kContinuationIdentity(), ActualStarted: true}}
				}
				if err := rk1WriteJSON(r.Ledger, l); err != nil {
					t.Fatal(err)
				}
			}
			if name == "current_output_escape" {
				launch = filepath.Join(filepath.Dir(r.Output), "elsewhere", id.key())
			}
			if _, err := rk1kReserveCodexLedger(r, id, launch); err == nil {
				t.Fatal("invalid continuation admitted a new debit")
			}
		})
	}
	t.Run("output_namespace", func(t *testing.T) {
		for _, history := range []string{"original", "retry"} {
			for _, alias := range []string{"dot", "case", "symlink", "symlinked_ancestor", "descendant", "ancestor", "symlinked_missing_descendant"} {
				t.Run(history+"/"+alias, func(t *testing.T) {
					c := rk1kNewContinuationControl(t)
					prior := c.original.Output
					if history == "retry" {
						prior = c.retry.Output
					}
					if err := os.MkdirAll(prior, 0700); err != nil {
						t.Fatal(err)
					}
					link := filepath.Join(filepath.Dir(c.current.Output), "output-alias")
					switch alias {
					case "dot":
						c.current.Output = prior + string(filepath.Separator) + "."
					case "case":
						c.current.Output = filepath.Join(filepath.Dir(prior), strings.ToUpper(filepath.Base(prior)))
						original, err := os.Stat(prior)
						if err != nil {
							t.Fatal(err)
						}
						aliased, err := os.Stat(c.current.Output)
						if os.IsNotExist(err) {
							t.Skip("filesystem uses case-sensitive directory names")
						}
						if err != nil || !os.SameFile(original, aliased) {
							t.Fatal("case alias did not identify the original directory")
						}
					case "symlink":
						if err := os.Symlink(prior, link); err != nil {
							t.Fatal(err)
						}
						c.current.Output = link
					case "symlinked_ancestor", "symlinked_missing_descendant":
						if err := os.Symlink(filepath.Dir(prior), link); err != nil {
							t.Fatal(err)
						}
						c.current.Output = filepath.Join(link, filepath.Base(prior))
						if alias == "symlinked_missing_descendant" {
							c.current.Output = filepath.Join(c.current.Output, "missing", "continuation")
						}
					case "descendant":
						c.current.Output = filepath.Join(prior, "continuation")
					case "ancestor":
						c.current.Output = filepath.Dir(prior)
					}
					before, err := rk1kFileHash(c.current.Ledger)
					if err != nil {
						t.Fatal(err)
					}
					if err := rk1kValidateContinuationHistory(&c.current); err == nil {
						t.Error("continuation history accepted an overlapping output namespace")
					}
					id := rk1kContinuationIdentity()
					if _, err := rk1kReserveCodexLedger(&c.current, id, filepath.Join(c.current.Output, "provider", id.key(), "launch-1")); err == nil {
						t.Error("overlapping output namespace admitted a new reservation")
					}
					after, err := rk1kFileHash(c.current.Ledger)
					if err != nil || after != before {
						t.Fatal("refused namespace changed the continuation ledger")
					}
				})
			}
		}
		for _, namespace := range []string{"fresh", "similar_name", "symlinked_fresh_ancestor"} {
			t.Run(namespace, func(t *testing.T) {
				c := rk1kNewContinuationControl(t)
				switch namespace {
				case "fresh":
					c.current.Output = filepath.Join(c.current.Output, "missing", "result")
				case "similar_name":
					c.current.Output = c.original.Output + "-continuation"
				case "symlinked_fresh_ancestor":
					link := filepath.Join(filepath.Dir(c.current.Output), "fresh-alias")
					if err := os.Symlink(t.TempDir(), link); err != nil {
						t.Fatal(err)
					}
					c.current.Output = filepath.Join(link, "missing", "result")
				}
				if err := rk1kValidateContinuationHistory(&c.current); err != nil {
					t.Fatalf("fresh output namespace rejected: %v", err)
				}
				id := rk1kContinuationIdentity()
				if _, err := rk1kReserveCodexLedger(&c.current, id, filepath.Join(c.current.Output, "provider", id.key(), "launch-1")); err != nil {
					t.Fatalf("fresh output namespace could not reserve: %v", err)
				}
			})
		}
	})
	t.Run("launch_symlink_escape", func(t *testing.T) {
		for _, history := range []string{"original", "retry"} {
			t.Run(history, func(t *testing.T) {
				c := rk1kNewContinuationControl(t)
				prior := c.original.Output
				if history == "retry" {
					prior = c.retry.Output
				}
				for _, path := range []string{prior, c.current.Output} {
					if err := os.MkdirAll(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(prior, filepath.Join(c.current.Output, "provider")); err != nil {
					t.Fatal(err)
				}
				if err := rk1kValidateContinuationHistory(&c.current); err != nil {
					t.Fatalf("distinct output root rejected before launch check: %v", err)
				}
				before, err := rk1kFileHash(c.current.Ledger)
				if err != nil {
					t.Fatal(err)
				}
				id := rk1kContinuationIdentity()
				if _, err := rk1kReserveCodexLedger(&c.current, id, filepath.Join(c.current.Output, "provider", id.key(), "launch-1")); err == nil {
					t.Error("launch symlink escaped into historical output")
				}
				after, err := rk1kFileHash(c.current.Ledger)
				if err != nil || after != before {
					t.Fatal("escaped launch changed the continuation ledger")
				}
			})
		}
	})
	t.Run("entry_mode_isolation", func(t *testing.T) {
		c := rk1kNewContinuationControl(t)
		path := filepath.Join(filepath.Dir(c.current.Ledger), "registration.json")
		for _, r := range []rk1kCodexRegistration{c.current, c.retry, c.original} {
			r.ExpiresAt = time.Now().Add(time.Hour)
			r.TestBinarySHA256 = c.current.TestBinarySHA256
			r.SourceFiles = c.current.SourceFiles
			if err := rk1WriteJSON(path, r); err != nil {
				t.Fatal(err)
			}
			digest, _ := rk1kFileHash(path)
			expected := "full"
			if r.Mode != "" {
				expected = r.Mode
			}
			for _, entry := range []string{"full", rk1kE0bRetryMode, rk1kFullContinuationMode} {
				_, err := rk1kLoadCodexRegistrationMode(path, digest, entry)
				if (err == nil) != (entry == expected) {
					t.Fatalf("mode=%q entry=%s error=%v", r.Mode, entry, err)
				}
			}
		}
	})
	t.Run("summary_no_double_count_or_fake_paid", func(t *testing.T) {
		c := rk1kNewContinuationControl(t)
		samples := []rk1kSample{{Launches: []rk1kLaunchMeta{{PaidStarts: 15}, {FakeStarts: 3, Seed: true}}}}
		summary := rk1kFullProofSummary(samples, &c.current)
		if summary["mode"] != rk1kFullContinuationMode || summary["knowledge_allocated"] != 17 || summary["prior_actual_starts"] != 2 || summary["new_actual_starts"] != 15 || summary["total_actual_starts"] != 17 || summary["remaining_cap"] != 0 || summary["fake_seed_starts"] != 3 {
			t.Fatal("continuation summary changed fixed accounting")
		}
		fake := rk1kFullProofSummary([]rk1kSample{{Launches: []rk1kLaunchMeta{{FakeStarts: 15}, {FakeStarts: 3, Seed: true}}}}, nil)
		if fake["paid_starts"] != 0 || fake["kind"] != "fake_framework_proof" || fake["knowledge_allocated"] != 0 || fake["prior_actual_starts"] != nil {
			t.Fatal("fake summary falsely claimed real historical or paid starts")
		}
		old := rk1kFullProofSummary(samples, &c.original)
		if old["kind"] != "real_codex_model" || old["knowledge_allocated"] != 15 || old["prior_actual_starts"] != nil {
			t.Fatal("original full summary changed")
		}
	})
}
