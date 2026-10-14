package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func newCollaborationHandoffFixture(t *testing.T) *collaborationQueueFixture {
	t.Helper()
	f := newCollaborationLifecycleFixture(t)
	// Handoff rows reference the issue; delete them before the issue cleanup.
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	return f
}

// Build a confirmation for service tests, including negative role/closed cases
// where the actor can no longer fetch fresh options. HTTP projection has its
// own authorization coverage in collaboration_handoff_status_test.go.
func handoffTestOptions(t *testing.T, f *collaborationQueueFixture, action string, requestID pgtype.UUID) CollaborationStatusOptions {
	t.Helper()
	issue, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := f.svc.Queries.ListIssueStatusEntries(f.ctx, db.ListIssueStatusEntriesParams{WorkspaceID: issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	var request db.IssueHandoff
	if requestID.Valid {
		request, err = f.svc.Queries.GetCollaborationInformationRequest(f.ctx, db.GetCollaborationInformationRequestParams{IssueID: issue.ID, RequestID: requestID})
		if err != nil {
			t.Fatal(err)
		}
	}
	options, err := collaborationStatusOptions(issue, action, catalog, request)
	if err != nil {
		t.Fatal(err)
	}
	return options
}

func handoffAsk(t *testing.T, f *collaborationQueueFixture, revision int64, target, note string) CollaborationInformationRequestParams {
	t.Helper()
	options := handoffTestOptions(t, f, "request_information", pgtype.UUID{})
	return CollaborationInformationRequestParams{CollaborationTransferParams: CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflowRequest(f, revision), TargetUserID: util.MustParseUUID(target)}, Note: note},
		ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion}
}

func handoffReturn(t *testing.T, f *collaborationQueueFixture, revision int64, request pgtype.UUID, status string) CollaborationInformationReturnParams {
	t.Helper()
	options := handoffTestOptions(t, f, "return_information", request)
	return CollaborationInformationReturnParams{CollaborationWorkflowRequest: workflowRequest(f, revision), InformationRequestID: request,
		ExpectedStatus: status, StatusOptionsVersion: options.StatusOptionsVersion}
}

func handoffAcceptance(t *testing.T, f *collaborationQueueFixture, revision int64, note string) CollaborationAcceptanceParams {
	t.Helper()
	options := handoffTestOptions(t, f, "submit_acceptance", pgtype.UUID{})
	return CollaborationAcceptanceParams{CollaborationWorkflowRequest: workflowRequest(f, revision), Note: note,
		ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion}
}

func activeInformationRequest(t *testing.T, f *collaborationQueueFixture) pgtype.UUID {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return access.InformationRequestID
}

func (f *collaborationQueueFixture) invite(t *testing.T, revision int64, target string) {
	t.Helper()
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, revision), TargetUserID: util.MustParseUUID(target)}
	if _, err := f.svc.InviteParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
}

// AC25: D asks P, P asks Q; each return lands on that layer's requester and
// restores its saved status, then the parent request becomes current again.
func TestCollaborationInformationChainReturnsLayerByLayer(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Handoff iteration", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
	f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
	waiting := f.submit(t, f.creator, "queued while people talk")
	run := f.run(t, f.submit(t, f.handler, "running while people talk"), 1)
	developer, product, specialist := collaborationContext(f.handler), f.ctx, collaborationContext(f.outsider)

	first, err := f.svc.RequestInformation(developer, handoffAsk(t, f, 1, f.creator, "Which locale wins?"))
	if err != nil || first.Code != "information_requested" || first.CollaborationRevision != 2 || first.QueueState != "active" || first.ControlVersion != 1 {
		t.Fatalf("first request: %+v %v", first, err)
	}
	r1 := activeInformationRequest(t, f)
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='blocked'`, f.issue.Issue.ID, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND requester_user_id=$2 AND to_user_id=$3 AND previous_status='todo' AND parent_request_id IS NULL AND note='Which locale wins?'`, r1, f.handler, f.creator) != 1 {
		t.Fatal("request did not move responsibility, block the issue and persist its requester")
	}
	// One business action leaves one iteration fact with handler and status.
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'user_id'=$2 AND after_facts->>'status_key'='blocked' AND after_facts->>'assignee_id'=$3`, iid, f.handler, f.creator) != 1 {
		t.Fatal("request lost its atomic iteration fact")
	}
	nested, err := f.svc.RequestInformation(product, handoffAsk(t, f, 2, f.outsider, "Please confirm the copy"))
	if err != nil || nested.Code != "information_requested" {
		t.Fatalf("nested request: %+v %v", nested, err)
	}
	r2 := activeInformationRequest(t, f)
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND parent_request_id=$2 AND requester_user_id=$3 AND previous_status='blocked' AND resolution='open'`, r2, r1, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='open'`, r1) != 1 {
		t.Fatal("nested request replaced instead of stacking on its parent")
	}
	// An older layer cannot be answered while a nested request is current.
	if _, err = f.svc.ReturnInformation(product, handoffReturn(t, f, 3, r1, "todo")); !collaborationErrorCode(err, "information_request_not_active") {
		t.Fatalf("parent returned before its child: %v", err)
	}
	view, err := f.svc.InformationRequestView(specialist, mustAccess(t, f, specialist))
	if err != nil || view == nil || view.ID != r2 || view.ReturnStatus != "blocked" || view.ReturnStatusFallback {
		t.Fatalf("nested view: %+v %v", view, err)
	}
	back, err := f.svc.ReturnInformation(specialist, handoffReturn(t, f, 3, r2, "blocked"))
	if err != nil || back.Code != "information_returned" {
		t.Fatalf("nested return: %+v %v", back, err)
	}
	if activeInformationRequest(t, f) != r1 || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='blocked'`, f.issue.Issue.ID, f.creator) != 1 {
		t.Fatal("nested return did not restore the parent request to its requester")
	}
	final := handoffReturn(t, f, 4, r1, "todo")
	done, err := f.svc.ReturnInformation(product, final)
	if err != nil || done.Code != "information_returned" || done.CollaborationRevision != 5 {
		t.Fatalf("final return: %+v %v", done, err)
	}
	if activeInformationRequest(t, f).Valid || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=ANY($1::uuid[]) AND resolution='returned'`, []pgtype.UUID{r1, r2}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='information_return' AND parent_request_id=ANY($2::uuid[])`, f.issue.Issue.ID, []pgtype.UUID{r1, r2}) != 2 {
		t.Fatal("chain did not unwind to the original handler and status")
	}
	replay, err := f.svc.ReturnInformation(product, final)
	if err != nil || !reflect.DeepEqual(replay, done) {
		t.Fatalf("return replay after responsibility moved: %+v %v", replay, err)
	}
	if _, err = f.svc.ReturnInformation(product, handoffReturn(t, f, 5, r1, "todo")); !collaborationErrorCode(err, "information_request_not_active") {
		t.Fatalf("consumed request returned twice: %v", err)
	}
	// R11/AC26: none of these handoffs touched the queue or the running work.
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND active_run_id=$2 AND control_version=1`, f.issue.SessionID, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND claim_generation=1`, run.ID) != 1 {
		t.Fatal("information handoffs changed queue or execution")
	}
	// Every other current participant hears of both returns; no actor notifies itself.
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='information_returned' AND recipient_user_id=$2`, f.issue.Issue.ID, f.handler) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification n JOIN collaboration_event e ON e.issue_id=n.issue_id AND e.event_seq=n.event_seq WHERE n.issue_id=$1 AND n.recipient_user_id=e.actor_user_id`, f.issue.Issue.ID) != 0 {
		t.Fatal("handoff notifications reached the wrong accounts")
	}
}

func mustAccess(t *testing.T, f *collaborationQueueFixture, ctx context.Context) CollaborationIssueAccess {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return access
}

// A departed or disabled requester is not re-invited; ordinary transfer by a
// current manager remains possible and supersedes the stranded request.
func TestCollaborationInformationReturnRefusesUnavailableRequester(t *testing.T) {
	for _, scenario := range []string{"removed", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			f.invite(t, 1, f.outsider)
			if _, err := f.svc.RequestInformation(collaborationContext(f.handler), handoffAsk(t, f, 2, f.outsider, "")); err != nil {
				t.Fatal(err)
			}
			request := activeInformationRequest(t, f)
			revision := int64(3)
			if scenario == "removed" {
				p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 3), TargetUserID: util.MustParseUUID(f.handler)}
				if r, err := f.svc.RemoveParticipant(f.ctx, p); err != nil || r.Code != "participant_removed" {
					t.Fatalf("remove requester: %+v %v", r, err)
				}
				revision = 4
			} else {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			}
			// Removing a nonhandler requester leaves the request with its recipient.
			if activeInformationRequest(t, f) != request {
				t.Fatal("requester departure silently changed the current request")
			}
			refused, err := f.svc.ReturnInformation(collaborationContext(f.outsider), handoffReturn(t, f, revision, request, "todo"))
			if !collaborationErrorCode(err, "return_target_unavailable") || refused.CollaborationRevision != revision {
				t.Fatalf("return to unavailable requester: %+v %v", refused, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='blocked'`, f.issue.Issue.ID, f.outsider) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL`, f.issue.Issue.ID, f.handler) != map[string]int{"removed": 0, "disabled": 1}[scenario] {
				t.Fatal("refused return changed responsibility or re-invited the requester")
			}
			transfer := handoffAsk(t, f, revision, f.creator, "Take this back")
			if r, err := f.svc.Transfer(collaborationContext(f.outsider), transfer.CollaborationTransferParams); err != nil || r.Code != "transferred" {
				t.Fatalf("ordinary transfer after refused return: %+v %v", r, err)
			}
			if activeInformationRequest(t, f).Valid || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, request) != 1 {
				t.Fatal("ordinary transfer did not end the stranded request")
			}
		})
	}
}

func TestCollaborationInformationReturnShowsReplacedStatus(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	// System keys cannot be archived; a custom source status can.
	f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "design_review", "name": "Design review", "category": "todo", "color": "#000000"})
	f.fx.Exec(t, `UPDATE issue SET status='design_review' WHERE id=$1`, f.issue.Issue.ID)
	if _, err := f.svc.RequestInformation(collaborationContext(f.handler), handoffAsk(t, f, 1, f.creator, "")); err != nil {
		t.Fatal(err)
	}
	request := activeInformationRequest(t, f)
	view, err := f.svc.InformationRequestView(f.ctx, mustAccess(t, f, f.ctx))
	if err != nil || view == nil || view.ReturnStatus != "design_review" || view.ReturnStatusFallback {
		t.Fatalf("saved custom status was not the restore target: %+v %v", view, err)
	}
	// The saved key leaves the selectable source catalog while the request is open.
	f.fx.Exec(t, `UPDATE issue_status SET archived_at=now() WHERE workspace_id=$1 AND key='design_review'`, f.workspace)
	view, err = f.svc.InformationRequestView(f.ctx, mustAccess(t, f, f.ctx))
	if err != nil || view == nil || view.ReturnStatus != "in_progress" || !view.ReturnStatusFallback || view.PreviousStatus != "design_review" {
		t.Fatalf("replacement was not shown: %+v %v", view, err)
	}
	stale := handoffReturn(t, f, 2, request, "design_review")
	refused, err := f.svc.ReturnInformation(f.ctx, stale)
	if !collaborationErrorCode(err, "status_options_changed") {
		t.Fatalf("silent fallback: %+v %v", refused, err)
	}
	if r, err := f.svc.ReturnInformation(f.ctx, handoffReturn(t, f, 2, request, "in_progress")); err != nil || r.Code != "information_returned" {
		t.Fatalf("confirmed fallback return: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='in_progress' AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("confirmed fallback was not applied")
	}
	// The refused key stays refused even though a later return succeeded.
	if replay, err := f.svc.ReturnInformation(f.ctx, stale); !collaborationErrorCode(err, "status_options_changed") || replay.Code != refused.Code || replay.CollaborationRevision != refused.CollaborationRevision {
		t.Fatalf("refusal replay: %+v %v", replay, err)
	}
}

// AC29/AC30: submission hands the open issue to the creator. It is not
// completion, keeps the former handler's collaboration rights and ends any
// information chain.
func TestCollaborationAcceptanceSubmissionReturnsOpenIssueToCreator(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	f.invite(t, 1, f.outsider)
	if _, err := f.svc.RequestInformation(collaborationContext(f.handler), handoffAsk(t, f, 2, f.outsider, "")); err != nil {
		t.Fatal(err)
	}
	request := activeInformationRequest(t, f)
	waiting := f.submit(t, f.creator, "still waiting")
	submit := handoffAcceptance(t, f, 3, "Ready")
	if _, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), submit); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("former handler submitted acceptance: %v", err)
	}
	submit.RequestID = uuid.NewString()
	r, err := f.svc.SubmitAcceptance(collaborationContext(f.outsider), submit)
	if err != nil || r.Code != "acceptance_submitted" || r.QueueState != "paused" || r.ControlVersion != 1 {
		t.Fatalf("acceptance: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='in_review'`, f.issue.Issue.ID, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, request) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' AND from_user_id=$2 AND to_user_id=$3 AND note='Ready'`, f.issue.Issue.ID, f.outsider, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 || activeInformationRequest(t, f).Valid {
		t.Fatal("acceptance did not return the open issue to the creator alone")
	}
	access := mustAccess(t, f, f.ctx)
	if !access.Capabilities.Complete || access.Capabilities.SubmitAcceptance || access.StatusCategory != "in_review" {
		t.Fatalf("creator cannot accept, or acceptance completed the issue: %+v", access)
	}
	former := mustAccess(t, f, collaborationContext(f.outsider))
	if !former.Capabilities.ControlAI || !former.Capabilities.Comment || former.Capabilities.Transfer || former.Capabilities.SubmitAcceptance {
		t.Fatalf("former handler rights: %+v", former.Capabilities)
	}
	// The creator now handles the issue but cannot submit to themselves.
	if _, err = f.svc.SubmitAcceptance(f.ctx, CollaborationAcceptanceParams{CollaborationWorkflowRequest: workflowRequest(f, 4)}); !collaborationErrorCode(err, "already_creator") {
		t.Fatalf("creator self submission: %v", err)
	}
}

func TestCollaborationAcceptanceRefusesDisabledCreator(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	if _, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), CollaborationAcceptanceParams{CollaborationWorkflowRequest: workflowRequest(f, 1)}); !collaborationErrorCode(err, "target_unavailable") {
		t.Fatalf("acceptance to disabled creator: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("refused acceptance changed the issue")
	}
}

func TestCollaborationInformationRequestRolesAndClosedIssue(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	f.invite(t, 1, f.outsider)
	// A participant without creator/handler identity cannot direct responsibility.
	if _, err := f.svc.RequestInformation(collaborationContext(f.outsider), handoffAsk(t, f, 2, f.creator, "")); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("plain participant requested information: %v", err)
	}
	for _, tc := range []struct{ actor, target string }{{f.handler, f.handler}, {f.creator, f.handler}} {
		if _, err := f.svc.RequestInformation(collaborationContext(tc.actor), handoffAsk(t, f, 2, tc.target, "")); !collaborationErrorCode(err, "invalid_information_target") {
			t.Fatalf("request to current handler: %v", err)
		}
	}
	if _, err := f.svc.RequestInformation(collaborationContext(f.handler), handoffAsk(t, f, 2, f.creator, "")); err != nil {
		t.Fatal(err)
	}
	request := activeInformationRequest(t, f)
	// The creator ends the issue; a request belonging to it cannot be answered.
	p := lifecycleParams(t, f, "cancel", "cancelled", 3, 1)
	if _, err := f.svc.ChangeLifecycle(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, request) != 1 || activeInformationRequest(t, f).Valid {
		t.Fatal("close kept the information chain")
	}
	closed := handoffReturn(t, f, 4, request, "todo")
	closed.ExpectedLifecycleEpoch = 2
	if _, err := f.svc.ReturnInformation(f.ctx, closed); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("return after close: %v", err)
	}
	ask := handoffAsk(t, f, 4, f.outsider, "")
	ask.ExpectedLifecycleEpoch = 2
	if _, err := f.svc.RequestInformation(f.ctx, ask); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("request after close: %v", err)
	}
}

func TestCollaborationInformationRequestRequiredWritesRollback(t *testing.T) {
	for _, needle := range []string{
		"UPDATE issue AS i SET", "UPDATE issue SET assignee_type", "INSERT INTO issue_handoff", "INSERT INTO issue_participant",
		"UPDATE issue_collaboration SET active_information_request_id = $1", "UPDATE issue_collaboration SET revision",
		"INSERT INTO collaboration_event", "INSERT INTO collaboration_notification", "UPDATE collaboration_operation",
	} {
		t.Run(needle, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			p := handoffAsk(t, f, 1, f.outsider, "")
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			if _, err := f.svc.RequestInformation(collaborationContext(f.handler), p); err == nil {
				t.Fatal("required write failure reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.outsider) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1 AND active_information_request_id IS NULL`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("required write failure committed a partial information request")
			}
			f.svc.TxStarter = f.pool
			if r, err := f.svc.RequestInformation(collaborationContext(f.handler), p); err != nil || r.Code != "information_requested" {
				t.Fatalf("original key after rollback: %+v %v", r, err)
			}
		})
	}
}
