package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

type taskKnowledgeFixture struct {
	*collaborationQueueFixture
	tasks     *TaskService
	task      db.AgentTaskQueue
	caller    TaskKnowledgeCaller
	inv       protocol.TaskKnowledgeInvocation
	candidate protocol.CollaborationKnowledgeProposal
}

func newTaskKnowledgeFixture(t *testing.T) *taskKnowledgeFixture {
	t.Helper()
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
		t.Fatal(err)
	}
	daemon := "knowledge-" + uuid.NewString()
	runtime := f.fx.Runtime(t, "Knowledge runtime", testutil.Cols{"provider": "claude", "runtime_mode": "local", "daemon_id": daemon})
	agentID := f.fx.Agent(t, "Knowledge agent", runtime, testutil.Cols{"runtime_mode": "local"})
	issue := f.fx.Issue(t, "Ordinary consumer", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
	id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1, "originator_user_id": f.creator, "accountable_user_id": f.creator, "max_attempts": 3})
	task, err := f.svc.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("mdt_" + uuid.NewString())
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": hash, "workspace_id": f.workspace, "daemon_id": daemon, "user_id": f.creator, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	tasks:=NewTaskService(f.svc.Queries,f.pool,nil,events.New())
	tasks.KnowledgeContextEnabled=true
	return &taskKnowledgeFixture{collaborationQueueFixture: f, tasks: tasks, task: task, caller: TaskKnowledgeCaller{RuntimeID: task.RuntimeID, Kind: "daemon_token", TokenHash: hash}, inv: protocol.TaskKnowledgeInvocation{ProtocolVersion: 1, Provider: "claude", Mode: "local", Builtin: true, ConfigurationID: "configured-runtime"}, candidate: p}
}

func (f *taskKnowledgeFixture) finalize(t *testing.T, chosen string) *protocol.TaskKnowledgeClaimDescriptor {
	t.Helper()
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, chosen)
	if err != nil {
		t.Fatal(err)
	}
	if chosen != "" && prior == "" {
		t.Fatal("fixture unexpectedly retired session")
	}
	_, err = f.tasks.FinalizeTaskClaim(f.ctx, f.task, db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_" + uuid.NewString()), TaskID: f.task.ID, AgentID: f.task.AgentID, WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(f.creator), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, d)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *taskKnowledgeFixture) prepareRequest(d *protocol.TaskKnowledgeClaimDescriptor) protocol.TaskKnowledgePrepareRequest {
	return protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)}
}
func (f *taskKnowledgeFixture) authorizeRequest(p protocol.TaskKnowledgePrepareResponse, d *protocol.TaskKnowledgeClaimDescriptor) protocol.TaskKnowledgeAuthorizeRequest {
	return protocol.TaskKnowledgeAuthorizeRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, ManifestSHA256: p.ManifestSHA256, InvocationFingerprint: p.InvocationFingerprint, PromptSHA256: strings.Repeat("b", 64), AttemptSequence: 1, AttemptNonce: uuid.NewString(), Session: d.Session}
}
func (f *taskKnowledgeFixture) observe(t *testing.T, d *protocol.TaskKnowledgeClaimDescriptor, a protocol.TaskKnowledgeAuthorization, kind, native string) {
	t.Helper()
	req := protocol.TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: a.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: a.AttemptSequence, AttemptNonce: a.AttemptNonce, Kind: kind}
	switch kind {
	case "started":
		req.ProcessID = 42
		req.ProcessInstanceID = "test-owned-process"
	case "native_session":
		req.NativeSessionID = native
		req.Source = "system"
	case "terminated":
		req.TerminationKind = "managed_launch_ended"
		req.WaitObserved = true
	}
	if err := f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeClaimFinalizationDisclosesNoPrivateFields(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_KNOWLEDGE", f.candidate.CandidateID, util.UUIDToString(f.issue.Issue.ID), "manifest", "contributions", "content_sha256"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("source-free descriptor contains %s", secret)
		}
	}
	task, err := f.svc.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Claims) != 1 || meta.Claims[0].Prepared != nil {
		t.Fatal("finalization disclosed knowledge or skipped non-comment negotiation")
	}
}

func TestKnowledgePrepareBeforeFinalizedClaimRefused(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err == nil || len(out.Contributions) != 0 {
		t.Fatal("unfinalized claim disclosed a private contribution")
	}
}

func TestKnowledgePreparationLostResponseReauthorizes(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	req := f.prepareRequest(d)
	first, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Contributions) != 1 || first.Contributions[0].Content != f.candidate.Content {
		t.Fatalf("eligible preparation: %+v", first)
	}
	replayed, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(replayed)
	if string(a) != string(b) {
		t.Fatal("same-key preparation changed its frozen response")
	}
	_, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(f.candidate.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "current authority changed"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if !collaborationErrorCode(err, "knowledge_context_changed") || len(after.Contributions) != 0 {
		t.Fatalf("lost response replay bypassed withdrawal: %+v %v", after, err)
	}
}

func TestKnowledgePreparationRejectsOldClaimGeneration(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	req := f.prepareRequest(d)
	f.fx.Exec(t, `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`, f.task.ID)
	out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err == nil || len(out.Contributions) != 0 {
		t.Fatal("old claim generation disclosed knowledge")
	}
}

func TestKnowledgeSessionPointerAndExposureBindAtomically(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if err != nil {
		t.Fatal(err)
	}
	f.observe(t, d, a, "started", "")
	if err := ValidateTaskKnowledgeSessionPointer(f.ctx, f.tasks.Queries, f.task.ID, "unobserved"); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("unobserved pointer accepted: %v", err)
	}
	f.observe(t, d, a, "native_session", "exact-native")
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	if row.SessionID.String != "exact-native" || meta.Claims[0].Attempts[0].BoundSessionID != "exact-native" || meta.Claims[0].Prepared.ManifestSHA256 != p.ManifestSHA256 {
		t.Fatal("pointer became visible without its exact exposure")
	}
	f.observe(t, d, a, "terminated", "")
}

func TestKnowledgeLegacyClientCannotResumeManagedSession(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if err != nil {
		t.Fatal(err)
	}
	f.observe(t, d, a, "started", "")
	f.observe(t, d, a, "native_session", "managed-native")
	for _, chosen := range []string{"managed-native", ""} {
		_, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, nil, chosen)
		if !collaborationErrorCode(err, "knowledge_invocation_unsupported") || prior != "" {
			t.Fatalf("old client reused or implicitly continued managed history: %q %v", prior, err)
		}
	}
}

func TestKnowledgeProvenLegacySessionRemainsResumable(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.fx.Exec(t, `UPDATE agent_task_queue SET session_id='legacy-native' WHERE id=$1`, f.task.ID)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, nil, "legacy-native")
	if err != nil || d != nil || prior != "legacy-native" {
		t.Fatalf("legacy session was reset: %q %v", prior, err)
	}
}

func TestKnowledgeAuthorityChangeBetweenDisclosureAndLaunch(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	_, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if !collaborationErrorCode(err, "knowledge_authorization_denied") {
		t.Fatalf("disabled runtime owner authorized launch: %v", err)
	}
	if f.fx.Count(t, `SELECT jsonb_array_length(knowledge_delivery->'claims'->0->'attempts') FROM agent_task_queue WHERE id=$1`, f.task.ID) != 0 {
		t.Fatal("denied launch consumed an authorization")
	}
}

func TestKnowledgePreparationWorkspaceDeleteLockOrder(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`, f.workspace); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()
	_, err = f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if !collaborationErrorCode(err, "knowledge_busy") {
		t.Fatalf("workspace conflict waited or imported reverse locks: %v", err)
	}
	if _, err = tx.Exec(f.ctx, `SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE NOWAIT`, f.task.ID); err != nil {
		t.Fatalf("preparation retained task before parent refusal: %v", err)
	}
}

func TestKnowledgePreparationFinalizerContentionUsesNowait(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, `SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE`, f.task.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()
	_, err = f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if !collaborationErrorCode(err, "knowledge_busy") {
		t.Fatalf("late task conflict waited: %v", err)
	}
	if _, err = tx.Exec(f.ctx, `SELECT id FROM workspace WHERE id=$1 FOR UPDATE NOWAIT`, f.workspace); err != nil {
		t.Fatalf("preparation retained parent lock after task refusal: %v", err)
	}
}

func TestKnowledgeFailureNeverCreatesAutomaticRetry(t *testing.T) {
	for _, reason := range []taskfailure.Reason{taskfailure.ReasonKnowledgeContextChanged, taskfailure.ReasonKnowledgeAuthorizationDenied, taskfailure.ReasonKnowledgeUnavailable, taskfailure.ReasonKnowledgeBusy, taskfailure.ReasonKnowledgeInvocationUnsupported} {
		t.Run(reason.String(), func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			row, err := f.tasks.FailTask(f.ctx, f.task.ID, reason.String(), "", "", "", taskfailure.ReasonAgentProviderNetwork.String(), false, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if row.FailureReason.String != reason.String() {
				t.Fatal("normalization replaced a terminal knowledge reason")
			}
			child, err := f.tasks.MaybeRetryFailedTask(f.ctx, *row)
			if err != nil || child != nil {
				t.Fatalf("knowledge refusal retried: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE parent_task_id=$1`, f.task.ID) != 0 {
				t.Fatal("failure created automatic input replay")
			}
		})
	}
}
