package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestManualRerunUsesNewSubmissionProofAndPreservesOriginal(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	for _, table := range []string{"agent", "issue"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	agentID := fx.Agent(t, "Rerun agent", util.UUIDToString(runtimeID))
	issueID := fx.Issue(t, "New manual submission", testutil.Cols{"assignee_id": agentID, "assignee_type": "agent"})
	originalInstallation := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	newInstallation := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	parentID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": "failed", "submitted_installation_id": originalInstallation})
	parent := mustManagedUUID(t, parentID)
	issue, err := f.svc.Queries.GetIssue(t.Context(), mustManagedUUID(t, issueID))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	for _, tc := range []struct {
		name     string
		context  context.Context
		expected pgtype.UUID
	}{{"web", t.Context(), pgtype.UUID{}}, {"desktop_b", auth.WithSubmissionInstallation(t.Context(), newInstallation), newInstallation}} {
		t.Run(tc.name, func(t *testing.T) {
			child, err := svc.enqueueIssueTask(tc.context, issue, pgtype.UUID{}, true, "", f.user, parent, pgtype.Timestamptz{})
			if err != nil {
				t.Fatal(err)
			}
			if child.SubmittedInstallationID != tc.expected || child.ExecutionInstallationID.Valid {
				t.Fatalf("manual rerun fabricated origin/execution: %+v", child)
			}
			fx.Exec(t, "UPDATE agent_task_queue SET status='failed' WHERE id=$1", child.ID)
		})
	}
	original, err := f.svc.Queries.GetAgentTask(t.Context(), parent)
	if err != nil || original.SubmittedInstallationID != originalInstallation {
		t.Fatal("original submission snapshot changed", err)
	}
}

func TestManualQuickRetryDiffersFromAutomaticRetrySubmission(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	f.fx.Exec(t, "CREATE TABLE agent (LIKE public.agent INCLUDING ALL)")
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	agentID := fx.Agent(t, "Retry agent", util.UUIDToString(runtimeID))
	originalInstallation := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	newInstallation := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	parentID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "failed", "submitted_installation_id": originalInstallation})
	parent := mustManagedUUID(t, parentID)
	for _, provided := range []pgtype.UUID{{}, newInstallation} {
		child, err := f.svc.Queries.CreateManualQuickCreateRetryTask(t.Context(), db.CreateManualQuickCreateRetryTaskParams{ActorUserID: f.user, NewTaskID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, SourceTaskID: parent, SubmittedInstallationID: provided})
		if err != nil {
			t.Fatal(err)
		}
		if child.SubmittedInstallationID != provided {
			t.Fatal("manual retry inherited another submission's device")
		}
		fx.Exec(t, "UPDATE agent_task_queue SET status='failed' WHERE id=$1", child.ID)
	}
	automatic, err := f.svc.Queries.CreateRetryTask(t.Context(), db.CreateRetryTaskParams{ID: parent, NewTaskID: pgtype.UUID{Bytes: uuid.New(), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if automatic.SubmittedInstallationID != originalInstallation || automatic.ExecutionInstallationID.Valid {
		t.Fatal("automatic continuation lost source or fabricated execution identity")
	}
}
