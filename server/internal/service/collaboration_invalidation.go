package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// stopInvalidatedWork borrows the writer's source/channel/target/grant locks.
// Grants are the pre-revocation rows; epoch zero with a user means participant
// departure. Resource loss holds its targets and stops exact frozen dependencies;
// only loss of a consumed input author pauses a channel.
func (t *collaborationQueueTx) stopInvalidatedWork(ctx context.Context, user pgtype.UUID, epoch int64, grants []db.CollaborationGrant, reason string) (*CollaborationQueueReceipt, error) {
	held := make(map[pgtype.UUID]bool)
	invalidGrants := make(map[pgtype.UUID]bool)
	for _, grant := range grants {
		if epoch > 0 && grant.OwnerAuthEpoch >= epoch {
			continue
		}
		invalidGrants[grant.ID] = true
		if grant.RevokedAt.Valid {
			continue
		}
		target, ok := t.target(grant.TargetID)
		if !ok {
			return nil, invalidCollaborationLedger("grant target missing")
		}
		if !grant.ConfigVersion.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion ||
			!grant.TargetEpoch.Valid || grant.TargetEpoch.Int64 != target.TargetEpoch {
			continue
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
		if err != nil {
			return nil, err
		}
		for _, resource := range roster {
			if collaborationResourceMatchesGrant(resource, grant) {
				held[target.ID] = true
			}
		}
	}
	holdReason := collaboration.HoldGrantRequired
	if reason == "runtime_rebound" || reason == "installation_binding_changed" || reason == "target_configuration_changed" || reason == "configuration_changed" {
		holdReason = collaboration.HoldConfigInvalid
	}
	for _, target := range t.targets {
		if held[target.ID] {
			if _, err := t.setTargetHold(ctx, target, holdReason, false); err != nil {
				return nil, err
			}
		}
	}
	if _, err := t.q.LockCollaborationSessionCommands(ctx, t.session.ID); err != nil {
		return nil, err
	}
	if err := t.q.CloseEmptyCollaborationBatches(ctx, t.session.ID); err != nil {
		return nil, err
	}
	groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	selected := make(map[pgtype.UUID]string)
	detached := 0
	authorChannels := make(map[pgtype.UUID]bool)
	byID := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
		joined, err := t.q.ListCollaborationGroupCommands(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		dependencies, err := t.q.ListCollaborationGroupDependencies(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		for _, input := range joined {
			if input.State != "waiting" {
				continue
			}
			authorInvalid := user.Valid && input.AuthorUserID == user && (epoch == 0 || input.AuthorAuthEpoch < epoch)
			grantInvalid := false
			for _, dependency := range dependencies {
				if dependency.CommandID != input.ID || dependency.DetachedAt.Valid {
					continue
				}
				frozen, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
				if err != nil {
					return nil, err
				}
				for _, resource := range frozen {
					id, err := util.ParseUUID(resource.GrantID)
					if err != nil {
						return nil, invalidCollaborationLedger("invalid unread dependency grant")
					}
					grantInvalid = grantInvalid || invalidGrants[id]
				}
			}
			if authorInvalid || grantInvalid {
				state := "waiting"
				if authorInvalid {
					state = "cancelled"
				}
				if err = t.detachUnreadInput(ctx, group, input, state, reason); err != nil {
					return nil, err
				}
				detached++
			}
		}
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: group.CommandID})
		if err != nil {
			return nil, err
		}
		if command.TargetID != group.TargetID || command.BatchID != group.BatchID || command.ChannelID != group.ChannelID {
			return nil, invalidCollaborationLedger("invalidated group command mismatch")
		}
		author := user.Valid && command.AuthorUserID == user && (epoch == 0 || command.AuthorAuthEpoch < epoch)
		batchAuthor, err := collaborationBatchContainsAuthor(ctx, t.q, group.BatchID, user, epoch)
		if err != nil {
			return nil, err
		}
		if author || batchAuthor {
			selected[group.ID] = "cancelled"
			authorChannels[group.ChannelID] = true
		}
		dependencies, err = t.q.ListCollaborationGroupDependencies(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		for _, dependency := range dependencies {
			if dependency.DetachedAt.Valid {
				continue
			}
			grants, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
			if err != nil {
				return nil, err
			}
			for _, grant := range grants {
				id, err := util.ParseUUID(grant.GrantID)
				if err != nil {
					return nil, invalidCollaborationLedger("dependency grant identity malformed")
				}
				if invalidGrants[id] && selected[group.ID] == "" {
					selected[group.ID] = "stopped"
				}
			}
		}
		roster, err := decodeCollaborationRoster[collaboration.GroupResource](group.Roster)
		if err != nil {
			return nil, err
		}
		for _, resource := range roster {
			grantID, err := util.ParseUUID(resource.GrantID)
			if err != nil {
				return nil, invalidCollaborationLedger("group grant identity malformed")
			}
			if invalidGrants[grantID] && selected[group.ID] == "" {
				selected[group.ID] = "stopped"
			}
		}
	}
	runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: run.CommandID})
		if err != nil {
			return nil, err
		}
		author := user.Valid && command.AuthorUserID == user && (epoch == 0 || command.AuthorAuthEpoch < epoch)
		batchAuthor, err := collaborationBatchContainsAuthor(ctx, t.q, run.BatchID, user, epoch)
		if err != nil {
			return nil, err
		}
		author = author || batchAuthor
		if !author && !invalidGrants[run.GrantID] {
			continue
		}
		group, exists := byID[run.GroupID]
		if !exists || group.TargetID != run.TargetID || group.ChannelID != run.ChannelID || group.BatchID != run.BatchID {
			return nil, invalidCollaborationLedger("invalidated run group mismatch")
		}
		if author {
			selected[group.ID] = "cancelled"
			authorChannels[group.ChannelID] = true
		} else if selected[group.ID] == "" {
			selected[group.ID] = "stopped"
		}
	}
	if len(held) == 0 && len(selected) == 0 && detached == 0 {
		return nil, nil
	}
	for _, channel := range t.channels {
		if !authorChannels[channel.ID] || channel.QueueState == "paused" {
			continue
		}
		channel, err = t.q.PauseCollaborationChannel(ctx, db.PauseCollaborationChannelParams{ChannelID: channel.ID, Reason: reason})
		if err != nil {
			return nil, err
		}
		if err = t.mirrorChannel(ctx, channel); err != nil {
			return nil, err
		}
	}
	r := t.receipt("already_ended", http.StatusOK)
	for _, group := range groups {
		disposition := selected[group.ID]
		if disposition == "" {
			continue
		}
		target, exists := t.target(group.TargetID)
		if !exists {
			return nil, invalidCollaborationLedger("invalidated group target missing")
		}
		stopped, err := t.stopGroup(ctx, target, group, disposition, reason)
		if err != nil {
			return nil, err
		}
		channel, _ := t.channel(group.ChannelID)
		stopped.ChannelID, stopped.Code = channel.ID, "stop_requested"
		stopped.QueueState, stopped.ControlVersion, stopped.GroupGeneration = channel.QueueState, channel.ControlVersion, channel.GroupGeneration
		r.Code = "stop_requested"
		r.Channels = append(r.Channels, stopped.channelResult())
		for _, alias := range t.targets {
			if alias.ChannelID == channel.ID {
				result := stopped
				result.TargetID = alias.ID
				r.Targets = append(r.Targets, result)
			}
		}
	}
	for _, target := range t.targets {
		if held[target.ID] {
			r.TargetHolds = append(r.TargetHolds, collaborationTargetHoldResult(target, string(holdReason)))
		}
	}
	return &r, nil
}

// A started batch retains every human authorization, even though one immutable
// input anchors its root group. Losing any contributing author stops the batch.
func collaborationBatchContainsAuthor(ctx context.Context, q *db.Queries, batchID, user pgtype.UUID, epoch int64) (bool, error) {
	if !batchID.Valid || !user.Valid {
		return false, nil
	}
	inputs, err := q.ListCollaborationBatchInputs(ctx, batchID)
	if err != nil {
		return false, err
	}
	for _, input := range inputs {
		if input.State == "in_flight" && input.AuthorUserID == user && (epoch == 0 || input.AuthorAuthEpoch < epoch) {
			return true, nil
		}
	}
	return false, nil
}
