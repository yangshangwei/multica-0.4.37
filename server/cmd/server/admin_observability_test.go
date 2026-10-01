package main

import (
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminDetectorHealthDoesNotHideFirstCycleFailure(t *testing.T) {
	f := newManagedRouterFixture(t)
	f.fx.Exec(t, "CREATE TABLE admin_alert_detector_state (LIKE public.admin_alert_detector_state INCLUDING ALL)")
	q := db.New(f.pool)
	org, err := q.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f.fx.InsertNoID(t, "admin_alert_detector_state", testutil.Cols{"organization_id": org.ID, "rule": "queue_timeout", "source_state": "unavailable", "last_started_at": now, "last_error_code": "source_unavailable"}, "organization_id=$1 AND rule='queue_timeout'", org.ID)
	svc := service.NewAdminAlertService(q, f.pool, "00000000-0000-4000-8000-000000000009", nil)
	actual := adminDetectorSource(t.Context(), svc)
	if actual.State != "unavailable" {
		t.Fatalf("first cycle failure hidden by another unobserved rule: %+v", actual)
	}
}

func TestAdminWorkerObservationFreshness(t *testing.T) {
	var state adminWorkerObservation
	if got := state.snapshot("task_coordinator", time.Now(), time.Minute); got.State != "unknown" || got.CheckedAt != nil {
		t.Fatal("unstarted worker appeared healthy")
	}
	state.record(nil)
	if got := state.snapshot("task_coordinator", time.Now(), time.Minute); got.State != "healthy" || got.CheckedAt == nil {
		t.Fatal("completed worker cycle not observed")
	}
	if got := state.snapshot("task_coordinator", time.Now().Add(2*time.Minute), time.Minute); got.State != "stale" {
		t.Fatal("stopped progress appeared healthy")
	}
}
