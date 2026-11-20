package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The title/edit matrices remain in collaboration_edit_test.go. These cases
// identify the winning priority and retain the exact original-key effects.
func TestCollaborationRecoveryPriorityConcurrentIdentity(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	for _, tc := range []struct {
		name           string
		sameKey        bool
		secondPriority string
		refusal        string
	}{
		{name: "same_key_same_priority", sameKey: true, secondPriority: "high"},
		{name: "same_key_different_priority", sameKey: true, secondPriority: "low", refusal: "idempotency_conflict"},
		{name: "different_keys_same_revision", secondPriority: "low", refusal: "stale_revision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id=$1`, f.issue.SessionID)
			f.fx.Cleanup(t, `DELETE FROM collaboration_batch WHERE session_id=$1`, f.issue.SessionID)
			input := f.submit(t, f.creator, "Retained input across priority identity competition")
			f.run(t, input, 1)
			f.submit(t, f.handler, "Retained waiting input across priority identity competition")
			before := mustAccess(t, f, f.ctx)
			beforeExecution, beforeInputs := sourceRoleExecutionSnapshot(t, f), sourceAuthorityExecutionState(t, f)
			notifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID)
			events := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
			first := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, before.Revision), Priority: editString("high")}
			second := first
			second.Priority = editString(tc.secondPriority)
			if !tc.sameKey {
				second.CollaborationWorkflowRequest = workflowRequest(f, before.Revision)
			}
			ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
			defer cancel()
			lead, follow := recoveryCASCommitOrder(t, f, ctx,
				func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
					return svc.EditIssue(ctx, first)
				},
				func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
					return svc.EditIssue(ctx, second)
				})
			want := CollaborationWorkflowReceipt{Status: http.StatusOK, Code: "issue_edited", CollaborationRevision: before.Revision + 1,
				QueueState: before.QueueState, ControlVersion: before.ControlVersion}
			if lead.err != nil || !reflect.DeepEqual(lead.receipt, want) {
				t.Fatalf("leading priority did not commit once: %+v %v", lead.receipt, lead.err)
			}
			if tc.refusal == "" {
				if follow.err != nil || !reflect.DeepEqual(follow.receipt, lead.receipt) {
					t.Fatalf("concurrent identical priority key did not converge: %+v %v", follow.receipt, follow.err)
				}
			} else {
				var refusal *CollaborationError
				if !errors.As(follow.err, &refusal) || refusal.Status != http.StatusConflict || refusal.Code != tc.refusal {
					t.Fatalf("competing priority used the wrong conflict: %+v %v", follow.receipt, follow.err)
				}
				if !tc.sameKey {
					wantRefusal := want
					wantRefusal.Status, wantRefusal.Code = http.StatusConflict, tc.refusal
					if !reflect.DeepEqual(follow.receipt, wantRefusal) {
						t.Fatalf("distinct losing key did not save the observed winning revision: %+v", follow.receipt)
					}
				}
			}
			current := recoveryCASIssueState(t, f, before, "high", f.handler, 1, 0)
			winningEvent := recoveryCASPriorityEvent(t, f, first.RequestID, f.creator, before.Issue.Priority, "high", lead.receipt,
				before.EventSequence+1, current.Issue.UpdatedAt.Time)
			original := recoveryCASSavedOperation(t, f, f.creator, first.RequestID, "edit_issue", first, lead.receipt)
			originals := []db.CollaborationOperation{original}
			operationCount := 1
			if !tc.sameKey {
				loser := recoveryCASSavedOperation(t, f, f.creator, second.RequestID, "edit_issue", second, follow.receipt)
				if loser.ID == original.ID || loser.PayloadHash == original.PayloadHash ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->>'request_id'=$2`, f.issue.Issue.ID, second.RequestID) != 0 {
					t.Fatal("distinct priority loser shared the winning receipt or acquired a business event")
				}
				originals = append(originals, loser)
				operationCount++
			} else if tc.refusal != "" {
				changedDigest := sha256.Sum256(fieldJSON(t, second))
				if original.PayloadHash == hex.EncodeToString(changedDigest[:]) {
					t.Fatal("changed same-key priority replaced the original payload identity")
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND kind='edit_issue'`, f.issue.Issue.ID) != operationCount ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != events+1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != notifications {
				t.Fatal("priority competition duplicated a receipt/event or changed responsibility/notifications")
			}

			// A later explicit observation and new key may apply the other value;
			// neither a saved stale refusal nor a changed same-key request may do it.
			fresh := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, current.Revision), Priority: editString("low")}
			freshReceipt, err := f.svc.EditIssue(f.ctx, fresh)
			if err != nil || freshReceipt.Status != http.StatusOK || freshReceipt.Code != "issue_edited" || freshReceipt.CollaborationRevision != before.Revision+2 ||
				fresh.RequestID == first.RequestID || fresh.RequestID == second.RequestID {
				t.Fatalf("fresh observed priority continuation: %+v %v", freshReceipt, err)
			}
			current = recoveryCASIssueState(t, f, before, "low", f.handler, 2, 0)
			freshEvent := recoveryCASPriorityEvent(t, f, fresh.RequestID, f.creator, "high", "low", freshReceipt,
				before.EventSequence+2, current.Issue.UpdatedAt.Time)
			freshOperation := recoveryCASSavedOperation(t, f, f.creator, fresh.RequestID, "edit_issue", fresh, freshReceipt)
			if freshEvent.ID == winningEvent.ID || freshOperation.ID == original.ID {
				t.Fatal("fresh priority reused an earlier effect or operation identity")
			}
			beforeReplay, beforeHistory := sourceRoleBusinessSnapshot(t, f), recoveryCASEventHistory(t, f)
			replayed, err := f.svc.EditIssue(f.ctx, first)
			if err != nil || !reflect.DeepEqual(replayed, lead.receipt) {
				t.Fatalf("old priority success was recomputed after continuation: %+v %v", replayed, err)
			}
			replayed, err = f.svc.EditIssue(f.ctx, second)
			if tc.refusal == "" {
				if err != nil || !reflect.DeepEqual(replayed, follow.receipt) {
					t.Fatalf("old identical priority key changed after continuation: %+v %v", replayed, err)
				}
			} else if !collaborationErrorCode(err, tc.refusal) || (!tc.sameKey && !reflect.DeepEqual(replayed, follow.receipt)) {
				t.Fatalf("old priority conflict became a new edit: %+v %v", replayed, err)
			}
			for _, operation := range append(originals, freshOperation) {
				recoveryCASOwnReceipt(t, f, f.creator, operation)
			}
			if !reflect.DeepEqual(recoveryCASEventForRequest(t, f, first.RequestID), winningEvent) ||
				!reflect.DeepEqual(recoveryCASEventForRequest(t, f, fresh.RequestID), freshEvent) ||
				sourceRoleBusinessSnapshot(t, f) != beforeReplay || recoveryCASEventHistory(t, f) != beforeHistory ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution || sourceAuthorityExecutionState(t, f) != beforeInputs ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND kind='edit_issue'`, f.issue.Issue.ID) != operationCount+1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != events+2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != notifications ||
				f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1`, f.workspace) != 0 {
				t.Fatal("priority recovery changed saved effects, retained execution, queue, notifications or iteration facts")
			}
		})
	}
}

func TestCollaborationRecoveryPriorityTransferCommitOrders(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	for _, first := range []string{"priority", "transfer"} {
		t.Run(first+"_first", func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id=$1`, f.issue.SessionID)
			f.fx.Cleanup(t, `DELETE FROM collaboration_batch WHERE session_id=$1`, f.issue.SessionID)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			input := f.submit(t, f.creator, "Retained running input across priority and transfer")
			f.run(t, input, 1)
			f.submit(t, f.handler, "Retained former-handler waiting input")
			before := mustAccess(t, f, f.ctx)
			actor := mustAccess(t, f, collaborationContext(f.handler))
			if actor.SourceMember || actor.Issue.CreatorID == util.MustParseUUID(f.handler) || !actor.Capabilities.Edit || !actor.Capabilities.Transfer {
				t.Fatal("commit-order fixture must use a current external noncreator handler")
			}
			beforeExecution, beforeInputs := sourceRoleExecutionSnapshot(t, f), sourceAuthorityExecutionState(t, f)
			notifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID)
			events := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
			priority := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, before.Revision), Priority: editString("high")}
			transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
				CollaborationWorkflowRequest: workflowRequest(f, before.Revision), TargetUserID: util.MustParseUUID(f.creator)}, Note: "Explicit priority recovery handoff"}
			edit := func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
				return svc.EditIssue(ctx, priority)
			}
			handoff := func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
				return svc.Transfer(ctx, transfer)
			}
			leading, following, refusal := edit, handoff, "stale_revision"
			winningCode, losingStatus := "issue_edited", http.StatusConflict
			winningPriority, winningHandler, transfers := "high", f.handler, int64(0)
			if first == "transfer" {
				leading, following, refusal = handoff, edit, "forbidden"
				winningCode, losingStatus = "transferred", http.StatusForbidden
				winningPriority, winningHandler, transfers = before.Issue.Priority, f.creator, 1
			}
			ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 10*time.Second)
			defer cancel()
			lead, follow := recoveryCASCommitOrder(t, f, ctx, leading, following)
			want := CollaborationWorkflowReceipt{Status: http.StatusOK, Code: winningCode, CollaborationRevision: before.Revision + 1,
				QueueState: before.QueueState, ControlVersion: before.ControlVersion}
			wantRefusal := want
			wantRefusal.Status, wantRefusal.Code = losingStatus, refusal
			var denial *CollaborationError
			if lead.err != nil || !reflect.DeepEqual(lead.receipt, want) || !errors.As(follow.err, &denial) || denial.Status != losingStatus || denial.Code != refusal ||
				!reflect.DeepEqual(follow.receipt, wantRefusal) {
				t.Fatalf("priority/transfer missed the current authority and revision commit boundary: leading=%+v/%v following=%+v/%v", lead.receipt, lead.err, follow.receipt, follow.err)
			}
			current := recoveryCASIssueState(t, f, before, winningPriority, winningHandler, 1, transfers)
			editOutcome, transferOutcome := lead, follow
			winningKey, losingKey := priority.RequestID, transfer.RequestID
			var winningEvent db.CollaborationEvent
			var originalHandoff db.IssueHandoff
			if first == "transfer" {
				editOutcome, transferOutcome = follow, lead
				winningKey, losingKey = transfer.RequestID, priority.RequestID
				winningEvent, originalHandoff = recoveryCASTransferEvent(t, f, transfer.RequestID, f.handler, transfer.Note, before.Issue.Status,
					transferOutcome.receipt, before.EventSequence+1)
			} else {
				winningEvent = recoveryCASPriorityEvent(t, f, priority.RequestID, f.handler, before.Issue.Priority, "high", editOutcome.receipt,
					before.EventSequence+1, current.Issue.UpdatedAt.Time)
			}
			oldEdit := recoveryCASSavedOperation(t, f, f.handler, priority.RequestID, "edit_issue", priority, editOutcome.receipt)
			oldTransfer := recoveryCASSavedOperation(t, f, f.handler, transfer.RequestID, "transfer", transfer, transferOutcome.receipt)
			if oldEdit.ID == oldTransfer.ID || oldEdit.PayloadHash == oldTransfer.PayloadHash ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->>'request_id'=$2`, f.issue.Issue.ID, losingKey) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND kind IN ('edit_issue','transfer')`, f.issue.Issue.ID) != 2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != events+1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != int(1-transfers) ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='transfer'`, f.issue.Issue.ID) != int(transfers) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != notifications+int(transfers) ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution || sourceAuthorityExecutionState(t, f) != beforeInputs {
				t.Fatal("losing priority/transfer partially applied, shared original effects or altered retained execution")
			}
			for _, original := range []db.CollaborationOperation{oldEdit, oldTransfer} {
				recoveryCASOwnReceipt(t, f, f.handler, original)
			}

			// Reread current identity/version and deliberately submit a new key.
			// A former handler's old POST does not restore current edit authority.
			freshActor := f.handler
			if first == "transfer" {
				freshActor = f.creator
			}
			observation := mustAccess(t, f, collaborationContext(freshActor))
			freshRequest := workflowRequest(f, observation.Revision)
			freshRequest.ExpectedLifecycleEpoch = observation.LifecycleEpoch
			freshRequest.ExpectedParticipantEpoch = observation.ParticipantEpoch.Int64
			freshRequest.ExpectedAccountEpoch = observation.AccountEpoch.Int64
			var freshReceipt CollaborationWorkflowReceipt
			var freshPayload any
			freshKind, freshCode := "transfer", "transferred"
			var err error
			if first == "transfer" {
				fresh := CollaborationEditParams{CollaborationWorkflowRequest: freshRequest, Priority: editString("high")}
				freshPayload, freshKind, freshCode = fresh, "edit_issue", "issue_edited"
				freshReceipt, err = f.svc.EditIssue(collaborationContext(freshActor), fresh)
			} else {
				fresh := transfer
				fresh.CollaborationWorkflowRequest = freshRequest
				freshPayload = fresh
				freshReceipt, err = f.svc.Transfer(collaborationContext(freshActor), fresh)
			}
			wantFresh := want
			wantFresh.Code, wantFresh.CollaborationRevision = freshCode, before.Revision+2
			if err != nil || !reflect.DeepEqual(freshReceipt, wantFresh) || freshRequest.RequestID == priority.RequestID || freshRequest.RequestID == transfer.RequestID {
				t.Fatalf("fresh authorized priority/transfer continuation: %+v %v", freshReceipt, err)
			}
			current = recoveryCASIssueState(t, f, before, "high", f.creator, 2, 1)
			var freshEvent db.CollaborationEvent
			finalHandoff := originalHandoff
			if first == "transfer" {
				freshEvent = recoveryCASPriorityEvent(t, f, freshRequest.RequestID, f.creator, before.Issue.Priority, "high", freshReceipt,
					before.EventSequence+2, current.Issue.UpdatedAt.Time)
			} else {
				freshEvent, finalHandoff = recoveryCASTransferEvent(t, f, freshRequest.RequestID, f.handler, transfer.Note, before.Issue.Status,
					freshReceipt, before.EventSequence+2)
			}
			freshOperation := recoveryCASSavedOperation(t, f, freshActor, freshRequest.RequestID, freshKind, freshPayload, freshReceipt)
			if freshEvent.ID == winningEvent.ID || freshOperation.ID == oldEdit.ID || freshOperation.ID == oldTransfer.ID || !finalHandoff.ID.Valid ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='transferred'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='transferred'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification n JOIN collaboration_event e ON e.issue_id=n.issue_id AND e.event_seq=n.event_seq
					WHERE n.issue_id=$1 AND n.kind='transferred' AND n.recipient_user_id=$2 AND e.kind='transferred' AND e.payload->>'handoff_id'=$3`,
					f.issue.Issue.ID, f.creator, util.UUIDToString(finalHandoff.ID)) != 1 {
				t.Fatal("fresh continuation lost exact edit/handoff/notification identities or duplicated a business effect")
			}
			former := mustAccess(t, f, collaborationContext(f.handler))
			if former.Capabilities.Edit || former.Capabilities.Transfer || former.SourceMember || !former.Capabilities.Read {
				t.Fatal("former noncreator handler retained write authority or lost its independent read basis")
			}
			beforeReplay, beforeHistory := sourceRoleBusinessSnapshot(t, f), recoveryCASEventHistory(t, f)
			for _, write := range []func(context.Context, *CollaborationService) (CollaborationWorkflowReceipt, error){edit, handoff} {
				_, err := write(collaborationContext(f.handler), f.svc)
				var denied *CollaborationError
				if !errors.As(err, &denied) || denied.Status != http.StatusForbidden || denied.Code != "forbidden" {
					t.Fatalf("an old POST restored former-handler write authority: %v", err)
				}
			}
			for _, original := range []db.CollaborationOperation{oldEdit, oldTransfer} {
				recoveryCASOwnReceipt(t, f, f.handler, original)
				if _, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, original.RequestID); !collaborationErrorCode(err, "not_found") {
					t.Fatalf("creator recovered another actor's original operation: %v", err)
				}
			}
			recoveryCASOwnReceipt(t, f, freshActor, freshOperation)
			if !reflect.DeepEqual(recoveryCASEventForRequest(t, f, winningKey), winningEvent) ||
				!reflect.DeepEqual(recoveryCASEventForRequest(t, f, freshRequest.RequestID), freshEvent) ||
				!reflect.DeepEqual(recoveryCASHandoff(t, f, finalHandoff.ID), finalHandoff) ||
				sourceRoleBusinessSnapshot(t, f) != beforeReplay || recoveryCASEventHistory(t, f) != beforeHistory ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution || sourceAuthorityExecutionState(t, f) != beforeInputs ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND kind IN ('edit_issue','transfer')`, f.issue.Issue.ID) != 3 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != events+2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != notifications+1 ||
				f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
				t.Fatal("former-handler recovery changed original success/refusal, business history, execution, queue or membership")
			}
		})
	}
}

type recoveryCASOutcome struct {
	receipt CollaborationWorkflowReceipt
	err     error
}

// Reuse the real transaction/PID barriers. Release, cancel and join both
// goroutines even when waitWorkflowBlocked or an early assertion calls Fatal.
func recoveryCASCommitOrder(t *testing.T, f *collaborationQueueFixture, ctx context.Context,
	leading, following func(context.Context, *CollaborationService) (CollaborationWorkflowReceipt, error)) (recoveryCASOutcome, recoveryCASOutcome) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	ready, release := make(chan int, 1), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var writers sync.WaitGroup
	defer func() {
		unblock()
		cancel()
		writers.Wait()
	}()
	svc := *f.svc
	svc.TxStarter = lifecycleCommitBarrierStarter{TxStarter: f.pool, ready: ready, release: release}
	var lead, follow recoveryCASOutcome
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	writers.Go(func() {
		lead.receipt, lead.err = leading(ctx, &svc)
		firstDone <- lead.err
	})
	var blocker int
	select {
	case blocker = <-ready:
	case err := <-firstDone:
		t.Fatalf("leading priority writer failed before its locked commit: %v", err)
	case <-ctx.Done():
		t.Fatal("leading priority writer did not reach its commit barrier")
	}
	writers.Go(func() {
		follow.receipt, follow.err = following(ctx, f.svc)
		secondDone <- follow.err
	})
	waitWorkflowBlocked(t, f, blocker, secondDone)
	unblock()
	writers.Wait()
	return lead, follow
}

func recoveryCASIssueState(t *testing.T, f *collaborationQueueFixture, before CollaborationIssueAccess,
	priority, handler string, mutations, transfers int64) CollaborationIssueAccess {
	t.Helper()
	current := mustAccess(t, f, f.ctx)
	wantIssue := before.Issue
	wantIssue.Priority, wantIssue.AssigneeID = priority, util.MustParseUUID(handler)
	wantIssue.Revision, wantIssue.UpdatedAt = before.Issue.Revision+mutations, current.Issue.UpdatedAt
	if !reflect.DeepEqual(current.Issue, wantIssue) || !current.Issue.UpdatedAt.Valid || current.Revision != before.Revision+mutations ||
		current.AuthorityEpoch != before.AuthorityEpoch+transfers || current.LifecycleEpoch != before.LifecycleEpoch ||
		current.QueueState != before.QueueState || current.ControlVersion != before.ControlVersion || current.EventSequence != before.EventSequence+mutations ||
		current.InformationRequestID != before.InformationRequestID || current.ParticipantEpoch != before.ParticipantEpoch || current.AccountEpoch != before.AccountEpoch || current.AIPolicy != before.AIPolicy {
		t.Fatalf("priority/transfer changed the wrong field, revision, authority or queue: issue=%+v collaboration=%d authority=%d lifecycle=%d event=%d",
			current.Issue, current.Revision, current.AuthorityEpoch, current.LifecycleEpoch, current.EventSequence)
	}
	return current
}

func recoveryCASSavedOperation(t *testing.T, f *collaborationQueueFixture, actor, requestID, kind string, payload any, want CollaborationWorkflowReceipt) db.CollaborationOperation {
	t.Helper()
	op, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID,
		ActorUserID: util.MustParseUUID(actor), RequestID: requestID})
	if err != nil {
		t.Fatal(err)
	}
	var saved CollaborationWorkflowReceipt
	if err = json.Unmarshal(op.Result, &saved); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(fieldJSON(t, payload))
	if !op.ID.Valid || op.ScopeKind != "issue" || op.ScopeID != f.issue.Issue.ID || op.ActorUserID != util.MustParseUUID(actor) || op.RequestID != requestID || op.Kind != kind ||
		op.PayloadHash != hex.EncodeToString(digest[:]) || op.ResolvedIssueID != f.issue.Issue.ID || !op.CreatedAt.Valid || !op.CompletedAt.Valid ||
		!op.ResultCode.Valid || op.ResultCode.String != want.Code || !bytes.Equal(fieldJSON(t, saved), fieldJSON(t, want)) {
		t.Fatalf("original priority/transfer operation lost its exact identity, payload or saved result: %+v", op)
	}
	return op
}

func recoveryCASOwnReceipt(t *testing.T, f *collaborationQueueFixture, actor string, original db.CollaborationOperation) {
	t.Helper()
	var saved CollaborationWorkflowReceipt
	if err := json.Unmarshal(original.Result, &saved); err != nil {
		t.Fatal(err)
	}
	ctx := collaborationContext(actor)
	own, err := f.svc.GetWorkflowOperation(ctx, f.issue.Issue.ID, original.RequestID)
	want := CollaborationWorkflowOperationReceipt{Status: saved.Status, Code: saved.Code, CollaborationRevision: saved.CollaborationRevision}
	if err != nil || !reflect.DeepEqual(own, want) {
		t.Fatalf("actor-owned GET recomputed an original priority/transfer result: %+v %v", own, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(fieldJSON(t, own), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["status"] == nil || fields["code"] == nil || fields["collaboration_revision"] == nil {
		t.Fatalf("historical priority/transfer GET disclosed content, responsibility or execution: %s", fieldJSON(t, own))
	}
	current, err := f.svc.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID,
		ActorUserID: util.MustParseUUID(actor), RequestID: original.RequestID})
	if err != nil || !reflect.DeepEqual(current, original) {
		t.Fatalf("recovery changed an immutable original operation row/result JSON: %+v %v", current, err)
	}
}

// outbox_state is an asynchronous delivery marker, not an immutable business
// effect. Keep it out of event identity and history equality assertions.
func recoveryCASEventForRequest(t *testing.T, f *collaborationQueueFixture, requestID string) db.CollaborationEvent {
	t.Helper()
	var event db.CollaborationEvent
	f.fx.QueryRow(t, `SELECT id,issue_id,event_seq,actor_user_id,kind,payload,created_at FROM collaboration_event
		WHERE issue_id=$1 AND payload->>'request_id'=$2`, f.issue.Issue.ID, requestID).
		Scan(&event.ID, &event.IssueID, &event.EventSeq, &event.ActorUserID, &event.Kind, &event.Payload, &event.CreatedAt)
	return event
}

func recoveryCASEventHistory(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var history string
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(e)-'outbox_state' ORDER BY e.event_seq),'[]'::jsonb)::text
		FROM collaboration_event e WHERE e.issue_id=$1`, f.issue.Issue.ID).Scan(&history)
	return history
}

func recoveryCASPriorityEvent(t *testing.T, f *collaborationQueueFixture, requestID, actor, before, after string,
	receipt CollaborationWorkflowReceipt, seq int64, editedAt time.Time) db.CollaborationEvent {
	t.Helper()
	event := recoveryCASEventForRequest(t, f, requestID)
	var history struct {
		Before    map[string]json.RawMessage   `json:"before"`
		After     map[string]json.RawMessage   `json:"after"`
		RequestID string                       `json:"request_id"`
		EditedAt  time.Time                    `json:"edited_at"`
		Receipt   CollaborationWorkflowReceipt `json:"receipt"`
	}
	if err := json.Unmarshal(event.Payload, &history); err != nil {
		t.Fatal(err)
	}
	if !event.ID.Valid || event.IssueID != f.issue.Issue.ID || event.Kind != "issue_edited" || event.ActorUserID != util.MustParseUUID(actor) ||
		event.EventSeq != seq || !event.CreatedAt.Valid || history.RequestID != requestID || history.EditedAt.IsZero() || !history.EditedAt.Equal(editedAt) ||
		!reflect.DeepEqual(history.Receipt, receipt) || !bytes.Equal(history.Before["priority"], fieldJSON(t, before)) || !bytes.Equal(history.After["priority"], fieldJSON(t, after)) {
		t.Fatalf("priority history lost its exact event/actor/key/time/receipt or before/after value: %+v %s", event, event.Payload)
	}
	delete(history.Before, "priority")
	delete(history.After, "priority")
	if !reflect.DeepEqual(history.Before, history.After) {
		t.Fatalf("priority-only request changed another metadata field: %s", event.Payload)
	}
	return event
}

func recoveryCASHandoff(t *testing.T, f *collaborationQueueFixture, handoffID pgtype.UUID) db.IssueHandoff {
	t.Helper()
	var handoff db.IssueHandoff
	f.fx.QueryRow(t, `SELECT id,issue_id,actor_user_id,from_user_id,to_user_id,purpose,note,requester_user_id,parent_request_id,previous_status,resolution,created_at
		FROM issue_handoff WHERE issue_id=$1 AND id=$2`, f.issue.Issue.ID, handoffID).
		Scan(&handoff.ID, &handoff.IssueID, &handoff.ActorUserID, &handoff.FromUserID, &handoff.ToUserID, &handoff.Purpose, &handoff.Note,
			&handoff.RequesterUserID, &handoff.ParentRequestID, &handoff.PreviousStatus, &handoff.Resolution, &handoff.CreatedAt)
	return handoff
}

func recoveryCASTransferEvent(t *testing.T, f *collaborationQueueFixture, requestID, actor, note, status string,
	receipt CollaborationWorkflowReceipt, seq int64) (db.CollaborationEvent, db.IssueHandoff) {
	t.Helper()
	event := recoveryCASEventForRequest(t, f, requestID)
	var history struct {
		RequestID string                       `json:"request_id"`
		Receipt   CollaborationWorkflowReceipt `json:"receipt"`
		HandoffID string                       `json:"handoff_id"`
		TargetID  string                       `json:"target_user_id"`
	}
	if err := json.Unmarshal(event.Payload, &history); err != nil {
		t.Fatal(err)
	}
	handoffID, err := util.ParseUUID(history.HandoffID)
	if err != nil || !event.ID.Valid || event.IssueID != f.issue.Issue.ID || event.Kind != "transferred" || event.ActorUserID != util.MustParseUUID(actor) ||
		event.EventSeq != seq || !event.CreatedAt.Valid || history.RequestID != requestID || history.TargetID != f.creator || !reflect.DeepEqual(history.Receipt, receipt) {
		t.Fatalf("transfer history lost its exact event/actor/key/receipt/target/handoff: %+v %s", event, event.Payload)
	}
	handoff := recoveryCASHandoff(t, f, handoffID)
	if handoff.ActorUserID != util.MustParseUUID(actor) || handoff.FromUserID != util.MustParseUUID(f.handler) || handoff.ToUserID != util.MustParseUUID(f.creator) ||
		handoff.Purpose != "transfer" || handoff.Note != note || handoff.PreviousStatus != status || !handoff.CreatedAt.Valid || handoff.Resolution != "open" ||
		handoff.RequesterUserID.Valid || handoff.ParentRequestID.Valid {
		t.Fatalf("transfer changed directed-request facts or lost its original responsibility transition: %+v", handoff)
	}
	return event, handoff
}
