package service

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

type collaborationContentFixture struct {
	*collaborationFixture
	issue CollaborationIssueAccess
}

func newCollaborationContentFixture(t *testing.T) *collaborationContentFixture {
	t.Helper()
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id=$1`, issue.SessionID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_batch WHERE session_id=$1`, issue.SessionID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_command WHERE session_id=$1`, issue.SessionID)
	return &collaborationContentFixture{collaborationFixture: f, issue: issue}
}

func (f *collaborationContentFixture) input(content string, targets ...pgtype.UUID) CollaborationCreateCommentParams {
	p := CollaborationCreateCommentParams{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: f.issue.Issue.ID,
		RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1},
		CollaborationCommentInput: CollaborationCommentInput{Content: content}}
	for _, id := range targets {
		p.TargetVersions = append(p.TargetVersions, CollaborationCommentTargetVersion{TargetID: id, ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1})
	}
	return p
}

func TestCollaborationCommentCommitPreservesValidTargetAndHumanWhenAnotherTargetWasRevoked(t *testing.T) {
	f := newCollaborationContentFixture(t)
	a, grantA := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	b, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	p := f.input("[@A](mention://agent/"+util.UUIDToString(a.ResourceID)+") [@B](mention://agent/"+util.UUIDToString(b.ResourceID)+") [@H](mention://member/"+f.handler+")", a.ID, b.ID)
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, p.CollaborationCommentInput)
	if err != nil || len(preview.Targets) != 2 || preview.Targets[0].State != "waiting" || preview.Targets[1].State != "waiting" {
		t.Fatalf("initial preview: %+v %v", preview, err)
	}
	f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),revocation_reason='fixture_revocation' WHERE id=$1`, grantA.ID)
	created, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || created.Code != "comment_created" || len(created.Targets) != 2 || created.Targets[0].State != "blocked" ||
		created.Targets[0].CommandID.Valid || created.Targets[1].State != "waiting" || !created.Targets[1].BatchID.Valid {
		t.Fatalf("invalid A swallowed B or comment: %+v %v", created, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='comment_created'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1 AND input_event_id=$2 AND content=$3 AND author_user_id=$4 AND state='waiting'`, b.ID, created.EventID, p.Content, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, a.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=$2 AND recipient_user_id=$3 AND is_mention AND source_kind='comment' AND source_id=$4`, f.issue.Issue.ID, created.EventSeq, f.handler, created.CommentID) != 1 {
		t.Fatal("comment/input/human notification did not commit exactly once")
	}
	// A replay freezes the original recipient set instead of notifying people
	// who joined after this comment was committed.
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
	replay, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || replay.CommentID != created.CommentID || replay.EventID != created.EventID ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=$2`, f.issue.Issue.ID, created.EventSeq) != 1 {
		t.Fatalf("request replay changed input/recipients: %+v %v", replay, err)
	}
	p.Content += " changed retry payload"
	if _, err = f.svc.CreateComment(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed retry was silently accepted: %v", err)
	}
}

func TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing(t *testing.T) {
	f := newCollaborationContentFixture(t)
	target, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	mention := "[@A](mention://agent/" + util.UUIDToString(target.ResourceID) + ") "
	firstInput := f.input(mention+"first", target.ID)
	first, err := f.svc.CreateComment(f.ctx, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := f.input(mention+"second", target.ID)
	second, err := f.svc.CreateComment(collaborationContext(f.handler), secondInput)
	if err != nil || first.Targets[0].BatchID != second.Targets[0].BatchID || first.Targets[0].ChannelID != second.Targets[0].ChannelID ||
		first.Targets[0].CommandID == second.Targets[0].CommandID {
		t.Fatalf("two inputs must share one pending batch and retain individual identities: %+v %v", second, err)
	}
	if _, err = f.svc.EditComment(f.ctx, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: first.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: "edited display only"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeleteComment(f.ctx, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: first.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 2}); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND content=$2 AND author_user_id=$3 AND state='waiting'`, first.Targets[0].CommandID, firstInput.Content, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND content=$2 AND author_user_id=$3 AND state='waiting'`, second.Targets[0].CommandID, secondInput.Content, f.handler) != 1 {
		t.Fatal("batch append or display edit lost immutable input/author")
	}
}

func TestCollaborationCommentCommitFailureRollsBackEveryEffect(t *testing.T) {
	for _, query := range []string{"CreateCollaborationComment", "CreateCollaborationCommand", "CreateCollaborationContentEvent", "CreateCollaborationContentNotification", "CompleteCollaborationOperation"} {
		t.Run(query, func(t *testing.T) {
			f := newCollaborationContentFixture(t)
			target, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			p := f.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") [@H](mention://member/"+f.handler+")", target.ID)
			f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: query}
			if _, err := f.svc.CreateComment(f.ctx, p); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("required create write fault not returned: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='comment_created'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='comment_created'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("failed comment committed a partial input, event or notification")
			}
		})
	}
}

func (f *collaborationContentFixture) comment(t *testing.T, author, content string, parent pgtype.UUID) pgtype.UUID {
	t.Helper()
	cols := testutil.Cols{"issue_id": f.issue.Issue.ID, "workspace_id": f.workspace, "author_type": "member", "author_id": author,
		"content": content, "type": "comment", "dispatch_eligible": false}
	if parent.Valid {
		cols["parent_id"] = parent
	}
	return util.MustParseUUID(f.fx.Insert(t, "comment", cols))
}

func TestCollaborationCommentAuthorEditRetainsHistoryAndDeletedReplyAnchor(t *testing.T) {
	f := newCollaborationContentFixture(t)
	comment := f.comment(t, f.handler, "original human input", pgtype.UUID{})
	reply := f.comment(t, f.creator, "reply remains", comment)
	p := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(),
		Content: "edited [@creator](mention://member/" + f.creator + ") @AI", ExpectedCommentRevision: 1}
	if _, err := f.svc.EditComment(f.ctx, p); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("creator rewrote another person's comment: %v", err)
	}
	ctx := collaborationContext(f.handler)
	first, err := f.svc.EditComment(ctx, p)
	if err != nil || first.Code != "comment_edited" || first.CommentRevision != 2 {
		t.Fatalf("author edit: %+v %v", first, err)
	}
	if _, err = f.svc.EditComment(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.RequestID, p.Content, p.ExpectedCommentRevision = uuid.NewString(), "", 2
	deleted, err := f.svc.DeleteComment(ctx, p)
	if err != nil || deleted.CommentRevision != 3 {
		t.Fatalf("author delete: %+v %v", deleted, err)
	}
	view, err := f.svc.GetComment(f.ctx, f.issue.Issue.ID, comment)
	if err != nil || view.DeletedAt == nil || view.Content != "" || view.CanEdit {
		t.Fatalf("missing tombstone: %+v %v", view, err)
	}
	child, err := f.svc.GetComment(ctx, f.issue.Issue.ID, reply)
	if err != nil || child.ParentID != comment || child.Content != "reply remains" {
		t.Fatalf("deletion broke reply anchor: %+v %v", child, err)
	}
	history, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, comment)
	if err != nil || len(history) != 2 || history[0].Before == nil || *history[0].Before != "original human input" ||
		history[0].After != "edited [@creator](mention://member/"+f.creator+") @AI" || history[1].After != "" ||
		history[0].Actor == nil || history[0].Actor.UserID != util.MustParseUUID(f.handler) {
		t.Fatalf("history lost actor or content: %+v %v", history, err)
	}
	p.RequestID, p.Content, p.ExpectedCommentRevision = uuid.NewString(), "revive deleted text", 3
	if _, err = f.svc.EditComment(ctx, p); !collaborationErrorCode(err, "comment_deleted") {
		t.Fatalf("deleted comment revived: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added' AND recipient_user_id=$2 AND source_id=$3`, f.issue.Issue.ID, f.creator, comment) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("edit/delete repeated a mention or executed @AI")
	}
}

func TestCollaborationCommentEditsSerializeAndUnknownCommitDoesNotRepeat(t *testing.T) {
	f := newCollaborationContentFixture(t)
	comment := f.comment(t, f.creator, "original", pgtype.UUID{})
	start := make(chan struct{})
	results := make(chan CollaborationCommentReceipt, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, content := range []string{"first edit", "second edit"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, err := f.svc.EditComment(f.ctx, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment,
				RequestID: uuid.NewString(), Content: content, ExpectedCommentRevision: 1})
			results <- r
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	success, stale := 0, 0
	for receipt := range results {
		if receipt.Status == http.StatusOK {
			success++
		} else if receipt.Code == "stale_comment_revision" {
			stale++
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("concurrent edits were not fenced: success=%d stale=%d", success, stale)
	}
	for err := range errs {
		if err != nil && !collaborationErrorCode(err, "stale_comment_revision") {
			t.Fatal(err)
		}
	}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	p := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(), Content: "saved despite lost reply", ExpectedCommentRevision: 2}
	if _, err := f.svc.EditComment(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain commit retried the mutation: begins=%d err=%v", fault.begins.Load(), err)
	}
	f.svc.TxStarter = f.pool
	receipt, err := f.svc.EditComment(f.ctx, p)
	if err != nil || receipt.CommentRevision != 3 {
		t.Fatalf("original key failed to recover: %+v %v", receipt, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='comment_edited'`, f.issue.Issue.ID) != 2 {
		t.Fatal("unknown commit duplicated history")
	}
}

func TestCollaborationCommentEditFailureRollsBackBodyHistoryAndNotification(t *testing.T) {
	for _, query := range []string{"EditCollaborationComment", "CreateCollaborationContentEvent", "CreateCollaborationContentNotification", "CompleteCollaborationOperation"} {
		t.Run(query, func(t *testing.T) {
			f := newCollaborationContentFixture(t)
			comment := f.comment(t, f.creator, "original", pgtype.UUID{})
			f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: query}
			p := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(),
				Content: "[@H](mention://member/" + f.handler + ")", ExpectedCommentRevision: 1}
			if _, err := f.svc.EditComment(f.ctx, p); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatalf("required persistence fault not returned: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND content='original' AND revision=1`, comment) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='comment_edited'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("failure left a partial comment edit")
			}
		})
	}
}

func TestCollaborationCommentSourceMemberAndPlainRepliesStayHumanOnly(t *testing.T) {
	f := newCollaborationContentFixture(t)
	target, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	p := f.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") [@H](mention://member/"+f.handler+")", target.ID)
	p.ExpectedParticipantEpoch, p.ExpectedAccountEpoch = 0, 0
	created, err := f.svc.CreateComment(collaborationContext(f.outsider), p)
	if err != nil || len(created.Targets) != 1 || created.Targets[0].State != "blocked" || created.Targets[0].Code != "not_participant" {
		t.Fatalf("source membership was not limited to a human comment: %+v %v", created, err)
	}
	parent := util.MustParseUUID(f.fx.Insert(t, "comment", testutil.Cols{"issue_id": f.issue.Issue.ID, "workspace_id": f.workspace,
		"author_type": "agent", "author_id": target.ResourceID, "type": "comment", "content": "Earlier agent output", "dispatch_eligible": false}))
	reply := f.input("A plain reply and unresolved @AI", target.ID)
	reply.ParentID = parent
	result, err := f.svc.CreateComment(f.ctx, reply)
	if err != nil || len(result.Targets) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatalf("reply/unused target versions implicitly queued execution: %+v %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=$2`, f.issue.Issue.ID, created.EventSeq) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.outsider) != 0 {
		t.Fatal("source comment changed participant authority or lost human notifications")
	}
}

func TestCollaborationCommentEditMentionDeltaUsesCurrentParticipants(t *testing.T) {
	f := newCollaborationContentFixture(t)
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
	old := "[@H](mention://member/" + f.handler + ")"
	comment := f.comment(t, f.creator, old, pgtype.UUID{})
	p := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(), ExpectedCommentRevision: 1,
		Content: old + " [@new](mention://member/" + f.outsider + ") [@self](mention://member/" + f.creator + ")"}
	if _, err := f.svc.EditComment(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added' AND recipient_user_id=$2`, f.issue.Issue.ID, f.outsider) != 1 {
		t.Fatal("comment edit notified an existing mention or the editor")
	}
	p.RequestID, p.ExpectedCommentRevision, p.Content = uuid.NewString(), 2, "[@all](mention://all/all)"
	if _, err := f.svc.EditComment(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	joined := f.fx.User(t, "Later participant", "later-"+uuid.NewString()+"@example.test")
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": joined}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, joined)
	p.RequestID, p.ExpectedCommentRevision, p.Content = uuid.NewString(), 3, "[@all](mention://all/all) changed surrounding prose"
	if _, err := f.svc.EditComment(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='mention_added'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("editing an old @all re-expanded recipients or executed AI")
	}
}
