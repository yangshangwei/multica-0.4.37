package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func TestCollaborationCLIOutputO09RegistrationAndFileGuards(t *testing.T) {
	if issueCommentAddCmd.Flags().Lookup("attachment") == nil {
		t.Fatal("existing issue comment command did not register --attachment")
	}
	for _, scenario := range []string{"url", "multiple", "empty", "external_override", "profile", "other_issue", "missing_request"} {
		t.Run(scenario, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			id := collaborationCLIEnvironment(t, server.URL)
			t.Chdir(t.TempDir())
			if err := os.WriteFile("output.txt", []byte("original file"), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := newIssueCommentAddTestCmd()
			cmd.Flags().String("request-id", "", "")
			cmd.Flags().String("profile", "", "")
			cmd.SetOut(io.Discard)
			_ = cmd.Flags().Set("content", "original comment")
			_ = cmd.Flags().Set("request-id", uuid.NewString())
			if scenario != "empty" {
				_ = cmd.Flags().Set("attachment", "output.txt")
			}
			issue := id.IssueID
			switch scenario {
			case "url":
				_ = cmd.Flags().Set("attachment", "https://example.test/file.txt")
			case "multiple":
				_ = cmd.Flags().Set("attachment", "output.txt")
			case "empty":
				_ = cmd.Flags().Set("attachment", "")
			case "external_override":
				_ = cmd.Flags().Set("allow-external-file", "true")
			case "profile":
				_ = cmd.Flags().Set("profile", "ordinary-trap")
			case "other_issue":
				issue = uuid.NewString()
			case "missing_request":
				_ = cmd.Flags().Set("request-id", "")
			}
			if err := runIssueCommentAdd(cmd, []string{issue}); err == nil || requests.Load() != 0 {
				t.Fatalf("invalid runtime attachment %s reached any HTTP/profile: requests=%d err=%v", scenario, requests.Load(), err)
			}
		})
	}
}

func TestCollaborationCLIOutputO09UsesBoundAgentCommentEndpoint(t *testing.T) {
	var id protocol.CollaborationRunIdentity
	agent, request, fileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	data := []byte("original runtime file")
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/collaboration/runtime/runs/" + id.RunID
		if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
			t.Error("file command borrowed ordinary actor authority")
		}
		if r.Method == http.MethodGet && r.URL.Path == base+"/context" {
			gets.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"identity": id, "issue_id": id.IssueID, "agent_id": agent})
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != base+"/comments/attachments" {
			t.Errorf("file command used unrelated endpoint: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		posts.Add(1)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("comment_id") != request || r.FormValue("content") != "original comment" || len(r.MultipartForm.Value) != 2 || len(r.MultipartForm.File["file"]) != 1 {
			t.Error("combined comment/file did not carry only its original identity")
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"comment_id": request, "run_ids": []string{}, "continuation_run_id": nil,
			"attachment": map[string]any{"id": fileID, "draft_id": nil, "issue_id": id.IssueID, "intended_issue_id": nil,
				"source_workspace_id": uuid.NewString(), "uploader_user_id": nil, "uploader_agent_id": agent, "run_id": id.RunID,
				"attempt_seq": "1", "name": "output.txt", "content_type": http.DetectContentType(data), "size_bytes": len(data), "version": "1",
				"created_at": "2026-11-19T01:02:03Z", "delivered_at": "2026-11-19T01:02:03Z"}})
	}))
	defer server.Close()
	id = collaborationCLIEnvironment(t, server.URL)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("output.txt", data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newIssueCommentAddTestCmd()
	cmd.Flags().String("request-id", "", "")
	_ = cmd.Flags().Set("request-id", request)
	_ = cmd.Flags().Set("content", "original comment")
	_ = cmd.Flags().Set("attachment", "output.txt")
	var output strings.Builder
	cmd.SetOut(&output)
	if err := runIssueCommentAdd(cmd, []string{id.IssueID}); err != nil || gets.Load() != 1 || posts.Load() != 1 || !strings.Contains(output.String(), fileID) {
		t.Fatalf("bound file command did not validate and print its complete receipt: gets=%d posts=%d err=%v", gets.Load(), posts.Load(), err)
	}
}

// The whole-command guard must admit only the existing command's exact file
// flag. It still rejects workspace/profile and external-file overrides.
func TestCollaborationCLIOutputO09WholeCommandGuardAllowsOnlyOneFile(t *testing.T) {
	id := collaborationCLIEnvironment(t, "http://127.0.0.1:1")
	root := &cobra.Command{Use: "multica", PersistentPreRunE: rootCmd.PersistentPreRunE}
	issue := &cobra.Command{Use: "issue"}
	comment := &cobra.Command{Use: "comment"}
	reached := false
	add := newIssueCommentAddTestCmd()
	add.Use = "add <issue-id>"
	add.Flags().String("request-id", "", "")
	add.RunE = func(*cobra.Command, []string) error { reached = true; return nil }
	comment.AddCommand(add)
	issue.AddCommand(comment)
	root.AddCommand(issue)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"issue", "comment", "add", id.IssueID, "--content", "original", "--attachment", "output.txt", "--request-id", uuid.NewString()})
	if err := root.Execute(); err != nil || !reached {
		t.Fatalf("one explicit output file was rejected by the actual root guard: %v", err)
	}
}
