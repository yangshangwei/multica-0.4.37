package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageCompletionLateWriteStarter struct {
	TxStarter
	future time.Time
	mutate func(context.Context, pgx.Tx) error
	hits   *int
}

type stageCompletionLateWriteTx struct {
	pgx.Tx
	starter stageCompletionLateWriteStarter
	armed   bool
}

type stageCompletionLateWriteRow struct {
	pgx.Row
	after func() error
}

func (row stageCompletionLateWriteRow) Scan(destinations ...any) error {
	if err := row.Row.Scan(destinations...); err != nil {
		return err
	}
	return row.after()
}

func (starter stageCompletionLateWriteStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	transaction, err := starter.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageCompletionLateWriteTx{Tx: transaction, starter: starter}, nil
}

func (transaction *stageCompletionLateWriteTx) QueryRow(ctx context.Context, statement string, args ...any) pgx.Row {
	row := transaction.Tx.QueryRow(ctx, statement, args...)
	if strings.Contains(statement, "-- name: CompleteCollaborationOperation") && len(args) > 0 && (args[0] == "acceptance_submitted" || args[0] == "completed") {
		return stageCompletionLateWriteRow{Row: row, after: func() error {
			transaction.armed = true
			if transaction.starter.mutate != nil {
				*transaction.starter.hits++
				return transaction.starter.mutate(ctx, transaction.Tx)
			}
			return nil
		}}
	}
	if transaction.armed && !transaction.starter.future.IsZero() && strings.Contains(statement, "-- name: ArtifactExecutionDatabaseTime ") {
		*transaction.starter.hits++
		return stageClockRow{Row: row, future: transaction.starter.future}
	}
	return row
}

func stageCompletionSubmit(t *testing.T, fixture *stageExecutionFixture) pgtype.UUID {
	t.Helper()
	options, err := fixture.svc.StatusOptions(fixture.stage.handlerCtx, fixture.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	params := CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.handlerCtx), ExpectedStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, Note: "An exact frozen submission", Basis: "The retained controlled evidence"}
	receipt, err := fixture.svc.SubmitAcceptance(fixture.stage.handlerCtx, params)
	if err != nil || receipt.DeliveryConfirmation == nil {
		t.Fatalf("verified submission: %+v %v", receipt, err)
	}
	return receipt.DeliveryConfirmation.SubmissionID
}

func stageCompletionAcceptParams(t *testing.T, fixture *stageExecutionFixture, submission pgtype.UUID) CollaborationLifecycleParams {
	t.Helper()
	options, err := fixture.svc.StatusOptions(fixture.ctx, fixture.issue.Issue.ID, "complete")
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationLifecycleParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.ctx), Action: "complete", TargetStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, SubmissionID: &submission, Note: "Accept only this exact result"}
}

func TestStageCompletionFinalSubmitAuthorityCannotChange(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	options, err := fixture.svc.StatusOptions(fixture.stage.handlerCtx, fixture.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	params := CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.handlerCtx), ExpectedStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, Note: "A result awaiting final checks", Basis: "Its original evidence"}
	before := fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, fixture.issue.Issue.ID)
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
		_, mutationErr := transaction.Exec(ctx, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, fixture.issue.Issue.ID, util.MustParseUUID(fixture.handler))
		return mutationErr
	}}
	receipt, err := fixture.svc.SubmitAcceptance(fixture.stage.handlerCtx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || err == nil {
		t.Fatalf("late participant generation passed the final submit guard: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	if count := fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, fixture.issue.Issue.ID); count != before {
		t.Fatalf("final authority refusal retained a submission: before=%d after=%d", before, count)
	}
}

func TestStageCompletionFinalClockRunsAfterWriterReceipts(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	beforeEvents := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID)
	decision, err := fixture.svc.Queries.GetCollaborationStageReviewHead(fixture.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: fixture.issue.Issue.ID, CandidateID: fixture.candidate.CandidateID, Step: "release"})
	if err != nil {
		t.Fatal(err)
	}
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, future: decision.ExpiresAt.Time.Add(time.Second)}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || !collaborationErrorCode(err, "stage_incomplete") || receipt.Code != "stage_incomplete" || len(receipt.StageBlockers) == 0 {
		t.Fatalf("late approval expiry passed the receipt boundary: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	if count := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID); count != beforeEvents {
		t.Fatalf("late refusal retained completion events: before=%d after=%d", beforeEvents, count)
	}
	issue, err := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil || issue.Status == "done" {
		t.Fatalf("late expiry committed done: %+v %v", issue, err)
	}
	recovered, err := fixture.svc.GetWorkflowOperation(fixture.ctx, fixture.issue.Issue.ID, params.RequestID)
	if err != nil || recovered.Code != "stage_incomplete" || len(recovered.StageBlockers) == 0 {
		t.Fatalf("late refusal recovery lost the original blockers: %+v %v", recovered, err)
	}
}

func TestStageCompletionSubmissionRequiresItsOriginalOperation(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	submission := stageCompletionSubmit(t, fixture)
	row, err := fixture.svc.Queries.GetCollaborationStageSubmission(fixture.ctx, db.GetCollaborationStageSubmissionParams{IssueID: fixture.issue.Issue.ID, SubmissionID: submission})
	if err != nil {
		t.Fatal(err)
	}
	fixture.fx.Exec(t, `DELETE FROM collaboration_stage_submission WHERE submission_id=$1`, submission)
	_, err = fixture.svc.Queries.CreateCollaborationStageSubmission(fixture.ctx, db.CreateCollaborationStageSubmissionParams{SubmissionID: row.SubmissionID, IssueID: row.IssueID, WorkspaceID: row.WorkspaceID,
		LifecycleEpoch: row.LifecycleEpoch, PolicyRevision: row.PolicyRevision, RoleRevision: row.RoleRevision, StageRevision: row.StageRevision, ScopeDecisionID: row.ScopeDecisionID,
		CandidateID: row.CandidateID, CandidateVersion: row.CandidateVersion, ContentDigest: row.ContentDigest, ReleaseDecisionID: row.ReleaseDecisionID, ExecutionObservationID: row.ExecutionObservationID,
		OperationID: util.MustParseUUID(uuid.NewString()), ActorUserID: row.ActorUserID})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, stageCompletionAcceptParams(t, fixture, submission))
	if !collaborationErrorCode(err, "stage_incomplete") || len(receipt.StageBlockers) != 1 || receipt.StageBlockers[0].Code != "stage_submission_unbound" {
		t.Fatalf("an unrelated original operation authorized the immutable binding: %+v %v", receipt, err)
	}
}

func stageCompletionExpectOwnedRowLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner pgx.Tx, statement string, args ...any) {
	t.Helper()
	var ownerPID int32
	if err := owner.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&ownerPID); err != nil {
		t.Fatal(err)
	}
	competitor, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer competitor.Rollback(ctx)
	var competitorPID int32
	if err = competitor.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&competitorPID); err != nil || competitorPID == ownerPID {
		t.Fatalf("row lock probe did not use a distinct actual connection: owner=%d competitor=%d err=%v", ownerPID, competitorPID, err)
	}
	var identifier pgtype.UUID
	err = competitor.QueryRow(ctx, statement, args...).Scan(&identifier)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55P03" {
		t.Fatalf("the actual owning transaction did not retain its exact row lock: identifier=%+v err=%v", identifier, err)
	}
	if err = competitor.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = owner.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	control, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Rollback(ctx)
	if err = control.QueryRow(ctx, statement, args...).Scan(&identifier); err != nil || !identifier.Valid {
		t.Fatalf("row lock survived the owning transaction or target was missing: identifier=%+v err=%v", identifier, err)
	}
}

func TestStageCompletionCandidateHeadLocksInOwningTransaction(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	transaction, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(fixture.ctx)
	issue, err := db.New(transaction).GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	access, err := completionCollaborationAccess(fixture.ctx, transaction, issue, util.MustParseUUID(fixture.creator))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := stageCompletionFacts(fixture.ctx, transaction, fixture.source, access)
	if err != nil || evidence.candidate.CandidateID != fixture.candidate.CandidateID {
		t.Fatalf("actual completion facts: %+v %v", evidence, err)
	}
	stageCompletionExpectOwnedRowLock(t, fixture.ctx, fixture.pool, transaction, "SELECT candidate_id FROM collaboration_stage_candidate WHERE candidate_id=$1 AND issue_id=$2 FOR UPDATE NOWAIT", fixture.candidate.CandidateID, issue.ID)
}

func TestStageCompletionImmutableSubmissionLocksInOwningTransaction(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	transaction, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(fixture.ctx)
	issue, err := db.New(transaction).GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	target := DeliveryCompletionTarget{Kind: "collaboration_submission", IssueID: util.UUIDToString(issue.ID), ExpectedIssueRevision: issue.Revision,
		ExpectedStatus: issue.Status, TargetStatus: params.TargetStatus, ExpectedCollaborationRevision: stageVersion(params.ExpectedCollaborationRevision),
		ExpectedLifecycleEpoch: stageVersion(params.ExpectedLifecycleEpoch), ExpectedParticipantEpoch: stageVersion(params.ExpectedParticipantEpoch),
		ExpectedAccountEpoch: stageVersion(params.ExpectedAccountEpoch), StatusOptionsVersion: params.StatusOptionsVersion, SubmissionID: func() *string { value := util.UUIDToString(submission); return &value }(), AcceptanceNote: params.Note}
	operation, err := db.New(transaction).CreateCollaborationOperation(fixture.ctx, db.CreateCollaborationOperationParams{ScopeKind: "issue", ScopeID: issue.ID,
		ActorUserID: util.MustParseUUID(fixture.creator), RequestID: params.RequestID, Kind: "complete", PayloadHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := prepareStageMutation(fixture.ctx, transaction, DeliveryMutationIntent{Before: issue, Candidate: issueWithCompletionStatus(issue, params.TargetStatus),
		Collaboration: &target, Credential: completionRequestCredential(fixture.ctx), StageExecutionSources: fixture.source, StageOperationID: operation.ID}, false)
	if err != nil || guard == nil {
		t.Fatalf("actual submission completion preparation: guard=%+v err=%v", guard, err)
	}
	stageCompletionExpectOwnedRowLock(t, fixture.ctx, fixture.pool, transaction, "SELECT submission_id FROM collaboration_stage_submission WHERE submission_id=$1 AND issue_id=$2 FOR UPDATE NOWAIT", submission, issue.ID)
}

func issueWithCompletionStatus(issue db.Issue, target string) db.Issue {
	issue.Status = target
	return issue
}

func TestStageCompletionFinalSubmitAccountCannotChange(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	options, err := fixture.svc.StatusOptions(fixture.stage.handlerCtx, fixture.issue.Issue.ID, "submit_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	params := CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.handlerCtx), ExpectedStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, Note: "A result awaiting final account checks", Basis: "Its original evidence"}
	before := fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, fixture.issue.Issue.ID)
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
		_, mutationErr := transaction.Exec(ctx, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, util.MustParseUUID(fixture.handler))
		return mutationErr
	}}
	receipt, err := fixture.svc.SubmitAcceptance(fixture.stage.handlerCtx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || err == nil {
		t.Fatalf("late account generation passed the final submit guard: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	if count := fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, fixture.issue.Issue.ID); count != before {
		t.Fatalf("final account refusal retained a submission: before=%d after=%d", before, count)
	}
}

func TestStageCompletionFinalOperationHeaderAndResultCannotChange(t *testing.T) {
	for _, change := range []string{"actor", "result"} {
		t.Run(change, func(t *testing.T) {
			fixture := newStageCompletionFixture(t)
			submission := stageCompletionSubmit(t, fixture)
			params := stageCompletionAcceptParams(t, fixture, submission)
			before := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID)
			hits := 0
			fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
				var mutationErr error
				if change == "actor" {
					_, mutationErr = transaction.Exec(ctx, `UPDATE collaboration_operation SET actor_user_id=$1 WHERE scope_id=$2 AND request_id=$3`, util.MustParseUUID(fixture.handler), fixture.issue.Issue.ID, params.RequestID)
				} else {
					_, mutationErr = transaction.Exec(ctx, `UPDATE collaboration_operation SET result=jsonb_set(result,'{delivery_confirmation,submission_id}',to_jsonb($1::text)) WHERE scope_id=$2 AND request_id=$3`, uuid.NewString(), fixture.issue.Issue.ID, params.RequestID)
				}
				return mutationErr
			}}
			receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
			fixture.svc.TxStarter = fixture.pool
			if hits == 0 || err == nil {
				t.Fatalf("changed original %s passed the final guard: hits=%d receipt=%+v err=%v", change, hits, receipt, err)
			}
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if readErr != nil || issue.Status == "done" || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID) != before {
				t.Fatalf("changed original %s retained completion effects: %+v %v", change, issue, readErr)
			}
		})
	}
}

type stageCompletionLateSource struct {
	StageExecutionSources
	fixture      *stageExecutionSourceFixture
	armed        bool
	targets      int
	metrics      int
	changeTarget bool
	changeMetric bool
}

func (source *stageCompletionLateSource) Targets(ctx context.Context, transaction pgx.Tx, workspace pgtype.UUID) ([]StageExecutionTarget, error) {
	targets, err := source.StageExecutionSources.Targets(ctx, transaction, workspace)
	if err == nil && source.armed {
		source.targets++
		if source.changeTarget && source.targets == 2 {
			source.fixture.mu.Lock()
			source.fixture.target.ConditionalVersion += ".late"
			source.fixture.mu.Unlock()
		}
	}
	return targets, err
}

func (source *stageCompletionLateSource) CheckMetricCurrent(ctx context.Context, transaction pgx.Tx, target StageExecutionTarget, metric StageRollbackMetric) error {
	err := source.StageExecutionSources.CheckMetricCurrent(ctx, transaction, target, metric)
	if err == nil && source.armed {
		source.metrics++
		if source.changeMetric && source.metrics == 1 {
			source.fixture.mu.Lock()
			source.fixture.metricSource.ConfigRevision = "2"
			source.fixture.mu.Unlock()
		}
	}
	return err
}

func TestStageCompletionFinalTargetCannotChangeAfterObservation(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	source := &stageCompletionLateSource{StageExecutionSources: fixture.source, fixture: fixture.source, changeTarget: true}
	fixture.svc.StageExecutionSources = source
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(context.Context, pgx.Tx) error {
		source.armed = true
		return nil
	}}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || source.targets < 2 || !collaborationErrorCode(err, "stage_incomplete") {
		t.Fatalf("target changed after its successful observation: targets=%d receipt=%+v err=%v", source.targets, receipt, err)
	}
	issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if readErr != nil || issue.Status == "done" {
		t.Fatalf("late target change retained done: %+v %v", issue, readErr)
	}
}

func TestStageCompletionFinalRollbackMetricCannotChangeAfterObservation(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	release, err := fixture.svc.Queries.GetCollaborationStageReviewHead(fixture.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: fixture.issue.Issue.ID, CandidateID: fixture.candidate.CandidateID, Step: "release"})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := fixture.svc.Queries.GetStageExecutionIntentByApproval(fixture.ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: fixture.issue.Issue.ID, ApprovalDecisionID: release.DecisionID})
	if err != nil {
		t.Fatal(err)
	}
	fact, err := fixture.svc.Queries.GetStageExecutionTerminal(fixture.ctx, intent.IntentID)
	if err != nil {
		t.Fatal(err)
	}
	metric := fixture.metric(t, util.UUIDToString(fact.ObservationID), 1, "anomaly")
	rollback := fixture.rollback(t, util.UUIDToString(release.DecisionID), util.UUIDToString(fact.ObservationID), metric)
	rolled, err := fixture.svc.PrepareStageExecution(context.Background(), rollback, fixture.source.consumer(rollback))
	if err != nil || rolled.State != "succeeded" {
		t.Fatalf("controlled rollback: %+v %v", rolled, err)
	}
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	source := &stageCompletionLateSource{StageExecutionSources: fixture.source, fixture: fixture.source, changeMetric: true}
	fixture.svc.StageExecutionSources = source
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(context.Context, pgx.Tx) error {
		source.armed = true
		return nil
	}}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || source.metrics == 0 || !collaborationErrorCode(err, "stage_incomplete") {
		t.Fatalf("metric changed after its successful observation: metrics=%d receipt=%+v err=%v", source.metrics, receipt, err)
	}
	issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if readErr != nil || issue.Status == "done" {
		t.Fatalf("late metric change retained done: %+v %v", issue, readErr)
	}
}

func stageCompletionM1Policy(t *testing.T, fixture *stageExecutionFixture, mode string) {
	t.Helper()
	fixture.fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": fixture.workspace, "project_id": fixture.stage.project, "revision": int64(1), "policy_version": DeliveryEvidencePolicy, "mode": mode, "stage": DeliveryPolicyStage, "reason": "Synthetic combined stage and M1 policy", "created_by": fixture.creator}, "project_id=$1", fixture.stage.project)
}

// The grant is created by the existing M1 issuer service from the real child
// artifact. Synthetic stage signatures and packager bytes are not S4 evidence.
func stageCompletionM1Grant(t *testing.T, fixture *stageExecutionFixture, ctx context.Context, operation string, target DeliveryCompletionTarget, consumer DeliveryCompletionConsumer) DeliveryCompletionBinding {
	t.Helper()
	transaction, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(ctx)
	queries := db.New(transaction)
	issue, err := queries.GetIssue(ctx, fixture.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	project, err := queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := queries.GetCollaborationStageCodeCandidate(ctx, db.GetCollaborationStageCodeCandidateParams{IssueID: issue.ID, CandidateID: fixture.candidate.CandidateID})
	if err != nil {
		t.Fatal(err)
	}
	content, err := decodeStageCode(candidate)
	if err != nil || content.Baseline.Artifact == nil || content.Source.ArtifactObservation == nil {
		t.Fatalf("original frozen artifact: %+v %v", content, err)
	}
	child, err := queries.GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
	if err != nil {
		t.Fatal(err)
	}
	now, err := queries.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request := DeliveryCompletionAuthorizationRequest{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: fixture.workspace, OperationID: operation,
		Target: target, Consumer: consumer, Project: DeliveryCompletionProject{ID: fixture.stage.project, ExpectedRevision: project.Revision, ExpectedPolicyRevision: 1},
		Evidence: DeliveryCompletionEvidence{IssueID: util.UUIDToString(child.ID), ExpectedIssueRevision: child.Revision, ApprovalID: content.Baseline.Artifact.ApprovalID,
			ExpectedSourceRevision: content.Baseline.PullRequest.HeadSHA, ExpectedSpecDigest: content.Source.ArtifactObservation.SpecDigest, ExpectedArtifactDigest: content.Baseline.Artifact.ArtifactDigest},
		Reason: "Synthetic human accepts this exact source for the original operation", ExpiresAt: now.Time.Add(30 * time.Second).UTC().Truncate(time.Microsecond)}
	view, err := CreateDeliveryCompletionAuthorizationInTx(ctx, transaction, ArtifactAccess{Credential: completionRequestCredential(ctx), RequestedWorkspaceID: fixture.workspace, CollaborationAdministrationEnabled: true}, util.MustParseUUID(uuid.NewString()), request)
	if err != nil {
		t.Fatalf("actual M1 authorization: %v", err)
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	fixture.fx.Cleanup(t, `DELETE FROM delivery_completion_authorization WHERE id=$1`, view.AuthorizationID)
	fixture.fx.Cleanup(t, `DELETE FROM delivery_completion_terminal WHERE authorization_id=$1`, view.AuthorizationID)
	return DeliveryCompletionBinding{AuthorizationID: view.AuthorizationID, OperationID: operation, ExpectedIssueRevision: target.ExpectedIssueRevision}
}

func stageCompletionM1ForLifecycle(t *testing.T, fixture *stageExecutionFixture, ctx context.Context, p CollaborationLifecycleParams) DeliveryCompletionBinding {
	t.Helper()
	access, err := fixture.svc.ResolveIssueAccess(fixture.ctx, nil, fixture.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	writer := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{actor: util.MustParseUUID(fixture.creator), access: access}}
	return stageCompletionM1Grant(t, fixture, ctx, p.RequestID, completionCollaborationTargetFromWorkflow(writer, p), DeliveryCompletionConsumer{Kind: "human", UserID: fixture.creator})
}

func TestStageCompletionFinalResultContentCannotChange(t *testing.T) {
	for _, variant := range []string{"submit", "submission", "self", "acceptance_reason"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newStageCompletionFixture(t)
			var submission pgtype.UUID
			if variant == "self" {
				_, err := fixture.svc.Transfer(fixture.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.ctx), TargetUserID: util.MustParseUUID(fixture.creator)}})
				if err != nil {
					t.Fatal(err)
				}
			} else if variant != "submit" {
				submission = stageCompletionSubmit(t, fixture)
			}
			before, err := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeEvents := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID)
			hits := 0
			fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
				if variant == "acceptance_reason" {
					_, err := transaction.Exec(ctx, `UPDATE collaboration_event SET payload=jsonb_set(payload,'{delivery_decision,reason}',to_jsonb('A different acceptance intent'::text)) WHERE issue_id=$1 AND kind='completed'`, before.ID)
					return err
				}
				_, err := transaction.Exec(ctx, `UPDATE collaboration_event SET payload=jsonb_set(payload,'{delivery,basis}',to_jsonb('A different retained basis'::text)) WHERE issue_id=$1 AND kind='acceptance_submitted'`, before.ID)
				return err
			}}
			var receipt CollaborationWorkflowReceipt
			if variant == "submit" {
				options, optionsErr := fixture.svc.StatusOptions(fixture.stage.handlerCtx, before.ID, "submit_acceptance")
				if optionsErr != nil {
					t.Fatal(optionsErr)
				}
				receipt, err = fixture.svc.SubmitAcceptance(fixture.stage.handlerCtx, CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.handlerCtx), ExpectedStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion, Note: "Original submitted result", Basis: "Original submitted basis"})
			} else {
				params := stageCompletionAcceptParams(t, fixture, submission)
				if variant == "self" {
					params.SubmissionID, params.Note, params.Basis = nil, "Original self result", "Original self basis"
				}
				receipt, err = fixture.svc.ChangeLifecycle(fixture.ctx, params)
			}
			fixture.svc.TxStarter = fixture.pool
			if hits != 1 || err == nil {
				t.Fatalf("changed %s content passed the actual final writer: hits=%d receipt=%+v err=%v", variant, hits, receipt, err)
			}
			after, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, before.ID)
			if readErr != nil || after.Status != before.Status || after.Revision != before.Revision || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID) != beforeEvents {
				t.Fatalf("changed %s retained partial result effects: before=%+v after=%+v err=%v", variant, before, after, readErr)
			}
		})
	}
}

func TestStageCompletionFinalM1ConsumptionCannotDisappear(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, fixture, "enforce")
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	binding := stageCompletionM1ForLifecycle(t, fixture, fixture.stage.reviewerCtx, params)
	params.Completion = &binding
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
		_, err := transaction.Exec(ctx, `DELETE FROM delivery_completion_terminal WHERE authorization_id=$1`, binding.AuthorizationID)
		return err
	}}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits != 1 || err == nil {
		t.Fatalf("missing original M1 terminal passed the stage final guard: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	issue, err := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil || issue.Status == "done" {
		t.Fatalf("missing M1 consumption retained done: %+v %v", issue, err)
	}
}

func TestStageCompletionFinalM1ExpiryAfterOriginalReceipt(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, fixture, "enforce")
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	binding := stageCompletionM1ForLifecycle(t, fixture, fixture.stage.reviewerCtx, params)
	params.Completion = &binding
	row, err := fixture.svc.Queries.GetDeliveryCompletionAuthorization(fixture.ctx, util.MustParseUUID(binding.AuthorizationID))
	if err != nil {
		t.Fatal(err)
	}
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, future: row.ExpiresAt.Time.Add(time.Second)}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits == 0 || err == nil {
		t.Fatalf("M1 expired after its original receipt but completion passed: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	issue, err := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil || issue.Status == "done" || fixture.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, binding.AuthorizationID) != 0 {
		t.Fatalf("late M1 expiry retained completion/consumption: %+v %v", issue, err)
	}
}

func TestStageCompletionFinalM1IssuerRevocationAfterOriginalReceipt(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, fixture, "enforce")
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	hash := strings.Repeat("d", 64)
	pat := fixture.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": fixture.stage.reviewer, "name": "Synthetic separate M1 issuer", "token_hash": hash, "token_prefix": "synthetic", "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	issuer := auth.WithAuthenticatedPrincipal(fixture.stage.reviewerCtx, auth.AuthenticatedPrincipal{UserID: fixture.stage.reviewer, Kind: auth.CredentialPAT, PATTokenHash: hash})
	issuer = auth.WithPasswordSession(issuer, auth.PasswordSession{UserID: fixture.stage.reviewer, Kind: "pat", Version: 1})
	binding := stageCompletionM1ForLifecycle(t, fixture, issuer, params)
	params.Completion = &binding
	hits := 0
	fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, transaction pgx.Tx) error {
		_, err := transaction.Exec(ctx, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, pat)
		return err
	}}
	receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
	fixture.svc.TxStarter = fixture.pool
	if hits != 1 || err == nil {
		t.Fatalf("the separately signed M1 issuer was revoked after its receipt: hits=%d receipt=%+v err=%v", hits, receipt, err)
	}
	issue, err := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
	if err != nil || issue.Status == "done" || fixture.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, binding.AuthorizationID) != 0 {
		t.Fatalf("late M1 issuer revocation retained completion: %+v %v", issue, err)
	}
}

// This exercises the borrowed validators inside the existing workflow owner.
// The only omitted production step is M1 Finish, so stage must refuse even when
// the status/result/Stop and original operation were otherwise recorded.
func TestStageCompletionGuardRequiresM1Finalization(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, fixture, "enforce")
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	binding := stageCompletionM1ForLifecycle(t, fixture, fixture.stage.reviewerCtx, params)
	params.Completion = &binding
	transaction, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(fixture.ctx)
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := fixture.svc.workflowInTx(fixture.ctx, transaction, util.MustParseUUID(fixture.creator), pgtype.UUID{}, params.CollaborationWorkflowRequest, "complete", hexHash(raw), func(writer *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		candidate := writer.access.Issue
		candidate.Status = params.TargetStatus
		target := completionCollaborationTargetFromWorkflow(writer, params)
		prepared, err := PrepareDeliveryMutationInTx(fixture.ctx, transaction, DeliveryMutationIntent{Before: writer.access.Issue, Candidate: candidate, Completion: params.Completion, Credential: completionRequestCredential(fixture.ctx), Collaboration: &target, StageExecutionSources: fixture.source, StageOperationID: writer.operation.ID})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if prepared.State != DeliveryPrepared || prepared.Prepared == nil || prepared.stage == nil {
			t.Fatalf("actual combined preparation missing: %+v", prepared)
		}
		if refusal, err := writer.prepareCompletion(fixture.ctx, params); refusal != nil || err != nil {
			t.Fatalf("original exact acceptance preparation: %+v %v", refusal, err)
		}
		result, err := writer.changeLifecycle(fixture.ctx, "complete", params.TargetStatus)
		writer.stagePrepared = prepared
		writer.stageWriterResult = DeliveryWriterResult{CollaborationOperationID: writer.operation.ID, CollaborationRevision: writer.access.Revision, SubmissionID: submission}
		return result, err
	})
	if err == nil {
		t.Fatalf("unfinalized M1 preparation passed the actual workflow final guard: %+v", receipt)
	}
}

// The ordinary artifact's real packager binding is owned by the independent
// source reviewer. Copy fixture wrappers so the collaboration creator itself
// remains unchanged, and retain the original source fixture contract.
func stageCompletionIndependentArtifact(t *testing.T, fixture *stageReviewFixture) {
	t.Helper()
	copy := *fixture
	stage := *fixture.collaborationStageFixture
	queue := *stage.collaborationQueueFixture
	base := *queue.collaborationFixture
	base.creator = fixture.reviewer
	queue.collaborationFixture = &base
	stage.collaborationQueueFixture = &queue
	copy.collaborationStageFixture = &stage
	stageArtifactReadFixture(t, &copy)
	fixture.artifact = copy.artifact
}

func stageCompletionIndependentProductRole(t *testing.T, fixture *stageReviewFixture) {
	t.Helper()
	roles := fixture.roleSet()
	roles.ProductApprover = util.MustParseUUID(fixture.reviewer)
	receipt, err := fixture.svc.ConfigureStageRoles(fixture.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: fixture.req(t, fixture.ctx), ExpectedPolicyRevision: fixture.policyRevision,
		ExpectedRoleRevision: fixture.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), Roles: roles})
	if err != nil || receipt.Code != "stage_roles_configured" {
		t.Fatalf("independent product role: %+v %v", receipt, err)
	}
	scope, err := fixture.svc.ConfirmStageScope(fixture.reviewerCtx, CollaborationStageScopeParams{CollaborationWorkflowRequest: fixture.req(t, fixture.reviewerCtx), ExpectedPolicyRevision: fixture.policyRevision,
		ExpectedRoleRevision: fixture.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedScopeRevision: fixture.stageInt(t, `SELECT scope_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		Children: []CollaborationStageScopeChild{{IssueID: util.MustParseUUID(fixture.child), ContentDigest: stageReadableDigest(t, stageChildView(t, fixture.view(t), fixture.child))}}})
	if err != nil || scope.Code != "stage_scope_confirmed" {
		t.Fatalf("independent product scope: %+v %v", scope, err)
	}
}

func stageCompletionAdmin(t *testing.T, fixture *stageExecutionFixture) *adminCollaborationFixture {
	t.Helper()
	admin := util.MustParseUUID(fixture.fx.User(t, "Synthetic independent completion administrator", "stage-admin-"+uuid.NewString()+"@example.invalid"))
	hash, err := auth.HashPassword(fixture.stage.reviewerCtx, platformAdminTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	fixture.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": admin, "username": strings.ReplaceAll(uuid.NewString(), "-", ""), "password_hash": hash}, "user_id=$1", admin)
	fixture.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": admin, "role": PlatformRoleSuperAdmin}, "user_id=$1", admin)
	org, err := fixture.svc.Queries.GetInternalOrganization(fixture.stage.reviewerCtx)
	if err != nil {
		t.Fatal(err)
	}
	var mapped pgtype.UUID
	fixture.fx.QueryRow(t, `SELECT organization_id FROM organization_workspace WHERE workspace_id=$1`, fixture.workspace).Scan(&mapped)
	if mapped != org.ID {
		t.Fatal("managed fixture must already belong to the current internal organization")
	}
	for _, target := range []pgtype.UUID{fixture.issue.Issue.ID, util.MustParseUUID(fixture.creator)} {
		for _, table := range []string{"admin_operation", "admin_audit_event"} {
			fixture.fx.Cleanup(t, "DELETE FROM "+table+" WHERE target_id=$1", target)
		}
	}
	for _, table := range []string{"collaboration_account_invalidation", "collaboration_account_fence"} {
		fixture.fx.Cleanup(t, "DELETE FROM "+table+" WHERE user_id=$1", util.MustParseUUID(fixture.creator))
	}
	context := auth.WithAuthenticatedPrincipal(adminCollaborationContext(admin, 1), stageCredential(util.UUIDToString(admin)).Principal)
	actions := NewAdminCollaborationService(fixture.svc.Queries, fixture.pool, true)
	actions.StageExecutionSources = fixture.source
	return &adminCollaborationFixture{collaborationQueueFixture: fixture.collaborationQueueFixture, actions: actions, org: org.ID, admin: admin, adminCtx: context}
}

func stageCompletionDisableCreator(t *testing.T, fixture *stageExecutionFixture, admin *adminCollaborationFixture) {
	t.Helper()
	_, err := NewPlatformAdminService(fixture.svc.Queries, fixture.pool).ChangeAccount(admin.adminCtx, PlatformAccountChangeParams{OrganizationID: admin.org,
		TargetUserID: util.MustParseUUID(fixture.creator), IdempotencyKey: util.MustParseUUID(uuid.NewString()), Action: "disable", ExpectedAuthVersion: 1,
		Reason: "Synthetic R38 creator disable with actual credential revocation", Password: platformAdminTestPassword, RequestID: uuid.NewString()})
	if err != nil {
		t.Fatalf("actual creator disable: %v", err)
	}
}

func stageCompletionRework(t *testing.T, fixture *stageExecutionFixture) {
	t.Helper()
	returned, err := fixture.svc.DecideStage(fixture.stage.releaseCtx, CollaborationStageReviewParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.releaseCtx), ExpectedPolicyRevision: fixture.stage.policyRevision,
		ExpectedRoleRevision: fixture.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: fixture.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		Step: "rollback", Decision: "return", Candidate: fixture.candidate, DependsOn: []string{}, Reason: "Synthetic explicit rework before final delivery"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.svc.AdvanceStage(fixture.ctx, fixture.stage.advanceParams(t, fixture.candidate, "development", returned.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
}

func TestStageCompletionAllM1ModesAndActualWriters(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			for _, variant := range []string{"submit", "submission", "self", "admin"} {
				t.Run(variant, func(t *testing.T) {
					for _, verified := range []bool{false, true} {
						name := "incomplete"
						if verified {
							name = "verified"
						}
						t.Run(name, func(t *testing.T) {
							setup := []func(*testing.T, *stageReviewFixture){stageCompletionIndependentArtifact}
							if variant == "admin" {
								setup = append(setup, stageCompletionIndependentProductRole)
							}
							fixture := newStageCompletionFixture(t, setup...)
							stageCompletionM1Policy(t, fixture, mode)
							var submission pgtype.UUID
							if variant == "submission" {
								submission = stageCompletionSubmit(t, fixture)
							} else if variant == "self" {
								_, err := fixture.svc.Transfer(fixture.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.ctx), TargetUserID: util.MustParseUUID(fixture.creator)}})
								if err != nil {
									t.Fatal(err)
								}
							}
							if !verified {
								stageCompletionRework(t, fixture)
							}
							var admin *adminCollaborationFixture
							if variant == "admin" {
								admin = stageCompletionAdmin(t, fixture)
								stageCompletionDisableCreator(t, fixture, admin)
							} else if variant != "submit" {
								// The real source member deletion leaves current collaboration
								// participation intact. Issuer P and consumer E stay distinct.
								member, err := fixture.svc.Queries.GetMemberByUserAndWorkspace(fixture.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(fixture.workspace), UserID: util.MustParseUUID(fixture.creator)})
								if err != nil {
									t.Fatal(err)
								}
								if err = fixture.svc.Queries.DeleteMember(fixture.ctx, member.ID); err != nil {
									t.Fatal(err)
								}
							}
							before, err := fixture.svc.Queries.GetIssue(fixture.stage.reviewerCtx, fixture.issue.Issue.ID)
							if err != nil {
								t.Fatal(err)
							}
							beforeHandoffs := fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, before.ID)
							beforeEvents := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID)
							var receipt CollaborationWorkflowReceipt
							var params CollaborationLifecycleParams
							var administrative AdminCollaborationLifecycleParams
							var binding *DeliveryCompletionBinding
							if variant == "submit" {
								options, optionsErr := fixture.svc.StatusOptions(fixture.stage.handlerCtx, before.ID, "submit_acceptance")
								if optionsErr != nil {
									t.Fatal(optionsErr)
								}
								receipt, err = fixture.svc.SubmitAcceptance(fixture.stage.handlerCtx, CollaborationAcceptanceParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.stage.handlerCtx), ExpectedStatus: options.DefaultStatus,
									StatusOptionsVersion: options.StatusOptionsVersion, Note: "Synthetic exact submitted result", Basis: "Actual retained source and stage facts"})
							} else if variant == "admin" {
								administrative = admin.params(t, "complete")
								if verified && mode == "enforce" {
									missing := administrative
									missing.IdempotencyKey = util.MustParseUUID(uuid.NewString())
									if _, missingErr := admin.actions.ChangeLifecycle(admin.adminCtx, missing); missingErr == nil {
										t.Fatal("stage-valid administrative completion omitted its M1 grant")
									}
									access := CollaborationIssueAccess{Issue: before, LifecycleEpoch: fixture.stage.stageInt(t, `SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1`)}
									writer := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{access: access}}
									grant := stageCompletionM1Grant(t, fixture, fixture.stage.reviewerCtx, util.UUIDToString(administrative.IdempotencyKey), completionAdminTarget(writer, administrative, nil), DeliveryCompletionConsumer{Kind: "admin", UserID: util.UUIDToString(admin.admin), OrganizationID: util.UUIDToString(admin.org)})
									binding, administrative.Completion = &grant, &grant
								}
								var result AdminCollaborationResult
								result, err = admin.actions.ChangeLifecycle(admin.adminCtx, administrative)
								receipt.Code = result.Result.Code
							} else {
								params = stageCompletionAcceptParams(t, fixture, submission)
								if variant == "self" {
									params.SubmissionID, params.Note, params.Basis = nil, "Synthetic exact self result", "Actual retained source and stage facts"
								}
								access, accessErr := fixture.svc.ResolveIssueAccess(fixture.ctx, nil, before.ID)
								if accessErr != nil || access.SourceMember || !access.Capabilities.Complete {
									t.Fatalf("consumer E must retain completion without source membership: %+v %v", access, accessErr)
								}
								if verified && mode == "enforce" {
									missing := params
									missing.RequestID = uuid.NewString()
									if missingReceipt, missingErr := fixture.svc.ChangeLifecycle(fixture.ctx, missing); missingErr == nil || missingReceipt.Status != 428 {
										t.Fatalf("stage-valid completion omitted its independent M1 grant: %+v %v", missingReceipt, missingErr)
									}
									grant := stageCompletionM1ForLifecycle(t, fixture, fixture.stage.reviewerCtx, params)
									binding, params.Completion = &grant, &grant
								}
								receipt, err = fixture.svc.ChangeLifecycle(fixture.ctx, params)
							}
							if !verified {
								blockers := receipt.StageBlockers
								if variant == "admin" {
									var blocked *StageCompletionError
									if errors.As(err, &blocked) {
										blockers = blocked.Blockers
									}
								}
								if err == nil || len(blockers) != 1 || blockers[0].Code != "stage_not_reached" {
									t.Fatalf("incomplete %s/%s lost the original stage refusal/blockers: error_type=%T receipt=%+v err=%v", mode, variant, err, receipt, err)
								}
								after, readErr := fixture.svc.Queries.GetIssue(fixture.stage.reviewerCtx, before.ID)
								if readErr != nil || after.Status != before.Status || after.Revision != before.Revision || fixture.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, before.ID) != beforeHandoffs || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID) != beforeEvents {
									t.Fatalf("incomplete %s/%s retained writer effects: %+v %v", mode, variant, after, readErr)
								}
								return
							}
							if err != nil || variant == "submit" && receipt.Code != "acceptance_submitted" || variant != "submit" && receipt.Code != "completed" {
								t.Fatalf("verified %s/%s: %+v %v", mode, variant, receipt, err)
							}
							if variant == "submit" {
								if fixture.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1`, before.ID) != 1 {
									t.Fatal("verified submission must retain one immutable candidate binding")
								}
								return
							}
							if variant == "admin" {
								if fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload ? 'delivery_decision'`, before.ID) != 0 {
									t.Fatal("R38 fabricated initiator acceptance")
								}
							} else if receipt.DeliveryConfirmation == nil || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1`, before.ID) != 1 {
								t.Fatal("verified ordinary collaboration completion lost its exact result binding")
							}
							if binding != nil && fixture.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, binding.AuthorizationID) != 1 {
								t.Fatal("enforce must consume exactly the original M1 authorization")
							}
							if variant == "admin" {
								if _, err = admin.actions.ChangeLifecycle(admin.adminCtx, admin.params(t, "reopen")); err != nil {
									t.Fatal(err)
								}
							} else {
								options, optionsErr := fixture.svc.StatusOptions(fixture.ctx, before.ID, "reopen")
								if optionsErr != nil {
									t.Fatal(optionsErr)
								}
								if _, err = fixture.svc.ChangeLifecycle(fixture.ctx, CollaborationLifecycleParams{CollaborationWorkflowRequest: fixture.stage.req(t, fixture.ctx), Action: "reopen", TargetStatus: options.DefaultStatus, StatusOptionsVersion: options.StatusOptionsVersion}); err != nil {
									t.Fatal(err)
								}
							}
							currentEvents := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID)
							if variant == "admin" {
								result, replayErr := admin.actions.ChangeLifecycle(admin.adminCtx, administrative)
								if replayErr != nil || !result.Replayed || result.Result.Code != "completed" {
									t.Fatalf("historical R38 key after reopen: %+v %v", result, replayErr)
								}
							} else if replay, replayErr := fixture.svc.ChangeLifecycle(fixture.ctx, params); replayErr != nil || replay.Code != "completed" || replay.CollaborationRevision != receipt.CollaborationRevision {
								t.Fatalf("historical completion key after reopen: %+v %v", replay, replayErr)
							}
							current, readErr := fixture.svc.Queries.GetIssue(fixture.stage.reviewerCtx, before.ID)
							if readErr != nil || current.Status == "done" || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, before.ID) != currentEvents {
								t.Fatalf("historical replay created a new completion: %+v %v", current, readErr)
							}
						})
					}
				})
			}
		})
	}
}

func stageCompletionReviewerPAT(t *testing.T, fixture *stageReviewFixture) {
	t.Helper()
	hash := hexHash([]byte(uuid.NewString()))
	fixture.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": fixture.reviewer, "name": "Synthetic original stage signer", "token_hash": hash, "token_prefix": "synthetic", "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	fixture.reviewerCtx = auth.WithAuthenticatedPrincipal(fixture.reviewerCtx, auth.AuthenticatedPrincipal{UserID: fixture.reviewer, Kind: auth.CredentialPAT, PATTokenHash: hash})
	fixture.reviewerCtx = auth.WithPasswordSession(fixture.reviewerCtx, auth.PasswordSession{UserID: fixture.reviewer, Kind: "pat", Version: 1})
}

func TestStageCompletionFinalCurrentCredentialHorizons(t *testing.T) {
	for _, subject := range []string{"stage_signer", "m1_issuer", "caller"} {
		t.Run(subject, func(t *testing.T) {
			setup := []func(*testing.T, *stageReviewFixture){stageArtifactReadFixture}
			if subject == "stage_signer" {
				setup = append(setup, stageCompletionReviewerPAT)
			}
			fixture := newStageCompletionFixture(t, setup...)
			submission := stageCompletionSubmit(t, fixture)
			params := stageCompletionAcceptParams(t, fixture, submission)
			ctx := fixture.ctx
			until := time.Now().UTC().Add(25 * time.Second).Truncate(time.Microsecond)
			if subject == "stage_signer" {
				credential := completionRequestCredential(fixture.stage.reviewerCtx)
				fixture.fx.Exec(t, `UPDATE personal_access_token SET expires_at=$2 WHERE token_hash=$1`, credential.Principal.PATTokenHash, until)
			} else if subject == "m1_issuer" {
				stageCompletionM1Policy(t, fixture, "enforce")
				hash := hexHash([]byte(uuid.NewString()))
				pat := fixture.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": fixture.stage.reviewer, "name": "Synthetic M1 issuer current expiry", "token_hash": hash, "token_prefix": "synthetic", "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
				issuer := auth.WithAuthenticatedPrincipal(fixture.stage.reviewerCtx, auth.AuthenticatedPrincipal{UserID: fixture.stage.reviewer, Kind: auth.CredentialPAT, PATTokenHash: hash})
				issuer = auth.WithPasswordSession(issuer, auth.PasswordSession{UserID: fixture.stage.reviewer, Kind: "pat", Version: 1})
				binding := stageCompletionM1ForLifecycle(t, fixture, issuer, params)
				params.Completion = &binding
				fixture.fx.Exec(t, `UPDATE personal_access_token SET expires_at=$2 WHERE id=$1`, pat, until)
			} else {
				credential := completionRequestCredential(ctx)
				credential.Principal.ExpiresAt = until
				ctx = auth.WithAuthenticatedPrincipal(ctx, credential.Principal)
			}
			before := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID)
			hits := 0
			fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, future: until.Add(time.Second)}
			receipt, err := fixture.svc.ChangeLifecycle(ctx, params)
			fixture.svc.TxStarter = fixture.pool
			if hits == 0 || err == nil {
				t.Fatalf("expired current %s passed the post-receipt clock: hits=%d receipt=%+v err=%v", subject, hits, receipt, err)
			}
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if readErr != nil || issue.Status == "done" || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID) != before {
				t.Fatalf("late current %s expiry retained writer effects: %+v %v", subject, issue, readErr)
			}
		})
	}
}

func TestStageCompletionSourceRevocationBothCommitOrders(t *testing.T) {
	for _, revokeFirst := range []bool{true, false} {
		name := "completion_first"
		if revokeFirst {
			name = "revocation_first"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newStageCompletionFixture(t, stageCompletionIndependentArtifact)
			submission := stageCompletionSubmit(t, fixture)
			params := stageCompletionAcceptParams(t, fixture, submission)
			member, err := fixture.svc.Queries.GetMemberByUserAndWorkspace(fixture.ctx, db.GetMemberByUserAndWorkspaceParams{UserID: util.MustParseUUID(fixture.stage.reviewer), WorkspaceID: util.MustParseUUID(fixture.workspace)})
			if err != nil {
				t.Fatal(err)
			}
			hits := 0
			if revokeFirst {
				competitor, err := fixture.pool.Begin(fixture.stage.reviewerCtx)
				if err != nil {
					t.Fatal(err)
				}
				defer competitor.Rollback(fixture.stage.reviewerCtx)
				if err = db.New(competitor).DeleteMember(fixture.stage.reviewerCtx, member.ID); err != nil {
					t.Fatal(err)
				}
				if err = competitor.Commit(fixture.stage.reviewerCtx); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, owner pgx.Tx) error {
					competitor, err := fixture.pool.Begin(ctx)
					if err != nil {
						return err
					}
					defer competitor.Rollback(ctx)
					var ownerPID, competitorPID int32
					if err = owner.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&ownerPID); err != nil {
						return err
					}
					if err = competitor.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&competitorPID); err != nil || ownerPID == competitorPID {
						t.Fatalf("source race needs distinct actual backends: owner=%d competitor=%d err=%v", ownerPID, competitorPID, err)
					}
					if _, err = competitor.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
						return err
					}
					err = db.New(competitor).DeleteMember(ctx, member.ID)
					var conflict *pgconn.PgError
					if !errors.As(err, &conflict) || conflict.Code != "55P03" {
						t.Fatalf("original member revocation overtook completion: err=%v", err)
					}
					return nil
				}}
			}
			receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
			fixture.svc.TxStarter = fixture.pool
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if revokeFirst {
				if err == nil || issue.Status == "done" || readErr != nil {
					t.Fatalf("committed source revocation permitted completion: %+v %+v %v %v", receipt, issue, err, readErr)
				}
				fixture.fx.Member(t, fixture.workspace, fixture.stage.reviewer, "admin")
				fresh := stageCompletionAcceptParams(t, fixture, submission)
				if replay, replayErr := fixture.svc.ChangeLifecycle(fixture.ctx, fresh); replayErr == nil || replay.Code != "stage_incomplete" {
					t.Fatalf("rejoined source member revived the old signature: %+v %v", replay, replayErr)
				}
			} else {
				if hits != 1 || err != nil || receipt.Code != "completed" || readErr != nil || issue.Status != "done" {
					t.Fatalf("completion-first source race: hits=%d receipt=%+v issue=%+v err=%v read=%v", hits, receipt, issue, err, readErr)
				}
				if err = fixture.svc.Queries.DeleteMember(fixture.ctx, member.ID); err != nil {
					t.Fatalf("committed completion leaked its source lock: %v", err)
				}
			}
		})
	}
}

func TestStageCompletionPrivateGuardTransactionAndSingleUse(t *testing.T) {
	fixture := newStageCompletionFixture(t)
	submission := stageCompletionSubmit(t, fixture)
	params := stageCompletionAcceptParams(t, fixture, submission)
	transaction, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(fixture.ctx)
	raw, _ := json.Marshal(params)
	var prepared DeliveryPrepareResult
	var after db.Issue
	var result DeliveryWriterResult
	_, err = fixture.svc.workflowInTx(fixture.ctx, transaction, util.MustParseUUID(fixture.creator), pgtype.UUID{}, params.CollaborationWorkflowRequest, "complete", hexHash(raw), func(writer *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		candidate := writer.access.Issue
		candidate.Status = params.TargetStatus
		target := completionCollaborationTargetFromWorkflow(writer, params)
		var err error
		prepared, err = PrepareDeliveryMutationInTx(fixture.ctx, transaction, DeliveryMutationIntent{Before: writer.access.Issue, Candidate: candidate, Credential: completionRequestCredential(fixture.ctx), Collaboration: &target, StageExecutionSources: fixture.source, StageOperationID: writer.operation.ID})
		if err != nil || prepared.State != DeliveryNotApplicable || prepared.stage == nil {
			t.Fatalf("actual stage-only preparation: %+v %v", prepared, err)
		}
		competitor, err := fixture.pool.Begin(fixture.ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if wrongErr := FinishStageMutationInTx(fixture.ctx, competitor, prepared, candidate, DeliveryWriterResult{CollaborationOperationID: writer.operation.ID}); wrongErr == nil {
			t.Fatal("guard passed in another real pgx transaction")
		}
		if err = competitor.Rollback(fixture.ctx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if refusal, err := writer.prepareCompletion(fixture.ctx, params); refusal != nil || err != nil {
			t.Fatalf("actual acceptance preparation: %+v %v", refusal, err)
		}
		receipt, err := writer.changeLifecycle(fixture.ctx, "complete", params.TargetStatus)
		after = writer.access.Issue
		result = DeliveryWriterResult{CollaborationOperationID: writer.operation.ID, CollaborationRevision: writer.access.Revision, SubmissionID: submission}
		if unfinishedErr := FinishStageMutationInTx(fixture.ctx, transaction, prepared, after, result); unfinishedErr == nil || prepared.stage.finished {
			t.Fatal("unfinalized original receipt passed or consumed its guard")
		}
		writer.stagePrepared, writer.stageWriterResult = prepared, result
		return receipt, err
	})
	if err != nil || !prepared.stage.finished {
		t.Fatalf("real workflow could not finalize its own private guard: %v", err)
	}
	if reused := FinishStageMutationInTx(fixture.ctx, transaction, prepared, after, result); reused == nil {
		t.Fatal("an actually finalized guard was consumed twice")
	}
}

func TestStageCompletionExecutionTrustBothCommitOrders(t *testing.T) {
	for _, revokeFirst := range []bool{true, false} {
		name := "completion_first"
		if revokeFirst {
			name = "trust_first"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newStageCompletionFixture(t)
			submission := stageCompletionSubmit(t, fixture)
			params := stageCompletionAcceptParams(t, fixture, submission)
			installation := fixture.source.target.Source.IntegrationID
			update := `UPDATE github_installation SET trust_revision=trust_revision+1 WHERE id=$1`
			hits := 0
			if revokeFirst {
				competitor, err := fixture.pool.Begin(fixture.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer competitor.Rollback(fixture.ctx)
				if _, err = competitor.Exec(fixture.ctx, update, installation); err != nil {
					t.Fatal(err)
				}
				if err = competitor.Commit(fixture.ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, owner pgx.Tx) error {
					competitor, err := fixture.pool.Begin(ctx)
					if err != nil {
						return err
					}
					defer competitor.Rollback(ctx)
					var ownerPID, competitorPID int32
					if err = owner.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&ownerPID); err != nil {
						return err
					}
					if err = competitor.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&competitorPID); err != nil || ownerPID == competitorPID {
						t.Fatalf("trust race used the same backend: owner=%d competitor=%d err=%v", ownerPID, competitorPID, err)
					}
					if _, err = competitor.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
						return err
					}
					_, err = competitor.Exec(ctx, update, installation)
					var conflict *pgconn.PgError
					if !errors.As(err, &conflict) || conflict.Code != "55P03" {
						t.Fatalf("trust rotation overtook the owning completion: %v", err)
					}
					return nil
				}}
			}
			receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
			fixture.svc.TxStarter = fixture.pool
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if revokeFirst {
				if err == nil || readErr != nil || issue.Status == "done" {
					t.Fatalf("committed trust rotation permitted completion: %+v %+v %v %v", receipt, issue, err, readErr)
				}
			} else if hits != 1 || err != nil || readErr != nil || issue.Status != "done" || receipt.Code != "completed" {
				t.Fatalf("completion-first trust race: hits=%d receipt=%+v issue=%+v err=%v read=%v", hits, receipt, issue, err, readErr)
			}
			if !revokeFirst {
				second, err := fixture.pool.Begin(fixture.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer second.Rollback(fixture.ctx)
				if _, err = second.Exec(fixture.ctx, update, installation); err != nil {
					t.Fatalf("trust update remained blocked after the owner commit: %v", err)
				}
				if err = second.Commit(fixture.ctx); err != nil {
					t.Fatal(err)
				}
				events := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID)
				if replay, err := fixture.svc.ChangeLifecycle(fixture.ctx, params); err != nil || replay.Code != "completed" || replay.CollaborationRevision != receipt.CollaborationRevision || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID) != events {
					t.Fatalf("post-commit trust rotation turned history into a new consumption: %+v %v", replay, err)
				}
			}
		})
	}
}

func TestStageCompletionRollbackMetricBothCommitOrders(t *testing.T) {
	for _, metricFirst := range []bool{true, false} {
		name := "completion_first"
		if metricFirst {
			name = "metric_first"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newStageCompletionFixture(t)
			release, err := fixture.svc.Queries.GetCollaborationStageReviewHead(fixture.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: fixture.issue.Issue.ID, CandidateID: fixture.candidate.CandidateID, Step: "release"})
			if err != nil {
				t.Fatal(err)
			}
			intent, err := fixture.svc.Queries.GetStageExecutionIntentByApproval(fixture.ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: fixture.issue.Issue.ID, ApprovalDecisionID: release.DecisionID})
			if err != nil {
				t.Fatal(err)
			}
			fact, err := fixture.svc.Queries.GetStageExecutionTerminal(fixture.ctx, intent.IntentID)
			if err != nil {
				t.Fatal(err)
			}
			metric := fixture.metric(t, util.UUIDToString(fact.ObservationID), 1, "anomaly")
			rollback := fixture.rollback(t, util.UUIDToString(release.DecisionID), util.UUIDToString(fact.ObservationID), metric)
			if result, err := fixture.svc.PrepareStageExecution(context.Background(), rollback, fixture.source.consumer(rollback)); err != nil || result.State != "succeeded" {
				t.Fatalf("controlled prior rollback: %+v %v", result, err)
			}
			submission := stageCompletionSubmit(t, fixture)
			params := stageCompletionAcceptParams(t, fixture, submission)
			row, err := fixture.svc.Queries.GetStageRollbackObservation(fixture.ctx, util.MustParseUUID(metric.ObservationID))
			if err != nil {
				t.Fatal(err)
			}
			newMetric, err := decodeStageMetric(row)
			if err != nil {
				t.Fatal(err)
			}
			newMetric.NativeID, newMetric.NativeVersion, newMetric.Condition = uuid.NewString(), "2", "healthy"
			proof := fixture.source.proof(stageSourceProofFixture{Kind: "metric", Metric: newMetric})
			hits := 0
			if metricFirst {
				if _, err = fixture.svc.RecordStageRollbackObservation(context.Background(), fact.ObservationID, proof); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.svc.TxStarter = stageCompletionLateWriteStarter{TxStarter: fixture.pool, hits: &hits, mutate: func(ctx context.Context, owner pgx.Tx) error {
					// The recorder owns a separate real transaction. Its nonblocking
					// stream fence must refuse while completion retains the old fact.
					competitor := NewCollaborationService(fixture.svc.Queries, fixture.pool, fixture.svc.Issues)
					competitor.StageExecutionSources = fixture.source
					if _, err := competitor.RecordStageRollbackObservation(ctx, fact.ObservationID, proof); !collaborationErrorCode(err, "stage_rollback_observation_unavailable") {
						t.Fatalf("new metric overtook completion's original stream fence: %v", err)
					}
					return nil
				}}
			}
			receipt, err := fixture.svc.ChangeLifecycle(fixture.ctx, params)
			fixture.svc.TxStarter = fixture.pool
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.ctx, fixture.issue.Issue.ID)
			if metricFirst {
				if err == nil || readErr != nil || issue.Status == "done" {
					t.Fatalf("new healthy metric permitted old rollback completion: %+v %+v %v %v", receipt, issue, err, readErr)
				}
			} else if hits != 1 || err != nil || readErr != nil || issue.Status != "done" || receipt.Code != "completed" {
				t.Fatalf("completion-first metric race: hits=%d receipt=%+v issue=%+v err=%v read=%v", hits, receipt, issue, err, readErr)
			}
			if !metricFirst {
				if recorded, err := fixture.svc.RecordStageRollbackObservation(context.Background(), fact.ObservationID, proof); err != nil || recorded.ObservationVersion != "2" {
					t.Fatalf("new metric did not actually commit after completion: %+v %v", recorded, err)
				}
				events := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID)
				if replay, err := fixture.svc.ChangeLifecycle(fixture.ctx, params); err != nil || replay.Code != "completed" || replay.CollaborationRevision != receipt.CollaborationRevision || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID) != events {
					t.Fatalf("post-commit metric turned history into a new consumption: %+v %v", replay, err)
				}
			}
		})
	}
}

func TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt(t *testing.T) {
	for _, refusal := range []string{"wrong_consumer", "agent_actor", "issuer_revoked", "consumer_revoked"} {
		t.Run(refusal, func(t *testing.T) {
			fixture := newStageCompletionFixture(t, stageCompletionIndependentArtifact)
			stageCompletionM1Policy(t, fixture, "enforce")
			submission := stageCompletionSubmit(t, fixture)
			member, err := fixture.svc.Queries.GetMemberByUserAndWorkspace(fixture.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(fixture.workspace), UserID: util.MustParseUUID(fixture.creator)})
			if err != nil {
				t.Fatal(err)
			}
			if err = fixture.svc.Queries.DeleteMember(fixture.ctx, member.ID); err != nil {
				t.Fatal(err)
			}
			params := stageCompletionAcceptParams(t, fixture, submission)
			issuer := fixture.stage.reviewerCtx
			var pat string
			if refusal == "issuer_revoked" {
				hash := hexHash([]byte(uuid.NewString()))
				pat = fixture.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": fixture.stage.reviewer, "name": "Synthetic separately revoked M1 issuer", "token_hash": hash, "token_prefix": "synthetic", "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
				issuer = auth.WithAuthenticatedPrincipal(issuer, auth.AuthenticatedPrincipal{UserID: fixture.stage.reviewer, Kind: auth.CredentialPAT, PATTokenHash: hash})
				issuer = auth.WithPasswordSession(issuer, auth.PasswordSession{UserID: fixture.stage.reviewer, Kind: "pat", Version: 1})
			}
			binding := stageCompletionM1ForLifecycle(t, fixture, issuer, params)
			params.Completion = &binding
			read, err := fixture.pool.Begin(fixture.ctx)
			if err != nil {
				t.Fatal(err)
			}
			view, readErr := GetDeliveryCompletionAuthorizationInTx(fixture.ctx, read, ArtifactAccess{Credential: completionRequestCredential(fixture.ctx), RequestedWorkspaceID: fixture.workspace}, util.MustParseUUID(binding.AuthorizationID))
			if err = read.Rollback(fixture.ctx); err != nil {
				t.Fatal(err)
			}
			if readErr != nil || view.AuthorizationID != binding.AuthorizationID || view.OperationID != params.RequestID || view.IssueID != util.UUIDToString(fixture.issue.Issue.ID) {
				t.Fatalf("outside consumer could not recover its minimal original grant: %+v %v", view, readErr)
			}
			encoded, _ := json.Marshal(view)
			for _, privateField := range []string{"artifact", "spec_digest", "source_revision", "issuer", "request_json", "token_hash"} {
				if strings.Contains(string(encoded), privateField) {
					t.Fatalf("minimal consumer receipt exposed %s", privateField)
				}
			}
			ctx := fixture.ctx
			switch refusal {
			case "wrong_consumer":
				ctx = fixture.stage.handlerCtx
			case "agent_actor":
				principal := completionRequestCredential(ctx).Principal
				principal.AgentActor = true
				ctx = auth.WithAuthenticatedPrincipal(ctx, principal)
			case "issuer_revoked":
				fixture.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, pat)
			case "consumer_revoked":
				revocation, err := fixture.pool.Begin(fixture.stage.reviewerCtx)
				if err != nil {
					t.Fatal(err)
				}
				defer revocation.Rollback(fixture.stage.reviewerCtx)
				if _, err = db.New(revocation).LockCollaborationUsers(fixture.stage.reviewerCtx, []pgtype.UUID{util.MustParseUUID(fixture.creator)}); err != nil {
					t.Fatal(err)
				}
				if _, err = auth.RevokePasswordCredentials(fixture.stage.reviewerCtx, revocation, util.MustParseUUID(fixture.creator), "admin:"+uuid.NewString()); err != nil {
					t.Fatal(err)
				}
				if err = revocation.Commit(fixture.stage.reviewerCtx); err != nil {
					t.Fatal(err)
				}
			}
			beforeEvents := fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.issue.Issue.ID)
			receipt, err := fixture.svc.ChangeLifecycle(ctx, params)
			issue, readErr := fixture.svc.Queries.GetIssue(fixture.stage.reviewerCtx, fixture.issue.Issue.ID)
			if err == nil || readErr != nil || issue.Status == "done" || receipt.DeliveryConfirmation != nil || fixture.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, binding.AuthorizationID) != 0 || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID) != beforeEvents {
				t.Fatalf("independent %s authority permitted effects: receipt=%+v issue=%+v err=%v read=%v", refusal, receipt, issue, err, readErr)
			}
			if refusal == "issuer_revoked" {
				original, recoveryErr := fixture.svc.GetWorkflowOperation(fixture.ctx, issue.ID, params.RequestID)
				if recoveryErr != nil || original.Code != receipt.Code || original.Status != receipt.Status || original.DeliveryConfirmation != nil {
					t.Fatalf("original refused-key recovery fabricated a confirmation: %+v %v", original, recoveryErr)
				}
				fixture.fx.Exec(t, `UPDATE personal_access_token SET revoked=false WHERE id=$1`, pat)
				replay, replayErr := fixture.svc.ChangeLifecycle(fixture.ctx, params)
				if replayErr == nil || replay.Code != receipt.Code || replay.Status != receipt.Status || replay.DeliveryConfirmation != nil || fixture.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, binding.AuthorizationID) != 0 || fixture.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue.ID) != beforeEvents {
					t.Fatalf("a restored issuer transformed the old refusal into completion: %+v %v", replay, replayErr)
				}
			}
		})
	}
}
