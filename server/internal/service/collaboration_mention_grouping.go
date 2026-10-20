package service

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Grouping describes eligible inputs in this snapshot. It is neither a staging
// batch identity nor an admission receipt; submission rechecks every decision.
type CollaborationCommentGrouping struct {
	Kind                string        `json:"kind"`
	TargetIDs           []pgtype.UUID `json:"target_ids"`
	HasExistingInputs   bool          `json:"has_existing_inputs"`
	FollowsPendingBatch bool          `json:"follows_pending_batch"`
}

type CollaborationCommentTargetPreview struct {
	CollaborationCommentTargetResult
	Grouping *CollaborationCommentGrouping `json:"grouping,omitempty"`
}

type collaborationPreviewBatch struct {
	grouping CollaborationCommentGrouping
	squad    pgtype.UUID
	count    int
	bytes    int
}

func (b *collaborationPreviewBatch) fits(target db.CollaborationTarget, size int) bool {
	nextBytes := size + 2
	if b.count > 0 {
		nextBytes = b.bytes + size + 1
	}
	return collaborationBatchTargetCompatible(b.squad, target) && b.count < maxCollaborationBatchInputs && nextBytes <= maxCollaborationBatchBytes
}

func (b *collaborationPreviewBatch) append(target db.CollaborationTarget, size int, existing bool) {
	if b.count == 0 {
		b.bytes = 2
	} else {
		b.bytes++
	}
	b.count++
	b.bytes += size
	if target.Kind == "squad" {
		b.squad = target.ID
	}
	if existing {
		b.grouping.HasExistingInputs = true
	} else {
		b.grouping.TargetIDs = append(b.grouping.TargetIDs, target.ID)
	}
}

type collaborationPreviewChannel struct {
	channel      db.CollaborationChannel
	pending      *collaborationPreviewBatch
	continuation *collaborationPreviewBatch
	groupTarget  pgtype.UUID
}

func (c *collaborationPreviewChannel) appendPending(target db.CollaborationTarget, size int, existing bool) *CollaborationCommentGrouping {
	if c.pending == nil || !c.pending.fits(target, size) {
		c.pending = &collaborationPreviewBatch{grouping: CollaborationCommentGrouping{Kind: "pending_batch", FollowsPendingBatch: c.pending != nil}}
	}
	c.pending.append(target, size, existing)
	return &c.pending.grouping
}

func (s *CollaborationService) previewCommentGrouping(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, actor pgtype.UUID,
	input CollaborationCommentInput, catalogue []CollaborationMentionTarget) ([]CollaborationCommentTargetPreview, error) {
	base := collaborationCommentTargetPreview(access, input, catalogue)
	result := make([]CollaborationCommentTargetPreview, len(base))
	needed := false
	for i, item := range base {
		result[i].CollaborationCommentTargetResult = item
		needed = needed || item.State == "waiting" && item.HoldReason == nil
	}
	if !needed {
		return result, nil
	}
	targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
	if err != nil {
		return nil, err
	}
	authorized := make(map[pgtype.UUID]bool, len(catalogue))
	for _, target := range catalogue {
		authorized[target.TargetID] = target.HoldReason == nil
	}
	byTarget := make(map[pgtype.UUID]db.CollaborationTarget, len(targets))
	for _, target := range targets {
		byTarget[target.ID] = target
	}
	channels, err := q.ListCollaborationChannels(ctx, access.SessionID)
	if err != nil {
		return nil, err
	}
	byRoot := make(map[pgtype.UUID]db.CollaborationChannel, len(channels))
	for _, channel := range channels {
		byRoot[channel.RootAgentID] = channel
	}
	gate, err := q.GetCollaborationGate(ctx)
	if err != nil {
		return nil, err
	}
	states := make(map[pgtype.UUID]*collaborationPreviewChannel)
	attachments := []byte("[]")
	if len(input.AttachmentIDs) > 0 {
		attachments, err = json.Marshal(input.AttachmentIDs)
		if err != nil {
			return nil, err
		}
	}
	for i := range result {
		item := &result[i]
		if item.State != "waiting" || item.HoldReason != nil {
			continue
		}
		target, exists := byTarget[item.TargetID]
		if !exists || !authorized[target.ID] {
			return nil, invalidCollaborationLedger("preview target missing")
		}
		root, err := collaborationRootAgent(target)
		if err != nil {
			return nil, err
		}
		state := states[root]
		if state == nil {
			state = &collaborationPreviewChannel{channel: byRoot[root]}
			states[root] = state
			if state.channel.ID.Valid {
				waiting, err := q.ListCollaborationChannelWaitingInputs(ctx, state.channel.ID)
				if err != nil {
					return nil, err
				}
				for _, earlier := range waiting {
					if earlier.GroupID.Valid {
						continue
					}
					other, exists := byTarget[earlier.TargetID]
					if !exists || earlier.SessionID != access.SessionID || other.ChannelID != state.channel.ID {
						return nil, invalidCollaborationLedger("preview waiting input scope mismatch")
					}
					if !authorized[other.ID] {
						continue
					}
					if code, err := collaborationCurrentBatchAuthors(ctx, q, access.Issue.ID, access.LifecycleEpoch, []db.CollaborationCommand{earlier}, "waiting"); err != nil {
						return nil, err
					} else if code != "" {
						continue
					}
					size, err := collaborationRunInputSize(earlier)
					if err != nil {
						return nil, err
					}
					state.appendPending(other, size, true)
				}
				if state.pending == nil && gate.Enabled && access.QueueState == "active" && state.channel.QueueState == "active" && state.channel.ActiveGroupID.Valid {
					state.continuation, state.groupTarget, err = s.previewSquadContinuation(ctx, q, access, state.channel, byTarget, authorized)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		if target.ChannelID.Valid && (target.ChannelID != state.channel.ID || target.QueueState != state.channel.QueueState || target.ActiveGroupID != state.channel.ActiveGroupID) {
			return nil, invalidCollaborationLedger("preview channel mismatch")
		}
		command := db.CollaborationCommand{ID: target.ID, AuthorUserID: actor, ParticipantEpoch: access.ParticipantEpoch.Int64,
			AuthorAuthEpoch: access.AccountEpoch.Int64, LifecycleEpoch: access.LifecycleEpoch,
			Content: util.SanitizeTextForPostgres(input.Content), Attachments: attachments, State: "waiting"}
		size, err := collaborationRunInputSize(command)
		if err != nil {
			return nil, err
		}
		if state.pending == nil && state.continuation != nil && (target.ID == state.groupTarget || target.Kind == "agent" && target.ResourceID == root) && state.continuation.fits(target, size) {
			state.continuation.append(target, size, false)
			item.Grouping = &state.continuation.grouping
		} else {
			item.Grouping = state.appendPending(target, size, false)
		}
	}
	return result, nil
}

// The execution projector validates retained identities without row locks. The
// remaining reads mirror tryJoinSquad's current-authority checks, but can only
// describe a candidate; they never call its mutation/locking admission helpers.
func (s *CollaborationService) previewSquadContinuation(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, channel db.CollaborationChannel,
	targets map[pgtype.UUID]db.CollaborationTarget, authorized map[pgtype.UUID]bool) (*collaborationPreviewBatch, pgtype.UUID, error) {
	group, err := q.GetCollaborationExecutionGroup(ctx, db.GetCollaborationExecutionGroupParams{SessionID: access.SessionID, ID: channel.ActiveGroupID})
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	target, exists := targets[group.TargetID]
	if !exists {
		return nil, pgtype.UUID{}, invalidCollaborationLedger("preview group target missing")
	}
	if target.Kind != "squad" || !authorized[target.ID] || target.QueueState != "active" || group.ExpansionSealedAt.Valid || group.SettledAt.Valid || group.ForcedDisposition.Valid ||
		group.TargetEpoch != target.TargetEpoch || group.ConfigVersion != target.ConfigVersion {
		return nil, pgtype.UUID{}, nil
	}
	staged, err := q.GetCollaborationBatch(ctx, group.BatchID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	if staged.State != "in_flight" || staged.GroupID != group.ID || staged.ChannelID != channel.ID || staged.EffectiveTargetID != target.ID || staged.RootCommandID != group.CommandID {
		return nil, pgtype.UUID{}, invalidCollaborationLedger("preview group batch mismatch")
	}
	reader := *s
	reader.Queries = q
	view, err := reader.ExecutionView(ctx, access)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	var parent db.CollaborationRun
	for _, run := range view.Runs {
		if run.GroupID == util.UUIDToString(group.ID) && run.SettledAt == nil && run.ForcedDisposition == nil && run.ID > util.UUIDToString(parent.ID) {
			parent, err = q.GetCollaborationRuntimeRun(ctx, util.MustParseUUID(run.ID))
			if err != nil {
				return nil, pgtype.UUID{}, err
			}
		}
	}
	if !parent.ID.Valid {
		return nil, pgtype.UUID{}, nil
	}
	grant, err := q.GetCollaborationGrant(ctx, parent.GrantID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	if grant.Epoch != parent.GrantEpoch || grant.TargetID != target.ID || grant.RuntimeID != parent.RuntimeID || grant.AgentID != parent.AgentID ||
		grant.BindingID != parent.BindingID || grant.BindingEpoch != parent.BindingEpoch || grant.InstallationID != parent.InstallationID ||
		grant.OwnerUserID != parent.ResourceOwnerUserID || util.UUIDToString(parent.DeploymentID) != auth.ManagedDeploymentID() {
		return nil, pgtype.UUID{}, nil
	}
	if code, err := collaborationRuntimeTargetSupported(ctx, q, target); err != nil {
		return nil, pgtype.UUID{}, err
	} else if code != "" {
		return nil, pgtype.UUID{}, nil
	}
	installation, err := q.GetManagedInstallation(ctx, parent.InstallationID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	if installation.Admission != "accepting" {
		return nil, pgtype.UUID{}, nil
	}
	inputs, err := q.ListCollaborationBatchInputs(ctx, group.BatchID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	batch := &collaborationPreviewBatch{squad: target.ID, grouping: CollaborationCommentGrouping{Kind: "squad_continuation"}}
	currentInputs := make(map[pgtype.UUID]db.CollaborationCommand)
	for _, input := range inputs {
		if !groupInputState(input, group.ID) {
			continue
		}
		if !authorized[input.TargetID] {
			return nil, pgtype.UUID{}, nil
		}
		if code, err := collaborationCurrentBatchAuthors(ctx, q, access.Issue.ID, access.LifecycleEpoch, []db.CollaborationCommand{input}, input.State); err != nil {
			return nil, pgtype.UUID{}, err
		} else if code != "" {
			return nil, pgtype.UUID{}, nil
		}
		currentInputs[input.ID] = input
		size, err := collaborationRunInputSize(input)
		if err != nil {
			return nil, pgtype.UUID{}, err
		}
		batch.append(target, size, true)
	}
	// Consumed root inputs consume context budget, but only unread joins are
	// existing contributions to the upcoming continuation.
	batch.grouping.HasExistingInputs = false
	for _, input := range currentInputs {
		batch.grouping.HasExistingInputs = batch.grouping.HasExistingInputs || input.State == "waiting"
	}
	dependencies, err := q.ListCollaborationGroupDependencies(ctx, group.ID)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}
	for _, dependency := range dependencies {
		if dependency.DetachedAt.Valid {
			continue
		}
		input, exists := currentInputs[dependency.CommandID]
		if !exists || input.TargetID != dependency.TargetID {
			return nil, pgtype.UUID{}, invalidCollaborationLedger("preview dependency scope mismatch")
		}
		delete(currentInputs, input.ID)
		current := targets[dependency.TargetID]
		if current.TargetEpoch != dependency.TargetEpoch || current.ConfigVersion != dependency.ConfigVersion || current.ConfigurationFingerprint.String != dependency.ConfigurationFingerprint {
			return nil, pgtype.UUID{}, nil
		}
		frozen, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
		if err != nil {
			return nil, pgtype.UUID{}, err
		}
		grants, err := q.ListCollaborationTargetGrants(ctx, current.ID)
		if err != nil {
			return nil, pgtype.UUID{}, err
		}
		actual := make([]collaborationFrozenGrant, 0, len(frozen))
		for _, grant := range grants {
			if !grant.RevokedAt.Valid && grant.TargetEpoch.Int64 == current.TargetEpoch && grant.ConfigVersion.Int64 == current.ConfigVersion && grant.ConfigurationFingerprint == current.ConfigurationFingerprint {
				actual = append(actual, freezeCollaborationGrant(grant))
			}
		}
		if len(actual) != len(frozen) || len(actual) == 0 {
			return nil, pgtype.UUID{}, nil
		}
		sort.Slice(actual, func(i, j int) bool { return actual[i].GrantID < actual[j].GrantID })
		sort.Slice(frozen, func(i, j int) bool { return frozen[i].GrantID < frozen[j].GrantID })
		for i := range frozen {
			if actual[i] != frozen[i] {
				return nil, pgtype.UUID{}, nil
			}
		}
	}
	if len(currentInputs) != 0 {
		return nil, pgtype.UUID{}, invalidCollaborationLedger("preview input dependencies missing")
	}
	return batch, target.ID, nil
}
