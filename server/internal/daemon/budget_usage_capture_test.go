package daemon

import (
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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
)

// Stage C: capturing the provider invocations of an attempt into its record.

// budgetUsageTestRequest is one request the daemon made to the test server.
type budgetUsageTestRequest struct {
	Method       string
	Path         string
	Capabilities string
	Body         string
}

// budgetUsageTestServer is a daemon API that accepts everything and records
// what it was asked.
type budgetUsageTestServer struct {
	URL string

	mu        sync.Mutex
	requests  []budgetUsageTestRequest
	onRequest func(budgetUsageTestRequest)
}

func budgetUsageTestNewServer(t *testing.T) *budgetUsageTestServer {
	t.Helper()
	return budgetUsageTestNewServerReplying(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/status") {
			_, _ = w.Write([]byte(`{"status":"running"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
}

// budgetUsageTestNewServerReplying records every request and then lets reply
// answer it. The budget usage route is answered here, and never takes a batch:
// the record of an attempt stays in the journal, where the tests that use
// this daemon API read it. What the daemon does with an answer is the subject
// of budget_usage_send_test.go.
func budgetUsageTestNewServerReplying(t *testing.T, reply http.HandlerFunc) *budgetUsageTestServer {
	t.Helper()
	s := &budgetUsageTestServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request := budgetUsageTestRequest{
			Method:       r.Method,
			Path:         r.URL.Path,
			Capabilities: r.Header.Get("X-Client-Capabilities"),
			Body:         string(body),
		}
		s.mu.Lock()
		s.requests = append(s.requests, request)
		hook := s.onRequest
		s.mu.Unlock()
		if hook != nil {
			hook(request)
		}
		if strings.HasPrefix(r.URL.Path, "/api/daemon/budget/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"budget ledger unavailable","code":"ledger_unavailable"}`))
			return
		}
		reply(w, r)
	}))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// seen returns the requests received so far, in order.
func (s *budgetUsageTestServer) seen() []budgetUsageTestRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]budgetUsageTestRequest(nil), s.requests...)
}

// budgetUsageTestBackend is an agent.Backend made of a function.
type budgetUsageTestBackend func(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error)

func (f budgetUsageTestBackend) Execute(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	return f(ctx, prompt, opts)
}

// budgetUsageTestReturning is a backend whose provider run ends at once with
// the given Result.
func budgetUsageTestReturning(result agent.Result) budgetUsageTestBackend {
	return func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
		messages := make(chan agent.Message)
		close(messages)
		results := make(chan agent.Result, 1)
		results <- result
		return &agent.Session{Messages: messages, Result: results}, nil
	}
}

// budgetUsageTestLateBackend is a backend whose provider run does not end by
// itself. Once released it offers its Result without ever parking on the
// hand-over: the Result channel is unbuffered and each offer is a
// non-blocking send, so the Result can only reach a receive that was already
// waiting when it was offered. A test that cancels the run before it releases
// the backend therefore knows which receive took the Result: never the first
// one of executeAndDrain, which the cancellation had already ended.
type budgetUsageTestLateBackend struct {
	result agent.Result

	startOnce sync.Once
	started   chan struct{} // closed when Execute was called
	release   chan struct{} // closed by the test to start offering the Result
	delivered chan struct{} // closed once the Result was taken
	stop      chan struct{} // closed when the test ends
}

func budgetUsageTestNewLateBackend(t *testing.T, result agent.Result) *budgetUsageTestLateBackend {
	t.Helper()
	b := &budgetUsageTestLateBackend{
		result:    result,
		started:   make(chan struct{}),
		release:   make(chan struct{}),
		delivered: make(chan struct{}),
		stop:      make(chan struct{}),
	}
	t.Cleanup(func() { close(b.stop) })
	return b
}

func (b *budgetUsageTestLateBackend) Execute(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
	messages := make(chan agent.Message)
	results := make(chan agent.Result)
	go func() {
		select {
		case <-b.release:
		case <-b.stop:
			return
		}
		for {
			select {
			case results <- b.result:
				close(b.delivered)
				return
			case <-b.stop:
				return
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}()
	b.startOnce.Do(func() { close(b.started) })
	return &agent.Session{Messages: messages, Result: results}, nil
}

// cancelThenRelease cancels the run once the backend was launched and only
// then lets the backend offer its Result.
func (b *budgetUsageTestLateBackend) cancelThenRelease(cancel context.CancelFunc) {
	go func() {
		select {
		case <-b.started:
		case <-b.stop:
			return
		}
		cancel()
		close(b.release)
	}()
}

// budgetUsageTestHookHandler passes the message of every log record to a
// function before it hands the record on. A test uses it to act at the exact
// point where the code under test logs something.
type budgetUsageTestHookHandler struct {
	slog.Handler
	hook func(message string)
}

func (h budgetUsageTestHookHandler) Handle(ctx context.Context, record slog.Record) error {
	h.hook(record.Message)
	return h.Handler.Handle(ctx, record)
}

func (h budgetUsageTestHookHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return budgetUsageTestHookHandler{Handler: h.Handler.WithAttrs(attrs), hook: h.hook}
}

func (h budgetUsageTestHookHandler) WithGroup(name string) slog.Handler {
	return budgetUsageTestHookHandler{Handler: h.Handler.WithGroup(name), hook: h.hook}
}

// budgetUsageTestUsage is what the fake provider runs of these tests report.
func budgetUsageTestUsage() map[string]agent.TokenUsage {
	return map[string]agent.TokenUsage{
		"model-x": {InputTokens: 11, OutputTokens: 7, CacheReadTokens: 5, CacheWriteTokens: 3, CostUSDTicks: 2},
	}
}

// budgetUsageTestCounts is budgetUsageTestUsage as a record holds it.
func budgetUsageTestCounts() map[string]budgetUsageCounts {
	return map[string]budgetUsageCounts{
		"model-x": {InputTokens: 11, OutputTokens: 7, CacheReadTokens: 5, CacheWriteTokens: 3, CostTicks: 2},
	}
}

// budgetUsageTestCaptureFixture is a daemon with a usable journal, for tests
// that run one provider invocation through the real executeAndDrain.
type budgetUsageTestCaptureFixture struct {
	daemon  *Daemon
	journal *budgetUsageJournal
	logger  *slog.Logger
	sink    *budgetUsageTestLog
	server  *budgetUsageTestServer
}

func budgetUsageTestNewCaptureFixture(t *testing.T) *budgetUsageTestCaptureFixture {
	t.Helper()
	server := budgetUsageTestNewServer(t)
	logger, sink := budgetUsageTestLogger()
	journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
	return &budgetUsageTestCaptureFixture{
		daemon:  &Daemon{client: NewClient(server.URL), logger: logger, budgetUsage: journal},
		journal: journal,
		logger:  logger,
		sink:    sink,
		server:  server,
	}
}

// budgetUsageTestCapture is what one recorded invocation handed back and what
// the record of its attempt holds afterwards.
type budgetUsageTestCapture struct {
	result  agent.Result
	tools   int32
	err     error
	attempt *budgetUsageAttempt
	record  budgetUsageRecord
}

// record runs one invocation of a new attempt through the recording wrapper,
// which calls the real executeAndDrain.
func (f *budgetUsageTestCaptureFixture) record(ctx context.Context, t *testing.T, backend agent.Backend, opts agent.ExecOptions, taskLog *slog.Logger) budgetUsageTestCapture {
	t.Helper()
	attempt, err := f.journal.begin(budgetUsageTestSeed(uuid.NewString()), f.logger)
	if err != nil {
		t.Fatalf("create the record of the attempt: %v", err)
	}
	var msgSeq atomic.Int32
	result, tools, runErr := f.daemon.executeAndDrainRecorded(withBudgetUsageAttempt(ctx, attempt), backend, "prompt", opts, taskLog, "task-1", "", &msgSeq)
	return budgetUsageTestCapture{result: result, tools: tools, err: runErr, attempt: attempt, record: attempt.snapshot()}
}

// budgetUsageTestRequireOutcome fails unless the record holds exactly one
// invocation, ended the given way with the given usage.
func budgetUsageTestRequireOutcome(t *testing.T, record budgetUsageRecord, wantEnded string, wantUsage map[string]budgetUsageCounts) {
	t.Helper()
	if len(record.Invocations) != 1 {
		t.Fatalf("record holds %d invocations, want 1: %+v", len(record.Invocations), record.Invocations)
	}
	invocation := record.Invocations[0]
	if !managedUUID.MatchString(invocation.InvocationID) {
		t.Errorf("invocation id %q is not a canonical lower-case UUID", invocation.InvocationID)
	}
	if invocation.Ended != wantEnded {
		t.Errorf("ended = %q, want %q", invocation.Ended, wantEnded)
	}
	if len(wantUsage) == 0 {
		if len(invocation.Usage) != 0 {
			t.Errorf("usage = %+v, want none", invocation.Usage)
		}
	} else if !reflect.DeepEqual(invocation.Usage, wantUsage) {
		t.Errorf("usage = %+v, want %+v", invocation.Usage, wantUsage)
	}
	if invocation.LaunchedAt.IsZero() || invocation.ObservedAt.IsZero() {
		t.Errorf("launched at %v, observed at %v: want both set", invocation.LaunchedAt, invocation.ObservedAt)
	}
}

// TestBudgetUsageEndedFollowsTheResultSource runs every way executeAndDrain
// can hand a Result back and checks what the record says about the
// invocation: how it ended, and whether it carries the usage of that Result.
//
// The made-up knowledge failure and the knowledge refusal of a launch that
// never started hand back the same Result content and end differently, as do
// a backend's own "failed" and "aborted": what decides is where the Result
// came from, which only executeAndDrain knows.
func TestBudgetUsageEndedFollowsTheResultSource(t *testing.T) {
	t.Parallel()

	usage := budgetUsageTestUsage()
	counts := budgetUsageTestCounts()
	knowledgeOptions := func() agent.ExecOptions {
		return agent.ExecOptions{Knowledge: &agent.KnowledgeLaunchIntent{}, Lifecycle: &agent.LaunchLifecycle{}}
	}
	returning := func(result agent.Result, opts agent.ExecOptions) func(*testing.T, *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
		return func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
			return f.record(context.Background(), t, budgetUsageTestReturning(result), opts, f.logger)
		}
	}
	// retagged runs a launch whose idle watchdog fires while the backend still
	// delivers its own Result, with the given status. The watchdog logs, then
	// marks itself fired and cancels. The backend hands its Result over while
	// the watchdog is still in that log call, so the Result wins over the
	// cancellation and the fired mark is set by the time the watchdog is
	// joined: executeAndDrain re-tags the backend's Result idle_watchdog.
	retagged := func(status string) func(*testing.T, *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
		return func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
			f.daemon.cfg.AgentIdleWatchdog = 100 * time.Millisecond
			messages := make(chan agent.Message)
			results := make(chan agent.Result)
			var once sync.Once
			taskLog := slog.New(budgetUsageTestHookHandler{Handler: f.logger.Handler(), hook: func(message string) {
				if !strings.HasPrefix(message, "idle watchdog firing") {
					return
				}
				once.Do(func() {
					close(messages)
					select {
					case results <- agent.Result{Status: status, Usage: usage}:
					case <-time.After(30 * time.Second):
					}
				})
			}})
			backend := budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
				return &agent.Session{Messages: messages, Result: results}, nil
			})
			return f.record(context.Background(), t, backend, agent.ExecOptions{}, taskLog)
		}
	}

	cases := []struct {
		name string
		run  func(*testing.T, *budgetUsageTestCaptureFixture) budgetUsageTestCapture
		// What executeAndDrain hands back to its caller.
		wantStatus string
		wantErr    bool
		// What the record says.
		wantEnded string
		wantUsage map[string]budgetUsageCounts
	}{
		{
			name:       "backend result completed",
			run:        returning(agent.Result{Status: "completed", Usage: usage}, agent.ExecOptions{}),
			wantStatus: "completed",
			wantEnded:  budgetUsageEndedReturned,
			wantUsage:  counts,
		},
		{
			name:       "backend result failed",
			run:        returning(agent.Result{Status: "failed", Error: "boom", Usage: usage}, agent.ExecOptions{}),
			wantStatus: "failed",
			wantEnded:  budgetUsageEndedReturned,
			wantUsage:  counts,
		},
		{
			name:       "backend result aborted",
			run:        returning(agent.Result{Status: "aborted", Usage: usage}, agent.ExecOptions{}),
			wantStatus: "aborted",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			name:       "backend result timeout",
			run:        returning(agent.Result{Status: "timeout", Usage: usage}, agent.ExecOptions{}),
			wantStatus: "timeout",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			name:       "backend result cancelled",
			run:        returning(agent.Result{Status: "cancelled", Usage: usage}, agent.ExecOptions{}),
			wantStatus: "cancelled",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			name:       "backend result without a status",
			run:        returning(agent.Result{Usage: usage}, agent.ExecOptions{}),
			wantStatus: "",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			// The watchdog stops the run and the backend reports the kill.
			name:       "backend result aborted, re-tagged idle_watchdog",
			run:        retagged("aborted"),
			wantStatus: "idle_watchdog",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			// The provider finished just as the watchdog fired. The status that
			// counts is the one handed back, so this run was cut short too.
			name:       "backend result completed, re-tagged idle_watchdog",
			run:        retagged("completed"),
			wantStatus: "idle_watchdog",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			// settleKnowledge turns the backend's own Result into "failed".
			// The status that counts is the one handed back.
			name:       "backend result settled as a knowledge failure",
			run:        returning(agent.Result{Status: "aborted", KnowledgeFailure: agent.KnowledgeContextChanged, Usage: usage}, knowledgeOptions()),
			wantStatus: "failed",
			wantEnded:  budgetUsageEndedReturned,
			wantUsage:  counts,
		},
		{
			name: "result channel closed without a value",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
					messages := make(chan agent.Message)
					close(messages)
					results := make(chan agent.Result)
					close(results)
					return &agent.Session{Messages: messages, Result: results}, nil
				})
				return f.record(context.Background(), t, backend, agent.ExecOptions{}, f.logger)
			},
			wantStatus: "",
			wantEnded:  budgetUsageEndedInterrupted,
		},
		{
			name: "made-up idle_watchdog",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				f.daemon.cfg.AgentIdleWatchdog = 50 * time.Millisecond
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "aborted", Usage: usage})
				return f.record(context.Background(), t, backend, agent.ExecOptions{}, f.logger)
			},
			wantStatus: "idle_watchdog",
			wantEnded:  budgetUsageEndedInterrupted,
		},
		{
			// The run is cancelled and the backend's Result arrives late.
			// executeAndDrain takes that Result only to learn whether the
			// process exited, drops it and hands back a Result of its own.
			name: "made-up cancelled, the backend's late result is dropped",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "aborted", Usage: usage})
				ctx, cancel := context.WithCancel(context.WithValue(context.Background(), cancelExecutionKey{}, &cancellationExecution{}))
				defer cancel()
				backend.cancelThenRelease(cancel)
				capture := f.record(ctx, t, backend, agent.ExecOptions{}, f.logger)
				select {
				case <-backend.delivered:
				case <-time.After(10 * time.Second):
					t.Error("the backend's late result was never taken, so this case did not run the path it names")
				}
				return capture
			},
			wantStatus: "cancelled",
			wantEnded:  budgetUsageEndedInterrupted,
		},
		{
			name: "made-up timeout",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "aborted", Usage: usage})
				ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
				defer cancel()
				return f.record(ctx, t, backend, agent.ExecOptions{}, f.logger)
			},
			wantStatus: "timeout",
			wantEnded:  budgetUsageEndedInterrupted,
		},
		{
			// A knowledge run is cancelled and the backend's Result arrives
			// within the wait that follows: it is the backend's own.
			name: "knowledge run cancelled, backend result aborted within the wait",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "aborted", Usage: usage})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				backend.cancelThenRelease(cancel)
				capture := f.record(ctx, t, backend, knowledgeOptions(), f.logger)
				select {
				case <-backend.delivered:
				case <-time.After(10 * time.Second):
					t.Error("the backend's result was never taken, so this case did not run the path it names")
				}
				return capture
			},
			wantStatus: "aborted",
			wantEnded:  budgetUsageEndedInterrupted,
			wantUsage:  counts,
		},
		{
			name: "knowledge run cancelled, backend result completed within the wait",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "completed", Usage: usage})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				backend.cancelThenRelease(cancel)
				capture := f.record(ctx, t, backend, knowledgeOptions(), f.logger)
				select {
				case <-backend.delivered:
				case <-time.After(10 * time.Second):
					t.Error("the backend's result was never taken, so this case did not run the path it names")
				}
				return capture
			},
			wantStatus: "completed",
			wantEnded:  budgetUsageEndedReturned,
			wantUsage:  counts,
		},
		{
			// A knowledge run is cancelled and the backend never answers:
			// after its ten second wait executeAndDrain makes a failure up.
			// The wait is fixed in executeAndDrain, so this case takes it.
			name: "knowledge run cancelled, made-up failure after the wait",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestNewLateBackend(t, agent.Result{Status: "aborted", Usage: usage})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				go func() {
					select {
					case <-backend.started:
						cancel()
					case <-backend.stop:
					}
				}()
				return f.record(ctx, t, backend, knowledgeOptions(), f.logger)
			},
			wantStatus: "failed",
			wantEnded:  budgetUsageEndedInterrupted,
		},
		{
			// The launch was refused before any process started. The Result
			// has the content of the made-up failure above; it ends differently.
			name: "backend.Execute refused the launch for a knowledge reason",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
					return nil, &agent.KnowledgeLaunchError{Reason: agent.KnowledgeUnavailable}
				})
				return f.record(context.Background(), t, backend, knowledgeOptions(), f.logger)
			},
			wantStatus: "failed",
			wantEnded:  budgetUsageEndedReturned,
		},
		{
			name: "backend.Execute returned an error",
			run: func(t *testing.T, f *budgetUsageTestCaptureFixture) budgetUsageTestCapture {
				backend := budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
					return nil, errors.New("the provider process could not be started")
				})
				return f.record(context.Background(), t, backend, agent.ExecOptions{}, f.logger)
			},
			wantStatus: "",
			wantErr:    true,
			wantEnded:  budgetUsageEndedReturned,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewCaptureFixture(t)
			capture := tc.run(t, f)

			// The wrapper hands back what executeAndDrain handed back.
			if (capture.err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want an error: %v", capture.err, tc.wantErr)
			}
			if capture.result.Status != tc.wantStatus {
				t.Fatalf("status handed back = %q, want %q", capture.result.Status, tc.wantStatus)
			}
			if len(tc.wantUsage) == 0 {
				if len(capture.result.Usage) != 0 {
					t.Errorf("usage handed back = %+v, want none", capture.result.Usage)
				}
			} else if !reflect.DeepEqual(capture.result.Usage, usage) {
				t.Errorf("usage handed back = %+v, want the backend's own %+v", capture.result.Usage, usage)
			}

			budgetUsageTestRequireOutcome(t, capture.record, tc.wantEnded, tc.wantUsage)
			stored := budgetUsageTestRequireStored(t, f.journal, capture.attempt, "after the invocation")
			budgetUsageTestRequireOutcome(t, stored, tc.wantEnded, tc.wantUsage)
		})
	}
}

// TestBudgetUsageMarksNameTheResultSource checks the marks of executeAndDrain
// themselves. The record cannot tell every source apart: a Result channel
// closed without a value yields an empty Result, which reads the same in the
// record whether it is taken for the backend's own or not.
func TestBudgetUsageMarksNameTheResultSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		backend agent.Backend
		want    budgetUsageResultSource
	}{
		{
			name:    "a value from the backend",
			backend: budgetUsageTestReturning(agent.Result{Status: "completed"}),
			want:    budgetUsageResultFromBackend,
		},
		{
			name: "result channel closed without a value",
			backend: budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
				messages := make(chan agent.Message)
				close(messages)
				results := make(chan agent.Result)
				close(results)
				return &agent.Session{Messages: messages, Result: results}, nil
			}),
			want: budgetUsageResultMadeUp,
		},
		{
			name: "backend.Execute returned an error",
			backend: budgetUsageTestBackend(func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
				return nil, errors.New("the provider process could not be started")
			}),
			want: budgetUsageResultFromFailedStart,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewCaptureFixture(t)
			origin := &budgetUsageResultOrigin{}
			var msgSeq atomic.Int32
			_, _, _ = f.daemon.executeAndDrain(withBudgetUsageResultOrigin(context.Background(), origin), tc.backend, "prompt", agent.ExecOptions{}, f.logger, "task-1", "", &msgSeq)
			if origin.source != tc.want {
				t.Fatalf("marked source = %d, want %d", origin.source, tc.want)
			}
		})
	}
}

// TestBudgetUsageInvocationOutcome is rule 3 as a table: where the Result came
// from and the status that was handed back decide how the invocation ended,
// and the source alone decides whether the usage on the Result is the
// provider's. Every Result here carries usage, so the table shows which
// sources' usage is taken and which is not, whatever the Result holds.
func TestBudgetUsageInvocationOutcome(t *testing.T) {
	usage := budgetUsageTestUsage()
	cases := []struct {
		name      string
		source    budgetUsageResultSource
		status    string
		wantEnded string
		wantUsage bool
	}{
		{"the backend's own Result, completed", budgetUsageResultFromBackend, "completed", budgetUsageEndedReturned, true},
		{"the backend's own Result, failed", budgetUsageResultFromBackend, "failed", budgetUsageEndedReturned, true},
		{"the backend's own Result, aborted", budgetUsageResultFromBackend, "aborted", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, timeout", budgetUsageResultFromBackend, "timeout", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, cancelled", budgetUsageResultFromBackend, "cancelled", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, blocked", budgetUsageResultFromBackend, "blocked", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, re-tagged idle_watchdog", budgetUsageResultFromBackend, "idle_watchdog", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, no status", budgetUsageResultFromBackend, "", budgetUsageEndedInterrupted, true},
		{"the backend's own Result, a status in another case", budgetUsageResultFromBackend, "Completed", budgetUsageEndedInterrupted, true},
		{"made up, idle_watchdog", budgetUsageResultMadeUp, "idle_watchdog", budgetUsageEndedInterrupted, false},
		{"made up, cancelled", budgetUsageResultMadeUp, "cancelled", budgetUsageEndedInterrupted, false},
		{"made up, timeout", budgetUsageResultMadeUp, "timeout", budgetUsageEndedInterrupted, false},
		{"made up, failed", budgetUsageResultMadeUp, "failed", budgetUsageEndedInterrupted, false},
		{"made up, completed", budgetUsageResultMadeUp, "completed", budgetUsageEndedInterrupted, false},
		{"made up, no status", budgetUsageResultMadeUp, "", budgetUsageEndedInterrupted, false},
		{"a launch that could not start, failed", budgetUsageResultFromFailedStart, "failed", budgetUsageEndedReturned, false},
		{"a launch that could not start, blocked", budgetUsageResultFromFailedStart, "blocked", budgetUsageEndedReturned, false},
		{"a launch that could not start, no status", budgetUsageResultFromFailedStart, "", budgetUsageEndedReturned, false},
		{"a source this code does not know", budgetUsageResultSource(99), "completed", budgetUsageEndedInterrupted, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ended, got := budgetUsageInvocationOutcome(tc.source, agent.Result{Status: tc.status, Usage: usage})
			if ended != tc.wantEnded {
				t.Errorf("ended = %q, want %q", ended, tc.wantEnded)
			}
			if tc.wantUsage && !reflect.DeepEqual(got, usage) {
				t.Errorf("usage = %+v, want the usage of the Result %+v", got, usage)
			}
			if !tc.wantUsage && got != nil {
				t.Errorf("usage = %+v, want none: this Result is not the provider's", got)
			}
		})
	}
}

// TestBudgetUsageResumedIsJudgedFromTheOptionsAsPassed pins which options
// make an invocation one that may continue earlier work. The same four inputs
// run through the real runTask in budget_usage_runtask_unix_test.go.
func TestBudgetUsageResumedIsJudgedFromTheOptionsAsPassed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts agent.ExecOptions
		want bool
	}{
		{name: "nothing carried over", opts: agent.ExecOptions{}, want: false},
		{name: "empty argument lists", opts: agent.ExecOptions{ExtraArgs: []string{}, CustomArgs: []string{}}, want: false},
		{name: "a session to resume", opts: agent.ExecOptions{ResumeSessionID: "session-1"}, want: true},
		{name: "daemon-level arguments", opts: agent.ExecOptions{ExtraArgs: []string{"--verbose"}}, want: true},
		{name: "agent custom arguments", opts: agent.ExecOptions{CustomArgs: []string{"-p"}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewCaptureFixture(t)
			capture := f.record(context.Background(), t, budgetUsageTestReturning(agent.Result{Status: "completed"}), tc.opts, f.logger)
			if len(capture.record.Invocations) != 1 {
				t.Fatalf("record holds %d invocations, want 1", len(capture.record.Invocations))
			}
			if got := capture.record.Invocations[0].Resumed; got != tc.want {
				t.Fatalf("resumed = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBudgetUsageWrapperWithoutAttemptIsExecuteAndDrain checks that a task
// without an attempt runs through the wrapper as it ran before the wrapper
// existed: the three values executeAndDrain hands back come back, the launch
// is not asked to mark where its Result came from, and nothing is recorded
// anywhere.
func TestBudgetUsageWrapperWithoutAttemptIsExecuteAndDrain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		backend budgetUsageTestBackend
		wantErr bool
	}{
		{
			name:    "a launch that returns a Result",
			backend: budgetUsageTestReturning(agent.Result{Status: "completed", Output: "done", SessionID: "session-1", Usage: budgetUsageTestUsage()}),
		},
		{
			name: "a launch that cannot start",
			backend: func(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
				return nil, errors.New("the provider process could not be started")
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewCaptureFixture(t)
			var launchCtx context.Context
			backend := budgetUsageTestBackend(func(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
				launchCtx = ctx
				return tc.backend(ctx, prompt, opts)
			})

			var directSeq atomic.Int32
			wantResult, wantTools, wantErr := f.daemon.executeAndDrain(context.Background(), backend, "prompt", agent.ExecOptions{}, f.logger, "task-1", "", &directSeq)
			if (wantErr != nil) != tc.wantErr {
				t.Fatalf("executeAndDrain error = %v, want an error: %v", wantErr, tc.wantErr)
			}
			if !tc.wantErr && (wantResult.Status != "completed" || len(wantResult.Usage) == 0) {
				t.Fatalf("executeAndDrain handed back %+v, want the backend's completed Result with its usage", wantResult)
			}

			launchCtx = nil
			var msgSeq atomic.Int32
			result, tools, err := f.daemon.executeAndDrainRecorded(context.Background(), backend, "prompt", agent.ExecOptions{}, f.logger, "task-1", "", &msgSeq)
			sameErr := (err == nil) == (wantErr == nil) && (err == nil || err.Error() == wantErr.Error())
			if !reflect.DeepEqual(result, wantResult) || tools != wantTools || !sameErr {
				t.Fatalf("wrapper handed back (%+v, %d, %v), want what executeAndDrain hands back (%+v, %d, %v)", result, tools, err, wantResult, wantTools, wantErr)
			}
			if launchCtx == nil {
				t.Fatal("the wrapper did not launch the backend")
			}
			if budgetUsageResultOriginFrom(launchCtx) != nil {
				t.Fatal("a launch without an attempt was asked to mark where its Result came from")
			}
			if names := budgetUsageTestNames(t, f.journal.directory); len(names) != 0 {
				t.Fatalf("a run without an attempt left files in the journal: %v", names)
			}
			if live := budgetUsageTestLiveCount(f.journal); live != 0 {
				t.Fatalf("a run without an attempt created %d records", live)
			}
		})
	}
}

// setOnRequest installs a function the server calls with each request, before
// it answers.
func (s *budgetUsageTestServer) setOnRequest(hook func(budgetUsageTestRequest)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRequest = hook
}

const (
	budgetUsageTestRuntimeID = "rt-budget"
	budgetUsageTestTaskID    = "0195f3a0-0000-7000-8000-0000000000f1"
)

// budgetUsageTestNewDaemon is a daemon whose handleTask can run a task with a
// test-made runner. journal may be nil: a daemon that did not declare the
// protocol.
func budgetUsageTestNewDaemon(server *budgetUsageTestServer, journal *budgetUsageJournal, logger *slog.Logger) *Daemon {
	return &Daemon{
		client:             NewClient(server.URL),
		logger:             logger,
		workspaces:         make(map[string]*workspaceState),
		runtimeIndex:       map[string]Runtime{budgetUsageTestRuntimeID: {ID: budgetUsageTestRuntimeID, Provider: "claude"}},
		cancelPollInterval: time.Hour,
		budgetUsage:        journal,
	}
}

func budgetUsageTestTask(attemptID string) Task {
	return Task{
		ID:              budgetUsageTestTaskID,
		WorkspaceID:     "ws-budget",
		RuntimeID:       budgetUsageTestRuntimeID,
		IssueID:         "issue-budget",
		AgentID:         "agent-budget",
		BudgetAttemptID: attemptID,
		Agent:           &AgentData{ID: "agent-budget", Name: "budget-agent"},
	}
}

// budgetUsageTestRunner is a task runner that launches the given backends one
// after the other through the recording wrapper, the way runTask launches a
// provider, and then reports the task completed with the given legacy usage.
func budgetUsageTestRunner(d *Daemon, legacyUsage []TaskUsageEntry, backends ...agent.Backend) taskRunnerFunc {
	return func(ctx context.Context, task Task, _ string, _ int, log *slog.Logger) (TaskResult, error) {
		var msgSeq atomic.Int32
		for _, backend := range backends {
			if _, _, err := d.executeAndDrainRecorded(ctx, backend, "prompt", agent.ExecOptions{}, log, task.ID, "", &msgSeq); err != nil {
				return TaskResult{}, err
			}
		}
		return TaskResult{Status: "completed", Comment: "done", Usage: legacyUsage}, nil
	}
}

// budgetUsageTestLiveAttempt returns the record the journal's own process
// holds for an attempt, or nil.
func budgetUsageTestLiveAttempt(journal *budgetUsageJournal, attemptID string) *budgetUsageAttempt {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	return journal.attempts[attemptID]
}

// budgetUsageTestLiveCount is the number of records the journal's own process
// holds.
func budgetUsageTestLiveCount(journal *budgetUsageJournal) int {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	return len(journal.attempts)
}

// budgetUsageTestLegacyRequests drops the requests of the budget usage route,
// which the legacy path never makes.
func budgetUsageTestLegacyRequests(requests []budgetUsageTestRequest) []budgetUsageTestRequest {
	legacy := make([]budgetUsageTestRequest, 0, len(requests))
	for _, request := range requests {
		if strings.HasPrefix(request.Path, "/api/daemon/budget/") {
			continue
		}
		legacy = append(legacy, request)
	}
	return legacy
}

// budgetUsageTestRequestsTo returns the requests whose path ends in suffix.
func budgetUsageTestRequestsTo(requests []budgetUsageTestRequest, suffix string) []budgetUsageTestRequest {
	var matching []budgetUsageTestRequest
	for _, request := range requests {
		if strings.HasSuffix(request.Path, suffix) {
			matching = append(matching, request)
		}
	}
	return matching
}

// budgetUsageTestRequireReports waits for the sends that follow a seal and
// requires that the budget usage route was called once per attempt, each time
// with the bytes of the attempt's sealed batch, and for nothing else.
func budgetUsageTestRequireReports(t *testing.T, server *budgetUsageTestServer, journal *budgetUsageJournal, attemptIDs ...string) {
	t.Helper()
	budgetUsageTestWaitForSends(t, journal)
	requests := server.seen()
	if sent := len(requests) - len(budgetUsageTestLegacyRequests(requests)); sent != len(attemptIDs) {
		t.Fatalf("%d requests went to the budget usage route, want %d: %+v", sent, len(attemptIDs), requests)
	}
	for _, attemptID := range attemptIDs {
		attempt := budgetUsageTestLiveAttempt(journal, attemptID)
		if attempt == nil {
			t.Fatalf("the journal holds no live record of attempt %s", attemptID)
		}
		reports := budgetUsageTestRequestsTo(requests, budgetUsageTestReportPath(attemptID))
		if len(reports) != 1 || reports[0].Method != http.MethodPost || reports[0].Body != string(attempt.snapshot().Batch) {
			t.Errorf("usage reports of attempt %s = %+v, want one POST with the sealed batch %s", attemptID, reports, attempt.snapshot().Batch)
		}
	}
}

// TestBudgetUsageHandleTaskRecordsAndSeals runs a task that carries a budget
// attempt through handleTask: the record is created before the run, each
// provider launch of the run becomes an invocation with its own usage, and
// the record is sealed as soon as the run is over.
func TestBudgetUsageHandleTaskRecordsAndSeals(t *testing.T) {
	server := budgetUsageTestNewServer(t)
	logger, sink := budgetUsageTestLogger()
	directory := filepath.Join(t.TempDir(), "journal")
	journal := budgetUsageTestOpenJournal(t, directory)
	// Each reading of the clock is one second after the one before, so the
	// times in the batch say in which order the record was changed.
	start := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	var readings atomic.Int64
	journal.now = func() time.Time { return start.Add(time.Duration(readings.Add(1)) * time.Second) }

	d := budgetUsageTestNewDaemon(server, journal, logger)
	// What the legacy path reports is the sum of both launches; the record
	// keeps them apart.
	legacyUsage := []TaskUsageEntry{
		{Provider: "claude", Model: "model-a", InputTokens: 11, OutputTokens: 22, CacheReadTokens: 33, CacheWriteTokens: 44, CostUSDTicks: 5},
		{Provider: "claude", Model: "model-b", OutputTokens: 6},
	}
	d.runner = budgetUsageTestRunner(d, legacyUsage,
		budgetUsageTestReturning(agent.Result{Status: "failed", Usage: map[string]agent.TokenUsage{
			"model-a": {InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4},
		}}),
		budgetUsageTestReturning(agent.Result{Status: "completed", Usage: map[string]agent.TokenUsage{
			"model-a": {InputTokens: 10, OutputTokens: 20, CacheReadTokens: 30, CacheWriteTokens: 40, CostUSDTicks: 5},
			"model-b": {OutputTokens: 6},
		}}),
	)

	// Sealed right after the run: when the first request that follows the run
	// arrives, the bytes are on disk.
	var sealedBeforeLegacyReport atomic.Bool
	server.setOnRequest(func(request budgetUsageTestRequest) {
		if !strings.HasSuffix(request.Path, "/usage") {
			return
		}
		if stored, err := journal.read(budgetUsageTestAttemptA); err == nil && len(stored.Batch) > 0 {
			sealedBeforeLegacyReport.Store(true)
		}
	})

	task := budgetUsageTestTask(budgetUsageTestAttemptA)
	d.handleTask(context.Background(), task, 0)

	if names := budgetUsageTestNames(t, directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
		t.Fatalf("journal holds %v, want the record of the attempt only", names)
	}
	attempt := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA)
	if attempt == nil {
		t.Fatal("the journal holds no live record of the attempt")
	}
	record := budgetUsageTestRequireStored(t, journal, attempt, "after the task")
	if record.AttemptID != budgetUsageTestAttemptA || record.TaskID != task.ID || record.WorkspaceID != task.WorkspaceID ||
		record.ServerURL != budgetUsageTestServerURL || record.ProducerEpoch != journal.epoch ||
		record.Adapter != "claude" || record.AdapterVersion != "" || record.NormalizerVersion != "1" {
		t.Errorf("record does not describe the attempt it was created for: %+v", record)
	}
	if !record.CreatedAt.Equal(start.Add(time.Second)) {
		t.Errorf("record created at %v, want the first reading of the clock: before the run", record.CreatedAt)
	}
	if len(record.Invocations) != 2 {
		t.Fatalf("record holds %d invocations, want one per provider launch (2)", len(record.Invocations))
	}

	wantBatch := `{"protocol_version":1,"batch_key":"` + record.BatchKey + `","producer_epoch":"` + journal.epoch +
		`","adapter":"claude","normalizer_version":"1","reported_at":"2026-11-17T08:00:06Z","invocations":[` +
		`{"invocation_id":"` + record.Invocations[0].InvocationID + `","resumed":false,"ended":"returned","observed_at":"2026-11-17T08:00:03Z",` +
		`"usage":[{"model":"model-a","tokens":{"input":"1","output":"2","cache_read":"3","cache_write":"4"}}]},` +
		`{"invocation_id":"` + record.Invocations[1].InvocationID + `","resumed":false,"ended":"returned","observed_at":"2026-11-17T08:00:05Z",` +
		`"usage":[{"model":"model-a","tokens":{"input":"10","output":"20","cache_read":"30","cache_write":"40"},"cost_ticks":"5"},` +
		`{"model":"model-b","tokens":{"input":"0","output":"6","cache_read":"0","cache_write":"0"}}]}]}`
	if got := string(record.Batch); got != wantBatch {
		t.Errorf("sealed batch\n got:  %s\n want: %s", got, wantBatch)
	}
	if !sealedBeforeLegacyReport.Load() {
		t.Error("the record was not sealed on disk when the first request after the run arrived")
	}

	requests := server.seen()
	// The legacy report still carries the usage of the whole task.
	wantLegacy := `{"usage":[{"provider":"claude","model":"model-a","input_tokens":11,"output_tokens":22,"cache_read_tokens":33,"cache_write_tokens":44,"cost_usd_ticks":5},` +
		`{"provider":"claude","model":"model-b","input_tokens":0,"output_tokens":6,"cache_read_tokens":0,"cache_write_tokens":0}]}`
	if legacy := budgetUsageTestRequestsTo(requests, "/api/daemon/tasks/"+task.ID+"/usage"); len(legacy) != 1 || legacy[0].Method != http.MethodPost || legacy[0].Body != wantLegacy {
		t.Errorf("legacy usage report = %+v, want one POST with the body %s", legacy, wantLegacy)
	}
	if completed := budgetUsageTestRequestsTo(requests, "/complete"); len(completed) != 1 {
		t.Errorf("the task was reported complete %d times, want once", len(completed))
	}
	// The seal is followed by one send of the sealed bytes.
	budgetUsageTestRequireReports(t, server, journal, budgetUsageTestAttemptA)
	// Rule 13 is about bodies; the body here is the sealed batch.
	logged := sink.String()
	for _, private := range []string{"protocol_version", record.BatchKey, `"invocations"`} {
		if strings.Contains(logged, private) {
			t.Errorf("the log carries batch content %q:\n%s", private, logged)
		}
	}
}

// TestBudgetUsageHandleTaskWithoutAttemptOrJournal runs the same task with
// and without an attempt id, on a daemon with and without a journal. Without
// both there is no record and no file, and in every combination the legacy
// requests are the ones the daemon makes today. A task that gets no record
// because the protocol does not apply to it is the ordinary case, and the
// protocol says nothing about it in the log.
func TestBudgetUsageHandleTaskWithoutAttemptOrJournal(t *testing.T) {
	type outcome struct {
		requests   []budgetUsageTestRequest
		sawAttempt bool
		journal    *budgetUsageJournal
		directory  string
		logged     string
	}
	run := func(t *testing.T, withJournal bool, attemptID string) outcome {
		t.Helper()
		server := budgetUsageTestNewServer(t)
		logger, sink := budgetUsageTestLogger()
		var out outcome
		if withJournal {
			out.directory = filepath.Join(t.TempDir(), "journal")
			out.journal = budgetUsageTestOpenJournal(t, out.directory)
		}
		d := budgetUsageTestNewDaemon(server, out.journal, logger)
		legacyUsage := []TaskUsageEntry{{Provider: "claude", Model: "model-x", InputTokens: 11, OutputTokens: 7, CacheReadTokens: 5, CacheWriteTokens: 3, CostUSDTicks: 2}}
		inner := budgetUsageTestRunner(d, legacyUsage, budgetUsageTestReturning(agent.Result{Status: "completed", Usage: budgetUsageTestUsage()}))
		d.runner = taskRunnerFunc(func(ctx context.Context, task Task, provider string, slot int, log *slog.Logger) (TaskResult, error) {
			out.sawAttempt = budgetUsageAttemptFrom(ctx) != nil
			return inner(ctx, task, provider, slot, log)
		})
		d.handleTask(context.Background(), budgetUsageTestTask(attemptID), 0)
		out.requests = server.seen()
		out.logged = sink.String()
		return out
	}

	// A task without an attempt on a daemon without a journal: the daemon as
	// it is today.
	baseline := run(t, false, "")
	if baseline.sawAttempt {
		t.Fatal("a run without a journal and without an attempt id carried a record")
	}
	if strings.Contains(baseline.logged, "budget usage") {
		t.Errorf("the daemon without the protocol logged about it:\n%s", baseline.logged)
	}
	wantPaths := []string{
		"POST /api/daemon/tasks/" + budgetUsageTestTaskID + "/usage",
		"POST /api/daemon/tasks/" + budgetUsageTestTaskID + "/progress",
		"GET /api/daemon/tasks/" + budgetUsageTestTaskID + "/status",
		"POST /api/daemon/tasks/" + budgetUsageTestTaskID + "/complete",
	}
	var gotPaths []string
	for _, request := range baseline.requests {
		gotPaths = append(gotPaths, request.Method+" "+request.Path)
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("requests of the baseline run = %v, want %v", gotPaths, wantPaths)
	}

	cases := []struct {
		name        string
		withJournal bool
		attemptID   string
		wantAttempt bool
		wantNames   []string
	}{
		{name: "attempt id, no journal", withJournal: false, attemptID: budgetUsageTestAttemptA},
		{name: "journal, no attempt id", withJournal: true, attemptID: "", wantNames: []string{}},
		{name: "journal and attempt id", withJournal: true, attemptID: budgetUsageTestAttemptA, wantAttempt: true, wantNames: []string{budgetUsageTestAttemptA + ".json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, tc.withJournal, tc.attemptID)
			if got.sawAttempt != tc.wantAttempt {
				t.Errorf("the run carried a record: %v, want %v", got.sawAttempt, tc.wantAttempt)
			}
			if legacy := budgetUsageTestLegacyRequests(got.requests); !reflect.DeepEqual(legacy, baseline.requests) {
				t.Errorf("legacy requests differ from the daemon without the protocol\n got:  %+v\n want: %+v", legacy, baseline.requests)
			}
			if !tc.wantAttempt {
				if sent := len(got.requests) - len(budgetUsageTestLegacyRequests(got.requests)); sent != 0 {
					t.Errorf("%d requests went to the budget usage route for a run without a record", sent)
				}
				if strings.Contains(got.logged, "budget usage") {
					t.Errorf("a run the protocol does not apply to logged about it:\n%s", got.logged)
				}
			}
			if tc.withJournal {
				if names := budgetUsageTestNames(t, got.directory); !reflect.DeepEqual(names, tc.wantNames) {
					t.Errorf("journal holds %v, want %v", names, tc.wantNames)
				}
				if live := budgetUsageTestLiveCount(got.journal); (live != 0) != tc.wantAttempt {
					t.Errorf("journal holds %d live records, want any: %v", live, tc.wantAttempt)
				}
			}
		})
	}
}

// TestBudgetUsageHandleTaskRunsWithoutARecordThatCannotBeCreated covers every
// reason a record is not created for a task that carries an attempt id. The
// task runs all the same, without the protocol: nothing is recorded for it
// and nothing is sealed.
func TestBudgetUsageHandleTaskRunsWithoutARecordThatCannotBeCreated(t *testing.T) {
	cases := []struct {
		name      string
		attemptID string
		// arrange puts the journal into the state the case names and returns
		// a check to run after the task.
		arrange func(t *testing.T, journal *budgetUsageJournal, logger *slog.Logger) func(t *testing.T)
		// wantLive is the number of records the journal's process holds
		// afterwards: the ones arrange created.
		wantLive int
		// wantReason is the step the warning names.
		wantReason string
		// posix cases need POSIX permission bits and a non-root user.
		posix bool
	}{
		{
			// Whether the attempt has a record cannot be told, so no second
			// one is attempted.
			name:       "the journal path is no directory any more",
			attemptID:  budgetUsageTestAttemptA,
			wantReason: "check for an existing usage report record",
			arrange: func(t *testing.T, journal *budgetUsageJournal, _ *slog.Logger) func(*testing.T) {
				if err := os.RemoveAll(journal.directory); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal.directory, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
				return func(t *testing.T) {
					if raw, err := os.ReadFile(journal.directory); err != nil || string(raw) != "not a directory" {
						t.Errorf("the unusable journal path was changed: %q, %v", raw, err)
					}
				}
			},
		},
		{
			// The attempt has no record; writing one fails.
			name:       "the journal directory lost its write permission",
			attemptID:  budgetUsageTestAttemptA,
			wantReason: "write usage report record",
			posix:      true,
			arrange: func(t *testing.T, journal *budgetUsageJournal, _ *slog.Logger) func(*testing.T) {
				if err := os.Chmod(journal.directory, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(journal.directory, 0o700) })
				return func(t *testing.T) {
					if err := os.Chmod(journal.directory, 0o700); err != nil {
						t.Fatal(err)
					}
					if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
						t.Errorf("journal holds %v, want nothing", names)
					}
				}
			},
		},
		{
			name:       "the attempt id is not a canonical UUID",
			attemptID:  strings.ToUpper(budgetUsageTestAttemptA),
			wantReason: "budget attempt id is not a canonical lower-case UUID",
			arrange: func(_ *testing.T, journal *budgetUsageJournal, _ *slog.Logger) func(*testing.T) {
				return func(t *testing.T) {
					if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
						t.Errorf("journal holds %v, want nothing", names)
					}
				}
			},
		},
		{
			// A record an earlier process left behind: the attempt keeps the
			// key it has.
			name:       "the attempt already has a record on disk",
			attemptID:  budgetUsageTestAttemptA,
			wantReason: "a usage report record for this budget attempt already exists",
			arrange: func(t *testing.T, journal *budgetUsageJournal, logger *slog.Logger) func(*testing.T) {
				earlier := budgetUsageTestOpenJournal(t, journal.directory)
				if _, err := earlier.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(journal.recordPath(budgetUsageTestAttemptA))
				if err != nil {
					t.Fatal(err)
				}
				return func(t *testing.T) {
					after, err := os.ReadFile(journal.recordPath(budgetUsageTestAttemptA))
					if err != nil || string(after) != string(before) {
						t.Errorf("the record of the earlier process was changed (%v)\n before: %s\n after:  %s", err, before, after)
					}
				}
			},
		},
		{
			// The attempt is live in this process and its file is gone. The
			// live record still holds the attempt's key.
			name:       "the attempt already has a record in this process",
			attemptID:  budgetUsageTestAttemptA,
			wantReason: "a usage report record for this budget attempt already exists",
			arrange: func(t *testing.T, journal *budgetUsageJournal, logger *slog.Logger) func(*testing.T) {
				live, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(journal.recordPath(budgetUsageTestAttemptA)); err != nil {
					t.Fatal(err)
				}
				before := budgetUsageTestRecordJSON(t, live.snapshot())
				return func(t *testing.T) {
					if got := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA); got != live {
						t.Error("the live record of the attempt was replaced")
					}
					if after := budgetUsageTestRecordJSON(t, live.snapshot()); after != before {
						t.Errorf("the live record of the attempt was changed\n before: %s\n after:  %s", before, after)
					}
					if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
						t.Errorf("journal holds %v, want nothing: no second record", names)
					}
				}
			},
			wantLive: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.posix && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("needs POSIX permission bits and a non-root user")
			}
			server := budgetUsageTestNewServer(t)
			logger, sink := budgetUsageTestLogger()
			journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
			check := tc.arrange(t, journal, logger)

			d := budgetUsageTestNewDaemon(server, journal, logger)
			ran, carriedRecord := false, false
			inner := budgetUsageTestRunner(d, nil, budgetUsageTestReturning(agent.Result{Status: "completed", Usage: budgetUsageTestUsage()}))
			d.runner = taskRunnerFunc(func(ctx context.Context, task Task, provider string, slot int, log *slog.Logger) (TaskResult, error) {
				ran = true
				carriedRecord = budgetUsageAttemptFrom(ctx) != nil
				return inner(ctx, task, provider, slot, log)
			})
			d.handleTask(context.Background(), budgetUsageTestTask(tc.attemptID), 0)

			if !ran {
				t.Fatal("the task did not run")
			}
			if carriedRecord {
				t.Error("the run carried a record that could not have been created")
			}
			requests := server.seen()
			if completed := budgetUsageTestRequestsTo(requests, "/complete"); len(completed) != 1 {
				t.Errorf("the task was reported complete %d times, want once", len(completed))
			}
			if sent := len(requests) - len(budgetUsageTestLegacyRequests(requests)); sent != 0 {
				t.Errorf("%d requests went to the budget usage route for a run without a record", sent)
			}
			if live := budgetUsageTestLiveCount(journal); live != tc.wantLive {
				t.Errorf("journal holds %d live records, want %d", live, tc.wantLive)
			}
			logged := sink.String()
			if !strings.Contains(logged, "level=WARN") || !strings.Contains(logged, "without the budget usage protocol") {
				t.Errorf("the log does not say that the task runs without the protocol:\n%s", logged)
			}
			if !strings.Contains(logged, tc.wantReason) {
				t.Errorf("the log does not name the reason %q:\n%s", tc.wantReason, logged)
			}
			// The attempt id is whatever the claim carried. The log names it
			// only when it is an id.
			if named, canonical := strings.Contains(logged, tc.attemptID), managedUUID.MatchString(tc.attemptID); named != canonical {
				t.Errorf("the log names the attempt id %q: %t, want %t:\n%s", tc.attemptID, named, canonical, logged)
			}
			check(t)
		})
	}
}

// TestBudgetUsageHandleTaskSealsFromMemoryWhenTheJournalFails breaks the
// journal after the record was created. The launch that follows cannot be
// written down; it stays in memory and is still sealed, and the task ends as
// it would have.
func TestBudgetUsageHandleTaskSealsFromMemoryWhenTheJournalFails(t *testing.T) {
	server := budgetUsageTestNewServer(t)
	logger, sink := budgetUsageTestLogger()
	directory := filepath.Join(t.TempDir(), "journal")
	journal := budgetUsageTestOpenJournal(t, directory)
	d := budgetUsageTestNewDaemon(server, journal, logger)
	inner := budgetUsageTestRunner(d, nil, budgetUsageTestReturning(agent.Result{Status: "completed", Usage: map[string]agent.TokenUsage{
		"secret-model-name": {InputTokens: 7},
	}}))
	d.runner = taskRunnerFunc(func(ctx context.Context, task Task, provider string, slot int, log *slog.Logger) (TaskResult, error) {
		if budgetUsageAttemptFrom(ctx) == nil {
			t.Error("the run carries no record although the journal was usable when it started")
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
		if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
			t.Error(err)
		}
		return inner(ctx, task, provider, slot, log)
	})
	d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)

	attempt := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA)
	if attempt == nil {
		t.Fatal("the journal holds no live record of the attempt")
	}
	record := attempt.snapshot()
	if len(record.Invocations) != 1 {
		t.Fatalf("record holds %d invocations, want the one that could not be written down", len(record.Invocations))
	}
	wantBatchEnd := `"invocations":[{"invocation_id":"` + record.Invocations[0].InvocationID + `","resumed":false,"ended":"returned","observed_at":"` +
		budgetUsageWireTime(record.Invocations[0].ObservedAt) + `","usage":[{"model":"secret-model-name","tokens":{"input":"7","output":"0","cache_read":"0","cache_write":"0"}}]}]}`
	if got := string(record.Batch); !strings.HasSuffix(got, wantBatchEnd) {
		t.Errorf("batch sealed from memory\n got:  %s\n want it to end with: %s", got, wantBatchEnd)
	}
	if completed := budgetUsageTestRequestsTo(server.seen(), "/complete"); len(completed) != 1 {
		t.Errorf("the task was reported complete %d times, want once", len(completed))
	}
	logged := sink.String()
	if got := strings.Count(logged, "could not write the usage report record"); got != 3 {
		t.Errorf("failed record writes logged %d warnings, want one per change (launch intent, outcome, sealed batch):\n%s", got, logged)
	}
	for _, private := range []string{"secret-model-name", `"tokens"`, `"invocations"`, record.BatchKey} {
		if strings.Contains(logged, private) {
			t.Errorf("the log carries record content %q:\n%s", private, logged)
		}
	}
}

// TestBudgetUsageHandleTaskSealsAnAttemptThatLaunchedNothing: a run that ends
// before any provider was launched still has a record, and its batch says
// that nothing was launched. Such a run never got to describe its launch, so
// the adapter the batch names is the one the record was created with.
func TestBudgetUsageHandleTaskSealsAnAttemptThatLaunchedNothing(t *testing.T) {
	cases := []struct {
		name        string
		prepare     func(d *Daemon)
		wantAdapter string
	}{
		{
			name:        "the provider's own executable",
			prepare:     func(*Daemon) {},
			wantAdapter: "claude",
		},
		{
			name: "the command of a custom runtime profile",
			prepare: func(d *Daemon) {
				d.runtimeIndex[budgetUsageTestRuntimeID] = Runtime{ID: budgetUsageTestRuntimeID, Provider: "claude", ProfileID: "profile-budget"}
				d.profileLaunchSpecs = map[string]profileLaunchSpec{"profile-budget": {path: "/custom/provider", version: "9.9.9"}}
			},
			wantAdapter: "custom-claude",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := budgetUsageTestNewServer(t)
			logger, _ := budgetUsageTestLogger()
			journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
			reportedAt := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
			journal.now = func() time.Time { return reportedAt }
			d := budgetUsageTestNewDaemon(server, journal, logger)
			tc.prepare(d)
			d.runner = taskRunnerFunc(func(context.Context, Task, string, int, *slog.Logger) (TaskResult, error) {
				return TaskResult{}, errors.New("the run ended before a provider was launched")
			})
			d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)

			attempt := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA)
			if attempt == nil {
				t.Fatal("the journal holds no live record of the attempt")
			}
			record := budgetUsageTestRequireStored(t, journal, attempt, "after the task")
			want := `{"protocol_version":1,"batch_key":"` + record.BatchKey + `","producer_epoch":"` + journal.epoch +
				`","adapter":"` + tc.wantAdapter + `","normalizer_version":"1","reported_at":"2026-11-17T08:00:00Z","invocations":[]}`
			if got := string(record.Batch); got != want {
				t.Errorf("sealed batch\n got:  %s\n want: %s", got, want)
			}
			if failed := budgetUsageTestRequestsTo(server.seen(), "/fail"); len(failed) != 1 {
				t.Errorf("the task was reported failed %d times, want once", len(failed))
			}
		})
	}
}

// TestBudgetUsageHandleTaskCreatesNoRecordBeforeTheRun: handleTask gives a
// claimed task up in two places before its run. The record is created
// immediately before the run, so neither of them leaves one behind: such an
// attempt has no file and reports nothing.
func TestBudgetUsageHandleTaskCreatesNoRecordBeforeTheRun(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(d *Daemon, task *Task)
	}{
		{
			name:    "the runtime of the claim is not hosted any more",
			prepare: func(_ *Daemon, task *Task) { task.RuntimeID = "rt-gone" },
		},
		{
			name: "the local directory of the task is refused",
			prepare: func(d *Daemon, task *Task) {
				d.cfg.DaemonID = "daemon-budget"
				task.ProjectResources = []ProjectResourceData{{
					ID:           "resource-budget",
					ResourceType: localDirectoryResourceType,
					ResourceRef:  json.RawMessage(`{"local_path":"/somewhere"}`),
				}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := budgetUsageTestNewServer(t)
			logger, _ := budgetUsageTestLogger()
			directory := filepath.Join(t.TempDir(), "journal")
			journal := budgetUsageTestOpenJournal(t, directory)
			d := budgetUsageTestNewDaemon(server, journal, logger)
			task := budgetUsageTestTask(budgetUsageTestAttemptA)
			tc.prepare(d, &task)
			ran := false
			d.runner = taskRunnerFunc(func(context.Context, Task, string, int, *slog.Logger) (TaskResult, error) {
				ran = true
				return TaskResult{Status: "completed"}, nil
			})
			d.handleTask(context.Background(), task, 0)

			if ran {
				t.Fatal("the task was run; this case must end before the run")
			}
			if failed := budgetUsageTestRequestsTo(server.seen(), "/fail"); len(failed) != 1 {
				t.Errorf("the task was reported failed %d times, want once", len(failed))
			}
			if names := budgetUsageTestNames(t, directory); len(names) != 0 {
				t.Errorf("journal holds %v, want no file", names)
			}
			if live := budgetUsageTestLiveCount(journal); live != 0 {
				t.Errorf("journal holds %d live records, want none", live)
			}
		})
	}
}

// TestBudgetUsageRecordIsOnDiskBeforeTheProviderStarts: a daemon that dies
// while a provider runs must leave a record that says the provider was
// launched. So when backend.Execute is entered the launch intent is on disk,
// without an outcome, and the outcome of the launch before it is on disk too.
func TestBudgetUsageRecordIsOnDiskBeforeTheProviderStarts(t *testing.T) {
	server := budgetUsageTestNewServer(t)
	logger, _ := budgetUsageTestLogger()
	journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
	d := budgetUsageTestNewDaemon(server, journal, logger)

	// What the journal holds on disk at the moment each provider is started.
	var atLaunch []budgetUsageRecord
	observing := func(result agent.Result) agent.Backend {
		provider := budgetUsageTestReturning(result)
		return budgetUsageTestBackend(func(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
			stored, err := journal.read(budgetUsageTestAttemptA)
			if err != nil {
				t.Errorf("read the record while launch %d starts: %v", len(atLaunch)+1, err)
			}
			atLaunch = append(atLaunch, stored)
			return provider(ctx, prompt, opts)
		})
	}
	d.runner = budgetUsageTestRunner(d, nil,
		observing(agent.Result{Status: "failed", Usage: budgetUsageTestUsage()}),
		observing(agent.Result{Status: "completed"}),
	)
	d.handleTask(context.Background(), budgetUsageTestTask(budgetUsageTestAttemptA), 0)

	if len(atLaunch) != 2 {
		t.Fatalf("%d providers were started, want 2", len(atLaunch))
	}
	first, second := atLaunch[0], atLaunch[1]
	if len(first.Batch) != 0 || len(second.Batch) != 0 {
		t.Errorf("the record was sealed while the run was still launching providers: %q, %q", first.Batch, second.Batch)
	}
	if len(first.Invocations) != 1 {
		t.Fatalf("on disk at the first launch: %d invocations, want the intent of this launch only", len(first.Invocations))
	}
	intent := first.Invocations[0]
	if !managedUUID.MatchString(intent.InvocationID) || intent.LaunchedAt.IsZero() {
		t.Errorf("launch intent on disk has no id or no launch time: %+v", intent)
	}
	if intent.Ended != "" || !intent.ObservedAt.IsZero() || intent.Usage != nil {
		t.Errorf("launch intent on disk already carries an outcome: %+v", intent)
	}
	if len(second.Invocations) != 2 {
		t.Fatalf("on disk at the second launch: %d invocations, want the first launch and the intent of this one", len(second.Invocations))
	}
	if second.Invocations[0].InvocationID != intent.InvocationID {
		t.Errorf("the first invocation changed its id: %q, then %q", intent.InvocationID, second.Invocations[0].InvocationID)
	}
	budgetUsageTestRequireOutcome(t, budgetUsageRecord{Invocations: second.Invocations[:1]}, budgetUsageEndedReturned, budgetUsageTestCounts())
	if next := second.Invocations[1]; next.Ended != "" || next.Usage != nil || next.InvocationID == intent.InvocationID {
		t.Errorf("second launch intent on disk: %+v, want a new id and no outcome", next)
	}

	// The key the record had on disk before the first provider started is the
	// key of the attempt for good: the sealed record and its batch carry it.
	sealed, err := journal.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatalf("read the record after the task: %v", err)
	}
	var batch struct {
		BatchKey    string `json:"batch_key"`
		Invocations []struct {
			InvocationID string `json:"invocation_id"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(sealed.Batch, &batch); err != nil {
		t.Fatalf("decode the sealed batch %q: %v", sealed.Batch, err)
	}
	if !budgetUsageBatchKeyPattern.MatchString(first.BatchKey) {
		t.Errorf("batch key on disk at the first launch %q does not match the frozen pattern", first.BatchKey)
	}
	if second.BatchKey != first.BatchKey || sealed.BatchKey != first.BatchKey || batch.BatchKey != first.BatchKey {
		t.Errorf("the batch key changed during the run: first launch %q, second launch %q, sealed record %q, batch %q",
			first.BatchKey, second.BatchKey, sealed.BatchKey, batch.BatchKey)
	}
	if len(batch.Invocations) != 2 || batch.Invocations[0].InvocationID != intent.InvocationID || batch.Invocations[1].InvocationID != second.Invocations[1].InvocationID {
		t.Errorf("the batch names the invocations %+v, want the ids that were on disk before each launch (%q, %q)",
			batch.Invocations, intent.InvocationID, second.Invocations[1].InvocationID)
	}
}
