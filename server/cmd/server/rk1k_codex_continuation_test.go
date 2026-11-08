//go:build agentintegration && !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const rk1kFullContinuationMode = "full_continuation"

func rk1kCodexCapForMode(mode string, cap int) bool {
	if mode == rk1kFullContinuationMode {
		return cap == 17
	}
	return (rk1kCodexEntryMode(mode, "full") || mode == rk1kE0bRetryMode) && cap == 15
}

// Resolve existing parents before appending directories a fresh run will create.
// A dangling link or an unresolved non-directory cannot establish a namespace.
func rk1kContinuationOutputPath(path string) (string, error) {
	invalid := errors.New("invalid continuation output namespace")
	if !filepath.IsAbs(path) {
		return "", invalid
	}
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		// Cleaning parent traversal before resolving links changes its meaning.
		if part == ".." {
			return "", invalid
		}
	}
	parent, suffix := filepath.Clean(path), ""
	for {
		_, err := os.Lstat(parent)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return "", invalid
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.IsDir() {
				return "", invalid
			}
			return filepath.Join(resolved, suffix), nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(parent) == parent {
			return "", invalid
		}
		suffix = filepath.Join(filepath.Base(parent), suffix)
		parent = filepath.Dir(parent)
	}
}

func rk1kContinuationOutputsOverlap(first, second string) bool {
	for _, pair := range [][2]string{{first, second}, {second, first}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
		// EvalSymlinks retains spelling on case-insensitive filesystems. Match
		// existing ancestors by identity as well, including a not-yet-created tail.
		base, err := os.Stat(pair[0])
		if err == nil {
			for path := pair[1]; ; path = filepath.Dir(path) {
				if info, err := os.Stat(path); err == nil && os.SameFile(base, info) {
					return true
				}
				if filepath.Dir(path) == path {
					break
				}
			}
		}
	}
	return false
}

// This is the one approved two-ledger continuation, not a recursive history
// walker. The retry's prior counter references the original start; it is never
// added again to the two unique, retained physical attempts.
func rk1kValidateContinuationHistory(r *rk1kCodexRegistration) error {
	refuse := errors.New("invalid full knowledge continuation history")
	if r.Mode != rk1kFullContinuationMode || r.Cap != 17 || r.AttemptLimit != 15 || r.PriorActualStarts != 2 || r.PriorDebits != 2 {
		return refuse
	}
	var retry rk1kCodexRegistration
	if rk1kReadBoundJSON(r.PriorRegistration, r.PriorRegistrationSHA256, &retry) != nil {
		return refuse
	}
	if retry.Mode != rk1kE0bRetryMode || !retry.Authorized || retry.SchemaVersion != 1 || retry.Provider != "codex" || retry.Ledger != r.PriorLedger || retry.Executable != r.Executable || retry.ExecutableSHA256 != r.ExecutableSHA256 || retry.Model != r.Model || retry.Version != r.Version || retry.ConfigSHA256 != r.ConfigSHA256 {
		return refuse
	}
	if rk1kValidateRetryHistory(&retry) != nil {
		return refuse
	}
	var ledger, original rk1kCodexLedger
	if rk1kReadBoundJSON(r.PriorLedger, r.PriorLedgerSHA256, &ledger) != nil || rk1kReadBoundJSON(retry.PriorLedger, retry.PriorLedgerSHA256, &original) != nil {
		return refuse
	}
	if ledger.Cap != 15 || ledger.Mode != rk1kE0bRetryMode || ledger.AttemptLimit != 1 || ledger.PriorLedgerSHA256 != retry.PriorLedgerSHA256 || ledger.PriorActualStarts != 1 || ledger.PriorDebits != 1 || !ledger.Stopped || len(ledger.Starts) != 1 || len(original.Starts) != 1 {
		return refuse
	}
	start, first := ledger.Starts[0], original.Starts[0]
	if start.Sequence != 1 || !start.Identity.valid() || start.Identity.Kind != "task" || !start.ActualStarted || start.StartedAt == "" || start.ReservedAt == "" || start.ProcessComplete || start.Complete || start.Identity.key() == first.Identity.key() || start.Launch != filepath.Join(retry.Output, "E0b", "provider", start.Identity.key(), "launch-1") {
		return refuse
	}
	firstAt, err := time.Parse(time.RFC3339Nano, first.StartedAt)
	if err != nil {
		return refuse
	}
	retryReserved, err := time.Parse(time.RFC3339Nano, start.ReservedAt)
	if err != nil || !retryReserved.After(firstAt) {
		return refuse
	}
	retryStarted, err := time.Parse(time.RFC3339Nano, start.StartedAt)
	if err != nil || retryStarted.Before(retryReserved) {
		return refuse
	}
	var initial rk1kCodexRegistration
	if rk1kReadBoundJSON(retry.PriorRegistration, retry.PriorRegistrationSHA256, &initial) != nil {
		return refuse
	}
	output, err := rk1kContinuationOutputPath(r.Output)
	if err != nil || !filepath.IsAbs(r.Ledger) {
		return refuse
	}
	for _, prior := range []string{initial.Output, retry.Output} {
		previous, err := rk1kContinuationOutputPath(prior)
		if err != nil || rk1kContinuationOutputsOverlap(output, previous) {
			return refuse
		}
	}
	// Distinct names alone do not reject symlink/hardlink aliases of old ledgers.
	paths := []string{r.PriorRegistration, retry.PriorRegistration, r.PriorLedger, retry.PriorLedger, r.Ledger}
	if r.RegistrationPath != "" {
		paths = append(paths, r.RegistrationPath)
	}
	infos := []os.FileInfo{}
	names := map[string]bool{}
	for _, path := range paths {
		if !filepath.IsAbs(path) || names[filepath.Clean(path)] {
			return refuse
		}
		names[filepath.Clean(path)] = true
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) && path == r.Ledger {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return refuse
		}
		for _, prior := range infos {
			if os.SameFile(info, prior) {
				return refuse
			}
		}
		infos = append(infos, info)
	}
	return nil
}

func rk1kValidateContinuationLedger(r *rk1kCodexRegistration, l *rk1kCodexLedger, id rk1kIdentity, launch string) error {
	if l.Cap != 17 || l.Mode != rk1kFullContinuationMode || l.AttemptLimit != 15 || l.PriorLedgerSHA256 != r.PriorLedgerSHA256 || l.PriorActualStarts != 2 || l.PriorDebits != 2 || len(l.Starts) >= 15 || l.PriorDebits+len(l.Starts) >= l.Cap {
		return errors.New("full continuation budget exhausted or ledger rebound")
	}
	if !id.valid() || !filepath.IsAbs(launch) {
		return errors.New("invalid continuation launch identity")
	}
	output, err := rk1kContinuationOutputPath(r.Output)
	if err != nil {
		return err
	}
	physicalLaunch, err := rk1kContinuationOutputPath(launch)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(output, physicalLaunch)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("continuation launch escaped its output namespace")
	}
	for i, start := range l.Starts {
		if start.Sequence != i+1 || !start.Identity.valid() || !start.ActualStarted || !start.ProcessComplete || !start.Complete {
			return errors.New("continuation predecessor incomplete or corrupt")
		}
	}
	return nil
}

func TestRK1KnowledgeRealContinuation(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("full continuation requires explicit authorization")
	}
	r, err := rk1kLoadCodexRegistrationMode(os.Getenv("RK1K_CODEX_REGISTRATION"), os.Getenv("RK1K_CODEX_REGISTRATION_SHA256"), rk1kFullContinuationMode)
	if err != nil {
		t.Fatal(err)
	}
	rk1kActiveReal = r
	defer func() {
		if t.Failed() {
			if err := rk1kStopCodexLedger(r); err != nil {
				t.Errorf("cannot persist full continuation stop: %v", err)
			}
		}
		rk1kActiveReal = nil
	}()
	t.Setenv("RK1K_TEST_PROVIDER", "codex")
	t.Setenv("RK1K_FAKE_PROOF_OUT", r.Output)
	rk1kRunProof(t)
}

func rk1kFullProofSummary(samples []rk1kSample, r *rk1kCodexRegistration) map[string]any {
	measured, seed, paid := rk1kCountStarts(samples)
	real, allocation := r != nil, 0
	kind := "fake_framework_proof"
	if real {
		kind = "real_codex_model"
		allocation = 15
	}
	summary := map[string]any{"schema_version": 1, "kind": kind, "measured_fake_starts": measured, "fake_seed_starts": seed, "paid_starts": paid, "knowledge_allocated": allocation, "real_runnable": real, "samples": samples}
	if r != nil && r.Mode == rk1kFullContinuationMode {
		summary["kind"] = "real_codex_full_continuation"
		summary["mode"] = r.Mode
		summary["knowledge_allocated"] = r.Cap
		summary["attempt_limit"] = r.AttemptLimit
		summary["prior_actual_starts"] = r.PriorActualStarts
		summary["prior_debits"] = r.PriorDebits
		summary["new_actual_starts"] = paid
		summary["total_actual_starts"] = r.PriorActualStarts + paid
		summary["remaining_cap"] = r.Cap - r.PriorDebits - paid
	}
	return summary
}
