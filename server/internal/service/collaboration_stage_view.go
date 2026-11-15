package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The stage projections are protected collaboration content: every read
// rechecks current issue authority inside one read-only snapshot.

type CollaborationStagePolicyView struct {
	Contract       string  `json:"contract"`
	ProjectID      *string `json:"project_id"`
	State          string  `json:"state"`
	PolicyRevision string  `json:"policy_revision"`
}

type CollaborationStageCurrent struct {
	Current       string `json:"current"`
	StageRevision string `json:"stage_revision"`
}

type CollaborationStageRolesView struct {
	RoleRevision    string                       `json:"role_revision"`
	ConfiguredBy    string                       `json:"configured_by"`
	ConfiguredAt    time.Time                    `json:"configured_at"`
	BusinessOwner   *CollaborationContentAccount `json:"business_owner"`
	Implementer     *CollaborationContentAccount `json:"implementer"`
	ProductApprover *CollaborationContentAccount `json:"product_approver"`
	CodeReviewer    *CollaborationContentAccount `json:"code_reviewer"`
	QAApprover      *CollaborationContentAccount `json:"qa_approver"`
	ReleaseApprover *CollaborationContentAccount `json:"release_approver"`
}

type CollaborationStageScopeView struct {
	ScopeRevision  string    `json:"scope_revision"`
	DecisionID     string    `json:"decision_id"`
	LifecycleEpoch string    `json:"lifecycle_epoch"`
	PolicyRevision string    `json:"policy_revision"`
	RoleRevision   string    `json:"role_revision"`
	ConfirmedBy    string    `json:"confirmed_by"`
	ConfirmedAt    time.Time `json:"confirmed_at"`
	ChildIssueIDs  []string  `json:"child_issue_ids"`
}

// CollaborationStagePullRequestView is a server-observed pull request head. It
// carries identifiers only: no title, URL, repository or author.
type CollaborationStagePullRequestView struct {
	Provider      string `json:"provider"`
	PullRequestID string `json:"pull_request_id"`
	HeadSHA       string `json:"head_sha"`
}

type CollaborationStageCodeView struct {
	PullRequest CollaborationStagePullRequestView `json:"pull_request"`
}

type CollaborationStageChildView struct {
	IssueID                string                              `json:"issue_id"`
	Identifier             string                              `json:"identifier"`
	Title                  string                              `json:"title"`
	CanOpen                bool                                `json:"can_open"`
	CreatorType            string                              `json:"creator_type"`
	Origin                 string                              `json:"origin"`
	Attached               bool                                `json:"attached"`
	Depth                  *int32                              `json:"depth"`
	ContentDigest          *string                             `json:"content_digest"`
	SemanticVersion        *string                             `json:"semantic_version"`
	ApprovedLifecycleEpoch *string                             `json:"approved_lifecycle_epoch"`
	ApprovalCurrent        bool                                `json:"approval_current"`
	ExecutionGated         bool                                `json:"execution_gated"`
	ProposalID             *string                             `json:"proposal_id"`
	Ordinal                *int32                              `json:"ordinal"`
	Code                   *CollaborationStageCodeView         `json:"code"`
	CodeState              string                              `json:"code_state"`
	PullRequests           []CollaborationStagePullRequestView `json:"pull_requests"`
}

type CollaborationStageChildrenView struct {
	Items     []CollaborationStageChildView `json:"items"`
	Truncated bool                          `json:"truncated"`
}

type CollaborationStageCapabilities struct {
	ConfigureRoles bool `json:"configure_roles"`
	CreateProposal bool `json:"create_proposal"`
	DecideProposal bool `json:"decide_proposal"`
	ConfirmScope   bool `json:"confirm_scope"`
}

type CollaborationStageView struct {
	ProtocolVersion       int                              `json:"protocol_version"`
	IssueID               string                           `json:"issue_id"`
	CollaborationRevision string                           `json:"collaboration_revision"`
	LifecycleEpoch        string                           `json:"lifecycle_epoch"`
	Policy                CollaborationStagePolicyView     `json:"policy"`
	Managed               bool                             `json:"managed"`
	NeedsConfirmation     bool                             `json:"needs_confirmation"`
	Missing               []string                         `json:"missing"`
	Stage                 *CollaborationStageCurrent       `json:"stage"`
	RoleRevision          string                           `json:"role_revision"`
	Roles                 *CollaborationStageRolesView     `json:"roles"`
	ScopeRevision         string                           `json:"scope_revision"`
	ScopeState            string                           `json:"scope_state"`
	Scope                 *CollaborationStageScopeView     `json:"scope"`
	Children              CollaborationStageChildrenView   `json:"children"`
	CodeCandidates        CollaborationStageCodeCandidates `json:"code_candidates"`
	Capabilities          CollaborationStageCapabilities   `json:"capabilities"`
}

type CollaborationStageProposalSourceView struct {
	Kind            string  `json:"kind"`
	CommentID       *string `json:"comment_id,omitempty"`
	CommentRevision *string `json:"comment_revision,omitempty"`
	RunID           *string `json:"run_id,omitempty"`
	AuthorType      *string `json:"author_type,omitempty"`
	AuthorID        *string `json:"author_id,omitempty"`
	AgentID         *string `json:"agent_id,omitempty"`
	Digest          *string `json:"digest,omitempty"`
}

type CollaborationStageDecisionView struct {
	DecisionID               string    `json:"decision_id"`
	Step                     string    `json:"step"`
	Decision                 string    `json:"decision"`
	CandidateID              string    `json:"candidate_id"`
	Version                  string    `json:"version"`
	ContentDigest            string    `json:"content_digest"`
	Role                     string    `json:"role"`
	RoleRevision             string    `json:"role_revision"`
	PolicyRevision           string    `json:"policy_revision"`
	LifecycleEpoch           string    `json:"lifecycle_epoch"`
	ActorUserID              string    `json:"actor_user_id"`
	Reason                   string    `json:"reason"`
	CreatedAt                time.Time `json:"created_at"`
	AcceptedWithModification bool      `json:"accepted_with_modification"`
}

type CollaborationStageCandidateSummary struct {
	CandidateID       string    `json:"candidate_id"`
	Version           string    `json:"version"`
	ContentDigest     string    `json:"content_digest"`
	ParentCandidateID *string   `json:"parent_candidate_id"`
	LifecycleEpoch    string    `json:"lifecycle_epoch"`
	Stale             bool      `json:"stale"`
	SubmittedBy       string    `json:"submitted_by"`
	CreatedAt         time.Time `json:"created_at"`
	Summary           string    `json:"summary"`
	ChildCount        int       `json:"child_count"`
}

type CollaborationStageProposalSummary struct {
	ProposalID   string                             `json:"proposal_id"`
	State        string                             `json:"state"`
	VersionCount string                             `json:"version_count"`
	CreatedAt    time.Time                          `json:"created_at"`
	Current      CollaborationStageCandidateSummary `json:"current"`
	Decision     *CollaborationStageDecisionView    `json:"decision"`
}

type CollaborationStageProposalPage struct {
	IssueID    string                              `json:"issue_id"`
	Items      []CollaborationStageProposalSummary `json:"items"`
	NextCursor *string                             `json:"next_cursor"`
}

type CollaborationStageCandidateView struct {
	CandidateID       string                               `json:"candidate_id"`
	Version           string                               `json:"version"`
	ContentDigest     string                               `json:"content_digest"`
	ParentCandidateID *string                              `json:"parent_candidate_id"`
	LifecycleEpoch    string                               `json:"lifecycle_epoch"`
	State             string                               `json:"state"`
	SubmittedBy       string                               `json:"submitted_by"`
	CreatedAt         time.Time                            `json:"created_at"`
	Content           collaboration.StageProposalContent   `json:"content"`
	Source            CollaborationStageProposalSourceView `json:"source"`
}

type CollaborationStageProposalChildView struct {
	Ordinal    int32  `json:"ordinal"`
	IssueID    string `json:"issue_id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	CanOpen    bool   `json:"can_open"`
	Attached   bool   `json:"attached"`
}

type CollaborationStageProposalView struct {
	IssueID           string                                `json:"issue_id"`
	ProposalID        string                                `json:"proposal_id"`
	State             string                                `json:"state"`
	Versions          []CollaborationStageCandidateView     `json:"versions"`
	VersionsTruncated bool                                  `json:"versions_truncated"`
	Decisions         []CollaborationStageDecisionView      `json:"decisions"`
	Children          []CollaborationStageProposalChildView `json:"children"`
}

type CollaborationStageProposalQuery struct {
	Limit  int
	Cursor string
}

const (
	stageViewChildLimit   = 200
	stageViewVersionLimit = 50
)

func stageReady(ctx context.Context, q *db.Queries) error {
	ready, err := q.CollaborationStageSchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready {
		return collaborationError(http.StatusServiceUnavailable, "stage_schema_unavailable")
	}
	return nil
}

func stageText(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	text := util.UUIDToString(value)
	return &text
}

// stageReadState loads the unlocked stage state for a read snapshot.
func stageReadState(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) (*collaborationStageState, CollaborationStagePolicyView, error) {
	issue := access.Issue
	state := &collaborationStageState{}
	policy := CollaborationStagePolicyView{Contract: collaboration.StageContract, ProjectID: stageText(issue.ProjectID), State: "none", PolicyRevision: "0"}
	if issue.ProjectID.Valid {
		row, found, err := currentStagePolicy(ctx, q, issue.WorkspaceID, issue.ProjectID)
		if err != nil {
			return nil, policy, err
		}
		if found {
			state.policy, policy.PolicyRevision, policy.State = row, stageVersion(row.Revision), "disabled"
			if row.Enabled && row.Contract == collaboration.StageContract {
				policy.State = "enabled"
			} else {
				state.policy.Enabled = false
			}
			if state.lastDisabled, err = q.GetProjectStagePolicyLastDisabled(ctx, db.GetProjectStagePolicyLastDisabledParams{WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID}); err != nil {
				return nil, policy, err
			}
		}
	}
	if err := loadStageDetails(ctx, q, issue, access.LifecycleEpoch, state, false); err != nil {
		return nil, policy, err
	}
	return state, policy, nil
}

// StageView projects the current stage facts of one collaboration root for a
// reader with current issue authority. Capabilities are display hints; every
// write rechecks them.
func (s *CollaborationService) StageView(ctx context.Context, issueID pgtype.UUID) (CollaborationStageView, error) {
	var out CollaborationStageView
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if err := stageReady(ctx, q); err != nil {
			return err
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		viewer, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		issue := access.Issue
		state, policy, err := stageReadState(ctx, q, access)
		if err != nil {
			return err
		}
		managed := state.policy.Enabled
		out = CollaborationStageView{ProtocolVersion: 1, IssueID: util.UUIDToString(issue.ID), CollaborationRevision: stageVersion(access.Revision),
			LifecycleEpoch: stageVersion(access.LifecycleEpoch), Policy: policy, Managed: managed, Missing: collaboration.StageMissing(state.facts),
			RoleRevision: stageVersion(state.task.RoleRevision), ScopeRevision: stageVersion(state.task.ScopeRevision),
			ScopeState: collaboration.StageScopeState(state.facts), Children: CollaborationStageChildrenView{Items: []CollaborationStageChildView{}}}
		out.NeedsConfirmation = len(out.Missing) != 0
		if out.CodeCandidates, err = stageDiscoverCodeCandidates(ctx, q, access, state); err != nil {
			return err
		}
		if managed {
			out.Stage = &CollaborationStageCurrent{Current: state.task.Stage, StageRevision: stageVersion(state.task.StageRevision)}
		}
		if role := state.role; role != nil {
			ids := []pgtype.UUID{role.BusinessOwnerUserID, role.ImplementerUserID, role.ProductApproverUserID, role.CodeReviewerUserID, role.QaApproverUserID, role.ReleaseApproverUserID}
			accounts, err := collaborationContentAccounts(ctx, q, issue.WorkspaceID, ids)
			if err != nil {
				return err
			}
			out.Roles = &CollaborationStageRolesView{RoleRevision: stageVersion(role.Revision), ConfiguredBy: util.UUIDToString(role.ConfiguredBy),
				ConfiguredAt: role.CreatedAt.Time.UTC(), BusinessOwner: accounts[ids[0]], Implementer: accounts[ids[1]], ProductApprover: accounts[ids[2]],
				CodeReviewer: accounts[ids[3]], QAApprover: accounts[ids[4]], ReleaseApprover: accounts[ids[5]]}
		}
		if scope := state.scope; scope != nil {
			var payload struct {
				Children []struct {
					IssueID string `json:"issue_id"`
				} `json:"children"`
			}
			if err := json.Unmarshal(scope.Payload, &payload); err != nil {
				return errors.New("invalid stage scope decision")
			}
			view := &CollaborationStageScopeView{ScopeRevision: stageVersion(state.task.ScopeRevision), DecisionID: util.UUIDToString(scope.DecisionID),
				LifecycleEpoch: stageVersion(scope.LifecycleEpoch), PolicyRevision: stageVersion(scope.PolicyRevision), RoleRevision: stageVersion(scope.RoleRevision),
				ConfirmedBy: util.UUIDToString(scope.ActorUserID), ConfirmedAt: scope.CreatedAt.Time.UTC(), ChildIssueIDs: []string{}}
			for _, child := range payload.Children {
				view.ChildIssueIDs = append(view.ChildIssueIDs, child.IssueID)
			}
			out.Scope = view
		}
		rows, err := q.ListCollaborationStageViewChildren(ctx, db.ListCollaborationStageViewChildrenParams{RootIssueID: issue.ID, WorkspaceID: issue.WorkspaceID, MaxDepth: stageAncestorDepth, RowLimit: stageViewChildLimit + 1})
		if err != nil {
			return err
		}
		if len(rows) > stageViewChildLimit {
			rows, out.Children.Truncated = rows[:stageViewChildLimit], true
		}
		prefix := ""
		if len(rows) > 0 {
			workspace, err := q.GetWorkspace(ctx, issue.WorkspaceID)
			if err != nil {
				return err
			}
			prefix = workspace.IssuePrefix
		}
		listed := make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			listed[i] = row.ID
		}
		heads, linked, err := stageLinkedPullRequests(ctx, q, issue.WorkspaceID, listed)
		if err != nil {
			return err
		}
		offered := make(map[pgtype.UUID][]CollaborationStagePullRequestView, len(rows))
		for _, pr := range linked {
			offered[pr.IssueID] = append(offered[pr.IssueID], CollaborationStagePullRequestView{Provider: pr.Provider, PullRequestID: util.UUIDToString(pr.PullRequestID), HeadSHA: pr.HeadSha})
		}
		for _, row := range rows {
			item := CollaborationStageChildView{IssueID: util.UUIDToString(row.ID), Identifier: IssueIdentifier(prefix, row.Number), Title: row.Title,
				CanOpen: access.SourceMember, CreatorType: row.CreatorType, Origin: "none", Attached: row.Depth > 0,
				ProposalID: stageText(row.ProposalID),
				CodeState:  "none", PullRequests: []CollaborationStagePullRequestView{}}
			if item.CanOpen {
				digest := collaboration.StageChildContentDigest(row.Title, row.Description.String)
				item.ContentDigest = &digest
			}
			if row.Depth > 0 {
				depth := row.Depth
				item.Depth = &depth
			}
			if prs := offered[row.ID]; prs != nil {
				item.PullRequests = prs
			}
			// The confirmed code version is evidence state only: a moved head or
			// a removed link never changes execution admission or the scope.
			if row.CodeProvider.Valid {
				item.Code = &CollaborationStageCodeView{PullRequest: CollaborationStagePullRequestView{Provider: row.CodeProvider.String,
					PullRequestID: util.UUIDToString(row.CodePullRequestID), HeadSHA: row.CodeHeadSha.String}}
				head, found := heads[stagePullRequestKey{row.ID, row.CodeProvider.String, row.CodePullRequestID}]
				switch {
				case !found:
					item.CodeState = "unavailable"
				case head != row.CodeHeadSha.String:
					item.CodeState = "changed"
				default:
					item.CodeState = "current"
				}
			}
			mapped := row.Origin.Valid
			if mapped {
				item.Origin = row.Origin.String
				semantic, approved := stageVersion(row.SemanticVersion.Int64), stageVersion(row.ApprovedLifecycleEpoch.Int64)
				item.SemanticVersion, item.ApprovedLifecycleEpoch = &semantic, &approved
				item.ApprovalCurrent = row.ApprovedLifecycleEpoch.Int64 == access.LifecycleEpoch
				if row.Ordinal.Valid {
					ordinal := row.Ordinal.Int32
					item.Ordinal = &ordinal
				}
			}
			item.ExecutionGated = collaboration.StageChildExecutionRefusal(collaboration.StageChildFacts{Managed: managed,
				MachineOrigin: item.Origin == "materialized" || row.CreatorType == "agent", Mapped: mapped,
				ApprovedLifecycle: row.ApprovedLifecycleEpoch.Int64, RootLifecycle: access.LifecycleEpoch}) != ""
			out.Children.Items = append(out.Children.Items, item)
		}
		if managed && access.Capabilities.ControlAI && !collaboration.TerminalCategory(access.StatusCategory) && access.QueueState != "closed" {
			approver := state.role != nil && state.role.ProductApproverUserID == viewer
			out.Capabilities = CollaborationStageCapabilities{CreateProposal: true, DecideProposal: approver, ConfirmScope: approver}
			if access.SourceMember {
				member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: viewer, WorkspaceID: issue.WorkspaceID})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				out.Capabilities.ConfigureRoles = err == nil && (member.Role == "owner" || member.Role == "admin")
			}
		}
		return nil
	})
	return out, err
}

func stageDecisionView(d db.CollaborationStageDecision, candidates map[pgtype.UUID]db.CollaborationStageCandidate) CollaborationStageDecisionView {
	view := CollaborationStageDecisionView{DecisionID: util.UUIDToString(d.DecisionID), Step: d.Step, Decision: d.Decision, CandidateID: util.UUIDToString(d.CandidateID),
		Version: stageVersion(d.CandidateVersion.Int64), ContentDigest: d.ContentDigest.String, Role: d.Role, RoleRevision: stageVersion(d.RoleRevision),
		PolicyRevision: stageVersion(d.PolicyRevision), LifecycleEpoch: stageVersion(d.LifecycleEpoch), ActorUserID: util.UUIDToString(d.ActorUserID),
		Reason: d.Reason, CreatedAt: d.CreatedAt.Time.UTC()}
	if candidate, ok := candidates[d.CandidateID]; ok {
		view.AcceptedWithModification = d.Decision == "accepted" && candidate.ParentCandidateID.Valid && candidate.SubmittedBy == d.ActorUserID
	}
	return view
}

func stageProposalState(decision *db.CollaborationStageDecision) string {
	if decision == nil {
		return "pending"
	}
	return decision.Decision
}

func stageCandidateDecision(ctx context.Context, q *db.Queries, c db.CollaborationStageCandidate) (*db.CollaborationStageDecision, error) {
	decision, err := q.GetCollaborationStageCandidateDecision(ctx, db.GetCollaborationStageCandidateDecisionParams{CandidateID: c.CandidateID, IssueID: c.IssueID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &decision, err
}

type stageProposalCursor struct {
	IssueID  string `json:"i"`
	ViewerID string `json:"v"`
	Before   string `json:"b"`
}

// ListStageProposals pages proposals newest first. The opaque cursor is bound to
// the issue and the viewer; every page rechecks current authority.
func (s *CollaborationService) ListStageProposals(ctx context.Context, issueID pgtype.UUID, query CollaborationStageProposalQuery) (CollaborationStageProposalPage, error) {
	out := CollaborationStageProposalPage{IssueID: util.UUIDToString(issueID), Items: []CollaborationStageProposalSummary{}}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 50 {
		return out, collaborationError(http.StatusBadRequest, "invalid_stage_request")
	}
	principal, _ := collaboration.PrincipalFromContext(ctx)
	var before pgtype.UUID
	if query.Cursor != "" {
		var cursor stageProposalCursor
		raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil || len(query.Cursor) > 512 || json.Unmarshal(raw, &cursor) != nil || cursor.IssueID != out.IssueID || cursor.ViewerID != principal.UserID {
			return out, collaborationError(http.StatusBadRequest, "invalid_stage_request")
		}
		if before, err = util.ParseUUID(cursor.Before); err != nil {
			return out, collaborationError(http.StatusBadRequest, "invalid_stage_request")
		}
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if err := stageReady(ctx, q); err != nil {
			return err
		}
		rows, err := q.ListCollaborationStageProposalHeads(ctx, db.ListCollaborationStageProposalHeadsParams{IssueID: issueID, BeforeProposalID: before, RowLimit: int32(query.Limit + 1)})
		if err != nil {
			return err
		}
		more := len(rows) > query.Limit
		if more {
			rows = rows[:query.Limit]
		}
		for _, row := range rows {
			head := row.CollaborationStageCandidate
			var content collaboration.StageProposalContent
			if err = json.Unmarshal(head.Content, &content); err != nil {
				return errors.New("invalid stage candidate content")
			}
			decision, err := stageCandidateDecision(ctx, q, head)
			if err != nil {
				return err
			}
			item := CollaborationStageProposalSummary{ProposalID: util.UUIDToString(head.ProposalID), State: stageProposalState(decision),
				VersionCount: stageVersion(head.Version), CreatedAt: row.ProposalCreatedAt.Time.UTC(),
				Current: CollaborationStageCandidateSummary{CandidateID: util.UUIDToString(head.CandidateID), Version: stageVersion(head.Version),
					ContentDigest: head.ContentDigest, ParentCandidateID: stageText(head.ParentCandidateID), LifecycleEpoch: stageVersion(head.LifecycleEpoch),
					Stale: head.LifecycleEpoch != access.LifecycleEpoch, SubmittedBy: util.UUIDToString(head.SubmittedBy), CreatedAt: head.CreatedAt.Time.UTC(),
					Summary: content.Summary, ChildCount: len(content.Children)}}
			if decision != nil {
				view := stageDecisionView(*decision, map[pgtype.UUID]db.CollaborationStageCandidate{head.CandidateID: head})
				item.Decision = &view
			}
			out.Items = append(out.Items, item)
		}
		if more {
			encoded, err := json.Marshal(stageProposalCursor{IssueID: out.IssueID, ViewerID: principal.UserID, Before: out.Items[len(out.Items)-1].ProposalID})
			if err != nil {
				return err
			}
			cursor := base64.RawURLEncoding.EncodeToString(encoded)
			out.NextCursor = &cursor
		}
		return nil
	})
	return out, err
}

func stageSourceView(raw []byte) (CollaborationStageProposalSourceView, error) {
	var view CollaborationStageProposalSourceView
	if err := json.Unmarshal(raw, &view); err != nil || (view.Kind != "manual" && view.Kind != "comment" && view.Kind != "run") {
		return view, errors.New("invalid stage candidate source")
	}
	return view, nil
}

// GetStageProposal returns one proposal's versions, decisions and, once
// accepted, its materialized children under current authority. This is the only
// place the child mapping is read; receipts never carry it.
func (s *CollaborationService) GetStageProposal(ctx context.Context, issueID, proposalID pgtype.UUID) (CollaborationStageProposalView, error) {
	out := CollaborationStageProposalView{IssueID: util.UUIDToString(issueID), ProposalID: util.UUIDToString(proposalID),
		Versions: []CollaborationStageCandidateView{}, Decisions: []CollaborationStageDecisionView{}, Children: []CollaborationStageProposalChildView{}}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if err := stageReady(ctx, q); err != nil {
			return err
		}
		versions, err := q.ListCollaborationStageProposalVersions(ctx, db.ListCollaborationStageProposalVersionsParams{IssueID: issueID, ProposalID: proposalID, Limit: stageViewVersionLimit + 1})
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			return collaborationError(http.StatusNotFound, "stage_proposal_not_found")
		}
		if len(versions) > stageViewVersionLimit {
			versions, out.VersionsTruncated = versions[:stageViewVersionLimit], true
		}
		decisions, err := q.ListCollaborationStageProposalDecisions(ctx, db.ListCollaborationStageProposalDecisionsParams{IssueID: issueID, ProposalID: proposalID})
		if err != nil {
			return err
		}
		decided := make(map[pgtype.UUID]string, len(decisions))
		for _, decision := range decisions {
			decided[decision.CandidateID] = decision.Decision
		}
		candidates := make(map[pgtype.UUID]db.CollaborationStageCandidate, len(versions))
		for i, version := range versions {
			candidates[version.CandidateID] = version
			var content collaboration.StageProposalContent
			if err = json.Unmarshal(version.Content, &content); err != nil {
				return errors.New("invalid stage candidate content")
			}
			source, err := stageSourceView(version.Source)
			if err != nil {
				return err
			}
			state := "superseded"
			if outcome, ok := decided[version.CandidateID]; ok {
				state = outcome
			} else if i == 0 {
				state = "pending"
			}
			if i == 0 {
				out.State = state
			}
			out.Versions = append(out.Versions, CollaborationStageCandidateView{CandidateID: util.UUIDToString(version.CandidateID), Version: stageVersion(version.Version),
				ContentDigest: version.ContentDigest, ParentCandidateID: stageText(version.ParentCandidateID), LifecycleEpoch: stageVersion(version.LifecycleEpoch),
				State: state, SubmittedBy: util.UUIDToString(version.SubmittedBy), CreatedAt: version.CreatedAt.Time.UTC(), Content: content, Source: source})
		}
		for _, decision := range decisions {
			out.Decisions = append(out.Decisions, stageDecisionView(decision, candidates))
		}
		children, err := q.ListCollaborationStageProposalChildren(ctx, db.ListCollaborationStageProposalChildrenParams{RootIssueID: issueID, ProposalID: proposalID})
		if err != nil {
			return err
		}
		prefix := ""
		if len(children) > 0 {
			workspace, err := q.GetWorkspace(ctx, access.Issue.WorkspaceID)
			if err != nil {
				return err
			}
			prefix = workspace.IssuePrefix
		}
		for _, child := range children {
			out.Children = append(out.Children, CollaborationStageProposalChildView{Ordinal: child.Ordinal.Int32, IssueID: util.UUIDToString(child.ChildIssueID),
				Identifier: IssueIdentifier(prefix, child.Number), Title: child.Title, CanOpen: access.SourceMember, Attached: child.ParentIssueID == issueID})
		}
		return nil
	})
	return out, err
}
