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

type executionProjectionFixture struct {
	session  db.CollaborationSession
	gate     db.CollaborationExecutionGate
	channels []db.CollaborationChannel
	targets  []db.CollaborationTarget
	commands []db.CollaborationCommand
	groups   []db.CollaborationExecutionGroup
	runs     []db.CollaborationRun
	inputs   []db.CollaborationGroupInput
}

func newExecutionProjectionFixture() executionProjectionFixture {
	id := func() pgtype.UUID { return util.MustParseUUID(uuid.NewString()) }
	session := db.CollaborationSession{ID: id(), IssueID: id(), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	channel := db.CollaborationChannel{ID: id(), SessionID: session.ID, RootAgentID: id(), QueueState: "paused", ControlVersion: 1, GroupGeneration: 1, NextCommandSeq: 2}
	target := db.CollaborationTarget{ID: id(), SessionID: session.ID, ChannelID: channel.ID, Kind: "agent", ResourceID: channel.RootAgentID,
		TargetEpoch: 1, ConfigVersion: 1, ControlVersion: 1, QueueState: "paused", GroupGeneration: 1, HoldVersion: 1}
	command := db.CollaborationCommand{ID: id(), SessionID: session.ID, TargetID: target.ID, ChannelID: channel.ID, BatchID: id(),
		InputEventID: id(), AuthorUserID: id(), Seq: 1, State: "in_flight", Content: "PRIVATE_INPUT_BODY"}
	group := db.CollaborationExecutionGroup{ID: id(), SessionID: session.ID, TargetID: target.ID, CommandID: command.ID,
		ChannelID: channel.ID, BatchID: command.BatchID, Generation: 1, TargetEpoch: 1, ConfigVersion: 1, State: "active"}
	command.GroupID, channel.ActiveGroupID, target.ActiveGroupID = group.ID, group.ID, group.ID
	run := db.CollaborationRun{ID: id(), SessionID: session.ID, TargetID: target.ID, CommandID: command.ID, GroupID: group.ID,
		ChannelID: channel.ID, BatchID: command.BatchID, AgentID: target.ResourceID,
		StepKind: pgtype.Text{String: "agent", Valid: true}, StepKey: pgtype.Text{String: "root", Valid: true},
		TargetEpoch: pgtype.Int8{Int64: 1, Valid: true}, ConfigVersion: pgtype.Int8{Int64: 1, Valid: true}, ProgressState: "running", ProofState: "unknown"}
	input := db.CollaborationGroupInput{GroupID: group.ID, CommandID: command.ID, ChannelID: channel.ID, BatchID: command.BatchID, Seq: 1, ConsumingRunID: run.ID}
	return executionProjectionFixture{session: session, gate: db.CollaborationExecutionGate{ID: 1, Epoch: 1},
		channels: []db.CollaborationChannel{channel}, targets: []db.CollaborationTarget{target}, commands: []db.CollaborationCommand{command},
		groups: []db.CollaborationExecutionGroup{group}, runs: []db.CollaborationRun{run}, inputs: []db.CollaborationGroupInput{input}}
}

func (f executionProjectionFixture) project() (CollaborationExecutionView, error) {
	return projectCollaborationExecution(f.session, f.gate, f.channels, f.targets, f.commands, f.groups, f.runs, f.inputs)
}

func (f *executionProjectionFixture) settle(proof string) {
	ended := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	f.channels[0].ActiveGroupID, f.targets[0].ActiveGroupID = pgtype.UUID{}, pgtype.UUID{}
	f.commands[0].State, f.groups[0].State = "stopped", "stopped"
	f.groups[0].SettledAt, f.groups[0].ExpansionSealedAt = ended, ended
	f.groups[0].ForcedDisposition = pgtype.Text{String: "stopped", Valid: true}
	f.runs[0].ProgressState, f.runs[0].ProofState, f.runs[0].SettledAt = "settled", proof, ended
	if proof == "exited_proven" {
		f.runs[0].AuthorizedAttemptSeq, f.runs[0].StartedAt = 1, ended
	}
	f.runs[0].SealedAttemptSeq = pgtype.Int8{Int64: f.runs[0].AuthorizedAttemptSeq, Valid: true}
}

func TestCollaborationExecutionViewHistoricalInputWire(t *testing.T) {
	f := newExecutionProjectionFixture()
	view, err := f.project()
	if err != nil {
		t.Fatal(err)
	}
	if view.ProtocolVersion != 4 || view.Channels[0].NextCommandSeq != "2" || view.Commands[0].GroupID == nil ||
		*view.Commands[0].GroupID != util.UUIDToString(f.groups[0].ID) || len(view.Groups[0].Inputs) != 1 {
		t.Fatalf("v4 current and historical membership missing: %+v", view)
	}
	input := view.Groups[0].Inputs[0]
	if input.CommandID != view.Commands[0].ID || input.ChannelID != view.Groups[0].ChannelID || input.BatchID != view.Groups[0].BatchID ||
		input.Sequence != "1" || input.ConsumingRunID != view.Runs[0].ID {
		t.Fatalf("frozen consumption identity changed: %+v", input)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "PRIVATE_") || strings.Count(string(encoded), `"next_command_seq"`) != 1 {
		t.Fatal("read projection retained private input or a target-owned FIFO counter")
	}
}

func TestCollaborationExecutionViewRejectsIncompleteConsumption(t *testing.T) {
	for name, mutate := range map[string]func(*executionProjectionFixture){
		"missing":               func(f *executionProjectionFixture) { f.inputs = nil },
		"duplicate":             func(f *executionProjectionFixture) { f.inputs = append(f.inputs, f.inputs[0]) },
		"different_group":       func(f *executionProjectionFixture) { f.inputs[0].GroupID = util.MustParseUUID(uuid.NewString()) },
		"different_command":     func(f *executionProjectionFixture) { f.inputs[0].CommandID = util.MustParseUUID(uuid.NewString()) },
		"different_channel":     func(f *executionProjectionFixture) { f.inputs[0].ChannelID = util.MustParseUUID(uuid.NewString()) },
		"different_batch":       func(f *executionProjectionFixture) { f.inputs[0].BatchID = util.MustParseUUID(uuid.NewString()) },
		"different_sequence":    func(f *executionProjectionFixture) { f.channels[0].NextCommandSeq = 3; f.inputs[0].Seq = 2 },
		"missing_consumer":      func(f *executionProjectionFixture) { f.inputs[0].ConsumingRunID = pgtype.UUID{} },
		"unknown_consumer":      func(f *executionProjectionFixture) { f.inputs[0].ConsumingRunID = util.MustParseUUID(uuid.NewString()) },
		"missing_current_group": func(f *executionProjectionFixture) { f.commands[0].GroupID = pgtype.UUID{} },
		"unknown_current_group": func(f *executionProjectionFixture) { f.commands[0].GroupID = util.MustParseUUID(uuid.NewString()) },
		"unknown_progress":      func(f *executionProjectionFixture) { f.runs[0].ProgressState = "future_progress" },
		"unknown_proof":         func(f *executionProjectionFixture) { f.runs[0].ProofState = "future_proof" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newExecutionProjectionFixture()
			mutate(&f)
			if _, err := f.project(); err == nil {
				t.Fatalf("invalid %s became a readable execution", name)
			}
		})
	}
}

func switchedExecutionProjectionFixture() executionProjectionFixture {
	f := newExecutionProjectionFixture()
	f.settle("exited_proven")
	f.targets[0].Kind, f.targets[0].ResourceID = "squad", util.MustParseUUID(uuid.NewString())
	f.runs[0].StepKind = pgtype.Text{String: "leader", Valid: true}
	f.channels[0].GroupGeneration, f.groups[0].Generation = 10, 9
	destination := db.CollaborationChannel{ID: util.MustParseUUID(uuid.NewString()), SessionID: f.session.ID, QueueState: "active", ControlVersion: 2, NextCommandSeq: 2}
	f.channels = append(f.channels, destination)
	f.targets[0].ChannelID, f.targets[0].QueueState, f.targets[0].ControlVersion = destination.ID, destination.QueueState, destination.ControlVersion
	f.targets[0].GroupGeneration, f.targets[0].ConfigVersion = 0, 2
	f.targets[0].HoldReason = pgtype.Text{String: "switch_hold", Valid: true}
	waiting := f.commands[0]
	waiting.ID, waiting.InputEventID, waiting.BatchID = util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	waiting.GroupID, waiting.ChannelID, waiting.State = pgtype.UUID{}, destination.ID, "waiting"
	f.commands = append(f.commands, waiting)
	return f
}

func TestCollaborationExecutionViewSwitchKeepsOriginalHistory(t *testing.T) {
	f := switchedExecutionProjectionFixture()
	view, err := f.project()
	if err != nil {
		t.Fatal(err)
	}
	if view.Targets[0].ChannelID == nil || *view.Targets[0].ChannelID != view.Channels[1].ID || view.Groups[0].ChannelID != view.Channels[0].ID ||
		view.Groups[0].Generation != "9" || view.Commands[1].ChannelID != view.Channels[1].ID || len(view.Groups[0].Inputs) != 1 {
		t.Fatalf("Switch rewrote old consumption or stranded waiting input: %+v", view)
	}
	for name, mutate := range map[string]func(*executionProjectionFixture){
		"missing_original_channel":   func(f *executionProjectionFixture) { f.channels = f.channels[1:] },
		"future_original_generation": func(f *executionProjectionFixture) { f.groups[0].Generation = 11 },
		"unfinished_original_run": func(f *executionProjectionFixture) {
			f.runs[0].ProgressState = "ending"
			f.runs[0].SettledAt = pgtype.Timestamptz{}
		},
		"moved_terminal_command": func(f *executionProjectionFixture) { f.commands[0].ChannelID = f.channels[1].ID },
		"stranded_waiting": func(f *executionProjectionFixture) {
			f.channels[0].NextCommandSeq = 3
			f.commands[1].ChannelID = f.channels[0].ID
			f.commands[1].Seq = 2
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := switchedExecutionProjectionFixture()
			mutate(&f)
			if _, err := f.project(); err == nil {
				t.Fatalf("invalid switched history %s became readable", name)
			}
		})
	}
}

func TestCollaborationExecutionViewNeverLaunchedInputReturns(t *testing.T) {
	for _, state := range []string{"waiting", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			f := newExecutionProjectionFixture()
			f.settle("never_launched_proven")
			f.commands[0].State = state
			if state == "waiting" {
				f.commands[0].GroupID, f.commands[0].BatchID = pgtype.UUID{}, util.MustParseUUID(uuid.NewString())
			}
			view, err := f.project()
			if err != nil || len(view.Groups) != 1 || len(view.Groups[0].Inputs) != 1 || view.Commands[0].State != state ||
				view.Groups[0].Inputs[0].BatchID != util.UUIDToString(f.inputs[0].BatchID) {
				t.Fatalf("safe %s lost its original consumption: %+v, %v", state, view, err)
			}
			f.runs[0].ProofState = "exited_proven"
			if _, err := f.project(); err == nil {
				t.Fatal("a real launch was treated as an unconsumed input")
			}
		})
	}
}

func TestCollaborationExecutionViewCancelledHistoryAfterSwitch(t *testing.T) {
	f := switchedExecutionProjectionFixture()
	cancelled := f.commands[0]
	cancelled.ID, cancelled.InputEventID = util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	cancelled.GroupID, cancelled.State, cancelled.Seq = pgtype.UUID{}, "cancelled", 2
	f.commands = append(f.commands, cancelled)
	f.channels[0].NextCommandSeq = 3
	view, err := f.project()
	if err != nil || len(view.Commands) != 3 || view.Commands[2].GroupID != nil || view.Commands[2].ChannelID != view.Channels[0].ID ||
		view.Commands[2].State != "cancelled" || len(view.Groups[0].Inputs) != 1 {
		t.Fatalf("unconsumed cancellation lost its original channel after Switch: %+v, %v", view, err)
	}
	f.commands[2].State = "waiting"
	if _, err := f.project(); err == nil {
		t.Fatal("waiting input stayed on a switched target's old channel")
	}
	f.commands[2].State, f.commands[2].GroupID = "cancelled", f.groups[0].ID
	if _, err := f.project(); err == nil {
		t.Fatal("unconsumed tombstone acquired a forged consumed group")
	}
}

func TestCollaborationExecutionViewUsesChannelFIFOAcrossAliases(t *testing.T) {
	f := newExecutionProjectionFixture()
	alias := f.targets[0]
	alias.ID, alias.Kind, alias.ResourceID = util.MustParseUUID(uuid.NewString()), "squad", util.MustParseUUID(uuid.NewString())
	alias.HoldReason = pgtype.Text{String: "grant_required", Valid: true}
	f.targets = append(f.targets, alias)
	waiting := f.commands[0]
	waiting.ID, waiting.InputEventID, waiting.BatchID = util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	waiting.TargetID, waiting.GroupID, waiting.State, waiting.Seq = alias.ID, pgtype.UUID{}, "waiting", 2
	f.commands = append(f.commands, waiting)
	f.channels[0].NextCommandSeq = 3
	if _, err := f.project(); err != nil {
		t.Fatal(err)
	}
	f.commands[1].Seq = 1
	if _, err := f.project(); err == nil {
		t.Fatal("two targets reused the same channel sequence")
	}
	f.commands[1].Seq, f.commands[1].TargetID, f.commands[1].InputEventID = 2, f.commands[0].TargetID, f.commands[0].InputEventID
	if _, err := f.project(); err == nil {
		t.Fatal("one input event was replayed for the same typed target")
	}
}

func TestCollaborationExecutionViewMixedBatchKeepsEffectiveSquad(t *testing.T) {
	f := newExecutionProjectionFixture()
	direct := f.targets[0]
	direct.ID = util.MustParseUUID(uuid.NewString())
	f.targets[0].Kind, f.targets[0].ResourceID = "squad", util.MustParseUUID(uuid.NewString())
	f.targets = append(f.targets, direct)
	f.runs[0].StepKind = pgtype.Text{String: "leader", Valid: true}
	f.commands[0].Seq, f.inputs[0].Seq, f.channels[0].NextCommandSeq = 2, 2, 3
	earlier := f.commands[0]
	earlier.ID, earlier.TargetID, earlier.Seq = util.MustParseUUID(uuid.NewString()), direct.ID, 1
	f.commands = append([]db.CollaborationCommand{earlier}, f.commands...)
	input := f.inputs[0]
	input.CommandID, input.Seq = earlier.ID, 1
	f.inputs = append([]db.CollaborationGroupInput{input}, f.inputs...)
	view, err := f.project()
	if err != nil || len(view.Groups[0].Inputs) != 2 || view.Groups[0].CommandID != view.Commands[1].ID || view.Groups[0].Inputs[0].CommandID != view.Commands[0].ID {
		t.Fatalf("earlier A input replaced the effective S representative: %+v, %v", view, err)
	}
	f.targets[1].ResourceID = util.MustParseUUID(uuid.NewString())
	if _, err := f.project(); err == nil {
		t.Fatal("unrelated direct agent was consumed by the historical squad leader")
	}
	f.targets[1].Kind = "squad"
	if _, err := f.project(); err == nil {
		t.Fatal("a second squad was consumed in the same batch")
	}
}

func TestCollaborationExecutionViewReturnRequiresEveryRunNeverLaunched(t *testing.T) {
	f := newExecutionProjectionFixture()
	f.targets[0].Kind, f.targets[0].ResourceID = "squad", util.MustParseUUID(uuid.NewString())
	f.runs[0].StepKind = pgtype.Text{String: "leader", Valid: true}
	f.settle("never_launched_proven")
	f.commands[0].State, f.commands[0].GroupID, f.commands[0].BatchID = "waiting", pgtype.UUID{}, util.MustParseUUID(uuid.NewString())
	member := f.runs[0]
	member.ID, member.ParentRunID = util.MustParseUUID(uuid.NewString()), f.runs[0].ID
	member.StepKind, member.StepKey = pgtype.Text{String: "member", Valid: true}, pgtype.Text{String: "member-1", Valid: true}
	member.ProofState = "exited_proven"
	f.runs = append(f.runs, member)
	if _, err := f.project(); err == nil {
		t.Fatal("one never-launched root proof hid a member's real execution")
	}
	f.runs[1].ProofState = "never_launched_proven"
	if _, err := f.project(); err != nil {
		t.Fatalf("complete nonlaunch proof was rejected: %v", err)
	}
	f.inputs[0].ConsumingRunID = member.ID
	if _, err := f.project(); err == nil {
		t.Fatal("member proof replaced the initial root's consumption identity")
	}
}

func TestCollaborationExecutionViewReturnedInputInLaterGroup(t *testing.T) {
	f := newExecutionProjectionFixture()
	f.settle("never_launched_proven")
	nextGroup := f.groups[0]
	nextGroup.ID, nextGroup.BatchID = util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	nextGroup.Generation, nextGroup.State, nextGroup.SettledAt, nextGroup.ExpansionSealedAt = 2, "active", pgtype.Timestamptz{}, pgtype.Timestamptz{}
	nextGroup.ForcedDisposition = pgtype.Text{}
	nextRun := f.runs[0]
	nextRun.ID, nextRun.GroupID, nextRun.BatchID = util.MustParseUUID(uuid.NewString()), nextGroup.ID, nextGroup.BatchID
	nextRun.ProgressState, nextRun.ProofState, nextRun.SettledAt = "prepared", "unknown", pgtype.Timestamptz{}
	nextRun.SealedAttemptSeq = pgtype.Int8{}
	nextInput := f.inputs[0]
	nextInput.GroupID, nextInput.BatchID, nextInput.ConsumingRunID = nextGroup.ID, nextGroup.BatchID, nextRun.ID
	f.groups, f.runs, f.inputs = append(f.groups, nextGroup), append(f.runs, nextRun), append(f.inputs, nextInput)
	f.commands[0].GroupID, f.commands[0].BatchID, f.commands[0].State = nextGroup.ID, nextGroup.BatchID, "in_flight"
	f.channels[0].GroupGeneration, f.targets[0].GroupGeneration = 2, 2
	f.channels[0].ActiveGroupID, f.targets[0].ActiveGroupID = nextGroup.ID, nextGroup.ID
	view, err := f.project()
	if err != nil || len(view.Groups) != 2 || len(view.Groups[0].Inputs) != 1 || len(view.Groups[1].Inputs) != 1 ||
		view.Groups[0].Inputs[0].CommandID != view.Groups[1].Inputs[0].CommandID || view.Commands[0].GroupID == nil || *view.Commands[0].GroupID != view.Groups[1].ID {
		t.Fatalf("second group lost the original never-launched membership: %+v, %v", view, err)
	}
	f.runs[0].ProofState = "exited_proven"
	if _, err := f.project(); err == nil {
		t.Fatal("a previously executed input acquired a new group")
	}
}
