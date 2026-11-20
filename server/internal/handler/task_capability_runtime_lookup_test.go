package handler

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTaskCapabilityFinalizationRecordsRuntimeLookup(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	ctx := t.Context()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	token, err := testHandler.Queries.GetTaskTokenByHash(ctx, auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	m := obsmetrics.NewBusinessMetrics()
	previous := testHandler.TaskService.Metrics
	testHandler.TaskService.Metrics = m
	t.Cleanup(func() { testHandler.TaskService.Metrics = previous })

	params := db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_runtime_lookup_finalization"), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: token.WorkspaceID, UserID: token.UserID, ExpiresAt: token.ExpiresAt, ApiCapability: token.ApiCapability}
	dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.issue)
	if _, err := testHandler.TaskService.FinalizeTaskClaim(ctx, task, params, nil, false, nil, service.TaskClaimBudgetProtocolNone); !errors.Is(err, auth.ErrTaskCapability) {
		t.Fatalf("changed project must refuse finalization: %v", err)
	}
	// The initial read and binding recheck each count, as does the capability read.
	key := obsmetrics.RuntimeLookupSourceTask + "/" + obsmetrics.RuntimeLookupResultOK
	if got := lookupSnapshot(t, m)[key]; got != 3 {
		t.Fatalf("claim finalization runtime reads = %v, want 3", got)
	}
}

// Authorization must read the same runtime snapshot as the token and resource,
// even if the current runtime owner changes after that snapshot was established.
func TestTaskCapabilityRuntimeLookupKeepsReadSnapshot(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	m := withTestMetrics(t)
	ctx := t.Context()
	token, err := testHandler.Queries.GetTaskTokenByHash(ctx, auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := testPool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	scoped := *testHandler
	scoped.Queries = db.New(tx)
	scoped.DB = tx
	if _, err = scoped.Queries.GetTaskTokenByHash(ctx, token.TokenHash); err != nil {
		t.Fatal(err)
	}

	otherOwner := dbfx.User(t, "Changed runtime owner", "capability-runtime-owner@multica.ai")
	dbfx.Exec(t, `UPDATE agent_runtime SET owner_id=$1 WHERE id=$2`, otherOwner, f.runtime)
	// Restore before the separately registered user fixture is removed.
	t.Cleanup(func() { dbfx.Exec(t, `UPDATE agent_runtime SET owner_id=$1 WHERE id=$2`, testUserID, f.runtime) })

	if _, err = scoped.validateTaskCapability(ctx, token, false); err != nil {
		t.Fatalf("runtime read escaped authorized snapshot: %v", err)
	}
	if _, err = testHandler.validateTaskCapability(ctx, token, false); !errors.Is(err, auth.ErrTaskCapability) {
		t.Fatalf("current runtime owner change must revoke authority: %v", err)
	}
	key := obsmetrics.RuntimeLookupSourceTask + "/" + obsmetrics.RuntimeLookupResultOK
	if got := lookupSnapshot(t, m)[key]; got != 2 {
		t.Fatalf("snapshot and current authorization runtime reads = %v, want 2", got)
	}
}
