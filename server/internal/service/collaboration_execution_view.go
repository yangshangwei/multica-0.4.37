package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationExecutionControlView struct {
	Scope          string `json:"scope"`
	QueueState     string `json:"queue_state"`
	ControlVersion string `json:"control_version"`
}

type CollaborationExecutionTargetView struct {
	ID              string                            `json:"id"`
	Kind            string                            `json:"kind"`
	ResourceID      string                            `json:"resource_id"`
	TargetEpoch     string                            `json:"target_epoch"`
	ConfigVersion   string                            `json:"config_version"`
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
	InputEventID string `json:"input_event_id"`
	Sequence     string `json:"sequence"`
	AuthorUserID string `json:"author_user_id"`
	State        string `json:"state"`
}

type CollaborationExecutionGroupView struct {
	ID                string  `json:"id"`
	TargetID          string  `json:"target_id"`
	CommandID         string  `json:"command_id"`
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

type CollaborationExecutionLegacyView struct {
	CommandIDs  []string `json:"command_ids"`
	RunIDs      []string `json:"run_ids"`
	ActiveRunID *string  `json:"active_run_id"`
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
	Targets           []CollaborationExecutionTargetView  `json:"targets"`
	Commands          []CollaborationExecutionCommandView `json:"commands"`
	Groups            []CollaborationExecutionGroupView   `json:"groups"`
	Runs              []CollaborationExecutionRunView     `json:"runs"`
	LegacyUnresolved  CollaborationExecutionLegacyView    `json:"legacy_unresolved"`
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
	return projectCollaborationExecution(session, targets, commands, groups, runs)
}

func projectCollaborationExecution(session db.CollaborationSession, targets []db.CollaborationTarget, commands []db.CollaborationCommand, groups []db.CollaborationExecutionGroup, runs []db.CollaborationRun) (CollaborationExecutionView, error) {
	invalid := errors.New("collaboration execution identities are unavailable")
	out := CollaborationExecutionView{
		ProtocolVersion: 2, IssueID: util.UUIDToString(session.IssueID), SessionID: util.UUIDToString(session.ID),
		Availability: "unavailable", UnavailableReason: "shared_runtime_unavailable", TargetSetVersion: strconv.FormatInt(session.TargetSetVersion, 10),
		Control: CollaborationExecutionControlView{Scope: "issue", QueueState: session.QueueState, ControlVersion: strconv.FormatInt(session.ControlVersion, 10)},
		Targets: make([]CollaborationExecutionTargetView, 0, len(targets)), Commands: make([]CollaborationExecutionCommandView, 0, len(commands)),
		Groups: make([]CollaborationExecutionGroupView, 0, len(groups)), Runs: make([]CollaborationExecutionRunView, 0, len(runs)),
		LegacyUnresolved: CollaborationExecutionLegacyView{CommandIDs: []string{}, RunIDs: []string{}, ActiveRunID: util.UUIDToPtr(session.ActiveRunID)},
	}
	if !session.ID.Valid || !session.IssueID.Valid || session.TargetSetVersion <= 0 || session.ControlVersion <= 0 {
		return CollaborationExecutionView{}, invalid
	}
	targetByID := make(map[pgtype.UUID]db.CollaborationTarget, len(targets))
	for _, target := range targets {
		if !target.ID.Valid || target.SessionID != session.ID || !target.ResourceID.Valid || target.TargetEpoch <= 0 || target.ConfigVersion <= 0 ||
			target.ControlVersion <= 0 || target.GroupGeneration < 0 || target.NextCommandSeq <= 0 {
			return CollaborationExecutionView{}, invalid
		}
		targetByID[target.ID] = target
		out.Targets = append(out.Targets, CollaborationExecutionTargetView{
			ID: util.UUIDToString(target.ID), Kind: target.Kind, ResourceID: util.UUIDToString(target.ResourceID),
			TargetEpoch: strconv.FormatInt(target.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(target.ConfigVersion, 10),
			Control:         CollaborationExecutionControlView{Scope: "target", QueueState: target.QueueState, ControlVersion: strconv.FormatInt(target.ControlVersion, 10)},
			GroupGeneration: strconv.FormatInt(target.GroupGeneration, 10), NextCommandSeq: strconv.FormatInt(target.NextCommandSeq, 10),
			ActiveGroupID: util.UUIDToPtr(target.ActiveGroupID), PauseReason: target.PauseReason, RevokedAt: util.TimestampToPtr(target.RevokedAt),
		})
	}
	commandByID := make(map[pgtype.UUID]db.CollaborationCommand, len(commands))
	for _, command := range commands {
		if !command.ID.Valid || command.SessionID != session.ID || command.Seq <= 0 {
			return CollaborationExecutionView{}, invalid
		}
		commandByID[command.ID] = command
		if command.LegacyUnresolved {
			if command.TargetID.Valid || command.InputEventID.Valid || command.LegacySessionID != session.ID {
				return CollaborationExecutionView{}, invalid
			}
			out.LegacyUnresolved.CommandIDs = append(out.LegacyUnresolved.CommandIDs, util.UUIDToString(command.ID))
			continue
		}
		target, ok := targetByID[command.TargetID]
		if !ok || !command.InputEventID.Valid || !command.AuthorUserID.Valid || command.Seq >= target.NextCommandSeq {
			return CollaborationExecutionView{}, invalid
		}
		out.Commands = append(out.Commands, CollaborationExecutionCommandView{
			ID: util.UUIDToString(command.ID), TargetID: util.UUIDToString(command.TargetID), InputEventID: util.UUIDToString(command.InputEventID),
			Sequence: strconv.FormatInt(command.Seq, 10), AuthorUserID: util.UUIDToString(command.AuthorUserID), State: command.State,
		})
	}
	groupByID := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	for _, group := range groups {
		target, targetOK := targetByID[group.TargetID]
		command, commandOK := commandByID[group.CommandID]
		if !group.ID.Valid || group.SessionID != session.ID || !targetOK || !commandOK || command.LegacyUnresolved || command.TargetID != group.TargetID ||
			group.Generation <= 0 || group.Generation > target.GroupGeneration || group.TargetEpoch <= 0 || group.TargetEpoch > target.TargetEpoch ||
			group.ConfigVersion <= 0 || group.ConfigVersion > target.ConfigVersion || (!group.SettledAt.Valid && (target.ActiveGroupID != group.ID || group.Generation != target.GroupGeneration)) {
			return CollaborationExecutionView{}, invalid
		}
		groupByID[group.ID] = group
		out.Groups = append(out.Groups, CollaborationExecutionGroupView{
			ID: util.UUIDToString(group.ID), TargetID: util.UUIDToString(group.TargetID), CommandID: util.UUIDToString(group.CommandID),
			Generation: strconv.FormatInt(group.Generation, 10), TargetEpoch: strconv.FormatInt(group.TargetEpoch, 10), ConfigVersion: strconv.FormatInt(group.ConfigVersion, 10),
			State: group.State, ExpansionSealedAt: util.TimestampToPtr(group.ExpansionSealedAt), ForcedDisposition: util.TextToPtr(group.ForcedDisposition), SettledAt: util.TimestampToPtr(group.SettledAt),
		})
	}
	for _, target := range targets {
		if target.ActiveGroupID.Valid {
			group, ok := groupByID[target.ActiveGroupID]
			if !ok || group.TargetID != target.ID || group.SettledAt.Valid {
				return CollaborationExecutionView{}, invalid
			}
		}
	}
	runByID := make(map[pgtype.UUID]db.CollaborationRun, len(runs))
	for _, run := range runs {
		if !run.ID.Valid || run.SessionID != session.ID {
			return CollaborationExecutionView{}, invalid
		}
		runByID[run.ID] = run
		if run.LegacyUnresolved {
			command, ok := commandByID[run.CommandID]
			if !ok || !command.LegacyUnresolved || run.TargetID.Valid || run.GroupID.Valid || run.LegacySessionID != session.ID || run.LegacyCommandID != run.CommandID {
				return CollaborationExecutionView{}, invalid
			}
			out.LegacyUnresolved.RunIDs = append(out.LegacyUnresolved.RunIDs, util.UUIDToString(run.ID))
			continue
		}
		group, ok := groupByID[run.GroupID]
		if !ok || group.CommandID != run.CommandID || group.TargetID != run.TargetID || !run.AgentID.Valid || !run.StepKind.Valid || !run.StepKey.Valid ||
			!run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion {
			return CollaborationExecutionView{}, invalid
		}
		out.Runs = append(out.Runs, CollaborationExecutionRunView{
			ID: util.UUIDToString(run.ID), CommandID: util.UUIDToString(run.CommandID), TargetID: util.UUIDToString(run.TargetID), GroupID: util.UUIDToString(run.GroupID),
			ParentRunID: util.UUIDToPtr(run.ParentRunID), StepKey: run.StepKey.String, StepKind: run.StepKind.String, AgentID: util.UUIDToString(run.AgentID),
			TargetEpoch: strconv.FormatInt(run.TargetEpoch.Int64, 10), ConfigVersion: strconv.FormatInt(run.ConfigVersion.Int64, 10),
			ProgressState: run.ProgressState, ProofState: run.ProofState, BusinessOutcome: util.TextToPtr(run.BusinessOutcome), ForcedDisposition: util.TextToPtr(run.ForcedDisposition),
			RecordingIncomplete: run.RecordingIncomplete, SettledAt: util.TimestampToPtr(run.SettledAt),
		})
	}
	for _, run := range runs {
		if run.ParentRunID.Valid {
			parent, ok := runByID[run.ParentRunID]
			if !ok || parent.LegacyUnresolved || parent.GroupID != run.GroupID || parent.ID == run.ID {
				return CollaborationExecutionView{}, invalid
			}
		}
	}
	if session.ActiveRunID.Valid {
		run, ok := runByID[session.ActiveRunID]
		if !ok || !run.LegacyUnresolved {
			return CollaborationExecutionView{}, invalid
		}
	}
	return out, nil
}
