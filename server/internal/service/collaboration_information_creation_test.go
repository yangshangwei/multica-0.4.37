package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationInformationCreationEffectiveProjectAndOriginalKeys(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	ws, project := util.MustParseUUID(f.workspace), util.MustParseUUID(f.project)
	options, err := f.svc.CreationOptions(f.ctx, ws)
	if err != nil || options.StagePolicyObservation == nil || options.StagePolicyObservation.State != "not_selected" || options.StagePolicyObservation.Managed != nil || options.StagePolicyObservation.ProjectID.Valid {
		t.Fatalf("unselected project became known/unmanaged: %+v %v", options, err)
	}
	off, err := f.svc.CreationOptionsForProject(f.ctx, ws, project, "")
	if err != nil || off.StagePolicyObservation == nil || off.StagePolicyObservation.PolicyRevision == nil || *off.StagePolicyObservation.PolicyRevision != "0" ||
		off.StagePolicyObservation.Managed == nil || *off.StagePolicyObservation.Managed {
		t.Fatalf("virtual policy revision0/off unavailable: %+v %v", off, err)
	}
	blocked := "blocked"
	legacy := f.params(t)
	legacy.ProjectID, legacy.Status = project, &blocked
	created, err := f.svc.Create(f.ctx, legacy)
	if err != nil || created.StatusCategory != "blocked" {
		t.Fatalf("unmanaged blocked creation changed: %+v %v", created, err)
	}
	f.revise(t, true)
	on, err := f.svc.CreationOptionsForProject(f.ctx, ws, project, "")
	if err != nil || on.StagePolicyObservation == nil || on.StagePolicyObservation.Managed == nil || !*on.StagePolicyObservation.Managed {
		t.Fatalf("managed selected-project observation absent: %+v %v", on, err)
	}
	for _, status := range on.Fields.Statuses {
		if status.Category == "blocked" {
			t.Fatal("managed options retained a blocked category")
		}
	}
	if replay, err := f.svc.Create(f.ctx, legacy); err != nil || replay.Issue.ID != created.Issue.ID {
		t.Fatalf("old successful creation re-ran new admission: %+v %v", replay, err)
	}
	// The ordinary parent uses the real writer so its number and workspace
	// counter remain consistent with the later collaboration creation.
	creator := util.MustParseUUID(f.creator)
	parent, err := f.svc.Issues.Create(f.ctx, IssueCreateParams{WorkspaceID: ws, ProjectID: project,
		Title: "Managed project ordinary parent", Status: "todo", Priority: "none", CreatorType: "member", CreatorID: creator},
		IssueCreateOpts{ActorUserID: creator})
	if err != nil || parent.Issue.WorkspaceID != ws || parent.Issue.ProjectID != project || parent.Issue.Number != created.Issue.Number+1 {
		t.Fatalf("ordinary parent did not use the source workspace's next real number: %+v %v", parent, err)
	}
	workspace, err := f.svc.Queries.GetWorkspace(f.ctx, ws)
	if err != nil || workspace.IssueCounter != parent.Issue.Number {
		t.Fatalf("ordinary parent number and source counter differ: %+v %v", workspace, err)
	}
	probe := f.params(t)
	probe.Status, probe.ParentIssueID, probe.ProjectID = &blocked, parent.Issue.ID, pgtype.UUID{}
	before := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, ws)
	if _, err := f.svc.Create(f.ctx, probe); !collaborationErrorCode(err, "status_unavailable") || before != f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, ws) {
		t.Fatalf("raw omitted project bypassed inherited managed blocked guard: %v", err)
	}
	result, err := f.svc.GetCreationResult(f.ctx, ws, probe.RequestID)
	if err != nil || result.Code != "status_unavailable" || result.Outcome != "rejected" {
		t.Fatalf("managed selection refusal not sealed under original key: %+v %v", result, err)
	}
	f.revise(t, false)
	if _, err := f.svc.Create(f.ctx, probe); !collaborationErrorCode(err, "status_unavailable") || before != f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, ws) {
		t.Fatalf("old blocked refusal became a deferred create after policy off: %v", err)
	}
	probe.RequestID = uuid.NewString()
	fresh, err := f.svc.Create(f.ctx, probe)
	if err != nil || fresh.Issue.ParentIssueID != parent.Issue.ID || fresh.Issue.ProjectID != project || fresh.StatusCategory != "blocked" ||
		fresh.Issue.Number != parent.Issue.Number+1 || before+1 != f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, ws) {
		t.Fatalf("fresh unmanaged intent did not independently inherit its real parent/project/number: %+v %v", fresh, err)
	}
	workspace, err = f.svc.Queries.GetWorkspace(f.ctx, ws)
	if err != nil || workspace.IssueCounter != fresh.Issue.Number {
		t.Fatalf("fresh creation did not advance the source counter once: %+v %v", workspace, err)
	}
	t.Logf("information_creation_parent_number=%d fresh_number=%d workspace_counter=%d", parent.Issue.Number, fresh.Issue.Number, workspace.IssueCounter)
}
