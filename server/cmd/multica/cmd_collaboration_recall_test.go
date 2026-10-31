package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func recallCLICommand() *cobra.Command {
	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().Int("limit", 50, "")
	cmd.Flags().Int("max-bytes", 65536, "")
	cmd.Flags().String("cursor", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func TestCollaborationCLIRecallExplicitEnvelopeAndMalformedResponse(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "malformed"}[malformed], func(t *testing.T) {
			var identity protocol.CollaborationRunIdentity
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/api/collaboration/runtime/runs/"+identity.RunID+"/comments" || r.URL.Query().Get("limit") != "1" || r.URL.Query().Get("max_bytes") != "4096" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer mcb_") {
					t.Error("recall escaped its explicit route/budget")
				}
				page := protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment]{ProtocolVersion: 1, Identity: &identity, IssueID: identity.IssueID, Items: []protocol.CollaborationRecallComment{}, Total: 0, Returned: 0, MaxBytes: 4096, TruncationReasons: []string{}}
				if malformed {
					page.ProtocolVersion = 99
				}
				for {
					raw, _ := json.Marshal(page)
					if len(raw)+1 == page.Bytes {
						break
					}
					page.Bytes = len(raw) + 1
				}
				_ = json.NewEncoder(w).Encode(page)
			}))
			defer server.Close()
			identity = collaborationCLIEnvironment(t, server.URL)
			cmd := recallCLICommand()
			_ = cmd.Flags().Set("limit", "1")
			_ = cmd.Flags().Set("max-bytes", "4096")
			var output bytes.Buffer
			cmd.SetOut(&output)
			err := runIssueCommentList(cmd, []string{identity.IssueID})
			if (err != nil) != malformed || requests != 1 {
				t.Fatalf("explicit recall: requests=%d err=%v", requests, err)
			}
			if !malformed && (!strings.HasPrefix(output.String(), "{") || !strings.Contains(output.String(), `"total"`)) {
				t.Fatal("new mode did not return envelope")
			}
		})
	}
}

func TestCollaborationCLIRecallGuardsBeforeHTTP(t *testing.T) {
	for _, scenario := range []string{"other_issue", "override", "filter", "limit", "cursor", "content_revision"} {
		t.Run(scenario, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
			defer server.Close()
			identity := collaborationCLIEnvironment(t, server.URL)
			cmd := recallCLICommand()
			cmd.Flags().String("server", "", "")
			cmd.Flags().String("thread", "", "")
			cmd.Flags().String("revision", "", "")
			_ = cmd.Flags().Set("limit", "1")
			issue := identity.IssueID
			operation := "issue comment list"
			args := []string{issue}
			switch scenario {
			case "other_issue":
				args[0] = uuid.NewString()
			case "override":
				_ = cmd.Flags().Set("server", server.URL)
			case "filter":
				_ = cmd.Flags().Set("thread", uuid.NewString())
			case "limit":
				_ = cmd.Flags().Set("limit", "0")
			case "cursor":
				_ = cmd.Flags().Set("cursor", "invalid")
			case "content_revision":
				operation = "issue comment content"
				cmd = recallCLICommand()
				cmd.Flags().String("revision", "01", "")
				args = append(args, uuid.NewString())
			}
			if _, err := runCollaborationCLI(cmd, args, operation); err == nil || requests != 0 {
				t.Fatalf("guard %s reached HTTP or passed: %v", scenario, err)
			}
		})
	}
}

func TestCollaborationCLIRecallContentOperation(t *testing.T) {
	var identity protocol.CollaborationRunIdentity
	comment := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/collaboration/runtime/runs/"+identity.RunID+"/comments/"+comment+"/content" || r.URL.Query().Get("revision") != "1" {
			t.Error("wrong content route")
		}
		chunk := protocol.CollaborationCommentContentChunk{ProtocolVersion: 1, Identity: identity, IssueID: identity.IssueID, CommentID: comment, Revision: "1", Content: "你🙂", TotalBytes: 7, Complete: true, MaxBytes: 65536}
		for {
			raw, _ := json.Marshal(chunk)
			if len(raw)+1 == chunk.Bytes {
				break
			}
			chunk.Bytes = len(raw) + 1
		}
		_ = json.NewEncoder(w).Encode(chunk)
	}))
	defer server.Close()
	identity = collaborationCLIEnvironment(t, server.URL)
	cmd := recallCLICommand()
	cmd.Flags().String("revision", "1", "")
	var output bytes.Buffer
	cmd.SetOut(&output)
	if _, err := runCollaborationCLI(cmd, []string{identity.IssueID, comment}, "issue comment content"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "你🙂") {
		t.Fatal("content lost UTF8")
	}
	if issueCommentListCmd.Flags().Lookup("max-bytes") == nil || issueCommentContentCmd.Flags().Lookup("revision") == nil || !collaborationCommandAllowed("issue comment content") {
		t.Fatal("recall registration/allowlist missing")
	}
}

func TestCollaborationRecallCursorRejectsMissingAndDuplicateFields(t *testing.T) {
	valid := protocol.EncodeCollaborationCommentCursor(time.Now().UTC(), uuid.NewString())
	if _, err := protocol.DecodeCollaborationCommentCursor(valid); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"e30", "bnVsbA", "invalid", strings.Repeat("x", 513)} {
		if _, err := protocol.DecodeCollaborationCommentCursor(value); err == nil {
			t.Fatal("invalid cursor accepted")
		}
	}
}
