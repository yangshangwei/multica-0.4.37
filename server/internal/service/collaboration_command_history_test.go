package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationCommandCancellationExactProvenance(t *testing.T) {
	for _, action := range []string{"manual", "remove", "leave", "complete", "cancel"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			input := f.submit(t, f.handler, "immutable cancelled input")
			next := f.submit(t, f.creator, "independent following input")
			actor, kind, reason := f.handler, "manual", "cancelled_by_participant"
			var err error
			switch action {
			case "manual":
				_, err = f.svc.CancelWaitingCommand(collaborationContext(actor), CollaborationCancelCommandParams{
					CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1,
				})
			case "remove":
				actor, kind, reason = f.creator, "automatic", "participant_removed"
				_, err = f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
			case "leave":
				kind, reason = "automatic", "participant_left"
				f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
				_, err = f.svc.Leave(collaborationContext(actor), workflowRequest(f, 1))
			case "complete":
				actor, kind, reason = f.creator, "automatic", "issue_completed"
				_, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "complete", "done", 1, 1))
			case "cancel":
				actor, kind, reason = f.creator, "automatic", "issue_cancelled"
				_, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "cancel", "cancelled", 1, 1))
			}
			if err != nil {
				t.Fatal(err)
			}
			var eventID *string
			f.fx.QueryRow(t, `SELECT to_jsonb(c)->>'cancellation_event_id' FROM collaboration_command c WHERE id=$1`, input.CommandID).Scan(&eventID)
			if eventID == nil {
				t.Fatal("cancelled command lost its exact cancellation event association")
			}
			var eventActor pgtype.UUID
			var eventTime pgtype.Timestamptz
			var payload json.RawMessage
			f.fx.QueryRow(t, `SELECT actor_user_id,created_at,payload FROM collaboration_event WHERE id=$1 AND issue_id=$2 AND kind='command_cancelled'`, *eventID, f.issue.Issue.ID).Scan(&eventActor, &eventTime, &payload)
			var facts struct {
				CommandID        pgtype.UUID `json:"command_id"`
				ChannelID        pgtype.UUID `json:"channel_id"`
				BatchID          pgtype.UUID `json:"batch_id"`
				TargetID         pgtype.UUID `json:"target_id"`
				CommandSequence  int64       `json:"command_sequence,string"`
				Reason           string      `json:"reason"`
				CancellationKind string      `json:"cancellation_kind"`
			}
			if err = json.Unmarshal(payload, &facts); err != nil {
				t.Fatal(err)
			}
			if eventActor != util.MustParseUUID(actor) || !eventTime.Valid || facts.CommandID != input.CommandID || facts.TargetID != f.targetID || facts.ChannelID != input.ChannelID || facts.BatchID != input.BatchID || facts.CommandSequence != input.CommandSequence || facts.Reason != reason || facts.CancellationKind != kind {
				t.Fatalf("cancellation lost its actual actor, time, reason or exact input identity: actor=%v time=%v facts=%+v", eventActor, eventTime, facts)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason=$2 AND content='immutable cancelled input'`, input.CommandID, reason) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatal("never-consumed cancellation lost its immutable input or fabricated a run")
			}
			if action != "complete" && action != "cancel" && f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND seq=$2`, next.CommandID, next.CommandSequence) != 1 {
				t.Fatal("single-author cancellation changed the next input or its FIFO position")
			}
			if action == "complete" || action == "cancel" {
				if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", 2, 2)); err != nil {
					t.Fatal(err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND cancellation_event_id=$2 AND reason=$3`, input.CommandID, *eventID, reason) != 1 {
					t.Fatal("reopen replaced cancellation evidence or revived old input")
				}
			}
		})
	}
}

func TestCollaborationCommandHistoryOutlivesCommentAndAttachmentDisplay(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collaboration_channel", "collaboration_batch", "collaboration_command"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE session_id=$1", issue.SessionID)
	}
	content := &collaborationContentFixture{collaborationFixture: f, issue: issue}
	target, _ := seedCollaborationTarget(t, f.fx, issue.Issue.ID, issue.SessionID, util.MustParseUUID(f.creator), 1)
	upload := attachmentUploadParams(f)
	upload.SourceWorkspaceID, upload.IssueID = pgtype.UUID{}, issue.Issue.ID
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := content.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") original instruction with a retained attachment", target.ID)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
	comment, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || len(comment.Targets) != 1 {
		t.Fatalf("comment input fixture: %+v %v", comment, err)
	}
	request := p.CollaborationQueueRequest
	request.RequestID = uuid.NewString()
	_, err = f.svc.CancelWaitingCommand(collaborationContext(f.handler), CollaborationCancelCommandParams{
		CollaborationQueueRequest: request, TargetID: target.ID, CommandID: comment.Targets[0].CommandID, ExpectedCollaborationRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.svc.CommandHistory(f.ctx, issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil || len(before.Items) != 1 {
		t.Fatalf("initial history: %+v %v", before, err)
	}
	if _, err = f.svc.EditComment(f.ctx, CollaborationEditCommentParams{IssueID: issue.Issue.ID, CommentID: comment.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: "edited display only", AttachmentIDs: &[]pgtype.UUID{}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeleteComment(f.ctx, CollaborationEditCommentParams{IssueID: issue.Issue.ID, CommentID: comment.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 2}); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []string{f.creator, f.handler} {
		page, err := f.svc.CommandHistory(collaborationContext(viewer), issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
		if err != nil || !reflect.DeepEqual(before, page) {
			t.Fatalf("refresh by another reader reconstructed history from editable comments: %+v %v", page, err)
		}
	}
	item := before.Items[0]
	if item.Content != p.Content || item.AuthorUserID != util.MustParseUUID(f.creator) || item.Author == nil || item.Author.Name != "Collaboration creator" || item.TargetID != target.ID || item.Target.Kind != "agent" || item.Target.ResourceID != target.ResourceID || item.CommentID == nil || *item.CommentID != util.UUIDToString(comment.CommentID) || item.InputEventSeq == nil || item.InputEventID != comment.EventID || !reflect.DeepEqual(item.AttachmentIDs, p.AttachmentIDs) || item.State != "cancelled" || item.Cancellation == nil || item.Cancellation.ActorUserID == nil || *item.Cancellation.ActorUserID != f.handler {
		t.Fatalf("history lost original input, attachment, author, target or cancelling handler: %+v", item)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, issue.SessionID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='command' AND removed_at IS NULL`, attachment.ID) != 1 {
		t.Fatal("never-consumed history required a run or removed its permanent attachment reference")
	}
	wire, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"can_cancel", "control_ai", "installation_id", "binding_id", "grant_id", "storage_key", "encryption_key"} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("history projected unrelated control or resource authority: %s", forbidden)
		}
	}
}

func TestCollaborationCommandHistoryPagingAndCurrentAuthority(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	var expected []string
	for i := 0; i < 3; i++ {
		expected = append(expected, util.UUIDToString(f.submit(t, f.handler, "private paged input").CommandID))
	}
	// Equal timestamps exercise the immutable UUID tie-break independently of
	// transaction-start ordering. This does not change the channel sequence.
	created := time.Now().UTC().Truncate(time.Microsecond)
	f.fx.Exec(t, `UPDATE collaboration_command SET created_at=$2 WHERE id=ANY($1::uuid[])`, expected, created)
	sort.Sort(sort.Reverse(sort.StringSlice(expected)))
	page, err := f.svc.CommandHistory(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || util.UUIDToString(page.Items[0].ID) != expected[0] {
		t.Fatalf("first history page: %+v %v", page, err)
	}
	firstCursor := *page.NextCursor
	got := []string{util.UUIDToString(page.Items[0].ID)}
	for page.NextCursor != nil {
		page, err = f.svc.CommandHistory(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1, Cursor: *page.NextCursor})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("history continuation: %+v %v", page, err)
		}
		got = append(got, util.UUIDToString(page.Items[0].ID))
		if len(got) > len(expected) {
			t.Fatal("history cursor repeated a page")
		}
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("history pagination omitted or duplicated inputs: got %v want %v", got, expected)
	}
	invalid := []CollaborationCommandHistoryQuery{{Limit: 0}, {Limit: 101}, {Limit: 1, Cursor: "not-base64"}, {Limit: 1, Cursor: "e30"}, {Limit: 1, Cursor: strings.Repeat("a", 1025)}}
	raw, err := base64.RawURLEncoding.DecodeString(firstCursor)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"issue_id", "command_id", "created_at"} {
		for _, value := range []any{nil, "", uuid.NewString()} {
			var fields map[string]any
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			fields[field] = value
			changed, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			invalid = append(invalid, CollaborationCommandHistoryQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(changed)})
			delete(fields, field)
			changed, _ = json.Marshal(fields)
			invalid = append(invalid, CollaborationCommandHistoryQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(changed)})
		}
	}
	duplicate := strings.TrimSuffix(string(raw), "}") + `,"issue_id":"` + util.UUIDToString(f.issue.Issue.ID) + `"}`
	invalid = append(invalid, CollaborationCommandHistoryQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString([]byte(duplicate))})
	for _, query := range invalid {
		if _, err = f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, query); !collaborationErrorCode(err, "invalid_command_history_query") {
			t.Fatalf("malformed/unbound history cursor was accepted: %v", err)
		}
	}
	other, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CommandHistory(f.ctx, other.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1, Cursor: firstCursor}); !collaborationErrorCode(err, "invalid_command_history_query") {
		t.Fatalf("cross-issue cursor was accepted: %v", err)
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud} {
		machine := auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: kind})
		if _, err = f.svc.CommandHistory(machine, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1}); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("machine inherited human command history: %v", err)
		}
	}
	if _, err = f.svc.CommandHistory(collaborationContext(f.outsider), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("outsider received private command history: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if _, err = f.svc.CommandHistory(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1, Cursor: firstCursor}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("warm reader/cursor retained revoked access: %v", err)
	}
	f.fx.Member(t, f.workspace, f.handler, "member")
	if page, err = f.svc.CommandHistory(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1, Cursor: firstCursor}); err != nil || len(page.Items) != 1 {
		t.Fatalf("independent source-member basis lost command history: %+v %v", page, err)
	}
	if _, err = f.svc.CancelWaitingCommand(collaborationContext(f.handler), CollaborationCancelCommandParams{
		CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: page.Items[0].ID, ExpectedCollaborationRevision: 1,
	}); err == nil {
		t.Fatal("source-only history read granted cancellation authority")
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	if _, err = f.svc.CommandHistory(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 1}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("disabled source member retained history: %v", err)
	}
}

func TestCollaborationCommandHistoryKeepsUnprovenLegacyCancellationUnknown(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	input := f.submit(t, f.handler, "legacy cancelled input")
	f.fx.Exec(t, `UPDATE collaboration_command SET state='cancelled',reason='participant_removed' WHERE id=$1`, input.CommandID)
	// Even an identical timestamp and a real removal actor do not establish
	// which of that operation's commands were actually cancelled.
	f.fx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "event_seq": 999, "actor_user_id": f.creator,
		"kind": "participant_removed", "payload": json.RawMessage(`{"cancelled_commands":1}`), "created_at": testutil.Raw("now()")})
	page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("legacy cancellation read: %+v %v", page, err)
	}
	cancellation := page.Items[0].Cancellation
	if page.Items[0].Reason == nil || *page.Items[0].Reason != "participant_removed" || cancellation == nil || !reflect.DeepEqual(*cancellation, CollaborationCommandCancellation{}) {
		t.Fatalf("legacy bulk event fabricated cancellation provenance: %+v", page.Items[0])
	}
}

func TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	succeeded := f.submitManaged(t, f.creator, "successful command without a reason")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	nonce := uuid.NewString()
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
		Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{
		Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nonce, Outcome: "success", Body: "completed without a cancellation reason",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var success *CollaborationCommandHistoryEntry
	for i := range page.Items {
		item := &page.Items[i]
		if item.ID == succeeded.CommandID {
			success = item
		}
	}
	if success == nil || success.State != "succeeded" || success.Reason != nil {
		t.Fatalf("successful command with no reason must project a nullable absent reason: %+v", success)
	}
	wire, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), `"reason":""`) || !strings.Contains(string(wire), `"reason":null`) {
		t.Fatalf("history wire must encode absent terminal reason as null and retain a real reason: %s", wire)
	}
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	cancelFixture := newCollaborationQueueFixture(t)
	cancelled := cancelFixture.submit(t, cancelFixture.handler, "manual cancellation keeps its reason")
	if _, err = cancelFixture.svc.CancelWaitingCommand(collaborationContext(cancelFixture.handler), CollaborationCancelCommandParams{
		CollaborationQueueRequest: cancelFixture.request(), TargetID: cancelFixture.targetID, CommandID: cancelled.CommandID, ExpectedCollaborationRevision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	cancelledPage, err := cancelFixture.svc.CommandHistory(cancelFixture.ctx, cancelFixture.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil || len(cancelledPage.Items) != 1 || cancelledPage.Items[0].Reason == nil || *cancelledPage.Items[0].Reason != "cancelled_by_participant" {
		t.Fatalf("nonempty cancellation reason was not preserved: %+v %v", cancelledPage, err)
	}
}

func TestCollaborationCommandHistoryLegacyManualAndBrokenEvidence(t *testing.T) {
	for _, scenario := range []string{"legacy_manual", "foreign_event", "wrong_command", "missing_actor", "wrong_reason", "missing_event"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			input := f.submit(t, f.handler, "retained original command")
			f.fx.Exec(t, `UPDATE collaboration_command SET state='cancelled',reason='cancelled_by_participant' WHERE id=$1`, input.CommandID)
			payload := map[string]any{"command_id": util.UUIDToString(input.CommandID), "target_id": util.UUIDToString(input.TargetID), "channel_id": util.UUIDToString(input.ChannelID), "batch_id": util.UUIDToString(input.BatchID), "command_sequence": "1", "action": "cancel_command", "code": "command_cancelled"}
			eventIssue := f.issue.Issue.ID
			if scenario == "foreign_event" {
				eventIssue = util.MustParseUUID(uuid.NewString())
			}
			if scenario == "wrong_command" {
				payload["command_id"] = uuid.NewString()
			}
			if scenario == "wrong_reason" {
				payload["reason"], payload["cancellation_kind"] = "different_reason", "manual"
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			cols := testutil.Cols{"issue_id": eventIssue, "event_seq": 999, "kind": "command_cancelled", "actor_user_id": f.creator, "payload": encoded}
			if scenario == "missing_actor" {
				delete(cols, "actor_user_id")
			}
			event := f.fx.Insert(t, "collaboration_event", cols)
			if scenario == "missing_event" {
				event = uuid.NewString()
			}
			// Simulate persisted pre-upgrade/manual evidence or corrupted linkage.
			// Production linkage has its own exact command/event SQL predicate.
			f.fx.Exec(t, `UPDATE collaboration_command SET cancellation_event_id=$2 WHERE id=$1`, input.CommandID, event)
			page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
			if scenario != "legacy_manual" {
				if err == nil || strings.Contains(err.Error(), "retained original command") {
					t.Fatalf("invalid cancellation association was displayed or exposed content in its error: %v", err)
				}
				return
			}
			if err != nil || len(page.Items) != 1 || page.Items[0].Cancellation == nil {
				t.Fatalf("precisely linked old manual cancellation became unreadable: %+v %v", page, err)
			}
			cancellation := page.Items[0].Cancellation
			if cancellation.Kind == nil || *cancellation.Kind != "manual" || cancellation.EventID == nil || *cancellation.EventID != event || cancellation.ActorUserID == nil || *cancellation.ActorUserID != f.creator || cancellation.CreatedAt == nil {
				t.Fatalf("old exact receipt lost provable actor/time: %+v", cancellation)
			}
		})
	}
}

func TestCollaborationCommandCancellationRollsBackEventAndState(t *testing.T) {
	for _, action := range []string{"manual", "complete"} {
		for _, query := range []string{"CreateCollaborationEvent", "LinkCollaborationCommandCancellation", "CompleteCollaborationOperation"} {
			t.Run(action+"/"+query, func(t *testing.T) {
				f := newCollaborationLifecycleFixture(t)
				input := f.submit(t, f.handler, "must survive failed cancellation")
				f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: query}
				var err error
				if action == "manual" {
					_, err = f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1})
				} else {
					_, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "complete", "done", 1, 1))
				}
				if err == nil || !strings.Contains(err.Error(), "injected") {
					t.Fatalf("required cancellation write fault was not returned: %v", err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND reason IS NULL AND cancellation_event_id IS NULL`, input.CommandID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='command_cancelled'`, f.issue.Issue.ID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='closed'`, f.issue.SessionID) != 0 {
					t.Fatal("failed cancellation left an event, half-cancelled input or closed session")
				}
			})
		}
	}
}

func TestCollaborationCommandCancellationUnknownCommitKeepsOneEvent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	input := f.submit(t, f.handler, "lost cancellation response")
	p := CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.CancelWaitingCommand(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("unknown cancellation commit was retried: %v", err)
	}
	f.svc.TxStarter = f.pool
	receipt, err := f.svc.GetCommandOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || receipt.CommandID != input.CommandID || receipt.Code != "command_cancelled" {
		t.Fatalf("original cancellation operation was not recovered: %+v %v", receipt, err)
	}
	if _, err = f.svc.CancelWaitingCommand(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil || len(page.Items) != 1 || page.Items[0].Cancellation == nil || page.Items[0].Cancellation.EventID == nil || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='command_cancelled'`, f.issue.Issue.ID) != 1 {
		t.Fatalf("original-key recovery duplicated or lost cancellation history: %+v %v", page, err)
	}
}
