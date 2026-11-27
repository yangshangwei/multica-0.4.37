package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestMaterialReviewSyncAbsenceRecovery(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	f.step(t)
	f.approve(t, f.detail(t, d.Material.ID))
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	first, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, first.TaskID)
	if f.active.Action != "review_sync" {
		t.Fatal("expected note operation")
	}
	f.finish(t) // Runtime stopped before writing the note.
	retry := func() {
		t.Helper()
		current := f.detail(t, d.Material.ID)
		if _, err := f.svc.Command(t.Context(), f.access, d.Material.ID, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: current.Material.Version}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ProcessNext(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	retry()
	op, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, op.TaskID)
	e := *f.active
	if e.Action != "observe" || e.ReconcileAction != "review_sync" {
		t.Fatal("recovery must only observe")
	}
	r, err := f.git.Observe(t.Context(), e.ReviewNumber)
	if err != nil {
		t.Fatal(err)
	}
	o := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: e.OperationID, TaskID: e.TaskID, RuntimeID: e.RuntimeID, ClaimGeneration: f.claim.ClaimGeneration, ExpectedDigest: e.ExpectedDigest, ObservationID: uuid.NewString(), HostChallenge: f.claim.HostChallenge, ObservedAt: time.Now(), Outcome: "not_found", SourceComplete: true, RepositoryIdentity: e.Provider + ":" + e.InstanceURL + ":" + e.Repository, Review: &protocol.MaterialReview{Number: r.Number, URL: r.URL, SourceBranch: r.Branch, BaseBranch: r.BaseBranch, HeadSHA: r.HeadSHA, State: r.State}}
	for _, c := range r.Changes {
		o.Review.Changes = append(o.Review.Changes, protocol.MaterialChange{Path: c.Path, Mode: c.Mode, Status: c.Status})
	}
	for _, v := range e.Files {
		v.Commit = r.HeadSHA
		o.Files = append(o.Files, v)
	}
	invalid := o
	invalid.SourceComplete = false
	if f.svc.ObserveOperation(t.Context(), f.task.ID, invalid) == nil {
		t.Fatal("incomplete absence accepted")
	}
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
	if got := f.detail(t, d.Material.ID); got.Baseline != nil || got.Operation.Status != "no_effect" {
		t.Fatal("missing note promoted material")
	}
	retry()
	op, err = f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	next, err := materialParse[protocol.MaterialExecution](op.Descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "review_sync" || next.DecisionID != e.DecisionID || next.OperationKey != e.OperationKey {
		t.Fatal("retry changed original decision or skipped note")
	}
}

func TestMaterialPlanTaskRejectsSameCarrierReconfiguration(t *testing.T) {
	f := newMaterialFixture(t, true)
	p := f.queuedPlan(t)
	f.admit(t, p.TaskID)
	if err := f.observe(t); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Tasks.StartTask(t.Context(), p.TaskID); err != nil {
		t.Fatal(err)
	}
	task, err := f.svc.Queries.GetAgentTask(t.Context(), p.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeMaterialPlanTask(t.Context(), f.svc.Queries, task); err != nil {
		t.Fatal(err)
	}
	f.settings.RequestID, f.settings.ExpectedVersion = uuid.NewString(), 2
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeMaterialPlanTask(t.Context(), f.svc.Queries, task); materialCode(err) != "material_plan_context_changed" {
		t.Fatalf("old plan silently adopted new configuration: %v", err)
	}
}

func TestMaterialDisableWithoutRuntimePreservesIntent(t *testing.T) {
	f := newMaterialFixture(t, false)
	f.settings.RequestID, f.settings.ExpectedVersion = uuid.NewString(), 1
	f.settings.Enabled = false
	f.settings.ExecutionRuntimeID = nil
	out, err := f.svc.Configure(t.Context(), f.access, f.settings)
	if err != nil {
		t.Fatalf("disabling must not require replacement runtime: %v", err)
	}
	if out.Settings.Enabled || out.Settings.ExecutionRuntimeID != nil {
		t.Fatal("disabled configuration was rebound")
	}
	replayed, err := f.svc.Configure(t.Context(), f.access, f.settings)
	if err != nil || material.JSONDigest(replayed) != material.JSONDigest(out) {
		t.Fatalf("disabled request replay changed: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id=$1)`, f.ws) != 0 {
		t.Fatal("disable dispatched work")
	}
	f.settings.RequestID, f.settings.ExpectedVersion = uuid.NewString(), out.Settings.Version
	f.settings.Enabled = true
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err == nil {
		t.Fatal("re-enabled without explicit runtime")
	}
	badID := "invalid-runtime"
	f.settings.Enabled, f.settings.ExecutionRuntimeID = false, &badID
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); materialCode(err) != "invalid_material_id" {
		t.Fatalf("disabled configuration persisted malformed runtime: %v", err)
	}
}

func TestMaterialDisableDeletedRuntime(t *testing.T) {
	f := newMaterialFixture(t, false)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	q := f.svc.Queries.WithTx(tx)
	if _, err = TeardownRuntime(t.Context(), q, util.MustParseUUID(f.runtime), RuntimeTeardownOptions{CancelNonTerminalTasks: true}); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteAgentRuntime(t.Context(), util.MustParseUUID(f.runtime)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.Enabled = uuid.NewString(), 1, false
	out, err := f.svc.Configure(t.Context(), f.access, f.settings)
	if err != nil || out.Settings.Enabled {
		t.Fatalf("cannot disable removed runtime: %v", err)
	}
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.Enabled = uuid.NewString(), 2, true
	if _, err = f.svc.Configure(t.Context(), f.access, f.settings); materialCode(err) != "material_runtime_unavailable" {
		t.Fatalf("missing runtime reused: %v", err)
	}
}

func TestMaterialPlanTaskCanReadAfterSubmission(t *testing.T) {
	f := newMaterialFixture(t, true)
	p := f.queuedPlan(t)
	f.admit(t, p.TaskID)
	if err := f.observe(t); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Tasks.StartTask(t.Context(), p.TaskID); err != nil {
		t.Fatal(err)
	}
	d := f.detail(t, util.UUIDToString(p.MaterialID))
	planContent := strings.Repeat("Exact implementation task breakdown.\n", 4)
	_, err := f.svc.Revise(t.Context(), f.access, d.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: d.Material.Version, Kind: "plan", Content: strings.Repeat("Complete technical design and constraints.\n", 4), PlanContent: &planContent, SourceBaselineID: &d.Baseline.ID, Sources: d.Revisions[0].Sources, AcceptanceCriteria: []string{"AC1: Preserve original acceptance"}, Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	task, err := f.svc.Queries.GetAgentTask(t.Context(), p.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err = AuthorizeMaterialPlanTask(t.Context(), f.svc.Queries, task); err != nil {
		t.Fatalf("submission lost its original authorized read/replay path: %v", err)
	}
}

func TestMaterialRecoveredReviewSyncContinuesToMerge(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	f.step(t)
	f.approve(t, f.detail(t, d.Material.ID))
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	op, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, op.TaskID)
	if err = f.git.SyncDecision(f.active.ReviewNumber, *f.active.AuditNote); err != nil {
		t.Fatal(err)
	}
	f.finish(t) // Remote note exists, but no observation reached the server.
	d = f.detail(t, d.Material.ID)
	if _, err = f.svc.Command(t.Context(), f.access, d.Material.ID, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: d.Material.Version}); err != nil {
		t.Fatal(err)
	}
	f.step(t) // Independent recovery confirms the original note.
	if f.lastErr != nil {
		t.Fatal(f.lastErr)
	}
	f.step(t) // Next eligible scheduler tick dispatches the approved merge.
	if f.lastErr != nil {
		t.Fatal(f.lastErr)
	}
	op, err = f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil || op.Action != "merge" {
		t.Fatalf("recovered review did not advance to approved merge: action=%s %v", op.Action, err)
	}
	if got := f.detail(t, d.Material.ID); got.Baseline == nil {
		t.Fatal("approved merge after review recovery did not establish baseline")
	}
}
