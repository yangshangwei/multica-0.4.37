package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationBodyEditNotifiesOnlyAddedHumanMentions(t *testing.T) {
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue.Issue.ID)
	p := CollaborationEditParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{
		CollaborationQueueRequest: CollaborationQueueRequest{IssueID: issue.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1,
			ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}, ExpectedCollaborationRevision: 1},
		Description: editString("[@H](mention://member/" + f.handler + ") [@outside](mention://member/" + f.outsider + ") @AI")}
	if _, err = f.svc.EditIssue(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.EditIssue(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.ListNotifications(collaborationContext(f.handler), CollaborationNotificationsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var mentions []CollaborationNotificationView
	for _, item := range page.Items {
		if item.Kind == "mention_added" {
			mentions = append(mentions, item)
		}
	}
	if len(mentions) != 1 || !mentions[0].IsMention || mentions[0].SourceKind != "body" || !mentions[0].SourceID.Valid || !mentions[0].CanOpen {
		t.Fatalf("body mention needs one durable, located notification: %+v", mentions)
	}
	p.RequestID, p.ExpectedCollaborationRevision = uuid.NewString(), 2
	p.Description = editString(*p.Description + " changed surrounding prose")
	if _, err = f.svc.EditIssue(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added'`, issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, issue.Issue.ID, f.outsider) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, issue.SessionID) != 0 {
		t.Fatal("an unchanged/invalid mention notified, enrolled, or executed")
	}
}

func TestCollaborationNotificationsLoseContentAndAnchorsWithAccess(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	p.Title = "private-notification-title-" + uuid.NewString()
	issue, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(f.fx.Insert(t, "collaboration_notification", testutil.Cols{
		"recipient_user_id": f.handler, "issue_id": issue.Issue.ID, "event_seq": 2,
		"kind": "comment_created", "is_mention": true, "source_kind": "comment", "source_id": uuid.NewString()}))
	ctx := collaborationContext(f.handler)
	before, err := f.svc.ListNotifications(ctx, CollaborationNotificationsQuery{Limit: 100})
	if err != nil || len(before.Items) != 2 || before.UnreadCount != 2 {
		t.Fatalf("offline notifications not readable: %+v %v", before, err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue.Issue.ID, f.handler)
	after, err := f.svc.ListNotifications(ctx, CollaborationNotificationsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range after.Items {
		if item.CanOpen || item.IssueTitle != nil || item.SourceID.Valid || item.SourceKind != "issue" {
			t.Fatalf("retained notification restored revoked object access: %+v", item)
		}
	}
	encoded, err := json.Marshal(after.Items)
	if err != nil || strings.Contains(string(encoded), p.Title) {
		t.Fatal("revoked notification serialized the private title")
	}
	first, err := f.svc.SetNotificationState(ctx, id, "read")
	if err != nil || first.ReadAt == nil {
		t.Fatalf("own content-free receipt must remain markable: %+v %v", first, err)
	}
	replay, err := f.svc.SetNotificationState(ctx, id, "read")
	if err != nil || !replay.ReadAt.Equal(*first.ReadAt) {
		t.Fatalf("read retry changed durable timestamp: %+v %v", replay, err)
	}
	if _, err = f.svc.SetNotificationState(f.ctx, id, "archive"); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("another account mutated recipient state: %v", err)
	}
	if _, err = f.svc.SetNotificationState(ctx, id, "archive"); err != nil {
		t.Fatal(err)
	}
	machine := auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: f.handler, Kind: auth.CredentialTask})
	if _, err = f.svc.ListNotifications(machine, CollaborationNotificationsQuery{Limit: 100}); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("machine inherited recipient notifications: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, issue.Issue.ID, f.handler) != 1 {
		t.Fatal("notification actions changed human responsibility")
	}
}
