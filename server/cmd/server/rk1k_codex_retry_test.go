//go:build agentintegration && !windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

const rk1kE0bRetryMode = "e0b_retry"

func rk1kCodexEntryMode(registered, entry string) bool {
	if entry == "full" {
		return registered == "" || registered == "full"
	}
	return entry == rk1kE0bRetryMode && registered == entry
}

func rk1kReadBoundJSON(path, digest string, value any) error {
	if !filepath.IsAbs(path) || len(digest) != 64 {
		return errors.New("invalid prior evidence binding")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if protocol.CollaborationKnowledgeHash(string(raw)) != digest {
		return errors.New("prior evidence changed")
	}
	return json.Unmarshal(raw, value)
}

// Historical authorization is read only. Its expiration and old source binary
// belong to that completed attempt, while its ledger and account namespace stay
// bound to this retry. No caller-provided count can replace the retained facts.
func rk1kValidateRetryHistory(r *rk1kCodexRegistration) error {
	refuse := errors.New("invalid Codex retry history or mode")
	if rk1kCodexEntryMode(r.Mode, "full") {
		if r.AttemptLimit != 0 && r.AttemptLimit != 15 || r.PriorLedger != "" || r.PriorLedgerSHA256 != "" || r.PriorActualStarts != 0 || r.PriorDebits != 0 || r.PriorRegistration != "" || r.PriorRegistrationSHA256 != "" {
			return refuse
		}
		return nil
	}
	if r.Mode != rk1kE0bRetryMode || r.AttemptLimit != 1 || r.Cap != 15 || r.PriorActualStarts != 1 || r.PriorDebits != 1 {
		return refuse
	}
	var prior rk1kCodexRegistration
	if rk1kReadBoundJSON(r.PriorRegistration, r.PriorRegistrationSHA256, &prior) != nil {
		return refuse
	}
	if !prior.Authorized || prior.SchemaVersion != 1 || !rk1kCodexEntryMode(prior.Mode, "full") || prior.Cap != 15 || prior.Provider != "codex" || prior.Ledger != r.PriorLedger || prior.Ledger == r.Ledger || prior.Output == r.Output || prior.Executable != r.Executable || prior.ExecutableSHA256 != r.ExecutableSHA256 || prior.Version != r.Version || prior.Model != r.Model || prior.ConfigSHA256 != r.ConfigSHA256 {
		return refuse
	}
	var ledger rk1kCodexLedger
	if rk1kReadBoundJSON(r.PriorLedger, r.PriorLedgerSHA256, &ledger) != nil {
		return refuse
	}
	if ledger.Cap != 15 || !ledger.Stopped || !rk1kCodexEntryMode(ledger.Mode, "full") || ledger.AttemptLimit != 0 || ledger.PriorLedgerSHA256 != "" || ledger.PriorActualStarts != 0 || ledger.PriorDebits != 0 || len(ledger.Starts) != 1 {
		return refuse
	}
	start := ledger.Starts[0]
	if start.Sequence != 1 || !start.Identity.valid() || start.Identity.Kind != "task" || !start.ActualStarted || start.StartedAt == "" || start.ReservedAt == "" || start.Complete || start.ProcessComplete {
		return refuse
	}
	expected := filepath.Join(prior.Output, "E0b", "provider", start.Identity.key(), "launch-1")
	if !filepath.IsAbs(prior.Output) || start.Launch != expected {
		return refuse
	}
	return nil
}

func rk1kValidateLedgerMode(r *rk1kCodexRegistration, l *rk1kCodexLedger, id rk1kIdentity, launch string) error {
	if rk1kCodexEntryMode(r.Mode, "full") {
		if !rk1kCodexEntryMode(l.Mode, "full") || l.AttemptLimit != 0 || l.PriorLedgerSHA256 != "" || l.PriorActualStarts != 0 || l.PriorDebits != 0 {
			return errors.New("full series cannot spend retry ledger")
		}
		return nil
	}
	if r.Mode != rk1kE0bRetryMode || l.Mode != r.Mode || l.AttemptLimit != 1 || l.PriorLedgerSHA256 != r.PriorLedgerSHA256 || l.PriorActualStarts != 1 || l.PriorDebits != 1 || len(l.Starts) >= 1 || l.PriorDebits+len(l.Starts) >= l.Cap {
		return errors.New("single Codex retry budget exhausted or rebound")
	}
	if !id.valid() || id.Kind != "task" || launch != filepath.Join(r.Output, "E0b", "provider", id.key(), "launch-1") {
		return errors.New("retry must execute only the E0b task")
	}
	return nil
}

type rk1kE0bRetrySummary struct {
	SchemaVersion      int          `json:"schema_version"`
	Kind               string       `json:"kind"`
	Mode               string       `json:"mode"`
	AttemptLimit       int          `json:"attempt_limit"`
	MeasuredFakeStarts int          `json:"measured_fake_starts"`
	FakeSeedStarts     int          `json:"fake_seed_starts"`
	PaidStarts         int          `json:"paid_starts"`
	PriorActualStarts  int          `json:"prior_actual_starts"`
	TotalActualStarts  int          `json:"total_actual_starts"`
	RemainingCap       int          `json:"remaining_cap"`
	Samples            []rk1kSample `json:"samples"`
}

func rk1kE0bOnlySummary(sample rk1kSample, real bool) (rk1kE0bRetrySummary, error) {
	samples := []rk1kSample{sample}
	fake, seed, paid := rk1kCountStarts(samples)
	summary := rk1kE0bRetrySummary{SchemaVersion: 1, Kind: "fake_e0b_retry_proof", Mode: rk1kE0bRetryMode, AttemptLimit: 1, MeasuredFakeStarts: fake, FakeSeedStarts: seed, PaidStarts: paid, Samples: samples}
	if sample.Label != "E0b" || sample.Surface != "chat" || sample.Identity.Kind != "task" || !sample.Identity.valid() || sample.State != "finished" || sample.Verdict != "pass" || len(sample.Launches) != 1 || seed != 0 {
		return summary, errors.New("invalid single E0b retry sample")
	}
	if real {
		if fake != 0 || paid != 1 {
			return summary, errors.New("real E0b retry must have exactly one physical start")
		}
		summary.Kind = "real_e0b_retry"
		summary.PriorActualStarts = 1
		summary.TotalActualStarts = 2
		summary.RemainingCap = 13
	} else if fake != 1 || paid != 0 {
		return summary, errors.New("fake E0b retry must have one fake start")
	}
	return summary, nil
}

func rk1kRunE0bOnly(t *testing.T, out string, real bool) {
	t.Helper()
	var sample rk1kSample
	if !t.Run("E0b", func(t *testing.T) { sample = rk1kRunE0b(t, out) }) {
		t.FailNow()
	}
	summary, err := rk1kE0bOnlySummary(sample, real)
	if err != nil {
		t.Fatal(err)
	}
	if err := rk1WriteJSON(filepath.Join(out, "summary.json"), summary); err != nil {
		t.Fatal(err)
	}
}

func TestRK1KnowledgeRealE0bRetry(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("real E0b retry requires explicit authorization")
	}
	registration, err := rk1kLoadCodexRegistrationMode(os.Getenv("RK1K_CODEX_REGISTRATION"), os.Getenv("RK1K_CODEX_REGISTRATION_SHA256"), rk1kE0bRetryMode)
	if err != nil {
		t.Fatal(err)
	}
	rk1kActiveReal = registration
	defer func() {
		if t.Failed() {
			if err := rk1kStopCodexLedger(registration); err != nil {
				t.Errorf("cannot persist retry stop: %v", err)
			}
		}
		rk1kActiveReal = nil
	}()
	t.Setenv("RK1K_TEST_PROVIDER", "codex")
	rk1kRunE0bOnly(t, registration.Output, true)
}

func TestRK1KnowledgeCodexE0bRetryFakeProof(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("fake E0b retry refuses real gate")
	}
	t.Setenv("RK1K_TEST_PROVIDER", "codex")
	out := os.Getenv("RK1K_FAKE_RETRY_PROOF_OUT")
	if out == "" {
		out = t.TempDir()
	}
	rk1kRunE0bOnly(t, out, false)
}
