package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This fixture publishes through the actual retained SQL primitives, backed by
// a real formal acceptance and succeeded synthetic model-attempt records. It
// neither calls an account model nor disables any immutable-history trigger.
func publishConsumptionProof(t *testing.T, f *collaborationQueueFixture, source protocol.CollaborationKnowledgeSource, previous *db.CollaborationKnowledgeCandidate, validityRevision int64, extra []protocol.CollaborationKnowledgeSource) db.CollaborationKnowledgeCandidate {
	t.Helper()
	q, ctx := f.svc.Queries, f.ctx
	issue, err := q.GetIssue(ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := q.GetKnowledgeSourceState(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	deliverySeq, _ := strconv.ParseInt(source.DeliveryEventSeq, 10, 64)
	acceptedSeq, _ := strconv.ParseInt(source.AcceptedEventSeq, 10, 64)
	accepted, available, err := knowledgeAcceptedDelivery(ctx, q, issue, util.MustParseUUID(source.SubmissionID), deliverySeq, acceptedSeq)
	if err != nil || !available || accepted.Source != source {
		t.Fatal("proof fixture needs its exact accepted source:", err)
	}
	job, err := q.GetKnowledgeAutomationJobForAcceptance(ctx, db.GetKnowledgeAutomationJobForAcceptanceParams{SourceIssueID: issue.ID, AcceptedEventSeq: acceptedSeq})
	if errors.Is(err, pgx.ErrNoRows) {
		job, err = q.CreateKnowledgeAutomationJob(ctx, db.CreateKnowledgeAutomationJobParams{ID: util.MustParseUUID(uuid.NewString()), SourceWorkspaceID: issue.WorkspaceID, SourceIssueID: issue.ID, ProjectID: issue.ProjectID,
			SubmissionID: util.MustParseUUID(source.SubmissionID), DeliveryEventSeq: deliverySeq, AcceptedEventSeq: acceptedSeq, ValidatedCloseEpoch: state.LifecycleEpoch,
			SourceReaderUserID: util.MustParseUUID(f.creator), BuiltinRuleVersion: "2026-11-14-v1", MaxCandidates: 3})
	}
	if err != nil {
		t.Fatal(err)
	}
	lease := util.MustParseUUID(uuid.NewString())
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET lease_token=$2,lease_generation=lease_generation+1,lease_expires_at=clock_timestamp()+interval '180 seconds' WHERE id=$1`, job.ID, lease)
	job, err = q.GetKnowledgeAutomationJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	intent, batchIndex, predecessor := "create", int16(6), []pgtype.UUID{}
	if previous != nil {
		intent, batchIndex, predecessor = "revalidate", 0, []pgtype.UUID{previous.ID}
	}
	batch, err := q.CreateKnowledgeAutomationBatch(ctx, db.CreateKnowledgeAutomationBatchParams{ID: util.MustParseUUID(uuid.NewString()), JobID: job.ID, BatchIndex: batchIndex, Intent: intent,
		PredecessorIds: predecessor, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.InitializeKnowledgeTarget(ctx, db.InitializeKnowledgeTargetParams{TargetKind: "project", TargetID: issue.ProjectID, WorkspaceID: issue.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	target, err := q.GetKnowledgeTarget(ctx, db.GetKnowledgeTargetParams{TargetKind: "project", TargetID: issue.ProjectID})
	if err != nil {
		t.Fatal(err)
	}
	dependencies := append([]protocol.CollaborationKnowledgeSource{source}, extra...)
	refs, err := json.Marshal(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	var generated, checked pgtype.UUID
	for _, stage := range []string{"generating", "checking"} {
		attempt, err := q.CreateKnowledgeModelAttempt(ctx, db.CreateKnowledgeModelAttemptParams{ID: util.MustParseUUID(uuid.NewString()), JobID: job.ID, BatchID: batch.ID, ExecutionRevision: job.ExecutionRevision,
			Stage: stage, AttemptNumber: 1, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration, ConfigurationFingerprint: strings.Repeat("a", 64), RequestedModel: "controlled-consumption-fixture",
			SourceSnapshotSha256: source.SHA256, TargetSnapshotSha256: strings.Repeat("b", 64), TargetRevision: target.KnowledgeRevision, RequestSha256: strings.Repeat("c", 64), InputSourceClosure: refs, SupportDependencies: refs})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = q.FinishKnowledgeModelAttempt(ctx, db.FinishKnowledgeModelAttemptParams{ID: attempt.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration,
			State: "succeeded", ResponseJson: []byte(`{"fixture":true}`), ActualModel: pgtype.Text{String: "controlled-consumption-fixture", Valid: true}, Usage: []byte(`{"status":"unknown"}`)}); err != nil {
			t.Fatal(err)
		}
		if stage == "generating" {
			generated = attempt.ID
		} else {
			checked = attempt.ID
		}
	}
	if _, err = q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration,
		Phase: "settling", GenerationAttemptID: generated, CheckAttemptID: checked}); err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q = q.WithTx(tx)
	var candidate db.CollaborationKnowledgeCandidate
	if previous == nil {
		sources, _ := json.Marshal([]protocol.CollaborationKnowledgeSource{source})
		candidate, err = q.CreateAutomatedKnowledgeCandidate(ctx, db.CreateAutomatedKnowledgeCandidateParams{ID: util.MustParseUUID(uuid.NewString()), JobID: job.ID, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration,
			Content: accepted.Result, Summary: "Accepted conclusion", ContentSha256: protocol.CollaborationKnowledgeHash(accepted.Result), Sources: sources,
			Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue","collaboration"]}`)})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		candidate = *previous
	}
	evaluate := db.CreateKnowledgeEvaluationParams{ID: util.MustParseUUID(uuid.NewString()), JobID: job.ID, BatchID: batch.ID, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration,
		CandidateID: candidate.ID, CandidateVersion: candidate.Version, CandidateContentSha256: candidate.ContentSha256, Intent: intent, Verdict: "pass", ReasonCode: "rules_passed", SupportDependencies: refs, ModelResult: []byte(`{"fixture":true}`)}
	if previous != nil {
		evaluate.PredecessorCandidateID = candidate.ID
	}
	evaluation, err := q.CreateKnowledgeEvaluation(ctx, evaluate)
	if err != nil {
		t.Fatal(err)
	}
	target, err = q.AdvanceKnowledgeTarget(ctx, db.AdvanceKnowledgeTargetParams{TargetKind: "project", TargetID: issue.ProjectID, KnowledgeRevision: target.KnowledgeRevision})
	if err != nil {
		t.Fatal(err)
	}
	if previous == nil {
		candidate, err = q.ApproveAutomatedKnowledgeCandidate(ctx, db.ApproveAutomatedKnowledgeCandidateParams{ID: candidate.ID, JobID: job.ID, LeaseToken: lease, LeaseGeneration: job.LeaseGeneration,
			EvaluationID: evaluation.ID, ExpectedStateRevision: candidate.StateRevision, ExpectedVersion: candidate.Version, ExpectedContentSha256: candidate.ContentSha256, TargetRevision: pgtype.Int8{Int64: target.KnowledgeRevision, Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = q.SetKnowledgeValidity(ctx, db.SetKnowledgeValidityParams{CandidateID: candidate.ID, EvaluationID: evaluation.ID, ExpectedRevision: validityRevision, ExpectedStateRevision: candidate.StateRevision}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return candidate
}

func TestKnowledgeAutomaticValidityPausesOnReopenWithoutOutbox(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
	candidate := publishConsumptionProof(t, f, source, nil, 0, nil)
	consumer := automaticConsumerTask(t, f, candidate.TargetProjectID)
	consumer.recordSession(t, "automatic-before-reopen")
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProtocolVersion, facts.ProjectID = 2, candidate.TargetProjectID
	first, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(first.Contributions) != 1 || first.Manifest.Entries[0].Validity == nil || first.Contributions[0].Sources[0] != source {
		t.Fatalf("approved automatic source was not exactly disclosed: items=%d err=%v", len(first.Contributions), err)
	}
	if status, err := f.svc.KnowledgeUsageStatus(f.ctx, f.svc.Queries, candidate); err != nil || status != "active" {
		t.Fatalf("valid automatic approval status=%s err=%v", status, err)
	}
	access := mustAccess(t, f, f.ctx)
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
		t.Fatal(err)
	}
	// No invalidation scanner or websocket drain runs between reopen and reads.
	after, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(after.Contributions) != 0 {
		t.Fatalf("reopen did not synchronously fence new disclosure: %v", err)
	}
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, first.Manifest); err != nil || ok {
		t.Fatalf("old exact manifest remained authorized after reopen: %t %v", ok, err)
	}
	if status, err := f.svc.KnowledgeUsageStatus(f.ctx, f.svc.Queries, candidate); err != nil || status != "paused_review" {
		t.Fatalf("approved history was not separately paused: %s %v", status, err)
	}
	consumer.nextTurn(t)
	if claim, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, "automatic-before-reopen"); !collaborationErrorCode(err, "knowledge_context_changed") || claim != nil || prior != "" {
		t.Fatal("paused automatic exposure silently fell back to a fresh session:", err)
	}
}

func TestKnowledgeAutomaticValidityKeepsPolicyOffHistory(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
	candidate := publishConsumptionProof(t, f, source, nil, 0, nil)
	_, err := f.svc.Queries.CreateProjectKnowledgePolicy(f.ctx, db.CreateProjectKnowledgePolicyParams{ProjectID: candidate.TargetProjectID, WorkspaceID: candidate.TargetWorkspaceID,
		IsEnabled: false, MaxCandidates: 1, BuiltinRuleVersion: "future-disabled-rule", ChangedByUserID: util.MustParseUUID(f.creator)})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE issue_collaboration SET ai_policy='manual' WHERE issue_id=$1`, candidate.SourceIssueID)
	if _, valid, err := knowledgeAutomaticValidity(f.ctx, f.svc.Queries, candidate); err != nil || !valid {
		t.Fatalf("current production policies revoked valid retained knowledge: %t %v", valid, err)
	}
}

func TestKnowledgeAutomaticValidityChecksNewSupportSources(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable", func(f *collaborationQueueFixture) {
		f.memoryUpdate(t, f.creator, 0, "New proof support")
	})
	memory := protocol.CollaborationKnowledgeSource{Kind: "memory", IssueID: source.IssueID, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("New proof support")}
	candidate := publishConsumptionProof(t, f, source, nil, 0, []protocol.CollaborationKnowledgeSource{memory})
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProtocolVersion, facts.ProjectID = 2, candidate.TargetProjectID
	hints, err := f.svc.CollectKnowledgeReadLockHints(f.ctx, f.svc.Queries, facts, nil)
	if err != nil || len(hints.Validities) != 1 {
		t.Fatalf("automatic head was not in the lock set: %v", err)
	}
	found := false
	for _, hint := range hints.Sources {
		for _, ref := range hint.Sources {
			found = found || ref == memory
		}
	}
	if !found {
		t.Fatal("new evaluation support absent from source locks")
	}
	first, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(first.Contributions) != 1 {
		t.Fatal("valid proof fixture failed:", err)
	}
	f.fx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=$2,purged_by_user_id=$3,purge_reason='proof revoked' WHERE issue_id=$1`, candidate.SourceIssueID, time.Now(), f.creator)
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, first.Manifest); err != nil || ok {
		t.Fatalf("new support loss was ignored by exact manifest: %t %v", ok, err)
	}
}

func TestKnowledgeReacceptanceCannotReplaceOldManifestProof(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
	candidate := publishConsumptionProof(t, f, source, nil, 0, nil)
	consumer := automaticConsumerTask(t, f, candidate.TargetProjectID)
	consumer.recordSession(t, "automatic-before-reacceptance")
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProtocolVersion, facts.ProjectID = 2, candidate.TargetProjectID
	before, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(before.Contributions) != 1 {
		t.Fatal("original proof fixture failed:", err)
	}
	access := mustAccess(t, f, f.ctx)
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
		t.Fatal(err)
	}
	access = mustAccess(t, f, f.ctx)
	accept := lifecycleParams(t, f, "complete", "done", access.Revision, access.LifecycleEpoch)
	accept.Note, accept.Basis = "Accepted reusable conclusion", "The exact result remains reusable"
	if _, err := f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
		t.Fatal(err)
	}
	summary := deliverySummaryForTest(t, f, f.ctx)
	issue, err := f.svc.Queries.GetIssue(f.ctx, candidate.SourceIssueID)
	if err != nil || summary.Latest == nil || summary.Latest.Decision == nil {
		t.Fatal("reacceptance fixture failed:", err)
	}
	newSource, available, err := knowledgeAcceptedDelivery(f.ctx, f.svc.Queries, issue, summary.Latest.ID, summary.Latest.EventSeq, summary.Latest.Decision.EventSeq)
	if err != nil || !available {
		t.Fatal("new accepted source unavailable:", err)
	}
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, before.Manifest); err != nil || ok {
		t.Fatal("acceptance alone restored an unreviewed old proof")
	}
	publishConsumptionProof(t, f, newSource.Source, &candidate, 1, nil)
	after, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(after.Contributions) != 1 || after.Manifest.Entries[0].Validity == nil || after.Manifest.Entries[0].Validity.Revision != "2" || after.ManifestSHA256 == before.ManifestSHA256 {
		t.Fatalf("revalidation did not establish a new exact proof: %v", err)
	}
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, before.Manifest); err != nil || ok {
		t.Fatal("new validity was substituted into an old manifest")
	}
	if _, err := f.svc.CollectKnowledgeReadLockHints(f.ctx, f.svc.Queries, facts, &before.Manifest); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatal("exact old preparation acquired a different proof's locks:", err)
	}
	consumer.nextTurn(t)
	if claim, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, "automatic-before-reacceptance"); !collaborationErrorCode(err, "knowledge_context_changed") || claim != nil || prior != "" {
		t.Fatal("new proof silently rebound an exposed prior session:", err)
	}
	fresh := consumer.finalize(t, "")
	prepared, err := consumer.tasks.PrepareTaskKnowledge(consumer.ctx, consumer.caller, consumer.task.ID, consumer.prepareRequest(fresh))
	if err != nil || len(prepared.Contributions) != 1 || prepared.Manifest.Entries[0].Validity == nil || prepared.Manifest.Entries[0].Validity.Revision != "2" {
		t.Fatal("explicitly fresh execution could not prepare the current proof:", err)
	}
}

func TestKnowledgeHumanApprovalOfAutomationProposalSurvivesReopen(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
	automatic := publishConsumptionProof(t, f, source, nil, 0, nil)
	job, err := f.svc.Queries.GetKnowledgeAutomationJob(f.ctx, automatic.AutomationJobID)
	if err != nil {
		t.Fatal(err)
	}
	refs, _ := json.Marshal([]protocol.CollaborationKnowledgeSource{source})
	manual, err := f.svc.Queries.CreateAutomatedKnowledgeCandidate(f.ctx, db.CreateAutomatedKnowledgeCandidateParams{ID: util.MustParseUUID(uuid.NewString()), JobID: job.ID, LeaseToken: job.LeaseToken,
		LeaseGeneration: job.LeaseGeneration, Content: "Human approved conclusion", Summary: "Human approved", ContentSha256: protocol.CollaborationKnowledgeHash("Human approved conclusion"), Sources: refs,
		Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithKnowledgeProtocolVersion(f.ctx, 2)
	if _, err = f.svc.ApproveKnowledge(ctx, manual.ID, protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: manual.ContentSha256}); err != nil {
		t.Fatal(err)
	}
	access := mustAccess(t, f, f.ctx)
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProtocolVersion, facts.ProjectID = 2, automatic.TargetProjectID
	result, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(result.Contributions) != 1 || result.Contributions[0].CandidateID != util.UUIDToString(manual.ID) || result.Manifest.Entries[0].Validity != nil {
		t.Fatalf("reopen paused a human approval based on proposal origin: %v", err)
	}
}

func TestKnowledgeReopenKeepsOtherSourceInSameProjectActive(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
	first := publishConsumptionProof(t, f, source, nil, 0, nil)
	create := f.params(t)
	create.ProjectID = first.TargetProjectID
	other, err := f.svc.Create(f.ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, other.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, other.Issue.ID)
	secondSource := &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: other}
	submit := handoffAcceptance(t, secondSource, 1, "A distinct reusable conclusion")
	submit.Basis = "The other source establishes this separate conclusion"
	receipt, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), submit)
	if err != nil || receipt.DeliveryConfirmation == nil {
		t.Fatal(err)
	}
	accept := lifecycleParams(t, secondSource, "complete", "done", 2, 1)
	accept.SubmissionID = &receipt.DeliveryConfirmation.SubmissionID
	if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
		t.Fatal(err)
	}
	summary := deliverySummaryForTest(t, secondSource, f.ctx)
	issue, err := f.svc.Queries.GetIssue(f.ctx, other.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, available, err := knowledgeAcceptedDelivery(f.ctx, f.svc.Queries, issue, summary.Latest.ID, summary.Latest.EventSeq, summary.Latest.Decision.EventSeq)
	if err != nil || !available {
		t.Fatal(err)
	}
	second := publishConsumptionProof(t, secondSource, snapshot.Source, nil, 0, nil)
	access := mustAccess(t, f, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProtocolVersion, facts.ProjectID = 2, first.TargetProjectID
	result, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(result.Contributions) != 1 || result.Contributions[0].CandidateID != util.UUIDToString(second.ID) {
		t.Fatalf("one source reopen paused another source in the same project: %v", err)
	}
}
