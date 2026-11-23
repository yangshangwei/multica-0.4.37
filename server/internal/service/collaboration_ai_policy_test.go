package service

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A policy event may advance event order; assisted must preserve every other
// execution fact rather than silently applying an old Resume observation.
func manualPolicyExecutionSnapshot(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(controlStateSnapshot(t, f)), &snapshot); err != nil {
		t.Fatal(err)
	}
	delete(snapshot, "events")
	var session map[string]json.RawMessage
	if err := json.Unmarshal(snapshot["session"], &session); err != nil {
		t.Fatal(err)
	}
	delete(session, "event_seq")
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	snapshot["session"] = encoded
	encoded, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestCollaborationManualPolicyTransitionRequiresInitiatorAndOriginalKey(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.handler, "Existing input remains waiting across policy changes")
	f.resumeManaged(t)
	p := CollaborationAIPolicyParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), AIPolicy: CollaborationAIPolicyManual}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	for _, actor := range []context.Context{collaborationPasswordContext(util.MustParseUUID(f.handler), 1), f.daemonCtx} {
		if _, err := f.svc.ChangeAIPolicy(actor, p); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("noninitiator or machine changed task policy: %v", err)
		}
		if controlStateSnapshot(t, f.collaborationQueueFixture) != before || mustAccess(t, f.collaborationQueueFixture, f.ctx).AIPolicy != CollaborationAIPolicyAssisted {
			t.Fatal("denied policy operation changed the task or execution")
		}
	}
	changed, err := f.svc.ChangeAIPolicy(f.ctx, p)
	if err != nil || changed.Code != "ai_policy_changed" || changed.CollaborationRevision != 2 || changed.QueueState != "paused" {
		t.Fatalf("initiator manual transition failed: %+v %v", changed, err)
	}
	access := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	if access.AIPolicy != CollaborationAIPolicyManual || access.Capabilities.SubmitCommand || !access.Capabilities.ControlAI || !access.Capabilities.ChangeAIPolicy ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE session_id=$1 AND queue_state <> 'paused'`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1 AND queue_state <> 'paused'`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != 1 {
		t.Fatal("manual transition lost human control, waiting input or the issue/target pause")
	}
	after := controlStateSnapshot(t, f.collaborationQueueFixture)
	replayed, err := f.svc.ChangeAIPolicy(f.ctx, p)
	if err != nil || !reflect.DeepEqual(replayed, changed) || controlStateSnapshot(t, f.collaborationQueueFixture) != after {
		t.Fatalf("original policy key repeated Stop or changed its receipt: %+v %v", replayed, err)
	}
	conflict := p
	conflict.AIPolicy = CollaborationAIPolicyAssisted
	if _, err = f.svc.ChangeAIPolicy(f.ctx, conflict); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed policy reused an original request key: %v", err)
	}
	noop := p
	noop.RequestID, noop.ExpectedCollaborationRevision = uuid.NewString(), 2
	r, err := f.svc.ChangeAIPolicy(f.ctx, noop)
	if err != nil || r.Code != "ai_policy_unchanged" || r.CollaborationRevision != 2 || controlStateSnapshot(t, f.collaborationQueueFixture) != after {
		t.Fatalf("explicit policy no-op changed controls or event history: %+v %v", r, err)
	}
	resume := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
	if _, err = f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "task_manual") {
		t.Fatalf("actual manual transition still allowed Resume: %v", err)
	}
	beforeAssisted := manualPolicyExecutionSnapshot(t, f.collaborationQueueFixture)
	assisted := noop
	assisted.RequestID, assisted.AIPolicy = uuid.NewString(), CollaborationAIPolicyAssisted
	r, err = f.svc.ChangeAIPolicy(f.ctx, assisted)
	if err != nil || r.Code != "ai_policy_changed" || r.CollaborationRevision != 3 || manualPolicyExecutionSnapshot(t, f.collaborationQueueFixture) != beforeAssisted {
		t.Fatalf("restoring assisted changed execution intent: %+v %v", r, err)
	}
	if list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("assisted policy launched waiting input without a fresh Resume: %+v %v", list, err)
	}
	if _, err = f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "task_manual") {
		t.Fatalf("old manual refusal became a deferred Resume: %v", err)
	}
}

func TestCollaborationManualPolicyPermittedRunPreservesOriginalProof(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "Input with a real server-issued permit")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	permit := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, permit); err != nil {
		t.Fatal(err)
	}
	waiting := f.submitManaged(t, f.handler, "Follow-up retained for a later explicit Resume")
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	var waitingBefore, waitingAfter, reservationBefore, reservationAfter string
	f.fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_command c WHERE id=$1`, waiting.CommandID).Scan(&waitingBefore)
	f.fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(r) ORDER BY r.run_id)::text FROM collaboration_runtime_reservation r WHERE run_id=$1`, runID).Scan(&reservationBefore)
	p := CollaborationAIPolicyParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), AIPolicy: CollaborationAIPolicyManual}
	if _, err = f.svc.ChangeAIPolicy(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_command c WHERE id=$1`, waiting.CommandID).Scan(&waitingAfter)
	f.fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(r) ORDER BY r.run_id)::text FROM collaboration_runtime_reservation r WHERE run_id=$1`, runID).Scan(&reservationAfter)
	if after.Fence != before.Fence || after.LaunchNonce != before.LaunchNonce || after.GroupID != before.GroupID || after.CommandID != before.CommandID ||
		after.TargetID != before.TargetID || after.ChannelID != before.ChannelID || after.BatchID != before.BatchID || after.InstallationID != before.InstallationID ||
		after.BindingID != before.BindingID || after.AuthorizedAttemptSeq != 1 || after.ProofState != "unknown" || after.SettledAt.Valid ||
		after.ForcedDisposition.String != "stopped" || after.DispositionReason.String != "task_manual" || waitingAfter != waitingBefore || reservationAfter != reservationBefore {
		t.Fatal("manual Stop fabricated settlement or changed original proof/input/reservation identity")
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), runID, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("manual Stop lost the original termination identity: %+v %v", intent, err)
	}
	// This is synthetic protocol proof, not evidence of a native process exit.
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatalf("manual policy rejected the original sealed proof: %v", err)
	}
	f.fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_command c WHERE id=$1`, waiting.CommandID).Scan(&waitingAfter)
	if waitingAfter != waitingBefore || mustAccess(t, f.collaborationQueueFixture, f.ctx).QueueState != "paused" {
		t.Fatal("proof resumed or rewrote waiting input")
	}
	p.RequestID, p.ExpectedCollaborationRevision, p.AIPolicy = uuid.NewString(), 2, CollaborationAIPolicyAssisted
	if _, err = f.svc.ChangeAIPolicy(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("assisted restored execution before explicit Resume: %+v %v", list, err)
	}
	f.resumeManaged(t)
	nextID, _, next := f.claimManaged(t)
	if nextID == runID || next.Identity.BatchID != util.UUIDToString(waiting.BatchID) || next.Identity.LaunchNonce == claim.Identity.LaunchNonce {
		t.Fatal("fresh Resume revived the old run instead of consuming retained follow-up")
	}
}

func TestCollaborationManualPolicyRequiredWritesRollBackTogether(t *testing.T) {
	for _, query := range []string{"UpdateCollaborationAIPolicy", "PauseCollaborationChannel", "CreateCollaborationEvent", "CompleteCollaborationOperation"} {
		t.Run(query, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "Retained input")
			f.resumeManaged(t)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			p := CollaborationAIPolicyParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), AIPolicy: CollaborationAIPolicyManual}
			f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: query}
			if _, err := f.svc.ChangeAIPolicy(f.ctx, p); err == nil {
				t.Fatalf("required %s failure was ignored", query)
			}
			if controlStateSnapshot(t, f.collaborationQueueFixture) != before || mustAccess(t, f.collaborationQueueFixture, f.ctx).AIPolicy != CollaborationAIPolicyAssisted ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("failed policy transaction committed partial policy, Stop or receipt")
			}
		})
	}
}

// Persisting the policy with otherwise-active execution deliberately represents
// an old queue observation. Admission must read the locked task policy itself.
func seedManualCollaborationPolicy(t *testing.T, f *collaborationQueueFixture) {
	t.Helper()
	if err := f.svc.Queries.UpdateCollaborationAIPolicy(f.ctx, db.UpdateCollaborationAIPolicyParams{IssueID: f.issue.Issue.ID, AiPolicy: "manual"}); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationManualPolicyBlocksExplicitInputAndResume(t *testing.T) {
	for _, action := range []string{"input", "resume"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.submit(t, f.creator, "retained waiting input")
			gate := enableControlGate(t, f.collaborationFixture)
			seedManualCollaborationPolicy(t, f)
			before := controlStateSnapshot(t, f)
			var receipt CollaborationQueueReceipt
			var err error
			if action == "input" {
				receipt, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
					InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "must not become AI input"})
			} else {
				receipt, err = f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: gate})
			}
			if !collaborationErrorCode(err, "task_manual") || receipt.Status != http.StatusConflict || receipt.Code != "task_manual" {
				t.Fatalf("manual %s must return its explicit durable refusal: %+v %v", action, receipt, err)
			}
			if controlStateSnapshot(t, f) != before {
				t.Fatal("manual admission refusal changed retained execution or inputs")
			}
		})
	}
}

func TestCollaborationManualPolicyFencesRuntimeAdmission(t *testing.T) {
	for _, action := range []string{"promotion", "claim", "permit"} {
		t.Run(action, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "admitted before the policy changes")
			f.resumeManaged(t)
			if action == "promotion" {
				seedManualCollaborationPolicy(t, f.collaborationQueueFixture)
				list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
				if err != nil || len(list.RunIDs) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
					t.Fatalf("manual task promoted input from a stale active queue: %+v %v", list, err)
				}
				return
			}
			runID, request, claim := f.claimManaged(t)
			seedManualCollaborationPolicy(t, f.collaborationQueueFixture)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			if action == "claim" {
				_, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
				if !collaborationErrorCode(err, "task_manual") {
					t.Fatalf("manual task allowed current runtime claim: %v", err)
				}
			} else {
				_, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity,
					AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
				if !collaborationErrorCode(err, "task_manual") {
					t.Fatalf("manual task issued a launch permit: %v", err)
				}
			}
			if !reflect.DeepEqual(controlStateSnapshot(t, f.collaborationQueueFixture), before) {
				t.Fatal("refused runtime admission changed the retained execution ledger")
			}
		})
	}
}
