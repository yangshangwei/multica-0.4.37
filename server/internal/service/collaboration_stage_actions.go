package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// CollaborationStageRoleSet names the real accounts of one configuration.
type CollaborationStageRoleSet struct {
	BusinessOwner   pgtype.UUID
	Implementer     pgtype.UUID
	ProductApprover pgtype.UUID
	CodeReviewer    pgtype.UUID
	QAApprover      pgtype.UUID
	ReleaseApprover pgtype.UUID
}

func (r CollaborationStageRoleSet) rules() collaboration.StageRoles {
	id := func(value pgtype.UUID) string {
		if !value.Valid || value.Bytes == [16]byte{} {
			return ""
		}
		return util.UUIDToString(value)
	}
	return collaboration.StageRoles{BusinessOwner: id(r.BusinessOwner), Implementer: id(r.Implementer), ProductApprover: id(r.ProductApprover),
		CodeReviewer: id(r.CodeReviewer), QAApprover: id(r.QAApprover), ReleaseApprover: id(r.ReleaseApprover)}
}

func (r CollaborationStageRoleSet) holders() []pgtype.UUID {
	seen := map[pgtype.UUID]bool{}
	var out []pgtype.UUID
	for _, id := range []pgtype.UUID{r.BusinessOwner, r.Implementer, r.ProductApprover, r.CodeReviewer, r.QAApprover, r.ReleaseApprover} {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

type CollaborationStageRolesParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	Roles                  CollaborationStageRoleSet
}

// CollaborationStageCandidateRef is the exact version a request acts on.
type CollaborationStageCandidateRef struct {
	CandidateID   pgtype.UUID
	Version       int64
	ContentDigest string
}

func (r CollaborationStageCandidateRef) valid() bool {
	return r.CandidateID.Valid && r.CandidateID.Bytes != [16]byte{} && r.Version > 0 && stageDigest(r.ContentDigest)
}

func (r CollaborationStageCandidateRef) matches(c db.CollaborationStageCandidate) bool {
	return r.CandidateID == c.CandidateID && r.Version == c.Version && r.ContentDigest == c.ContentDigest
}

// CollaborationStageProposalSource is what the human states about the origin.
// The server verifies it and records its own run/agent/comment facts.
type CollaborationStageProposalSource struct {
	Kind            string
	CommentID       pgtype.UUID
	CommentRevision int64
	RunID           pgtype.UUID
}

func (s CollaborationStageProposalSource) valid() bool {
	switch s.Kind {
	case "manual":
		return !s.CommentID.Valid && s.CommentRevision == 0 && !s.RunID.Valid
	case "comment":
		return s.CommentID.Valid && s.CommentRevision > 0 && !s.RunID.Valid
	case "run":
		return s.RunID.Valid && !s.CommentID.Valid && s.CommentRevision == 0
	default:
		return false
	}
}

type CollaborationStageProposalParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	// ProposalID and ExpectedCandidate are set only for a revision.
	ProposalID        pgtype.UUID
	ExpectedCandidate *CollaborationStageCandidateRef
	Content           collaboration.StageProposalContent
	Source            CollaborationStageProposalSource
}

type CollaborationStageDecisionParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	ProposalID             pgtype.UUID
	Candidate              CollaborationStageCandidateRef
	Decision               string
	Reason                 string
}

// CollaborationStageCodeRef names the pull request head a confirmation carries
// forward. It is a claim until the server finds that pull request linked to the
// child with exactly this head.
type CollaborationStageCodeRef struct {
	Provider      string
	PullRequestID pgtype.UUID
	HeadSHA       string
}

// stageHeadSHA accepts a full lowercase SHA-1 or SHA-256 commit id only.
func stageHeadSHA(value string) bool {
	return (len(value) == 40 || len(value) == 64) && stageHex(value)
}

func (r *CollaborationStageCodeRef) valid() bool {
	return r == nil || ((r.Provider == "github" || r.Provider == "vcs") && r.PullRequestID.Valid && r.PullRequestID.Bytes != [16]byte{} && stageHeadSHA(r.HeadSHA))
}

type stagePullRequestKey struct {
	issue    pgtype.UUID
	provider string
	id       pgtype.UUID
}

// stageLinkedPullRequests reads the server-observed heads of the pull requests
// really linked to these issues. Reference-only links are not code versions.
func stageLinkedPullRequests(ctx context.Context, q *db.Queries, workspace pgtype.UUID, issues []pgtype.UUID) (map[stagePullRequestKey]string, []db.ListCollaborationStageChildPullRequestsRow, error) {
	heads := map[stagePullRequestKey]string{}
	if len(issues) == 0 {
		return heads, nil, nil
	}
	rows, err := q.ListCollaborationStageChildPullRequests(ctx, db.ListCollaborationStageChildPullRequestsParams{IssueIds: issues, WorkspaceID: workspace})
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		heads[stagePullRequestKey{row.IssueID, row.Provider, row.PullRequestID}] = row.HeadSha
	}
	return heads, rows, nil
}

type CollaborationStageScopeChild struct {
	IssueID       pgtype.UUID
	ContentDigest string
	// Code is nil for an explicit "no code version is carried forward".
	Code *CollaborationStageCodeRef
}

type CollaborationStageScopeParams struct {
	CollaborationWorkflowRequest
	ExpectedPolicyRevision int64
	ExpectedRoleRevision   int64
	ExpectedScopeRevision  int64
	Children               []CollaborationStageScopeChild
	Reason                 string
}

func stageInvalid() (CollaborationWorkflowReceipt, error) {
	return CollaborationWorkflowReceipt{}, collaborationError(http.StatusBadRequest, "invalid_stage_request")
}

// stageHuman rejects machine, agent-actor and restricted credentials before any
// request validation, so a malformed machine request learns nothing but 403.
func stageHuman(ctx context.Context) error {
	if principal, ok := collaboration.PrincipalFromContext(ctx); !ok || !principal.Human() {
		return collaborationError(http.StatusForbidden, "forbidden")
	}
	return nil
}

func stageReason(reason string) bool {
	return utf8.ValidString(reason) && !strings.ContainsRune(reason, 0) && utf8.RuneCountInString(reason) <= collaboration.StageReasonMaxRunes
}

// ConfigureStageRoles records one immutable role configuration. Only a current
// source owner/admin who participates in the task may configure; every named
// account must be a current, eligible participant so the workflow's user locks
// already cover it. An occupational title grants nothing.
func (s *CollaborationService) ConfigureStageRoles(ctx context.Context, p CollaborationStageRolesParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	rules := p.Roles.rules()
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 0 || collaboration.ValidateStageRoles(rules) == "invalid_stage_request" {
		return stageInvalid()
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindConfigureRoles, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if !t.sourceMemberLocked || (t.sourceMember.Role != "owner" && t.sourceMember.Role != "admin") {
			return t.result("stage_configuration_denied", http.StatusForbidden), nil
		}
		if state.task.RoleRevision != p.ExpectedRoleRevision {
			return t.result("stage_roles_changed", http.StatusConflict), nil
		}
		if code := collaboration.ValidateStageRoles(rules); code != "" {
			return t.result(code, http.StatusConflict), nil
		}
		participants, err := t.q.ListCollaborationStageParticipants(ctx, t.access.Issue.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		active := make(map[pgtype.UUID]bool, len(participants))
		for _, participant := range participants {
			active[participant.UserID] = true
		}
		for _, holder := range p.Roles.holders() {
			eligible := active[holder]
			if eligible {
				if eligible, err = t.eligibleRecipient(ctx, holder); err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
			}
			if !eligible {
				return t.result("stage_role_target_unavailable", http.StatusConflict), nil
			}
		}
		if role := state.role; role != nil && role.BusinessOwnerUserID == p.Roles.BusinessOwner && role.ImplementerUserID == p.Roles.Implementer &&
			role.ProductApproverUserID == p.Roles.ProductApprover && role.CodeReviewerUserID == p.Roles.CodeReviewer &&
			role.QaApproverUserID == p.Roles.QAApprover && role.ReleaseApproverUserID == p.Roles.ReleaseApprover {
			t.stageConfirmation = &CollaborationStageConfirmation{Action: "configure_roles", IssueID: util.UUIDToString(t.access.Issue.ID), RoleRevision: stageVersion(role.Revision)}
			return t.result("stage_roles_unchanged", http.StatusOK), nil
		}
		if err = t.ensureStageTask(ctx, state); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		revision := state.task.RoleRevision + 1
		if _, err = t.q.CreateCollaborationStageRole(ctx, db.CreateCollaborationStageRoleParams{IssueID: t.access.Issue.ID, WorkspaceID: t.access.Issue.WorkspaceID,
			Revision: revision, BusinessOwnerUserID: p.Roles.BusinessOwner, ImplementerUserID: p.Roles.Implementer, ProductApproverUserID: p.Roles.ProductApprover,
			CodeReviewerUserID: p.Roles.CodeReviewer, QaApproverUserID: p.Roles.QAApprover, ReleaseApproverUserID: p.Roles.ReleaseApprover,
			PolicyRevision: state.policy.Revision, LifecycleEpoch: t.access.LifecycleEpoch, ConfiguredBy: t.actor, OperationID: t.operation.ID}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if _, err = t.q.SetCollaborationStageRoleRevision(ctx, db.SetCollaborationStageRoleRevisionParams{IssueID: t.access.Issue.ID, RoleRevision: revision}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = &CollaborationStageConfirmation{Action: "configure_roles", IssueID: util.UUIDToString(t.access.Issue.ID), RoleRevision: stageVersion(revision)}
		return t.finishWith(ctx, "stage_roles_configured", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{
			"role_revision": stageVersion(revision), "policy_revision": stageVersion(state.policy.Revision)}})
	})
}

func stageRefusal(refusal *CollaborationWorkflowReceipt) CollaborationWorkflowReceipt {
	if refusal == nil {
		return CollaborationWorkflowReceipt{}
	}
	return *refusal
}

// stageSource verifies the stated origin against this issue and returns the
// server's own facts. A client can never name an agent author.
func (t *collaborationWorkflowTx) stageSource(ctx context.Context, source CollaborationStageProposalSource) ([]byte, bool, error) {
	facts := map[string]any{"kind": source.Kind}
	switch source.Kind {
	case "comment":
		comment, err := t.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{CommentID: source.CommentID, IssueID: t.access.Issue.ID, WorkspaceID: t.access.Issue.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (comment.DeletedAt.Valid || comment.Revision != source.CommentRevision)) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		digest := sha256.Sum256([]byte(comment.Content))
		facts["comment_id"], facts["comment_revision"] = util.UUIDToString(comment.ID), stageVersion(comment.Revision)
		facts["author_type"], facts["author_id"], facts["digest"] = comment.AuthorType, util.UUIDToString(comment.AuthorID), hex.EncodeToString(digest[:])
	case "run":
		run, err := t.q.GetCollaborationStageRunSource(ctx, db.GetCollaborationStageRunSourceParams{RunID: source.RunID, IssueID: t.access.Issue.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		facts["run_id"], facts["agent_id"] = util.UUIDToString(run.ID), util.UUIDToString(run.AgentID)
	}
	encoded, err := json.Marshal(facts)
	return encoded, err == nil, err
}

func (t *collaborationWorkflowTx) createStageCandidate(ctx context.Context, state *collaborationStageState, proposal pgtype.UUID, parent *db.CollaborationStageCandidate,
	content collaboration.StageProposalContent, source []byte) (db.CollaborationStageCandidate, error) {
	canonical, digest, err := collaboration.StageProposalDigest(content)
	if err != nil {
		return db.CollaborationStageCandidate{}, err
	}
	version, parentID := int64(1), pgtype.UUID{}
	if parent != nil {
		version, parentID = parent.Version+1, parent.CandidateID
	}
	return t.q.CreateCollaborationStageCandidate(ctx, db.CreateCollaborationStageCandidateParams{CandidateID: dbid.NewV7(), IssueID: t.access.Issue.ID,
		WorkspaceID: t.access.Issue.WorkspaceID, ProposalID: proposal, Version: version, ParentCandidateID: parentID, Content: canonical, ContentDigest: digest,
		Source: source, LifecycleEpoch: t.access.LifecycleEpoch, SubmittedBy: t.actor, OperationID: t.operation.ID})
}

func stageCandidateConfirmation(action string, c db.CollaborationStageCandidate) *CollaborationStageConfirmation {
	out := &CollaborationStageConfirmation{Action: action, IssueID: util.UUIDToString(c.IssueID), ProposalID: util.UUIDToString(c.ProposalID),
		CandidateID: util.UUIDToString(c.CandidateID), Version: stageVersion(c.Version), ContentDigest: c.ContentDigest}
	if c.ParentCandidateID.Valid {
		out.ParentCandidateID = util.UUIDToString(c.ParentCandidateID)
	}
	return out
}

func stageCandidateFacts(c db.CollaborationStageCandidate) map[string]any {
	return map[string]any{"stage": map[string]any{"proposal_id": util.UUIDToString(c.ProposalID), "candidate_id": util.UUIDToString(c.CandidateID),
		"version": stageVersion(c.Version), "content_digest": c.ContentDigest, "parent_candidate_id": stageText(c.ParentCandidateID)}}
}

// CreateStageProposal files version 1 of a new proposal. Any current human
// participant may file one from readable agent output or by hand; it creates no
// issue and makes nothing executable.
func (s *CollaborationService) CreateStageProposal(ctx context.Context, p CollaborationStageProposalParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	content, err := collaboration.NormalizeStageProposal(p.Content)
	if err != nil || p.ExpectedPolicyRevision < 1 || !p.Source.valid() || p.ProposalID.Valid || p.ExpectedCandidate != nil {
		return stageInvalid()
	}
	p.Content = content
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindCreateProposal, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		source, ok, err := t.stageSource(ctx, p.Source)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !ok {
			return t.result("stage_source_unavailable", http.StatusConflict), nil
		}
		candidate, err := t.createStageCandidate(ctx, state, dbid.NewV7(), nil, content, source)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = stageCandidateConfirmation("create_proposal", candidate)
		return t.finishWith(ctx, "stage_proposal_created", pgtype.UUID{}, nil, stageCandidateFacts(candidate))
	})
}

// stageProposalHead resolves the newest version and its terminal decision.
func (t *collaborationWorkflowTx) stageProposalHead(ctx context.Context, proposal pgtype.UUID) (db.CollaborationStageCandidate, *db.CollaborationStageDecision, bool, error) {
	head, err := t.q.GetCollaborationStageProposalHead(ctx, db.GetCollaborationStageProposalHeadParams{IssueID: t.access.Issue.ID, ProposalID: proposal})
	if errors.Is(err, pgx.ErrNoRows) {
		return head, nil, false, nil
	}
	if err != nil {
		return head, nil, false, err
	}
	decision, err := t.q.GetCollaborationStageCandidateDecision(ctx, db.GetCollaborationStageCandidateDecisionParams{CandidateID: head.CandidateID, IssueID: t.access.Issue.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return head, nil, true, nil
	}
	if err != nil {
		return head, nil, false, err
	}
	return head, &decision, true, nil
}

// ReviseStageProposal adds a new immutable version on top of the exact newest
// one. The earlier version stays readable. An accepted proposal is final.
func (s *CollaborationService) ReviseStageProposal(ctx context.Context, p CollaborationStageProposalParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	content, err := collaboration.NormalizeStageProposal(p.Content)
	if err != nil || p.ExpectedPolicyRevision < 1 || !p.Source.valid() || !p.ProposalID.Valid || p.ExpectedCandidate == nil || !p.ExpectedCandidate.valid() {
		return stageInvalid()
	}
	p.Content = content
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindReviseProposal, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		head, decision, found, err := t.stageProposalHead(ctx, p.ProposalID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !found {
			return t.result("stage_proposal_not_found", http.StatusNotFound), nil
		}
		if decision != nil && decision.Decision == "accepted" {
			return t.result("stage_proposal_already_materialized", http.StatusConflict), nil
		}
		if !p.ExpectedCandidate.matches(head) {
			return t.result("stage_candidate_changed", http.StatusConflict), nil
		}
		_, digest, err := collaboration.StageProposalDigest(content)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if digest == head.ContentDigest && head.LifecycleEpoch == t.access.LifecycleEpoch {
			return t.result("stage_proposal_unchanged", http.StatusConflict), nil
		}
		source, ok, err := t.stageSource(ctx, p.Source)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !ok {
			return t.result("stage_source_unavailable", http.StatusConflict), nil
		}
		candidate, err := t.createStageCandidate(ctx, state, p.ProposalID, &head, content, source)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = stageCandidateConfirmation("revise_proposal", candidate)
		return t.finishWith(ctx, "stage_proposal_revised", pgtype.UUID{}, nil, stageCandidateFacts(candidate))
	})
}

// stageMaterializationPermit lets exactly one accepted proposal create its
// children through IssueService.CreateInTx. It is unexported and bound to the
// root: no transport, header or request field can supply it.
type stageMaterializationPermit struct {
	root pgtype.UUID
}

// DecideStageProposal records the product approver's decision on the exact
// newest version. Acceptance materializes 1-20 ordinary children, the decision
// and the unique proposal mapping in this one transaction; nothing is enqueued.
// Any child refusal rolls every creation effect back.
func (s *CollaborationService) DecideStageProposal(ctx context.Context, p CollaborationStageDecisionParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 0 || !p.ProposalID.Valid || !p.Candidate.valid() || !stageReason(p.Reason) ||
		(p.Decision != "accept" && p.Decision != "reject") || (p.Decision == "reject" && strings.TrimSpace(p.Reason) == "") {
		return stageInvalid()
	}
	// Entitlements are resolved before any lock; CreateInTx rechecks them.
	var policy IssueCountPolicy
	if p.Decision == "accept" && s.Issues != nil {
		if hint, err := s.Queries.GetCollaborationStageRoot(ctx, p.IssueID); err == nil {
			policy = ResolveIssueCountPolicy(ctx, s.Issues.Entitlements, hint.WorkspaceID)
		}
	}
	receipt, err := s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindDecideProposal, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal := t.stageProductApproverRefusal(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		head, previous, found, err := t.stageProposalHead(ctx, p.ProposalID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		switch {
		case !found:
			return t.result("stage_proposal_not_found", http.StatusNotFound), nil
		case previous != nil && previous.Decision == "accepted":
			return t.result("stage_proposal_already_materialized", http.StatusConflict), nil
		case !p.Candidate.matches(head):
			return t.result("stage_candidate_changed", http.StatusConflict), nil
		case previous != nil:
			return t.result("stage_candidate_not_pending", http.StatusConflict), nil
		}
		if p.Decision == "accept" && len(collaboration.StageMissing(state.facts)) != 0 {
			return t.result("stage_needs_confirmation", http.StatusConflict), nil
		}
		if head.LifecycleEpoch != t.access.LifecycleEpoch {
			return t.result("stage_candidate_stale", http.StatusConflict), nil
		}
		decisionID, payload := dbid.NewV7(), map[string]any{"candidate_submitted_by": util.UUIDToString(head.SubmittedBy)}
		if head.ParentCandidateID.Valid {
			payload["parent_candidate_id"] = util.UUIDToString(head.ParentCandidateID)
		}
		var created []db.Issue
		if p.Decision == "accept" {
			if !t.sourceMemberLocked {
				return t.result("stage_source_creation_denied", http.StatusForbidden), nil
			}
			var content collaboration.StageProposalContent
			if err = json.Unmarshal(head.Content, &content); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if content, err = collaboration.NormalizeStageProposal(content); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			created, refusal, err = t.materializeStageChildren(ctx, s.Issues, content, policy)
			if refusal != nil || err != nil {
				return stageRefusal(refusal), err
			}
			ids := make([]string, len(created))
			for i, issue := range created {
				ids[i] = util.UUIDToString(issue.ID)
				if _, err = t.q.CreateCollaborationStageChild(ctx, db.CreateCollaborationStageChildParams{ChildIssueID: issue.ID, RootIssueID: t.access.Issue.ID,
					WorkspaceID: t.access.Issue.WorkspaceID, ProposalID: head.ProposalID, CandidateID: head.CandidateID,
					Ordinal: pgtype.Int4{Int32: int32(i + 1), Valid: true}, DecisionID: decisionID, LifecycleEpoch: t.access.LifecycleEpoch}); err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
			}
			payload["child_issue_ids"] = ids
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		outcome := map[string]string{"accept": "accepted", "reject": "rejected"}[p.Decision]
		if _, err = t.q.CreateCollaborationStageDecision(ctx, db.CreateCollaborationStageDecisionParams{DecisionID: decisionID, IssueID: t.access.Issue.ID,
			WorkspaceID: t.access.Issue.WorkspaceID, Step: "proposal", Decision: outcome, ProposalID: head.ProposalID, CandidateID: head.CandidateID,
			CandidateVersion: pgtype.Int8{Int64: head.Version, Valid: true}, ContentDigest: pgtype.Text{String: head.ContentDigest, Valid: true},
			RoleRevision: state.task.RoleRevision, PolicyRevision: state.policy.Revision, LifecycleEpoch: t.access.LifecycleEpoch, ActorUserID: t.actor,
			ParticipantEpoch: t.participant.Epoch, AccountEpoch: t.account.Epoch, CredentialKind: stageCredentialKind(ctx),
			Reason: strings.TrimSpace(p.Reason), Payload: encoded, OperationID: t.operation.ID}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		confirmation := stageCandidateConfirmation("decide_proposal", head)
		confirmation.ParentCandidateID = ""
		confirmation.Decision, confirmation.DecisionID = outcome, util.UUIDToString(decisionID)
		confirmation.RoleRevision, confirmation.PolicyRevision = stageVersion(state.task.RoleRevision), stageVersion(state.policy.Revision)
		t.stageConfirmation = confirmation
		facts := stageCandidateFacts(head)
		facts["stage"].(map[string]any)["decision_id"] = confirmation.DecisionID
		receipt, err := t.finishWith(ctx, "stage_proposal_"+outcome, pgtype.UUID{}, nil, facts)
		if err == nil && len(created) > 0 {
			receipt.stageMaterialized = &stageMaterialization{issues: created}
		}
		return receipt, err
	})
	if err == nil && receipt.stageMaterialized != nil && s.Issues != nil {
		// List refresh only: no assignment, enqueue or execution event.
		for _, issue := range receipt.stageMaterialized.issues {
			s.Issues.publishStageMaterialized(ctx, issue)
		}
	}
	return receipt, err
}

// materializeStageChildren creates every child through the ordinary creation
// path inside a savepoint. A business refusal of any child discards all of them
// (rows, counter, position) and becomes one stored refusal.
func (t *collaborationWorkflowTx) materializeStageChildren(ctx context.Context, issues *IssueService, content collaboration.StageProposalContent, policy IssueCountPolicy) ([]db.Issue, *CollaborationWorkflowReceipt, error) {
	if issues == nil {
		return nil, nil, errors.New("stage materialization requires the issue service")
	}
	if _, err := t.tx.Exec(ctx, "SAVEPOINT stage_materialization"); err != nil {
		return nil, nil, err
	}
	root := t.access.Issue
	created := make([]db.Issue, 0, len(content.Children))
	for _, child := range content.Children {
		result, err := issues.CreateInTx(ctx, t.tx, IssueCreateParams{WorkspaceID: root.WorkspaceID, Title: child.Title,
			Description: pgtype.Text{String: child.Description, Valid: true}, Status: "todo", Priority: child.Priority,
			CreatorType: "member", CreatorID: t.actor, ParentIssueID: root.ID, ProjectID: root.ProjectID,
			stagePermit: &stageMaterializationPermit{root: root.ID}}, policy)
		if err != nil {
			var delivery DeliveryCompletionError
			if !errors.Is(err, ErrActiveDuplicate) && !errors.Is(err, ErrIssueStatusUnavailable) && !errors.As(err, &delivery) && collaborationCreationBusinessRejection(err) == "" {
				return nil, nil, err
			}
			if _, err = t.tx.Exec(ctx, "ROLLBACK TO SAVEPOINT stage_materialization"); err != nil {
				return nil, nil, err
			}
			if _, err = t.tx.Exec(ctx, "RELEASE SAVEPOINT stage_materialization"); err != nil {
				return nil, nil, err
			}
			receipt := t.result("stage_child_creation_refused", http.StatusConflict)
			return nil, &receipt, nil
		}
		created = append(created, result.Issue)
	}
	if _, err := t.tx.Exec(ctx, "RELEASE SAVEPOINT stage_materialization"); err != nil {
		return nil, nil, err
	}
	return created, nil, nil
}

// publishStageMaterialized refreshes ordinary lists after the commit. It is the
// ordinary issue:created notification, never an assignment or task event.
func (s *IssueService) publishStageMaterialized(ctx context.Context, issue db.Issue) {
	if s == nil || s.Bus == nil {
		return
	}
	prefix := ""
	if workspace, err := s.Queries.GetWorkspace(ctx, issue.WorkspaceID); err == nil {
		prefix = workspace.IssuePrefix
	}
	s.publishIssueCreated(issue, nil, nil, "member", util.UUIDToString(issue.CreatorID), IssueCreateOpts{
		BroadcastPayload: func(created db.Issue, _ []db.Attachment, _ []db.IssueLabel) map[string]any {
			return map[string]any{"issue": IssueToMap(created, prefix)}
		}})
}

// ConfirmStageScope is the product approver's explicit statement, after first
// enablement or a reopen, of which existing ordinary issues below the root carry
// into the current lifecycle and with which code version. A code version is
// accepted only as a pull request the server finds linked to that issue with
// exactly the stated head; "none" is explicit. It keeps each issue's identity
// and creates, assigns, enqueues or re-dispatches nothing.
func (s *CollaborationService) ConfirmStageScope(ctx context.Context, p CollaborationStageScopeParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 0 || p.ExpectedScopeRevision < 0 || !stageReason(p.Reason) || len(p.Children) > collaboration.StageScopeMaxChildren {
		return stageInvalid()
	}
	ids, expected := make([]pgtype.UUID, 0, len(p.Children)), make(map[pgtype.UUID]CollaborationStageScopeChild, len(p.Children))
	bindsCode := false
	for _, child := range p.Children {
		if _, duplicate := expected[child.IssueID]; duplicate || !child.IssueID.Valid || child.IssueID == p.IssueID || !stageDigest(child.ContentDigest) || !child.Code.valid() {
			return stageInvalid()
		}
		expected[child.IssueID] = child
		ids = append(ids, child.IssueID)
		bindsCode = bindsCode || child.Code != nil
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindConfirmScope, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal := t.stageProductApproverRefusal(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		if state.task.ScopeRevision != p.ExpectedScopeRevision {
			return t.result("stage_scope_changed", http.StatusConflict), nil
		}
		if len(p.Children) > 0 && !t.access.SourceMember {
			return t.result("stage_scope_child_unavailable", http.StatusConflict), nil
		}
		// A5 consumes current human authority now; unlike A7 it retains no
		// standing credential expiry after the scope confirmation commits.
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		root := t.access.Issue
		// Root before child, NOWAIT: a busy ordinary writer rolls this attempt back.
		rows, err := t.q.LockCollaborationStageScopeChildren(ctx, ids)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if len(rows) != len(ids) {
			return t.result("stage_scope_child_unavailable", http.StatusConflict), nil
		}
		if bindsCode {
			// Reuse the delivery observer's NOWAIT fence: connector writers do
			// not share issue locks, and association inserts need fencing too.
			// Hold the observed PR/head/link facts until confirmation commits.
			if _, err = t.tx.Exec(ctx, `LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request IN SHARE MODE NOWAIT`); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		heads, _, err := stageLinkedPullRequests(ctx, t.q, root.WorkspaceID, ids)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		for _, row := range rows {
			if row.WorkspaceID != root.WorkspaceID || row.Collaboration ||
				(row.AdmissionStatus != "not_required" && row.AdmissionStatus != "accepted") {
				return t.result("stage_scope_child_unavailable", http.StatusConflict), nil
			}
			// The issue must currently sit below this root, at any depth, with no
			// other collaboration issue in between.
			governing, unresolved, err := stageGoverningRoot(ctx, t.q, row.ParentIssueID)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if unresolved || governing.row.ID != root.ID {
				return t.result("stage_scope_child_unavailable", http.StatusConflict), nil
			}
			mapping, err := t.q.GetCollaborationStageChild(ctx, row.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return CollaborationWorkflowReceipt{}, err
			}
			if err == nil && mapping.RootIssueID != root.ID {
				return t.result("stage_scope_child_unavailable", http.StatusConflict), nil
			}
			if collaboration.StageChildContentDigest(row.Title, row.Description.String) != expected[row.ID].ContentDigest {
				return t.result("stage_scope_changed", http.StatusConflict), nil
			}
			if code := expected[row.ID].Code; code != nil {
				head, linked := heads[stagePullRequestKey{row.ID, code.Provider, code.PullRequestID}]
				if !linked {
					return t.result("stage_scope_code_unavailable", http.StatusConflict), nil
				}
				if head != code.HeadSHA {
					return t.result("stage_scope_changed", http.StatusConflict), nil
				}
			}
		}
		confirmed := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			entry := map[string]any{"issue_id": util.UUIDToString(row.ID), "content_digest": expected[row.ID].ContentDigest, "code": nil}
			if code := expected[row.ID].Code; code != nil {
				entry["code"] = map[string]any{"pull_request": map[string]string{"provider": code.Provider,
					"pull_request_id": util.UUIDToString(code.PullRequestID), "head_sha": code.HeadSHA}}
			}
			confirmed = append(confirmed, entry)
		}
		payload, err := json.Marshal(map[string]any{"children": confirmed})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = t.ensureStageTask(ctx, state); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		decisionID := dbid.NewV7()
		if _, err = t.q.CreateCollaborationStageDecision(ctx, db.CreateCollaborationStageDecisionParams{DecisionID: decisionID, IssueID: root.ID, WorkspaceID: root.WorkspaceID,
			Step: "scope", Decision: "confirmed", RoleRevision: state.task.RoleRevision, PolicyRevision: state.policy.Revision, LifecycleEpoch: t.access.LifecycleEpoch,
			ActorUserID: t.actor, ParticipantEpoch: t.participant.Epoch, AccountEpoch: t.account.Epoch, CredentialKind: stageCredentialKind(ctx),
			Reason: strings.TrimSpace(p.Reason), Payload: payload, OperationID: t.operation.ID}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		for _, row := range rows {
			binding := db.ConfirmCollaborationStageChildParams{ChildIssueID: row.ID, RootIssueID: root.ID,
				WorkspaceID: root.WorkspaceID, DecisionID: decisionID, LifecycleEpoch: t.access.LifecycleEpoch}
			if code := expected[row.ID].Code; code != nil {
				binding.CodeProvider, binding.CodePullRequestID = pgtype.Text{String: code.Provider, Valid: true}, code.PullRequestID
				binding.CodeHeadSha = pgtype.Text{String: code.HeadSHA, Valid: true}
			}
			if _, err = t.q.ConfirmCollaborationStageChild(ctx, binding); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		task, err := t.q.SetCollaborationStageScope(ctx, db.SetCollaborationStageScopeParams{IssueID: root.ID, DecisionID: decisionID, ExpectedScopeRevision: p.ExpectedScopeRevision})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = &CollaborationStageConfirmation{Action: "confirm_scope", IssueID: util.UUIDToString(root.ID), DecisionID: util.UUIDToString(decisionID),
			ScopeRevision: stageVersion(task.ScopeRevision), LifecycleEpoch: stageVersion(t.access.LifecycleEpoch)}
		return t.finishWith(ctx, "stage_scope_confirmed", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{
			"decision_id": util.UUIDToString(decisionID), "scope_revision": stageVersion(task.ScopeRevision), "lifecycle_epoch": stageVersion(t.access.LifecycleEpoch), "child_count": len(rows)}})
	})
}
