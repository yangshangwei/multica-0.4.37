package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/spf13/cobra"
)

const (
	materialTestProject = "10000000-0000-4000-8000-000000000001"
	materialTestID      = "10000000-0000-4000-8000-000000000002"
	materialTestWS      = "10000000-0000-4000-8000-000000000003"
	materialTestKey     = "10000000-0000-4000-8000-000000000004"
	materialTestRev     = "10000000-0000-4000-8000-000000000005"
	materialTestSource  = "10000000-0000-4000-8000-000000000006"
)

func materialTestInput() material.RevisionInput {
	return material.RevisionInput{RequestID: materialTestKey, ExpectedVersion: 2, Kind: "requirement",
		Content:            "Users can refund an order and receive a traceable result.",
		Sources:            []material.Source{{ResourceID: materialTestSource, Commit: strings.Repeat("a", 40), Path: "src/refund.go"}},
		AcceptanceCriteria: []string{"AC1: A refund has one durable receipt"}, BlockingQuestions: []string{}, Ready: true}
}

func materialTestDetail() material.Detail {
	input := materialTestInput()
	at := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	return material.Detail{
		Material:  material.Material{ID: materialTestID, WorkspaceID: materialTestWS, ProjectID: materialTestProject, Title: "Refunds", Path: "requirements/REQ-1/prd.md", Version: 3, Status: "publishing", CreatedAt: at, UpdatedAt: at},
		Revisions: []material.Revision{{ID: materialTestRev, Kind: input.Kind, Content: input.Content, ContentSHA256: material.Digest(input.Content), Sources: input.Sources, AcceptanceCriteria: input.AcceptanceCriteria, BlockingQuestions: input.BlockingQuestions, PublicationStatus: "publishing", CreatedAt: at}},
		Decisions: []material.Decision{}, Comments: []material.Comment{}, Capabilities: material.Capabilities{CanEdit: true},
	}
}

func materialTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func materialTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	t.Chdir(t.TempDir())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("MULTICA_SERVER_URL", server.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", materialTestWS)
	t.Setenv("MULTICA_TOKEN", "mul_test_material")
	return server
}

func runMaterialTest(t *testing.T, input []byte, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceErrors: true, SilenceUsage: true}
	addCommonProfileFlags(root)
	project := &cobra.Command{Use: "project"}
	project.AddCommand(newProjectMaterialCommand())
	root.AddCommand(project)
	root.SetArgs(append([]string{"project", "material"}, args...))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetIn(bytes.NewReader(input))
	err := root.Execute()
	return out.String(), err
}

func TestProjectMaterialRegistered(t *testing.T) {
	group := commandChild(projectCmd, "material")
	if group == nil {
		t.Fatal("project material command missing")
	}
	for _, name := range []string{"list", "get", "create", "revise"} {
		if commandChild(group, name) == nil {
			t.Fatalf("project material %s missing", name)
		}
	}
	for _, name := range []string{"approve", "configure", "start"} {
		if commandChild(group, name) != nil {
			t.Fatalf("unexpected authority command %s", name)
		}
	}
}

func TestProjectMaterialReadScopesAndJSON(t *testing.T) {
	for _, action := range []string{"list", "get"} {
		t.Run(action, func(t *testing.T) {
			var calls atomic.Int32
			materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				wantPath := "/api/projects/" + materialTestProject + "/materials"
				if action == "get" {
					wantPath += "/" + materialTestID
				}
				if r.Method != http.MethodGet || r.URL.Path != wantPath || r.Header.Get("X-Workspace-ID") != materialTestWS || r.Header.Get("Authorization") != "Bearer mul_test_material" {
					t.Errorf("wrong request scope: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if action == "list" {
					_ = json.NewEncoder(w).Encode(material.List{WorkspaceID: materialTestWS, ProjectID: materialTestProject, Materials: []material.Material{materialTestDetail().Material}})
				} else {
					_ = json.NewEncoder(w).Encode(materialTestDetail())
				}
			})
			args := []string{action, materialTestProject}
			if action == "get" {
				args = append(args, materialTestID)
			}
			out, err := runMaterialTest(t, nil, args...)
			if err != nil || !json.Valid([]byte(out)) || !strings.Contains(out, materialTestID) || calls.Load() != 1 {
				t.Fatalf("read result out=%q calls=%d err=%v", out, calls.Load(), err)
			}
		})
	}
}

func TestProjectMaterialCreateAndExplicitReplay(t *testing.T) {
	var bodies [][]byte
	var keys []string
	materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/projects/"+materialTestProject+"/materials" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		var request material.CreateInput
		if json.Unmarshal(body, &request) != nil || request.RequestID != materialTestKey || request.Title != "Refunds" {
			t.Error("wrong create input")
		}
		_ = json.NewEncoder(w).Encode(materialTestDetail())
	})
	args := []string{"create", materialTestProject, "--title", "Refunds", "--request-id", materialTestKey}
	for range 2 {
		out, err := runMaterialTest(t, nil, args...)
		if err != nil || !json.Valid([]byte(out)) {
			t.Fatalf("create failed: %v %q", err, out)
		}
	}
	if len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) || keys[0] != materialTestKey || keys[1] != materialTestKey {
		t.Fatal("explicit replay changed the original identity/body")
	}
}

func TestProjectMaterialRevisionFileAndStdin(t *testing.T) {
	for _, fromFile := range []bool{false, true} {
		t.Run(fmtBool(fromFile), func(t *testing.T) {
			var calls atomic.Int32
			materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/projects/"+materialTestProject+"/materials/"+materialTestID+"/revisions" || r.Header.Get("Idempotency-Key") != materialTestKey {
					t.Errorf("wrong revision request %s %s", r.Method, r.URL.Path)
				}
				var input material.RevisionInput
				if json.NewDecoder(r.Body).Decode(&input) != nil || !bytes.Equal(materialTestJSON(t, input), materialTestJSON(t, materialTestInput())) {
					t.Error("revision input changed")
				}
				_ = json.NewEncoder(w).Encode(materialTestDetail())
			})
			raw := materialTestJSON(t, materialTestInput())
			file := "-"
			if fromFile {
				file = filepath.Join(t.TempDir(), "revision.json")
				if err := os.WriteFile(file, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := runMaterialTest(t, raw, "revise", materialTestProject, materialTestID, "--file", file)
			if err != nil || !json.Valid([]byte(out)) || calls.Load() != 1 {
				t.Fatalf("revision failed calls=%d err=%v out=%q", calls.Load(), err, out)
			}
		})
	}
}

func fmtBool(value bool) string {
	if value {
		return "file"
	}
	return "stdin"
}

func TestProjectMaterialInvalidInputMakesNoRequest(t *testing.T) {
	var calls atomic.Int32
	materialTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	valid := string(materialTestJSON(t, materialTestInput()))
	tests := map[string]string{
		"null": "null", "array": "[]", "trailing": valid + `{}`, "trailing garbage": valid + `x`,
		"unknown":         strings.TrimSuffix(valid, "}") + `,"approve":true}`,
		"case alias":      strings.Replace(valid, `"ready"`, `"Ready"`, 1),
		"duplicate":       strings.TrimSuffix(valid, "}") + `,"ready":false}`,
		"null array":      strings.Replace(valid, `"blocking_questions":[]`, `"blocking_questions":null`, 1),
		"missing array":   strings.Replace(valid, `"blocking_questions":[],`, "", 1),
		"null element":    strings.Replace(valid, `"blocking_questions":[]`, `"blocking_questions":[null]`, 1),
		"null scalar":     strings.Replace(valid, `"ready":true`, `"ready":null`, 1),
		"bad version":     strings.Replace(valid, `"expected_version":2`, `"expected_version":0`, 1),
		"unknown nested":  strings.Replace(valid, `"path":"src/refund.go"`, `"path":"src/refund.go","token":"forbidden"`, 1),
		"invalid SHA":     strings.Replace(valid, strings.Repeat("a", 40), "main", 1),
		"prefix resource": strings.Replace(valid, materialTestSource, "resource", 1),
		"input limit":     strings.Repeat(" ", materialCLIInputLimit+1),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			out, err := runMaterialTest(t, []byte(raw), "revise", materialTestProject, materialTestID, "--file", "-")
			if err == nil || out != "" {
				t.Fatalf("invalid input accepted err=%v out=%q", err, out)
			}
		})
	}
	for _, args := range [][]string{
		{"list", "project-prefix"}, {"get", materialTestProject, "00000000-0000-0000-0000-000000000000"},
		{"create", materialTestProject, "--title", "Refunds"}, {"create", materialTestProject, "--request-id", materialTestKey},
		{"revise", materialTestProject, materialTestID},
	} {
		if out, err := runMaterialTest(t, nil, args...); err == nil || out != "" {
			t.Fatalf("invalid flags accepted: %v", args)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid input made %d requests", calls.Load())
	}
}

func TestProjectMaterialMalformedSuccessIsNotPrinted(t *testing.T) {
	valid := string(materialTestJSON(t, materialTestDetail()))
	for name, raw := range map[string]string{
		"null": "null", "missing": `{}`, "trailing": valid + `{}`, "foreign project": strings.Replace(valid, materialTestProject, materialTestSource, 1),
		"foreign material":     strings.Replace(valid, materialTestID, materialTestSource, 1),
		"wrong workspace":      strings.Replace(valid, materialTestWS, materialTestSource, 1),
		"null revisions":       strings.Replace(valid, `"revisions":[`, `"revisions":null,"ignored":[`, 1),
		"missing capabilities": strings.Replace(valid, `"can_review":false,`, "", 1),
		"wrong digest":         strings.Replace(valid, material.Digest(materialTestInput().Content), strings.Repeat("f", 64), 1),
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			materialTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = io.WriteString(w, raw) })
			out, err := runMaterialTest(t, nil, "get", materialTestProject, materialTestID)
			if err == nil || out != "" || calls.Load() != 1 {
				t.Fatalf("malformed success accepted/retried: %q %v calls=%d", out, err, calls.Load())
			}
		})
	}
}

func TestProjectMaterialNoAutomaticRetryOrRedirect(t *testing.T) {
	for _, status := range []int{http.StatusConflict, http.StatusServiceUnavailable, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/unexpected-second-request")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"code":"original_request_unconfirmed"}`)
			})
			_ = server
			out, err := runMaterialTest(t, nil, "create", materialTestProject, "--title", "Refunds", "--request-id", materialTestKey)
			var response *cli.HTTPError
			if !errors.As(err, &response) || response.StatusCode != status || out != "" || calls.Load() != 1 {
				t.Fatalf("unexpected retry/result: %q %v calls=%d", out, err, calls.Load())
			}
		})
	}
}

func TestProjectMaterialErrorDoesNotEchoPrivateResponse(t *testing.T) {
	const private = "internal requirement text echoed by the proxy"
	materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, private)
	})
	out, err := runMaterialTest(t, nil, "create", materialTestProject, "--title", "Refunds", "--request-id", materialTestKey)
	var response *cli.HTTPError
	if !errors.As(err, &response) || response.StatusCode != http.StatusBadGateway {
		t.Fatal("could not classify the failed mutation")
	}
	if out != "" || strings.Contains(err.Error(), private) || strings.Contains(response.Body, private) {
		t.Fatal("private response reached CLI diagnostics")
	}
}

func TestProjectMaterialOrdinaryTaskAndCollaborationBoundaries(t *testing.T) {
	var calls atomic.Int32
	materialTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer mat_test_material" || r.Header.Get("X-Agent-ID") != materialTestSource || r.Header.Get("X-Task-ID") != materialTestKey {
			t.Error("ordinary task attribution lost")
		}
		_ = json.NewEncoder(w).Encode(materialTestDetail())
	})
	t.Setenv("MULTICA_AGENT_ID", materialTestSource)
	t.Setenv("MULTICA_TASK_ID", materialTestKey)
	t.Setenv("MULTICA_TOKEN", "mat_test_material")
	if _, err := runMaterialTest(t, nil, "get", materialTestProject, materialTestID); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cli.CollaborationContextEnv, "present")
	out, err := runMaterialTest(t, nil, "get", materialTestProject, materialTestID)
	if err == nil || out != "" || calls.Load() != 1 || !strings.Contains(err.Error(), "collaboration") {
		t.Fatalf("collaboration escaped ordinary client boundary: %q %v", out, err)
	}
}
