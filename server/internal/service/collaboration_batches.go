package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const maxCollaborationBatchInputs = 1024
const maxCollaborationBatchBytes = (2 << 20) - (32 << 10)

// One projection owns both the admission byte budget and the eventual claim.
func collaborationRunInputs(commands []db.CollaborationCommand) ([]protocol.CollaborationRunInput, error) {
	inputs := make([]protocol.CollaborationRunInput, 0, len(commands))
	for _, command := range commands {
		if command.State != "waiting" && command.State != "in_flight" {
			continue
		}
		attachments := []string{}
		if len(command.Attachments) > 0 {
			if err := json.Unmarshal(command.Attachments, &attachments); err != nil {
				return nil, invalidCollaborationLedger("invalid immutable attachments")
			}
		}
		inputs = append(inputs, protocol.CollaborationRunInput{CommandID: util.UUIDToString(command.ID), AuthorUserID: util.UUIDToString(command.AuthorUserID),
			ParticipantEpoch: strconv.FormatInt(command.ParticipantEpoch, 10), AccountEpoch: strconv.FormatInt(command.AuthorAuthEpoch, 10), LifecycleEpoch: strconv.FormatInt(command.LifecycleEpoch, 10),
			Body: command.Content, AttachmentIDs: attachments})
	}
	return inputs, nil
}

// AppendCollaborationInputInTx is the comment writer's only queue primitive.
// It never commits or writes an event/notification. Domain refusals return a
// receipt so another valid target or human mention can still be committed.
// The caller holds sorted users and source issue/session/targets/channels before
// participants/grants. All authorization reads use the same transaction.
func (s *CollaborationService) AppendCollaborationInputInTx(ctx context.Context, tx pgx.Tx, p CollaborationSubmitCommandParams) (CollaborationQueueReceipt, error) {
	q := s.Queries.WithTx(tx)
	t := &collaborationQueueTx{q: q}
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return t.receipt("forbidden", http.StatusForbidden), nil
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return t.receipt("forbidden", http.StatusForbidden), nil
	}
	t.actor = actor
	if !p.IssueID.Valid || !p.TargetID.Valid || !p.InputEventID.Valid || p.ExpectedTargetEpoch < 1 || p.ExpectedConfigVersion < 1 || strings.TrimSpace(p.Content) == "" {
		return t.receipt("invalid_command_target", http.StatusBadRequest), nil
	}
	t.access, err = s.ResolveIssueAccess(ctx, q, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.session, err = q.GetCollaborationSession(ctx, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.targets, err = q.ListCollaborationTargets(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if !t.access.Capabilities.ControlAI {
		return t.receipt("forbidden", http.StatusForbidden), nil
	}
	t.participant, err = q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: p.IssueID, UserID: actor})
	if errors.Is(err, pgx.ErrNoRows) {
		return t.receipt("forbidden", http.StatusForbidden), nil
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.account, err = q.GetCollaborationAccountFence(ctx, actor)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if t.access.LifecycleEpoch != p.ExpectedLifecycleEpoch || t.participant.Epoch != p.ExpectedParticipantEpoch ||
		t.account.Epoch != p.ExpectedAccountEpoch || t.participant.RevokedAt.Valid {
		return t.receipt("stale_authority", http.StatusConflict), nil
	}
	receipt, _, err := t.appendInput(ctx, p)
	return receipt, err
}

func collaborationRootAgent(target db.CollaborationTarget) (pgtype.UUID, error) {
	if target.Kind == string(collaboration.TargetAgent) {
		return target.ResourceID, nil
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, resource := range roster {
		if resource.Role == "leader" {
			return util.ParseUUID(resource.AgentID)
		}
	}
	return pgtype.UUID{}, invalidCollaborationLedger("target has no root agent")
}

func (t *collaborationQueueTx) ensureTargetChannel(ctx context.Context, target db.CollaborationTarget) (db.CollaborationChannel, error) {
	root, err := collaborationRootAgent(target)
	if err != nil {
		return db.CollaborationChannel{}, err
	}
	channel, err := t.q.EnsureCollaborationChannel(ctx, db.EnsureCollaborationChannelParams{SessionID: t.session.ID, RootAgentID: root})
	if err != nil {
		return db.CollaborationChannel{}, err
	}
	if target.ChannelID != channel.ID {
		if target.ChannelID.Valid {
			return db.CollaborationChannel{}, invalidCollaborationLedger("configured target changed its root channel")
		}
		updated, err := t.q.BindCollaborationTargetChannel(ctx, db.BindCollaborationTargetChannelParams{TargetID: target.ID, SessionID: t.session.ID, ChannelID: channel.ID})
		if err != nil {
			return db.CollaborationChannel{}, err
		}
		t.replaceTarget(updated)
	}
	return channel, nil
}

func (t *collaborationQueueTx) appendInput(ctx context.Context, p CollaborationSubmitCommandParams) (CollaborationQueueReceipt, bool, error) {
	if !t.access.Capabilities.SubmitCommand || (t.access.Issue.AdmissionStatus != "not_required" && t.access.Issue.AdmissionStatus != "accepted") {
		return t.receipt("command_unavailable", http.StatusConflict), false, nil
	}
	target, ok := t.target(p.TargetID)
	if !ok {
		return t.receipt("target_not_found", http.StatusNotFound), false, nil
	}
	prior, err := t.q.GetCollaborationCommandInput(ctx, db.GetCollaborationCommandInputParams{TargetID: target.ID, InputEventID: p.InputEventID})
	if err == nil {
		if prior.Content != p.Content || prior.AuthorUserID != t.actor || prior.ParticipantEpoch != t.participant.Epoch || prior.AuthorAuthEpoch != t.account.Epoch || prior.LifecycleEpoch != t.access.LifecycleEpoch {
			return t.targetReceipt("input_event_conflict", http.StatusConflict, target), false, nil
		}
		r := t.targetReceipt("command_queued", http.StatusCreated, target)
		r.CommandID, r.CommandSequence, r.ChannelID, r.BatchID = prior.ID, prior.Seq, prior.ChannelID, prior.BatchID
		return r, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationQueueReceipt{}, false, err
	}
	if target.TargetEpoch != p.ExpectedTargetEpoch || target.ConfigVersion != p.ExpectedConfigVersion {
		return t.targetReceipt("stale_target", http.StatusConflict, target), false, nil
	}
	if code, err := t.authorizeTargetInput(ctx, target); code != "" || err != nil {
		if err != nil {
			return CollaborationQueueReceipt{}, false, err
		}
		return t.targetReceipt(code, http.StatusConflict, target), false, nil
	}
	channel, err := t.ensureTargetChannel(ctx, target)
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	// Input identity is immutable even when its waiting batch changes effective
	// role from direct agent A to a squad led by A.
	waiting, err := t.q.LockCollaborationWaitingBatches(ctx, channel.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	var batch db.CollaborationBatch
	coalesce := false
	for _, candidate := range waiting {
		effective, exists := t.target(candidate.EffectiveTargetID)
		if !exists {
			return CollaborationQueueReceipt{}, false, invalidCollaborationLedger("waiting batch target missing")
		}
		if target.Kind == string(collaboration.TargetAgent) || candidate.EffectiveTargetID == target.ID || effective.Kind == string(collaboration.TargetAgent) {
			batch = candidate
			if target.Kind == string(collaboration.TargetSquad) && effective.Kind == string(collaboration.TargetAgent) {
				coalesce = true
			}
			break
		}
	}
	var commands []db.CollaborationCommand
	if batch.ID.Valid {
		commands, err = t.q.ListCollaborationBatchInputs(ctx, batch.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, false, err
		}
	}
	commands = append(commands, db.CollaborationCommand{ID: target.ID, AuthorUserID: t.actor, ParticipantEpoch: t.participant.Epoch, AuthorAuthEpoch: t.account.Epoch, LifecycleEpoch: t.access.LifecycleEpoch, Content: p.Content, State: "waiting"})
	inputs, err := collaborationRunInputs(commands)
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	encoded, err := json.Marshal(inputs)
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	if len(inputs) > maxCollaborationBatchInputs || len(encoded) > maxCollaborationBatchBytes {
		return t.targetReceipt("batch_capacity_exceeded", http.StatusConflict, target), false, nil
	}
	if coalesce {
		batch, err = t.q.CoalesceCollaborationBatchTarget(ctx, db.CoalesceCollaborationBatchTargetParams{BatchID: batch.ID, TargetID: target.ID})
		if err != nil {
			return CollaborationQueueReceipt{}, false, err
		}
	}
	if !batch.ID.Valid {
		seq, err := t.q.AllocateCollaborationBatchSequence(ctx, channel.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, false, err
		}
		batch, err = t.q.CreateCollaborationBatch(ctx, db.CreateCollaborationBatchParams{SessionID: t.session.ID, ChannelID: channel.ID, EffectiveTargetID: target.ID, Seq: seq})
		if err != nil {
			return CollaborationQueueReceipt{}, false, err
		}
	}
	seq, err := t.q.AllocateCollaborationCommandSequence(ctx, db.AllocateCollaborationCommandSequenceParams{SessionID: t.session.ID, TargetID: target.ID})
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	command, err := t.q.CreateCollaborationCommand(ctx, db.CreateCollaborationCommandParams{SessionID: t.session.ID, TargetID: target.ID, InputEventID: p.InputEventID,
		Seq: seq, AuthorUserID: t.actor, ParticipantEpoch: t.participant.Epoch, AuthorAuthEpoch: t.account.Epoch, LifecycleEpoch: t.access.LifecycleEpoch, Content: p.Content,
		ChannelID: channel.ID, BatchID: batch.ID})
	if err != nil {
		return CollaborationQueueReceipt{}, false, err
	}
	r := t.targetReceipt("command_queued", http.StatusCreated, target)
	r.CommandID, r.CommandSequence, r.ChannelID, r.BatchID = command.ID, command.Seq, channel.ID, batch.ID
	return r, true, nil
}
