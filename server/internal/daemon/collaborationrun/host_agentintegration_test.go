//go:build agentintegration

package collaborationrun

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TestHostProviderAccountSmoke exercises a selected authenticated ordinary
// backend through HostExecutor/Runner. Its server, issue, working directory,
// ledger and current Multica CLI are test-owned. It is account/adapter evidence,
// not a substitute for the real-server authorization or native fault matrices.
func TestHostProviderAccountSmoke(t *testing.T) {
	// This must precede executable discovery, version probes and account access.
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("set MULTICA_RUN_REAL_AGENT_SMOKE=1 only after explicit account/quota approval")
	}
	if testing.Short() {
		t.Skip("authenticated host smoke is excluded from short tests")
	}
	provider := os.Getenv("MULTICA_HOST_SMOKE_PROVIDER")
	switch provider {
	case "claude", "codex", "opencode":
	default:
		t.Fatal("select MULTICA_HOST_SMOKE_PROVIDER=claude, codex, or opencode")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	executable := os.Getenv("MULTICA_HOST_SMOKE_EXECUTABLE")
	if executable != "" && !filepath.IsAbs(executable) {
		t.Fatal("MULTICA_HOST_SMOKE_EXECUTABLE must be an explicit absolute path")
	}
	if executable == "" {
		var err error
		executable, err = exec.LookPath(provider)
		if err != nil {
			t.Fatal("selected Provider executable is unavailable")
		}
		executable, err = filepath.Abs(executable)
		if err != nil {
			t.Fatal("selected Provider executable could not be resolved")
		}
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
	detected, err := agent.DetectVersion(probeCtx, agent.NewCommand(executable, nil))
	probeCancel()
	if err != nil {
		t.Fatal("selected Provider version diagnostic failed")
	}
	// Share production's exact known-banner normalization and publish only the
	// observed version, never arbitrary diagnostics, paths or account data.
	version, validVersion := agent.NormalizeHostProviderVersion(provider, detected)
	if !validVersion || strings.HasPrefix(version, "fixture-") || agent.CheckMinVersion(provider, version) != nil {
		t.Fatal("selected Provider has no supported observed version")
	}
	t.Logf("provider=%s version=%s os=%s arch=%s", provider, version, runtime.GOOS, runtime.GOARCH)

	cliExecutable := buildHostAccountSmokeCLI(t, ctx)
	config := &HostConfig{Provider: provider, ProviderVersion: version, ProviderMode: "local", AccountSource: "runtime_configuration",
		Executable: executable, Limits: preparationTestCapabilities().Limits}
	config.Limits.TimeoutSeconds = 30
	capabilities, err := config.Capabilities()
	if err != nil {
		t.Fatal("selected Provider combination does not admit host execution")
	}
	request, claim := hostTestClaim(t, capabilities)
	const expected = "HOST_SMOKE_OK"
	prompt := "Use only the supplied multica issue get " + claim.Identity.IssueID + " command to read this test issue. " +
		"Then reply with exactly HOST_SMOKE_OK and no other text. Do not modify files or accounts, submit comments, " +
		"use other tools, inspect account data, or start background work."
	claim.Inputs = claim.Inputs[:1]
	claim.Inputs[0].Body, claim.Inputs[0].AttachmentIDs = prompt, []string{}
	claim.Preparation.CommandIDs = []string{claim.Inputs[0].CommandID}
	claim.Preparation.Attachments = []protocol.CollaborationPreparationAttachment{}
	claim.Preparation.Git, claim.Preparation.Limits = nil, capabilities.Limits
	claim.Preparation.Invocation = protocol.CollaborationPreparationInvocation{}
	if protocol.SealCollaborationPreparation(claim.Preparation) != nil || protocol.ValidateCollaborationPreparation(claim.Preparation, claim.Identity, claim.Inputs) != nil {
		t.Fatal("test-owned host preparation could not be sealed")
	}

	fixture := &hostRunFixture{t: t, request: request, claim: claim, permits: map[string]protocol.CollaborationRunPermitRequest{},
		terminations: map[string]protocol.CollaborationLaunchTermination{}}
	var contextReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeHostAccountSmokeRequest(fixture, r) {
			t.Error("host smoke attempted an unrelated or unauthorized HTTP operation")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/context") {
			if r.Header.Get("X-Client-Platform") != "cli" {
				t.Error("test issue context was not requested through the supplied Multica CLI")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			contextReads.Add(1)
			view := cli.CollaborationRuntimeContext{Identity: claim.Identity, IssueID: claim.Identity.IssueID, Title: "Host Provider smoke",
				Description: prompt, AgentID: claim.AgentID, StepKind: "agent", Inputs: claim.Inputs, Comments: []cli.CollaborationRuntimeComment{}}
			_ = json.NewEncoder(w).Encode(view)
			return
		}
		fixture.serve(w, r)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal("test-owned collaboration HTTP client could not be created")
	}
	runner := &Runner{Ledger: newHostTestLedger(t), Client: client,
		Host: &HostExecutor{Config: config, Directory: filepath.Join(t.TempDir(), "workspaces"), CLIExecutable: cliExecutable,
			Logger: logger, Options: agent.ExecOptions{Timeout: 2 * time.Minute}},
		StopPollInterval: 100 * time.Millisecond, PermitAttempts: 2, PermitBackoff: 10 * time.Millisecond}
	fixture.runner, fixture.server = runner, server
	runErr := runner.Run(ctx, claim.Identity.RunID, request)
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.proofs) != 1 || fixture.proofs[0].State != "host_exit_observed" || protocol.ValidateCollaborationHostProof(fixture.proofs[0]) != nil ||
		len(fixture.permits) == 0 || len(fixture.proofs[0].Launches) != len(fixture.permits) || len(fixture.terminations) != len(fixture.permits) {
		t.Fatal("real Provider did not produce complete managed termination and original launch receipts")
	}
	for _, launch := range fixture.proofs[0].Launches {
		if launch.Kind != "managed_launch_ended" || launch.Identity != claim.Identity || fixture.permits[launch.AttemptSeq].AttemptNonce != launch.AttemptNonce {
			t.Fatal("a real Provider launch lacked its original waited lifecycle observation")
		}
	}
	if runErr != nil {
		t.Fatal("ordinary host execution failed; private Provider diagnostics are not logged by this smoke")
	}
	if contextReads.Load() == 0 {
		t.Fatal("the real Provider did not read the test-owned issue through the current Multica CLI")
	}
	if len(fixture.outcomes) != 1 || fixture.outcomes[0].Identity != claim.Identity || fixture.outcomes[0].Outcome != "success" ||
		!strings.Contains(fixture.outcomes[0].Body, expected) {
		t.Fatal("real Provider did not return the explicit successful HOST_SMOKE_OK outcome")
	}
	local, err := runner.Ledger.Lock(request.DeploymentID, request.InstallationID, claim.Identity.RunID)
	if err != nil {
		t.Fatal("original host ledger could not be reopened")
	}
	defer local.Close()
	if local.record == nil || local.record.Phase != phaseAcknowledged || local.record.Host == nil || !local.record.Host.OutcomeRecorded ||
		local.record.Proof == nil || local.record.Proof.ReceiptID != fixture.proofs[0].ReceiptID || retainedHostRun(*local.record).Unconfirmed {
		t.Fatal("managed completion or original outcome/proof ACK was not retained durably")
	}
}

func buildHostAccountSmokeCLI(t *testing.T, ctx context.Context) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(source) {
		t.Fatal("smoke requires the current repository source checkout without trimmed test paths")
	}
	module := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	name := "multica"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(t.TempDir(), name)
	build := exec.CommandContext(ctx, "go", "build", "-o", executable, "./cmd/multica")
	build.Dir, build.Stdout, build.Stderr = module, io.Discard, io.Discard
	if build.Run() != nil {
		t.Fatal("could not build the current Multica CLI for the test-owned issue")
	}
	return executable
}

func authorizeHostAccountSmokeRequest(fixture *hostRunFixture, r *http.Request) bool {
	if r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
		return false
	}
	prefix := "/api/collaboration/runtime/runs/" + fixture.claim.Identity.RunID
	if !strings.HasPrefix(r.URL.Path, prefix+"/") {
		return false
	}
	suffix := strings.TrimPrefix(r.URL.Path, prefix)
	token := r.Header.Get("Authorization")
	if r.Method == http.MethodPost && (suffix == "/claim" || suffix == "/permit") {
		return token == "Bearer mdt_fixture"
	}
	if r.Method == http.MethodGet && suffix == "/stop-intent" || r.Method == http.MethodPost && suffix == "/proof" {
		return token == "Bearer "+fixture.claim.TerminationToken
	}
	if r.Method == http.MethodPost && strings.HasPrefix(suffix, "/launches/") && strings.HasSuffix(suffix, "/termination") {
		id := strings.TrimSuffix(strings.TrimPrefix(suffix, "/launches/"), "/termination")
		parsed, err := uuid.Parse(id)
		return err == nil && parsed.String() == id && token == "Bearer "+fixture.claim.TerminationToken
	}
	if r.Method != http.MethodGet || suffix != "/context" && suffix != "/preparation" {
		if r.Method != http.MethodPost || suffix != "/outcome" && suffix != "/preparation/observations" {
			return false
		}
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	for seq := range fixture.permits {
		if token == "Bearer mcb_"+strings.Repeat(seq, 64) {
			return true
		}
	}
	return false
}
