package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationDeliveryProjectionRequiresRetainedSourceCommentScope(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	p := handoffAcceptance(t, f, 1, "An immutable result independently recorded from discussion")
	r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
	if err != nil {
		t.Fatal(err)
	}
	// A valid UUID alone cannot establish a retained comment relationship.
	f.fx.Exec(t, `UPDATE collaboration_event SET payload=jsonb_set(payload,'{delivery,source_comment_id}',to_jsonb($2::text)) WHERE issue_id=$1 AND kind='acceptance_submitted'`, f.issue.Issue.ID, uuid.NewString())
	view := deliverySummaryForTest(t, f, f.ctx)
	if view.Current == nil || view.Current.EvidenceState != "unavailable" || view.Current.SourceCommentID.Valid || view.Current.Basis != nil {
		t.Fatalf("an unscoped source comment became trusted delivery evidence: %+v", view)
	}
	complete := lifecycleParams(t, f, "complete", "done", 2, 1)
	complete.SubmissionID = &r.DeliveryConfirmation.SubmissionID
	if _, err = f.svc.ChangeLifecycle(f.ctx, complete); !collaborationErrorCode(err, "delivery_unavailable") {
		t.Fatalf("unscoped source-comment evidence satisfied acceptance: %v", err)
	}
}

func TestCollaborationDeliveryProjectionRequiresRetainedAttachmentSource(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	f.svc.Storage = storage.NewLocalStorageFromEnv()
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id IN (SELECT id FROM collaboration_attachment WHERE issue_id=$1)`, f.issue.Issue.ID)
	upload := CollaborationAttachmentUpload{DraftID: util.MustParseUUID(uuid.NewString()), IssueID: f.issue.Issue.ID,
		RequestID: uuid.NewString(), Name: "retained-source-evidence.txt", ContentType: "text/plain", Data: []byte("Private evidence scoped to its original source")}
	handler := collaborationContext(f.handler)
	file, err := f.svc.UploadAttachmentDraft(handler, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := handoffAcceptance(t, f, 1, "A result with protected file evidence")
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{file.ID}
	r, err := f.svc.SubmitAcceptance(handler, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE collaboration_attachment SET source_workspace_id=$2 WHERE id=$1`, file.ID, uuid.NewString())
	view := deliverySummaryForTest(t, f, f.ctx)
	if view.Current == nil || view.Current.EvidenceState != "unavailable" || len(view.Current.AttachmentIDs) != 0 || view.Current.Basis != nil {
		t.Fatalf("a mismatched attachment source became trusted delivery evidence: %+v", view)
	}
	complete := lifecycleParams(t, f, "complete", "done", 2, 1)
	complete.SubmissionID = &r.DeliveryConfirmation.SubmissionID
	if _, err = f.svc.ChangeLifecycle(f.ctx, complete); !collaborationErrorCode(err, "delivery_unavailable") {
		t.Fatalf("wrong-source file evidence satisfied acceptance: %v", err)
	}
}

func TestCollaborationDeliveryProjectionRequiresExactSubmissionHandoff(t *testing.T) {
	for _, field := range []string{"from_user_id", "to_user_id", "event_target"} {
		t.Run(field, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), handoffAcceptance(t, f, 1, "A result with an exact author and recipient"))
			if err != nil {
				t.Fatal(err)
			}
			if field == "event_target" {
				f.fx.Exec(t, `UPDATE collaboration_event SET payload=jsonb_set(payload,'{target_user_id}',to_jsonb($2::text)) WHERE issue_id=$1 AND kind='acceptance_submitted'`, f.issue.Issue.ID, f.outsider)
			} else {
				// The selected column is one of the two fixed test cases above.
				f.fx.Exec(t, `UPDATE issue_handoff SET `+field+`=$2 WHERE id=$1`, r.DeliveryConfirmation.SubmissionID, f.outsider)
			}
			access := mustAccess(t, f, f.ctx)
			view, err := f.svc.DeliveryView(f.ctx, access)
			if err != nil || view != nil {
				t.Fatalf("mismatched %s became a trusted delivery: %+v %v", field, view, err)
			}
			if _, err = f.svc.ListAcceptanceSubmissions(f.ctx, f.issue.Issue.ID, CollaborationDeliveryHistoryQuery{Limit: 10}); !errors.Is(err, errCollaborationDeliveryEvidence) {
				t.Fatalf("history trusted a mismatched submission identity: %v", err)
			}
		})
	}
}

func TestCollaborationDeliveryProjectionRequiresInitiatorDecision(t *testing.T) {
	for _, kind := range []string{"completed", "cancelled", "acceptance_returned"} {
		t.Run(kind, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), handoffAcceptance(t, f, 1, "A result requiring its initiator's decision"))
			if err != nil {
				t.Fatal(err)
			}
			id := r.DeliveryConfirmation.SubmissionID
			if kind == "acceptance_returned" {
				_, err = f.svc.ReturnAcceptance(f.ctx, deliveryReturnParams(t, f, 2, id, f.handler, "Please revise the result"))
			} else {
				action, status := "complete", "done"
				if kind == "cancelled" {
					action, status = "cancel", "cancelled"
				}
				p := lifecycleParams(t, f, action, status, 2, 1)
				if action == "complete" {
					p.SubmissionID = &id
				}
				_, err = f.svc.ChangeLifecycle(f.ctx, p)
			}
			if err != nil {
				t.Fatal(err)
			}
			f.fx.Exec(t, `UPDATE collaboration_event SET actor_user_id=$2 WHERE issue_id=$1 AND kind=$3`, f.issue.Issue.ID, f.handler, kind)
			view := deliverySummaryForTest(t, f, f.ctx)
			if view.Latest == nil || view.Latest.EvidenceState != "unavailable" || view.Latest.Decision != nil {
				t.Fatalf("a noninitiator %s event invented a decision: %+v", kind, view)
			}
		})
	}
}

func TestCollaborationDeliveryProjectionRequiresExactRevisionHandoff(t *testing.T) {
	for _, field := range []string{"handoff_id", "event_target", "actor_user_id", "from_user_id", "to_user_id", "purpose", "note"} {
		t.Run(field, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			r, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), handoffAcceptance(t, f, 1, "A result with a later revision decision"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.ReturnAcceptance(f.ctx, deliveryReturnParams(t, f, 2, r.DeliveryConfirmation.SubmissionID, f.handler, "The exact revision reason")); err != nil {
				t.Fatal(err)
			}
			var handoff string
			f.fx.QueryRow(t, `SELECT payload->>'handoff_id' FROM collaboration_event WHERE issue_id=$1 AND kind='acceptance_returned'`, f.issue.Issue.ID).Scan(&handoff)
			switch field {
			case "handoff_id":
				f.fx.Exec(t, `UPDATE collaboration_event SET payload=payload-'handoff_id' WHERE issue_id=$1 AND kind='acceptance_returned'`, f.issue.Issue.ID)
			case "event_target":
				f.fx.Exec(t, `UPDATE collaboration_event SET payload=jsonb_set(payload,'{target_user_id}',to_jsonb($2::text)) WHERE issue_id=$1 AND kind='acceptance_returned'`, f.issue.Issue.ID, f.outsider)
			case "purpose":
				f.fx.Exec(t, `UPDATE issue_handoff SET purpose='information' WHERE id=$1`, util.MustParseUUID(handoff))
			case "note":
				f.fx.Exec(t, `UPDATE issue_handoff SET note='A different handoff reason' WHERE id=$1`, util.MustParseUUID(handoff))
			default:
				f.fx.Exec(t, `UPDATE issue_handoff SET `+field+`=$2 WHERE id=$1`, util.MustParseUUID(handoff), f.outsider)
			}
			view := deliverySummaryForTest(t, f, f.ctx)
			if view.Latest == nil || view.Latest.EvidenceState != "unavailable" || view.Latest.Decision != nil {
				t.Fatalf("mismatched revision %s became a trusted decision: %+v", field, view)
			}
		})
	}
}
