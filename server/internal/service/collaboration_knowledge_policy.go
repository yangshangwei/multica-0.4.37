package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const ProjectKnowledgeRuleVersion = "project-knowledge-v1"

type ProjectKnowledgePolicyView struct {
	KnowledgeProjection
	ProjectID       string  `json:"project_id"`
	WorkspaceID     string  `json:"workspace_id"`
	Revision        string  `json:"revision"`
	IsEnabled       bool    `json:"is_enabled"`
	MaxCandidates   int     `json:"max_candidates"`
	RuleVersion     string  `json:"rule_version"`
	Origin          string  `json:"origin"`
	ChangedByUserID *string `json:"changed_by_user_id"`
	UpdatedAt       *string `json:"updated_at"`
	CanManage       bool    `json:"can_manage"`
}

type ProjectKnowledgePolicyRequest struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision string `json:"expected_revision"`
	IsEnabled        bool   `json:"is_enabled"`
	MaxCandidates    int    `json:"max_candidates"`
}

type ProjectKnowledgePolicyReceipt struct {
	ProtocolVersion int    `json:"protocol_version"`
	RequestID       string `json:"request_id"`
	ProjectID       string `json:"project_id"`
	Code            string `json:"code"`
	Revision        string `json:"revision"`
}

func knowledgeRevision(value string) (int64, bool) {
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n >= 0 && strconv.FormatInt(n, 10) == value
}

func knowledgePolicyView(project, workspace pgtype.UUID, row db.ProjectKnowledgePolicy) ProjectKnowledgePolicyView {
	view := ProjectKnowledgePolicyView{ProjectID: knowledgeUUIDString(project), WorkspaceID: knowledgeUUIDString(workspace),
		Revision: "0", IsEnabled: true, MaxCandidates: 3, RuleVersion: ProjectKnowledgeRuleVersion, Origin: "system", CanManage: true}
	if row.ProjectID.Valid {
		view.Revision, view.IsEnabled, view.MaxCandidates = strconv.FormatInt(row.Revision, 10), row.IsEnabled, int(row.MaxCandidates)
		// Settings survive built-in rule upgrades; the executable rule version
		// comes from this server, not the historical override's last writer.
		view.Origin = "human"
		actor, updated := knowledgeUUIDString(row.ChangedByUserID), row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano)
		view.ChangedByUserID, view.UpdatedAt = &actor, &updated
	}
	return view
}

// Reading the effective policy grants no model, source or target authority.
func knowledgeEffectivePolicy(ctx context.Context, q *db.Queries, project, workspace pgtype.UUID) (ProjectKnowledgePolicyView, error) {
	row, err := q.GetProjectKnowledgePolicy(ctx, db.GetProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace})
	if errors.Is(err, pgx.ErrNoRows) {
		return knowledgePolicyView(project, workspace, db.ProjectKnowledgePolicy{}), nil
	}
	if err != nil {
		return ProjectKnowledgePolicyView{}, err
	}
	return knowledgePolicyView(project, workspace, row), nil
}

func knowledgeHumanActor(ctx context.Context, q *db.Queries) (pgtype.UUID, error) {
	p, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !p.Human() {
		return pgtype.UUID{}, knowledgeFailure(403, "knowledge_permission_denied")
	}
	actor, err := util.ParseUUID(p.UserID)
	if err != nil {
		return actor, knowledgeFailure(403, "knowledge_permission_denied")
	}
	users, err := q.LockKnowledgeUsers(ctx, []pgtype.UUID{actor})
	if err != nil {
		return actor, err
	}
	if len(users) != 1 {
		return actor, knowledgeFailure(403, "knowledge_permission_denied")
	}
	if err = authorizeWorkflowAccount(ctx, q, actor, users[0]); err != nil {
		return actor, err
	}
	_, err = knowledgePAT(ctx, q, actor, true)
	return actor, err
}

// The human policy writer is independent from the acceptance reader whose
// source authority the worker must check later. Defaults have no human owner.
func knowledgeLockProjectConfirmer(ctx context.Context, q *db.Queries, actor, project, workspace pgtype.UUID) error {
	if _, err := q.LockKnowledgeWorkspace(ctx, workspace); err != nil {
		return err
	}
	if err := q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: workspace, UserID: actor}); err != nil {
		return err
	}
	member, err := q.LockKnowledgeMember(ctx, db.LockKnowledgeMemberParams{WorkspaceID: workspace, UserID: actor})
	if errors.Is(err, pgx.ErrNoRows) {
		return knowledgeFailure(403, "knowledge_permission_denied")
	}
	if err != nil {
		return err
	}
	if _, err = q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: project, WorkspaceID: workspace}); err != nil {
		return err
	}
	if member.Role == "owner" || member.Role == "admin" {
		return knowledgeLockPolicyFence(ctx, q, project)
	}
	lead, err := q.LockKnowledgeProjectLead(ctx, db.LockKnowledgeProjectLeadParams{ID: project, WorkspaceID: workspace})
	if err != nil {
		return err
	}
	if lead.LeadType.String != "member" || lead.LeadID != actor {
		return knowledgeFailure(403, "knowledge_permission_denied")
	}
	return knowledgeLockPolicyFence(ctx, q, project)
}

func knowledgeLockPolicyFence(ctx context.Context, q *db.Queries, project pgtype.UUID) error {
	locked, err := q.TryKnowledgePolicyFence(ctx, knowledgeUUIDString(project))
	if err != nil {
		return err
	}
	if !locked {
		return &pgconn.PgError{Code: "55P03", Message: "knowledge policy is busy"}
	}
	return nil
}

func (s *CollaborationService) GetProjectKnowledgePolicy(ctx context.Context, project, workspace pgtype.UUID) (ProjectKnowledgePolicyView, error) {
	projection := knowledgeNewProjection(ctx)
	projection.ProtocolVersion = 2
	view, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (ProjectKnowledgePolicyView, error) {
		q := s.Queries.WithTx(tx)
		actor, err := knowledgeHumanActor(ctx, q)
		if err != nil {
			return ProjectKnowledgePolicyView{}, err
		}
		if err = knowledgeLockProjectConfirmer(ctx, q, actor, project, workspace); err != nil {
			return ProjectKnowledgePolicyView{}, err
		}
		view, err := knowledgeEffectivePolicy(ctx, q, project, workspace)
		if err != nil {
			return view, err
		}
		expiry, err := knowledgePAT(ctx, q, actor, false)
		if err != nil {
			return view, err
		}
		view.KnowledgeProjection = projection
		view.clampCredential(expiry)
		return view, knowledgeProjectionCurrent(view.KnowledgeProjection)
	})
	return view, knowledgePublicError(err)
}

func (s *CollaborationService) UpdateProjectKnowledgePolicy(ctx context.Context, project, workspace pgtype.UUID, p ProjectKnowledgePolicyRequest) (ProjectKnowledgePolicyReceipt, error) {
	revision, ok := knowledgeRevision(p.ExpectedRevision)
	request, requestErr := util.ParseUUID(p.RequestID)
	if !ok || requestErr != nil || knowledgeUUIDString(request) != p.RequestID || !project.Valid || !workspace.Valid || p.MaxCandidates < 1 || p.MaxCandidates > 3 {
		return ProjectKnowledgePolicyReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	raw, _ := json.Marshal(struct {
		Workspace string                        `json:"workspace_id"`
		Request   ProjectKnowledgePolicyRequest `json:"request"`
	}{knowledgeUUIDString(workspace), p})
	hash := protocol.CollaborationKnowledgeHash(string(raw))
	r, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (ProjectKnowledgePolicyReceipt, error) {
		q := s.Queries.WithTx(tx)
		actor, err := knowledgeHumanActor(ctx, q)
		if err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		previous, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "project", ScopeID: project, ActorUserID: actor, RequestID: p.RequestID})
		if err == nil {
			if previous.Kind != "knowledge_policy" || previous.PayloadHash != hash {
				return ProjectKnowledgePolicyReceipt{}, knowledgeFailure(409, "idempotency_conflict")
			}
			return decodeProjectKnowledgePolicyReceipt(previous, project, p.RequestID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		if err = knowledgeLockProjectConfirmer(ctx, q, actor, project, workspace); err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		current, err := knowledgeEffectivePolicy(ctx, q, project, workspace)
		if err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		op, err := q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "project", ScopeID: project, ActorUserID: actor,
			RequestID: p.RequestID, Kind: "knowledge_policy", PayloadHash: hash})
		if err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		receipt := ProjectKnowledgePolicyReceipt{ProtocolVersion: 2, RequestID: p.RequestID, ProjectID: knowledgeUUIDString(project), Code: "knowledge_policy_revision_conflict", Revision: current.Revision}
		if current.Revision == p.ExpectedRevision {
			var policy db.ProjectKnowledgePolicy
			if revision == 0 {
				policy, err = q.CreateProjectKnowledgePolicy(ctx, db.CreateProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace,
					IsEnabled: p.IsEnabled, MaxCandidates: int16(p.MaxCandidates), BuiltinRuleVersion: ProjectKnowledgeRuleVersion, ChangedByUserID: actor})
			} else {
				policy, err = q.UpdateProjectKnowledgePolicy(ctx, db.UpdateProjectKnowledgePolicyParams{ProjectID: project, WorkspaceID: workspace,
					ExpectedRevision: revision, IsEnabled: p.IsEnabled, MaxCandidates: int16(p.MaxCandidates), BuiltinRuleVersion: ProjectKnowledgeRuleVersion, ChangedByUserID: actor})
			}
			if err != nil {
				return ProjectKnowledgePolicyReceipt{}, err
			}
			receipt.Code, receipt.Revision = "knowledge_policy_updated", strconv.FormatInt(policy.Revision, 10)
		}
		encoded, _ := json.Marshal(receipt)
		_, err = q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: op.ID, ResultCode: receipt.Code, Result: encoded})
		return receipt, err
	})
	if err == nil && r.Code != "knowledge_policy_updated" {
		err = knowledgeFailure(409, r.Code)
	}
	return r, knowledgePublicError(err)
}

func decodeProjectKnowledgePolicyReceipt(op db.CollaborationOperation, project pgtype.UUID, request string) (ProjectKnowledgePolicyReceipt, error) {
	var r ProjectKnowledgePolicyReceipt
	if !op.CompletedAt.Valid || json.Unmarshal(op.Result, &r) != nil || r.ProtocolVersion != 2 || r.ProjectID != knowledgeUUIDString(project) || r.RequestID != request || r.Code != op.ResultCode.String {
		return r, knowledgeFailure(503, "knowledge_unavailable")
	}
	return r, nil
}

func (s *CollaborationService) GetProjectKnowledgePolicyOperation(ctx context.Context, project pgtype.UUID, request string) (ProjectKnowledgePolicyReceipt, error) {
	r, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (ProjectKnowledgePolicyReceipt, error) {
		q := s.Queries.WithTx(tx)
		actor, err := knowledgeHumanActor(ctx, q)
		if err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "project", ScopeID: project, ActorUserID: actor, RequestID: request})
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectKnowledgePolicyReceipt{}, knowledgeFailure(404, "operation_not_found")
		}
		if err != nil {
			return ProjectKnowledgePolicyReceipt{}, err
		}
		if op.Kind != "knowledge_policy" {
			return ProjectKnowledgePolicyReceipt{}, knowledgeFailure(404, "operation_not_found")
		}
		return decodeProjectKnowledgePolicyReceipt(op, project, request)
	})
	return r, knowledgePublicError(err)
}
