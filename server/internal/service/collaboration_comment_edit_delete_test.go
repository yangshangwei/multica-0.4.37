package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory(t *testing.T) {
	for _, first := range []string{"edit", "delete"} {
		t.Run(first+"_first", func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationContentFixture(t)
			t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
			t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
			f.svc.Storage = storage.NewLocalStorageFromEnv()
			upload := attachmentUploadParams(f.collaborationFixture)
			upload.SourceWorkspaceID, upload.IssueID = pgtype.UUID{}, f.issue.Issue.ID
			attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
			if err != nil {
				t.Fatal(err)
			}
			f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, attachment.ID)
			f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, attachment.ID)
			target, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			input := f.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") immutable original input", target.ID)
			input.DraftID, input.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
			created, err := f.svc.CreateComment(f.ctx, input)
			if err != nil || len(created.Targets) != 1 || !created.Targets[0].CommandID.Valid {
				t.Fatalf("attached immutable input fixture: %+v %v", created, err)
			}
			command := created.Targets[0]
			run := seedCollaborationGroupRun(t, f.fx, f.issue.SessionID, CollaborationQueueReceipt{CommandID: command.CommandID, TargetID: target.ID,
				ChannelID: command.ChannelID, BatchID: command.BatchID, CommandSequence: command.CommandSequence}, 1)
			reply := f.comment(t, f.handler, "reply retains its deleted parent anchor", created.CommentID)
			protected := func() string {
				var state string
				f.fx.QueryRow(t, `SELECT jsonb_build_object(
					'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
					'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
					'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE s.id=$2),
					'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$2),
					'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$2),
					'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$2),
					'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$2),
					'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.command_id) FROM collaboration_group_input i WHERE i.group_id=$3),
					'dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.command_id) FROM collaboration_group_dependency d WHERE d.group_id=$3),
					'audit',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$1 AND e.kind NOT IN ('comment_edited','comment_deleted')),
					'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1),
					'legacy_tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id=$1),
					'attachments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM collaboration_attachment a WHERE a.issue_id=$1),
					'frozen_references',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.reference_kind,r.reference_id,r.reference_revision)
						FROM collaboration_attachment_reference r WHERE r.attachment_id=$4 AND r.reference_kind<>'comment'),
					'reply',(SELECT to_jsonb(c) FROM comment c WHERE c.id=$5)
					)::text`, f.issue.Issue.ID, f.issue.SessionID, run.GroupID, attachment.ID, reply).Scan(&state)
				return state
			}
			before := protected()
			edit := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: created.CommentID, RequestID: uuid.NewString(),
				ExpectedCommentRevision: 1, Content: "edited human display only"}
			deletion := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: created.CommentID, RequestID: uuid.NewString(), ExpectedCommentRevision: 1}
			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			type outcome struct {
				receipt CollaborationCommentReceipt
				err     error
			}
			writeEdit := func(s *CollaborationService) outcome {
				r, err := s.EditComment(ctx, edit)
				return outcome{r, err}
			}
			writeDelete := func(s *CollaborationService) outcome {
				r, err := s.DeleteComment(ctx, deletion)
				return outcome{r, err}
			}
			leading, following := writeEdit, writeDelete
			if first == "delete" {
				leading, following = writeDelete, writeEdit
			}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			firstDone := make(chan outcome, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case result := <-firstDone:
				t.Fatalf("leading %s failed before its locked commit: %+v %v", first, result.receipt, result.err)
			case <-ctx.Done():
				t.Fatal("leading content writer did not reach its commit barrier")
			}
			secondDone := make(chan outcome, 1)
			blockedDone := make(chan error, 1)
			go func() {
				result := following(f.svc)
				secondDone <- result
				blockedDone <- result.err
			}()
			waitFIFOBlocked(t, &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: f.issue, targetID: target.ID}, blocker, blockedDone)
			unblock()
			lead, follow := <-firstDone, <-secondDone
			if lead.err != nil || lead.receipt.Status != http.StatusOK || lead.receipt.CommentRevision != 2 {
				t.Fatalf("leading content write did not commit exactly once: %+v %v", lead.receipt, lead.err)
			}
			expectedRefusal := "stale_comment_revision"
			if first == "delete" {
				expectedRefusal = "comment_deleted"
			}
			if !collaborationErrorCode(follow.err, expectedRefusal) || follow.receipt.Code != expectedRefusal || follow.receipt.Status != http.StatusConflict || follow.receipt.CommentRevision != 2 {
				t.Fatalf("following content write missed the committed boundary: %+v %v", follow.receipt, follow.err)
			}
			editOutcome, deleteOutcome := lead, follow
			if first == "delete" {
				editOutcome, deleteOutcome = follow, lead
			}
			// Omitted empty attachment lists compare by the persisted wire;
			// nil and allocated empty slices encode the same original receipt.
			sameReceipt := func(a, b CollaborationCommentReceipt) bool {
				t.Helper()
				first, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				second, err := json.Marshal(b)
				if err != nil {
					t.Fatal(err)
				}
				return bytes.Equal(first, second)
			}
			for _, original := range []struct {
				write func(*CollaborationService) outcome
				want  outcome
				key   string
			}{{writeEdit, editOutcome, edit.RequestID}, {writeDelete, deleteOutcome, deletion.RequestID}} {
				replay := original.write(f.svc)
				if !sameReceipt(replay.receipt, original.want.receipt) || (original.want.err == nil && replay.err != nil) ||
					(original.want.err != nil && !collaborationErrorCode(replay.err, expectedRefusal)) {
					t.Fatalf("original-key content retry recomputed its outcome: %+v %v", replay.receipt, replay.err)
				}
				own, err := f.svc.GetCommentOperation(f.ctx, f.issue.Issue.ID, original.key)
				if err != nil || !sameReceipt(own, original.want.receipt) {
					t.Fatalf("original content operation lost its exact result: %+v %v", own, err)
				}
			}
			wantRevision, wantHistory := int64(2), 2
			finalDeletionKey := deletion.RequestID
			if first == "edit" {
				fresh := deletion
				fresh.RequestID, fresh.ExpectedCommentRevision = uuid.NewString(), 2
				deleted, err := f.svc.DeleteComment(f.ctx, fresh)
				if err != nil || deleted.Code != "comment_deleted" || deleted.CommentRevision != 3 {
					t.Fatalf("explicit fresh delete after stale refusal: %+v %v", deleted, err)
				}
				wantRevision, wantHistory = 3, 3
				finalDeletionKey = fresh.RequestID
			}
			view, err := f.svc.GetComment(f.ctx, f.issue.Issue.ID, created.CommentID)
			if err != nil || view.DeletedAt == nil || view.Content != "" || view.CommentRevision != wantRevision || view.CanEdit || len(view.AttachmentIDs) != 0 {
				t.Fatalf("edit/delete race failed to retain its final tombstone: %+v %v", view, err)
			}
			history, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, created.CommentID)
			if err != nil || len(history) != wantHistory || history[0].Kind != "comment_created" || history[0].After != input.Content || history[0].CommentRevision != 1 ||
				!reflect.DeepEqual(history[0].AfterAttachmentIDs, input.AttachmentIDs) {
				t.Fatalf("race erased the original input history: %+v %v", history, err)
			}
			last := history[len(history)-1]
			beforeDelete := input.Content
			if first == "edit" {
				beforeDelete = edit.Content
				if history[1].Kind != "comment_edited" || history[1].Before == nil || *history[1].Before != input.Content || history[1].After != edit.Content || history[1].CommentRevision != 2 ||
					!reflect.DeepEqual(history[1].BeforeAttachmentIDs, input.AttachmentIDs) || !reflect.DeepEqual(history[1].AfterAttachmentIDs, input.AttachmentIDs) {
					t.Fatalf("edit-first history lost its exact transition: %+v", history[1])
				}
			}
			if last.Kind != "comment_deleted" || last.Before == nil || *last.Before != beforeDelete || last.After != "" || last.CommentRevision != wantRevision ||
				!reflect.DeepEqual(last.BeforeAttachmentIDs, input.AttachmentIDs) || len(last.AfterAttachmentIDs) != 0 {
				t.Fatalf("deletion history lost body/attachment identity: %+v", last)
			}
			for i, entry := range history {
				if entry.Actor == nil || entry.Actor.UserID != util.MustParseUUID(f.creator) || entry.CreatedAt.IsZero() || (i > 0 && entry.EventSeq <= history[i-1].EventSeq) {
					t.Fatalf("content history lost exact operator/time/order: %+v", entry)
				}
			}
			operations := f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID)
			for _, original := range []struct {
				write func(*CollaborationService) outcome
				want  outcome
			}{{writeEdit, editOutcome}, {writeDelete, deleteOutcome}} {
				replay := original.write(f.svc)
				if !sameReceipt(replay.receipt, original.want.receipt) || (original.want.err == nil && replay.err != nil) ||
					(original.want.err != nil && !collaborationErrorCode(replay.err, expectedRefusal)) {
					t.Fatalf("tombstone recomputed an earlier success/refusal: %+v %v", replay.receipt, replay.err)
				}
			}
			resurrection := edit
			resurrection.RequestID, resurrection.ExpectedCommentRevision = uuid.NewString(), wantRevision
			if _, err = f.svc.EditComment(f.ctx, resurrection); !collaborationErrorCode(err, "comment_deleted") {
				t.Fatalf("fresh edit resurrected deleted content: %v", err)
			}
			afterHistory, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, created.CommentID)
			if err != nil || !reflect.DeepEqual(history, afterHistory) || protected() != before ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != operations+1 ||
				f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND parent_id=$2 AND content='reply retains its deleted parent anchor' AND revision=1`, reply, created.CommentID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='comment' AND removed_at IS NULL`, attachment.ID) != 0 {
				t.Fatal("content retry/deletion changed history, reply, delivered attachment or frozen AI state")
			}
			row, err := f.svc.GetAttachment(collaborationContext(f.handler), f.issue.Issue.ID, pgtype.UUID{}, attachment.ID)
			if err != nil {
				t.Fatal(err)
			}
			data, err := f.svc.ReadAttachmentBytes(collaborationContext(f.handler), row)
			if err != nil || !bytes.Equal(data, upload.Data) {
				t.Fatalf("delete/edit removed delivered attachment bytes: %v", err)
			}
			var facts struct {
				RequestID string `json:"request_id"`
			}
			var payload []byte
			f.fx.QueryRow(t, `SELECT payload FROM collaboration_event WHERE id=$1 AND issue_id=$2`, last.EventID, f.issue.Issue.ID).Scan(&payload)
			if err := json.Unmarshal(payload, &facts); err != nil || facts.RequestID != finalDeletionKey {
				t.Fatalf("deletion history lost its original request identity: %v", err)
			}
		})
	}
}
