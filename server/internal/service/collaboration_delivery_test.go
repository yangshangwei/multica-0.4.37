package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationGenericStatusesCannotManufactureReview(t *testing.T) {
	for _, action := range []string{"change_status", "reopen"} {
		if collaborationStatusCategoryAllowed(action, "in_review") {
			t.Errorf("%s permits review without an exact submitted result", action)
		}
	}
	if !collaborationStatusCategoryAllowed("return_information", "in_review") {
		t.Fatal("information return must retain the exact request's saved review stage")
	}
}

func deliverySummaryForTest(t *testing.T, f *collaborationQueueFixture, ctx context.Context) CollaborationDeliverySummary {
	t.Helper()
	var summary CollaborationDeliverySummary
	err := f.svc.readCollaborationContent(ctx, f.issue.Issue.ID, func(q *db.Queries, access CollaborationIssueAccess) error {
		var err error
		summary, err = collaborationDeliverySummary(ctx, q, access)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func deliveryReturnParams(t *testing.T, f *collaborationQueueFixture, revision int64, submission pgtype.UUID, target, reason string) CollaborationAcceptanceReturnParams {
	t.Helper()
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "return_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationAcceptanceReturnParams{CollaborationWorkflowRequest: workflowRequest(f, revision), SubmissionID: submission,
		TargetUserID: util.MustParseUUID(target), Note: reason, ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion}
}

func TestCollaborationDeliveryRevisionResubmissionAndExactAcceptance(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	result, basis := "  First result\nwith exact formatting  ", "  Basis: the recorded checks passed.\n"
	p := handoffAcceptance(t, f, 1, result)
	p.Basis = basis
	submitted, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
	if err != nil || submitted.DeliveryConfirmation == nil || len(submitted.DeliveryConfirmation.AttachmentIDs) != 0 {
		t.Fatalf("text-only result did not confirm its exact identity: %+v %v", submitted, err)
	}
	id := submitted.DeliveryConfirmation.SubmissionID
	summary := deliverySummaryForTest(t, f, f.ctx)
	if summary.Current == nil || summary.Latest == nil || summary.Current.ID != id || summary.Current.AuthorUserID != util.MustParseUUID(f.handler) ||
		summary.Current.Result == nil || *summary.Current.Result != result || summary.Current.Basis == nil || *summary.Current.Basis != basis ||
		summary.Current.EvidenceState != "available" || summary.Current.Decision != nil {
		t.Fatalf("submission snapshot lost its exact prose/author: %+v", summary)
	}
	before := *summary.Current
	transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 2), TargetUserID: util.MustParseUUID(f.outsider)}}
	if _, err = f.svc.Transfer(f.ctx, transfer); !collaborationErrorCode(err, "acceptance_pending") {
		t.Fatalf("generic transfer skipped acceptance: %v", err)
	}
	if _, err = f.svc.ChangeStatus(f.ctx, statusParams(t, f, "todo", 2)); !collaborationErrorCode(err, "acceptance_pending") {
		t.Fatalf("generic status skipped acceptance: %v", err)
	}
	// Request/return information preserves this exact pending delivery. A plain
	// comment supplies material but cannot perform the return or acceptance.
	ask := handoffAsk(t, f, 2, f.handler, "Please explain the verification method")
	if _, err = f.svc.RequestInformation(f.ctx, ask); err != nil {
		t.Fatal(err)
	}
	requestID := activeInformationRequest(t, f)
	if _, err = f.svc.CreateComment(collaborationContext(f.handler), CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(),
		CollaborationCommentInput: CollaborationCommentInput{Content: "The requested method and details"}}); err != nil {
		t.Fatal(err)
	}
	during := deliverySummaryForTest(t, f, f.ctx)
	if during.Current == nil || !reflect.DeepEqual(before, *during.Current) || activeInformationRequest(t, f) != requestID {
		t.Fatal("information or an ordinary comment changed the submitted snapshot")
	}
	blocked := lifecycleParams(t, f, "complete", "done", 3, 1)
	blocked.SubmissionID = &id
	if _, err = f.svc.ChangeLifecycle(f.ctx, blocked); !collaborationErrorCode(err, "acceptance_pending") {
		t.Fatalf("acceptance skipped an open information request: %v", err)
	}
	if _, err = f.svc.ReturnInformation(collaborationContext(f.handler), handoffReturn(t, f, 3, requestID, "in_review")); err != nil {
		t.Fatal(err)
	}
	returned := deliveryReturnParams(t, f, 4, id, f.handler, "  Add the missing reproducible check.  ")
	blank := returned
	blank.RequestID, blank.Note = uuid.NewString(), " \n\t"
	if _, err = f.svc.ReturnAcceptance(f.ctx, blank); !collaborationErrorCode(err, "revision_reason_required") {
		t.Fatalf("blank revision reason was accepted: %v", err)
	}
	if _, err = f.svc.ReturnAcceptance(collaborationContext(f.handler), returned); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("participant returned an initiator's submission: %v", err)
	}
	if receipt, err := f.svc.ReturnAcceptance(f.ctx, returned); err != nil || receipt.Code != "acceptance_returned" {
		t.Fatalf("exact revision failed: %+v %v", receipt, err)
	}
	afterReturn := deliverySummaryForTest(t, f, f.ctx)
	if afterReturn.Current != nil || afterReturn.Latest == nil || afterReturn.Latest.Decision == nil || afterReturn.Latest.Decision.Kind != "returned" ||
		*afterReturn.Latest.Decision.Reason != returned.Note || afterReturn.Latest.Decision.TargetUserID != util.MustParseUUID(f.handler) ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='in_progress' AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatalf("revision lost its reason, exact decision or recipient: %+v", afterReturn)
	}
	second := handoffAcceptance(t, f, 5, "The revised result")
	second.Basis = "The requested reproducible check now passes"
	r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), second)
	if err != nil || r.DeliveryConfirmation == nil || r.DeliveryConfirmation.SubmissionID == id {
		t.Fatalf("resubmission did not allocate a new exact result: %+v %v", r, err)
	}
	secondID := r.DeliveryConfirmation.SubmissionID
	old := lifecycleParams(t, f, "complete", "done", 6, 1)
	old.SubmissionID = &id
	if _, err = f.svc.ChangeLifecycle(f.ctx, old); !collaborationErrorCode(err, "submission_changed") {
		t.Fatalf("old submission completed a revised result: %v", err)
	}
	accept := old
	accept.RequestID, accept.SubmissionID, accept.Note = uuid.NewString(), &secondID, "Accepted after the requested verification"
	if _, err = f.svc.ChangeLifecycle(collaborationContext(f.handler), accept); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("external author accepted their own result: %v", err)
	}
	completed, err := f.svc.ChangeLifecycle(f.ctx, accept)
	if err != nil || completed.Code != "completed" || completed.DeliveryConfirmation == nil || completed.DeliveryConfirmation.SubmissionID != secondID {
		t.Fatalf("initiator's exact acceptance failed: %+v %v", completed, err)
	}
	page, err := f.svc.ListAcceptanceSubmissions(f.ctx, f.issue.Issue.ID, CollaborationDeliveryHistoryQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != secondID || page.Items[0].Decision == nil || page.Items[0].Decision.Kind != "accepted" || page.NextCursor == nil {
		t.Fatalf("accepted result history unavailable: %+v %v", page, err)
	}
	older, err := f.svc.ListAcceptanceSubmissions(f.ctx, f.issue.Issue.ID, CollaborationDeliveryHistoryQuery{Limit: 1, Cursor: *page.NextCursor})
	if err != nil || len(older.Items) != 1 || older.Items[0].ID != id || older.Items[0].Decision == nil || older.Items[0].Decision.Kind != "returned" || older.NextCursor != nil {
		t.Fatalf("prior result/decision were not retained: %+v %v", older, err)
	}
	if _, err = f.svc.ListAcceptanceSubmissions(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationDeliveryHistoryQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "invalid_delivery_query") {
		t.Fatalf("another viewer used an issue history cursor: %v", err)
	}
	if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", 7, 2)); err != nil {
		t.Fatal(err)
	}
	reopened := deliverySummaryForTest(t, f, f.ctx)
	if reopened.Current != nil || reopened.Latest == nil || reopened.Latest.ID != secondID || reopened.Latest.Decision == nil || reopened.Latest.Decision.Kind != "accepted" {
		t.Fatal("reopen erased history or made old acceptance pending")
	}
	if _, err = f.svc.ChangeLifecycle(f.ctx, old); !collaborationErrorCode(err, "submission_changed") {
		t.Fatalf("original refused acceptance key recomputed after reopen: %v", err)
	}
}

func TestCollaborationLegacyDeliveryRequiresExplicitReplacement(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), handoffAcceptance(t, f, 1, "Legacy recorded result"))
	if err != nil {
		t.Fatal(err)
	}
	id := r.DeliveryConfirmation.SubmissionID
	f.fx.Exec(t, `UPDATE collaboration_event SET payload=payload-'delivery' WHERE issue_id=$1 AND kind='acceptance_submitted'`, f.issue.Issue.ID)
	legacy := deliverySummaryForTest(t, f, f.ctx)
	if legacy.Current == nil || legacy.Current.ID != id || legacy.Current.EvidenceState != "legacy" || legacy.Current.Basis != nil || legacy.Current.Decision != nil {
		t.Fatalf("legacy evidence invented a basis or decision: %+v", legacy)
	}
	complete := lifecycleParams(t, f, "complete", "done", 2, 1)
	complete.SubmissionID = &id
	if _, err = f.svc.ChangeLifecycle(f.ctx, complete); !collaborationErrorCode(err, "delivery_unavailable") {
		t.Fatalf("legacy evidence was accepted: %v", err)
	}
	// This is now creator=current handler. Use the real status-options read,
	// then explicitly replace the legacy result instead of a self-transfer.
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatalf("legacy creator-handler has no repair action: %v", err)
	}
	replacement := CollaborationAcceptanceParams{CollaborationWorkflowRequest: workflowRequest(f, 2), Note: "Explicit replacement result",
		Basis: "Verified with a current supporting check", ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion}
	replaced, err := f.svc.SubmitAcceptance(f.ctx, replacement)
	if err != nil || replaced.DeliveryConfirmation == nil || replaced.DeliveryConfirmation.SubmissionID == id {
		t.Fatalf("legacy result could not be explicitly replaced: %+v %v", replaced, err)
	}
	newID := replaced.DeliveryConfirmation.SubmissionID
	accept := lifecycleParams(t, f, "complete", "done", 3, 1)
	accept.SubmissionID = &newID
	if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
		t.Fatalf("repaired result could not be accepted: %v", err)
	}
	history, err := f.svc.ListAcceptanceSubmissions(f.ctx, f.issue.Issue.ID, CollaborationDeliveryHistoryQuery{Limit: 10})
	if err != nil || len(history.Items) != 2 || history.Items[0].ID != newID || history.Items[0].Decision == nil || history.Items[0].Decision.Kind != "accepted" ||
		history.Items[1].ID != id || history.Items[1].EvidenceState != "legacy" || history.Items[1].Decision != nil {
		t.Fatalf("replacement invented or erased legacy history: %+v %v", history, err)
	}
}

func TestCollaborationSelfDeliveryIsAtomicAndRecoverable(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	p.Note, p.Basis = "Self-handled completed result", "The supporting check passed"
	f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: "CreateCollaborationEvent"}
	if _, err := f.svc.ChangeLifecycle(f.ctx, p); err == nil {
		t.Fatal("required self-result event failure was ignored")
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND assignee_id=$2`, f.issue.Issue.ID, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("failed self completion committed partial result, lifecycle or receipt")
	}
	f.svc.TxStarter = f.pool
	r, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || r.DeliveryConfirmation == nil || r.CollaborationRevision != 2 {
		t.Fatalf("self delivery did not atomically complete once: %+v %v", r, err)
	}
	replayed, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || !reflect.DeepEqual(replayed, r) || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' AND from_user_id=to_user_id`, f.issue.Issue.ID) != 1 {
		t.Fatalf("self delivery replay duplicated a result: %+v %v", replayed, err)
	}
	summary := deliverySummaryForTest(t, f, f.ctx)
	if summary.Current != nil || summary.Latest == nil || summary.Latest.Decision == nil || summary.Latest.Decision.Kind != "accepted" ||
		summary.Latest.AuthorUserID != util.MustParseUUID(f.creator) || *summary.Latest.Result != p.Note || *summary.Latest.Basis != p.Basis {
		t.Fatalf("self delivery projection lost real result/decision: %+v", summary)
	}
	// Own result recovery grants no content access after participation/source loss.
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
	if _, err = f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("revoked creator kept content access: %v", err)
	}
	recovered, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || recovered.DeliveryConfirmation == nil || !reflect.DeepEqual(recovered.DeliveryConfirmation, r.DeliveryConfirmation) {
		t.Fatalf("own recovery lost the immutable result tuple: %+v %v", recovered, err)
	}
	raw, err := json.Marshal(recovered)
	if err != nil || strings.Contains(string(raw), p.Note) || strings.Contains(string(raw), p.Basis) || strings.Contains(string(raw), "actor_user_id") {
		t.Fatalf("own result recovery leaked private content: %s %v", raw, err)
	}
}

func TestCollaborationDeliveryRequiresBasisAndBoundsProse(t *testing.T) {
	for _, tc := range []struct {
		name, result, basis, code string
	}{
		{"blank_basis", "A completed result", " \n\t", "basis_required"},
		{"result_bytes", strings.Repeat("界", 21846), "Verified by the stated checks", "result_too_large"},
		{"basis_bytes", "A completed result", strings.Repeat("界", 21846), "basis_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			p := handoffAcceptance(t, f, 1, tc.result)
			p.Basis = tc.basis
			receipt, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
			if !collaborationErrorCode(err, tc.code) || receipt.Status != http.StatusBadRequest {
				t.Fatalf("%s was not a durable prose refusal: %+v %v", tc.name, receipt, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 {
				t.Fatal("prose refusal wrote a result or changed responsibility")
			}
			replayed, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
			if !collaborationErrorCode(err, tc.code) || replayed.Code != receipt.Code || replayed.CollaborationRevision != receipt.CollaborationRevision {
				t.Fatalf("original refusal recomputed: %+v %v", replayed, err)
			}
			p.Note, p.Basis = "Corrected result", "Corrected basis"
			if _, err = f.svc.SubmitAcceptance(collaborationContext(f.handler), p); !collaborationErrorCode(err, "idempotency_conflict") {
				t.Fatalf("changed result reused a refused key: %v", err)
			}
		})
	}
}

func TestCollaborationDeliveryRejectsBlankResult(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	p := handoffAcceptance(t, f, 1, " \n\t")
	r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
	if !collaborationErrorCode(err, "result_required") || r.Status != http.StatusBadRequest {
		t.Fatalf("blank result was not a durable refusal: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance'`, f.issue.Issue.ID) != 0 {
		t.Fatal("blank result created a submission")
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("refused result moved responsibility or status")
	}
	replay, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
	if !collaborationErrorCode(err, "result_required") || replay.Code != r.Code || replay.CollaborationRevision != r.CollaborationRevision {
		t.Fatalf("blank-result original key recomputed: %+v %v", replay, err)
	}
}

func TestCollaborationExternalCompletionRequiresDelivery(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	r, err := f.svc.ChangeLifecycle(f.ctx, p)
	if !collaborationErrorCode(err, "delivery_required") || r.Status != http.StatusConflict {
		t.Fatalf("external work completed without delivery: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND assignee_id=$2`, f.issue.Issue.ID, util.MustParseUUID(f.handler)) != 1 {
		t.Fatal("delivery refusal changed the source issue")
	}
}

func TestCollaborationDeliveryRetainsCommentSnapshotAndEncryptedEvidence(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	f.svc.Storage = storage.NewLocalStorageFromEnv()
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id IN (SELECT id FROM collaboration_attachment WHERE issue_id=$1)`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM comment_history WHERE comment_id IN (SELECT id FROM comment WHERE issue_id=$1)`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
	upload := CollaborationAttachmentUpload{DraftID: util.MustParseUUID(uuid.NewString()), IssueID: f.issue.Issue.ID,
		RequestID: uuid.NewString(), Name: "result-evidence.txt", ContentType: "text/plain", Data: []byte("immutable delivery evidence")}
	handler := collaborationContext(f.handler)
	file, err := f.svc.UploadAttachmentDraft(handler, upload)
	if err != nil {
		t.Fatal(err)
	}
	comment, err := f.svc.CreateComment(handler, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(),
		CollaborationCommentInput: CollaborationCommentInput{Content: "The original result from a discussion comment", DraftID: &upload.DraftID, AttachmentIDs: []pgtype.UUID{file.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	p := handoffAcceptance(t, f, 1, "The original result from a discussion comment")
	p.Basis, p.SourceCommentID, p.AttachmentIDs = "The attached evidence verifies the outcome", &comment.CommentID, []pgtype.UUID{file.ID}
	submitted, err := f.svc.SubmitAcceptance(handler, p)
	if err != nil || submitted.DeliveryConfirmation == nil || !reflect.DeepEqual(submitted.DeliveryConfirmation.AttachmentIDs, p.AttachmentIDs) {
		t.Fatalf("submission did not bind exact retained evidence: %+v %v", submitted, err)
	}
	snapshot := *deliverySummaryForTest(t, f, f.ctx).Current
	empty := []pgtype.UUID{}
	if _, err = f.svc.EditComment(handler, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: "Edited after formal submission", AttachmentIDs: &empty}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeleteComment(handler, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 2}); err != nil {
		t.Fatal(err)
	}
	if current := deliverySummaryForTest(t, f, f.ctx).Current; current == nil || !reflect.DeepEqual(snapshot, *current) {
		t.Fatal("comment editing/deletion mutated formal result, basis, author, time or evidence")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='issue' AND reference_id=$2 AND removed_at IS NULL`, file.ID, f.issue.Issue.ID) != 1 {
		t.Fatal("comment removal lost the permanent issue evidence reference")
	}
	row, err := f.svc.GetAttachment(handler, f.issue.Issue.ID, pgtype.UUID{}, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.svc.ReadAttachmentBytes(handler, row)
	if err != nil || !bytes.Equal(data, upload.Data) {
		t.Fatal("retained protected evidence bytes became unavailable")
	}
	accept := lifecycleParams(t, f, "complete", "done", 2, 1)
	accept.SubmissionID = &snapshot.ID
	r, err := f.svc.ChangeLifecycle(f.ctx, accept)
	if err != nil || r.DeliveryConfirmation == nil || !reflect.DeepEqual(r.DeliveryConfirmation.AttachmentIDs, p.AttachmentIDs) {
		t.Fatalf("acceptance failed to confirm existing evidence without rebinding: %+v %v", r, err)
	}
	remove := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 3), TargetUserID: util.MustParseUUID(f.handler)}
	remove.ExpectedLifecycleEpoch = 2
	if _, err = f.svc.RemoveParticipant(f.ctx, remove); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.GetAttachment(handler, f.issue.Issue.ID, pgtype.UUID{}, file.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("removed participant retained evidence read access: %v", err)
	}
	recovery, err := f.svc.GetWorkflowOperation(handler, f.issue.Issue.ID, p.RequestID)
	if err != nil || !reflect.DeepEqual(recovery.DeliveryConfirmation, submitted.DeliveryConfirmation) {
		t.Fatalf("own original result tuple was lost after access removal: %+v %v", recovery, err)
	}
	raw, err := json.Marshal(recovery)
	if err != nil || bytes.Contains(raw, []byte(p.Note)) || bytes.Contains(raw, []byte(p.Basis)) || bytes.Contains(raw, []byte("storage_key")) {
		t.Fatal("own recovery exposed protected result content or file storage")
	}
}

func TestCollaborationRawReviewSelfResultAndParentLifecycleStayExplicit(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	parent := f.fx.Issue(t, "Ordinary parent remains open", testutil.Cols{"status": "in_progress"})
	f.fx.Exec(t, `UPDATE issue SET assignee_id=$2,status='in_review',parent_issue_id=$3 WHERE id=$1`, f.issue.Issue.ID, f.creator, parent)
	before := deliverySummaryForTest(t, f, f.ctx)
	if before.Current != nil || before.Latest != nil {
		t.Fatal("a raw review status invented a delivery")
	}
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	p.Note, p.Basis = "Explicitly recorded result for the historical review", "A current supporting check was supplied"
	r, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || r.DeliveryConfirmation == nil || r.ParentWakeup == nil {
		t.Fatalf("explicit self-result repair did not commit a result/decision: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='in_progress'`, parent) != 1 {
		t.Fatal("collaboration completion completed its ordinary parent")
	}
	view := deliverySummaryForTest(t, f, f.ctx)
	if view.Current != nil || view.Latest == nil || view.Latest.Decision == nil || view.Latest.Decision.Kind != "accepted" ||
		view.Latest.AuthorUserID != util.MustParseUUID(f.creator) || *view.Latest.Result != p.Note || *view.Latest.Basis != p.Basis {
		t.Fatal("historical review repair inferred or lost the explicit result and decision")
	}
}

func TestCollaborationDeliveryMalformedFactsRequireExplicitReplacement(t *testing.T) {
	for _, field := range []string{"version", "author_user_id", "lifecycle_epoch"} {
		t.Run(field, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), handoffAcceptance(t, f, 1, "Recorded original result"))
			if err != nil {
				t.Fatal(err)
			}
			value := `"` + uuid.NewString() + `"`
			if field == "version" {
				value = `99`
			} else if field == "lifecycle_epoch" {
				value = `"9007199254740993"`
			}
			f.fx.Exec(t, `UPDATE collaboration_event SET payload=jsonb_set(payload,ARRAY['delivery',$2],$3::jsonb) WHERE issue_id=$1 AND kind='acceptance_submitted'`, f.issue.Issue.ID, field, value)
			view := deliverySummaryForTest(t, f, f.ctx)
			if view.Current == nil || view.Current.EvidenceState != "unavailable" || view.Current.Basis != nil || view.Current.Decision != nil {
				t.Fatalf("malformed %s facts became usable evidence: %+v", field, view)
			}
			p := lifecycleParams(t, f, "complete", "done", 2, 1)
			p.SubmissionID = &r.DeliveryConfirmation.SubmissionID
			if _, err = f.svc.ChangeLifecycle(f.ctx, p); !collaborationErrorCode(err, "delivery_unavailable") {
				t.Fatalf("malformed evidence was accepted: %v", err)
			}
			options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "submit_acceptance")
			if err != nil {
				t.Fatal(err)
			}
			replaced, err := f.svc.SubmitAcceptance(f.ctx, CollaborationAcceptanceParams{CollaborationWorkflowRequest: workflowRequest(f, 2),
				Note: "Explicit replacement result", Basis: "A current verified basis", ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion})
			if err != nil || replaced.DeliveryConfirmation == nil || replaced.DeliveryConfirmation.SubmissionID == r.DeliveryConfirmation.SubmissionID {
				t.Fatalf("malformed evidence has no explicit repair: %+v %v", replaced, err)
			}
		})
	}
}
