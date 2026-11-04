//go:build agentintegration && !windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type rk1kSchedule struct {
	Name       string
	Entries    []string
	Starts     int
	SeedStarts int
}

var rk1kSchedules = []rk1kSchedule{
	{"RK1-E0b", []string{"ordinary-baseline"}, 1, 0},
	{"RK1-K1", []string{"pending", "approved", "corrected", "withdrawn"}, 4, 0},
	{"RK1-K2", []string{"native-fresh", "native-resume", "frozen-refusal"}, 2, 0},
	{"RK1-K3", []string{"issue", "comment", "chat", "quick_create", "autopilot", "root", "member", "continuation"}, 8, 3},
}

// This sibling has no paid registration. Runtime25 and original14 cannot
// become a knowledge allocation through environment variables.
func rk1kRealAdmission() error {
	return errors.New("knowledge allocation is 0; paid source is unbound and registration is not runnable")
}

func TestRK1KnowledgeRealModel(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("real knowledge entry requires separate explicit authorization")
	}
	// Keep this before executable/version/account/profile discovery.
	if err := rk1kRealAdmission(); err != nil {
		t.Fatal(err)
	}
}

func TestRK1KnowledgeEntryGuard(t *testing.T) {
	if rk1kRealAdmission() == nil {
		t.Fatal("unallocated knowledge accepted real admission")
	}
	if len(rk1kSchedules) != 4 || rk1kSchedules[0].Name != "RK1-E0b" ||
		rk1kSchedules[1].Starts != 4 || rk1kSchedules[2].Starts != 2 || rk1kSchedules[3].Starts != 8 ||
		rk1kSchedules[3].SeedStarts != 3 {
		t.Fatal("fixed measured/seed schedule changed")
	}
	want := []string{"issue", "comment", "chat", "quick_create", "autopilot", "root", "member", "continuation"}
	if !slices.Equal(rk1kSchedules[3].Entries, want) {
		t.Fatal("K3 producer/role order changed")
	}
	id := uuid.NewString()
	task := rk1kIdentity{Kind: "task", TaskID: id, Workspace: uuid.NewString(), Agent: uuid.NewString()}
	if !task.valid() || task.key() == id {
		t.Fatal("ordinary task lost its separate namespace")
	}
	task.RunID = id
	if task.valid() {
		t.Fatal("ordinary task accepted a fabricated run identity")
	}
}

func rk1kLinkSelf(t *testing.T, dir, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.Link(self, path); err != nil {
		raw, err := os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func rk1kGuardFrame(t *testing.T) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []map[string]string{{"type": "text", "text": "Guard input without private material"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestRK1KnowledgeWrapperGuard(t *testing.T) {
	for _, test := range []struct {
		name, change string
		mode         string
		want         int
	}{
		{"ordinary_capture", "", "fake", 0},
		{"marker_missing", "marker", "fake", 26},
		{"identity_missing", "task", "fake", 27},
		{"identity_noncanonical", "uuid", "fake", 27},
		{"identity_ambiguous", "run", "fake", 27},
		{"credential_wrong_purpose", "token", "fake", 27},
		{"no_installed_real_target", "real", "real", 41},
		{"passthrough_standin", "", "real", 0},
		{"standin_symlink_refused", "symlink", "real", 41},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			provider := rk1kLinkSelf(t, dir, rk1kProviderName)
			task := uuid.NewString()
			env := map[string]string{"PATH": "/usr/bin:/bin", "HOME": dir, "DATABASE_URL": "",
				rk1kPlanDirEnv: dir, "MULTICA_TASK_ID": task, "MULTICA_TOKEN": "mat_test_created_guard_token",
				"MULTICA_WORKSPACE_ID": uuid.NewString(), "MULTICA_AGENT_ID": uuid.NewString(),
				"MULTICA_QUICK_CREATE_TASK_ID": task,
				"MULTICA_SERVER_URL":           "http://127.0.0.1:1", "RK1K_PRIVATE_HINT": "must-not-reach-standin"}
			plan := rk1Plan{Mode: test.mode, Behavior: "guard", Params: map[string]string{}}
			if test.mode == "real" {
				plan.Real = rk1kLinkSelf(t, dir, rk1kStandInName)
			}
			switch test.change {
			case "marker":
				delete(env, rk1kPlanDirEnv)
			case "task":
				delete(env, "MULTICA_TASK_ID")
			case "uuid":
				env["MULTICA_TASK_ID"] = "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"
			case "run":
				env["MULTICA_COLLABORATION_RUN_ID"] = uuid.NewString()
			case "token":
				env["MULTICA_TOKEN"] = "mdt_test_created_guard_token"
			case "real":
				plan.Real = filepath.Join(dir, "missing-ambient-claude")
			case "symlink":
				if err := os.Remove(plan.Real); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(provider, plan.Real); err != nil {
					t.Fatal(err)
				}
			}
			if err := rk1WriteJSON(filepath.Join(dir, "task-"+task+".json"), plan); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), provider, "-p", "--input-format", "stream-json")
			for key, value := range env {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			cmd.Stdin = bytes.NewReader(rk1kGuardFrame(t))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != test.want {
				t.Fatalf("wrapper code=%d want=%d stderr_bytes=%d", code, test.want, stderr.Len())
			}
			dirs, err := filepath.Glob(filepath.Join(dir, "task-"+task, "launch-*"))
			if err != nil {
				t.Fatal(err)
			}
			if test.want != 0 {
				if len(output) != 0 || len(dirs) != 0 {
					t.Fatal("refused wrapper created a launch/output or entered TestMain")
				}
				return
			}
			if len(dirs) != 1 || !bytes.Contains(output, []byte(`"type":"result"`)) {
				t.Fatal("ordinary wrapper did not capture and finish exactly one fake launch")
			}
			raw, err := os.ReadFile(filepath.Join(dirs[0], "stdin-first.jsonl"))
			if err != nil || !bytes.Equal(raw, rk1kGuardFrame(t)) {
				t.Fatal("captured first frame changed")
			}
			var meta rk1kLaunchMeta
			raw, err = os.ReadFile(filepath.Join(dirs[0], "launch.json"))
			if err != nil || json.Unmarshal(raw, &meta) != nil || meta.Identity.TaskID != task ||
				meta.Identity.Kind != "task" || meta.Identity.RunID != "" || meta.PaidStarts != 0 || meta.FakeStarts != 1 ||
				!meta.ResultSeen || !rk1kUUID(meta.NativeID) {
				t.Fatal("wrapper relabeled ordinary identity or start accounting")
			}
			if test.mode == "real" {
				var keys []string
				raw, err := os.ReadFile(filepath.Join(dir, "task-"+task+".sim-env.json"))
				if err != nil || json.Unmarshal(raw, &keys) != nil {
					t.Fatal("pass-through child environment was not observed")
				}
				for _, key := range keys {
					if rk1RemovedKey(key) || strings.HasPrefix(key, "RK1K_") {
						t.Fatalf("test variable reached the stand-in: %s", key)
					}
				}
				for _, key := range []string{"MULTICA_TASK_ID", "MULTICA_TOKEN", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID", "MULTICA_SERVER_URL", "MULTICA_QUICK_CREATE_TASK_ID"} {
					if !slices.Contains(keys, key) {
						t.Fatalf("ordinary business environment lost %s", key)
					}
				}
			}
		})
	}
}

func TestRK1KnowledgePromptData(t *testing.T) {
	content := "A policy may mention Approved managed knowledge [without creating another JSON block]."
	source := uuid.NewString()
	raw, err := json.Marshal([]protocol.CollaborationKnowledgeContribution{{
		CandidateID: uuid.NewString(), Version: "1", ContentSHA256: protocol.CollaborationKnowledgeHash(content),
		Target:        protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: uuid.NewString(), ProjectID: uuid.NewString()},
		SourceIssueID: source, Summary: "Source-backed parser control", Content: content,
		Sources:  []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: source, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("Source body")}},
		Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, prompt string
		want         string
		refused      bool
	}{
		{"ordinary_actual_header", "Accepted ordinary input\n\n" + rk1kOrdinaryKnowledgeMarker + string(raw) + "\n", content, false},
		{"host_actual_header", "Frozen handoff" + rk1kHostKnowledgeMarker + string(raw), content, false},
		{"ordinary_no_material", "Accepted input without a managed data block", "", false},
		{"bare_words_are_not_granted_data", "Approved managed knowledge " + string(raw), "", false},
		{"trailing_data_refused", rk1kHostKnowledgeMarker + string(raw) + " additional text", "", true},
		{"missing_array_refused", rk1kOrdinaryKnowledgeMarker, "", true},
		{"empty_host_array", rk1kHostKnowledgeMarker + "[]", "", false},
		{"null_array_refused", rk1kHostKnowledgeMarker + "null", "", true},
		{"ambiguous_blocks_refused", rk1kHostKnowledgeMarker + string(raw) + rk1kOrdinaryKnowledgeMarker + string(raw), "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			items, err := rk1kContributions(test.prompt)
			if test.refused {
				if err == nil {
					t.Fatal("malformed or ambiguous managed data accepted")
				}
				return
			}
			if err != nil || test.want == "" && len(items) != 0 || test.want != "" && (len(items) != 1 || items[0].Content != test.want) {
				t.Fatal("captured managed data differs from actual production input")
			}
		})
	}
}

func TestRK1KnowledgeTraceCredentialLabels(t *testing.T) {
	for _, test := range []struct{ name, authorization, cookie, want string }{
		{"JWT", "Bearer eyJhbGciOiJub25lIn0.eyJzdWIiOiJmaXh0dXJlIn0.syntheticsignature", "", "jwt"},
		{"unknown", "Bearer neverpublishthisfixturekey", "", "unknown"},
		{"cookie", "", "session=neverpublishfixturecookie", "cookie"},
		{"daemon", "Bearer mdt_test_created_fixture_token", "", "mdt"},
		{"task", "Bearer mat_test_created_fixture_token", "", "mat"},
		{"business", "Bearer mcb_test_created_fixture_token", "", "mcb"},
		{"none", "", "", "none"},
	} {
		t.Run(test.name, func(t *testing.T) {
			trace := &rk1kTrace{&rk1Trace{next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}}
			request := httptest.NewRequest(http.MethodPost, "http://fixture.invalid/api/fixture", nil)
			request.Header.Set("Authorization", test.authorization)
			request.Header.Set("Cookie", test.cookie)
			trace.ServeHTTP(httptest.NewRecorder(), request)
			calls := trace.snapshot()
			if len(calls) != 1 || calls[0].Credential != test.want {
				t.Fatal("trace credential must be a fixed purpose label")
			}
			raw, err := json.Marshal(calls)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{strings.TrimPrefix(test.authorization, "Bearer "), test.cookie} {
				if secret != "" && bytes.Contains(raw, []byte(secret)) {
					t.Fatal("trace persisted an authentication secret")
				}
			}
		})
	}
}

func TestRK1KnowledgeNativeMetadataUsesRawOutput(t *testing.T) {
	native := uuid.NewString()
	init := `{"type":"system","subtype":"init","session_id":"` + native + `"}` + "\n"
	result := `{"type":"result","session_id":"` + native + `","result":"DONE"}` + "\n"
	for _, test := range []struct {
		name, raw, wantID string
		wantResult        bool
	}{
		{"complete", init + result, native, true},
		{"missing_native", "{\"type\":\"result\",\"result\":\"DONE\"}\n", "", true},
		{"missing_result", init, native, false},
		{"invalid_raw", init + "not a JSON result", "", false},
		{"changed_native", init + strings.ReplaceAll(result, native, uuid.NewString()), "", true},
		{"duplicate_result", init + result + result, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			observed, resultSeen := rk1kObserveNativeResult([]byte(test.raw))
			if observed != test.wantID || resultSeen != test.wantResult {
				t.Fatal("native/result metadata was invented or lost")
			}
		})
	}
}
