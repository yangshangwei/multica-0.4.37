package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const collaborationSquadFakeArg = "__multica_collaboration_squad_fake"
const collaborationSquadCLIFakeArg = "__multica_collaboration_squad_cli_fake"

func init() {
	if len(os.Args) == 3 && (os.Args[1] == collaborationSquadFakeArg || os.Args[1] == collaborationSquadCLIFakeArg) {
		os.Exit(collaborationSquadFake(os.Args[2]))
	}
}

// This test-owned static executable uses only the run's business capability.
// The supervisor alone holds the termination credential and seals OS evidence.
func collaborationSquadFake(address string) int {
	var input collaborationrun.ProviderInput
	if json.NewDecoder(os.Stdin).Decode(&input) != nil {
		return 30
	}
	base := os.Getenv("MULTICA_COLLABORATION_SERVER") + "/api/collaboration/runtime/runs/" + input.Identity.RunID
	call := func(method, path string, value any, out any) error {
		body, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if os.Args[1] == collaborationSquadCLIFakeArg {
			if os.Getenv("MULTICA_TOKEN") != "" || os.Getenv("MULTICA_COLLABORATION_RUN_ID") != input.Identity.RunID || os.Getenv("MULTICA_COLLABORATION_ISSUE_ID") != input.Identity.IssueID {
				return fmt.Errorf("CLI did not receive only the admitted business identity")
			}
			var payload map[string]string
			if value != nil && json.Unmarshal(body, &payload) != nil {
				return fmt.Errorf("invalid fixture command payload")
			}
			args := []string{"issue", "get", input.Identity.IssueID, "--output", "json"}
			if path == "/comments" {
				args = []string{"issue", "comment", "add", input.Identity.IssueID, "--content", payload["content"], "--request-id", payload["comment_id"], "--output", "json"}
			} else if path == "/squad-activity" {
				args = []string{"squad", "activity", input.Identity.IssueID, payload["outcome"], "--reason", payload["reason"], "--request-id", payload["event_id"], "--output", "json"}
			}
			var stderr bytes.Buffer
			command := exec.Command("/multica", args...)
			command.Stderr = &stderr
			result, err := command.Output()
			if err != nil {
				return fmt.Errorf("test CLI command failed: %w: %s", err, stderr.String())
			}
			if out != nil {
				return json.Unmarshal(result, out)
			}
			return nil
		}
		r, err := http.NewRequest(method, base+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		r.Header.Set("Authorization", "Bearer "+os.Getenv("MULTICA_COLLABORATION_TOKEN"))
		r.Header.Set("Content-Type", "application/json")
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(r)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode >= 400 {
			return fmt.Errorf("business endpoint status %d", response.StatusCode)
		}
		if out != nil {
			return json.NewDecoder(response.Body).Decode(out)
		}
		return nil
	}
	var view service.CollaborationRuntimeContext
	if call(http.MethodGet, "/context", nil, &view) != nil {
		return 31
	}
	if view.StepKind == "leader" {
		member := ""
		for _, candidate := range view.Roster {
			if candidate.Role == "member" {
				member = util.UUIDToString(candidate.AgentID)
			}
		}
		if member == "" {
			return 32
		}
		body := map[string]string{"comment_id": uuid.NewString(), "content": "[@Member](mention://agent/" + member + ") Check the implementation"}
		var receipt, replay service.CollaborationRuntimeCommentReceipt
		if call(http.MethodPost, "/comments", body, &receipt) != nil || call(http.MethodPost, "/comments", body, &replay) != nil ||
			len(receipt.RunIDs) != 1 || len(replay.RunIDs) != 1 || receipt.RunIDs[0] != replay.RunIDs[0] {
			return 33
		}
	}
	if view.StepKind == "member" {
		if view.DispatchComment == "" || call(http.MethodPost, "/comments", map[string]string{"comment_id": uuid.NewString(), "content": "Member found an issue; leader must decide"}, nil) != nil {
			return 34
		}
	}
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return 35
	}
	defer connection.Close()
	if json.NewEncoder(connection).Encode(view) != nil {
		return 36
	}
	var release [1]byte
	if _, err = io.ReadFull(connection, release[:]); err != nil {
		return 37
	}
	if view.StepKind == "member" {
		if json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": "failure", "body": "Member evaluation requires a leader decision"}) != nil {
			return 38
		}
		return 0
	}
	outcome := "no_action"
	if view.StepKind == "leader" {
		outcome = "action"
	}
	if call(http.MethodPost, "/squad-activity", map[string]string{"event_id": uuid.NewString(), "outcome": outcome, "reason": "Recorded turn decision"}, nil) != nil {
		return 39
	}
	return 0
}

func registerCollaborationRouteSquad(t *testing.T, f *collaborationRouteFixture) (string, string) {
	t.Helper()
	member := f.managed.fx.Agent(t, "Independent squad member", f.runtime, testutil.Cols{"owner_id": f.owner, "kind": "user", "max_concurrent_tasks": 1})
	squad := f.managed.fx.Squad(t, "Native capacity-one squad", f.agent)
	f.managed.fx.SquadMember(t, squad, "agent", member)
	base := "/api/collaboration/issues/" + f.issue
	var registered service.CollaborationQueueReceipt
	f.managed.request(t, f.ownerToken, false, http.MethodPost, base+"/targets", map[string]any{
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1", "expected_target_set_version": "2",
		"kind": "squad", "resource_id": squad, "roster": []collaboration.TargetResource{
			{Role: "leader", AgentID: f.agent, RuntimeID: f.runtime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.managed.fx.WorkspaceID},
			{Role: "member", AgentID: member, RuntimeID: f.runtime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.managed.fx.WorkspaceID},
		},
	}, nil).Want(http.StatusCreated).JSON(&registered)
	f.target = util.UUIDToString(registered.TargetID)
	for _, agent := range []string{f.agent, member} {
		copy := *f
		copy.agent = agent
		consentCollaborationRouteTarget(t, &copy)
	}
	return squad, member
}

func TestCollaborationSquadCapacityOneRoutesLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	f := newCollaborationRouteFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(cancel)
	squad, _ := registerCollaborationRouteSquad(t, f)
	base := "/api/collaboration/issues/" + f.issue
	comment := func(text string) service.CollaborationCommentReceipt {
		t.Helper()
		var receipt service.CollaborationCommentReceipt
		f.managed.request(t, f.contributorToken, false, http.MethodPost, base+"/comments", map[string]any{
			"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
			"content":         "[@Squad](mention://squad/" + squad + ") " + text,
			"target_versions": []map[string]string{{"target_id": f.target, "expected_target_epoch": "1", "expected_config_version": "1"}},
		}, nil).Want(http.StatusCreated).JSON(&receipt)
		return receipt
	}
	input := comment("Review using the squad")
	if len(input.Targets) != 1 || !input.Targets[0].CommandID.Valid {
		t.Fatalf("public squad mention did not enqueue its authorized input: %+v", input)
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	controller := collaborationRouteController(t, "succeed", listener.Addr().String())
	controller.Arguments = []string{collaborationSquadFakeArg, listener.Addr().String()}
	ledger, err := collaborationrun.OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion, DeploymentID: f.deployment,
		InstallationID: f.installationID, RuntimeID: f.runtime, BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	seen := map[string]bool{}
	joinedID, cancelledID := "", ""
	for _, step := range []string{"leader", "member", "continuation"} {
		list, err := client.List(ctx, f.runtime)
		if err != nil || len(list.RunIDs) == 0 {
			t.Fatalf("%s was not queued: %+v %v", step, list, err)
		}
		run := list.RunIDs[0]
		if seen[run] {
			t.Fatal("continuation reopened a sealed run")
		}
		seen[run] = true
		done := make(chan error, 1)
		go func() {
			done <- (&collaborationrun.Runner{Ledger: ledger, Client: client, Controller: controller}).Run(ctx, run, request)
		}()
		_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(25 * time.Second))
		connection, err := listener.Accept()
		if err != nil {
			t.Fatalf("%s provider failed to reach the business API: %v", step, err)
		}
		var observed service.CollaborationRuntimeContext
		if err = json.NewDecoder(connection).Decode(&observed); err != nil {
			t.Fatal(err)
		}
		expectedInputs := 1
		if step == "continuation" {
			expectedInputs = 2
		}
		if observed.StepKind != step || observed.Identity.RunID != run || observed.Identity.TargetID != f.target || len(observed.Inputs) != expectedInputs ||
			observed.Inputs[0].AuthorUserID != f.contributor || f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE released_at IS NULL`) != 1 {
			t.Fatalf("%s lost identity, authorship or capacity: %+v", step, observed)
		}
		if step == "leader" {
			joined, cancelled := comment("Include this followup in the next leader turn"), comment("Cancel this unread followup")
			if len(joined.Targets) != 1 || joined.Targets[0].Code != "command_joined_squad" || len(cancelled.Targets) != 1 || cancelled.Targets[0].Code != "command_joined_squad" {
				t.Fatalf("public human followup did not join the live squad: %+v %+v", joined, cancelled)
			}
			joinedID, cancelledID = util.UUIDToString(joined.Targets[0].CommandID), util.UUIDToString(cancelled.Targets[0].CommandID)
			f.managed.request(t, f.contributorToken, false, http.MethodPost, base+"/commands/"+cancelledID+"/cancel", map[string]string{
				"target_id": f.target, "expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1", "expected_collaboration_revision": "1",
			}, nil).Want(http.StatusOK)
			var detail struct {
				Execution service.CollaborationExecutionView `json:"execution"`
			}
			f.managed.request(t, f.ownerToken, false, http.MethodGet, base, nil, nil).Want(http.StatusOK).JSON(&detail)
			if detail.Execution.ProtocolVersion != 5 || len(detail.Execution.Groups) != 1 || len(detail.Execution.Groups[0].UnreadInputs) != 1 ||
				detail.Execution.Groups[0].UnreadInputs[0].CommandID != joinedID || detail.Execution.Groups[0].QueuedContinuationRunID == nil {
				t.Fatalf("public v5 read lost unread/consumed distinction: %+v", detail.Execution)
			}
		}
		if step == "continuation" && (observed.Inputs[1].CommandID != joinedID || observed.Inputs[1].CommandID == cancelledID) {
			t.Fatal("continuation consumed cancelled input or lost the joined followup")
		}
		_, _ = connection.Write([]byte{1})
		_ = connection.Close()
		if err = <-done; err != nil {
			t.Fatalf("%s did not produce and acknowledge a native sealed proof: %v", step, err)
		}
		if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE released_at IS NULL`) != 0 {
			t.Fatalf("%s native proof retained its physical slot", step)
		}
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.session) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id IN ($1,$2) AND state='succeeded'`, input.Targets[0].CommandID, joinedID) != 2 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, cancelledID) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND proof_state='exited_proven' AND settled_at IS NOT NULL`, f.session) != 3 {
		t.Fatal("the final leader result or complete per-run proof ledger was lost")
	}
}
