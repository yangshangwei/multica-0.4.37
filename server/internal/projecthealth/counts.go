package projecthealth

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func statusCategories(statuses []Status) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"} {
		out[key] = key
	}
	for _, s := range statuses {
		switch s.Category {
		case "backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled":
			out[s.Key] = s.Category
		}
	}
	return out
}

func countScope(byStatus map[string]int64, categories map[string]string) Counts {
	c := Counts{Total: ptr(int64(0)), Completed: ptr(int64(0)), Cancelled: ptr(int64(0)), Open: ptr(int64(0)), UnknownStatus: ptr(int64(0))}
	for status, n := range byStatus {
		*c.Total += n
		category, known := categories[status]
		if !known {
			*c.UnknownStatus += n
		}
		switch category {
		case "done":
			*c.Completed += n
		case "cancelled":
			*c.Cancelled += n
		default:
			*c.Open += n
		}
	}
	return c
}

// CollectProjectCounts is the list/detail compatibility projection of the same
// status classification used by Compute. It performs two queries for any number
// of projects, and does not pretend to have collected risk or runtime inputs.
func CollectProjectCounts(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, projectIDs []pgtype.UUID) (map[string]Counts, map[string]bool, error) {
	statuses, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: workspaceID, IncludeArchived: true})
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.ListProjectHealthStatusCounts(ctx, db.ListProjectHealthStatusCountsParams{WorkspaceID: workspaceID, ProjectIds: projectIDs})
	if err != nil {
		return nil, nil, err
	}
	catalog := make([]Status, 0, len(statuses))
	for _, s := range statuses {
		catalog = append(catalog, Status{Key: s.Key, Category: s.Category})
	}
	categories := statusCategories(catalog)
	groups := map[string]map[string]int64{}
	for _, id := range projectIDs {
		groups[uuid(id)] = map[string]int64{}
	}
	for _, row := range rows {
		id := uuid(row.ProjectID)
		if groups[id] != nil {
			groups[id][row.Status] += row.IssueCount
		}
	}
	counts := map[string]Counts{}
	complete := map[string]bool{}
	for id, group := range groups {
		counts[id] = countScope(group, categories)
		complete[id] = *counts[id].UnknownStatus == 0
	}
	return counts, complete, nil
}

func CollectCounts(ctx context.Context, q *db.Queries, workspaceID, projectID pgtype.UUID) (Counts, bool, error) {
	counts, complete, err := CollectProjectCounts(ctx, q, workspaceID, []pgtype.UUID{projectID})
	if err != nil {
		return Counts{}, false, err
	}
	return counts[uuid(projectID)], complete[uuid(projectID)], nil
}
