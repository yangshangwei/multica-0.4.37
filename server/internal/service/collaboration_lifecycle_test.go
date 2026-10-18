package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func newCollaborationLifecycleFixture(t *testing.T) *collaborationQueueFixture {
	t.Helper()
	f := newCollaborationQueueFixture(t)
	// The low-level Workspace fixture does not run real workspace creation's
	// status seeding. Lifecycle actions require the actual source catalog.
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	return f
}

func lifecycleParams(t *testing.T, f *collaborationQueueFixture, action, target string, revision, epoch int64) CollaborationLifecycleParams {
	t.Helper()
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, action)
	if err != nil {
		t.Fatal(err)
	}
	p := CollaborationLifecycleParams{CollaborationWorkflowRequest: workflowRequest(f, revision), Action: action, TargetStatus: target, StatusOptionsVersion: options.StatusOptionsVersion}
	p.ExpectedLifecycleEpoch = epoch
	return p
}

func statusParams(t *testing.T, f *collaborationQueueFixture, target string, revision int64) CollaborationStatusParams {
	t.Helper()
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "change_status")
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationStatusParams{CollaborationWorkflowRequest: workflowRequest(f, revision), TargetStatus: target, StatusOptionsVersion: options.StatusOptionsVersion}
}

func TestCollaborationLifecycleCloseAndReopenPreservePermanentQueue(t *testing.T) {
	for _, tc := range []struct {
		name, action, target string
		attempts             int64
		priorStop            bool
	}{
		{name: "complete_permitted", action: "complete", target: "done", attempts: 1},
		{name: "cancel_never_permitted", action: "cancel", target: "cancelled"},
		{name: "cancel_preserves_first_stop", action: "cancel", target: "cancelled", attempts: 1, priorStop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			// Source departure must not invalidate the independent resource owner.
			grant := workflowGrant(t, f, f.handler)
			f.targetID = grantTarget(t, f, grant)
			first := f.submit(t, f.handler, "current command")
			run := f.run(t, first, tc.attempts)
			second := f.submit(t, f.creator, "creator waiting")
			third := f.submit(t, f.handler, "handler waiting")
			information := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator, "from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "todo"})
			f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, information)
			f.fx.Insert(t, "collaboration_termination", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": run.ID, "fence": run.Fence, "launch_nonce": run.LaunchNonce,
				"deployment_id": uuid.NewString(), "installation_id": run.InstallationID, "identity_kind": "enrolled", "public_key": []byte("fixture supervisor"), "token_hash": uuid.NewString()})
			if tc.priorStop {
				f.fx.Exec(t, `UPDATE collaboration_run SET forced_disposition='stopped', disposition_reason='participant_stop', progress_state='ending',claim_generation=2 WHERE id=$1`, run.ID)
			}
			// Source membership is independent of the creator's participation.
			f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
			f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false, creation_enabled=false WHERE id=1`)
			p := lifecycleParams(t, f, tc.action, tc.target, 1, 1)
			r, err := f.svc.ChangeLifecycle(f.ctx, p)
			stopped, found := groupControlResult(r.Execution, run.GroupID)
			if err != nil || !found || r.QueueState != "closed" || r.ControlVersion != 2 || r.CollaborationRevision != 2 || stopped.GroupSettled != (tc.attempts == 0) {
				t.Fatalf("close: %+v %v", r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status=$2 AND assignee_id=$3 AND creator_id=$4`, f.issue.Issue.ID, tc.target, f.handler, f.creator) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=2 AND authority_epoch=2 AND lifecycle_epoch=2 AND active_information_request_id IS NULL`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, information) != 1 {
				t.Fatal("close lost status, responsibility or lifecycle versions")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled' AND reason=$2`, []pgtype.UUID{second.CommandID, third.CommandID}, "issue_"+r.Code) != 2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, run.ID) != 1 {
				t.Fatal("close did not cancel every waiting command or retain narrow stop intent")
			}
			closedRun, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
			if err != nil {
				t.Fatal(err)
			}
			if closedRun.Fence != run.Fence || closedRun.LaunchNonce != run.LaunchNonce || closedRun.ClaimGeneration != 2 || closedRun.SettledAt.Valid != (tc.attempts == 0) {
				t.Fatalf("close changed immutable identity/proof: %+v", closedRun)
			}
			if tc.attempts > 0 && (closedRun.ProofState != "unknown" || closedRun.ForcedDisposition.String != "stopped") {
				t.Fatal("potentially live run was classified as cancelled or proved exited")
			}
			if tc.priorStop && closedRun.DispositionReason.String != "participant_stop" {
				t.Fatal("close replaced the first forced root")
			}
			replay, err := f.svc.ChangeLifecycle(f.ctx, p)
			if err != nil || !reflect.DeepEqual(replay, r) {
				t.Fatalf("closed-state replay: %+v %v", replay, err)
			}
			request := f.request()
			request.ExpectedLifecycleEpoch = 2
			if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: request, TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "must not execute"}); err == nil {
				t.Fatal("closed issue accepted command")
			}
			p = lifecycleParams(t, f, "reopen", "todo", 2, 2)
			r, err = f.svc.ChangeLifecycle(f.ctx, p)
			if err != nil || r.QueueState != "paused" || r.ControlVersion != 3 || r.CollaborationRevision != 3 {
				t.Fatalf("reopen: %+v %v", r, err)
			}
			after, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
			if err != nil || !reflect.DeepEqual(after, closedRun) {
				t.Fatalf("reopen rewrote old run: %+v %v", after, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=3 AND authority_epoch=3 AND lifecycle_epoch=3`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled'`, []pgtype.UUID{second.CommandID, third.CommandID}) != 2 {
				t.Fatal("reopen duplicated the session or resurrected cancelled commands")
			}
			request = f.request()
			request.ExpectedLifecycleEpoch = 3
			if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: request, TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "new intent after reopen"}); err != nil {
				t.Fatal(err)
			}
			if tc.attempts > 0 {
				request.RequestID = uuid.NewString()
				control := f.issueControl(t)
				control.CollaborationQueueRequest = request
				_, err = f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: control, ExpectedExecutionEpoch: 2})
				if !collaborationErrorCode(err, "collaboration_execution_unavailable") {
					t.Fatalf("reopen bypassed the disabled execution gate: %v", err)
				}
				// With the global gate available, explicit Resume All may release
				// the issue pause, but the unproven channel must remain paused.
				executionEpoch := enableControlGate(t, f.collaborationFixture)
				control.RequestID = uuid.NewString()
				resumed, resumeErr := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: control, ExpectedExecutionEpoch: executionEpoch})
				blocked, found := groupControlResult(&resumed, run.GroupID)
				if resumeErr != nil || resumed.Code != "partially_resumed" || !found || blocked.Code != "stop_unconfirmed" || blocked.QueueState != "paused" {
					t.Fatalf("reopen cleared execution barrier: %+v %v", resumed, resumeErr)
				}
				retainedRun, readErr := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
				if readErr != nil || !reflect.DeepEqual(retainedRun, closedRun) {
					t.Fatalf("Resume changed the retained proof identity: %+v %v", retainedRun, readErr)
				}
			}
		})
	}
}

func TestCollaborationLifecycleAuthorityAndExplicitReopen(t *testing.T) {
	for _, actorKind := range []string{"handler", "source_admin", "disabled_creator", "revoked_creator"} {
		t.Run(actorKind, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			p := lifecycleParams(t, f, "complete", "done", 1, 1)
			actor := f.handler
			switch actorKind {
			case "source_admin":
				actor = f.outsider
				f.fx.Member(t, f.workspace, actor, "admin")
			case "disabled_creator":
				actor = f.creator
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, actor)
			case "revoked_creator":
				actor = f.creator
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, actor)
			}
			if _, err := f.svc.ChangeLifecycle(collaborationContext(actor), p); err == nil {
				t.Fatal("non-active creator completed issue")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, f.issue.Issue.ID) != 1 {
				t.Fatal("refused lifecycle changed state")
			}
		})
	}
	f := newCollaborationLifecycleFixture(t)
	p := statusParams(t, f, "in_review", 1)
	f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
	f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
	for _, actor := range []string{f.creator, f.handler} {
		p.RequestID = uuid.NewString()
		if _, err := f.svc.ChangeStatus(collaborationContext(actor), p); !collaborationErrorCode(err, "issue_closed") {
			t.Fatalf("generic status reopened ended issue: %v", err)
		}
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, ""} {
		ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: kind})
		if _, err := (&CollaborationService{}).ChangeLifecycle(ctx, CollaborationLifecycleParams{}); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("machine lifecycle: %v", err)
		}
	}
}

func TestCollaborationStatusChangesOnlyStatusAndRecordsIteration(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Current iteration", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
	f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
	req := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator, "from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "todo"})
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, req)
	command := f.submit(t, f.handler, "immutable running input")
	run := f.run(t, command, 1)
	waiting := f.submit(t, f.creator, "waiting input")
	beforeNotifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID)
	for i, target := range []string{"blocked", "in_review"} {
		p := statusParams(t, f, target, int64(i+1))
		r, err := f.svc.ChangeStatus(collaborationContext(f.handler), p)
		if err != nil || r.Code != "status_changed" || r.ControlVersion != 1 || r.QueueState != "active" {
			t.Fatalf("manual status: %+v %v", r, err)
		}
		replay, err := f.svc.ChangeStatus(collaborationContext(f.handler), p)
		if err != nil || !reflect.DeepEqual(replay, r) {
			t.Fatalf("status replay: %+v %v", replay, err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=3 AND authority_epoch=1 AND lifecycle_epoch=1 AND active_information_request_id=$2`, f.issue.Issue.ID, req) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='open'`, req) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND active_group_id=$2 AND next_command_seq=3 AND control_version=1`, f.targetID, run.GroupID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND content='waiting input'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != beforeNotifications {
		t.Fatal("manual status changed authority, information request, queue or notifications")
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'user_id'=$2 AND after_facts->>'status_key'=ANY($3::text[])`, iid, f.handler, []string{"blocked", "in_review"}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='status_changed' AND actor_user_id=$2 AND payload->>'previous_status'=ANY($3::text[])`, f.issue.Issue.ID, f.handler, []string{"todo", "blocked"}) != 2 {
		t.Fatal("manual status lost atomic actor/before/after audit")
	}
	current, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
	if err != nil || !reflect.DeepEqual(current, run) {
		t.Fatalf("manual status changed active execution: %+v %v", current, err)
	}
}

func TestCollaborationStatusOptionsAreScopedAndRefusalsReplay(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	custom := f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "qa_ready", "name": "Ready", "category": "in_review", "color": "#000000"})
	p := statusParams(t, f, "qa_ready", 1)
	f.fx.Exec(t, `UPDATE issue_status SET archived_at=now(), updated_at=now() WHERE id=$1`, custom)
	r, err := f.svc.ChangeStatus(f.ctx, p)
	if !collaborationErrorCode(err, "status_options_changed") || r.StatusOptions == nil || r.StatusOptions.StatusOptionsVersion == p.StatusOptionsVersion {
		t.Fatalf("catalog change not reported: %+v %v", r, err)
	}
	f.fx.Exec(t, `UPDATE issue_status SET archived_at=NULL, updated_at=now() WHERE id=$1`, custom)
	replay, err := f.svc.ChangeStatus(f.ctx, p)
	if !collaborationErrorCode(err, "status_options_changed") || replay.Code != r.Code || replay.Status != r.Status || replay.CollaborationRevision != r.CollaborationRevision ||
		replay.StatusOptions == nil || replay.StatusOptions.StatusOptionsVersion == r.StatusOptions.StatusOptionsVersion {
		t.Fatalf("catalog refusal lost original result or current options: %+v %v", replay, err)
	}
	stored, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(f.creator), RequestID: p.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeWorkflowReceipt(stored)
	if err != nil || !reflect.DeepEqual(original, r) || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, f.issue.Issue.ID) != 1 {
		t.Fatalf("reprojection changed historical receipt or retried mutation: %+v %v", original, err)
	}
	for _, target := range []string{"done", "cancelled", "unknown"} {
		p = statusParams(t, f, target, 1)
		if _, err = f.svc.ChangeStatus(f.ctx, p); !collaborationErrorCode(err, "status_unavailable") {
			t.Fatalf("manual status accepted invalid category: %s %v", target, err)
		}
	}
	other := f.fx.Workspace(t, "Other catalog", "other-catalog-"+uuid.NewString())
	f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": other, "key": "other_only", "name": "Other only", "category": "todo", "color": "#000000"})
	opts, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "change_status")
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range opts.Options {
		if option.Key == "other_only" || option.Category == "done" || option.Category == "cancelled" {
			t.Fatalf("unavailable source option leaked: %+v", option)
		}
	}
	p = statusParams(t, f, "qa_ready", 1)
	p.StatusOptionsVersion = ""
	if _, err = f.svc.ChangeStatus(f.ctx, p); !collaborationErrorCode(err, "invalid_status_request") {
		t.Fatalf("missing options version accepted: %v", err)
	}
}

func TestCollaborationStatusRefusalReplayDoesNotRestoreIneligibleOptions(t *testing.T) {
	for _, scenario := range []string{"ended", "revoked_manager"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			custom := f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "ready", "name": "Ready", "category": "in_review", "color": "#000000"})
			p := statusParams(t, f, "ready", 1)
			actor := f.creator
			if scenario == "revoked_manager" {
				actor = f.handler
			}
			ctx := collaborationContext(actor)
			f.fx.Exec(t, `UPDATE issue_status SET name='Updated name', updated_at=now() WHERE id=$1`, custom)
			refused, err := f.svc.ChangeStatus(ctx, p)
			if !collaborationErrorCode(err, "status_options_changed") || refused.StatusOptions == nil {
				t.Fatalf("initial refusal: %+v %v", refused, err)
			}
			if scenario == "ended" {
				closeRequest := lifecycleParams(t, f, "complete", "done", 1, 1)
				if _, err = f.svc.ChangeLifecycle(f.ctx, closeRequest); err != nil {
					t.Fatal(err)
				}
			} else {
				f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
			}
			replay, err := f.svc.ChangeStatus(ctx, p)
			if scenario == "ended" {
				if !collaborationErrorCode(err, "status_options_changed") || replay.StatusOptions != nil || replay.Code != refused.Code || replay.CollaborationRevision != refused.CollaborationRevision {
					t.Fatalf("ended replay disclosed stale options: %+v %v", replay, err)
				}
			} else if !collaborationErrorCode(err, "forbidden") || replay.StatusOptions != nil {
				t.Fatalf("former manager recovered action catalog: %+v %v", replay, err)
			}
			minimal, err := f.svc.GetWorkflowOperation(ctx, f.issue.Issue.ID, p.RequestID)
			if err != nil || minimal.Code != refused.Code || minimal.CollaborationRevision != refused.CollaborationRevision {
				t.Fatalf("minimal refusal recovery changed: %+v %v", minimal, err)
			}
		})
	}
}

func TestCollaborationLifecycleRequiredWritesRollbackAndLostCommitRecovers(t *testing.T) {
	for _, needle := range []string{"UPDATE issue AS i SET", "UPDATE collaboration_session SET queue_state", "UPDATE collaboration_command SET state", "UPDATE collaboration_run", "UPDATE issue_collaboration SET", "INSERT INTO collaboration_event", "UPDATE collaboration_operation"} {
		t.Run(needle, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			run := f.run(t, f.submit(t, f.handler, "current"), 1)
			f.submit(t, f.creator, "waiting")
			p := lifecycleParams(t, f, "complete", "done", 1, 1)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			if _, err := f.svc.ChangeLifecycle(f.ctx, p); err == nil {
				t.Fatal("injected failure was ignored")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1 AND lifecycle_epoch=1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_group_id=$2`, f.targetID, run.GroupID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("required failure partially committed lifecycle")
			}
		})
	}
	f := newCollaborationLifecycleFixture(t)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.ChangeLifecycle(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain lifecycle commit auto-retried: %v", err)
	}
	f.svc.TxStarter = f.pool
	r, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil || r.Code != "completed" {
		t.Fatalf("lost lifecycle reply recovery: %+v %v", r, err)
	}
	op, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || op.Code != r.Code {
		t.Fatalf("minimal lifecycle operation recovery: %+v %v", op, err)
	}
}

func TestCollaborationLifecycleConcurrentCloseAndSubmission(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	start := make(chan struct{})
	done := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Go(func() { <-start; _, err := f.svc.ChangeLifecycle(f.ctx, p); done <- err })
	wg.Go(func() {
		<-start
		_, err := f.svc.SubmitCommand(collaborationContext(f.handler), CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "concurrent input"})
		done <- err
	})
	close(start)
	wg.Wait()
	close(done)
	for err := range done {
		if err != nil && !collaborationErrorCode(err, "stale_authority") && !collaborationErrorCode(err, "command_unavailable") {
			t.Fatal(err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='closed'`, f.issue.SessionID) != 1 {
		t.Fatal("close race left dispatchable waiting work")
	}
}

func TestCollaborationLifecycleRechecksCatalogAndPermitAfterLockWait(t *testing.T) {
	for _, scenario := range []string{"catalog", "permit", "former_handler"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			custom := f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "finished", "name": "Finished", "category": "done", "color": "#000000"})
			p := lifecycleParams(t, f, "complete", "finished", 1, 1)
			sp := statusParams(t, f, "in_progress", 1)
			run := f.run(t, f.submit(t, f.handler, "potential launch"), 0)
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			q := db.New(tx)
			switch scenario {
			case "catalog":
				err = q.LockIssueStatusCatalog(f.ctx, util.MustParseUUID(f.workspace))
			case "permit":
				_, err = q.LockCollaborationSession(f.ctx, f.issue.Issue.ID)
			case "former_handler":
				_, err = q.LockCollaborationUsers(f.ctx, []pgtype.UUID{util.MustParseUUID(f.handler)})
			}
			if err != nil {
				t.Fatal(err)
			}
			var blocker int
			if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				var err error
				if scenario == "former_handler" {
					_, err = f.svc.ChangeStatus(collaborationContext(f.handler), sp)
				} else {
					_, err = f.svc.ChangeLifecycle(f.ctx, p)
				}
				done <- err
			}()
			waitWorkflowBlocked(t, f, blocker, done)
			want := ""
			switch scenario {
			case "catalog":
				_, err = tx.Exec(f.ctx, `UPDATE issue_status SET archived_at=now(),updated_at=now() WHERE id=$1`, custom)
				want = "status_options_changed"
			case "permit":
				_, err = tx.Exec(f.ctx, `UPDATE collaboration_run SET authorized_attempt_seq=1 WHERE id=$1`, run.ID)
			case "former_handler":
				_, err = tx.Exec(f.ctx, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
				want = "forbidden"
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			err = <-done
			if want == "" && err != nil || want != "" && !collaborationErrorCode(err, want) {
				t.Fatalf("stale lock facts used: %v", err)
			}
			if scenario == "permit" && f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND authorized_attempt_seq=1 AND proof_state='unknown' AND settled_at IS NULL AND forced_disposition='stopped'`, run.ID) != 1 {
				t.Fatal("concurrent permit was treated as never launched")
			}
		})
	}
}

func TestCollaborationLifecycleVersionsRemainExact(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	const large int64 = 9007199254740993
	f.fx.Exec(t, `UPDATE issue_collaboration SET revision=$2, lifecycle_epoch=$2, authority_epoch=$2 WHERE issue_id=$1`, f.issue.Issue.ID, large)
	f.fx.Exec(t, `UPDATE collaboration_session SET control_version=$2 WHERE id=$1`, f.issue.SessionID, large)
	p := lifecycleParams(t, f, "complete", "done", large, large)
	r, err := f.svc.ChangeLifecycle(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(raw), `"collaboration_revision":"9007199254740994"`) || !strings.Contains(string(raw), `"control_version":"9007199254740994"`) {
		t.Fatalf("large versions narrowed: %s %v", raw, err)
	}
}

func TestCollaborationStatusOptionsRequireEligibleRoleAndActualCatalog(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	f.fx.Member(t, f.workspace, f.outsider, "admin")
	for _, tc := range []struct{ actor, action string }{
		{f.handler, "complete"}, {f.handler, "cancel"}, {f.handler, "reopen"},
		{f.outsider, "change_status"}, {f.creator, "reopen"},
	} {
		if _, err := f.svc.StatusOptions(collaborationContext(tc.actor), f.issue.Issue.ID, tc.action); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("ineligible action disclosed catalog: %+v %v", tc, err)
		}
	}
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
	if _, err := f.svc.StatusOptions(collaborationContext(f.outsider), f.issue.Issue.ID, "change_status"); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("ordinary participant received source options: %v", err)
	}
	for _, field := range []string{"source_status", "canonical_target"} {
		p := lifecycleParams(t, f, "complete", "done", 1, 1)
		switch field {
		case "source_status":
			f.fx.Exec(t, `UPDATE issue SET status='missing_status' WHERE id=$1`, f.issue.Issue.ID)
		case "canonical_target":
			f.fx.Exec(t, `DELETE FROM issue_status WHERE workspace_id=$1 AND key='done'`, f.workspace)
		}
		if _, err := f.svc.ChangeLifecycle(f.ctx, p); !collaborationErrorCode(err, "status_unavailable") {
			t.Fatalf("missing catalog silently defaulted: %s %v", field, err)
		}
		f.fx.Exec(t, `UPDATE issue SET status='todo' WHERE id=$1`, f.issue.Issue.ID)
	}
}

func TestCollaborationLifecycleConcurrentSameRequestHasOneEffect(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	p := lifecycleParams(t, f, "complete", "done", 1, 1)
	start := make(chan struct{})
	type result struct {
		receipt CollaborationWorkflowReceipt
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { <-start; r, err := f.svc.ChangeLifecycle(f.ctx, p); results <- result{r, err} })
	}
	close(start)
	wg.Wait()
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || !reflect.DeepEqual(a.receipt, b.receipt) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='completed'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND control_version=2`, f.issue.SessionID) != 1 {
		t.Fatalf("same key duplicated lifecycle side effects: %+v %+v", a, b)
	}
	p.TargetStatus = "cancelled"
	if _, err := f.svc.ChangeLifecycle(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed payload reused completed key: %v", err)
	}
}

type lifecycleCommitBarrierStarter struct {
	TxStarter
	ready   chan int
	release chan struct{}
}
type lifecycleCommitBarrierTx struct {
	pgx.Tx
	ready   chan int
	release chan struct{}
}

func (s lifecycleCommitBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &lifecycleCommitBarrierTx{Tx: tx, ready: s.ready, release: s.release}, nil
}

func (tx *lifecycleCommitBarrierTx) Commit(ctx context.Context) error {
	tx.ready <- int(tx.Conn().PgConn().PID())
	select {
	case <-tx.release:
		return tx.Tx.Commit(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCollaborationStatusCatalogRowsRemainLockedUntilCommit(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	custom := f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "review_ready", "name": "Review ready", "category": "in_review", "color": "#000000"})
	p := statusParams(t, f, "review_ready", 1)
	ready, release := make(chan int, 1), make(chan struct{})
	f.svc.TxStarter = lifecycleCommitBarrierStarter{TxStarter: f.pool, ready: ready, release: release}
	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := f.svc.ChangeStatus(ctx, p); done <- err }()
	var blocker int
	select {
	case blocker = <-ready:
	case <-ctx.Done():
		t.Fatal("status write did not reach commit barrier")
	}
	metadataDone := make(chan error, 1)
	go func() {
		_, err := f.pool.Exec(ctx, `UPDATE issue_status SET name='Changed name', updated_at=now() WHERE id=$1`, custom)
		metadataDone <- err
	}()
	waitWorkflowBlocked(t, f, blocker, metadataDone)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-metadataDone; err != nil {
		t.Fatal(err)
	}
	f.svc.TxStarter = f.pool
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "change_status")
	if err != nil || options.StatusOptionsVersion == p.StatusOptionsVersion {
		t.Fatalf("committed catalog metadata did not change digest: %+v %v", options, err)
	}
}

func TestCollaborationLifecycleIterationFailureRollsBackCloseAndReopen(t *testing.T) {
	for _, action := range []string{"complete", "reopen"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Lifecycle period", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
			f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
			f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
			f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
			target, before, queue := "done", "todo", "paused"
			if action == "reopen" {
				target, before, queue = "todo", "done", "closed"
				f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
			}
			p := lifecycleParams(t, f, action, target, 1, 1)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "INSERT INTO iteration_event"}
			if _, err := f.svc.ChangeLifecycle(f.ctx, p); err == nil {
				t.Fatal("failed iteration fact was ignored")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status=$2`, f.issue.Issue.ID, before) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state=$2 AND control_version=1`, f.issue.SessionID, queue) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND lifecycle_epoch=1 AND revision=1`, f.issue.Issue.ID) != 1 {
				t.Fatal("iteration error partially changed lifecycle")
			}
			f.svc.TxStarter = f.pool
			if _, err := f.svc.ChangeLifecycle(f.ctx, p); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'user_id'=$2 AND before_facts->>'status_key'=$3 AND after_facts->>'status_key'=$4`, iid, f.creator, before, target) != 1 {
				t.Fatal("lifecycle did not record actual source status facts")
			}
		})
	}
}
