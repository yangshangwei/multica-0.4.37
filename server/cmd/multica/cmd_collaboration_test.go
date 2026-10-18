package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func collaborationCLIEnvironment(t *testing.T, server string) protocol.CollaborationRunIdentity {
	t.Helper()
	identity := protocol.CollaborationRunIdentity{DeploymentID: uuid.NewString(), InstallationID: uuid.NewString(), IssueID: uuid.NewString(), SessionID: uuid.NewString(),
		TargetID: uuid.NewString(), ChannelID: uuid.NewString(), BatchID: uuid.NewString(), GroupID: uuid.NewString(), RunID: uuid.NewString(), LaunchNonce: uuid.NewString(), Fence: "1", ClaimGeneration: "1"}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_COLLABORATION_TOKEN", "mcb_"+strings.Repeat("a", 64))
	t.Setenv("MULTICA_COLLABORATION_SERVER", server)
	t.Setenv("MULTICA_COLLABORATION_CONTEXT", string(encoded))
	t.Setenv("MULTICA_COLLABORATION_ISSUE_ID", identity.IssueID)
	t.Setenv("MULTICA_COLLABORATION_RUN_ID", identity.RunID)
	// A complete ordinary context is a trap: narrow commands must ignore it.
	t.Setenv("MULTICA_SERVER_URL", server)
	t.Setenv("MULTICA_WORKSPACE_ID", uuid.NewString())
	t.Setenv("MULTICA_TOKEN", "mul_fixture_must_not_be_used")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
	t.Setenv(cli.TaskConfigRootEnv, t.TempDir())
	return identity
}

func TestCollaborationCLIRejectsOrdinaryClientFallback(t *testing.T) {
	collaborationCLIEnvironment(t, "http://127.0.0.1:1")
	client, err := newAPIClient(&cobra.Command{Use: "list"})
	if err == nil || client != nil {
		t.Fatal("collaboration process obtained an ordinary client and ambient member credential")
	}
}

func TestCollaborationCLIIssueGetUsesOnlyItsRunContext(t *testing.T) {
	var identity protocol.CollaborationRunIdentity
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/collaboration/runtime/runs/"+identity.RunID+"/context" || r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) ||
			r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
			t.Errorf("collaboration read escaped its narrow run identity: path=%s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"identity": identity, "issue_id": identity.IssueID, "title": "Scoped issue", "description": "Shared content", "agent_id": uuid.NewString(), "step_kind": "agent", "roster": []any{}, "inputs": []any{}, "comments": []any{}})
	}))
	defer server.Close()
	identity = collaborationCLIEnvironment(t, server.URL)
	cmd := &cobra.Command{Use: "get"}
	cmd.Flags().String("output", "json", "")
	cmd.SetOut(io.Discard)
	if err := runIssueGet(cmd, []string{identity.IssueID}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("narrow issue read used %d requests, want one context read", requests)
	}
}

func TestCollaborationCLICommentListDoesNotLoadWorkspaceActors(t *testing.T) {
	for _, output := range []string{"json", "table"} {
		t.Run(output, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			commentID, authorID := uuid.NewString(), uuid.NewString()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/api/collaboration/runtime/runs/"+id.RunID+"/context" || r.Header.Get("X-Workspace-ID") != "" {
					t.Errorf("comment display requested an ordinary workspace resource: %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"identity": id, "issue_id": id.IssueID,
					"comments": []map[string]any{{"id": commentID, "author_id": authorID, "author_type": "agent", "content": "Latest member result"}}})
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			cmd := &cobra.Command{Use: "list"}
			cmd.Flags().String("output", output, "")
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := runIssueCommentList(cmd, []string{id.IssueID}); err != nil || requests.Load() != 1 || !strings.Contains(out.String(), commentID) || !strings.Contains(out.String(), "Latest member result") {
				t.Fatalf("run-scoped comment list lost content or used extra lookups: requests=%d err=%v", requests.Load(), err)
			}
		})
	}
}

func TestCollaborationCLIRejectsOtherIssueAndUnsupportedFilesBeforeHTTP(t *testing.T) {
	for _, scenario := range []string{"other_issue", "attachment", "content_file", "missing_request"} {
		t.Run(scenario, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
			defer server.Close()
			id := collaborationCLIEnvironment(t, server.URL)
			cmd := newIssueCommentAddTestCmd()
			cmd.Flags().String("request-id", "", "")
			_ = cmd.Flags().Set("content", "shared instruction")
			if scenario != "missing_request" {
				_ = cmd.Flags().Set("request-id", uuid.NewString())
			}
			issue := id.IssueID
			switch scenario {
			case "other_issue":
				issue = uuid.NewString()
			case "attachment":
				_ = cmd.Flags().Set("attachment", "/does-not-exist/private-file")
			case "content_file":
				_ = cmd.Flags().Set("content-file", "/does-not-exist/private-file")
			}
			if err := runIssueCommentAdd(cmd, []string{issue}); err == nil || requests != 0 {
				t.Fatalf("unsupported collaboration input reached HTTP: requests=%d err=%v", requests, err)
			}
		})
	}
}

func TestCollaborationCLIWholeCommandGuard(t *testing.T) {
	for _, path := range [][]string{{"config", "show"}, {"auth", "status"}, {"workspace", "list"}, {"issue", "update"}, {"daemon", "start"}} {
		t.Run(strings.Join(path, "_"), func(t *testing.T) {
			collaborationCLIEnvironment(t, "http://127.0.0.1:1")
			reached := false
			root := &cobra.Command{Use: "multica", PersistentPreRunE: rootCmd.PersistentPreRunE}
			parent := &cobra.Command{Use: path[0]}
			parent.AddCommand(&cobra.Command{Use: path[1], RunE: func(*cobra.Command, []string) error { reached = true; return nil }})
			root.AddCommand(parent)
			root.SetArgs(path)
			root.SetErr(io.Discard)
			root.SetOut(io.Discard)
			if err := root.Execute(); err == nil || reached {
				t.Fatal("unsupported command reached its ordinary implementation")
			}
		})
	}
}

func TestCollaborationCLIRetriesUncertainWriteWithOriginalIdentity(t *testing.T) {
	for _, operation := range []string{"comment", "activity"} {
		t.Run(operation, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			key := uuid.NewString()
			var posts atomic.Int32
			var bodies sync.Mutex
			var firstBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" {
					t.Error("writer borrowed ordinary credentials or workspace")
				}
				base := "/api/collaboration/runtime/runs/" + id.RunID
				if r.Method == http.MethodGet && r.URL.Path == base+"/context" {
					_ = json.NewEncoder(w).Encode(map[string]any{"identity": id, "issue_id": id.IssueID})
					return
				}
				path, field := "/comments", "comment_id"
				if operation == "activity" {
					path, field = "/squad-activity", "event_id"
				}
				if r.Method != http.MethodPost || r.URL.Path != base+path {
					t.Errorf("writer used an ordinary or unrelated endpoint: %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				data, _ := io.ReadAll(r.Body)
				var body map[string]any
				if json.Unmarshal(data, &body) != nil || body[field] != key {
					t.Error("writer did not preserve the caller's operation identity")
				}
				bodies.Lock()
				defer bodies.Unlock()
				if posts.Add(1) == 1 {
					firstBody = data
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				if !bytes.Equal(firstBody, data) {
					t.Error("retry changed its original body or operation key")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"comment_id": key, "code": "squad_activity_recorded"})
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			cmd := newIssueCommentAddTestCmd()
			invoke := func() error { return runIssueCommentAdd(cmd, []string{id.IssueID}) }
			if operation == "activity" {
				cmd = &cobra.Command{Use: "activity"}
				cmd.Flags().String("reason", "", "")
				cmd.Flags().String("output", "json", "")
				_ = cmd.Flags().Set("reason", "turn completed")
				invoke = func() error { return runSquadActivity(cmd, []string{id.IssueID, "no_action"}) }
			} else {
				_ = cmd.Flags().Set("content", "[@member](mention://agent/"+uuid.NewString()+") implement this")
			}
			cmd.Flags().String("request-id", "", "")
			_ = cmd.Flags().Set("request-id", key)
			cmd.SetOut(io.Discard)
			err := invoke()
			if err == nil || !strings.Contains(cli.FormatError(err, false), key) || posts.Load() != 1 {
				t.Fatalf("uncertain result did not retain its explicit recovery identity: posts=%d err=%v", posts.Load(), err)
			}
			if err := invoke(); err != nil || posts.Load() != 2 {
				t.Fatalf("exact original write did not recover: posts=%d err=%v", posts.Load(), err)
			}
		})
	}
}

func TestCollaborationCLIRejectsMismatchedContextAndCapabilityWithoutLoginAdvice(t *testing.T) {
	for _, response := range []string{"other_run", "401", "403"} {
		t.Run(response, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response == "other_run" {
					other := id
					other.RunID = uuid.NewString()
					_ = json.NewEncoder(w).Encode(map[string]any{"identity": other, "issue_id": id.IssueID})
				} else if response == "401" {
					w.WriteHeader(401)
				} else {
					w.WriteHeader(403)
				}
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			cmd := &cobra.Command{Use: "get"}
			cmd.Flags().String("output", "json", "")
			var output bytes.Buffer
			cmd.SetOut(&output)
			err := runIssueGet(cmd, []string{id.IssueID})
			if err == nil || output.Len() != 0 || strings.Contains(cli.FormatError(err, false), "login") {
				t.Fatalf("wrong or revoked run leaked data or requested human login: %v", err)
			}
		})
	}
}
