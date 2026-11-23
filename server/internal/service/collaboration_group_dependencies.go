package service

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Each input owns its dependencies. A later unread input must never replace the
// original batch's consent, or make removing one author revoke another's work.
type collaborationFrozenGrant struct {
	collaboration.GroupResource
	OwnerAuthEpoch        int64  `json:"owner_auth_epoch,string"`
	OwnerParticipantEpoch int64  `json:"owner_participant_epoch,string"`
	IdentityKind          string `json:"identity_kind"`
	InstallationID        string `json:"installation_id"`
	BindingID             string `json:"binding_id"`
	BindingEpoch          int64  `json:"binding_epoch,string"`
}

func freezeCollaborationGrant(grant db.CollaborationGrant) collaborationFrozenGrant {
	return collaborationFrozenGrant{
		GroupResource: collaboration.GroupResource{TargetResource: collaboration.TargetResource{
			Role: grant.Role.String, AgentID: util.UUIDToString(grant.AgentID), RuntimeID: util.UUIDToString(grant.RuntimeID),
			ExecutionWorkspaceID: util.UUIDToString(grant.ExecutionWorkspaceID), OwnerUserID: util.UUIDToString(grant.OwnerUserID)},
			GrantID: util.UUIDToString(grant.ID), GrantEpoch: grant.Epoch},
		OwnerAuthEpoch: grant.OwnerAuthEpoch, OwnerParticipantEpoch: grant.OwnerParticipantEpoch.Int64,
		IdentityKind: grant.IdentityKind, InstallationID: util.UUIDToString(grant.InstallationID),
		BindingID: util.UUIDToString(grant.BindingID), BindingEpoch: grant.BindingEpoch,
	}
}

func (t *collaborationQueueTx) frozenTargetGrants(ctx context.Context, target db.CollaborationTarget) ([]collaborationFrozenGrant, error) {
	if code, err := t.authorizeTargetInput(ctx, target); err != nil {
		return nil, err
	} else if code != "" {
		return nil, collaborationError(http.StatusConflict, code)
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		return nil, err
	}
	grants, err := t.q.ListCollaborationTargetGrants(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	frozen := make([]collaborationFrozenGrant, 0, len(roster))
	for _, resource := range roster {
		var match db.CollaborationGrant
		for _, grant := range grants {
			if grant.RevokedAt.Valid || grant.TargetID != target.ID || grant.TargetEpoch.Int64 != target.TargetEpoch ||
				grant.ConfigVersion.Int64 != target.ConfigVersion || grant.ConfigurationFingerprint != target.ConfigurationFingerprint ||
				!collaborationResourceMatchesGrant(resource, grant) {
				continue
			}
			if match.ID.Valid {
				return nil, invalidCollaborationLedger("ambiguous frozen grant")
			}
			match = grant
		}
		if !match.ID.Valid {
			return nil, collaborationError(http.StatusConflict, "grant_required")
		}
		frozen = append(frozen, freezeCollaborationGrant(match))
	}
	sort.Slice(frozen, func(i, j int) bool { return frozen[i].GrantID < frozen[j].GrantID })
	return frozen, nil
}

func (t *collaborationQueueTx) freezeGroupDependencies(ctx context.Context, groupID pgtype.UUID, inputs []db.CollaborationCommand) error {
	byTarget := make(map[pgtype.UUID][]byte)
	for _, input := range inputs {
		target, exists := t.target(input.TargetID)
		if !exists {
			return invalidCollaborationLedger("dependency target missing")
		}
		encoded, exists := byTarget[target.ID]
		if !exists {
			grants, err := t.frozenTargetGrants(ctx, target)
			if err != nil {
				return err
			}
			encoded, err = json.Marshal(grants)
			if err != nil {
				return err
			}
			byTarget[target.ID] = encoded
		}
		if err := t.q.CreateCollaborationGroupDependency(ctx, db.CreateCollaborationGroupDependencyParams{
			GroupID: groupID, CommandID: input.ID, TargetID: target.ID, TargetEpoch: target.TargetEpoch,
			ConfigVersion: target.ConfigVersion, ConfigurationFingerprint: target.ConfigurationFingerprint.String, Grants: encoded,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *collaborationRuntimeState) currentFrozenDependencies(ctx context.Context) error {
	dependencies, err := r.q.ListCollaborationGroupDependencies(ctx, r.group.ID)
	if err != nil {
		return err
	}
	byCommand := make(map[pgtype.UUID]db.CollaborationCommand, len(r.inputs))
	for _, input := range r.inputs {
		if groupInputState(input, r.group.ID) {
			byCommand[input.ID] = input
		}
	}
	for _, dependency := range dependencies {
		if dependency.DetachedAt.Valid {
			continue
		}
		input, exists := byCommand[dependency.CommandID]
		if !exists || input.TargetID != dependency.TargetID || input.GroupID != dependency.GroupID {
			return invalidCollaborationLedger("frozen dependency input mismatch")
		}
		delete(byCommand, input.ID)
		target, exists := r.targetByID(dependency.TargetID)
		if !exists || target.TargetEpoch != dependency.TargetEpoch || target.ConfigVersion != dependency.ConfigVersion ||
			target.ConfigurationFingerprint.String != dependency.ConfigurationFingerprint {
			return collaborationError(http.StatusConflict, "stale_authority")
		}
		frozen, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
		if err != nil {
			return err
		}
		current, err := r.frozenTargetGrants(ctx, target)
		if err != nil {
			return err
		}
		if len(frozen) != len(current) || len(frozen) == 0 {
			return collaborationError(http.StatusConflict, "stale_authority")
		}
		sort.Slice(frozen, func(i, j int) bool { return frozen[i].GrantID < frozen[j].GrantID })
		for i := range frozen {
			if frozen[i] != current[i] {
				return collaborationError(http.StatusConflict, "stale_authority")
			}
		}
	}
	if len(byCommand) != 0 {
		return invalidCollaborationLedger("frozen input dependencies missing")
	}
	return nil
}
