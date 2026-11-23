package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func f2ReportIssue(t *testing.T, project string) string {
	t.Helper()
	cols := testutil.Cols{}
	if project != "" {
		cols["project_id"] = project
	}
	id := dbfx.Issue(t, "F2 report", cols)
	dbfx.Cleanup(t, "DELETE FROM report_snapshot WHERE report_issue_id=$1", id)
	dbfx.Cleanup(t, "DELETE FROM report_snapshot_control WHERE report_issue_id=$1", id)
	return id
}
func f2CaptureBody(t *testing.T, id string) map[string]any {
	t.Helper()
	var revision int64
	dbfx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", id).Scan(&revision)
	end := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	return map[string]any{"protocol_version": 1, "expected_issue_revision": revision, "start": end.Add(-24 * time.Hour).Format(time.RFC3339Nano), "end": end.Format(time.RFC3339Nano), "timezone": "Asia/Shanghai", "period_mode": "rolling_24h", "correction_of": nil}
}
func f2HumanRequest(method, id, snapshot, key string, body any) *http.Request {
	path := "/api/issues/" + id + "/report-snapshots"
	if method == "GET" {
		if snapshot == "" {
			path += "/current"
		} else {
			path += "/" + snapshot
		}
	}
	req := testutil.WithURLParams(testutil.JSONRequest(method, path, body), "id", id, "snapshotId", snapshot)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	return req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
}
func f2Call(t *testing.T, h *Handler, req *http.Request) *testutil.Response {
	t.Helper()
	var action http.HandlerFunc = h.GetReportSnapshot
	if req.Method == "POST" {
		action = h.CreateReportSnapshot
		if strings.HasSuffix(req.URL.Path, "/withdrawals") {
			action = h.WithdrawReportSnapshots
		}
	}
	return testutil.Call(t, middleware.PreserveReportSnapshotHeaders(action).ServeHTTP, req)
}
func f2Capture(t *testing.T, h *Handler, id, key string, body any, status int) service.ReportSnapshotResponse {
	t.Helper()
	var out service.ReportSnapshotResponse
	f2Call(t, h, f2HumanRequest("POST", id, "", key, body)).Want(status).JSON(&out)
	return out
}
func f2StoredInternal(t *testing.T, snapshotID string) reportsnapshot.Snapshot {
	t.Helper()
	var raw []byte
	dbfx.QueryRow(t, "SELECT snapshot_json FROM report_snapshot WHERE workspace_id=$1 AND id=$2", testWorkspaceID, snapshotID).Scan(&raw)
	var stored struct {
		Internal json.RawMessage `json:"internal"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	value, err := reportsnapshot.Decode(stored.Internal)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func f2Source(t *testing.T, s reportsnapshot.Snapshot, kind string) reportsnapshot.SourceFact {
	t.Helper()
	for _, source := range s.Facts.Sources {
		if source.Kind == kind {
			return source
		}
	}
	t.Fatalf("missing source %s", kind)
	return reportsnapshot.SourceFact{}
}

func TestF2ReportSnapshotCaptureReplayCorrectionAndFrozenSources(t *testing.T) {
	project := dbfx.Project(t, "F2 project")
	report := f2ReportIssue(t, project)
	source := dbfx.Issue(t, "Business source", testutil.Cols{"project_id": project, "status": "done"})
	body := f2CaptureBody(t, report)
	key := uuid.NewString()
	var beforeRevision int64
	dbfx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", report).Scan(&beforeRevision)
	beforeComments := dbfx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1", report)
	first := f2Capture(t, testHandler, report, key, body, 201)
	if first.Replayed || first.Snapshot.SnapshotID == "" || first.Snapshot.Complete {
		t.Fatalf("capture must retain unresolved production history: %+v", first)
	}
	if err := reportsnapshot.ValidatePublic(first.Snapshot); err != nil {
		t.Fatal(err)
	}
	intent, err := reportsnapshot.CanonicalJSON(map[string]any{"report_issue_id": report, "request_key": key, "request": body})
	if err != nil {
		t.Fatal(err)
	}
	commitment := sha256.Sum256(intent)
	if first.CaptureReceipt == nil || first.CaptureReceipt.RequestKey != key || first.CaptureReceipt.ReportIssueID != report || first.CaptureReceipt.SnapshotID != first.Snapshot.SnapshotID || first.CaptureReceipt.RequestDigest != hex.EncodeToString(commitment[:]) {
		t.Fatal("capture receipt did not bind the exact caller's canonical intent")
	}
	if len(first.Snapshot.Facts.Issues) != 2 {
		t.Fatalf("wrong cohort: %+v", first.Snapshot.Facts.Issues)
	}
	for _, issue := range first.Snapshot.Facts.Issues {
		if issue.ID == report && issue.Classification != "report" {
			t.Fatal("current report not excluded")
		}
		if issue.ID == source && issue.Classification != "unknown" {
			t.Fatal("unproven business purpose guessed")
		}
	}
	dbfx.Exec(t, "UPDATE issue SET status='todo',revision=revision+1 WHERE id=$1", source)
	// A successful original request survives an unrelated report revision change.
	dbfx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", report)
	replay := f2Capture(t, testHandler, report, key, body, 200)
	if !replay.Replayed || replay.Snapshot.ProjectionDigest != first.Snapshot.ProjectionDigest {
		t.Fatal("original request was recomputed")
	}
	f2Call(t, testHandler, f2HumanRequest("POST", report, "", uuid.NewString(), body)).Want(409)
	body["expected_issue_revision"] = beforeRevision + 1
	f2Call(t, testHandler, f2HumanRequest("POST", report, "", uuid.NewString(), body)).Want(409) // same-cycle root is separate from request-key replay
	body["correction_of"] = first.Snapshot.SnapshotID
	corrected := f2Capture(t, testHandler, report, uuid.NewString(), body, 201)
	if corrected.Snapshot.CorrectionOf == nil || *corrected.Snapshot.CorrectionOf != first.Snapshot.SnapshotID || corrected.Snapshot.SnapshotID == first.Snapshot.SnapshotID {
		t.Fatal("correction did not get new identity")
	}
	var old, current service.ReportSnapshotResponse
	f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(200).JSON(&old)
	f2Call(t, testHandler, f2HumanRequest("GET", report, "", "", nil)).Want(200).JSON(&current)
	if old.Snapshot.ProjectionDigest != first.Snapshot.ProjectionDigest || current.Snapshot.SnapshotID != corrected.Snapshot.SnapshotID {
		t.Fatal("current alias rewrote immutable read")
	}
	dbfx.Exec(t, "DELETE FROM issue WHERE id=$1", source)
	f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(200).JSON(&old)
	if old.Snapshot.ProjectionDigest != first.Snapshot.ProjectionDigest {
		t.Fatal("source deletion changed frozen facts")
	}
	if dbfx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1", report) != beforeComments {
		t.Fatal("snapshot published a report comment")
	}
	var afterRevision int64
	dbfx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", report).Scan(&afterRevision)
	if afterRevision != beforeRevision+1 {
		t.Fatal("capture modified report revision")
	}
}

func TestF2ReportSnapshotRequiresBoundStoredPublicProjection(t *testing.T) {
	report := f2ReportIssue(t, "")
	first := f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	var raw []byte
	dbfx.QueryRow(t, "SELECT snapshot_json FROM report_snapshot WHERE id=$1", first.Snapshot.SnapshotID).Scan(&raw)
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	other := f2ReportIssue(t, "")
	second := f2Capture(t, testHandler, other, uuid.NewString(), f2CaptureBody(t, other), 201)
	wrongPublic, err := json.Marshal(second.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"legacy_internal_only", stored["internal"]},
		{"different_public_identity", []byte(`{"storage_version":2,"internal":` + string(stored["internal"]) + `,"public":` + string(wrongPublic) + `,"audit":` + string(stored["audit"]) + `,"audit_digest":` + string(stored["audit_digest"]) + `}`)},
		{"duplicate_storage_field", []byte(strings.TrimSuffix(string(raw), "}") + `,"storage_version":2}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbfx.Exec(t, "UPDATE report_snapshot SET snapshot_json=$1 WHERE id=$2", tc.raw, first.Snapshot.SnapshotID)
			defer dbfx.Exec(t, "UPDATE report_snapshot SET snapshot_json=$1 WHERE id=$2", raw, first.Snapshot.SnapshotID)
			response := f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(503)
			if strings.Contains(response.Body.String(), "\"snapshot\"") || strings.Contains(response.Body.String(), first.Snapshot.ProjectionDigest) {
				t.Fatal("unsupported storage reconstructed or leaked a public projection")
			}
		})
	}
}

func TestF2ReportSnapshotRowSealBindsPublicAndCollectionAudit(t *testing.T) {
	report := f2ReportIssue(t, "")
	first := f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	var raw []byte
	var originalSeal string
	dbfx.QueryRow(t, "SELECT snapshot_json,seal_digest FROM report_snapshot WHERE id=$1", first.Snapshot.SnapshotID).Scan(&raw, &originalSeal)
	internal := f2StoredInternal(t, first.Snapshot.SnapshotID)
	alternateInput := internal.Input
	alternateInput.Facts.Issues = append([]reportsnapshot.IssueFact{}, internal.Facts.Issues...)
	future := "2099-01-01"
	alternateInput.Facts.Issues[0].DueDate = &future
	alternateInternal, err := reportsnapshot.Build(alternateInput)
	if err != nil {
		t.Fatal(err)
	}
	alternatePublic, err := reportsnapshot.ProjectPublic(alternateInternal)
	if err != nil {
		t.Fatal(err)
	}
	if err = reportsnapshot.ValidatePublic(alternatePublic); err != nil {
		t.Fatal(err)
	}
	originalMetrics, _ := reportsnapshot.CanonicalJSON(first.Snapshot.Metrics)
	alternateMetrics, _ := reportsnapshot.CanonicalJSON(alternatePublic.Metrics)
	if !bytes.Equal(originalMetrics, alternateMetrics) || alternatePublic.SnapshotID != first.Snapshot.SnapshotID || alternatePublic.ProjectionDigest == first.Snapshot.ProjectionDigest {
		t.Fatal("fixture must change valid public facts while retaining identity and result values")
	}
	digestOf := func(value any) string {
		encoded, err := reportsnapshot.CanonicalJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		d := sha256.Sum256(encoded)
		return hex.EncodeToString(d[:])
	}
	for _, kind := range []string{"public", "audit"} {
		t.Run(kind, func(t *testing.T) {
			var stored map[string]json.RawMessage
			if err := json.Unmarshal(raw, &stored); err != nil {
				t.Fatal(err)
			}
			publicDigest := first.Snapshot.ProjectionDigest
			if kind == "public" {
				stored["public"], err = json.Marshal(alternatePublic)
				if err != nil {
					t.Fatal(err)
				}
				publicDigest = alternatePublic.ProjectionDigest
			} else {
				var audit map[string]any
				if err := json.Unmarshal(stored["audit"], &audit); err != nil {
					t.Fatal(err)
				}
				changed := false
				for _, source := range audit["sources"].([]any) {
					pages, _ := source.(map[string]any)["pages"].([]any)
					for _, item := range pages {
						page := item.(map[string]any)
						if page["requested_limit"].(float64) > page["observed_count"].(float64) {
							page["requested_limit"] = page["requested_limit"].(float64) + 1
							changed = true
							break
						}
					}
					if changed {
						break
					}
				}
				if !changed {
					t.Fatal("fixture has no exhaustive bounded page")
				}
				stored["audit"], err = json.Marshal(audit)
				if err != nil {
					t.Fatal(err)
				}
				stored["audit_digest"], _ = json.Marshal(digestOf(audit))
			}
			changedRaw, err := json.Marshal(stored)
			if err != nil {
				t.Fatal(err)
			}
			dbfx.Exec(t, "UPDATE report_snapshot SET snapshot_json=$1 WHERE id=$2", changedRaw, first.Snapshot.SnapshotID)
			defer dbfx.Exec(t, "UPDATE report_snapshot SET snapshot_json=$1,seal_digest=$2 WHERE id=$3", raw, originalSeal, first.Snapshot.SnapshotID)
			f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(503)
			// Updating only the fixture's composite seal proves the alternate is
			// independently valid: the original pair binding caused the denial.
			var auditDigest string
			if err := json.Unmarshal(stored["audit_digest"], &auditDigest); err != nil {
				t.Fatal(err)
			}
			newSeal := digestOf(map[string]any{"storage_version": 2, "snapshot_id": internal.SnapshotID, "workspace_id": internal.WorkspaceID, "report_issue_id": internal.ReportIssueID, "project_id": internal.ProjectID, "internal_digest": internal.SnapshotDigest, "public_digest": publicDigest, "audit_digest": auditDigest})
			dbfx.Exec(t, "UPDATE report_snapshot SET seal_digest=$1 WHERE id=$2", newSeal, first.Snapshot.SnapshotID)
			f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(200)
		})
	}
}

func TestF2ReportSnapshotActivityScanBeyondTimelineCap(t *testing.T) {
	project := dbfx.Project(t, "F2 large history")
	report := f2ReportIssue(t, project)
	source := dbfx.Issue(t, "History source", testutil.Cols{"project_id": project})
	// A bulk fixture keeps the test focused on source pagination, not setup round trips.
	dbfx.Exec(t, `INSERT INTO activity_log(id,workspace_id,issue_id,action,details,created_at) SELECT gen_random_uuid(),$1,$2,'status_changed','{"from":"todo","to":"done"}'::jsonb,'2026-01-01'::timestamptz FROM generate_series(1,2101)`, testWorkspaceID, source)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE issue_id=$1", source)
	response := f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	captured := f2StoredInternal(t, response.Snapshot.SnapshotID)
	if len(response.Snapshot.Facts.Activities) != 0 {
		t.Fatal("unproven native activity leaked into public projection")
	}
	activity := f2Source(t, captured, "activities")
	if len(captured.Facts.Activities) != 2101 || !activity.ReadComplete || activity.ProducerComplete || activity.Watermark != nil {
		t.Fatalf("scan completeness confused with producer proof: %+v / %d", activity, len(captured.Facts.Activities))
	}
}

func TestF2ReportSnapshotStrictRequestAndHeaderBoundaries(t *testing.T) {
	report := f2ReportIssue(t, "")
	body := f2CaptureBody(t, report)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"duplicate key", func(r *http.Request) { r.Header.Add("Idempotency-Key", uuid.NewString()) }},
		{"workspace conflict", func(r *http.Request) { r.Header.Set("X-Workspace-ID", uuid.NewString()) }},
		{"actor spoof", func(r *http.Request) { r.Header.Set("X-Actor-Source", "member") }},
		{"duplicate workspace", func(r *http.Request) { r.Header.Add("X-Workspace-ID", testWorkspaceID) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := f2HumanRequest("POST", report, "", uuid.NewString(), json.RawMessage(raw))
			tc.mutate(req)
			want := 403
			if tc.name == "duplicate key" {
				want = 400
			}
			f2Call(t, testHandler, req).Want(want)
		})
	}
	for _, bad := range []string{strings.TrimSuffix(string(raw), "}") + `,"protocol_version":1}`, strings.Replace(string(raw), `"timezone"`, `"Timezone"`, 1), strings.TrimSuffix(string(raw), "}") + `,"as_of":"2026-01-01T00:00:00Z"}`} {
		f2Call(t, testHandler, f2HumanRequest("POST", report, "", uuid.NewString(), bad)).Want(400)
	}
	if dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 0 {
		t.Fatal("rejected capture persisted state")
	}
	first := f2Capture(t, testHandler, report, uuid.NewString(), body, 201)
	if first.Snapshot.ProjectID != nil || first.Snapshot.Facts.Project != nil {
		t.Fatal("no-project capture invented project scope")
	}
}

type f2SourceFailureStarter struct {
	inner         txStarter
	commitFailure bool
	source        string
}

func (s f2SourceFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	return f2SourceFailureTx{Tx: tx, commitFailure: s.commitFailure, source: s.source}, err
}

type f2SourceFailureTx struct {
	pgx.Tx
	commitFailure bool
	source        string
}

func (tx f2SourceFailureTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.source != "" && strings.Contains(sql, tx.source) {
		return nil, &pgconn.PgError{Code: "42501", Message: "test-owned source unavailable"}
	}
	return tx.Tx.Query(ctx, sql, args...)
}

type f2SourceFailureRow struct{ err error }

func (row f2SourceFailureRow) Scan(...any) error { return row.err }

func (tx f2SourceFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.source != "" && strings.Contains(sql, tx.source) {
		return f2SourceFailureRow{err: &pgconn.PgError{Code: "42501", Message: "test-owned source unavailable"}}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx f2SourceFailureTx) Commit(ctx context.Context) error {
	if tx.commitFailure {
		return errors.New("test-owned commit failure")
	}
	return tx.Tx.Commit(ctx)
}

func TestF2ReportSnapshotSourceGapAndFailedSeal(t *testing.T) {
	project := dbfx.Project(t, "F2 gap")
	report := f2ReportIssue(t, project)
	// Production classification is independently unproven for this ordinary
	// source. Pure reducer tests own the certified-business/history matrix;
	// this case proves the real failed query retains its source gap and nulls.
	dbfx.Issue(t, "F2 source requiring historical evidence", testutil.Cols{"project_id": project})
	body := f2CaptureBody(t, report)
	h := *testHandler
	h.TxStarter = f2SourceFailureStarter{inner: testHandler.TxStarter, source: "ListReportSnapshotSourceActivities"}
	response := f2Capture(t, &h, report, uuid.NewString(), body, 201)
	captured := f2StoredInternal(t, response.Snapshot.SnapshotID)
	source := f2Source(t, captured, "activities")
	if source.ReadComplete || source.Count != nil {
		t.Fatal("unreadable activity source lost its unknown extent")
	}
	if captured.Complete || response.Snapshot.Complete {
		t.Fatal("unresolved business/history candidate was reported complete")
	}
	closedFound := false
	for _, metric := range response.Snapshot.Metrics {
		if metric.ID == "window.closed" {
			closedFound = true
			if metric.Quality != "unknown" || string(metric.Value) != "null" || metric.Coverage.Unknown.Count == nil || *metric.Coverage.Unknown.Count != 1 {
				t.Fatal("unverified business/history candidate became an exact zero")
			}
		}
	}
	if !closedFound {
		t.Fatal("window.closed metric missing")
	}
	found := false
	for _, gap := range captured.Facts.Gaps {
		if gap.Source == "activities" && gap.Code == "source_unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatal("source permission failure was not retained as a gap")
	}
	other := f2ReportIssue(t, project)
	h.TxStarter = f2SourceFailureStarter{inner: testHandler.TxStarter, commitFailure: true}
	failed := f2Call(t, &h, f2HumanRequest("POST", other, "", uuid.NewString(), f2CaptureBody(t, other))).Want(503)
	if strings.Contains(failed.Body.String(), "projection_digest") || dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", other) != 0 {
		t.Fatal("failed seal exposed or retained result")
	}
}

func TestF2ReportSnapshotReportOnlyGapKeepsSourceUnknown(t *testing.T) {
	project := dbfx.Project(t, "F2 report-only scope")
	report := f2ReportIssue(t, project)
	h := *testHandler
	h.TxStarter = f2SourceFailureStarter{inner: testHandler.TxStarter, source: "ListReportSnapshotSourceActivities"}
	response := f2Capture(t, &h, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	stored := f2StoredInternal(t, response.Snapshot.SnapshotID)
	source := f2Source(t, stored, "activities")
	if source.ReadComplete || source.Count != nil {
		t.Fatal("irrelevant activity failure was falsely marked a complete source")
	}
	foundGap, foundClosed := false, false
	for _, gap := range stored.Facts.Gaps {
		if gap.Source == "activities" && gap.Code == "source_unavailable" {
			foundGap = true
		}
	}
	for _, metric := range response.Snapshot.Metrics {
		if metric.ID == "window.closed" {
			foundClosed = true
			if metric.Quality != "exact" || string(metric.Value) != "0" || metric.Coverage.Excluded != 1 || metric.Coverage.Unknown.Count == nil || *metric.Coverage.Unknown.Count != 0 {
				t.Fatal("known report exclusion was turned into an unknown business candidate")
			}
		}
	}
	// Completeness describes the declared metrics. It never erases an
	// irrelevant source gap or certifies the activity producer's history.
	if !foundGap || !foundClosed || !stored.Complete || !response.Snapshot.Complete {
		t.Fatal("report-only metric completeness and source completeness were conflated")
	}
}

func TestF2ReportSnapshotUpdateGapDoesNotDiscardProjectFacts(t *testing.T) {
	project := dbfx.Project(t, "F2 independent update source", testutil.Cols{"status": "in_progress", "due_date": "2000-01-01"})
	report := f2ReportIssue(t, project)
	h := *testHandler
	h.TxStarter = f2SourceFailureStarter{inner: testHandler.TxStarter, source: "GetReportSnapshotSourceLatestUpdate"}
	response := f2Capture(t, &h, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	seen := map[string]bool{}
	for _, metric := range response.Snapshot.Metrics {
		switch metric.ID {
		case "project.overdue":
			seen[metric.ID] = true
			if metric.Quality != "exact" || string(metric.Value) != "true" {
				t.Fatal("update failure discarded independent project overdue facts")
			}
		case "project.latest_update_at", "project.progress_age_days", "project.health":
			seen[metric.ID] = true
			if metric.Quality != "unknown" || string(metric.Value) != "null" {
				t.Fatal("update failure was hidden by complete unrelated sources")
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("expected independent and dependent metrics were absent")
	}
	stored := f2StoredInternal(t, response.Snapshot.SnapshotID)
	if !f2Source(t, stored, "project").ReadComplete || f2Source(t, stored, "updates").ReadComplete {
		t.Fatal("source-specific read coverage was conflated")
	}
}

func TestF2ReportSnapshotConcurrentOriginalRequest(t *testing.T) {
	report := f2ReportIssue(t, "")
	body := f2CaptureBody(t, report)
	key := uuid.NewString()
	var wg sync.WaitGroup
	outcomes := make(chan service.ReportSnapshotResponse, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := f2Call(t, testHandler, f2HumanRequest("POST", report, "", key, body))
			if response.Code != 200 && response.Code != 201 {
				t.Errorf("concurrent capture returned %d: %s", response.Code, response.Body.String())
				return
			}
			var out service.ReportSnapshotResponse
			response.JSON(&out)
			outcomes <- out
		}()
	}
	wg.Wait()
	close(outcomes)
	id := ""
	for out := range outcomes {
		if id != "" && out.Snapshot.SnapshotID != id {
			t.Fatal("one request produced different snapshots")
		}
		id = out.Snapshot.SnapshotID
	}
	if dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 1 {
		t.Fatal("duplicate original request persisted more than once")
	}
}

func TestF2ReportSnapshotOwnedDeleteCleanup(t *testing.T) {
	report := f2ReportIssue(t, "")
	f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	if err := testHandler.Queries.DeleteIssue(context.Background(), db.DeleteIssueParams{ID: parseUUID(report), WorkspaceID: parseUUID(testWorkspaceID)}); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 0 {
		t.Fatal("report deletion retained owning snapshot")
	}
	if dbfx.Count(t, "SELECT count(*) FROM report_snapshot_control WHERE report_issue_id=$1", report) != 0 {
		t.Fatal("report deletion retained its control fence")
	}
}
