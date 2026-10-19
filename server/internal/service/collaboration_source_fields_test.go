package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationParentSearchCharacterLimit(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		valid       bool
	}{
		{"ascii_at_limit", strings.Repeat("a", 200), true},
		{"chinese_at_limit", strings.Repeat("研", 200), true},
		{"chinese_over_limit", strings.Repeat("研", 201), false},
		{"ascii_over_limit", strings.Repeat("a", 201), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collaborationParentSearch([]string{tc.query})
			if tc.valid && (err != nil || got != tc.query) {
				t.Fatalf("valid character-bounded search refused: %v", err)
			}
			if !tc.valid && !collaborationErrorCode(err, "invalid_request") {
				t.Fatalf("overlong search accepted: %v", err)
			}
		})
	}
}

func sourceFieldIteration(t *testing.T, f *collaborationFixture, name string) pgtype.UUID {
	t.Helper()
	id := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": name, "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, id)
	f.fx.Cleanup(t, `DELETE FROM iteration_participation WHERE iteration_id=$1`, id)
	return util.MustParseUUID(id)
}

func TestCollaborationSourceStructureAuthorityAndAtomicEdit(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	project := util.MustParseUUID(f.fx.Project(t, "Source project"))
	parent := util.MustParseUUID(f.fx.Issue(t, "Source parent"))
	stage := pgtype.Int4{Int32: 2, Valid: true}
	p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &project, ParentIssueID: &parent, Stage: &stage}
	refusal, err := f.svc.EditIssue(collaborationContext(f.handler), p)
	if !collaborationErrorCode(err, "forbidden") || refusal.Code != "forbidden" {
		t.Fatalf("external handler changed source structure: %+v %v", refusal, err)
	}
	p.RequestID = uuid.NewString()
	receipt, err := f.svc.EditIssue(f.ctx, p)
	if err != nil || receipt.Code != "issue_edited" || receipt.CollaborationRevision != 2 {
		t.Fatalf("source editor rejected: %+v %v", receipt, err)
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil || access.Issue.ProjectID != project || access.Issue.ParentIssueID != parent || access.Issue.Stage != stage || access.Issue.Revision != f.issue.Issue.Revision+1 {
		t.Fatalf("source edit was not a single atomic revision: %+v %v", access.Issue, err)
	}
	if access.Issue.AssigneeID != f.issue.Issue.AssigneeID || access.QueueState != f.issue.QueueState {
		t.Fatal("source edit changed handling or AI control")
	}
	if _, err = f.svc.EditIssue(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 1 {
		t.Fatal("replay duplicated field history")
	}
	options, err := f.svc.FieldOptions(f.ctx, f.issue.Issue.ID)
	if err != nil || options.SourceOptions == nil || len(options.SourceOptions.Parents) == 0 {
		t.Fatalf("source choices unavailable: %+v %v", options, err)
	}
	external, err := f.svc.FieldOptions(collaborationContext(f.handler), f.issue.Issue.ID)
	if err != nil || external.SourceOptions != nil {
		t.Fatalf("external handler received source catalogs: %+v %v", external, err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	p.CollaborationWorkflowRequest = workflowRequest(f, 2)
	p.Stage = &pgtype.Int4{Int32: 3, Valid: true}
	refusal, err = f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "forbidden") || refusal.Code != "forbidden" {
		t.Fatalf("departed source editor retained structure writes: %+v %v", refusal, err)
	}
}

func TestCollaborationSourceStructureRejectsCyclesAndMixedFailure(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	child := util.MustParseUUID(f.fx.Issue(t, "Child", testutil.Cols{"parent_issue_id": f.issue.Issue.ID}))
	p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ParentIssueID: &child, Title: editString("Must not apply")}
	r, err := f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "parent_cycle") || r.Code != "parent_cycle" {
		t.Fatalf("cycle not refused: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND parent_issue_id IS NULL`, f.issue.Issue.ID, f.issue.Issue.Title) != 1 {
		t.Fatal("cycle refusal partially applied content")
	}
	p.ParentIssueID, p.RequestID = &pgtype.UUID{}, uuid.NewString()
	r, err = f.svc.EditIssue(f.ctx, p)
	if err != nil || r.Code != "issue_edited" {
		t.Fatalf("explicit clear refused: %+v %v", r, err)
	}
}

func TestCollaborationIterationAssignmentVersionsReasonsAndReplay(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	first, second := sourceFieldIteration(t, f.collaborationFixture, "First"), sourceFieldIteration(t, f.collaborationFixture, "Second")
	p := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 1), CurrentIterationID: first, ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1}
	r, err := f.svc.ChangeIteration(collaborationContext(f.handler), p)
	if !collaborationErrorCode(err, "forbidden") || r.Code != "forbidden" {
		t.Fatalf("external assignment: %+v %v", r, err)
	}
	p.RequestID = uuid.NewString()
	r, err = f.svc.ChangeIteration(f.ctx, p)
	if err != nil || r.Code != "iteration_changed" {
		t.Fatalf("source join: %+v %v", r, err)
	}
	joined, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil || joined.Issue.CurrentIterationID != first || joined.Revision != 2 {
		t.Fatalf("join not atomic: %+v %v", joined, err)
	}
	move := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 2), CurrentIterationID: second, ExpectedIssueRevision: joined.Issue.Revision, ExpectedIterationRevision: 1}
	r, err = f.svc.ChangeIteration(f.ctx, move)
	if !collaborationErrorCode(err, "iteration_validation_failed") || r.Code != "iteration_validation_failed" {
		t.Fatalf("missing move reason: %+v %v", r, err)
	}
	refusedID := move.RequestID
	move.RequestID, move.Reason = uuid.NewString(), "Deliver in the next period"
	r, err = f.svc.ChangeIteration(f.ctx, move)
	if err != nil || r.Code != "iteration_changed" {
		t.Fatalf("source move: %+v %v", r, err)
	}
	moved, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil || moved.Issue.CurrentIterationID != second || moved.Revision != 3 || moved.QueueState != joined.QueueState {
		t.Fatalf("move result: %+v %v", moved, err)
	}
	move.RequestID, move.Reason = refusedID, ""
	r, err = f.svc.ChangeIteration(f.ctx, move)
	if !collaborationErrorCode(err, "iteration_validation_failed") || r.Code != "iteration_validation_failed" {
		t.Fatalf("old refusal changed after later move: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND current_joined_at IS NOT NULL`, f.issue.Issue.ID) != 1 {
		t.Fatal("move retained two current participations")
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE issue_id=$1 AND reason=$2`, f.issue.Issue.ID, "Deliver in the next period") != 2 {
		t.Fatal("move lost source/target reason facts")
	}
	remove := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 3), ExpectedIssueRevision: moved.Issue.Revision, Reason: "No longer planned"}
	r, err = f.svc.ChangeIteration(f.ctx, remove)
	if err != nil || r.Code != "iteration_changed" {
		t.Fatalf("source remove: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND current_joined_at IS NOT NULL`, f.issue.Issue.ID) != 0 {
		t.Fatal("remove did not close current participation")
	}
}

func TestCollaborationCreationInitialFieldsAreOneRecoverableIssue(t *testing.T) {
	f := newCollaborationFixture(t)
	period := sourceFieldIteration(t, f, "Initial period")
	label := util.MustParseUUID(f.fx.Insert(t, "issue_label", testutil.Cols{"workspace_id": f.workspace, "name": "Initial label", "resource_type": "issue", "color": "#123456"}))
	property := f.fx.Insert(t, "issue_property", testutil.Cols{"workspace_id": f.workspace, "name": "Initial checkbox", "type": "checkbox"})
	project := util.MustParseUUID(f.fx.Project(t, "Initial project"))
	parent := util.MustParseUUID(f.fx.Issue(t, "Initial parent", testutil.Cols{"project_id": project}))
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, f.workspace)
	p := f.params(t)
	p.Status, p.Priority, p.StartDate, p.DueDate = editString("in_progress"), editString("high"), editString("2026-10-19"), editString("2026-10-30")
	p.ProjectID, p.ParentIssueID, p.Stage, p.LabelIDs = project, parent, &pgtype.Int4{Int32: 2, Valid: true}, []pgtype.UUID{label}
	p.Properties = map[string]json.RawMessage{property: json.RawMessage(`false`)}
	p.CurrentIterationID, p.ExpectedIterationRevision = &period, 1
	created, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_to_label WHERE issue_id=$1`, created.Issue.ID)
	if created.Issue.Status != "in_progress" || created.Issue.Priority != "high" || created.Issue.StartDate.Time.Format("2006-01-02") != "2026-10-19" || created.Issue.DueDate.Time.Format("2006-01-02") != "2026-10-30" ||
		created.Issue.ProjectID != project || created.Issue.ParentIssueID != parent || created.Issue.Stage.Int32 != 2 || created.Issue.CurrentIterationID != period || created.QueueState != "paused" || created.Revision != 1 {
		t.Fatalf("initial values not part of creation: %+v", created)
	}
	var stored map[string]any
	if err = json.Unmarshal(created.Issue.Properties, &stored); err != nil || stored[property] != false {
		t.Fatalf("initial property lost: %s %v", created.Issue.Properties, err)
	}
	replayed, err := f.svc.Create(f.ctx, p)
	if err != nil || replayed.Issue.ID != created.Issue.ID {
		t.Fatalf("creation replay: %+v %v", replayed, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE issue_id=$1`, created.Issue.ID) != 1 || f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1`, created.Issue.ID) != 1 {
		t.Fatal("creation replay duplicated initial facts")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, created.SessionID) != 0 {
		t.Fatal("initial fields triggered AI")
	}
}

func TestCollaborationIterationCompletedJoinRequiresExplicitConfirmation(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	period := sourceFieldIteration(t, f.collaborationFixture, "Active delivery")
	f.fx.Exec(t, `UPDATE iteration SET status='active',started_at=clock_timestamp(),started_by=$2 WHERE id=$1`, period, f.creator)
	f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
	f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 1), CurrentIterationID: period, ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1}
	refused, err := f.svc.ChangeIteration(f.ctx, p)
	if !collaborationErrorCode(err, "iteration_validation_failed") || refused.Code != "iteration_validation_failed" {
		t.Fatalf("completed join skipped confirmation: %+v %v", refused, err)
	}
	p.RequestID, p.AllowCompleted = uuid.NewString(), true
	r, err := f.svc.ChangeIteration(f.ctx, p)
	if err != nil || r.Code != "iteration_changed" || r.QueueState != "closed" {
		t.Fatalf("confirmed completed join changed lifecycle: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done' AND current_iteration_id=$2`, f.issue.Issue.ID, period) != 1 {
		t.Fatal("completed join changed the business state")
	}
}

func TestCollaborationCreationInitialIterationRefusalNeverPartiallyCreates(t *testing.T) {
	f := newCollaborationFixture(t)
	period := sourceFieldIteration(t, f, "Selected period")
	p := f.params(t)
	p.CurrentIterationID, p.ExpectedIterationRevision = &period, 2
	if _, err := f.svc.Create(f.ctx, p); !collaborationErrorCode(err, "iteration_revision_conflict") {
		t.Fatalf("stale initial iteration: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 || f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, period) != 0 {
		t.Fatal("stale initial membership partially created a collaboration")
	}
	f.fx.Exec(t, `UPDATE iteration SET revision=2 WHERE id=$1`, period)
	if _, err := f.svc.Create(f.ctx, p); !collaborationErrorCode(err, "iteration_revision_conflict") {
		t.Fatalf("old refused creation became executable: %v", err)
	}
	p.RequestID = uuid.NewString()
	created, err := f.svc.Create(f.ctx, p)
	if err != nil || created.Issue.CurrentIterationID != period {
		t.Fatalf("new explicit creation did not use the current period: %+v %v", created, err)
	}
}
