package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationRegisteredTargetExecutionProjection(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	path := "/api/collaboration/issues/" + f.issue
	var out struct {
		Execution    service.CollaborationExecutionView `json:"execution"`
		Capabilities struct {
			Read          bool `json:"read"`
			ControlAI     bool `json:"control_ai"`
			SubmitCommand bool `json:"submit_command"`
		} `json:"capabilities"`
		WriteContext *struct{} `json:"write_context"`
	}
	f.managed.request(t, f.ownerToken, false, http.MethodGet, path, nil, nil).Want(http.StatusOK).JSON(&out)
	if out.Execution.ProtocolVersion != 5 || len(out.Execution.Targets) != 1 || len(out.Execution.Channels) != 1 || !out.Capabilities.Read ||
		!out.Capabilities.ControlAI || out.WriteContext == nil || out.Capabilities.SubmitCommand || out.Execution.Availability != "unavailable" {
		t.Fatalf("registered target lost its authorized, gated projection: %+v", out)
	}
	target, channel := out.Execution.Targets[0], out.Execution.Channels[0]
	if target.ID != f.target || target.ChannelID == nil || *target.ChannelID != channel.ID || target.Control.QueueState != "paused" ||
		channel.QueueState != target.Control.QueueState || channel.ControlVersion != target.Control.ControlVersion || channel.PauseReason != target.PauseReason ||
		target.HoldReason == nil || *target.HoldReason != "initial" || target.HoldVersion != "2" || target.ActiveGroupID != nil || channel.ActiveGroupID != nil || channel.NextCommandSeq != "1" {
		t.Fatalf("real registration/consent bypassed channel binding or cleared the initial hold: target=%+v channel=%+v", target, channel)
	}
	input := f.comment(t, true, "Retained waiting input")
	f.managed.request(t, f.ownerToken, false, http.MethodGet, path, nil, nil).Want(http.StatusOK).JSON(&out)
	if len(out.Execution.Commands) != 1 || out.Execution.Commands[0].State != "waiting" ||
		out.Execution.Commands[0].ID != util.UUIDToString(input.Targets[0].CommandID) || out.Execution.Commands[0].ChannelID != channel.ID ||
		len(out.Execution.Groups) != 0 || len(out.Execution.Runs) != 0 || out.Execution.Targets[0].HoldVersion != target.HoldVersion ||
		out.Execution.Commands[0].GroupID != nil || out.Execution.Channels[0].NextCommandSeq != "2" {
		t.Fatalf("registered target input became unreadable or changed execution intent: %+v", out.Execution)
	}
	reader := f.managed.user(t, "")
	f.managed.fx.Member(t, f.managed.fx.WorkspaceID, reader, "member")
	out.WriteContext = nil
	f.managed.request(t, platformRouterJWT(t, reader, nil), false, http.MethodGet, path, nil, nil).Want(http.StatusOK).JSON(&out)
	if !out.Capabilities.Read || out.Capabilities.ControlAI || out.Capabilities.SubmitCommand || out.WriteContext != nil {
		t.Fatalf("source-only reader acquired participant controls: %+v", out)
	}
	// A fixture fix must not weaken the real read boundary for corrupt mirrors.
	f.managed.fx.Exec(t, `UPDATE collaboration_target SET pause_reason='inconsistent_fixture' WHERE id=$1`, f.target)
	refused := f.managed.request(t, f.ownerToken, false, http.MethodGet, path, nil, nil).Want(http.StatusServiceUnavailable).Map()
	if refused["code"] != "collaboration_unavailable" || refused["execution"] != nil {
		t.Fatalf("inconsistent control mirrors were presented as an executable or empty projection: %v", refused)
	}
}

func TestCollaborationUpgradedTargetExecutionProjection(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	input := f.comment(t, true, "Input retained across the hold upgrade")
	// Replay the pre-hold Reconfigure writer against an actually registered
	// target. Its control change was local; its channel and waiting input stayed.
	if err := db.New(f.managed.pool).RevokeCollaborationTargetConsents(t.Context(), db.RevokeCollaborationTargetConsentsParams{
		TargetID: util.MustParseUUID(f.target), Reason: "configuration_changed",
	}); err != nil {
		t.Fatal(err)
	}
	var schema string
	f.managed.fx.QueryRow(t, "SELECT current_schema()").Scan(&schema)
	if !strings.HasPrefix(schema, "admin_router_") {
		t.Fatal("upgrade rehearsal requires the private router fixture schema")
	}
	f.managed.fx.Exec(t, "ALTER TABLE "+pgx.Identifier{schema, "collaboration_target"}.Sanitize()+" DROP COLUMN hold_reason, DROP COLUMN hold_version")
	f.managed.fx.Exec(t, `UPDATE collaboration_target SET config_version=config_version+1,target_epoch=target_epoch+1,
		control_version=control_version+1,queue_state='paused',pause_reason='configuration_changed',revoked_at=NULL WHERE id=$1`, f.target)
	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "698_collaboration_target_holds.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.managed.pool.Exec(t.Context(), string(up)); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Execution service.CollaborationExecutionView `json:"execution"`
	}
	f.managed.request(t, f.ownerToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue, nil, nil).Want(http.StatusOK).JSON(&out)
	if len(out.Execution.Targets) != 1 || len(out.Execution.Channels) != 1 || len(out.Execution.Commands) != 1 {
		t.Fatalf("upgrade lost the source's target/channel/waiting history: %+v", out.Execution)
	}
	target, channel, command := out.Execution.Targets[0], out.Execution.Channels[0], out.Execution.Commands[0]
	if target.HoldReason == nil || *target.HoldReason != "switch_hold" || target.HoldVersion != "1" ||
		target.ConfigVersion != "2" || target.TargetEpoch != "2" || target.Control.QueueState != "paused" || channel.QueueState != "paused" ||
		target.Control.ControlVersion != channel.ControlVersion || target.PauseReason != channel.PauseReason ||
		command.ID != util.UUIDToString(input.Targets[0].CommandID) || command.State != "waiting" || command.ChannelID != channel.ID ||
		len(out.Execution.Groups) != 0 || len(out.Execution.Runs) != 0 {
		t.Fatalf("upgrade changed execution intent or retained input: %+v", out.Execution)
	}
}
