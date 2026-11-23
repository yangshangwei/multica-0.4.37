package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationActivityRetainsExactHandoffAndEditHistory(t *testing.T) {
	f := newCollaborationContentFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	request := CollaborationWorkflowRequest{CollaborationQueueRequest: f.input("").CollaborationQueueRequest, ExpectedCollaborationRevision: 1}
	transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(f.creator)}, Note: "Retained handoff note"}
	if receipt, err := f.svc.Transfer(f.ctx, transfer); err != nil || receipt.Code != "transferred" {
		t.Fatalf("transfer: %+v %v", receipt, err)
	}
	if _, err := f.svc.Transfer(f.ctx, transfer); err != nil {
		t.Fatal(err)
	}
	title := "Edited title"
	request.RequestID, request.ExpectedCollaborationRevision = uuid.NewString(), 2
	if receipt, err := f.svc.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: request, Title: &title}); err != nil || receipt.Code != "issue_edited" {
		t.Fatalf("edit: %+v %v", receipt, err)
	}
	page, err := f.svc.ListActivity(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2})
	if err != nil || !page.HasMore || len(page.Items) != 2 {
		t.Fatalf("activity page: %+v %v", page, err)
	}
	edit, handoff := page.Items[0], page.Items[1]
	if edit.Kind != "issue_edited" || len(edit.Changes) != 1 || edit.Changes[0].Field != "title" || *edit.Changes[0].After != title || *edit.Changes[0].Before != f.issue.Issue.Title {
		t.Fatalf("lost before/after values: %+v", edit)
	}
	if handoff.Kind != "transferred" || handoff.Handoff == nil || handoff.Handoff.Note != transfer.Note || handoff.Handoff.From.UserID != util.MustParseUUID(f.handler) || handoff.Handoff.To.UserID != util.MustParseUUID(f.creator) {
		t.Fatalf("lost exact handoff provenance: %+v", handoff)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatal("replay created another handoff")
	}
	next, err := f.svc.ListActivity(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2, BeforeSeq: pgtype.Int8{Int64: handoff.EventSeq, Valid: true}})
	if err != nil || next.HasMore || len(next.Items) != 1 || next.Items[0].Kind != "created" || next.Items[0].EventSeq >= handoff.EventSeq {
		t.Fatalf("unstable history pagination: %+v %v", next, err)
	}
	encoded, err := json.Marshal(page.Items)
	if err != nil || strings.Contains(string(encoded), "@example.test") || strings.Contains(string(encoded), `"receipt"`) || strings.Contains(string(encoded), `"request_id"`) {
		t.Fatalf("private implementation fields leaked: %v", err)
	}
}

func TestCollaborationActivityRejectsLostAuthorityAndRedactsUnknownPayloads(t *testing.T) {
	f := newCollaborationContentFixture(t)
	f.fx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "event_seq": int64(9007199254740993), "actor_user_id": f.creator, "kind": "future_event", "payload": json.RawMessage(`{"private_body":"DO_NOT_PROJECT","token":"DO_NOT_PROJECT"}`)})
	f.fx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "event_seq": int64(9007199254740994), "actor_user_id": f.creator, "kind": "issue_edited", "payload": json.RawMessage(`{"before":{"title":{"secret":"DO_NOT_PROJECT"}},"after":{"title":"DO_NOT_PROJECT"}}`)})
	page, err := f.svc.ListActivity(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Items[0].DetailsAvailable || page.Items[1].DetailsAvailable || page.Items[1].Kind != "other" {
		t.Fatalf("unknown/malformed activity became readable detail: %+v %v", page, err)
	}
	encoded, err := json.Marshal(page.Items)
	if err != nil || strings.Contains(string(encoded), "DO_NOT_PROJECT") || !strings.Contains(string(encoded), `"9007199254740994"`) {
		t.Fatal("event projection leaked payload or rounded its version")
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if _, err := f.svc.ListActivity(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("removed external participant read history: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
	if _, err := f.svc.ListActivity(machine, f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("machine read history through creator attribution: %v", err)
	}
	if _, err := f.svc.ListActivity(f.ctx, f.issue.Issue.ID, CollaborationActivityQuery{Limit: 2}); err != nil {
		t.Fatalf("independent source member lost history: %v", err)
	}
}
