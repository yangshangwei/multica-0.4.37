package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

// Query-plan evidence for the frozen keyset contract. Rows are confined to
// this fixture workspace; no provider, issue execution or real agent is used.
func TestIssueBudgetReadQueryPlans(t *testing.T) {
	f := newIssueBudgetFixture(t)
	root := f.ledgerRoot(t)
	f.fx.Exec(t, `INSERT INTO issue_budget_attempt
		(id,workspace_id,budget_root_id,task_id,claim_generation,runtime_id,agent_id,issue_id,created_at)
		SELECT md5($1::text||':attempt:'||g)::uuid,$1::uuid,
		CASE WHEN g<=20000 THEN $2::uuid ELSE md5($1::text||':root:'||((g-20001)/50))::uuid END,
		md5($1::text||':task:'||g)::uuid,1,md5($1::text||':runtime')::uuid,md5($1::text||':agent')::uuid,
		md5($1::text||':issue:'||g)::uuid,'2026-11-19T00:00:00Z'::timestamptz+g*interval '1 microsecond'
		FROM generate_series(1,30000) g`, f.workspace, util.UUIDToString(root.id))
	f.fx.Exec(t, `INSERT INTO issue_budget_observation
		(id,workspace_id,budget_root_id,kind,reason,task_id,claim_generation,created_at)
		SELECT md5($1::text||':observation:'||g)::uuid,$1::uuid,
		CASE WHEN g<=20000 THEN $2::uuid ELSE md5($1::text||':root:'||((g-20001)/50))::uuid END,
		'coverage_gap','in_flight_at_bind',md5($1::text||':task:'||g)::uuid,1,
		'2026-11-19T00:00:00Z'::timestamptz+g*interval '1 microsecond'
		FROM generate_series(1,30000) g`, f.workspace, util.UUIDToString(root.id))
	f.fx.Exec(t, "ANALYZE issue_budget_attempt")
	f.fx.Exec(t, "ANALYZE issue_budget_observation")
	queries, err := os.ReadFile("../../pkg/db/queries/issue_budget_read.sql")
	if err != nil {
		t.Fatal(err)
	}
	query := func(name string) string {
		_, tail, ok := strings.Cut(string(queries), "-- name: "+name+" :many\n")
		if !ok {
			t.Fatal("page query missing")
		}
		tail, _, _ = strings.Cut(tail, "-- name:")
		tail = strings.TrimSuffix(strings.TrimSpace(tail), ";")
		return strings.NewReplacer("sqlc.embed(o)", "o.*", "@workspace_id", "$1", "@budget_root_id", "$2",
			"@before_created_at", "$3", "@before_id", "$4", "@page_size", "$5").Replace(tail)
	}
	for _, name := range []string{"ListIssueBudgetAttemptPage", "ListIssueBudgetObservationPage"} {
		for _, middle := range []bool{false, true} {
			before := pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}
			if middle {
				before = pgtype.Timestamptz{Time: time.Date(2026, 11, 19, 0, 0, 0, 10000000, time.UTC), Valid: true}
			}
			var plan []byte
			if err := f.pool.QueryRow(context.Background(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query(name), f.ws, root.id, before, pgtype.UUID{Valid: true}, int32(51)).Scan(&plan); err != nil {
				t.Fatal(err)
			}
			logIssueBudgetPlan(t, name, middle, plan)
		}
	}
	var plan []byte
	if err := f.pool.QueryRow(context.Background(), `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)
		SELECT * FROM issue_budget_operation WHERE workspace_id=$1 AND scope_kind='issue' AND scope_id=$2 AND operation_key=$3`,
		f.ws, root.row(t).RootIssueID, "unused-original-key").Scan(&plan); err != nil {
		t.Fatal(err)
	}
	logIssueBudgetPlan(t, "GetIssueBudgetOperationByKey", false, plan)
}

func logIssueBudgetPlan(t *testing.T, name string, middle bool, data []byte) {
	t.Helper()
	var plan []map[string]any
	if err := json.Unmarshal(data, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("invalid explain: %s (%v)", data, err)
	}
	var nodes []map[string]any
	var visit func(map[string]any)
	visit = func(p map[string]any) {
		n := map[string]any{}
		for _, key := range []string{"Node Type", "Index Name", "Actual Rows", "Actual Loops", "Rows Removed by Filter", "Shared Hit Blocks", "Shared Read Blocks", "Sort Method"} {
			if v, ok := p[key]; ok {
				n[key] = v
			}
		}
		nodes = append(nodes, n)
		if children, ok := p["Plans"].([]any); ok {
			for _, child := range children {
				visit(child.(map[string]any))
			}
		}
	}
	visit(plan[0]["Plan"].(map[string]any))
	compact, err := json.Marshal(nodes)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s middle=%v execution_ms=%v nodes=%s", name, middle, plan[0]["Execution Time"], compact)
}
