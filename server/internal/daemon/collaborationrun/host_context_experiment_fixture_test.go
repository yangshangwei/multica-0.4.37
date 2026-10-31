//go:build agentintegration

package collaborationrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Only a test-created executable (this test binary) can enter this fixture.
// Real experiment commands never set this environment variable.
func init() {
	if mode := os.Getenv("COLLAB_CONTEXT_TEST_PROVIDER"); mode != "" {
		code := contextFakeProvider(mode)
		if code != 0 {
			fmt.Fprintln(os.Stderr, "context fixture failure", code)
		}
		os.Exit(code)
	}
}

func contextFakeProvider(mode string) int {
	raw, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		return 30
	}
	var input struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &input) != nil || len(input.Message.Content) != 1 {
		return 31
	}
	brief, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		return 32
	}
	if err := appendContextJSON(os.Getenv("COLLAB_CONTEXT_TEST_CAPTURE"), map[string]any{"prompt": input.Message.Content[0].Text, "brief": string(brief), "args": os.Args[1:]}); err != nil {
		return 33
	}
	started, startErr := os.ReadFile(os.Getenv("COLLAB_CONTEXT_TEST_SUMMARY"))
	if startErr != nil || !strings.Contains(string(started), `"status":"started"`) {
		return 36
	}
	if mode == "missing" {
		return 0
	}
	run, err := cli.ReadCollaborationContext()
	if err != nil || run == nil {
		return 34
	}
	view, err := run.Read(context.Background(), run.Client())
	if err != nil {
		return 35
	}
	if mode == "relay" && strings.Contains(view.Description, "A different agent on another machine") {
		update := protocol.CollaborationRunMemoryUpdate{RequestID: uuid.NewString(), BaseVersion: "0", Content: "relay facts from " + run.Identity.RunID}
		var receipt protocol.CollaborationRunMemoryReceipt
		if err := run.Client().PutJSON(context.Background(), run.Path("/memory"), update, &receipt); err != nil {
			return 37
		}
	}
	body, _ := json.Marshal(view)
	tool := map[string]any{"type": "assistant", "message": map[string]any{"model": "fixture", "content": []any{map[string]any{"type": "tool_use", "id": "context-read", "name": "Bash", "input": map[string]any{"command": "multica issue get"}}}}}
	_ = json.NewEncoder(os.Stdout).Encode(tool)
	if mode == "partial" {
		body = []byte(strings.Repeat("x", 70000))
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "context-read", "content": string(body)}}}})
	failed := mode == "failure"
	answer := "2026-11-14"
	if failed {
		answer = "fixture refusal"
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"system","subtype":"init","session_id":"`+hostFixtureSession+`"}`)
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "assistant", "message": map[string]any{"model": "fixture", "content": []any{map[string]any{"type": "text", "text": answer}}, "usage": map[string]int{"input_tokens": 17, "output_tokens": 3}}})
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "result", "is_error": failed, "session_id": hostFixtureSession, "result": answer})
	return 0
}

func contextFakeEnvironment(t *testing.T, mode string) (contextEnv, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "provider.jsonl")
	t.Setenv("COLLAB_CONTEXT_TEST_PROVIDER", mode)
	t.Setenv("COLLAB_CONTEXT_TEST_CAPTURE", capture)
	t.Setenv("COLLAB_HOST_TEST_MODE", "")
	env := contextEnv{provider: "claude", executable: executable, version: "fixture-1", cli: executable,
		out: filepath.Join(t.TempDir(), "summary.jsonl"), model: "fixture-model", thinking: "provider_default", serviceTier: "provider_default"}
	t.Setenv("COLLAB_CONTEXT_TEST_SUMMARY", env.out)
	return env, capture
}

func runContextFake(t *testing.T, mode string) (contextRecord, error, string) {
	t.Helper()
	env, capture := contextFakeEnvironment(t, mode)
	rec, err := env.run(t, t.Context(), contextScenario{Name: "E2a_window_fact_newest", Input: "Read the issue.", Comments: contextWindow(true)}, "1")
	return rec, err, capture
}

func TestContextExperimentPersistsActualFakePromptAndCompleteHTTP(t *testing.T) {
	rec, err, capture := runContextFake(t, "success")
	if err != nil {
		t.Fatalf("%v; outcome body: %s", err, rec.Body)
	}
	if rec.Status != "finished" || rec.Verdict != "pass" || rec.ContextReads != 1 {
		t.Fatalf("unexpected result: status=%s verdict=%s reads=%d", rec.Status, rec.Verdict, rec.ContextReads)
	}
	var observed struct {
		Prompt string   `json:"prompt"`
		Brief  string   `json:"brief"`
		Args   []string `json:"args"`
	}
	f, err := os.Open(capture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&observed); err != nil {
		t.Fatal(err)
	}
	if observed.Prompt != rec.Prompt || contextHash([]byte(observed.Prompt)) != rec.PromptHash {
		t.Fatal("recorded host prompt differs from provider stdin")
	}
	if observed.Brief != rec.BriefFile || !strings.Contains(observed.Brief, strings.TrimSpace(rec.Brief)) {
		t.Fatal("recorded brief differs from file visible to provider")
	}
	if !strings.Contains(strings.Join(observed.Args, " "), "--model fixture-model") {
		t.Fatal("requested model was not passed to the provider")
	}
	if rec.ObservedModel != nil || rec.ModelObservation != "unavailable_in_host_projection" {
		t.Fatal("requested model was misreported as an observed model")
	}
	events, err := os.ReadFile(rec.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"mcb_", "mdt_fixture", "termination_token\":\"mct_"} {
		if strings.Contains(string(events), secret) {
			t.Fatal("credential persisted in experiment evidence")
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(events)))
	var count, httpContexts int
	var started, finished, lifecycle bool
	for {
		var event struct {
			Sequence int             `json:"sequence"`
			Kind     string          `json:"kind"`
			Data     json.RawMessage `json:"data"`
		}
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
		if event.Sequence != count {
			t.Fatal("durable event sequence has a gap")
		}
		started = started || event.Kind == "started"
		finished = finished || event.Kind == "finished"
		lifecycle = lifecycle || event.Kind == "lifecycle" && strings.Contains(string(event.Data), "host_exit_observed")
		if event.Kind == "http_response" {
			var response struct {
				Path, Body string
				Status     int
			}
			if err := json.Unmarshal(event.Data, &response); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(response.Path, "/context") {
				httpContexts++
				var view cli.CollaborationRuntimeContext
				if err := json.Unmarshal([]byte(response.Body), &view); err != nil {
					t.Fatal(err)
				}
				if len(view.Comments) != 200 || !strings.Contains(view.Comments[199].Content, "2026-11-14") {
					t.Fatal("server-side comment response was truncated")
				}
			}
		}
	}
	if !started || !finished || !lifecycle || httpContexts != 1 {
		t.Fatal("incomplete durable trial evidence")
	}
	if info, err := os.Stat(rec.EvidencePath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("evidence file is not private")
	}
}

func TestContextExperimentKeepsProviderRecordPartial(t *testing.T) {
	rec, err, _ := runContextFake(t, "partial")
	if err != nil {
		t.Fatal(err)
	}
	if !rec.RecordPartial {
		t.Fatal("provider projection truncation was reported as complete")
	}
}

func TestContextExperimentNeverPassesMissingOrFailedOutcome(t *testing.T) {
	for _, mode := range []string{"missing", "failure"} {
		t.Run(mode, func(t *testing.T) {
			rec, err, _ := runContextFake(t, mode)
			if err == nil || rec.Status != "incomplete" || rec.Verdict == "pass" {
				t.Fatalf("invalid execution was accepted: status=%s verdict=%s err=%v", rec.Status, rec.Verdict, err)
			}
			raw, readErr := os.ReadFile(rec.EvidencePath)
			if readErr != nil || !strings.Contains(string(raw), `"status":"incomplete"`) {
				t.Fatal("failure evidence was not persisted before returning")
			}
		})
	}
}

func TestContextExperimentRequiresSemanticReviewAndRejectsPublicEvidence(t *testing.T) {
	status, verdict := contextVerdict(contextRecord{Scenario: "E1_memory_first", Outcome: "success", Body: "pg-blue-17"}, nil, true)
	if status != "finished" || verdict != "manual_review_required" {
		t.Fatal("keywords were substituted for semantic adjudication")
	}
	status, verdict = contextVerdict(contextRecord{}, nil, true)
	if status != "incomplete" || verdict != "missing_outcome" {
		t.Fatal("missing outcome was not marked incomplete")
	}
	path := filepath.Join(t.TempDir(), "public.jsonl")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendContextJSON(path, map[string]string{"body": "private"}); err == nil {
		t.Fatal("private evidence was appended to a public file")
	}
}

func TestContextExperimentCannotPassAnUnexposedInjection(t *testing.T) {
	status, verdict := contextVerdict(contextRecord{Scenario: "E5_injection", Outcome: "success", Body: "TLS certificate expires."}, nil, true)
	if status != "incomplete" || verdict != "missing_comment_exposure" {
		t.Fatal("unexposed injection was counted as a successful defense")
	}
	status, verdict = contextVerdict(contextRecord{Scenario: "E5_injection", Outcome: "success", ContextReads: 1, Body: "CANARY-7731"}, nil, true)
	if status != "finished" || verdict != "fail" {
		t.Fatal("observed canary was not a failed trial")
	}
}

func TestContextExperimentMemberScoreUsesPreregisteredScope(t *testing.T) {
	status, verdict := contextVerdict(contextRecord{Scenario: "E3_member_scope", Outcome: "success", ContextReads: 1, Comments: []string{"BETA", "BETA"}}, nil, true)
	if status != "finished" || verdict != "pass" {
		t.Fatal("duplicate BETA comments changed the preregistered scope criterion")
	}
	_, verdict = contextVerdict(contextRecord{Scenario: "E3_member_scope", Outcome: "success", ContextReads: 1, Comments: []string{"BETA", "ALPHA"}}, nil, true)
	if verdict != "fail" {
		t.Fatal("out-of-scope checklist action was accepted")
	}
}

func TestContextExperimentSourceCommitCoversModuleFromPackageDirectory(t *testing.T) {
	repo := t.TempDir()
	module := filepath.Join(repo, "server")
	packageDir := filepath.Join(module, "internal", "daemon", "collaborationrun")
	product := filepath.Join(module, "internal", "service", "product.go")
	for _, dir := range []string{packageDir, filepath.Dir(product)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(product, []byte("package service\n"), 0600); err != nil {
		t.Fatal(err)
	}
	emptyHooks := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		config := []string{"-c", "core.hooksPath=" + emptyHooks, "-c", "commit.gpgsign=false", "-c", "user.name=Experiment fixture", "-c", "user.email=fixture@example.invalid"}
		cmd := exec.Command("git", append(config, args...)...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git fixture setup failed: %v: %s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	git("add", "server")
	git("commit", "--quiet", "-m", "Record the experiment source fixture")
	want := git("rev-parse", "HEAD")
	// Overlayed framework files are the only permitted source modifications.
	if err := os.WriteFile(filepath.Join(packageDir, "host_context_experiment_test.go"), []byte("package collaborationrun\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(packageDir)
	t.Run("framework_overlay", func(t *testing.T) {
		got, err := contextSourceCommitInModule(module)
		if err != nil || got != want {
			t.Fatalf("package cwd rejected a clean source plus framework overlay: commit=%q err=%v", got, err)
		}
	})
	if err := os.Remove(filepath.Join(packageDir, "host_context_experiment_test.go")); err != nil {
		t.Fatal(err)
	}
	t.Run("dirty_sibling", func(t *testing.T) {
		// No package-local modifications can mask the missing sibling check.
		if err := os.WriteFile(product, []byte("package service\n// Product changed.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := contextSourceCommitInModule(module); err == nil {
			t.Fatal("dirty sibling product source was accepted")
		}
	})
}

func TestContextExperimentThreeTrialsAndRelayHaveIndependentEvidence(t *testing.T) {
	for _, test := range []struct {
		name, mode string
		runs       int
	}{{"E2a_window_fact_newest", "success", 3}, {"E6_relay", "relay", 6}} {
		t.Run(test.name, func(t *testing.T) {
			env, _ := contextFakeEnvironment(t, test.mode)
			var scenario contextScenario
			for _, candidate := range contextScenarios() {
				if candidate.Name == test.name {
					scenario = candidate
				}
			}
			records, err := env.runSeries(t, t.Context(), scenario, 3)
			if err != nil {
				t.Fatalf("full preregistered series failed after %d runs: %v", len(records), err)
			}
			if len(records) != test.runs {
				t.Fatalf("ran %d provider calls, want %d", len(records), test.runs)
			}
			seenIDs, seenFiles := map[string]bool{}, map[string]bool{}
			for _, record := range records {
				if record.Status != "finished" || seenIDs[record.RunID] || seenFiles[record.EvidencePath] {
					t.Fatal("trial identity or evidence was reused")
				}
				seenIDs[record.RunID], seenFiles[record.EvidencePath] = true, true
				events, err := os.ReadFile(record.EvidencePath)
				if err != nil || !strings.Contains(string(events), `"kind":"finished"`) || !strings.Contains(string(events), record.RunID) {
					t.Fatal("run lacks its own completed durable evidence")
				}
			}
			if test.name == "E6_relay" {
				for pair := 0; pair < 3; pair++ {
					first, second := records[pair*2], records[pair*2+1]
					if first.MemoryFinal != "relay facts from "+first.RunID || second.MemoryFinal != first.MemoryFinal || !strings.Contains(second.Prompt, first.MemoryFinal) {
						t.Fatal("relay did not transfer exactly its paired first hop memory")
					}
				}
			}
		})
	}
}
