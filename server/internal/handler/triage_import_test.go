package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func triageImportSetup(t *testing.T) {
	t.Helper()
	triageEnableForTest(t)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM triage_import_row WHERE workspace_id=$1`, testWorkspaceID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM triage_import_batch WHERE workspace_id=$1`, testWorkspaceID)
	})
}
func triageImportPreviewForTest(t *testing.T, content string) TriageImportPreview {
	t.Helper()
	return testutil.Decode[TriageImportPreview](t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "incoming.csv", "csv": content}), 200)
}
func triageImportCommitForTest(t *testing.T, id string, rows ...map[string]any) TriageImportResult {
	t.Helper()
	return testutil.Decode[TriageImportResult](t, testHandler.CommitTriageImport, withURLParam(newRequest("POST", "/api/triage/imports/"+id+"/commit", map[string]any{"rows": rows}), "id", id), 200)
}
func TestTriageImportPreviewIsInertAndDurable(t *testing.T) {
	triageImportSetup(t)
	var before, after int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, testWorkspaceID).Scan(&before)
	input := map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "incoming.csv", "csv": "title,description,project,labels,state,external_id\nFirst,\"line1\nline2\",unknown,unknown,done,ext1\n,missing title,,,,\nSame ID,,,,,ext1\n"}
	first := testutil.Decode[TriageImportPreview](t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", input), 200)
	second := testutil.Decode[TriageImportPreview](t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", input), 200)
	if first.BatchID != second.BatchID || len(first.Rows) != 3 || len(first.Rows[0].Warnings) < 3 || len(first.Rows[1].Errors) == 0 || !first.Rows[2].Duplicate {
		t.Fatalf("preview diagnostics/idempotency: %+v", first)
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, testWorkspaceID).Scan(&after)
	if before != after {
		t.Fatal("preview created issues")
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_notification WHERE workspace_id=$1`, testWorkspaceID).Scan(&after)
	if after != 0 {
		t.Fatal("preview sent notifications")
	}
	input["csv"] = "title\nchanged"
	testutil.Call(t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", input)).Want(409)
}
func TestTriageImportPartialRetryDuplicateAndDeletedReplay(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "title,external_id,due_date\nOne,repeat,\nTwo,repeat,\nBad,,2026-02-30\nLater,,\n")
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2}, map[string]any{"row_number": 3})
	if result.Created != 1 || result.Skipped != 1 || result.Failed != 1 {
		t.Fatalf("partial: %+v", result)
	}
	firstID := *result.Results[0].IssueID
	result = triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2, "import_duplicate": true}, map[string]any{"row_number": 4})
	if result.Created != 3 || *result.Results[0].IssueID != firstID {
		t.Fatalf("retry: %+v", result)
	}
	var tasks, notices int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT issue_id FROM triage_import_row WHERE batch_id=$1)`, p.BatchID).Scan(&tasks)
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_notification WHERE batch_id=$1`, p.BatchID).Scan(&notices)
	if tasks != 0 || notices != 1 {
		t.Fatalf("import executed or duplicated summary: tasks=%d notices=%d", tasks, notices)
	}
	dbfx.Exec(t, `DELETE FROM issue_triage WHERE issue_id=$1`, firstID)
	dbfx.Exec(t, `DELETE FROM issue WHERE id=$1`, firstID)
	result = triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1})
	if result.Created != 1 || *result.Results[0].IssueID != firstID {
		t.Fatal("deleted import identity lost")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE id=$1`, firstID).Scan(&count)
	if count != 0 {
		t.Fatal("retry recreated deleted issue")
	}
	other := triageImportPreviewForTest(t, "title,external_id\nAcross batches,repeat\n")
	if !other.Rows[0].Duplicate {
		t.Fatal("cross-batch external identity lost")
	}
	result = triageImportCommitForTest(t, other.BatchID, map[string]any{"row_number": 1})
	if result.Skipped != 1 || result.Created != 0 {
		t.Fatal("external duplicate created without explicit override")
	}
}
func TestTriageImportReferenceRevalidationAndFailureCSV(t *testing.T) {
	triageImportSetup(t)
	project := dbfx.Project(t, "CSV project")
	p := triageImportPreviewForTest(t, "title,project,description,due_date\nGood,CSV project,\"quoted, value\",\n=SUM(A1),,\"line1\nline2\",bad\n")
	dbfx.Exec(t, `DELETE FROM project WHERE id=$1`, project)
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2})
	if result.Created != 0 || result.Failed != 2 {
		t.Fatalf("deleted reference must fail until corrected new preview: %+v", result)
	}
	response := testutil.Call(t, testHandler.DownloadTriageImportFailures, withURLParam(newRequest("GET", "/api/triage/imports/failures", nil), "id", p.BatchID)).Want(200)
	rows, err := csv.NewReader(strings.NewReader(response.Text())).ReadAll()
	if err != nil || len(rows) != 3 || rows[2][0] != "'=SUM(A1)" || rows[2][2] != "line1\nline2" {
		t.Fatalf("failure CSV not faithful/safe: %q %v", response.Text(), err)
	}
}
func TestTriageImportAccessAndEncodingLimits(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "title\nOne\n")
	other := dbfx.User(t, "CSV other", "csv-other@multica.test")
	dbfx.Member(t, testWorkspaceID, other, "member")
	get := withURLParam(newRequest("GET", "/api/triage/imports", nil), "id", p.BatchID)
	get.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.GetTriageImport, get).Want(404)
	denied := newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "x.csv", "csv": "title\nX"})
	denied.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.PreviewTriageImport, denied).Want(403)
	invalid := newRequest("POST", "/api/triage/imports/preview", nil)
	invalid.Body = io.NopCloser(strings.NewReader("{\"request_id\":\"" + uuidToString(dbid.NewV7()) + "\",\"filename\":\"x.csv\",\"csv\":\"title\\n\xff\"}"))
	testutil.Call(t, testHandler.PreviewTriageImport, invalid).Want(400)
	for _, content := range []string{"title\n\ufffd", "title\n" + strings.Repeat("x", 5*1024*1024), "title\n" + strings.Repeat("x\n", 1001)} {
		testutil.Call(t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "x.csv", "csv": content})).Want(400)
	}
}

// The label matrix lives in triagecsv/parse_test.go; this proves the
// normalized value is what intake stores on the created issue.
func TestTriageImportStoresCanonicalPriorityFromChineseLabel(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "标题,优先级\n中文优先级,紧急\n")
	if len(p.Rows[0].Errors) != 0 || p.Rows[0].Values["priority"] != "urgent" {
		t.Fatalf("preview did not normalize priority: %+v", p.Rows[0])
	}
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1})
	if result.Created != 1 {
		t.Fatalf("import failed: %+v", result)
	}
	var item TriageItem
	testutil.Call(t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", *result.Results[0].IssueID)).Want(200).JSON(&item)
	if item.Issue.Priority != "urgent" {
		t.Fatalf("issue priority = %q, want urgent", item.Issue.Priority)
	}
}

func TestTriageImportResolvesScopedNamesAndRetainsCandidatesOnly(t *testing.T) {
	triageImportSetup(t)
	project := dbfx.Project(t, "CSV target")
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "CSV label", "resource_type": "issue", "color": "#888888"})
	recipient := dbfx.User(t, "CSV reviewer", "csv-reviewer@multica.test")
	dbfx.Member(t, testWorkspaceID, recipient, "member")
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET responsibility_mode='assign', responsibility_member_id=$2 WHERE workspace_id=$1`, testWorkspaceID, recipient)
	agent := dbfx.Agent(t, "CSV visible agent", "", testutil.Cols{"owner_id": recipient})
	p := triageImportPreviewForTest(t, "title,project,labels,assignee,description,status,iteration\nAgent row,CSV target,CSV label,CSV visible agent,@someone,done,Current\nMember row,,,csv-reviewer@multica.test,,,\n")
	if len(p.Rows[0].Warnings) != 2 || len(p.Rows[1].Warnings) != 0 {
		t.Fatalf("valid candidates lost: %+v", p.Rows)
	}
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2})
	if result.Created != 2 {
		t.Fatalf("resolved candidates failed: %+v", result)
	}
	for index, row := range result.Results {
		var item TriageItem
		testutil.Call(t, testHandler.GetTriageItem, withURLParam(newRequest("GET", "/api/triage/items", nil), "id", *row.IssueID)).Want(200).JSON(&item)
		if item.Issue.ProjectID != nil || item.Issue.AssigneeID != nil || item.Issue.Status != "backlog" || item.Issue.AdmissionStatus != "pending" {
			t.Fatalf("candidate became formal assignment: %+v", item)
		}
		if index == 0 && (item.CandidateProjectID == nil || *item.CandidateProjectID != project || item.CandidateAssigneeID == nil || *item.CandidateAssigneeID != agent || item.Issue.Labels == nil || len(*item.Issue.Labels) != 1 || (*item.Issue.Labels)[0].ID != label) {
			t.Fatalf("candidate fields were not retained: %+v", item)
		}
		if index == 1 && (item.CandidateAssigneeType == nil || *item.CandidateAssigneeType != "member" || item.CandidateAssigneeID == nil || *item.CandidateAssigneeID != recipient) {
			t.Fatalf("email must resolve to user identity: %+v", item)
		}
		if item.ReviewerID == nil || *item.ReviewerID != recipient || item.BatchID == nil || *item.BatchID != p.BatchID || item.Source != "csv" || item.Filename == nil || *item.Filename != "incoming.csv" || item.RowNumber == nil || *item.RowNumber != int32(index+1) {
			t.Fatalf("source/reviewer attribution missing: %+v", item)
		}
	}
	triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1}, map[string]any{"row_number": 2})
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE details->>'batch_id'=$1`, p.BatchID).Scan(&count)
	if count != 2 {
		t.Fatalf("expected one summary each for importer and reviewer, got %d", count)
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE details->>'batch_id'=$1 AND details->>'created'='2'`, p.BatchID).Scan(&count)
	if count != 2 {
		t.Fatal("summary totals not cumulative")
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT issue_id FROM triage_import_row WHERE batch_id=$1)`, p.BatchID).Scan(&count)
	if count != 0 {
		t.Fatal("CSV candidate or mention started an agent")
	}
}

func TestTriageImportHidesForeignAmbiguousAndPrivateCandidates(t *testing.T) {
	triageImportSetup(t)
	member := dbfx.User(t, "CSV member", "csv-member@multica.test")
	dbfx.Member(t, testWorkspaceID, member, "member")
	dbfx.Agent(t, "Hidden CSV agent", "")
	dbfx.Project(t, "Ambiguous CSV project")
	dbfx.Project(t, "Ambiguous CSV project")
	otherWS := dbfx.Workspace(t, "CSV foreign", "csv-foreign")
	dbfx.Project(t, "Foreign CSV project", testutil.Cols{"workspace_id": otherWS})
	request := newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "scoped.csv", "csv": "title,project,assignee\nHidden,Foreign CSV project,Hidden CSV agent\nAmbiguous,Ambiguous CSV project,\n"})
	request.Header.Set("X-User-ID", member)
	preview := testutil.Decode[TriageImportPreview](t, testHandler.PreviewTriageImport, request, 200)
	if len(preview.Rows[0].Warnings) != 2 || len(preview.Rows[1].Warnings) != 1 {
		t.Fatalf("candidate scope warnings missing: %+v", preview.Rows)
	}
	var plansJSON []byte
	dbfx.QueryRow(t, `SELECT normalized FROM triage_import_row WHERE batch_id=$1 AND row_number=1`, preview.BatchID).Scan(&plansJSON)
	var plan triageImportPlan
	if err := json.Unmarshal(plansJSON, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Input.CandidateProjectID != nil || plan.Input.CandidateAssigneeID != nil {
		t.Fatal("unavailable IDs leaked into normalized intake")
	}
	for _, route := range []struct {
		method  string
		handler http.HandlerFunc
		body    any
	}{{"GET", testHandler.GetTriageImport, nil}, {"GET", testHandler.DownloadTriageImportFailures, nil}, {"POST", testHandler.CommitTriageImport, map[string]any{"rows": []map[string]any{{"row_number": 1}}}}} {
		request := withURLParam(newRequest(route.method, "/api/triage/imports", route.body), "id", preview.BatchID)
		testutil.Call(t, route.handler, request).Want(404)
		request = withURLParam(newRequest(route.method, "/api/triage/imports", route.body), "id", preview.BatchID)
		request.Header.Set("X-User-ID", member)
		request.Header.Set("X-Workspace-ID", otherWS)
		testutil.Call(t, route.handler, request).Want(403)
	}
	dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, member)
	request = withURLParam(newRequest("POST", "/api/triage/imports/commit", map[string]any{"rows": []map[string]any{{"row_number": 1}}}), "id", preview.BatchID)
	request.Header.Set("X-User-ID", member)
	testutil.Call(t, testHandler.CommitTriageImport, request).Want(403)
}

func TestTriageImportRetryAfterReferenceRepairIsAtomic(t *testing.T) {
	triageImportSetup(t)
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "CSV stale label", "resource_type": "issue", "color": "#888888"})
	p := triageImportPreviewForTest(t, "title,labels\nAtomic row,CSV stale label\n")
	dbfx.Exec(t, `UPDATE issue_label SET resource_type='agent' WHERE id=$1`, label)
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1})
	if result.Failed != 1 {
		t.Fatalf("invalidated label should fail: %+v", result)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Atomic row'`, testWorkspaceID).Scan(&count)
	if count != 0 {
		t.Fatal("failed row left orphan issue")
	}
	dbfx.Exec(t, `UPDATE issue_label SET resource_type='issue' WHERE id=$1`, label)
	result = triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1})
	if result.Created != 1 {
		t.Fatalf("repair should retry unsuccessful row: %+v", result)
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM inbox_item WHERE details->>'batch_id'=$1 AND details->>'created'='1' AND details->>'failed'='0'`, p.BatchID).Scan(&count)
	if count != 1 {
		t.Fatal("retry added summary or failed to update cumulative totals")
	}
}

func TestTriageImportConcurrentExternalIDDedup(t *testing.T) {
	triageImportSetup(t)
	first := triageImportPreviewForTest(t, "title,external_id\nRace first,race-key\n")
	second := triageImportPreviewForTest(t, "title,external_id\nRace second,race-key\n")
	type outcome struct {
		row TriageImportRow
		err error
	}
	output := make(chan outcome, 2)
	for _, p := range []TriageImportPreview{first, second} {
		go func(batch string) {
			row, err := testHandler.commitTriageImportRow(newRequest("POST", "/api/triage/imports/commit", nil), parseUUID(batch), triageImportSelection{RowNumber: 1})
			output <- outcome{row, err}
		}(p.BatchID)
	}
	created, skipped := 0, 0
	for range 2 {
		got := <-output
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.row.Status == "created" {
			created++
		}
		if got.row.Status == "skipped" {
			skipped++
		}
	}
	if created != 1 || skipped != 1 {
		t.Fatalf("concurrent batches bypassed external dedup: created=%d skipped=%d", created, skipped)
	}
}

func TestTriageImportThousandRowBaseline(t *testing.T) {
	triageImportSetup(t)
	var data strings.Builder
	data.WriteString("title,description,external_id\n")
	rows := make([]map[string]any, 1000)
	for index := range rows {
		fmt.Fprintf(&data, "CSV baseline %04d,Description,%s-%d\n", index, testWorkspaceID, index)
		rows[index] = map[string]any{"row_number": index + 1}
	}
	start := time.Now()
	p := triageImportPreviewForTest(t, data.String())
	previewTime := time.Since(start)
	start = time.Now()
	result := triageImportCommitForTest(t, p.BatchID, rows...)
	commitTime := time.Since(start)
	if result.Created != 1000 || len(p.Rows) != 1000 {
		t.Fatalf("1000-row limit lost records: preview=%d created=%d failed=%d", len(p.Rows), result.Created, result.Failed)
	}
	t.Logf("1000-row local PostgreSQL baseline: preview=%s commit=%s", previewTime, commitTime)
}

func TestTriageImportDisabledWriteAndMalformedSelection(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "title\nOne\nTwo\n")
	for _, rows := range []any{[]map[string]any{}, []map[string]any{{"row_number": 0}}, []map[string]any{{"row_number": 1}, {"row_number": 1}}, []map[string]any{{"row_number": 1}, {"row_number": 3}}} {
		testutil.Call(t, testHandler.CommitTriageImport, withURLParam(newRequest("POST", "/api/triage/imports/commit", map[string]any{"rows": rows}), "id", p.BatchID)).Want(400)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM triage_import_row WHERE batch_id=$1 AND status!='ready'`, p.BatchID).Scan(&count)
	if count != 0 {
		t.Fatal("invalid selection partially committed")
	}
	dbfx.Exec(t, `UPDATE workspace_triage_settings SET enabled=false WHERE workspace_id=$1`, testWorkspaceID)
	testutil.Call(t, testHandler.CommitTriageImport, withURLParam(newRequest("POST", "/api/triage/imports/commit", map[string]any{"rows": []map[string]any{{"row_number": 1}}}), "id", p.BatchID)).Want(409)
	testutil.Call(t, testHandler.GetTriageImport, withURLParam(newRequest("GET", "/api/triage/imports", nil), "id", p.BatchID)).Want(200)
	testutil.Call(t, testHandler.DownloadTriageImportFailures, withURLParam(newRequest("GET", "/api/triage/imports/failures", nil), "id", p.BatchID)).Want(200)
	testutil.Call(t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "x.csv", "csv": "title\nX"})).Want(409)
}

func TestTriageImportSameRowConcurrentRetryAndLongExternalID(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "title,external_id\nLong ID,"+strings.Repeat("x", 1001)+"\nRetry row,stable-row\n")
	if len(p.Rows[0].Errors) == 0 {
		t.Fatal("overlong indexed external identity must fail during preview")
	}
	type outcome struct {
		row TriageImportRow
		err error
	}
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			row, err := testHandler.commitTriageImportRow(newRequest("POST", "/api/triage/imports/commit", nil), parseUUID(p.BatchID), triageImportSelection{RowNumber: 2})
			results <- outcome{row, err}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.row.IssueID == nil || second.row.IssueID == nil || *first.row.IssueID != *second.row.IssueID {
		t.Fatalf("same-row retry lost identity: %+v %+v", first, second)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Retry row'`, testWorkspaceID).Scan(&count)
	if count != 1 {
		t.Fatal("concurrent same-row retries duplicated issue")
	}
}

func TestTriageImportNULCellPreservesFailureEvidence(t *testing.T) {
	triageImportSetup(t)
	p := triageImportPreviewForTest(t, "title,description\nNUL row,before\x00after\n")
	if len(p.Rows[0].Errors) == 0 {
		t.Fatal("NUL source cell must be a row error")
	}
	result := triageImportCommitForTest(t, p.BatchID, map[string]any{"row_number": 1})
	if result.Failed != 1 || result.Created != 0 {
		t.Fatal("NUL row became an issue")
	}
	response := testutil.Call(t, testHandler.DownloadTriageImportFailures, withURLParam(newRequest("GET", "/api/triage/imports/failures", nil), "id", p.BatchID)).Want(200)
	cells, err := csv.NewReader(strings.NewReader(response.Text())).ReadAll()
	if err != nil || len(cells) != 2 || cells[1][1] != "before\x00after" {
		t.Fatalf("original invalid cell evidence lost: %q %v", response.Text(), err)
	}
	testutil.Call(t, testHandler.PreviewTriageImport, newRequest("POST", "/api/triage/imports/preview", map[string]any{"request_id": uuidToString(dbid.NewV7()), "filename": "x.csv", "csv": "title,invalid\x00header\nTask,cell\n"})).Want(400)
}
