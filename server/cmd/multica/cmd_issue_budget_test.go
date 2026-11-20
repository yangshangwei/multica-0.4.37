package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/cli"
)

// Tests of `multica issue budget` (design 12.6). The commands run against a
// fake server that answers with the fixture files of design 12.9 and records
// every request. Nothing here reads a clock, a network, a database, the
// operator's profile or an installed agent CLI.
//
// The conversion of dollars has its table in cmd_issue_budget_money_test.go,
// and the check of the fixture files is in cmd_issue_budget_fixtures_test.go.

// The ids of the fixture files, and the keys and flags that give the request
// fixtures byte for byte.
const (
	budgetTestWorkspace = "6b1f0c2a-9d3e-4f5a-8b7c-1d2e3f4a5b6c"
	budgetTestToken     = "mul_fake_human"
	budgetTestIssue     = "MUL-12"
	budgetTestRoot      = "7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f"
	budgetTestRoot2     = "b3a81f6d-2c47-4e95-8d0a-9f1e7c5b3a26"

	budgetTestEnableKey   = "3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10"
	budgetTestSetLimitKey = "9b2f7c4e-1d3a-4e6b-8f50-2c7d9e1a4b63"
	budgetTestSetModeKey  = "5c8e1f2a-7b4d-4a90-b3e6-0d1f2a3b4c5d"
	budgetTestCloseKey    = "e4a7d2c9-3b5f-4c18-9a6e-7f0b1c2d3e4f"
	budgetTestReopenKey   = "2d6b8f0c-4e1a-4d37-a59c-8b7e6f5a4d3c"

	budgetTestEnablePath     = "/api/issues/" + budgetTestIssue + "/budget"
	budgetTestRootPath       = "/api/issue-budgets/" + budgetTestRoot
	budgetTestOperationsPath = budgetTestRootPath + "/operations"
)

// The five writes with every flag given. Each sends the bytes of its request
// fixture.
var (
	budgetEnableArgs = []string{"enable", budgetTestIssue, "--request-key", budgetTestEnableKey, "--reason", "Q4 launch",
		"--limit", "25", "--attempt-reserve", "none", "--yes"}
	budgetSetLimitArgs = []string{"set-limit", "--root", budgetTestRoot, "--request-key", budgetTestSetLimitKey,
		"--reason", "reserve half a dollar per run, no cap yet", "--expect-revision", "3", "--limit", "none", "--attempt-reserve", "0.5", "--yes"}
	budgetSetModeArgs = []string{"set-mode", "--root", budgetTestRoot, "--request-key", budgetTestSetModeKey,
		"--reason", "迁移期间暂停观察", "--expect-revision", "3", "--mode", "off", "--yes"}
	budgetCloseArgs = []string{"close", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey,
		"--reason", "work is done", "--expect-revision", "3", "--yes"}
	budgetReopenArgs = []string{"reopen", "--root", budgetTestRoot, "--request-key", budgetTestReopenKey,
		"--reason", "one more round of fixes", "--expect-revision", "4", "--yes"}
)

// budgetFakeRequest is one request as the fake server received it.
type budgetFakeRequest struct {
	Method string
	Target string // the request target as it was sent: escaped path and query
	Header http.Header
	Body   []byte
}

// budgetFake is the fake server. Each test plans the answer of the routes it
// expects; a request to any other route is answered with a 599, and
// wantRequests names the requests a test allows.
type budgetFake struct {
	t        *testing.T
	mu       sync.Mutex
	requests []budgetFakeRequest
	answers  map[string]func(http.ResponseWriter, *http.Request)
}

func newBudgetFake(t *testing.T) *budgetFake {
	t.Helper()
	fake := &budgetFake{t: t, answers: map[string]func(http.ResponseWriter, *http.Request){}}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)

	// The command gets its server, workspace and credential the way the
	// snapshot tests give them, and nothing of the machine it runs on.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_SERVER_URL", server.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", budgetTestWorkspace)
	t.Setenv("MULTICA_TOKEN", budgetTestToken)
	for _, name := range []string{
		"MULTICA_AGENT_ID", "MULTICA_TASK_ID", "MULTICA_DAEMON_PORT", cli.TaskConfigRootEnv,
		cli.CollaborationTokenEnv, cli.CollaborationServerEnv, cli.CollaborationContextEnv, cli.CollaborationIssueEnv, cli.CollaborationRunEnv,
		"MULTICA_DEBUG",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("LC_ALL", "C")
	return fake
}

func (f *budgetFake) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, budgetFakeRequest{Method: r.Method, Target: r.RequestURI, Header: r.Header.Clone(), Body: body})
	answer := f.answers[r.Method+" "+r.URL.EscapedPath()]
	f.mu.Unlock()
	if answer == nil {
		w.WriteHeader(599)
		return
	}
	answer(w, r)
}

// answer plans a status and a body for one route.
func (f *budgetFake) answer(method, path string, status int, body []byte) {
	f.handle(method, path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	})
}

// fixture plans a fixture file as the answer of one route.
func (f *budgetFake) fixture(method, path string, status int, name string) {
	f.t.Helper()
	f.answer(method, path, status, budgetFixture(f.t, name))
}

func (f *budgetFake) handle(method, path string, handler func(http.ResponseWriter, *http.Request)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers[method+" "+path] = handler
}

// seen returns the requests received so far.
func (f *budgetFake) seen() []budgetFakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]budgetFakeRequest(nil), f.requests...)
}

// targets returns "METHOD target" of the requests received so far, in order.
func (f *budgetFake) targets() []string {
	var out []string
	for _, request := range f.seen() {
		out = append(out, request.Method+" "+request.Target)
	}
	return out
}

// wantRequests fails the test unless the server saw exactly these requests, in
// this order.
func (f *budgetFake) wantRequests(want ...string) {
	f.t.Helper()
	got := f.targets()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		f.t.Fatalf("the server saw %d request(s) %q, want %d: %q", len(got), got, len(want), want)
	}
}

// budgetRun is what one run of a command gave the operator.
type budgetRun struct {
	stdout string
	stderr string // what main prints for the error; empty on success
	exit   int    // the exit code main would use
}

// runBudgetCLI runs `multica issue budget <args>` the way main does.
func runBudgetCLI(t *testing.T, args ...string) budgetRun {
	t.Helper()
	return runBudgetCLIContext(t, context.Background(), args...)
}

func runBudgetCLIContext(t *testing.T, ctx context.Context, args ...string) budgetRun {
	t.Helper()
	root := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("profile", "", "")
	root.PersistentFlags().String("server-url", "", "")
	root.PersistentFlags().String("workspace-id", "", "")
	issue := &cobra.Command{Use: "issue"}
	issue.AddCommand(newIssueBudgetCommand())
	root.AddCommand(issue)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"issue", "budget"}, args...))
	err := root.ExecuteContext(ctx)
	return budgetRun{stdout: stdout.String(), stderr: cli.FormatError(err, false), exit: cli.ExitCodeFor(err)}
}

// budgetUUIDShaped finds everything in a text that looks like a UUID.
var budgetUUIDShaped = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// wantOnlyTypedIDs fails the test when the run printed something shaped like a
// UUID that the operator did not type: a command must never put a key in front
// of the operator that is not theirs.
func (r budgetRun) wantOnlyTypedIDs(t *testing.T, typed ...string) {
	t.Helper()
	allowed := map[string]bool{}
	for _, id := range typed {
		allowed[id] = true
	}
	for _, found := range budgetUUIDShaped.FindAllString(r.stdout+"\n"+r.stderr, -1) {
		if !allowed[found] {
			t.Errorf("the output holds %s, which the operator did not type:\nstdout: %s\nstderr: %s", found, r.stdout, r.stderr)
		}
	}
}

// budgetCells gives what a command printed line by line, with every run of two
// or more spaces (the padding of the table writer) turned into " | ", so that a
// test can name a whole line without counting spaces.
func budgetCells(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		lines = append(lines, budgetPadding.ReplaceAllString(strings.TrimRight(line, " "), " | "))
	}
	return lines
}

var budgetPadding = regexp.MustCompile(` {2,}`)

// wantBudgetOutput fails the test unless the command printed exactly these
// lines, in the form of budgetCells.
func wantBudgetOutput(t *testing.T, got string, want ...string) {
	t.Helper()
	cells := budgetCells(got)
	if strings.Join(cells, "\n") == strings.Join(want, "\n") {
		return
	}
	for i := 0; i < len(cells) || i < len(want); i++ {
		var gotLine, wantLine string
		if i < len(cells) {
			gotLine = cells[i]
		}
		if i < len(want) {
			wantLine = want[i]
		}
		if gotLine != wantLine {
			t.Errorf("line %d\n got: %s\nwant: %s", i+1, gotLine, wantLine)
		}
	}
	t.Fatalf("the command printed %d line(s), want %d:\n%s", len(cells), len(want), got)
}

// budgetLine returns the value of the one line of a table that has this label.
func budgetLine(t *testing.T, text, label string) string {
	t.Helper()
	var found []string
	for _, line := range budgetCells(text) {
		if value, ok := strings.CutPrefix(line, label+" | "); ok {
			found = append(found, value)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one line with the label %q, found %d in:\n%s", label, len(found), text)
	}
	return found[0]
}

// budgetReplace changes the one place of an answer that holds old.
func budgetReplace(t *testing.T, data []byte, old, new string) []byte {
	t.Helper()
	if count := bytes.Count(data, []byte(old)); count != 1 {
		t.Fatalf("%s is in the answer %d times, want once", old, count)
	}
	return bytes.Replace(data, []byte(old), []byte(new), 1)
}

// budgetRootWriteAnswer is the answer to a committed write of one kind on the
// root of the fixtures: the set-limit fixture with another kind.
func budgetRootWriteAnswer(t *testing.T, kind string, replayed bool) []byte {
	t.Helper()
	data := budgetReplace(t, budgetFixture(t, "write-200-replayed-set-limit.json"), `"operation_kind":"set_limit"`, `"operation_kind":"`+kind+`"`)
	if !replayed {
		data = budgetReplace(t, data, `"replayed":true`, `"replayed":false`)
	}
	return data
}

// budgetErrorJSON is the body of a refusal by the budget handlers.
func budgetErrorJSON(code string) []byte {
	return []byte(`{"error":"` + code + `","code":"` + code + `"}` + "\n")
}

// The group is registered under `multica issue` with its nine subcommands, and
// every subcommand prints a table unless json is asked for.
func TestIssueBudgetIsRegisteredWithItsSubcommands(t *testing.T) {
	found, _, err := issueCmd.Find([]string{"budget"})
	if err != nil || found != issueBudgetCmd {
		t.Fatalf("`multica issue budget` is not registered under the issue command (found %q, %v)", found.CommandPath(), err)
	}
	var names []string
	for _, sub := range issueBudgetCmd.Commands() {
		names = append(names, sub.Name())
		if output := sub.Flags().Lookup("output"); output == nil || output.DefValue != "table" {
			t.Errorf("%s: --output must be there and default to table", sub.Name())
		}
	}
	sort.Strings(names)
	if got, want := strings.Join(names, " "), "attempts close enable observations operation reopen set-limit set-mode show"; got != want {
		t.Fatalf("subcommands = %s, want %s", got, want)
	}
}

// The help of the group tells a script how to read what became of a write:
// exit code 0 for a commit only, and otherwise the first words of the message.
// A refusal by a conflict and an unknown outcome share exit code 1, so the
// words the help quotes must be the ones the commands print.
func TestIssueBudgetHelpSaysHowToTellTheOutcomesApart(t *testing.T) {
	help := issueBudgetCmd.Long
	if !strings.Contains(help, "A write exits with 0 only when its operation is committed.") {
		t.Errorf("the help does not say what exit code 0 means:\n%s", help)
	}
	write := budgetWriteCases[0]
	for _, tc := range []struct {
		name  string
		body  string
		first string // the first words of the message, as the help quotes them
	}{
		{"a 409 that is a refusal", string(budgetErrorJSON("already_enabled")), "refused ("},
		{"a 409 that is not an answer of the budget handlers", `{"error":"already_enabled"}`, "outcome unknown: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(help, `"`+tc.first+`"`) {
				t.Errorf("the help does not quote %q:\n%s", tc.first, help)
			}
			fake := newBudgetFake(t)
			fake.answer("POST", write.path, 409, []byte(tc.body))
			run := runBudgetCLI(t, write.args...)
			if run.exit != 1 || !strings.HasPrefix(run.stderr, tc.first) {
				t.Errorf("exit %d, stderr %q: want exit 1 and a message that starts with %q", run.exit, run.stderr, tc.first)
			}
			fake.wantRequests("POST " + write.path)
		})
	}
}

// Each subcommand sends one request (`show <issue>` two), to the route of
// design 12.2, with the workspace and the credential of the operator and
// without an agent or task identity. A write sends the bytes of its request
// fixture, without the line feed that ends the file.
func TestIssueBudgetRequests(t *testing.T) {
	const nastyCursor = "a+b/c=d&e f"
	type planned struct {
		method, path string
		status       int
		body         func(t *testing.T) []byte
	}
	fixture := func(name string) func(t *testing.T) []byte {
		return func(t *testing.T) []byte { return budgetFixture(t, name) }
	}
	for _, tc := range []struct {
		name    string
		args    []string
		answers []planned
		want    []string // the requests, as "METHOD target"
		body    string   // the request fixture of a write
	}{
		{
			name:    "enable",
			args:    budgetEnableArgs,
			answers: []planned{{"POST", budgetTestEnablePath, 201, fixture("write-201-enable.json")}},
			want:    []string{"POST " + budgetTestEnablePath},
			body:    "enable-request.json",
		},
		{
			name:    "set-limit",
			args:    budgetSetLimitArgs,
			answers: []planned{{"POST", budgetTestOperationsPath, 200, fixture("write-200-replayed-set-limit.json")}},
			want:    []string{"POST " + budgetTestOperationsPath},
			body:    "operation-set-limit-request.json",
		},
		{
			name: "set-mode",
			args: budgetSetModeArgs,
			answers: []planned{{"POST", budgetTestOperationsPath, 201, func(t *testing.T) []byte {
				return budgetRootWriteAnswer(t, "set_mode", false)
			}}},
			want: []string{"POST " + budgetTestOperationsPath},
			body: "operation-set-mode-request.json",
		},
		{
			name: "close",
			args: budgetCloseArgs,
			answers: []planned{{"POST", budgetTestOperationsPath, 201, func(t *testing.T) []byte {
				return budgetRootWriteAnswer(t, "close", false)
			}}},
			want: []string{"POST " + budgetTestOperationsPath},
			body: "operation-close-request.json",
		},
		{
			name: "reopen",
			args: budgetReopenArgs,
			answers: []planned{{"POST", budgetTestOperationsPath, 201, func(t *testing.T) []byte {
				return budgetRootWriteAnswer(t, "reopen", false)
			}}},
			want: []string{"POST " + budgetTestOperationsPath},
			body: "operation-reopen-request.json",
		},
		{
			name:    "show by root",
			args:    []string{"show", "--root", budgetTestRoot},
			answers: []planned{{"GET", budgetTestRootPath, 200, fixture("show-open.json")}},
			want:    []string{"GET " + budgetTestRootPath},
		},
		{
			name: "show by issue",
			args: []string{"show", budgetTestIssue},
			answers: []planned{
				{"GET", budgetTestEnablePath, 200, fixture("issue-binding.json")},
				{"GET", budgetTestRootPath, 200, fixture("show-open.json")},
			},
			want: []string{"GET " + budgetTestEnablePath, "GET " + budgetTestRootPath},
		},
		{
			name:    "operation by root",
			args:    []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestSetLimitKey},
			answers: []planned{{"GET", budgetTestOperationsPath + "/" + budgetTestSetLimitKey, 200, fixture("operation-committed-stale.json")}},
			want:    []string{"GET " + budgetTestOperationsPath + "/" + budgetTestSetLimitKey},
		},
		{
			name:    "operation by issue",
			args:    []string{"operation", "--issue", budgetTestIssue, "--request-key", budgetTestEnableKey},
			answers: []planned{{"GET", budgetTestEnablePath + "/operations/" + budgetTestEnableKey, 200, fixture("operation-committed.json")}},
			want:    []string{"GET " + budgetTestEnablePath + "/operations/" + budgetTestEnableKey},
		},
		{
			name:    "observations",
			args:    []string{"observations", "--root", budgetTestRoot},
			answers: []planned{{"GET", budgetTestRootPath + "/observations", 200, fixture("observations-last-page.json")}},
			want:    []string{"GET " + budgetTestRootPath + "/observations"},
		},
		{
			name:    "observations with a limit and a cursor",
			args:    []string{"observations", "--root", budgetTestRoot, "--limit", "2", "--cursor", "abc"},
			answers: []planned{{"GET", budgetTestRootPath + "/observations", 200, fixture("observations-empty.json")}},
			want:    []string{"GET " + budgetTestRootPath + "/observations?cursor=abc&limit=2"},
		},
		{
			name:    "attempts",
			args:    []string{"attempts", "--root", budgetTestRoot},
			answers: []planned{{"GET", budgetTestRootPath + "/attempts", 200, fixture("attempts-unavailable.json")}},
			want:    []string{"GET " + budgetTestRootPath + "/attempts"},
		},
		{
			name:    "attempts with a cursor that needs escaping",
			args:    []string{"attempts", "--root", budgetTestRoot, "--limit", "100", "--cursor", nastyCursor},
			answers: []planned{{"GET", budgetTestRootPath + "/attempts", 200, fixture("attempts-empty.json")}},
			want:    []string{"GET " + budgetTestRootPath + "/attempts?cursor=a%2Bb%2Fc%3Dd%26e+f&limit=100"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newBudgetFake(t)
			for _, answer := range tc.answers {
				fake.answer(answer.method, answer.path, answer.status, answer.body(t))
			}
			run := runBudgetCLI(t, tc.args...)
			if run.exit != 0 || run.stderr != "" {
				t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
			}
			fake.wantRequests(tc.want...)
			for _, request := range fake.seen() {
				if got := request.Header.Get("X-Workspace-ID"); got != budgetTestWorkspace {
					t.Errorf("%s: X-Workspace-ID = %q, want the workspace of the operator", request.Target, got)
				}
				if got := request.Header.Get("Authorization"); got != "Bearer "+budgetTestToken {
					t.Errorf("%s: Authorization = %q, want the token of the operator", request.Target, got)
				}
				for _, name := range []string{"X-Agent-ID", "X-Task-ID", "Idempotency-Key"} {
					if values := request.Header.Values(name); len(values) != 0 {
						t.Errorf("%s: header %s = %q, want none", request.Target, name, values)
					}
				}
				if request.Method == http.MethodGet {
					if len(request.Body) != 0 {
						t.Errorf("%s: a read sent a body: %s", request.Target, request.Body)
					}
					continue
				}
				want := bytes.TrimSuffix(budgetFixture(t, tc.body), []byte("\n"))
				if !bytes.Equal(request.Body, want) {
					t.Errorf("%s: body\n got: %s\nwant: %s", request.Target, request.Body, want)
				}
				if got := request.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("%s: Content-Type = %q", request.Target, got)
				}
			}
			if cursor := fake.seen()[len(fake.seen())-1]; strings.Contains(cursor.Target, "%2B") {
				parsed, err := url.ParseRequestURI(cursor.Target)
				if err != nil || parsed.Query().Get("cursor") != nastyCursor {
					t.Errorf("the server reads the cursor back as %q (%v), want %q", parsed.Query().Get("cursor"), err, nastyCursor)
				}
			}
		})
	}
}

// What the operator types as an issue goes into the path as one segment.
func TestIssueBudgetEscapesTheIssueInThePath(t *testing.T) {
	const issue = "we/ird id?x=1#f"
	const escaped = "/api/issues/we%2Fird%20id%3Fx=1%23f/budget"
	for _, tc := range []struct {
		name   string
		args   []string
		method string
		path   string
		status int
		answer string
	}{
		{"enable", append([]string{"enable", issue}, budgetEnableArgs[2:]...), "POST", escaped, 201, "write-201-enable.json"},
		{"show", []string{"show", issue}, "GET", escaped, 404, "error-not-found.json"},
		{"operation", []string{"operation", "--issue", issue, "--request-key", budgetTestEnableKey}, "GET", escaped + "/operations/" + budgetTestEnableKey, 200, "operation-committed.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.fixture(tc.method, tc.path, tc.status, tc.answer)
			runBudgetCLI(t, tc.args...)
			fake.wantRequests(tc.method + " " + tc.path)
		})
	}
}

// The body of a write is the same bytes on every Go line whatever the reason
// holds, and the server reads the reason back as it was typed.
func TestIssueBudgetWriteBodyBytes(t *testing.T) {
	lineSeparator, emoji := string(rune(0x2028)), string(rune(0x1F600))
	reason := "a <b> & \"c\" \\ d\te" + lineSeparator + "f " + emoji + " é"
	// U+ stands for a backslash and a u: the encoder writes these five
	// characters as escapes.
	wantReason := strings.ReplaceAll(`a U+003cbU+003e U+0026 \"c\" \\ d\teU+2028f `, "U+", "\\u") + emoji + " é"
	want := `{"kind":"close","request_key":"` + budgetTestCloseKey + `","reason":"` + wantReason + `","expected_policy_revision":"3"}`

	fake := newBudgetFake(t)
	fake.answer("POST", budgetTestOperationsPath, 201, budgetRootWriteAnswer(t, "close", false))
	run := runBudgetCLI(t, "close", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey, "--reason", reason, "--expect-revision", "3", "--yes")
	if run.exit != 0 {
		t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
	}
	fake.wantRequests("POST " + budgetTestOperationsPath)
	body := fake.seen()[0].Body
	if string(body) != want {
		t.Fatalf("body\n got: %s\nwant: %s", body, want)
	}
	var decoded struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Reason != reason {
		t.Fatalf("the server reads the reason back as %q (%v), want %q", decoded.Reason, err, reason)
	}

	// What else a reason must be is the server's to say: an empty or a blank
	// one is sent as it was typed, and the refusal is the server's.
	for name, typed := range map[string]string{"an empty reason": "", "a blank reason": " \t "} {
		t.Run(name+" is sent as it was typed", func(t *testing.T) {
			fake := newBudgetFake(t)
			fake.answer("POST", budgetTestOperationsPath, 400, budgetErrorJSON("invalid_request"))
			run := runBudgetCLI(t, "close", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey, "--reason", typed, "--expect-revision", "3", "--yes")
			if run.exit != 5 || run.stdout != "" || !strings.HasPrefix(run.stderr, "refused (invalid_request): ") {
				t.Errorf("exit %d, stdout %q, stderr %q: want the refusal of the server", run.exit, run.stdout, run.stderr)
			}
			fake.wantRequests("POST " + budgetTestOperationsPath)
			var sent struct {
				Reason *string `json:"reason"`
			}
			if err := json.Unmarshal(fake.seen()[0].Body, &sent); err != nil || sent.Reason == nil || *sent.Reason != typed {
				t.Errorf("body %s (%v): want the reason %q as it was typed", fake.seen()[0].Body, err, typed)
			}
		})
	}
}

// These commands are for people, the reads as much as the writes: every kind of
// credential or surrounding that is not a person at a terminal is refused
// before anything is sent.
func TestIssueBudgetIsForPeopleOnly(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		{"a read", []string{"show", "--root", budgetTestRoot}},
		{"a write", budgetCloseArgs},
	}
	credentials := []struct {
		name    string
		set     func(t *testing.T)
		profile string
	}{
		{name: "a task token", set: func(t *testing.T) { t.Setenv("MULTICA_TOKEN", cli.TaskTokenPrefix+"fake_task") }},
		{name: "a daemon token", set: func(t *testing.T) { t.Setenv("MULTICA_TOKEN", "mdt_fake_daemon") }},
		{name: "a cloud token", set: func(t *testing.T) { t.Setenv("MULTICA_TOKEN", auth.CloudPATPrefix+"fake_cloud") }},
		{name: "an agent id beside a human token", set: func(t *testing.T) { t.Setenv("MULTICA_AGENT_ID", "agent-1") }},
		{name: "a task id beside a human token", set: func(t *testing.T) { t.Setenv("MULTICA_TASK_ID", "task-1") }},
		{name: "the surrounding of a daemon task", set: func(t *testing.T) {
			t.Setenv("MULTICA_TOKEN", cli.TaskTokenPrefix+"fake_task")
			t.Setenv("MULTICA_AGENT_ID", "agent-1")
			t.Setenv("MULTICA_TASK_ID", "task-1")
		}},
		{name: "a task profile root", set: func(t *testing.T) { t.Setenv(cli.TaskConfigRootEnv, t.TempDir()) }},
		{name: "a daemon port beside a human token", set: func(t *testing.T) { t.Setenv("MULTICA_DAEMON_PORT", "19514") }},
		{name: "a daemon port with a task token", set: func(t *testing.T) {
			t.Setenv("MULTICA_DAEMON_PORT", "19514")
			t.Setenv("MULTICA_TOKEN", cli.TaskTokenPrefix+"fake_task")
		}},
		{name: "a collaboration run", set: func(t *testing.T) { t.Setenv(cli.CollaborationTokenEnv, "fake_collaboration") }},
		{name: "no token at all", set: func(t *testing.T) { t.Setenv("MULTICA_TOKEN", "") }},
		{name: "a task token that comes from the profile", profile: "robot", set: func(t *testing.T) {
			saveBudgetProfile(t, "robot", cli.TaskTokenPrefix+"fake_profile_task")
		}},
	}
	for _, command := range commands {
		for _, credential := range credentials {
			t.Run(command.name+" with "+credential.name, func(t *testing.T) {
				fake := newBudgetFake(t)
				credential.set(t)
				args := command.args
				if credential.profile != "" {
					args = append(append([]string{}, args...), "--profile", credential.profile)
				}
				run := runBudgetCLI(t, args...)
				if run.exit == 0 || run.stderr == "" {
					t.Errorf("exit %d, stderr %q: want a refusal", run.exit, run.stderr)
				}
				if run.stdout != "" {
					t.Errorf("stdout = %q, want nothing", run.stdout)
				}
				fake.wantRequests()
			})
		}
	}

	// The check is the first thing every subcommand does, not only the two
	// above: each of them, with everything it needs typed in full, is refused
	// and sends nothing.
	everySubcommand := [][]string{
		budgetEnableArgs, budgetSetLimitArgs, budgetSetModeArgs, budgetCloseArgs, budgetReopenArgs,
		{"show", "--root", budgetTestRoot},
		{"show", budgetTestIssue},
		{"operation", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey},
		{"operation", "--issue", budgetTestIssue, "--request-key", budgetTestEnableKey},
		{"observations", "--root", budgetTestRoot},
		{"attempts", "--root", budgetTestRoot},
	}
	for _, args := range everySubcommand {
		for _, credential := range credentials {
			if credential.profile != "" {
				continue
			}
			name := args[0]
			if len(args) > 1 {
				name += " " + args[1]
			}
			t.Run("every subcommand: "+name+" with "+credential.name, func(t *testing.T) {
				fake := newBudgetFake(t)
				credential.set(t)
				for _, output := range []string{"table", "json"} {
					run := runBudgetCLI(t, append(append([]string{}, args...), "--output", output)...)
					if run.exit == 0 || run.stderr == "" || run.stdout != "" {
						t.Errorf("--output %s: exit %d, stdout %q, stderr %q: want a refusal and nothing printed", output, run.exit, run.stdout, run.stderr)
					}
				}
				fake.wantRequests()
			})
		}
	}

	// The same profile with the token of a person is let through, so the
	// refusal above is about the token and not about the profile.
	t.Run("a read with a human token that comes from the profile", func(t *testing.T) {
		fake := newBudgetFake(t)
		fake.fixture("GET", budgetTestRootPath, 200, "show-open.json")
		saveBudgetProfile(t, "person", "mul_fake_profile_human")
		run := runBudgetCLI(t, "show", "--root", budgetTestRoot, "--profile", "person")
		if run.exit != 0 {
			t.Fatalf("exit %d, stderr %q", run.exit, run.stderr)
		}
		fake.wantRequests("GET " + budgetTestRootPath)
		if got := fake.seen()[0].Header.Get("Authorization"); got != "Bearer mul_fake_profile_human" {
			t.Fatalf("Authorization = %q, want the token of the profile", got)
		}
	})
}

// saveBudgetProfile writes a named profile under the temporary home of the
// test and takes the token out of the environment, so the profile gives it.
func saveBudgetProfile(t *testing.T, profile, token string) {
	t.Helper()
	t.Setenv("MULTICA_TOKEN", "")
	config := cli.CLIConfig{Token: token, ServerURL: os.Getenv("MULTICA_SERVER_URL"), WorkspaceID: budgetTestWorkspace}
	if err := cli.SaveCLIConfigForProfile(config, profile); err != nil {
		t.Fatalf("save the profile: %v", err)
	}
}

// The environment is where an agent or task identity comes from, and the check
// above stops there. Should a client ever carry one by another way, it is
// refused as well.
func TestIssueBudgetRefusesAClientThatIsNotAPerson(t *testing.T) {
	for name, client := range map[string]*cli.APIClient{
		"an agent id":    {Token: budgetTestToken, AgentID: "agent-1"},
		"a task id":      {Token: budgetTestToken, TaskID: "task-1"},
		"a task token":   {Token: cli.TaskTokenPrefix + "fake_task"},
		"a daemon token": {Token: "mdt_fake_daemon"},
		"a cloud token":  {Token: auth.CloudPATPrefix + "fake_cloud"},
		"no token":       {},
	} {
		if err := budgetHumanCredential(client, "show"); err == nil {
			t.Errorf("a client with %s was accepted", name)
		}
	}
	if err := budgetHumanCredential(&cli.APIClient{Token: budgetTestToken}, "show"); err != nil {
		t.Errorf("a client with the token of a person was refused: %v", err)
	}
}

// A workspace is needed before anything is sent: every budget route is inside
// one.
func TestIssueBudgetNeedsAWorkspace(t *testing.T) {
	for name, workspace := range map[string]string{"none": "", "not a UUID": "my-workspace", "upper case": strings.ToUpper(budgetTestWorkspace)} {
		for _, args := range [][]string{{"show", "--root", budgetTestRoot}, budgetCloseArgs} {
			t.Run(args[0]+" with "+name, func(t *testing.T) {
				fake := newBudgetFake(t)
				t.Setenv("MULTICA_WORKSPACE_ID", workspace)
				run := runBudgetCLI(t, args...)
				if run.exit == 0 || !strings.Contains(run.stderr, "workspace") || run.stdout != "" {
					t.Errorf("exit %d, stdout %q, stderr %q: want an error about the workspace", run.exit, run.stdout, run.stderr)
				}
				fake.wantRequests()
			})
		}
	}
}

// budgetWithout takes one flag, and its value when it has one, out of a command
// line.
func budgetWithout(args []string, flag string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if flag != "--yes" {
				i++
			}
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// budgetTypedIDs returns everything UUID-shaped that a command line holds.
func budgetTypedIDs(args []string) []string {
	return budgetUUIDShaped.FindAllString(strings.Join(args, " "), -1)
}

// wantBudgetLocalRefusal runs a command line that must be refused before
// anything is sent: an error that names what is wrong, nothing on stdout, no
// request, and no key in the error that the operator did not type.
func wantBudgetLocalRefusal(t *testing.T, mention string, args ...string) {
	t.Helper()
	fake := newBudgetFake(t)
	run := runBudgetCLI(t, args...)
	if run.exit == 0 || !strings.Contains(run.stderr, mention) {
		t.Errorf("exit %d, stderr %q: want an error that names %s", run.exit, run.stderr, mention)
	}
	if run.stdout != "" {
		t.Errorf("stdout = %q, want nothing", run.stdout)
	}
	run.wantOnlyTypedIDs(t, budgetTypedIDs(args)...)
	fake.wantRequests()
}

// Nothing is filled in for the operator: every flag of a write must be given,
// --yes included, and so must what a read is addressed by.
func TestIssueBudgetRequiresEveryFlag(t *testing.T) {
	for _, full := range [][]string{budgetEnableArgs, budgetSetLimitArgs, budgetSetModeArgs, budgetCloseArgs, budgetReopenArgs} {
		for _, flag := range full {
			if !strings.HasPrefix(flag, "--") {
				continue
			}
			t.Run(full[0]+" without "+flag, func(t *testing.T) {
				wantBudgetLocalRefusal(t, flag, budgetWithout(full, flag)...)
			})
		}
		t.Run(full[0]+" with --yes=false", func(t *testing.T) {
			wantBudgetLocalRefusal(t, "--yes", append(budgetWithout(full, "--yes"), "--yes=false")...)
		})
	}
	for _, tc := range []struct {
		name    string
		mention string
		args    []string
	}{
		{"enable without an issue", "issue", append([]string{"enable"}, budgetEnableArgs[2:]...)},
		{"enable with two issues", "issue", append([]string{"enable", "MUL-13"}, budgetEnableArgs[1:]...)},
		{"enable with an empty issue", "issue", append([]string{"enable", ""}, budgetEnableArgs[2:]...)},
		{"close with an argument", "argument", append([]string{"close", budgetTestIssue}, budgetCloseArgs[1:]...)},
		{"show without a root or an issue", "--root", []string{"show"}},
		{"show with a root and an issue", "--root", []string{"show", budgetTestIssue, "--root", budgetTestRoot}},
		{"show with two issues", "issue", []string{"show", budgetTestIssue, "MUL-13"}},
		{"show with an empty issue", "issue", []string{"show", ""}},
		{"operation without a key", "--request-key", []string{"operation", "--root", budgetTestRoot}},
		{"operation without a root or an issue", "--issue", []string{"operation", "--request-key", budgetTestCloseKey}},
		{"operation with a root and an issue", "--issue", []string{"operation", "--root", budgetTestRoot, "--issue", budgetTestIssue, "--request-key", budgetTestCloseKey}},
		{"operation with an empty issue", "--issue", []string{"operation", "--issue", "", "--request-key", budgetTestEnableKey}},
		{"operation with an argument", "argument", []string{"operation", budgetTestIssue, "--root", budgetTestRoot, "--request-key", budgetTestCloseKey}},
		{"observations without a root", "--root", []string{"observations"}},
		{"attempts without a root", "--root", []string{"attempts", "--limit", "5"}},
		{"attempts with an argument", "argument", []string{"attempts", budgetTestRoot}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantBudgetLocalRefusal(t, tc.mention, tc.args...)
		})
	}
}

// A flag in a form the server would not take is refused before anything is
// sent. The forms of an amount have their own table in
// cmd_issue_budget_money_test.go; here one of each kind shows that the flag is
// read through it.
func TestIssueBudgetRefusesAFlagInTheWrongForm(t *testing.T) {
	with := func(full []string, flag, value string) []string {
		out := append([]string{}, full...)
		for i := range out {
			if out[i] == flag {
				out[i+1] = value
				return out
			}
		}
		return append(out, flag, value)
	}
	show := []string{"show", "--root", budgetTestRoot}
	operation := []string{"operation", "--root", budgetTestRoot, "--request-key", budgetTestCloseKey}
	observations := []string{"observations", "--root", budgetTestRoot}
	notUTF8 := string([]byte{'b', 'a', 'd', 0xff})
	for _, tc := range []struct {
		name  string
		full  []string
		flag  string
		value string
	}{
		{"a key in upper case", budgetCloseArgs, "--request-key", strings.ToUpper(budgetTestCloseKey)},
		{"a key of zeros", budgetCloseArgs, "--request-key", "00000000-0000-0000-0000-000000000000"},
		{"a key without dashes", budgetEnableArgs, "--request-key", strings.ReplaceAll(budgetTestEnableKey, "-", "")},
		{"a key in braces", budgetSetLimitArgs, "--request-key", "{" + budgetTestSetLimitKey + "}"},
		{"a key as a URN", budgetSetModeArgs, "--request-key", "urn:uuid:" + budgetTestSetModeKey},
		{"an empty key", budgetReopenArgs, "--request-key", ""},
		{"a key that is a word", operation, "--request-key", "my-first-key"},
		{"a key in upper case for the query", operation, "--request-key", strings.ToUpper(budgetTestCloseKey)},

		{"an issue identifier as the root", budgetCloseArgs, "--root", budgetTestIssue},
		{"a root in upper case", budgetSetLimitArgs, "--root", strings.ToUpper(budgetTestRoot)},
		{"a root of zeros", show, "--root", "00000000-0000-0000-0000-000000000000"},
		{"an empty root", observations, "--root", ""},
		{"an issue identifier as the root of a query", operation, "--root", budgetTestIssue},

		{"revision zero", budgetCloseArgs, "--expect-revision", "0"},
		{"a negative revision", budgetCloseArgs, "--expect-revision", "-1"},
		{"a revision with a sign", budgetReopenArgs, "--expect-revision", "+4"},
		{"a revision with a leading zero", budgetSetLimitArgs, "--expect-revision", "03"},
		{"a revision with a fraction", budgetSetModeArgs, "--expect-revision", "3.0"},
		{"a revision with an exponent", budgetCloseArgs, "--expect-revision", "1e2"},
		{"a revision with a space", budgetCloseArgs, "--expect-revision", " 3"},
		{"an empty revision", budgetCloseArgs, "--expect-revision", ""},
		{"a revision above the largest", budgetCloseArgs, "--expect-revision", "9223372036854775808"},
		{"a revision that is a word", budgetCloseArgs, "--expect-revision", "latest"},

		{"a limit with eleven decimals", budgetEnableArgs, "--limit", "1.00000000001"},
		{"a negative limit", budgetSetLimitArgs, "--limit", "-1"},
		{"an empty limit", budgetSetLimitArgs, "--limit", ""},
		{"a reserve with an exponent", budgetEnableArgs, "--attempt-reserve", "1e3"},
		{"a reserve spelled None", budgetSetLimitArgs, "--attempt-reserve", "None"},

		{"mode on", budgetSetModeArgs, "--mode", "on"},
		{"mode Off", budgetSetModeArgs, "--mode", "Off"},
		{"mode enforce", budgetSetModeArgs, "--mode", "enforce"},
		{"an empty mode", budgetSetModeArgs, "--mode", ""},

		{"a reason that is not UTF-8", budgetCloseArgs, "--reason", notUTF8},
		{"a reason that is not UTF-8 on enable", budgetEnableArgs, "--reason", notUTF8},

		{"output yaml on a read", show, "--output", "yaml"},
		{"output JSON on a write", budgetCloseArgs, "--output", "JSON"},
		{"an empty output", observations, "--output", ""},

		{"a page of zero rows", observations, "--limit", "0"},
		{"a page of 101 rows", observations, "--limit", "101"},
		{"a negative page", observations, "--limit", "-1"},
		{"a page with a fraction", []string{"attempts", "--root", budgetTestRoot}, "--limit", "1.5"},
		{"a page that is a word", observations, "--limit", "all"},
		{"an empty page", observations, "--limit", ""},
		{"an empty cursor", observations, "--cursor", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantBudgetLocalRefusal(t, tc.flag, with(tc.full, tc.flag, tc.value)...)
		})
	}
}
