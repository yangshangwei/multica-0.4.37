package service

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"testing"
)

func TestCodexKnowledgeInvocationRequiresBuiltinMatchingLocalRuntime(t *testing.T) {
	runtime := db.AgentRuntime{Provider: "codex", RuntimeMode: "local", OwnerID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, DaemonID: pgtype.Text{String: "test-daemon", Valid: true}}
	inv := protocol.TaskKnowledgeInvocation{ProtocolVersion: 1, Provider: "codex", Mode: "local", Builtin: true, ConfigurationID: "test-config"}
	if !taskKnowledgeInvocationSupported(&inv, runtime, db.Agent{}) {
		t.Fatal("local built-in Codex did not negotiate")
	}
	for _, args := range [][]byte{[]byte(`["--profile","other"]`), []byte(`["-c","model_provider=other"]`), []byte(`{"arg":true}`)} {
		if taskKnowledgeInvocationSupported(&inv, runtime, db.Agent{CustomArgs: args}) {
			t.Fatal("unbound Codex argument admitted")
		}
	}
	runtime.ProfileID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if taskKnowledgeInvocationSupported(&inv, runtime, db.Agent{}) {
		t.Fatal("custom profile admitted as built-in Codex")
	}
	runtime.ProfileID = pgtype.UUID{}
	runtime.Provider = "claude"
	if taskKnowledgeInvocationSupported(&inv, runtime, db.Agent{}) {
		t.Fatal("mismatched runtime admitted")
	}
}

func newCodexTaskKnowledgeFixture(t *testing.T) *taskKnowledgeFixture {
	t.Helper()
	f := newTaskKnowledgeFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='codex' WHERE id=$1`, f.task.RuntimeID)
	f.inv.Provider = "codex"
	return f
}

func TestCodexKnowledgeNativeExposureResumeAndWithdrawal(t *testing.T) {
	f := newCodexTaskKnowledgeFixture(t)
	native := uuid.NewString()
	first := f.recordSession(t, native)
	if first.Provider != "codex" || first.Session.Provider != "codex" {
		t.Fatal("Codex history mislabeled")
	}
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), native, "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	d := f.finalize(t, native)
	if d.Session.Generation != first.Session.Generation {
		t.Fatal("resume replaced exposed generation")
	}
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	if p.Session.NativeSessionID != native || len(p.Manifest.Entries) != 1 {
		t.Fatal("exact native exposure not prepared")
	}
	f.withdraw(t, f.candidate.CandidateID)
	if _, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d)); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("withdrawn Codex exposure authorized: %v", err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Claims[0].Attempts) != 0 {
		t.Fatal("refused next turn recorded a launch")
	}
}

func TestCodexKnowledgeExposedHistoryRefusesLegacyAndChangedConfiguration(t *testing.T) {
	f := newCodexTaskKnowledgeFixture(t)
	native := uuid.NewString()
	f.recordSession(t, native)
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), native, "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	for _, chosen := range []string{native, ""} {
		if _, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, nil, chosen); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
			t.Fatalf("legacy Codex reused private history: %v", err)
		}
	}
	f.inv.ConfigurationID = "changed-codex-account"
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, native)
	if err != nil {
		t.Fatal(err)
	}
	if prior != "" || d.Session.NativeSessionID != "" {
		t.Fatal("changed configuration reused the old native thread")
	}
}

func TestCodexCollaborationKnowledgeAuthorizesExactProviderAndRevokes(t *testing.T) {
	f := newProviderRuntimeKnowledgeFixture(t, "codex", true, true)
	if f.claim.KnowledgeContext == nil || f.claim.Preparation.Provider.Provider != "codex" {
		t.Fatal("Codex host did not negotiate knowledge")
	}
	bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil || len(bundle.Contributions) != 1 {
		t.Fatalf("prepare: %v contributions=%d", err, len(bundle.Contributions))
	}
	if _, err = f.authorize(t, bundle); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(f.proposal.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "revoke Codex contribution"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("withdrawn Codex receipt disclosed: %v", err)
	}
}

func TestCodexCollaborationKnowledgeRefusesProviderChange(t *testing.T) {
	f := newProviderRuntimeKnowledgeFixture(t, "codex", true, true)
	if f.claim.KnowledgeContext == nil {
		t.Fatal("Codex knowledge not negotiated")
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='claude' WHERE id=$1`, f.runtime.ID)
	if _, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare); err == nil {
		t.Fatal("changed provider reused Codex preparation")
	}
}
