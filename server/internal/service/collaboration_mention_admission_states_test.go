package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Each case enters through the public human comment service, not SubmitCommand.
// Permits are persisted service authority; outcomes and sealed proofs are
// synthetic protocol inputs and do not observe a provider or kernel process.
func TestCollaborationMentionCommentAdmissionAcrossExecutionStates(t *testing.T) {
	for _, scenario := range []struct {
		name, state, code, waitingReason string
	}{
		{"idle", "waiting", "command_queued", "shared_runtime_unavailable"},
		{"busy", "waiting", "command_queued", "target_busy"},
		{"paused", "waiting", "command_queued", "issue_paused"},
		{"unconfirmed_stop", "waiting", "command_queued", "target_paused"},
		{"revoked_resource", "blocked", "target_unavailable", ""},
		{"removed_author", "", "not_found", ""},
		{"terminal_issue", "blocked", "issue_closed", ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			mention := "[@A](mention://agent/" + util.UUIDToString(f.grant.AgentID) + ") "
			create := func(actor, content string) CollaborationCommentReceipt {
				t.Helper()
				p := mentionAdmissionComment(t, f, content)
				r, err := f.svc.CreateComment(collaborationPasswordContext(util.MustParseUUID(actor), 1), p)
				if err != nil || r.Code != "comment_created" || len(r.Targets) != 1 || r.Targets[0].State != "waiting" {
					t.Fatalf("initial typed comment did not retain waiting input: %+v %v", r, err)
				}
				mentionAdmissionStoredComment(t, f, actor, p, r)
				return r
			}
			// An actual earlier comment input is explicitly cancelled. Later @,
			// display edits, polling and Resume must never revive this input.
			cancelled := create(f.creator, mention+"explicitly cancelled earlier input")
			if _, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{
				CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: cancelled.Targets[0].CommandID,
				ExpectedCollaborationRevision: 1,
			}); err != nil {
				t.Fatal(err)
			}
			var original, pending CollaborationCommentReceipt
			var oldRun pgtype.UUID
			var oldClaim protocol.CollaborationRunClaimResponse
			var oldPermit protocol.CollaborationRunPermitResponse
			var stopRequest CollaborationStopQueueParams
			startOriginal := func() {
				t.Helper()
				original = create(f.handler, mention+"original active comment")
				f.resumeManaged(t)
				oldRun, _, oldClaim = f.claimManaged(t)
				mentionAdmissionClaim(t, f, original.Targets[0], f.handler, mention+"original active comment", oldRun, oldClaim)
				var err error
				oldPermit, err = f.svc.PermitRuntimeRun(f.daemonCtx, oldRun, protocol.CollaborationRunPermitRequest{
					Identity: oldClaim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			switch scenario.name {
			case "idle":
				f.resumeManaged(t)
			case "busy":
				startOriginal()
			case "paused":
				f.resumeManaged(t)
				if _, err := f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
					t.Fatal(err)
				}
			case "unconfirmed_stop":
				startOriginal()
				stopRequest = managedTargetResume(t, f, f.targetID).CollaborationStopQueueParams
				stopped, err := f.svc.StopQueue(f.ctx, stopRequest)
				if err != nil || stopped.GroupSettled || stopped.GroupID != util.MustParseUUID(oldClaim.Identity.GroupID) {
					t.Fatalf("Stop prematurely proved the permitted original: %+v %v", stopped, err)
				}
			case "revoked_resource":
				startOriginal()
				pending = create(f.creator, mention+"waiting before resource withdrawal")
				if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{
					CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch,
				}); err != nil {
					t.Fatal(err)
				}
			case "removed_author":
				startOriginal()
				pending = create(f.handler, mention+"removed author's pending input")
				if _, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{
					CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.handler),
				}); err != nil {
					t.Fatal(err)
				}
			case "terminal_issue":
				if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
				startOriginal()
				pending = create(f.creator, mention+"waiting before explicit completion")
				if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f.collaborationQueueFixture, "complete", "done", 1, 1)); err != nil {
					t.Fatal(err)
				}
			}
			oldCommands := mentionAdmissionCommands(t, f)
			before := mentionAdmissionSnapshot(t, f, scenario.state == "waiting")
			p := mentionAdmissionComment(t, f, mention+"trigger in "+scenario.name)
			actor := f.creator
			if scenario.name == "removed_author" {
				actor = f.handler
			}
			receipt, err := f.svc.CreateComment(collaborationPasswordContext(util.MustParseUUID(actor), 1), p)
			if scenario.name == "removed_author" {
				if !collaborationErrorCode(err, scenario.code) || receipt.CommentID.Valid ||
					f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND content=$2`, f.issue.Issue.ID, p.Content) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
					t.Fatalf("removed external author reentered by typed @ comment: %+v %v", receipt, err)
				}
			} else {
				if err != nil || len(receipt.Targets) != 1 || receipt.Targets[0].State != scenario.state ||
					receipt.Targets[0].Code != scenario.code || receipt.Targets[0].WaitingReason != scenario.waitingReason {
					t.Fatalf("comment did not distinguish %s target admission: %+v %v", scenario.name, receipt, err)
				}
				mentionAdmissionStoredComment(t, f, actor, p, receipt)
				if scenario.state == "waiting" {
					item := receipt.Targets[0]
					if item.TargetID != f.targetID || item.ResourceID != f.grant.AgentID || item.ChannelID != queueTarget(t, f.collaborationQueueFixture, f.targetID).ChannelID ||
						!item.CommandID.Valid || !item.BatchID.Valid || item.GroupID.Valid || item.CommandSequence < 1 ||
						f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND input_event_id=$2 AND author_user_id=$3
							AND content=$4 AND target_id=$5 AND channel_id=$6 AND batch_id=$7 AND seq=$8 AND state='waiting' AND group_id IS NULL`,
							item.CommandID, receipt.EventID, actor, p.Content, item.TargetID, item.ChannelID, item.BatchID, item.CommandSequence) != 1 {
						t.Fatal("waiting receipt lost its exact event/author/target/channel/batch input identity or fabricated a start")
					}
				} else if receipt.Targets[0].CommandID.Valid || receipt.Targets[0].BatchID.Valid || receipt.Targets[0].GroupID.Valid {
					t.Fatal("blocked @ fabricated command or execution identity")
				}
			}
			if before != mentionAdmissionSnapshot(t, f, scenario.state == "waiting") {
				t.Fatal("typed comment changed a control/hold/proof/capacity barrier or prior execution")
			}
			wantCommands := len(oldCommands)
			if scenario.state == "waiting" {
				wantCommands++
			}
			currentCommands := mentionAdmissionCommands(t, f)
			if len(currentCommands) != wantCommands {
				t.Fatal("@ target refusal/submission inserted the wrong number of inputs")
			}
			for _, old := range oldCommands {
				if !mentionAdmissionRetainsCommand(currentCommands, old) {
					t.Fatal("a new comment mutated or revived a prior immutable/cancelled/stopped input")
				}
			}
			// Plain comments/replies, even with stale/unused target versions, stay
			// human-only. Editing a typed @ updates display, never frozen input.
			inertBefore := mentionAdmissionSnapshot(t, f, false)
			plainInput := mentionAdmissionComment(t, f, "plain human comment in "+scenario.name)
			plain, err := f.svc.CreateComment(f.ctx, plainInput)
			if err != nil || len(plain.Targets) != 0 {
				t.Fatalf("plain comment acquired target execution: %+v %v", plain, err)
			}
			mentionAdmissionStoredComment(t, f, f.creator, plainInput, plain)
			replyInput := mentionAdmissionComment(t, f, "ordinary reply with unresolved @AI in "+scenario.name)
			replyInput.ParentID = plain.CommentID
			reply, err := f.svc.CreateComment(f.ctx, replyInput)
			if err != nil || len(reply.Targets) != 0 {
				t.Fatalf("plain reply/unused versions implicitly invoked its parent: %+v %v", reply, err)
			}
			mentionAdmissionStoredComment(t, f, f.creator, replyInput, reply)
			editID := receipt.CommentID
			if !editID.Valid {
				editID = plain.CommentID
			}
			edited, err := f.svc.EditComment(f.ctx, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: editID,
				RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: mention + "edited display in " + scenario.name})
			if err != nil || edited.Code != "comment_edited" || len(edited.Targets) != 0 || inertBefore != mentionAdmissionSnapshot(t, f, false) ||
				!reflect.DeepEqual(currentCommands, mentionAdmissionCommands(t, f)) {
				t.Fatalf("plain comment/reply/typed edit changed execution or original input: %+v %v", edited, err)
			}
			if scenario.name != "idle" {
				for range 2 {
					list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
					if err != nil {
						t.Fatal(err)
					}
					// A permitted healthy run remains a recoverable candidate. It
					// must be the original run, never this comment's waiting batch.
					if scenario.name == "busy" {
						if len(list.RunIDs) != 1 || list.RunIDs[0] != util.UUIDToString(oldRun) ||
							f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 ||
							f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, receipt.Targets[0].CommandID) != 1 {
							t.Fatalf("busy poll admitted the new comment instead of recovering original run: %+v", list)
						}
					} else if len(list.RunIDs) != 0 {
						t.Fatalf("%s comment/poll bypassed a suspended barrier: %+v", scenario.name, list)
					}
				}
			}
			if oldRun.Valid && scenario.name != "busy" {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped'
					AND proof_state='unknown' AND settled_at IS NULL`, oldRun) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, oldRun) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, oldRun) != 1 {
					t.Fatal("comment/reply/edit/poll invented old termination proof or released real capacity")
				}
			}
			switch scenario.name {
			case "busy":
				if receipt.Targets[0].BatchID == original.Targets[0].BatchID ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='unknown' AND forced_disposition IS NULL AND settled_at IS NULL`, oldRun) != 1 {
					t.Fatal("busy @ injected itself into the running batch or stopped the active execution")
				}
				if err := f.svc.RecordRuntimeOutcome(context.Background(), oldRun, oldPermit.BusinessToken, protocol.CollaborationRunOutcome{
					Identity: oldClaim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: oldPermit.AttemptNonce,
					Outcome: "success", Body: "synthetic original comment execution success",
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.AcceptRuntimeProof(context.Background(), oldRun, oldClaim.TerminationToken, managedProof(oldClaim.Identity, oldPermit.AttemptNonce)); err != nil {
					t.Fatal(err)
				}
			case "paused":
				f.resumeManaged(t)
			case "unconfirmed_stop":
				intent := managedTargetResume(t, f, f.targetID)
				beforeRefusal := controlStateSnapshot(t, f.collaborationQueueFixture)
				refused, err := f.svc.ResumeQueue(f.ctx, intent)
				if !collaborationErrorCode(err, "stop_unconfirmed") || beforeRefusal != controlStateSnapshot(t, f.collaborationQueueFixture) {
					t.Fatalf("new @ bypassed stop-unconfirmed Resume refusal: %+v %v", refused, err)
				}
				if _, err := f.svc.AcceptRuntimeProof(context.Background(), oldRun, oldClaim.TerminationToken, managedProof(oldClaim.Identity, oldPermit.AttemptNonce)); err != nil {
					t.Fatal(err)
				}
				afterProof := controlStateSnapshot(t, f.collaborationQueueFixture)
				if list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err != nil || len(list.RunIDs) != 0 || afterProof != controlStateSnapshot(t, f.collaborationQueueFixture) {
					t.Fatalf("proof receipt automatically resumed the comment: %+v %v", list, err)
				}
				if replay, err := f.svc.ResumeQueue(f.ctx, intent); !collaborationErrorCode(err, "stop_unconfirmed") ||
					!reflect.DeepEqual(replay, refused) || afterProof != controlStateSnapshot(t, f.collaborationQueueFixture) {
					t.Fatalf("old refused comment-era Resume gained future intent: %+v %v", replay, err)
				}
				if _, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID)); err != nil {
					t.Fatal(err)
				}
			}
			if scenario.state == "waiting" {
				runID, _, claim := f.claimManaged(t)
				mentionAdmissionClaim(t, f, receipt.Targets[0], actor, p.Content, runID, claim)
				if runID == oldRun || claim.Identity.GroupID == oldClaim.Identity.GroupID || claim.Identity.LaunchNonce == oldClaim.Identity.LaunchNonce {
					t.Fatal("new comment reuses a stopped/busy original execution identity")
				}
				permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
					Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
				})
				if err != nil || f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1 AND attempt_seq=1 AND attempt_nonce=$2`, runID, permit.AttemptNonce) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, receipt.Targets[0].CommandID) != 1 {
					t.Fatalf("actual managed start did not consume the exact waiting @ input: %+v %v", permit, err)
				}
				if err := f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{
					Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce,
					Outcome: "success", Body: "synthetic direct @ comment result",
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
					t.Fatal(err)
				}
				if scenario.name == "unconfirmed_stop" && f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, original.Targets[0].CommandID) != 1 {
					t.Fatal("fresh explicit Resume restarted the old stopped comment input")
				}
			}
			if pending.CommentID.Valid {
				state, reason := "waiting", ""
				if scenario.name == "removed_author" {
					state, reason = "cancelled", "participant_removed"
				}
				if scenario.name == "terminal_issue" {
					state, reason = "cancelled", "issue_completed"
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state=$2 AND COALESCE(reason,'')=$3`, pending.Targets[0].CommandID, state, reason) != 1 {
					t.Fatal("direct @ or inert human content restored a cancelled input or erased waiting input")
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='cancelled_by_participant'`, cancelled.Targets[0].CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND dispatch_eligible=true`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("mention/plain/reply/edit paths revived old cancellation or entered ordinary AI dispatch")
			}
			t.Logf("state=%s issue=%s actor=%s target=%s channel=%s comment=%s event=%s old_run=%s", scenario.name,
				util.UUIDToString(f.issue.Issue.ID), actor, util.UUIDToString(f.targetID), util.UUIDToString(queueTarget(t, f.collaborationQueueFixture, f.targetID).ChannelID),
				util.UUIDToString(receipt.CommentID), util.UUIDToString(receipt.EventID), util.UUIDToString(oldRun))
		})
	}
}

func mentionAdmissionComment(t *testing.T, f *managedCollaborationFixture, content string) CollaborationCreateCommentParams {
	t.Helper()
	access := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	request := f.request()
	request.ExpectedLifecycleEpoch = access.LifecycleEpoch
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	return CollaborationCreateCommentParams{CollaborationQueueRequest: request, CollaborationCommentInput: CollaborationCommentInput{Content: content},
		TargetVersions: []CollaborationCommentTargetVersion{{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion}}}
}

func mentionAdmissionStoredComment(t *testing.T, f *managedCollaborationFixture, actor string, p CollaborationCreateCommentParams, r CollaborationCommentReceipt) {
	t.Helper()
	if r.Code != "comment_created" || !r.CommentID.Valid || !r.EventID.Valid || r.EventSeq < 1 ||
		f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND issue_id=$2 AND author_id=$3 AND content=$4 AND dispatch_eligible=false
			AND parent_id IS NOT DISTINCT FROM $5::uuid`, r.CommentID, f.issue.Issue.ID, actor, p.Content, p.ParentID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE id=$1 AND issue_id=$2 AND event_seq=$3 AND actor_user_id=$4
			AND kind='comment_created' AND payload->>'comment_id'=$5 AND payload->>'request_id'=$6`,
			r.EventID, f.issue.Issue.ID, r.EventSeq, actor, util.UUIDToString(r.CommentID), p.RequestID) != 1 {
		t.Fatal("public comment lost its persisted human body/parent/event identity or became ordinary-dispatch eligible")
	}
}

func mentionAdmissionClaim(t *testing.T, f *managedCollaborationFixture, input CollaborationCommentTargetResult, actor, body string, run pgtype.UUID, claim protocol.CollaborationRunClaimResponse) {
	t.Helper()
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(input.CommandID) || claim.Inputs[0].AuthorUserID != actor || claim.Inputs[0].Body != body ||
		claim.Identity.IssueID != util.UUIDToString(f.issue.Issue.ID) || claim.Identity.SessionID != util.UUIDToString(f.issue.SessionID) ||
		claim.Identity.TargetID != util.UUIDToString(input.TargetID) || claim.Identity.ChannelID != util.UUIDToString(input.ChannelID) ||
		claim.Identity.BatchID != util.UUIDToString(input.BatchID) || claim.Identity.RunID != util.UUIDToString(run) || claim.Identity.GroupID == "" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1 AND channel_id=$2 AND batch_id=$3 AND consuming_run_id=$4`,
			input.CommandID, input.ChannelID, input.BatchID, run) != 1 {
		t.Fatal("managed Claim lost the original typed @ command/author/body/target/channel/batch membership")
	}
}

func mentionAdmissionCommands(t *testing.T, f *managedCollaborationFixture) []db.CollaborationCommand {
	t.Helper()
	commands, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	return commands
}

func mentionAdmissionRetainsCommand(commands []db.CollaborationCommand, old db.CollaborationCommand) bool {
	for _, command := range commands {
		if command.ID == old.ID {
			return reflect.DeepEqual(command, old)
		}
	}
	return false
}

func mentionAdmissionSnapshot(t *testing.T, f *managedCollaborationFixture, allowAppend bool) string {
	t.Helper()
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(controlStateSnapshot(t, f.collaborationQueueFixture)), &snapshot); err != nil {
		t.Fatal(err)
	}
	var session map[string]json.RawMessage
	if err := json.Unmarshal(snapshot["session"], &session); err != nil {
		t.Fatal(err)
	}
	delete(session, "event_seq") // Human comment events legitimately advance it.
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	snapshot["session"] = encoded
	delete(snapshot, "events")
	if allowAppend {
		// Successful @ changes only staging input and allocation counters here.
		// Every old command is still compared byte-for-byte by the caller.
		delete(snapshot, "commands")
		var channels []map[string]json.RawMessage
		if err := json.Unmarshal(snapshot["channels"], &channels); err != nil {
			t.Fatal(err)
		}
		for _, channel := range channels {
			delete(channel, "next_command_seq")
			delete(channel, "next_batch_seq")
		}
		encoded, err = json.Marshal(channels)
		if err != nil {
			t.Fatal(err)
		}
		snapshot["channels"] = encoded
	} else {
		var batches string
		f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(b) ORDER BY b.id),'[]'::jsonb)::text
			FROM collaboration_batch b WHERE b.session_id=$1`, f.issue.SessionID).Scan(&batches)
		snapshot["batches"] = json.RawMessage(batches)
	}
	encoded, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
