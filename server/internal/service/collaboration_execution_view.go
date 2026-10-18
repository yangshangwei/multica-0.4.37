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
	NextCommandSeq  string                            `json:"next_command_seq"`
	ActiveGroupID   *string                           `json:"active_group_id"`
	PauseReason     string                            `json:"pause_reason"`
	RevokedAt       *string                           `json:"revoked_at"`
}

type CollaborationExecutionCommandView struct {
	ID           string `json:"id"`
	TargetID     string `json:"target_id"`
	ChannelID    string `json:"channel_id"`
	BatchID      string `json:"batch_id"`
	InputEventID string `json:"input_event_id"`
	Sequence     string `json:"sequence"`
	AuthorUserID string `json:"author_user_id"`
	State        string `json:"state"`
}

type CollaborationExecutionGroupView struct {
	ID                string  `json:"id"`
	TargetID          string  `json:"target_id"`
	CommandID         string  `json:"command_id"`
	ChannelID         string  `json:"channel_id"`
	BatchID           string  `json:"batch_id"`
	Generation        string  `json:"generation"`
	TargetEpoch       string  `json:"target_epoch"`
	ConfigVersion     string  `json:"config_version"`
	State             string  `json:"state"`
	ExpansionSealedAt *string `json:"expansion_sealed_at"`
	ForcedDisposition *string `json:"forced_disposition"`
	SettledAt         *string `json:"settled_at"`
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
// an admission decision. Runtime identities, roster, inputs and tokens stay out.
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
	return projectCollaborationExecution(session, gate, channels, targets, commands, groups, runs)
}

func projectCollaborationExecution(session db.CollaborationSession, gate db.CollaborationExecutionGate, channels []db.CollaborationChannel, targets []db.CollaborationTarget, commands []db.CollaborationCommand, groups []db.CollaborationExecutionGroup, runs []db.CollaborationRun) (CollaborationExecutionView, error) {
	invalid := errors.New("collaboration execution identities are unavailable")
	out := CollaborationExecutionView{
		ProtocolVersion: 3, IssueID: util.UUIDToString(session.IssueID), SessionID: util.UUIDToString(session.ID),
		Availability: "unavailable", UnavailableReason: "shared_runtime_unavailable", TargetSetVersion: strconv.FormatInt(session.TargetSetVersion, 10),
		Control:       CollaborationExecutionControlView{Scope: "issue", QueueState: session.QueueState, ControlVersion: strconv.FormatInt(session.ControlVersion, 10)},
		ExecutionGate: CollaborationExecutionGateView{Enabled: gate.Enabled, Epoch: strconv.FormatInt(gate.Epoch, 10)},
		Channels:      make([]CollaborationExecutionChannelView, 0, len(channels)),
		Targets:       make([]CollaborationExecutionTargetView, 0, len(targets)), Commands: make([]CollaborationExecutionCommandView, 0, len(commands)),
		Groups: make([]CollaborationExecutionGroupView, 0, len(groups)), Runs: make([]CollaborationExecutionRunView, 0, len(runs)),
	}
	if !session.ID.Valid || !session.IssueID.Valid || session.TargetSetVersion <= 0 || session.ControlVersion <= 0 || gate.ID != 1 || gate.Epoch <= 0 {
		return CollaborationExecutionView{}, invalid
	}
	channelByID := make(map[pgtype.UUID]db.CollaborationChannel, len(channels))
	for _, channel := range channels {
		if !channel.ID.Valid || channel.SessionID != session.ID || channel.ControlVersion <= 0 || channel.GroupGeneration < 0 ||
			(channel.QueueState != "active" && channel.QueueState != "paused") {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := channelByID[channel.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		channelByID[channel.ID] = channel
		out.Channels = append(out.Channels, CollaborationExecutionChannelView{
			ID: util.UUIDToString(channel.ID), QueueState: channel.QueueState, ControlVersion: strconv.FormatInt(channel.ControlVersion, 10),
			GroupGeneration: strconv.FormatInt(channel.GroupGeneration, 10), ActiveGroupID: util.UUIDToPtr(channel.ActiveGroupID), PauseReason: channel.PauseReason,
		})
	}
	targetByID := make(map[pgtype.UUID]db.CollaborationTarget, len(targets))
	for _, target := range targets {
		if !target.ID.Valid || target.SessionID != session.ID || !target.ResourceID.Valid || target.TargetEpoch <= 0 || target.ConfigVersion <= 0 ||
			target.ControlVersion <= 0 || target.GroupGeneration < 0 || target.NextCommandSeq <= 0 || target.HoldVersion <= 0 ||
			(!target.ChannelID.Valid && (target.GroupGeneration != 0 || target.NextCommandSeq != 1 || target.ActiveGroupID.Valid)) {
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
		targetByID[target.ID] = target
		out.Targets = append(out.Targets, CollaborationExecutionTargetView{
			ID: util.UUIDToString(target.ID), ChannelID: util.UUIDToPtr(target.ChannelID), Kind: target.Kind, ResourceID: util.UUIDToString(target.ResourceID),
			TargetEpoch: strconv.FormatInt(target.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(target.ConfigVersion, 10),
			HoldReason: util.TextToPtr(target.HoldReason), HoldVersion: strconv.FormatInt(target.HoldVersion, 10),
			Control:         CollaborationExecutionControlView{Scope: "target", QueueState: target.QueueState, ControlVersion: strconv.FormatInt(target.ControlVersion, 10)},
			GroupGeneration: strconv.FormatInt(target.GroupGeneration, 10), NextCommandSeq: strconv.FormatInt(target.NextCommandSeq, 10),
			ActiveGroupID: util.UUIDToPtr(target.ActiveGroupID), PauseReason: target.PauseReason, RevokedAt: util.TimestampToPtr(target.RevokedAt),
		})
	}
	commandByID := make(map[pgtype.UUID]db.CollaborationCommand, len(commands))
	batchChannels := make(map[pgtype.UUID]pgtype.UUID)
	for _, command := range commands {
		if !command.ID.Valid || command.SessionID != session.ID || command.Seq <= 0 || !command.ChannelID.Valid || !command.BatchID.Valid {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := commandByID[command.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		commandByID[command.ID] = command

		target, ok := targetByID[command.TargetID]
		if !ok || command.ChannelID != target.ChannelID || !command.InputEventID.Valid || !command.AuthorUserID.Valid || command.Seq >= target.NextCommandSeq {
			return CollaborationExecutionView{}, invalid
		}
		if channel, exists := batchChannels[command.BatchID]; exists && channel != command.ChannelID {
			return CollaborationExecutionView{}, invalid
		}
		batchChannels[command.BatchID] = command.ChannelID
		out.Commands = append(out.Commands, CollaborationExecutionCommandView{
			ID: util.UUIDToString(command.ID), TargetID: util.UUIDToString(command.TargetID), InputEventID: util.UUIDToString(command.InputEventID),
			ChannelID: util.UUIDToString(command.ChannelID), BatchID: util.UUIDToString(command.BatchID),
			Sequence: strconv.FormatInt(command.Seq, 10), AuthorUserID: util.UUIDToString(command.AuthorUserID), State: command.State,
		})
	}
	groupByID := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	groupByBatch := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	type channelGeneration struct {
		channel    pgtype.UUID
		generation int64
	}
	groupGenerations := make(map[channelGeneration]bool, len(groups))
	activeChannels := make(map[pgtype.UUID]bool)
	for _, group := range groups {
		target, targetOK := targetByID[group.TargetID]
		command, commandOK := commandByID[group.CommandID]
		if !group.ID.Valid || group.SessionID != session.ID || !targetOK || !commandOK || command.TargetID != group.TargetID ||
			!group.ChannelID.Valid || !group.BatchID.Valid || group.ChannelID != target.ChannelID || command.ChannelID != group.ChannelID || command.BatchID != group.BatchID ||
			group.Generation <= 0 || group.Generation > target.GroupGeneration || group.TargetEpoch <= 0 || group.TargetEpoch > target.TargetEpoch ||
			group.ConfigVersion <= 0 || group.ConfigVersion > target.ConfigVersion || (!group.SettledAt.Valid && (target.ActiveGroupID != group.ID || group.Generation != target.GroupGeneration)) {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := groupByID[group.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := groupByBatch[group.BatchID]; exists {
			return CollaborationExecutionView{}, invalid
		}
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
			(terminal && (!group.ExpansionSealedAt.Valid || command.State != group.State || group.ForcedDisposition.Valid && group.ForcedDisposition.String != group.State)) ||
			(!terminal && command.State != "in_flight") {
			return CollaborationExecutionView{}, invalid
		}
		groupByID[group.ID] = group
		groupByBatch[group.BatchID] = group
		out.Groups = append(out.Groups, CollaborationExecutionGroupView{
			ID: util.UUIDToString(group.ID), TargetID: util.UUIDToString(group.TargetID), CommandID: util.UUIDToString(group.CommandID),
			ChannelID: util.UUIDToString(group.ChannelID), BatchID: util.UUIDToString(group.BatchID),
			Generation: strconv.FormatInt(group.Generation, 10), TargetEpoch: strconv.FormatInt(group.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(group.ConfigVersion, 10),
			State: group.State, ExpansionSealedAt: util.TimestampToPtr(group.ExpansionSealedAt), ForcedDisposition: util.TextToPtr(group.ForcedDisposition), SettledAt: util.TimestampToPtr(group.SettledAt),
		})
	}
	for _, command := range commands {
		group, exists := groupByBatch[command.BatchID]
		if !exists {
			if command.State != "waiting" && command.State != "cancelled" {
				return CollaborationExecutionView{}, invalid
			}
			continue
		}
		expectedState := "in_flight"
		if group.SettledAt.Valid {
			expectedState = group.State
		}
		// Cancelled waiting inputs retain their batch identity without running.
		if command.ChannelID != group.ChannelID || command.State != "cancelled" && command.State != expectedState {
			return CollaborationExecutionView{}, invalid
		}
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
	for _, run := range runs {
		if !run.ID.Valid || run.SessionID != session.ID {
			return CollaborationExecutionView{}, invalid
		}
		if _, exists := runByID[run.ID]; exists {
			return CollaborationExecutionView{}, invalid
		}
		runByID[run.ID] = run

		group, ok := groupByID[run.GroupID]
		if !ok || group.CommandID != run.CommandID || group.TargetID != run.TargetID || !run.AgentID.Valid || !run.StepKind.Valid || !run.StepKey.Valid ||
			run.ChannelID != group.ChannelID || run.BatchID != group.BatchID ||
			!run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion {
			return CollaborationExecutionView{}, invalid
		}
		if run.SettledAt.Valid != (run.ProgressState == "settled") || (group.SettledAt.Valid && !run.SettledAt.Valid) ||
			(run.SettledAt.Valid && run.ProofState != "never_launched_proven" && run.ProofState != "never_started_proven" && run.ProofState != "exited_proven") {
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
		out.Runs = append(out.Runs, CollaborationExecutionRunView{
			ID: util.UUIDToString(run.ID), CommandID: util.UUIDToString(run.CommandID), TargetID: util.UUIDToString(run.TargetID), GroupID: util.UUIDToString(run.GroupID),
			ChannelID: util.UUIDToString(run.ChannelID), BatchID: util.UUIDToString(run.BatchID),
			ParentRunID: util.UUIDToPtr(run.ParentRunID), StepKey: run.StepKey.String, StepKind: run.StepKind.String, AgentID: util.UUIDToString(run.AgentID),
			TargetEpoch: strconv.FormatInt(run.TargetEpoch.Int64, 10), ConfigVersion: strconv.FormatInt(run.ConfigVersion.Int64, 10),
			ProgressState: run.ProgressState, ProofState: run.ProofState, BusinessOutcome: util.TextToPtr(run.BusinessOutcome), ForcedDisposition: util.TextToPtr(run.ForcedDisposition),
			RecordingIncomplete: run.RecordingIncomplete, SettledAt: util.TimestampToPtr(run.SettledAt),
		})
	}
	for _, group := range groups {
		if group.SettledAt.Valid && !rootGroups[group.ID] {
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
