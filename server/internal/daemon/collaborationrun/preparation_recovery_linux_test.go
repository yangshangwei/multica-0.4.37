//go:build linux

package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func blockingPreparedGitController(t *testing.T) *NamespaceController {
	t.Helper()
	c := preparedNativeController(t, "static_json_v1", true)
	file := bundleTestFile(t, "blocked.bundle", []byte("fixture blocking Git archive"), 0600)
	c.Bundle.Git.BundleFile, c.Bundle.Git.BundleSHA256, c.Bundle.Git.SizeBytes = file.Source, file.SHA256, file.SizeBytes
	return c
}

// A comm value is only a deterministic test barrier. It never supplies a
// termination proof; the assertions below still require the retained ledger.
func waitPreparedFixture(ctx context.Context, present bool) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			name, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
			if strings.TrimSpace(string(name)) == "git-prep-block" {
				found = true
				break
			}
		}
		if found == present {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func TestLinuxPreparedStopDuringGitUsesExitedProofWithoutProvider(t *testing.T) {
	c := blockingPreparedGitController(t)
	observed := make(chan error, 1)
	h, err := preparedNativeRun(t, c, 0, 0, func(h *preparedNativeHTTP) {
		go func() {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			err := waitPreparedFixture(ctx, true)
			if err == nil {
				h.mu.Lock()
				h.stop = true
				h.mu.Unlock()
			}
			observed <- err
		}()
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-observed; err != nil {
		t.Fatal("Git never reached the controlled stop barrier")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.outcomes) != 0 || len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" || h.proofs[0].AttemptNonce != h.permit.AttemptNonce || h.checks != 2 {
		t.Fatalf("Stop during preparation changed outcome/proof semantics: outcomes=%d proofs=%+v checks=%d", len(h.outcomes), h.proofs, h.checks)
	}
}

type preparedSupervisorFixture struct {
	Bundle          *ProviderBundleConfig                 `json:"bundle"`
	Directory       string                                `json:"directory"`
	LedgerDirectory string                                `json:"ledger_directory"`
	ServerURL       string                                `json:"server_url"`
	Request         protocol.CollaborationRunClaimRequest `json:"request"`
	RunID           string                                `json:"run_id"`
}

func runPreparedSupervisorFixture() int {
	raw, err := os.ReadFile(os.Getenv("S4_PREPARED_SUPERVISOR_FIXTURE"))
	if err != nil {
		return 110
	}
	var fixture preparedSupervisorFixture
	if decodeStrict(raw, &fixture) != nil {
		return 111
	}
	ledger, err := OpenLedger(fixture.LedgerDirectory)
	if err != nil {
		return 112
	}
	client, err := NewClient(fixture.ServerURL, "mdt_fixture", nil)
	if err != nil {
		return 113
	}
	self, err := os.Executable()
	if err != nil {
		return 114
	}
	runner := &Runner{Ledger: ledger, Client: client, Controller: &NamespaceController{Bundle: fixture.Bundle, Directory: fixture.Directory, HelperExecutable: self}, RuntimeConfigEpoch: "5", StopPollInterval: 20 * time.Millisecond}
	finished, crash := make(chan struct{}), make(chan struct{})
	go func() { _ = runner.Run(context.Background(), fixture.RunID, fixture.Request); close(finished) }()
	go func() { var value [1]byte; _, _ = os.Stdin.Read(value[:]); close(crash) }()
	select {
	case <-finished:
		return 115
	case <-crash:
		return 23
	}
}

func TestLinuxPreparedSupervisorCrashDuringGitRetainsUnknown(t *testing.T) {
	c := blockingPreparedGitController(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := c.Probe(ctx); err != nil {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatalf("required native preparation containment: %v", err)
		}
		t.Skipf("native preparation unavailable: %v", err)
	}
	h, server := preparedNativeHTTPFixture(t, c, 0, 0)
	defer server.Close()
	ledgerDirectory := filepath.Join(t.TempDir(), "ledger")
	fixture := preparedSupervisorFixture{Bundle: c.Bundle, Directory: c.Directory, LedgerDirectory: ledgerDirectory, ServerURL: server.URL, Request: h.request, RunID: h.claim.Identity.RunID}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "supervisor.json")
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, self, "--prepared-supervisor-fixture")
	cmd.Env = []string{"S4_PREPARED_SUPERVISOR_FIXTURE=" + config, "S4_EXPLICIT_NATIVE_KEY=explicit-native-fixture-key", "S4_AMBIENT_SECRET=unrelated-fixture-secret"}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err := waitPreparedFixture(ctx, true); err != nil {
		t.Fatal("Git did not reach the crash barrier")
	}
	if _, err := input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	err = cmd.Wait()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 23 {
		t.Fatalf("supervisor did not take the controlled crash: %v", err)
	}
	if err := waitPreparedFixture(ctx, false); err != nil {
		t.Fatal("Git helper outlived supervisor loss")
	}
	ledger, err := OpenLedger(ledgerDirectory)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ledger.Retained()
	if err != nil || len(entries) != 1 || !entries[0].Unconfirmed || entries[0].CanProve {
		t.Fatalf("crash released the original capacity: %+v %v", entries, err)
	}
	client, err := NewClient(server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	restart := &Runner{Ledger: ledger, Client: client, Controller: c}
	if err := restart.Run(ctx, fixture.RunID, fixture.Request); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("restart fabricated recovery after unobserved preparation exit: %v", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.outcomes) != 0 || len(h.proofs) != 0 {
		t.Fatal("crash fabricated provider output or an exit proof")
	}
}
