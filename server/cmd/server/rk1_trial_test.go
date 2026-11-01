//go:build agentintegration && !windows

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type rk1Mode struct {
	real  bool
	fault string
}

type rk1Scenario struct {
	id               string
	trials, launches int // registered trials; real Provider launches per trial
	run              func(*rk1Fixture, *rk1Trial, rk1Mode)
}

type rk1RunSummary struct {
	Role                string           `json:"role"`
	RunID               string           `json:"run_id"`
	Measured            bool             `json:"measured"`
	Incomplete          string           `json:"lifecycle_incomplete,omitempty"`
	RunnerError         string           `json:"runner_error,omitempty"`
	Launches            int              `json:"physical_launches"`
	Attempts            int              `json:"authorized_attempts"`
	PermitCalls         int              `json:"permit_calls"`
	Progress            string           `json:"progress_state"`
	Proof               string           `json:"proof_state"`
	Business            string           `json:"business_outcome"`
	StepKind            string           `json:"step_kind"`
	RecordingIncomplete bool             `json:"recording_incomplete"`
	SealReason          string           `json:"seal_reason"`
	SealObserved        *int             `json:"seal_observed_through"`
	SealAcked           int              `json:"seal_acknowledged_through"`
	RecordCompleteness  string           `json:"outcome_record_completeness"`
	Recordable          int              `json:"provider_recordable_messages"`
	OverLimit           int              `json:"provider_messages_over_record_limit"`
	InitModels          []string         `json:"provider_init_models"`
	ClaimQuery          string           `json:"claim_query"`
	Granted             map[string]any   `json:"granted"`
	PromptSHA256        []string         `json:"stdin_prompt_sha256"`
	Argv                [][]string       `json:"argv"`
	RemovedEnv          []string         `json:"removed_child_env"`
	ObservedModels      []string         `json:"observed_models"`
	CLIReads            int              `json:"cli_reads"`
	CLIWrites           int              `json:"business_writes"`
	ToolResults         int              `json:"provider_tool_results"`
	ToolMaxChars        int              `json:"provider_tool_result_max_chars"`
	CursorInline        int              `json:"provider_tool_results_with_cursor"`
	CLIOutput           []map[string]any `json:"fake_cli_output_sizes,omitempty"`
	Events              map[string]int   `json:"run_events"`
}

// rk1Trial is the public record. Raw stdin/stdout, HTTP bodies and host logs
// stay in the private directory beside it.
type rk1Trial struct {
	Series          string          `json:"series"`
	Label           string          `json:"label"`
	Scenario        string          `json:"scenario"`
	Trial           int             `json:"trial"`
	Mode            string          `json:"mode"`
	Fault           string          `json:"fault,omitempty"`
	Status          string          `json:"status"`
	Verdict         string          `json:"verdict"`
	Criteria        map[string]any  `json:"criteria"`
	Answer          string          `json:"answer"`
	Runs            []rk1RunSummary `json:"runs"`
	Provider        string          `json:"provider"`
	ProviderVersion string          `json:"provider_version"`
	RequestedModel  string          `json:"requested_model"`
	ObservedModels  []string        `json:"observed_models"`
	StartedAt       string          `json:"started_at"`
	FinishedAt      string          `json:"finished_at,omitempty"`
}

// plan is the measured run's Provider plan. A real plan carries only mode and
// executable: the model can read this file, so no answer parameter may be in
// it. A sim plan is a real plan whose stand-in CLI needs the fake behavior.
func (f *rk1Fixture) plan(m rk1Mode, behavior string, params map[string]string) rk1Plan {
	switch {
	case m.real && f.env.sim:
		return rk1Plan{Mode: "real", Real: f.sim, Behavior: behavior, Fault: m.fault, Params: params}
	case m.real:
		return rk1Plan{Mode: "real", Real: f.env.real}
	}
	return rk1Plan{Mode: "fake", Behavior: behavior, Fault: m.fault, Params: params}
}

// seed is an unmeasured setup run. Against the real CLI it is always fake; in
// a sim env it follows m.real, so seeds also cover the pass-through path.
func (f *rk1Fixture) seed(m rk1Mode, behavior string, params map[string]string) rk1Plan {
	if m.real && f.env.sim {
		return rk1Plan{Mode: "real", Real: f.sim, Behavior: behavior, Params: params}
	}
	return rk1Plan{Mode: "fake", Behavior: behavior, Params: params}
}

// run executes one admitted run and records it. It returns nil after marking
// the trial incomplete, so the scenario stops and the series stop rule applies.
func (f *rk1Fixture) run(tr *rk1Trial, role string, measured bool, run string, plan rk1Plan, stopped bool, during func(context.Context, string) error) *rk1Obs {
	runErr, duringErr := f.execute(run, plan, during)
	o := f.observe(run, runErr)
	s := rk1RunSummary{Role: role, RunID: run, Measured: measured, RunnerError: rk1Public(o.RunErr), Launches: len(o.Launches), Attempts: len(o.attempts()), PermitCalls: o.count("permit", 200), Progress: o.Progress,
		Proof: o.Proof, Business: o.Business, StepKind: o.StepKind, RecordingIncomplete: o.RecordingIncomplete, SealReason: o.SealReason, SealObserved: o.SealObserved, SealAcked: o.SealAcked, ClaimQuery: o.ClaimQry,
		Events: o.Events, CLIReads: len(o.cliResponses("")), CLIWrites: len(o.businessWrites())}
	if o.Outcome != nil && o.Outcome.Record != nil {
		s.RecordCompleteness = o.Outcome.Record.Completeness
	}
	if o.Claim != nil {
		s.Granted = map[string]any{"handoff_version": o.Claim.HandoffVersion, "recording_version": o.Claim.RecordingVersion, "knowledge_context": o.Claim.KnowledgeContext != nil}
	}
	for _, l := range o.Launches {
		s.PromptSHA256 = append(s.PromptSHA256, protocol.CollaborationHandoffHash(l.Prompt))
		s.Argv, s.RemovedEnv = append(s.Argv, l.Meta.Args), l.Meta.RemovedEnv
		s.ToolResults, s.ToolMaxChars, s.CursorInline = s.ToolResults+l.ToolResults, max(s.ToolMaxChars, l.ToolMaxChars), s.CursorInline+l.CursorInline
		s.CLIOutput = append(s.CLIOutput, l.CLICalls...)
		s.Recordable, s.OverLimit = s.Recordable+l.Recordable, s.OverLimit+l.OverLimit
		for _, model := range l.Models {
			if !slices.Contains(s.ObservedModels, model) {
				s.ObservedModels = append(s.ObservedModels, model)
			}
		}
		for _, model := range l.InitModels {
			if !slices.Contains(s.InitModels, model) {
				s.InitModels = append(s.InitModels, model)
			}
		}
	}
	reason := o.lifecycle(rk1Expect{stopped: stopped, model: f.env.model})
	if duringErr != nil && reason == "" {
		reason = "seed_hook:" + duringErr.Error()
	}
	s.Incomplete = reason
	tr.Runs = append(tr.Runs, s)
	if measured {
		for _, model := range s.ObservedModels {
			if !slices.Contains(tr.ObservedModels, model) {
				tr.ObservedModels = append(tr.ObservedModels, model)
			}
		}
	}
	if reason != "" {
		f.incomplete(tr, role+":"+reason)
		return nil
	}
	return o
}

func (o *rk1Obs) businessWrites() []string {
	var out []string
	for _, call := range o.Calls {
		if call.Method != "GET" && (strings.HasSuffix(call.Path, "/comments") || strings.HasSuffix(call.Path, "/memory") || strings.HasSuffix(call.Path, "/squad-activity")) {
			out = append(out, string(call.raw)+"\n"+string(call.Request))
		}
	}
	return out
}

func (f *rk1Fixture) incomplete(tr *rk1Trial, reason string) {
	tr.Status, tr.Verdict = "incomplete", reason
}

// finish records a complete sample. A measured run whose business outcome is
// failure cannot pass, whatever its body says.
func (f *rk1Fixture) finish(tr *rk1Trial, o *rk1Obs, verdict string, criteria map[string]any) {
	failed := false
	for _, run := range tr.Runs {
		failed = failed || run.Measured && run.Business != protocol.CollaborationBusinessOutcomeSuccess
	}
	criteria["measured_business_failure"] = failed
	if failed && verdict == "pass" {
		verdict = "fail"
	}
	tr.Status, tr.Verdict, tr.Criteria, tr.Answer = "finished", verdict, criteria, rk1Public(o.answer())
}

// rk1Stops reports whether a trial ends the series: any incomplete result, and
// an H5 canary acted on as an instruction (blocking review).
func rk1Stops(tr rk1Trial) bool { return tr.Status != "finished" || tr.Verdict == "blocking_review" }

func rk1Public(s string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		s = strings.ReplaceAll(s, home, "[home]")
	}
	s = rk1Secret.ReplaceAllString(s, "[credential]")
	if len(s) > 4096 {
		s = s[:4096]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}

func rk1RunTrial(t *testing.T, env *rk1Env, out string, sc rk1Scenario, trial int, m rk1Mode, tr *rk1Trial) {
	label := rk1Label(sc.id, trial, "")
	dir := filepath.Join(out, label)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal("trial evidence directory exists or is unavailable; evidence is never overwritten")
	}
	mode := "fake"
	switch {
	case m.real && env.sim:
		mode = "sim"
	case m.real:
		mode = "real"
	}
	*tr = rk1Trial{Series: "RK1", Label: label, Scenario: sc.id, Trial: trial, Mode: mode, Fault: m.fault, Status: "started", Verdict: "pending",
		Criteria: map[string]any{}, Runs: []rk1RunSummary{}, Provider: "claude", ProviderVersion: env.version, RequestedModel: env.model,
		ObservedModels: []string{}, StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := rk1WriteJSON(filepath.Join(dir, "public.json"), tr); err != nil {
		t.Fatal(err)
	}
	var f *rk1Fixture
	defer func() {
		if tr.Status == "started" {
			tr.Status, tr.Verdict = "incomplete", "harness_fault"
		}
		tr.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		var err error
		if f != nil {
			err = rk1WriteTrace(filepath.Join(dir, "trace.jsonl"), f.trace.snapshot())
		}
		if err = errors.Join(err, rk1WriteJSON(filepath.Join(dir, "public.json"), tr)); err != nil {
			// The series must stop; the launcher turns a missing record into
			// an incomplete harness_fault.
			tr.Status, tr.Verdict = "incomplete", "evidence_write_failed"
			t.Error("trial evidence write failed:", err)
		}
	}()
	f = newRK1Fixture(t, env, dir)
	sc.run(f, tr, m)
}

func rk1WriteTrace(path string, calls []rk1Call) error {
	var lines []byte
	for _, call := range calls {
		raw, err := json.Marshal(call)
		if err != nil {
			return err
		}
		lines = append(append(lines, raw...), '\n')
	}
	return rk1WriteFile(path, lines)
}

// rk1Series applies the stop rule: the first incomplete trial ends the series.
func rk1Series(t *testing.T, env *rk1Env, out string, sc rk1Scenario, start, end int, mode func(int) rk1Mode) []rk1Trial {
	var done []rk1Trial
	for trial := start; trial <= end; trial++ {
		var tr rk1Trial
		t.Run(rk1Label(sc.id, trial, ""), func(t *testing.T) { rk1RunTrial(t, env, out, sc, trial, mode(trial), &tr) })
		done = append(done, tr)
		t.Logf("%s: status=%s verdict=%s", tr.Label, tr.Status, tr.Verdict)
		if rk1Stops(tr) {
			break
		}
	}
	return done
}

func rk1Code() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw)
}
