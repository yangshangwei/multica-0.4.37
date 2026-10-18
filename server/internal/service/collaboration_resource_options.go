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
	out := CollaborationResourceOptions{ProtocolVersion: 1, IssueID: issueID, ExecutionWorkspaceID: workspaceID, Items: make([]CollaborationResourceOption, 0)}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.Invite || !access.Capabilities.SubmitCommand || visible == nil || !workspaceID.Valid {
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
			if !agent.RuntimeID.Valid {
				continue
			}
			runtime, err := q.GetAgentRuntime(ctx, agent.RuntimeID)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if runtime.WorkspaceID != workspaceID || !runtime.OwnerID.Valid {
				continue
			}
			targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
			if err != nil {
				return err
			}
			if !visible(member, agent, runtime, targets) {
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
			_, reason, err := collaborationConfigurationSnapshot(ctx, q, kind, id, roster)
			if err != nil {
				return err
			}
			item := CollaborationResourceOption{Kind: kind, ResourceID: id, Name: name, Available: reason == "", Reason: reason, Resources: make([]CollaborationResourceView, 0, len(roster))}
			for _, resource := range roster {
				owner, err := util.ParseUUID(resource.OwnerUserID)
				if err != nil {
					return err
				}
				participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: owner})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				eligible, eligibilityErr := collaborationRecipientEligible(ctx, q, owner, workspaceID)
				if eligibilityErr != nil {
					return eligibilityErr
				}
				if err != nil || participant.RevokedAt.Valid {
					item.Available, item.Reason = false, "resource_owner_not_participant"
				} else if !eligible {
					item.Available, item.Reason = false, "resource_owner_unavailable"
				}
				view, err := collaborationResourceSummary(ctx, q, access, resource)
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
