package service

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationDiscussionResolveUnresolveUsesCurrentReadBasis(t *testing.T) {
	for _, basis := range []string{"participant", "source_member"} {
		t.Run(basis, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationContentFixture(t)
			actor := f.handler
			if basis == "source_member" {
				actor = f.outsider
				f.fx.Member(t, f.workspace, actor, "member")
				if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, actor) != 0 {
					t.Fatal("source-only resolver unexpectedly has participant authority")
				}
			} else if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor) != 0 {
				t.Fatal("external participant unexpectedly has source membership")
			}
			ctx := collaborationContext(actor)
			target, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			input := f.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") retained external human instruction", target.ID)
			comment, err := f.svc.CreateComment(collaborationContext(f.handler), input)
			if err != nil || len(comment.Targets) != 1 || !comment.Targets[0].CommandID.Valid {
				t.Fatalf("retained AI input fixture: %+v %v", comment, err)
			}
			command := comment.Targets[0]
			seedCollaborationGroupRun(t, f.fx, f.issue.SessionID, CollaborationQueueReceipt{CommandID: command.CommandID, TargetID: target.ID,
				ChannelID: command.ChannelID, BatchID: command.BatchID, CommandSequence: command.CommandSequence}, 1)
			beforeHistory, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, comment.CommentID)
			if err != nil || len(beforeHistory) != 1 {
				t.Fatalf("retained content history fixture: %+v %v", beforeHistory, err)
			}
			protected := func() string {
				var state string
				f.fx.QueryRow(t, `SELECT jsonb_build_object(
					'comment',(SELECT to_jsonb(c)-'resolved_at'-'resolved_by_type'-'resolved_by_id' FROM comment c WHERE c.id=$1),
					'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$2),
					'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$2),
					'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE s.id=$3),
					'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$3),
					'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$3),
					'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$3),
					'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$3),
					'audit',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$2 AND e.kind NOT LIKE 'discussion_%'),
					'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$2),
					'inbox',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM inbox_item i WHERE i.issue_id=$2),
					'legacy_tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id=$2)
					)::text`, comment.CommentID, f.issue.Issue.ID, f.issue.SessionID).Scan(&state)
				return state
			}
			before := protected()
			check := func(resolver string, events int) {
				t.Helper()
				view, err := f.svc.GetComment(f.ctx, f.issue.Issue.ID, comment.CommentID)
				if err != nil || view.Content != input.Content || view.CommentRevision != 1 {
					t.Fatalf("discussion changed the immutable comment: %+v %v", view, err)
				}
				if resolver == "" {
					if view.ResolvedAt != nil || view.ResolvedBy != nil || f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND resolved_at IS NULL AND resolved_by_type IS NULL AND resolved_by_id IS NULL`, comment.CommentID) != 1 {
						t.Fatal("successful unresolve did not clear the exact resolution metadata")
					}
				} else if view.ResolvedAt == nil || view.ResolvedBy == nil || util.UUIDToString(view.ResolvedBy.UserID) != resolver ||
					f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND resolved_at IS NOT NULL`, f.issue.Issue.ID) != 1 {
					t.Fatalf("resolution lost its real actor or duplicated the thread record: %+v", view)
				}
				history, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, comment.CommentID)
				if err != nil || !reflect.DeepEqual(history, beforeHistory) || protected() != before ||
					f.fx.Count(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2`, f.workspace, f.issue.Issue.ID) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind IN ('discussion_comment_resolve','discussion_comment_unresolve')`, f.issue.Issue.ID) != events {
					t.Fatal("resolution/replay changed content history, AI, audit, notifications or source inbox")
				}
			}
			resolve := CollaborationDiscussionParams{IssueID: f.issue.Issue.ID, CommentID: comment.CommentID, RequestID: uuid.NewString(), Action: "resolve"}
			resolved, err := f.svc.ChangeDiscussion(ctx, resolve)
			if err != nil || resolved.Code != "comment_resolved" || resolved.Status != http.StatusOK || resolved.Emoji != nil {
				t.Fatalf("authorized %s resolve: %+v %v", basis, resolved, err)
			}
			check(actor, 1)
			unresolve := resolve
			unresolve.Action, unresolve.RequestID = "unresolve", uuid.NewString()
			unresolved, err := f.svc.ChangeDiscussion(ctx, unresolve)
			if err != nil || unresolved.Code != "comment_unresolved" || unresolved.Status != http.StatusOK || unresolved.Emoji != nil {
				t.Fatalf("authorized %s unresolve: %+v %v", basis, unresolved, err)
			}
			check("", 2)
			for _, original := range []struct {
				params  CollaborationDiscussionParams
				receipt CollaborationDiscussionReceipt
			}{{resolve, resolved}, {unresolve, unresolved}} {
				replay, err := f.svc.ChangeDiscussion(ctx, original.params)
				if err != nil || !reflect.DeepEqual(replay, original.receipt) {
					t.Fatalf("original discussion action changed on replay: %+v %v", replay, err)
				}
				own, err := f.svc.GetDiscussionOperation(ctx, f.issue.Issue.ID, original.params.RequestID)
				if err != nil || !reflect.DeepEqual(own, original.receipt) {
					t.Fatalf("own discussion recovery lost the exact action: %+v %v", own, err)
				}
				check("", 2)
			}
			later := resolve
			later.RequestID = uuid.NewString()
			if _, err = f.svc.ChangeDiscussion(f.ctx, later); err != nil {
				t.Fatal(err)
			}
			if replay, err := f.svc.ChangeDiscussion(ctx, unresolve); err != nil || !reflect.DeepEqual(replay, unresolved) {
				t.Fatalf("old unresolve outcome was recomputed: %+v %v", replay, err)
			}
			check(f.creator, 3)
			fresh := unresolve
			fresh.RequestID = uuid.NewString()
			freshReceipt, err := f.svc.ChangeDiscussion(ctx, fresh)
			if err != nil || freshReceipt.Code != "comment_unresolved" {
				t.Fatalf("fresh explicit unresolve: %+v %v", freshReceipt, err)
			}
			check("", 4)
			if basis == "source_member" {
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor)
			} else {
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, actor)
			}
			operations := f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID)
			for _, requestID := range []string{fresh.RequestID, uuid.NewString()} {
				denied := fresh
				denied.RequestID = requestID
				if _, err = f.svc.ChangeDiscussion(ctx, denied); !collaborationErrorCode(err, "not_found") {
					t.Fatalf("lost %s authority reused or reserved an unresolve: %v", basis, err)
				}
			}
			machine := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialTask})
			if _, err = f.svc.ChangeDiscussion(machine, fresh); !collaborationErrorCode(err, "forbidden") {
				t.Fatalf("machine identity inherited the human resolution: %v", err)
			}
			own, err := f.svc.GetDiscussionOperation(ctx, f.issue.Issue.ID, fresh.RequestID)
			if err != nil || !reflect.DeepEqual(own, freshReceipt) || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != operations {
				t.Fatalf("lost access erased own outcome or wrote a partial action: %+v %v", own, err)
			}
			check("", 4)
		})
	}
}
