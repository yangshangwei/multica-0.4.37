package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func daemonUsageScene(t *testing.T) (*claimBudgetScene, claimBudgetTask, string, string) {
	t.Helper()
	s := newClaimBudgetScene(t)
	task := s.queueTask(t, "Usage")
	root := s.enableRoot(t, task.issue)
	var out claimBudgetSingleResponse
	s.claimSingle(t, claimBudgetHeader(claimBudgetCapability)).Want(200).JSON(&out)
	if len(out.Task) == 0 {
		t.Fatal("claim missing task")
	}
	return s, task, out.Task.text(t, "budget_attempt_id"), root.BudgetRootID
}
func daemonUsageBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../pkg/protocol/testdata/budget_usage_v1_batch.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func daemonUsageRequest(s *claimBudgetScene, attempt string, body any) *http.Request {
	req := testutil.JSONRequest("POST", "/api/daemon/budget/attempts/"+attempt+"/usage", body)
	req = req.WithContext(middleware.WithDaemonContext(req.Context(), s.workspace, claimBudgetDaemonID))
	return testutil.WithURLParams(req, "attemptId", attempt)
}

type usageUnreadBody struct{ read bool }

func (b *usageUnreadBody) Read([]byte) (int, error) {
	b.read = true
	return 0, errors.New("must not read body")
}
func (b *usageUnreadBody) Close() error { return nil }

func TestDaemonBudgetUsageAuthorizationAndValidationOrder(t *testing.T) {
	s, _, attempt, _ := daemonUsageScene(t)
	initial := s.budgetRows(t)
	for _, tc := range []struct {
		name, id, workspace string
		status              int
	}{{"invalid id", "bad", s.workspace, 400}, {"missing", uuid.NewString(), s.workspace, 404}, {"workspace", attempt, uuid.NewString(), 404}} {
		t.Run(tc.name, func(t *testing.T) {
			body := &usageUnreadBody{}
			req := daemonUsageRequest(s, tc.id, nil)
			req.Body = body
			req = req.WithContext(middleware.WithDaemonContext(req.Context(), tc.workspace, claimBudgetDaemonID))
			response := testutil.Call(t, testHandler.ReportBudgetAttemptUsage, req).Want(tc.status)
			if tc.status == 404 && response.Text() != "{\"error\":\"not found\"}\n" {
				t.Fatalf("404 disclosed attempt: %s", response.Text())
			}
			if body.read {
				t.Fatal("body read before authority")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		body   any
		status int
		code   string
	}{{"oversize", strings.Repeat("x", service.IssueBudgetUsageMaxBytes+1), 413, "request_too_large"}, {"invalid", "{}", 400, "invalid_request"}} {
		t.Run(tc.name, func(t *testing.T) {
			response := testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, tc.body)).Want(tc.status).Map()
			if response["code"] != tc.code {
				t.Fatalf("response = %v", response)
			}
		})
	}
	s.fx.Exec(t, `UPDATE issue_budget_attempt SET protocol_version=NULL WHERE id=$1`, attempt)
	// Decode precedes protocol comparison.
	testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, "{}")).Want(400)
	got := testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, daemonUsageBytes(t))).Want(409).Map()
	if got["code"] != "protocol_mismatch" {
		t.Fatalf("protocol = %v", got)
	}
	if s.budgetRows(t) != initial {
		t.Fatal("refused requests wrote budget rows")
	}
}

func TestDaemonBudgetUsageRuntimeRemovalAndDatabaseErrors(t *testing.T) {
	s, task, attempt, _ := daemonUsageScene(t)
	// Removing the runtime makes only an authorized workspace see this 404.
	// Task/runtime relationships are absent from the retained budget attempt.
	s.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, task.id)
	s.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE id=$1`, task.agent)
	s.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, s.runtime)
	body := &usageUnreadBody{}
	req := daemonUsageRequest(s, attempt, nil)
	req.Body = body
	response := testutil.Call(t, testHandler.ReportBudgetAttemptUsage, req).Want(404)
	if response.Map()["error"] != "runtime not found" || body.read {
		t.Fatal("runtime gate ordering")
	}
	// The attempt SELECT actually fails, rather than turning a database error
	// into a 404 that would make the daemon discard its retained report.
	h := *testHandler
	h.Queries = db.New(usageFailDB{DBTX: testPool})
	testutil.Call(t, h.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, nil)).Want(500)
	h.Queries = db.New(usageFailRuntimeDB{DBTX: testPool})
	testutil.Call(t, h.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, nil)).Want(500)
}

type usageFailRuntimeDB struct{ db.DBTX }

func (d usageFailRuntimeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "GetAgentRuntime") {
		return usageFailRow{}
	}
	return d.DBTX.QueryRow(ctx, sql, args...)
}

type usageFailDB struct{ db.DBTX }

func (usageFailDB) QueryRow(context.Context, string, ...any) pgx.Row { return usageFailRow{} }

type usageFailRow struct{}

func (usageFailRow) Scan(...any) error { return errors.New("synthetic database read failure") }

func TestDaemonBudgetUsageHTTPReceiptsAndLegacyIndependence(t *testing.T) {
	s, task, attempt, root := daemonUsageScene(t)
	usage := service.NewIssueBudgetUsageService(testHandler.Queries, testHandler.TxStarter)
	usage.Evidence = service.IssueBudgetUsageEvidenceTable{{Adapter: "claude", NormalizerVersion: "1"}: {FreshOwnUsage: true, ResumedOwnUsage: true, ReportedCost: true}}
	raw := daemonUsageBytes(t)
	received := time.Date(2026, 11, 20, 2, 3, 4, 0, time.UTC)
	report := func(w http.ResponseWriter, r *http.Request) {
		testHandler.reportBudgetAttemptUsage(w, r, received, usage)
	}
	first := testutil.Call(t, report, daemonUsageRequest(s, attempt, raw)).Want(200).Map()
	if first["status"] != "recorded" || first["replayed"] != false {
		t.Fatalf("first response = %v", first)
	}
	for _, field := range []string{"budget_root_id", "ledger_revision", "actual_known_ticks", "estimated_ticks", "attempt_id"} {
		if _, ok := first[field]; ok {
			t.Fatalf("daemon response discloses %s", field)
		}
	}
	for _, r := range first["receipts"].([]any) {
		if len(r.(map[string]any)) != 4 {
			t.Fatalf("receipt not minimal: %v", r)
		}
	}
	var actual int64
	s.fx.QueryRow(t, `SELECT actual_known_ticks FROM issue_budget_root WHERE id=$1`, root).Scan(&actual)
	if actual != 81_000_011 {
		t.Fatalf("actual = %d", actual)
	}
	legacy := func(cost int64) *testutil.Response {
		req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+task.id+"/usage", map[string]any{"usage": []map[string]any{{"provider": "claude", "model": "<synthetic>", "cost_usd_ticks": cost}}}, s.workspace, claimBudgetDaemonID)
		return testutil.Call(t, testHandler.ReportTaskUsage, testutil.WithURLParams(req, "taskId", task.id)).Want(200)
	}
	legacy(81_000_000)
	legacy(99)
	if n := s.fx.Count(t, `SELECT count(*) FROM task_usage WHERE task_id=$1`, task.id); n != 1 {
		t.Fatalf("legacy usage count = %d", n)
	}
	s.fx.Exec(t, `UPDATE task_usage SET cost_usd_ticks=999 WHERE task_id=$1`, task.id)
	s.fx.Exec(t, `DELETE FROM task_usage WHERE task_id=$1`, task.id)
	// Seed the hourly projection, then rewrite and remove it too. None is an
	// input to the budget ledger.
	s.fx.InsertNoID(t, "task_usage_hourly", testutil.Cols{"bucket_hour": testutil.Raw("date_trunc('hour',now())"), "workspace_id": s.workspace, "runtime_id": s.runtime, "agent_id": task.agent, "project_id": nil, "provider": "claude", "model": "<synthetic>", "cost_usd_ticks": 81_000_000}, "workspace_id=$1", s.workspace)
	s.fx.Exec(t, `UPDATE task_usage_hourly SET cost_usd_ticks=7 WHERE workspace_id=$1`, s.workspace)
	s.fx.Exec(t, `DELETE FROM task_usage_hourly WHERE workspace_id=$1`, s.workspace)
	s.fx.QueryRow(t, `SELECT actual_known_ticks FROM issue_budget_root WHERE id=$1`, root).Scan(&actual)
	if actual != 81_000_011 {
		t.Fatalf("legacy projection moved budget: %d", actual)
	}
	replay := testutil.Call(t, report, daemonUsageRequest(s, attempt, raw)).Want(200).Map()
	if replay["replayed"] != true {
		t.Fatal("not replayed")
	}
	query := daemonUsageRequest(s, attempt, nil)
	query.Method = "GET"
	query = testutil.WithURLParams(query, "attemptId", attempt, "batchKey", "0195f3a2-0000-7000-8000-000000000002")
	got := testutil.Call(t, testHandler.GetBudgetAttemptUsage, query).Want(200).Map()
	if got["replayed"] != true {
		t.Fatal("query did not recover")
	}
	query = testutil.WithURLParams(query, "attemptId", attempt, "batchKey", "unknown_original_key")
	got = testutil.Call(t, testHandler.GetBudgetAttemptUsage, query).Want(200).Map()
	if got["status"] != "unknown" || got["query_path"] == nil {
		t.Fatalf("unknown = %v", got)
	}
}

func TestDaemonBudgetUsageTaskCredentialRejectedByDaemonAuth(t *testing.T) {
	s, task, attempt, _ := daemonUsageScene(t)
	token := "mat_usage_" + uuid.NewString()
	if _, err := testHandler.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(token), TaskID: parseUUID(task.id), AgentID: parseUUID(task.agent), WorkspaceID: parseUUID(s.workspace), UserID: parseUUID(testUserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	s.fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, task.id)
	before := s.budgetRows(t)
	r := chi.NewRouter()
	r.Use(middleware.DaemonAuth(testHandler.Queries, nil, nil, nil))
	r.Post("/api/daemon/budget/attempts/{attemptId}/usage", testHandler.ReportBudgetAttemptUsage)
	r.Get("/api/daemon/budget/attempts/{attemptId}/usage/{batchKey}", testHandler.GetBudgetAttemptUsage)
	for _, method := range []string{"POST", "GET"} {
		path := "/api/daemon/budget/attempts/" + attempt + "/usage"
		if method == "GET" {
			path += "/unknown_original_key"
		}
		req := testutil.JSONRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-User-ID", testUserID)
		req.Header.Set("X-Workspace-ID", s.workspace)
		testutil.Call(t, r.ServeHTTP, req).Want(401)
	}
	if s.budgetRows(t) != before {
		t.Fatal("task credential wrote usage")
	}
}

func TestDaemonBudgetUsageAtomicRefusalVersusLegacy(t *testing.T) {
	s, task, attempt, _ := daemonUsageScene(t)
	before := s.budgetRows(t)
	raw := strings.Replace(string(daemonUsageBytes(t)), `"ended":"interrupted"`, `"ended":"invalid"`, 1)
	testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, raw)).Want(400)
	if s.budgetRows(t) != before {
		t.Fatal("partial batch committed")
	}
	// The legacy handler intentionally warns per failed row and answers 200.
	h := *testHandler
	h.Queries = db.New(usageFailExecDB{DBTX: testPool})
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+task.id+"/usage", map[string]any{"usage": []map[string]any{{"provider": "claude", "model": "valid", "input_tokens": 1}, {"provider": "claude", "model": "failed", "input_tokens": 1}}}, s.workspace, claimBudgetDaemonID)
	testutil.Call(t, h.ReportTaskUsage, testutil.WithURLParams(req, "taskId", task.id)).Want(200)
	if n := s.fx.Count(t, `SELECT count(*) FROM task_usage WHERE task_id=$1 AND model='valid'`, task.id); n != 1 {
		t.Fatal("legacy valid row did not survive failed row")
	}
	if n := s.fx.Count(t, `SELECT count(*) FROM task_usage WHERE task_id=$1 AND model='failed'`, task.id); n != 0 {
		t.Fatal("legacy failed row unexpectedly stored")
	}
}

type usageFailExecDB struct{ db.DBTX }

func (d usageFailExecDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "UpsertTaskUsage") && len(args) > 2 && args[2] == "failed" {
		return pgconn.CommandTag{}, errors.New("synthetic usage insert failure")
	}
	return d.DBTX.Exec(ctx, sql, args...)
}

var _ io.ReadCloser = (*usageUnreadBody)(nil)

func TestDaemonBudgetUsageLateHoldAndRevokedWorkspace(t *testing.T) {
	s, task, attempt, root := daemonUsageScene(t)
	ctx := context.Background()
	s.fx.Exec(t, `UPDATE agent_task_queue SET status='cancelled',completed_at=now() WHERE id=$1`, task.id)
	budgets := service.NewIssueBudgetService(testHandler.Queries, testHandler.TxStarter)
	if _, err := budgets.ReconcileLifecycle(ctx, parseUUID(s.workspace), parseUUID(root)); err != nil {
		t.Fatal(err)
	}
	before := s.budgetRows(t)
	// A PAT/JWT caller whose current membership is gone receives the same 404
	// as an absent attempt, even though the retained attempt and hold exist.
	user := s.fx.User(t, "Removed usage operator", "removed-usage-"+uuid.NewString()+"@example.test")
	s.fx.Member(t, s.workspace, user, "member")
	s.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, s.workspace, user)
	req := testutil.WithURLParams(testutil.JSONRequest("POST", "/api/daemon/budget/attempts/"+attempt+"/usage", daemonUsageBytes(t)), "attemptId", attempt)
	req.Header.Set("X-User-ID", user)
	testutil.Call(t, testHandler.ReportBudgetAttemptUsage, req).Want(404)
	if s.budgetRows(t) != before {
		t.Fatal("revoked caller wrote budget rows")
	}
	var unknown int64
	s.fx.QueryRow(t, `SELECT unknown_count FROM issue_budget_root WHERE id=$1`, root).Scan(&unknown)
	if unknown == 0 {
		t.Fatal("revoked report cleared unknown hold")
	}
	// Original runtime access remains valid after cancellation and accepts the
	// late report without reopening the attempt or releasing its reservation.
	var stateBefore, stateAfter string
	s.fx.QueryRow(t, `SELECT state FROM issue_budget_attempt WHERE id=$1`, attempt).Scan(&stateBefore)
	testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, daemonUsageBytes(t))).Want(200)
	s.fx.QueryRow(t, `SELECT state FROM issue_budget_attempt WHERE id=$1`, attempt).Scan(&stateAfter)
	if stateBefore != stateAfter {
		t.Fatal("report changed attempt state")
	}
}

func TestDaemonBudgetUsageHTTPStateAndErrorMapping(t *testing.T) {
	s, _, attempt, root := daemonUsageScene(t)
	ctx := context.Background()
	raw := daemonUsageBytes(t)
	s.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=1 WHERE id=$1`, root)
	got := testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, raw)).Want(503).Map()
	if got["code"] != "ledger_unavailable" {
		t.Fatalf("unreadable response = %v", got)
	}
	s.fx.Exec(t, `UPDATE issue_budget_root SET actual_known_ticks=0 WHERE id=$1`, root)
	budgets := service.NewIssueBudgetService(testHandler.Queries, testHandler.TxStarter)
	p := service.IssueBudgetManageParams{WorkspaceID: parseUUID(s.workspace), BudgetRootID: parseUUID(root), OperationKey: "http_observation_off", ExpectedPolicyRevision: 1, Reason: "http switch test", ActorUserID: parseUUID(testUserID)}
	if _, err := budgets.SetMode(ctx, p, "off"); err != nil {
		t.Fatal(err)
	}
	before := s.budgetRows(t)
	got = testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, raw)).Want(200).Map()
	if got["status"] != "not_recorded" || got["reason"] != "observation_off" || s.budgetRows(t) != before {
		t.Fatalf("off response = %v", got)
	}
	p.ExpectedPolicyRevision = 2
	p.OperationKey = "http_observation_on"
	if _, err := budgets.SetMode(ctx, p, "observe"); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, raw)).Want(200)
	got = testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, append(raw, ' '))).Want(409).Map()
	if got["code"] != "operation_conflict" {
		t.Fatalf("operation conflict = %v", got)
	}
	different := strings.Replace(string(raw), "0195f3a2-0000-7000-8000-000000000002", "another_batch_original_key", 1)
	got = testutil.Call(t, testHandler.ReportBudgetAttemptUsage, daemonUsageRequest(s, attempt, different)).Want(409).Map()
	if got["code"] != "batch_conflict" {
		t.Fatalf("batch conflict = %v", got)
	}
}
