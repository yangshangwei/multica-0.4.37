package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTriageIterationCapabilityUsesSettingsTransaction(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	triageEnableForTest(t)
	config := testPool.Config()
	config.MaxConns, config.MinConns = 1, 0
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h.Queries, h.DB, h.TxStarter = db.New(pool), pool, pool
	var revision int64
	dbfx.QueryRow(t, "SELECT revision FROM workspace_triage_settings WHERE workspace_id=$1", testWorkspaceID).Scan(&revision)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	request := newRequest("PUT", "/api/triage/settings", map[string]any{
		"enabled": true, "acceptance_status": "todo", "require_priority": true,
		"responsibility_mode": "none", "expected_revision": revision,
	}).WithContext(ctx)
	var updated TriageSettings
	testutil.Call(t, h.UpdateTriageSettings, request).Want(200).JSON(&updated)
	if !updated.IterationAssignment || !updated.RequirePriority || updated.Revision != revision+1 {
		t.Fatalf("settings or capability did not commit together: %+v", updated)
	}
	var fetched TriageSettings
	testutil.Call(t, h.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil)).Want(200).JSON(&fetched)
	if !fetched.IterationAssignment || fetched.Revision != updated.Revision || !fetched.RequirePriority {
		t.Fatalf("subsequent read disagrees with saved settings: %+v", fetched)
	}
}
