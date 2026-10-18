package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func workflowRequest(f *collaborationQueueFixture, revision int64) CollaborationWorkflowRequest {
	return CollaborationWorkflowRequest{CollaborationQueueRequest: f.request(), ExpectedCollaborationRevision: revision}
}

func workflowGrant(t *testing.T, f *collaborationQueueFixture, owner string) pgtype.UUID {
	t.Helper()
	_, grant := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(owner), 1)
	return grant.ID
}

func groupControlResult(receipt *CollaborationQueueReceipt, group pgtype.UUID) (CollaborationTargetControlResult, bool) {
	if receipt != nil {
		for _, result := range receipt.Targets {
			if result.GroupID == group {
				return result, true
			}
		}
	}
	return CollaborationTargetControlResult{}, false
}

func grantTarget(t *testing.T, f *collaborationQueueFixture, grant pgtype.UUID) pgtype.UUID {
	t.Helper()
	var target pgtype.UUID
	f.fx.QueryRow(t, `SELECT target_id FROM collaboration_grant WHERE id=$1`, grant).Scan(&target)
	return target
}

func TestCollaborationWorkflowRemovalRevokesAuthorAndResourceWithoutInventingProof(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		attempts   int64
		owner      bool
		priorStop  bool
		badRunning bool
	}{
		{name: "never_permitted_author"},
		{name: "permitted_author", attempts: 1},
		{name: "permitted_owner_other_author", attempts: 1, owner: true},
		{name: "never_permitted_owner_other_author", owner: true},
		{name: "preserve_first_stop", attempts: 1, priorStop: true},
		{name: "contradictory_running_without_permit", badRunning: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			author := f.handler
			if scenario.owner {
				author = f.creator
			}
			grant := workflowGrant(t, f, f.handler)
			if scenario.owner {
				f.targetID = grantTarget(t, f, grant)
			}
			first := f.submit(t, author, "running content stays immutable")
			run := f.run(t, first, scenario.attempts)
			ownWaiting := f.submit(t, f.handler, "removed author waiting")
			otherWaiting := f.submit(t, f.creator, "remaining author waiting")
			if scenario.priorStop {
				f.fx.Exec(t, `UPDATE collaboration_run SET forced_disposition='stopped', disposition_reason='participant_stop', progress_state='ending',claim_generation=2 WHERE id=$1`, run.ID)
			}
			if scenario.badRunning {
				f.fx.Exec(t, `UPDATE collaboration_run SET progress_state='running' WHERE id=$1`, run.ID)
			}
			f.fx.Insert(t, "collaboration_termination", testutil.Cols{
				"issue_id": f.issue.Issue.ID, "run_id": run.ID, "fence": run.Fence, "launch_nonce": run.LaunchNonce,
				"deployment_id": uuid.NewString(), "installation_id": run.InstallationID, "identity_kind": "enrolled",
				"public_key": []byte("fake supervisor key"), "token_hash": uuid.NewString(),
			})
			p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)}
			r, err := f.svc.RemoveParticipant(f.ctx, p)
			settled := scenario.attempts == 0 && !scenario.badRunning
			stopped, found := groupControlResult(r.Execution, run.GroupID)
			wantQueue, wantControl := "paused", int64(2)
			if scenario.owner {
				wantQueue, wantControl = "active", 1
			}
			if err != nil || !found || r.Code != "participant_removed" || stopped.GroupSettled != settled || stopped.QueueState != wantQueue || stopped.ControlVersion != wantControl || r.QueueState != "active" || r.ControlVersion != 1 {
				t.Fatalf("removal: %+v %v", r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='grant_required'`, grantTarget(t, f, grant)) != 1 {
				t.Fatal("departed owner lost its independent target hold")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NOT NULL AND epoch=2`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, f.issue.Issue.ID, f.creator) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='removal_return' AND from_user_id=$2 AND to_user_id=$3`, f.issue.Issue.ID, f.handler, f.creator) != 1 {
				t.Fatal("removal did not atomically revoke and return responsibility")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='participant_removed' AND content='removed author waiting' AND seq=2`, ownWaiting.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND seq=3`, otherWaiting.CommandID) != 1 {
				t.Fatal("removal lost immutable content, own cancellation or another author's waiting input")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=2`, grant) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, run.ID) != 1 {
				t.Fatal("grant or supervisor stop intent was lost")
			}
			current, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
			if err != nil {
				t.Fatal(err)
			}
			wantDisposition := "stopped"
			if settled && !scenario.owner && !scenario.priorStop {
				wantDisposition = "cancelled"
			}
			if current.ForcedDisposition.String != wantDisposition || current.SettledAt.Valid != settled || current.Fence != run.Fence || current.LaunchNonce != run.LaunchNonce || current.ClaimGeneration != 2 {
				t.Fatalf("run identity/disposition/proof changed incorrectly: %+v", current)
			}
			if !settled && current.ProofState != "unknown" {
				t.Fatal("removal fabricated process proof")
			}
			commandState := "in_flight"
			if settled {
				commandState = "cancelled"
				if scenario.owner {
					commandState = "waiting"
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state=$2`, first.CommandID, commandState) != 1 {
				t.Fatal("removal changed a potentially running command to a terminal state or misclassified unlaunched work")
			}
			if scenario.priorStop && current.DispositionReason.String != "participant_stop" {
				t.Fatal("departure reversed the first termination root")
			}
			replay, err := f.svc.RemoveParticipant(f.ctx, p)
			if err != nil || !reflect.DeepEqual(replay, r) {
				t.Fatalf("removal replay: %+v %v", replay, err)
			}
			p.RequestID = uuid.NewString()
			p.ExpectedCollaborationRevision = 2
			if _, err = f.svc.InviteParticipant(f.ctx, p); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL AND epoch=3`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=2`, grant) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, ownWaiting.CommandID) != 1 {
				t.Fatal("reinvite resurrected revoked identities, grants or commands")
			}
			if _, err = f.svc.SubmitCommand(collaborationContext(f.handler), CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "stale client after rejoin"}); !collaborationErrorCode(err, "stale_authority") {
				t.Fatalf("rejoin revived old participant epoch: %v", err)
			}
		})
	}
}

func TestCollaborationWorkflowVoluntaryLeaveAndMinimalRecovery(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	for _, tc := range []struct{ actor, code string }{{f.creator, "creator_cannot_leave"}, {f.handler, "handler_must_transfer"}} {
		if _, err := f.svc.Leave(collaborationContext(tc.actor), workflowRequest(f, 1)); !collaborationErrorCode(err, tc.code) {
			t.Fatalf("%s leave: %v", tc.code, err)
		}
	}
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.creator)}
	if _, err := f.svc.RemoveParticipant(f.ctx, p); !collaborationErrorCode(err, "creator_cannot_be_removed") {
		t.Fatalf("creator self remove: %v", err)
	}
	p.TargetUserID = util.MustParseUUID(f.outsider)
	p.RequestID = uuid.NewString()
	if _, err := f.svc.InviteParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	f.submit(t, f.outsider, "waiting before leave")
	leave := workflowRequest(f, 2)
	r, err := f.svc.Leave(collaborationContext(f.outsider), leave)
	if err != nil || r.Code != "participant_left" || r.QueueState != "paused" || r.ControlVersion != 1 {
		t.Fatalf("voluntary leave: %+v %v", r, err)
	}
	recovered, err := f.svc.GetWorkflowOperation(collaborationContext(f.outsider), f.issue.Issue.ID, leave.RequestID)
	if err != nil || recovered.Code != r.Code || recovered.CollaborationRevision != r.CollaborationRevision {
		t.Fatalf("departed actor recovery: %+v %v", recovered, err)
	}
	encoded, err := json.Marshal(recovered)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"queue", "run", "control", "target", "handler", "content", "note", "user_id"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("recovery leaked %s: %s", forbidden, encoded)
		}
	}
	if _, err = f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, leave.RequestID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("another actor recovered receipt: %v", err)
	}
	if _, err = f.svc.ResolveIssueAccess(collaborationContext(f.outsider), nil, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("departed participant retained content: %v", err)
	}
	f.fx.Member(t, f.workspace, f.outsider, "member")
	access, err := f.svc.ResolveIssueAccess(collaborationContext(f.outsider), nil, f.issue.Issue.ID)
	if err != nil || !access.Capabilities.Comment || access.Capabilities.ControlAI || access.Capabilities.Invite {
		t.Fatalf("independent source membership: %+v %v", access, err)
	}
}

func TestCollaborationWorkflowDepartureOnlyStopsAffectedExecution(t *testing.T) {
	for _, scenario := range []string{"unrelated_participant", "idle_active_owner", "leaving_author", "leaving_owner"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
			if _, err := f.svc.InviteParticipant(f.ctx, p); err != nil {
				t.Fatal(err)
			}
			if scenario == "idle_active_owner" || scenario == "leaving_owner" {
				f.targetID = grantTarget(t, f, workflowGrant(t, f, f.outsider))
			}
			var run db.CollaborationRun
			if scenario != "idle_active_owner" {
				author := f.handler
				if scenario == "leaving_author" {
					author = f.outsider
				}
				run = f.run(t, f.submit(t, author, "current execution"), 1)
			} else {
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='active' WHERE id=$1`, f.issue.SessionID)
				f.fx.Exec(t, `UPDATE collaboration_target SET queue_state='active' WHERE id=$1`, f.targetID)
				f.fx.Exec(t, `UPDATE collaboration_channel SET queue_state='active' WHERE id=(SELECT channel_id FROM collaboration_target WHERE id=$1)`, f.targetID)
			}
			f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false, enabled=false WHERE id=1`)
			var r CollaborationWorkflowReceipt
			var err error
			if strings.HasPrefix(scenario, "leaving_") {
				r, err = f.svc.Leave(collaborationContext(f.outsider), workflowRequest(f, 2))
			} else {
				p.CollaborationWorkflowRequest = workflowRequest(f, 2)
				r, err = f.svc.RemoveParticipant(f.ctx, p)
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "unrelated_participant" {
				if r.QueueState != "active" || r.ControlVersion != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND claim_generation=1`, run.ID) != 1 {
					t.Fatal("unrelated departure stopped the current execution")
				}
			} else {
				wantQueue, wantControl := "paused", int64(2)
				if scenario == "idle_active_owner" || scenario == "leaving_owner" {
					wantQueue, wantControl = "active", 1
				}
				if r.QueueState != "active" || r.ControlVersion != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state=$2 AND control_version=$3`, f.targetID, wantQueue, wantControl) != 1 {
					t.Fatalf("departure coupled owner hold with author channel pause: %+v", r)
				}
			}
			if scenario == "idle_active_owner" || scenario == "leaving_owner" {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NOT NULL`, f.targetID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='grant_required'`, f.targetID) != 1 {
					t.Fatal("owner departure retained active resource identity")
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 {
				t.Fatal("nonhandler departure changed responsibility")
			}
		})
	}
}

func TestCollaborationWorkflowReplaysExactLargeVersionsAndDisabledRefusal(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	const revision = int64(9007199254740993)
	f.fx.Exec(t, `UPDATE issue_collaboration SET revision=$2 WHERE issue_id=$1`, f.issue.Issue.ID, revision)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, revision), TargetUserID: util.MustParseUUID(f.outsider)}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.outsider)
	refusal, err := f.svc.InviteParticipant(f.ctx, p)
	if !collaborationErrorCode(err, "target_unavailable") || refusal.CollaborationRevision != revision {
		t.Fatalf("disabled target receipt: %+v %v", refusal, err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.outsider)
	replay, err := f.svc.InviteParticipant(f.ctx, p)
	if !collaborationErrorCode(err, "target_unavailable") || !reflect.DeepEqual(replay, refusal) {
		t.Fatalf("target recovery revived refused invitation: %+v %v", replay, err)
	}
	p.RequestID = uuid.NewString()
	accepted, err := f.svc.InviteParticipant(f.ctx, p)
	if err != nil || accepted.CollaborationRevision != revision+1 {
		t.Fatalf("fresh invitation: %+v %v", accepted, err)
	}
	p.RequestID = uuid.NewString()
	p.ExpectedCollaborationRevision = revision + 1
	already, err := f.svc.InviteParticipant(f.ctx, p)
	if err != nil || already.Code != "already_participant" || already.CollaborationRevision != revision+1 {
		t.Fatalf("already enrolled invitation: %+v %v", already, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND epoch=1`, f.issue.Issue.ID, f.outsider) != 1 {
		t.Fatal("repeat invitation advanced an active participant epoch")
	}
}

func TestCollaborationWorkflowNotificationsAndWorkbench(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	if _, err := f.svc.InviteParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=2`, f.issue.Issue.ID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND recipient_user_id=$2`, f.issue.Issue.ID, f.creator) != 0 {
		t.Fatal("invite notifications omitted participant or included actor")
	}
	p.CollaborationWorkflowRequest = workflowRequest(f, 2)
	if _, err := f.svc.Transfer(collaborationContext(f.handler), CollaborationTransferParams{CollaborationParticipantParams: p}); err != nil {
		t.Fatal(err)
	}
	// Archiving does not participate in the workbench predicate.
	f.fx.Exec(t, `UPDATE collaboration_notification SET read_at=now(), archived_at=now() WHERE issue_id=$1`, f.issue.Issue.ID)
	for _, tc := range []struct {
		actor, view string
		count       int64
	}{{f.outsider, "current", 1}, {f.handler, "current", 0}, {f.handler, "participating", 1}} {
		page, err := f.svc.ListWorkbench(collaborationContext(tc.actor), CollaborationWorkbenchQuery{View: tc.view, Limit: 20})
		if err != nil || page.Total != tc.count {
			t.Fatalf("workbench %s/%s: %+v %v", tc.actor, tc.view, page, err)
		}
	}
	p.CollaborationWorkflowRequest = workflowRequest(f, 3)
	if _, err := f.svc.RemoveParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=4 AND recipient_user_id=$2 AND kind='removed'`, f.issue.Issue.ID, f.outsider) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND event_seq=4`, f.issue.Issue.ID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1`, f.workspace) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("notification recipients or source inbox isolation violated")
	}
}

func TestCollaborationWorkflowCurrentAuthorityAndPersistentRefusals(t *testing.T) {
	for _, scenario := range []string{"source_member", "former_handler", "disabled_target", "missing_target", "source_departure", "stale_revision", "stale_epoch", "closed_transfer", "closed_session", "unknown_status"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
			actor, code := f.handler, "forbidden"
			switch scenario {
			case "source_member":
				actor = f.outsider
				f.fx.Member(t, f.workspace, actor, "owner")
			case "former_handler":
				f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
			case "disabled_target":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.outsider)
				code = "target_unavailable"
			case "missing_target":
				p.TargetUserID = util.MustParseUUID(uuid.NewString())
				code = "target_unavailable"
			case "source_departure":
				actor = f.creator
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor)
				code = ""
			case "stale_revision":
				p.ExpectedCollaborationRevision = 2
				code = "stale_revision"
			case "stale_epoch":
				p.ExpectedAccountEpoch = 2
				code = "stale_authority"
			case "closed_transfer":
				f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
				code = "issue_closed"
			case "closed_session":
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
				code = "issue_closed"
			case "unknown_status":
				f.fx.Exec(t, `UPDATE issue SET status='unknown_custom_status' WHERE id=$1`, f.issue.Issue.ID)
				code = "status_unavailable"
			}
			r, err := f.svc.Transfer(collaborationContext(actor), CollaborationTransferParams{CollaborationParticipantParams: p})
			if code == "" && err != nil || code != "" && !collaborationErrorCode(err, code) {
				t.Fatalf("%s: %+v %v", scenario, r, err)
			}
			if r.CollaborationRevision < 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND actor_user_id=$2 AND request_id=$3 AND completed_at IS NOT NULL`, f.issue.Issue.ID, actor, p.RequestID) != 1 {
				t.Fatal("authorized object refusal was not durably receipted")
			}
			if code != "" && f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, p.TargetUserID) != 0 {
				t.Fatal("refused transfer enrolled target")
			}
		})
	}
	// Machine and unknown callers are rejected before any database dependency.
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, ""} {
		ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: uuid.NewString(), Kind: kind})
		if _, err := (&CollaborationService{}).Leave(ctx, CollaborationWorkflowRequest{}); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("machine %s: %v", kind, err)
		}
	}
}

func TestCollaborationWorkflowConcurrentIdentityAndRevision(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	start := make(chan struct{})
	type result struct {
		r   CollaborationWorkflowReceipt
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			<-start
			r, err := f.svc.InviteParticipant(f.ctx, p)
			results <- result{r, err}
		})
	}
	close(start)
	wg.Wait()
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || a.r != b.r || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='participant_added'`, f.issue.Issue.ID) != 1 {
		t.Fatalf("concurrent same-key join: %+v %+v", a, b)
	}
	p.TargetUserID = util.MustParseUUID(f.handler)
	if _, err := f.svc.InviteParticipant(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed payload replay: %v", err)
	}
	start = make(chan struct{})
	for _, target := range []string{f.creator, f.outsider} {
		wg.Go(func() {
			<-start
			r, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
				CollaborationWorkflowRequest: workflowRequest(f, 2), TargetUserID: util.MustParseUUID(target)}})
			results <- result{r, err}
		})
	}
	close(start)
	wg.Wait()
	a, b = <-results, <-results
	if (a.err == nil) == (b.err == nil) || (a.err != nil && !collaborationErrorCode(a.err, "stale_revision")) || (b.err != nil && !collaborationErrorCode(b.err, "stale_revision")) {
		t.Fatalf("competing transfers did not produce one revision winner: %+v %+v", a, b)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='transfer'`, f.issue.Issue.ID) != 1 {
		t.Fatal("concurrent transfers duplicated handoff")
	}
}

func waitWorkflowBlocked(t *testing.T, f *collaborationQueueFixture, blocker int, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		f.fx.QueryRow(t, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked)
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("workflow did not wait for the expected database lock")
		}
		select {
		case err := <-done:
			t.Fatalf("workflow bypassed lock: %v", err)
		default:
		}
	}
}

func TestCollaborationWorkflowRechecksAccountsAndRolesAfterLock(t *testing.T) {
	for _, scenario := range []string{"target_disabled", "former_handler"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			lockUser := f.outsider
			if scenario == "former_handler" {
				lockUser = f.handler
			}
			if _, err = db.New(tx).LockCollaborationUsers(f.ctx, []pgtype.UUID{util.MustParseUUID(lockUser)}); err != nil {
				t.Fatal(err)
			}
			var blocker int
			if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := f.svc.InviteParticipant(collaborationContext(f.handler), CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)})
				done <- err
			}()
			waitWorkflowBlocked(t, f, blocker, done)
			want := "target_unavailable"
			if scenario == "target_disabled" {
				_, err = tx.Exec(f.ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, lockUser)
			} else {
				want = "forbidden"
				_, err = tx.Exec(f.ctx, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			if err = <-done; !collaborationErrorCode(err, want) {
				t.Fatalf("workflow used stale pre-lock identity: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.outsider) != 0 {
				t.Fatal("stale actor/target enrolled a participant")
			}
		})
	}
}

func TestCollaborationWorkflowRemovalRechecksPermitUnderSessionLock(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	run := f.run(t, f.submit(t, f.handler, "about to be permitted"), 0)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = db.New(tx).LockCollaborationSession(f.ctx, f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	var blocker int
	if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
		done <- err
	}()
	waitWorkflowBlocked(t, f, blocker, done)
	// This is a competing permit fixture, not an implemented launch API.
	if _, err = tx.Exec(f.ctx, `UPDATE collaboration_run SET authorized_attempt_seq=1 WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND authorized_attempt_seq=1 AND proof_state='unknown' AND settled_at IS NULL AND forced_disposition='stopped'`, run.ID) != 1 {
		t.Fatal("removal settled or classified a concurrently permitted run as never-launched cancellation")
	}
}

type workflowFaultStarter struct {
	TxStarter
	needle string
}

type workflowFaultTx struct {
	pgx.Tx
	needle string
}

func (s workflowFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &workflowFaultTx{Tx: tx, needle: s.needle}, nil
}

func (t *workflowFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(strings.Join(strings.Fields(sql), " "), t.needle) {
		return queueFailureRow{errors.New("injected required workflow write failure")}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

func (t *workflowFaultTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(strings.Join(strings.Fields(sql), " "), t.needle) {
		return pgconn.CommandTag{}, errors.New("injected required workflow write failure")
	}
	return t.Tx.Exec(ctx, sql, args...)
}

func (t *workflowFaultTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(strings.Join(strings.Fields(sql), " "), t.needle) {
		return nil, errors.New("injected required workflow write failure")
	}
	return t.Tx.Query(ctx, sql, args...)
}

func TestCollaborationWorkflowRequiredWriteFailuresRollbackEverything(t *testing.T) {
	for _, needle := range []string{
		"UPDATE issue SET assignee_type", "INSERT INTO issue_handoff", "UPDATE issue_participant SET", "UPDATE collaboration_grant SET",
		"UPDATE collaboration_command SET", "UPDATE collaboration_channel SET queue_state", "UPDATE collaboration_target t SET", "UPDATE collaboration_execution_group", "UPDATE collaboration_delegation", "UPDATE collaboration_run SET",
		"UPDATE collaboration_termination SET", "UPDATE issue_collaboration SET revision", "INSERT INTO collaboration_event",
		"INSERT INTO collaboration_notification", "UPDATE collaboration_operation",
	} {
		t.Run(needle, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			grant := workflowGrant(t, f, f.handler)
			f.targetID = grantTarget(t, f, grant)
			run := f.run(t, f.submit(t, f.handler, "running before failed removal"), 1)
			waiting := f.submit(t, f.handler, "waiting before failed removal")
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)}
			if _, err := f.svc.RemoveParticipant(f.ctx, p); err == nil {
				t.Fatal("required write failure reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL AND epoch=1`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=1`, grant) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND active_group_id=$2 AND config_version=1 AND control_version=1`, f.targetID, run.GroupID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND proof_state='unknown' AND claim_generation=1`, run.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='participant_removed'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("required write failure committed partial participant/handler/queue/resource/event/receipt state")
			}
		})
	}
}

func TestCollaborationWorkflowLostCommitRecoveryAndRefusedKey(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}, Note: "recover exactly this transfer"}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.Transfer(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain commit was successful or retried: %v, %d", err, fault.begins.Load())
	}
	f.svc.TxStarter = f.pool
	r, err := f.svc.Transfer(f.ctx, p)
	if err != nil || r.Code != "transferred" || r.CollaborationRevision != 2 || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatalf("original-key recovery: %+v %v", r, err)
	}
	p.RequestID = uuid.NewString()
	p.ExpectedCollaborationRevision = 3
	refused, err := f.svc.Transfer(f.ctx, p)
	if !collaborationErrorCode(err, "stale_revision") {
		t.Fatalf("expected stale refusal: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_collaboration SET revision=3 WHERE issue_id=$1`, f.issue.Issue.ID)
	replay, err := f.svc.Transfer(f.ctx, p)
	if !collaborationErrorCode(err, "stale_revision") || !reflect.DeepEqual(replay, refused) {
		t.Fatalf("refused key became a future transfer: %+v %v", replay, err)
	}
}

func TestCollaborationWorkflowLostLeaveReplyRecoversWithoutObjectAccess(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	if _, err := f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}); err != nil {
		t.Fatal(err)
	}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	p := workflowRequest(f, 2)
	ctx := collaborationContext(f.outsider)
	if _, err := f.svc.Leave(ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain leave commit was successful or retried: %v", err)
	}
	f.svc.TxStarter = f.pool
	if _, err := f.svc.Leave(ctx, p); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("departed actor unexpectedly retained mutation access: %v", err)
	}
	r, err := f.svc.GetWorkflowOperation(ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || r.Code != "participant_left" || r.CollaborationRevision != 3 {
		t.Fatalf("lost leave reply cannot be recovered safely: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='participant_left'`, f.issue.Issue.ID) != 1 {
		t.Fatal("lost leave reply duplicated the departure event")
	}
}

func TestCollaborationWorkflowTransferRecordsIterationAndSupersedesInformation(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Current iteration", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
	f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
	request := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator, "from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "todo"})
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, request)
	p := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}, Note: "ordinary next stage"}
	if _, err := f.svc.Transfer(collaborationContext(f.handler), p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, request) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND active_information_request_id IS NULL`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'user_id'=$2 AND before_facts->>'assignee_id'=$2 AND after_facts->>'assignee_id'=$3`, iid, f.handler, f.outsider) != 1 {
		t.Fatal("ordinary transfer failed to supersede information flow or record actual assignee facts")
	}
}

func TestCollaborationWorkflowInviteTransferPreservesExecution(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	command := f.submit(t, f.handler, "immutable running command")
	run := f.run(t, command, 1)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	added, err := f.svc.InviteParticipant(f.ctx, p)
	if err != nil || added.Code != "participant_added" || added.CollaborationRevision != 2 {
		t.Fatalf("invite: %+v %v", added, err)
	}
	replay, err := f.svc.InviteParticipant(f.ctx, p)
	if err != nil || replay != added {
		t.Fatalf("invite replay: %+v %v", replay, err)
	}
	transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflowRequest(f, 2), TargetUserID: p.TargetUserID}, Note: "Stage ready for testing"}
	result, err := f.svc.Transfer(collaborationContext(f.handler), transfer)
	if err != nil || result.Code != "transferred" || result.CollaborationRevision != 3 {
		t.Fatalf("transfer: %+v %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND creator_id=$3 AND status='todo'`, f.issue.Issue.ID, f.outsider, f.creator) != 1 {
		t.Fatal("transfer failed to move responsibility without changing source status/creator")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND active_group_id=$2 AND control_version=1 AND config_version=1`, f.targetID, run.GroupID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL AND claim_generation=1`, run.ID) != 1 {
		t.Fatal("ordinary transfer changed execution")
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.outsider) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2`, f.issue.Issue.ID, f.outsider) != 0 {
		t.Fatal("invitation invented workspace membership or personal resource consent")
	}
	transfer.RequestID = uuid.NewString()
	transfer.ExpectedCollaborationRevision = 3
	if _, err = f.svc.Transfer(collaborationContext(f.handler), transfer); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("former handler retained transfer authority: %v", err)
	}
	access, err := f.svc.ResolveIssueAccess(collaborationContext(f.handler), nil, f.issue.Issue.ID)
	if err != nil || !access.Capabilities.ControlAI || access.Capabilities.Transfer {
		t.Fatalf("former handler permissions: %+v %v", access, err)
	}
}
