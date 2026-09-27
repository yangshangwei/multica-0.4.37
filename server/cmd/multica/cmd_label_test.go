package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const testLabelUUID = "11111111-1111-1111-1111-111111111111"

func newLabelCreateTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "create"}
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("color", "", "")
	cmd.Flags().String("resource-type", "issue", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func newLabelListTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().String("resource-type", "", "")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().Bool("full-id", false, "")
	return cmd
}

func newLabelUpdateTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "update"}
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("color", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func newLabelDeleteTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "delete"}
	cmd.Flags().String("output", "json", "")
	return cmd
}

func setCLITestServerEnv(t *testing.T, serverURL string) {
	t.Helper()
	t.Setenv("MULTICA_SERVER_URL", serverURL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")
}

func TestRunLabelCreateSendsExpectedRequest(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/labels" {
			t.Fatalf("path = %q, want /api/labels", r.URL.Path)
		}
		if r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Fatalf("X-Workspace-ID = %q, want ws-1", r.Header.Get("X-Workspace-ID"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    testLabelUUID,
			"name":  body["name"],
			"color": body["color"],
		})
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)

	cmd := newLabelCreateTestCmd()
	_ = cmd.Flags().Set("name", "Bug")
	_ = cmd.Flags().Set("color", "#ef4444")

	out, err := captureStdout(t, func() error { return runLabelCreate(cmd, nil) })
	if err != nil {
		t.Fatalf("runLabelCreate: %v", err)
	}
	if body["name"] != "Bug" || body["color"] != "#ef4444" {
		t.Fatalf("body = %#v, want name/color", body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v\n%s", err, out)
	}
	if got["id"] != testLabelUUID || got["name"] != "Bug" {
		t.Fatalf("stdout = %#v, want created label", got)
	}
}

func TestRunLabelUpdateSendsOnlyChangedFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/labels/"+testLabelUUID {
			t.Fatalf("path = %q, want label path", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    testLabelUUID,
			"name":  body["name"],
			"color": "#3b82f6",
		})
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)

	cmd := newLabelUpdateTestCmd()
	_ = cmd.Flags().Set("name", "Feature")

	if _, err := captureStdout(t, func() error { return runLabelUpdate(cmd, []string{testLabelUUID}) }); err != nil {
		t.Fatalf("runLabelUpdate: %v", err)
	}
	if len(body) != 1 || body["name"] != "Feature" {
		t.Fatalf("body = %#v, want only name field", body)
	}
}

func TestRunLabelDeletePrintsJsonConfirmation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/labels/"+testLabelUUID {
			t.Fatalf("path = %q, want label path", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)

	cmd := newLabelDeleteTestCmd()
	_ = cmd.Flags().Set("output", "json")

	out, err := captureStdout(t, func() error { return runLabelDelete(cmd, []string{testLabelUUID}) })
	if err != nil {
		t.Fatalf("runLabelDelete: %v", err)
	}
	if !strings.Contains(out, `"deleted": true`) || !strings.Contains(out, testLabelUUID) {
		t.Fatalf("stdout = %q, want deleted JSON confirmation", out)
	}
}

func TestRunLabelCreateRequiresNameAndColor(t *testing.T) {
	cmd := newLabelCreateTestCmd()
	if err := runLabelCreate(cmd, nil); err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("runLabelCreate error = %v, want missing name", err)
	}

	_ = cmd.Flags().Set("name", "Bug")
	if err := runLabelCreate(cmd, nil); err == nil || !strings.Contains(err.Error(), "--color is required") {
		t.Fatalf("runLabelCreate error = %v, want missing color", err)
	}
}

func TestRunLabelCreateWithResourceType(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/labels" {
			t.Fatalf("path = %q, want /api/labels", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":            testLabelUUID,
			"name":          body["name"],
			"resource_type": body["resource_type"],
			"color":         body["color"],
		})
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)

	cmd := newLabelCreateTestCmd()
	_ = cmd.Flags().Set("name", "mattpocock")
	_ = cmd.Flags().Set("color", "#3b82f6")
	_ = cmd.Flags().Set("resource-type", "skill")

	out, err := captureStdout(t, func() error { return runLabelCreate(cmd, nil) })
	if err != nil {
		t.Fatalf("runLabelCreate: %v", err)
	}
	if body["name"] != "mattpocock" || body["color"] != "#3b82f6" || body["resource_type"] != "skill" {
		t.Fatalf("body = %#v, want name/color/resource_type", body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v\n%s", err, out)
	}
	if got["resource_type"] != "skill" || got["name"] != "mattpocock" {
		t.Fatalf("stdout = %#v, want created skill label", got)
	}
}

func TestRunLabelListWithResourceType(t *testing.T) {
	var queryResource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/labels" {
			t.Fatalf("path = %q, want /api/labels", r.URL.Path)
		}
		queryResource = r.URL.Query().Get("resource_type")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"labels": []map[string]any{
				{
					"id":            testLabelUUID,
					"name":          "frontend",
					"resource_type": queryResource,
					"color":         "#3b82f6",
				},
			},
		})
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)

	cmd := newLabelListTestCmd()
	_ = cmd.Flags().Set("resource-type", "skill")

	out, err := captureStdout(t, func() error { return runLabelList(cmd, nil) })
	if err != nil {
		t.Fatalf("runLabelList: %v", err)
	}
	if queryResource != "skill" {
		t.Fatalf("queryResource = %q, want skill", queryResource)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["resource_type"] != "skill" {
		t.Fatalf("stdout = %#v, want skill label list", got)
	}
}

func TestRunLabelUpdateDescription(t *testing.T) {
	for _, description := range []string{"Review helpers", ""} {
		t.Run(description, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/api/labels/"+testLabelUUID {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": testLabelUUID})
			}))
			defer srv.Close()
			setCLITestServerEnv(t, srv.URL)
			cmd := newLabelUpdateTestCmd()
			if err := cmd.Flags().Set("description", description); err != nil {
				t.Fatal(err)
			}
			if _, err := captureStdout(t, func() error { return runLabelUpdate(cmd, []string{testLabelUUID}) }); err != nil {
				t.Fatalf("runLabelUpdate: %v", err)
			}
			if len(body) != 1 || body["description"] != description {
				t.Fatalf("body = %#v, want only description %q", body, description)
			}
		})
	}
}

func TestRunIssueLabelCommandsKeepIssueScopedPrefixes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	issueID := "22222222-2222-2222-2222-222222222222"
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Error("issue label request lost its workspace")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labels":
			if r.URL.Query().Get("resource_type") != "issue" || r.URL.Query().Get("workspace_id") != "ws-1" {
				t.Errorf("label prefix lookup crossed resource/workspace scope: %s", r.URL.RawQuery)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/issues/"+issueID+"/labels":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["label_id"] != testLabelUUID {
				t.Errorf("attach body = %#v, decode error = %v", body, err)
			}
			methods = append(methods, r.Method)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/issues/"+issueID+"/labels/"+testLabelUUID:
			methods = append(methods, r.Method)
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/issues/"+issueID+"/labels":
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"labels": []map[string]any{{"id": testLabelUUID}}})
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)
	cmd := newSkillLabelTestCmd("label")
	if _, err := captureStdout(t, func() error { return runIssueLabelAdd(cmd, []string{issueID, "1111"}) }); err != nil {
		t.Fatalf("runIssueLabelAdd: %v", err)
	}
	if _, err := captureStdout(t, func() error { return runIssueLabelRemove(cmd, []string{issueID, "1111"}) }); err != nil {
		t.Fatalf("runIssueLabelRemove: %v", err)
	}
	if got := strings.Join(methods, ","); got != "POST,DELETE" {
		t.Fatalf("mutations = %s, want POST,DELETE", got)
	}
}
