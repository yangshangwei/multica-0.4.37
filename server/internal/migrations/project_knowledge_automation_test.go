package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeAutomationFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*_project_knowledge_automation*.up.sql")
	if err != nil || len(files) != 16 {
		t.Fatalf("automation schema and fifteen concurrent index migrations: %d %v", len(files), err)
	}
	return files
}

func knowledgeAutomationUpgrade(t *testing.T, pool *pgxpool.Pool, fx *testutil.Fixture) {
	t.Helper()
	fx.Exec(t, `CREATE TABLE IF NOT EXISTS collaboration_operation (LIKE public.collaboration_operation INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
	fx.Exec(t, `CREATE TABLE IF NOT EXISTS collaboration_run_event (LIKE public.collaboration_run_event INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
	for _, path := range knowledgeAutomationFiles(t) {
		knowledgeApply(t, pool, path)
	}
	knowledgeApply(t, pool, "../../migrations/860_knowledge_automation_execution_policy.up.sql")
}

func knowledgeAutomationSchema(t *testing.T) (*pgxpool.Pool, *testutil.Fixture) {
	t.Helper()
	pool, fx := knowledgeSchema(t)
	knowledgeAutomationUpgrade(t, pool, fx)
	return pool, fx
}

func knowledgeAutomationClaim(t *testing.T, q *db.Queries) db.CollaborationKnowledgeJob {
	t.Helper()
	ctx := context.Background()
	params := db.CreateKnowledgeAutomationJobParams{
		ID: knowledgeUUID(), SourceWorkspaceID: knowledgeUUID(), SourceIssueID: knowledgeUUID(), ProjectID: knowledgeUUID(),
		SubmissionID: knowledgeUUID(), DeliveryEventSeq: 3, AcceptedEventSeq: 4, ValidatedCloseEpoch: 2,
		SourceReaderUserID: knowledgeUUID(), BuiltinRuleVersion: "2026-11-14-v1", MaxCandidates: 3,
	}
	job, err := q.CreateKnowledgeAutomationJob(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	params.ID = knowledgeUUID()
	if _, err = q.CreateKnowledgeAutomationJob(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("same acceptance allocated another job: %v", err)
	}
	claimed, err := q.ClaimKnowledgeAutomationJob(ctx, knowledgeUUID())
	if err != nil || claimed.ID != job.ID || claimed.LeaseGeneration != 1 || !claimed.LeaseToken.Valid {
		t.Fatalf("claim identity: %+v %v", claimed, err)
	}
	return claimed
}

func knowledgeAutomationBatch(t *testing.T, q *db.Queries, job db.CollaborationKnowledgeJob) db.CollaborationKnowledgeBatch {
	t.Helper()
	batch, err := q.CreateKnowledgeAutomationBatch(context.Background(), db.CreateKnowledgeAutomationBatchParams{
		ID: knowledgeUUID(), JobID: job.ID, BatchIndex: 6, Intent: "create", PredecessorIds: []pgtype.UUID{},
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration,
	})
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestKnowledgeAutomationMigrationPreservesLegacyAndVersionBoundary(t *testing.T) {
	pool, fx := knowledgeSchema(t)
	id := knowledgeCandidate(t, fx)
	fx.Exec(t, `UPDATE collaboration_knowledge_candidate SET state='approved',state_revision=2,approved_by_user_id=proposed_by_user_id,
		approved_at=clock_timestamp(),published_target_revision=1,last_decided_by_user_id=proposed_by_user_id,last_decided_at=clock_timestamp() WHERE id=$1`, id)
	var before string
	fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_knowledge_candidate c WHERE id=$1`, id).Scan(&before)
	knowledgeAutomationUpgrade(t, pool, fx)
	var after string
	fx.QueryRow(t, `SELECT (to_jsonb(c)-ARRAY['proposal_origin','decision_origin','approval_origin','automation_job_id','automation_policy_revision','automation_rule_version','approval_job_id','last_decision_job_id'])::text FROM collaboration_knowledge_candidate c WHERE id=$1`, id).Scan(&after)
	if after != before || fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE proposal_origin='human' AND decision_origin='human' AND approval_origin='human' AND automation_job_id IS NULL`) != 1 {
		t.Fatal("migration rewrote old approval or invented system attribution")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE conrelid='collaboration_knowledge_candidate'::regclass AND conname='collaboration_knowledge_candidate_check5' AND pg_get_constraintdef(oid) LIKE '%expires_at%'`) != 1 {
		t.Fatal("original expiry constraint identity was lost")
	}
	if fx.Count(t, `SELECT count(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND contype IN ('f','p','u')`) != 0 {
		t.Fatal("migration added foreign keys or inline indexes")
	}
	for _, file := range knowledgeAutomationFiles(t) {
		knowledgeApply(t, pool, file)
	}
	files := knowledgeAutomationFiles(t)
	for i := len(files) - 1; i >= 0; i-- {
		knowledgeApply(t, pool, strings.TrimSuffix(files[i], ".up.sql")+".down.sql")
	}
	fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_knowledge_candidate c WHERE id=$1`, id).Scan(&after)
	if before != after {
		t.Fatal("empty automation downgrade changed old retained human history")
	}
	knowledgeAutomationUpgrade(t, pool, fx)
	fx.Insert(t, "agent_task_queue", testutil.Cols{"knowledge_context_version": 2, "knowledge_delivery": `{"version":2}`})
	knowledgeError(t, pool, "23514", `INSERT INTO agent_task_queue (knowledge_context_version) VALUES (3)`)
	knowledgeError(t, pool, "23514", `INSERT INTO agent_task_queue (knowledge_delivery) VALUES ('{}')`)
	for _, file := range files {
		raw, err := os.ReadFile(strings.TrimSuffix(file, ".up.sql") + ".down.sql")
		if err != nil {
			t.Fatal(err)
		}
		knowledgeError(t, pool, "55000", string(raw))
	}
	if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE knowledge_context_version=2`) != 1 {
		t.Fatal("refused downgrade altered v2 history")
	}
}

func TestKnowledgeAutomationPolicyAndCompleteBatchInventory(t *testing.T) {
	pool, fx := knowledgeAutomationSchema(t)
	q, ctx := db.New(pool), context.Background()
	project, workspace, actor := knowledgeUUID(), knowledgeUUID(), knowledgeUUID()
	if _, err := q.GetProjectKnowledgePolicy(ctx, db.GetProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing override must remain absent: %v", err)
	}
	policy, err := q.CreateProjectKnowledgePolicy(ctx, db.CreateProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace,
		IsEnabled: false, MaxCandidates: 2, BuiltinRuleVersion: "2026-11-14-v1", ChangedByUserID: actor})
	if err != nil || policy.Revision != 1 || policy.IsEnabled {
		t.Fatalf("explicit disabled override: %+v %v", policy, err)
	}
	knowledgeApply(t, pool, knowledgeAutomationFiles(t)[0])
	policy, err = q.GetProjectKnowledgePolicy(ctx, db.GetProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace})
	if err != nil || policy.IsEnabled || policy.Revision != 1 {
		t.Fatal("reapplied default replaced explicit false")
	}
	change := db.UpdateProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace, ExpectedRevision: 1,
		IsEnabled: true, MaxCandidates: 3, BuiltinRuleVersion: "2026-11-14-v1", ChangedByUserID: actor}
	if _, err = q.UpdateProjectKnowledgePolicy(ctx, change); err != nil {
		t.Fatal(err)
	}
	if _, err = q.UpdateProjectKnowledgePolicy(ctx, change); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale policy revision changed settings: %v", err)
	}
	fx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "project", "scope_id": project, "actor_user_id": actor,
		"request_id": "first-policy-request", "kind": "knowledge_policy", "payload_hash": strings.Repeat("a", 64), "result": `{"revision":"1"}`})
	fx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "project", "scope_id": project, "actor_user_id": actor,
		"request_id": "second-policy-request", "kind": "knowledge_policy", "payload_hash": strings.Repeat("b", 64), "result": `{"revision":"2"}`})
	if fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='project' AND scope_id=$1`, project) != 2 {
		t.Fatal("older policy request receipt was overwritten")
	}
	job := knowledgeAutomationClaim(t, q)
	manifest := []byte(`["review-0","review-1","review-2","review-3","review-4","review-5","create"]`)
	if _, err = q.FreezeKnowledgeAutomationBatches(ctx, db.FreezeKnowledgeAutomationBatchesParams{ID: job.ID, BatchManifest: manifest, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration}); err != nil {
		t.Fatal(err)
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_job SET batch_manifest='["truncated"]' WHERE id=$1`, job.ID)
	for index := int16(0); index < 6; index++ {
		ids := []pgtype.UUID{knowledgeUUID(), knowledgeUUID(), knowledgeUUID()}
		if index == 5 {
			ids = ids[:1]
		}
		if _, err = q.CreateKnowledgeAutomationBatch(ctx, db.CreateKnowledgeAutomationBatchParams{ID: knowledgeUUID(), JobID: job.ID,
			BatchIndex: index, Intent: "revalidate", PredecessorIds: ids, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration}); err != nil {
			t.Fatal(err)
		}
	}
	knowledgeAutomationBatch(t, q, job)
	batches, err := q.ListKnowledgeAutomationBatches(ctx, job.ID)
	count := 0
	for _, batch := range batches {
		count += len(batch.PredecessorIds)
	}
	if err != nil || len(batches) != 7 || count != 16 {
		t.Fatalf("complete inventory must retain all16 plus one create batch: %d/%d %v", len(batches), count, err)
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_batch SET predecessor_ids='{}' WHERE job_id=$1 AND batch_index=0`, job.ID)
	knowledgeError(t, pool, "23514", `INSERT INTO collaboration_knowledge_batch (job_id,batch_index,intent) VALUES ($1,7,'create')`, job.ID)
	knowledgeError(t, pool, "23514", `DELETE FROM collaboration_knowledge_job WHERE id=$1`, job.ID)
}

func TestKnowledgeAutomationAttemptsAndHumanDecisionRemainAuthoritative(t *testing.T) {
	pool, fx := knowledgeAutomationSchema(t)
	q, ctx := db.New(pool), context.Background()
	job := knowledgeAutomationClaim(t, q)
	batch := knowledgeAutomationBatch(t, q, job)
	closure := []byte(`[{"source":"primary"},{"source":"private-comparison-not-in-related-refs"}]`)
	dependencies := []byte(`[{"source":"new-accepted-delivery"}]`)
	var generated, checked db.CollaborationKnowledgeAttempt
	for _, stage := range []string{"generating", "checking"} {
		attempt, err := q.CreateKnowledgeModelAttempt(ctx, db.CreateKnowledgeModelAttemptParams{ID: knowledgeUUID(), JobID: job.ID, BatchID: batch.ID,
			ExecutionRevision: 1, Stage: stage, AttemptNumber: 1, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration,
			ConfigurationFingerprint: strings.Repeat("a", 64), RequestedModel: "fake-model", SourceSnapshotSha256: strings.Repeat("b", 64),
			TargetSnapshotSha256: strings.Repeat("c", 64), RequestSha256: strings.Repeat("d", 64), InputSourceClosure: closure, SupportDependencies: dependencies})
		if err != nil {
			t.Fatal(err)
		}
		knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_attempt SET input_source_closure='[]' WHERE id=$1`, attempt.ID)
		attempt, err = q.FinishKnowledgeModelAttempt(ctx, db.FinishKnowledgeModelAttemptParams{ID: attempt.ID, JobID: job.ID, ExecutionRevision: 1,
			LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, State: "succeeded", ResponseJson: []byte(`{"related_refs":[]}`),
			ActualModel: pgtype.Text{String: "fake-returned-model", Valid: true}, Usage: []byte(`{"status":"known","input_tokens":0,"output_tokens":0}`)})
		if err != nil {
			t.Fatal(err)
		}
		knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_attempt SET usage='{"status":"unknown"}' WHERE id=$1`, attempt.ID)
		if stage == "generating" {
			generated = attempt
		} else {
			checked = attempt
		}
	}
	if _, err := q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: 1,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "settling", GenerationAttemptID: generated.ID, CheckAttemptID: checked.ID}); err != nil {
		t.Fatal(err)
	}
	body := "Amounts are expressed in cents."
	candidate, err := q.CreateAutomatedKnowledgeCandidate(ctx, db.CreateAutomatedKnowledgeCandidateParams{ID: knowledgeUUID(), JobID: job.ID,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Content: body, Summary: "Amount unit", ContentSha256: protocol.CollaborationKnowledgeHash(body),
		Sources: []byte(`[{"kind":"delivery"}]`), Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue"]}`)})
	if err != nil || candidate.ProposedByUserID.Valid || candidate.ProposalOrigin != "automation" || candidate.AutomationJobID != job.ID {
		t.Fatalf("system proposal impersonated the acceptance reader: %+v %v", candidate, err)
	}
	evaluation, err := q.CreateKnowledgeEvaluation(ctx, db.CreateKnowledgeEvaluationParams{ID: knowledgeUUID(), JobID: job.ID, BatchID: batch.ID,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, CandidateID: candidate.ID, CandidateVersion: candidate.Version,
		CandidateContentSha256: candidate.ContentSha256, Intent: "create", Verdict: "pass", ReasonCode: "rules_passed",
		SupportDependencies: dependencies, ModelResult: []byte(`{"related_refs":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err = json.Unmarshal(evaluation.InputSourceClosure, &actual); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(closure, &expected); err != nil {
		t.Fatal(err)
	}
	actualJSON, _ := json.Marshal(actual)
	expectedJSON, _ := json.Marshal(expected)
	if string(actualJSON) != string(expectedJSON) {
		t.Fatal("evaluation lost an actual input source omitted from model related_refs")
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_evaluation SET verdict='duplicate' WHERE id=$1`, evaluation.ID)
	knowledgeError(t, pool, "23514", `DELETE FROM collaboration_knowledge_evaluation WHERE id=$1`, evaluation.ID)
	candidate, err = q.ApproveAutomatedKnowledgeCandidate(ctx, db.ApproveAutomatedKnowledgeCandidateParams{ID: candidate.ID, JobID: job.ID,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, EvaluationID: evaluation.ID, ExpectedVersion: 1, ExpectedStateRevision: 1,
		ExpectedContentSha256: candidate.ContentSha256, TargetRevision: pgtype.Int8{Int64: 1, Valid: true}})
	if err != nil || candidate.ApprovedByUserID.Valid || candidate.LastDecidedByUserID.Valid || candidate.ApprovalOrigin != "automation" {
		t.Fatalf("automatic approval invented a human: %+v %v", candidate, err)
	}
	validityParams := db.SetKnowledgeValidityParams{CandidateID: candidate.ID, EvaluationID: evaluation.ID, ExpectedStateRevision: 2}
	validity, err := q.SetKnowledgeValidity(ctx, validityParams)
	if err != nil || validity.Revision != 1 || validity.AcceptedEventSeq != job.AcceptedEventSeq {
		t.Fatalf("exact accepted validity missing: %+v %v", validity, err)
	}
	if _, err = q.SetKnowledgeValidity(ctx, validityParams); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("same proof must recover, not advance its head: %v", err)
	}
	human := knowledgeUUID()
	withdrawn, err := q.WithdrawKnowledgeCandidate(ctx, db.WithdrawKnowledgeCandidateParams{ID: candidate.ID, ExpectedStateRevision: 2,
		ActorID: human, Reason: pgtype.Text{String: "Human correction", Valid: true}})
	if err != nil || withdrawn.DecisionOrigin != "human" || withdrawn.LastDecidedByUserID != human || withdrawn.ApprovalOrigin != "automation" || withdrawn.ApprovalJobID != job.ID {
		t.Fatalf("human withdrawal lost original system approval history: %+v %v", withdrawn, err)
	}
	disposition := db.KnowledgeAutomationHasHumanDispositionParams{SourceIssueID: job.SourceIssueID, ProjectID: job.ProjectID,
		WorkspaceID: job.SourceWorkspaceID, Content: body, Applicability: candidate.Applicability}
	if blocked, err := q.KnowledgeAutomationHasHumanDisposition(ctx, disposition); err != nil || !blocked {
		t.Fatalf("fresh-ID equivalent could bypass human withdrawal: %t %v", blocked, err)
	}
	disposition.Content = "Amounts are not expressed in cents."
	if blocked, err := q.KnowledgeAutomationHasHumanDisposition(ctx, disposition); err != nil || blocked {
		t.Fatalf("human-disposition check conflated a different statement: %t %v", blocked, err)
	}
	validityParams.ExpectedRevision, validityParams.ExpectedStateRevision = 1, 3
	if _, err = q.SetKnowledgeValidity(ctx, validityParams); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("late proof recovered a withdrawn candidate: %v", err)
	}
	if _, err = q.ReleaseKnowledgeAutomationJob(ctx, db.ReleaseKnowledgeAutomationJobParams{ID: job.ID, ExecutionRevision: 1,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "completed", Result: []byte(`{"saved":1}`),
		NextAttemptAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_job SET result='{}' WHERE id=$1`, job.ID)
	if fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_job WHERE id=$1 AND phase='completed' AND lease_token IS NULL AND lease_expires_at IS NULL`, job.ID) != 1 {
		t.Fatal("terminal result and lease release did not commit together")
	}
}

func TestKnowledgeAutomationUnknownAttemptCannotBeRewritten(t *testing.T) {
	pool, _ := knowledgeAutomationSchema(t)
	q, ctx := db.New(pool), context.Background()
	job := knowledgeAutomationClaim(t, q)
	batch := knowledgeAutomationBatch(t, q, job)
	attempt, err := q.CreateKnowledgeModelAttempt(ctx, db.CreateKnowledgeModelAttemptParams{ID: knowledgeUUID(), JobID: job.ID, BatchID: batch.ID,
		ExecutionRevision: 1, Stage: "generating", AttemptNumber: 1, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration,
		ConfigurationFingerprint: strings.Repeat("a", 64), RequestedModel: "fake", SourceSnapshotSha256: strings.Repeat("b", 64),
		TargetSnapshotSha256: strings.Repeat("c", 64), RequestSha256: strings.Repeat("d", 64), InputSourceClosure: []byte(`[]`), SupportDependencies: []byte(`[]`)})
	if err != nil {
		t.Fatal(err)
	}
	finish := db.FinishKnowledgeModelAttemptParams{ID: attempt.ID, JobID: job.ID, ExecutionRevision: 1,
		LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, State: "uncertain", Usage: []byte(`{"status":"unknown"}`),
		ReasonCode: pgtype.Text{String: "model_attempt_uncertain", Valid: true}}
	if _, err = q.FinishKnowledgeModelAttempt(ctx, finish); err != nil {
		t.Fatal(err)
	}
	finish.State, finish.ResponseJson = "succeeded", []byte(`{}`)
	if _, err = q.FinishKnowledgeModelAttempt(ctx, finish); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("late result overwrote an uncertain attempt: %v", err)
	}
	knowledgeError(t, pool, "23514", `UPDATE collaboration_knowledge_attempt SET state='running',finished_at=NULL WHERE id=$1`, attempt.ID)
	knowledgeError(t, pool, "23514", `DELETE FROM collaboration_knowledge_attempt WHERE id=$1`, attempt.ID)
}

func TestKnowledgeAutomationRevalidationInventoryIncludesExpired(t *testing.T) {
	pool, fx := knowledgeAutomationSchema(t)
	q := db.New(pool)
	job := knowledgeAutomationClaim(t, q)
	for index := 0; index < 4; index++ {
		id := knowledgeUUID()
		body := "Retained automatic candidate " + string(rune('a'+index))
		fx.Insert(t, "collaboration_knowledge_candidate", testutil.Cols{
			"id": id, "lineage_id": id, "version": 1, "source_workspace_id": job.SourceWorkspaceID,
			"source_issue_id": job.SourceIssueID, "target_kind": "project", "target_workspace_id": job.SourceWorkspaceID, "target_project_id": job.ProjectID,
			"content": body, "summary": "retained", "content_sha256": protocol.CollaborationKnowledgeHash(body),
			"sources": `[{"kind":"delivery"}]`, "evidence": `[]`, "applicability": `{"surfaces":["issue"]}`,
			"proposed_at": time.Now().Add(-2 * time.Hour), "expires_at": time.Now().Add(-time.Hour),
			"proposal_origin": "automation", "automation_job_id": job.ID, "automation_policy_revision": 0, "automation_rule_version": job.BuiltinRuleVersion,
			"state": "approved", "state_revision": 2, "approval_origin": "automation", "approval_job_id": job.ID,
			"approved_at": time.Now().Add(-90 * time.Minute), "published_target_revision": index + 1,
			"decision_origin": "automation", "last_decision_job_id": job.ID, "last_decided_at": time.Now().Add(-90 * time.Minute),
		})
	}
	rows, err := q.ListKnowledgeAutomationRevalidationCandidates(context.Background(), db.ListKnowledgeAutomationRevalidationCandidatesParams{
		SourceIssueID: job.SourceIssueID, TargetProjectID: job.ProjectID, TargetWorkspaceID: job.SourceWorkspaceID,
	})
	if err != nil || len(rows) != 4 {
		t.Fatalf("full review inventory lost expired predecessors or stopped at3: %d %v", len(rows), err)
	}
}
