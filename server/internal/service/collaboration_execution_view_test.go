package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationExecutionViewRejectsUnboundCurrentIdentity(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	gate := db.CollaborationExecutionGate{ID: 1, Epoch: 1}
	view, err := projectCollaborationExecution(session, gate, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "legacy") || view.Channels == nil || view.Targets == nil || view.Commands == nil || view.Groups == nil || view.Runs == nil {
		t.Fatalf("current projection must contain explicit collections without obsolete mappings: %s", encoded)
	}
	command := db.CollaborationCommand{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, Seq: 1}
	if _, err := projectCollaborationExecution(session, gate, nil, nil, []db.CollaborationCommand{command}, nil, nil); err == nil {
		t.Fatal("unbound current command must not become an idle projection")
	}
	run := db.CollaborationRun{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, CommandID: command.ID, ProgressState: "ending", ProofState: "unknown"}
	if _, err := projectCollaborationExecution(session, gate, nil, nil, nil, nil, []db.CollaborationRun{run}); err == nil {
		t.Fatal("unbound current run must not disappear from proof accounting")
	}
}

func TestCollaborationExecutionViewPreservesBatchAndChannelIdentity(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	gate := db.CollaborationExecutionGate{ID: 1, Epoch: 1}
	channelID, batchID, groupID := util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	channel := db.CollaborationChannel{ID: channelID, SessionID: session.ID, QueueState: "paused", ControlVersion: 1, GroupGeneration: 1, ActiveGroupID: groupID}
	target := db.CollaborationTarget{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, ChannelID: channelID,
		Kind: "agent", ResourceID: util.MustParseUUID(uuid.NewString()), TargetEpoch: 1, ConfigVersion: 1, ControlVersion: 1,
		QueueState: "paused", GroupGeneration: 1, NextCommandSeq: 3, ActiveGroupID: groupID, HoldVersion: 1}
	first := db.CollaborationCommand{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, TargetID: target.ID,
		ChannelID: channelID, BatchID: batchID, InputEventID: util.MustParseUUID(uuid.NewString()), AuthorUserID: util.MustParseUUID(uuid.NewString()), Seq: 1, State: "in_flight"}
	second := first
	second.ID, second.InputEventID, second.Seq = util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), 2
	group := db.CollaborationExecutionGroup{ID: groupID, SessionID: session.ID, TargetID: target.ID, CommandID: first.ID,
		ChannelID: channelID, BatchID: batchID, Generation: 1, TargetEpoch: 1, ConfigVersion: 1, State: "active"}
	run := db.CollaborationRun{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, TargetID: target.ID,
		CommandID: first.ID, GroupID: groupID, ChannelID: channelID, BatchID: batchID, AgentID: target.ResourceID,
		StepKind: pgtype.Text{String: "agent", Valid: true}, StepKey: pgtype.Text{String: "root", Valid: true},
		TargetEpoch: pgtype.Int8{Int64: 1, Valid: true}, ConfigVersion: pgtype.Int8{Int64: 1, Valid: true},
		ProgressState: "running", ProofState: "unknown"}
	project := func(commands []db.CollaborationCommand, targets []db.CollaborationTarget) (CollaborationExecutionView, error) {
		return projectCollaborationExecution(session, gate, []db.CollaborationChannel{channel}, targets, commands, []db.CollaborationExecutionGroup{group}, []db.CollaborationRun{run})
	}
	view, err := project([]db.CollaborationCommand{first, second}, []db.CollaborationTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ProtocolVersion int `json:"protocol_version"`
		Targets         []struct {
			ChannelID string `json:"channel_id"`
		} `json:"targets"`
		Commands []struct {
			ChannelID string `json:"channel_id"`
			BatchID   string `json:"batch_id"`
		} `json:"commands"`
		Groups []struct {
			ChannelID string `json:"channel_id"`
			BatchID   string `json:"batch_id"`
		} `json:"groups"`
		Runs []struct {
			ChannelID string `json:"channel_id"`
			BatchID   string `json:"batch_id"`
		} `json:"runs"`
	}
	if err = json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.ProtocolVersion != 3 || len(wire.Commands) != 2 || len(wire.Groups) != 1 ||
		wire.Targets[0].ChannelID != util.UUIDToString(channelID) || wire.Commands[1].ChannelID != util.UUIDToString(channelID) ||
		wire.Commands[1].BatchID != wire.Groups[0].BatchID || wire.Groups[0].BatchID != util.UUIDToString(batchID) ||
		len(wire.Runs) != 1 || wire.Runs[0].ChannelID != util.UUIDToString(channelID) || wire.Runs[0].BatchID != util.UUIDToString(batchID) {
		t.Fatalf("read wire lost the shared batch/channel identity: %s", encoded)
	}
	wrong := second
	wrong.BatchID = util.MustParseUUID(uuid.NewString())
	if _, err = project([]db.CollaborationCommand{first, wrong}, []db.CollaborationTarget{target}); err == nil {
		t.Fatal("in-flight input without its own batch group must not be accepted")
	}
	alias := target
	alias.ID, alias.Kind, alias.ResourceID = util.MustParseUUID(uuid.NewString()), "squad", util.MustParseUUID(uuid.NewString())
	if _, err = project([]db.CollaborationCommand{first, second}, []db.CollaborationTarget{target, alias}); err != nil {
		t.Fatalf("same-channel alias lost the current group: %v", err)
	}
	alias.ChannelID = util.MustParseUUID(uuid.NewString())
	if _, err = project([]db.CollaborationCommand{first, second}, []db.CollaborationTarget{target, alias}); err == nil {
		t.Fatal("a different channel must not claim another channel's active group")
	}
	cancelled := second
	cancelled.State = "cancelled"
	if _, err := project([]db.CollaborationCommand{first, cancelled}, []db.CollaborationTarget{target}); err != nil {
		t.Fatalf("cancelled waiting input must retain its batch without blocking the other input: %v", err)
	}
	for _, field := range []string{"channel", "batch", "proof", "parent"} {
		t.Run("invalid_run_"+field, func(t *testing.T) {
			wrong := run
			switch field {
			case "channel":
				wrong.ChannelID = util.MustParseUUID(uuid.NewString())
			case "batch":
				wrong.BatchID = util.MustParseUUID(uuid.NewString())
			case "proof":
				wrong.ProgressState = "settled"
				wrong.SettledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			case "parent":
				wrong.ParentRunID = util.MustParseUUID(uuid.NewString())
			}
			if _, err := projectCollaborationExecution(session, gate, []db.CollaborationChannel{channel}, []db.CollaborationTarget{target}, []db.CollaborationCommand{first, second}, []db.CollaborationExecutionGroup{group}, []db.CollaborationRun{wrong}); err == nil {
				t.Fatalf("run with invalid %s became readable", field)
			}
		})
	}
	t.Run("settled_batch", func(t *testing.T) {
		finishedTarget, finishedGroup, finishedRun, finishedInput := target, group, run, first
		finishedChannel := channel
		finishedChannel.ActiveGroupID = pgtype.UUID{}
		finishedTarget.ActiveGroupID = pgtype.UUID{}
		finishedGroup.State, finishedInput.State = "succeeded", "succeeded"
		finishedGroup.SettledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		finishedGroup.ExpansionSealedAt = finishedGroup.SettledAt
		finishedRun.ProgressState, finishedRun.ProofState = "settled", "exited_proven"
		finishedRun.SettledAt = finishedGroup.SettledAt
		if _, err := projectCollaborationExecution(session, gate, []db.CollaborationChannel{finishedChannel}, []db.CollaborationTarget{finishedTarget}, []db.CollaborationCommand{finishedInput, cancelled}, []db.CollaborationExecutionGroup{finishedGroup}, []db.CollaborationRun{finishedRun}); err != nil {
			t.Fatalf("settled batch with a cancelled waiting input was rejected: %v", err)
		}
		if _, err := projectCollaborationExecution(session, gate, []db.CollaborationChannel{finishedChannel}, []db.CollaborationTarget{finishedTarget}, []db.CollaborationCommand{finishedInput, cancelled}, []db.CollaborationExecutionGroup{finishedGroup}, nil); err == nil {
			t.Fatal("settled group lost its root run and terminal proof")
		}
	})
}

func TestCollaborationExecutionViewAllowsNullChannelOnlyBeforeInput(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	gate := db.CollaborationExecutionGate{ID: 1, Epoch: 1}
	target := db.CollaborationTarget{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID,
		Kind: "agent", ResourceID: util.MustParseUUID(uuid.NewString()), TargetEpoch: 1, ConfigVersion: 1,
		ControlVersion: 1, QueueState: "paused", NextCommandSeq: 1, HoldVersion: 1}
	view, err := projectCollaborationExecution(session, gate, nil, []db.CollaborationTarget{target}, nil, nil, nil)
	if err != nil || len(view.Targets) != 1 || view.Targets[0].ChannelID != nil {
		t.Fatalf("unbound target must expose an explicit null channel: %v", err)
	}
	encoded, err := json.Marshal(view)
	if err != nil || !strings.Contains(string(encoded), `"channel_id":null`) {
		t.Fatal("unbound channel was omitted from the v3 wire")
	}
	target.NextCommandSeq = 2
	if _, err := projectCollaborationExecution(session, gate, nil, []db.CollaborationTarget{target}, nil, nil, nil); err == nil {
		t.Fatal("target with input history silently lost its channel")
	}
}

func TestCollaborationExecutionViewKeepsHoldsIndependentAndRequiresChannelFacts(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "active", ControlVersion: 1, TargetSetVersion: 2}
	gate := db.CollaborationExecutionGate{ID: 1, Epoch: 9223372036854775807, Enabled: false}
	channel := db.CollaborationChannel{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, QueueState: "active", ControlVersion: 9007199254740993}
	target := db.CollaborationTarget{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, ChannelID: channel.ID,
		Kind: "agent", ResourceID: util.MustParseUUID(uuid.NewString()), TargetEpoch: 1, ConfigVersion: 1, QueueState: "active",
		ControlVersion: channel.ControlVersion, NextCommandSeq: 1, HoldVersion: 9007199254740993}
	held := target
	held.ID, held.Kind, held.ResourceID = util.MustParseUUID(uuid.NewString()), "squad", util.MustParseUUID(uuid.NewString())
	held.HoldReason, held.HoldVersion = pgtype.Text{String: "grant_required", Valid: true}, 9223372036854775807
	retained := channel
	retained.ID, retained.QueueState, retained.PauseReason = util.MustParseUUID(uuid.NewString()), "paused", "participant_stop"
	view, err := projectCollaborationExecution(session, gate, []db.CollaborationChannel{channel, retained}, []db.CollaborationTarget{target, held}, nil, nil, nil)
	if err != nil || len(view.Channels) != 2 || len(view.Targets) != 2 || view.Targets[0].HoldReason != nil || view.Targets[1].HoldReason == nil ||
		*view.Targets[1].HoldReason != "grant_required" || view.Targets[1].HoldVersion != "9223372036854775807" || view.ExecutionGate.Epoch != "9223372036854775807" || view.ExecutionGate.Enabled {
		t.Fatalf("hold, retained channel or exact gate facts were lost: %+v, %v", view, err)
	}
	for _, field := range []string{"missing_channel", "duplicate_channel", "queue_state", "control_version", "group_generation", "active_group", "pause_reason", "hold_reason", "hold_version", "gate_epoch", "retained_group"} {
		t.Run(field, func(t *testing.T) {
			channels, targets, changedGate := []db.CollaborationChannel{channel, retained}, []db.CollaborationTarget{target, held}, gate
			switch field {
			case "missing_channel":
				channels = nil
			case "duplicate_channel":
				channels = append(channels, channel)
			case "queue_state":
				channels[0].QueueState = "paused"
			case "control_version":
				channels[0].ControlVersion++
			case "group_generation":
				channels[0].GroupGeneration++
			case "active_group":
				channels[0].ActiveGroupID = util.MustParseUUID(uuid.NewString())
			case "pause_reason":
				channels[0].PauseReason = "other"
			case "hold_reason":
				targets[0].HoldReason = pgtype.Text{String: "future_hold", Valid: true}
			case "hold_version":
				targets[0].HoldVersion = 0
			case "gate_epoch":
				changedGate.Epoch = 0
			case "retained_group":
				channels[1].ActiveGroupID = util.MustParseUUID(uuid.NewString())
			}
			if _, err := projectCollaborationExecution(session, changedGate, channels, targets, nil, nil, nil); err == nil {
				t.Fatalf("invalid %s was accepted", field)
			}
		})
	}
}
