package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func TestCollaborationMemoryCommandsAreRegisteredAndAllowed(t *testing.T) {
	for _, path := range [][]string{{"collaboration", "memory", "show"}, {"collaboration", "memory", "update"}} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil || cmd.Name() != path[2] {
			t.Fatalf("%v is not registered: %v", path, err)
		}
		if operation := strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" "); !collaborationCommandAllowed(operation) {
			t.Fatalf("%q is missing from the collaboration allowlist", operation)
		}
	}
}

func TestCollaborationMemoryShowReadsOnlyTheRunDocument(t *testing.T) {
	for _, output := range []string{"table", "json"} {
		t.Run(output, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/api/collaboration/runtime/runs/"+id.RunID+"/memory" ||
					r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" {
					t.Errorf("memory read escaped its run identity: %s %s", r.Method, r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemory{IssueID: id.IssueID, RunID: id.RunID, Version: "3", Content: "Current state: green", Bytes: 20, MaxBytes: 65536})
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			cmd := newCollaborationMemoryShowCmd()
			_ = cmd.Flags().Set("output", output)
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := runCollaborationMemoryShow(cmd, []string{id.IssueID}); err != nil || requests.Load() != 1 {
				t.Fatalf("show failed: requests=%d err=%v", requests.Load(), err)
			}
			want := "version 3 · 20/65536 bytes\nCurrent state: green\n"
			if output == "json" {
				want = `"content": "Current state: green"`
			}
			if !strings.Contains(out.String(), want) {
				t.Fatalf("show printed %q", out.String())
			}
		})
	}
}

func TestCollaborationMemoryShowRejectsAnotherRunsDocument(t *testing.T) {
	var id protocol.CollaborationRunIdentity
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemory{IssueID: id.IssueID, RunID: uuid.NewString(), Version: "1", Content: "other", Bytes: 5})
	}))
	defer server.Close()
	id = collaborationCLIEnvironment(t, server.URL)
	cmd := newCollaborationMemoryShowCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runCollaborationMemoryShow(cmd, []string{id.IssueID}); err == nil || out.Len() != 0 {
		t.Fatalf("another run's document was printed: %v", err)
	}
}

func TestCollaborationMemoryUpdateSendsTheWholeDocument(t *testing.T) {
	for _, source := range []string{"file", "stdin"} {
		t.Run(source, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			key := uuid.NewString()
			const document = "Current state: handler suite green\nNext steps: wire the panel"
			var puts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/api/collaboration/runtime/runs/"+id.RunID+"/memory" || r.Header.Get("X-Workspace-ID") != "" {
					t.Errorf("memory write used an unrelated endpoint: %s %s", r.Method, r.URL.Path)
				}
				puts.Add(1)
				var body protocol.CollaborationRunMemoryUpdate
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RequestID != key || body.BaseVersion != "2" || body.Content != document {
					t.Errorf("memory write body changed: %+v %v", body, err)
				}
				version, size := "3", len(document)
				_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemoryReceipt{Code: "memory_updated", IssueID: id.IssueID, RunID: id.RunID, RequestID: key,
					Version: &version, CurrentVersion: "3", Bytes: &size, MaxBytes: 65536})
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			cmd := newCollaborationMemoryUpdateCmd()
			_ = cmd.Flags().Set("base-version", "2")
			_ = cmd.Flags().Set("request-id", key)
			var out bytes.Buffer
			cmd.SetOut(&out)
			var err error
			if source == "file" {
				dir := t.TempDir()
				t.Chdir(dir)
				if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(document+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				_ = cmd.Flags().Set("content-file", "memory.md")
				err = runCollaborationMemoryUpdate(cmd, []string{id.IssueID})
			} else {
				_ = cmd.Flags().Set("content-stdin", "true")
				pipeStdin(t, document+"\n", func() { err = runCollaborationMemoryUpdate(cmd, []string{id.IssueID}) })
			}
			if err != nil || puts.Load() != 1 || !strings.Contains(out.String(), "saved as version 3") {
				t.Fatalf("update failed: puts=%d err=%v out=%q", puts.Load(), err, out.String())
			}
		})
	}
}

func TestCollaborationMemoryUpdateRejectsBadInputBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  map[string]string
		file string
		want string
	}{
		{name: "missing_request", set: map[string]string{"request-id": ""}, want: "--request-id"},
		{name: "noncanonical_request", set: map[string]string{"request-id": strings.ToUpper(uuid.NewString())}, want: "--request-id"},
		{name: "missing_base", set: map[string]string{"base-version": ""}, want: "--base-version"},
		{name: "negative_base", set: map[string]string{"base-version": "-1"}, want: "--base-version"},
		{name: "noncanonical_base", set: map[string]string{"base-version": "02"}, want: "--base-version"},
		{name: "missing_content", set: map[string]string{"content-file": ""}, want: "--content-stdin or --content-file"},
		{name: "two_sources", set: map[string]string{"content-stdin": "true"}, want: "mutually exclusive"},
		{name: "external_file", set: map[string]string{"content-file": "/does-not-exist/private-file"}, want: "inside the working directory"},
		{name: "local_link", file: "See [log](file:///Users/fixture/run.log)", want: "runtime-local path"},
		{name: "other_issue", want: "admitted issue UUID"},
		{name: "workspace_override", set: map[string]string{"workspace-id": uuid.NewString()}, want: "--workspace-id is unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
			defer server.Close()
			id := collaborationCLIEnvironment(t, server.URL)
			dir := t.TempDir()
			t.Chdir(dir)
			content := tc.file
			if content == "" {
				content = "Current state: fine"
			}
			if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := newCollaborationMemoryUpdateCmd()
			// The root's persistent flags merge into this set at run time.
			cmd.Flags().String("workspace-id", "", "")
			for name, value := range map[string]string{"base-version": "0", "request-id": uuid.NewString(), "content-file": "memory.md"} {
				_ = cmd.Flags().Set(name, value)
			}
			for name, value := range tc.set {
				_ = cmd.Flags().Set(name, value)
			}
			issue := id.IssueID
			if tc.name == "other_issue" {
				issue = uuid.NewString()
			}
			err := runCollaborationMemoryUpdate(cmd, []string{issue})
			if err == nil || requests != 0 || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("bad input was not refused before HTTP: requests=%d err=%v", requests, err)
			}
			if strings.Contains(err.Error(), "allow-external-file") || strings.Contains(err.Error(), "--content,") {
				t.Fatal("refusal named a flag this command does not have")
			}
		})
	}
}

func TestCollaborationMemoryUpdateRefusalsNameTheNextAction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"stale", http.StatusConflict, `{"code":"stale_memory_version","version":null,"current_version":"7","max_bytes":65536}`, "multica collaboration memory show {issue}`, merge your changes into that version, then update again with --base-version 7"},
		{"too_large", http.StatusRequestEntityTooLarge, `{"code":"memory_too_large","version":null,"current_version":"2","bytes":70000,"max_bytes":65536}`, "70000 bytes exceeds the 65536-byte limit"},
		{"reused_request", http.StatusConflict, `{"code":"memory_write_conflict","version":null,"current_version":"3","max_bytes":65536}`, "(another --base-version or content)"},
		{"unsupported", http.StatusConflict, `{"code":"memory_unsupported","error":"memory_unsupported"}`, "unavailable for this run"},
		{"revoked", http.StatusForbidden, `{"code":"business_capability_required"}`, "Stop this turn"},
		{"unknown", http.StatusBadGateway, `{}`, "Retry the unchanged command with --request-id"},
		{"invalid_content", http.StatusBadRequest, `{"code":"invalid_memory_content","version":null,"current_version":"2","max_bytes":65536}`, "UTF-8 text without NUL"},
		// The handler's read bound answers before the service, without a receipt.
		{"body_too_large", http.StatusRequestEntityTooLarge, `{"code":"invalid_runtime_request","error":"Invalid runtime request"}`, "Shorten the document (keep conclusions and pointers, move details to a comment), then run `multica collaboration memory show {issue}`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			id := collaborationCLIEnvironment(t, server.URL)
			dir := t.TempDir()
			t.Chdir(dir)
			if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("Current state: fine"), 0600); err != nil {
				t.Fatal(err)
			}
			key := uuid.NewString()
			cmd := newCollaborationMemoryUpdateCmd()
			for name, value := range map[string]string{"base-version": "2", "request-id": key, "content-file": "memory.md"} {
				_ = cmd.Flags().Set(name, value)
			}
			err := runCollaborationMemoryUpdate(cmd, []string{id.IssueID})
			message := cli.FormatError(err, false)
			// The stale hint must be a runnable command, including the issue.
			want := strings.ReplaceAll(tc.want, "{issue}", id.IssueID)
			if err == nil || !strings.Contains(message, want) || strings.Contains(message, "login") {
				t.Fatalf("refusal did not name the next action: %q", message)
			}
			if tc.name == "unknown" && !strings.Contains(message, key) {
				t.Fatal("uncertain write did not keep its original request ID")
			}
		})
	}
}

func TestCollaborationMemoryShowRefusalAndUpdateJSON(t *testing.T) {
	var id protocol.CollaborationRunIdentity
	key := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"code":"memory_unsupported","error":"memory_unsupported"}`)
			return
		}
		version, size := "1", 19
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemoryReceipt{Code: "memory_updated", IssueID: id.IssueID, RunID: id.RunID, RequestID: key,
			Version: &version, CurrentVersion: "1", Bytes: &size, MaxBytes: 65536})
	}))
	defer server.Close()
	id = collaborationCLIEnvironment(t, server.URL)
	show := newCollaborationMemoryShowCmd()
	var shown bytes.Buffer
	show.SetOut(&shown)
	err := runCollaborationMemoryShow(show, []string{id.IssueID})
	if err == nil || shown.Len() != 0 || !strings.Contains(cli.FormatError(err, false), "unavailable for this run") {
		t.Fatalf("unsupported show was not explained: %v", err)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("Current state: fine"), 0600); err != nil {
		t.Fatal(err)
	}
	update := newCollaborationMemoryUpdateCmd()
	for name, value := range map[string]string{"base-version": "0", "request-id": key, "content-file": "memory.md", "output": "json"} {
		_ = update.Flags().Set(name, value)
	}
	var out bytes.Buffer
	update.SetOut(&out)
	if err := runCollaborationMemoryUpdate(update, []string{id.IssueID}); err != nil {
		t.Fatal(err)
	}
	var receipt protocol.CollaborationRunMemoryReceipt
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.RequestID != key || receipt.Version == nil || *receipt.Version != "1" {
		t.Fatalf("json receipt was not printed: %q %v", out.String(), err)
	}
}

// collaborationMemoryTestRoot mirrors the production root: its guard, the
// persistent flags that merge into every command, and the real subcommands.
func collaborationMemoryTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "multica", PersistentPreRunE: rootCmd.PersistentPreRunE, SilenceUsage: true, SilenceErrors: true}
	for _, name := range []string{"server-url", "workspace-id", "profile"} {
		root.PersistentFlags().String(name, "", "")
	}
	root.PersistentFlags().Bool("debug", false, "")
	collaboration := &cobra.Command{Use: "collaboration"}
	memory := &cobra.Command{Use: "memory"}
	memory.AddCommand(newCollaborationMemoryShowCmd(), newCollaborationMemoryUpdateCmd())
	collaboration.AddCommand(memory)
	root.AddCommand(collaboration)
	return root
}

func TestCollaborationMemoryCommandsThroughTheRoot(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     func(issue string) []string
		requests int32
		want     string
	}{
		{"show", func(issue string) []string { return []string{"show", issue, "--debug"} }, 1, ""},
		{"show_workspace_override", func(issue string) []string { return []string{"show", issue, "--workspace-id", uuid.NewString()} }, 0, "--workspace-id is unavailable"},
		{"show_profile_override", func(issue string) []string { return []string{"show", issue, "--profile", "dev"} }, 0, "--profile is unavailable"},
		{"show_server_override", func(issue string) []string { return []string{"show", issue, "--server-url", "http://127.0.0.1:1"} }, 0, "--server-url is unavailable"},
		{"show_extra_argument", func(issue string) []string { return []string{"show", issue, "extra"} }, 0, "accepts 1 arg"},
		{"update", func(issue string) []string {
			return []string{"update", issue, "--base-version", "0", "--request-id", uuid.NewString(), "--content-file", "memory.md"}
		}, 1, ""},
		{"update_unknown_flag", func(issue string) []string {
			return []string{"update", issue, "--base-version", "0", "--request-id", uuid.NewString(), "--content-file", "memory.md", "--allow-external-file"}
		}, 0, "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var id protocol.CollaborationRunIdentity
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemory{IssueID: id.IssueID, RunID: id.RunID, Version: "0", MaxBytes: 65536})
					return
				}
				var body protocol.CollaborationRunMemoryUpdate
				_ = json.NewDecoder(r.Body).Decode(&body)
				version, size := "1", 19
				_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemoryReceipt{Code: "memory_updated", IssueID: id.IssueID, RunID: id.RunID, RequestID: body.RequestID,
					Version: &version, CurrentVersion: "1", Bytes: &size, MaxBytes: 65536})
			}))
			defer server.Close()
			id = collaborationCLIEnvironment(t, server.URL)
			dir := t.TempDir()
			t.Chdir(dir)
			if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("Current state: fine"), 0600); err != nil {
				t.Fatal(err)
			}
			root := collaborationMemoryTestRoot()
			root.SetArgs(append([]string{"collaboration", "memory"}, tc.args(id.IssueID)...))
			var out, errOut bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errOut)
			err := root.Execute()
			if requests.Load() != tc.requests {
				t.Fatalf("made %d requests, want %d (err=%v)", requests.Load(), tc.requests, err)
			}
			if tc.want == "" {
				if err != nil || out.Len() == 0 {
					t.Fatalf("command failed through the root: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error()+errOut.String(), tc.want) {
				t.Fatalf("refusal = %v (stderr %q), want %q", err, errOut.String(), tc.want)
			}
		})
	}
}

func TestCollaborationMemoryRefusesOutsideACollaborationRun(t *testing.T) {
	for _, name := range []string{"MULTICA_COLLABORATION_TOKEN", "MULTICA_COLLABORATION_SERVER", "MULTICA_COLLABORATION_CONTEXT", "MULTICA_COLLABORATION_ISSUE_ID", "MULTICA_COLLABORATION_RUN_ID"} {
		t.Setenv(name, "")
	}
	for _, cmd := range []*cobra.Command{newCollaborationMemoryShowCmd(), newCollaborationMemoryUpdateCmd()} {
		err := cmd.RunE(cmd, []string{uuid.NewString()})
		if err == nil || !strings.Contains(err.Error(), "only available inside a collaboration run") {
			t.Fatalf("%s ran outside a collaboration run: %v", cmd.Name(), err)
		}
	}
}
