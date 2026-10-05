package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

// validateProjectStatus must accept the five DB-backed statuses and reject
// anything else with a message that lists the valid values. `project create`,
// `project update`, and `project status` all share it (#3925: `--status active`
// used to reach the server and 500 on the CHECK constraint).
func TestValidateProjectStatus(t *testing.T) {
	for _, s := range validProjectStatuses {
		if err := validateProjectStatus(s); err != nil {
			t.Errorf("status %q should be valid, got: %v", s, err)
		}
	}
	err := validateProjectStatus("active")
	if err == nil {
		t.Fatal("status \"active\" should be rejected")
	}
	if !strings.Contains(err.Error(), "planned") {
		t.Errorf("error should list valid statuses, got: %v", err)
	}
}

// newProjectResourceUpdateTestCmd mirrors the flag surface of
// projectResourceUpdateCmd so unit tests can exercise the shortcut-flag plumbing
// without spinning up a server.
func newProjectResourceUpdateTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "update"}
	c.Flags().String("url", "", "")
	c.Flags().String("default-branch-hint", "", "")
	c.Flags().String("local-path", "", "")
	c.Flags().String("daemon-id", "", "")
	c.Flags().String("ref-label", "", "")
	c.Flags().String("execution-mode", "", "")
	c.Flags().String("ref", "", "")
	c.Flags().String("label", "", "")
	c.Flags().Bool("clear-label", false, "")
	c.Flags().Int32("position", 0, "")
	c.Flags().String("output", "json", "")
	return c
}

// TestBuildResourceRefFromFlagsGithubMergesHint pins the nit fix from MUL-2662
// review round 2: `multica project resource update <p> <r> --default-branch-hint x`
// must rebuild the full github_repo payload by merging the existing `url` —
// otherwise the server sees `{default_branch_hint: "x"}` and 400s.
func TestBuildResourceRefFromFlagsGithubMergesHint(t *testing.T) {
	t.Run("hint-only edit preserves existing url", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("default-branch-hint", "main")
		existing := map[string]any{"url": "https://github.com/multica-ai/multica"}

		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true when default-branch-hint is set")
		}
		if ref["url"] != "https://github.com/multica-ai/multica" {
			t.Errorf("expected merged url, got %v", ref["url"])
		}
		if ref["default_branch_hint"] != "main" {
			t.Errorf("expected merged hint=main, got %v", ref["default_branch_hint"])
		}
	})

	t.Run("hint=empty clears the hint but keeps url", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("default-branch-hint", "")
		existing := map[string]any{
			"url":                 "https://github.com/multica-ai/multica",
			"default_branch_hint": "stale",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if ref["url"] != "https://github.com/multica-ai/multica" {
			t.Errorf("expected url to survive empty-hint clear, got %v", ref["url"])
		}
		if _, ok := ref["default_branch_hint"]; ok {
			t.Errorf("expected default_branch_hint to be cleared, got %v", ref["default_branch_hint"])
		}
	})

	t.Run("url override survives merge", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("url", "https://github.com/multica-ai/new-repo")
		existing := map[string]any{
			"url":                 "https://github.com/multica-ai/multica",
			"default_branch_hint": "main",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if ref["url"] != "https://github.com/multica-ai/new-repo" {
			t.Errorf("expected overridden url, got %v", ref["url"])
		}
		if ref["default_branch_hint"] != "main" {
			t.Errorf("expected merged hint to persist, got %v", ref["default_branch_hint"])
		}
	})

	t.Run("checkout ref edit preserves existing url and hint", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref", "release/v2")
		existing := map[string]any{
			"url":                 "https://github.com/multica-ai/multica",
			"default_branch_hint": "main",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if ref["url"] != "https://github.com/multica-ai/multica" {
			t.Errorf("expected merged url, got %v", ref["url"])
		}
		if ref["default_branch_hint"] != "main" {
			t.Errorf("expected merged hint to persist, got %v", ref["default_branch_hint"])
		}
		if ref["ref"] != "release/v2" {
			t.Errorf("expected checkout ref release/v2, got %v", ref["ref"])
		}
	})

	t.Run("empty checkout ref clears existing ref", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref", "")
		existing := map[string]any{
			"url": "https://github.com/multica-ai/multica",
			"ref": "stale",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if _, ok := ref["ref"]; ok {
			t.Errorf("expected checkout ref to be cleared, got %v", ref["ref"])
		}
		if ref["url"] != "https://github.com/multica-ai/multica" {
			t.Errorf("expected merged url, got %v", ref["url"])
		}
	})

	t.Run("hint-only with no existing url fails fast", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("default-branch-hint", "main")
		_, _, err := buildResourceRefFromFlags(cmd, "github_repo", nil)
		if err == nil {
			t.Fatalf("expected error when no existing url is available to merge")
		}
	})

	t.Run("no flags set returns has=false", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		ref, has, err := buildResourceRefFromFlags(cmd, "github_repo", map[string]any{"url": "https://x"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if has {
			t.Errorf("expected has=false when no shortcut flag is set, got ref=%v", ref)
		}
	})
}

func TestBuildResourceRefFromRefFlagKeepsJSONEscapeHatch(t *testing.T) {
	t.Run("json payload wins over github shortcuts", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("url", "https://github.com/multica-ai/ignored")
		_ = cmd.Flags().Set("ref", `{"url":"https://github.com/multica-ai/multica","ref":"release/v2"}`)

		ref, has, err := buildResourceRefFromRefFlag(cmd, "github_repo", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		payload, ok := ref.(map[string]any)
		if !ok {
			t.Fatalf("expected JSON object payload, got %T", ref)
		}
		if payload["url"] != "https://github.com/multica-ai/multica" {
			t.Errorf("json payload url = %v", payload["url"])
		}
		if payload["ref"] != "release/v2" {
			t.Errorf("json payload ref = %v", payload["ref"])
		}
	})

	t.Run("broken json is still rejected", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("url", "https://github.com/multica-ai/multica")
		_ = cmd.Flags().Set("ref", `{"url":`)

		_, _, err := buildResourceRefFromRefFlag(cmd, "github_repo", nil)
		if err == nil {
			t.Fatalf("expected invalid JSON error")
		}
		if !strings.Contains(err.Error(), "not valid JSON") {
			t.Fatalf("error = %q, want JSON guidance", err)
		}
	})

	// A github_repo checkout ref that happens to be valid JSON as a bare scalar
	// (a numeric tag, an all-digit short SHA, true/false/null) must be treated
	// as a checkout ref, not silently parsed into the JSON escape hatch. Only
	// "{...}" / "[...]" shaped values are the escape hatch.
	t.Run("bare scalar github ref is a checkout ref, not JSON", func(t *testing.T) {
		for _, gitRef := range []string{"2024", "1234567", "true", "null"} {
			cmd := newProjectResourceUpdateTestCmd()
			_ = cmd.Flags().Set("url", "https://github.com/multica-ai/multica")
			_ = cmd.Flags().Set("ref", gitRef)

			ref, has, err := buildResourceRefFromRefFlag(cmd, "github_repo", nil)
			if err != nil {
				t.Fatalf("ref %q: unexpected error: %v", gitRef, err)
			}
			if !has {
				t.Fatalf("ref %q: expected has=true", gitRef)
			}
			payload, ok := ref.(map[string]any)
			if !ok {
				t.Fatalf("ref %q: expected github_repo shortcut map, got %T", gitRef, ref)
			}
			if payload["url"] != "https://github.com/multica-ai/multica" {
				t.Errorf("ref %q: url = %v", gitRef, payload["url"])
			}
			if payload["ref"] != gitRef {
				t.Errorf("ref %q: checkout ref = %v, want %q", gitRef, payload["ref"], gitRef)
			}
		}
	})

	// The checkout-ref shortcut only applies to github_repo: every other
	// resource type must still reject a non-JSON --ref so the generic escape
	// hatch stays typed.
	t.Run("non-json ref rejected for non-github type", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref", "develop")

		_, _, err := buildResourceRefFromRefFlag(cmd, "local_directory", nil)
		if err == nil {
			t.Fatalf("expected error for non-JSON --ref on local_directory")
		}
		if !strings.Contains(err.Error(), "not valid JSON") {
			t.Fatalf("error = %q, want JSON guidance", err)
		}
	})
}

// TestBuildResourceRefFromFlagsLocalDirectoryMerges covers the same merge
// behavior for local_directory: partial edits keep unmentioned fields from the
// existing ref.
func TestBuildResourceRefFromFlagsLocalDirectoryMerges(t *testing.T) {
	t.Run("ref-label only edit preserves existing path + daemon", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref-label", "renamed")
		existing := map[string]any{
			"local_path": "/Users/foo/work/a",
			"daemon_id":  "d1",
			"label":      "old",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "local_directory", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if ref["local_path"] != "/Users/foo/work/a" {
			t.Errorf("local_path missing after merge: %v", ref["local_path"])
		}
		if ref["daemon_id"] != "d1" {
			t.Errorf("daemon_id missing after merge: %v", ref["daemon_id"])
		}
		if ref["label"] != "renamed" {
			t.Errorf("label not overridden: %v", ref["label"])
		}
	})

	t.Run("local-path only without existing daemon fails", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("local-path", "/Users/foo/work/b")
		_, _, err := buildResourceRefFromFlags(cmd, "local_directory", nil)
		if err == nil {
			t.Fatalf("expected error when daemon_id is missing from both flags and existing ref")
		}
	})

	t.Run("ref-label cleared on empty input", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref-label", "")
		existing := map[string]any{
			"local_path": "/Users/foo/work/a",
			"daemon_id":  "d1",
			"label":      "to-clear",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "local_directory", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatalf("expected has=true")
		}
		if _, ok := ref["label"]; ok {
			t.Errorf("expected embedded label to be cleared, got %v", ref["label"])
		}
	})
}

// Switching an existing resource between in_place and worktree is a one-flag
// edit, and an unrelated edit must not silently reset the mode — that would
// drop a user back to serialised tasks without telling them.
func TestBuildResourceRefFromFlagsLocalDirectoryExecutionMode(t *testing.T) {
	t.Run("sets worktree mode", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("execution-mode", "worktree")
		existing := map[string]any{"local_path": "/Users/foo/work/a", "daemon_id": "d1"}
		ref, has, err := buildResourceRefFromFlags(cmd, "local_directory", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatal("expected has=true")
		}
		if ref["execution_mode"] != "worktree" {
			t.Errorf("execution_mode = %v, want worktree", ref["execution_mode"])
		}
	})

	t.Run("unrelated edit preserves existing mode", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("ref-label", "renamed")
		existing := map[string]any{
			"local_path":     "/Users/foo/work/a",
			"daemon_id":      "d1",
			"execution_mode": "worktree",
		}
		ref, _, err := buildResourceRefFromFlags(cmd, "local_directory", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref["execution_mode"] != "worktree" {
			t.Errorf("execution_mode lost on an unrelated edit: %v", ref["execution_mode"])
		}
	})

	t.Run("empty value clears back to the default", func(t *testing.T) {
		cmd := newProjectResourceUpdateTestCmd()
		_ = cmd.Flags().Set("execution-mode", "")
		existing := map[string]any{
			"local_path":     "/Users/foo/work/a",
			"daemon_id":      "d1",
			"execution_mode": "worktree",
		}
		ref, has, err := buildResourceRefFromFlags(cmd, "local_directory", existing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !has {
			t.Fatal("expected has=true")
		}
		if _, ok := ref["execution_mode"]; ok {
			t.Errorf("expected execution_mode cleared, got %v", ref["execution_mode"])
		}
	})
}

func newProjectUpdateRevisionTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "update", RunE: runProjectUpdate, Args: exactArgs(1), SilenceUsage: true, SilenceErrors: true}
	addCommonProfileFlags(cmd)
	for _, name := range []string{"title", "description", "status", "icon", "lead", "start-date", "due-date"} {
		cmd.Flags().String(name, "", "")
	}
	cmd.Flags().Int64("expected-description-revision", 0, "")
	cmd.Flags().Int64("expected-revision", 0, "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func TestProjectUpdateRevisionFlagsRegistered(t *testing.T) {
	for _, name := range []string{"expected-description-revision", "expected-revision"} {
		flag := projectUpdateCmd.Flags().Lookup(name)
		if flag == nil || flag.Value.Type() != "int64" {
			t.Errorf("project update must expose int64 --%s", name)
		}
	}
}

func TestRunProjectUpdateExplicitRevisions(t *testing.T) {
	const projectID = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name  string
		flags []string
		want  map[string]string
	}{
		{"description with both versions", []string{"--description", "Reviewed text", "--expected-description-revision", "4", "--expected-revision", "9"}, map[string]string{"description": `"Reviewed text"`, "expected_description_revision": "4", "expected_revision": "9"}},
		{"clear description", []string{"--description=", "--expected-description-revision", "2"}, map[string]string{"description": `""`, "expected_description_revision": "2"}},
		{"legacy description", []string{"--description", "Old client edit"}, map[string]string{"description": `"Old client edit"`}},
		{"other fields unchanged", []string{"--title", "Renamed", "--status", "paused", "--start-date", "2026-10-05", "--due-date=", "--expected-revision", "7"}, map[string]string{"title": `"Renamed"`, "status": `"paused"`, "start_date": `"2026-10-05"`, "due_date": `""`, "expected_revision": "7"}},
		{"safe maximum", []string{"--description", "Boundary", "--expected-description-revision", "9007199254740991", "--expected-revision", "9007199254740991"}, map[string]string{"description": `"Boundary"`, "expected_description_revision": "9007199254740991", "expected_revision": "9007199254740991"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var requests []string
			var body map[string]json.RawMessage
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				if r.Method != http.MethodPut || r.URL.Path != "/api/projects/"+projectID {
					w.WriteHeader(500)
					return
				}
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					t.Errorf("decode update: %v", e)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"` + projectID + `"}`))
			}))
			defer server.Close()
			setCLITestServerEnv(t, server.URL)
			cmd := newProjectUpdateRevisionTestCmd()
			cmd.SetArgs(append([]string{projectID}, tc.flags...))
			if _, err := captureStdout(t, cmd.Execute); err != nil {
				t.Fatal(err)
			}
			if len(requests) != 1 || requests[0] != "PUT /api/projects/"+projectID {
				t.Fatalf("must send one update with no version GET: %v", requests)
			}
			if len(body) != len(tc.want) {
				t.Fatalf("body has unexpected keys: %s", body)
			}
			for key, want := range tc.want {
				if string(body[key]) != want {
					t.Errorf("body[%s]=%s, want %s", key, body[key], want)
				}
			}
		})
	}
}

func TestRunProjectUpdateRejectsInvalidRevisionFlags(t *testing.T) {
	t.Chdir(t.TempDir())
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	for _, name := range []string{"expected-description-revision", "expected-revision"} {
		for _, value := range []string{"0", "-1", "9007199254740992", "9223372036854775807", "9223372036854775808", "1.5", "abc", ""} {
			t.Run(name+"/"+value, func(t *testing.T) {
				cmd := newProjectUpdateRevisionTestCmd()
				cmd.SetArgs([]string{"11111111-1111-4111-8111-111111111111", "--description", "edit", "--" + name + "=" + value})
				err := cmd.Execute()
				if err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("invalid --%s=%q should report usage: %v", name, value, err)
				}
			})
		}
	}
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"--expected-description-revision=1"}, "--description"},
		{[]string{"--title=rename", "--expected-description-revision=1"}, "--description"},
		{[]string{"--expected-revision=1"}, "no fields to update"},
	} {
		cmd := newProjectUpdateRevisionTestCmd()
		cmd.SetArgs(append([]string{"11111111-1111-4111-8111-111111111111"}, tc.args...))
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("flags %v: %v", tc.args, err)
		}
	}
	if requests != 0 {
		t.Fatalf("local revision usage errors made %d requests", requests)
	}
}

func TestRunProjectUpdateConflictDoesNotRefreshOrRetry(t *testing.T) {
	const projectID = "11111111-1111-4111-8111-111111111111"
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict, http.StatusPreconditionRequired} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Chdir(t.TempDir())
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"read the project description revision before editing","code":"description_revision_required","current":{"description_revision":42}}`))
			}))
			defer server.Close()
			setCLITestServerEnv(t, server.URL)
			cmd := newProjectUpdateRevisionTestCmd()
			args := []string{projectID, "--description", "Attempt"}
			if status != http.StatusPreconditionRequired {
				args = append(args, "--expected-description-revision=2")
			}
			cmd.SetArgs(args)
			out, err := captureStdout(t, cmd.Execute)
			var apiErr *cli.HTTPError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
				t.Fatalf("must preserve HTTP %d: %v", status, err)
			}
			if requests != 1 || out != "" {
				t.Fatalf("rejected update refreshed, retried or printed success: %d requests, %q", requests, out)
			}
			if status == http.StatusPreconditionRequired && !strings.Contains(cli.FormatError(err, false), "--expected-description-revision") {
				t.Fatalf("428 must explain the missing revision in normal CLI output: %s", cli.FormatError(err, false))
			}

		})
	}
}

func TestRunProjectUpdatePrefixResolutionDoesNotSupplyVersions(t *testing.T) {
	const projectID = "11111111-1111-4111-8111-111111111111"
	for _, explicit := range []bool{false, true} {
		t.Run(strconv.FormatBool(explicit), func(t *testing.T) {
			t.Chdir(t.TempDir())
			var requests []string
			var body map[string]json.RawMessage
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && r.URL.Path == "/api/projects" {
					_, _ = w.Write([]byte(`{"projects":[{"id":"` + projectID + `","title":"Project","description_revision":42,"revision":99}]}`))
					return
				}
				if r.Method == http.MethodPut && r.URL.Path == "/api/projects/"+projectID {
					_ = json.NewDecoder(r.Body).Decode(&body)
					_, _ = w.Write([]byte(`{"id":"` + projectID + `"}`))
					return
				}
				w.WriteHeader(500)
			}))
			defer server.Close()
			setCLITestServerEnv(t, server.URL)
			cmd := newProjectUpdateRevisionTestCmd()
			args := []string{"11111111", "--description", "Reviewed separately"}
			if explicit {
				args = append(args, "--expected-description-revision=3", "--expected-revision=7")
			}
			cmd.SetArgs(args)
			if _, err := captureStdout(t, cmd.Execute); err != nil {
				t.Fatal(err)
			}
			if len(requests) != 2 || requests[0] != "GET /api/projects" || requests[1] != "PUT /api/projects/"+projectID {
				t.Fatalf("unexpected lookup/retry: %v", requests)
			}
			if explicit {
				if string(body["expected_description_revision"]) != "3" || string(body["expected_revision"]) != "7" {
					t.Fatalf("replaced caller versions with lookup result: %s", body)
				}
			} else if _, ok := body["expected_description_revision"]; ok {
				t.Fatalf("invented description token from lookup: %s", body)
			} else if _, ok := body["expected_revision"]; ok {
				t.Fatalf("invented project token from lookup: %s", body)
			}
		})
	}
}
