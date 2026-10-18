package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationExecutionControlView struct {
	Scope          string `json:"scope"`
	QueueState     string `json:"queue_state"`
	ControlVersion string `json:"control_version"`
}

type CollaborationExecutionGateView struct {
	Enabled bool   `json:"enabled"`
	Epoch   string `json:"epoch"`
}

type CollaborationExecutionChannelView struct {
	ID              string  `json:"id"`
	QueueState      string  `json:"queue_state"`
	ControlVersion  string  `json:"control_version"`
	GroupGeneration string  `json:"group_generation"`
	NextCommandSeq  string  `json:"next_command_seq"`
	ActiveGroupID   *string `json:"active_group_id"`
	PauseReason     string  `json:"pause_reason"`
}

type CollaborationExecutionTargetView struct {
	ID              string                            `json:"id"`
	ChannelID       *string                           `json:"channel_id"`
	Kind            string                            `json:"kind"`
	ResourceID      string                            `json:"resource_id"`
	TargetEpoch     string                            `json:"target_epoch"`
	ConfigVersion   string                            `json:"config_version"`
	HoldReason      *string                           `json:"hold_reason"`
	HoldVersion     string                            `json:"hold_version"`
	Control         CollaborationExecutionControlView `json:"control"`
	GroupGeneration string                            `json:"group_generation"`
	ActiveGroupID   *string                           `json:"active_group_id"`
	PauseReason     string                            `json:"pause_reason"`
	RevokedAt       *string                           `json:"revoked_at"`
}

type CollaborationExecutionCommandView struct {
	ID           string  `json:"id"`
	TargetID     string  `json:"target_id"`
	ChannelID    string  `json:"channel_id"`
	BatchID      string  `json:"batch_id"`
	GroupID      *string `json:"group_id"`
	InputEventID string  `json:"input_event_id"`
	Sequence     string  `json:"sequence"`
	AuthorUserID string  `json:"author_user_id"`
	State        string  `json:"state"`
}

type CollaborationExecutionGroupInputView struct {
	CommandID      string `json:"command_id"`
	ChannelID      string `json:"channel_id"`
	BatchID        string `json:"batch_id"`
	Sequence       string `json:"sequence"`
	ConsumingRunID string `json:"consuming_run_id"`
}

type CollaborationExecutionGroupView struct {
	ID                string                                 `json:"id"`
	TargetID          string                                 `json:"target_id"`
	CommandID         string                                 `json:"command_id"`
	ChannelID         string                                 `json:"channel_id"`
	BatchID           string                                 `json:"batch_id"`
	Generation        string                                 `json:"generation"`
	TargetEpoch       string                                 `json:"target_epoch"`
	ConfigVersion     string                                 `json:"config_version"`
	State             string                                 `json:"state"`
	ExpansionSealedAt *string                                `json:"expansion_sealed_at"`
	ForcedDisposition *string                                `json:"forced_disposition"`
	SettledAt         *string                                `json:"settled_at"`
	Inputs            []CollaborationExecutionGroupInputView `json:"inputs"`
}

type CollaborationExecutionRunView struct {
	ID                  string  `json:"id"`
	CommandID           string  `json:"command_id"`
	TargetID            string  `json:"target_id"`
	GroupID             string  `json:"group_id"`
	ChannelID           string  `json:"channel_id"`
	BatchID             string  `json:"batch_id"`
	ParentRunID         *string `json:"parent_run_id"`
	StepKey             string  `json:"step_key"`
	StepKind            string  `json:"step_kind"`
	AgentID             string  `json:"agent_id"`
	TargetEpoch         string  `json:"target_epoch"`
	ConfigVersion       string  `json:"config_version"`
	ProgressState       string  `json:"progress_state"`
	ProofState          string  `json:"proof_state"`
	BusinessOutcome     *string `json:"business_outcome"`
	ForcedDisposition   *string `json:"forced_disposition"`
	RecordingIncomplete bool    `json:"recording_incomplete"`
	SettledAt           *string `json:"settled_at"`
}

// CollaborationExecutionView is a read projection, never a grant catalogue or
// an admission decision. Runtime identities, roster, input bodies and tokens stay out.
type CollaborationExecutionView struct {
	ProtocolVersion   int                                 `json:"protocol_version"`
	IssueID           string                              `json:"issue_id"`
	SessionID         string                              `json:"session_id"`
	Availability      string                              `json:"availability"`
	UnavailableReason string                              `json:"unavailable_reason"`
	TargetSetVersion  string                              `json:"target_set_version"`
	Control           CollaborationExecutionControlView   `json:"control"`
	ExecutionGate     CollaborationExecutionGateView      `json:"execution_gate"`
	Channels          []CollaborationExecutionChannelView `json:"channels"`
	Targets           []CollaborationExecutionTargetView  `json:"targets"`
	Commands          []CollaborationExecutionCommandView `json:"commands"`
	Groups            []CollaborationExecutionGroupView   `json:"groups"`
	Runs              []CollaborationExecutionRunView     `json:"runs"`
}

// ExecutionView borrows the caller's authorized, repeatable-read snapshot,
// just like IssueFields. It performs no writes and does not reauthorize grants.
func (s *CollaborationService) ExecutionView(ctx context.Context, access CollaborationIssueAccess) (CollaborationExecutionView, error) {
	var out CollaborationExecutionView
	if !access.Capabilities.Read {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	session, err := s.Queries.GetCollaborationSession(ctx, access.Issue.ID)
	if err != nil {
		return out, err
	}
	if session.ID != access.SessionID || session.IssueID != access.Issue.ID || session.QueueState != access.QueueState || session.ControlVersion != access.ControlVersion {
		return out, errors.New("collaboration execution snapshot mismatch")
	}
	gate, err := s.Queries.GetCollaborationGate(ctx)
	if err != nil {
		return out, err
	}
	channels, err := s.Queries.ListCollaborationChannels(ctx, session.ID)
	if err != nil {
		return out, err
	}
	targets, err := s.Queries.ListCollaborationTargets(ctx, session.ID)
	if err != nil {
		return out, err
	}
	commands, err := s.Queries.ListCollaborationCommands(ctx, session.ID)
	if err != nil {
		return out, err
	}
	groups, err := s.Queries.ListCollaborationExecutionGroups(ctx, session.ID)
	if err != nil {
		return out, err
	}
	runs, err := s.Queries.ListCollaborationRuns(ctx, session.ID)
	if err != nil {
		return out, err
	}
	inputs, err := s.Queries.ListCollaborationSessionGroupInputs(ctx, session.ID)
	if err != nil {
		return out, err
	}
	return projectCollaborationExecution(session, gate, channels, targets, commands, groups, runs, inputs)
}

func projectCollaborationExecution(session db.CollaborationSession, gate db.CollaborationExecutionGate, channels []db.CollaborationChannel, targets []db.CollaborationTarget, commands []db.CollaborationCommand, groups []db.CollaborationExecutionGroup, runs []db.CollaborationRun, inputs []db.CollaborationGroupInput) (CollaborationExecutionView, error) {
	invalid := errors.New("collaboration execution identities are unavailable")
	out := CollaborationExecutionView{
		ProtocolVersion: 4, IssueID: util.UUIDToString(session.IssueID), SessionID: util.UUIDToString(session.ID),
		Availability: "unavailable", UnavailableReason: "shared_runtime_unavailable", TargetSetVersion: strconv.FormatInt(session.TargetSetVersion, 10),
		Control:       CollaborationExecutionControlView{Scope: "issue", QueueState: session.QueueState, ControlVersion: strconv.FormatInt(session.ControlVersion, 10)},
		ExecutionGate: CollaborationExecutionGateView{Enabled: gate.Enabled, Epoch: strconv.FormatInt(gate.Epoch, 10)},
		Channels:      make([]CollaborationExecutionChannelView, 0, len(channels)),
		Targets:       make([]CollaborationExecutionTargetView, 0, len(targets)), Commands: make([]CollaborationExecutionCommandView, 0, len(commands)),
		Groups: make([]CollaborationExecutionGroupView, 0, len(groups)), Runs: make([]CollaborationExecutionRunView, 0, len(runs)),
	}
	if !session.ID.Valid || !session.IssueID.Valid || session.TargetSetVersion <= 0 || session.ControlVersion <= 0 || gate.ID != 1 || gate.Epoch <= 0 ||
		(session.QueueState != "active" && session.QueueState != "paused" && session.QueueState != "closed") {
		return CollaborationExecutionView{}, invalid
	}
	channelByID := make(map[pgtype.UUID]db.CollaborationChannel, len(channels))
	for _, channel := range channels {
		if !channel.ID.Valid || channel.SessionID != session.ID || channel.ControlVersion <= 0 || channel.GroupGeneration < 0 || channel.NextCommandSeq <= 0 ||
			(channel.QueueState != "active" && channel.QueueState != "paused") {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := channelByID[channel.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		channelByID[channel.ID] = channel
		out.Channels = append(out.Channels, CollaborationExecutionChannelView{
			ID: util.UUIDToString(channel.ID), QueueState: channel.QueueState, ControlVersion: strconv.FormatInt(channel.ControlVersion, 10),
			GroupGeneration: strconv.FormatInt(channel.GroupGeneration, 10), NextCommandSeq: strconv.FormatInt(channel.NextCommandSeq, 10), ActiveGroupID: util.UUIDToPtr(channel.ActiveGroupID), PauseReason: channel.PauseReason,
		})
	}
	targetByID := make(map[pgtype.UUID]db.CollaborationTarget, len(targets))
	targetIdentities := make(map[string]bool, len(targets))
	for _, target := range targets {
		if !target.ID.Valid || target.SessionID != session.ID || !target.ResourceID.Valid || target.TargetEpoch <= 0 || target.ConfigVersion <= 0 ||
			target.ControlVersion <= 0 || target.GroupGeneration < 0 || target.HoldVersion <= 0 ||
			(!target.ChannelID.Valid && (target.GroupGeneration != 0 || target.ActiveGroupID.Valid)) ||
			(target.Kind != "agent" && target.Kind != "squad") || (target.QueueState != "active" && target.QueueState != "paused") {
			return CollaborationExecutionView{}, invalid
		}
		if target.HoldReason.Valid && !collaboration.TargetHoldReason(target.HoldReason.String).Valid() {
			return CollaborationExecutionView{}, invalid
		}
		if target.ChannelID.Valid {
			channel, exists := channelByID[target.ChannelID]
			if !exists || channel.QueueState != target.QueueState || channel.ControlVersion != target.ControlVersion ||
				channel.GroupGeneration != target.GroupGeneration || channel.ActiveGroupID != target.ActiveGroupID || channel.PauseReason != target.PauseReason {
				return CollaborationExecutionView{}, invalid
			}
		}
		if _, exists := targetByID[target.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		identity := target.Kind + ":" + util.UUIDToString(target.ResourceID)
		if targetIdentities[identity] {
			return CollaborationExecutionView{}, invalid
		}
		targetIdentities[identity] = true
		targetByID[target.ID] = target
		out.Targets = append(out.Targets, CollaborationExecutionTargetView{
			ID: util.UUIDToString(target.ID), ChannelID: util.UUIDToPtr(target.ChannelID), Kind: target.Kind, ResourceID: util.UUIDToString(target.ResourceID),
			TargetEpoch: strconv.FormatInt(target.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(target.ConfigVersion, 10),
			HoldReason: util.TextToPtr(target.HoldReason), HoldVersion: strconv.FormatInt(target.HoldVersion, 10),
			Control:         CollaborationExecutionControlView{Scope: "target", QueueState: target.QueueState, ControlVersion: strconv.FormatInt(target.ControlVersion, 10)},
			GroupGeneration: strconv.FormatInt(target.GroupGeneration, 10),
			ActiveGroupID:   util.UUIDToPtr(target.ActiveGroupID), PauseReason: target.PauseReason, RevokedAt: util.TimestampToPtr(target.RevokedAt),
		})
	}
	commandByID := make(map[pgtype.UUID]db.CollaborationCommand, len(commands))
	batchChannels := make(map[pgtype.UUID]pgtype.UUID)
	type channelSequence struct {
		channel pgtype.UUID
		seq     int64
	}
	sequenceOwners := make(map[channelSequence]pgtype.UUID, len(commands))
	inputTargets := make(map[[2]pgtype.UUID]bool, len(commands))
	for _, command := range commands {
		if !command.ID.Valid || command.SessionID != session.ID || command.Seq <= 0 || !command.ChannelID.Valid || !command.BatchID.Valid {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := commandByID[command.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		commandByID[command.ID] = command

		target, ok := targetByID[command.TargetID]
		channel, channelOK := channelByID[command.ChannelID]
		if !ok || !target.ChannelID.Valid || !channelOK || !command.InputEventID.Valid || !command.AuthorUserID.Valid || command.Seq >= channel.NextCommandSeq ||
			((command.State == "waiting" || command.State == "in_flight") && command.ChannelID != target.ChannelID) {
			return CollaborationExecutionView{}, invalid
		}
		switch command.State {
		case "waiting", "in_flight", "succeeded", "failed", "stopped", "cancelled":
		default:
			return CollaborationExecutionView{}, invalid
		}
		position := channelSequence{command.ChannelID, command.Seq}
		inputTarget := [2]pgtype.UUID{command.InputEventID, command.TargetID}
		if _, exists := sequenceOwners[position]; exists || inputTargets[inputTarget] {
			return CollaborationExecutionView{}, invalid
		}
		sequenceOwners[position], inputTargets[inputTarget] = command.ID, true
		if channel, exists := batchChannels[command.BatchID]; exists && channel != command.ChannelID {
			return CollaborationExecutionView{}, invalid
		}
		batchChannels[command.BatchID] = command.ChannelID
		out.Commands = append(out.Commands, CollaborationExecutionCommandView{
			ID: util.UUIDToString(command.ID), TargetID: util.UUIDToString(command.TargetID), InputEventID: util.UUIDToString(command.InputEventID),
			ChannelID: util.UUIDToString(command.ChannelID), BatchID: util.UUIDToString(command.BatchID), GroupID: util.UUIDToPtr(command.GroupID),
			Sequence: strconv.FormatInt(command.Seq, 10), AuthorUserID: util.UUIDToString(command.AuthorUserID), State: command.State,
		})
	}
	groupByID := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	groupByBatch := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	groupIndexes := make(map[pgtype.UUID]int, len(groups))
	type channelGeneration struct {
		channel    pgtype.UUID
		generation int64
	}
	groupGenerations := make(map[channelGeneration]bool, len(groups))
	activeChannels := make(map[pgtype.UUID]bool)
	for _, group := range groups {
		target, targetOK := targetByID[group.TargetID]
		command, commandOK := commandByID[group.CommandID]
		channel, channelOK := channelByID[group.ChannelID]
		if !group.ID.Valid || group.SessionID != session.ID || !targetOK || !commandOK || command.TargetID != group.TargetID ||
			!channelOK || !group.BatchID.Valid || group.Generation <= 0 || group.Generation > channel.GroupGeneration || group.TargetEpoch <= 0 || group.TargetEpoch > target.TargetEpoch ||
			group.ConfigVersion <= 0 || group.ConfigVersion > target.ConfigVersion ||
			(!group.SettledAt.Valid && (target.ChannelID != group.ChannelID || channel.ActiveGroupID != group.ID || group.Generation != channel.GroupGeneration)) ||
			(group.ForcedDisposition.Valid && group.ForcedDisposition.String != "stopped" && group.ForcedDisposition.String != "cancelled") {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := groupByID[group.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := groupByBatch[group.BatchID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		if channel, exists := batchChannels[group.BatchID]; exists && channel != group.ChannelID {
			return CollaborationExecutionView{}, invalid
		}
		batchChannels[group.BatchID] = group.ChannelID
		generation := channelGeneration{group.ChannelID, group.Generation}
		if groupGenerations[generation] || (!group.SettledAt.Valid && activeChannels[group.ChannelID]) {
			return CollaborationExecutionView{}, invalid
		}
		groupGenerations[generation] = true
		if !group.SettledAt.Valid {
			activeChannels[group.ChannelID] = true
		}
		terminal := false
		switch group.State {
		case "active", "waiting_children", "ending":
		case "succeeded", "failed", "stopped", "cancelled":
			terminal = true
		default:
			return CollaborationExecutionView{}, invalid
		}
		if terminal != group.SettledAt.Valid ||
			(terminal && (!group.ExpansionSealedAt.Valid || group.ForcedDisposition.Valid && group.ForcedDisposition.String != group.State)) {
			return CollaborationExecutionView{}, invalid
		}
		groupByID[group.ID] = group
		groupByBatch[group.BatchID] = group
		groupIndexes[group.ID] = len(out.Groups)
		out.Groups = append(out.Groups, CollaborationExecutionGroupView{
			ID: util.UUIDToString(group.ID), TargetID: util.UUIDToString(group.TargetID), CommandID: util.UUIDToString(group.CommandID),
			ChannelID: util.UUIDToString(group.ChannelID), BatchID: util.UUIDToString(group.BatchID),
			Generation: strconv.FormatInt(group.Generation, 10), TargetEpoch: strconv.FormatInt(group.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(group.ConfigVersion, 10),
			State: group.State, ExpansionSealedAt: util.TimestampToPtr(group.ExpansionSealedAt), ForcedDisposition: util.TextToPtr(group.ForcedDisposition), SettledAt: util.TimestampToPtr(group.SettledAt),
			Inputs: make([]CollaborationExecutionGroupInputView, 0),
		})
	}
	for _, channel := range channels {
		if channel.ActiveGroupID.Valid {
			group, ok := groupByID[channel.ActiveGroupID]
			if !ok || group.ChannelID != channel.ID || group.SettledAt.Valid || group.Generation != channel.GroupGeneration {
				return CollaborationExecutionView{}, invalid
			}
		}
	}
	runByID := make(map[pgtype.UUID]db.CollaborationRun, len(runs))
	rootGroups := make(map[pgtype.UUID]bool)
	runSteps := make(map[string]bool, len(runs))
	allRunsNeverLaunched := make(map[pgtype.UUID]bool, len(groups))
	for _, run := range runs {
		if !run.ID.Valid || run.SessionID != session.ID {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := runByID[run.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		runByID[run.ID] = run

		group, ok := groupByID[run.GroupID]
		if !ok || group.CommandID != run.CommandID || group.TargetID != run.TargetID || !run.AgentID.Valid || !run.StepKind.Valid || !run.StepKey.Valid || run.StepKey.String == "" || len(run.StepKey.String) > 128 ||
			run.ChannelID != group.ChannelID || run.BatchID != group.BatchID ||
			!run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion {
			return CollaborationExecutionView{}, invalid
		}
		step := util.UUIDToString(run.GroupID) + ":" + run.StepKey.String
		if runSteps[step] || (run.BusinessOutcome.Valid && run.BusinessOutcome.String != "success" && run.BusinessOutcome.String != "failure") ||
			(run.ForcedDisposition.Valid && run.ForcedDisposition.String != "stopped" && run.ForcedDisposition.String != "cancelled") {
			return CollaborationExecutionView{}, invalid
		}
		runSteps[step] = true
		switch run.ProgressState {
		case "prepared", "dispatched", "preparing", "launch_authorized", "running", "ending", "settled":
		default:
			return CollaborationExecutionView{}, invalid
		}
		switch run.ProofState {
		case "unknown", "never_launched_proven", "never_started_proven", "exited_proven":
		default:
			return CollaborationExecutionView{}, invalid
		}
		provenEnded := collaborationRunProvenEnded(run)
		if run.SettledAt.Valid != (run.ProgressState == "settled") || (group.SettledAt.Valid && !run.SettledAt.Valid) ||
			(run.SettledAt.Valid && !provenEnded) {
			return CollaborationExecutionView{}, invalid
		}
		target := targetByID[run.TargetID]
		if (target.Kind == "agent" && (run.StepKind.String != "agent" || run.AgentID != target.ResourceID)) || (target.Kind == "squad" && run.StepKind.String == "agent") {
			return CollaborationExecutionView{}, invalid
		}
		switch run.StepKind.String {
		case "agent", "leader":
			if run.ParentRunID.Valid || rootGroups[run.GroupID] {
				return CollaborationExecutionView{}, invalid
			}
			rootGroups[run.GroupID] = true
		case "member", "continuation":
			if !run.ParentRunID.Valid {
				return CollaborationExecutionView{}, invalid
			}
		default:
			return CollaborationExecutionView{}, invalid
		}
		previous, seen := allRunsNeverLaunched[run.GroupID]
		if !seen {
			previous = true
		}
		allRunsNeverLaunched[run.GroupID] = previous && provenEnded && run.ProofState == "never_launched_proven"
		out.Runs = append(out.Runs, CollaborationExecutionRunView{
			ID: util.UUIDToString(run.ID), CommandID: util.UUIDToString(run.CommandID), TargetID: util.UUIDToString(run.TargetID), GroupID: util.UUIDToString(run.GroupID),
			ChannelID: util.UUIDToString(run.ChannelID), BatchID: util.UUIDToString(run.BatchID),
			ParentRunID: util.UUIDToPtr(run.ParentRunID), StepKey: run.StepKey.String, StepKind: run.StepKind.String, AgentID: util.UUIDToString(run.AgentID),
			TargetEpoch: strconv.FormatInt(run.TargetEpoch.Int64, 10), ConfigVersion: strconv.FormatInt(run.ConfigVersion.Int64, 10),
			ProgressState: run.ProgressState, ProofState: run.ProofState, BusinessOutcome: util.TextToPtr(run.BusinessOutcome), ForcedDisposition: util.TextToPtr(run.ForcedDisposition),
			RecordingIncomplete: run.RecordingIncomplete, SettledAt: util.TimestampToPtr(run.SettledAt),
		})
	}
	inputsByGroupCommand := make(map[[2]pgtype.UUID]db.CollaborationGroupInput, len(inputs))
	type groupSequence struct {
		group pgtype.UUID
		seq   int64
	}
	groupSequences := make(map[groupSequence]bool, len(inputs))
	for _, input := range inputs {
		group, groupOK := groupByID[input.GroupID]
		command, commandOK := commandByID[input.CommandID]
		consumer, consumerOK := runByID[input.ConsumingRunID]
		if !groupOK || !commandOK || input.ChannelID != group.ChannelID || input.BatchID != group.BatchID || input.Seq <= 0 ||
			input.Seq >= channelByID[group.ChannelID].NextCommandSeq || !consumerOK || consumer.GroupID != group.ID ||
			(consumer.StepKind.String != "agent" && consumer.StepKind.String != "leader") {
			return CollaborationExecutionView{}, invalid
		}
		target, commandTarget := targetByID[group.TargetID], targetByID[command.TargetID]
		if command.TargetID != target.ID && (target.Kind != "squad" || commandTarget.Kind != "agent" || commandTarget.ResourceID != consumer.AgentID) {
			return CollaborationExecutionView{}, invalid
		}
		key, sequence := [2]pgtype.UUID{input.GroupID, input.CommandID}, groupSequence{input.GroupID, input.Seq}
		if _, exists := inputsByGroupCommand[key]; exists || groupSequences[sequence] {
			return CollaborationExecutionView{}, invalid
		}
		position := channelSequence{input.ChannelID, input.Seq}
		if previous, exists := sequenceOwners[position]; exists && previous != input.CommandID {
			return CollaborationExecutionView{}, invalid
		}
		sequenceOwners[position], groupSequences[sequence], inputsByGroupCommand[key] = input.CommandID, true, input
		if command.GroupID != group.ID {
			// This slice only consumes initial root inputs. A returned command
			// preserves its original membership and needs complete nonlaunch proof.
			if !group.SettledAt.Valid || group.State == "succeeded" || !allRunsNeverLaunched[group.ID] {
				return CollaborationExecutionView{}, invalid
			}
			if current, exists := groupByID[command.GroupID]; exists && current.ChannelID == group.ChannelID && current.Generation <= group.Generation {
				return CollaborationExecutionView{}, invalid
			}
		}
		index := groupIndexes[group.ID]
		out.Groups[index].Inputs = append(out.Groups[index].Inputs, CollaborationExecutionGroupInputView{
			CommandID: util.UUIDToString(input.CommandID), ChannelID: util.UUIDToString(input.ChannelID), BatchID: util.UUIDToString(input.BatchID),
			Sequence: strconv.FormatInt(input.Seq, 10), ConsumingRunID: util.UUIDToString(input.ConsumingRunID),
		})
	}
	for _, group := range groups {
		if _, exists := inputsByGroupCommand[[2]pgtype.UUID{group.ID, group.CommandID}]; !exists || !rootGroups[group.ID] {
			return CollaborationExecutionView{}, invalid
		}
	}
	for _, command := range commands {
		if !command.GroupID.Valid {
			if command.State != "waiting" && command.State != "cancelled" {
				return CollaborationExecutionView{}, invalid
			}
			if _, exists := groupByBatch[command.BatchID]; exists && command.State == "waiting" {
				return CollaborationExecutionView{}, invalid
			}
			continue
		}
		group, groupOK := groupByID[command.GroupID]
		input, inputOK := inputsByGroupCommand[[2]pgtype.UUID{command.GroupID, command.ID}]
		if !groupOK || !inputOK || command.ChannelID != group.ChannelID || command.BatchID != group.BatchID || command.Seq != input.Seq {
			return CollaborationExecutionView{}, invalid
		}
		expected := "in_flight"
		if group.SettledAt.Valid {
			expected = group.State
		}
		cancelledBeforeLaunch := command.State == "cancelled" && group.SettledAt.Valid && group.State != "succeeded" && allRunsNeverLaunched[group.ID]
		if command.State != expected && !cancelledBeforeLaunch {
			return CollaborationExecutionView{}, invalid
		}
	}
	checked := make(map[pgtype.UUID]bool, len(runs))
	for _, run := range runs {
		path := make(map[pgtype.UUID]bool)
		for current := run; !checked[current.ID]; {
			if path[current.ID] {
				return CollaborationExecutionView{}, invalid
			}
			path[current.ID] = true
			if !current.ParentRunID.Valid {
				break
			}
			parent, ok := runByID[current.ParentRunID]
			if !ok || parent.GroupID != current.GroupID {
				return CollaborationExecutionView{}, invalid
			}
			current = parent
		}
		for id := range path {
			checked[id] = true
		}
	}

	return out, nil
}
