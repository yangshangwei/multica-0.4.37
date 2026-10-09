package handler

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestHistoricalIterationFilterOptions(t *testing.T) {
	project, actor, label := uuid.NewString(), uuid.NewString(), uuid.NewString()
	items := []iteration.HistoricalIssue{
		{IssueID: "b", StatusKey: "retired", ProjectID: &project, ProjectName: strPtr("Frozen project"), AssigneeType: strPtr("agent"), AssigneeID: &actor, AssigneeName: strPtr("Frozen agent"), Labels: []iteration.HistoricalLabel{{ID: label, Name: "Frozen label"}}},
		{IssueID: "a", StatusKey: "todo", AssigneeType: strPtr("member"), AssigneeID: &actor, AssigneeName: strPtr("Frozen member")},
		{IssueID: "c", StatusKey: "todo"},
		{IssueID: "d", StatusKey: "retired", ProjectID: &project, ProjectName: strPtr("Frozen project"), Labels: []iteration.HistoricalLabel{{ID: label, Name: "Frozen label"}}},
	}
	options := historicalIterationFilterOptions(items)
	if !reflect.DeepEqual(options.Statuses, []string{"retired", "todo"}) || len(options.Projects) != 2 || len(options.Assignees) != 3 || len(options.Labels) != 1 {
		t.Fatalf("options must be distinct by stored/typed identity: %+v", options)
	}
	if options.Projects[1].Name == nil || *options.Projects[1].Name != "Frozen project" || options.Assignees[1].Type == nil || *options.Assignees[1].Type != "agent" || options.Assignees[2].Type == nil || *options.Assignees[2].Type != "member" || options.Labels[0].Name != "Frozen label" {
		t.Fatalf("stored display names lost: %+v", options)
	}
	items[0], items[1] = items[1], items[0]
	if !reflect.DeepEqual(options, historicalIterationFilterOptions(items)) {
		t.Fatal("input order must not change the filter metadata")
	}
	raw, err := json.Marshal(historicalIterationFilterOptions(nil))
	if err != nil || string(raw) != `{"statuses":[],"projects":[],"assignees":[],"labels":[]}` {
		t.Fatalf("known empty scope must produce empty arrays: %s, %v", raw, err)
	}
}

func TestIterationIssuesFilterOptionsCoverUnfilteredFrozenProjection(t *testing.T) {
	f := newHistoryFixture(t)
	at := f.start.Add(time.Hour)
	// The facet-bearing task must be beyond the first UUID-ordered task page.
	facetID := ""
	for _, id := range f.issues {
		if id > facetID {
			facetID = id
		}
	}
	project := dbfx.Project(t, "Live project")
	label := dbfx.Insert(t, "issue_label", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Live label", "color": "#888888"})
	dbfx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": facetID, "label_id": label}, "issue_id=$1 AND label_id=$2", facetID, label)
	history := loadHistoryFixture(t, f, at)
	facetIdentifier := ""
	// Original and closeout scope can have different stored display facts.
	for index := range history.Original {
		if history.Original[index].IssueID == facetID {
			history.Original[index].ProjectID = &project
			history.Original[index].ProjectName = strPtr("Original project")
			history.Original[index].StatusKey = "original_status"
		}
	}
	for index := range history.Scope {
		if history.Scope[index].IssueID == facetID {
			facetIdentifier = history.Scope[index].Identifier
			history.Scope[index].ProjectID = &project
			history.Scope[index].ProjectName = strPtr("Frozen project")
			history.Scope[index].AssigneeType = strPtr("member")
			history.Scope[index].AssigneeID = &testUserID
			history.Scope[index].AssigneeName = strPtr("Frozen member")
			history.Scope[index].StatusKey = "frozen_status"
			history.Scope[index].Labels = []iteration.HistoricalLabel{{ID: label, Name: "Frozen label"}}
		}
	}
	destinations := make([]iteration.Destination, 0, len(history.Scope))
	for _, item := range history.Scope {
		destinations = append(destinations, iteration.Destination{IssueID: item.IssueID, RolloverCountBefore: item.RolloverCount, RolloverCountAfter: item.RolloverCount})
	}
	snapshot, err := iteration.BuildSnapshot(history, uuid.NewString(), "completed", "Completed", at, at, destinations)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.InsertNoID(t, "iteration_snapshot", testutil.Cols{"workspace_id": testWorkspaceID, "iteration_id": f.id, "operation_id": snapshot.OperationID, "body": body, "created_at": at}, "iteration_id=$1", f.id)
	dbfx.Exec(t, "UPDATE iteration SET status='completed',logical_ended_at=$2,processed_at=$2 WHERE id=$1", f.id, at)
	dbfx.Exec(t, "UPDATE project SET title='Renamed live project' WHERE id=$1", project)
	dbfx.Exec(t, "UPDATE issue_label SET name='Renamed live label' WHERE id=$1", label)

	var unfiltered, filtered, original struct {
		Items   []iteration.HistoricalIssue `json:"items"`
		Total   int                         `json:"total"`
		Options iterationIssueFilterOptions `json:"filter_options"`
	}
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=1")).Want(200).JSON(&unfiltered)
	if len(unfiltered.Items) != 1 || unfiltered.Total != 8 || unfiltered.Items[0].IssueID == facetID {
		t.Fatalf("task page should remain paginated: %+v", unfiltered)
	}
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=1&search=no-matches&status=todo&project_id=null")).Want(200).JSON(&filtered)
	if filtered.Total != 0 || !reflect.DeepEqual(unfiltered.Options, filtered.Options) {
		t.Fatalf("search and task filters must not shrink available choices: %+v", filtered)
	}
	for _, field := range []string{"search=" + url.QueryEscape(facetIdentifier), "status=frozen_status", "project_id=" + url.QueryEscape(project), "assignee_id=" + url.QueryEscape(testUserID), "label_id=" + url.QueryEscape(label)} {
		testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?limit=1&"+field)).Want(200).JSON(&filtered)
		if filtered.Total != 1 || filtered.Items[0].IssueID != facetID || !reflect.DeepEqual(filtered.Options, unfiltered.Options) {
			t.Fatalf("stored choice %s must select its frozen task: %+v", field, filtered)
		}
	}
	if !reflect.DeepEqual(unfiltered.Options, historicalIterationFilterOptions(snapshot.Scope)) {
		t.Fatalf("scope choices did not use the complete frozen projection: %+v", unfiltered.Options)
	}
	testutil.Call(t, testHandler.ListIterationIssues, historyRequest(f.id, "?scope=original&limit=1")).Want(200).JSON(&original)
	if !reflect.DeepEqual(original.Options, historicalIterationFilterOptions(snapshot.Original)) || reflect.DeepEqual(original.Options, unfiltered.Options) {
		t.Fatalf("original choices must use their own stored projection: %+v", original.Options)
	}
}
