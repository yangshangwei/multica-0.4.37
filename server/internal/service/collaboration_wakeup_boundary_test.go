package service

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHandoffTwoMembersAndClaimBoundary(t *testing.T) {
	f, members := newWakeupBoundarySquad(t)
	rootInput := f.submitManaged(t, f.creator, "root request with immutable author")
	f.resumeManaged(t)
	root, request, rootClaim := f.claimManaged(t)
	rootToken, rootNonce := permitSquadRun(t, f, root, rootClaim)
	joined := f.submitManaged(t, f.handler, "join before the first continuation claim")
	if joined.Code != "command_joined_squad" {
		t.Fatalf("human input did not join the running squad: %+v", joined)
	}
	group := util.MustParseUUID(rootClaim.Identity.GroupID)
	first, err := f.svc.Queries.GetCollaborationQueuedContinuation(t.Context(), group)
	if err != nil {
		t.Fatal(err)
	}
	memberRuns := make([]pgtype.UUID, 0, len(members))
	for i, member := range members {
		p := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: fmt.Sprintf("[@member](mention://agent/%s) investigate part %d", util.UUIDToString(member), i+1)}
		receipt, err := f.svc.CreateRuntimeComment(t.Context(), root, rootToken, p)
		if err != nil || len(receipt.RunIDs) != 1 {
			t.Fatalf("member %d dispatch: %+v: %v", i, receipt, err)
		}
		memberRuns = append(memberRuns, receipt.RunIDs[0])
	}
	finishSquadRun(t, f, root, rootClaim, rootToken, rootNonce, "success")
	firstCauses := map[string]string{"input:" + util.UUIDToString(joined.CommandID): util.UUIDToString(root)}
	resultBodies := []string{"parser result: preserve field order", "catalog result: missing fixture is unresolved"}
	outcomes := []string{"success", "failure"}
	resultSources := make([]string, len(members))
	for i, run := range memberRuns {
		claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, request)
		if err != nil {
			t.Fatal(err)
		}
		token, nonce := permitSquadRun(t, f, run, claim)
		comment := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: resultBodies[i]}
		for range 2 {
			receipt, err := f.svc.CreateRuntimeComment(t.Context(), run, token, comment)
			if err != nil || receipt.ContinuationRunID != first.ID {
				t.Fatalf("member comment or exact replay created a second queued leader: %+v: %v", receipt, err)
			}
		}
		firstCauses["comment:"+util.UUIDToString(comment.CommentID)] = util.UUIDToString(run)
		outcome := protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nonce, Outcome: outcomes[i], Body: resultBodies[i]}
		if err = f.svc.RecordRuntimeOutcome(t.Context(), run, token, outcome); err != nil {
			t.Fatal(err)
		}
		source, err := f.svc.Queries.GetCollaborationRuntimeEvent(t.Context(), db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: outcome.EventID})
		if err != nil {
			t.Fatal(err)
		}
		resultSources[i] = util.UUIDToString(source.ID)
		proof := managedProof(claim.Identity, nonce)
		for range 2 {
			if _, err = f.svc.AcceptRuntimeProof(t.Context(), run, claim.TerminationToken, proof); err != nil {
				t.Fatalf("member proof or its exact replay failed: %v", err)
			}
		}
		firstCauses["proof:"+util.UUIDToString(run)] = util.UUIDToString(run)
	}
	if f.fx.Count(t, `SELECT count(DISTINCT agent_id) FROM collaboration_run WHERE id=ANY($1::uuid[]) AND business_outcome IS NOT NULL AND settled_at IS NOT NULL`, memberRuns) != 2 {
		t.Fatal("the two outcomes must come from two distinct squad members")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND step_kind='continuation'`, group) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, joined.CommandID) != 1 {
		t.Fatal("pre-claim causes duplicated a leader or consumed the joined input early")
	}
	// Polling drains both durable proof causes into the same prepared turn.
	continuation, _, claimed := f.claimManaged(t)
	if continuation != first.ID || len(claimed.Inputs) != 2 || claimed.Inputs[0].CommandID != util.UUIDToString(rootInput.CommandID) ||
		claimed.Inputs[1].CommandID != util.UUIDToString(joined.CommandID) || claimed.Inputs[1].AuthorUserID != f.handler {
		t.Fatalf("coalesced claim lost its original inputs, order or author: %+v", claimed.Inputs)
	}
	firstToken, firstNonce := permitSquadRun(t, f, continuation, claimed)
	snapshotRequest := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()}
	frozen, err := f.svc.RuntimeHandoffSnapshot(t.Context(), continuation, firstToken, snapshotRequest)
	if err != nil {
		t.Fatal(err)
	}
	assertWakeupBoundaryCauses(t, frozen.WakeupCauses, firstCauses)
	for i, run := range memberRuns {
		index := slices.IndexFunc(frozen.Results.Items, func(result protocol.CollaborationHandoffResult) bool { return result.RunID == util.UUIDToString(run) })
		if index < 0 {
			t.Fatalf("member %d result is absent", i)
		}
		result := frozen.Results.Items[index]
		if result.Role != "member" || result.ParentRunID != util.UUIDToString(root) || result.Outcome != outcomes[i] || result.ProofState != "exited_proven" ||
			result.ResultState != "present" || result.SourceKind != "provider_result" || result.SourceID != resultSources[i] || result.FinalAnswerState != "unknown" {
			t.Fatalf("member %d result lost outcome/source provenance or invented a final answer: %+v", i, result)
		}
		index = slices.IndexFunc(frozen.Sources, func(source protocol.CollaborationHandoffSource) bool { return source.ID == result.SourceID })
		if index < 0 || frozen.Sources[index].Content != resultBodies[i] || frozen.Sources[index].RunID != util.UUIDToString(run) || frozen.Sources[index].Trust != "data_only" {
			t.Fatalf("member %d result reference no longer resolves to its original data-only body", i)
		}
	}

	// A plain human discussion is useful evidence, but is not an admitted input
	// or an automatic wake cause merely because its author may submit commands.
	discussion, err := f.svc.CreateComment(f.ctx, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(),
		CollaborationCommentInput: CollaborationCommentInput{Content: "human discussion without an execution target"}})
	if err != nil || len(discussion.Targets) != 0 {
		t.Fatalf("ordinary discussion unexpectedly submitted work: %+v: %v", discussion, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND step_kind='continuation'`, group) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 2 {
		t.Fatal("ordinary discussion acquired command or continuation authority")
	}
	lateInput := f.submitManaged(t, f.handler, "join after the first continuation claim")
	if lateInput.Code != "command_joined_squad" {
		t.Fatalf("late accepted input did not join the next turn: %+v", lateInput)
	}
	second, err := f.svc.Queries.GetCollaborationQueuedContinuation(t.Context(), group)
	if err != nil || second.ID == continuation || second.LeaderTurn.Int64 != first.LeaderTurn.Int64+1 {
		t.Fatalf("late input did not allocate exactly the next leader turn: %+v: %v", second, err)
	}
	lateComment := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@leader](mention://agent/" + util.UUIDToString(f.grant.AgentID) + ") review the late evidence next turn"}
	for range 2 {
		receipt, err := f.svc.CreateRuntimeComment(t.Context(), continuation, firstToken, lateComment)
		if err != nil || receipt.ContinuationRunID != second.ID {
			t.Fatalf("post-claim comment or replay reused the frozen turn or created a duplicate: %+v: %v", receipt, err)
		}
	}
	// A fresh delegation supplies a proof after the claim boundary, while the
	// two original member outcomes above already belong to the first turn.
	lateDispatch, err := f.svc.CreateRuntimeComment(t.Context(), continuation, firstToken, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(members[0]) + ") verify the first turn's conclusion"})
	if err != nil || len(lateDispatch.RunIDs) != 1 {
		t.Fatalf("late member dispatch: %+v: %v", lateDispatch, err)
	}
	replayedClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation, request)
	if err != nil || !reflect.DeepEqual(replayedClaim.Inputs, claimed.Inputs) {
		t.Fatalf("late input changed the original claimed prompt: %+v: %v", replayedClaim.Inputs, err)
	}
	replayedSnapshot, err := f.svc.RuntimeHandoffSnapshot(t.Context(), continuation, firstToken, snapshotRequest)
	if err != nil || !reflect.DeepEqual(replayedSnapshot, frozen) {
		t.Fatalf("late facts changed the frozen handoff snapshot: %v", err)
	}
	liveFirst, err := f.svc.RuntimeHandoff(t.Context(), continuation, firstToken, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	assertWakeupBoundaryCauses(t, liveFirst.WakeupCauses, firstCauses)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, lateInput.CommandID) != 1 {
		t.Fatal("late input became consumed by the already claimed leader")
	}
	finishSquadRun(t, f, continuation, claimed, firstToken, firstNonce, "success")
	lateMember := lateDispatch.RunIDs[0]
	lateClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, lateMember, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lateClaim.Inputs, claimed.Inputs) {
		t.Fatal("delegation from the first turn acquired the unread late input")
	}
	lateToken, lateNonce := permitSquadRun(t, f, lateMember, lateClaim)
	finishSquadRun(t, f, lateMember, lateClaim, lateToken, lateNonce, "success")
	next, _, nextClaim := f.claimManaged(t)
	if next != second.ID || len(nextClaim.Inputs) != 3 || nextClaim.Inputs[2].CommandID != util.UUIDToString(lateInput.CommandID) || nextClaim.Inputs[2].AuthorUserID != f.handler {
		t.Fatalf("next continuation lost late input or duplicated a leader: %+v", nextClaim.Inputs)
	}
	nextToken, nextNonce := permitSquadRun(t, f, next, nextClaim)
	nextHandoff, err := f.svc.RuntimeHandoff(t.Context(), next, nextToken, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	lateCauses := map[string]string{
		"input:" + util.UUIDToString(lateInput.CommandID):     util.UUIDToString(continuation),
		"comment:" + util.UUIDToString(lateComment.CommentID): util.UUIDToString(continuation),
		"proof:" + util.UUIDToString(lateMember):              util.UUIDToString(lateMember),
	}
	assertWakeupBoundaryCauses(t, nextHandoff.WakeupCauses, lateCauses)
	wantReaders := map[string][]string{
		util.UUIDToString(rootInput.CommandID): {util.UUIDToString(root), util.UUIDToString(memberRuns[0]), util.UUIDToString(memberRuns[1]), util.UUIDToString(continuation), util.UUIDToString(lateMember), util.UUIDToString(next)},
		util.UUIDToString(joined.CommandID):    {util.UUIDToString(continuation), util.UUIDToString(lateMember), util.UUIDToString(next)},
		util.UUIDToString(lateInput.CommandID): {util.UUIDToString(next)},
	}
	if nextHandoff.Inputs.Total != 3 || nextHandoff.Inputs.Returned != 3 || len(nextHandoff.Inputs.Items) != 3 {
		t.Fatalf("input projection lost a claimed input: %+v", nextHandoff.Inputs)
	}
	for _, input := range nextHandoff.Inputs.Items {
		want, exists := wantReaders[input.CommandID]
		slices.Sort(want)
		got := slices.Clone(input.ReaderRunIDs)
		slices.Sort(got)
		if !exists || input.ReadersTruncated || !slices.Equal(got, want) {
			t.Fatalf("claim boundary lost or invented prompt readers for %s: got=%v want=%v", input.CommandID, got, want)
		}
		delete(wantReaders, input.CommandID)
	}
	if len(wantReaders) != 0 {
		t.Fatal("input projection duplicated one input instead of returning every reader set")
	}
	history, err := f.svc.RunHandoff(f.ctx, f.issue.Issue.ID, continuation, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	assertWakeupBoundaryCauses(t, history.WakeupCauses, firstCauses)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND step_kind='continuation'`, group) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id WHERE r.group_id=$1 AND e.kind='wakeup_assigned'`, group) != len(firstCauses)+len(lateCauses) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id WHERE r.group_id=$1 AND e.kind='wakeup_pending'`, group) != 3 {
		t.Fatal("coalescing, pending-proof drain or replay lost a source or created duplicate work")
	}
	finishSquadRun(t, f, next, nextClaim, nextToken, nextNonce, "success")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND state='succeeded' AND settled_at IS NOT NULL AND NOT wake_pending`, group) != 1 {
		t.Fatal("the final leader did not settle the group after consuming every cause")
	}
}

func newWakeupBoundarySquad(t *testing.T) (*managedCollaborationFixture, []pgtype.UUID) {
	t.Helper()
	f := newManagedCollaborationFixture(t)
	leader := util.UUIDToString(f.grant.AgentID)
	squad := f.fx.Squad(t, "Wakeup boundary squad", leader)
	roster := []collaboration.TargetResource{{Role: "leader", AgentID: leader, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}}
	var members []pgtype.UUID
	for i := range 2 {
		member := f.fx.Agent(t, fmt.Sprintf("Wakeup member %d", i+1), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
		f.fx.SquadMember(t, squad, "agent", member)
		members = append(members, util.MustParseUUID(member))
		roster = append(roster, collaboration.TargetResource{Role: "member", AgentID: member, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator})
	}
	target, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad,
		ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: 2, Roster: roster})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = target.TargetID
	for _, resource := range roster {
		consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), util.MustParseUUID(resource.AgentID))
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	return f, members
}

func assertWakeupBoundaryCauses(t *testing.T, causes protocol.CollaborationHandoffCauses, want map[string]string) {
	t.Helper()
	if causes.Total != len(want) || causes.Returned != len(want) || len(causes.Items) != len(want) || causes.Truncated || causes.NextCursor != "" {
		t.Fatalf("wakeup source inventory is incomplete: %+v; want=%v", causes, want)
	}
	seen := make(map[string]bool, len(want))
	lastSequence := int64(0)
	for _, cause := range causes.Items {
		key := cause.SourceKind + ":" + cause.SourceID
		sequence, err := strconv.ParseInt(cause.Sequence, 10, 64)
		if seen[key] || cause.ID == "" || want[key] == "" || want[key] != cause.SourceRunID || err != nil || sequence <= lastSequence {
			t.Fatalf("wakeup cause lost identity, attribution, uniqueness or source order: %+v; want=%v", cause, want)
		}
		seen[key], lastSequence = true, sequence
	}
}
