package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationInformationRepairAuthorityCommitOrders(t *testing.T) {
	for _, first := range []string{"repair", "requester_removal"} {
		t.Run(first+"_first", func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationStageFixture(t)
			f.revise(t, true)
			id := f.issue.Issue.ID
			if _, err := f.svc.RequestInformation(f.handlerCtx, informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)); err != nil {
				t.Fatal(err)
			}
			repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", true)
			remove := CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, f.ctx), TargetUserID: util.MustParseUUID(f.handler)}
			repairCall := func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
				return svc.RepairInformation(ctx, repair)
			}
			removeCall := func(ctx context.Context, svc *CollaborationService) (CollaborationWorkflowReceipt, error) {
				return svc.RemoveParticipant(ctx, remove)
			}
			lead, follow := repairCall, removeCall
			if first == "requester_removal" {
				lead, follow = removeCall, repairCall
			}
			ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
			defer cancel()
			leading, following := recoveryCASCommitOrder(t, f.collaborationQueueFixture, ctx, lead, follow)
			if leading.err != nil || !collaborationErrorCode(following.err, "stale_revision") {
				t.Fatalf("real writer order did not retain winning effect/stale refusal: lead=%+v follow=%+v", leading, following)
			}
			if first == "repair" {
				if leading.receipt.Code != "information_repaired" || following.receipt.Code != "stale_revision" || mustAccess(t, f.collaborationQueueFixture, f.ctx).StatusCategory != "todo" ||
					f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, f.handler) != 1 {
					t.Fatal("stale removal changed the committed repair or current requester")
				}
			} else {
				if leading.receipt.Code != "participant_removed" || following.receipt.Code != "stale_revision" || mustAccess(t, f.collaborationQueueFixture, f.ctx).StatusCategory != "blocked" {
					t.Fatal("stale repair escaped current security removal")
				}
				fresh := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", true)
				if result, err := f.svc.RepairInformation(f.ctx, fresh); err != nil || result.Code != "information_repaired" ||
					f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, f.handler) != 0 {
					t.Fatalf("fresh authorized repair reenrolled old requester: %+v %v", result, err)
				}
			}
		})
	}
}

type informationLateClockStarter struct {
	TxStarter
	hits *int
}

type informationLateClockTx struct {
	pgx.Tx
	hits *int
}

type informationLateClockRow struct {
	pgx.Row
	after func() error
}

func (row informationLateClockRow) Scan(destinations ...any) error {
	if err := row.Row.Scan(destinations...); err != nil {
		return err
	}
	return row.after()
}

func (starter informationLateClockStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &informationLateClockTx{Tx: tx, hits: starter.hits}, nil
}

func (tx *informationLateClockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: CompleteCollaborationOperation") && len(args) > 0 && args[0] == "information_requested" {
		return informationLateClockRow{Row: row, after: func() error {
			*tx.hits++
			_, err := tx.Tx.Exec(ctx, `SELECT pg_sleep(0.6)`)
			return err
		}}
	}
	return row
}

func TestCollaborationInformationFinalPersistableClockRollsBackAndSealsRefusal(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	params := informationTestAsk(t, f.svc, id, f.handlerCtx, f.creator)
	estimate := informationTime(time.Now().Add(300 * time.Millisecond).Truncate(time.Microsecond))
	params.EstimatedResolutionAt = &estimate
	before := sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture)
	hits := 0
	f.svc.TxStarter = informationLateClockStarter{TxStarter: f.pool, hits: &hits}
	result, err := f.svc.RequestInformation(f.handlerCtx, params)
	f.svc.TxStarter = f.pool
	if hits != 1 || !collaborationErrorCode(err, "information_metadata_invalid") || result.Status != 400 || before != sourceRoleBusinessSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("actual DBclock after writer receipt retained expired metadata/effects: hits=%d result=%+v err=%v", hits, result, err)
	}
	if replay, err := f.svc.RequestInformation(f.handlerCtx, params); !collaborationErrorCode(err, "information_metadata_invalid") || replay.CollaborationRevision != result.CollaborationRevision ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, id) != 0 {
		t.Fatalf("old final-clock refusal recomputed: %+v %v", replay, err)
	}
}
