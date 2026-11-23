package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (w *KnowledgeAutomationWorker) settle(ctx context.Context, job db.CollaborationKnowledgeJob, batch db.CollaborationKnowledgeBatch, expected knowledgeAutomationSnapshot, candidates []knowledgeAutomationCandidate, checks []knowledgeAutomationCheck) error {
	_, err := collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (bool, error) {
		q := w.Service.Queries.WithTx(tx)
		snapshot, err := w.snapshot(ctx, q, job, &batch)
		if err != nil {
			return false, err
		}
		if snapshot.SourceHash != expected.SourceHash || snapshot.TargetHash != expected.TargetHash {
			return false, automationPending("snapshot_changed")
		}
		current, err := q.GetKnowledgeAutomationBatch(ctx, db.GetKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID})
		if err != nil {
			return false, err
		}
		if current.Phase == "completed" {
			return true, nil
		}
		if _, err = q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "settling", GenerationAttemptID: current.GenerationAttemptID, CheckAttemptID: current.CheckAttemptID, Result: current.Result}); err != nil {
			return false, err
		}
		retained, _, err := knowledgeAutomationRetained(current)
		if err != nil {
			return false, err
		}
		result := knowledgeAutomationBatchResult{Outcomes: retained}
		capacity := []db.CollaborationKnowledgeCandidate{}
		for _, row := range snapshot.Rows {
			if !row.ExpiresAt.Valid || row.ExpiresAt.Time.After(snapshot.CheckedAt) {
				capacity = append(capacity, row)
			}
		}
		applicability, _ := json.Marshal(knowledgeAutomationApplicability)
		counter := snapshot.Counter
		advanced := false
		pending := false
		seenContent := map[string]bool{}
		for index, candidate := range candidates {
			originalRef := candidate.Ref
			if batch.Intent == "revalidate" {
				originalRef = snapshot.OriginalRefs[index]
			}
			check := checks[index]
			if check.Ref != candidate.Ref {
				return false, automationPending("model_output_invalid")
			}
			var predecessor db.CollaborationKnowledgeCandidate
			predecessorRef := ""
			if batch.Intent == "revalidate" {
				predecessor = snapshot.Predecessors[index]
				for _, comparison := range snapshot.Input.Comparisons {
					if comparison.PredecessorFor == candidate.Ref {
						predecessorRef = comparison.Ref
					}
				}
			}
			outcome, reason := knowledgeAutomationRuleOutcome(candidate, check, predecessorRef)
			if !slices.Contains(candidate.Sources, snapshot.Sources[0]) {
				outcome, reason = "review_pending", "new_acceptance_unsupported"
			}
			if predecessor.ID.Valid && (predecessor.ExpiresAt.Valid && !predecessor.ExpiresAt.Time.After(snapshot.CheckedAt)) {
				pending = true
				result.Outcomes = append(result.Outcomes, knowledgeAutomationOutcome{ExecutionRevision: job.ExecutionRevision, Ref: originalRef, CandidateID: knowledgeUUIDString(predecessor.ID), Outcome: "review_pending", ReasonCode: "candidate_expired"})
				continue
			}
			key := normalizeKnowledgeAutomationContent(candidate.Content)
			if seenContent[key] {
				if candidate.Intent == "create" {
					outcome, reason = "duplicate", "exact_duplicate"
				} else {
					outcome, reason = "review_pending", "duplicate_other_knowledge"
				}
			}
			seenContent[key] = true
			blocked, err := q.KnowledgeAutomationHasHumanDisposition(ctx, db.KnowledgeAutomationHasHumanDispositionParams{SourceIssueID: job.SourceIssueID, ProjectID: job.ProjectID, WorkspaceID: job.SourceWorkspaceID, Content: candidate.Content, Applicability: applicability})
			if err != nil {
				return false, err
			}
			if blocked {
				outcome, reason = "review_pending", "human_decision_changed"
			}
			if outcome == "accept" {
				for _, existing := range capacity {
					if existing.ID == predecessor.ID {
						continue
					}
					var scope protocol.CollaborationKnowledgeApplicability
					if json.Unmarshal(existing.Applicability, &scope) != nil {
						return false, automationPending("comparison_unavailable")
					}
					if slices.Equal(scope.Surfaces, knowledgeAutomationApplicability.Surfaces) && normalizeKnowledgeAutomationContent(existing.Content) == normalizeKnowledgeAutomationContent(candidate.Content) {
						if candidate.Intent == "create" {
							outcome, reason = "duplicate", "exact_duplicate"
						} else {
							outcome, reason = "review_pending", "duplicate_other_knowledge"
						}
						break
					}
				}
			}
			if outcome == "duplicate" || outcome == "transient" {
				result.Outcomes = append(result.Outcomes, knowledgeAutomationOutcome{ExecutionRevision: job.ExecutionRevision, Ref: originalRef, Outcome: outcome, ReasonCode: reason})
				continue
			}
			row := predecessor
			if candidate.Intent != "revalidate" {
				id := knowledgeAutomationID(batch.ID, "candidate:"+strconv.FormatInt(job.ExecutionRevision, 10)+":"+originalRef)
				row, err = q.GetKnowledgeCandidate(ctx, id)
				if errors.Is(err, pgx.ErrNoRows) {
					sources, _ := json.Marshal(candidate.Sources)
					row, err = q.CreateAutomatedKnowledgeCandidate(ctx, db.CreateAutomatedKnowledgeCandidateParams{ID: id, JobID: job.ID, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Content: candidate.Content, Summary: candidate.Summary, ContentSha256: protocol.CollaborationKnowledgeHash(candidate.Content), Sources: sources, Evidence: []byte(`[]`), Applicability: applicability, ExpiresAt: predecessor.ExpiresAt, SupersedesCandidateID: predecessor.ID, ExpectedPreviousVersion: predecessor.Version, ExpectedPreviousStateRevision: predecessor.StateRevision})
				}
				if err != nil {
					return false, err
				}
				if row.State != "pending" || row.ProposalOrigin != "automation" || row.AutomationJobID != job.ID || row.Content != candidate.Content || row.Summary != candidate.Summary {
					return false, automationPending("candidate_changed")
				}
			}
			if outcome == "accept" {
				items := []protocol.CollaborationKnowledgeContribution{}
				for _, existing := range capacity {
					if existing.LineageID == row.LineageID {
						continue
					}
					item, e := knowledgeContribution(existing)
					if e != nil {
						return false, e
					}
					items = append(items, item)
				}
				item, e := knowledgeContribution(row)
				if e != nil {
					return false, e
				}
				items = append(items, item)
				if protocol.ValidateCollaborationKnowledgeTargetBudget(items) != nil {
					outcome, reason = "review_pending", "knowledge_target_budget_exceeded"
				}
			}
			dependencies := append([]protocol.CollaborationKnowledgeSource{}, candidate.Sources...)
			if !slices.Contains(dependencies, snapshot.Sources[0]) {
				dependencies = append(dependencies, snapshot.Sources[0])
			}
			support, _ := json.Marshal(dependencies)
			modelResult, _ := json.Marshal(check)
			verdict := "review_pending"
			if outcome == "accept" {
				verdict = "pass"
			}
			evaluation, err := q.CreateKnowledgeEvaluation(ctx, db.CreateKnowledgeEvaluationParams{ID: knowledgeAutomationID(batch.ID, "evaluation:"+candidate.Ref+":"+knowledgeUUIDString(current.CheckAttemptID)), JobID: job.ID, BatchID: batch.ID, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration,
				CandidateID: row.ID, CandidateVersion: row.Version, CandidateContentSha256: row.ContentSha256, Intent: candidate.Intent, PredecessorCandidateID: predecessor.ID, Verdict: verdict, ReasonCode: reason, SupportDependencies: support, ModelResult: modelResult})
			if err != nil {
				return false, err
			}
			if outcome == "accept" {
				if !advanced {
					counter, err = q.AdvanceKnowledgeTarget(ctx, db.AdvanceKnowledgeTargetParams{TargetKind: "project", TargetID: job.ProjectID, KnowledgeRevision: counter.KnowledgeRevision})
					if err != nil {
						return false, err
					}
					advanced = true
				}
				if candidate.Intent != "revalidate" {
					if predecessor.ID.Valid {
						if _, err = q.SupersedeAutomatedKnowledgeCandidate(ctx, db.SupersedeAutomatedKnowledgeCandidateParams{ID: predecessor.ID, ReplacementID: row.ID, JobID: job.ID, ExpectedStateRevision: predecessor.StateRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration}); err != nil {
							return false, err
						}
					}
					row, err = q.ApproveAutomatedKnowledgeCandidate(ctx, db.ApproveAutomatedKnowledgeCandidateParams{ID: row.ID, JobID: job.ID, EvaluationID: evaluation.ID, ExpectedStateRevision: row.StateRevision, ExpectedVersion: row.Version, ExpectedContentSha256: row.ContentSha256, TargetRevision: pgtype.Int8{Int64: counter.KnowledgeRevision, Valid: true}, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration})
					if err != nil {
						return false, err
					}
				}
				revision := int64(0)
				if candidate.Intent == "revalidate" {
					head, e := q.GetKnowledgeValidity(ctx, row.ID)
					if e != nil {
						return false, e
					}
					revision = head.Revision
				}
				if _, err = q.SetKnowledgeValidity(ctx, db.SetKnowledgeValidityParams{CandidateID: row.ID, EvaluationID: evaluation.ID, ExpectedStateRevision: row.StateRevision, ExpectedRevision: revision}); err != nil {
					return false, err
				}
				next := capacity[:0]
				for _, existing := range capacity {
					if existing.LineageID != row.LineageID {
						next = append(next, existing)
					}
				}
				capacity = append(next, row)
				outcome = "saved"
				if candidate.Intent == "revalidate" {
					outcome = "restored"
				} else if candidate.Intent == "revise" {
					outcome = "revised"
				}
			} else {
				pending = true
			}
			result.Outcomes = append(result.Outcomes, knowledgeAutomationOutcome{ExecutionRevision: job.ExecutionRevision, Ref: originalRef, CandidateID: knowledgeUUIDString(row.ID), Outcome: outcome, ReasonCode: reason})
		}
		if advanced {
			if _, err = q.EnqueueKnowledgeTargetHint(ctx, db.EnqueueKnowledgeTargetHintParams{ID: knowledgeAutomationID(batch.ID, "hint"), CauseID: batch.ID, TargetKind: pgtype.Text{String: "project", Valid: true}, TargetID: job.ProjectID, TargetWorkspaceID: job.SourceWorkspaceID, KnowledgeRevision: pgtype.Int8{Int64: counter.KnowledgeRevision, Valid: true}}); err != nil {
				return false, err
			}
		}
		phase := "completed"
		if pending {
			phase = "review_pending"
		}
		raw, _ := json.Marshal(result)
		_, err = q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: phase, GenerationAttemptID: current.GenerationAttemptID, CheckAttemptID: current.CheckAttemptID, Result: raw})
		return true, err
	})
	return err
}
