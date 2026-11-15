package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Workflow kinds of the fixed business-stage contract. Each is an ordinary
// collaboration workflow operation: same actor/epoch checks, same original-key
// receipt, same event and notification ledger.
const (
	stageKindConfigureRoles = "stage_configure_roles"
	stageKindCreateProposal = "stage_create_proposal"
	stageKindReviseProposal = "stage_revise_proposal"
	stageKindDecideProposal = "stage_decide_proposal"
	stageKindConfirmScope   = "stage_confirm_scope"
	stageKindCreateCode     = "stage_create_code_candidate"
	stageKindDecide         = "stage_decide"
	stageKindRevoke         = "stage_revoke_decision"
	stageKindAdvance        = "stage_advance"
)

func collaborationStageKind(kind string) bool {
	switch kind {
	case stageKindConfigureRoles, stageKindCreateProposal, stageKindReviseProposal, stageKindDecideProposal, stageKindConfirmScope,
		stageKindCreateCode, stageKindDecide, stageKindRevoke, stageKindAdvance:
		return true
	default:
		return false
	}
}

// Stage writes that consume source authority retain the member-row fence.
// A missing row alone does not refuse empty scope confirmations or returns.
func collaborationStageSourceMemberKind(kind string) bool {
	return kind == stageKindConfigureRoles || kind == stageKindDecideProposal || kind == stageKindConfirmScope || kind == stageKindRevoke || kind == stageKindCreateCode || kind == stageKindDecide || kind == stageKindAdvance
}

// CollaborationStageConfirmation is the closed, content-free confirmation of
// one stage action. Fields of other variants stay absent; it never carries
// prose, child lists, logs or credentials.
type CollaborationStageConfirmation struct {
	Action            string     `json:"action"`
	IssueID           string     `json:"issue_id,omitempty"`
	ProjectID         string     `json:"project_id,omitempty"`
	PolicyRevision    string     `json:"policy_revision,omitempty"`
	Enabled           *bool      `json:"enabled,omitempty"`
	RoleRevision      string     `json:"role_revision,omitempty"`
	ProposalID        string     `json:"proposal_id,omitempty"`
	CandidateID       string     `json:"candidate_id,omitempty"`
	Version           string     `json:"version,omitempty"`
	ContentDigest     string     `json:"content_digest,omitempty"`
	ParentCandidateID string     `json:"parent_candidate_id,omitempty"`
	Decision          string     `json:"decision,omitempty"`
	DecisionID        string     `json:"decision_id,omitempty"`
	ScopeRevision     string     `json:"scope_revision,omitempty"`
	LifecycleEpoch    string     `json:"lifecycle_epoch,omitempty"`
	Step              string     `json:"step,omitempty"`
	Stage             string     `json:"stage,omitempty"`
	StageRevision     string     `json:"stage_revision,omitempty"`
	RevocationID      string     `json:"revocation_id,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// The two nullable fields belong only to their new closed variants. Preserve
// every existing proposal receipt's omission rules when encoding them.
func (c CollaborationStageConfirmation) MarshalJSON() ([]byte, error) {
	type plain CollaborationStageConfirmation
	raw, err := json.Marshal(plain(c))
	if err != nil {
		return nil, err
	}
	if c.Action != "create_code_candidate" && c.Action != "decide_stage" {
		return raw, nil
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if c.Action == "create_code_candidate" && c.ParentCandidateID == "" {
		fields["parent_candidate_id"] = json.RawMessage(`null`)
	}
	if c.Action == "decide_stage" && c.ExpiresAt == nil {
		fields["expires_at"] = json.RawMessage(`null`)
	}
	return json.Marshal(fields)
}

func stageVersion(value int64) string { return strconv.FormatInt(value, 10) }

func stagePositiveVersion(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0 && strconv.FormatInt(parsed, 10) == value
}

func stageUUID(value string) bool {
	id, err := util.ParseUUID(value)
	return err == nil && id.Valid && id.Bytes != [16]byte{} && util.UUIDToString(id) == value
}

func stageHex(value string) bool {
	for _, c := range []byte(value) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return value != ""
}

func stageDigest(value string) bool {
	return len(value) == 64 && stageHex(value)
}

// valid checks the exact variant for the workflow kind that stored it. A stored
// receipt that fails this is unusable, never a partially trusted success.
func (c CollaborationStageConfirmation) valid(kind string) bool {
	blank := func(values ...string) bool {
		for _, value := range values {
			if value != "" {
				return false
			}
		}
		return true
	}
	if !stageUUID(c.IssueID) || c.ProjectID != "" || c.Enabled != nil {
		return false
	}
	candidate := stageUUID(c.ProposalID) && stageUUID(c.CandidateID) && stagePositiveVersion(c.Version) && stageDigest(c.ContentDigest)
	codeCandidate := stageUUID(c.CandidateID) && stagePositiveVersion(c.Version) && stageDigest(c.ContentDigest) && c.ProposalID == ""
	if kind != stageKindCreateCode && kind != stageKindDecide && kind != stageKindRevoke && kind != stageKindAdvance &&
		(!blank(c.Step, c.Stage, c.StageRevision, c.RevocationID) || c.ExpiresAt != nil) {
		return false
	}
	switch kind {
	case stageKindCreateCode:
		return c.Action == "create_code_candidate" && codeCandidate && ((c.Version == "1" && c.ParentCandidateID == "") || (c.Version != "1" && stageUUID(c.ParentCandidateID))) &&
			blank(c.PolicyRevision, c.RoleRevision, c.Decision, c.DecisionID, c.ScopeRevision, c.LifecycleEpoch, c.Step, c.Stage, c.StageRevision, c.RevocationID) && c.ExpiresAt == nil
	case stageKindDecide:
		return c.Action == "decide_stage" && codeCandidate && stageUUID(c.DecisionID) && stagePositiveVersion(c.RoleRevision) && stagePositiveVersion(c.PolicyRevision) &&
			(c.Step == "code_review" || c.Step == "qa" || c.Step == "release" || c.Step == "rollback") && ((c.Decision == "approved" && c.ExpiresAt != nil) || (c.Decision == "returned" && c.ExpiresAt == nil)) &&
			blank(c.ParentCandidateID, c.ScopeRevision, c.LifecycleEpoch, c.Stage, c.StageRevision, c.RevocationID)
	case stageKindRevoke:
		return c.Action == "revoke_decision" && stageUUID(c.DecisionID) && stageUUID(c.RevocationID) && blank(c.PolicyRevision, c.RoleRevision, c.ProposalID, c.CandidateID, c.Version, c.ContentDigest, c.ParentCandidateID, c.Decision, c.ScopeRevision, c.LifecycleEpoch, c.Step, c.Stage, c.StageRevision) && c.ExpiresAt == nil
	case stageKindAdvance:
		return c.Action == "advance_stage" && stagePositiveVersion(c.StageRevision) && (c.Stage == "development" || c.Stage == "code_review" || c.Stage == "qa" || c.Stage == "release" || c.Stage == "verification") &&
			blank(c.PolicyRevision, c.RoleRevision, c.ProposalID, c.CandidateID, c.Version, c.ContentDigest, c.ParentCandidateID, c.Decision, c.DecisionID, c.ScopeRevision, c.LifecycleEpoch, c.Step, c.RevocationID) && c.ExpiresAt == nil
	case stageKindConfigureRoles:
		return c.Action == "configure_roles" && stagePositiveVersion(c.RoleRevision) &&
			blank(c.PolicyRevision, c.ProposalID, c.CandidateID, c.Version, c.ContentDigest, c.ParentCandidateID, c.Decision, c.DecisionID, c.ScopeRevision, c.LifecycleEpoch)
	case stageKindCreateProposal:
		return c.Action == "create_proposal" && candidate && c.Version == "1" &&
			blank(c.PolicyRevision, c.RoleRevision, c.ParentCandidateID, c.Decision, c.DecisionID, c.ScopeRevision, c.LifecycleEpoch)
	case stageKindReviseProposal:
		return c.Action == "revise_proposal" && candidate && c.Version != "1" && stageUUID(c.ParentCandidateID) &&
			blank(c.PolicyRevision, c.RoleRevision, c.Decision, c.DecisionID, c.ScopeRevision, c.LifecycleEpoch)
	case stageKindDecideProposal:
		return c.Action == "decide_proposal" && candidate && (c.Decision == "accepted" || c.Decision == "rejected") && stageUUID(c.DecisionID) &&
			stagePositiveVersion(c.RoleRevision) && stagePositiveVersion(c.PolicyRevision) && blank(c.ParentCandidateID, c.ScopeRevision, c.LifecycleEpoch)
	case stageKindConfirmScope:
		return c.Action == "confirm_scope" && stageUUID(c.DecisionID) && stagePositiveVersion(c.ScopeRevision) && stagePositiveVersion(c.LifecycleEpoch) &&
			blank(c.PolicyRevision, c.RoleRevision, c.ProposalID, c.CandidateID, c.Version, c.ContentDigest, c.ParentCandidateID, c.Decision)
	default:
		return false
	}
}

// stageMaterialization is the transient post-commit effect of one acceptance.
type stageMaterialization struct {
	issues []db.Issue
}

// collaborationStageState is the stage truth for one managed root, read under
// the workflow's root locks plus a shared project lock. It is rebuilt for every
// request and never cached.
type collaborationStageState struct {
	policy       db.ProjectStagePolicy
	lastDisabled int64
	task         db.CollaborationStageTask
	taskExists   bool
	role         *db.CollaborationStageRole
	scope        *db.CollaborationStageDecision
	facts        collaboration.StageScopeFacts
}

// stageRoot holds the unlocked root facts used by ordinary child gates.
type stageRoot struct {
	row     db.GetCollaborationStageRootRow
	policy  db.ProjectStagePolicy
	managed bool
}

// loadStageRoot resolves whether an issue is a managed collaboration root. A
// missing collaboration row, project or enabled policy means "not managed";
// a query failure is returned so callers fail closed.
func loadStageRoot(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (stageRoot, error) {
	var out stageRoot
	if !issueID.Valid {
		return out, nil
	}
	row, err := q.GetCollaborationStageRoot(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.row = row
	if !row.ProjectID.Valid {
		return out, nil
	}
	policy, err := q.GetProjectStagePolicy(ctx, db.GetProjectStagePolicyParams{WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.policy, out.managed = policy, policy.Enabled && policy.Contract == collaboration.StageContract
	return out, nil
}

// stageTaskDefaults is the virtual head of a root that has no stage row yet.
// The row is created by the first successful stage write, never by a refusal.
func stageTaskDefaults(issue db.Issue) db.CollaborationStageTask {
	return db.CollaborationStageTask{IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, Stage: collaboration.StageDevelopment, StageRevision: 1}
}

func loadStageTask(ctx context.Context, q *db.Queries, issue db.Issue, lock bool) (db.CollaborationStageTask, bool, error) {
	var task db.CollaborationStageTask
	var err error
	if lock {
		task, err = q.LockCollaborationStageTask(ctx, issue.ID)
	} else {
		task, err = q.GetCollaborationStageTask(ctx, issue.ID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return stageTaskDefaults(issue), false, nil
	}
	return task, err == nil, err
}

// stageScopeSignerCurrent rechecks the confirming account against the current
// role holder and its participant/account generation. Removing and re-adding a
// participant, moving the role or revoking credentials never revives a
// confirmation.
func stageScopeSignerCurrent(ctx context.Context, q *db.Queries, issueID pgtype.UUID, role *db.CollaborationStageRole, scope *db.CollaborationStageDecision) (bool, error) {
	if role == nil || scope == nil || scope.ActorUserID != role.ProductApproverUserID || scope.RoleRevision != role.Revision {
		return false, nil
	}
	participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: scope.ActorUserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if participant.RevokedAt.Valid || participant.Epoch != scope.ParticipantEpoch {
		return false, nil
	}
	account, err := q.GetCollaborationAccountFence(ctx, scope.ActorUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return account.Epoch == scope.AccountEpoch, nil
}

// loadStageDetails fills role, scope and the derived confirmation facts for a
// policy that is already known.
func loadStageDetails(ctx context.Context, q *db.Queries, issue db.Issue, lifecycle int64, state *collaborationStageState, lock bool) error {
	var err error
	state.task, state.taskExists, err = loadStageTask(ctx, q, issue, lock)
	if err != nil {
		return err
	}
	if state.task.RoleRevision > 0 {
		role, err := q.GetCollaborationStageRole(ctx, db.GetCollaborationStageRoleParams{IssueID: issue.ID, Revision: state.task.RoleRevision})
		if err != nil {
			return err
		}
		state.role = &role
	}
	if state.task.ScopeDecisionID.Valid {
		scope, err := q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{DecisionID: state.task.ScopeDecisionID, IssueID: issue.ID})
		if err != nil {
			return err
		}
		state.scope = &scope
	}
	state.facts = collaboration.StageScopeFacts{Managed: state.policy.Enabled, RoleRevision: state.task.RoleRevision,
		RootLifecycle: lifecycle, PolicyLastDisabled: state.lastDisabled}
	if state.scope != nil {
		state.facts.ScopeDecision, state.facts.ScopeLifecycle, state.facts.ScopePolicyRevision = true, state.scope.LifecycleEpoch, state.scope.PolicyRevision
		if state.facts.SignerCurrent, err = stageScopeSignerCurrent(ctx, q, issue.ID, state.role, state.scope); err != nil {
			return err
		}
	}
	return nil
}

// loadStage reads the managed-root state inside a workflow mutation. The shared
// project lock keeps a concurrent policy revision out until this operation
// commits; NOWAIT contention rolls the whole attempt back for the owner's retry.
// A returned receipt is a stored refusal; nothing has been written.
func (t *collaborationWorkflowTx) loadStage(ctx context.Context, expectedPolicyRevision int64) (*collaborationStageState, *CollaborationWorkflowReceipt, error) {
	refuse := func(code string, status int) (*collaborationStageState, *CollaborationWorkflowReceipt, error) {
		receipt := t.result(code, status)
		return nil, &receipt, nil
	}
	issue := t.access.Issue
	if !issue.ProjectID.Valid {
		return refuse("stage_policy_disabled", http.StatusConflict)
	}
	if _, err := t.q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return refuse("stage_policy_disabled", http.StatusConflict)
		}
		return nil, nil, err
	}
	var ready bool
	if err := t.tx.QueryRow(ctx, collaborationStageReadinessSQL).Scan(&ready); err != nil {
		return nil, nil, err
	}
	if !ready {
		return nil, nil, collaborationError(http.StatusServiceUnavailable, "stage_schema_unavailable")
	}
	state := &collaborationStageState{}
	policy, err := t.q.GetProjectStagePolicy(ctx, db.GetProjectStagePolicyParams{WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!policy.Enabled || policy.Contract != collaboration.StageContract)) {
		return refuse("stage_policy_disabled", http.StatusConflict)
	}
	if err != nil {
		return nil, nil, err
	}
	if policy.Revision != expectedPolicyRevision {
		return refuse("stage_policy_changed", http.StatusConflict)
	}
	state.policy = policy
	if state.lastDisabled, err = t.q.GetProjectStagePolicyLastDisabled(ctx, db.GetProjectStagePolicyLastDisabledParams{WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID}); err != nil {
		return nil, nil, err
	}
	if err = loadStageDetails(ctx, t.q, issue, t.access.LifecycleEpoch, state, true); err != nil {
		return nil, nil, err
	}
	return state, nil, nil
}

// ensureStageTask creates the mutable head on the first successful write.
func (t *collaborationWorkflowTx) ensureStageTask(ctx context.Context, state *collaborationStageState) error {
	if state.taskExists {
		return nil
	}
	if err := t.q.EnsureCollaborationStageTask(ctx, db.EnsureCollaborationStageTaskParams{IssueID: t.access.Issue.ID, WorkspaceID: t.access.Issue.WorkspaceID}); err != nil {
		return err
	}
	task, err := t.q.LockCollaborationStageTask(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	state.task, state.taskExists = task, true
	return nil
}

// stageClosed mirrors the ordinary workflow rule: a closed lifecycle accepts no
// new stage fact. Reopening starts a new lifecycle that needs new confirmation.
func (t *collaborationWorkflowTx) stageClosed() bool {
	return collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" ||
		!collaboration.NonterminalCategory(t.access.StatusCategory)
}

// stageRoleRefusal checks the configuration observation and the exact holder of
// the required role. The actor's account, participation and epochs were already
// rechecked by the workflow owner under its user locks.
func (t *collaborationWorkflowTx) stageProductApproverRefusal(state *collaborationStageState, expectedRoleRevision int64) *CollaborationWorkflowReceipt {
	var receipt CollaborationWorkflowReceipt
	switch {
	case state.role == nil:
		receipt = t.result("stage_roles_unconfigured", http.StatusConflict)
	case state.task.RoleRevision != expectedRoleRevision:
		receipt = t.result("stage_roles_changed", http.StatusConflict)
	case state.role.ProductApproverUserID != t.actor:
		receipt = t.result("stage_role_required", http.StatusForbidden)
	default:
		return nil
	}
	return &receipt
}

func stageCredentialKind(ctx context.Context) string {
	principal, _ := collaboration.PrincipalFromContext(ctx)
	return string(principal.Kind)
}
