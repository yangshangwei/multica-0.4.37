package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func budgetManagementRequest(workspace, user, role, method, path string, body any, params ...string) *http.Request {
	r := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "X-User-ID", user, "X-Workspace-ID", workspace)
	r = testutil.WithURLParams(r, params...)
	ctx := auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT})
	ctx = middleware.SetMemberContext(ctx, workspace, db.Member{WorkspaceID: parseUUID(workspace), UserID: parseUUID(user), Role: role})
	return r.WithContext(ctx)
}

func requireBudgetError(t *testing.T, response *testutil.Response, status int, code string) {
	t.Helper()
	response.Want(status)
	want := fmt.Sprintf("{\"error\":%q,\"code\":%q}\n", code, code)
	if response.Body.String() != want || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("budget error/header mismatch: %s %v", response.Body, response.Header())
	}
}

func requireBudgetFixtureKeys(t *testing.T, response *testutil.Response, fixture string, resultFixture ...string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/issue_budget_http_v1/" + fixture + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if json.Unmarshal(raw, &want) != nil || json.Unmarshal(response.Body.Bytes(), &got) != nil {
		t.Fatal("invalid fixture or response JSON")
	}
	if len(resultFixture) == 1 {
		// The lookup envelope is shared; its result has the exact field set
		// of the original write variant, including enable's omitted zeros.
		raw, err := os.ReadFile("testdata/issue_budget_http_v1/" + resultFixture[0] + ".json")
		var original map[string]any
		if err != nil || json.Unmarshal(raw, &original) != nil {
			t.Fatalf("read result fixture: %v", err)
		}
		want.(map[string]any)["result"] = original["result"]
	}
	var compare func(any, any, string)
	compare = func(want, got any, path string) {
		if examples, ok := want.([]any); ok {
			rows, ok := got.([]any)
			if !ok {
				t.Fatalf("%s: expected nonnullable list, got %T", path, got)
			}
			if len(examples) > 0 {
				for i, row := range rows {
					compare(examples[0], row, fmt.Sprintf("%s[%d]", path, i))
				}
			}
			return
		}
		wm, object := want.(map[string]any)
		if !object || got == nil {
			return
		}
		gm, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected object, got %T", path, got)
		}
		keys := func(m map[string]any) []string {
			out := make([]string, 0, len(m))
			for k := range m {
				out = append(out, k)
			}
			sort.Strings(out)
			return out
		}
		if !reflect.DeepEqual(keys(wm), keys(gm)) {
			t.Fatalf("%s fixture keys %v != response %v", path, keys(wm), keys(gm))
		}
		for k, w := range wm {
			compare(w, gm[k], path+"."+k)
		}
	}
	compare(want, got, fixture)
}

// B39/B58: every endpoint rejects the caller before body parsing or database
// access. A nil Handler would panic if any of these requests touched a query.
func TestIssueBudgetManagementHumanGate(t *testing.T) {
	h := &Handler{}
	workspace, user := uuid.NewString(), uuid.NewString()
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "future_credential", ""} {
		for _, call := range []http.HandlerFunc{h.EnableIssueBudget, h.GetIssueBudgetBinding, h.GetIssueBudgetEnableOperation,
			h.ShowIssueBudget, h.ManageIssueBudget, h.GetIssueBudgetOperation, h.ListIssueBudgetObservations, h.ListIssueBudgetAttempts} {
			r := budgetManagementRequest(workspace, user, "owner", "POST", "/api/issue-budgets/bad", "{malformed", "rootId", "bad", "id", "bad", "key", "bad")
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: user, Kind: kind}))
			requireBudgetError(t, testutil.Call(t, call, r), 403, "budget_forbidden")
		}
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialJWT, auth.CredentialPAT} {
		for _, scenario := range []string{"restricted", "agent_actor", "member", "missing_principal", "missing_member", "X-Agent-ID", "X-Task-ID", "X-Actor-Source"} {
			for _, call := range []http.HandlerFunc{h.ShowIssueBudget, h.EnableIssueBudget} {
				r := budgetManagementRequest(workspace, user, "owner", "POST", "/api/issue-budgets/bad", "{broken", "rootId", "bad")
				p := auth.AuthenticatedPrincipal{UserID: user, Kind: kind}
				switch scenario {
				case "restricted":
					p.Restricted = true
				case "agent_actor":
					p.AgentActor = true
				case "member":
					r = r.WithContext(middleware.SetMemberContext(r.Context(), workspace, db.Member{WorkspaceID: parseUUID(workspace), UserID: parseUUID(user), Role: "member"}))
				case "missing_principal":
					p.UserID = ""
				case "missing_member":
					r = r.WithContext(context.Background())
				default:
					r.Header.Set(scenario, "") // presence alone is deny-only
					if scenario == "X-Actor-Source" {
						r.Header.Add(scenario, "agent")
					}
				}
				r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), p))
				requireBudgetError(t, testutil.Call(t, call, r), 403, "budget_forbidden")
			}
		}
	}
	for _, path := range []string{"/api/issues/x/budget", "/api/issues/x/budget/operations/key", "/api/issue-budgets/x"} {
		for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "future"} {
			r := budgetManagementRequest(workspace, user, "owner", "GET", path, nil)
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: user, Kind: kind}))
			gate := h.RequireIssueBudgetHuman(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("task credential reached inner capability guard") }))
			requireBudgetError(t, testutil.Call(t, gate.ServeHTTP, r), 403, "budget_forbidden")
		}
	}
	// Middleware preserves the original hint even when Auth removes it.
	for _, header := range []string{"X-Agent-ID", "X-Task-ID", "X-Actor-Source"} {
		r := budgetManagementRequest(workspace, user, "owner", "GET", "/api/issue-budgets/x", nil)
		r.Header.Set(header, "untrusted")
		gate := h.RequireIssueBudgetHuman(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("stripped hint granted human access") }))
		strip := middleware.IssueBudgetNoStore(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(header)
			gate.ServeHTTP(w, r)
		}))
		requireBudgetError(t, testutil.Call(t, strip.ServeHTTP, r), 403, "budget_forbidden")
	}
	for _, path := range []string{"/api/daemon/budget/usage", "/api/issues/x/comments"} {
		gate := h.RequireIssueBudgetHuman(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
		testutil.Call(t, gate.ServeHTTP, testutil.JSONRequest("GET", path, nil)).Want(204)
	}
}

// B39/B40/B41: owner and admin exercise all eight routes; stale revision and
// membership refusals retain every budget row and leak no revision.
func TestIssueBudgetManagementOperations(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			s := newClaimBudgetScene(t)
			s.fx.Exec(t, `UPDATE member SET role=$3 WHERE workspace_id=$1 AND user_id=$2`, s.workspace, testUserID, role)
			issue := s.fx.Issue(t, "management operations")
			request := func(call http.HandlerFunc, method, path string, body any, params ...string) *testutil.Response {
				return testutil.Call(t, call, budgetManagementRequest(s.workspace, testUserID, role, method, path, body, params...))
			}
			key := uuid.NewString()
			body := map[string]any{"request_key": key, "reason": "budget test", "limit_ticks": "250", "attempt_reserve_ticks": "10"}
			response := request(testHandler.EnableIssueBudget, "POST", "/api/issues/"+issue+"/budget", body, "id", issue).Want(201)
			var enabled service.IssueBudgetWriteResponse
			response.JSON(&enabled)
			requireBudgetFixtureKeys(t, response, "write-201-enable-no-gaps")
			root := enabled.Result.BudgetRootID
			replay := request(testHandler.EnableIssueBudget, "POST", "/api/issues/"+issue+"/budget", body, "id", issue).Want(200)
			var replayed service.IssueBudgetWriteResponse
			replay.JSON(&replayed)
			if !replayed.Replayed || !reflect.DeepEqual(replayed.Result, enabled.Result) {
				t.Fatalf("original-key replay changed result: %+v", replayed)
			}
			binding := request(testHandler.GetIssueBudgetBinding, "GET", "/api/issues/"+issue+"/budget", nil, "id", issue).Want(200)
			requireBudgetFixtureKeys(t, binding, "issue-binding")
			var bound service.IssueBudgetBinding
			binding.JSON(&bound)
			if !bound.IsRootIssue || bound.BudgetRootID != root {
				t.Fatalf("binding = %+v", bound)
			}
			show := request(testHandler.ShowIssueBudget, "GET", "/api/issue-budgets/"+root, nil, "rootId", root).Want(200)
			requireBudgetFixtureKeys(t, show, "show-known")
			var shown service.IssueBudgetShow
			show.JSON(&shown)
			if !shown.TotalKnown || shown.RemainingTicks == nil || *shown.RemainingTicks != 250 {
				t.Fatalf("empty root projection = %+v", shown)
			}
			readEnable := request(testHandler.GetIssueBudgetEnableOperation, "GET", "/api/issues/"+issue+"/budget/operations/"+key, nil, "id", issue, "key", key).Want(200)
			requireBudgetFixtureKeys(t, readEnable, "operation-committed", "write-201-enable-no-gaps")
			for _, tc := range []struct {
				call            http.HandlerFunc
				suffix, fixture string
			}{
				{testHandler.ListIssueBudgetObservations, "observations", "observations-empty"},
				{testHandler.ListIssueBudgetAttempts, "attempts", "attempts-empty"},
			} {
				page := request(tc.call, "GET", "/api/issue-budgets/"+root+"/"+tc.suffix, nil, "rootId", root).Want(200)
				fixture, err := os.ReadFile("testdata/issue_budget_http_v1/" + tc.fixture + ".json")
				if err != nil || !bytes.Equal(page.Body.Bytes(), fixture) {
					t.Fatalf("empty page differs: %s %v", page.Body, err)
				}
			}
			for i, change := range []map[string]any{
				{"kind": "set_limit", "limit_ticks": "400", "attempt_reserve_ticks": nil},
				{"kind": "set_mode", "mode": "off"}, {"kind": "set_mode", "mode": "observe"},
				{"kind": "close"}, {"kind": "reopen"},
			} {
				change["request_key"], change["reason"], change["expected_policy_revision"] = uuid.NewString(), "policy change", fmt.Sprint(i+1)
				result := request(testHandler.ManageIssueBudget, "POST", "/api/issue-budgets/"+root+"/operations", change, "rootId", root).Want(201)
				var written service.IssueBudgetWriteResponse
				result.JSON(&written)
				if written.Result.PolicyRevision != budget.Count(i+2) {
					t.Fatalf("revision = %v", written.Result.PolicyRevision)
				}
				if i == 2 && written.Result.PolicyEpoch != 2 {
					t.Fatalf("restart reused epoch: %+v", written)
				}
				read := request(testHandler.GetIssueBudgetOperation, "GET", "/api/issue-budgets/"+root+"/operations/"+change["request_key"].(string), nil, "rootId", root, "key", change["request_key"].(string)).Want(200)
				requireBudgetFixtureKeys(t, read, "operation-committed-stale")
				var committed service.IssueBudgetOperationLookup
				read.JSON(&committed)
				if committed.Stale || !reflect.DeepEqual(committed.Result, written.Result) {
					t.Fatalf("stored operation disagrees: %+v", committed)
				}
			}
			before := issueBudgetRootRows(t, root)
			staleBody := map[string]any{"kind": "close", "request_key": uuid.NewString(), "reason": "stale", "expected_policy_revision": "1"}
			requireBudgetError(t, request(testHandler.ManageIssueBudget, "POST", "/api/issue-budgets/"+root+"/operations", staleBody, "rootId", root), 409, "policy_revision_conflict")
			requireIssueBudgetRootRows(t, root, before, "stale revision")
			// Simulate a demotion after workspace middleware authorized this request.
			s.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, s.workspace, testUserID)
			requireBudgetError(t, request(testHandler.EnableIssueBudget, "POST", "/api/issues/"+issue+"/budget", body, "id", issue), 403, "budget_forbidden")
			requireIssueBudgetRootRows(t, root, before, "demoted replay")
			admin := s.fx.User(t, "other admin", "budget-admin-"+uuid.NewString()+"@example.test")
			s.fx.Member(t, s.workspace, admin, "admin")
			r := budgetManagementRequest(s.workspace, admin, "admin", "GET", "/api/issues/"+issue+"/budget/operations/"+key, nil, "id", issue, "key", key)
			var recovered service.IssueBudgetOperationLookup
			testutil.Call(t, testHandler.GetIssueBudgetEnableOperation, r).Want(200).JSON(&recovered)
			if !recovered.Stale || !reflect.DeepEqual(recovered.Result, enabled.Result) {
				t.Fatalf("other admin cannot recover stale original result: %+v", recovered)
			}
		})
	}
}

func TestIssueBudgetManagementPrivacyAndParsingOrder(t *testing.T) {
	s := newClaimBudgetScene(t)
	issue := s.fx.Issue(t, "private budget root")
	root := s.enableRoot(t, issue).BudgetRootID
	other := newClaimBudgetScene(t)
	before := issueBudgetRootRows(t, root)
	for _, scope := range []string{"root", "issue"} {
		for _, lookup := range []bool{false, true} {
			id, param, call, prefix := root, "rootId", testHandler.ShowIssueBudget, "/api/issue-budgets/"
			if scope == "issue" {
				id, param, call, prefix = issue, "id", testHandler.GetIssueBudgetBinding, "/api/issues/"
			}
			if lookup && scope == "issue" {
				call = testHandler.GetIssueBudgetEnableOperation
			} else if lookup {
				call = testHandler.GetIssueBudgetOperation
			}
			var replies []string
			for _, target := range []string{id, uuid.NewString()} {
				path := prefix + target
				if scope == "issue" {
					path += "/budget"
				}
				key := uuid.NewString()
				if lookup {
					path += "/operations/" + key
				}
				r := budgetManagementRequest(other.workspace, testUserID, "owner", "GET", path, nil, param, target, "key", key)
				response := testutil.Call(t, call, r)
				requireBudgetError(t, response, 404, "not_found")
				replies = append(replies, response.Body.String())
			}
			if replies[0] != replies[1] {
				t.Fatal("foreign resource distinguished from absent resource")
			}
		}
	}
	for _, target := range []string{root, uuid.NewString()} {
		body := map[string]any{"kind": "close", "request_key": uuid.NewString(), "reason": "foreign", "expected_policy_revision": "1"}
		r := budgetManagementRequest(other.workspace, testUserID, "owner", "POST", "/api/issue-budgets/"+target+"/operations", body, "rootId", target)
		requireBudgetError(t, testutil.Call(t, testHandler.ManageIssueBudget, r), 404, "not_found")
	}
	for _, role := range []string{"member", "owner"} {
		status, code := 403, "budget_forbidden"
		if role == "owner" {
			status, code = 400, "invalid_request"
		}
		for _, body := range []string{"{bad", strings.Repeat(" ", 16<<10) + "{}"} {
			r := budgetManagementRequest(s.workspace, testUserID, role, "POST", "/api/issue-budgets/absent/operations", body, "rootId", uuid.NewString())
			requireBudgetError(t, testutil.Call(t, testHandler.ManageIssueBudget, r), status, code)
		}
	}
	r := budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issue-budgets/"+root+"/operations/UPPER", nil, "rootId", root, "key", strings.ToUpper(uuid.NewString()))
	requireBudgetError(t, testutil.Call(t, testHandler.GetIssueBudgetOperation, r), 400, "invalid_request")
	requireIssueBudgetRootRows(t, root, before, "privacy and parsing refusals")
}

func TestIssueBudgetManagementLoaderAndTombstone(t *testing.T) {
	s := newClaimBudgetScene(t)
	issue := s.fx.Issue(t, "private title must not be retained")
	root := s.enableRoot(t, issue).BudgetRootID
	key := uuid.NewString()
	var number int32
	s.fx.QueryRow(t, `SELECT number FROM issue WHERE id=$1`, issue).Scan(&number)
	prefix := testHandler.getIssuePrefix(context.Background(), parseUUID(s.workspace))
	identifier := fmt.Sprintf("%s-%d", prefix, number)
	r := budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issues/"+identifier+"/budget/operations/"+key, nil, "id", identifier, "key", key)
	response := testutil.Call(t, testHandler.GetIssueBudgetEnableOperation, r).Want(200)
	requireBudgetFixtureKeys(t, response, "operation-unknown")
	var unknown service.IssueBudgetOperationUnknown
	response.JSON(&unknown)
	if unknown.RetryPath != "/api/issues/"+issue+"/budget/operations/"+key {
		t.Fatalf("retry path is not canonical: %+v", unknown)
	}
	shared := s.fx.Issue(t, "collaboration")
	s.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
	for _, id := range []string{shared, uuid.NewString()} {
		for _, tc := range []struct {
			call   http.HandlerFunc
			method string
			enable bool
		}{
			{testHandler.EnableIssueBudget, "POST", true}, {testHandler.GetIssueBudgetBinding, "GET", false}, {testHandler.GetIssueBudgetEnableOperation, "GET", false},
		} {
			body := map[string]any{"request_key": uuid.NewString(), "reason": "enable", "limit_ticks": nil, "attempt_reserve_ticks": nil}
			r := budgetManagementRequest(s.workspace, testUserID, "owner", tc.method, "/api/issues/"+id+"/budget", body, "id", id, "key", key)
			status, code := 404, "not_found"
			if id == shared && tc.enable {
				status, code = 422, "not_eligible"
			}
			requireBudgetError(t, testutil.Call(t, tc.call, r), status, code)
		}
	}
	// The actual issue deletion seam writes the tombstone, and issue routes
	// become unavailable while root-addressed recovery remains readable.
	testutil.Call(t, testHandler.DeleteIssue, budgetManagementRequest(s.workspace, testUserID, "owner", "DELETE", "/api/issues/"+issue, nil, "id", issue)).Want(204)
	r = budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issue-budgets/"+root, nil, "rootId", root)
	response = testutil.Call(t, testHandler.ShowIssueBudget, r).Want(200)
	var shown service.IssueBudgetShow
	response.JSON(&shown)
	if shown.AdmissionState != "tombstoned" || bytes.Contains(response.Body.Bytes(), []byte("private title")) || bytes.Contains(response.Body.Bytes(), []byte(identifier)) {
		t.Fatalf("tombstone leaked source content: %s", response.Body)
	}
	for _, call := range []http.HandlerFunc{testHandler.GetIssueBudgetBinding, testHandler.GetIssueBudgetEnableOperation} {
		r := budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issues/"+issue+"/budget", nil, "id", issue, "key", key)
		requireBudgetError(t, testutil.Call(t, call, r), 404, "not_found")
	}
}

// B48/B50/B52: read the real persisted ledger with nonzero A/R/H, preserve
// unknowns after off, and leave shared project policy and old quota untouched.
func TestIssueBudgetManagementShowCoverageAndOff(t *testing.T) {
	s := newClaimBudgetScene(t)
	issue := s.fx.Issue(t, "coverage read")
	root := s.enableRoot(t, issue).BudgetRootID
	policyTables := []string{"project", "project_delivery_policy", "project_knowledge_policy", "autopilot_quota_period", "autopilot_quota_reservation"}
	policyRows := func() map[string]string {
		rows := map[string]string{}
		for _, table := range policyTables {
			var value string
			s.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY to_jsonb(p)::text),'[]'::jsonb)::text FROM `+table+` p`).Scan(&value)
			rows[table] = value
		}
		return rows
	}
	policiesBefore := policyRows()
	a, b, unsized, zero, missing := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	entries := []budget.Entry{
		{ID: "reserve:" + a, Kind: budget.KindReserve, ReservationID: a, ReservedTicks: 100},
		{ID: "reserve:" + b, Kind: budget.KindReserve, ReservationID: b, ReservedTicks: 50},
		{ID: "usage:" + a, Kind: budget.KindUsage, ReservationID: a, Usage: budget.Usage{
			InvocationID: "inv-actual", ScopeID: "scope-actual", Mode: budget.ModeDelta,
			Cost: budget.Cost{Basis: budget.CostReported, Ticks: 30, Currency: budget.CurrencyUSD},
		}},
		{ID: "hold:" + a, Kind: budget.KindHold, ReservationID: a, HoldReason: budget.HoldLostContact},
		{ID: "reserve:" + unsized, Kind: budget.KindReserve, ReservationID: unsized, Unsized: true},
		{ID: "reserve:" + zero, Kind: budget.KindReserve, ReservationID: zero},
	}
	ledger, receipts, err := budget.NewLedger().With(entries...)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range entries {
		raw, digest, err := budget.EncodeEntry(e)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := json.Marshal(receipts[i])
		if err != nil {
			t.Fatal(err)
		}
		cols := testutil.Cols{"workspace_id": s.workspace, "budget_root_id": root, "seq": i + 1,
			"entry_id": e.ID, "kind": string(e.Kind), "attempt_id": e.ReservationID,
			"entry_json": raw, "entry_digest": digest, "receipt": string(receipt), "source": "server", "schema_version": int(budget.EntryFormVersion)}
		if e.Kind == budget.KindUsage {
			cols["invocation_id"], cols["scope_id"] = e.Usage.InvocationID, e.Usage.ScopeID
		}
		s.fx.InsertNoID(t, "issue_budget_entry", cols, "budget_root_id=$1 AND seq=$2", root, i+1)
	}
	projection := ledger.Projection()
	s.fx.Exec(t, `UPDATE issue_budget_root SET ledger_revision=$2,actual_known_ticks=30,reserved_ticks=50,unknown_hold_ticks=70,unknown_count=2,limit_ticks=250 WHERE id=$1`, root, len(entries))
	for i, id := range []string{a, b, unsized, zero, missing} {
		cols := issueBudgetAttemptCols(root, uuid.NewString(), issue)
		cols["id"], cols["workspace_id"] = id, s.workspace
		cols["reserved_ticks"] = []any{100, 50, nil, 0, 5}[i]
		if i == 0 {
			cols["state"] = "lost_contact"
		}
		s.fx.Insert(t, "issue_budget_attempt", cols)
	}
	s.fx.Insert(t, "issue_budget_observation", testutil.Cols{"id": uuid.NewString(), "workspace_id": s.workspace,
		"budget_root_id": root, "kind": "coverage_gap", "reason": "observation_off", "detail": `{"previous_mode":"off"}`})
	readShow := func() (service.IssueBudgetShow, *testutil.Response) {
		r := budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issue-budgets/"+root, nil, "rootId", root)
		var out service.IssueBudgetShow
		response := testutil.Call(t, testHandler.ShowIssueBudget, r).Want(200).JSON(&out)
		return out, response
	}
	shown, response := readShow()
	requireBudgetFixtureKeys(t, response, "show-open")
	if shown.Amounts == nil || shown.Amounts.ActualKnown != 30 || shown.Amounts.Reserved != 50 || shown.Amounts.UnknownHold != 70 || shown.TotalKnown || shown.RemainingTicks != nil || *shown.Coverage.IncludedAttempts != projection.Coverage.IncludedAttempts {
		t.Fatalf("nonzero show = %+v", shown)
	}
	wantReasons := []service.IssueBudgetUnknownCount{{Reason: "lost_contact", Count: 1}, {Reason: "observation_off", Count: 1}, {Reason: "reservation_unsized", Count: 1}}
	if !reflect.DeepEqual(shown.Unknown.ByReason, wantReasons) {
		t.Fatalf("unknown reasons = %+v", shown.Unknown)
	}
	r := budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issue-budgets/"+root+"/attempts", nil, "rootId", root)
	var attempts service.IssueBudgetAttemptPage
	page := testutil.Call(t, testHandler.ListIssueBudgetAttempts, r).Want(200).JSON(&attempts)
	requireBudgetFixtureKeys(t, page, "attempts-page")
	if len(attempts.Attempts) != 5 {
		t.Fatalf("attempt count = %d", len(attempts.Attempts))
	}
	for _, row := range attempts.Attempts {
		want, ok := ledger.Reservation(row.AttemptID)
		if ok && (row.Reservation == nil || *row.Reservation != want) || !ok && row.Reservation != nil {
			t.Fatalf("reservation differs from persisted ledger: %+v", row)
		}
		if row.AttemptID == unsized && row.ReservedTicks != nil || row.AttemptID == zero && (row.ReservedTicks == nil || *row.ReservedTicks != 0) {
			t.Fatalf("unsized and zero conflated: %+v", row)
		}
	}
	r = budgetManagementRequest(s.workspace, testUserID, "owner", "GET", "/api/issue-budgets/"+root+"/observations", nil, "rootId", root)
	observations := testutil.Call(t, testHandler.ListIssueBudgetObservations, r).Want(200)
	// detail is caller-opaque and has varying keys; freeze the envelope and
	// row shape separately from the byte-preserving JSON object below.
	var observationPage service.IssueBudgetObservationPage
	observations.JSON(&observationPage)
	if len(observationPage.Observations) != 1 || observationPage.Observations[0].Covered || string(observationPage.Observations[0].Detail) != `{"previous_mode":"off"}` {
		t.Fatalf("observation read = %+v", observationPage)
	}
	change := map[string]any{"kind": "set_mode", "mode": "off", "request_key": uuid.NewString(), "reason": "pause observation", "expected_policy_revision": "1"}
	r = budgetManagementRequest(s.workspace, testUserID, "owner", "POST", "/api/issue-budgets/"+root+"/operations", change, "rootId", root)
	testutil.Call(t, testHandler.ManageIssueBudget, r).Want(201)
	off, _ := readShow()
	if off.Mode != "off" || !reflect.DeepEqual(off.Amounts, shown.Amounts) || !reflect.DeepEqual(off.Unknown, shown.Unknown) {
		t.Fatalf("off erased amounts/unknowns: %+v", off)
	}
	if !reflect.DeepEqual(policyRows(), policiesBefore) {
		t.Fatal("budget management changed shared project policies or old quota")
	}
}

// B42's new-reservation half: an ordinary capability check that refuses a
// previously prepared claim must run before any attempt or ledger write.
func TestIssueBudgetManagementRevokedInvokeCannotReserve(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	root := enableIssueBudget(t, testWorkspaceID, f.issue)
	ctx := context.Background()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	token, err := testHandler.Queries.GetTaskTokenByHash(ctx, auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	prepared := db.CreateTaskTokenParams{
		TokenHash: auth.HashToken("mat_budget_recheck_" + uuid.NewString()), TaskID: task.ID, AgentID: task.AgentID,
		WorkspaceID: token.WorkspaceID, UserID: token.UserID, ExpiresAt: token.ExpiresAt, ApiCapability: token.ApiCapability,
	}
	before := issueBudgetRootRows(t, root)
	tokensBefore := dbfx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, f.task)
	other := dbfx.User(t, "New private owner", "new-budget-owner-"+uuid.NewString()+"@example.test")
	dbfx.Exec(t, `UPDATE agent SET owner_id=$2,permission_mode='private' WHERE id=$1`, f.agent, other)
	dbfx.Cleanup(t, `UPDATE agent SET owner_id=$2 WHERE id=$1`, f.agent, testUserID)
	_, err = testHandler.TaskService.FinalizeTaskClaim(ctx, task, prepared, nil, false, nil, service.TaskClaimBudgetProtocolV1)
	if !errors.Is(err, auth.ErrTaskCapability) {
		t.Fatalf("revoked invocation did not fail capability check: %v", err)
	}
	requireIssueBudgetRootRows(t, root, before, "refused finalization")
	if got := dbfx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, f.task); got != tokensBefore {
		t.Fatalf("refused finalization changed token count %d -> %d", tokensBefore, got)
	}
}

func TestIssueBudgetManagementWorkspaceMiddleware(t *testing.T) {
	s := newClaimBudgetScene(t)
	root := s.enableRoot(t, s.fx.Issue(t, "membership boundary")).BudgetRootID
	user := s.fx.User(t, "Not a member", "budget-outsider-"+uuid.NewString()+"@example.test")
	before := issueBudgetRootRows(t, root)
	for _, path := range []string{"/api/issue-budgets/" + root, "/api/issue-budgets/" + uuid.NewString(), "/api/issue-budgets/" + root + "/operations/" + uuid.NewString()} {
		r := testutil.WithHeaders(testutil.JSONRequest("GET", path, nil), "X-User-ID", user, "X-Workspace-ID", s.workspace)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT}))
		gate := testHandler.RequireIssueBudgetWorkspace(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("nonmember reached budget handler") }))
		requireBudgetError(t, testutil.Call(t, gate.ServeHTTP, r), 403, "budget_forbidden")
	}
	// The same middleware preserves existing behavior on every other route.
	gate := testHandler.RequireIssueBudgetWorkspace(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("nonmember reached ordinary route") }))
	r := testutil.WithHeaders(testutil.JSONRequest("GET", "/api/issues", nil), "X-User-ID", user, "X-Workspace-ID", s.workspace)
	ordinary := testutil.Call(t, gate.ServeHTTP, r).Want(404)
	if ordinary.Map()["code"] != "workspace_access_denied" {
		t.Fatalf("ordinary workspace middleware behavior changed: %s", ordinary.Body)
	}
	requireIssueBudgetRootRows(t, root, before, "nonmember middleware")
}
