package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

// Request encoding, reference validation and identity binding are owned by
// internal/cli/collaboration_handoff_test.go; the decode matrix by protocol's
// source-page tests. These cases cover registration, the run guard and wiring.

func TestCollaborationHandoffCommandsAreRegisteredAndAllowed(t *testing.T) {
	for _, path := range [][]string{{"collaboration", "handoff", "show"}, {"collaboration", "handoff", "source"}} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil || cmd.Name() != path[2] {
			t.Fatalf("%v is not registered: %v", path, err)
		}
		if operation := strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" "); !collaborationCommandAllowed(operation) {
			t.Fatalf("%q is missing from the collaboration allowlist", operation)
		}
	}
	if !strings.Contains(collaborationHandoffShowCmd.Long, `mode "live"`) || !strings.Contains(collaborationHandoffShowCmd.Long, "not the startup snapshot") {
		t.Fatal("show help does not separate the live read from the frozen startup snapshot")
	}
	// The brief names these flags; each must exist on its registered command.
	for cmd, flags := range map[*cobra.Command][]string{
		collaborationHandoffShowCmd:   {"cursor"},
		collaborationHandoffSourceCmd: {"kind", "source-run-id", "version", "sha256", "offset"},
		issueCommentListCmd:           {"limit", "max-bytes", "cursor"},
		issueCommentContentCmd:        {"revision", "cursor"},
	} {
		for _, flag := range flags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s has no --%s", cmd.CommandPath(), flag)
			}
		}
	}
}

func handoffCLIEnvelope(id protocol.CollaborationRunIdentity) protocol.CollaborationHandoffEnvelope {
	return protocol.CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: id.IssueID, RunID: id.RunID, Mode: "live", EventSequence: "9",
		Authority: protocol.CollaborationHandoffAuthority{Status: "in_progress"}, Assignment: protocol.CollaborationHandoffAssignment{Role: "continuation", StepKey: "leader-2"},
		Memory: protocol.CollaborationHandoffMemory{Version: "0", Availability: "absent"},
		Inputs: protocol.CollaborationHandoffInputs{Items: []protocol.CollaborationHandoffInput{}}, Results: protocol.CollaborationHandoffResults{Items: []protocol.CollaborationHandoffResult{}},
		WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "unknown"},
		Sources:      []protocol.CollaborationHandoffSource{}, Missing: []string{}}
}

// A model follows next_cursor across result pages, then next_offset across one
// truncated source, using only the printed JSON and the documented commands.
func TestCollaborationHandoffCommandsWalkTruncatedResultsAndSource(t *testing.T) {
	var id protocol.CollaborationRunIdentity
	const cursor = "eyJraW5kIjoicmVzdWx0cyJ9"
	firstChunk, secondChunk := "成员结果：选用共享数据库。", "理由：迁移已在克隆库验证。"
	body := firstChunk + secondChunk
	resultRun, sourceID := uuid.NewString(), uuid.NewString()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		base := "/api/collaboration/runtime/runs/" + id.RunID
		if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" {
			t.Error("handoff read borrowed ordinary credentials")
		}
		switch r.URL.Path {
		case base + "/handoff":
			envelope := handoffCLIEnvelope(id)
			envelope.Results.Total = 2
			if r.URL.Query().Get("cursor") == "" {
				envelope.Results.Returned, envelope.Results.Truncated, envelope.Results.NextCursor = 1, true, cursor
				envelope.Results.Items = []protocol.CollaborationHandoffResult{{RunID: uuid.NewString(), Role: "continuation", StepKey: "leader-2", Progress: "running", ProofState: "pending", ResultState: "absent", FinalAnswerState: "unknown"}}
			} else if r.URL.Query().Get("cursor") == cursor {
				envelope.Results.Returned = 1
				envelope.Results.Items = []protocol.CollaborationHandoffResult{{RunID: resultRun, Role: "member", StepKey: "dispatch-1", Progress: "settled", Outcome: "success", ProofState: "host_exit_observed",
					ResultState: "present", FinalAnswerState: "unknown", SourceID: sourceID, SourceKind: "provider_result"}}
				envelope.Sources = []protocol.CollaborationHandoffSource{{Kind: "provider_result", ID: sourceID, RunID: resultRun, Version: "5", SHA256: protocol.CollaborationHandoffHash(body),
					Content: firstChunk, Truncated: true, Availability: "available", Trust: "data_only"}}
			} else {
				t.Errorf("unexpected handoff cursor %q", r.URL.Query().Get("cursor"))
			}
			_ = json.NewEncoder(w).Encode(envelope)
		case base + "/handoff/sources/" + sourceID:
			q := r.URL.Query()
			if q.Get("kind") != "provider_result" || q.Get("source_run_id") != resultRun || q.Get("version") != "5" || q.Get("sha256") != protocol.CollaborationHandoffHash(body) {
				t.Errorf("source reference changed: %s", r.URL.RawQuery)
			}
			offset, _ := strconv.Atoi(q.Get("offset"))
			end := len(body)
			if offset == 0 {
				end = len(firstChunk)
			}
			page := protocol.CollaborationHandoffSourcePage{ProtocolVersion: 1, IssueID: id.IssueID, RunID: id.RunID, Offset: offset, TotalBytes: len(body),
				Source: protocol.CollaborationHandoffSource{Kind: "provider_result", ID: sourceID, RunID: resultRun, Version: "5", SHA256: protocol.CollaborationHandoffHash(body),
					Content: body[offset:end], Truncated: end < len(body), Availability: "available", Trust: "data_only"}}
			if end < len(body) {
				page.NextOffset = &end
			}
			_ = json.NewEncoder(w).Encode(page)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	id = collaborationCLIEnvironment(t, server.URL)

	var source protocol.CollaborationHandoffSource
	next := ""
	for page := 0; page < 3; page++ {
		cmd := newCollaborationHandoffShowCmd()
		if next != "" {
			_ = cmd.Flags().Set("cursor", next)
		}
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := runCollaborationHandoffShow(cmd, []string{id.IssueID}); err != nil {
			t.Fatal(err)
		}
		var envelope protocol.CollaborationHandoffEnvelope
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("show printed an envelope its own strict decoder refuses: %v", err)
		}
		for _, s := range envelope.Sources {
			if s.Truncated {
				source = s
			}
		}
		if next = envelope.Results.NextCursor; next == "" {
			break
		}
	}
	if source.ID != sourceID {
		t.Fatal("the truncated source on the second results page was not reached")
	}
	assembled, offset := "", 0
	for chunk := 0; chunk < 3; chunk++ {
		cmd := newCollaborationHandoffSourceCmd()
		for name, value := range map[string]string{"kind": source.Kind, "source-run-id": source.RunID, "version": source.Version, "sha256": source.SHA256, "offset": strconv.Itoa(offset)} {
			_ = cmd.Flags().Set(name, value)
		}
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := runCollaborationHandoffSource(cmd, []string{id.IssueID, source.ID}); err != nil {
			t.Fatal(err)
		}
		var page protocol.CollaborationHandoffSourcePage
		if err := json.Unmarshal(out.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		assembled += page.Source.Content
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if assembled != body || requests.Load() != 4 {
		t.Fatalf("recall assembled %q in %d requests", assembled, requests.Load())
	}
}

func TestCollaborationHandoffRejectsAnotherRunsEnvelope(t *testing.T) {
	var id protocol.CollaborationRunIdentity
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other := id
		other.RunID = uuid.NewString()
		_ = json.NewEncoder(w).Encode(handoffCLIEnvelope(other))
	}))
	defer server.Close()
	id = collaborationCLIEnvironment(t, server.URL)
	cmd := newCollaborationHandoffShowCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runCollaborationHandoffShow(cmd, []string{id.IssueID}); err == nil || out.Len() != 0 {
		t.Fatalf("another run's handoff was printed: %v", err)
	}
}

// collaborationHandoffTestRoot mirrors the production root: its guard, its
// version flag, the persistent flags that merge into every command and the
// real subcommands.
func collaborationHandoffTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "multica", Version: "test", PersistentPreRunE: rootCmd.PersistentPreRunE, SilenceUsage: true, SilenceErrors: true}
	for _, name := range []string{"server-url", "workspace-id", "profile"} {
		root.PersistentFlags().String(name, "", "")
	}
	root.PersistentFlags().Bool("debug", false, "")
	collaboration := &cobra.Command{Use: "collaboration"}
	handoff := &cobra.Command{Use: "handoff"}
	handoff.AddCommand(newCollaborationHandoffShowCmd(), newCollaborationHandoffSourceCmd())
	collaboration.AddCommand(handoff)
	root.AddCommand(collaboration)
	return root
}

func TestCollaborationHandoffCommandsThroughTheRoot(t *testing.T) {
	sha := protocol.CollaborationHandoffHash("note")
	for _, tc := range []struct {
		name     string
		args     func(issue, source string) []string
		requests int32
		want     string
	}{
		{"show", func(issue, _ string) []string { return []string{"show", issue, "--debug"} }, 1, ""},
		{"source", func(issue, source string) []string {
			return []string{"source", issue, source, "--kind", "handoff", "--sha256", sha, "--output", "json"}
		}, 1, ""},
		{"source_version_flag", func(issue, source string) []string {
			return []string{"source", issue, source, "--kind", "provider_result", "--source-run-id", uuid.NewString(), "--version", "5", "--sha256", sha}
		}, 1, ""},
		{"show_other_issue", func(_, _ string) []string { return []string{"show", uuid.NewString()} }, 0, "admitted issue UUID"},
		{"show_workspace_override", func(issue, _ string) []string { return []string{"show", issue, "--workspace-id", uuid.NewString()} }, 0, "--workspace-id is unavailable"},
		{"show_profile_override", func(issue, _ string) []string { return []string{"show", issue, "--profile", "dev"} }, 0, "--profile is unavailable"},
		{"source_server_override", func(issue, source string) []string {
			return []string{"source", issue, source, "--kind", "handoff", "--sha256", sha, "--server-url", "http://127.0.0.1:1"}
		}, 0, "--server-url is unavailable"},
		{"show_table_output", func(issue, _ string) []string { return []string{"show", issue, "--output", "table"} }, 0, "only --output json"},
		{"show_source_flag", func(issue, _ string) []string { return []string{"show", issue, "--kind", "handoff"} }, 0, "unknown flag"},
		{"source_comment_flag", func(issue, source string) []string {
			return []string{"source", issue, source, "--kind", "handoff", "--sha256", sha, "--limit", "1"}
		}, 0, "unknown flag"},
		{"source_missing_id", func(issue, _ string) []string { return []string{"source", issue, "--kind", "handoff", "--sha256", sha} }, 0, "accepts 2 args"},
		{"show_extra_argument", func(issue, _ string) []string { return []string{"show", issue, "extra"} }, 0, "accepts 1 arg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			var requests atomic.Int32
			sourceID := uuid.NewString()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if strings.HasSuffix(r.URL.Path, "/handoff") {
					_ = json.NewEncoder(w).Encode(handoffCLIEnvelope(id))
					return
				}
				q := r.URL.Query()
				page := protocol.CollaborationHandoffSourcePage{ProtocolVersion: 1, IssueID: id.IssueID, RunID: id.RunID, TotalBytes: 4,
					Source: protocol.CollaborationHandoffSource{Kind: q.Get("kind"), ID: sourceID, RunID: q.Get("source_run_id"), Version: q.Get("version"), SHA256: q.Get("sha256"),
						Content: "note", Availability: "available", Trust: "data_only"}}
				_ = json.NewEncoder(w).Encode(page)
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			root := collaborationHandoffTestRoot()
			root.SetArgs(append([]string{"collaboration", "handoff"}, tc.args(id.IssueID, sourceID)...))
			var out, errOut bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errOut)
			err := root.Execute()
			if requests.Load() != tc.requests {
				t.Fatalf("made %d requests, want %d (err=%v)", requests.Load(), tc.requests, err)
			}
			if tc.want == "" {
				if err != nil || !strings.HasPrefix(out.String(), "{") {
					t.Fatalf("command failed through the root: %v %q", err, out.String())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error()+errOut.String(), tc.want) {
				t.Fatalf("refusal = %v (stderr %q), want %q", err, errOut.String(), tc.want)
			}
		})
	}
}

func TestCollaborationHandoffRefusesOutsideACollaborationRun(t *testing.T) {
	for _, name := range []string{"MULTICA_COLLABORATION_TOKEN", "MULTICA_COLLABORATION_SERVER", "MULTICA_COLLABORATION_CONTEXT", "MULTICA_COLLABORATION_ISSUE_ID", "MULTICA_COLLABORATION_RUN_ID"} {
		t.Setenv(name, "")
	}
	for _, tc := range []struct {
		cmd  *cobra.Command
		args []string
	}{
		{newCollaborationHandoffShowCmd(), []string{uuid.NewString()}},
		{newCollaborationHandoffSourceCmd(), []string{uuid.NewString(), uuid.NewString()}},
	} {
		err := tc.cmd.RunE(tc.cmd, tc.args)
		if err == nil || !strings.Contains(err.Error(), "only available inside a collaboration run") {
			t.Fatalf("%s ran outside a collaboration run: %v", tc.cmd.Name(), err)
		}
	}
}
