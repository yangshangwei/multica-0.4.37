package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Full-router wiring owns the pre-auth cache policy and credential boundary.
// Detailed policy and ledger matrices live in the service and handler suites.
func budgetRouterCall(t *testing.T, token, method, path string, body any, headers map[string]string) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%s %s: budget response missing no-store", method, path)
	}
	return response
}

func budgetRouterFixture(t *testing.T) (*testutil.Fixture, string, service.IssueBudgetWriteResponse) {
	t.Helper()
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	issue := fx.Issue(t, "Budget router private title")
	var created service.IssueBudgetWriteResponse
	body := map[string]any{"request_key": uuid.NewString(), "reason": "Router budget", "limit_ticks": "100", "attempt_reserve_ticks": "10"}
	budgetRouterCall(t, testToken, "POST", "/api/issues/"+issue+"/budget", body, nil).Want(201).JSON(&created)
	for _, table := range []string{"issue_budget_root", "issue_budget_member", "issue_budget_attempt", "issue_budget_entry", "issue_budget_operation", "issue_budget_observation"} {
		column := "budget_root_id"
		if table == "issue_budget_root" {
			column = "id"
		}
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE "+column+"=$1", created.Result.BudgetRootID)
	}
	return fx, issue, created
}

func TestIssueBudgetRouterManagementRecoveryAndTombstone(t *testing.T) {
	fx, issue, created := budgetRouterFixture(t)
	rootPath := "/api/issue-budgets/" + created.Result.BudgetRootID
	issuePath := "/api/issues/" + issue + "/budget"
	var binding service.IssueBudgetBinding
	budgetRouterCall(t, testToken, "GET", issuePath, nil, nil).Want(200).JSON(&binding)
	if binding.BudgetRootID != created.Result.BudgetRootID || !binding.IsRootIssue {
		t.Fatalf("wrong binding: %+v", binding)
	}
	var show service.IssueBudgetShow
	budgetRouterCall(t, testToken, "GET", rootPath, nil, nil).Want(200).JSON(&show)
	if show.Amounts == nil || show.RemainingTicks == nil || *show.RemainingTicks != 100 || !show.TotalKnown {
		t.Fatalf("new root projection: %+v", show)
	}
	for _, suffix := range []string{"/observations", "/attempts"} {
		response := budgetRouterCall(t, testToken, "GET", rootPath+suffix, nil, nil).Want(200)
		if !strings.Contains(response.Body.String(), `"next_cursor":null`) || !strings.Contains(response.Body.String(), `:[]`) {
			t.Fatalf("empty page changed nullable/array contract: %s", response.Body)
		}
	}
	key := uuid.NewString()
	unknown := budgetRouterCall(t, testToken, "GET", rootPath+"/operations/"+key, nil, nil).Want(200).Map()
	if unknown["state"] != "unknown" || unknown["retry_path"] != rootPath+"/operations/"+key {
		t.Fatalf("unknown operation: %+v", unknown)
	}
	body := map[string]any{"kind": "close", "request_key": key, "reason": "Close same root", "expected_policy_revision": "1"}
	var closed, replay service.IssueBudgetWriteResponse
	budgetRouterCall(t, testToken, "POST", rootPath+"/operations", body, nil).Want(201).JSON(&closed)
	budgetRouterCall(t, testToken, "POST", rootPath+"/operations", body, nil).Want(200).JSON(&replay)
	a, _ := json.Marshal(closed.Result)
	b, _ := json.Marshal(replay.Result)
	if !replay.Replayed || !bytes.Equal(a, b) {
		t.Fatal("original-key replay changed the receipt")
	}
	var receipt service.IssueBudgetOperationLookup
	budgetRouterCall(t, testToken, "GET", rootPath+"/operations/"+key, nil, nil).Want(200).JSON(&receipt)
	if receipt.State != "committed" || receipt.Stale || receipt.Result.OperationID != closed.Result.OperationID {
		t.Fatalf("receipt lookup: %+v", receipt)
	}
	// A genuine issue deletion must retain the economic identity while removing
	// the issue-facing lookup. Do not fabricate a tombstone row in this test.
	req := testutil.JSONRequest("DELETE", "/api/issues/"+issue, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(204)
	response := budgetRouterCall(t, testToken, "GET", rootPath, nil, nil).Want(200)
	response.JSON(&show)
	if show.AdmissionState != "tombstoned" || show.RootIssueID != issue || strings.Contains(response.Body.String(), "private title") {
		t.Fatalf("deleted root projection: %s", response.Body)
	}
	budgetRouterCall(t, testToken, "GET", issuePath, nil, nil).Want(404)
	if fx.Count(t, "SELECT count(*) FROM issue_budget_root WHERE id=$1", created.Result.BudgetRootID) != 1 {
		t.Fatal("deletion removed budget root")
	}
}

func TestIssueBudgetRouterCredentialBoundaryBeforeCapabilityGuard(t *testing.T) {
	_, issue, created := budgetRouterFixture(t)
	rootPath := "/api/issue-budgets/" + created.Result.BudgetRootID
	paths := []struct{ method, path string }{
		{"POST", "/api/issues/" + issue + "/budget"}, {"GET", "/api/issues/" + issue + "/budget"},
		{"GET", "/api/issues/" + issue + "/budget/operations/" + uuid.NewString()}, {"GET", rootPath},
		{"POST", rootPath + "/operations"}, {"GET", rootPath + "/operations/" + uuid.NewString()},
		{"GET", rootPath + "/observations"}, {"GET", rootPath + "/attempts"},
	}
	for _, profile := range []struct {
		key    string
		scoped bool
	}{{"progress-reporter", true}, {"delivery-reviewer", true}, {"custom-budget-test", false}} {
		t.Run(profile.key, func(t *testing.T) {
			f := newTargetedCapabilityFixture(t, profile.key, 2, profile.scoped)
			for _, path := range paths {
				response := budgetRouterCall(t, f.token, path.method, path.path, nil, nil).Want(403)
				if response.Body.String() != "{\"error\":\"budget_forbidden\",\"code\":\"budget_forbidden\"}\n" {
					t.Fatalf("%s bypassed budget's minimal denial: %s", profile.key, response.Body)
				}
			}
			budgetRouterCall(t, f.token, "POST", "/api/daemon/budget/attempts/"+uuid.NewString()+"/usage", nil, nil).Want(401)
		})
	}
	for _, hints := range []map[string]string{{"X-Agent-ID": ""}, {"X-Task-ID": ""}, {"X-Actor-Source": "member"}, {"X-Actor-Source": "task_token"}} {
		response := budgetRouterCall(t, testToken, "GET", rootPath, nil, hints).Want(403)
		if strings.Contains(response.Body.String(), created.Result.BudgetRootID) {
			t.Fatal("actor-hint refusal disclosed root")
		}
	}
	budgetRouterCall(t, testToken, "GET", rootPath, nil, map[string]string{"X-Actor-Source": ""}).Want(200)
	for _, token := range []string{"", "mdt_unknown", "mcn_unknown"} {
		for _, path := range paths {
			budgetRouterCall(t, token, path.method, path.path, nil, nil).Want(401)
		}
	}
}

func TestIssueBudgetRouterHumanPATAndCrossWorkspacePrivacy(t *testing.T) {
	fx, _, created := budgetRouterFixture(t)
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": testUserID, "name": "budget human", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "expires_at": time.Now().Add(time.Hour)})
	path := "/api/issue-budgets/" + created.Result.BudgetRootID
	budgetRouterCall(t, pat, "GET", path, nil, nil).Want(200)
	other := fx.Workspace(t, "Other budget workspace", "budget-other-"+uuid.NewString())
	fx.Member(t, other, testUserID, "owner")
	foreign := budgetRouterCall(t, testToken, "GET", path, nil, map[string]string{"X-Workspace-ID": other}).Want(404)
	absent := budgetRouterCall(t, testToken, "GET", "/api/issue-budgets/"+uuid.NewString(), nil, map[string]string{"X-Workspace-ID": other}).Want(404)
	if foreign.Body.String() != absent.Body.String() {
		t.Fatal("foreign root existence leaked")
	}
	member := fx.User(t, "Budget outsider", "budget-outsider-"+uuid.NewString()+"@example.test")
	memberToken, err := generateTestJWT(member, "budget-outsider@example.test", "Budget outsider")
	if err != nil {
		t.Fatal(err)
	}
	denied := budgetRouterCall(t, memberToken, "GET", path, nil, nil).Want(403)
	if denied.Body.String() != "{\"error\":\"budget_forbidden\",\"code\":\"budget_forbidden\"}\n" {
		t.Fatalf("workspace middleware changed budget denial: %s", denied.Body)
	}
}

func TestIssueBudgetRouterClaimUsageAndOriginalBatchRecovery(t *testing.T) {
	fx, issue, created := budgetRouterFixture(t)
	runtime := fx.Runtime(t, "budget router runtime")
	agent := fx.Agent(t, "budget router agent", runtime)
	task := fx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime})
	req := testutil.JSONRequest("POST", "/api/daemon/runtimes/"+runtime+"/tasks/claim", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Client-Capabilities", "budget-usage-v1")
	var claim struct {
		Task struct {
			ID      string `json:"id"`
			Attempt string `json:"budget_attempt_id"`
		} `json:"task"`
	}
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(200).JSON(&claim)
	if claim.Task.ID != task || claim.Task.Attempt == "" {
		t.Fatalf("claim identity: %+v", claim)
	}
	key := "router_original_batch_key"
	path := "/api/daemon/budget/attempts/" + claim.Task.Attempt + "/usage"
	body := map[string]any{"protocol_version": 1, "batch_key": key, "producer_epoch": "router-epoch", "adapter": "claude", "normalizer_version": "1", "reported_at": "2026-11-20T01:00:00Z", "invocations": []any{map[string]any{"invocation_id": uuid.NewString(), "resumed": false, "ended": "returned", "observed_at": "2026-11-20T01:00:00Z", "usage": []any{map[string]any{"model": "claude-fable-5", "tokens": map[string]string{"input": "0", "output": "0", "cache_read": "0", "cache_write": "0"}, "cost_ticks": "30"}}}}}
	var first, replay service.IssueBudgetUsageResponse
	budgetRouterCall(t, testToken, "POST", path, body, nil).Want(200).JSON(&first)
	budgetRouterCall(t, testToken, "POST", path, body, nil).Want(200).JSON(&replay)
	if first.Status != "recorded" || replay.Replayed == nil || !*replay.Replayed {
		t.Fatalf("usage replay: %+v / %+v", first, replay)
	}
	budgetRouterCall(t, testToken, "GET", path+"/"+key, nil, nil).Want(200)
	var show service.IssueBudgetShow
	budgetRouterCall(t, testToken, "GET", "/api/issue-budgets/"+created.Result.BudgetRootID, nil, nil).Want(200).JSON(&show)
	if show.Amounts == nil || show.Amounts.ActualKnown != 0 || show.Amounts.Estimated != 30 || show.Coverage.IncludedAttempts == nil || *show.Coverage.IncludedAttempts != 1 {
		t.Fatalf("production evidence table must estimate Claude, never claim actual: %+v", show)
	}
	if got := fx.Count(t, "SELECT count(*) FROM issue_budget_operation WHERE scope_id=$1 AND operation_kind='usage_batch'", claim.Task.Attempt); got != 1 {
		t.Fatal(fmt.Sprintf("batch recorded %d times", got))
	}
}
