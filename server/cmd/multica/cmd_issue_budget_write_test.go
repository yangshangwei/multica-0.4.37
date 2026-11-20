package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
)

// Tests of the three outcomes of a budget write (design 12.6): committed,
// refused, unknown. The harness is in cmd_issue_budget_test.go.

// budgetWriteCase is one write with every flag given, and what its outcome is
// reported with.
type budgetWriteCase struct {
	name  string
	args  []string
	path  string
	kind  string // the operation_kind of its result
	other string // another kind
	key   string
	root  string // "" for enable, which is addressed by an issue
	byKey string // the command that asks what became of it
	show  string // the command a stale revision sends the operator to
}

var budgetWriteCases = []budgetWriteCase{
	{
		name: "enable", args: budgetEnableArgs, path: budgetTestEnablePath, kind: "enable", other: "close", key: budgetTestEnableKey,
		byKey: "multica issue budget operation --issue MUL-12 --request-key " + budgetTestEnableKey,
		show:  "multica issue budget show MUL-12",
	},
	{
		name: "set-limit", args: budgetSetLimitArgs, path: budgetTestOperationsPath, kind: "set_limit", other: "set_mode", key: budgetTestSetLimitKey, root: budgetTestRoot,
		byKey: "multica issue budget operation --root " + budgetTestRoot + " --request-key " + budgetTestSetLimitKey,
		show:  "multica issue budget show --root " + budgetTestRoot,
	},
	{
		name: "set-mode", args: budgetSetModeArgs, path: budgetTestOperationsPath, kind: "set_mode", other: "set_limit", key: budgetTestSetModeKey, root: budgetTestRoot,
		byKey: "multica issue budget operation --root " + budgetTestRoot + " --request-key " + budgetTestSetModeKey,
		show:  "multica issue budget show --root " + budgetTestRoot,
	},
	{
		name: "close", args: budgetCloseArgs, path: budgetTestOperationsPath, kind: "close", other: "reopen", key: budgetTestCloseKey, root: budgetTestRoot,
		byKey: "multica issue budget operation --root " + budgetTestRoot + " --request-key " + budgetTestCloseKey,
		show:  "multica issue budget show --root " + budgetTestRoot,
	},
	{
		name: "reopen", args: budgetReopenArgs, path: budgetTestOperationsPath, kind: "reopen", other: "close", key: budgetTestReopenKey, root: budgetTestRoot,
		byKey: "multica issue budget operation --root " + budgetTestRoot + " --request-key " + budgetTestReopenKey,
		show:  "multica issue budget show --root " + budgetTestRoot,
	},
}

// answer is the body the server gives when this write commits now.
func (w budgetWriteCase) answer(t *testing.T) []byte {
	t.Helper()
	if w.kind == "enable" {
		return budgetFixture(t, "write-201-enable.json")
	}
	return budgetRootWriteAnswer(t, w.kind, false)
}

// typed is every id the operator typed.
func (w budgetWriteCase) typed() []string { return budgetTypedIDs(w.args) }

const (
	budgetWrittenNow = "committed: the operation was written now"
	budgetReplayed   = "committed: this request key had already committed this operation; nothing was written now"
)

func TestIssueBudgetWriteCommitted(t *testing.T) {
	t.Run("201 for an enable", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("POST", budgetTestEnablePath, 201, "write-201-enable.json")
		run := runBudgetCLI(t, budgetEnableArgs...)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			budgetWrittenNow,
			"OPERATION | c4e1a2f7-08b3-4b9d-a6c5-3e9f1d2b7a80",
			"KIND | enable",
			"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
			"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
			"STATE | open",
			"MODE | observe",
			"POLICY REVISION | 1",
			"POLICY EPOCH | 1",
			"LIMIT | 25.00 USD (250000000000 ticks)",
			"ATTEMPT RESERVE | not set",
			"REASON | Q4 launch",
			"ACTIVATED AT | 2026-11-19T01:02:03.456789Z",
			"ISSUES BOUND | 4, the root issue included",
			"IN FLIGHT AT ACTIVATION | 1; already running when the root was enabled, so unknown from before activation and not a zero cost",
		)
		fake.wantRequests("POST " + budgetTestEnablePath)
	})

	// 12.9 leaves the two counts of an enable out when they are zero. That is
	// the one place where a key that is not there is read as a number.
	t.Run("201 for an enable that leaves out a count of zero", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("POST", budgetTestEnablePath, 201, "write-201-enable-no-gaps.json")
		run := runBudgetCLI(t, budgetEnableArgs...)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			budgetWrittenNow,
			"OPERATION | 6a0f3e9d-5c2b-4a17-8f64-d3c2b1a09e8f",
			"KIND | enable",
			"BUDGET ROOT | b3a81f6d-2c47-4e95-8d0a-9f1e7c5b3a26",
			"ROOT ISSUE | 8c1d5e9f-0a2b-4c3d-9e4f-5a6b7c8d9e0f",
			"STATE | open",
			"MODE | observe",
			"POLICY REVISION | 1",
			"POLICY EPOCH | 1",
			"LIMIT | not set",
			"ATTEMPT RESERVE | not set",
			"REASON | observe only, no cap",
			"ACTIVATED AT | 2026-11-19T02:15:40Z",
			"ISSUES BOUND | 1, the root issue included",
			"IN FLIGHT AT ACTIVATION | 0",
		)
	})

	t.Run("200 for a key that had already committed", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("POST", budgetTestOperationsPath, 200, "write-200-replayed-set-limit.json")
		run := runBudgetCLI(t, budgetSetLimitArgs...)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			budgetReplayed,
			"OPERATION | 1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79",
			"KIND | set_limit",
			"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
			"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
			"STATE | open",
			"MODE | observe",
			"POLICY REVISION | 4",
			"POLICY EPOCH | 1",
			"LIMIT | not set",
			"ATTEMPT RESERVE | 0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap",
			"REASON | reserve half a dollar per run, no cap yet",
		)
		fake.wantRequests("POST " + budgetTestOperationsPath)
	})

	for _, write := range budgetWriteCases {
		t.Run(write.name+": 201, and 200 with replayed", func(t *testing.T) {
			for _, answer := range []struct {
				status int
				body   []byte
				first  string
			}{
				{201, write.answer(t), budgetWrittenNow},
				{200, budgetReplace(t, write.answer(t), `"replayed":false`, `"replayed":true`), budgetReplayed},
				// The status says written now and the body says replayed. Both
				// say committed; the command goes by the body.
				{201, budgetReplace(t, write.answer(t), `"replayed":false`, `"replayed":true`), budgetReplayed},
			} {
				fake := newBudgetFake(t)
				fake.answer("POST", write.path, answer.status, answer.body)
				run := runBudgetCLI(t, write.args...)
				if run.exit != 0 || run.stderr != "" {
					t.Fatalf("HTTP %d: exit %d, stderr %q", answer.status, run.exit, run.stderr)
				}
				cells := budgetCells(run.stdout)
				if cells[0] != answer.first {
					t.Errorf("HTTP %d: first line %q, want %q", answer.status, cells[0], answer.first)
				}
				if got := budgetLine(t, run.stdout, "KIND"); got != write.kind {
					t.Errorf("HTTP %d: KIND %q, want %q", answer.status, got, write.kind)
				}
				fake.wantRequests("POST " + write.path)
			}
		})
	}

	// The result of an operation on a root has no activation time and no counts;
	// those lines belong to an enable.
	t.Run("a result that is not an enable has no activation lines", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("POST", budgetTestOperationsPath, 201, budgetRootWriteAnswer(t, "close", false))
		run := runBudgetCLI(t, budgetCloseArgs...)
		for _, label := range []string{"ACTIVATED AT", "ISSUES BOUND", "IN FLIGHT AT ACTIVATION"} {
			if strings.Contains(run.stdout, label) {
				t.Errorf("the result of a close prints %s:\n%s", label, run.stdout)
			}
		}
	})
}

// --output json prints the answer to a write as it was received, byte for
// byte, including a field this command does not know.
func TestIssueBudgetWriteOutputJSON(t *testing.T) {
	for name, body := range map[string][]byte{
		"the fixture":              budgetFixture(t, "write-201-enable.json"),
		"with a field from later":  budgetReplace(t, budgetFixture(t, "write-201-enable.json"), `"replayed":`, `"added_later":{"x":[1,2]},"replayed":`),
		"with spaces and no final": []byte(`{ "replayed" : false, "result" : ` + string(budgetReplace(t, budgetFixture(t, "write-201-enable.json"), `{"replayed":false,"result":`, ``))),
	} {
		t.Run(name, func(t *testing.T) {
			body = bytes.TrimSuffix(body, []byte("\n"))
			if name == "the fixture" {
				body = append(body, '\n')
			}
			fake := newBudgetFake(t)
			fake.answer("POST", budgetTestEnablePath, 201, body)
			run := runBudgetCLI(t, append(append([]string{}, budgetEnableArgs...), "--output", "json")...)
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			if run.stdout != string(body) {
				t.Fatalf("stdout is not the answer as it was received:\n got: %q\nwant: %q", run.stdout, body)
			}
		})
	}

	// With --output json too, a body is printed only when it is the answer to a
	// write that is committed. Whoever reads stdout must never be handed the body
	// of a refusal or of an unknown outcome as if it were a result.
	enable := budgetFixture(t, "write-201-enable.json")
	for name, answer := range map[string]struct {
		status int
		body   []byte
		stderr string
	}{
		"a refusal":                        {409, budgetErrorJSON("already_enabled"), "refused (already_enabled): "},
		"a 401":                            {401, []byte(`{"error":"invalid or expired token"}`), "refused (401): "},
		"a 500":                            {500, budgetErrorJSON("internal_error"), "outcome unknown: "},
		"a 201 cut in the middle":          {201, enable[:len(enable)/2], "outcome unknown: "},
		"a 201 of another kind":            {201, budgetRootWriteAnswer(t, "close", false), "outcome unknown: "},
		"a 201 without the mode":           {201, budgetReplace(t, enable, `"mode":"observe",`, ``), "outcome unknown: "},
		"a 200 that does not say replayed": {200, enable, "outcome unknown: "},
	} {
		t.Run("nothing is printed for "+name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("POST", budgetTestEnablePath, answer.status, answer.body)
			run := runBudgetCLI(t, append(append([]string{}, budgetEnableArgs...), "--output", "json")...)
			if run.exit == 0 || run.stdout != "" {
				t.Errorf("exit %d, stdout %q: want an error and nothing printed", run.exit, run.stdout)
			}
			if !strings.HasPrefix(run.stderr, answer.stderr) {
				t.Errorf("stderr %q: want it to start with %q", run.stderr, answer.stderr)
			}
			fake.wantRequests("POST " + budgetTestEnablePath)
		})
	}
}

// budgetRefusalCodes is every code of design 12.3 with the status it comes
// with and the exit code main gives that status.
var budgetRefusalCodes = []struct {
	code   string
	status int
	exit   int
}{
	{"invalid_request", 400, 5},
	{"budget_forbidden", 403, 3},
	{"not_found", 404, 4},
	{"not_eligible", 422, 5},
	{"already_enabled", 409, 1},
	{"root_conflict", 409, 1},
	{"parent_cycle", 409, 1},
	{"operation_conflict", 409, 1},
	{"policy_revision_conflict", 409, 1},
	{"root_deleted", 409, 1},
	{"state_conflict", 409, 1},
}

// A refusal is a 401, or a 4xx with the error body of 12.9 and a code of 12.3.
// The command prints the code and what it means, says that nothing was written
// and sends nothing more. Every write is run against every code: a command
// reports a code the same way whether or not the server gives it to that
// command today.
func TestIssueBudgetWriteRefused(t *testing.T) {
	for _, write := range budgetWriteCases {
		meanings := map[string]string{}
		for _, refusal := range budgetRefusalCodes {
			t.Run(write.name+" refused with "+refusal.code, func(t *testing.T) {
				fake := newBudgetFake(t)
				fake.answer("POST", write.path, refusal.status, budgetErrorJSON(refusal.code))
				run := runBudgetCLI(t, write.args...)
				if run.exit != refusal.exit {
					t.Errorf("exit %d, want %d", run.exit, refusal.exit)
				}
				if run.stdout != "" {
					t.Errorf("stdout = %q, want nothing", run.stdout)
				}
				first, rest, _ := strings.Cut(run.stderr, "\n")
				meaning, found := strings.CutPrefix(first, "refused ("+refusal.code+"): ")
				meaning, said := strings.CutSuffix(meaning, ". Nothing was written.")
				if !found || !said || meaning == "" {
					t.Fatalf("stderr %q: want `refused (%s): <what it means>. Nothing was written.`", run.stderr, refusal.code)
				}
				meanings[meaning] = refusal.code
				if strings.Contains(run.stderr, "outcome unknown") {
					t.Errorf("a refusal is reported as an unknown outcome: %q", run.stderr)
				}
				if refusal.code == "policy_revision_conflict" {
					if !strings.Contains(rest, `"`+write.show+`"`) || !strings.Contains(rest, "decide again") {
						t.Errorf("stderr %q: want the operator sent to %q to decide again", run.stderr, write.show)
					}
				} else if rest != "" {
					t.Errorf("stderr has more than the refusal: %q", rest)
				}
				run.wantOnlyTypedIDs(t, write.typed()...)
				// One request: the command neither sends the write again nor
				// fetches the revision for the operator.
				fake.wantRequests("POST " + write.path)
			})
		}
		if len(meanings) != len(budgetRefusalCodes) {
			t.Errorf("%s: %d codes have %d different meanings: %v", write.name, len(budgetRefusalCodes), len(meanings), meanings)
		}

		for name, body := range map[string]string{
			"the body of the auth middleware": `{"error":"invalid or expired token"}` + "\n",
			"an empty body":                   "",
			"a page":                          "<html><body>401 Unauthorized</body></html>",
		} {
			t.Run(write.name+" refused with 401 and "+name, func(t *testing.T) {
				fake := newBudgetFake(t)
				fake.answer("POST", write.path, 401, []byte(body))
				run := runBudgetCLI(t, write.args...)
				if run.exit != 3 || run.stdout != "" {
					t.Errorf("exit %d, stdout %q: want exit 3 and nothing printed", run.exit, run.stdout)
				}
				if !strings.HasPrefix(run.stderr, "refused (401): ") || !strings.Contains(run.stderr, "Nothing was written.") {
					t.Errorf("stderr %q: want `refused (401): ...` and that nothing was written", run.stderr)
				}
				run.wantOnlyTypedIDs(t, write.typed()...)
				fake.wantRequests("POST " + write.path)
			})
		}

		// The status of a refusal is not compared with its code: a code of 12.3
		// under any 4xx is a refusal, also under a status the handlers do not
		// give it today. The exit code follows the status.
		for _, other := range []struct {
			name, code string
			status     int
			exit       int
		}{
			{"state_conflict under a 400", "state_conflict", 400, 5},
			{"invalid_request under a 409", "invalid_request", 409, 1},
			{"not_found under a 410", "not_found", 410, 1},
		} {
			t.Run(write.name+" refused with "+other.name, func(t *testing.T) {
				fake := newBudgetFake(t)
				fake.answer("POST", write.path, other.status, budgetErrorJSON(other.code))
				run := runBudgetCLI(t, write.args...)
				if run.exit != other.exit || run.stdout != "" {
					t.Errorf("exit %d, stdout %q: want exit %d and nothing printed", run.exit, run.stdout, other.exit)
				}
				if !strings.HasPrefix(run.stderr, "refused ("+other.code+"): ") || !strings.Contains(run.stderr, "Nothing was written.") {
					t.Errorf("stderr %q: want `refused (%s): ...` and that nothing was written", run.stderr, other.code)
				}
				run.wantOnlyTypedIDs(t, write.typed()...)
				fake.wantRequests("POST " + write.path)
			})
		}
	}
}

// wantBudgetUnknownOutcome checks what the operator is told when the outcome of
// a write is not known: the words "outcome unknown", the command that asks by
// the key they typed, the reminder to keep the key and the flags; never the
// word "failed", never another key, nothing on stdout, exit 1.
func wantBudgetUnknownOutcome(t *testing.T, run budgetRun, byKey string, typed ...string) {
	t.Helper()
	if run.exit != 1 {
		t.Errorf("exit %d, want 1", run.exit)
	}
	if run.stdout != "" {
		t.Errorf("stdout = %q, want nothing", run.stdout)
	}
	if !strings.HasPrefix(run.stderr, "outcome unknown: ") {
		t.Errorf("stderr %q: want it to start with `outcome unknown: `", run.stderr)
	}
	if !strings.Contains("\n"+run.stderr+"\n", "\n  "+byKey+"\n") {
		t.Errorf("stderr %q: want the line `  %s`", run.stderr, byKey)
	}
	if !strings.Contains(run.stderr, "Keep the original --request-key and the exact flags") {
		t.Errorf("stderr %q: want the reminder to keep the key and the flags", run.stderr)
	}
	if !strings.Contains(run.stderr, `answers "unknown", run this very same command again`) {
		t.Errorf("stderr %q: want the way forward when the query answers unknown", run.stderr)
	}
	lower := strings.ToLower(run.stderr)
	for _, word := range []string{"fail", "new key", "another key", "fresh key", "different key", "refused"} {
		if strings.Contains(lower, word) {
			t.Errorf("stderr %q: must not say %q", run.stderr, word)
		}
	}
	run.wantOnlyTypedIDs(t, typed...)
}

// Everything that is neither a commit nor a refusal is an unknown outcome. The
// request went out once and is not sent again.
func TestIssueBudgetWriteOutcomeUnknown(t *testing.T) {
	const strangerID = "11111111-2222-4333-8444-555555555555"
	plan := func(status int, body func(t *testing.T, write budgetWriteCase) []byte) func(*testing.T, *budgetFake, budgetWriteCase) context.Context {
		return func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", write.path, status, body(t, write))
			return context.Background()
		}
	}
	text := func(body string) func(*testing.T, budgetWriteCase) []byte {
		return func(*testing.T, budgetWriteCase) []byte { return []byte(body) }
	}
	changed := func(old, new string) func(*testing.T, budgetWriteCase) []byte {
		return func(t *testing.T, write budgetWriteCase) []byte { return budgetReplace(t, write.answer(t), old, new) }
	}
	for _, tc := range []struct {
		name     string
		rootOnly bool // only for a write that is addressed by a root
		plan     func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context
	}{
		{name: "the connection is closed without an answer", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.handle("POST", write.path, func(w http.ResponseWriter, _ *http.Request) {
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Errorf("hijack: %v", err)
					return
				}
				_ = connection.Close()
			})
			return context.Background()
		}},
		// The stand-in for a timeout that needs no clock: the operator's side
		// gives up while the server still holds the request.
		{name: "the wait ends while the server holds the request", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			fake.handle("POST", write.path, func(_ http.ResponseWriter, r *http.Request) {
				cancel()
				<-r.Context().Done()
			})
			return ctx
		}},
		{name: "a 500 with the error shape", plan: plan(500, text(string(budgetErrorJSON("internal_error"))))},
		{name: "a 500 with a code of a refusal", plan: plan(500, text(string(budgetErrorJSON("not_found"))))},
		{name: "a 502 with an HTML body", plan: plan(502, text("<html><body><h1>502 Bad Gateway</h1><p>upstream request "+strangerID+" failed</p></body></html>"))},
		{name: "a 503 without a body", plan: plan(503, text(""))},
		{name: "a 201 with a body cut in the middle", plan: plan(201, func(t *testing.T, write budgetWriteCase) []byte {
			answer := write.answer(t)
			return answer[:len(answer)/2]
		})},
		{name: "a 201 whose body ends before its declared length", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			answer := write.answer(t)
			fake.handle("POST", write.path, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "100000")
				w.WriteHeader(201)
				_, _ = w.Write(answer)
			})
			return context.Background()
		}},
		{name: "a 201 without a body", plan: plan(201, text(""))},
		{name: "a 201 with two answers", plan: plan(201, func(t *testing.T, write budgetWriteCase) []byte {
			return append(write.answer(t), write.answer(t)...)
		})},
		{name: "a 201 longer than the command reads", plan: plan(201, func(t *testing.T, write budgetWriteCase) []byte {
			return append(write.answer(t), bytes.Repeat([]byte(" "), 2<<20)...)
		})},
		{name: "a 201 with the answer of another route", plan: plan(201, func(t *testing.T, _ budgetWriteCase) []byte {
			return budgetFixture(t, "show-open.json")
		})},
		{name: "a 201 whose result names another root", rootOnly: true, plan: plan(201, changed(budgetTestRoot, budgetTestRoot2))},
		{name: "a 201 whose result has another kind", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", write.path, 201, budgetReplace(t, write.answer(t), `"operation_kind":"`+write.kind+`"`, `"operation_kind":"`+write.other+`"`))
			return context.Background()
		}},
		{name: "a 201 whose result has a kind of a later server", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", write.path, 201, budgetReplace(t, write.answer(t), `"operation_kind":"`+write.kind+`"`, `"operation_kind":"`+write.kind+`_v2"`))
			return context.Background()
		}},
		{name: "a 201 with a revision as a number", plan: plan(201, changed(`"policy_epoch":"1"`, `"policy_epoch":1`))},
		{name: "a 201 with an amount as a number", plan: plan(201, changed(`"limit_ticks":`, `"limit_ticks":250000000000,"x":`))},
		{name: "a 201 without the mode", plan: plan(201, changed(`"mode":"observe",`, ``))},
		{name: "a 201 without the limit", plan: plan(201, changed(`"limit_ticks":`, `"limit":`))},
		{name: "a 201 with a state of null", plan: plan(201, changed(`"admission_state":"open"`, `"admission_state":null`))},
		{name: "a 201 without replayed", plan: plan(201, changed(`"replayed":false,`, ``))},
		{name: "a 201 with replayed as a word", plan: plan(201, changed(`"replayed":false`, `"replayed":"false"`))},
		{name: "a 201 with the mode twice", plan: plan(201, changed(`"mode":"observe",`, `"mode":"observe","mode":"observe",`))},
		{name: "a 200 that does not say replayed", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", write.path, 200, write.answer(t))
			return context.Background()
		}},
		{name: "a 202", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", write.path, 202, write.answer(t))
			return context.Background()
		}},
		{name: "a 204", plan: plan(204, text(""))},
		{name: "a 302 to a route that would say committed", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("GET", "/elsewhere", 201, write.answer(t))
			fake.answer("POST", "/elsewhere", 201, write.answer(t))
			fake.handle("POST", write.path, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "/elsewhere")
				w.WriteHeader(302)
			})
			return context.Background()
		}},
		{name: "a 307 to a route that would say committed", plan: func(t *testing.T, fake *budgetFake, write budgetWriteCase) context.Context {
			fake.answer("POST", "/elsewhere", 201, write.answer(t))
			fake.handle("POST", write.path, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "/elsewhere")
				w.WriteHeader(307)
			})
			return context.Background()
		}},
		{name: "a 404 with the body of another handler", plan: plan(404, text(`{"error":"issue not found"}`+"\n"))},
		{name: "a 404 that is plain text", plan: plan(404, text("404 page not found\n"))},
		{name: "a 403 without a code", plan: plan(403, text(`{"error":"not a member of this workspace"}`+"\n"))},
		// The two answers of the workspace membership check in front of the
		// handlers, byte for byte (internal/middleware/workspace.go). The first
		// has both keys of the error shape and is still not an answer of the
		// budget handlers: its two values differ, and its code is not one of 12.3.
		{name: "the 404 of the workspace membership check", plan: plan(404, text(`{"error":"workspace not found","code":"workspace_access_denied"}`))},
		{name: "the 403 of the workspace role check", plan: plan(403, text(`{"error":"insufficient permissions"}`))},
		// A code that 12.3 does not have is no refusal under a 404 either: a 404
		// is not read as not_found on the strength of its status.
		{name: "a 404 with a code this command does not know", plan: plan(404, text(string(budgetErrorJSON("brand_new_code"))))},
		{name: "a 400 from in front of the handler", plan: plan(400, text(`{"error":"workspace `+strangerID+` is not valid"}`+"\n"))},
		{name: "a 409 with a code this command does not know", plan: plan(409, text(string(budgetErrorJSON("brand_new_code"))))},
		{name: "a 409 with internal_error as its code", plan: plan(409, text(string(budgetErrorJSON("internal_error"))))},
		{name: "a 409 whose two codes differ", plan: plan(409, text(`{"error":"state_conflict","code":"not_found"}`+"\n"))},
		{name: "a 409 with a code and no error", plan: plan(409, text(`{"code":"state_conflict"}`+"\n"))},
		{name: "a 409 with a code that is null", plan: plan(409, text(`{"error":"state_conflict","code":null}`+"\n"))},
		{name: "a 429", plan: plan(429, text(`{"error":"rate limited"}`+"\n"))},
	} {
		for _, write := range budgetWriteCases {
			if tc.rootOnly && write.root == "" {
				continue
			}
			t.Run(write.name+": "+tc.name, func(t *testing.T) {
				fake := newBudgetFake(t)
				ctx := tc.plan(t, fake, write)
				run := runBudgetCLIContext(t, ctx, write.args...)
				wantBudgetUnknownOutcome(t, run, write.byKey, write.typed()...)
				fake.wantRequests("POST " + write.path)
			})
		}
	}

	// An enable has no root to compare, so the result of any root belongs to
	// it; but it must be the result of an enable, activation time included.
	t.Run("enable: a 201 without the activation time", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("POST", budgetTestEnablePath, 201, budgetReplace(t, budgetFixture(t, "write-201-enable.json"), `"activated_at":"2026-11-19T01:02:03.456789Z",`, ``))
		run := runBudgetCLI(t, budgetEnableArgs...)
		wantBudgetUnknownOutcome(t, run, budgetWriteCases[0].byKey, budgetWriteCases[0].typed()...)
		fake.wantRequests("POST " + budgetTestEnablePath)
	})
}

// What the transport says about an error is not shown to the operator: Go
// words a certificate that is not trusted as "failed to verify certificate",
// and an unknown outcome must never read as a failure.
func TestIssueBudgetUnknownOutcomeDoesNotRepeatTheTransport(t *testing.T) {
	fake := newBudgetFake(t)
	var reached atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshake is logged by the server
	server.StartTLS()
	t.Cleanup(server.Close)
	t.Setenv("MULTICA_SERVER_URL", server.URL)

	// The premise: a client that does not know the certificate gets an error
	// that says "failed".
	plain := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{}}}
	if _, err := plain.Get(server.URL); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("the premise of this test does not hold: the transport error is %v", err)
	}

	write := budgetWriteCases[3]
	run := runBudgetCLI(t, write.args...)
	wantBudgetUnknownOutcome(t, run, write.byKey, write.typed()...)
	if strings.Contains(run.stderr, "certificate") || strings.Contains(run.stderr, "x509") {
		t.Errorf("stderr repeats the transport: %q", run.stderr)
	}
	if reached.Load() != 0 {
		t.Errorf("the server was reached %d time(s) through a handshake that cannot complete", reached.Load())
	}
	fake.wantRequests()
}

// The command that asks by the key is one the operator can run as it is: it
// repeats the flags they typed that choose the server, the workspace and the
// profile, and it quotes an issue the shell would split.
func TestIssueBudgetByKeyCommandIsRunnable(t *testing.T) {
	t.Run("the flags that choose where the request goes", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("POST", budgetTestOperationsPath, 500, budgetErrorJSON("internal_error"))
		saveBudgetProfile(t, "person", budgetTestToken)
		serverURL := mustBudgetEnv(t, "MULTICA_SERVER_URL")
		const workspace = "0f1e2d3c-4b5a-4968-8778-695a4b3c2d1e"
		t.Setenv("MULTICA_SERVER_URL", "")
		t.Setenv("MULTICA_WORKSPACE_ID", "")
		args := append(append([]string{}, budgetCloseArgs...), "--profile", "person", "--server-url", serverURL, "--workspace-id", workspace)
		run := runBudgetCLI(t, args...)
		byKey := "multica issue budget operation --root " + budgetTestRoot + " --request-key " + budgetTestCloseKey +
			" --profile person --server-url " + serverURL + " --workspace-id " + workspace
		wantBudgetUnknownOutcome(t, run, byKey, budgetTestRoot, budgetTestCloseKey, workspace)
		fake.wantRequests("POST " + budgetTestOperationsPath)
		if got := fake.seen()[0].Header.Get("X-Workspace-ID"); got != workspace {
			t.Errorf("X-Workspace-ID = %q, want the one that was typed", got)
		}
	})

	t.Run("an issue the shell would split", func(t *testing.T) {
		const issue = "it's #1"
		fake := newBudgetFake(t)
		fake.answer("POST", "/api/issues/it%27s%20%231/budget", 500, budgetErrorJSON("internal_error"))
		run := runBudgetCLI(t, append([]string{"enable", issue}, budgetEnableArgs[2:]...)...)
		wantBudgetUnknownOutcome(t, run, `multica issue budget operation --issue 'it'\''s #1' --request-key `+budgetTestEnableKey, budgetTestEnableKey)
		fake.wantRequests("POST /api/issues/it%27s%20%231/budget")
	})
}

func mustBudgetEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is not set", name)
	}
	return value
}

// budgetOneAnswer stands in for the transport: it gives every request the same
// answer and keeps the requests, so that a test sees what net/http was handed
// without a connection.
type budgetOneAnswer struct {
	status   int
	requests []*http.Request
}

func (a *budgetOneAnswer) RoundTrip(req *http.Request) (*http.Response, error) {
	a.requests = append(a.requests, req)
	return &http.Response{
		StatusCode: a.status,
		Header:     http.Header{"Location": []string{"/api/elsewhere"}},
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

// A write reaches the wire once, also below this command. net/http puts the
// body of a POST on the wire a second time only when it can get that body
// again through GetBody: when an Idempotency-Key header marks the request as
// safe to repeat and a kept connection broke, over HTTP/2 when the peer refused
// or reset the stream, and to follow a 307 or a 308. The request of a write
// gives it none of that, whatever the answer is.
func TestIssueBudgetWriteGivesTheTransportNothingToReplay(t *testing.T) {
	for _, status := range []int{201, 302, 307, 308, 500} {
		transport := &budgetOneAnswer{status: status}
		client := &cli.APIClient{
			BaseURL:     "http://budget.invalid",
			WorkspaceID: budgetTestWorkspace,
			Token:       budgetTestToken,
			HTTPClient:  &http.Client{Transport: transport},
		}
		answer, err := budgetRequest(context.Background(), client, http.MethodPost, budgetTestOperationsPath, []byte(`{"kind":"close"}`))
		if err != nil || answer.status != status {
			t.Fatalf("HTTP %d: status %d, error %v: want the answer as it came", status, answer.status, err)
		}
		if len(transport.requests) != 1 {
			t.Fatalf("HTTP %d: net/http was handed %d requests, want 1", status, len(transport.requests))
		}
		sent := transport.requests[0]
		if sent.Method != http.MethodPost || sent.Body == nil || sent.Body == http.NoBody || sent.ContentLength == 0 {
			t.Errorf("HTTP %d: %s with a body of %d bytes, want the POST with its body", status, sent.Method, sent.ContentLength)
		}
		if sent.GetBody != nil {
			t.Errorf("HTTP %d: the write carries GetBody, so net/http may send its body again", status)
		}
		for _, name := range []string{"Idempotency-Key", "X-Idempotency-Key"} {
			if values := sent.Header.Values(name); len(values) != 0 {
				t.Errorf("HTTP %d: header %s = %q marks the write as safe to repeat", status, name, values)
			}
		}
	}
}
