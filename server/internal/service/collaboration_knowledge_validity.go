package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeAutomaticProof struct {
	Binding      protocol.TaskKnowledgeValidity
	EvaluationID pgtype.UUID
	Dependencies []protocol.CollaborationKnowledgeSource
}

func knowledgeValidityBinding(head db.CollaborationKnowledgeValidity) protocol.TaskKnowledgeValidity {
	return protocol.TaskKnowledgeValidity{Revision: strconv.FormatInt(head.Revision, 10), AcceptedEventSeq: strconv.FormatInt(head.AcceptedEventSeq, 10), SubmissionID: util.UUIDToString(head.SubmissionID), ValidatedCloseEpoch: strconv.FormatInt(head.ValidatedCloseEpoch, 10)}
}

func knowledgeSupportDependencies(raw []byte) ([]protocol.CollaborationKnowledgeSource, bool) {
	var refs []protocol.CollaborationKnowledgeSource
	if len(raw) > 128<<10 || json.Unmarshal(raw, &refs) != nil || len(refs) == 0 || len(refs) > 8 {
		return nil, false
	}
	out := make([]protocol.CollaborationKnowledgeSource, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(out, ref) {
			out = append(out, ref)
		}
	}
	return out, true
}

// knowledgeAutomaticValidity is an availability proof, not read authority.
// Callers must separately authorize every source in Dependencies for the
// current consumer and freeze the source/candidate/head before disclosure.
func knowledgeAutomaticValidity(ctx context.Context, q *db.Queries, row db.CollaborationKnowledgeCandidate) (knowledgeAutomaticProof, bool, error) {
	var proof knowledgeAutomaticProof
	if row.ApprovalOrigin == "human" {
		return proof, true, nil
	}
	if row.ApprovalOrigin != "automation" || row.State != "approved" || row.TargetKind != "project" || !row.TargetProjectID.Valid || row.TargetWorkspaceID != row.SourceWorkspaceID {
		return proof, false, nil
	}
	if q == nil {
		return proof, false, knowledgeFailure(503, "knowledge_unavailable")
	}
	head, err := q.GetKnowledgeValidity(ctx, row.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	evaluation, err := q.GetKnowledgeEvaluation(ctx, head.EvaluationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if head.CandidateID != row.ID || head.Revision < 1 || evaluation.Verdict != "pass" || evaluation.CandidateID != row.ID || evaluation.CandidateVersion != row.Version || evaluation.CandidateContentSha256 != row.ContentSha256 ||
		evaluation.SourceWorkspaceID != row.SourceWorkspaceID || evaluation.SourceIssueID != row.SourceIssueID || evaluation.ProjectID != row.TargetProjectID ||
		head.SourceIssueID != evaluation.SourceIssueID || head.ProjectID != evaluation.ProjectID || head.SubmissionID != evaluation.SubmissionID || head.AcceptedEventSeq != evaluation.AcceptedEventSeq || head.ValidatedCloseEpoch != evaluation.ValidatedCloseEpoch {
		return proof, false, nil
	}
	job, err := q.GetKnowledgeAutomationJob(ctx, evaluation.JobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if job.SourceIssueID != row.SourceIssueID || job.SourceWorkspaceID != row.SourceWorkspaceID || job.ProjectID != row.TargetProjectID || job.SubmissionID != head.SubmissionID || job.AcceptedEventSeq != head.AcceptedEventSeq || job.ValidatedCloseEpoch != head.ValidatedCloseEpoch {
		return proof, false, nil
	}
	check, err := q.GetKnowledgeModelAttempt(ctx, db.GetKnowledgeModelAttemptParams{ID: evaluation.CheckAttemptID, JobID: evaluation.JobID})
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if check.State != "succeeded" || check.Stage != "checking" || check.BatchID != evaluation.BatchID || check.PolicyRevision != evaluation.PolicyRevision || check.SourceSnapshotSha256 != evaluation.SourceSnapshotSha256 || check.TargetSnapshotSha256 != evaluation.TargetSnapshotSha256 || check.TargetRevision != evaluation.TargetRevision || evaluation.BuiltinRuleVersion == "" {
		return proof, false, nil
	}
	generation, err := q.GetKnowledgeModelAttempt(ctx, db.GetKnowledgeModelAttemptParams{ID: evaluation.GenerationAttemptID, JobID: evaluation.JobID})
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if generation.State != "succeeded" || generation.Stage != "generating" || generation.BatchID != evaluation.BatchID || generation.ExecutionRevision != check.ExecutionRevision || generation.PolicyRevision != evaluation.PolicyRevision || generation.SourceSnapshotSha256 != check.SourceSnapshotSha256 || generation.TargetSnapshotSha256 != check.TargetSnapshotSha256 || generation.TargetRevision != check.TargetRevision {
		return proof, false, nil
	}
	dependencies, valid := knowledgeSupportDependencies(evaluation.SupportDependencies)
	if !valid {
		return proof, false, nil
	}
	checkedDependencies, valid := knowledgeSupportDependencies(check.SupportDependencies)
	if !valid {
		return proof, false, nil
	}
	for _, ref := range dependencies {
		if ref.IssueID != util.UUIDToString(row.SourceIssueID) || !slices.Contains(checkedDependencies, ref) {
			return proof, false, nil
		}
	}
	issue, err := q.GetIssue(ctx, row.SourceIssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if issue.WorkspaceID != row.SourceWorkspaceID || issue.ProjectID != row.TargetProjectID {
		return proof, false, nil
	}
	state, err := q.GetKnowledgeSourceState(ctx, row.SourceIssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	session, err := q.GetCollaborationSession(ctx, row.SourceIssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return proof, false, nil
	}
	if err != nil {
		return proof, false, err
	}
	if state.LifecycleEpoch != head.ValidatedCloseEpoch || session.QueueState != "closed" {
		return proof, false, nil
	}
	boundary, err := q.GetCollaborationDeliveryLifecycleBoundary(ctx, row.SourceIssueID)
	if err != nil || boundary != head.AcceptedEventSeq {
		return proof, false, err
	}
	delivery, valid, err := knowledgeAcceptedDelivery(ctx, q, issue, job.SubmissionID, job.DeliveryEventSeq, job.AcceptedEventSeq)
	if err != nil || !valid || delivery.AcceptedByUserID != job.SourceReaderUserID || !slices.Contains(dependencies, delivery.Source) {
		return proof, false, err
	}
	original, err := knowledgeCandidateSources(row)
	if err != nil {
		return proof, false, nil
	}
	available, err := (&CollaborationService{}).KnowledgeSourceAvailability(ctx, q, row.SourceIssueID, original)
	if err != nil || !available {
		return proof, false, err
	}
	for _, ref := range dependencies {
		id, err := util.ParseUUID(ref.IssueID)
		if err != nil {
			return proof, false, nil
		}
		available, err := (&CollaborationService{}).KnowledgeSourceAvailability(ctx, q, id, []protocol.CollaborationKnowledgeSource{ref})
		if err != nil || !available {
			return proof, false, err
		}
	}
	return knowledgeAutomaticProof{Binding: knowledgeValidityBinding(head), EvaluationID: head.EvaluationID, Dependencies: dependencies}, true, nil
}

// KnowledgeUsageStatus reports current availability only. It grants no source
// read and deliberately does not reapply today's production policy to history.
func (s *CollaborationService) KnowledgeUsageStatus(ctx context.Context, q *db.Queries, row db.CollaborationKnowledgeCandidate) (string, error) {
	if row.State != "approved" || row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(time.Now()) {
		return "inactive", nil
	}
	_, valid, err := knowledgeAutomaticValidity(ctx, q, row)
	if err != nil {
		return "", err
	}
	if !valid {
		return "paused_review", nil
	}
	return "active", nil
}
