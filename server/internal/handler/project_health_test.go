package handler

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/projecthealth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func healthProject(t *testing.T) string {
	t.Helper()
	return dbfx.Project(t, "Health contract", testutil.Cols{"lead_type": "member", "lead_id": testUserID})
}
func overview(t *testing.T, id string) ProjectOverview {
	t.Helper()
	var out ProjectOverview
	testutil.Call(t, testHandler.GetProjectOverview, withURLParam(newRequest("GET", "/api/projects/"+id+"/overview", nil), "id", id)).Want(200).JSON(&out)
	return out
}
func healthCount(t *testing.T, name string, actual *int64, want int64) {
	t.Helper()
	if actual == nil || *actual != want {
		t.Fatalf("%s: %v, expected %d", name, actual, want)
	}
}

func TestProjectHealthFormalGoldenScope(t *testing.T) {
	id := healthProject(t)
	for i, s := range []string{"done", "done", "done", "done", "done", "done", "cancelled", "cancelled", "todo", "todo"} {
		dbfx.Issue(t, fmt.Sprintf("golden %d", i), testutil.Cols{"project_id": id, "status": s, "assignee_type": "member", "assignee_id": testUserID})
	}
	for _, s := range []string{"pending", "rejected", "duplicate"} {
		dbfx.Issue(t, s, testutil.Cols{"project_id": id, "admission_status": s, "status": "done"})
	}
	other := healthProject(t)
	dbfx.Issue(t, "another project", testutil.Cols{"project_id": other, "status": "done"})
	out := overview(t, id)
	c := out.Statistics.Counts
	healthCount(t, "total", c.Total, 10)
	healthCount(t, "completed", c.Completed, 6)
	healthCount(t, "cancelled", c.Cancelled, 2)
	healthCount(t, "open", c.Open, 2)
	if out.Statistics.ClosureRatio == nil || *out.Statistics.ClosureRatio != 0.8 || !out.Statistics.Complete {
		t.Fatalf("golden snapshot: %+v", out.Statistics)
	}
	var legacy ProjectResponse
	testutil.Call(t, testHandler.GetProject, withURLParam(newRequest("GET", "/api/projects/"+id, nil), "id", id)).Want(200).JSON(&legacy)
	if legacy.IssueCount != 10 || legacy.DoneCount != 8 {
		t.Fatalf("legacy counts changed: %+v", legacy)
	}
	healthCount(t, "legacy completed additive field", legacy.CompletedIssueCount, 6)
	healthCount(t, "legacy cancelled additive field", legacy.CancelledIssueCount, 2)
	healthCount(t, "legacy open additive field", legacy.OpenIssueCount, 2)
	if !legacy.StatisticsComplete {
		t.Fatal("valid legacy projection marked incomplete")
	}
}

func TestProjectHealthRiskOverlapPaginationAndLiveContinuation(t *testing.T) {
	id := healthProject(t)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
	parentID, childID := uuid.NewString(), uuid.NewString()
	if parentID > childID {
		parentID, childID = childID, parentID
	}
	// The cursor is ordered by ID, not by fixture insertion time.
	parent := dbfx.Issue(t, "risk parent", testutil.Cols{"id": parentID, "project_id": id, "status": "blocked", "due_date": yesterday})
	child := dbfx.Issue(t, "risk child", testutil.Cols{"id": childID, "project_id": id, "parent_issue_id": parent, "status": "todo", "due_date": yesterday})
	dbfx.Issue(t, "due today", testutil.Cols{"project_id": id, "due_date": time.Now().UTC().Format(time.DateOnly), "assignee_type": "member", "assignee_id": testUserID})
	initial := overview(t, id)
	healthCount(t, "overdue", initial.Statistics.Counts.Overdue, 2)
	healthCount(t, "union", initial.Statistics.Counts.RiskUnion, 2)
	page := func(query string, status int) ProjectRiskPage {
		t.Helper()
		var out ProjectRiskPage
		testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?"+query, nil), "id", id)).Want(status).JSON(&out)
		return out
	}
	first := page("signal=overdue&limit=1&snapshot_version="+initial.Statistics.SnapshotVersion, 200)
	if first.Total != 2 || len(first.Items) != 1 || first.NextCursor == nil || first.Refreshed {
		t.Fatalf("first page: %+v", first)
	}
	second := page("signal=overdue&limit=1&cursor="+*first.NextCursor, 200)
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID || second.NextCursor != nil {
		t.Fatalf("second page: %+v", second)
	}
	dbfx.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", child)
	refreshed := page("signal=overdue&limit=1&cursor="+*first.NextCursor, 200)
	if !refreshed.Refreshed || refreshed.Total != 1 || len(refreshed.Items) != 0 || refreshed.NextCursor != nil {
		t.Fatalf("mixed old/new pagination: %+v", refreshed)
	}
	for _, signal := range []string{"blocked", "unassigned"} {
		p := page("signal="+signal, 200)
		if p.Total != 1 || p.Items[0].ID != parent {
			t.Fatalf("overlap %s: %+v", signal, p)
		}
	}
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=all", nil), "id", id)).Want(http.StatusBadRequest)
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=overdue&cursor="+*first.NextCursor+"x", nil), "id", id)).Want(http.StatusBadRequest)
}

func healthRiskPage(t *testing.T, project, query string) ProjectRiskPage {
	t.Helper()
	var out ProjectRiskPage
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+project+"/health/issues?"+query, nil), "id", project)).Want(200).JSON(&out)
	return out
}

func healthOrderedIssue(t *testing.T, project string, n int, status string) string {
	t.Helper()
	return dbfx.Issue(t, fmt.Sprintf("ordered risk %d", n), testutil.Cols{"id": fmt.Sprintf("00000000-0000-7000-8000-%012d", n), "project_id": project, "status": status})
}

func TestProjectHealthLiveCursorReentryAndCurrentSuffix(t *testing.T) {
	project := healthProject(t)
	low := healthOrderedIssue(t, project, 10, "todo")
	anchor := healthOrderedIssue(t, project, 20, "blocked")
	removed := healthOrderedIssue(t, project, 30, "blocked")
	high := healthOrderedIssue(t, project, 40, "blocked")
	first := healthRiskPage(t, project, "signal=blocked&limit=1")
	if first.Items[0].ID != anchor || first.NextCursor == nil {
		t.Fatalf("initial anchor: %+v", first)
	}
	dbfx.Exec(t, "UPDATE issue SET status='blocked' WHERE id=$1", low)
	dbfx.Exec(t, "UPDATE issue SET admission_status='pending' WHERE id=$1", removed)
	inserted := healthOrderedIssue(t, project, 50, "blocked")
	next := healthRiskPage(t, project, "signal=blocked&limit=1&cursor="+*first.NextCursor+"&snapshot_version="+first.SnapshotVersion)
	if !next.Refreshed || next.Total != 4 || len(next.Items) != 1 || next.Items[0].ID != high || next.NextCursor == nil || next.SnapshotVersion != next.Overview.Statistics.SnapshotVersion {
		t.Fatalf("changed snapshot did not continue after anchor: %+v", next)
	}
	last := healthRiskPage(t, project, "signal=blocked&limit=1&cursor="+*next.NextCursor+"&snapshot_version="+next.SnapshotVersion)
	if last.Refreshed || last.Total != 4 || len(last.Items) != 1 || last.Items[0].ID != inserted || last.NextCursor != nil {
		t.Fatalf("returned-version continuation: %+v", last)
	}
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+project+"/health/issues?signal=blocked&cursor="+*next.NextCursor+"&snapshot_version="+first.SnapshotVersion, nil), "id", project)).Want(400)
	restart := healthRiskPage(t, project, "signal=blocked&limit=1&snapshot_version="+first.SnapshotVersion)
	if !restart.Refreshed || restart.Items[0].ID != low {
		t.Fatalf("explicit first-page refresh missed reentered low ID: %+v", restart)
	}
	var expected []string
	dbfx.QueryRow(t, "SELECT array_agg(id::text ORDER BY id) FROM issue WHERE workspace_id=$1 AND project_id=$2 AND admission_status IN ('not_required','accepted') AND status='blocked'", testWorkspaceID, project).Scan(&expected)
	seen := []string{restart.Items[0].ID}
	page := restart
	for page.NextCursor != nil {
		page = healthRiskPage(t, project, "signal=blocked&limit=1&cursor="+*page.NextCursor+"&snapshot_version="+page.SnapshotVersion)
		if page.Refreshed || page.Total != int64(len(expected)) {
			t.Fatalf("stable traversal lost snapshot: %+v", page)
		}
		for _, row := range page.Items {
			seen = append(seen, row.ID)
		}
	}
	if !reflect.DeepEqual(seen, expected) {
		t.Fatalf("full current formal scope: got %v want %v", seen, expected)
	}
}

func TestProjectHealthLiveCursorSurvivesMissingOrNonformalAnchor(t *testing.T) {
	for _, change := range []string{"delete", "move", "pending", "rejected", "duplicate"} {
		t.Run(change, func(t *testing.T) {
			project := healthProject(t)
			other := healthProject(t)
			healthOrderedIssue(t, project, 10, "blocked")
			anchor := healthOrderedIssue(t, project, 20, "blocked")
			high := healthOrderedIssue(t, project, 30, "blocked")
			first := healthRiskPage(t, project, "signal=blocked&limit=2")
			if first.NextCursor == nil || first.Items[1].ID != anchor {
				t.Fatalf("initial cursor: %+v", first)
			}
			switch change {
			case "delete":
				dbfx.Exec(t, "DELETE FROM issue WHERE id=$1", anchor)
			case "move":
				dbfx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", anchor, other)
			default:
				dbfx.Exec(t, "UPDATE issue SET admission_status=$2 WHERE id=$1", anchor, change)
			}
			next := healthRiskPage(t, project, "signal=blocked&limit=2&cursor="+*first.NextCursor)
			if !next.Refreshed || next.Total != 2 || len(next.Items) != 1 || next.Items[0].ID != high || next.NextCursor != nil {
				t.Fatalf("%s anchor changed current suffix: %+v", change, next)
			}
		})
	}
}

func TestProjectHealthLiveCursorEmptySuffixRetainsFullRiskTotal(t *testing.T) {
	project := healthProject(t)
	low := healthOrderedIssue(t, project, 10, "todo")
	healthOrderedIssue(t, project, 20, "blocked")
	tail := healthOrderedIssue(t, project, 30, "blocked")
	first := healthRiskPage(t, project, "signal=blocked&limit=1")
	dbfx.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", tail)
	dbfx.Exec(t, "UPDATE issue SET status='blocked' WHERE id=$1", low)
	next := healthRiskPage(t, project, "signal=blocked&limit=1&cursor="+*first.NextCursor)
	if !next.Refreshed || len(next.Items) != 0 || next.NextCursor != nil || next.Total != 2 || *next.Overview.Statistics.Counts.Blocked != 2 {
		t.Fatalf("empty suffix became empty project or reset: %+v", next)
	}
	restarted := healthRiskPage(t, project, "signal=blocked&limit=1")
	if restarted.Items[0].ID != low || restarted.Total != 2 {
		t.Fatalf("from-start did not include risk before anchor: %+v", restarted)
	}
}

func TestProjectHealthArchivedCustomStatusAndUnknown(t *testing.T) {
	id := healthProject(t)
	dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": testWorkspaceID, "key": "health_custom_review", "name": "Historical review", "color": "#123456", "category": "in_review", "archived_at": time.Now()})
	dbfx.Issue(t, "custom", testutil.Cols{"project_id": id, "status": "health_custom_review", "assignee_type": "member", "assignee_id": testUserID})
	first := overview(t, id)
	healthCount(t, "archived review", first.Statistics.Counts.InReview, 1)
	dbfx.Exec(t, "UPDATE issue_status SET name='Renamed label' WHERE workspace_id=$1 AND key='health_custom_review'", testWorkspaceID)
	if overview(t, id).Statistics.SnapshotVersion != first.Statistics.SnapshotVersion {
		t.Fatal("visible label changed canonical facts")
	}
	dbfx.Issue(t, "unknown", testutil.Cols{"project_id": id, "status": "health_unknown", "assignee_type": "member", "assignee_id": testUserID})
	unknown := overview(t, id)
	healthCount(t, "unknown retained", unknown.Statistics.Counts.Total, 2)
	healthCount(t, "unknown open", unknown.Statistics.Counts.Open, 2)
	if unknown.Statistics.Complete || unknown.Statistics.Health != "unavailable" {
		t.Fatalf("unknown status success: %+v", unknown)
	}
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=in_review", nil), "id", id)).Want(503)
}

func TestProjectHealthRepeatableReadUsesCallerTransaction(t *testing.T) {
	id := healthProject(t)
	issueID := dbfx.Issue(t, "concurrent task", testutil.Cols{"project_id": id, "status": "todo"})
	ctx := context.Background()
	var first projecthealth.Collection
	err := testHandler.runProjectTransaction(ctx, parseUUID(testWorkspaceID), parseUUID(testUserID), func(_ pgx.Tx, q *db.Queries) error {
		p, e := q.LockProjectForAssociation(ctx, db.LockProjectForAssociationParams{ID: parseUUID(id), WorkspaceID: parseUUID(testWorkspaceID)})
		if e != nil {
			return e
		}
		first, e = projecthealth.Collect(ctx, q, p, time.Now())
		if e != nil {
			return e
		}
		if _, e = testPool.Exec(ctx, "UPDATE issue SET status='done' WHERE id=$1", issueID); e != nil {
			return e
		}
		second, e := projecthealth.Collect(ctx, q, p, time.Now())
		if e != nil {
			return e
		}
		if first.Statistics.SnapshotVersion != second.Statistics.SnapshotVersion || *second.Statistics.Counts.Completed != 0 {
			t.Fatal("collector escaped caller RR snapshot")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	next := overview(t, id)
	if *next.Statistics.Counts.Completed != 1 || next.Statistics.SnapshotVersion == first.Statistics.SnapshotVersion {
		t.Fatal("next transaction did not observe committed changes")
	}
}

func TestProjectHealthAssigneeLifecycleAndRuntimeAreSeparate(t *testing.T) {
	id := healthProject(t)
	outsider := dbfx.User(t, "Private agent owner", "health-private-owner@example.test")
	runtime := dbfx.Runtime(t, "offline health", testutil.Cols{"status": "offline"})
	offline := dbfx.Agent(t, "private offline", runtime, testutil.Cols{"owner_id": outsider})
	archived := dbfx.Agent(t, "archived reference", runtime, testutil.Cols{"archived_at": time.Now()})
	departed := dbfx.User(t, "Departed assignee", "health-departed@example.test")
	leaderRuntime := dbfx.Runtime(t, "leader online")
	leader := dbfx.Agent(t, "leader", leaderRuntime)
	squad := dbfx.Squad(t, "mixed availability squad", leader)
	dbfx.SquadMember(t, squad, "agent", leader)
	dbfx.SquadMember(t, squad, "agent", offline)
	brokenSquad := dbfx.Squad(t, "archived leader squad", archived)
	for _, a := range []struct{ kind, id string }{{"member", testUserID}, {"member", departed}, {"agent", offline}, {"agent", archived}, {"squad", squad}, {"squad", brokenSquad}} {
		dbfx.Issue(t, a.kind+" "+a.id, testutil.Cols{"project_id": id, "status": "todo", "assignee_type": a.kind, "assignee_id": a.id})
	}
	before := overview(t, id)
	healthCount(t, "invalid references", before.Statistics.Counts.Unassigned, 3)
	healthCount(t, "valid offline references", before.Statistics.Counts.ExecutionEnvironmentUnavailable, 2)
	dbfx.Exec(t, "UPDATE agent_runtime SET status='online' WHERE id=$1", runtime)
	after := overview(t, id)
	healthCount(t, "environment recovered", after.Statistics.Counts.ExecutionEnvironmentUnavailable, 0)
	healthCount(t, "lifecycle unchanged", after.Statistics.Counts.Unassigned, 3)
	if before.Statistics.SnapshotVersion == after.Statistics.SnapshotVersion {
		t.Fatal("runtime change not versioned")
	}
}

func TestProjectHealthAcceptanceSelectionUsesPublicationAndDescription(t *testing.T) {
	id := healthProject(t)
	dbfx.Exec(t, "UPDATE project SET description_revision=2 WHERE id=$1", id)
	author := dbfx.User(t, "Former author", "health-former-author@example.test")
	publish := func(at string, descriptionRevision int64, conclusion string) string {
		t.Helper()
		update := dbfx.Insert(t, "project_update", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": id, "author_user_id": author, "published_at": at})
		acceptance := fmt.Sprintf(`{"conclusion":%q,"scope":"scope","explanation":"evidence","description_revision":%d,"description_snapshot":"frozen"}`, conclusion, descriptionRevision)
		dbfx.Insert(t, "project_update_revision", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": id, "update_id": update, "revision": 1, "editor_user_id": author, "kind": "acceptance", "body": "acceptance", "acceptance": acceptance})
		return update
	}
	publish("2026-10-01T10:00:00Z", 2, "passed")
	current := publish("2026-10-02T10:00:00Z", 2, "failed")
	latest := publish("2026-10-03T10:00:00Z", 1, "partial")
	out := overview(t, id)
	if out.LatestAcceptance == nil || out.CurrentDescriptionAcceptance == nil || out.LatestAcceptance.UpdateID != latest || out.LatestAcceptance.ApplicableToCurrentDescription || out.CurrentDescriptionAcceptance.UpdateID != current || out.CurrentDescriptionAcceptance.Conclusion != "failed" {
		t.Fatalf("acceptance selection: %+v", out)
	}
	if out.LatestAcceptance.Author.Availability != "departed" || out.LatestAcceptance.Author.Name == nil {
		t.Fatalf("historical attribution: %+v", out.LatestAcceptance.Author)
	}
	dbfx.Insert(t, "project_update_revision", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": id, "update_id": current, "revision": 2, "editor_user_id": testUserID, "kind": "acceptance", "body": "corrected", "correction_reason": "new explanation", "acceptance": `{"conclusion":"failed","scope":"scope","description_revision":2,"description_snapshot":"frozen"}`})
	dbfx.Exec(t, "UPDATE project_update SET current_revision=2 WHERE id=$1", current)
	corrected := overview(t, id)
	if corrected.LatestAcceptance.UpdateID != latest || corrected.CurrentDescriptionAcceptance.Revision != 2 || corrected.Statistics.SnapshotVersion != out.Statistics.SnapshotVersion {
		t.Fatal("correction changed publication ordering or statistics version")
	}
	dbfx.Exec(t, "DELETE FROM \"user\" WHERE id=$1", author)
	deleted := overview(t, id).LatestAcceptance.Author
	if deleted.ID != author || deleted.Availability != "deleted" || deleted.Name != nil || deleted.AvatarURL != nil {
		t.Fatalf("deleted author leaked or lost identity: %+v", deleted)
	}
}

func TestProjectHealthBoundariesAndNoTaskSideEffects(t *testing.T) {
	id := healthProject(t)
	foreign := dbfx.Insert(t, "workspace", testutil.Cols{"name": "Health foreign", "slug": "health-foreign"})
	dbfx.Issue(t, "foreign inconsistent project", testutil.Cols{"workspace_id": foreign, "project_id": id, "status": "done"})
	out := overview(t, id)
	healthCount(t, "cross-workspace excluded", out.Statistics.Counts.Total, 0)
	if out.Statistics.Health != "empty" || out.Statistics.ClosureRatio != nil {
		t.Fatalf("empty: %+v", out)
	}
	outsider := dbfx.User(t, "Health outsider", "health-outsider@example.test")
	request := withURLParam(newRequest("GET", "/api/projects/"+id+"/overview", nil), "id", id)
	request.Header.Set("X-User-ID", outsider)
	testutil.Call(t, testHandler.GetProjectOverview, request).Want(403)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE project_id=$1)", id); n != 0 {
		t.Fatalf("read enqueued %d executions", n)
	}
}

func TestProjectHealthQueryFailureDoesNotBecomeZero(t *testing.T) {
	id := healthProject(t)
	// An actual PostgreSQL search path fault leaves authorization/project rows
	// readable while the status input is unavailable. No database mock is used.
	dbfx.Exec(t, "CREATE SCHEMA health_missing_status")
	dbfx.Cleanup(t, "DROP SCHEMA health_missing_status CASCADE")
	for _, table := range []string{"workspace", "member", "project"} {
		dbfx.Exec(t, "CREATE VIEW health_missing_status."+table+" AS SELECT * FROM public."+table)
	}
	config := testPool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = "health_missing_status,pg_catalog"
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := *testHandler
	h.Queries = db.New(pool)
	h.TxStarter = pool
	var out map[string]any
	testutil.Call(t, h.GetProjectOverview, withURLParam(newRequest("GET", "/api/projects/"+id+"/overview", nil), "id", id)).Want(200).JSON(&out)
	statistics, ok := out["statistics"].(map[string]any)
	if !ok || statistics["complete"] != false || statistics["health"] != "unavailable" {
		t.Fatalf("missing SQL input became success: %v", out)
	}
	counts := statistics["counts"].(map[string]any)
	if counts["total"] != nil || counts["completed"] != nil || counts["overdue"] != nil {
		t.Fatalf("unavailable input became zero: %v", counts)
	}
	testutil.Call(t, h.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=blocked", nil), "id", id)).Want(503)
}

func TestProjectHealthCursorScopeAndLimitValidation(t *testing.T) {
	id := healthProject(t)
	other := healthProject(t)
	for n := 0; n < 2; n++ {
		dbfx.Issue(t, "blocked", testutil.Cols{"project_id": id, "status": "blocked"})
	}
	var first ProjectRiskPage
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=blocked&limit=1", nil), "id", id)).Want(200).JSON(&first)
	if first.NextCursor == nil {
		t.Fatal("missing next cursor")
	}
	for _, tc := range []struct{ project, query string }{
		{other, "signal=blocked&cursor=" + *first.NextCursor},
		{id, "signal=overdue&cursor=" + *first.NextCursor},
		{id, "signal=blocked&limit=101"},
		{id, "signal=blocked&limit=0"},
	} {
		testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+tc.project+"/health/issues?"+tc.query, nil), "id", tc.project)).Want(400)
	}
	var all ProjectRiskPage
	testutil.Call(t, testHandler.GetProjectHealthIssues, withURLParam(newRequest("GET", "/api/projects/"+id+"/health/issues?signal=blocked&assignee_id=personal-filter&show_sub_issues=false", nil), "id", id)).Want(200).JSON(&all)
	if all.Total != 2 || len(all.Items) != 2 {
		t.Fatalf("personal filters changed health scope: %+v", all)
	}
}

func TestProjectHealthBatchCountsMatchIndividualSnapshots(t *testing.T) {
	first, empty, unknown := healthProject(t), healthProject(t), healthProject(t)
	dbfx.Issue(t, "completed", testutil.Cols{"project_id": first, "status": "done"})
	dbfx.Issue(t, "cancelled", testutil.Cols{"project_id": first, "status": "cancelled"})
	dbfx.Issue(t, "unknown", testutil.Cols{"project_id": unknown, "status": "not_in_catalog"})
	ids := []pgtype.UUID{parseUUID(first), parseUUID(empty), parseUUID(unknown)}
	counts, complete, err := projecthealth.CollectProjectCounts(context.Background(), testHandler.Queries, parseUUID(testWorkspaceID), ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first, empty, unknown} {
		current := overview(t, id)
		if *counts[id].Total != *current.Statistics.Counts.Total || *counts[id].Completed != *current.Statistics.Counts.Completed || *counts[id].Cancelled != *current.Statistics.Counts.Cancelled || *counts[id].Open != *current.Statistics.Counts.Open || complete[id] != current.Statistics.Complete {
			t.Fatalf("batch projection differs: %s", id)
		}
	}
	if !complete[first] || !complete[empty] || complete[unknown] {
		t.Fatalf("cross-project completeness: %+v", complete)
	}
}

func TestProjectHealthClosedScopeDoesNotImplyAcceptanceOrChangeProjectState(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		statuses             []string
		completed, cancelled int64
	}{
		{"all_done", []string{"done", "done"}, 2, 0},
		{"all_cancelled", []string{"cancelled", "cancelled"}, 0, 2},
		{"mixed_done_cancelled", []string{"done", "cancelled"}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := healthProject(t)
			for _, status := range tc.statuses {
				dbfx.Issue(t, "Closed scope "+status, testutil.Cols{"project_id": project, "status": status})
			}
			result := overview(t, project)
			healthCount(t, "total", result.Statistics.Counts.Total, 2)
			healthCount(t, "completed", result.Statistics.Counts.Completed, tc.completed)
			healthCount(t, "cancelled", result.Statistics.Counts.Cancelled, tc.cancelled)
			healthCount(t, "open", result.Statistics.Counts.Open, 0)
			if !result.Statistics.Complete || result.Statistics.ClosureRatio == nil || *result.Statistics.ClosureRatio != 1 {
				t.Fatalf("closed formal scope: %+v", result.Statistics)
			}
			if result.LatestAcceptance != nil || result.CurrentDescriptionAcceptance != nil {
				t.Fatalf("closure inferred project acceptance: %+v", result)
			}
			var status string
			dbfx.QueryRow(t, "SELECT status FROM project WHERE workspace_id=$1 AND id=$2", testWorkspaceID, project).Scan(&status)
			if status != "planned" {
				t.Fatalf("closure changed project status to %q", status)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE workspace_id=$1 AND project_id=$2", testWorkspaceID, project); n != 0 {
				t.Fatalf("closure created %d progress/acceptance records", n)
			}
		})
	}
}
