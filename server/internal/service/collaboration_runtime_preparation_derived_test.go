package service

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationRuntimePreparationDerivedLateDeliveryDoesNotStarveHealthyRoot(t *testing.T) {
	f, members, root := newRuntimeScanMembers(t, 1)
	parentBefore, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	preparationDeliveredFile(t, f)
	healthy := preparationIndependentIssue(t, f)
	claimedHealthy := false
	refused := map[string]bool{}
	for range 4 {
		list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
		if err != nil || len(list.RunIDs) != 1 {
			t.Fatalf("one candidate while the namespace is free: %+v %v", list, err)
		}
		runID := util.MustParseUUID(list.RunIDs[0])
		run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, f.managedClaimRequest())
		if run.SessionID == healthy.issue.SessionID {
			if err != nil || claim.Identity.IssueID != util.UUIDToString(healthy.issue.Issue.ID) || claim.Inputs[0].Body != "healthy text-only issue" {
				t.Fatalf("healthy independent root could not actually claim: %v", err)
			}
			claimedHealthy = true
			break
		}
		if run.GroupID != parentBefore.GroupID || !collaborationErrorCode(err, "runtime_preparation_unsupported") {
			t.Fatalf("unexpected candidate/refusal while recovering preparation: %s %v", run.StepKind.String, err)
		}
		refused[run.StepKind.String] = true
	}
	if !claimedHealthy || !refused["member"] || !refused["continuation"] {
		t.Fatalf("unclaimed derived preparation kept the pending-derived fence forever: healthy=%v refused=%v", claimedHealthy, refused)
	}
	pending, err := f.svc.Queries.HasCollaborationRuntimePendingDerivedRun(f.ctx, db.HasCollaborationRuntimePendingDerivedRunParams{RuntimeID: f.runtime.ID, BindingID: f.binding.ID})
	if err != nil || pending {
		t.Fatalf("completed nonlaunch derived work still blocks root admission: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND authorized_attempt_seq=0 AND business_outcome IS NULL`, members[0]) != 1 {
		t.Fatal("derived refusal invented a provider result or missed its factual nonlaunch")
	}
	parentAfter, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, root)
	if err != nil || !reflect.DeepEqual(parentBefore, parentAfter) {
		t.Fatalf("derived refusal rewrote the parent's original proof: %v", err)
	}
}

func TestCollaborationRuntimePreparationDerivedRefusalPreservesLiveSibling(t *testing.T) {
	f, members, _ := newRuntimeScanMembers(t, 2)
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, members[0], f.managedClaimRequest())
	if err != nil {
		t.Fatal(err)
	}
	_, permit := permitPreparedClaim(t, f, members[0], claim)
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, members[0])
	if err != nil {
		t.Fatal(err)
	}
	capabilityBefore, err := f.svc.Queries.GetCollaborationBusinessCapability(f.ctx, db.GetCollaborationBusinessCapabilityParams{RunID: members[0], AttemptSeq: 1})
	if err != nil {
		t.Fatal(err)
	}
	targetBefore := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	preparationDeliveredFile(t, f)
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, members[1], f.managedClaimRequest()); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("unclaimed sibling preparation refusal: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND settled_at IS NOT NULL`, members[1]) != 1 {
		t.Fatal("unclaimed sibling retained an impossible preparation candidate")
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, members[0])
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refusing one unclaimed member mutated its live sibling: %v", err)
	}
	capabilityAfter, err := f.svc.Queries.GetCollaborationBusinessCapability(f.ctx, db.GetCollaborationBusinessCapabilityParams{RunID: members[0], AttemptSeq: 1})
	if err != nil || !reflect.DeepEqual(capabilityBefore, capabilityAfter) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, members[0]) != 1 {
		t.Fatalf("live sibling lost its capability or real reservation: %v", err)
	}
	if _, err = f.svc.RuntimeContext(context.Background(), members[0], permit.BusinessToken); err != nil {
		t.Fatalf("live sibling lost current business authority: %v", err)
	}
	targetAfter := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if targetAfter.QueueState != targetBefore.QueueState || targetAfter.ControlVersion != targetBefore.ControlVersion ||
		targetAfter.HoldReason != targetBefore.HoldReason || targetAfter.HoldVersion != targetBefore.HoldVersion {
		t.Fatal("derived preparation changed pause or hold intent")
	}
}

func TestCollaborationRuntimePreparationDerivedRefusalPreservesIssuedClaim(t *testing.T) {
	for _, permitted := range []bool{false, true} {
		t.Run(strconv.FormatBool(permitted), func(t *testing.T) {
			f, members, _ := newRuntimeScanMembers(t, 1)
			claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, members[0], f.managedClaimRequest())
			if err != nil {
				t.Fatal(err)
			}
			if permitted {
				permitPreparedClaim(t, f, members[0], claim)
			}
			before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, members[0])
			if err != nil {
				t.Fatal(err)
			}
			preparationDeliveredFile(t, f)
			if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, members[0], f.managedClaimRequest()); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
				t.Fatalf("late file did not refuse the legacy replay: %v", err)
			}
			after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, members[0])
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("preparation refusal changed an issued derived claim/permit: %v", err)
			}
		})
	}
}

func TestCollaborationRuntimePreparationDerivedRefusalRollsBackRequiredWrites(t *testing.T) {
	f, members, _ := newRuntimeScanMembers(t, 1)
	preparationDeliveredFile(t, f)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, eventFailure: true}
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, members[0], f.managedClaimRequest()); err == nil || collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("failed required write was reported as a factual refusal: %v", err)
	}
	f.svc.TxStarter = f.pool
	if after := controlStateSnapshot(t, f.collaborationQueueFixture); before != after {
		t.Fatal("failed derived refusal left partial proof/delegation/wakeup effects")
	}
}

func TestCollaborationRuntimePreparationDerivedEvidenceReadFailureDoesNotSettle(t *testing.T) {
	f, members, _ := newRuntimeScanMembers(t, 1)
	preparationDeliveredFile(t, f)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	readFailure := errors.New("injected temporary evidence lookup failure")
	f.svc.TxStarter = &runtimeScanFaultStarter{TxStarter: f.pool, query: "CollaborationRunHasPreparationStartEvidence",
		faults: map[pgtype.UUID]error{members[0]: readFailure}}
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, members[0], f.managedClaimRequest()); !errors.Is(err, readFailure) {
		t.Fatalf("temporary evidence failure became a committed preparation refusal: %v", err)
	}
	f.svc.TxStarter = f.pool
	if after := controlStateSnapshot(t, f.collaborationQueueFixture); before != after {
		t.Fatal("temporary evidence failure changed proof, reservation or delegation state")
	}
}

func TestCollaborationRuntimePreparationDerivedOldRequestPreservesCapableCandidate(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	registerPreparationFixture(t, f)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(target), Roster: roster}); err != nil {
		t.Fatal(err)
	}
	for _, resource := range roster {
		consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), util.MustParseUUID(resource.AgentID))
	}
	f.submitManaged(t, f.creator, "capable squad starts without files")
	f.resumeManaged(t)
	root, request, rootClaim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, rootClaim)
	receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"})
	if err != nil || len(receipt.RunIDs) != 1 {
		t.Fatalf("delegated member fixture: %v", err)
	}
	finishSquadRun(t, f, root, rootClaim, token, nonce, "success")
	preparationDeliveredFile(t, f)
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, receipt.RunIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, receipt.RunIDs[0], request); !collaborationErrorCode(err, "runtime_preparation_unsupported") {
		t.Fatalf("legacy request did not refuse a file requirement: %v", err)
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, receipt.RunIDs[0])
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("an old request settled a candidate its real runtime can prepare: %v", err)
	}
	claimPreparedCandidate(t, f, receipt.RunIDs[0])
}

func TestCollaborationRuntimePreparationJoinBeforeFirstClaimReleasesOnlyUnclaimedWork(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	original := f.submitManaged(t, f.creator, "root awaiting its first claim")
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("initial reserved root: %+v %v", list, err)
	}
	root := util.MustParseUUID(list.RunIDs[0])
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	joined := f.submitManaged(t, f.handler, "unread input queued before root claim")
	if joined.Code != "command_joined_squad" {
		t.Fatal("fixture did not queue an unread M11 continuation")
	}
	preparationDeliveredFile(t, f)
	healthy := preparationIndependentIssue(t, f)
	claimedHealthy := false
	for range 4 {
		list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
		if err != nil || len(list.RunIDs) != 1 {
			t.Fatalf("reserved root/continuation recovery: %+v %v", list, err)
		}
		claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, util.MustParseUUID(list.RunIDs[0]), f.managedClaimRequest())
		if err == nil && claim.Identity.IssueID == util.UUIDToString(healthy.issue.Issue.ID) {
			claimedHealthy = true
			break
		}
		if !collaborationErrorCode(err, "runtime_preparation_unsupported") {
			t.Fatalf("unexpected unclaimed preparation result: %v", err)
		}
	}
	if !claimedHealthy {
		t.Fatal("an unread pre-claim continuation stranded the reserved unsupported root")
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, root)
	if err != nil || after.ProofState != "never_launched_proven" || after.AuthorizedAttemptSeq != 0 || after.BusinessOutcome.Valid ||
		after.LaunchNonce != before.LaunchNonce || after.Fence != before.Fence || after.ClaimGeneration != before.ClaimGeneration {
		t.Fatalf("unclaimed root lost its exact nonlaunch identity: %v", err)
	}
	for _, input := range []struct {
		id  string
		seq int64
	}{{util.UUIDToString(original.CommandID), 1}, {util.UUIDToString(joined.CommandID), 2}} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL AND seq=$2`, input.id, input.seq) != 1 {
			t.Fatal("root/unread input was lost, reordered or falsely executed")
		}
	}
}
