package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Tests of the commands that read a budget root: show, operation, observations
// and attempts. The harness is in cmd_issue_budget_test.go.

const (
	budgetTestBindingPath      = "/api/issues/" + budgetTestIssue + "/budget"
	budgetTestObservationsPath = budgetTestRootPath + "/observations"
	budgetTestAttemptsPath     = budgetTestRootPath + "/attempts"

	budgetHalfDollarReserve = "ATTEMPT RESERVE | 0.50 USD (5000000000 ticks) for each execution; a declared reservation, not an estimate and not a cap"
)

// budgetNotCounted is the three names of the fixtures that a root does not
// count.
var budgetNotCounted = []string{
	"NOT COUNTED | task_without_issue; not counted, which is not a zero cost",
	"NOT COUNTED | collaboration_host_run; not counted, which is not a zero cost",
	"NOT COUNTED | plugin_external_call; not counted, which is not a zero cost",
}

// budgetShowTables is what `show` prints for each show fixture, line by line.
var budgetShowTables = []struct {
	fixture string
	root    string
	lines   []string
}{
	{"show-open.json", budgetTestRoot, []string{
		"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
		"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
		"STATE | open",
		"MODE | observe",
		"POLICY REVISION | 3",
		"POLICY EPOCH | 1",
		"LEDGER REVISION | 42",
		"ACTIVATED AT | 2026-11-19T01:02:03.456789Z",
		"SETTLED AS OF REVISION | not settled",
		"LIMIT | 25.00 USD (250000000000 ticks)",
		budgetHalfDollarReserve,
		"LEDGER | available",
		"ACTUAL KNOWN (A) | 7.35 USD (73500000000 ticks)",
		"ESTIMATED (E) | 0.12 USD (1200000000 ticks); outside A and outside the limit",
		"RESERVED (R) | 1.00 USD (10000000000 ticks)",
		"UNKNOWN HOLD (H) | 0.50 USD (5000000000 ticks)",
		"OVERRUN | 0.00 USD (0 ticks)",
		"UNKNOWN | lost_contact: 1",
		"UNKNOWN | observation_off: 2",
		"UNKNOWN BEFORE ACTIVATION | 1",
		"TOTAL KNOWN | no",
		"REMAINING | unknown; not everything is known",
		"EXECUTIONS INCLUDED | 12",
	}},
	{"show-known.json", budgetTestRoot, []string{
		"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
		"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
		"STATE | open",
		"MODE | observe",
		"POLICY REVISION | 3",
		"POLICY EPOCH | 1",
		"LEDGER REVISION | 57",
		"ACTIVATED AT | 2026-11-19T01:02:03.456789Z",
		"SETTLED AS OF REVISION | not settled",
		"LIMIT | 25.00 USD (250000000000 ticks)",
		budgetHalfDollarReserve,
		"LEDGER | available",
		"ACTUAL KNOWN (A) | 7.35 USD (73500000000 ticks)",
		"ESTIMATED (E) | 0.12 USD (1200000000 ticks); outside A and outside the limit",
		"RESERVED (R) | 1.00 USD (10000000000 ticks)",
		"UNKNOWN HOLD (H) | 0.00 USD (0 ticks)",
		"OVERRUN | 0.03 USD (300000000 ticks)",
		"UNKNOWN | none",
		"UNKNOWN BEFORE ACTIVATION | 0",
		"TOTAL KNOWN | yes",
		"REMAINING | 16.65 USD (166500000000 ticks)",
		"EXECUTIONS INCLUDED | 12",
	}},
	{"show-no-limit.json", budgetTestRoot2, []string{
		"BUDGET ROOT | b3a81f6d-2c47-4e95-8d0a-9f1e7c5b3a26",
		"ROOT ISSUE | 8c1d5e9f-0a2b-4c3d-9e4f-5a6b7c8d9e0f",
		"STATE | open",
		"MODE | observe",
		"POLICY REVISION | 1",
		"POLICY EPOCH | 1",
		"LEDGER REVISION | 9",
		"ACTIVATED AT | 2026-11-19T02:15:40Z",
		"SETTLED AS OF REVISION | not settled",
		"LIMIT | not set",
		"ATTEMPT RESERVE | not set",
		"LEDGER | available",
		"ACTUAL KNOWN (A) | 0.42 USD (4200000000 ticks)",
		"ESTIMATED (E) | 0.00 USD (0 ticks); outside A and outside the limit",
		"RESERVED (R) | 0.00 USD (0 ticks)",
		"UNKNOWN HOLD (H) | 0.00 USD (0 ticks)",
		"OVERRUN | 0.00 USD (0 ticks)",
		"UNKNOWN | none",
		"UNKNOWN BEFORE ACTIVATION | 0",
		"TOTAL KNOWN | yes",
		"REMAINING | unknown; no limit is set",
		"EXECUTIONS INCLUDED | 3",
	}},
	{"show-unavailable.json", budgetTestRoot, []string{
		"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
		"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
		"STATE | open",
		"MODE | observe",
		"POLICY REVISION | 3",
		"POLICY EPOCH | 1",
		"LEDGER REVISION | 42",
		"ACTIVATED AT | 2026-11-19T01:02:03.456789Z",
		"SETTLED AS OF REVISION | not settled",
		"LIMIT | 25.00 USD (250000000000 ticks)",
		budgetHalfDollarReserve,
		"LEDGER | unavailable; the ledger cannot be read, so every amount is unknown",
		"ACTUAL KNOWN (A) | unknown",
		"ESTIMATED (E) | unknown; outside A and outside the limit",
		"RESERVED (R) | unknown",
		"UNKNOWN HOLD (H) | unknown",
		"OVERRUN | unknown",
		"UNKNOWN | ledger_unreadable: 3",
		"UNKNOWN | observation_off: 2",
		"UNKNOWN BEFORE ACTIVATION | 1",
		"TOTAL KNOWN | no",
		"REMAINING | unknown",
		"EXECUTIONS INCLUDED | unknown",
	}},
	{"show-tombstoned-off.json", budgetTestRoot, []string{
		"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
		"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
		"STATE | tombstoned; the root issue was deleted",
		"MODE | off; nothing is recorded while observation is off",
		"POLICY REVISION | 4",
		"POLICY EPOCH | 1",
		"LEDGER REVISION | 42",
		"ACTIVATED AT | 2026-11-19T01:02:03.456789Z",
		"SETTLED AS OF REVISION | not settled",
		"LIMIT | 25.00 USD (250000000000 ticks)",
		budgetHalfDollarReserve,
		"LEDGER | available",
		"ACTUAL KNOWN (A) | 7.35 USD (73500000000 ticks)",
		"ESTIMATED (E) | 0.12 USD (1200000000 ticks); outside A and outside the limit",
		"RESERVED (R) | 1.00 USD (10000000000 ticks)",
		"UNKNOWN HOLD (H) | 0.50 USD (5000000000 ticks)",
		"OVERRUN | 0.00 USD (0 ticks)",
		"UNKNOWN | lost_contact: 1",
		"UNKNOWN | observation_off: 2",
		"UNKNOWN BEFORE ACTIVATION | 1",
		"TOTAL KNOWN | no",
		"REMAINING | unknown; not everything is known",
		"EXECUTIONS INCLUDED | 12",
	}},
}

// budgetAmountLabels are the lines of `show` that hold an amount of the
// ledger.
var budgetAmountLabels = []string{"ACTUAL KNOWN (A)", "ESTIMATED (E)", "RESERVED (R)", "UNKNOWN HOLD (H)", "OVERRUN", "REMAINING"}

// showBudgetRoot runs `show --root` against one answer and returns what it
// printed.
func showBudgetRoot(t *testing.T, root string, body []byte) budgetRun {
	t.Helper()
	fake := newBudgetFake(t)
	path := "/api/issue-budgets/" + root
	fake.answer("GET", path, 200, body)
	run := runBudgetCLI(t, "show", "--root", root)
	fake.wantRequests("GET " + path)
	return run
}

// `show` prints a root line by line (rule 6 of the brief): what it is, its
// policy, each amount on a line of its own, each reason of an unknown on a line
// of its own, the unknown from before activation apart from them, and what the
// root does not count.
func TestIssueBudgetShowTable(t *testing.T) {
	for _, tc := range budgetShowTables {
		t.Run(tc.fixture, func(t *testing.T) {
			data := budgetFixture(t, tc.fixture)
			run := showBudgetRoot(t, tc.root, data)
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			wantBudgetOutput(t, run.stdout, append(append([]string{}, tc.lines...), budgetNotCounted...)...)

			// The remaining amount is a number only when the answer gives one.
			remaining := budgetLine(t, run.stdout, "REMAINING")
			null := bytes.Contains(data, []byte(`"remaining_ticks":null`))
			if null && remaining != "unknown" && !strings.HasPrefix(remaining, "unknown; ") {
				t.Errorf("REMAINING = %q for a remaining amount of null, want unknown", remaining)
			}
			if !null && strings.Contains(remaining, "unknown") {
				t.Errorf("REMAINING = %q for a remaining amount that is given", remaining)
			}
			if strings.ContainsAny(remaining, "0123456789") == null {
				t.Errorf("REMAINING = %q: a number is printed exactly when the answer gives one (null: %v)", remaining, null)
			}
		})
	}
}

// With a ledger that cannot be read no amount line holds a digit, and one line
// says why.
func TestIssueBudgetShowWithoutALedgerShowsNoAmount(t *testing.T) {
	run := showBudgetRoot(t, budgetTestRoot, budgetFixture(t, "show-unavailable.json"))
	for _, label := range budgetAmountLabels {
		value := budgetLine(t, run.stdout, label)
		if strings.ContainsAny(value, "0123456789") {
			t.Errorf("%s = %q: holds a digit although the ledger cannot be read", label, value)
		}
		if !strings.HasPrefix(value, "unknown") {
			t.Errorf("%s = %q, want unknown", label, value)
		}
	}
	if count := strings.Count(run.stdout, "the ledger cannot be read"); count != 1 {
		t.Errorf("%d line(s) say that the ledger cannot be read, want one:\n%s", count, run.stdout)
	}
	if got := budgetLine(t, run.stdout, "EXECUTIONS INCLUDED"); got != "unknown" {
		t.Errorf("EXECUTIONS INCLUDED = %q for null, want unknown", got)
	}
	// What a manager set is still shown: it is not an amount of the ledger.
	if got := budgetLine(t, run.stdout, "LIMIT"); got != "25.00 USD (250000000000 ticks)" {
		t.Errorf("LIMIT = %q", got)
	}
}

// Closing a root, switching its observation off or deleting its root issue
// changes no amount: the same amounts are printed as before.
func TestIssueBudgetShowKeepsTheAmountsOfARootThatIsOff(t *testing.T) {
	before := showBudgetRoot(t, budgetTestRoot, budgetFixture(t, "show-open.json"))
	after := showBudgetRoot(t, budgetTestRoot, budgetFixture(t, "show-tombstoned-off.json"))
	for _, label := range append([]string{"LIMIT", "ATTEMPT RESERVE", "UNKNOWN BEFORE ACTIVATION", "EXECUTIONS INCLUDED"}, budgetAmountLabels...) {
		if was, is := budgetLine(t, before.stdout, label), budgetLine(t, after.stdout, label); was != is {
			t.Errorf("%s: %q before, %q after", label, was, is)
		}
	}
	if got := budgetLine(t, after.stdout, "STATE"); !strings.HasPrefix(got, "tombstoned") {
		t.Errorf("STATE = %q, want tombstoned", got)
	}
	if got := budgetLine(t, after.stdout, "MODE"); !strings.HasPrefix(got, "off") {
		t.Errorf("MODE = %q, want off", got)
	}
}

// `show <issue>` asks which root the issue belongs to and then shows that root.
func TestIssueBudgetShowByIssue(t *testing.T) {
	bindingLines := []string{
		"ISSUE | 5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25",
		"IS ROOT ISSUE | no",
		"BIND SOURCE | create",
		"BOUND AT | 2026-11-19T03:20:40.871536Z",
	}

	t.Run("two requests in order, the binding and then the root", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("GET", budgetTestBindingPath, 200, "issue-binding.json")
		fake.fixture("GET", budgetTestRootPath, 200, "show-open.json")
		run := runBudgetCLI(t, "show", budgetTestIssue)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		want := append(append(append([]string{}, bindingLines...), budgetShowTables[0].lines...), budgetNotCounted...)
		wantBudgetOutput(t, run.stdout, want...)
		fake.wantRequests("GET "+budgetTestBindingPath, "GET "+budgetTestRootPath)
	})

	t.Run("the root issue itself", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("GET", budgetTestBindingPath, 200, budgetReplace(t, budgetFixture(t, "issue-binding.json"), `"is_root_issue":false`, `"is_root_issue":true`))
		fake.fixture("GET", budgetTestRootPath, 200, "show-open.json")
		run := runBudgetCLI(t, "show", budgetTestIssue)
		if got := budgetLine(t, run.stdout, "IS ROOT ISSUE"); got != "yes" {
			t.Errorf("IS ROOT ISSUE = %q, want yes", got)
		}
	})

	// A 404 on the first request is the plain answer of an issue without a
	// root; nothing more is asked.
	for name, body := range map[string][]byte{
		"with the error body of the budget handlers": budgetFixture(t, "error-not-found.json"),
		"with the error body of the issue loader":    []byte(`{"error":"issue not found"}` + "\n"),
	} {
		t.Run("a 404 on the binding "+name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("GET", budgetTestBindingPath, 404, body)
			run := runBudgetCLI(t, "show", budgetTestIssue)
			if run.exit != 4 || run.stdout != "" {
				t.Errorf("exit %d, stdout %q: want exit 4 and nothing printed", run.exit, run.stdout)
			}
			if !strings.Contains(run.stderr, "belongs to no budget root, or is not visible") || !strings.Contains(run.stderr, budgetTestIssue) {
				t.Errorf("stderr %q: want that %s belongs to no budget root, or is not visible", run.stderr, budgetTestIssue)
			}
			if strings.Contains(run.stderr, "\n") {
				t.Errorf("stderr %q: want one plain line", run.stderr)
			}
			fake.wantRequests("GET " + budgetTestBindingPath)
		})
	}

	// The root of the second request comes from the first answer. It is used
	// only when that answer was understood and names a root.
	for name, binding := range map[string][]byte{
		"a binding without the root":                 budgetReplace(t, budgetFixture(t, "issue-binding.json"), `"budget_root_id":"`+budgetTestRoot+`",`, ``),
		"a binding with a root of null":              budgetReplace(t, budgetFixture(t, "issue-binding.json"), `"budget_root_id":"`+budgetTestRoot+`"`, `"budget_root_id":null`),
		"a binding with a root that is not an id":    budgetReplace(t, budgetFixture(t, "issue-binding.json"), budgetTestRoot, "../../admin"),
		"a binding with a root in upper case":        budgetReplace(t, budgetFixture(t, "issue-binding.json"), budgetTestRoot, strings.ToUpper(budgetTestRoot)),
		"a binding without is_root_issue":            budgetReplace(t, budgetFixture(t, "issue-binding.json"), `"is_root_issue":false,`, ``),
		"a binding that is another answer":           budgetFixture(t, "operation-unknown.json"),
		"a binding cut in the middle":                budgetFixture(t, "issue-binding.json")[:80],
		"a binding that is followed by another body": append(budgetFixture(t, "issue-binding.json"), budgetFixture(t, "issue-binding.json")...),
	} {
		t.Run(name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("GET", budgetTestBindingPath, 200, binding)
			run := runBudgetCLI(t, "show", budgetTestIssue)
			if run.exit != 1 || run.stdout != "" {
				t.Errorf("exit %d, stdout %q: want exit 1 and nothing printed", run.exit, run.stdout)
			}
			if !strings.Contains(run.stderr, "not understood") {
				t.Errorf("stderr %q: want the answer reported as not understood", run.stderr)
			}
			fake.wantRequests("GET " + budgetTestBindingPath)
		})
	}

	t.Run("the root that is shown is not the one the binding named", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("GET", budgetTestBindingPath, 200, "issue-binding.json")
		fake.fixture("GET", budgetTestRootPath, 200, "show-no-limit.json")
		run := runBudgetCLI(t, "show", budgetTestIssue)
		if run.exit != 1 || run.stdout != "" {
			t.Errorf("exit %d, stdout %q: want exit 1 and nothing printed", run.exit, run.stdout)
		}
		if !strings.Contains(run.stderr, "another budget root") {
			t.Errorf("stderr %q: want the answer reported as that of another budget root", run.stderr)
		}
		fake.wantRequests("GET "+budgetTestBindingPath, "GET "+budgetTestRootPath)
	})

	t.Run("refused on the binding", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("GET", budgetTestBindingPath, 403, budgetErrorJSON("budget_forbidden"))
		run := runBudgetCLI(t, "show", budgetTestIssue)
		if run.exit != 3 || run.stdout != "" || !strings.HasPrefix(run.stderr, "refused (budget_forbidden): ") {
			t.Errorf("exit %d, stdout %q, stderr %q: want exit 3 and `refused (budget_forbidden): ...`", run.exit, run.stdout, run.stderr)
		}
		fake.wantRequests("GET " + budgetTestBindingPath)
	})
}

// --output json prints the answer to a read as it was received, byte for byte.
// For `show <issue>` that is the answer about the root: the answer about the
// binding only says where to ask.
func TestIssueBudgetReadOutputJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		path    string
		fixture string
	}{
		{"show", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath, "show-open.json"},
		{"show without a ledger", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath, "show-unavailable.json"},
		{"show by issue", []string{"show", budgetTestIssue}, budgetTestRootPath, "show-known.json"},
		{"operation committed", []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestSetLimitKey}, budgetTestOperationsPath + "/" + budgetTestSetLimitKey, "operation-committed-stale.json"},
		{"operation unknown", []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestEnableKey}, budgetTestOperationsPath + "/" + budgetTestEnableKey, "operation-unknown.json"},
		{"observations", []string{"observations", "--root", budgetTestRoot}, budgetTestObservationsPath, "observations-page.json"},
		{"observations, empty", []string{"observations", "--root", budgetTestRoot}, budgetTestObservationsPath, "observations-empty.json"},
		{"attempts", []string{"attempts", "--root", budgetTestRoot}, budgetTestAttemptsPath, "attempts-page.json"},
		{"attempts without a ledger", []string{"attempts", "--root", budgetTestRoot}, budgetTestAttemptsPath, "attempts-unavailable.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.fixture("GET", budgetTestBindingPath, 200, "issue-binding.json")
			fake.fixture("GET", tc.path, 200, tc.fixture)
			run := runBudgetCLI(t, append(append([]string{}, tc.args...), "--output", "json")...)
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			if want := string(budgetFixture(t, tc.fixture)); run.stdout != want {
				t.Fatalf("stdout is not the answer as it was received:\n got: %q\nwant: %q", run.stdout, want)
			}
		})
	}

	t.Run("a field of a later server is kept", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-open.json"), `"mode":"observe",`, `"mode":"observe", "added_later" : {"deep":[1,2]},`)
		fake := newBudgetFake(t)
		fake.answer("GET", budgetTestRootPath, 200, body)
		run := runBudgetCLI(t, "show", "--root", budgetTestRoot, "--output", "json")
		if run.stdout != string(body) {
			t.Fatalf("stdout is not the answer as it was received:\n got: %q\nwant: %q", run.stdout, body)
		}
	})

	// json is a way of printing, not a way around reading: an answer that is
	// not understood is not passed on as if it were one.
	stale := budgetFixture(t, "operation-committed-stale.json")
	byKey := []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestSetLimitKey}
	byKeyPath := budgetTestOperationsPath + "/" + budgetTestSetLimitKey
	for _, tc := range []struct {
		name  string
		args  []string
		path  string
		body  []byte
		holds string
	}{
		{"show", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath,
			budgetReplace(t, budgetFixture(t, "show-open.json"), `"total_known":false`, `"total_known":null`), "not understood"},
		{"show, about another root", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath,
			budgetReplace(t, budgetFixture(t, "show-open.json"), `"budget_root_id":"`+budgetTestRoot+`"`, `"budget_root_id":"`+budgetTestRoot2+`"`), "another budget root"},
		{"show, amounts of null beside a ledger that can be read", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath,
			budgetReplace(t, budgetFixture(t, "show-unavailable.json"), `"ledger_state":"unavailable"`, `"ledger_state":"available"`), "not understood"},
		{"operation, a state of a later server", byKey, byKeyPath,
			budgetReplace(t, stale, `"state":"committed"`, `"state":"pending"`), "not understood"},
		{"operation, committed without stale", byKey, byKeyPath,
			budgetReplace(t, stale, `"stale":true,`, ``), "not understood"},
		{"operation, committed with a result without its mode", byKey, byKeyPath,
			budgetReplace(t, stale, `"mode":"observe",`, ``), "not understood"},
		{"operation, a committed enable without its activation time", byKey, byKeyPath,
			budgetReplace(t, budgetFixture(t, "operation-committed.json"), `"activated_at":"2026-11-19T01:02:03.456789Z",`, ``), "not understood"},
		{"operation, of another root", byKey, byKeyPath,
			budgetReplace(t, stale, `"budget_root_id":"`+budgetTestRoot+`"`, `"budget_root_id":"`+budgetTestRoot2+`"`), "another budget root"},
		{"operation, unknown without the path to try again", byKey, byKeyPath,
			budgetReplace(t, budgetFixture(t, "operation-unknown.json"), `,"retry_path":"`+budgetTestRootPath+`/operations/`+budgetTestEnableKey+`"`, ``), "not understood"},
		{"observations", []string{"observations", "--root", budgetTestRoot}, budgetTestObservationsPath,
			budgetReplace(t, budgetFixture(t, "observations-page.json"), `"covered":true`, `"covered":null`), "not understood"},
		{"attempts", []string{"attempts", "--root", budgetTestRoot}, budgetTestAttemptsPath,
			budgetReplace(t, budgetFixture(t, "attempts-page.json"), `"ledger_state":"available",`, ``), "not understood"},
	} {
		t.Run("an answer that is not understood is not printed: "+tc.name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("GET", tc.path, 200, tc.body)
			run := runBudgetCLI(t, append(append([]string{}, tc.args...), "--output", "json")...)
			if run.exit != 1 || run.stdout != "" || !strings.Contains(run.stderr, tc.holds) {
				t.Fatalf("exit %d, stdout %q, stderr %q: want exit 1, nothing printed and stderr to hold %q", run.exit, run.stdout, run.stderr, tc.holds)
			}
			fake.wantRequests("GET " + tc.path)
		})
	}
}

// Answers are read leniently: what a later server adds is ignored, and a value
// of a list of names that this command does not know is printed as it came.
func TestIssueBudgetShowReadsLeniently(t *testing.T) {
	plain := showBudgetRoot(t, budgetTestRoot, budgetFixture(t, "show-open.json"))

	t.Run("an extra field changes nothing", func(t *testing.T) {
		body := budgetFixture(t, "show-open.json")
		body = budgetReplace(t, body, `"mode":"observe",`, `"mode":"observe","added_later":{"deep":[1,2,{"x":null}]},`)
		body = budgetReplace(t, body, `"overrun_ticks":"0"}`, `"overrun_ticks":"0","refund_ticks":"7"}`)
		body = budgetReplace(t, body, `{"reason":"lost_contact","count":1}`, `{"reason":"lost_contact","count":1,"since":"2026-11-19T00:00:00Z"}`)
		body = budgetReplace(t, body, `"included_attempts":12,`, `"included_attempts":12,"excluded_attempts":4,`)
		body = budgetReplace(t, body, `"pre_activation":1}`, `"pre_activation":1,"post_activation":null}`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		if run.exit != 0 || run.stdout != plain.stdout {
			t.Fatalf("exit %d; the table changed:\n got:\n%s\nwant:\n%s", run.exit, run.stdout, plain.stdout)
		}
	})

	for _, tc := range []struct {
		name, old, new, label, want string
	}{
		{"a mode of a later server", `"mode":"observe"`, `"mode":"enforce"`, "MODE", "enforce"},
		{"a state of a later server", `"admission_state":"open"`, `"admission_state":"paused"`, "STATE", "paused"},
		{"a ledger state of a later server", `"ledger_state":"available"`, `"ledger_state":"rebuilding"`, "LEDGER", "rebuilding"},
	} {
		t.Run(tc.name+" is printed as it came", func(t *testing.T) {
			run := showBudgetRoot(t, budgetTestRoot, budgetReplace(t, budgetFixture(t, "show-open.json"), tc.old, tc.new))
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			if got := budgetLine(t, run.stdout, tc.label); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.label, got, tc.want)
			}
		})
	}

	t.Run("a reason and a name of a later server are printed as they came", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-open.json"), `"reason":"lost_contact"`, `"reason":"cosmic_rays"`)
		body = budgetReplace(t, body, `"plugin_external_call"`, `"something_new"`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		cells := "\n" + strings.Join(budgetCells(run.stdout), "\n") + "\n"
		for _, line := range []string{"UNKNOWN | cosmic_rays: 1", "NOT COUNTED | something_new; not counted, which is not a zero cost"} {
			if !strings.Contains(cells, "\n"+line+"\n") {
				t.Errorf("want the line %q in:\n%s", line, run.stdout)
			}
		}
	})

	// A ledger state this command does not know is not taken for "available":
	// without amounts every amount is unknown.
	t.Run("a ledger state of a later server without amounts", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-unavailable.json"), `"ledger_state":"unavailable"`, `"ledger_state":"rebuilding"`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		if run.exit != 0 {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		for _, label := range budgetAmountLabels {
			if value := budgetLine(t, run.stdout, label); !strings.HasPrefix(value, "unknown") {
				t.Errorf("%s = %q, want unknown", label, value)
			}
		}
	})

	// The remaining amount is the server's to give. When it gives none and the
	// answer does not say why, the line says unknown and nothing more: the
	// command does not work it out from the limit and the amounts.
	t.Run("a remaining amount of null that the answer does not explain", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-known.json"), `"remaining_ticks":"166500000000"`, `"remaining_ticks":null`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		if got := budgetLine(t, run.stdout, "REMAINING"); run.exit != 0 || got != "unknown" {
			t.Errorf("exit %d, REMAINING = %q, want unknown", run.exit, got)
		}
	})

	// Nor is it taken for "unavailable": the amounts that came with it are
	// printed as they came.
	t.Run("a ledger state of a later server keeps the amounts it came with", func(t *testing.T) {
		run := showBudgetRoot(t, budgetTestRoot, budgetReplace(t, budgetFixture(t, "show-open.json"), `"ledger_state":"available"`, `"ledger_state":"rebuilding"`))
		if run.exit != 0 {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		for _, label := range append([]string{"EXECUTIONS INCLUDED"}, budgetAmountLabels...) {
			if was, is := budgetLine(t, plain.stdout, label), budgetLine(t, run.stdout, label); was != is {
				t.Errorf("%s: %q with a ledger that is available, %q with one that is rebuilding", label, was, is)
			}
		}
	})

	// A list without a name is said in words: the line a reader looks for is
	// never missing.
	t.Run("an empty list of what is not counted prints none", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-open.json"), `"not_applicable":["task_without_issue","collaboration_host_run","plugin_external_call"]`, `"not_applicable":[]`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		if got := budgetLine(t, run.stdout, "NOT COUNTED"); run.exit != 0 || got != "none" {
			t.Errorf("exit %d, NOT COUNTED = %q, want none", run.exit, got)
		}
	})

	t.Run("a text that would move the cursor is shown quoted", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "show-open.json"), `"mode":"observe"`, `"mode":"obs\u001b[2Jerve\nSTATE closed"`)
		run := showBudgetRoot(t, budgetTestRoot, body)
		if strings.ContainsAny(run.stdout, "\x1b") || strings.Count(run.stdout, "\nSTATE") != 1 {
			t.Fatalf("a text of the answer reaches the terminal as it came:\n%q", run.stdout)
		}
		if got := budgetLine(t, run.stdout, "MODE"); got != `"obs\x1b[2Jerve\nSTATE closed"` {
			t.Errorf("MODE = %s", got)
		}
	})
}

// wantBudgetNotUnderstood checks how a read reports an answer it cannot use:
// an error that names the field, exit 1, and nothing of the answer printed.
func wantBudgetNotUnderstood(t *testing.T, run budgetRun, field string) {
	t.Helper()
	if run.exit != 1 {
		t.Errorf("exit %d, want 1", run.exit)
	}
	if run.stdout != "" {
		t.Errorf("stdout = %q: nothing of an answer that is not understood may be printed", run.stdout)
	}
	if !strings.Contains(run.stderr, "not understood") {
		t.Errorf("stderr %q: want the answer reported as not understood", run.stderr)
	}
	if field != "" && !strings.Contains(run.stderr, `"`+field+`"`) {
		t.Errorf("stderr %q: want the field %q named", run.stderr, field)
	}
}

// A missing field, a number where a decimal string belongs or a null where the
// shape has none makes the whole answer "not understood". Nothing is printed
// from it: a missing limit must not read as "not set", a missing amount not as
// zero.
func TestIssueBudgetShowAnswerNotUnderstood(t *testing.T) {
	const byReason = `"by_reason":[{"reason":"lost_contact","count":1},{"reason":"observation_off","count":2}]`
	for _, tc := range []struct {
		name, old, new, field string
	}{
		{"an amount as a JSON number", `"limit_ticks":"250000000000"`, `"limit_ticks":250000000000`, "limit_ticks"},
		{"an amount of the ledger as a JSON number", `"actual_known_ticks":"73500000000"`, `"actual_known_ticks":73500000000`, "amounts.actual_known_ticks"},
		{"an amount with a leading zero", `"limit_ticks":"250000000000"`, `"limit_ticks":"0250000000000"`, "limit_ticks"},
		{"an amount with a sign", `"limit_ticks":"250000000000"`, `"limit_ticks":"-250000000000"`, "limit_ticks"},
		{"an amount in dollars", `"limit_ticks":"250000000000"`, `"limit_ticks":"25.00"`, "limit_ticks"},
		{"an amount that is empty", `"limit_ticks":"250000000000"`, `"limit_ticks":""`, "limit_ticks"},
		{"an amount above the largest", `"limit_ticks":"250000000000"`, `"limit_ticks":"9223372036854775808"`, "limit_ticks"},
		{"a missing remaining_ticks", `"remaining_ticks":null,`, ``, "remaining_ticks"},
		{"a missing limit_ticks", `"limit_ticks":"250000000000",`, ``, "limit_ticks"},
		{"a missing attempt_reserve_ticks", `"attempt_reserve_ticks":"5000000000",`, ``, "attempt_reserve_ticks"},
		{"a total_known of null", `"total_known":false`, `"total_known":null`, "total_known"},
		{"a total_known that is a word", `"total_known":false`, `"total_known":"false"`, "total_known"},
		{"a missing total_known", `"total_known":false,`, ``, "total_known"},
		{"a revision as a JSON number", `"policy_revision":"3"`, `"policy_revision":3`, "policy_revision"},
		{"a ledger revision of null", `"ledger_revision":"42"`, `"ledger_revision":null`, "ledger_revision"},
		{"a count as a string", `"pre_activation":1`, `"pre_activation":"1"`, "unknown.pre_activation"},
		{"a count with a fraction", `"pre_activation":1`, `"pre_activation":1.0`, "unknown.pre_activation"},
		{"a count below zero", `{"reason":"lost_contact","count":1}`, `{"reason":"lost_contact","count":-1}`, "unknown.by_reason[0].count"},
		{"a reason without its count", `{"reason":"observation_off","count":2}`, `{"reason":"observation_off"}`, "unknown.by_reason[1].count"},
		{"reasons of null", byReason, `"by_reason":null`, "unknown.by_reason"},
		{"a mode of null", `"mode":"observe"`, `"mode":null`, "mode"},
		{"a state that is a number", `"admission_state":"open"`, `"admission_state":1`, "admission_state"},
		{"a missing ledger_state", `"ledger_state":"available",`, ``, "ledger_state"},
		{"an activation time of null", `"activated_at":"2026-11-19T01:02:03.456789Z"`, `"activated_at":null`, "activated_at"},
		{"an amount of the ledger that is missing", `,"overrun_ticks":"0"`, ``, "amounts.overrun_ticks"},
		{"an amount of the ledger of null", `"estimated_ticks":"1200000000"`, `"estimated_ticks":null`, "amounts.estimated_ticks"},
		{"a missing included_attempts", `"included_attempts":12,`, ``, "coverage.included_attempts"},
		{"a name that is null", `"collaboration_host_run"`, `null`, "coverage.not_applicable[1]"},
		{"a missing settled_as_of_revision", `"settled_as_of_revision":null,`, ``, "settled_as_of_revision"},
		{"a field that is there twice", `"mode":"observe",`, `"mode":"observe","mode":"off",`, "mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := showBudgetRoot(t, budgetTestRoot, budgetReplace(t, budgetFixture(t, "show-open.json"), tc.old, tc.new))
			wantBudgetNotUnderstood(t, run, tc.field)
		})
	}

	open := budgetFixture(t, "show-open.json")
	for name, body := range map[string][]byte{
		"an answer cut in the middle":       open[:len(open)/2],
		"an empty answer":                   {},
		"an answer of null":                 []byte("null\n"),
		"an answer that is a list":          append(append([]byte("["), bytes.TrimSpace(open)...), "]\n"...),
		"an answer followed by another":     append(append([]byte{}, open...), open...),
		"the answer of another route":       budgetFixture(t, "attempts-page.json"),
		"a page instead of an answer":       []byte("<html><body>welcome</body></html>"),
		"no amounts with a readable ledger": budgetReplace(t, budgetFixture(t, "show-unavailable.json"), `"ledger_state":"unavailable"`, `"ledger_state":"available"`),
		"a missing amounts key":             budgetReplace(t, budgetFixture(t, "show-unavailable.json"), `"amounts":null,`, ``),
	} {
		t.Run(name, func(t *testing.T) {
			wantBudgetNotUnderstood(t, showBudgetRoot(t, budgetTestRoot, body), "")
		})
	}

	t.Run("the answer about another root", func(t *testing.T) {
		run := showBudgetRoot(t, budgetTestRoot, budgetReplace(t, open, `"budget_root_id":"`+budgetTestRoot+`"`, `"budget_root_id":"`+budgetTestRoot2+`"`))
		if run.exit != 1 || run.stdout != "" || !strings.Contains(run.stderr, "another budget root") {
			t.Fatalf("exit %d, stdout %q, stderr %q: want exit 1, nothing printed and the answer reported as that of another budget root", run.exit, run.stdout, run.stderr)
		}
	})

	// The server says that the ledger cannot be read and sends amounts all the
	// same. What it says cannot be read is not shown as a number.
	t.Run("amounts beside a ledger that cannot be read are not shown", func(t *testing.T) {
		run := showBudgetRoot(t, budgetTestRoot, budgetReplace(t, budgetFixture(t, "show-known.json"), `"ledger_state":"available"`, `"ledger_state":"unavailable"`))
		if run.exit != 0 {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		for _, label := range budgetAmountLabels {
			if value := budgetLine(t, run.stdout, label); !strings.HasPrefix(value, "unknown") || strings.ContainsAny(value, "0123456789") {
				t.Errorf("%s = %q, want unknown", label, value)
			}
		}
	})
}

var (
	// budgetEnableResult is the result of the enable of the fixtures as a
	// table.
	budgetEnableResult = []string{
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
	}
	// budgetSetLimitResult is the result of the set-limit of the fixtures.
	budgetSetLimitResult = []string{
		"OPERATION | 1a9d3c5e-7f2b-4086-b4d1-6e8c0a2f4b79",
		"KIND | set_limit",
		"BUDGET ROOT | 7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f",
		"ROOT ISSUE | 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314",
		"STATE | open",
		"MODE | observe",
		"POLICY REVISION | 4",
		"POLICY EPOCH | 1",
		"LIMIT | not set",
		budgetHalfDollarReserve,
		"REASON | reserve half a dollar per run, no cap yet",
	}
)

// `operation` asks what became of a write by the key it was sent with. There
// are two answers: committed, with the result, or unknown.
func TestIssueBudgetOperation(t *testing.T) {
	const (
		committed   = "committed: an operation is committed under this key"
		byIssuePath = budgetTestBindingPath + "/operations/" + budgetTestEnableKey
		byRootPath  = budgetTestOperationsPath + "/" + budgetTestSetLimitKey
	)
	byIssue := []string{"operation", "--issue", budgetTestIssue, "--request-key", budgetTestEnableKey}
	byRoot := []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestSetLimitKey}

	t.Run("committed", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("GET", byIssuePath, 200, "operation-committed.json")
		run := runBudgetCLI(t, byIssue...)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout, append([]string{committed}, budgetEnableResult...)...)
		fake.wantRequests("GET " + byIssuePath)
	})

	t.Run("committed and stale", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("GET", byRootPath, 200, "operation-committed-stale.json")
		run := runBudgetCLI(t, byRoot...)
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout, append([]string{
			committed,
			"stale: the policy of this budget root has changed since this operation; what follows is the result of the operation, not the policy of today",
		}, budgetSetLimitResult...)...)
		fake.wantRequests("GET " + byRootPath)
	})

	// Unknown is an answer, not an error: the command exits 0 and says what the
	// answer does and does not tell.
	for name, tc := range map[string]struct {
		args []string
		path string
	}{
		"unknown, asked by the root":  {byRoot, byRootPath},
		"unknown, asked by the issue": {byIssue, byIssuePath},
	} {
		t.Run(name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.fixture("GET", tc.path, 200, "operation-unknown.json")
			run := runBudgetCLI(t, tc.args...)
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			wantBudgetOutput(t, run.stdout,
				"unknown: no committed operation is visible under this key yet",
				"That is not a refusal and does not say that the write is lost.",
				"To go on, run the original command again exactly as it was: the same --request-key and the same flags.",
			)
			if strings.Contains(strings.ToLower(run.stdout), "fail") || strings.Contains(run.stdout, "/api/") {
				t.Errorf("stdout %q: must not say \"fail\" and must not print the path of the answer", run.stdout)
			}
			run.wantOnlyTypedIDs(t)
			fake.wantRequests("GET " + tc.path)
		})
	}

	// An operation of any kind may be committed under a key, also one this
	// command does not know.
	t.Run("a kind of a later server is printed as it came", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("GET", byRootPath, 200, budgetReplace(t, budgetFixture(t, "operation-committed-stale.json"), `"operation_kind":"set_limit"`, `"operation_kind":"transfer"`))
		run := runBudgetCLI(t, byRoot...)
		if run.exit != 0 {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		if got := budgetLine(t, run.stdout, "KIND"); got != "transfer" {
			t.Errorf("KIND = %q, want transfer", got)
		}
	})

	stale := budgetFixture(t, "operation-committed-stale.json")
	for _, tc := range []struct {
		name  string
		body  []byte
		field string
	}{
		{"a state of a later server", budgetReplace(t, stale, `"state":"committed"`, `"state":"pending"`), "state"},
		{"a state of null", budgetReplace(t, stale, `"state":"committed"`, `"state":null`), "state"},
		{"no state", budgetReplace(t, stale, `"state":"committed",`, ``), "state"},
		{"committed without stale", budgetReplace(t, stale, `"stale":true,`, ``), "stale"},
		{"committed with a stale of null", budgetReplace(t, stale, `"stale":true`, `"stale":null`), "stale"},
		{"committed without a result", budgetReplace(t, budgetFixture(t, "operation-unknown.json"), `"state":"unknown"`, `"state":"committed","stale":false`), "result"},
		{"committed with a result without its mode", budgetReplace(t, stale, `"mode":"observe",`, ``), "result.mode"},
		{"committed with a revision as a number", budgetReplace(t, stale, `"policy_revision":"4"`, `"policy_revision":4`), "result.policy_revision"},
		{"committed with a limit that is missing", budgetReplace(t, stale, `"limit_ticks":null,`, ``), "result.limit_ticks"},
		{"a committed enable without its activation time", budgetReplace(t, budgetFixture(t, "operation-committed.json"), `"activated_at":"2026-11-19T01:02:03.456789Z",`, ``), "result.activated_at"},
		{"unknown without the path to try again", budgetReplace(t, budgetFixture(t, "operation-unknown.json"), `,"retry_path":"`+budgetTestRootPath+`/operations/`+budgetTestEnableKey+`"`, ``), "retry_path"},
		{"an answer cut in the middle", stale[:len(stale)/2], ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("GET", byRootPath, 200, tc.body)
			run := runBudgetCLI(t, byRoot...)
			wantBudgetNotUnderstood(t, run, tc.field)
			fake.wantRequests("GET " + byRootPath)
		})
	}

	t.Run("asked by a root, answered with the operation of another root", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.answer("GET", byRootPath, 200, budgetReplace(t, stale, `"budget_root_id":"`+budgetTestRoot+`"`, `"budget_root_id":"`+budgetTestRoot2+`"`))
		run := runBudgetCLI(t, byRoot...)
		if run.exit != 1 || run.stdout != "" || !strings.Contains(run.stderr, "another budget root") {
			t.Fatalf("exit %d, stdout %q, stderr %q: want exit 1, nothing printed and the answer reported as that of another budget root", run.exit, run.stdout, run.stderr)
		}
	})
}

const (
	budgetObservationsHeader = "ID | CREATED AT | KIND | REASON | COVERED | ISSUE | TASK | CLAIM | ATTEMPT | DETAIL"
	budgetAttemptsHeader     = "ATTEMPT | CREATED AT | STATE | RESERVED | RESERVATION | ACTUAL KNOWN | UNSPENT | OVERRUN | PROTOCOL | CLAIM | TASK | ISSUE | CONTEXT PARENT | AGENT | RUNTIME"

	budgetObservationsCursor = "MjAyNi0xMS0xOVQwMzoyMDo0MC44NzE1MzZaLDRkNWU2ZjcwLTgxOTItNGEzYi1iYzRkLTVlNmY3MDgxOTIwMw"
	budgetAttemptsCursor     = "MjAyNi0xMS0xOVQwMzoyMDo0MS4xMTgyMDRaLGU4ZDdjNmI1LWE0OTMtNDI4MS1iMDZmLTVlNGQzYzJiMWEwOQ"
)

// listBudget runs one of the two list commands against one answer.
func listBudget(t *testing.T, list string, body []byte, flags ...string) budgetRun {
	t.Helper()
	fake := newBudgetFake(t)
	fake.answer("GET", budgetTestRootPath+"/"+list, 200, body)
	run := runBudgetCLI(t, append([]string{list, "--root", budgetTestRoot}, flags...)...)
	if got := fake.targets(); len(got) != 1 || !strings.HasPrefix(got[0], "GET "+budgetTestRootPath+"/"+list) {
		t.Fatalf("the server saw %q, want one request for the %s", got, list)
	}
	return run
}

// `observations` prints one row for each observation and the command that
// fetches the next page.
func TestIssueBudgetObservations(t *testing.T) {
	t.Run("a page with a next page", func(t *testing.T) {
		run := listBudget(t, "observations", budgetFixture(t, "observations-page.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			budgetObservationsHeader,
			`ab12cd34-ef56-4a78-9b90-c1d2e3f4a5b6 | 2026-11-19T05:41:09.250113Z | would_refuse | unbounded_unknown | no | 5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25 | 1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b | 1 | 6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b | {"by_reason":{"lost_contact":1,"observation_off":2}}`,
			`0f9e8d7c-6b5a-4493-8271-605f4e3d2c1b | 2026-11-19T04:13:57.004321Z | attribution_difference | created_outside_context_parent | no | f2a4c6e8-1b3d-4f5a-97b9-d1e3f5a7c9b0 | c2d3e4f5-a6b7-48c9-90d1-e2f3a4b5c6d7 | 1 | b0c1d2e3-f4a5-46b7-88c9-d0e1f2a3b4c5 | {}`,
			`4d5e6f70-8192-4a3b-bc4d-5e6f70819203 | 2026-11-19T03:20:40.871536Z | coverage_gap | in_flight_at_bind | yes | d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19 | 71829304-a5b6-4c7d-8e9f-0a1b2c3d4e5f | 2 | - | {"task_status":"running"}`,
			"next page: multica issue budget observations --root "+budgetTestRoot+" --cursor "+budgetObservationsCursor,
		)
	})

	t.Run("the last page", func(t *testing.T) {
		run := listBudget(t, "observations", budgetFixture(t, "observations-last-page.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			budgetObservationsHeader,
			`c9b8a7f6-e5d4-43c2-a1b0-9f8e7d6c5b4a | 2026-11-19T02:40:18.660902Z | coverage_gap | observation_off | no | - | - | - | - | {"policy_epoch":"1"}`,
			`58697a8b-9cad-4bce-8fd0-e1f203142536 | 2026-11-19T01:02:03.456789Z | coverage_gap | in_flight_legacy | no | 5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25 | d0e1f2a3-b4c5-46d7-a8e9-f0a1b2c3d4e5 | 2 | - | {"task_status":"running"}`,
			"no further page",
		)
	})

	t.Run("a page without a row", func(t *testing.T) {
		run := listBudget(t, "observations", budgetFixture(t, "observations-empty.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout, "no observations on this page", "no further page")
	})

	// The command of the next page keeps the size of the page that was typed
	// and takes the cursor of the answer, quoted for the shell where it needs
	// to be; the cursor that was typed is the one of this page and is gone.
	t.Run("the next page keeps --limit and takes the new cursor", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "observations-page.json"), budgetObservationsCursor, "a+b/c=d&e f")
		run := listBudget(t, "observations", body, "--limit", "3", "--cursor", "typed-cursor")
		lines := budgetCells(run.stdout)
		if got, want := lines[len(lines)-1], "next page: multica issue budget observations --root "+budgetTestRoot+" --limit 3 --cursor 'a+b/c=d&e f'"; got != want {
			t.Errorf("last line\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("a cursor that cannot be shown is not put into a command", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "observations-page.json"), budgetObservationsCursor, `abc\u001b[2Jdef`)
		run := listBudget(t, "observations", body)
		if run.exit != 0 || strings.ContainsAny(run.stdout, "\x1b") {
			t.Fatalf("exit %d; the cursor reaches the terminal as it came: %q", run.exit, run.stdout)
		}
		lines := budgetCells(run.stdout)
		if got := lines[len(lines)-1]; !strings.HasPrefix(got, "next page: ") || !strings.Contains(got, "--output json") || strings.Contains(got, "--cursor") {
			t.Errorf("last line %q: want the operator sent to --output json for the cursor", got)
		}
	})

	// --cursor refuses an empty value, so a command with one would not run. The
	// page is not taken for the last one either: null says that, and "" is not null.
	t.Run("an empty cursor is not put into a command", func(t *testing.T) {
		for _, list := range []struct{ name, fixture, cursor string }{
			{"observations", "observations-page.json", budgetObservationsCursor},
			{"attempts", "attempts-page.json", budgetAttemptsCursor},
		} {
			run := listBudget(t, list.name, budgetReplace(t, budgetFixture(t, list.fixture), `"next_cursor":"`+list.cursor+`"`, `"next_cursor":""`))
			lines := budgetCells(run.stdout)
			if got, want := lines[len(lines)-1], "next page: there is one, but its position cannot be put into a command here; read it with --output json"; run.exit != 0 || got != want {
				t.Errorf("%s: exit %d, last line\n got: %s\nwant: %s", list.name, run.exit, got, want)
			}
		}
	})

	t.Run("a detail spread over lines is one cell", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "observations-page.json"), `"detail":{"task_status":"running"}`, "\"detail\":{\n  \"task_status\": \"running\",\n  \"note\": \"a  b\"\n}")
		run := listBudget(t, "observations", body)
		if got := strings.Count(run.stdout, "\n"); got != 5 {
			t.Fatalf("%d lines, want 5 (the header, three rows, the next page):\n%s", got, run.stdout)
		}
		if !strings.Contains(run.stdout, `{"task_status":"running","note":"a  b"}`) {
			t.Errorf("the detail is not printed as it came, without the space between its parts:\n%s", run.stdout)
		}
	})

	t.Run("a kind and a reason of a later server are printed as they came", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "observations-page.json"), `"kind":"would_refuse","reason":"unbounded_unknown"`, `"kind":"would_defer","reason":"queue_full"`)
		run := listBudget(t, "observations", body)
		if row := budgetCells(run.stdout)[1]; !strings.Contains(row, " | would_defer | queue_full | ") {
			t.Errorf("row %q: want the kind and the reason as they came", row)
		}
	})

	page := budgetFixture(t, "observations-page.json")
	for _, tc := range []struct {
		name  string
		body  []byte
		field string
	}{
		{"no next_cursor", budgetReplace(t, page, `,"next_cursor":"`+budgetObservationsCursor+`"`, ``), "next_cursor"},
		{"a next_cursor that is a number", budgetReplace(t, page, `"next_cursor":"`+budgetObservationsCursor+`"`, `"next_cursor":7`), "next_cursor"},
		{"observations of null", budgetReplace(t, budgetFixture(t, "observations-empty.json"), `"observations":[]`, `"observations":null`), "observations"},
		{"a covered of null", budgetReplace(t, page, `"covered":true`, `"covered":null`), "observations[2].covered"},
		{"a detail that is a list", budgetReplace(t, page, `"detail":{}`, `"detail":[]`), "observations[1].detail"},
		{"a detail of null", budgetReplace(t, page, `"detail":{}`, `"detail":null`), "observations[1].detail"},
		{"a claim as a number", budgetReplace(t, page, `"claim_generation":"2"`, `"claim_generation":2`), "observations[2].claim_generation"},
		{"a row without its attempt", budgetReplace(t, page, `"attempt_id":null,`, ``), "observations[2].attempt_id"},
		{"a row without its id", budgetReplace(t, page, `"id":"0f9e8d7c-6b5a-4493-8271-605f4e3d2c1b",`, ``), "observations[1].id"},
		{"a row that is null", budgetReplace(t, budgetFixture(t, "observations-empty.json"), `"observations":[]`, `"observations":[null]`), "observations[0].id"},
		{"the answer of the other list", budgetFixture(t, "attempts-page.json"), "observations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantBudgetNotUnderstood(t, listBudget(t, "observations", tc.body), tc.field)
		})
	}
}

// `attempts` prints one row for each execution. A reservation without a size
// prints as "unsized" and never as 0, and an execution claimed without a
// protocol version prints as an old daemon.
func TestIssueBudgetAttempts(t *testing.T) {
	const (
		half      = "0.50 USD (5000000000 ticks)"
		zero      = "0.00 USD (0 ticks)"
		member    = "5e2d9a17-4c6b-4f03-9b8e-7a1c0d3f6e25"
		other     = "d91c4a6e-2f3b-4857-a0c9-6e5d4c3b2a19"
		agentA    = "4b8d2f6a-0c1e-4a3b-9d5f-7e9a1c3b5d7f"
		agentB    = "a7c9e1b3-d5f7-4092-b4d6-f8a0c2e4b6d8"
		runtime   = "3e5a7c9b-1d2f-4b4d-8f6a-0c2e4a6b8d0f"
		runtimeB  = "9f1b3d5e-7a9c-4e0b-a2d4-6f8b0d2c4e6a"
		rootIssue = "0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314"
	)
	row := func(cells ...string) string { return strings.Join(cells, " | ") }

	t.Run("a page with a next page", func(t *testing.T) {
		run := listBudget(t, "attempts", budgetFixture(t, "attempts-page.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			"ledger: available",
			budgetAttemptsHeader,
			row("6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b", "2026-11-19T05:41:09.118204Z", "open", half, "open", "0.12 USD (1200000000 ticks)", "0.38 USD (3800000000 ticks)", zero, "1", "1", "1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b", member, "-", agentA, runtime),
			row("0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "2026-11-19T05:12:44.730015Z", "open", "unsized", "open", "0.073 USD (730000000 ticks)", "unsized", "unsized", "1", "3", "8b7a6c5d-4e3f-4201-b9a8-c7d6e5f4a3b2", other, "-", agentB, runtime),
			row("7e6d5c4b-3a29-4180-9f7e-6d5c4b3a2918", "2026-11-19T04:58:21.904417Z", "lost_contact", half, "held", "0.21 USD (2100000000 ticks)", "0.29 USD (2900000000 ticks)", zero, "1", "1", "5d4c3b2a-1908-4f7e-ad6c-5b4a39281706", member, "-", agentB, runtime),
			row("b0c1d2e3-f4a5-46b7-88c9-d0e1f2a3b4c5", "2026-11-19T04:12:55.310628Z", "settled", half, "settled", "0.62 USD (6200000000 ticks)", zero, "0.12 USD (1200000000 ticks)", "1", "1", "c2d3e4f5-a6b7-48c9-90d1-e2f3a4b5c6d7", "-", rootIssue, agentA, runtime),
			row("2c4e6a8b-0d1f-4357-9b9d-bf1d3f5b7d9f", "2026-11-19T03:47:02Z", "process_exit", half, "held", zero, half, zero, "old daemon", "2", "94a5b6c7-d8e9-4f0a-b1c2-d3e4f5a6b7c8", other, "-", agentA, runtimeB),
			row("e8d7c6b5-a493-4281-b06f-5e4d3c2b1a09", "2026-11-19T03:20:41.118204Z", "superseded_unproven", half, "no reservation", "-", "-", "-", "1", "1", "3a4b5c6d-7e8f-4901-a2b3-c4d5e6f7a8b9", member, "-", agentB, runtime),
			"next page: multica issue budget attempts --root "+budgetTestRoot+" --cursor "+budgetAttemptsCursor,
		)
	})

	// Without a ledger nothing is known of a reservation, which is not the
	// same as an execution that has none.
	t.Run("a page without a ledger", func(t *testing.T) {
		run := listBudget(t, "attempts", budgetFixture(t, "attempts-unavailable.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout,
			"ledger: unavailable; the ledger cannot be read, so what the reservations hold is unknown",
			budgetAttemptsHeader,
			row("6f1e2d3c-4b5a-4978-8a6b-5c4d3e2f1a0b", "2026-11-19T05:41:09.118204Z", "open", half, "unknown", "unknown", "unknown", "unknown", "1", "1", "1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b", member, "-", agentA, runtime),
			row("0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", "2026-11-19T05:12:44.730015Z", "open", "unsized", "unknown", "unknown", "unknown", "unknown", "1", "3", "8b7a6c5d-4e3f-4201-b9a8-c7d6e5f4a3b2", other, "-", agentB, runtime),
			row("2c4e6a8b-0d1f-4357-9b9d-bf1d3f5b7d9f", "2026-11-19T03:47:02Z", "process_exit", half, "unknown", "unknown", "unknown", "unknown", "old daemon", "2", "94a5b6c7-d8e9-4f0a-b1c2-d3e4f5a6b7c8", other, "-", agentA, runtimeB),
			"no further page",
		)
	})

	// A ledger that cannot be read gives nothing to go by, also when the answer
	// carries reservations all the same.
	t.Run("a page without a ledger that carries reservations", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "attempts-page.json"), `"ledger_state":"available"`, `"ledger_state":"unavailable"`)
		run := listBudget(t, "attempts", body)
		lines := budgetCells(run.stdout)
		if run.exit != 0 || len(lines) != 9 {
			t.Fatalf("exit %d, %d lines, want the ledger, the header, six rows and the last line:\n%s", run.exit, len(lines), run.stdout)
		}
		for _, line := range lines[2:8] {
			cells := strings.Split(line, " | ")
			for _, column := range []int{4, 5, 6, 7} {
				if cells[column] != "unknown" {
					t.Errorf("row %s: column %d = %q, want unknown", cells[0], column+1, cells[column])
				}
			}
		}
	})

	t.Run("a page without a row", func(t *testing.T) {
		run := listBudget(t, "attempts", budgetFixture(t, "attempts-empty.json"))
		if run.exit != 0 || run.stderr != "" {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		wantBudgetOutput(t, run.stdout, "ledger: available", "no attempts on this page", "no further page")
	})

	// The row of the fixture whose reservation has no size: the ledger holds 0
	// for it in three places, and none of them is printed as an amount.
	t.Run("an unsized reservation never prints as 0", func(t *testing.T) {
		run := listBudget(t, "attempts", budgetFixture(t, "attempts-page.json"))
		cells := strings.Split(budgetCells(run.stdout)[3], " | ")
		if cells[0] != "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d" {
			t.Fatalf("row %q is not the row of the unsized reservation", cells)
		}
		for _, column := range []int{3, 6, 7} {
			if cells[column] != "unsized" {
				t.Errorf("column %d = %q, want unsized", column+1, cells[column])
			}
		}
		if strings.Contains(strings.Join(cells, " | "), zero) {
			t.Errorf("row %q: an unsized reservation is printed as 0", cells)
		}
	})

	// A reservation of zero that has a size is another fact, and is printed as
	// the amount it is.
	t.Run("a sized reservation of zero prints as an amount", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "attempts-page.json"), `"reserved_ticks":"5000000000","protocol_version":1,"created_at":"2026-11-19T05:41:09.118204Z"`, `"reserved_ticks":"0","protocol_version":1,"created_at":"2026-11-19T05:41:09.118204Z"`)
		run := listBudget(t, "attempts", body)
		if cells := strings.Split(budgetCells(run.stdout)[2], " | "); cells[3] != zero {
			t.Errorf("RESERVED = %q, want %q", cells[3], zero)
		}
	})

	t.Run("the next page keeps --limit and takes the new cursor", func(t *testing.T) {
		run := listBudget(t, "attempts", budgetFixture(t, "attempts-page.json"), "--limit", "6", "--cursor", "typed-cursor")
		lines := budgetCells(run.stdout)
		if got, want := lines[len(lines)-1], "next page: multica issue budget attempts --root "+budgetTestRoot+" --limit 6 --cursor "+budgetAttemptsCursor; got != want {
			t.Errorf("last line\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("a state of a later server is printed as it came", func(t *testing.T) {
		body := budgetReplace(t, budgetFixture(t, "attempts-page.json"), `"state":"lost_contact"`, `"state":"paused"`)
		body = budgetReplace(t, body, `"status":"settled"`, `"status":"refunded"`)
		body = budgetReplace(t, body, `"ledger_state":"available"`, `"ledger_state":"rebuilding"`)
		run := listBudget(t, "attempts", body)
		lines := budgetCells(run.stdout)
		if lines[0] != "ledger: rebuilding" {
			t.Errorf("first line %q, want the ledger state as it came", lines[0])
		}
		if cells := strings.Split(lines[4], " | "); cells[2] != "paused" {
			t.Errorf("STATE = %q, want paused", cells[2])
		}
		if cells := strings.Split(lines[5], " | "); cells[4] != "refunded" {
			t.Errorf("RESERVATION = %q, want refunded", cells[4])
		}
		// A ledger state this command does not know is not taken for
		// "available": the execution without a reservation is not said to
		// have none.
		if cells := strings.Split(lines[7], " | "); cells[4] != "unknown" || cells[5] != "unknown" {
			t.Errorf("row %q: want the reservation and its amounts unknown", cells)
		}
	})

	page := budgetFixture(t, "attempts-page.json")
	for _, tc := range []struct {
		name  string
		body  []byte
		field string
	}{
		{"no ledger_state", budgetReplace(t, page, `"ledger_state":"available",`, ``), "ledger_state"},
		{"no next_cursor", budgetReplace(t, page, `,"next_cursor":"`+budgetAttemptsCursor+`"`, ``), "next_cursor"},
		{"a protocol version as a string", budgetReplace(t, page, `"protocol_version":null`, `"protocol_version":"1"`), "attempts[4].protocol_version"},
		{"a row without its protocol version", budgetReplace(t, page, `"protocol_version":null,`, ``), "attempts[4].protocol_version"},
		{"a reserve as a number", budgetReplace(t, page, `"reserved_ticks":null`, `"reserved_ticks":0`), "attempts[1].reserved_ticks"},
		{"a reservation without unsized", budgetReplace(t, page, `"unsized":true,`, ``), "attempts[1].reservation.unsized"},
		{"a reservation with an unsized of null", budgetReplace(t, page, `"unsized":true`, `"unsized":null`), "attempts[1].reservation.unsized"},
		{"a reservation with an amount of null", budgetReplace(t, page, `"actual_known_ticks":"730000000"`, `"actual_known_ticks":null`), "attempts[1].reservation.actual_known_ticks"},
		{"a row without its reservation", budgetReplace(t, page, `,"reservation":null`, ``), "attempts[5].reservation"},
		{"a claim as a number", budgetReplace(t, page, `"claim_generation":"3"`, `"claim_generation":3`), "attempts[1].claim_generation"},
		{"a row without its issue", budgetReplace(t, page, `"issue_id":null,`, ``), "attempts[3].issue_id"},
		{"a state of null", budgetReplace(t, page, `"state":"settled"`, `"state":null`), "attempts[3].state"},
		{"the answer of the other list", budgetFixture(t, "observations-page.json"), "ledger_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantBudgetNotUnderstood(t, listBudget(t, "attempts", tc.body), tc.field)
		})
	}
}

// A read that is refused, or that gets no answer of the budget API, is an
// error that says which of the two it is; nothing is printed as if it were an
// answer.
func TestIssueBudgetReadRefusedOrUnanswered(t *testing.T) {
	reads := []struct {
		name string
		args []string
		path string
	}{
		{"show", []string{"show", "--root", budgetTestRoot}, budgetTestRootPath},
		{"operation", []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey}, budgetTestOperationsPath + "/" + budgetTestCloseKey},
		{"observations", []string{"observations", "--root", budgetTestRoot}, budgetTestObservationsPath},
		{"attempts", []string{"attempts", "--root", budgetTestRoot}, budgetTestAttemptsPath},
	}
	// An answer of each read that the command would understand and print.
	understood := map[string]string{
		budgetTestRootPath:         "show-open.json",
		reads[1].path:              "operation-unknown.json",
		budgetTestObservationsPath: "observations-page.json",
		budgetTestAttemptsPath:     "attempts-page.json",
	}
	answered := func(status int, body string) func(*testing.T, *budgetFake, string) {
		return func(_ *testing.T, fake *budgetFake, path string) { fake.answer("GET", path, status, []byte(body)) }
	}
	for _, tc := range []struct {
		name    string
		plan    func(t *testing.T, fake *budgetFake, path string)
		exit    int
		starts  string
		holds   string
		without string
	}{
		{name: "a 401", plan: answered(401, `{"error":"invalid or expired token"}`), exit: 3, starts: "refused (401): "},
		{name: "budget_forbidden", plan: answered(403, string(budgetErrorJSON("budget_forbidden"))), exit: 3, starts: "refused (budget_forbidden): "},
		{name: "not_found", plan: answered(404, string(budgetErrorJSON("not_found"))), exit: 4, starts: "refused (not_found): "},
		{name: "invalid_request", plan: answered(400, string(budgetErrorJSON("invalid_request"))), exit: 5, starts: "refused (invalid_request): "},
		{name: "a 404 of another handler", plan: answered(404, "404 page not found\n"), exit: 4, holds: "HTTP 404", without: "refused"},
		{name: "a 403 without a code", plan: answered(403, `{"error":"not a member of this workspace"}`), exit: 3, holds: "HTTP 403", without: "refused"},
		// The two answers of the workspace membership check, byte for byte
		// (internal/middleware/workspace.go): neither is an answer of the budget
		// handlers, so neither is reported as a refusal by them.
		{name: "the 404 of the workspace membership check", plan: answered(404, `{"error":"workspace not found","code":"workspace_access_denied"}`), exit: 4, holds: "HTTP 404", without: "refused"},
		{name: "the 403 of the workspace role check", plan: answered(403, `{"error":"insufficient permissions"}`), exit: 3, holds: "HTTP 403", without: "refused"},
		{name: "a 404 with a code this command does not know", plan: answered(404, string(budgetErrorJSON("brand_new_code"))), exit: 4, holds: "HTTP 404", without: "refused"},
		{name: "a 500", plan: answered(500, string(budgetErrorJSON("internal_error"))), exit: 1, holds: "HTTP 500", without: "refused"},
		// A refusal is a 4xx. The same body under a 5xx says nothing about
		// what the server decided.
		{name: "a 500 with the body of a refusal", plan: answered(500, string(budgetErrorJSON("not_found"))), exit: 1, holds: "HTTP 500", without: "refused"},
		{name: "a 503 with the body of a refusal", plan: answered(503, string(budgetErrorJSON("budget_forbidden"))), exit: 1, holds: "HTTP 503", without: "refused"},
		{name: "a 502 with a page", plan: answered(502, "<html><body>502 Bad Gateway</body></html>"), exit: 1, holds: "HTTP 502", without: "<html"},
		{name: "a 204", plan: answered(204, ""), exit: 1, holds: "HTTP 204"},
		// The answer is one the command would print, followed by space up to
		// more than it reads: it is not cut to its beginning and shown, and it
		// is not said that no answer came.
		{name: "a 200 longer than the command reads", plan: func(t *testing.T, fake *budgetFake, path string) {
			body := append(bytes.TrimSpace(budgetFixture(t, understood[path])), bytes.Repeat([]byte(" "), budgetMaxAnswerBytes)...)
			fake.answer("GET", path, 200, body)
		}, exit: 1, holds: "HTTP 200 with more than this command reads", without: "no answer"},
		{name: "a 302 to a route that would answer", plan: func(t *testing.T, fake *budgetFake, path string) {
			fake.fixture("GET", "/elsewhere", 200, "show-open.json")
			fake.handle("GET", path, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "/elsewhere")
				w.WriteHeader(302)
			})
		}, exit: 1, holds: "HTTP 302"},
		{name: "a connection closed without an answer", plan: func(t *testing.T, fake *budgetFake, path string) {
			fake.handle("GET", path, func(w http.ResponseWriter, _ *http.Request) {
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Errorf("hijack: %v", err)
					return
				}
				_ = connection.Close()
			})
		}, exit: 2, starts: "no answer came back from the server"},
	} {
		for _, read := range reads {
			t.Run(read.name+": "+tc.name, func(t *testing.T) {
				fake := newBudgetFake(t)
				tc.plan(t, fake, read.path)
				run := runBudgetCLI(t, read.args...)
				if run.exit != tc.exit {
					t.Errorf("exit %d, want %d", run.exit, tc.exit)
				}
				if run.stdout != "" {
					t.Errorf("stdout = %q, want nothing", run.stdout)
				}
				if !strings.HasPrefix(run.stderr, tc.starts) || !strings.Contains(run.stderr, tc.holds) {
					t.Errorf("stderr %q: want it to start with %q and to hold %q", run.stderr, tc.starts, tc.holds)
				}
				if tc.without != "" && strings.Contains(run.stderr, tc.without) {
					t.Errorf("stderr %q: must not hold %q", run.stderr, tc.without)
				}
				if strings.Contains(run.stderr, "Nothing was written") || strings.Contains(run.stderr, "outcome unknown") {
					t.Errorf("stderr %q: a read is reported with the words of a write", run.stderr)
				}
				fake.wantRequests("GET " + read.path)
			})
		}
	}
}

// A text of the server is printed as it came only when every character of it
// can be seen. Any other text comes out as one quoted Go string in the place
// where the text belongs, so an answer cannot move the cursor, cannot clear
// the screen and cannot add a line that looks like one of the table's.
func TestIssueBudgetServerTextCannotDrawOnTheTerminal(t *testing.T) {
	texts := []struct {
		name    string
		text    string
		printed string
	}{
		{"an escape sequence", "ok\x1b[2J\x1b[Hgone", `"ok\x1b[2J\x1b[Hgone"`},
		{"a line feed", "ok\nSTATE closed", `"ok\nSTATE closed"`},
		{"a carriage return", "ok\rMODE off", `"ok\rMODE off"`},
		{"an empty text", "", `""`},
	}
	show := func(t *testing.T, body []byte) budgetRun { return showBudgetRoot(t, budgetTestRoot, body) }
	observations := func(t *testing.T, body []byte) budgetRun { return listBudget(t, "observations", body) }
	enable := func(t *testing.T, body []byte) budgetRun {
		fake := newBudgetFake(t)
		fake.answer("POST", budgetTestEnablePath, 201, body)
		return runBudgetCLI(t, budgetEnableArgs...)
	}
	const firstObservation = "ab12cd34-ef56-4a78-9b90-c1d2e3f4a5b6 | "
	for _, place := range []struct {
		name    string
		fixture string
		member  string // the member of the fixture that holds the text, up to its value
		was     string // the text the fixture has there
		line    string // how the one line that shows the text begins
		run     func(t *testing.T, body []byte) budgetRun
	}{
		{"the reason of a result", "write-201-enable.json", `"reason":`, "Q4 launch", "REASON | ", enable},
		{"the kind of an observation", "observations-page.json", `"kind":`, "would_refuse", firstObservation, observations},
		{"the reason of an observation", "observations-page.json", `"reason":`, "unbounded_unknown", firstObservation, observations},
		{"a state this command does not know", "show-open.json", `"admission_state":`, "open", "STATE | ", show},
		{"a mode this command does not know", "show-open.json", `"mode":`, "observe", "MODE | ", show},
	} {
		for _, tc := range texts {
			t.Run(place.name+": "+tc.name, func(t *testing.T) {
				fixture := budgetFixture(t, place.fixture)
				plain := place.run(t, fixture)
				if plain.exit != 0 || plain.stderr != "" {
					t.Fatalf("the fixture as it is: exit %d, stderr %q", plain.exit, plain.stderr)
				}
				encoded, err := json.Marshal(tc.text)
				if err != nil {
					t.Fatal(err)
				}
				run := place.run(t, budgetReplace(t, fixture, place.member+`"`+place.was+`"`, place.member+string(encoded)))
				if run.exit != 0 || run.stderr != "" {
					t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
				}
				if strings.ContainsAny(run.stdout, "\x1b\r") {
					t.Errorf("the text reaches the terminal as it came:\n%q", run.stdout)
				}
				// The lines are those of the fixture as it is, but for the one
				// that shows the text: there the quoted text stands where the
				// text of the fixture stood. No line is added and none is lost.
				want := budgetCells(plain.stdout)
				shown := 0
				for i, line := range want {
					if !strings.HasPrefix(line, place.line) {
						continue
					}
					shown++
					want[i] = strings.Replace(line, " | "+place.was, " | "+tc.printed, 1)
					if want[i] == line {
						t.Fatalf("the line %q does not hold %q", line, place.was)
					}
				}
				if shown != 1 {
					t.Fatalf("%d lines begin with %q, want one:\n%s", shown, place.line, plain.stdout)
				}
				wantBudgetOutput(t, run.stdout, want...)
			})
		}
	}
}
