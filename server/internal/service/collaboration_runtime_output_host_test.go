package service

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimeOutputO01SquadFilePreservesOriginalDelegation(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	dir := configureRuntimeOutputStorage(t, f)
	f.submitManaged(t, f.creator, "produce a squad output file")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, run, claim)
	output := &runtimeOutputFixture{managedCollaborationFixture: f, dir: dir, run: run, claim: claim, permit: protocol.CollaborationRunPermitResponse{BusinessToken: token, AttemptSeq: "1", AttemptNonce: nonce}}
	p := runtimeOutputInput()
	p.Content = "[@member](mention://agent/" + util.UUIDToString(member) + ") use this original file [@human](mention://member/" + f.handler + ")"
	original, err := output.write(p)
	if err != nil || len(original.RunIDs) != 1 || original.Attachment == nil {
		t.Fatalf("file comment did not preserve the real squad writer: %+v %v", original, err)
	}
	replay, err := output.write(p)
	if err != nil || !reflect.DeepEqual(original, replay) {
		t.Fatalf("squad file replay changed its original file/delegation receipt: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_delegation WHERE source_comment_id=$1 AND run_id=$2`, p.CommentID, original.RunIDs[0]) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2 AND recipient_user_id=$3`, f.issue.Issue.ID, p.CommentID, f.handler) != 1 {
		t.Fatal("squad file replay duplicated its original derived run, delegation or human notification")
	}
	assertRuntimeOutputCount(t, output, 1, 1)
}

func TestCollaborationRuntimeOutputO07HostAttemptOwnershipAndFrozenCurrentFiles(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	dir := configureRuntimeOutputStorage(t, f)
	submitHostCommand(t, f, f.creator, "original host output attempt")
	run, claim := claimHostRun(t, f, caps)
	firstLaunch := hostPermitRequest(claim, 1, "provider")
	firstPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, firstLaunch)
	if err != nil {
		t.Fatal(err)
	}
	first := &runtimeOutputFixture{managedCollaborationFixture: f, dir: dir, run: run, claim: claim, permit: firstPermit}
	original := runtimeOutputInput()
	file, err := first.write(original)
	if err != nil {
		t.Fatal(err)
	}
	for range 31 {
		if _, err = first.write(runtimeOutputInput()); err != nil {
			t.Fatal(err)
		}
	}
	end := hostLaunchEnded(firstLaunch, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), run, claim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	secondLaunch := hostPermitRequest(claim, 2, "provider")
	secondPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, secondLaunch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.write(original); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("old attempt token replayed a committed output: %v", err)
	}
	if _, err = f.svc.RuntimeAttachment(context.Background(), run, file.Attachment.ID, 1, firstPermit.BusinessToken); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("old attempt token retained output bytes: %v", err)
	}
	second := &runtimeOutputFixture{managedCollaborationFixture: f, dir: dir, run: run, claim: claim, permit: secondPermit}
	if _, err = second.write(original); !collaborationErrorCode(err, "comment_conflict") {
		t.Fatalf("new attempt reattributed the old comment/file UUID: %v", err)
	}
	if _, err = f.svc.RuntimeAttachment(context.Background(), run, file.Attachment.ID, 1, secondPermit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
		t.Fatalf("new attempt read old output outside its original frozen manifest: %v", err)
	}
	current, err := second.write(runtimeOutputInput())
	if err != nil || current.Attachment.AttemptSeq == nil || *current.Attachment.AttemptSeq != "2" {
		t.Fatalf("per-attempt output capacity or actual attribution was lost: %+v %v", current, err)
	}
	outcome := second.outcome()
	outcome.ProtocolVersion, outcome.Containment = 3, protocol.CollaborationContainmentHostProcessSupervision
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, secondPermit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	e, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: outcome.EventID})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeCollaborationStoredOutcome(e.Payload)
	if err != nil || !reflect.DeepEqual(saved.OutputAttachmentIDs, []string{util.UUIDToString(current.Attachment.ID)}) {
		t.Fatalf("current outcome included earlier-attempt files or lost current output: %+v %v", saved.OutputAttachmentIDs, err)
	}
	if len(runtimeOutputObjectNames(t, dir)) != 33 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE run_id=$1 AND attempt_seq=1`, run) != 32 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE run_id=$1 AND attempt_seq=2`, run) != 1 {
		t.Fatal("current-attempt outcome rewrote earlier output history")
	}
}

func TestCollaborationRuntimeOutputO08HostCheckpointSealReportAndOriginalACK(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	dir := configureRuntimeOutputStorage(t, f)
	submitHostCommand(t, f, f.creator, "retained host output and test log")
	run, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, launch)
	if err != nil {
		t.Fatal(err)
	}
	output := &runtimeOutputFixture{managedCollaborationFixture: f, dir: dir, run: run, claim: claim, permit: permit}
	file, err := output.write(runtimeOutputInput())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := checkpointFor(claim, permit)
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), run, permit.BusinessToken, checkpoint); err != nil {
		t.Fatal(err)
	}
	body := "retained outcome test log"
	outcome := output.outcome()
	outcome.ProtocolVersion, outcome.Containment = 3, protocol.CollaborationContainmentHostProcessSupervision
	outcome.Record = &protocol.CollaborationRunRecord{Version: 2, StreamID: checkpoint.StreamID, MessageOffset: 1, Completeness: "complete", Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: body}}}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), run, permit.BusinessToken, checkpoint); err != nil {
		t.Fatalf("original checkpoint replay failed after the extended outcome: %v", err)
	}
	observed := 2
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, StreamID: checkpoint.StreamID, ObservedThrough: &observed, AcknowledgedThrough: 1, Reason: "completed"}
	retained, err := f.svc.SealRuntimeRecording(context.Background(), run, claim.TerminationToken, seal)
	if err != nil || !retained.Sealed || retained.RetainedThrough != 2 {
		t.Fatalf("extended outcome became unreadable to seal/retention: %+v %v", retained, err)
	}
	e, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: outcome.EventID})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := f.svc.Queries.GetCollaborationAttempt(f.ctx, db.GetCollaborationAttemptParams{RunID: run, AttemptSeq: 1})
	if err != nil {
		t.Fatal(err)
	}
	log := protocol.CollaborationTestLog{Kind: "recorded", EventID: util.UUIDToString(e.ID), MessageIndex: 0, BodySHA256: protocol.CollaborationKnowledgeHash(body)}
	if got, err := resolveTestReportLog(f.ctx, f.svc.Queries, run, claim.Identity, attempt, log); err != nil || got != body {
		t.Fatalf("outcome-backed test report log rejected frozen output references: %v", err)
	}
	if state, err := readCollaborationRecording(f.ctx, f.svc.Queries, f.issue.Issue.ID, run, math.MaxInt64); err != nil || len(state.messages) != 2 {
		t.Fatalf("extended recording did not retain its exact stream: %v", err)
	}
	viewer := WithCollaborationExecutionViewVersion(f.ctx, 6)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	results := 0
	for _, item := range page.Items {
		if item.Kind == "result" {
			results++
			if item.AttachmentIDs == nil || !reflect.DeepEqual(*item.AttachmentIDs, []string{util.UUIDToString(file.Attachment.ID)}) {
				t.Fatal("public result records lost the original frozen output ID")
			}
		}
	}
	if results != 1 || page.NextCursor != nil {
		t.Fatal("records did not return the complete original outcome")
	}
	f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),epoch=epoch+1 WHERE id=$1`, f.grant.ID)
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, outcome); err != nil {
		t.Fatalf("content-free complete original ACK acquired a new current-authority requirement: %v", err)
	}
	changed := outcome
	changed.Record = &protocol.CollaborationRunRecord{Version: 2, StreamID: checkpoint.StreamID, MessageOffset: 1, Completeness: "complete", Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "changed retained log"}}}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, changed); !collaborationErrorCode(err, "outcome_conflict") {
		t.Fatalf("extended stored ACK stopped comparing the full original record: %v", err)
	}
}

func TestCollaborationRuntimeOutputO07ExplicitNewRunKeepsOriginalFailure(t *testing.T) {
	base, caps := newHostCollaborationFixture(t)
	dir := configureRuntimeOutputStorage(t, base)
	submitHostCommand(t, base, base.creator, "produce an original host output before failure")
	run, claim := claimHostRun(t, base, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := base.svc.PermitRuntimeRun(base.daemonCtx, run, launch)
	if err != nil {
		t.Fatal(err)
	}
	f := &runtimeOutputFixture{managedCollaborationFixture: base, dir: dir, run: run, claim: claim, permit: permit}
	originalInput := runtimeOutputInput()
	originalFile, err := f.write(originalInput)
	if err != nil {
		t.Fatal(err)
	}
	failure := f.outcome()
	failure.ProtocolVersion, failure.Containment = 3, protocol.CollaborationContainmentHostProcessSupervision
	failure.Outcome, failure.Body = "failure", "Original failed result"
	if err = f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, failure); err != nil {
		t.Fatal(err)
	}
	ended := hostLaunchEnded(launch, true)
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), f.run, f.claim.TerminationToken, ended); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), f.run, f.claim.TerminationToken, hostFinalProof(t, f.claim, ended)); err != nil {
		t.Fatal(err)
	}
	oldEvent, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: failure.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.write(originalInput); err == nil {
		t.Fatal("terminal run accepted a late committed-output replay")
	}
	submitHostCommand(t, base, base.creator, "explicit new work after failed result")
	next, nextClaim := claimHostRun(t, base, caps)
	if next == f.run {
		t.Fatal("explicit new work reused the failed execution identity")
	}
	nextPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, next, hostPermitRequest(nextClaim, 1, "provider"))
	if err != nil {
		t.Fatal(err)
	}
	second := &runtimeOutputFixture{managedCollaborationFixture: f.managedCollaborationFixture, dir: f.dir, run: next, claim: nextClaim, permit: nextPermit}
	if _, err = second.write(runtimeOutputInput()); err != nil {
		t.Fatal(err)
	}
	success := second.outcome()
	success.ProtocolVersion, success.Containment = 3, protocol.CollaborationContainmentHostProcessSupervision
	if err = f.svc.RecordRuntimeOutcome(context.Background(), next, nextPermit.BusinessToken, success); err != nil {
		t.Fatal(err)
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: failure.EventID})
	if err != nil || !reflect.DeepEqual(oldEvent, after) {
		t.Fatalf("new success rewrote the original failed result: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND run_id=$2 AND attempt_seq=1`, originalFile.Attachment.ID, f.run) != 1 {
		t.Fatal("output recovery dispatched an extra run or rewrote original file attribution")
	}
}

func TestCollaborationRuntimeOutputO08OldNoFileEventBytesRemainStable(t *testing.T) {
	p := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "original no-file JSON"}
	receipt := CollaborationRuntimeCommentReceipt{CommentID: p.CommentID, RunIDs: []pgtype.UUID{}}
	type legacyReceipt struct {
		CommentID         pgtype.UUID   `json:"comment_id"`
		RunIDs            []pgtype.UUID `json:"run_ids"`
		ContinuationRunID pgtype.UUID   `json:"continuation_run_id"`
	}
	legacy, err := json.Marshal(struct {
		Input   CollaborationRuntimeCommentParams `json:"input"`
		Receipt legacyReceipt                     `json:"receipt"`
	}{Input: p, Receipt: legacyReceipt{CommentID: p.CommentID, RunIDs: []pgtype.UUID{}}})
	if err != nil {
		t.Fatal(err)
	}
	current, err := json.Marshal(collaborationRuntimeCommentEvent{Input: collaborationRuntimeCommentInput{CollaborationRuntimeCommentParams: p}, Receipt: receipt})
	if err != nil || string(legacy) != string(current) {
		t.Fatalf("no-file event acquired an attempt/file extension or changed old bytes: %s %s %v", legacy, current, err)
	}
}
