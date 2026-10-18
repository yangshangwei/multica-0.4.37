package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRealtimeRuntimeTransitionsAreDurableAndReplaySafe(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "PRIVATE_INPUT_SENTINEL")
	f.resumeManaged(t)
	run, claimRequest, claim := f.claimManaged(t)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, claimRequest); err != nil {
		t.Fatal(err)
	}
	request := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, run, request); err != nil {
		t.Fatal(err)
	}
	outcome := protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: request.AttemptNonce, Outcome: "success", Body: "PRIVATE_RESULT_SENTINEL"}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	proof := managedProof(claim.Identity, request.AttemptNonce)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"runtime_prepared", "runtime_claimed", "runtime_permitted", "runtime_outcome_recorded", "runtime_ended", "execution_settled"} {
		if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind=$2 AND outbox_state='pending'`, f.issue.Issue.ID, kind); count != 1 {
			t.Fatalf("%s: durable realtime event count=%d, want 1 including retries", kind, count)
		}
	}
	if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND (payload::text LIKE '%PRIVATE_%' OR payload::text LIKE '%mcb_%' OR payload::text LIKE '%mct_%')`, f.issue.Issue.ID); count != 0 {
		t.Fatalf("runtime invalidations copied private data or credentials: %d", count)
	}
}
