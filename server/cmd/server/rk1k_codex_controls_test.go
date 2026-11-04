//go:build agentintegration && !windows

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRK1KnowledgeCodexRPCControls(t *testing.T) {
	native := uuid.NewString()
	var input bytes.Buffer
	enc := json.NewEncoder(&input)
	for _, v := range []any{
		map[string]any{"id": 1, "method": "initialize", "params": map[string]any{}},
		map[string]any{"method": "initialized"},
		map[string]any{"id": 2, "method": "thread/start", "params": map[string]any{"ephemeral": true}},
		map[string]any{"id": 3, "method": "turn/start", "params": map[string]any{"threadId": native, "input": []map[string]string{{"type": "text", "text": "No policy is delivered."}}}},
	} {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	plan := rk1Plan{Mode: "fake", Behavior: "guard", Params: map[string]string{"native_id": native}}
	if code := rk1kCodexFake(plan, rk1kIdentity{}, bytes.NewReader(input.Bytes()), &output); code != 0 {
		t.Fatalf("native fake RPC exit %d", code)
	}
	var frames []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		var frame map[string]any
		if json.Unmarshal(line, &frame) != nil {
			t.Fatal("bad fixture")
		}
		frames = append(frames, frame)
	}
	mutate := func(kind string) []byte {
		var b bytes.Buffer
		for _, original := range frames {
			raw, _ := json.Marshal(original)
			var f map[string]any
			_ = json.Unmarshal(raw, &f)
			method, _ := f["method"].(string)
			if kind == "wrong_item_turn" && method == "item/completed" {
				f["params"].(map[string]any)["turnId"] = "wrong-turn"
			}
			if kind == "missing_turn_response" && f["id"] == float64(3) {
				continue
			}
			if kind == "turn_error" && f["id"] == float64(3) {
				delete(f, "result")
				f["error"] = map[string]any{"code": -1, "message": "rejected"}
			}
			_ = json.NewEncoder(&b).Encode(f)
			if kind == "duplicate_terminal" && method == "turn/completed" {
				_ = json.NewEncoder(&b).Encode(f)
			}
			if kind == "duplicate_response" && f["id"] == float64(2) {
				_ = json.NewEncoder(&b).Encode(f)
			}
		}
		if kind == "post_terminal_item" {
			for _, f := range frames {
				if f["method"] == "item/completed" {
					_ = json.NewEncoder(&b).Encode(f)
				}
			}
		}
		return b.Bytes()
	}
	resumeInput := func(threadID string) []byte {
		var b bytes.Buffer
		for _, line := range bytes.Split(bytes.TrimSpace(input.Bytes()), []byte{'\n'}) {
			var f map[string]any
			_ = json.Unmarshal(line, &f)
			if f["method"] == "thread/start" {
				f["method"] = "thread/resume"
				f["params"] = map[string]any{"threadId": threadID}
			}
			_ = json.NewEncoder(&b).Encode(f)
		}
		return b.Bytes()
	}
	for _, tc := range []struct {
		name                    string
		in, out                 []byte
		wantComplete, wantError bool
	}{
		{"complete", input.Bytes(), output.Bytes(), true, false},
		{"exact_resume", resumeInput(native), output.Bytes(), true, false},
		{"wrong_requested_resume", resumeInput(uuid.NewString()), output.Bytes(), false, true},
		{"wrong_item_turn", input.Bytes(), mutate("wrong_item_turn"), false, true},
		{"post_terminal_item", input.Bytes(), mutate("post_terminal_item"), true, true},
		{"missing_turn_response", input.Bytes(), mutate("missing_turn_response"), false, true},
		{"turn_error", input.Bytes(), mutate("turn_error"), false, true},
		{"duplicate_terminal", input.Bytes(), mutate("duplicate_terminal"), true, true},
		{"duplicate_response", input.Bytes(), mutate("duplicate_response"), false, true},
		{"missing_terminal", input.Bytes(), bytes.ReplaceAll(output.Bytes(), []byte("turn/completed"), []byte("unrelated/event")), false, false},
		{"wrong_thread", input.Bytes(), bytes.ReplaceAll(output.Bytes(), []byte(native), []byte(uuid.NewString())), false, true},
		{"invalid_json", input.Bytes(), []byte("{bad}\n"), false, true},
		{"duplicate_input", append(append([]byte{}, input.Bytes()...), input.Bytes()...), output.Bytes(), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := rk1kObserveCodex(tc.in, tc.out)
			if (err != nil) != tc.wantError || o.Complete != tc.wantComplete {
				t.Fatalf("complete=%v error=%v", o.Complete, err)
			}
			if err == nil && tc.wantComplete && (!o.Ephemeral || o.Native != native || o.Answer != "NONE") {
				t.Fatalf("unexpected native observation %+v", o)
			}
		})
	}
}

func TestRK1KnowledgeCodexLedgerStopsAfterIncomplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := rk1WriteJSON(path, rk1kCodexLedger{Cap: 15, Starts: []rk1kCodexStart{{Sequence: 1}}}); err != nil {
		t.Fatal(err)
	}
	reservation := rk1kCodexReservation{path: path, sequence: 1}
	if err := reservation.finish(false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger rk1kCodexLedger
	if json.Unmarshal(raw, &ledger) != nil || !ledger.Stopped || ledger.Starts[0].Complete {
		t.Fatal("incomplete physical execution did not stop ledger")
	}
	if err := os.WriteFile(path+".lock", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := rk1kWithCodexLedger(path, func(*rk1kCodexLedger) error { t.Fatal("concurrent ledger admitted"); return nil }); err == nil {
		t.Fatal("concurrent ownership was ignored")
	}
}

func TestRK1KnowledgeCodexRealPlanDoesNotCarryFakeAnswers(t *testing.T) {
	f := &rk1kFixture{real: &rk1kCodexRegistration{Executable: "/private/codex", RegistrationPath: "/private/registration.json", RegistrationSHA256: strings.Repeat("a", 64)}}
	plan := f.executionPlan(rk1Plan{Mode: "fake", Behavior: "chat", Fault: "wrong", Params: map[string]string{"expected_answer": "secret", "native_id": uuid.NewString()}})
	if plan.Mode != "real" || plan.Fault != "" || len(plan.Params) != 2 || plan.Params["expected_answer"] != "" || plan.Params["native_id"] != "" {
		t.Fatalf("fake expectation reached real plan: %+v", plan)
	}
	seed := f.executionPlan(rk1Plan{Mode: "fake", Params: map[string]string{"seed": "true"}})
	if seed.Mode != "fake" || seed.Real != "" {
		t.Fatal("simulated prefix became a real execution")
	}
}

func TestRK1KnowledgeCodexPhysicalBudget(t *testing.T) {
	id := func() rk1kIdentity {
		return rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
	}
	for _, name := range []string{"cap", "duplicate", "incomplete", "stopped", "missing", "corrupt", "write_failure"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "ledger.json")
			identity := id()
			ledger := rk1kCodexLedger{Cap: 15}
			switch name {
			case "cap":
				for i := 0; i < 15; i++ {
					ledger.Starts = append(ledger.Starts, rk1kCodexStart{Sequence: i + 1, Identity: id(), ActualStarted: true, Complete: true})
				}
			case "duplicate":
				ledger.Starts = []rk1kCodexStart{{Sequence: 1, Identity: identity, ActualStarted: true, Complete: true}}
			case "incomplete":
				ledger.Starts = []rk1kCodexStart{{Sequence: 1, Identity: id(), ActualStarted: true}}
			case "stopped":
				ledger.Stopped = true
			}
			if name != "missing" {
				if err := rk1WriteJSON(path, ledger); err != nil {
					t.Fatal(err)
				}
			}
			if name == "corrupt" {
				if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "write_failure" {
				err := rk1kWithCodexLedger(path, func(*rk1kCodexLedger) error {
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.Mkdir(path, 0700)
				})
				if err == nil {
					t.Fatal("durability failure accepted")
				}
				return
			}
			if _, err := rk1kReserveCodexLedger(&rk1kCodexRegistration{Ledger: path}, identity, filepath.Join(dir, "launch")); err == nil {
				t.Fatal("forbidden next physical execution admitted")
			}
		})
	}
	t.Run("fifteen_only", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.json")
		if err := rk1WriteJSON(path, rk1kCodexLedger{Cap: 15}); err != nil {
			t.Fatal(err)
		}
		r := &rk1kCodexRegistration{Ledger: path}
		for i := 0; i < 15; i++ {
			identity := id()
			slot, err := rk1kReserveCodexLedger(r, identity, "/owned/launch")
			if err != nil {
				t.Fatal(err)
			}
			if err := slot.started(); err != nil {
				t.Fatal(err)
			}
			if err := slot.finish(true); err != nil {
				t.Fatal(err)
			}
			if err := rk1kAcceptCodexSample(r, rk1kSample{Identity: identity, State: "finished", Verdict: "pass"}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := rk1kReserveCodexLedger(r, id(), "/owned/launch16"); err == nil {
			t.Fatal("sixteenth process admitted")
		}
	})
}

func TestRK1KnowledgeCodexRegistrationIntegrity(t *testing.T) {
	for _, change := range []string{"valid", "unauthorized", "expired", "wrong_provider", "wrong_cap", "source", "config", "binary", "executable", "registration"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.go")
			config := filepath.Join(dir, "config.toml")
			executable := filepath.Join(dir, "fake-codex")
			for _, path := range []string{source, config, executable} {
				if err := os.WriteFile(path, []byte("test-owned inert bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			hash := func(path string) string {
				value, err := rk1kFileHash(path)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			r := rk1kCodexRegistration{SchemaVersion: 1, Authorized: true, Provider: "codex", Executable: executable, ExecutableSHA256: hash(executable), Version: "0.165.0", Model: "test-model", ConfigPath: config, ConfigSHA256: hash(config), TestBinarySHA256: hash(self), SourceFiles: map[string]string{source: hash(source)}, Cap: 15, Ledger: filepath.Join(dir, "ledger.json"), Output: filepath.Join(dir, "out"), Database: "multica_rk1k_control", ExpiresAt: time.Now().Add(time.Hour)}
			switch change {
			case "unauthorized":
				r.Authorized = false
			case "expired":
				r.ExpiresAt = time.Now().Add(-time.Hour)
			case "wrong_provider":
				r.Provider = "claude"
			case "wrong_cap":
				r.Cap = 16
			case "binary":
				r.TestBinarySHA256 = strings.Repeat("0", 64)
			}
			path := filepath.Join(dir, "registration.json")
			if err := rk1WriteJSON(path, r); err != nil {
				t.Fatal(err)
			}
			digest := hash(path)
			mutate := ""
			switch change {
			case "source":
				mutate = source
			case "config":
				mutate = config
			case "executable":
				mutate = executable
			case "registration":
				mutate = path
			}
			if mutate != "" {
				if err := os.WriteFile(mutate, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err = rk1kLoadCodexRegistration(path, digest)
			if (err == nil) != (change == "valid") {
				t.Fatalf("registration %s: error=%v", change, err)
			}
		})
	}
}

func TestRK1KnowledgeCodexSampleAcknowledgment(t *testing.T) {
	for _, publish := range []bool{false, true} {
		t.Run(map[bool]string{false: "publication_failed", true: "published"}[publish], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "ledger.json")
			if err := rk1WriteJSON(path, rk1kCodexLedger{Cap: 15}); err != nil {
				t.Fatal(err)
			}
			r := &rk1kCodexRegistration{Ledger: path}
			id := rk1kIdentity{Kind: "task", TaskID: uuid.NewString(), Workspace: uuid.NewString(), Agent: uuid.NewString()}
			slot, err := rk1kReserveCodexLedger(r, id, "/owned/launch")
			if err != nil {
				t.Fatal(err)
			}
			if err := slot.started(); err != nil {
				t.Fatal(err)
			}
			if err := slot.finish(true); err != nil {
				t.Fatal(err)
			}
			next := id
			next.TaskID = uuid.NewString()
			if _, err := rk1kReserveCodexLedger(r, next, "/owned/next"); err == nil {
				t.Fatal("process terminal bypassed outer sample publication")
			}
			sample := rk1kSample{Identity: id, State: "finished", Verdict: "pass"}
			dest := filepath.Join(dir, "sample.json")
			if !publish {
				if err := os.Mkdir(dest, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err = rk1WriteJSON(dest, sample)
			if publish {
				if err != nil {
					t.Fatal(err)
				}
				if err := rk1kAcceptCodexSample(r, sample); err != nil {
					t.Fatal(err)
				}
				if _, err := rk1kReserveCodexLedger(r, next, "/owned/next"); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("expected publication failure")
				}
				if err := rk1kStopCodexLedger(r); err != nil {
					t.Fatal(err)
				}
				if _, err := rk1kReserveCodexLedger(r, next, "/owned/next"); err == nil {
					t.Fatal("failed outer publication allowed next start")
				}
			}
		})
	}
}
