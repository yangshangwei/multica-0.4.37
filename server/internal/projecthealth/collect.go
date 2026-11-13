package projecthealth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Collect only uses q. The caller owns authorization and the RR transaction;
// this function must never borrow a pool connection or start a transaction.
func Collect(ctx context.Context, q *db.Queries, p db.Project, now time.Time) (Collection, error) {
	in, names, err := collectInput(ctx, q, p, now)
	if err != nil {
		return Collection{}, err
	}
	out, err := Compute(in)
	if err != nil {
		return Collection{}, err
	}
	out.StatusNames = names
	return out, nil
}

// CollectInput captures the same input used by Collect, through the caller's
// queries only. Its private reference facts must be projected before disclosure.
func CollectInput(ctx context.Context, q *db.Queries, p db.Project, now time.Time) (Input, error) {
	in, _, err := collectInput(ctx, q, p, now)
	return in, err
}

func collectInput(ctx context.Context, q *db.Queries, p db.Project, now time.Time) (Input, map[string]string, error) {
	ws, err := q.GetWorkspace(ctx, p.WorkspaceID)
	if err != nil {
		return Input{}, nil, err
	}
	in := Input{Project: Project{WorkspaceID: uuid(p.WorkspaceID), ID: uuid(p.ID), Revision: p.Revision, Status: p.Status, DueDate: date(p.DueDate), LeadType: p.LeadType.String, LeadID: uuid(p.LeadID), InProgressSince: timestamp(p.InProgressSince), InProgressSinceSource: p.InProgressSinceSource.String}, Now: now, Timezone: text(ws.PlanningTimezone)}
	statuses, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: p.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return unavailableInput(in, "status_catalog_unavailable", err)
	}
	rows, err := q.ListProjectHealthIssues(ctx, db.ListProjectHealthIssuesParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID})
	if err != nil {
		return unavailableInput(in, "issues_unavailable", err)
	}
	refInputs, err := collectReferenceInputs(ctx, q, p, in)
	if err != nil {
		return Input{}, nil, err
	}
	latest, err := q.GetProjectHealthLatestUpdate(ctx, db.GetProjectHealthLatestUpdateParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID})
	if err != nil {
		return unavailableInput(in, "updates_unavailable", err)
	}
	in.LatestUpdateAt = timestamp(latest)
	for _, s := range statuses {
		in.Statuses = append(in.Statuses, Status{Key: s.Key, Category: s.Category})
	}
	for _, i := range rows {
		in.Issues = append(in.Issues, Issue{ID: uuid(i.ID), Status: i.Status, DueDate: date(i.DueDate), AssigneeType: i.AssigneeType.String, AssigneeID: uuid(i.AssigneeID)})
	}
	in.References = resolveReferences(in, refInputs)
	names := map[string]string{}
	for _, status := range statuses {
		if !status.IsSystem {
			names[status.Key] = status.Name
		}
	}
	return in, names, nil
}

func unavailableInput(in Input, reason string, cause error) (Input, map[string]string, error) {
	_, err := unavailable(in, reason, cause)
	return Input{}, nil, err
}

func uuid(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}
func date(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	return ptr(v.Time.Format(time.DateOnly))
}
func timestamp(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return ptr(v.Time.UTC())
}
func text(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return ptr(v.String)
}
