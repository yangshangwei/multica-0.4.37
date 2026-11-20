//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Stage C through the real runTask. A test-created executable stands in for
// the provider CLI: no test here resolves or runs an agent CLI installed on
// this machine, and none of them reaches a provider.

const (
	budgetUsageTestProviderVersion = "2.1.34"
	budgetUsageTestLaunchEnd       = "--invocation-end--"
	budgetUsageTestFollowUpTaskID  = "0195f3a0-0000-7000-8000-0000000000f2"

	// What the legacy usage report of a task carries after one fresh provider
	// run, and after a refused resume followed by a fresh run.
	budgetUsageTestFreshLegacyBody = `{"usage":[{"provider":"claude","model":"model-budget","input_tokens":20,"output_tokens":9,"cache_read_tokens":4,"cache_write_tokens":2}]}`
	budgetUsageTestRetryLegacyBody = `{"usage":[{"provider":"claude","model":"model-budget","input_tokens":23,"output_tokens":10,"cache_read_tokens":4,"cache_write_tokens":2}]}`
)

// budgetUsageTestProvider is a shell script that speaks enough of claude's
// stream-json protocol for runTask to launch it as the "claude" provider.
type budgetUsageTestProvider struct {
	// path is the executable.
	path string
	// argsFile receives the arguments of every launch, one per line, each
	// launch closed by budgetUsageTestLaunchEnd.
	argsFile string
	// rejectFile, for as long as it exists, makes a launch that carries
	// --resume fail the way claude fails when the session to resume is gone.
	rejectFile string
}

func budgetUsageTestNewProvider(t *testing.T) *budgetUsageTestProvider {
	t.Helper()
	directory := t.TempDir()
	p := &budgetUsageTestProvider{
		path:       filepath.Join(directory, "claude"),
		argsFile:   filepath.Join(directory, "launches.txt"),
		rejectFile: filepath.Join(directory, "reject-resume"),
	}
	script := `#!/bin/sh
printf '%s\n' "$@" >> "` + p.argsFile + `"
printf '%s\n' '` + budgetUsageTestLaunchEnd + `' >> "` + p.argsFile + `"
resuming=0
for arg in "$@"; do
	case "$arg" in
	--version)
		printf '%s\n' '` + budgetUsageTestProviderVersion + ` (Claude Code)'
		exit 0
		;;
	--help)
		printf '%s\n' 'Usage: claude [options]'
		exit 0
		;;
	--print)
		# Model discovery: this provider has no catalog to list.
		exit 3
		;;
	--resume)
		resuming=1
		;;
	esac
done
IFS= read -r _
if [ "$resuming" = 1 ] && [ -e "` + p.rejectFile + `" ]; then
	printf '%s\n' '{"type":"assistant","message":{"role":"assistant","model":"model-budget","usage":{"input_tokens":3,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0},"content":[]}}'
	printf '%s\n' 'No conversation found with session ID: gone' >&2
	exit 1
fi
printf '%s\n' '{"type":"system","session_id":"session-budget"}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"session-budget","result":"done","modelUsage":{"model-budget":{"inputTokens":20,"outputTokens":9,"cacheReadInputTokens":4,"cacheCreationInputTokens":2}}}'
`
	if err := os.WriteFile(p.path, []byte(script), 0o755); err != nil {
		t.Fatalf("write the fake provider: %v", err)
	}
	return p
}

// rejectResume makes every later launch that carries --resume fail as a
// refused resume.
func (p *budgetUsageTestProvider) rejectResume(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(p.rejectFile, nil, 0o644); err != nil {
		t.Fatalf("arm the refused resume: %v", err)
	}
}

// launches returns the argument list of every launch of the fake provider so
// far, in order.
func (p *budgetUsageTestProvider) launches(t *testing.T) [][]string {
	t.Helper()
	raw, err := os.ReadFile(p.argsFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read the launches of the fake provider: %v", err)
	}
	var launches [][]string
	current := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if line == budgetUsageTestLaunchEnd {
			launches = append(launches, current)
			current = []string{}
			continue
		}
		current = append(current, line)
	}
	return launches
}

// budgetUsageTestCountArg counts how often a launch carries an argument.
func budgetUsageTestCountArg(args []string, arg string) int {
	count := 0
	for _, candidate := range args {
		if candidate == arg {
			count++
		}
	}
	return count
}

// taskLaunches are the launches that ran a task. The daemon starts claude for
// a task with -p; model discovery uses --print and the probes use --version
// and --help.
func (p *budgetUsageTestProvider) taskLaunches(t *testing.T) [][]string {
	t.Helper()
	var launches [][]string
	for _, launch := range p.launches(t) {
		if budgetUsageTestCountArg(launch, "-p") > 0 {
			launches = append(launches, launch)
		}
	}
	return launches
}

// budgetUsageTestRunTaskFixture is a daemon whose handleTask runs the real
// runTask against the fake provider.
type budgetUsageTestRunTaskFixture struct {
	daemon   *Daemon
	provider *budgetUsageTestProvider
	server   *budgetUsageTestServer
	// journal is nil for a daemon that did not declare the protocol.
	journal   *budgetUsageJournal
	directory string
	sink      *budgetUsageTestLog
	// beforeRun, when set, runs after handleTask created the record of the
	// attempt and before runTask starts.
	beforeRun func()
}

func budgetUsageTestNewRunTaskFixture(t *testing.T, withJournal bool) *budgetUsageTestRunTaskFixture {
	t.Helper()
	provider := budgetUsageTestNewProvider(t)
	// Like the daemon API of the other runTask tests of this package: every
	// request is accepted with an empty reply.
	server := budgetUsageTestNewServerReplying(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	logger, sink := budgetUsageTestLogger()
	f := &budgetUsageTestRunTaskFixture{provider: provider, server: server, sink: sink}
	if withJournal {
		f.directory = filepath.Join(t.TempDir(), "journal")
		f.journal = budgetUsageTestOpenJournal(t, f.directory)
	}
	f.daemon = &Daemon{
		client:             NewClient(server.URL),
		logger:             logger,
		workspaces:         make(map[string]*workspaceState),
		runtimeIndex:       map[string]Runtime{budgetUsageTestRuntimeID: {ID: budgetUsageTestRuntimeID, Provider: "claude"}},
		activeEnvRoots:     make(map[string]int),
		agentVersions:      map[string]string{"claude": budgetUsageTestProviderVersion},
		cancelPollInterval: time.Hour,
		budgetUsage:        f.journal,
		cfg: Config{
			WorkspacesRoot: t.TempDir(),
			AgentTimeout:   time.Minute,
			ServerBaseURL:  server.URL,
			Agents:         map[string]AgentEntry{"claude": {Path: provider.path}},
		},
	}
	return f
}

// handle runs one claimed task through handleTask with the real runTask and
// returns what runTask handed back.
func (f *budgetUsageTestRunTaskFixture) handle(task Task) (TaskResult, error) {
	var (
		result TaskResult
		runErr error
	)
	f.daemon.runner = taskRunnerFunc(func(ctx context.Context, task Task, provider string, slot int, log *slog.Logger) (TaskResult, error) {
		if f.beforeRun != nil {
			f.beforeRun()
		}
		result, runErr = f.daemon.runTask(ctx, task, provider, slot, log)
		return result, runErr
	})
	f.daemon.handleTask(context.Background(), task, 0)
	return result, runErr
}

// handleCompleted is handle for a task that has to run to completion.
func (f *budgetUsageTestRunTaskFixture) handleCompleted(t *testing.T, task Task) TaskResult {
	t.Helper()
	result, err := f.handle(task)
	if err != nil {
		t.Fatalf("runTask: %v\nlog:\n%s", err, f.sink.String())
	}
	if result.Status != "completed" {
		t.Fatalf("task status = %q, want completed: %+v\nlog:\n%s", result.Status, result, f.sink.String())
	}
	return result
}

// budgetUsageTestRunTaskTask is a task for the fake provider. attemptID may be
// empty: a task the server did not open a budget attempt for.
func budgetUsageTestRunTaskTask(taskID, attemptID string) Task {
	return Task{
		ID:              taskID,
		WorkspaceID:     "ws-budget",
		RuntimeID:       budgetUsageTestRuntimeID,
		IssueID:         "issue-budget",
		AgentID:         "agent-budget",
		AuthToken:       "mat_budget_usage",
		BudgetAttemptID: attemptID,
		Agent:           &AgentData{ID: "agent-budget", Name: "budget-agent"},
	}
}

// budgetUsageTestFollowUp is the task that continues the session of an
// earlier task in that task's workdir.
func budgetUsageTestFollowUp(t *testing.T, first TaskResult, attemptID string) Task {
	t.Helper()
	if first.SessionID == "" || first.WorkDir == "" {
		t.Fatalf("the first task left nothing to resume: %+v", first)
	}
	task := budgetUsageTestRunTaskTask(budgetUsageTestFollowUpTaskID, attemptID)
	task.PriorSessionID = first.SessionID
	task.PriorWorkDir = first.WorkDir
	return task
}

// budgetUsageTestRecordOnDisk reads the record of an attempt from disk as it
// is right now, sealed or not.
func budgetUsageTestRecordOnDisk(t *testing.T, journal *budgetUsageJournal, attemptID string) budgetUsageRecord {
	t.Helper()
	record, err := journal.read(attemptID)
	if err != nil {
		t.Errorf("read the record of attempt %s: %v", attemptID, err)
	}
	return record
}

// budgetUsageTestSealedBatch reads the record of an attempt from disk and
// decodes its batch the way the server does: strictly, refusing a field the
// protocol does not name.
func budgetUsageTestSealedBatch(t *testing.T, journal *budgetUsageJournal, attemptID string) (budgetUsageRecord, budgetUsageWireBatch) {
	t.Helper()
	record, err := journal.read(attemptID)
	if err != nil {
		t.Fatalf("read the record of attempt %s: %v", attemptID, err)
	}
	if len(record.Batch) == 0 {
		t.Fatalf("the record of attempt %s is not sealed: %+v", attemptID, record)
	}
	decoder := json.NewDecoder(bytes.NewReader(record.Batch))
	decoder.DisallowUnknownFields()
	var batch budgetUsageWireBatch
	if err := decoder.Decode(&batch); err != nil {
		t.Fatalf("decode the batch of attempt %s: %v\n%s", attemptID, err, record.Batch)
	}
	return record, batch
}

// budgetUsageTestWantInvocation is what a test expects of one invocation of a
// sealed batch. Ids and times are checked for their form only.
type budgetUsageTestWantInvocation struct {
	Resumed bool
	Ended   string
	Usage   []budgetUsageWireUsage
}

func budgetUsageTestRequireInvocations(t *testing.T, batch budgetUsageWireBatch, want ...budgetUsageTestWantInvocation) {
	t.Helper()
	if len(batch.Invocations) != len(want) {
		t.Fatalf("batch carries %d invocations, want %d: %+v", len(batch.Invocations), len(want), batch.Invocations)
	}
	seen := make(map[string]bool, len(want))
	for i, invocation := range batch.Invocations {
		if !managedUUID.MatchString(invocation.InvocationID) || seen[invocation.InvocationID] {
			t.Errorf("invocation %d: id %q is not a canonical UUID of its own", i, invocation.InvocationID)
		}
		seen[invocation.InvocationID] = true
		if observed, err := time.Parse(time.RFC3339, invocation.ObservedAt); err != nil || !strings.HasSuffix(invocation.ObservedAt, "Z") || observed.IsZero() {
			t.Errorf("invocation %d: observed_at %q is not a UTC time: %v", i, invocation.ObservedAt, err)
		}
		if invocation.Resumed != want[i].Resumed {
			t.Errorf("invocation %d: resumed = %t, want %t", i, invocation.Resumed, want[i].Resumed)
		}
		if invocation.Ended != want[i].Ended {
			t.Errorf("invocation %d: ended = %q, want %q", i, invocation.Ended, want[i].Ended)
		}
		if invocation.Usage == nil {
			t.Errorf("invocation %d: usage is null or missing", i)
		}
		if len(want[i].Usage) == 0 {
			if len(invocation.Usage) != 0 {
				t.Errorf("invocation %d: usage = %+v, want none", i, invocation.Usage)
			}
		} else if !reflect.DeepEqual(invocation.Usage, want[i].Usage) {
			t.Errorf("invocation %d: usage = %+v, want %+v", i, invocation.Usage, want[i].Usage)
		}
	}
}

// budgetUsageTestFreshUsage is what a completed run of the fake provider
// used, as a batch carries it.
func budgetUsageTestFreshUsage() []budgetUsageWireUsage {
	return []budgetUsageWireUsage{{
		Model:  "model-budget",
		Tokens: budgetUsageWireTokens{Input: "20", Output: "9", CacheRead: "4", CacheWrite: "2"},
	}}
}

// budgetUsageTestRejectedUsage is what a refused resume of the fake provider
// used before it failed.
func budgetUsageTestRejectedUsage() []budgetUsageWireUsage {
	return []budgetUsageWireUsage{{
		Model:  "model-budget",
		Tokens: budgetUsageWireTokens{Input: "3", Output: "1", CacheRead: "0", CacheWrite: "0"},
	}}
}

// budgetUsageTestOnlyBody returns the body of the one request whose path ends
// in suffix.
func budgetUsageTestOnlyBody(t *testing.T, server *budgetUsageTestServer, suffix string) string {
	t.Helper()
	matching := budgetUsageTestRequestsTo(server.seen(), suffix)
	if len(matching) != 1 {
		t.Fatalf("%d requests to %s, want 1: %+v", len(matching), suffix, matching)
	}
	return matching[0].Body
}

// budgetUsageTestRequireNoBudgetRequest fails when the daemon called the
// budget usage route: a run without a record reports nothing there.
func budgetUsageTestRequireNoBudgetRequest(t *testing.T, server *budgetUsageTestServer) {
	t.Helper()
	requests := server.seen()
	if legacy := budgetUsageTestLegacyRequests(requests); len(legacy) != len(requests) {
		t.Fatalf("the daemon called the budget usage route: %+v", requests)
	}
}

// budgetUsageTestArgValue returns the argument that follows flag in a launch,
// or "" when the launch does not carry the flag.
func budgetUsageTestArgValue(args []string, flag string) string {
	for i, candidate := range args {
		if candidate == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// TestBudgetUsageRunTaskResumedFollowsTheLaunchInput runs the four inputs
// that decide `resumed` through the real runTask. An invocation is resumed
// when its launch was given a session to resume, daemon-level arguments or
// the agent's custom arguments, and only then.
func TestBudgetUsageRunTaskResumedFollowsTheLaunchInput(t *testing.T) {
	t.Parallel()

	t.Run("no resume id and no extra arguments", func(t *testing.T) {
		t.Parallel()
		f := budgetUsageTestNewRunTaskFixture(t, true)
		f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA))

		_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
		budgetUsageTestRequireInvocations(t, batch,
			budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		launches := f.provider.taskLaunches(t)
		if len(launches) != 1 || budgetUsageTestCountArg(launches[0], "--resume") != 0 {
			t.Fatalf("task launches = %q, want one without --resume", launches)
		}
	})

	t.Run("daemon-level arguments", func(t *testing.T) {
		t.Parallel()
		f := budgetUsageTestNewRunTaskFixture(t, true)
		f.daemon.cfg.ClaudeArgs = []string{"--debug"}
		f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA))

		_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
		budgetUsageTestRequireInvocations(t, batch,
			budgetUsageTestWantInvocation{Resumed: true, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		launches := f.provider.taskLaunches(t)
		if len(launches) != 1 || budgetUsageTestCountArg(launches[0], "--debug") != 1 || budgetUsageTestCountArg(launches[0], "--resume") != 0 {
			t.Fatalf("task launches = %q, want one with --debug and without --resume", launches)
		}
	})

	t.Run("agent custom arguments the adapter drops", func(t *testing.T) {
		t.Parallel()
		f := budgetUsageTestNewRunTaskFixture(t, true)
		task := budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA)
		// claude's adapter refuses a custom -p, so the launch carries only its
		// own. What counts is the arguments as they were passed to the launch,
		// not what the adapter kept of them.
		task.Agent.CustomArgs = []string{"-p"}
		f.handleCompleted(t, task)

		_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
		budgetUsageTestRequireInvocations(t, batch,
			budgetUsageTestWantInvocation{Resumed: true, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		launches := f.provider.taskLaunches(t)
		if len(launches) != 1 || budgetUsageTestCountArg(launches[0], "-p") != 1 || budgetUsageTestCountArg(launches[0], "--resume") != 0 {
			t.Fatalf("task launches = %q, want one with a single -p and without --resume", launches)
		}
	})

	t.Run("resume id", func(t *testing.T) {
		t.Parallel()
		f := budgetUsageTestNewRunTaskFixture(t, true)
		first := f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA))
		f.handleCompleted(t, budgetUsageTestFollowUp(t, first, budgetUsageTestAttemptB))

		_, firstBatch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
		budgetUsageTestRequireInvocations(t, firstBatch,
			budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptB)
		budgetUsageTestRequireInvocations(t, batch,
			budgetUsageTestWantInvocation{Resumed: true, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		launches := f.provider.taskLaunches(t)
		if len(launches) != 2 || budgetUsageTestCountArg(launches[0], "--resume") != 0 || budgetUsageTestArgValue(launches[1], "--resume") != first.SessionID {
			t.Fatalf("task launches = %q, want a fresh one and one that resumes %s", launches, first.SessionID)
		}
	})
}

// TestBudgetUsageRunTaskFreshSessionRetryIsTwoInvocations covers the second
// launch site of runTask. When the provider refuses to resume, runTask
// launches it again with a fresh session. The batch names both launches,
// each with what it used itself, while the legacy usage report still carries
// their sum, exactly as on a daemon without the protocol.
func TestBudgetUsageRunTaskFreshSessionRetryIsTwoInvocations(t *testing.T) {
	t.Parallel()

	followUpUsage := "/tasks/" + budgetUsageTestFollowUpTaskID + "/usage"
	run := func(t *testing.T, withProtocol bool) *budgetUsageTestRunTaskFixture {
		t.Helper()
		f := budgetUsageTestNewRunTaskFixture(t, withProtocol)
		firstAttempt, followUpAttempt := "", ""
		if withProtocol {
			firstAttempt, followUpAttempt = budgetUsageTestAttemptA, budgetUsageTestAttemptB
		}
		first := f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, firstAttempt))
		f.provider.rejectResume(t)
		f.handleCompleted(t, budgetUsageTestFollowUp(t, first, followUpAttempt))

		launches := f.provider.taskLaunches(t)
		if len(launches) != 3 || budgetUsageTestArgValue(launches[1], "--resume") != first.SessionID || budgetUsageTestCountArg(launches[2], "--resume") != 0 {
			t.Fatalf("task launches = %q, want the first task, a refused resume of %s and a fresh retry", launches, first.SessionID)
		}
		if body := budgetUsageTestOnlyBody(t, f.server, followUpUsage); body != budgetUsageTestRetryLegacyBody {
			t.Fatalf("legacy usage report of the follow-up task = %s, want the sum of both launches %s", body, budgetUsageTestRetryLegacyBody)
		}
		if withProtocol {
			budgetUsageTestRequireReports(t, f.server, f.journal, firstAttempt, followUpAttempt)
		} else {
			budgetUsageTestRequireNoBudgetRequest(t, f.server)
		}
		return f
	}

	t.Run("with the protocol", func(t *testing.T) {
		t.Parallel()
		f := run(t, true)
		_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptB)
		budgetUsageTestRequireInvocations(t, batch,
			budgetUsageTestWantInvocation{Resumed: true, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestRejectedUsage()},
			budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()},
		)
	})

	t.Run("without the protocol", func(t *testing.T) {
		t.Parallel()
		run(t, false)
	})
}

// TestBudgetUsageRunTaskNamesTheAdapter checks what the batch says about the
// launch: the provider and the CLI version resolved for it, and for the
// command of a custom runtime profile the name custom-<provider>, which the
// provider's own evidence does not cover.
func TestBudgetUsageRunTaskNamesTheAdapter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		prepare     func(t *testing.T, f *budgetUsageTestRunTaskFixture)
		wantAdapter string
		wantVersion string
	}{
		{
			name:        "the provider's own executable",
			prepare:     func(*testing.T, *budgetUsageTestRunTaskFixture) {},
			wantAdapter: "claude",
			wantVersion: budgetUsageTestProviderVersion,
		},
		{
			name:        "the provider's own executable, version unknown",
			prepare:     func(_ *testing.T, f *budgetUsageTestRunTaskFixture) { f.daemon.agentVersions = nil },
			wantAdapter: "claude",
		},
		{
			name: "the command of a custom runtime profile",
			prepare: func(_ *testing.T, f *budgetUsageTestRunTaskFixture) {
				// A custom runtime can live on a host that has no agent of the
				// provider installed.
				f.daemon.cfg.Agents = map[string]AgentEntry{}
				f.daemon.runtimeIndex[budgetUsageTestRuntimeID] = Runtime{ID: budgetUsageTestRuntimeID, Provider: "claude", ProfileID: "profile-budget"}
				f.daemon.profileLaunchSpecs = map[string]profileLaunchSpec{"profile-budget": {path: f.provider.path, version: "9.9.9"}}
			},
			wantAdapter: "custom-claude",
			wantVersion: "9.9.9",
		},
		{
			// The record is created with what the daemon knows at the claim. The
			// batch names what the run launched.
			name: "a custom runtime profile that took the runtime over after the claim",
			prepare: func(t *testing.T, f *budgetUsageTestRunTaskFixture) {
				f.beforeRun = func() {
					if created := budgetUsageTestRecordOnDisk(t, f.journal, budgetUsageTestAttemptA); created.Adapter != "claude" || created.AdapterVersion != "" {
						t.Errorf("the record was created with adapter %q version %q, want claude and no version", created.Adapter, created.AdapterVersion)
					}
					f.daemon.mu.Lock()
					defer f.daemon.mu.Unlock()
					f.daemon.runtimeIndex[budgetUsageTestRuntimeID] = Runtime{ID: budgetUsageTestRuntimeID, Provider: "claude", ProfileID: "profile-budget"}
					f.daemon.profileLaunchSpecs = map[string]profileLaunchSpec{"profile-budget": {path: f.provider.path, version: "9.9.9"}}
				}
			},
			wantAdapter: "custom-claude",
			wantVersion: "9.9.9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewRunTaskFixture(t, true)
			tc.prepare(t, f)
			f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA))

			record, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
			if batch.Adapter != tc.wantAdapter || batch.AdapterVersion != tc.wantVersion || batch.NormalizerVersion != "1" {
				t.Fatalf("batch names adapter %q version %q normalizer %q, want %q %q \"1\"\n%s",
					batch.Adapter, batch.AdapterVersion, batch.NormalizerVersion, tc.wantAdapter, tc.wantVersion, record.Batch)
			}
			if carries := bytes.Contains(record.Batch, []byte(`"adapter_version"`)); carries != (tc.wantVersion != "") {
				t.Fatalf("batch carries adapter_version: %t, want %t\n%s", carries, tc.wantVersion != "", record.Batch)
			}
			budgetUsageTestRequireInvocations(t, batch,
				budgetUsageTestWantInvocation{Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
			if launches := f.provider.taskLaunches(t); len(launches) != 1 {
				t.Fatalf("task launches = %q, want one", launches)
			}
		})
	}
}

// TestBudgetUsageRunTaskProbesAndDiscoveryAreNotInvocations makes runTask
// start the provider's executable for more than the task: a thinking level on
// the agent sends it to the provider's model catalog first, which probes the
// version, asks for the model list and reads the help text. Only the launch
// that runs the task is an invocation.
func TestBudgetUsageRunTaskProbesAndDiscoveryAreNotInvocations(t *testing.T) {
	t.Parallel()

	f := budgetUsageTestNewRunTaskFixture(t, true)
	task := budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA)
	task.Agent.ThinkingLevel = "high"
	f.handleCompleted(t, task)

	var versionProbes, discoveries, taskRuns int
	launches := f.provider.launches(t)
	for _, launch := range launches {
		switch {
		case budgetUsageTestCountArg(launch, "--version") > 0:
			versionProbes++
		case budgetUsageTestCountArg(launch, "--print") > 0:
			discoveries++
		case budgetUsageTestCountArg(launch, "-p") > 0:
			taskRuns++
		}
	}
	if versionProbes == 0 || discoveries == 0 || taskRuns != 1 {
		t.Fatalf("the provider was started for %d version probes, %d model discoveries and %d task runs, want at least one probe, at least one discovery and exactly one task run: %q",
			versionProbes, discoveries, taskRuns, launches)
	}
	_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
	budgetUsageTestRequireInvocations(t, batch,
		budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
}

// TestBudgetUsageRunTaskLaunchThatCannotStart points the provider at an
// executable that does not exist. The launch was intended and its outcome is
// known: it returned without using anything.
func TestBudgetUsageRunTaskLaunchThatCannotStart(t *testing.T) {
	t.Parallel()

	f := budgetUsageTestNewRunTaskFixture(t, true)
	f.daemon.cfg.Agents = map[string]AgentEntry{"claude": {Path: filepath.Join(t.TempDir(), "provider-that-is-not-there")}}
	if _, err := f.handle(budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA)); err == nil {
		t.Fatalf("runTask started a provider that does not exist\nlog:\n%s", f.sink.String())
	}

	_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
	budgetUsageTestRequireInvocations(t, batch,
		budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned})
	if launches := f.provider.launches(t); len(launches) != 0 {
		t.Fatalf("the fake provider was started: %q", launches)
	}
	requests := f.server.seen()
	if failed := budgetUsageTestRequestsTo(requests, "/tasks/"+budgetUsageTestTaskID+"/fail"); len(failed) != 1 {
		t.Fatalf("%d fail reports, want 1: %+v", len(failed), requests)
	}
	if reported := budgetUsageTestRequestsTo(requests, "/tasks/"+budgetUsageTestTaskID+"/usage"); len(reported) != 0 {
		t.Fatalf("a launch that used nothing was reported on the legacy usage route: %+v", reported)
	}
	budgetUsageTestRequireReports(t, f.server, f.journal, budgetUsageTestAttemptA)
}

// TestBudgetUsageRunTaskThatLaunchesNothing runs a task for a provider this
// daemon has no executable for: runTask returns before any launch. The
// attempt still gets its batch, with an empty list of invocations.
func TestBudgetUsageRunTaskThatLaunchesNothing(t *testing.T) {
	t.Parallel()

	f := budgetUsageTestNewRunTaskFixture(t, true)
	f.daemon.cfg.Agents = map[string]AgentEntry{}
	if _, err := f.handle(budgetUsageTestRunTaskTask(budgetUsageTestTaskID, budgetUsageTestAttemptA)); err == nil || !strings.Contains(err.Error(), "no agent configured") {
		t.Fatalf("runTask error = %v, want the missing agent", err)
	}

	record, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
	budgetUsageTestRequireInvocations(t, batch)
	if !bytes.HasSuffix(record.Batch, []byte(`,"invocations":[]}`)) {
		t.Fatalf("batch does not end with an empty array of invocations:\n%s", record.Batch)
	}
	if batch.Adapter != "claude" || bytes.Contains(record.Batch, []byte(`"adapter_version"`)) {
		t.Fatalf("batch of a run that resolved no launch names adapter %q:\n%s", batch.Adapter, record.Batch)
	}
	if launches := f.provider.launches(t); len(launches) != 0 {
		t.Fatalf("the fake provider was started: %q", launches)
	}
}

// TestBudgetUsageRunTaskWithoutTheProtocol runs the same task with and
// without each half of what switches the protocol on. A record exists only
// when the daemon has a journal and the task carries an attempt id, and the
// legacy usage report is the same request in every case.
func TestBudgetUsageRunTaskWithoutTheProtocol(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		withJournal bool
		attemptID   string
		wantRecord  bool
	}{
		{name: "no journal and no attempt id"},
		{name: "attempt id, no journal", attemptID: budgetUsageTestAttemptA},
		{name: "journal, no attempt id", withJournal: true},
		{name: "journal and attempt id", withJournal: true, attemptID: budgetUsageTestAttemptA, wantRecord: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := budgetUsageTestNewRunTaskFixture(t, tc.withJournal)
			f.handleCompleted(t, budgetUsageTestRunTaskTask(budgetUsageTestTaskID, tc.attemptID))

			if body := budgetUsageTestOnlyBody(t, f.server, "/tasks/"+budgetUsageTestTaskID+"/usage"); body != budgetUsageTestFreshLegacyBody {
				t.Fatalf("legacy usage report = %s, want %s", body, budgetUsageTestFreshLegacyBody)
			}
			requests := f.server.seen()
			if completed := budgetUsageTestRequestsTo(requests, "/tasks/"+budgetUsageTestTaskID+"/complete"); len(completed) != 1 {
				t.Fatalf("%d completion reports, want 1: %+v", len(completed), requests)
			}
			if tc.wantRecord {
				budgetUsageTestRequireReports(t, f.server, f.journal, budgetUsageTestAttemptA)
			} else {
				budgetUsageTestRequireNoBudgetRequest(t, f.server)
			}
			if launches := f.provider.taskLaunches(t); len(launches) != 1 {
				t.Fatalf("task launches = %q, want one", launches)
			}
			if !tc.withJournal {
				return
			}
			names := budgetUsageTestNames(t, f.directory)
			live := budgetUsageTestLiveCount(f.journal)
			if !tc.wantRecord {
				if len(names) != 0 || live != 0 {
					t.Fatalf("a task without an attempt id left %q in the journal and %d live records", names, live)
				}
				return
			}
			if !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) || live != 1 {
				t.Fatalf("journal holds %q and %d live records, want the record of the attempt", names, live)
			}
			_, batch := budgetUsageTestSealedBatch(t, f.journal, budgetUsageTestAttemptA)
			budgetUsageTestRequireInvocations(t, batch,
				budgetUsageTestWantInvocation{Resumed: false, Ended: budgetUsageEndedReturned, Usage: budgetUsageTestFreshUsage()})
		})
	}
}
