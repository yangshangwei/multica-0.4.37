package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Stage D: declaring the protocol, and sending the batch of an attempt.

// budgetUsageTestCall is one request the daemon made to budgetUsageTestAPI.
type budgetUsageTestCall struct {
	Method        string
	Path          string
	Authorization string
	ContentType   string
	Capabilities  string
	Body          []byte
}

// budgetUsageTestAnswer answers one usage report. nth counts the reports the
// server has received, the one being answered included.
type budgetUsageTestAnswer func(nth int, w http.ResponseWriter)

// budgetUsageTestAPI is a daemon API whose budget usage route answers what a
// test tells it to. Every other route accepts the request, like the daemon API
// of the capture tests.
type budgetUsageTestAPI struct {
	URL    string
	server *httptest.Server

	mu       sync.Mutex
	calls    []budgetUsageTestCall
	reported int
	answer   budgetUsageTestAnswer
}

func budgetUsageTestNewAPI(t *testing.T, answer budgetUsageTestAnswer) *budgetUsageTestAPI {
	t.Helper()
	api := &budgetUsageTestAPI{answer: answer}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		call := budgetUsageTestCall{
			Method:        r.Method,
			Path:          r.URL.Path,
			Authorization: r.Header.Get("Authorization"),
			ContentType:   r.Header.Get("Content-Type"),
			Capabilities:  r.Header.Get("X-Client-Capabilities"),
			Body:          body,
		}
		api.mu.Lock()
		api.calls = append(api.calls, call)
		nth := 0
		if budgetUsageTestIsReport(call) {
			api.reported++
			nth = api.reported
		}
		answer := api.answer
		api.mu.Unlock()
		if nth > 0 {
			answer(nth, w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/status") {
			_, _ = io.WriteString(w, `{"status":"running"}`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(api.server.Close)
	api.URL = api.server.URL
	return api
}

// budgetUsageTestReportPath is the route of the usage report of an attempt.
func budgetUsageTestReportPath(attemptID string) string {
	return "/api/daemon/budget/attempts/" + attemptID + "/usage"
}

func budgetUsageTestIsReport(call budgetUsageTestCall) bool {
	return strings.HasPrefix(call.Path, "/api/daemon/budget/")
}

// seen returns every request received so far, in order.
func (a *budgetUsageTestAPI) seen() []budgetUsageTestCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]budgetUsageTestCall(nil), a.calls...)
}

// reports returns the requests to the budget usage route, in order.
func (a *budgetUsageTestAPI) reports() []budgetUsageTestCall {
	var reports []budgetUsageTestCall
	for _, call := range a.seen() {
		if budgetUsageTestIsReport(call) {
			reports = append(reports, call)
		}
	}
	return reports
}

// others returns method and path of every request outside the budget usage
// route, in order.
func (a *budgetUsageTestAPI) others() []string {
	var others []string
	for _, call := range a.seen() {
		if !budgetUsageTestIsReport(call) {
			others = append(others, call.Method+" "+call.Path)
		}
	}
	return others
}

// budgetUsageTestReplying answers a report with a status and a body.
func budgetUsageTestReplying(status int, body string) budgetUsageTestAnswer {
	return func(_ int, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// budgetUsageTestRedirecting answers a report with a redirect to a page that
// accepts whatever it is asked, as the sign-in page of a device in between
// would.
func budgetUsageTestRedirecting(status int) budgetUsageTestAnswer {
	return func(_ int, w http.ResponseWriter) {
		w.Header().Set("Location", "/landing")
		w.WriteHeader(status)
	}
}

// budgetUsageTestCutShort answers a report with a body that ends before the
// length it declared: an answer that cannot be read to its end.
func budgetUsageTestCutShort(status int) budgetUsageTestAnswer {
	return func(_ int, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"result":"recorded"`)
	}
}

// budgetUsageTestHangingUp reads a report and closes the connection without
// an answer.
func budgetUsageTestHangingUp(t *testing.T) budgetUsageTestAnswer {
	return func(_ int, w http.ResponseWriter) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server cannot take over its connection")
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("take over the connection: %v", err)
			return
		}
		_ = conn.Close()
	}
}

// budgetUsageTestThen answers the first report with first and every later one
// with rest.
func budgetUsageTestThen(first, rest budgetUsageTestAnswer) budgetUsageTestAnswer {
	return func(nth int, w http.ResponseWriter) {
		if nth == 1 {
			first(nth, w)
			return
		}
		rest(nth, w)
	}
}

// budgetUsageTestRecorded is the answer of a server that took the batch.
func budgetUsageTestRecorded() budgetUsageTestAnswer {
	return budgetUsageTestReplying(http.StatusOK, `{"result":"recorded"}`)
}

const budgetUsageTestToken = "mul_budget_usage_test_token"

// budgetUsageTestSendingDaemon is a daemon that talks to api and keeps its
// usage report records in a journal of its own.
func budgetUsageTestSendingDaemon(t *testing.T, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
	t.Helper()
	logger, logs := budgetUsageTestLogger()
	journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
	d := budgetUsageTestNewDaemon(&budgetUsageTestServer{URL: api.URL}, journal, logger)
	d.client.SetToken(budgetUsageTestToken)
	return d, journal, logs
}

// budgetUsageTestSealed creates the record of an attempt that launched one
// provider process, seals it and returns the bytes of its batch.
func budgetUsageTestSealed(t *testing.T, journal *budgetUsageJournal, logger *slog.Logger, seed budgetUsageSeed) []byte {
	t.Helper()
	attempt, err := journal.begin(seed, logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	attempt.describeLaunch("claude", false, "2.1.34")
	invocationID := attempt.beginInvocation(false)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	batch := attempt.seal()
	if len(batch) == 0 {
		t.Fatal("the attempt has no batch")
	}
	budgetUsageTestRequireStored(t, journal, attempt, "after sealing")
	return batch
}

// budgetUsageTestWaitForSends returns once no send that handleTask started in
// the background is still running.
func budgetUsageTestWaitForSends(t *testing.T, journal *budgetUsageJournal) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		journal.background.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("a usage report sent in the background did not end")
	}
}

// budgetUsageTestCapabilityCount says how often a capabilities header names a
// capability.
func budgetUsageTestCapabilityCount(header, capability string) int {
	count := 0
	for _, declared := range strings.Split(header, ",") {
		if declared == capability {
			count++
		}
	}
	return count
}

// budgetUsageTestBatchKey returns the batch key a request body carries.
func budgetUsageTestBatchKey(t *testing.T, body []byte) string {
	t.Helper()
	var batch struct {
		BatchKey string `json:"batch_key"`
	}
	if err := json.Unmarshal(body, &batch); err != nil {
		t.Fatalf("decode the batch %q: %v", body, err)
	}
	return batch.BatchKey
}

// budgetUsageTestCallsTo returns the calls whose path ends in suffix.
func budgetUsageTestCallsTo(calls []budgetUsageTestCall, suffix string) []budgetUsageTestCall {
	var matching []budgetUsageTestCall
	for _, call := range calls {
		if strings.HasSuffix(call.Path, suffix) {
			matching = append(matching, call)
		}
	}
	return matching
}

// budgetUsageTestProfileDaemon is a daemon as Run has it right before it opens
// the journal: a profile whose state directory lies under a root of the test's
// own, and no journal yet. It returns the daemon, its log and the directory
// the journal belongs in.
func budgetUsageTestProfileDaemon(t *testing.T, api *budgetUsageTestAPI) (*Daemon, *budgetUsageTestLog, string) {
	t.Helper()
	t.Setenv(cli.TaskConfigRootEnv, t.TempDir())
	logger, logs := budgetUsageTestLogger()
	d := budgetUsageTestNewDaemon(&budgetUsageTestServer{URL: api.URL}, nil, logger)
	d.cfg.ServerBaseURL = api.URL
	d.cfg.Profile = "budget-profile"
	profileDirectory, err := cli.ProfileDir(d.cfg.Profile)
	if err != nil {
		t.Fatalf("resolve the profile directory: %v", err)
	}
	return d, logs, filepath.Join(profileDirectory, "budget-usage-reports")
}

// budgetUsageTestDeclared makes the daemon claim a task over HTTP and open
// its WebSocket, and returns the capabilities header of each request.
func budgetUsageTestDeclared(t *testing.T, d *Daemon, api *budgetUsageTestAPI) (claim, handshake string) {
	t.Helper()
	before := len(api.seen())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := d.client.ClaimTask(ctx, budgetUsageTestRuntimeID); err != nil {
		t.Fatalf("claim over HTTP: %v", err)
	}
	// The test server does not speak WebSocket: the handshake request arrives
	// and the connection attempt ends there.
	if _, err := d.runTaskWakeupConnection(ctx, []string{budgetUsageTestRuntimeID}, make(chan taskWakeup, 1), nil); err == nil {
		t.Fatal("the WebSocket handshake with a server that does not upgrade succeeded")
	}
	calls := api.seen()[before:]
	claims := budgetUsageTestCallsTo(calls, "/api/daemon/runtimes/"+budgetUsageTestRuntimeID+"/tasks/claim")
	handshakes := budgetUsageTestCallsTo(calls, "/api/daemon/ws")
	if len(claims) != 1 || len(handshakes) != 1 {
		t.Fatalf("%d claim requests and %d handshake requests, want one of each: %+v", len(claims), len(handshakes), calls)
	}
	return claims[0].Capabilities, handshakes[0].Capabilities
}

// budgetUsageTestCompletedRun is a provider run that ends by itself and used
// something.
func budgetUsageTestCompletedRun() budgetUsageTestBackend {
	return budgetUsageTestReturning(agent.Result{Status: "completed", Usage: budgetUsageTestUsage()})
}

// TestBudgetUsageCapabilityIsDeclaredWithAUsableJournal: the daemon opens the
// journal in a directory of its own under the profile's state directory, and
// from then on declares the protocol where a claim is made: in the
// capabilities header of the HTTP claim and of the WebSocket handshake.
func TestBudgetUsageCapabilityIsDeclaredWithAUsableJournal(t *testing.T) {
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, logs, directory := budgetUsageTestProfileDaemon(t, api)

	claim, handshake := budgetUsageTestDeclared(t, d, api)
	for name, header := range map[string]string{"claim over HTTP": claim, "WebSocket handshake": handshake} {
		if got := budgetUsageTestCapabilityCount(header, protocol.DaemonCapabilityBudgetUsageV1); got != 0 {
			t.Fatalf("before the journal is open the %s declares the protocol: %s", name, header)
		}
	}

	d.initializeBudgetUsage()

	journal := d.budgetUsage
	if journal == nil {
		t.Fatalf("the daemon has no journal\nlog:\n%s", logs.String())
	}
	if journal.directory != directory {
		t.Errorf("journal directory = %s, want %s: a directory of its own under the profile's state directory", journal.directory, directory)
	}
	if journal.serverURL != api.URL {
		t.Errorf("records are created under the server %q, want the daemon's server %q", journal.serverURL, api.URL)
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		t.Fatalf("the journal directory does not exist: %v", err)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("a journal that was just opened holds %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("opening a usable journal logged %d warnings:\n%s", got, logs.String())
	}

	claim, handshake = budgetUsageTestDeclared(t, d, api)
	for name, header := range map[string]string{"claim over HTTP": claim, "WebSocket handshake": handshake} {
		if got := budgetUsageTestCapabilityCount(header, protocol.DaemonCapabilityBudgetUsageV1); got != 1 {
			t.Errorf("with a usable journal the %s names the protocol %d times, want once: %s", name, got, header)
		}
		if !strings.HasPrefix(header, daemonClientCapabilities()+",") {
			t.Errorf("the %s lost a capability the daemon declared before: %s", name, header)
		}
	}

	// A record is created under the daemon's own server and reported there.
	d.runner = budgetUsageTestRunner(d, nil, budgetUsageTestCompletedRun())
	d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)
	budgetUsageTestWaitForSends(t, journal)
	if reports := api.reports(); len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
		t.Errorf("usage reports = %+v, want one for the attempt of the task", reports)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal still holds %v after the report was delivered", names)
	}
}

// TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal: a daemon that
// cannot keep records says one warning, does not declare the protocol
// anywhere, and runs tasks as it did before the protocol.
func TestBudgetUsageCapabilityIsNotDeclaredWithoutAUsableJournal(t *testing.T) {
	cases := []struct {
		name string
		// spoil makes the journal unusable before the daemon opens it.
		spoil func(t *testing.T, directory string)
	}{
		{"the journal path is a file", func(t *testing.T, directory string) {
			if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"the journal directory cannot be written", func(t *testing.T, directory string) {
			budgetUsageTestRequirePOSIX(t)
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			budgetUsageTestChmod(t, directory, 0o500)
		}},
		{"the profile directory cannot be resolved", func(t *testing.T, _ string) {
			t.Setenv(cli.TaskConfigRootEnv, filepath.Join("relative", "root"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
			d, logs, directory := budgetUsageTestProfileDaemon(t, api)
			tc.spoil(t, directory)

			d.initializeBudgetUsage()

			if d.budgetUsage != nil {
				t.Fatalf("the daemon holds a journal in %s", d.budgetUsage.directory)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 1 {
				t.Errorf("a journal that cannot be opened logged %d warnings, want 1:\n%s", got, logs.String())
			}
			claim, handshake := budgetUsageTestDeclared(t, d, api)
			for name, header := range map[string]string{"claim over HTTP": claim, "WebSocket handshake": handshake} {
				if header != daemonClientCapabilities() {
					t.Errorf("without a journal the %s declares %s, want what the daemon declared before the protocol: %s", name, header, daemonClientCapabilities())
				}
			}

			// Tasks still run, a task with a budget attempt included.
			d.runner = budgetUsageTestRunner(d, nil, budgetUsageTestCompletedRun())
			d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)
			if completed := budgetUsageTestCallsTo(api.seen(), "/tasks/"+budgetUsageTestTaskID+"/complete"); len(completed) != 1 {
				t.Errorf("the task was reported complete %d times, want once", len(completed))
			}
			if reports := api.reports(); len(reports) != 0 {
				t.Errorf("a daemon without a journal sent %d usage reports", len(reports))
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 1 {
				t.Errorf("%d warnings after the task, want still 1:\n%s", got, logs.String())
			}
		})
	}
}

const (
	budgetUsageTestWorkspaceA = "0195f3b0-0000-7000-8000-0000000000a1"
	budgetUsageTestWorkspaceB = "0195f3b0-0000-7000-8000-0000000000b2"
	budgetUsageTestWorkspaceC = "0195f3b0-0000-7000-8000-0000000000c3"
)

// budgetUsageTestInstallBinding gives a managed installation a verified
// binding for a workspace, with a credential of its own.
func budgetUsageTestInstallBinding(t *testing.T, client *Client, workspaceID, token string) {
	t.Helper()
	err := client.installManagedCredential(workspaceID, ManagedDaemonCredential{
		BindingID:         "55555555-5555-4555-8555-555555555555",
		BindingEpoch:      "1",
		CapabilityVersion: "1",
		DaemonToken:       token,
		ExpiresAt:         time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		PrincipalUserID:   "66666666-6666-4666-8666-666666666666",
		AuthVersion:       "1",
	})
	if err != nil {
		t.Fatalf("install the binding of workspace %s: %v", workspaceID, err)
	}
}

// budgetUsageTestWorkspaceClient returns the client a managed installation
// uses for a workspace.
func budgetUsageTestWorkspaceClient(t *testing.T, client *Client, workspaceID string) *Client {
	t.Helper()
	client.managed.mu.RLock()
	defer client.managed.mu.RUnlock()
	workspace := client.managed.workspaces[workspaceID]
	if workspace == nil {
		t.Fatalf("no client for workspace %s", workspaceID)
	}
	return workspace.client
}

// TestBudgetUsageCapabilityReachesManagedWorkspaceClients: a managed
// installation claims through one client per workspace. Opening the journal
// declares the protocol on the clients that exist, and a client made later
// declares it from its first request.
func TestBudgetUsageCapabilityReachesManagedWorkspaceClients(t *testing.T) {
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, logs, _ := budgetUsageTestProfileDaemon(t, api)
	d.client.enableManagedTransport()
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceA, "mdt_workspace_a")
	existing := budgetUsageTestWorkspaceClient(t, d.client, budgetUsageTestWorkspaceA)
	if got := budgetUsageTestCapabilityCount(existing.capabilities(), protocol.DaemonCapabilityBudgetUsageV1); got != 0 {
		t.Fatalf("before the journal is open a workspace client declares the protocol: %s", existing.capabilities())
	}

	d.initializeBudgetUsage()
	if d.budgetUsage == nil {
		t.Fatalf("the daemon has no journal\nlog:\n%s", logs.String())
	}
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceB, "mdt_workspace_b")
	later := budgetUsageTestWorkspaceClient(t, d.client, budgetUsageTestWorkspaceB)

	for name, client := range map[string]*Client{
		"the client of the installation":                          d.client,
		"a workspace client that existed when the journal opened": existing,
		"a workspace client made after the journal opened":        later,
	} {
		if got := budgetUsageTestCapabilityCount(client.capabilities(), protocol.DaemonCapabilityBudgetUsageV1); got != 1 {
			t.Errorf("%s names the protocol %d times, want once: %s", name, got, client.capabilities())
		}
	}
	for name, client := range map[string]*Client{"existing": existing, "later": later} {
		if got := budgetUsageTestCapabilityCount(client.capabilities(), protocol.DaemonCapabilityManagedInstallationV1); got != 1 {
			t.Errorf("the %s workspace client names the managed installation capability %d times, want once: %s", name, got, client.capabilities())
		}
	}
}

// budgetUsageTestReplyText stands in every refusal of these tests for text of
// a reply body that must not reach the log.
const budgetUsageTestReplyText = "REPLY-TEXT-THAT-MUST-NOT-BE-LOGGED"

// budgetUsageTestRefusing answers a report with an error of the shape the
// server writes: a sentence and a code.
func budgetUsageTestRefusing(status int, code string) budgetUsageTestAnswer {
	return budgetUsageTestReplying(status, `{"error":"`+budgetUsageTestReplyText+`","code":"`+code+`"}`)
}

// budgetUsageTestRequireNoBodyLogged fails when the log carries text of a
// request body or of a reply body.
func budgetUsageTestRequireNoBodyLogged(t *testing.T, logged string, batch []byte) {
	t.Helper()
	for _, private := range []string{budgetUsageTestReplyText, budgetUsageTestBatchKey(t, batch), "protocol_version", `"invocations"`, "model-x", "result", "recorded"} {
		if strings.Contains(logged, private) {
			t.Errorf("the log carries %q, which is text of a request body or of a reply body:\n%s", private, logged)
		}
	}
}

// TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord sends the batch of one
// sealed record and gives it every kind of answer. Any 2xx means delivered:
// the record is deleted. 400, 404, 409, 413 and 422 are final: the record is
// deleted with one warning. Everything else keeps the record as it is, and
// the next send carries the same bytes under the same key.
func TestBudgetUsageAnswerDecidesWhatBecomesOfTheRecord(t *testing.T) {
	const (
		delivered = "delivered"
		refused   = "refused"
		kept      = "kept"
	)
	type answerCase struct {
		name   string
		answer budgetUsageTestAnswer
		want   string
		// status and code are what the warning of a refusal names.
		status int
		code   string
	}
	cases := []answerCase{
		{name: "200 recorded", answer: budgetUsageTestReplying(http.StatusOK, `{"result":"recorded"}`), want: delivered},
		{name: "200 not recorded", answer: budgetUsageTestReplying(http.StatusOK, `{"result":"not_recorded","reason":"observation_off"}`), want: delivered},
		{name: "200 with an empty body", answer: budgetUsageTestReplying(http.StatusOK, ""), want: delivered},
		{name: "200 with a body that is not JSON", answer: budgetUsageTestReplying(http.StatusOK, "<html>recorded</html>"), want: delivered},
		{name: "201", answer: budgetUsageTestReplying(http.StatusCreated, `{}`), want: delivered},
		{name: "202", answer: budgetUsageTestReplying(http.StatusAccepted, `{}`), want: delivered},
		{name: "204", answer: budgetUsageTestReplying(http.StatusNoContent, ""), want: delivered},
		{name: "299", answer: budgetUsageTestReplying(299, ""), want: delivered},

		{name: "400 invalid_request", answer: budgetUsageTestRefusing(http.StatusBadRequest, "invalid_request"), want: refused, status: 400, code: "invalid_request"},
		{name: "404 not_found", answer: budgetUsageTestRefusing(http.StatusNotFound, "not_found"), want: refused, status: 404, code: "not_found"},
		{name: "409 operation_conflict", answer: budgetUsageTestRefusing(http.StatusConflict, "operation_conflict"), want: refused, status: 409, code: "operation_conflict"},
		{name: "409 batch_conflict", answer: budgetUsageTestRefusing(http.StatusConflict, "batch_conflict"), want: refused, status: 409, code: "batch_conflict"},
		{name: "409 protocol_mismatch", answer: budgetUsageTestRefusing(http.StatusConflict, "protocol_mismatch"), want: refused, status: 409, code: "protocol_mismatch"},
		{name: "413 request_too_large", answer: budgetUsageTestRefusing(http.StatusRequestEntityTooLarge, "request_too_large"), want: refused, status: 413, code: "request_too_large"},
		{name: "422 invalid_batch", answer: budgetUsageTestRefusing(http.StatusUnprocessableEntity, "invalid_batch"), want: refused, status: 422, code: "invalid_batch"},

		{name: "401", answer: budgetUsageTestRefusing(http.StatusUnauthorized, "unauthorized"), want: kept},
		{name: "402", answer: budgetUsageTestRefusing(http.StatusPaymentRequired, "payment_required"), want: kept},
		{name: "403", answer: budgetUsageTestRefusing(http.StatusForbidden, "forbidden"), want: kept},
		{name: "405", answer: budgetUsageTestRefusing(http.StatusMethodNotAllowed, "method_not_allowed"), want: kept},
		{name: "408", answer: budgetUsageTestRefusing(http.StatusRequestTimeout, "request_timeout"), want: kept},
		{name: "410", answer: budgetUsageTestRefusing(http.StatusGone, "gone"), want: kept},
		{name: "418", answer: budgetUsageTestRefusing(http.StatusTeapot, "teapot"), want: kept},
		{name: "425", answer: budgetUsageTestRefusing(http.StatusTooEarly, "too_early"), want: kept},
		{name: "429", answer: budgetUsageTestRefusing(http.StatusTooManyRequests, "too_many_requests"), want: kept},
		{name: "500", answer: budgetUsageTestRefusing(http.StatusInternalServerError, "internal_error"), want: kept},
		{name: "502", answer: budgetUsageTestReplying(http.StatusBadGateway, "<html>bad gateway</html>"), want: kept},
		{name: "503 ledger_unavailable", answer: budgetUsageTestRefusing(http.StatusServiceUnavailable, "ledger_unavailable"), want: kept},
		{name: "504", answer: budgetUsageTestReplying(http.StatusGatewayTimeout, ""), want: kept},
		{name: "300 with a body", answer: budgetUsageTestReplying(http.StatusMultipleChoices, `{"result":"recorded"}`), want: kept},
		{name: "301 redirect", answer: budgetUsageTestRedirecting(http.StatusMovedPermanently), want: kept},
		{name: "302 redirect", answer: budgetUsageTestRedirecting(http.StatusFound), want: kept},
		{name: "303 redirect", answer: budgetUsageTestRedirecting(http.StatusSeeOther), want: kept},
		{name: "304", answer: budgetUsageTestReplying(http.StatusNotModified, ""), want: kept},
		{name: "307 redirect", answer: budgetUsageTestRedirecting(http.StatusTemporaryRedirect), want: kept},
		{name: "308 redirect", answer: budgetUsageTestRedirecting(http.StatusPermanentRedirect), want: kept},
		{name: "200 with a body that ends early", answer: budgetUsageTestCutShort(http.StatusOK), want: kept},
		{name: "400 with a body that ends early", answer: budgetUsageTestCutShort(http.StatusBadRequest), want: kept},
		{name: "the connection is closed without an answer", answer: budgetUsageTestHangingUp(t), want: kept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestThen(tc.answer, budgetUsageTestRecorded()))
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
			attempt := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA)
			batchKey := attempt.snapshot().BatchKey
			fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			ctx := context.Background()

			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)

			reports := api.reports()
			if len(reports) != 1 {
				t.Fatalf("%d usage reports, want 1: %+v", len(reports), reports)
			}
			if !bytes.Equal(reports[0].Body, batch) {
				t.Fatalf("the report does not carry the sealed bytes\n got:  %s\n want: %s", reports[0].Body, batch)
			}
			if landed := budgetUsageTestCallsTo(api.seen(), "/landing"); len(landed) != 0 {
				t.Fatalf("the daemon followed a redirect: %+v", landed)
			}
			logged := logs.String()
			budgetUsageTestRequireNoBodyLogged(t, logged, batch)

			if tc.want == kept {
				if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
					t.Errorf("the record file changed although the report was not delivered\n before: %s\n after:  %s", fileBefore, got)
				}
				if live := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA); live != attempt {
					t.Errorf("the journal's process holds %p for the attempt, want the record that was kept %p", live, attempt)
				}
				if got := budgetUsageTestWarnings(logged); got != 0 {
					t.Errorf("a report that is kept for the next send logged %d warnings:\n%s", got, logged)
				}

				// The next send: the same bytes under the same key.
				d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
				reports = api.reports()
				if len(reports) != 2 {
					t.Fatalf("%d usage reports after the second send, want 2", len(reports))
				}
				if !bytes.Equal(reports[1].Body, reports[0].Body) {
					t.Errorf("the second send carries other bytes than the first\n first:  %s\n second: %s", reports[0].Body, reports[1].Body)
				}
				if got := budgetUsageTestBatchKey(t, reports[1].Body); got != batchKey {
					t.Errorf("the second send carries the key %q, want the key of the record %q", got, batchKey)
				}
				if reports[1].Path != reports[0].Path {
					t.Errorf("the second send went to %s, the first to %s", reports[1].Path, reports[0].Path)
				}
			} else {
				warnings := budgetUsageTestWarningLines(logged)
				switch tc.want {
				case delivered:
					if len(warnings) != 0 {
						t.Errorf("a delivered report logged %d warnings:\n%s", len(warnings), logged)
					}
				case refused:
					if len(warnings) != 1 {
						t.Fatalf("a report that was refused for good logged %d warnings, want 1:\n%s", len(warnings), logged)
					}
					budgetUsageTestRequireLogAttr(t, warnings[0], "status", strconv.Itoa(tc.status))
					budgetUsageTestRequireLogAttr(t, warnings[0], "code", tc.code)
					budgetUsageTestRequireLogAttr(t, warnings[0], "attempt_id", budgetUsageTestAttemptA)
				}
			}

			// Delivered, refused, or delivered by the second send: the record is
			// gone, file and entry, and nothing sends it again.
			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Errorf("the journal still holds %v", names)
			}
			if live := budgetUsageTestLiveCount(journal); live != 0 {
				t.Errorf("the journal's process still holds %d records", live)
			}
			sent := len(api.reports())
			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
			if got := len(api.reports()); got != sent {
				t.Errorf("a record that is gone was sent again: %d reports, want %d", got, sent)
			}
			budgetUsageTestRequireNoBodyLogged(t, logs.String(), batch)
		})
	}
}

// budgetUsageTestWarningLines returns the warnings of a log.
func budgetUsageTestWarningLines(logged string) []string {
	var warnings []string
	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, "level=WARN") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

// budgetUsageTestLogAttr returns the value of an attribute of a log line.
func budgetUsageTestLogAttr(line, key string) (string, bool) {
	for _, field := range strings.Fields(line) {
		if value, ok := strings.CutPrefix(field, key+"="); ok {
			return value, true
		}
	}
	return "", false
}

func budgetUsageTestRequireLogAttr(t *testing.T, line, key, want string) {
	t.Helper()
	if got, ok := budgetUsageTestLogAttr(line, key); !ok || got != want {
		t.Errorf("the log line names %s=%q (present: %v), want %q: %s", key, got, ok, want, line)
	}
}

// TestBudgetUsageRefusalWarningNamesTheCodeAndNeverTheBody: the warning of a
// final refusal names the status and the error code of the answer. The code
// is the answer's "code"; an answer without one gives its "error" when that
// has the form of a code. Text that is not a code is not logged, and neither
// is anything else of the answer or of the batch.
func TestBudgetUsageRefusalWarningNamesTheCodeAndNeverTheBody(t *testing.T) {
	const marker = budgetUsageTestReplyText
	longestCode := "a" + strings.Repeat("b", 63)
	cases := []struct {
		name string
		body string
		// wantCode is the code the warning names; empty means that it names none.
		wantCode string
	}{
		{"a code next to a sentence", `{"error":"` + marker + `","code":"attempt_not_found"}`, "attempt_not_found"},
		{"no code and an error in the form of a code", `{"error":"invalid_request","detail":"` + marker + `"}`, "invalid_request"},
		{"no code and an error that is a sentence", `{"error":"` + marker + ` not found"}`, ""},
		{"an empty code counts as no code", `{"code":"","error":"batch_conflict","detail":"` + marker + `"}`, "batch_conflict"},
		{"a code that is a sentence", `{"code":"` + marker + ` not found","error":"invalid_request"}`, ""},
		{"a code that starts with a digit", `{"code":"4xx","detail":"` + marker + `"}`, ""},
		{"a code with an upper-case letter", `{"code":"Invalid_request","detail":"` + marker + `"}`, ""},
		{"a code with a hyphen", `{"code":"invalid-request","detail":"` + marker + `"}`, ""},
		{"a code of 64 bytes", `{"code":"` + longestCode + `","detail":"` + marker + `"}`, longestCode},
		{"a code of 65 bytes", `{"code":"` + longestCode + `b","detail":"` + marker + `"}`, ""},
		{"a code that is not a string", `{"code":404,"error":"invalid_request","detail":"` + marker + `"}`, ""},
		{"a body that is not JSON", marker + " <html>", ""},
		{"a JSON body that is not an object", `["` + marker + `"]`, ""},
		{"an empty body", "", ""},
		{"a code behind more text than the daemon reads for it", `{"detail":"` + strings.Repeat("x", 256<<10) + marker + `","code":"invalid_request"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestReplying(http.StatusBadRequest, tc.body))
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))

			d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)

			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Fatalf("the journal still holds %v after a final refusal", names)
			}
			logged := logs.String()
			warnings := budgetUsageTestWarningLines(logged)
			if len(warnings) != 1 {
				t.Fatalf("%d warnings, want 1:\n%s", len(warnings), logged)
			}
			budgetUsageTestRequireLogAttr(t, warnings[0], "status", "400")
			if got, named := budgetUsageTestLogAttr(warnings[0], "code"); got != tc.wantCode || named != (tc.wantCode != "") {
				t.Errorf("the warning names the code %q (present: %v), want %q: %s", got, named, tc.wantCode, warnings[0])
			}
			budgetUsageTestRequireNoBodyLogged(t, logged, batch)
		})
	}
}

// budgetUsageTestHold makes a server hold its first usage report open until
// the test lets it go.
type budgetUsageTestHold struct {
	arrived     chan struct{} // closed when the held report reached the server
	release     chan struct{} // closed to let the server answer
	arriveOnce  sync.Once
	releaseOnce sync.Once
}

// budgetUsageTestNewHoldingAPI is a daemon API that holds its first usage
// report open and then answers it, and every later report, with answer. The
// report is let go when the test ends at the latest, before the server closes.
func budgetUsageTestNewHoldingAPI(t *testing.T, answer budgetUsageTestAnswer) (*budgetUsageTestAPI, *budgetUsageTestHold) {
	t.Helper()
	hold := &budgetUsageTestHold{arrived: make(chan struct{}), release: make(chan struct{})}
	api := budgetUsageTestNewAPI(t, func(nth int, w http.ResponseWriter) {
		if nth == 1 {
			hold.arriveOnce.Do(func() { close(hold.arrived) })
			<-hold.release
		}
		answer(nth, w)
	})
	t.Cleanup(hold.letGo)
	return api, hold
}

func (h *budgetUsageTestHold) letGo() {
	h.releaseOnce.Do(func() { close(h.release) })
}

// budgetUsageTestAwait fails the test when done is not closed in time.
func budgetUsageTestAwait(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out waiting: %s", what)
	}
}

// budgetUsageTestLegacyCalls returns method, path and body of every request
// outside the budget usage route, in order.
func budgetUsageTestLegacyCalls(api *budgetUsageTestAPI) []string {
	var calls []string
	for _, call := range api.seen() {
		if !budgetUsageTestIsReport(call) {
			calls = append(calls, call.Method+" "+call.Path+" "+string(call.Body))
		}
	}
	return calls
}

// budgetUsageTestFailingRunner is a task runner that launches one provider
// process and then fails the task.
func budgetUsageTestFailingRunner(d *Daemon, backend agent.Backend) taskRunnerFunc {
	return func(ctx context.Context, task Task, _ string, _ int, log *slog.Logger) (TaskResult, error) {
		var msgSeq atomic.Int32
		if _, _, err := d.executeAndDrainRecorded(ctx, backend, "prompt", agent.ExecOptions{}, log, task.ID, "", &msgSeq); err != nil {
			return TaskResult{}, err
		}
		return TaskResult{}, errors.New("the run failed")
	}
}

// TestBudgetUsageHandleTaskSendsTheSealedBatch: as soon as the run is over and
// the record is sealed, the daemon posts the bytes it persisted to the usage
// route of the attempt, with the credential of its other requests. Once the
// server took them the record is gone.
func TestBudgetUsageHandleTaskSendsTheSealedBatch(t *testing.T) {
	var (
		journalAtArrival atomic.Pointer[budgetUsageJournal]
		persisted        atomic.Pointer[[]byte]
	)
	api := budgetUsageTestNewAPI(t, func(nth int, w http.ResponseWriter) {
		// What the record holds on disk while the report is being answered.
		if journal := journalAtArrival.Load(); journal != nil {
			if stored, err := journal.read(budgetUsageTestAttemptA); err == nil {
				persisted.Store(&stored.Batch)
			}
		}
		budgetUsageTestRecorded()(nth, w)
	})
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	journalAtArrival.Store(journal)
	d.runner = budgetUsageTestRunner(d, nil, budgetUsageTestCompletedRun())

	d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)
	budgetUsageTestWaitForSends(t, journal)

	reports := api.reports()
	if len(reports) != 1 {
		t.Fatalf("%d usage reports, want 1: %+v", len(reports), reports)
	}
	report := reports[0]
	if report.Method != http.MethodPost || report.Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
		t.Errorf("the report is %s %s, want POST %s", report.Method, report.Path, budgetUsageTestReportPath(budgetUsageTestAttemptA))
	}
	if report.ContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", report.ContentType)
	}
	if report.Authorization != "Bearer "+budgetUsageTestToken {
		t.Errorf("Authorization = %q, want the daemon's own credential", report.Authorization)
	}
	if !strings.HasPrefix(report.Capabilities, daemonClientCapabilities()) {
		t.Errorf("the report does not identify the daemon like its other requests: X-Client-Capabilities = %q", report.Capabilities)
	}
	stored := persisted.Load()
	if stored == nil || len(*stored) == 0 {
		t.Fatal("the record held no batch bytes on disk when its report arrived")
	}
	if !bytes.Equal(report.Body, *stored) {
		t.Errorf("the report does not carry the persisted bytes\n sent:      %s\n persisted: %s", report.Body, *stored)
	}
	invocations := budgetUsageTestBatchInvocations(t, report.Body)
	if len(invocations) != 1 || invocations[0].Ended != budgetUsageEndedReturned || len(invocations[0].Usage) != 1 {
		t.Errorf("the batch does not report the one launch of the run: %s", report.Body)
	}

	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Errorf("the journal still holds %v after the report was delivered", names)
	}
	if live := budgetUsageTestLiveCount(journal); live != 0 {
		t.Errorf("the journal's process still holds %d records", live)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("a delivered report logged %d warnings:\n%s", got, logs.String())
	}
	budgetUsageTestRequireNoBodyLogged(t, logs.String(), report.Body)
}

// TestBudgetUsageHeldOpenReportDoesNotDelayOrChangeTheTerminalReport: a
// server that does not answer the usage report holds up nothing. The task is
// reported complete or failed while the report is still open, with the very
// requests of a daemon without the protocol.
func TestBudgetUsageHeldOpenReportDoesNotDelayOrChangeTheTerminalReport(t *testing.T) {
	cases := []struct {
		name     string
		runner   func(d *Daemon) taskRunnerFunc
		terminal string
	}{
		{"a task that completes", func(d *Daemon) taskRunnerFunc {
			return budgetUsageTestRunner(d, nil, budgetUsageTestCompletedRun())
		}, "/complete"},
		{"a task that fails", func(d *Daemon) taskRunnerFunc {
			return budgetUsageTestFailingRunner(d, budgetUsageTestCompletedRun())
		}, "/fail"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := budgetUsageTestTask(budgetUsageTestAttemptA)

			// The same task on a daemon that did not declare the protocol.
			plainAPI := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
			plainLogger, _ := budgetUsageTestLogger()
			plain := budgetUsageTestNewDaemon(&budgetUsageTestServer{URL: plainAPI.URL}, nil, plainLogger)
			plain.client.SetToken(budgetUsageTestToken)
			plain.runner = tc.runner(plain)
			plain.handleTask(context.Background(), task, 0)
			want := budgetUsageTestLegacyCalls(plainAPI)
			if terminal := budgetUsageTestCallsTo(plainAPI.seen(), "/tasks/"+task.ID+tc.terminal); len(terminal) != 1 {
				t.Fatalf("without the protocol the task has %d %s reports, want 1: %v", len(terminal), tc.terminal, want)
			}

			api, hold := budgetUsageTestNewHoldingAPI(t, budgetUsageTestRecorded())
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			d.runner = tc.runner(d)
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				d.handleTask(context.Background(), task, 0)
			}()

			budgetUsageTestAwait(t, hold.arrived, "the usage report reaches the server")
			budgetUsageTestAwait(t, returned, "handleTask returns while the usage report is still open")
			if got := budgetUsageTestLegacyCalls(api); !reflect.DeepEqual(got, want) {
				t.Errorf("the requests of the task differ from those of a daemon without the protocol\n got:  %v\n want: %v", got, want)
			}
			if names := budgetUsageTestNames(t, journal.directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
				t.Errorf("the journal holds %v while the report is open, want the record of the attempt", names)
			}

			// The answer comes after the task is over: the report still counts.
			hold.letGo()
			budgetUsageTestWaitForSends(t, journal)
			if reports := api.reports(); len(reports) != 1 {
				t.Errorf("%d usage reports, want 1", len(reports))
			}
			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Errorf("the journal still holds %v after the report was answered", names)
			}
			if got := budgetUsageTestLegacyCalls(api); !reflect.DeepEqual(got, want) {
				t.Errorf("the answer to the usage report made the daemon send something else\n got:  %v\n want: %v", got, want)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 0 {
				t.Errorf("%d warnings:\n%s", got, logs.String())
			}
		})
	}
}

// budgetUsageTestWatchedRun is a provider run that goes on until the test lets
// it end, and that keeps the context it runs under: the test can see whether
// the daemon interrupted it.
type budgetUsageTestWatchedRun struct {
	result  agent.Result
	started chan struct{} // closed when the run was launched
	finish  chan struct{} // closed by the test to let the run end

	mu  sync.Mutex
	ctx context.Context
}

func budgetUsageTestNewWatchedRun(result agent.Result) *budgetUsageTestWatchedRun {
	return &budgetUsageTestWatchedRun{result: result, started: make(chan struct{}), finish: make(chan struct{})}
}

func (r *budgetUsageTestWatchedRun) Execute(ctx context.Context, _ string, _ agent.ExecOptions) (*agent.Session, error) {
	r.mu.Lock()
	r.ctx = ctx
	r.mu.Unlock()
	messages := make(chan agent.Message)
	results := make(chan agent.Result, 1)
	go func() {
		select {
		case <-r.finish:
		case <-ctx.Done():
		}
		results <- r.result
		close(messages)
	}()
	close(r.started)
	return &agent.Session{Messages: messages, Result: results}, nil
}

// interrupted says whether the context of the run was cancelled.
func (r *budgetUsageTestWatchedRun) interrupted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ctx != nil && r.ctx.Err() != nil
}

// TestBudgetUsageNotFoundAnswerOnlyDropsTheRecord: on its other routes the
// daemon reads a 404 that says "runtime not found", "task not found" or
// "workspace not found" as an order: register the runtime again, interrupt
// the agent, drop the workspace. From the usage route the same answer means
// one thing only, the record is refused for good. A task that is running goes
// on, completes and is reported once.
func TestBudgetUsageNotFoundAnswerOnlyDropsTheRecord(t *testing.T) {
	for _, text := range []string{"runtime not found", "task not found", "workspace not found"} {
		t.Run(text, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestReplying(http.StatusNotFound, `{"error":"`+text+`"}`))
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			// The record of an attempt that is over, waiting to be sent.
			budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptB))
			// A task whose provider is in the middle of its run.
			run := budgetUsageTestNewWatchedRun(agent.Result{Status: "completed", Usage: budgetUsageTestUsage()})
			t.Cleanup(func() {
				select {
				case <-run.finish:
				default:
					close(run.finish)
				}
			})
			d.runner = budgetUsageTestRunner(d, nil, run)
			task := budgetUsageTestTask(budgetUsageTestAttemptA)
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				d.handleTask(context.Background(), task, 0)
			}()
			budgetUsageTestAwait(t, run.started, "the provider of the task is launched")

			d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptB)

			if names := budgetUsageTestNames(t, journal.directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
				t.Errorf("the journal holds %v, want only the record of the running task: the refused record is gone", names)
			}
			if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptB) != nil {
				t.Error("the journal's process still holds the refused record")
			}
			if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
				t.Error("the journal's process lost the record of the running task")
			}
			warnings := budgetUsageTestWarningLines(logs.String())
			if len(warnings) != 1 {
				t.Fatalf("%d warnings after the refusal, want 1:\n%s", len(warnings), logs.String())
			}
			budgetUsageTestRequireLogAttr(t, warnings[0], "status", "404")
			budgetUsageTestRequireLogAttr(t, warnings[0], "attempt_id", budgetUsageTestAttemptB)
			if code, named := budgetUsageTestLogAttr(warnings[0], "code"); named {
				t.Errorf("the warning names the code %q; the answer carried a sentence, not a code", code)
			}
			if strings.Contains(logs.String(), text) {
				t.Errorf("the log carries the text of the answer %q:\n%s", text, logs.String())
			}

			// Nothing else follows from the answer.
			d.mu.Lock()
			_, registered := d.runtimeIndex[budgetUsageTestRuntimeID]
			d.mu.Unlock()
			if !registered {
				t.Error("the daemon dropped its runtime")
			}
			if run.interrupted() {
				t.Error("the daemon interrupted the agent of the running task")
			}
			select {
			case <-returned:
				t.Fatal("the running task ended with the refusal")
			default:
			}

			close(run.finish)
			budgetUsageTestAwait(t, returned, "the task ends once its provider is done")
			budgetUsageTestWaitForSends(t, journal)

			// The report of the task itself got the same answer: its record is
			// gone as well, and the task is reported complete all the same.
			reports := api.reports()
			if len(reports) != 2 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptB) || reports[1].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
				t.Fatalf("usage reports = %+v, want one per attempt", reports)
			}
			if invocations := budgetUsageTestBatchInvocations(t, reports[1].Body); len(invocations) != 1 || invocations[0].Ended != budgetUsageEndedReturned {
				t.Errorf("the batch of the task does not say that its provider returned: %s", reports[1].Body)
			}
			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Errorf("the journal still holds %v", names)
			}
			if completed := budgetUsageTestCallsTo(api.seen(), "/tasks/"+task.ID+"/complete"); len(completed) != 1 {
				t.Errorf("the task was reported complete %d times, want once", len(completed))
			}
			if failed := budgetUsageTestCallsTo(api.seen(), "/tasks/"+task.ID+"/fail"); len(failed) != 0 {
				t.Errorf("the task was reported failed %d times", len(failed))
			}
			for _, other := range api.others() {
				if !strings.Contains(other, " /api/daemon/tasks/"+task.ID+"/") {
					t.Errorf("the daemon made a request that is not about the task: %s", other)
				}
			}
			if got := len(budgetUsageTestWarningLines(logs.String())); got != 2 {
				t.Errorf("%d warnings, want 2: one per refused record\n%s", got, logs.String())
			}
			if strings.Contains(logs.String(), text) {
				t.Errorf("the log carries the text of the answer %q:\n%s", text, logs.String())
			}
		})
	}
}

// TestBudgetUsageRecordIsNeverSentTwiceAtOnce: the send that follows the seal
// and the resend loop share one mark per record. While a send is under way a
// second one leaves the record alone; once it is over the record can be sent
// again.
func TestBudgetUsageRecordIsNeverSentTwiceAtOnce(t *testing.T) {
	api, hold := budgetUsageTestNewHoldingAPI(t, budgetUsageTestThen(
		budgetUsageTestRefusing(http.StatusServiceUnavailable, "ledger_unavailable"), budgetUsageTestRecorded()))
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	ctx := context.Background()

	first := make(chan struct{})
	go func() {
		defer close(first)
		d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	}()
	budgetUsageTestAwait(t, hold.arrived, "the first send reaches the server")

	second := make(chan struct{})
	go func() {
		defer close(second)
		d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	}()
	budgetUsageTestAwait(t, second, "a second send returns while the first is under way")
	if reports := api.reports(); len(reports) != 1 {
		t.Fatalf("%d usage reports while the first send is under way, want 1", len(reports))
	}

	hold.letGo()
	budgetUsageTestAwait(t, first, "the first send ends")
	if names := budgetUsageTestNames(t, journal.directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
		t.Fatalf("the journal holds %v, want the record that was not delivered", names)
	}

	// The mark went with the first send.
	d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	reports := api.reports()
	if len(reports) != 2 {
		t.Fatalf("%d usage reports after the first send ended, want 2", len(reports))
	}
	for i, report := range reports {
		if !bytes.Equal(report.Body, batch) {
			t.Errorf("report %d does not carry the sealed bytes: %s", i+1, report.Body)
		}
	}
	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Errorf("the journal still holds %v after the report was delivered", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageSendEndsWithTheDaemonAndKeepsTheRecord: a send that is under
// way when the daemon stops ends with it. The record stays for the next start.
func TestBudgetUsageSendEndsWithTheDaemonAndKeepsTheRecord(t *testing.T) {
	api, hold := budgetUsageTestNewHoldingAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	}()
	budgetUsageTestAwait(t, hold.arrived, "the send reaches the server")
	stop()
	budgetUsageTestAwait(t, done, "the send ends when the daemon stops")

	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
		t.Errorf("the record file changed\n before: %s\n after:  %s", fileBefore, got)
	}
	if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
		t.Error("the journal's process lost the record")
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("a send that ended with the daemon logged %d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageUnreachableServerKeepsTheRecord: no connection, no answer.
// The record stays as it is, and a server that is down is not worth a warning
// per send.
func TestBudgetUsageUnreachableServerKeepsTheRecord(t *testing.T) {
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
	api.server.Close()

	d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)

	if reports := api.reports(); len(reports) != 0 {
		t.Fatalf("a closed server received %d usage reports", len(reports))
	}
	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
		t.Errorf("the record file changed\n before: %s\n after:  %s", fileBefore, got)
	}
	if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
		t.Error("the journal's process lost the record")
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("a send without a connection logged %d warnings:\n%s", got, logs.String())
	}
}

// budgetUsageTestExpireBinding makes the binding of a workspace one whose
// credential ran out.
func budgetUsageTestExpireBinding(t *testing.T, client *Client, workspaceID string) {
	t.Helper()
	client.managed.mu.Lock()
	defer client.managed.mu.Unlock()
	workspace := client.managed.workspaces[workspaceID]
	if workspace == nil {
		t.Fatalf("no binding for workspace %s", workspaceID)
	}
	workspace.credential.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
}

// TestBudgetUsageManagedInstallationSendsThroughTheWorkspaceOfTheRecord: a
// managed installation holds one credential per workspace. A record is sent
// with the credential of the workspace it was claimed in and with no other; a
// record whose workspace has no valid binding stays until there is one.
func TestBudgetUsageManagedInstallationSendsThroughTheWorkspaceOfTheRecord(t *testing.T) {
	const (
		attemptInA       = "0195f3c0-0000-7000-8000-000000000001"
		attemptInB       = "0195f3c0-0000-7000-8000-000000000002"
		attemptUnbound   = "0195f3c0-0000-7000-8000-000000000003"
		attemptExpired   = "0195f3c0-0000-7000-8000-000000000004"
		installationAuth = "Bearer " + budgetUsageTestToken
	)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	d.client.enableManagedTransport()
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceA, "mdt_workspace_a")
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceB, "mdt_workspace_b")
	ctx := context.Background()

	seal := func(attemptID, workspaceID string) []byte {
		seed := budgetUsageTestSeed(attemptID)
		seed.WorkspaceID = workspaceID
		return budgetUsageTestSealed(t, journal, d.logger, seed)
	}
	// requireSent delivers the record of an attempt and requires that exactly
	// one more report went out, for that attempt, with that credential.
	requireSent := func(attemptID string, batch []byte, wantToken string) {
		t.Helper()
		before := len(api.reports())
		d.deliverBudgetUsage(ctx, attemptID)
		reports := api.reports()
		if len(reports) != before+1 {
			t.Fatalf("attempt %s: %d new usage reports, want 1", attemptID, len(reports)-before)
		}
		report := reports[before]
		if report.Path != budgetUsageTestReportPath(attemptID) || !bytes.Equal(report.Body, batch) {
			t.Errorf("attempt %s: the report is %s with the body %s", attemptID, report.Path, report.Body)
		}
		if report.Authorization != "Bearer "+wantToken {
			t.Errorf("attempt %s: Authorization = %q, want the credential of the record's workspace %q", attemptID, report.Authorization, "Bearer "+wantToken)
		}
		if _, err := os.Lstat(journal.recordPath(attemptID)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("attempt %s: the record is still on disk after it was delivered: %v", attemptID, err)
		}
	}
	// requireKept delivers the record of an attempt and requires that nothing
	// went out and that the record is as it was.
	requireKept := func(attemptID, why string) {
		t.Helper()
		before := len(api.reports())
		fileBefore := budgetUsageTestFileText(t, journal, attemptID)
		d.deliverBudgetUsage(ctx, attemptID)
		if got := len(api.reports()); got != before {
			t.Fatalf("attempt %s: %d usage reports went out although %s", attemptID, got-before, why)
		}
		if got := budgetUsageTestFileText(t, journal, attemptID); got != fileBefore {
			t.Errorf("attempt %s: the record file changed although %s", attemptID, why)
		}
		if budgetUsageTestLiveAttempt(journal, attemptID) == nil {
			t.Errorf("attempt %s: the journal's process lost the record although %s", attemptID, why)
		}
	}

	batchA := seal(attemptInA, budgetUsageTestWorkspaceA)
	batchB := seal(attemptInB, budgetUsageTestWorkspaceB)
	requireSent(attemptInB, batchB, "mdt_workspace_b")
	requireSent(attemptInA, batchA, "mdt_workspace_a")

	// A workspace the installation holds no binding for.
	batchUnbound := seal(attemptUnbound, budgetUsageTestWorkspaceC)
	requireKept(attemptUnbound, "its workspace has no binding")
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceC, "mdt_workspace_c")
	requireSent(attemptUnbound, batchUnbound, "mdt_workspace_c")

	// A workspace whose credential ran out, and then got a new one.
	batchExpired := seal(attemptExpired, budgetUsageTestWorkspaceA)
	budgetUsageTestExpireBinding(t, d.client, budgetUsageTestWorkspaceA)
	requireKept(attemptExpired, "the credential of its workspace ran out")
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceA, "mdt_workspace_a_renewed")
	requireSent(attemptExpired, batchExpired, "mdt_workspace_a_renewed")

	for _, report := range api.reports() {
		if report.Authorization == installationAuth {
			t.Errorf("a report went out with the credential of the installation: %s", report.Path)
		}
		if got := budgetUsageTestCapabilityCount(report.Capabilities, protocol.DaemonCapabilityManagedInstallationV1); got != 1 {
			t.Errorf("the report to %s was not sent by a workspace client: X-Client-Capabilities = %q", report.Path, report.Capabilities)
		}
	}
	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Errorf("the journal still holds %v", names)
	}
	// A record that waits for a binding is the ordinary state of a managed
	// installation between two credentials: no warning.
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageAnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved: the
// answer of the server stands even when the daemon cannot delete the record
// file. The daemon says so once, does not send the batch a second time, and
// removes the file when the directory lets it.
func TestBudgetUsageAnsweredRecordIsNotSentAgainWhenItsFileCannotBeRemoved(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	cases := []struct {
		name   string
		answer budgetUsageTestAnswer
		// warnings is what the answer and the failed removal log together.
		warnings int
	}{
		{"delivered", budgetUsageTestRecorded(), 1},
		{"refused for good", budgetUsageTestRefusing(http.StatusBadRequest, "invalid_request"), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, tc.answer)
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
			ctx := context.Background()
			// Files of this directory can be read and cannot be removed.
			budgetUsageTestChmod(t, journal.directory, 0o500)

			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)

			if reports := api.reports(); len(reports) != 1 || !bytes.Equal(reports[0].Body, batch) {
				t.Fatalf("usage reports = %+v, want the sealed batch once", reports)
			}
			if names := budgetUsageTestNames(t, journal.directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
				t.Fatalf("the journal holds %v; the test wants a record file that could not be removed", names)
			}
			if live := budgetUsageTestLiveCount(journal); live != 0 {
				t.Errorf("the journal's process still holds %d records after the answer", live)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
				t.Errorf("%d warnings, want %d:\n%s", got, tc.warnings, logs.String())
			}

			// The next round finds the file. The batch is not sent again, and the
			// daemon does not repeat itself.
			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
			if reports := api.reports(); len(reports) != 1 {
				t.Errorf("%d usage reports, want 1: the answered batch was sent again", len(reports))
			}
			if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
				t.Errorf("%d warnings after the next round, want still %d:\n%s", got, tc.warnings, logs.String())
			}

			// The directory lets go: the file is removed, and nothing is sent.
			if err := os.Chmod(journal.directory, 0o700); err != nil {
				t.Fatal(err)
			}
			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Errorf("the journal still holds %v", names)
			}
			if reports := api.reports(); len(reports) != 1 {
				t.Errorf("%d usage reports, want 1", len(reports))
			}
			if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
				t.Errorf("%d warnings at the end, want still %d:\n%s", got, tc.warnings, logs.String())
			}
			budgetUsageTestRequireNoBodyLogged(t, logs.String(), batch)
		})
	}
}
