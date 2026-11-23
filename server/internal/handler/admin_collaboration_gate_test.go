package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type adminGateResponse struct {
	Scope                string   `json:"scope"`
	CreationEnabled      bool     `json:"creation_enabled"`
	ExecutionEnabled     bool     `json:"execution_enabled"`
	HostExecutionEnabled bool     `json:"host_execution_enabled"`
	ExecutionEpoch       string   `json:"execution_epoch"`
	AllowedActions       []string `json:"allowed_actions"`
}

type adminGateChange struct {
	Operation map[string]any    `json:"operation"`
	Target    adminGateResponse `json:"target"`
}

// closedCollaborationGate starts from the shipped default and restores the
// shared singleton row, which other DB-backed suites also read.
func closedCollaborationGate(t *testing.T) {
	t.Helper()
	var enabled, creation, host bool
	var epoch int64
	dbfx.QueryRow(t, `SELECT enabled, creation_enabled, host_execution_enabled, epoch FROM collaboration_execution_gate WHERE id=1`).Scan(&enabled, &creation, &host, &epoch)
	dbfx.Cleanup(t, `UPDATE collaboration_execution_gate SET enabled=$1, creation_enabled=$2, host_execution_enabled=$3, epoch=$4, freeze_operation_id=NULL WHERE id=1`, enabled, creation, host, epoch)
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false, creation_enabled=false, freeze_operation_id=NULL WHERE id=1`)
}

func adminGateRouter() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/collaboration/gate", testHandler.AdminCollaborationGate)
	router.Post("/api/admin/collaboration/gate", testHandler.AdminChangeCollaborationGate)
	router.Get("/api/admin/operations", testHandler.AdminOperations)
	router.Get("/api/admin/audit", testHandler.AdminAudit)
	return router
}

func gateEpoch(t *testing.T) string {
	t.Helper()
	var epoch string
	dbfx.QueryRow(t, `SELECT epoch::text FROM collaboration_execution_gate WHERE id=1`).Scan(&epoch)
	return epoch
}

func TestAdminCollaborationGateOpensCreationAndExecutionWithReceipts(t *testing.T) {
	login := adminHandlerSetup(t)
	closedCollaborationGate(t)
	router := adminGateRouter()
	var gate adminGateResponse
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/collaboration/gate", nil, "").Want(200).JSON(&gate)
	if gate.CreationEnabled || gate.ExecutionEnabled || strings.Join(gate.AllowedActions, ",") != "open_creation,open_execution" {
		t.Fatalf("closed gate view: %+v", gate)
	}

	open := map[string]any{"action": "open_creation", "reason": "Pilot cross-workspace collaboration"}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_creation"}, uuid.NewString()).Want(400)
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", open, "").Want(400)
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "close_execution", "reason": "No freeze exists"}, uuid.NewString()).Want(400)

	key := uuid.NewString()
	var first adminGateChange
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", open, key).Want(200).JSON(&first)
	if !first.Target.CreationEnabled || first.Target.ExecutionEnabled || first.Operation["state"] != "succeeded" || first.Operation["result_code"] != "collaboration_creation_opened" {
		t.Fatalf("open creation: %+v", first)
	}
	epoch := gateEpoch(t)
	if first.Target.ExecutionEpoch != epoch {
		t.Fatalf("creation response epoch %s, row %s", first.Target.ExecutionEpoch, epoch)
	}
	var replay adminGateChange
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", open, key).Want(200).JSON(&replay)
	if replay.Operation["id"] != first.Operation["id"] {
		t.Fatal("repeat request created another operation")
	}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "close_creation", "reason": "Different request"}, key).Want(409)

	// A stale open is refused, and the refusal is the original key's receipt.
	conflictKey := uuid.NewString()
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", open, conflictKey).Want(409)
	var found struct {
		Items []map[string]any `json:"items"`
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/operations?idempotency_key="+conflictKey, nil, "").Want(200).JSON(&found)
	if len(found.Items) != 1 || found.Items[0]["state"] != "failed" || found.Items[0]["result_code"] != "collaboration_gate_conflict" {
		t.Fatalf("conflict lacks original-key receipt: %+v", found.Items)
	}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", open, conflictKey).Want(200).JSON(&replay)
	if replay.Operation["state"] != "failed" {
		t.Fatal("replayed refusal did not stay failed")
	}

	// Opening execution names the epoch the administrator saw; creation never does.
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "No epoch"}, uuid.NewString()).Want(400)
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "close_creation", "reason": "Epoch is unrelated", "expected_execution_epoch": epoch}, uuid.NewString()).Want(400)
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "Bad epoch", "expected_execution_epoch": "07"}, uuid.NewString()).Want(400)
	// A close made after the request was prepared raises the epoch, so the old
	// open is refused with a receipt instead of reopening execution.
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET epoch=epoch+1 WHERE id=1`)
	staleKey := uuid.NewString()
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "Prepared before a close", "expected_execution_epoch": epoch}, staleKey).Want(409)
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/operations?idempotency_key="+staleKey, nil, "").Want(200).JSON(&found)
	if len(found.Items) != 1 || found.Items[0]["result_code"] != "collaboration_gate_conflict" || dbfx.Count(t, `SELECT count(*) FROM collaboration_execution_gate WHERE enabled`) != 0 {
		t.Fatalf("stale open reopened execution or left no receipt: %+v", found.Items)
	}
	epoch = gateEpoch(t)
	paused := collaborationBoundaryIssue(t, testWorkspaceID)
	queues := func() string {
		var states string
		dbfx.QueryRow(t, `SELECT string_agg(queue_state, ',' ORDER BY queue_state) FROM collaboration_session WHERE issue_id=$1`, paused).Scan(&states)
		return states
	}
	before := queues()
	var execution adminGateChange
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "Pilot agents", "expected_execution_epoch": epoch}, uuid.NewString()).Want(200).JSON(&execution)
	if after := queues(); after != before || after == "active" {
		t.Fatalf("opening execution changed a queue: %s -> %s", before, after)
	}
	if !execution.Target.ExecutionEnabled || !execution.Target.CreationEnabled || execution.Target.ExecutionEpoch == epoch || gateEpoch(t) != execution.Target.ExecutionEpoch {
		t.Fatalf("opening execution must raise the epoch: before %s, %+v", epoch, execution.Target)
	}
	if strings.Join(execution.Target.AllowedActions, ",") != "close_creation" {
		t.Fatalf("closing execution must not be offered: %+v", execution.Target.AllowedActions)
	}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "Again", "expected_execution_epoch": execution.Target.ExecutionEpoch}, uuid.NewString()).Want(409)

	epoch = gateEpoch(t)
	var closed adminGateChange
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "close_creation", "reason": "Pilot finished"}, uuid.NewString()).Want(200).JSON(&closed)
	if closed.Target.CreationEnabled || !closed.Target.ExecutionEnabled || gateEpoch(t) != epoch {
		t.Fatalf("closing creation must leave execution and its epoch alone: %+v", closed.Target)
	}

	if n := dbfx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE operation_id=$1`, first.Operation["id"]); n != 3 {
		t.Fatalf("successful change audit phases=%d", n)
	}
	var audit struct {
		Items []map[string]any `json:"items"`
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/audit?target_kind=collaboration_gate&phase=succeeded&actor_user_id="+login.User.ID, nil, "").Want(200).JSON(&audit)
	if len(audit.Items) != 3 {
		t.Fatalf("succeeded gate events=%d", len(audit.Items))
	}
	for _, item := range audit.Items {
		if item["target_id"] != gate.Scope || item["action"] != "collaboration.gate" {
			t.Fatalf("gate event target: %+v", item)
		}
		if item["operation_id"] == first.Operation["id"] {
			before, after := item["before_state"].(map[string]any), item["after_state"].(map[string]any)
			if before["creation_enabled"] != false || after["creation_enabled"] != true || after["execution_epoch"] != first.Target.ExecutionEpoch {
				t.Fatalf("gate snapshot projection: before=%v after=%v", before, after)
			}
		}
	}
}

func TestAdminCollaborationGateObserverAndFreezeBoundaries(t *testing.T) {
	login := adminHandlerSetup(t)
	closedCollaborationGate(t)
	router := adminGateRouter()
	observer := passwordRegister(t, "gateobserver"+strings.ReplaceAll(uuid.NewString(), "-", "")[:10])
	dbfx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": observer.User.ID, "role": "platform_observer"}, "user_id=$1", observer.User.ID)
	var gate adminGateResponse
	alertHTTPCall(t, router, observer.Token, "GET", "/api/admin/collaboration/gate", nil, "").Want(200).JSON(&gate)
	if len(gate.AllowedActions) != 0 {
		t.Fatalf("observer received controls: %+v", gate.AllowedActions)
	}
	alertHTTPCall(t, router, observer.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_creation", "reason": "Observer attempt"}, uuid.NewString()).Want(403)
	ordinary := passwordRegister(t, "gateordinary"+strings.ReplaceAll(uuid.NewString(), "-", "")[:10])
	alertHTTPCall(t, router, ordinary.Token, "GET", "/api/admin/collaboration/gate", nil, "").Want(403)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_execution_gate WHERE creation_enabled`) != 0 {
		t.Fatal("refused request opened creation")
	}

	// A recorded freeze keeps execution closed until its scan completes.
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET freeze_operation_id=$1 WHERE id=1`, uuid.NewString())
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/collaboration/gate", nil, "").Want(200).JSON(&gate)
	if strings.Join(gate.AllowedActions, ",") != "open_creation" {
		t.Fatalf("frozen gate offered execution: %+v", gate.AllowedActions)
	}
	alertHTTPCall(t, router, login.Token, "POST", "/api/admin/collaboration/gate", map[string]any{"action": "open_execution", "reason": "During freeze", "expected_execution_epoch": gate.ExecutionEpoch}, uuid.NewString()).Want(409)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_execution_gate WHERE enabled`) != 0 {
		t.Fatal("execution opened during a freeze")
	}
	alertHTTPCall(t, router, login.Token, "GET", "/api/admin/collaboration/gate?x=1", nil, "").Want(400)
}
