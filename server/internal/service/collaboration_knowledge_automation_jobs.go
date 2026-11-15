package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type KnowledgeAutomationCounts struct {
	Saved    int `json:"saved"`
	Restored int `json:"restored"`
	Revised  int `json:"revised"`
	Pending  int `json:"pending"`
	Skipped  int `json:"skipped"`
}

type KnowledgeAutomationAttemptView struct {
	Stage string        `json:"stage"`
	State string        `json:"state"`
	Model *string       `json:"model"`
	Usage llm.JSONUsage `json:"usage"`
}

type KnowledgeAutomationJobView struct {
	ID               string                           `json:"id"`
	IssueID          string                           `json:"issue_id"`
	ProjectID        string                           `json:"project_id"`
	SubmissionID     string                           `json:"submission_id"`
	AcceptedEventSeq string                           `json:"accepted_event_seq"`
	Revision         string                           `json:"revision"`
	Phase            string                           `json:"phase"`
	ReasonCode       *string                          `json:"reason_code"`
	SourceCurrent    bool                             `json:"source_current"`
	CanRetry         bool                             `json:"can_retry"`
	Counts           KnowledgeAutomationCounts        `json:"counts"`
	Attempts         []KnowledgeAutomationAttemptView `json:"attempts"`
}

type KnowledgeAutomationJobsView struct {
	KnowledgeProjection
	IssueID string                       `json:"issue_id"`
	Items   []KnowledgeAutomationJobView `json:"items"`
	HasMore bool                         `json:"has_more"`
}

type KnowledgeAutomationRetryRequest struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision string `json:"expected_revision"`
}

type KnowledgeAutomationRetryReceipt struct {
	ProtocolVersion int    `json:"protocol_version"`
	RequestID       string `json:"request_id"`
	IssueID         string `json:"issue_id"`
	JobID           string `json:"job_id"`
	Revision        string `json:"revision"`
	Code            string `json:"code"`
}

type knowledgeAutomationOutcome struct {
	ExecutionRevision int64  `json:"execution_revision,string"`
	Ref               string `json:"ref"`
	CandidateID       string `json:"candidate_id,omitempty"`
	Outcome           string `json:"outcome"`
	ReasonCode        string `json:"reason_code"`
}

// Only unresolved slots enter an explicit retry. Settled review slots retain
// their fixed original refs; create retries retain published successes and
// have a separate acceptance-wide publication budget.
func knowledgeAutomationRetained(batch db.CollaborationKnowledgeBatch) ([]knowledgeAutomationOutcome, map[string]bool, error) {
	retained := []knowledgeAutomationOutcome{}
	refs := map[string]bool{}
	if len(batch.Result) == 0 {
		return retained, refs, nil
	}
	var result knowledgeAutomationBatchResult
	if json.Unmarshal(batch.Result, &result) != nil {
		return nil, nil, automationPending("result_unavailable")
	}
	for _, outcome := range result.Outcomes {
		if outcome.Outcome == "review_pending" {
			continue
		}
		switch outcome.Outcome {
		case "saved", "restored", "revised", "duplicate", "transient":
		default:
			return nil, nil, automationPending("result_unavailable")
		}
		if batch.Intent == "create" && outcome.Outcome != "saved" {
			continue
		}
		if batch.Intent == "revalidate" {
			index, err := strconv.Atoi(strings.TrimPrefix(outcome.Ref, "k"))
			if err != nil || index < 1 || index > len(batch.PredecessorIds) || outcome.Ref != "k"+strconv.Itoa(index) || refs[outcome.Ref] {
				return nil, nil, automationPending("result_unavailable")
			}
			refs[outcome.Ref] = true
		}
		retained = append(retained, outcome)
	}
	return retained, refs, nil
}

type knowledgeAutomationBatchResult struct {
	Outcomes []knowledgeAutomationOutcome `json:"outcomes"`
}

// This hook borrows the existing acceptance transaction, after its exact
// completed event has been saved. It never locks the target knowledge counter.
func enqueueAcceptedKnowledgeAutomation(ctx context.Context, t *collaborationWorkflowTx, acceptedSeq int64) error {
	if t.deliveryDecision == nil || t.deliveryDecision.Kind != "accepted" || !t.access.Issue.ProjectID.Valid || t.session.QueueState != "closed" {
		return nil
	}
	state, err := t.q.GetKnowledgeSourceState(ctx, t.access.Issue.ID)
	if err != nil || state.AiPolicy == "manual" {
		return err
	}
	policy, err := knowledgeEffectivePolicy(ctx, t.q, t.access.Issue.ProjectID, t.access.Issue.WorkspaceID)
	if err != nil || !policy.IsEnabled {
		return err
	}
	events, err := t.q.ListCollaborationDeliveryEvents(ctx, db.ListCollaborationDeliveryEventsParams{IssueID: t.access.Issue.ID, ThroughSeq: acceptedSeq, RowLimit: 1})
	if err != nil {
		return err
	}
	if len(events) != 1 {
		return knowledgeFailure(409, "knowledge_source_unavailable")
	}
	delivery, available, err := knowledgeAcceptedDelivery(ctx, t.q, t.access.Issue, t.deliveryDecision.SubmissionID, events[0].EventSeq, acceptedSeq)
	if err != nil {
		return err
	}
	if !available || delivery.AcceptedByUserID != t.actor {
		return knowledgeFailure(409, "knowledge_source_unavailable")
	}
	revision, _ := knowledgeRevision(policy.Revision)
	_, err = t.q.CreateKnowledgeAutomationJob(ctx, db.CreateKnowledgeAutomationJobParams{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceWorkspaceID: t.access.Issue.WorkspaceID, SourceIssueID: t.access.Issue.ID, ProjectID: t.access.Issue.ProjectID,
		SubmissionID: t.deliveryDecision.SubmissionID, DeliveryEventSeq: events[0].EventSeq, AcceptedEventSeq: acceptedSeq,
		ValidatedCloseEpoch: t.access.LifecycleEpoch, SourceReaderUserID: t.actor, PolicyRevision: revision,
		BuiltinRuleVersion: policy.RuleVersion, MaxCandidates: int16(policy.MaxCandidates)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func knowledgeProjectCanManage(ctx context.Context, q *db.Queries, actor, project, workspace pgtype.UUID) (bool, error) {
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor, WorkspaceID: workspace})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	p, err := q.GetKnowledgeProjectAuthority(ctx, db.GetKnowledgeProjectAuthorityParams{ID: project, WorkspaceID: workspace})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && (member.Role == "owner" || member.Role == "admin" || p.LeadType.String == "member" && p.LeadID == actor), err
}

func (s *CollaborationService) ListKnowledgeAutomationJobs(ctx context.Context, issueID pgtype.UUID) (KnowledgeAutomationJobsView, error) {
	out := KnowledgeAutomationJobsView{KnowledgeProjection: knowledgeNewProjection(ctx), IssueID: knowledgeUUIDString(issueID), Items: []KnowledgeAutomationJobView{}}
	out.ProtocolVersion = 2
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		p, ok := collaboration.PrincipalFromContext(ctx)
		if !ok || !p.Human() {
			return knowledgeFailure(403, "knowledge_permission_denied")
		}
		actor, err := util.ParseUUID(p.UserID)
		if err != nil {
			return err
		}
		jobs, err := q.ListKnowledgeAutomationJobs(ctx, db.ListKnowledgeAutomationJobsParams{SourceWorkspaceID: access.Issue.WorkspaceID, SourceIssueID: issueID, Limit: 21})
		if err != nil {
			return err
		}
		out.HasMore = len(jobs) > 20
		if out.HasMore {
			jobs = jobs[:20]
		}
		for _, job := range jobs {
			view := KnowledgeAutomationJobView{ID: knowledgeUUIDString(job.ID), IssueID: knowledgeUUIDString(issueID), ProjectID: knowledgeUUIDString(job.ProjectID),
				SubmissionID: knowledgeUUIDString(job.SubmissionID), AcceptedEventSeq: strconv.FormatInt(job.AcceptedEventSeq, 10), Revision: strconv.FormatInt(job.ExecutionRevision, 10),
				Phase: job.Phase, SourceCurrent: access.LifecycleEpoch == job.ValidatedCloseEpoch && access.Issue.ProjectID == job.ProjectID,
				Attempts: []KnowledgeAutomationAttemptView{}}
			if job.ReasonCode.Valid {
				reason := job.ReasonCode.String
				view.ReasonCode = &reason
			}
			manage, err := knowledgeProjectCanManage(ctx, q, actor, job.ProjectID, job.SourceWorkspaceID)
			if err != nil {
				return err
			}
			view.CanRetry = manage && view.SourceCurrent && (job.Phase == "review_pending" || job.Phase == "waiting_configuration")
			batches, err := q.ListKnowledgeAutomationBatches(ctx, job.ID)
			if err != nil {
				return err
			}
			for _, batch := range batches {
				var result knowledgeAutomationBatchResult
				if len(batch.Result) > 0 && json.Unmarshal(batch.Result, &result) != nil {
					return knowledgeFailure(503, "knowledge_unavailable")
				}
				for _, outcome := range result.Outcomes {
					switch outcome.Outcome {
					case "saved":
						view.Counts.Saved++
					case "restored":
						view.Counts.Restored++
					case "revised":
						view.Counts.Revised++
					case "review_pending":
						if outcome.CandidateID != "" {
							view.Counts.Pending++
						}
					case "duplicate", "transient":
						view.Counts.Skipped++
					}
				}
			}
			attempts, err := q.ListKnowledgeExecutionModelUsage(ctx, db.ListKnowledgeExecutionModelUsageParams{JobID: job.ID, ExecutionRevision: job.ExecutionRevision})
			if err != nil {
				return err
			}
			if len(attempts) > 42 {
				return knowledgeFailure(503, "knowledge_unavailable")
			}
			for _, attempt := range attempts {
				u := KnowledgeAutomationAttemptView{Stage: attempt.Stage, State: attempt.State, Usage: llm.JSONUsage{Status: llm.UsageUnknown}}
				if json.Unmarshal(attempt.Usage, &u.Usage) != nil || !knowledgeAutomationUsageValid(u.Usage) {
					return knowledgeFailure(503, "knowledge_unavailable")
				}
				if attempt.ActualModel.Valid {
					model := attempt.ActualModel.String
					u.Model = &model
				}
				view.Attempts = append(view.Attempts, u)
			}
			out.Items = append(out.Items, view)
		}
		expiry, err := knowledgePAT(ctx, q, actor, false)
		if err != nil {
			return err
		}
		out.clampCredential(expiry)
		current, err := s.ResolveIssueAccess(ctx, q, issueID)
		if err != nil {
			return err
		}
		if !current.Capabilities.Read {
			return knowledgeFailure(403, "knowledge_permission_denied")
		}
		return knowledgeProjectionCurrent(out.KnowledgeProjection)
	})
	return out, knowledgePublicError(err)
}

func knowledgeAutomationUsageValid(usage llm.JSONUsage) bool {
	primary, known := 0, 0
	for index, value := range []*int64{usage.InputTokens, usage.OutputTokens, usage.TotalTokens, usage.CachedInputTokens, usage.ReasoningTokens} {
		if value != nil {
			if *value < 0 {
				return false
			}
			known++
			if index < 3 {
				primary++
			}
		}
	}
	switch usage.Status {
	case llm.UsageKnown:
		return primary == 3
	case llm.UsagePartial:
		return known > 0 && primary < 3
	case llm.UsageUnknown:
		return known == 0
	default:
		return false
	}
}

func (s *CollaborationService) RetryKnowledgeAutomationJob(ctx context.Context, issueID, jobID pgtype.UUID, p KnowledgeAutomationRetryRequest) (KnowledgeAutomationRetryReceipt, error) {
	revision, valid := knowledgeRevision(p.ExpectedRevision)
	request, err := util.ParseUUID(p.RequestID)
	if !valid || revision < 1 || err != nil || knowledgeUUIDString(request) != p.RequestID {
		return KnowledgeAutomationRetryReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	raw, _ := json.Marshal(struct {
		Job     string
		Request KnowledgeAutomationRetryRequest
	}{knowledgeUUIDString(jobID), p})
	hash := protocol.CollaborationKnowledgeHash(string(raw))
	r, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (KnowledgeAutomationRetryReceipt, error) {
		q := s.Queries.WithTx(tx)
		actor, err := knowledgeHumanActor(ctx, q)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		previous, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: p.RequestID})
		if err == nil {
			if previous.Kind != "knowledge_job_retry" || previous.PayloadHash != hash {
				return KnowledgeAutomationRetryReceipt{}, knowledgeFailure(409, "idempotency_conflict")
			}
			return decodeKnowledgeAutomationRetryReceipt(previous, issueID, jobID, p.RequestID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		job, err := q.GetKnowledgeAutomationJob(ctx, jobID)
		if err != nil || job.SourceIssueID != issueID {
			return KnowledgeAutomationRetryReceipt{}, knowledgeFailure(404, "knowledge_job_not_found")
		}
		if err = knowledgeLockProjectConfirmer(ctx, q, actor, job.ProjectID, job.SourceWorkspaceID); err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		issue, err := q.LockKnowledgeSourceIssue(ctx, issueID)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		state, err := q.LockKnowledgeSourceCollaboration(ctx, issueID)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		if _, err = q.LockKnowledgeSourceSession(ctx, issueID); err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		read, err := knowledgeUserReadsSource(ctx, q, issueID, actor)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		if !read {
			return KnowledgeAutomationRetryReceipt{}, knowledgeFailure(403, "knowledge_permission_denied")
		}
		job, err = q.LockKnowledgeAutomationJob(ctx, jobID)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		policy, err := knowledgeEffectivePolicy(ctx, q, job.ProjectID, job.SourceWorkspaceID)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		op, err := q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: p.RequestID, Kind: "knowledge_job_retry", PayloadHash: hash})
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		receipt := KnowledgeAutomationRetryReceipt{ProtocolVersion: 2, RequestID: p.RequestID, IssueID: knowledgeUUIDString(issueID), JobID: knowledgeUUIDString(jobID), Revision: strconv.FormatInt(job.ExecutionRevision, 10), Code: "knowledge_job_revision_conflict"}
		switch {
		case state.LifecycleEpoch != job.ValidatedCloseEpoch || issue.ProjectID != job.ProjectID:
			receipt.Code = "knowledge_source_changed"
		case state.AiPolicy == "manual" || !policy.IsEnabled:
			receipt.Code = "knowledge_automation_disabled"
		case job.ExecutionRevision == revision && (job.Phase == "review_pending" || job.Phase == "waiting_configuration"):
			policyRevision, _ := knowledgeRevision(policy.Revision)
			updated, err := q.RetryKnowledgeAutomationJob(ctx, db.RetryKnowledgeAutomationJobParams{ID: job.ID, ExpectedExecutionRevision: revision,
				PolicyRevision: policyRevision, RuleVersion: policy.RuleVersion, MaxCandidates: int16(policy.MaxCandidates)})
			if err != nil {
				return KnowledgeAutomationRetryReceipt{}, err
			}
			if _, err = q.ResetKnowledgeAutomationBatchForRetry(ctx, db.ResetKnowledgeAutomationBatchForRetryParams{JobID: job.ID, ExecutionRevision: updated.ExecutionRevision}); err != nil {
				return KnowledgeAutomationRetryReceipt{}, err
			}
			receipt.Revision, receipt.Code = strconv.FormatInt(updated.ExecutionRevision, 10), "knowledge_job_retried"
		}
		encoded, _ := json.Marshal(receipt)
		_, err = q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: op.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: issueID})
		return receipt, err
	})
	if err == nil && r.Code != "knowledge_job_retried" {
		err = knowledgeFailure(409, r.Code)
	}
	return r, knowledgePublicError(err)
}

func decodeKnowledgeAutomationRetryReceipt(op db.CollaborationOperation, issue, job pgtype.UUID, request string) (KnowledgeAutomationRetryReceipt, error) {
	var r KnowledgeAutomationRetryReceipt
	if !op.CompletedAt.Valid || json.Unmarshal(op.Result, &r) != nil || r.ProtocolVersion != 2 || r.RequestID != request || r.IssueID != knowledgeUUIDString(issue) || r.JobID != knowledgeUUIDString(job) || r.Code != op.ResultCode.String {
		return r, knowledgeFailure(503, "knowledge_unavailable")
	}
	return r, nil
}

func (s *CollaborationService) GetKnowledgeAutomationRetryOperation(ctx context.Context, issue, job pgtype.UUID, request string) (KnowledgeAutomationRetryReceipt, error) {
	r, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (KnowledgeAutomationRetryReceipt, error) {
		q := s.Queries.WithTx(tx)
		actor, err := knowledgeHumanActor(ctx, q)
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issue, ActorUserID: actor, RequestID: request})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && op.Kind != "knowledge_job_retry" {
			return KnowledgeAutomationRetryReceipt{}, knowledgeFailure(404, "operation_not_found")
		}
		if err != nil {
			return KnowledgeAutomationRetryReceipt{}, err
		}
		return decodeKnowledgeAutomationRetryReceipt(op, issue, job, request)
	})
	return r, knowledgePublicError(err)
}
