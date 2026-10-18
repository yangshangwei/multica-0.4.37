package service

import (
	"context"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationSquadNonlaunchCannotReturnInputReadByLaterRun(t *testing.T) {
	f, memberRuntime, member := newManagedSquadResourcesFixture(t, true)
	f.submitManaged(t, f.creator, "root instruction")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") review the result",
	})
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("member dispatch: %+v %v", dispatch, err)
	}
	joined := f.submitManaged(t, f.handler, "include this followup")
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	continuation, err := f.svc.Queries.GetCollaborationQueuedContinuation(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
	if err != nil {
		t.Fatal(err)
	}
	firstClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	memberRun := dispatch.RunIDs[0]
	claimedMember, _, memberClaim := memberRuntime.claimManaged(t)
	if claimedMember != memberRun {
		t.Fatal("member claim changed the delegated run")
	}
	found := false
	for _, input := range memberClaim.Inputs {
		found = found || input.CommandID == util.UUIDToString(joined.CommandID)
	}
	if !found {
		t.Fatal("regression requires a later run to have read the joined input")
	}
	memberToken, memberNonce := permitSquadRun(t, memberRuntime, memberRun, memberClaim)
	proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: firstClaim.Identity,
		LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0", State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), continuation.ID, firstClaim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight' AND group_id=$2`, joined.CommandID, continuation.GroupID) != 1 {
		t.Fatal("a member still holding the input lost its group dependency")
	}
	guardTx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = guardTx.Rollback(context.Background()) }()
	q := f.svc.Queries.WithTx(guardTx)
	seq, err := q.AllocateCollaborationBatchSequence(f.ctx, continuation.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := q.CreateCollaborationBatch(f.ctx, db.CreateCollaborationBatchParams{SessionID: f.issue.SessionID, ChannelID: continuation.ChannelID, EffectiveTargetID: f.targetID, Seq: seq})
	if err != nil {
		t.Fatal(err)
	}
	if err = q.DetachCollaborationGroupDependency(f.ctx, db.DetachCollaborationGroupDependencyParams{GroupID: continuation.GroupID, CommandID: joined.CommandID, Reason: "never_launched"}); err != nil {
		t.Fatal(err)
	}
	_, err = q.ReturnNeverLaunchedCollaborationInput(f.ctx, db.ReturnNeverLaunchedCollaborationInputParams{CommandID: joined.CommandID, GroupID: continuation.GroupID, BatchID: batch.ID, State: "waiting", Reason: "never_launched"})
	if err == nil || !strings.Contains(err.Error(), "potentially launched reader") {
		t.Fatalf("database guard accepted a return while another reader may be live: %v", err)
	}
	if err = guardTx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, memberRuntime, memberRun, memberClaim, memberToken, memberNonce, "success")
	last, _, lastClaim := f.claimManaged(t)
	lastToken, lastNonce := permitSquadRun(t, f, last, lastClaim)
	finishSquadRun(t, f, last, lastClaim, lastToken, lastNonce, "success")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='succeeded' AND group_id=$2`, joined.CommandID, continuation.GroupID) != 1 {
		t.Fatal("a nonlaunch proof returned input already read by a later launched run")
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.ExecutionView(f.ctx, access)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range view.Groups[0].Inputs {
		if input.CommandID != util.UUIDToString(joined.CommandID) {
			continue
		}
		if len(input.ReaderRunIDs) != 3 || !sort.StringsAreSorted(input.ReaderRunIDs) {
			t.Fatalf("execution view did not retain every reader deterministically: %+v", input)
		}
		for _, reader := range []string{util.UUIDToString(continuation.ID), util.UUIDToString(memberRun), util.UUIDToString(last)} {
			if !slices.Contains(input.ReaderRunIDs, reader) {
				t.Fatalf("execution view omitted reader %s", reader)
			}
		}
	}
}

func TestCollaborationSquadNonlaunchReturnsInputBeforeOtherMembersFinish(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "root instruction")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") keep working",
	})
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("member dispatch: %+v %v", dispatch, err)
	}
	joined := f.submitManaged(t, f.handler, "cancel after proven nonlaunch")
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	continuation, err := f.svc.Queries.GetCollaborationQueuedContinuation(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
	if err != nil {
		t.Fatal(err)
	}
	continuationClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: continuationClaim.Identity,
		LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0", State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), continuation.ID, continuationClaim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, joined.CommandID) != 1 {
		t.Fatal("proven unread input remained consumed until the whole squad finished")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND settled_at IS NULL AND expansion_sealed_at IS NULL AND forced_disposition IS NULL`, continuation.GroupID) != 1 {
		t.Fatal("returning unread input ended the member's group")
	}
	assertSquadExecutionRead(t, f, 0)
	_, err = f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: joined.CommandID, ExpectedCollaborationRevision: 1})
	if err != nil {
		t.Fatalf("nonlaunch proof did not restore waiting cancellation: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), continuation.ID, continuationClaim.TerminationToken, proof); err != nil {
		t.Fatalf("proof replay after cancellation: %v", err)
	}
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, dispatch.RunIDs[0], request)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range memberClaim.Inputs {
		if input.CommandID == util.UUIDToString(joined.CommandID) {
			t.Fatal("later member received a returned or cancelled input")
		}
	}
	assertSquadExecutionRead(t, f, 0)
}

func TestCollaborationSquadContinuationContextKeepsLatestMemberReply(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	old := time.Now().Add(-time.Hour)
	for i := range 200 {
		f.fx.Comment(t, util.UUIDToString(f.issue.Issue.ID), "older discussion", testutil.Cols{"created_at": old.Add(time.Duration(i) * time.Second)})
	}
	f.submitManaged(t, f.creator, "review the current result")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") report the result",
	})
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("member dispatch: %+v %v", dispatch, err)
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	memberRun, _, memberClaim := f.claimManaged(t)
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	reply, err := f.svc.CreateRuntimeComment(context.Background(), memberRun, memberToken, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "The current result needs a leader decision", ParentID: dispatch.CommentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, f, memberRun, memberClaim, memberToken, memberNonce, "success")
	last, _, lastClaim := f.claimManaged(t)
	lastToken, _ := permitSquadRun(t, f, last, lastClaim)
	view, err := f.svc.RuntimeContext(context.Background(), last, lastToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Comments) != 200 || view.Comments[len(view.Comments)-1].ID != reply.CommentID {
		t.Fatal("bounded continuation context omitted the newest member result")
	}
	for i := 1; i < len(view.Comments); i++ {
		if view.Comments[i].CreatedAt.Before(view.Comments[i-1].CreatedAt) {
			t.Fatal("recent context lost chronological order")
		}
	}
}
