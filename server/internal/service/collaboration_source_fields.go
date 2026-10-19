package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/admission"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationSourceReference struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
}

type CollaborationIterationReference struct {
	CollaborationSourceReference
	Revision int64  `json:"revision,string"`
	Status   string `json:"status"`
}

type CollaborationSourceOptions struct {
	Projects          []CollaborationSourceReference    `json:"projects"`
	Parents           []CollaborationSourceReference    `json:"parents"`
	ParentsTruncated  bool                              `json:"parents_truncated"`
	Iterations        []CollaborationIterationReference `json:"iterations"`
	IterationsEnabled bool                              `json:"iterations_enabled"`
}

type CollaborationSourceStructure struct {
	IssueRevision int64                            `json:"issue_revision,string"`
	Project       *CollaborationSourceReference    `json:"project"`
	ParentIssue   *CollaborationSourceReference    `json:"parent_issue"`
	Iteration     *CollaborationIterationReference `json:"iteration"`
	Stage         *int32                           `json:"stage"`
}

// Call only from a snapshot that has proved current source editing authority.
func collaborationSourceOptions(ctx context.Context, q *db.Queries, workspace, excluded pgtype.UUID, search string) (*CollaborationSourceOptions, error) {
	out := &CollaborationSourceOptions{Projects: []CollaborationSourceReference{}, Parents: []CollaborationSourceReference{}, Iterations: []CollaborationIterationReference{}, IterationsEnabled: true}
	projects, err := q.ListProjects(ctx, db.ListProjectsParams{WorkspaceID: workspace})
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		out.Projects = append(out.Projects, CollaborationSourceReference{ID: util.UUIDToString(project.ID), Name: project.Title})
	}
	ws, err := q.GetWorkspace(ctx, workspace)
	if err != nil {
		return nil, err
	}
	parents, err := q.ListCollaborationSourceParents(ctx, db.ListCollaborationSourceParentsParams{WorkspaceID: workspace, ExcludedIssueID: excluded, Search: search})
	if err != nil {
		return nil, err
	}
	if len(parents) > 200 {
		out.ParentsTruncated = true
		parents = parents[:200]
	}
	for _, parent := range parents {
		out.Parents = append(out.Parents, CollaborationSourceReference{ID: util.UUIDToString(parent.ID), Name: parent.Title,
			Identifier: fmt.Sprintf("%s-%d", util.ResolveIssuePrefix(ws.IssuePrefix, ws.Name), parent.Number)})
	}
	settings, err := q.GetIterationSettings(ctx, workspace)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		out.IterationsEnabled = settings.Enabled
	}
	periods, err := q.ListOpenIterations(ctx, workspace)
	if err != nil {
		return nil, err
	}
	for _, period := range periods {
		out.Iterations = append(out.Iterations, collaborationIterationReference(period))
	}
	return out, nil
}

func collaborationIterationReference(period db.Iteration) CollaborationIterationReference {
	return CollaborationIterationReference{CollaborationSourceReference: CollaborationSourceReference{ID: util.UUIDToString(period.ID), Name: period.Name}, Revision: period.Revision, Status: period.Status}
}

// References disclose no destination content or openable route. Source members
// use the independent authorized catalog when changing these values.
func collaborationSourceStructure(ctx context.Context, q *db.Queries, issue db.Issue) (*CollaborationSourceStructure, error) {
	out := &CollaborationSourceStructure{IssueRevision: issue.Revision}
	if issue.Stage.Valid {
		stage := issue.Stage.Int32
		out.Stage = &stage
	}
	if issue.ProjectID.Valid {
		project, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		out.Project = &CollaborationSourceReference{ID: util.UUIDToString(issue.ProjectID), Name: project.Title}
	}
	if issue.ParentIssueID.Valid {
		parent, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issue.ParentIssueID, WorkspaceID: issue.WorkspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		out.ParentIssue = &CollaborationSourceReference{ID: util.UUIDToString(issue.ParentIssueID), Name: parent.Title}
		if parent.ID.Valid {
			ws, err := q.GetWorkspace(ctx, issue.WorkspaceID)
			if err != nil {
				return nil, err
			}
			out.ParentIssue.Identifier = fmt.Sprintf("%s-%d", util.ResolveIssuePrefix(ws.IssuePrefix, ws.Name), parent.Number)
		}
	}
	if issue.CurrentIterationID.Valid {
		period, err := q.GetIteration(ctx, db.GetIterationParams{WorkspaceID: issue.WorkspaceID, ID: issue.CurrentIterationID})
		if err != nil {
			return nil, err
		}
		reference := collaborationIterationReference(period)
		out.Iteration = &reference
	}
	return out, nil
}

func collaborationParentSearch(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 || utf8.RuneCountInString(values[0]) > 200 {
		return "", collaborationError(400, "invalid_request")
	}
	return strings.TrimSpace(values[0]), nil
}

// The I1 workspace fence serializes parent changes. Lock every ancestor against
// deletion and reject cycles rather than accepting an incomplete depth walk.
func validateCollaborationParent(ctx context.Context, q *db.Queries, workspace, issueID, parentID pgtype.UUID) (string, error) {
	seen := map[pgtype.UUID]bool{issueID: true}
	for parentID.Valid {
		if seen[parentID] {
			return "parent_cycle", nil
		}
		seen[parentID] = true
		parent, err := q.LockCollaborationSourceParent(ctx, db.LockCollaborationSourceParentParams{WorkspaceID: workspace, IssueID: parentID})
		if errors.Is(err, pgx.ErrNoRows) {
			return "parent_issue_not_found", nil
		}
		if err != nil {
			return "", err
		}
		if !admission.Formal(parent.AdmissionStatus) {
			return "triage_review_required", nil
		}
		parentID = parent.ParentIssueID
	}
	return "", nil
}

func (t *collaborationWorkflowTx) prepareSourceStructure(ctx context.Context, p CollaborationEditParams, after *db.Issue) (string, error) {
	if p.ProjectID != nil {
		after.ProjectID = *p.ProjectID
	}
	if p.ParentIssueID != nil {
		after.ParentIssueID = *p.ParentIssueID
	}
	if p.Stage != nil {
		if p.Stage.Valid && p.Stage.Int32 < 1 {
			return "invalid_stage", nil
		}
		after.Stage = *p.Stage
	}
	if after.ProjectID.Valid && after.ProjectID != t.access.Issue.ProjectID {
		_, err := t.q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: after.ProjectID, WorkspaceID: after.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return "project_not_found", nil
		}
		if err != nil {
			return "", err
		}
	}
	if after.ParentIssueID.Valid && after.ParentIssueID != t.access.Issue.ParentIssueID {
		return validateCollaborationParent(ctx, t.q, after.WorkspaceID, after.ID, after.ParentIssueID)
	}
	return "", nil
}
