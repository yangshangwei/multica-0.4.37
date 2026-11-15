package service

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Ordinary child moves remain legal and invalidate their semantic baseline.
func CheckStageProjectChangeInTx(ctx context.Context, tx pgx.Tx, before, after db.Issue) error {
	if before.ProjectID == after.ProjectID || !before.ProjectID.Valid {
		return nil
	}
	q := db.New(tx)
	shared, err := q.IsCollaborationIssue(ctx, before.ID)
	if err != nil || !shared {
		return err
	}
	project, err := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: before.ProjectID, WorkspaceID: before.WorkspaceID})
	if err != nil {
		return err
	}
	return CheckStageProjectDeleteInTx(ctx, tx, project)
}
func CheckStageProjectDeleteInTx(ctx context.Context, tx pgx.Tx, project db.Project) error {
	q := db.New(tx)
	if _, err := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: project.ID, WorkspaceID: project.WorkspaceID}); err != nil {
		return err
	}
	if err := stageReady(ctx, q); err != nil {
		return err
	}
	policy, found, err := currentStagePolicy(ctx, q, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	// An absent policy is virtual revision 0, disabled.
	if found && policy.Contract != collaboration.StageContract {
		return collaborationError(503, "stage_schema_unavailable")
	}
	if policy.Enabled {
		return DeliveryCompletionError{Status: 409, Code: "stage_policy_exit_required"}
	}
	return nil
}
