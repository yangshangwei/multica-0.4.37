package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func iterationBoundaryMove(t *testing.T, fx *testutil.Fixture, target *string, issueIDs ...string) iteration.Draft {
	t.Helper()
	reason := "Move the selected issues"
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{}}
	for _, id := range issueIDs {
		row, err := db.New(fx.Pool).GetIssue(t.Context(), util.MustParseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		draft.Moves = append(draft.Moves, iteration.Move{IssueID: id, ExpectedIssueRevision: row.Revision, ExpectedSourceID: iterationString(row.CurrentIterationID), TargetID: target})
	}
	return draft
}

func TestIterationMovePreviewRejectsCollaborationForSourceMembers(t *testing.T) {
	for _, action := range []string{"join", "switch", "remove"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationFixture(t)
			first, next := sourceFieldIteration(t, f, "Current period"), sourceFieldIteration(t, f, "Next period")
			params := f.params(t)
			params.HandlerID = util.MustParseUUID(f.creator)
			if action != "join" {
				params.CurrentIterationID, params.ExpectedIterationRevision = &first, 1
			}
			shared, err := f.svc.Create(f.ctx, params)
			if err != nil {
				t.Fatal(err)
			}
			ordinary := f.fx.Issue(t, "Ordinary mixed companion")
			member := f.fx.User(t, "Source member", uuid.NewString()+"@test.invalid")
			f.fx.Member(t, f.workspace, member, "member")
			var target *string
			if action != "remove" {
				target = iterationString(next)
			}
			svc := &IterationService{TxStarter: f.pool, AuthorizeIssues: func(context.Context, pgx.Tx, []db.Issue) error { return nil }}
			draft := iterationBoundaryMove(t, f.fx, target, ordinary, util.UUIDToString(shared.Issue.ID))
			for _, actor := range []string{member, f.creator} {
				preview, err := svc.Preview(collaborationContext(actor), util.MustParseUUID(f.workspace), util.MustParseUUID(actor), draft, lifecycleAuth)
				var refusal *iteration.OperationError
				if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "unsupported_collaboration_action" || len(preview.Issues) != 0 {
					t.Fatalf("%s caller %s received a collaboration move preview: %+v %v", action, actor, preview, err)
				}
			}
		})
	}
}

func TestIterationMoveApplyRechecksCollaborationAndRejectsWholeMixedSet(t *testing.T) {
	for _, action := range []string{"join", "switch", "remove"} {
		t.Run(action, func(t *testing.T) {
			fx, svc := lifecycleFixture(t)
			first, next := lifecycleCreate(t, fx, svc), lifecycleCreate(t, fx, svc)
			ordinary := fx.Issue(t, "Ordinary companion")
			shared := fx.Issue(t, "Shared companion", testutil.Cols{"assignee_type": "member", "assignee_id": fx.UserID})
			if action != "join" {
				lifecycleMove(t, fx, svc, ordinary, &first)
				lifecycleMove(t, fx, svc, shared, &first)
			}
			var target *string
			if action != "remove" {
				target = &next
			}
			draft := iterationBoundaryMove(t, fx, target, ordinary, shared)
			ws, actor := lifecycleIDs(fx)
			preview, err := svc.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
			if err != nil || len(preview.InvalidItems) != 0 {
				t.Fatalf("pre-boundary preview: %+v %v", preview, err)
			}
			// Publish the fixture's collaboration identity after an ordinary
			// preview. Apply must inspect current locked rows, never rely on a
			// preview/client mode bit. No production conversion is introduced.
			fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
			fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": shared})
			fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": shared, "user_id": fx.UserID}, "issue_id=$1 AND user_id=$2", shared, fx.UserID)
			events := fx.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1`, fx.WorkspaceID)
			participations := fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE workspace_id=$1`, fx.WorkspaceID)
			key := uuid.NewString()
			_, err = svc.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: key, Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
			var refusal *iteration.OperationError
			if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "unsupported_collaboration_action" {
				t.Fatalf("%s bypassed current collaboration identity: %v", action, err)
			}
			for _, move := range draft.Moves {
				row, err := db.New(fx.Pool).GetIssue(t.Context(), util.MustParseUUID(move.IssueID))
				if err != nil {
					t.Fatal(err)
				}
				source, _ := iterationUUID(move.ExpectedSourceID)
				if row.Revision != move.ExpectedIssueRevision || row.CurrentIterationID != source || row.IterationRolloverCount != 0 {
					t.Fatalf("mixed refusal partially changed %s: %+v", move.IssueID, row)
				}
			}
			if fx.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1`, fx.WorkspaceID) != events || fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE workspace_id=$1`, fx.WorkspaceID) != participations ||
				fx.Count(t, `SELECT count(*) FROM iteration_operation WHERE request_id=$1`, key) != 0 || fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1`, shared) != 1 {
				t.Fatal("mixed refusal retained membership, history or operation side effects")
			}
		})
	}
}

func TestIterationPeriodClosureStillRollsOverCollaboration(t *testing.T) {
	f := newCollaborationFixture(t)
	first, next := sourceFieldIteration(t, f, "Current period"), sourceFieldIteration(t, f, "Next period")
	today := time.Now().UTC().Format(time.DateOnly)
	f.fx.Exec(t, `UPDATE iteration SET start_date=$2::date,end_date=$2::date+13 WHERE id=ANY($1::uuid[])`, []pgtype.UUID{first, next}, today)
	for _, table := range []string{"iteration_snapshot", "iteration_notification", "iteration_operation"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, f.workspace)
	}
	params := f.params(t)
	params.HandlerID, params.CurrentIterationID, params.ExpectedIterationRevision = util.MustParseUUID(f.creator), &first, 1
	shared, err := f.svc.Create(f.ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	svc := &IterationService{TxStarter: f.pool, AuthorizeIssues: func(context.Context, pgx.Tx, []db.Issue) error { return nil }}
	source, target, issueID := util.UUIDToString(first), util.UUIDToString(next), util.UUIDToString(shared.Issue.ID)
	start := lifecycleDraft(t, f.fx, source, "start")
	start.Start = &iteration.StartDraft{TargetID: source, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, f.fx, svc, start)
	close := closureDraft(t, f.fx, source, issueID, "end", &target)
	lifecycleApply(t, f.fx, svc, close)
	after, err := f.svc.ResolveIssueAccess(f.ctx, nil, shared.Issue.ID)
	if err != nil || after.Issue.CurrentIterationID != next || after.Issue.IterationRolloverCount != 1 || after.Revision != shared.Revision || after.QueueState != shared.QueueState || after.Issue.AssigneeID != shared.Issue.AssigneeID {
		t.Fatalf("period rollover changed collaboration semantics: %+v %v", after, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1`, first) != 1 || f.fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND iteration_id=$2 AND current_joined_at IS NOT NULL`, shared.Issue.ID, next) != 1 {
		t.Fatal("period rollover did not retain history and next membership")
	}
}
