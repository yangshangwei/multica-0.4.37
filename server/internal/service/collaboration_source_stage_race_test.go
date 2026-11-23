package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

func sourceStageFixture(t *testing.T) (*collaborationQueueFixture, pgtype.UUID) {
	t.Helper()
	f := newCollaborationQueueFixture(t)
	f.fx.Member(t, f.workspace, f.handler, "member")
	parent := util.MustParseUUID(f.fx.Issue(t, "Stage-only source parent"))
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2,stage=1 WHERE id=$1`, f.issue.Issue.ID, parent)
	f.issue.Issue.ParentIssueID, f.issue.Issue.Stage = parent, pgtype.Int4{Int32: 1, Valid: true}
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	input := f.submit(t, f.creator, "Frozen input across source stage changes")
	f.run(t, input, 1)
	return f, parent
}

func assertSourceStage(t *testing.T, f *collaborationQueueFixture, parent pgtype.UUID, stage int32, edits, transfers int64) {
	t.Helper()
	var currentParent, handler pgtype.UUID
	var currentStage pgtype.Int4
	var ordinaryRevision, collaborationRevision int64
	var title string
	f.fx.QueryRow(t, `SELECT i.parent_issue_id,i.stage,i.revision,c.revision,i.assignee_id,i.title
		FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1`, f.issue.Issue.ID).
		Scan(&currentParent, &currentStage, &ordinaryRevision, &collaborationRevision, &handler, &title)
	if currentParent != parent || currentStage != (pgtype.Int4{Int32: stage, Valid: true}) || title != f.issue.Issue.Title ||
		ordinaryRevision != f.issue.Issue.Revision+edits+transfers || collaborationRevision != 1+edits+transfers || !handler.Valid {
		t.Fatalf("stage-only edit changed the wrong source fields or revisions: parent=%v stage=%v ordinary=%d collaboration=%d", currentParent, currentStage, ordinaryRevision, collaborationRevision)
	}
	if int64(f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID)) != edits {
		t.Fatal("stage-only edit history was missing or duplicated")
	}
}

func TestCollaborationSourceManagerStageOnlyEditRecordsOnce(t *testing.T) {
	for _, actorRole := range []string{"creator", "current_handler"} {
		t.Run(actorRole, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f, parent := sourceStageFixture(t)
			actor := f.creator
			if actorRole == "current_handler" {
				actor = f.handler
			}
			ctx := collaborationContext(actor)
			beforeExecution := sourceRoleExecutionSnapshot(t, f)
			request := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Stage: &pgtype.Int4{Int32: 2, Valid: true}}
			receipt, err := f.svc.EditIssue(ctx, request)
			if err != nil || receipt.Code != "issue_edited" || receipt.CollaborationRevision != 2 {
				t.Fatalf("current source manager could not change only stage: %+v %v", receipt, err)
			}
			assertSourceStage(t, f, parent, 2, 1, 0)
			var payload []byte
			var eventActor pgtype.UUID
			f.fx.QueryRow(t, `SELECT payload,actor_user_id FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID).Scan(&payload, &eventActor)
			var history struct {
				Before struct {
					Stage int32 `json:"stage"`
				} `json:"before"`
				After struct {
					Stage int32 `json:"stage"`
				} `json:"after"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(payload, &history); err != nil || history.Before.Stage != 1 || history.After.Stage != 2 ||
				history.RequestID != request.RequestID || eventActor != util.MustParseUUID(actor) {
				t.Fatalf("stage history lost actor, original request or before/after values: %+v %v", history, err)
			}
			beforeReplay := sourceRoleBusinessSnapshot(t, f)
			replay, err := f.svc.EditIssue(ctx, request)
			if err != nil || !reflect.DeepEqual(replay, receipt) || sourceRoleBusinessSnapshot(t, f) != beforeReplay ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution {
				t.Fatalf("stage replay changed business/history or existing execution: %+v %v", replay, err)
			}
		})
	}
}

func TestCollaborationSourceStageTransferBothCommitOrders(t *testing.T) {
	for _, first := range []string{"stage", "transfer"} {
		t.Run(first+"_first", func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f, parent := sourceStageFixture(t)
			beforeExecution := sourceRoleExecutionSnapshot(t, f)
			ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
			defer cancel()
			request := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Stage: &pgtype.Int4{Int32: 2, Valid: true}}
			change := func(starter TxStarter) error {
				svc := *f.svc
				svc.TxStarter = starter
				_, err := svc.EditIssue(ctx, request)
				return err
			}
			transferRequest := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
				CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.creator)}}
			transfer := func(starter TxStarter) error {
				svc := *f.svc
				svc.TxStarter = starter
				_, err := svc.Transfer(ctx, transferRequest)
				return err
			}
			leading, following, refusal := change, transfer, "stale_revision"
			if first == "transfer" {
				leading, following, refusal = transfer, change, "forbidden"
			}
			lead, follow := sourceAuthorityCommitOrder(t, f, ctx, leading, following)
			if lead != nil || !collaborationErrorCode(follow, refusal) {
				t.Fatalf("stage/transfer commit ordering accepted stale authority: %v %v", lead, follow)
			}
			stage, edits := int32(1), int64(0)
			if first == "stage" {
				stage, edits = 2, 1
				transferRequest.CollaborationWorkflowRequest = workflowRequest(f, 2)
				if _, err := f.svc.Transfer(ctx, transferRequest); err != nil {
					t.Fatalf("fresh transfer after committed stage: %v", err)
				}
			}
			assertSourceStage(t, f, parent, stage, edits, 1)
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, f.issue.Issue.ID, f.creator) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='transfer'`, f.issue.Issue.ID) != 1 {
				t.Fatal("stage/transfer race lost final handler or duplicated transfer history")
			}
			request.CollaborationWorkflowRequest = workflowRequest(f, 2+edits)
			request.Stage = &pgtype.Int4{Int32: 3, Valid: true}
			beforeRefusal := sourceRoleBusinessSnapshot(t, f)
			if err := change(f.pool); !collaborationErrorCode(err, "forbidden") || sourceRoleBusinessSnapshot(t, f) != beforeRefusal ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution {
				t.Fatalf("former source handler regained stage authority or changed execution: %v", err)
			}
		})
	}
}

func TestCollaborationSourceStageDepartureBothCommitOrders(t *testing.T) {
	for _, first := range []string{"stage", "departure"} {
		t.Run(first+"_first", func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f, parent := sourceStageFixture(t)
			beforeExecution := sourceRoleExecutionSnapshot(t, f)
			ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
			defer cancel()
			request := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Stage: &pgtype.Int4{Int32: 2, Valid: true}}
			change := func(starter TxStarter) error {
				svc := *f.svc
				svc.TxStarter = starter
				_, err := svc.EditIssue(ctx, request)
				return err
			}
			departure := func(starter TxStarter) error {
				tx, err := starter.Begin(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(ctx, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler); err != nil {
					return err
				}
				return tx.Commit(ctx)
			}
			leading, following := change, departure
			if first == "departure" {
				leading, following = departure, change
			}
			lead, follow := sourceAuthorityCommitOrder(t, f, ctx, leading, following)
			if lead != nil || first == "stage" && follow != nil || first == "departure" && !collaborationErrorCode(follow, "forbidden") {
				t.Fatalf("stage/departure did not hold or recheck current membership: %v %v", lead, follow)
			}
			stage, edits := int32(1), int64(0)
			if first == "stage" {
				stage, edits = 2, 1
			}
			assertSourceStage(t, f, parent, stage, edits, 0)
			if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
				t.Fatal("source departure did not commit")
			}
			request.CollaborationWorkflowRequest = workflowRequest(f, 1+edits)
			request.RequestID = uuid.NewString()
			request.Stage = &pgtype.Int4{Int32: 3, Valid: true}
			beforeRefusal := sourceRoleBusinessSnapshot(t, f)
			if err := change(f.pool); !collaborationErrorCode(err, "forbidden") || sourceRoleBusinessSnapshot(t, f) != beforeRefusal ||
				sourceRoleExecutionSnapshot(t, f) != beforeExecution {
				t.Fatalf("departed source handler regained stage writes or changed execution: %v", err)
			}
		})
	}
}
