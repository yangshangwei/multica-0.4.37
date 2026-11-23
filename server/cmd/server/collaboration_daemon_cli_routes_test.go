package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("MULTICA_RUN_COLLAB_RUNTIME_E2E") != "1" {
		t.Skip("requires explicit Linux namespace integration environment")
	}
	cliPath, daemonPath := os.Getenv("MULTICA_TEST_COLLAB_CLI"), os.Getenv("MULTICA_TEST_COLLAB_DAEMON")
	if cliPath == "" || daemonPath == "" {
		t.Fatal("requires test-built CLI and daemon test executables")
	}
	f := newCollaborationRouteFixture(t)
	directTarget := f.target
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	t.Cleanup(cancel)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	controller := collaborationRouteController(t, "succeed", listener.Addr().String())
	upstream, err := url.Parse(f.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	proofs := make(chan string, 8)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ModifyResponse = func(response *http.Response) error {
		path := response.Request.URL.Path
		if response.StatusCode == http.StatusOK && strings.HasPrefix(path, "/api/collaboration/runtime/runs/") && strings.HasSuffix(path, "/proof") {
			proofs <- strings.TrimSuffix(strings.TrimPrefix(path, "/api/collaboration/runtime/runs/"), "/proof")
		}
		return nil
	}
	forwarder := httptest.NewServer(proxy)
	t.Cleanup(forwarder.Close)
	var daemonID string
	f.managed.fx.QueryRow(t, `SELECT daemon_id FROM agent_runtime WHERE id=$1`, f.runtime).Scan(&daemonID)
	helper := exec.CommandContext(ctx, daemonPath, "__multica_collaboration_router_poller")
	input, err := helper.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	helper.Stderr = &diagnostics
	if err = helper.Start(); err != nil {
		t.Fatal(err)
	}
	var waitOnce sync.Once
	var waitErr error
	wait := func() error { waitOnce.Do(func() { waitErr = helper.Wait() }); return waitErr }
	t.Cleanup(func() {
		_ = input.Close()
		cancel()
		_ = wait()
	})
	if err = json.NewEncoder(input).Encode(map[string]any{"ServerURL": forwarder.URL, "WorkspaceID": f.managed.fx.WorkspaceID,
		"RuntimeID": f.runtime, "DaemonID": daemonID, "DeploymentID": f.deployment, "InstallationID": f.installationID,
		"Directory": t.TempDir(), "Executable": controller.Executable, "CLIExecutable": cliPath,
		"Arguments": []string{collaborationSquadCLIFakeArg, listener.Addr().String()}, "Credential": f.binding}); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(output)
	var ready struct {
		Ready bool `json:"ready"`
		Max   int  `json:"max_concurrent_tasks"`
	}
	if err = decoder.Decode(&ready); err != nil || !ready.Ready || ready.Max != 1 {
		cancel()
		_ = wait()
		t.Fatalf("real daemon poller did not initialize at capacity one: %+v err=%v: %s", ready, err, diagnostics.String())
	}
	squad, member := registerCollaborationRouteSquad(t, f)
	comment := func(target, kind, resource, token, body string) service.CollaborationCommentReceipt {
		t.Helper()
		var receipt service.CollaborationCommentReceipt
		f.managed.request(t, token, false, http.MethodPost, "/api/collaboration/issues/"+f.issue+"/comments", map[string]any{
			"content":                  "[@Target](mention://" + kind + "/" + resource + ") " + body,
			"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1",
			"target_versions": []map[string]string{{"target_id": target, "expected_target_epoch": "1", "expected_config_version": "1"}},
		}, nil).Want(http.StatusCreated).JSON(&receipt)
		return receipt
	}
	first := comment(f.target, "squad", squad, f.contributorToken, "Perform the complete CLI squad handoff")
	if len(first.Targets) != 1 || first.Targets[0].Code != "command_queued" {
		t.Fatalf("squad input was not queued: %+v", first)
	}
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	seen := map[string]bool{}
	groupID, joinedID := "", ""
	for _, step := range []string{"leader", "member", "continuation"} {
		_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(25 * time.Second))
		connection, err := listener.Accept()
		if err != nil {
			cancel()
			_ = wait()
			t.Fatalf("daemon failed to execute %s through CLI: %v: %s", step, err, diagnostics.String())
		}
		var observed service.CollaborationRuntimeContext
		if err = json.NewDecoder(connection).Decode(&observed); err != nil {
			_ = connection.Close()
			t.Fatal(err)
		}
		id := observed.Identity.RunID
		if groupID == "" {
			groupID = observed.Identity.GroupID
		}
		expectedAgent := f.agent
		if step == "member" {
			expectedAgent = member
		}
		if seen[id] || len(observed.Inputs) == 0 || observed.StepKind != step || observed.Identity.GroupID != groupID || observed.Identity.TargetID != f.target ||
			util.UUIDToString(observed.AgentID) != expectedAgent || observed.Inputs[0].AuthorUserID != f.contributor ||
			f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE released_at IS NULL`) != 1 {
			_ = connection.Close()
			t.Fatalf("%s changed run identity, authority or real capacity", step)
		}
		seen[id] = true
		if step == "member" {
			joined := comment(directTarget, "agent", f.agent, f.ownerToken, "Join the next leader turn while the member works")
			if len(joined.Targets) != 1 || joined.Targets[0].Code != "command_joined_squad" || util.UUIDToString(joined.Targets[0].GroupID) != groupID {
				_ = connection.Close()
				t.Fatalf("human leader mention did not join the current squad: %+v", joined)
			}
			joinedID = util.UUIDToString(joined.Targets[0].CommandID)
		}
		if step == "continuation" && (len(observed.Inputs) != 2 || observed.Inputs[1].CommandID != joinedID || observed.Inputs[1].AuthorUserID != f.owner) {
			_ = connection.Close()
			t.Fatal("CLI continuation lost the newly consumed input and its distinct author")
		}
		_, err = connection.Write([]byte{1})
		_ = connection.Close()
		if err != nil {
			t.Fatal(err)
		}
		select {
		case proven := <-proofs:
			if proven != id {
				t.Fatal("daemon proof belonged to a different turn")
			}
		case <-ctx.Done():
			t.Fatalf("%s never supplied its real sealed proof", step)
		}
	}
	_ = input.Close()
	var finished struct {
		Claims          int  `json:"claims"`
		InvalidCapacity bool `json:"invalid_capacity"`
		AvailableSlots  int  `json:"available_slots"`
		ActiveTasks     int  `json:"active_tasks"`
	}
	if err = decoder.Decode(&finished); err != nil {
		t.Fatal(err)
	}
	if err = wait(); err != nil {
		t.Fatalf("daemon helper failed: %v: %s", err, diagnostics.String())
	}
	if finished.Claims != 3 || finished.InvalidCapacity || finished.AvailableSlots != 1 || finished.ActiveTasks != 0 {
		t.Fatalf("production poller did not preserve/release its single machine slot: %+v", finished)
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, groupID) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND proof_state='exited_proven' AND settled_at IS NOT NULL AND authorized_attempt_seq=1`, groupID) != 3 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id IN ($1,$2) AND state='succeeded'`, first.Targets[0].CommandID, joinedID) != 2 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_delegation WHERE group_id=$1`, groupID) != 1 {
		t.Fatal("CLI replay, final leader outcome or independent proof ledger was lost")
	}
}
