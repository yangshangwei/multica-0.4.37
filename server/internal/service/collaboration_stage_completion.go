package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// StageCompletionError contains only bounded, source-safe refusal facts.
// It does not contain retained evidence or confer artifact read authority.
type StageCompletionError struct{ Blockers []CollaborationStageBlocker }

func (e *StageCompletionError) Error() string { return "stage_incomplete" }
func (e *StageCompletionError) Unwrap() error { return collaborationError(409, "stage_incomplete") }

func stageCompletionRefusal(stage, code string) error {
	if stage == "" {
		stage = "development"
	}
	return &StageCompletionError{Blockers: []CollaborationStageBlocker{{Code: code, Stage: stage}}}
}

type stageCompletionEvidence struct {
	state     *collaborationStageState
	candidate db.CollaborationStageCandidate
	release   db.CollaborationStageDecision
	fact      db.CollaborationStageExecutionObservation
	expires   time.Time
}

// stageMutationGuard belongs to one actual writer transaction and one allowed
// transition. It cannot be supplied by a client or reused in another tx.
type stageMutationGuard struct {
	tx              pgx.Tx
	sources         StageExecutionSources
	access          CollaborationIssueAccess
	before          db.Issue
	target          DeliveryCompletionTarget
	evidence        stageCompletionEvidence
	credential      ArtifactCredentialSnapshot
	operation       db.CollaborationOperation
	admin           *db.AdminOperation
	delivery        *DeliveryPreparedMutation
	deliveryTarget  DeliveryCompletionTarget
	deliveryDigest  string
	operationResult []byte
	writerResult    DeliveryWriterResult
	boundSubmission pgtype.UUID
	submit          bool
	targetStatus    string
	finished        bool
}

func checkStageSubmissionSchema(ctx context.Context, tx pgx.Tx) error {
	var version string
	err := tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE version='864_collaboration_stage_submission_ready' FOR SHARE NOWAIT`).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return collaborationError(503, "stage_schema_unavailable")
	}
	if err != nil {
		return err
	}
	q := db.New(tx)
	ready, err := q.CollaborationStageSubmissionReady(ctx)
	if err != nil {
		return err
	}
	if !ready.Valid || !ready.Bool {
		return collaborationError(503, "stage_schema_unavailable")
	}
	return nil
}

func stageCompletionState(ctx context.Context, tx pgx.Tx, issue db.Issue, lifecycle int64) (*collaborationStageState, error) {
	q := db.New(tx)
	shared, err := q.IsCollaborationIssue(ctx, issue.ID)
	if err != nil || !shared || !issue.ProjectID.Valid {
		return nil, err
	}
	if _, err = q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID}); err != nil {
		return nil, err
	}
	if err = stageReady(ctx, q); err != nil {
		return nil, err
	}
	policy, _, err := currentStagePolicy(ctx, q, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, nil
	}
	if policy.Contract != collaboration.StageContract {
		return nil, collaborationError(503, "stage_schema_unavailable")
	}
	if err = stageExecutionReady(ctx, q); err != nil {
		return nil, err
	}
	if err = checkStageSubmissionSchema(ctx, tx); err != nil {
		return nil, err
	}
	state := &collaborationStageState{policy: policy}
	if state.lastDisabled, err = q.GetProjectStagePolicyLastDisabled(ctx, db.GetProjectStagePolicyLastDisabledParams{WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID}); err != nil {
		return nil, err
	}
	if err = loadStageDetails(ctx, q, issue, lifecycle, state, true); err != nil {
		return nil, err
	}
	return state, nil
}

// Root completion consumes retained approvers' current artifact authority. The
// actual completer keeps their own identity and need not read that artifact.
func stageCompletionFacts(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, access CollaborationIssueAccess) (out stageCompletionEvidence, err error) {
	return stageCompletionFactsWithFinalCheck(ctx, tx, sources, access, nil)
}

func stageCompletionFactsWithFinalCheck(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, access CollaborationIssueAccess, finalCheck func() (time.Time, error)) (out stageCompletionEvidence, err error) {
	q := db.New(tx)
	state, err := stageCompletionState(ctx, tx, access.Issue, access.LifecycleEpoch)
	if err != nil || state == nil {
		return out, err
	}
	out.state = state
	refuse := func(code string) (stageCompletionEvidence, error) {
		if !access.SourceMember && code != "stage_roles_unconfigured" && code != "stage_scope_unconfirmed" && code != "stage_not_reached" {
			code = "stage_evidence_unavailable"
		}
		return out, stageCompletionRefusal(state.task.Stage, code)
	}
	if state.role == nil {
		return refuse("stage_roles_unconfigured")
	}
	if len(collaboration.StageMissing(state.facts)) != 0 {
		return refuse("stage_scope_unconfirmed")
	}
	if state.task.Stage != "verification" {
		return refuse("stage_not_reached")
	}
	out.candidate, err = q.LockCollaborationStageCompletionCodeHead(ctx, db.LockCollaborationStageCompletionCodeHeadParams{IssueID: access.Issue.ID, WorkspaceID: access.Issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return refuse("stage_code_candidate_missing")
	}
	if err != nil {
		return out, err
	}
	content, err := decodeStageCode(out.candidate)
	if err != nil {
		return out, err
	}
	child, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: util.MustParseUUID(content.Baseline.ChildIssueID), WorkspaceID: access.Issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return refuse("stage_evidence_unavailable")
	}
	if err != nil {
		return out, err
	}
	if child.ParentIssueID != access.Issue.ID || child.ProjectID != access.Issue.ProjectID {
		return refuse("stage_candidate_stale")
	}
	if err = lockStageCodeEvidence(ctx, tx); err != nil {
		return out, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	observed, code, err := stageCodeCurrent(ctx, q, access, state, out.candidate, content, now.Time)
	if err != nil {
		return out, err
	}
	if code != "" {
		return refuse(code)
	}
	if code = stageCIRefusal(observed.CI); code != "" {
		return refuse(code)
	}
	out.release, err = q.GetCollaborationStageReviewHead(ctx, db.GetCollaborationStageReviewHeadParams{IssueID: access.Issue.ID, CandidateID: out.candidate.CandidateID, Step: "release"})
	if errors.Is(err, pgx.ErrNoRows) {
		return refuse("stage_release_approval_missing")
	}
	if err != nil {
		return out, err
	}
	selected := out.release
	rollback, e := q.GetCollaborationStageReviewHead(ctx, db.GetCollaborationStageReviewHeadParams{IssueID: access.Issue.ID, CandidateID: out.candidate.CandidateID, Step: "rollback"})
	if e == nil && rollback.Decision == "approved" {
		selected = rollback
	} else if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	chain, err := stageExecutionApprovalChain(ctx, tx, sources, access, state, out.candidate, content, selected, observed, now.Time)
	if err != nil {
		if code := stageExecutionCode(err); code != "" {
			if strings.HasPrefix(code, "stage_execution_") || strings.HasPrefix(code, "stage_rollback_") {
				code = "stage_evidence_changed"
			}
			return refuse(code)
		}
		return out, err
	}
	intent, err := q.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: access.Issue.ID, ApprovalDecisionID: selected.DecisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return refuse("stage_execution_fact_missing")
	}
	if err != nil {
		return out, err
	}
	out.fact, err = q.GetStageExecutionTerminal(ctx, intent.IntentID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && out.fact.Result != "succeeded" {
		return refuse("stage_execution_fact_missing")
	}
	if err != nil {
		return out, err
	}
	invocation, err := stageExecutionInvocation(intent)
	if err != nil {
		return out, err
	}
	var fact StageExecutionObservation
	if json.Unmarshal(out.fact.Observation, &fact) != nil || !stageObservationValid(invocation, fact, now.Time) {
		return refuse("stage_evidence_changed")
	}
	target, err := stageResolveExecutionTarget(ctx, tx, sources, access.Issue.WorkspaceID, intent.TargetDigest)
	if err != nil {
		if code := stageExecutionCode(err); code != "" {
			return refuse("stage_evidence_unavailable")
		}
		return out, err
	}
	if intent.CandidateID != out.candidate.CandidateID || invocation.ArtifactDigest != selectedArtifact(selected) || target.ConditionalVersion != fact.AfterConditionalVersion || !reflect.DeepEqual(target.CurrentArtifactDigest, fact.AfterArtifactDigest) {
		return refuse("stage_evidence_changed")
	}
	// Reobserve the ordinary child as a retained issuer, never as the completer.
	if content.Baseline.Artifact == nil {
		return refuse("stage_evidence_unavailable")
	}
	current, signer, err := stageSignerCurrent(ctx, q, state.role, out.release, now.Time)
	if err != nil {
		return out, err
	}
	if !current {
		return refuse("stage_signer_changed")
	}
	observation, err := stageArtifactObservationAs(ctx, tx, child, util.MustParseUUID(content.Baseline.Artifact.ApprovalID), content.Baseline.PullRequest.HeadSHA, signer.credential(util.UUIDToString(out.release.ActorUserID)))
	if stageArtifactUnavailable(err) {
		return refuse("stage_signer_changed")
	}
	if err != nil {
		return out, err
	}
	original := content.Source.ArtifactObservation
	if observation.Verdict != "pass" || original == nil || observation.ArtifactDigest != content.Baseline.Artifact.ArtifactDigest || observation.ReceiptDigest != original.ReceiptDigest || observation.ExecutionID != original.ExecutionID || observation.ProducerTaskGeneration != original.ProducerTaskGeneration {
		return refuse("stage_evidence_changed")
	}
	if observation.ValidUntil == nil || original.ValidUntil == nil {
		return refuse("stage_evidence_unavailable")
	}
	out.expires = stageEarlier(*observation.ValidUntil, *original.ValidUntil)
	var rollbackDeployment StageExecutionDeployment
	var rollbackMetric *CollaborationStageRollbackRef
	if selected.Step == "rollback" {
		payload, err := stageExecutionPayload(selected)
		if err != nil {
			return out, err
		}
		if payload.Execution == nil || payload.Execution.DeploymentID == nil || payload.Execution.Metric == nil {
			return refuse("stage_evidence_unavailable")
		}
		deployment, _, err := stageExecutionDeploymentFor(ctx, q, util.MustParseUUID(*payload.Execution.DeploymentID), access.Issue.ID, out.candidate.CandidateID)
		if err != nil {
			return out, err
		}
		metric, err := stageCurrentRollbackMetric(ctx, tx, sources, access.Issue.ID, out.candidate.CandidateID, deployment, *payload.Execution.Metric, now.Time)
		if err != nil {
			if stageExecutionCode(err) != "" {
				return refuse("stage_evidence_changed")
			}
			return out, err
		}
		out.expires = stageEarlier(out.expires, metric.ValidUntil)
		rollbackDeployment, rollbackMetric = deployment, payload.Execution.Metric
	}
	// Original result/M1 and all potentially slow source checks precede the
	// final wall clock. The trusted sources retain their current trust locks.
	finalNow, authorityExpires, code, err := checkStageFinalAuthorityWithFinalCheck(ctx, tx, access, state, chain, func() (time.Time, error) {
		var extraExpiry time.Time
		if finalCheck != nil {
			var checkErr error
			extraExpiry, checkErr = finalCheck()
			if checkErr != nil {
				return time.Time{}, checkErr
			}
		}
		finalTarget, checkErr := stageResolveExecutionTarget(ctx, tx, sources, access.Issue.WorkspaceID, intent.TargetDigest)
		if checkErr != nil {
			if stageExecutionCode(checkErr) != "" {
				checkErr = stageCompletionRefusal(state.task.Stage, "stage_evidence_changed")
			}
			return time.Time{}, checkErr
		}
		if !reflect.DeepEqual(finalTarget, target) || finalTarget.ConditionalVersion != fact.AfterConditionalVersion || !reflect.DeepEqual(finalTarget.CurrentArtifactDigest, fact.AfterArtifactDigest) {
			return time.Time{}, stageCompletionRefusal(state.task.Stage, "stage_evidence_changed")
		}
		if rollbackMetric != nil {
			metric, checkErr := stageCurrentRollbackMetric(ctx, tx, sources, access.Issue.ID, out.candidate.CandidateID, rollbackDeployment, *rollbackMetric, now.Time)
			if checkErr != nil {
				if stageExecutionCode(checkErr) != "" {
					checkErr = stageCompletionRefusal(state.task.Stage, "stage_evidence_changed")
				}
				return time.Time{}, checkErr
			}
			if extraExpiry.IsZero() || metric.ValidUntil.Before(extraExpiry) {
				extraExpiry = metric.ValidUntil
			}
		}
		return extraExpiry, nil
	})
	if err != nil {
		return out, err
	}
	if code == "forbidden" {
		return out, collaborationError(403, code)
	}
	if code != "" {
		return refuse(code)
	}
	if observed.CI.ObservedAt == nil || !finalNow.Before(observed.CI.ObservedAt.Add(stageCIHorizon)) {
		return refuse("stage_ci_stale")
	}
	out.expires = stageEarlier(out.expires, observed.CI.ObservedAt.Add(stageCIHorizon))
	if !authorityExpires.IsZero() {
		out.expires = stageEarlier(out.expires, authorityExpires)
	}
	if !finalNow.Before(out.expires) {
		return refuse("stage_evidence_changed")
	}
	return out, nil
}

func selectedArtifact(d db.CollaborationStageDecision) string {
	p, err := stageExecutionPayload(d)
	if err != nil || p.Execution == nil {
		return ""
	}
	return p.Execution.ArtifactDigest
}

func prepareStageMutation(ctx context.Context, tx pgx.Tx, in DeliveryMutationIntent, submit bool) (*stageMutationGuard, error) {
	q := db.New(tx)
	shared, err := q.IsCollaborationIssue(ctx, in.Before.ID)
	if err != nil || !shared {
		return nil, err
	}
	c, err := q.LockIssueCollaboration(ctx, in.Before.ID)
	if err != nil {
		return nil, err
	}
	state, err := stageCompletionState(ctx, tx, in.Before, c.LifecycleEpoch)
	if err != nil || state == nil {
		return nil, err
	}
	if in.Collaboration == nil {
		return nil, stageCompletionRefusal(state.task.Stage, "stage_submission_unbound")
	}
	g := &stageMutationGuard{tx: tx, sources: in.StageExecutionSources, before: in.Before, target: *in.Collaboration, credential: in.Credential, submit: submit, targetStatus: in.Candidate.Status}
	if err = g.prepareBinding(ctx, in); err != nil {
		return nil, err
	}
	if g.access.Revision != c.Revision || g.access.LifecycleEpoch != c.LifecycleEpoch || !reflect.DeepEqual(g.access.Issue, in.Before) {
		return nil, completionConflict()
	}
	g.evidence, err = stageCompletionFacts(ctx, tx, in.StageExecutionSources, g.access)
	if err != nil || g.evidence.state == nil {
		return nil, err
	}
	if !submit && g.target.Kind == "collaboration_submission" {
		if g.target.SubmissionID == nil || !stageUUID(*g.target.SubmissionID) {
			return nil, stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		g.boundSubmission = util.MustParseUUID(*g.target.SubmissionID)
		if err = g.checkSubmission(ctx, g.boundSubmission); err != nil {
			return nil, err
		}
	}
	if err = g.freezeDelivery(ctx, in.stageSubmissionInput); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *stageMutationGuard) checkSubmission(ctx context.Context, id pgtype.UUID) error {
	if !id.Valid || g.boundSubmission != id {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	row, err := db.New(g.tx).LockCollaborationStageSubmission(ctx, db.LockCollaborationStageSubmissionParams{IssueID: g.before.ID, WorkspaceID: g.before.WorkspaceID, SubmissionID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err != nil {
		return err
	}
	e := g.evidence
	if row.WorkspaceID != g.before.WorkspaceID || row.LifecycleEpoch != g.access.LifecycleEpoch || row.CandidateID != e.candidate.CandidateID || row.CandidateVersion != e.candidate.Version || row.ContentDigest != e.candidate.ContentDigest || row.PolicyRevision != e.state.policy.Revision || row.RoleRevision != e.state.task.RoleRevision || row.ScopeDecisionID != e.state.scope.DecisionID || row.StageRevision != e.state.task.StageRevision || row.ReleaseDecisionID != e.release.DecisionID || row.ExecutionObservationID != e.fact.ObservationID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	return g.checkSubmissionOperation(ctx, row)
}
func (g *stageMutationGuard) bindSubmission(ctx context.Context, id, operation pgtype.UUID) error {
	if g == nil {
		return nil
	}
	if g.finished || g.boundSubmission.Valid || !id.Valid || operation != g.operation.ID || !g.submit && g.target.Kind != "collaboration_self" {
		return completionConflict()
	}
	e := g.evidence
	_, err := db.New(g.tx).CreateCollaborationStageSubmission(ctx, db.CreateCollaborationStageSubmissionParams{SubmissionID: id, IssueID: g.before.ID, WorkspaceID: g.before.WorkspaceID, LifecycleEpoch: g.access.LifecycleEpoch, PolicyRevision: e.state.policy.Revision, RoleRevision: e.state.task.RoleRevision, StageRevision: e.state.task.StageRevision, ScopeDecisionID: e.state.scope.DecisionID, CandidateID: e.candidate.CandidateID, CandidateVersion: e.candidate.Version, ContentDigest: e.candidate.ContentDigest, ReleaseDecisionID: e.release.DecisionID, ExecutionObservationID: e.fact.ObservationID, OperationID: operation, ActorUserID: util.MustParseUUID(g.credential.Principal.UserID)})
	if err == nil {
		g.boundSubmission = id
	}
	return err
}

// FinishStageMutationInTx must be the last check after all writer side effects,
// including operation receipts, outbox/history and M1 terminal consumption.
func FinishStageMutationInTx(ctx context.Context, tx pgx.Tx, prepared DeliveryPrepareResult, after db.Issue, result DeliveryWriterResult) error {
	if tx == nil || !after.ID.Valid || !after.WorkspaceID.Valid {
		return completionConflict()
	}
	g := prepared.stage
	if g == nil {
		q := db.New(tx)
		shared, err := q.IsCollaborationIssue(ctx, after.ID)
		if err != nil || !shared {
			return err
		}
		current, err := q.LockIssueCollaboration(ctx, after.ID)
		if err != nil {
			return err
		}
		state, err := stageCompletionState(ctx, tx, after, current.LifecycleEpoch)
		if err != nil {
			return err
		}
		if state != nil {
			return stageCompletionRefusal(state.task.Stage, "stage_preparation_required")
		}
		return nil
	}
	if g.finished || reflect.TypeOf(tx) != reflect.TypeOf(g.tx) || !reflect.TypeOf(tx).Comparable() || g.tx != tx {
		return completionConflict()
	}
	if g.delivery == nil {
		if prepared.State != DeliveryNotApplicable || prepared.Prepared != nil || prepared.Receipt != nil {
			return completionConflict()
		}
	} else if prepared.State != DeliveryPrepared || prepared.Prepared != g.delivery || prepared.Receipt != nil || !g.delivery.finished || g.delivery.tx != tx {
		return completionConflict()
	}
	q := db.New(tx)
	actual, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: g.before.ID, WorkspaceID: g.before.WorkspaceID})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, after) || after.Status != g.targetStatus {
		return completionConflict()
	}
	comparison := after
	expectedRevision := g.before.Revision + 1
	if g.submit {
		comparison.AssigneeID = g.before.AssigneeID
		comparison.AssigneeType = g.before.AssigneeType
		expectedRevision = g.before.Revision
		if g.before.Status != g.targetStatus {
			expectedRevision++
		}
		if g.before.AssigneeID != g.before.CreatorID {
			expectedRevision++
		}
		if after.AssigneeID != g.before.CreatorID || after.AssigneeType.String != "member" {
			return completionConflict()
		}
	}
	if !completionContentUnchanged(g.before, comparison) || after.Revision != expectedRevision {
		return completionConflict()
	}
	c, err := q.LockIssueCollaboration(ctx, g.before.ID)
	if err != nil {
		return err
	}
	expectedLifecycle := g.access.LifecycleEpoch
	if !g.submit {
		expectedLifecycle++
	}
	if c.LifecycleEpoch != expectedLifecycle || c.Revision != g.access.Revision+1 || result.CollaborationRevision != c.Revision || !result.CollaborationOperationID.Valid {
		return completionConflict()
	}
	if c.AuthorityEpoch != g.access.AuthorityEpoch+1 {
		return stageCompletionRefusal("verification", "stage_evidence_changed")
	}
	if g.submit || g.target.Kind != "collaboration_admin" {
		if !result.SubmissionID.Valid {
			return completionConflict()
		}
		if err = g.checkSubmission(ctx, result.SubmissionID); err != nil {
			return err
		}
	}
	evidence, err := stageCompletionFactsWithFinalCheck(ctx, tx, g.sources, g.access, func() (time.Time, error) {
		if err := g.checkFinalBinding(ctx, after, result); err != nil {
			return time.Time{}, err
		}
		return g.checkFinalM1(ctx, after, result)
	})
	if err != nil {
		return err
	}
	if evidence.state == nil || evidence.candidate.CandidateID != g.evidence.candidate.CandidateID || evidence.candidate.Version != g.evidence.candidate.Version || evidence.candidate.ContentDigest != g.evidence.candidate.ContentDigest || evidence.release.DecisionID != g.evidence.release.DecisionID || evidence.fact.ObservationID != g.evidence.fact.ObservationID || evidence.state.task.StageRevision != g.evidence.state.task.StageRevision || evidence.state.task.RoleRevision != g.evidence.state.task.RoleRevision || evidence.state.policy.Revision != g.evidence.state.policy.Revision || evidence.state.scope.DecisionID != g.evidence.state.scope.DecisionID {
		return stageCompletionRefusal("verification", "stage_evidence_changed")
	}
	g.finished = true
	return nil
}
