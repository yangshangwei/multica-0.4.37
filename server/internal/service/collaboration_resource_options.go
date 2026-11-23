package service

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationResourceOption struct {
	Kind       collaboration.TargetKind    `json:"kind"`
	ResourceID pgtype.UUID                 `json:"resource_id"`
	Name       string                      `json:"name"`
	Available  bool                        `json:"available"`
	Reason     string                      `json:"reason"`
	Resources  []CollaborationResourceView `json:"resources"`
}

type CollaborationResourceOptions struct {
	ProtocolVersion      int                           `json:"protocol_version"`
	IssueID              pgtype.UUID                   `json:"issue_id"`
	ExecutionWorkspaceID pgtype.UUID                   `json:"execution_workspace_id"`
	Items                []CollaborationResourceOption `json:"items"`
}

// The HTTP adapter supplies its established inventory visibility predicate.
// Scope, human authority, complete roster and owner eligibility remain here in
// one authorized snapshot. No client-supplied predicate or ambient workspace is used.
func (s *CollaborationService) ResourceOptions(ctx context.Context, issueID, workspaceID pgtype.UUID,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool) (CollaborationResourceOptions, error) {
	return s.resourceOptions(ctx, issueID, workspaceID, visible, false)
}

func (s *CollaborationService) ResourceOfferOptions(ctx context.Context, issueID, workspaceID pgtype.UUID,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool) (CollaborationResourceOptions, error) {
	return s.resourceOptions(ctx, issueID, workspaceID, visible, true)
}

func (s *CollaborationService) resourceOptions(ctx context.Context, issueID, workspaceID pgtype.UUID,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool, ownedOnly bool) (CollaborationResourceOptions, error) {
	out := CollaborationResourceOptions{ProtocolVersion: 1, IssueID: issueID, ExecutionWorkspaceID: workspaceID, Items: make([]CollaborationResourceOption, 0)}
	if err := validateCollaborationExecutionRead(ctx); err != nil {
		return out, err
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.ControlAI || !ownedOnly && !access.Capabilities.Invite || !access.Capabilities.SubmitCommand || visible == nil || !workspaceID.Valid {
			return collaborationError(http.StatusNotFound, "resource_options_unavailable")
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		actor, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return err
		}
		member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspaceID, UserID: actor})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "resource_workspace_unavailable")
		}
		if err != nil {
			return err
		}
		agents, err := q.ListAgents(ctx, workspaceID)
		if err != nil {
			return err
		}
		resources := make(map[pgtype.UUID]collaboration.TargetResource)
		names := make(map[pgtype.UUID]string)
		for _, agent := range agents {
			runtime, allowed, err := s.collaborationVisibleInventoryResource(ctx, q, member, agent, visible)
			if err != nil {
				return err
			}
			if !allowed {
				continue
			}
			resources[agent.ID] = collaboration.TargetResource{Role: "agent", AgentID: util.UUIDToString(agent.ID), RuntimeID: util.UUIDToString(runtime.ID),
				ExecutionWorkspaceID: util.UUIDToString(workspaceID), OwnerUserID: util.UUIDToString(runtime.OwnerID)}
			names[agent.ID] = agent.Name
		}
		appendOption := func(kind collaboration.TargetKind, id pgtype.UUID, name string, roster []collaboration.TargetResource) error {
			if !validCollaborationRoster(kind, id, roster) {
				return nil
			}
			if ownedOnly {
				owned := false
				for _, resource := range roster {
					owned = owned || resource.OwnerUserID == util.UUIDToString(actor)
				}
				if !owned {
					return nil
				}
			}
			_, reason, err := collaborationConfigurationSnapshot(ctx, q, kind, id, roster)
			if err != nil {
				return err
			}
			ownerReason, err := (&collaborationQueueTx{q: q, access: access}).validateResourceOwners(ctx, roster)
			if err != nil {
				return err
			}
			if ownerReason != "" {
				reason = ownerReason
			}
			if ownedOnly && reason == "" {
				_, reason, err = s.collaborationOfferAuthorities(ctx, q, issueID, roster)
				if err != nil {
					return err
				}
			}
			if reason == "" && CollaborationExecutionViewVersion(ctx) != 6 {
				host, err := collaborationRosterUsesHost(ctx, q, roster)
				if err != nil {
					return err
				}
				if host {
					reason = "collaboration_client_upgrade_required"
				}
			}
			item := CollaborationResourceOption{Kind: kind, ResourceID: id, Name: name, Available: reason == "", Reason: reason, Resources: make([]CollaborationResourceView, 0, len(roster))}
			for _, resource := range roster {
				view, err := s.collaborationResourceSummary(ctx, q, access, resource)
				if err != nil {
					return err
				}
				item.Resources = append(item.Resources, view)
			}
			out.Items = append(out.Items, item)
			return nil
		}
		for _, agent := range agents {
			if resource, ok := resources[agent.ID]; ok {
				if err := appendOption(collaboration.TargetAgent, agent.ID, names[agent.ID], []collaboration.TargetResource{resource}); err != nil {
					return err
				}
			}
		}
		squads, err := q.ListSquads(ctx, workspaceID)
		if err != nil {
			return err
		}
		for _, squad := range squads {
			leader, ok := resources[squad.LeaderID]
			if !ok {
				continue
			}
			leader.Role = "leader"
			roster := []collaboration.TargetResource{leader}
			members, err := q.ListSquadMembers(ctx, squad.ID)
			if err != nil {
				return err
			}
			complete := true
			seen := map[pgtype.UUID]bool{squad.LeaderID: true}
			for _, member := range members {
				if member.MemberType != "agent" || seen[member.MemberID] {
					continue
				}
				resource, ok := resources[member.MemberID]
				if !ok {
					complete = false
					break
				}
				resource.Role = "member"
				roster = append(roster, resource)
				seen[member.MemberID] = true
			}
			if complete {
				sort.Slice(roster, func(i, j int) bool { return roster[i].AgentID < roster[j].AgentID })
				if err := appendOption(collaboration.TargetSquad, squad.ID, squad.Name, roster); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return out, err
}

// Inventory options and direct offers use the same visibility predicate and
// transaction-local lookups. A caller cannot bypass its picker with a POST.
func (s *CollaborationService) collaborationVisibleInventoryResource(ctx context.Context, q *db.Queries, member db.Member, agent db.Agent,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool) (db.AgentRuntime, bool, error) {
	if visible == nil || !agent.RuntimeID.Valid || agent.WorkspaceID != member.WorkspaceID {
		return db.AgentRuntime{}, false, nil
	}
	runtime, err := s.collaborationRuntimeLookup(q).Get(ctx, agent.RuntimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AgentRuntime{}, false, nil
	}
	if err != nil {
		return db.AgentRuntime{}, false, err
	}
	if runtime.WorkspaceID != member.WorkspaceID || !runtime.OwnerID.Valid {
		return db.AgentRuntime{}, false, nil
	}
	targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return db.AgentRuntime{}, false, err
	}
	return runtime, visible(member, agent, runtime, targets), nil
}

func (s *CollaborationService) collaborationOfferRosterVisible(ctx context.Context, q *db.Queries, actor, workspace pgtype.UUID, roster []collaboration.TargetResource,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool) (string, error) {
	if visible == nil {
		return "resource_options_unavailable", nil
	}
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspace, UserID: actor})
	if errors.Is(err, pgx.ErrNoRows) {
		return "resource_workspace_unavailable", nil
	}
	if err != nil {
		return "", err
	}
	owned := false
	for _, resource := range roster {
		agentID, err := util.ParseUUID(resource.AgentID)
		if err != nil {
			return "", err
		}
		agent, err := q.GetAgent(ctx, agentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "resource_options_unavailable", nil
		}
		if err != nil {
			return "", err
		}
		runtime, allowed, err := s.collaborationVisibleInventoryResource(ctx, q, member, agent, visible)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "resource_options_unavailable", nil
		}
		owned = owned || runtime.OwnerID == actor
	}
	if !owned {
		return "resource_not_owned", nil
	}
	return "", nil
}
