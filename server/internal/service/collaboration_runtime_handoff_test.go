package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHandoffFrozenAssignmentAndSnapshot(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "Original intent, preserve this author")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	instruction := "[@member](mention://agent/" + util.UUIDToString(member) + ") inspect only the parser"
	comment := util.MustParseUUID(uuid.NewString())
	receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: comment, Content: instruction})
	if err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	memberRun := receipt.RunIDs[0]
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	// The original admitted event remains authoritative after a current comment edit.
	if _, err = f.pool.Exec(f.ctx, "UPDATE comment SET content='changed comment must not change delegation',revision=revision+1 WHERE id=$1", comment); err != nil {
		t.Fatal(err)
	}
	current, err := f.svc.RuntimeContext(context.Background(), memberRun, memberToken)
	if err != nil || current.DispatchComment != instruction {
		t.Fatalf("dispatch: %+v %v", current, err)
	}
	p := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString(), MemoryVersion: ""}
	frozen, err := f.svc.RuntimeHandoffSnapshot(context.Background(), memberRun, memberToken, p)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Assignment.Role != "member" || frozen.Assignment.ParentRunID != util.UUIDToString(root) || frozen.Assignment.SourceID == "" {
		t.Fatalf("assignment: %+v", frozen.Assignment)
	}
	found := false
	for _, s := range frozen.Sources {
		if s.Kind == "delegation" {
			found = s.Content == instruction && s.Trust == "admitted_assignment"
		}
	}
	if !found {
		t.Fatal("admitted delegation was not projected")
	}
	if len(frozen.Inputs.Items) != 1 || len(frozen.Inputs.Items[0].ReaderRunIDs) != 2 {
		t.Fatalf("complete prompt readers absent: %+v", frozen.Inputs)
	}
	raw, _ := json.Marshal(frozen)
	if _, err = f.pool.Exec(f.ctx, "UPDATE issue SET assignee_id=$1 WHERE id=$2", util.MustParseUUID(f.creator), f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	live, err := f.svc.RuntimeHandoff(context.Background(), memberRun, memberToken, CollaborationHandoffQuery{})
	if err != nil || live.Authority.HandlerID != f.creator || frozen.Authority.HandlerID == live.Authority.HandlerID {
		t.Fatalf("live and frozen authority not distinct: %+v %+v %v", frozen.Authority, live.Authority, err)
	}

	replay, err := f.svc.RuntimeHandoffSnapshot(context.Background(), memberRun, memberToken, p)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(replay)
	if string(raw) != string(again) {
		t.Fatal("snapshot replay changed")
	}
	binding := protocol.CollaborationHandoffBinding{SnapshotID: p.RequestID, PromptSHA256: protocol.CollaborationHandoffHash("actual prompt"), AttemptSeq: "1", AttemptNonce: memberNonce}
	if _, err = f.svc.BindRuntimeHandoff(context.Background(), memberRun, memberToken, binding); err != nil {
		t.Fatal(err)
	}
	binding.PromptSHA256 = protocol.CollaborationHandoffHash("different prompt")
	if _, err = f.svc.BindRuntimeHandoff(context.Background(), memberRun, memberToken, binding); !collaborationErrorCode(err, "handoff_binding_conflict") {
		t.Fatalf("changed prompt accepted: %v", err)
	}
	if _, err = f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RuntimeHandoffSnapshot(context.Background(), memberRun, memberToken, p); err == nil {
		t.Fatal("revoked run recovered private snapshot")
	}
	human, err := f.svc.RunHandoff(f.ctx, f.issue.Issue.ID, memberRun, CollaborationHandoffQuery{})
	if err != nil || human.Assignment.SourceID == "" {
		t.Fatalf("human retained history: %+v %v", human, err)
	}
}

func TestCollaborationHandoffPreservesEveryMergedWakeCause(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "root request")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"})
	if err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	m := receipt.RunIDs[0]
	mc, err := f.svc.ClaimRuntimeRun(f.daemonCtx, m, request)
	if err != nil {
		t.Fatal(err)
	}
	mt, mn := permitSquadRun(t, f, m, mc)
	for i := 0; i < 2; i++ {
		if _, err = f.svc.CreateRuntimeComment(context.Background(), m, mt, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "member evidence"}); err != nil {
			t.Fatal(err)
		}
	}
	finishSquadRun(t, f, m, mc, mt, mn, "failure")
	continuation, _, cc := f.claimManaged(t)
	ct, _ := permitSquadRun(t, f, continuation, cc)
	envelope, err := f.svc.RuntimeHandoff(context.Background(), continuation, ct, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range envelope.WakeupCauses.Items {
		counts[c.SourceKind]++
	}
	if counts["comment"] != 2 || counts["proof"] != 1 {
		t.Fatalf("merged sources lost: %+v", envelope.WakeupCauses)
	}
	if envelope.Results.Total < 3 {
		t.Fatalf("member results absent: %+v", envelope.Results)
	}
	// Every cause of a continuation queued by this code is assigned under the
	// group lock, so all three reads must say the inventory is complete.
	snapshot, err := f.svc.RuntimeHandoffSnapshot(context.Background(), continuation, ct, protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	human, err := f.svc.RunHandoff(f.ctx, f.issue.Issue.ID, continuation, CollaborationHandoffQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for name, read := range map[string]protocol.CollaborationHandoffEnvelope{"live": envelope, "snapshot": snapshot, "human": human} {
		if read.WakeupCauses.Completeness != "recorded" {
			t.Fatalf("%s read called a fully recorded continuation's causes unknown", name)
		}
	}
	// A continuation queued before cause recording existed keeps its old key
	// format and may hold only part of its causes; it must stay unknown.
	row, err := f.svc.Queries.GetCollaborationRecordedRun(f.ctx, db.GetCollaborationRecordedRunParams{IssueID: f.issue.Issue.ID, RunID: continuation})
	if err != nil {
		t.Fatal(err)
	}
	legacy := row
	legacy.StepKey = pgtype.Text{String: "continuation:" + uuid.NewString(), Valid: true}
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	var old protocol.CollaborationHandoffEnvelope
	old.WakeupCauses = protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "unknown"}
	if err = projectHandoffCauses(f.ctx, f.svc.Queries, access, legacy, collaborationHandoffCursor{}, &old); err != nil {
		t.Fatal(err)
	}
	if old.WakeupCauses.Completeness != "unknown" || old.WakeupCauses.Total != envelope.WakeupCauses.Total {
		t.Fatalf("legacy continuation claimed a complete inventory: %+v", old.WakeupCauses)
	}
}

func TestCollaborationHandoffSnapshotCannotResurrectPurgedMemory(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	f.memoryUpdate(t, f.creator, 0, "private version one")
	f.submitManaged(t, f.creator, "work")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, run, claim)
	p := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString(), MemoryVersion: "1"}
	snapshot, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, token, p)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Memory.SHA256 != protocol.CollaborationHandoffHash("private version one") {
		t.Fatal("wrong frozen memory")
	}
	f.memoryUpdate(t, f.creator, 1, "safe replacement")
	if _, err = f.svc.PurgeMemoryVersion(f.ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "remove secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RuntimeHandoffSnapshot(context.Background(), run, token, p); !collaborationErrorCode(err, "handoff_source_unavailable") {
		t.Fatalf("purged source revived: %v", err)
	}
	var stored string
	f.fx.QueryRow(t, `SELECT payload::text FROM collaboration_run_event WHERE run_id=$1 AND kind='handoff_snapshot'`, run).Scan(&stored)
	if strings.Contains(stored, "private version one") {
		t.Fatal("purgeable text duplicated into snapshot")
	}
}
