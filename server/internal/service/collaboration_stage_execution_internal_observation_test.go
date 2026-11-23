package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestStageExecutionInternalObservationPreservesInvocationBinding(t *testing.T) {
	f := newStageExecutionFixture(t)
	request, _ := f.release(t)
	f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	_, err := f.svc.PrepareStageExecution(context.Background(), request, f.source.consumer(request))
	f.svc.TxStarter = f.pool
	if err == nil || f.source.calls != 0 {
		t.Fatalf("ambiguous commit did not retain an undispatched intent: %v calls=%d", err, f.source.calls)
	}
	row, err := f.svc.Queries.GetStageExecutionIntentByApproval(f.ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: request.IssueID, ApprovalDecisionID: request.ApprovalDecisionID})
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := stageExecutionInvocation(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*StageExecutionInvocation)
	}{
		{"intent", func(inv *StageExecutionInvocation) { inv.IntentID = uuid.NewString() }},
		{"issue", func(inv *StageExecutionInvocation) { inv.IssueID = uuid.NewString() }},
		{"approval", func(inv *StageExecutionInvocation) { inv.ApprovalDecisionID = uuid.NewString() }},
		{"kind", func(inv *StageExecutionInvocation) { inv.Kind = "rollback" }},
		{"target", func(inv *StageExecutionInvocation) { inv.Target.ConditionalVersion += ".changed" }},
		{"target_digest", func(inv *StageExecutionInvocation) { inv.TargetDigest = "changed" }},
		{"artifact_digest", func(inv *StageExecutionInvocation) { inv.ArtifactDigest = "changed" }},
		{"request_key", func(inv *StageExecutionInvocation) { inv.ExternalRequestKey = uuid.NewString() }},
		{"deadline", func(inv *StageExecutionInvocation) {
			inv.DispatchExpiresAt = inv.DispatchExpiresAt.Add(time.Nanosecond)
		}},
		{"deployment", func(inv *StageExecutionInvocation) { id := uuid.NewString(); inv.DeploymentID = &id }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := &stageExecutionEntry{invocation: invocation}
			entry.consumed.Store(true)
			tc.change(&entry.invocation)
			if _, err := f.svc.recordStageInternalObservation(f.ctx, row, entry, "unknown"); !collaborationErrorCode(err, "stage_execution_conflict") {
				t.Fatalf("changed execution binding accepted: %v", err)
			}
			if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_execution_observation WHERE intent_id=$1`, row.IntentID); count != 0 {
				t.Fatalf("rejected binding wrote %d observations", count)
			}
		})
	}
	entry := &stageExecutionEntry{invocation: invocation}
	if _, err := f.svc.recordStageInternalObservation(f.ctx, row, entry, "unknown"); !collaborationErrorCode(err, "stage_execution_conflict") {
		t.Fatalf("unconsumed entry accepted: %v", err)
	}
	entry.consumed.Store(true)
	// pgx uses time.Local, while JSON decodes a zero-offset instant as UTC.
	// A distinct location reproduces that round trip on every host timezone.
	entry.invocation.DispatchExpiresAt = invocation.DispatchExpiresAt.In(time.FixedZone("fixture UTC", 0))
	receipt, err := f.svc.recordStageInternalObservation(f.ctx, row, entry, "unknown")
	if err != nil || receipt.State != "unknown" || receipt.FactID != nil || receipt.IntentID != invocation.IntentID || f.source.calls != 0 {
		t.Fatalf("same instant lost the retained internal outcome: %+v %v calls=%d", receipt, err, f.source.calls)
	}
	if entry.invocation.DispatchExpiresAt.Location().String() != "fixture UTC" {
		t.Fatal("binding comparison mutated the consumed entry")
	}
}
