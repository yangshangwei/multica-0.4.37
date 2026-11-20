package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The claim endpoints of the budget step: what ClaimTaskByRuntime and
// ClaimTasksByRuntime tell the service about the claiming daemon, and what
// they answer with what the service hands back. What the budget step writes
// for a claim is the service's and is tested there
// (internal/service/issue_budget_finalize_test.go); these tests read the
// attempt row to see which protocol an endpoint passed on and which attempt it
// named, and count rows to see that a claim it turned away left none.

const (
	claimBudgetDaemonID = "claim-budget-daemon"

	// The wire value of the capability, spelled out: a daemon in the field
	// sends these bytes, whatever the constant is renamed to.
	claimBudgetCapability = "budget-usage-v1"
)

// claimBudgetCapabilities is the X-Client-Capabilities header of a claim. The
// zero value sends no header at all.
type claimBudgetCapabilities struct {
	sent  bool
	value string
}

func claimBudgetHeader(value string) claimBudgetCapabilities {
	return claimBudgetCapabilities{sent: true, value: value}
}

// claimBudgetScene is a workspace of its own with one runtime. The binding
// lock is per workspace and the budget tables have no foreign keys, so a
// workspace per test keeps a held lock and the "nothing was written" counts
// away from every other test of the package.
type claimBudgetScene struct {
	fx        *testutil.Fixture
	workspace string
	runtime   string
}

func newClaimBudgetScene(t *testing.T) *claimBudgetScene {
	t.Helper()
	workspace := dbfx.Workspace(t, "Claim budget", "claim-budget-"+uuid.NewString())
	fx := testutil.New(testPool, workspace, testUserID)
	// Nothing cascades into the budget tables: remove what a root and its
	// claims leave in this workspace.
	for _, table := range []string{
		"issue_budget_observation", "issue_budget_operation", "issue_budget_entry",
		"issue_budget_attempt", "issue_budget_member", "issue_budget_root",
	} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE workspace_id = $1", workspace)
	}
	fx.Member(t, workspace, testUserID, "owner")
	return &claimBudgetScene{fx: fx, workspace: workspace, runtime: fx.Runtime(t, "Claim budget runtime")}
}

// claimBudgetTask is one queued task of an issue of the scene.
type claimBudgetTask struct {
	id    string
	issue string
	agent string
}

// queueTask queues a task of a new issue for a new agent of the scene's
// runtime. Every task gets an agent of its own: a runtime hands out one task
// per agent at a time.
func (s *claimBudgetScene) queueTask(t *testing.T, name string, issue ...testutil.Cols) claimBudgetTask {
	t.Helper()
	agent := s.fx.Agent(t, name+" agent", s.runtime)
	issueID := s.fx.Issue(t, name+" issue", append([]testutil.Cols{{"status": "in_progress"}}, issue...)...)
	task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": s.runtime, "issue_id": issueID})
	return claimBudgetTask{id: task, issue: issueID, agent: agent}
}

// enableRoot makes issueID the root issue of an observing budget root, the
// way the product does it. Issues below it at that moment become members.
func (s *claimBudgetScene) enableRoot(t *testing.T, issueID string) service.IssueBudgetOperationResult {
	t.Helper()
	result, err := service.NewIssueBudgetService(db.New(testPool), testPool).Enable(context.Background(), service.IssueBudgetEnableParams{
		WorkspaceID:  parseUUID(s.workspace),
		IssueID:      parseUUID(issueID),
		OperationKey: "enable-" + issueID,
		Reason:       "claim endpoint test",
		ActorUserID:  parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("setup: enable a budget root on issue %s: %v", issueID, err)
	}
	return result
}

// claimSingle claims through the single-runtime endpoint as a daemon of the
// scene's workspace.
func (s *claimBudgetScene) claimSingle(t *testing.T, capabilities claimBudgetCapabilities) *testutil.Response {
	t.Helper()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+s.runtime+"/tasks/claim", nil, s.workspace, claimBudgetDaemonID)
	req = testutil.WithURLParams(req, "runtimeId", s.runtime)
	if capabilities.sent {
		req.Header.Set("X-Client-Capabilities", capabilities.value)
	}
	return testutil.Call(t, testHandler.ClaimTaskByRuntime, req)
}

// claimBatch claims through the batch endpoint for the scene's runtime.
func (s *claimBudgetScene) claimBatch(t *testing.T, capabilities claimBudgetCapabilities) *testutil.Response {
	t.Helper()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/claim", s.batchBody(), s.workspace, claimBudgetDaemonID)
	if capabilities.sent {
		req.Header.Set("X-Client-Capabilities", capabilities.value)
	}
	return testutil.Call(t, testHandler.ClaimTasksByRuntime, req)
}

func (s *claimBudgetScene) batchBody() map[string]any {
	return map[string]any{"daemon_id": claimBudgetDaemonID, "runtime_ids": []string{s.runtime}, "max_tasks": 5}
}

// claimBudgetPayload is a claimed task as the daemon receives it, kept as raw
// fields so a test can tell an absent key from an empty value.
type claimBudgetPayload map[string]json.RawMessage

// text returns the string value of key, or "" when the payload has no such
// key.
func (p claimBudgetPayload) text(t *testing.T, key string) string {
	t.Helper()
	raw, ok := p[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("claim payload: %s is not a string: %s", key, raw)
	}
	return value
}

type claimBudgetSingleResponse struct {
	Task claimBudgetPayload `json:"task"`
}

type claimBudgetBatchResponse struct {
	Tasks []claimBudgetPayload `json:"tasks"`
}

// payload returns the item of the batch that carries taskID.
func (r claimBudgetBatchResponse) payload(t *testing.T, taskID string) claimBudgetPayload {
	t.Helper()
	for _, item := range r.Tasks {
		if item.text(t, "id") == taskID {
			return item
		}
	}
	t.Fatalf("the batch of %d tasks does not carry task %s", len(r.Tasks), taskID)
	return nil
}

// taskClaim reads the status and the claim generation of a task row.
func (s *claimBudgetScene) taskClaim(t *testing.T, taskID string) (status string, generation int64) {
	t.Helper()
	s.fx.QueryRow(t, `SELECT status, claim_generation FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status, &generation)
	return status, generation
}

// taskTokens counts the task tokens minted for a task.
func (s *claimBudgetScene) taskTokens(t *testing.T, taskID string) int {
	t.Helper()
	return s.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id = $1`, taskID)
}

// claimBudgetAttempt is what these tests read off an attempt row.
type claimBudgetAttempt struct {
	ID              string
	ProtocolVersion *int32
}

// protocol is the stored protocol version, "NULL" when there is none.
func (a claimBudgetAttempt) protocol() string {
	if a.ProtocolVersion == nil {
		return "NULL"
	}
	return strconv.Itoa(int(*a.ProtocolVersion))
}

// attempt reads the attempt row of one claim of a task, and fails the test
// unless that claim has exactly one.
func (s *claimBudgetScene) attempt(t *testing.T, taskID string, generation int64) claimBudgetAttempt {
	t.Helper()
	if n := s.fx.Count(t, `SELECT count(*) FROM issue_budget_attempt WHERE workspace_id = $1 AND task_id = $2 AND claim_generation = $3`,
		s.workspace, taskID, generation); n != 1 {
		t.Fatalf("task %s has %d attempt rows for claim generation %d, want 1", taskID, n, generation)
	}
	var attempt claimBudgetAttempt
	s.fx.QueryRow(t, `SELECT id::text, protocol_version FROM issue_budget_attempt WHERE workspace_id = $1 AND task_id = $2 AND claim_generation = $3`,
		s.workspace, taskID, generation).Scan(&attempt.ID, &attempt.ProtocolVersion)
	return attempt
}

// attempts counts the attempt rows of a task over all its claims.
func (s *claimBudgetScene) attempts(t *testing.T, taskID string) int {
	t.Helper()
	return s.fx.Count(t, `SELECT count(*) FROM issue_budget_attempt WHERE task_id = $1`, taskID)
}

// legacyProtocolGaps counts the observations of the workspace that give
// legacy_protocol as their reason, and those among them that are a coverage
// gap naming one claim: its attempt, its task and its claim generation.
func (s *claimBudgetScene) legacyProtocolGaps(t *testing.T, attemptID, taskID string, generation int64) (all, naming int) {
	t.Helper()
	all = s.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE workspace_id = $1 AND reason = 'legacy_protocol'`, s.workspace)
	naming = s.fx.Count(t, `
		SELECT count(*) FROM issue_budget_observation
		WHERE workspace_id = $1 AND kind = 'coverage_gap' AND reason = 'legacy_protocol'
		  AND attempt_id = $2 AND task_id = $3 AND claim_generation = $4`,
		s.workspace, attemptID, taskID, generation)
	return all, naming
}

// claimBudgetRows counts the rows of each budget table in a workspace.
type claimBudgetRows struct {
	Roots, Members, Attempts, Entries, Operations, Observations int
}

func (s *claimBudgetScene) budgetRows(t *testing.T) claimBudgetRows {
	t.Helper()
	count := func(table string) int {
		return s.fx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id = $1", s.workspace)
	}
	return claimBudgetRows{
		Roots:        count("issue_budget_root"),
		Members:      count("issue_budget_member"),
		Attempts:     count("issue_budget_attempt"),
		Entries:      count("issue_budget_entry"),
		Operations:   count("issue_budget_operation"),
		Observations: count("issue_budget_observation"),
	}
}

// bindingLockHolders counts the sessions that hold the exclusive side of the
// workspace's binding lock, as pg_locks shows them. The key is the one the
// LockIssueBudgetBinding query derives; a 64-bit advisory key is split over
// classid and objid.
func (s *claimBudgetScene) bindingLockHolders(t *testing.T) int {
	t.Helper()
	return s.fx.Count(t, `
		SELECT count(*) FROM pg_locks l
		WHERE l.locktype = 'advisory'
		  AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())
		  AND l.mode = 'ExclusiveLock' AND l.granted
		  AND l.objsubid = 1
		  AND ((l.classid::bigint << 32) | l.objid::bigint)
		      = hashtextextended('issue_budget_binding:' || $1::uuid::text, 0)`, s.workspace)
}

// holdBindingLock takes the binder's side of the workspace binding lock in a
// transaction on a connection of its own, and returns the function that ends
// that transaction. The lock is read back from pg_locks on both sides, so a
// claim made in between meets a held lock and a claim made after release
// meets none.
func (s *claimBudgetScene) holdBindingLock(t *testing.T) (release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("setup: begin the binder's transaction: %v", err)
	}
	held := true
	t.Cleanup(func() {
		if held {
			_ = tx.Rollback(ctx)
		}
	})
	if err := db.New(tx).LockIssueBudgetBinding(ctx, parseUUID(s.workspace)); err != nil {
		t.Fatalf("setup: take the binding lock: %v", err)
	}
	if n := s.bindingLockHolders(t); n != 1 {
		t.Fatalf("setup: the binding lock has %d exclusive holders, want 1", n)
	}
	return func() {
		t.Helper()
		held = false
		if err := tx.Rollback(ctx); err != nil {
			t.Fatalf("end the binder's transaction: %v", err)
		}
		if n := s.bindingLockHolders(t); n != 0 {
			t.Fatalf("the binding lock has %d exclusive holders after the binder's transaction ended, want 0", n)
		}
	}
}

// claimBudgetLog collects what the process logs through slog, one record per
// line. Writes are serialized: goroutines other tests left behind may log
// while a test reads.
type claimBudgetLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *claimBudgetLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// about returns the records that contain needle.
func (l *claimBudgetLog) about(needle string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var records []string
	for _, line := range strings.Split(l.buf.String(), "\n") {
		if strings.Contains(line, needle) {
			records = append(records, line)
		}
	}
	return records
}

// captureClaimBudgetLog routes the default logger into a buffer until the
// test ends. slog.SetDefault also redirects the log package, so its writer
// and flags are put back with the logger.
func captureClaimBudgetLog(t *testing.T) *claimBudgetLog {
	t.Helper()
	captured := &claimBudgetLog{}
	logger, writer, flags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.NewTextHandler(captured, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(logger)
		log.SetOutput(writer)
		log.SetFlags(flags)
	})
	return captured
}

// claimBudgetRedis answers the commands the empty-claim cache sends from a
// map, so the cache can run in this suite, which has no Redis server. It is a
// go-redis hook that never calls the next one, so no command reaches a
// connection. Expiry is not modelled: a verdict lives for minutes and a test
// for milliseconds.
type claimBudgetRedis struct {
	mu      sync.Mutex
	values  map[string]string
	unknown []string
}

func (f *claimBudgetRedis) DialHook(next redis.DialHook) redis.DialHook { return next }

func (f *claimBudgetRedis) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error { return f.answer(cmd) }
}

func (f *claimBudgetRedis) ProcessPipelineHook(redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(_ context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if err := f.answer(cmd); err != nil {
				return err
			}
		}
		return nil
	}
}

func (f *claimBudgetRedis) answer(cmd redis.Cmder) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	args := cmd.Args()
	key := func(i int) string {
		if i >= len(args) {
			return ""
		}
		return fmt.Sprint(args[i])
	}
	switch c := cmd.(type) {
	case *redis.StringCmd:
		if cmd.Name() == "get" {
			value, ok := f.values[key(1)]
			if !ok {
				c.SetErr(redis.Nil)
				return redis.Nil
			}
			c.SetVal(value)
			return nil
		}
	case *redis.SliceCmd:
		if cmd.Name() == "mget" {
			values := make([]any, 0, len(args)-1)
			for i := 1; i < len(args); i++ {
				if value, ok := f.values[key(i)]; ok {
					values = append(values, value)
				} else {
					values = append(values, nil)
				}
			}
			c.SetVal(values)
			return nil
		}
	case *redis.StatusCmd:
		if cmd.Name() == "set" {
			f.values[key(1)] = key(2)
			c.SetVal("OK")
			return nil
		}
	case *redis.IntCmd:
		if cmd.Name() == "incr" {
			n, _ := strconv.ParseInt(f.values[key(1)], 10, 64)
			f.values[key(1)] = strconv.FormatInt(n+1, 10)
			c.SetVal(n + 1)
			return nil
		}
	case *redis.BoolCmd:
		if cmd.Name() == "expire" {
			_, ok := f.values[key(1)]
			c.SetVal(ok)
			return nil
		}
	}
	// The cache swallows a failed command and goes to the database, which
	// would let a test pass without the cache: keep what was not answered.
	f.unknown = append(f.unknown, fmt.Sprint(args...))
	err := fmt.Errorf("claim budget test redis: no answer for %v", args)
	cmd.SetErr(err)
	return err
}

// unanswered returns the commands the fake had no answer for.
func (f *claimBudgetRedis) unanswered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.unknown...)
}

// installClaimBudgetEmptyClaimCache gives the handler's task service the
// empty-claim cache a deployment with Redis has, until the test ends. The
// suite runs without one.
func installClaimBudgetEmptyClaimCache(t *testing.T) (*service.EmptyClaimCache, *claimBudgetRedis) {
	t.Helper()
	fake := &claimBudgetRedis{values: map[string]string{}}
	client := redis.NewClient(&redis.Options{
		Addr: "claim-budget-test.invalid:6379",
		Dialer: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("claim budget test redis has no server")
		},
	})
	client.AddHook(fake)
	cache := service.NewEmptyClaimCache(client)
	previous := testHandler.TaskService.EmptyClaim
	testHandler.TaskService.EmptyClaim = cache
	t.Cleanup(func() {
		testHandler.TaskService.EmptyClaim = previous
		_ = client.Close()
	})
	return cache, fake
}

// TestClaimBudgetSingleRuntimeDeclaredProtocol is H01: a daemon that declares
// the budget usage capability claims a task of a member issue through the
// single-runtime endpoint. The payload names the attempt the claim opened, and
// the attempt records protocol version 1.
func TestClaimBudgetSingleRuntimeDeclaredProtocol(t *testing.T) {
	scene := newClaimBudgetScene(t)
	task := scene.queueTask(t, "declared")
	scene.enableRoot(t, task.issue)

	var out claimBudgetSingleResponse
	scene.claimSingle(t, claimBudgetHeader(claimBudgetCapability)).Want(http.StatusOK).JSON(&out)
	if got := out.Task.text(t, "id"); got != task.id {
		t.Fatalf("claimed task = %q, want the queued task %s", got, task.id)
	}

	status, generation := scene.taskClaim(t, task.id)
	if status != "dispatched" {
		t.Fatalf("task status = %s, want dispatched", status)
	}
	attempt := scene.attempt(t, task.id, generation)
	if got := out.Task.text(t, "budget_attempt_id"); got != attempt.ID {
		t.Fatalf("payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
	}
	if n := scene.attempts(t, task.id); n != 1 {
		t.Fatalf("the task has %d attempt rows after one claim, want 1", n)
	}
	if got := attempt.protocol(); got != "1" {
		t.Fatalf("attempt protocol_version = %s, want 1", got)
	}
	if all, _ := scene.legacyProtocolGaps(t, attempt.ID, task.id, generation); all != 0 {
		t.Fatalf("a daemon that declared the protocol left %d legacy_protocol observations, want 0", all)
	}
}

// TestClaimBudgetSingleRuntimeUndeclaredProtocol is H02: the same claim by a
// daemon that sends no capability header. It still gets the attempt id; the
// attempt records no protocol version and one coverage gap says why.
func TestClaimBudgetSingleRuntimeUndeclaredProtocol(t *testing.T) {
	scene := newClaimBudgetScene(t)
	task := scene.queueTask(t, "undeclared")
	scene.enableRoot(t, task.issue)

	var out claimBudgetSingleResponse
	scene.claimSingle(t, claimBudgetCapabilities{}).Want(http.StatusOK).JSON(&out)
	if got := out.Task.text(t, "id"); got != task.id {
		t.Fatalf("claimed task = %q, want the queued task %s", got, task.id)
	}

	_, generation := scene.taskClaim(t, task.id)
	attempt := scene.attempt(t, task.id, generation)
	if got := out.Task.text(t, "budget_attempt_id"); got != attempt.ID {
		t.Fatalf("payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
	}
	if got := attempt.protocol(); got != "NULL" {
		t.Fatalf("attempt protocol_version = %s, want NULL for a daemon that declared no protocol", got)
	}
	all, naming := scene.legacyProtocolGaps(t, attempt.ID, task.id, generation)
	if all != 1 || naming != 1 {
		t.Fatalf("legacy_protocol observations = %d, of which %d are a coverage gap naming attempt %s, task %s and claim generation %d; want 1 and 1",
			all, naming, attempt.ID, task.id, generation)
	}
}

// TestClaimBudgetBatch is H03: the batch endpoint hands out two tasks of
// member issues in one batch, the root issue's and a child issue's. Each item
// names its own attempt, and every attempt records what the daemon declared.
func TestClaimBudgetBatch(t *testing.T) {
	cases := []struct {
		name         string
		capabilities claimBudgetCapabilities
		protocol     string
		gapsPerClaim int
	}{
		{"declared", claimBudgetHeader(claimBudgetCapability), "1", 0},
		{"not declared", claimBudgetCapabilities{}, "NULL", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			root := scene.queueTask(t, "root")
			child := scene.queueTask(t, "child", testutil.Cols{"parent_issue_id": root.issue})
			scene.enableRoot(t, root.issue)

			var out claimBudgetBatchResponse
			scene.claimBatch(t, tc.capabilities).Want(http.StatusOK).JSON(&out)
			if len(out.Tasks) != 2 {
				t.Fatalf("the batch carries %d tasks, want the 2 queued ones", len(out.Tasks))
			}

			attemptIDs := map[string]string{}
			for _, task := range []claimBudgetTask{root, child} {
				payload := out.payload(t, task.id)
				status, generation := scene.taskClaim(t, task.id)
				if status != "dispatched" {
					t.Fatalf("task %s status = %s, want dispatched", task.id, status)
				}
				attempt := scene.attempt(t, task.id, generation)
				if got := payload.text(t, "budget_attempt_id"); got != attempt.ID {
					t.Fatalf("task %s: payload budget_attempt_id = %q, want its attempt row %s", task.id, got, attempt.ID)
				}
				if got := attempt.protocol(); got != tc.protocol {
					t.Fatalf("task %s: attempt protocol_version = %s, want %s", task.id, got, tc.protocol)
				}
				all, naming := scene.legacyProtocolGaps(t, attempt.ID, task.id, generation)
				if all != 2*tc.gapsPerClaim || naming != tc.gapsPerClaim {
					t.Fatalf("task %s: legacy_protocol observations = %d, of which %d are a coverage gap naming this claim; want %d and %d",
						task.id, all, naming, 2*tc.gapsPerClaim, tc.gapsPerClaim)
				}
				attemptIDs[task.id] = attempt.ID
			}
			if attemptIDs[root.id] == attemptIDs[child.id] {
				t.Fatalf("both items of the batch name attempt %s; each claim has its own", attemptIDs[root.id])
			}
		})
	}
}

// TestClaimBudgetOutsideEveryRoot is H04: the workspace has a root, and the
// claimed task's issue is not under it. The payload has no budget_attempt_id
// key at all, and the claim wrote no budget row.
func TestClaimBudgetOutsideEveryRoot(t *testing.T) {
	cases := []struct {
		name         string
		batch        bool
		capabilities claimBudgetCapabilities
	}{
		{"single runtime, declared", false, claimBudgetHeader(claimBudgetCapability)},
		{"single runtime, not declared", false, claimBudgetCapabilities{}},
		{"batch, declared", true, claimBudgetHeader(claimBudgetCapability)},
		{"batch, not declared", true, claimBudgetCapabilities{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			scene.enableRoot(t, scene.fx.Issue(t, "root elsewhere"))
			task := scene.queueTask(t, "outside")
			before := scene.budgetRows(t)

			var payload claimBudgetPayload
			if tc.batch {
				var out claimBudgetBatchResponse
				scene.claimBatch(t, tc.capabilities).Want(http.StatusOK).JSON(&out)
				payload = out.payload(t, task.id)
			} else {
				var out claimBudgetSingleResponse
				scene.claimSingle(t, tc.capabilities).Want(http.StatusOK).JSON(&out)
				payload = out.Task
			}
			if got := payload.text(t, "id"); got != task.id {
				t.Fatalf("claimed task = %q, want the queued task %s", got, task.id)
			}
			if raw, ok := payload["budget_attempt_id"]; ok {
				t.Fatalf("the payload of a task outside every root has budget_attempt_id %s; the key must be absent", raw)
			}
			if after := scene.budgetRows(t); after != before {
				t.Fatalf("budget rows of the workspace = %+v after the claim, want them unchanged at %+v", after, before)
			}
		})
	}
}

// TestClaimBudgetCapabilityHeader is H05: the capability is declared by one
// item of the comma-separated header that equals its name once the spaces
// around it are trimmed, and by nothing else. Read off the attempt row.
func TestClaimBudgetCapabilityHeader(t *testing.T) {
	cases := []struct {
		name     string
		header   string
		protocol string
	}{
		{"among others, with spaces", "a, budget-usage-v1 ,b", "1"},
		{"a longer name", "budget-usage-v10", "NULL"},
		{"a shorter name", "budget-usage", "NULL"},
		{"other letter case", "Budget-Usage-V1", "NULL"},
		{"an empty header", "", "NULL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			task := scene.queueTask(t, "header")
			scene.enableRoot(t, task.issue)

			var out claimBudgetSingleResponse
			scene.claimSingle(t, claimBudgetHeader(tc.header)).Want(http.StatusOK).JSON(&out)
			if got := out.Task.text(t, "id"); got != task.id {
				t.Fatalf("claimed task = %q, want the queued task %s", got, task.id)
			}
			_, generation := scene.taskClaim(t, task.id)
			if got := scene.attempt(t, task.id, generation).protocol(); got != tc.protocol {
				t.Fatalf("X-Client-Capabilities %q: attempt protocol_version = %s, want %s", tc.header, got, tc.protocol)
			}
		})
	}
}

// TestClaimBudgetSingleRuntimeBindingInProgress is H06: a binder holds the
// workspace's binding lock while a daemon claims through the single-runtime
// endpoint. The daemon is told there is nothing for it, in the bytes it gets
// when the runtime has no task; the task is back in the queue and the claim
// left nothing. Once the binder is done the next claim hands that task out.
func TestClaimBudgetSingleRuntimeBindingInProgress(t *testing.T) {
	scene := newClaimBudgetScene(t)
	declared := claimBudgetHeader(claimBudgetCapability)

	// What the endpoint answers a runtime that has no task.
	nothing := scene.claimSingle(t, declared).Want(http.StatusOK)

	task := scene.queueTask(t, "bound")
	scene.enableRoot(t, task.issue)
	before := scene.budgetRows(t)
	_, queuedGeneration := scene.taskClaim(t, task.id)

	release := scene.holdBindingLock(t)
	refused := scene.claimSingle(t, declared).Want(http.StatusOK)
	if got := strings.TrimSuffix(refused.Text(), "\n"); got != `{"task":null}` {
		t.Fatalf("answer while a binding is in progress = %s, want {\"task\":null}", got)
	}
	if !bytes.Equal(refused.Body.Bytes(), nothing.Body.Bytes()) {
		t.Fatalf("answer while a binding is in progress = %q, want the bytes of the answer for a runtime without a task, %q",
			refused.Text(), nothing.Text())
	}

	// The claim was made and handed back: the row is queued again, one claim
	// generation on. Without the generation this would also pass for a task
	// the runtime never saw.
	status, refusedGeneration := scene.taskClaim(t, task.id)
	if status != "queued" || refusedGeneration != queuedGeneration+1 {
		t.Fatalf("task after the refused claim: status %s, claim generation %d; want queued and %d", status, refusedGeneration, queuedGeneration+1)
	}
	if after := scene.budgetRows(t); after != before {
		t.Fatalf("budget rows of the workspace = %+v after the refused claim, want them unchanged at %+v", after, before)
	}
	if n := scene.taskTokens(t, task.id); n != 0 {
		t.Fatalf("the refused claim left %d task tokens, want 0", n)
	}

	release()

	var out claimBudgetSingleResponse
	scene.claimSingle(t, declared).Want(http.StatusOK).JSON(&out)
	if got := out.Task.text(t, "id"); got != task.id {
		t.Fatalf("claim after the binding ended returned task %q, want the requeued task %s", got, task.id)
	}
	status, generation := scene.taskClaim(t, task.id)
	if status != "dispatched" || generation != refusedGeneration+1 {
		t.Fatalf("task after the next claim: status %s, claim generation %d; want dispatched and %d", status, generation, refusedGeneration+1)
	}
	attempt := scene.attempt(t, task.id, generation)
	if got := out.Task.text(t, "budget_attempt_id"); got != attempt.ID {
		t.Fatalf("payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
	}
	if n := scene.attempts(t, task.id); n != 1 {
		t.Fatalf("the task has %d attempt rows, want 1: the refused claim has none", n)
	}
}

// TestClaimBudgetBindingInProgressInvalidatesEmptyVerdict is the cache half
// of H06, with the empty-claim cache a deployment with Redis has. While the
// refused claim holds the task as dispatched, another poll of the runtime
// finds nothing queued and caches that verdict under the version it sampled
// before. Handing the task back must put that verdict out of date, or the
// daemon is told "nothing" until the verdict expires.
func TestClaimBudgetBindingInProgressInvalidatesEmptyVerdict(t *testing.T) {
	scene := newClaimBudgetScene(t)
	declared := claimBudgetHeader(claimBudgetCapability)
	cache, fake := installClaimBudgetEmptyClaimCache(t)
	ctx := context.Background()

	task := scene.queueTask(t, "cached")
	scene.enableRoot(t, task.issue)
	_, queuedGeneration := scene.taskClaim(t, task.id)

	// The hazard is real in this wiring: a verdict that is current hides a
	// queued task from the endpoint, which answers without reading the queue.
	cache.MarkEmpty(ctx, scene.runtime, cache.CurrentVersion(ctx, scene.runtime))
	hidden := scene.claimSingle(t, declared).Want(http.StatusOK)
	if status, generation := scene.taskClaim(t, task.id); strings.TrimSuffix(hidden.Text(), "\n") != `{"task":null}` || status != "queued" || generation != queuedGeneration {
		t.Fatalf("premise: a current empty verdict did not hide the queued task: answer %q, status %s, claim generation %d", hidden.Text(), status, generation)
	}
	// An enqueue puts a verdict out of date; the fixture's insert is not one.
	cache.Bump(ctx, scene.runtime)

	release := scene.holdBindingLock(t)

	// The other poll samples the version before the refused claim is done.
	sampled := cache.CurrentVersion(ctx, scene.runtime)
	refused := scene.claimSingle(t, declared).Want(http.StatusOK)
	if got := strings.TrimSuffix(refused.Text(), "\n"); got != `{"task":null}` {
		t.Fatalf("answer while a binding is in progress = %s, want {\"task\":null}", got)
	}
	status, refusedGeneration := scene.taskClaim(t, task.id)
	if status != "queued" || refusedGeneration != queuedGeneration+1 {
		t.Fatalf("task after the refused claim: status %s, claim generation %d; want queued and %d", status, refusedGeneration, queuedGeneration+1)
	}
	// Its verdict lands: nothing was queued while the claim held the task.
	cache.MarkEmpty(ctx, scene.runtime, sampled)

	release()

	var out claimBudgetSingleResponse
	scene.claimSingle(t, declared).Want(http.StatusOK).JSON(&out)
	if got := out.Task.text(t, "id"); got != task.id {
		t.Fatalf("claim after the binding ended returned task %q, want the requeued task %s: an empty verdict from before the requeue hid it", got, task.id)
	}
	if status, generation := scene.taskClaim(t, task.id); status != "dispatched" || generation != refusedGeneration+1 {
		t.Fatalf("task after the next claim: status %s, claim generation %d; want dispatched and %d", status, generation, refusedGeneration+1)
	}
	if unanswered := fake.unanswered(); len(unanswered) != 0 {
		t.Fatalf("the cache sent commands the test's Redis has no answer for, so it did not run as a cache: %q", unanswered)
	}
}

// TestClaimBudgetBatchBindingInProgress is H07: the same through the batch
// endpoint, with two queued tasks in the locked workspace, one of a member
// issue and one of an issue outside every root. A claim cannot tell yet
// whether its issue is being bound, so both are left out; the next batch
// carries both.
func TestClaimBudgetBatchBindingInProgress(t *testing.T) {
	scene := newClaimBudgetScene(t)
	declared := claimBudgetHeader(claimBudgetCapability)

	// What the endpoint answers when nothing was claimed.
	nothing := scene.claimBatch(t, declared).Want(http.StatusOK)

	member := scene.queueTask(t, "member")
	outside := scene.queueTask(t, "outside")
	scene.enableRoot(t, member.issue)
	tasks := []claimBudgetTask{member, outside}
	before := scene.budgetRows(t)
	queuedGeneration := map[string]int64{}
	for _, task := range tasks {
		_, queuedGeneration[task.id] = scene.taskClaim(t, task.id)
	}

	release := scene.holdBindingLock(t)
	refused := scene.claimBatch(t, declared).Want(http.StatusOK)
	if got := strings.TrimSuffix(refused.Text(), "\n"); got != `{"tasks":[]}` {
		t.Fatalf("answer while a binding is in progress = %s, want {\"tasks\":[]}", got)
	}
	if !bytes.Equal(refused.Body.Bytes(), nothing.Body.Bytes()) {
		t.Fatalf("answer while a binding is in progress = %q, want the bytes of the answer when nothing was claimed, %q",
			refused.Text(), nothing.Text())
	}

	refusedGeneration := map[string]int64{}
	for _, task := range tasks {
		status, generation := scene.taskClaim(t, task.id)
		if status != "queued" || generation != queuedGeneration[task.id]+1 {
			t.Fatalf("task %s after the refused batch: status %s, claim generation %d; want queued and %d",
				task.id, status, generation, queuedGeneration[task.id]+1)
		}
		if n := scene.taskTokens(t, task.id); n != 0 {
			t.Fatalf("the refused claim of task %s left %d task tokens, want 0", task.id, n)
		}
		refusedGeneration[task.id] = generation
	}
	if after := scene.budgetRows(t); after != before {
		t.Fatalf("budget rows of the workspace = %+v after the refused batch, want them unchanged at %+v", after, before)
	}

	release()

	var out claimBudgetBatchResponse
	scene.claimBatch(t, declared).Want(http.StatusOK).JSON(&out)
	if len(out.Tasks) != 2 {
		t.Fatalf("the batch after the binding ended carries %d tasks, want the 2 requeued ones", len(out.Tasks))
	}
	for _, task := range tasks {
		out.payload(t, task.id)
		status, generation := scene.taskClaim(t, task.id)
		if status != "dispatched" || generation != refusedGeneration[task.id]+1 {
			t.Fatalf("task %s after the next batch: status %s, claim generation %d; want dispatched and %d",
				task.id, status, generation, refusedGeneration[task.id]+1)
		}
	}
	_, generation := scene.taskClaim(t, member.id)
	attempt := scene.attempt(t, member.id, generation)
	if got := out.payload(t, member.id).text(t, "budget_attempt_id"); got != attempt.ID {
		t.Fatalf("member task: payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
	}
	if raw, ok := out.payload(t, outside.id)["budget_attempt_id"]; ok {
		t.Fatalf("the payload of the task outside every root has budget_attempt_id %s; the key must be absent", raw)
	}
	if n := scene.attempts(t, member.id) + scene.attempts(t, outside.id); n != 1 {
		t.Fatalf("the two tasks have %d attempt rows, want 1: the member task's claim after the binding ended", n)
	}
}

// TestClaimBudgetFinalizeFailure is H08: a finalization that fails for
// another reason than a binding is still a failure. An attempt row already
// stands where the claim's own would go, so the budget step's insert is
// refused by the key on task and claim generation. The single-runtime
// endpoint answers 500, the batch endpoint leaves the task out, and either
// way the task is requeued and the claim leaves nothing.
func TestClaimBudgetFinalizeFailure(t *testing.T) {
	for _, endpoint := range []string{"single runtime", "batch"} {
		t.Run(endpoint, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			declared := claimBudgetHeader(claimBudgetCapability)
			task := scene.queueTask(t, "collision")
			root := scene.enableRoot(t, task.issue)
			_, queuedGeneration := scene.taskClaim(t, task.id)
			standing := scene.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
				"id":               uuid.NewString(),
				"workspace_id":     scene.workspace,
				"budget_root_id":   root.BudgetRootID,
				"task_id":          task.id,
				"claim_generation": queuedGeneration + 1,
				"runtime_id":       scene.runtime,
				"agent_id":         task.agent,
				"issue_id":         task.issue,
			})
			before := scene.budgetRows(t)

			if endpoint == "batch" {
				var out claimBudgetBatchResponse
				scene.claimBatch(t, declared).Want(http.StatusOK).JSON(&out)
				if len(out.Tasks) != 0 {
					t.Fatalf("the batch carries %d tasks, want none: the only claim failed to finalize", len(out.Tasks))
				}
			} else {
				var out map[string]string
				scene.claimSingle(t, declared).Want(http.StatusInternalServerError).JSON(&out)
				if out["error"] != "failed to finalize task claim" {
					t.Fatalf("error = %q, want the answer of a failed finalization, \"failed to finalize task claim\"", out["error"])
				}
			}

			status, generation := scene.taskClaim(t, task.id)
			if status != "queued" || generation != queuedGeneration+1 {
				t.Fatalf("task after the failed claim: status %s, claim generation %d; want queued and %d", status, generation, queuedGeneration+1)
			}
			if after := scene.budgetRows(t); after != before {
				t.Fatalf("budget rows of the workspace = %+v after the failed claim, want them unchanged at %+v", after, before)
			}
			if got := scene.attempt(t, task.id, generation).ID; got != standing {
				t.Fatalf("attempt of the failed claim = %s, want the row that stood there before it, %s", got, standing)
			}
			if n := scene.taskTokens(t, task.id); n != 0 {
				t.Fatalf("the failed claim left %d task tokens, want 0", n)
			}
		})
	}
}

// TestClaimBudgetDaemonRPC is H09: a claim over the daemon WebSocket RPC
// carries the capabilities the daemon sent when it opened the socket, which
// the handler keeps in the connection's identity.
func TestClaimBudgetDaemonRPC(t *testing.T) {
	cases := []struct {
		name         string
		capabilities string
		protocol     string
	}{
		{"declared among others", "admin_cancel_ack_v1, budget-usage-v1", "1"},
		{"other capabilities only", "admin_cancel_ack_v1", "NULL"},
		{"none", "", "NULL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			task := scene.queueTask(t, "rpc")
			scene.enableRoot(t, task.issue)

			identity := daemonws.ClientIdentity{
				DaemonID:     claimBudgetDaemonID,
				UserID:       testUserID,
				WorkspaceID:  scene.workspace,
				WorkspaceIDs: []string{scene.workspace},
				RuntimeIDs:   []string{scene.runtime},
				Capabilities: tc.capabilities,
			}
			body, err := json.Marshal(scene.batchBody())
			if err != nil {
				t.Fatalf("encode the claim request: %v", err)
			}
			status, raw, err := testHandler.DaemonRPCHandler(context.Background(), identity, "tasks.claim", body)
			if err != nil {
				t.Fatalf("tasks.claim over RPC: %v", err)
			}
			if status != http.StatusOK {
				t.Fatalf("tasks.claim over RPC: status %d, want 200: %s", status, raw)
			}
			var out claimBudgetBatchResponse
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("tasks.claim over RPC: decode response: %v: %s", err, raw)
			}
			if len(out.Tasks) != 1 {
				t.Fatalf("tasks.claim over RPC carries %d tasks, want the queued one", len(out.Tasks))
			}

			_, generation := scene.taskClaim(t, task.id)
			attempt := scene.attempt(t, task.id, generation)
			if got := out.payload(t, task.id).text(t, "budget_attempt_id"); got != attempt.ID {
				t.Fatalf("payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
			}
			if got := attempt.protocol(); got != tc.protocol {
				t.Fatalf("connection capabilities %q: attempt protocol_version = %s, want %s", tc.capabilities, got, tc.protocol)
			}
		})
	}
}

// TestClaimBudgetClosedRoot is H10: this slice observes and refuses nothing.
// A claim an enforcing root would have refused, here because the root is
// closed, is answered like any other: the task and its attempt id.
func TestClaimBudgetClosedRoot(t *testing.T) {
	scene := newClaimBudgetScene(t)
	task := scene.queueTask(t, "closed root")
	root := scene.enableRoot(t, task.issue)
	if _, err := service.NewIssueBudgetService(db.New(testPool), testPool).Close(context.Background(), service.IssueBudgetManageParams{
		WorkspaceID:            parseUUID(scene.workspace),
		BudgetRootID:           parseUUID(root.BudgetRootID),
		OperationKey:           "close-" + root.BudgetRootID,
		ExpectedPolicyRevision: int64(root.PolicyRevision),
		Reason:                 "claim endpoint test",
		ActorUserID:            parseUUID(testUserID),
	}); err != nil {
		t.Fatalf("setup: close the budget root: %v", err)
	}

	var out claimBudgetSingleResponse
	scene.claimSingle(t, claimBudgetHeader(claimBudgetCapability)).Want(http.StatusOK).JSON(&out)
	if got := out.Task.text(t, "id"); got != task.id {
		t.Fatalf("claimed task = %q, want the queued task %s", got, task.id)
	}
	status, generation := scene.taskClaim(t, task.id)
	if status != "dispatched" {
		t.Fatalf("task status = %s, want dispatched", status)
	}
	attempt := scene.attempt(t, task.id, generation)
	if got := out.Task.text(t, "budget_attempt_id"); got != attempt.ID {
		t.Fatalf("payload budget_attempt_id = %q, want the attempt row %s", got, attempt.ID)
	}
	// The premise held for this very claim: the service recorded that an
	// enforcing root would have refused it.
	if n := scene.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE workspace_id = $1 AND kind = 'would_refuse' AND attempt_id = $2`,
		scene.workspace, attempt.ID); n == 0 {
		t.Fatalf("no would_refuse observation names attempt %s: the claim was not one a closed root would refuse", attempt.ID)
	}
}

// TestClaimBudgetBindingInProgressLogsAtInfo is H11: a claim handed back for
// a binding in progress is not a failure of the claim, and neither endpoint
// logs it as one.
func TestClaimBudgetBindingInProgressLogsAtInfo(t *testing.T) {
	for _, endpoint := range []string{"single runtime", "batch"} {
		t.Run(endpoint, func(t *testing.T) {
			scene := newClaimBudgetScene(t)
			declared := claimBudgetHeader(claimBudgetCapability)
			task := scene.queueTask(t, "logged")
			scene.enableRoot(t, task.issue)
			scene.holdBindingLock(t)

			captured := captureClaimBudgetLog(t)
			if endpoint == "batch" {
				scene.claimBatch(t, declared).Want(http.StatusOK)
			} else {
				scene.claimSingle(t, declared).Want(http.StatusOK)
			}
			if status, _ := scene.taskClaim(t, task.id); status != "queued" {
				t.Fatalf("task status = %s, want queued: the claim was not handed back", status)
			}

			var refusals []string
			for _, record := range captured.about("task_id=" + task.id) {
				if strings.Contains(record, "level=ERROR") {
					t.Fatalf("a claim handed back for a binding in progress was logged as an error: %s", record)
				}
				if strings.Contains(record, "budget binding in progress") {
					refusals = append(refusals, record)
				}
			}
			if len(refusals) != 1 || !strings.Contains(refusals[0], "level=INFO") {
				t.Fatalf("records of the refusal = %q, want one record at level INFO", refusals)
			}
		})
	}
}
