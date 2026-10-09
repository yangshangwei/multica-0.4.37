package handler

import (
	"fmt"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// The iteration picker lists unplanned open work through include_no_iteration;
// the facet must apply before LIMIT and COUNT and compose with the status
// category facet the picker sends alongside it.
func TestListIssues_IncludeNoIterationIsServerSide(t *testing.T) {
	token := fmt.Sprintf("no-iteration-%d", time.Now().UnixNano())
	metadata := fmt.Sprintf(`{"no_iteration_test":%q}`, token)
	planned := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": token, "timezone": "UTC", "start_date": "2026-10-15", "end_date": "2026-10-28", "created_by": testUserID})
	unplannedTodo := dbfx.Issue(t, "Unplanned todo", testutil.Cols{"metadata": metadata})
	unplannedDone := dbfx.Issue(t, "Unplanned done", testutil.Cols{"metadata": metadata, "status": "done"})
	dbfx.Issue(t, "Planned todo", testutil.Cols{"metadata": metadata, "current_iteration_id": planned})
	dbfx.Issue(t, "Untriaged todo", testutil.Cols{"metadata": metadata, "admission_status": "pending"})

	list := func(query string) ([]string, int64) {
		t.Helper()
		var out struct {
			Issues []IssueResponse `json:"issues"`
			Total  int64           `json:"total"`
		}
		path := "/api/issues?metadata=" + url.QueryEscape(metadata) + query
		testutil.Call(t, testHandler.ListIssues, newRequest("GET", path, nil)).Want(200).JSON(&out)
		ids := make([]string, 0, len(out.Issues))
		for _, issue := range out.Issues {
			ids = append(ids, issue.ID)
		}
		sort.Strings(ids)
		return ids, out.Total
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"&include_no_iteration=true", []string{unplannedTodo, unplannedDone}},
		{"&include_no_iteration=true&status_categories=backlog,todo,in_progress,in_review,blocked", []string{unplannedTodo}},
		// One row per page must still count every unplanned match.
		{"&include_no_iteration=true&limit=1", nil},
	} {
		got, total := list(tc.query)
		if tc.want == nil {
			if len(got) != 1 || total != 2 {
				t.Fatalf("query %q returned %d rows with total %d, want 1 row of total 2", tc.query, len(got), total)
			}
			continue
		}
		sort.Strings(tc.want)
		if fmt.Sprint(got) != fmt.Sprint(tc.want) || total != int64(len(tc.want)) {
			t.Fatalf("query %q ids = %v (total %d), want %v", tc.query, got, total, tc.want)
		}
	}
}
