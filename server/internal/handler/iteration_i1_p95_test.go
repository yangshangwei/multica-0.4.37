package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Run separately from correctness tests on an exclusive migrated database.
// The first sample is reported separately; it is not a claim of cold disk cache.
// Each subsequent cycle exercises real start/close writes, never request replay.
func TestIterationI1P95(t *testing.T) {
	if os.Getenv("MULTICA_RUN_I1_P95") != "1" {
		t.Skip("opt-in: MULTICA_RUN_I1_P95=1 with an exclusive DATABASE_URL")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Fatal("explicit exclusive DATABASE_URL required")
	}
	h := lifecycleHTTPHandler(t)
	tracer := &iterationCapacityTracer{}
	application := "i1-p95-" + uuid.NewString()
	measured, _ := iterationCapacityHandler(t, application, tracer)
	measured.FeatureFlags = h.FeatureFlags
	h = measured
	observer, err := testPool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Release()
	observeCtx, stopObserving := context.WithCancel(t.Context())
	defer stopObserving()
	waits := make(chan iterationCapacityWaits, 1)
	ready := make(chan struct{})
	go func() { waits <- sampleIterationCapacityWaits(observeCtx, observer.Conn(), application, ready) }()
	<-ready
	dbfx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", testWorkspaceID)
	var postgres string
	dbfx.QueryRow(t, "SELECT version()").Scan(&postgres)
	t.Logf("environment go=%s os=%s arch=%s cpus=%d postgres=%s", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), postgres)
	const count = 1000
	const samples, warmup = 30, 5
	measurements := map[string][]float64{"detail": {}, "preview": {}, "closure": {}}
	first := lifecycleHTTPCreate(t, h, "P95 source")
	projects := make([]string, 10)
	for n := range projects {
		projects[n] = dbfx.Project(t, fmt.Sprintf("P95 project %d", n))
	}
	labels := make([]string, 6)
	for n := range labels {
		labels[n] = dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": fmt.Sprintf("P95 label %d", n), "resource_type": "issue", "color": "#123456"})
	}
	priorities := []string{"urgent", "high", "medium", "low", "none"}
	ids := make([]string, 0, count)
	moves := make([]iteration.Move, 0, count)
	for n := range count {
		id := dbfx.Issue(t, fmt.Sprintf("P95 work %04d", n), testutil.Cols{"project_id": projects[n%len(projects)], "assignee_type": "member", "assignee_id": testUserID, "priority": priorities[n%len(priorities)]})
		for j := range 3 {
			label := labels[(n+j)%len(labels)]
			dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": id, "label_id": label}, "issue_id=$1 AND label_id=$2", id, label)
		}
		ids = append(ids, id)
		moves = append(moves, iteration.Move{IssueID: id, ExpectedIssueRevision: 1, TargetID: &first})
	}
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Moves: moves})
	for name, statement := range map[string]string{
		"members": "SELECT id FROM issue WHERE workspace_id=$1 AND current_iteration_id=$2 ORDER BY id",
		"events":  "SELECT * FROM iteration_event WHERE workspace_id=$1 AND iteration_id=$2 ORDER BY sequence",
		"labels":  "SELECT x.issue_id,l.id,l.name FROM issue_to_label x JOIN issue_label l ON l.id=x.label_id JOIN issue i ON i.id=x.issue_id WHERE i.workspace_id=$1 AND i.current_iteration_id=$2",
	} {
		var plan []byte
		dbfx.QueryRow(t, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+statement, testWorkspaceID, first).Scan(&plan)
		t.Logf("query_plan_%s=%s", name, plan)
	}
	for sample := range samples + warmup {
		next := lifecycleHTTPCreate(t, h, fmt.Sprintf("P95 next %02d", sample))
		row, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(first)})
		if err != nil {
			t.Fatal(err)
		}
		lifecycleHTTPApply(t, h, iteration.Draft{Operation: "start", IterationID: &first, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 2, Start: &iteration.StartDraft{TargetID: first, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
		var detail struct {
			Iteration  iteration.Iteration  `json:"iteration"`
			Statistics iteration.Statistics `json:"statistics"`
		}
		began := time.Now()
		testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+first, first, nil)).Want(200).JSON(&detail)
		measurements["detail"] = append(measurements["detail"], float64(time.Since(began).Microseconds())/1000)
		if detail.Statistics.Original != count || detail.Statistics.Current != count {
			t.Fatalf("sample %d incomplete statistics: %+v", sample, detail.Statistics)
		}
		moves = moves[:0]
		for _, id := range ids {
			issue, err := h.Queries.GetIssue(t.Context(), parseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			moves = append(moves, iteration.Move{IssueID: id, ExpectedIssueRevision: issue.Revision, ExpectedSourceID: &first, TargetID: &next})
		}
		reason := "Measured carryover"
		draft := iteration.Draft{Operation: "end", IterationID: &first, ExpectedIterationRevision: &detail.Iteration.Revision, ExpectedScopeRevision: &detail.Iteration.ScopeRevision, ExpectedSettingsRevision: 2, Reason: &reason, Moves: moves}
		var preview iteration.Preview
		began = time.Now()
		testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", draft)).Want(200).JSON(&preview)
		measurements["preview"] = append(measurements["preview"], float64(time.Since(began).Microseconds())/1000)
		if !preview.Complete || preview.TotalAffected != count || len(preview.InvalidItems) != 0 {
			t.Fatalf("sample %d incomplete preview: %+v", sample, preview)
		}
		var result iteration.WriteResult
		began = time.Now()
		testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})).Want(200).JSON(&result)
		measurements["closure"] = append(measurements["closure"], float64(time.Since(began).Microseconds())/1000)
		if got := dbfx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1 AND iteration_rollover_count=$2", next, sample+1); got != count {
			t.Fatalf("sample %d only %d tasks rolled over exactly once", sample, got)
		}
		if got := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot s, jsonb_array_elements(s.body->'scope') item WHERE s.iteration_id=$1 AND jsonb_array_length(item->'labels')=3 AND item->>'priority' IS NOT NULL", first); got != count {
			t.Fatalf("sample %d froze display facts for only %d members", sample, got)
		}
		t.Logf("sample=%d detail_ms=%.3f preview_ms=%.3f closure_ms=%.3f", sample, measurements["detail"][sample], measurements["preview"][sample], measurements["closure"][sample])
		first = next
	}
	for name, values := range measurements {
		sorted := append([]float64(nil), values[warmup:]...)
		sort.Float64s(sorted)
		t.Logf("%s samples=%d first_ms=%.3f p50_ms=%.3f p95_ms=%.3f max_ms=%.3f", name, len(sorted), values[0], sorted[14], sorted[28], sorted[29])
	}
	stopObserving()
	waitSummary := <-waits
	if waitSummary.err != nil {
		t.Fatal(waitSummary.err)
	}
	encoded, err := json.Marshal(map[string]any{"samples_ms": measurements, "warmup_samples": warmup, "measured_samples": samples, "tasks_per_cycle": count, "distribution": "1000 todo; 10 projects; owner assignee; five priorities; three of six labels per task; real membership, baseline and repeated carryover", "cache": "first access then warm; OS/PG cache not flushed", "postgres": postgres, "go": runtime.Version(), "sql_statements": tracer.statements.Load(), "lock_wait": map[string]any{"samples": waitSummary.samples, "blocked_samples": waitSummary.blocked, "sampled_blocked_ms": waitSummary.blockedTime.Milliseconds(), "events": waitSummary.events}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("I1_P95_JSON=%s", encoded)
	for name, values := range measurements {
		sorted := append([]float64(nil), values[warmup:]...)
		sort.Float64s(sorted)
		if sorted[28] > 2000 {
			t.Errorf("%s P95 %.3f ms exceeds frozen 2000 ms target", name, sorted[28])
		}
	}
}

// Fifty-one populated plans deliberately exceed the default first list page.
// All 1,530 members must be processed in each single disable transaction.
func TestIterationI1DisableP95(t *testing.T) {
	if os.Getenv("MULTICA_RUN_I1_P95") != "1" {
		t.Skip("opt-in: MULTICA_RUN_I1_P95=1 with an exclusive DATABASE_URL")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Fatal("explicit exclusive DATABASE_URL required")
	}
	h := lifecycleHTTPHandler(t)
	dbfx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", testWorkspaceID)
	const plans, perPlan, samples, warmup = 51, 30, 30, 5
	labels := make([]string, 3)
	for n := range labels {
		labels[n] = dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": fmt.Sprintf("Disable label %d", n), "resource_type": "issue", "color": "#123456"})
	}
	ids := make([]string, 0, plans*perPlan)
	for n := range plans * perPlan {
		id := dbfx.Issue(t, fmt.Sprintf("Disable capacity %d", n), testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
		ids = append(ids, id)
		for _, label := range labels {
			dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": id, "label_id": label}, "issue_id=$1 AND label_id=$2", id, label)
		}
	}
	var previewTimes, applyTimes []float64
	settingsRevision := int64(2)
	for sample := range samples + warmup {
		targets := make([]string, plans)
		for i := range plans {
			targets[i] = lifecycleHTTPCreate(t, h, fmt.Sprintf("Disable sample %d plan %d", sample, i))
		}
		moves := make([]iteration.Move, len(ids))
		for i, id := range ids {
			row, err := h.Queries.GetIssue(t.Context(), parseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			moves[i] = iteration.Move{IssueID: id, ExpectedIssueRevision: row.Revision, TargetID: &targets[i/perPlan]}
		}
		lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: settingsRevision, Moves: moves})
		active, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(targets[0])})
		if err != nil {
			t.Fatal(err)
		}
		lifecycleHTTPApply(t, h, iteration.Draft{Operation: "start", IterationID: &targets[0], ExpectedIterationRevision: &active.Revision, ExpectedScopeRevision: &active.ScopeRevision, ExpectedSettingsRevision: settingsRevision, Start: &iteration.StartDraft{TargetID: targets[0], Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
		reason := "Measure whole workspace disable"
		draft := iteration.Draft{Operation: "disable", ExpectedSettingsRevision: settingsRevision, Reason: &reason, Moves: []iteration.Move{}}
		var preview iteration.Preview
		began := time.Now()
		testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", draft)).Want(200).JSON(&preview)
		previewTimes = append(previewTimes, float64(time.Since(began).Microseconds())/1000)
		if preview.TotalAffected != plans*perPlan || len(preview.Iterations) != plans || !preview.Complete || len(preview.InvalidItems) != 0 {
			t.Fatalf("disable omitted plans or members: %+v", preview)
		}
		began = time.Now()
		testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})).Want(200)
		applyTimes = append(applyTimes, float64(time.Since(began).Microseconds())/1000)
		if dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND current_iteration_id IS NOT NULL", testWorkspaceID) != 0 {
			t.Fatal("disable partially released the workspace")
		}
		if dbfx.Count(t, "SELECT count(*) FROM iteration WHERE workspace_id=$1 AND status IN ('active','planned')", testWorkspaceID) != 0 {
			t.Fatal("disable omitted future plans")
		}
		settingsRevision++
		if sample+1 < samples+warmup {
			testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", map[string]any{"request_id": uuid.NewString(), "expected_revision": settingsRevision, "confirmed_timezone": "UTC"})).Want(200)
			settingsRevision++
		}
		t.Logf("disable_sample=%d plans=%d members=%d preview_ms=%.3f apply_ms=%.3f", sample, plans, len(ids), previewTimes[sample], applyTimes[sample])
	}
	for name, values := range map[string][]float64{"disable_preview": previewTimes, "disable_apply": applyTimes} {
		sorted := append([]float64(nil), values[warmup:]...)
		sort.Float64s(sorted)
		t.Logf("%s samples=%d p50_ms=%.3f p95_ms=%.3f max_ms=%.3f", name, len(sorted), sorted[14], sorted[28], sorted[29])
	}
	encoded, err := json.Marshal(map[string]any{"plans": plans, "members": len(ids), "warmup_samples": warmup, "measured_samples": samples, "preview_ms": previewTimes, "disable_ms": applyTimes})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("I1_DISABLE_P95_JSON=%s", encoded)
	for name, values := range map[string][]float64{"disable_preview": previewTimes, "disable_apply": applyTimes} {
		sorted := append([]float64(nil), values[warmup:]...)
		sort.Float64s(sorted)
		if sorted[28] > 3000 {
			t.Errorf("%s P95 %.3f ms exceeds frozen 3000 ms target", name, sorted[28])
		}
	}
}
