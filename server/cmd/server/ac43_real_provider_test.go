//go:build agentintegration && !windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This explicitly authorized test proves runtime selection and human/grant
// provenance. Different configurations are not proof of independent billing;
// external gateway account/quota reconciliation is excluded by the user.
func TestAC43RealProviderProvenance(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("explicit real-provider authorization is required")
	}
	expectedDB := os.Getenv("AC43_DATABASE_NAME")
	if expectedDB == "" {
		t.Skip("AC43 requires its explicit task database assignment")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || !strings.HasPrefix(expectedDB, "multica_ac43_") || cfg.ConnConfig.Database != expectedDB {
		t.Fatal("AC43 requires its exact allocated database")
	}
	var actualDB string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&actualDB) != nil || actualDB != expectedDB {
		t.Fatal("AC43 database identity mismatch")
	}
	out := ac43Absolute(t, "AC43_OUT")
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal("AC43 output must be a new private directory", err)
	}
	selection := os.Getenv("AC43_SCENARIOS")
	if selection == "" {
		selection = "full"
	}
	if selection != "full" && selection != "claude_flow" {
		t.Fatal("AC43_SCENARIOS must be full or claude_flow")
	}
	cli := ac43Absolute(t, "AC43_CLI")
	configs := []*collaborationrun.HostConfig{ac43Config(t, "CLAUDE", "claude"), ac43Config(t, "CODEX", "codex")}
	evidence := map[string]any{"scope": "runtime_provenance_only", "scenario_selection": selection, "external_billing_reconciliation": "excluded_by_user", "database": actualDB, "started_at": time.Now().UTC().Format(time.RFC3339Nano), "status": "in_progress", "same_os_user": true, "os_security_isolation": false}
	ac43Write(t, filepath.Join(out, "summary.json"), evidence)
	defer func() {
		evidence["finished_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if t.Failed() {
			evidence["status"] = "failed"
		}
		ac43Write(t, filepath.Join(out, "summary.json"), evidence)
	}()
	f := newAC43Fixture(t, out, cli, configs)
	a, b := f.lanes[0], f.lanes[1]
	evidence["actors"] = map[string]string{"creator": f.creator, "original_handler": f.handler, "resource_owner": f.owner}
	evidence["lanes"] = []map[string]any{ac43LaneFacts(a), ac43LaneFacts(b)}
	ac43Write(t, filepath.Join(out, "summary.json"), evidence)
	var runs []map[string]any
	record := func(label string, lane *ac43Lane, authors []string, marker string) map[string]any {
		value := f.execute(out, cli, label, lane, f.next(lane), authors, marker)
		runs = append(runs, value)
		evidence["runs"] = runs
		ac43Write(t, filepath.Join(out, "summary.json"), evidence)
		return value
	}
	prompt := func(marker string) string {
		return "Reply with exactly " + marker + ". Do not use tools, modify files, inspect accounts or credentials, post comments, or start background work."
	}

	f.submit(f.creator, a.target, prompt("AC43_A_OK"))
	f.raw.control(false)
	firstA := record("multiple_targets_A", a, []string{f.creator}, "AC43_A_OK")

	f.raw.control(true)
	f.submit(f.creator, a.target, prompt("AC43_MIXED_OK"))
	f.submit(f.handler, a.target, prompt("AC43_MIXED_OK"))
	f.raw.control(false)
	mixed := record("two_human_authors_A", a, []string{f.creator, f.handler}, "AC43_MIXED_OK")
	ac43SameGrant(t, firstA, mixed)
	evidence["scenarios_passed"] = []string{"mixed_human_authors"}
	ac43Write(t, filepath.Join(out, "summary.json"), evidence)

	beforeHandler := f.handler
	transfer, err := f.raw.svc.Transfer(ac43Actor(beforeHandler), service.CollaborationTransferParams{CollaborationParticipantParams: service.CollaborationParticipantParams{CollaborationWorkflowRequest: f.workflow(beforeHandler), TargetUserID: util.MustParseUUID(f.creator)}, Note: "AC43 acceptance transfer preserving runtime and grant"})
	if err != nil || transfer.Code != "transferred" {
		t.Fatalf("ordinary transfer: %v code=%s", err, transfer.Code)
	}
	f.handler = f.creator
	var observedHandler string
	f.raw.base.managed.fx.QueryRow(t, "SELECT assignee_id::text FROM issue WHERE id=$1", f.raw.issue).Scan(&observedHandler)
	if observedHandler != f.handler {
		t.Fatal("ordinary transfer did not change handler")
	}
	evidence["transfer"] = map[string]any{"from": beforeHandler, "to": f.handler, "receipt": transfer}
	f.submit(f.handler, a.target, prompt("AC43_TRANSFER_OK"))
	after := record("ordinary_transfer_A", a, []string{f.handler}, "AC43_TRANSFER_OK")
	ac43SameGrant(t, firstA, after)
	evidence["scenarios_passed"] = []string{"mixed_human_authors", "ordinary_transfer"}
	ac43Write(t, filepath.Join(out, "summary.json"), evidence)
	if selection == "claude_flow" {
		evidence["status"] = "passed_partial_runtime_provenance"
		evidence["scenarios_not_run"] = []string{"multiple_targets_B", "cross_environment_squad"}
		return
	}
	f.submit(f.creator, b.target, prompt("AC43_B_OK"))
	record("multiple_targets_B", b, []string{f.creator}, "AC43_B_OK")
	evidence["scenarios_passed"] = []string{"multiple_targets", "mixed_human_authors", "ordinary_transfer"}
	ac43Write(t, filepath.Join(out, "summary.json"), evidence)

	squad := f.raw.base.managed.fx.Squad(t, "AC43 cross-environment squad", a.agent)
	f.raw.base.managed.fx.SquadMember(t, squad, "agent", b.agent)
	target := f.register(collaboration.TargetSquad, squad, []*ac43Lane{a, b})
	assignment := "On your first leader turn, use the supplied multica issue comment command to post exactly one comment: [@Member](mention://agent/" + b.agent + ") Reply with exactly AC43_MEMBER_OK, without tools or changes. Then reply AC43_DELEGATED and stop. On a continuation after the member completed, do not delegate again; reply with exactly AC43_SQUAD_OK. Do not inspect files, accounts or credentials or start background work."
	f.submit(f.creator, target, assignment)
	f.raw.control(false)
	leader := record("squad_leader_A", a, []string{f.creator}, "AC43_DELEGATED")
	member := record("squad_member_B", b, []string{f.creator}, "AC43_MEMBER_OK")
	continuation := record("squad_continuation_A", a, []string{f.creator}, "AC43_SQUAD_OK")
	if member["step_kind"] != "member" || member["parent_run_id"] != leader["run_id"] || continuation["step_kind"] != "continuation" || member["group_id"] != leader["group_id"] || continuation["group_id"] != leader["group_id"] {
		t.Fatal("cross-environment squad lineage is not proven")
	}
	evidence["status"] = "passed_runtime_provenance"
	evidence["scenarios_passed"] = []string{"multiple_targets", "mixed_human_authors", "ordinary_transfer", "cross_environment_squad"}
}

func ac43Absolute(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if !filepath.IsAbs(value) {
		t.Fatalf("%s must be absolute", key)
	}
	return value
}
func ac43Config(t *testing.T, key, provider string) *collaborationrun.HostConfig {
	t.Helper()
	executable := ac43Absolute(t, "AC43_"+key+"_EXECUTABLE")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	detected, err := agent.DetectVersion(ctx, agent.NewCommand(executable, nil))
	if err != nil {
		t.Fatalf("%s version probe failed", provider)
	}
	version, ok := agent.NormalizeHostProviderVersion(provider, detected)
	if !ok || strings.HasPrefix(version, "fixture-") || agent.CheckMinVersion(provider, version) != nil {
		t.Fatalf("%s version is unsupported", provider)
	}
	config := &collaborationrun.HostConfig{Provider: provider, ProviderVersion: version, ProviderMode: "local", AccountSource: "runtime_configuration", Executable: executable, DefaultModel: os.Getenv("AC43_" + key + "_MODEL"), Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 300}}
	if config.DefaultModel == "" {
		t.Fatalf("AC43_%s_MODEL is required", key)
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	return config
}
func ac43LaneFacts(lane *ac43Lane) map[string]any {
	return map[string]any{"name": lane.name, "runtime_id": lane.runtime, "installation_id": lane.installation, "binding_id": lane.binding.BindingID, "provider": lane.config.Provider, "version": lane.config.ProviderVersion, "model_requested": lane.config.DefaultModel, "capability_digest": lane.request.CapabilityDigest}
}
func ac43Write(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		err = os.WriteFile(path, append(raw, '\n'), 0600)
	}
	if err != nil {
		t.Fatal("write AC43 evidence:", err)
	}
}
func ac43SameGrant(t *testing.T, a, b map[string]any) {
	t.Helper()
	for _, key := range []string{"runtime_id", "installation_id", "binding_id", "binding_epoch", "grant_id", "grant_epoch", "resource_epoch", "resource_owner_id", "provider", "capability_digest", "config_version", "invocation_sha256"} {
		if !reflect.DeepEqual(a[key], b[key]) {
			t.Fatalf("grant/configuration changed at %s", key)
		}
	}
}

func (f *ac43Fixture) execute(out, cli, label string, lane *ac43Lane, run string, authors []string, marker string) map[string]any {
	t := f.raw.t
	evidence := map[string]any{"label": label, "run_id": run, "started_at": time.Now().UTC().Format(time.RFC3339Nano), "status": "in_progress", "provider": lane.config.Provider, "capability_digest": lane.request.CapabilityDigest}
	evidencePath := filepath.Join(out, label+".json")
	ac43Write(t, evidencePath, evidence)
	defer func() {
		evidence["finished_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if t.Failed() {
			evidence["status"] = "failed"
		}
		ac43Write(t, evidencePath, evidence)
	}()
	ledger, workspace := ac43ExecutionPaths(t, out, lane, run)
	logFile, err := os.OpenFile(filepath.Join(out, "host-"+run+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	host := &collaborationrun.HostExecutor{Config: lane.config, Directory: workspace, CLIExecutable: cli, Logger: slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})), Options: agent.ExecOptions{Timeout: 5 * time.Minute}}
	runner := &collaborationrun.Runner{Ledger: ledger, Client: lane.client, Host: host, StopPollInterval: 100 * time.Millisecond, PermitAttempts: 2, PermitBackoff: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
	defer cancel()
	runErr := func() error {
		restore := ac43WithoutTestEnvironment()
		defer restore()
		return runner.Run(ctx, run, lane.request)
	}()
	calls := f.raw.trace.snapshot()
	row, err := f.raw.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]any{"runtime_id": util.UUIDToString(row.RuntimeID), "installation_id": util.UUIDToString(row.InstallationID), "binding_id": util.UUIDToString(row.BindingID), "grant_id": util.UUIDToString(row.GrantID), "grant_epoch": row.GrantEpoch, "binding_epoch": row.BindingEpoch, "resource_epoch": row.ResourceEpoch, "resource_owner_id": util.UUIDToString(row.ResourceOwnerUserID), "agent_id": util.UUIDToString(row.AgentID), "group_id": util.UUIDToString(row.GroupID), "parent_run_id": util.UUIDToString(row.ParentRunID), "step_kind": row.StepKind.String, "progress": row.ProgressState, "proof": row.ProofState, "business_outcome": row.BusinessOutcome.String, "authorized_attempts": row.AuthorizedAttemptSeq, "sealed_attempts": row.SealedAttemptSeq.Int64, "config_version": row.ConfigVersion.Int64} {
		evidence[k] = v
	}
	var issueCreator, issueHandler string
	f.raw.base.managed.fx.QueryRow(t, "SELECT creator_id::text, assignee_id::text FROM issue WHERE id=$1", f.raw.issue).Scan(&issueCreator, &issueHandler)
	evidence["issue_creator_id"], evidence["current_handler_id"] = issueCreator, issueHandler
	if issueCreator != f.creator || issueHandler != f.handler {
		t.Fatal("persisted issue actor identity mismatch")
	}
	var claim *protocol.CollaborationRunClaimResponse
	var outcome *protocol.CollaborationRunOutcome
	var proof *protocol.CollaborationRunProof
	var selected []rk1Call
	prefix := "/api/collaboration/runtime/runs/" + run + "/"
	for _, call := range calls {
		if !strings.HasPrefix(call.Path, prefix) {
			continue
		}
		selected = append(selected, call)
		switch strings.TrimPrefix(call.Path, prefix) {
		case "claim":
			if call.Status == 200 {
				var value protocol.CollaborationRunClaimResponse
				if json.Unmarshal(call.raw, &value) == nil {
					claim = &value
				}
			}
		case "outcome":
			if call.Status == 200 {
				var value protocol.CollaborationRunOutcome
				if json.Unmarshal(call.Request, &value) == nil {
					outcome = &value
				}
			}
		case "proof":
			if call.Status == 200 {
				var value protocol.CollaborationRunProof
				if json.Unmarshal(call.Request, &value) == nil {
					proof = &value
				}
			}
		}
	}
	ac43Write(t, filepath.Join(out, label+"-http.private.json"), selected)
	if claim != nil {
		evidence["inputs"] = claim.Inputs
		if claim.HostContext != nil {
			evidence["agent_owner_id"] = claim.HostContext.AgentOwnerUserID
		}
		if claim.Preparation != nil {
			raw, _ := json.Marshal(claim.Preparation.Invocation)
			sum := sha256.Sum256(raw)
			evidence["invocation_sha256"] = hex.EncodeToString(sum[:])
		}
	}
	if outcome != nil {
		evidence["answer"] = outcome.Body
		if outcome.Record != nil {
			evidence["usage"] = outcome.Record.Usage
			evidence["record_completeness"] = outcome.Record.Completeness
		}
	}
	evidence["proof_receipt"] = proof
	if runErr != nil {
		evidence["runner_failed"] = true
		ac43Write(t, evidencePath, evidence)
		t.Fatalf("%s runner failed; private diagnostic retained", label)
	}
	if claim == nil || outcome == nil || proof == nil || row.ProgressState != "settled" || row.ProofState != "host_exit_observed" || row.BusinessOutcome.String != "success" || row.SealedAttemptSeq.Int64 != row.AuthorizedAttemptSeq || row.AuthorizedAttemptSeq < 1 {
		t.Fatalf("%s lifecycle incomplete", label)
	}
	if evidence["runtime_id"] != lane.runtime || evidence["installation_id"] != lane.installation || evidence["binding_id"] != lane.binding.BindingID || evidence["agent_id"] != lane.agent || evidence["resource_owner_id"] != f.owner || evidence["agent_owner_id"] != f.owner || claim.ResourceOwnerID != f.owner {
		t.Fatalf("%s executing identity mismatch", label)
	}
	got := make([]string, 0, len(claim.Inputs))
	for _, input := range claim.Inputs {
		got = append(got, input.AuthorUserID)
	}
	sort.Strings(got)
	want := append([]string(nil), authors...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s author provenance mismatch: got %v want %v", label, got, want)
	}
	if strings.TrimSpace(outcome.Body) != marker {
		t.Fatalf("%s did not return its assigned marker", label)
	}
	var resources []service.CollaborationRunRecordItem
	cursor := ""
	for {
		page, err := f.raw.svc.RunRecords(service.WithCollaborationExecutionViewVersion(ac43Actor(f.creator), 6), util.MustParseUUID(f.raw.issue), util.MustParseUUID(run), service.CollaborationRunRecordsQuery{Limit: 100, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if page.Resource.RuntimeID != lane.runtime || page.Resource.ResourceOwnerUserID != f.owner || page.Resource.AgentOwnerUserID != f.owner || page.Resource.GrantID != evidence["grant_id"] || page.Resource.Provider == nil || *page.Resource.Provider != lane.config.Provider || page.Resource.AccountSource == nil || *page.Resource.AccountSource != "runtime_configuration" {
			t.Fatalf("%s public resource provenance mismatch", label)
		}
		evidence["public_resource"] = page.Resource
		evidence["public_usage"] = page.Usage
		for _, item := range page.Items {
			if item.Kind == "input" {
				resources = append(resources, item)
			}
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	got = nil
	for _, item := range resources {
		got = append(got, item.AuthorUserID)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s public record author provenance mismatch", label)
	}
	evidence["public_inputs"] = resources
	evidence["status"] = "passed_runtime_provenance"
	return evidence
}

// The fixture-only contract never resolves or invokes an installed agent. It
// proves real enrollment, creator/owner separation, grants and transfer before
// consuming any provider calls.
func TestAC43FixtureContract(t *testing.T) {
	expectedDB := os.Getenv("AC43_DATABASE_NAME")
	if expectedDB == "" {
		t.Skip("AC43 requires its explicit task database assignment")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || !strings.HasPrefix(expectedDB, "multica_ac43_") || cfg.ConnConfig.Database != expectedDB {
		t.Fatal("AC43 fixture requires its exact allocated database")
	}
	var actual string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&actual) != nil || actual != expectedDB {
		t.Fatal("AC43 fixture database mismatch")
	}
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("fixture must not enable real calls")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-provider")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 70\n"), 0700); err != nil {
		t.Fatal(err)
	}
	configs := []*collaborationrun.HostConfig{}
	for _, provider := range []string{"claude", "codex"} {
		configs = append(configs, &collaborationrun.HostConfig{Provider: provider, ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "runtime_configuration", Executable: fake, DefaultModel: "fixture-model", Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 300}})
	}
	f := newAC43Fixture(t, dir, fake, configs)
	for _, lane := range f.lanes {
		f.submit(f.creator, lane.target, "fixture identity only")
	}
	f.raw.control(false)
	for _, lane := range f.lanes {
		run := f.next(lane)
		ledger, workspace := ac43ExecutionPaths(t, dir, lane, run)
		locked, err := ledger.Lock(f.raw.base.deployment, lane.installation, run)
		if err != nil {
			t.Fatal(err)
		}
		if err := locked.Close(); err != nil {
			t.Fatal(err)
		}
		host := &collaborationrun.HostExecutor{Config: lane.config, Directory: workspace, CLIExecutable: fake}
		if err := host.Probe(t.Context()); err != nil {
			t.Fatal("fixture host path probe", err)
		}
		for _, path := range []string{filepath.Join(dir, "private-ledger"), filepath.Join(dir, "private-ledger", run), filepath.Join(dir, lane.name), workspace} {
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
				t.Fatal("execution directory is not private", err)
			}
		}
		row, err := f.raw.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
		if err != nil {
			t.Fatal(err)
		}
		if util.UUIDToString(row.RuntimeID) != lane.runtime || util.UUIDToString(row.InstallationID) != lane.installation || util.UUIDToString(row.ResourceOwnerUserID) != f.owner || row.GrantEpoch < 1 {
			t.Fatal("fixture runtime or grant mismatch")
		}
	}
	f.raw.control(true)
	transfer, err := f.raw.svc.Transfer(ac43Actor(f.handler), service.CollaborationTransferParams{CollaborationParticipantParams: service.CollaborationParticipantParams{CollaborationWorkflowRequest: f.workflow(f.handler), TargetUserID: util.MustParseUUID(f.creator)}})
	if err != nil || transfer.Code != "transferred" {
		t.Fatalf("fixture transfer: %v %s", err, transfer.Code)
	}
	a, b := f.lanes[0], f.lanes[1]
	squad := f.raw.base.managed.fx.Squad(t, "AC43 fixture squad", a.agent)
	f.raw.base.managed.fx.SquadMember(t, squad, "agent", b.agent)
	_ = f.register(collaboration.TargetSquad, squad, []*ac43Lane{a, b})
	t.Log("AC43 fixture: three humans, two native installations/runtimes, grants, transfer and cross-environment squad registration verified; no provider launch")
}

// Ledger/Host Probe create their own leaf directory, not missing ancestors.
// The task owns these parents and creates them before invoking production code.
func ac43ExecutionPaths(t *testing.T, out string, lane *ac43Lane, run string) (*collaborationrun.Ledger, string) {
	t.Helper()
	for _, parent := range []string{filepath.Join(out, "private-ledger"), filepath.Join(out, lane.name)} {
		if err := os.MkdirAll(parent, 0700); err != nil {
			t.Fatal("prepare private execution parent", err)
		}
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
			t.Fatal("execution parent must be a private directory", err)
		}
	}
	ledger, err := collaborationrun.OpenLedger(filepath.Join(out, "private-ledger", run))
	if err != nil {
		t.Fatal(err)
	}
	return ledger, filepath.Join(out, lane.name, "workspaces")
}
