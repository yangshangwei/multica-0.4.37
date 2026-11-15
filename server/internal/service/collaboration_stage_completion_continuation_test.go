package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageCompletionMissingLedgerStarter struct {
	TxStarter
	hits *int
}

type stageCompletionMissingLedgerTx struct {
	pgx.Tx
	hits *int
}

type stageCompletionMissingLedgerRow struct{}

func (stageCompletionMissingLedgerRow) Scan(...any) error { return pgx.ErrNoRows }

func (starter stageCompletionMissingLedgerStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	transaction, err := starter.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageCompletionMissingLedgerTx{Tx: transaction, hits: starter.hits}, nil
}

func (transaction *stageCompletionMissingLedgerTx) QueryRow(ctx context.Context, statement string, args ...any) pgx.Row {
	if strings.Contains(statement, "864_collaboration_stage_submission_ready") && strings.Contains(statement, "schema_migrations") {
		*transaction.hits++
		return stageCompletionMissingLedgerRow{}
	}
	return transaction.Tx.QueryRow(ctx, statement, args...)
}

func TestStageCompletionGuardRequiresPreparedTransaction(t *testing.T) {
	if err := FinishStageMutationInTx(context.Background(), nil, DeliveryPrepareResult{}, db.Issue{}, DeliveryWriterResult{}); err == nil {
		t.Fatal("a missing transaction and guard passed the final writer check")
	}
}

func TestStageCompletionGuardCannotBeOmittedForManagedRoot(t *testing.T) {
	fixture := newCollaborationStageFixture(t)
	fixture.revise(t, true)
	transaction, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(fixture.ctx)
	issue, err := db.New(transaction).GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = FinishStageMutationInTx(fixture.ctx, transaction, DeliveryPrepareResult{State: DeliveryNotApplicable}, issue, DeliveryWriterResult{}); err == nil {
		t.Fatal("an enabled root passed the final writer check without preparation")
	}
}

func TestStageCompletionPolicyActivationRequiresExact864Ledger(t *testing.T) {
	fixture := newCollaborationStageFixture(t)
	hits := 0
	fixture.svc.TxStarter = stageCompletionMissingLedgerStarter{TxStarter: fixture.pool, hits: &hits}
	params := StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(fixture.workspace), ProjectID: util.MustParseUUID(fixture.project),
		RequestID: uuid.NewString(), ExpectedRevision: 0, Enabled: true, Reason: "Explicitly activate every final writer", Credential: stageCredential(fixture.creator)}
	_, err := fixture.svc.ReviseStagePolicy(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || !collaborationErrorCode(err, "stage_schema_unavailable") {
		t.Fatalf("structural readiness substituted for the exact final ledger: hits=%d err=%v", hits, err)
	}
	if count := fixture.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, fixture.project); count != 0 {
		t.Fatalf("missing final ledger activated a policy: rows=%d", count)
	}
}

func TestStageCompletionStoredBlockersRecoverWithoutFreshEffects(t *testing.T) {
	fixture := newCollaborationStageFixture(t)
	fixture.revise(t, true)
	options, err := fixture.svc.StatusOptions(fixture.handlerCtx, fixture.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	params := CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.req(t, fixture.handlerCtx), ExpectedStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, Note: "A retained result", Basis: "The original verification basis"}
	receipt, err := fixture.svc.SubmitAcceptance(fixture.handlerCtx, params)
	if !collaborationErrorCode(err, "stage_incomplete") || len(receipt.StageBlockers) == 0 || receipt.StageBlockersTruncated == nil {
		t.Fatalf("missing persisted stage refusal: %+v %v", receipt, err)
	}
	events := fixture.stageInt(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`)
	fixture.revise(t, false)
	recovered, err := fixture.svc.GetWorkflowOperation(fixture.handlerCtx, fixture.issue.Issue.ID, params.RequestID)
	if err != nil || recovered.Status != 409 || recovered.Code != "stage_incomplete" || len(recovered.StageBlockers) == 0 {
		t.Fatalf("original refusal recovery lost its bounded blockers: %+v %v", recovered, err)
	}
	replayed, err := fixture.svc.SubmitAcceptance(fixture.handlerCtx, params)
	if !collaborationErrorCode(err, "stage_incomplete") || replayed.Code != receipt.Code || fixture.stageInt(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`) != events {
		t.Fatalf("policy change revived the original refused write: %+v %v", replayed, err)
	}
}

func TestStageCompletionActivityUsesOnlyClosedSafeMetadata(t *testing.T) {
	identifier := uuid.NewString()
	for _, testCase := range []struct {
		kind string
		raw  string
	}{
		{"stage_roles_configured", `{"role_revision":"3","private":"secret"}`},
		{"stage_proposal_created", `{"proposal_id":"` + identifier + `","candidate_id":"` + identifier + `","version":"1","content_digest":"secret"}`},
		{"stage_code_candidate_created", `{"candidate_id":"` + identifier + `","version":"1","parent_candidate_id":null,"source":"secret"}`},
		{"stage_scope_confirmed", `{"decision_id":"` + identifier + `","scope_revision":"2","lifecycle_epoch":"1","children":["secret"]}`},
		{"stage_decision_recorded", `{"candidate_id":"` + identifier + `","decision_id":"` + identifier + `","step":"release","reason":"secret"}`},
		{"stage_execution_observed", `{"artifact_digest":"secret"}`},
	} {
		t.Run(testCase.kind, func(t *testing.T) {
			metadata, valid := stageActivityProjection(testCase.kind, json.RawMessage(testCase.raw))
			if !valid || strings.Contains(string(metadata), "secret") {
				t.Fatalf("unsafe activity metadata: valid=%v metadata=%s", valid, metadata)
			}
		})
	}
	if _, valid := stageActivityProjection("stage_decision_recorded", json.RawMessage(`{"candidate_id":"`+identifier+`","decision_id":"`+identifier+`","step":"operator"}`)); valid {
		t.Fatal("an unknown human decision step produced safe activity")
	}
	if _, valid := stageActivityProjection("stage_roles_configured", json.RawMessage(`{"role_revision":"1","role_revision":"2"}`)); valid {
		t.Fatal("duplicate retained metadata produced a successful activity projection")
	}
	if !errors.Is(stageCompletionMissingLedgerRow{}.Scan(), pgx.ErrNoRows) {
		t.Fatal("the ledger seam did not preserve the real missing-row condition")
	}
}
