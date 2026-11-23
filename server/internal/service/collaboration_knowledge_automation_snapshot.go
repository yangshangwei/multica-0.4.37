package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeAutomationStop struct{ Reason, Phase string }

func (e *knowledgeAutomationStop) Error() string { return e.Reason }
func automationPending(reason string) error {
	return &knowledgeAutomationStop{reason, "review_pending"}
}

type knowledgeAutomationSnapshot struct {
	Input                  knowledgeAutomationInput
	Job                    db.CollaborationKnowledgeJob
	Issue                  db.Issue
	Counter                db.CollaborationKnowledgeTarget
	Rows                   []db.CollaborationKnowledgeCandidate
	Predecessors           []db.CollaborationKnowledgeCandidate
	OriginalRefs           []string
	Sources                []protocol.CollaborationKnowledgeSource
	Closure                []protocol.CollaborationKnowledgeSource
	SourceHash, TargetHash string
	CheckedAt              time.Time
}

var knowledgeAutomationApplicability = protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat", "quick_create", "autopilot", "collaboration"}}

// This transaction stabilizes exactly the sources sent to a stage and the
// complete comparison set. It grants a narrow server purpose, never a fake
// human principal. The real acceptance reader is checked directly.
func (w *KnowledgeAutomationWorker) snapshot(ctx context.Context, q *db.Queries, job db.CollaborationKnowledgeJob, batch *db.CollaborationKnowledgeBatch) (knowledgeAutomationSnapshot, error) {
	out := knowledgeAutomationSnapshot{Job: job, Input: knowledgeAutomationInput{Sources: []knowledgeAutomationSource{}, Predecessors: []knowledgeAutomationPredecessor{}, Comparisons: []knowledgeAutomationComparison{}, MaxCandidates: int(job.ExecutionMaxCandidates)}}
	settledRefs := map[string]bool{}
	if batch != nil {
		_, refs, err := knowledgeAutomationRetained(*batch)
		if err != nil {
			return out, err
		}
		settledRefs = refs
	}
	users, err := q.LockKnowledgeUsers(ctx, []pgtype.UUID{job.SourceReaderUserID})
	if err != nil {
		return out, err
	}
	if len(users) != 1 || users[0].DisabledAt.Valid {
		return out, automationPending("source_permission_lost")
	}
	read, err := knowledgeUserReadsSource(ctx, q, job.SourceIssueID, job.SourceReaderUserID)
	if err != nil {
		return out, err
	}
	if !read {
		return out, automationPending("source_permission_lost")
	}
	metadata, err := q.ListKnowledgeTargetLockHints(ctx, db.ListKnowledgeTargetLockHintsParams{TargetKind: "project", TargetWorkspaceID: job.SourceWorkspaceID, ProjectID: job.ProjectID})
	if err != nil {
		return out, err
	}
	if len(metadata) > 16 {
		return out, automationPending("target_budget_exceeded")
	}
	hints := KnowledgeReadLockHints{Sources: []KnowledgeSourceLockHint{}, CandidateIDs: []pgtype.UUID{}, Validities: map[pgtype.UUID]knowledgeAutomaticProof{}}
	lineageRoots := []pgtype.UUID{}
	groups := map[pgtype.UUID]*KnowledgeSourceLockHint{}
	add := func(issue, workspace pgtype.UUID, refs []protocol.CollaborationKnowledgeSource) {
		g := groups[issue]
		if g == nil {
			g = &KnowledgeSourceLockHint{IssueID: issue, WorkspaceID: workspace, Sources: []protocol.CollaborationKnowledgeSource{}}
			groups[issue] = g
		}
		for _, ref := range refs {
			if !slices.Contains(g.Sources, ref) {
				g.Sources = append(g.Sources, ref)
			}
		}
	}
	currentRef := protocol.CollaborationKnowledgeSource{Kind: "delivery", IssueID: knowledgeUUIDString(job.SourceIssueID), SubmissionID: knowledgeUUIDString(job.SubmissionID), DeliveryEventSeq: strconv.FormatInt(job.DeliveryEventSeq, 10), AcceptedEventSeq: strconv.FormatInt(job.AcceptedEventSeq, 10)}
	add(job.SourceIssueID, job.SourceWorkspaceID, []protocol.CollaborationKnowledgeSource{currentRef})
	for _, row := range metadata {
		refs, err := knowledgeCandidateSources(db.CollaborationKnowledgeCandidate{Sources: row.Sources, SourceIssueID: row.SourceIssueID})
		if err != nil {
			return out, err
		}
		add(row.SourceIssueID, row.SourceWorkspaceID, refs)
		hints.CandidateIDs = append(hints.CandidateIDs, row.ID)
	}
	if batch != nil {
		for index, id := range batch.PredecessorIds {
			if settledRefs["k"+strconv.Itoa(index+1)] {
				continue
			}
			row, err := q.GetKnowledgeCandidate(ctx, id)
			if err != nil {
				return out, automationPending("predecessor_changed")
			}
			refs, err := knowledgeCandidateSources(row)
			if err != nil {
				return out, err
			}
			add(row.SourceIssueID, row.SourceWorkspaceID, refs)
			hints.CandidateIDs = append(hints.CandidateIDs, id)
			lineageRoots = append(lineageRoots, row.LineageID)
		}
	}
	slices.SortFunc(hints.CandidateIDs, knowledgeCompareUUID)
	hints.CandidateIDs = slices.Compact(hints.CandidateIDs)
	heads, err := q.ListKnowledgeValidityLockHints(ctx, hints.CandidateIDs)
	if err != nil {
		return out, err
	}
	for _, head := range heads {
		deps, valid := knowledgeSupportDependencies(head.SupportDependencies)
		if !valid {
			return out, automationPending("comparison_unavailable")
		}
		proof := knowledgeAutomaticProof{Binding: protocol.TaskKnowledgeValidity{Revision: strconv.FormatInt(head.Revision, 10), AcceptedEventSeq: strconv.FormatInt(head.AcceptedEventSeq, 10), SubmissionID: knowledgeUUIDString(head.SubmissionID), ValidatedCloseEpoch: strconv.FormatInt(head.ValidatedCloseEpoch, 10)}, EvaluationID: head.EvaluationID, Dependencies: deps}
		hints.Validities[head.CandidateID] = proof
		for _, ref := range deps {
			id, e := util.ParseUUID(ref.IssueID)
			if e != nil || id != head.SourceIssueID || groups[id] == nil {
				return out, automationPending("comparison_unavailable")
			}
			add(id, groups[id].WorkspaceID, []protocol.CollaborationKnowledgeSource{ref})
		}
	}
	// Root rows serialize version allocation. Their old proof dependencies do
	// not become inputs merely because an inactive root is retained history.
	hints.CandidateIDs = append(hints.CandidateIDs, lineageRoots...)
	slices.SortFunc(hints.CandidateIDs, knowledgeCompareUUID)
	hints.CandidateIDs = slices.Compact(hints.CandidateIDs)
	workspaces := []pgtype.UUID{job.SourceWorkspaceID}
	for _, g := range groups {
		hints.Sources = append(hints.Sources, *g)
		workspaces = append(workspaces, g.WorkspaceID)
	}
	slices.SortFunc(workspaces, knowledgeCompareUUID)
	workspaces = slices.Compact(workspaces)
	for _, workspace := range workspaces {
		if _, err = q.LockKnowledgeWorkspace(ctx, workspace); err != nil {
			return out, err
		}
		if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: workspace, UserID: job.SourceReaderUserID}); err != nil {
			return out, err
		}
		if _, err = q.LockKnowledgeMember(ctx, db.LockKnowledgeMemberParams{WorkspaceID: workspace, UserID: job.SourceReaderUserID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
	}
	if _, err = q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: job.ProjectID, WorkspaceID: job.SourceWorkspaceID}); err != nil {
		return out, &knowledgeAutomationStop{"project_unavailable", "cancelled"}
	}
	if err = knowledgeLockPolicyFence(ctx, q, job.ProjectID); err != nil {
		return out, err
	}
	policy, err := knowledgeEffectivePolicy(ctx, q, job.ProjectID, job.SourceWorkspaceID)
	if err != nil {
		return out, err
	}
	if !policy.IsEnabled {
		return out, &knowledgeAutomationStop{"automation_disabled", "skipped"}
	}
	if policy.Revision != strconv.FormatInt(job.ExecutionPolicyRevision, 10) || policy.RuleVersion != job.ExecutionRuleVersion || policy.MaxCandidates != int(job.ExecutionMaxCandidates) {
		return out, automationPending("policy_changed")
	}
	target := protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: knowledgeUUIDString(job.SourceWorkspaceID), ProjectID: knowledgeUUIDString(job.ProjectID)}
	out.Counter, err = lockKnowledgeCounter(ctx, q, target, false)
	if err != nil {
		return out, err
	}
	slices.SortFunc(hints.Sources, func(a, b KnowledgeSourceLockHint) int { return knowledgeCompareUUID(a.IssueID, b.IssueID) })
	if err = w.Service.LockKnowledgeReadSources(ctx, q, hints); err != nil {
		return out, err
	}
	if _, err = q.LockKnowledgeCandidates(ctx, hints.CandidateIDs); err != nil {
		return out, err
	}
	out.Job, err = q.LockKnowledgeAutomationJob(ctx, job.ID)
	if err != nil {
		return out, err
	}
	if out.Job.LeaseToken != job.LeaseToken || out.Job.LeaseGeneration != job.LeaseGeneration || out.Job.ExecutionRevision != job.ExecutionRevision {
		return out, automationPending("lease_changed")
	}
	now, err := q.GetKnowledgeDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	out.CheckedAt = now.Time
	if !out.Job.LeaseExpiresAt.Valid || !out.Job.LeaseExpiresAt.Time.After(now.Time) {
		return out, automationPending("lease_changed")
	}
	out.Issue, err = q.GetIssue(ctx, job.SourceIssueID)
	if err != nil {
		return out, automationPending("source_unavailable")
	}
	state, err := q.GetKnowledgeSourceState(ctx, job.SourceIssueID)
	if err != nil {
		return out, err
	}
	session, err := q.GetCollaborationSession(ctx, job.SourceIssueID)
	if err != nil {
		return out, err
	}
	if out.Issue.ProjectID != job.ProjectID || state.LifecycleEpoch != job.ValidatedCloseEpoch || session.QueueState != "closed" {
		return out, &knowledgeAutomationStop{"source_changed", "cancelled"}
	}
	if state.AiPolicy == "manual" {
		return out, &knowledgeAutomationStop{"task_manual", "skipped"}
	}
	for _, g := range hints.Sources {
		ok, e := knowledgeUserReadsSource(ctx, q, g.IssueID, job.SourceReaderUserID)
		if e != nil {
			return out, e
		}
		if !ok {
			return out, automationPending("comparison_unavailable")
		}
	}
	delivery, available, err := knowledgeAcceptedDelivery(ctx, q, out.Issue, job.SubmissionID, job.DeliveryEventSeq, job.AcceptedEventSeq)
	if err != nil {
		return out, err
	}
	if !available || delivery.AcceptedByUserID != job.SourceReaderUserID {
		return out, automationPending("source_unavailable")
	}
	if batch != nil && batch.Intent == "create" {
		published, e := q.CountKnowledgeAutomationPublishedLineages(ctx, job.ID)
		if e != nil {
			return out, e
		}
		if published >= int64(out.Input.MaxCandidates) {
			return out, automationPending("new_candidate_budget_exceeded")
		}
		out.Input.MaxCandidates -= int(published)
	}
	out.Sources = []protocol.CollaborationKnowledgeSource{delivery.Source}
	out.Input.Sources = append(out.Input.Sources, knowledgeAutomationSource{Ref: "s1", Source: delivery.Source, Result: delivery.Result, Basis: delivery.Basis, Canonical: delivery.Canonical})
	if batch != nil {
		for index, id := range batch.PredecessorIds {
			originalRef := "k" + strconv.Itoa(index+1)
			if settledRefs[originalRef] {
				continue
			}
			row, err := q.GetKnowledgeCandidate(ctx, id)
			if err != nil {
				return out, automationPending("predecessor_changed")
			}
			humanOverride, err := q.KnowledgeLineageHasNewerHumanDecision(ctx, db.KnowledgeLineageHasNewerHumanDecisionParams{LineageID: row.LineageID, Version: row.Version})
			if err != nil {
				return out, err
			}
			if row.State != "approved" || row.ApprovalOrigin != "automation" || row.SourceIssueID != job.SourceIssueID || row.TargetProjectID != job.ProjectID || humanOverride {
				return out, automationPending("human_decision_changed")
			}
			var scope protocol.CollaborationKnowledgeApplicability
			if json.Unmarshal(row.Applicability, &scope) != nil || !slices.Equal(scope.Surfaces, knowledgeAutomationApplicability.Surfaces) {
				return out, automationPending("candidate_scope_changed")
			}
			refs, err := knowledgeCandidateSources(row)
			if err != nil {
				return out, err
			}
			if proof, exists := hints.Validities[row.ID]; exists {
				refs = append(refs, proof.Dependencies...)
			}
			for _, ref := range refs {
				if ref.IssueID != knowledgeUUIDString(job.SourceIssueID) {
					return out, automationPending("source_unavailable")
				}
				if slices.Contains(out.Sources, ref) {
					continue
				}
				body, ok, e := knowledgeSourceText(ctx, q, out.Issue, ref)
				if e != nil {
					return out, e
				}
				if !ok || protocol.CollaborationKnowledgeHash(body) != ref.SHA256 {
					return out, automationPending("source_unavailable")
				}
				material := knowledgeAutomationSource{Ref: "s" + strconv.Itoa(len(out.Sources)+1), Source: ref, Result: body, Canonical: body}
				if ref.Kind == "delivery" {
					var text struct {
						Result string `json:"result"`
						Basis  string `json:"basis"`
					}
					if json.Unmarshal([]byte(body), &text) != nil {
						return out, automationPending("source_unavailable")
					}
					material.Result, material.Basis = text.Result, text.Basis
				}
				out.Sources = append(out.Sources, ref)
				out.Input.Sources = append(out.Input.Sources, material)
			}
			out.Predecessors = append(out.Predecessors, row)
			out.OriginalRefs = append(out.OriginalRefs, originalRef)
			out.Input.Predecessors = append(out.Input.Predecessors, knowledgeAutomationPredecessor{Ref: "k" + strconv.Itoa(len(out.Predecessors)), Content: row.Content, Summary: row.Summary})
		}
	}
	predecessorIDs := []pgtype.UUID{}
	for _, row := range out.Predecessors {
		predecessorIDs = append(predecessorIDs, row.ID)
	}
	rows, err := q.ListKnowledgeAutomationComparisonRows(ctx, db.ListKnowledgeAutomationComparisonRowsParams{TargetProjectID: job.ProjectID, TargetWorkspaceID: job.SourceWorkspaceID, CheckedAt: now, PredecessorIds: predecessorIDs})
	if err != nil {
		return out, err
	}
	if len(rows) > 16 {
		return out, automationPending("target_budget_exceeded")
	}
	var capacity []protocol.CollaborationKnowledgeContribution
	out.Closure = slices.Clone(out.Sources)
	for _, row := range rows {
		predecessor := ""
		for index, p := range out.Predecessors {
			if p.ID == row.ID {
				predecessor = "k" + strconv.Itoa(index+1)
			}
		}
		if row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(now.Time) && predecessor == "" {
			continue
		}
		// Discovery precedes the target lock. A publication in that interval
		// cannot add a body whose source, membership and candidate were not
		// included in our lock graph. Retry the whole transaction before any
		// request is recorded or sent; never compare only the old readable set.
		group := groups[row.SourceIssueID]
		if !slices.Contains(hints.CandidateIDs, row.ID) || group == nil || group.WorkspaceID != row.SourceWorkspaceID {
			return out, &pgconn.PgError{Code: "40001", Message: "knowledge comparison lock discovery changed"}
		}
		refs, e := knowledgeCandidateSources(row)
		if e != nil {
			return out, e
		}
		for _, ref := range refs {
			if !slices.Contains(group.Sources, ref) {
				return out, &pgconn.PgError{Code: "40001", Message: "knowledge comparison sources changed"}
			}
		}
		if row.ApprovalOrigin == "automation" {
			proof, found := hints.Validities[row.ID]
			if !found {
				return out, automationPending("comparison_unavailable")
			}
			for _, ref := range proof.Dependencies {
				if !slices.Contains(group.Sources, ref) {
					return out, automationPending("comparison_unavailable")
				}
			}
		}
		out.Rows = append(out.Rows, row)
		item, e := knowledgeContribution(row)
		if e != nil {
			return out, e
		}
		if !row.ExpiresAt.Valid || row.ExpiresAt.Time.After(now.Time) {
			capacity = append(capacity, item)
		}
		available, e := w.Service.KnowledgeSourceAvailability(ctx, q, row.SourceIssueID, refs)
		if e != nil {
			return out, e
		}
		if !available {
			return out, automationPending("comparison_unavailable")
		}
		proof, active, e := knowledgeAutomaticValidity(ctx, q, row)
		if e != nil {
			return out, e
		}
		if !active && row.SourceIssueID != job.SourceIssueID && predecessor == "" {
			continue
		}
		if head, ok := hints.Validities[row.ID]; ok {
			refs = append(refs, head.Dependencies...)
		} else {
			refs = append(refs, proof.Dependencies...)
		}
		for _, ref := range refs {
			if !slices.Contains(out.Closure, ref) {
				out.Closure = append(out.Closure, ref)
			}
		}
		out.Input.Comparisons = append(out.Input.Comparisons, knowledgeAutomationComparison{Ref: "c" + strconv.Itoa(len(out.Input.Comparisons)+1), Content: row.Content, Summary: row.Summary, PredecessorFor: predecessor})
	}
	if err = protocol.ValidateCollaborationKnowledgeTargetBudget(capacity); err != nil {
		return out, automationPending("target_budget_exceeded")
	}
	if len(out.Sources) > 8 {
		return out, automationPending("source_budget_exceeded")
	}
	sourceRaw, _ := json.Marshal(struct {
		Sources      []knowledgeAutomationSource
		Predecessors []db.CollaborationKnowledgeCandidate
	}{out.Input.Sources, out.Predecessors})
	targetRaw, _ := json.Marshal(struct {
		Revision    int64
		Rows        []db.CollaborationKnowledgeCandidate
		Comparisons []knowledgeAutomationComparison
		Closure     []protocol.CollaborationKnowledgeSource
	}{out.Counter.KnowledgeRevision, out.Rows, out.Input.Comparisons, out.Closure})
	out.SourceHash, out.TargetHash = protocol.CollaborationKnowledgeHash(string(sourceRaw)), protocol.CollaborationKnowledgeHash(string(targetRaw))
	return out, nil
}
