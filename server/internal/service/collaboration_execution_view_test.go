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
	view, err := projectCollaborationExecution(session, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "legacy") || view.Targets == nil || view.Commands == nil || view.Groups == nil || view.Runs == nil {
		t.Fatalf("current projection must contain explicit collections without obsolete mappings: %s", encoded)
	}
	command := db.CollaborationCommand{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, Seq: 1}
	if _, err := projectCollaborationExecution(session, nil, []db.CollaborationCommand{command}, nil, nil); err == nil {
		t.Fatal("unbound current command must not become an idle projection")
	}
	run := db.CollaborationRun{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, CommandID: command.ID, ProgressState: "ending", ProofState: "unknown"}
	if _, err := projectCollaborationExecution(session, nil, nil, nil, []db.CollaborationRun{run}); err == nil {
		t.Fatal("unbound current run must not disappear from proof accounting")
	}
}

func TestCollaborationExecutionViewPreservesBatchAndChannelIdentity(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	channelID, batchID, groupID := util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	target := db.CollaborationTarget{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, ChannelID: channelID,
		Kind: "agent", ResourceID: util.MustParseUUID(uuid.NewString()), TargetEpoch: 1, ConfigVersion: 1, ControlVersion: 1,
		QueueState: "paused", GroupGeneration: 1, NextCommandSeq: 3, ActiveGroupID: groupID}
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
		return projectCollaborationExecution(session, targets, commands, []db.CollaborationExecutionGroup{group}, []db.CollaborationRun{run})
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
			if _, err := projectCollaborationExecution(session, []db.CollaborationTarget{target}, []db.CollaborationCommand{first, second}, []db.CollaborationExecutionGroup{group}, []db.CollaborationRun{wrong}); err == nil {
				t.Fatalf("run with invalid %s became readable", field)
			}
		})
	}
	t.Run("settled_batch", func(t *testing.T) {
		finishedTarget, finishedGroup, finishedRun, finishedInput := target, group, run, first
		finishedTarget.ActiveGroupID = pgtype.UUID{}
		finishedGroup.State, finishedInput.State = "succeeded", "succeeded"
		finishedGroup.SettledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		finishedGroup.ExpansionSealedAt = finishedGroup.SettledAt
		finishedRun.ProgressState, finishedRun.ProofState = "settled", "exited_proven"
		finishedRun.SettledAt = finishedGroup.SettledAt
		if _, err := projectCollaborationExecution(session, []db.CollaborationTarget{finishedTarget}, []db.CollaborationCommand{finishedInput, cancelled}, []db.CollaborationExecutionGroup{finishedGroup}, []db.CollaborationRun{finishedRun}); err != nil {
			t.Fatalf("settled batch with a cancelled waiting input was rejected: %v", err)
		}
		if _, err := projectCollaborationExecution(session, []db.CollaborationTarget{finishedTarget}, []db.CollaborationCommand{finishedInput, cancelled}, []db.CollaborationExecutionGroup{finishedGroup}, nil); err == nil {
			t.Fatal("settled group lost its root run and terminal proof")
		}
	})
}

func TestCollaborationExecutionViewAllowsNullChannelOnlyBeforeInput(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()), QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1}
	target := db.CollaborationTarget{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID,
		Kind: "agent", ResourceID: util.MustParseUUID(uuid.NewString()), TargetEpoch: 1, ConfigVersion: 1,
		ControlVersion: 1, QueueState: "paused", NextCommandSeq: 1}
	view, err := projectCollaborationExecution(session, []db.CollaborationTarget{target}, nil, nil, nil)
	if err != nil || len(view.Targets) != 1 || view.Targets[0].ChannelID != nil {
		t.Fatalf("unbound target must expose an explicit null channel: %v", err)
	}
	encoded, err := json.Marshal(view)
	if err != nil || !strings.Contains(string(encoded), `"channel_id":null`) {
		t.Fatal("unbound channel was omitted from the v3 wire")
	}
	target.NextCommandSeq = 2
	if _, err := projectCollaborationExecution(session, []db.CollaborationTarget{target}, nil, nil, nil); err == nil {
		t.Fatal("target with input history silently lost its channel")
	}
}
