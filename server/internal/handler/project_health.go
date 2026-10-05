package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projecthealth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ProjectAcceptanceSummary struct {
	UpdateID                       string                  `json:"update_id"`
	Revision                       int64                   `json:"revision"`
	PublishedAt                    time.Time               `json:"published_at"`
	Conclusion                     string                  `json:"conclusion"`
	DescriptionRevision            int64                   `json:"description_revision"`
	ApplicableToCurrentDescription bool                    `json:"applicable_to_current_description"`
	Author                         ProjectHistoricalMember `json:"author"`
}
type ProjectOverview struct {
	WorkspaceID                  string                           `json:"workspace_id"`
	ProjectID                    string                           `json:"project_id"`
	DescriptionRevision          int64                            `json:"description_revision"`
	Statistics                   projecthealth.StatisticsSnapshot `json:"statistics"`
	LatestAcceptance             *ProjectAcceptanceSummary        `json:"latest_acceptance"`
	CurrentDescriptionAcceptance *ProjectAcceptanceSummary        `json:"current_description_acceptance"`
}
type ProjectRiskPage struct {
	WorkspaceID     string          `json:"workspace_id"`
	ProjectID       string          `json:"project_id"`
	Signal          string          `json:"signal"`
	Items           []IssueResponse `json:"items"`
	Total           int64           `json:"total"`
	SnapshotVersion string          `json:"snapshot_version"`
	Refreshed       bool            `json:"refreshed"`
	Overview        ProjectOverview `json:"overview"`
	NextCursor      *string         `json:"next_cursor"`
}
type projectRiskCursor struct {
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	Signal      string `json:"signal"`
	Version     string `json:"version"`
	LastID      string `json:"last_id"`
}

func (h *Handler) projectHealthScope(r *http.Request) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	p, err := util.ParseUUID(chi.URLParam(r, "id"))
	if err != nil {
		return p, pgtype.UUID{}, pgtype.UUID{}, projectErr(400, "invalid_request", "invalid project id")
	}
	ws, err := util.ParseUUID(h.resolveWorkspaceID(r))
	if err != nil {
		return p, ws, pgtype.UUID{}, projectErr(400, "invalid_request", "invalid workspace id")
	}
	actor, err := util.ParseUUID(requestUserID(r))
	if err != nil {
		return p, ws, actor, projectErr(401, "unauthenticated", "user not authenticated")
	}
	return p, ws, actor, nil
}

func projectHealthOverview(ctx context.Context, q *db.Queries, p db.Project, collection projecthealth.Collection) (ProjectOverview, error) {
	out := ProjectOverview{WorkspaceID: uuidToString(p.WorkspaceID), ProjectID: uuidToString(p.ID), DescriptionRevision: p.DescriptionRevision, Statistics: collection.Statistics}
	rows, err := q.ListProjectHealthAcceptanceSummaries(ctx, db.ListProjectHealthAcceptanceSummariesParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID, DescriptionRevision: p.DescriptionRevision})
	if err != nil {
		return out, err
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AuthorUserID)
	}
	members, err := projectUpdateMembers(ctx, q, p.WorkspaceID, ids)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		var acceptance struct {
			Conclusion          string `json:"conclusion"`
			DescriptionRevision int64  `json:"description_revision"`
		}
		if err := json.Unmarshal(row.Acceptance, &acceptance); err != nil {
			return out, err
		}
		if acceptance.DescriptionRevision < 1 || (acceptance.Conclusion != "passed" && acceptance.Conclusion != "partial" && acceptance.Conclusion != "failed") {
			return out, errors.New("invalid stored project acceptance")
		}
		summary := &ProjectAcceptanceSummary{UpdateID: uuidToString(row.ID), Revision: row.CurrentRevision, PublishedAt: row.PublishedAt.Time.UTC(), Conclusion: acceptance.Conclusion, DescriptionRevision: acceptance.DescriptionRevision, ApplicableToCurrentDescription: acceptance.DescriptionRevision == p.DescriptionRevision, Author: members[row.AuthorUserID]}
		if out.LatestAcceptance == nil {
			out.LatestAcceptance = summary
		}
		if summary.ApplicableToCurrentDescription {
			out.CurrentDescriptionAcceptance = summary
		}
	}
	return out, nil
}

func (h *Handler) GetProjectOverview(w http.ResponseWriter, r *http.Request) {
	p, ws, actor, err := h.projectHealthScope(r)
	var out ProjectOverview
	if err == nil {
		err = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
			project, e := q.LockProjectForAssociation(r.Context(), db.LockProjectForAssociationParams{ID: p, WorkspaceID: ws})
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_not_found", "project not found")
			}
			if e != nil {
				return e
			}
			collection, e := projecthealth.Collect(r.Context(), q, project, time.Now())
			if e != nil {
				var input *projecthealth.UnavailableError
				if errors.As(e, &input) {
					out = ProjectOverview{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(p), DescriptionRevision: project.DescriptionRevision, Statistics: input.Statistics}
				}
				return e
			}
			out, e = projectHealthOverview(r.Context(), q, project, collection)
			return e
		})
	}
	if err != nil {
		var input *projecthealth.UnavailableError
		if errors.As(err, &input) {
			writeJSON(w, 200, out)
			return
		}
		writeProjectHealthError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func writeProjectHealthError(w http.ResponseWriter, err error) {
	var api *projectAPIError
	if !errors.As(err, &api) {
		err = projectErr(503, "project_health_unavailable", "project health is temporarily unavailable")
	}
	writeProjectAPIError(w, err)
}

func (h *Handler) GetProjectHealthIssues(w http.ResponseWriter, r *http.Request) {
	p, ws, actor, err := h.projectHealthScope(r)
	if err != nil {
		writeProjectHealthError(w, err)
		return
	}
	signal := r.URL.Query().Get("signal")
	if signal != "blocked" && signal != "overdue" && signal != "unassigned" && signal != "in_review" {
		writeProjectHealthError(w, projectErr(400, "invalid_request", "invalid health signal"))
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			writeProjectHealthError(w, projectErr(400, "invalid_request", "limit must be between 1 and 100"))
			return
		}
		limit = n
	}
	version := r.URL.Query().Get("snapshot_version")
	var cursor projectRiskCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		data, e := base64.RawURLEncoding.DecodeString(raw)
		if e == nil {
			e = json.Unmarshal(data, &cursor)
		}
		_, idErr := util.ParseUUID(cursor.LastID)
		if e != nil || idErr != nil || cursor.WorkspaceID != uuidToString(ws) || cursor.ProjectID != uuidToString(p) || cursor.Signal != signal || cursor.Version == "" || (version != "" && version != cursor.Version) {
			writeProjectHealthError(w, projectErr(400, "invalid_request", "invalid health cursor"))
			return
		}
		version = cursor.Version
	}
	var out ProjectRiskPage
	err = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		project, e := q.LockProjectForAssociation(r.Context(), db.LockProjectForAssociationParams{ID: p, WorkspaceID: ws})
		if errors.Is(e, pgx.ErrNoRows) {
			return projectErr(404, "project_not_found", "project not found")
		}
		if e != nil {
			return e
		}
		collection, e := projecthealth.Collect(r.Context(), q, project, time.Now())
		if e != nil {
			var input *projecthealth.UnavailableError
			if errors.As(e, &input) {
				out.Overview = ProjectOverview{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(p), DescriptionRevision: project.DescriptionRevision, Statistics: input.Statistics}
			}
			return e
		}
		overview, e := projectHealthOverview(r.Context(), q, project, collection)
		if e != nil {
			return e
		}
		if !collection.Statistics.Complete {
			return &projectAPIError{Status: 503, Code: "project_health_unavailable", Message: "project statistics are incomplete", Current: overview}
		}
		ids := collection.Risks[signal]
		current := collection.Statistics.SnapshotVersion
		refreshed := version != "" && version != current
		start := 0
		// A changed snapshot does not discard the ordering boundary. Recompute
		// the live suffix so concurrent edits cannot starve every later page.
		if cursor.LastID != "" {
			start = sort.Search(len(ids), func(i int) bool { return ids[i] > cursor.LastID })
		}
		end := min(start+limit, len(ids))
		pageIDs := make([]pgtype.UUID, 0, end-start)
		for _, id := range ids[start:end] {
			pageIDs = append(pageIDs, parseUUID(id))
		}
		rows, e := q.GetProjectHealthIssueRows(r.Context(), db.GetProjectHealthIssueRowsParams{WorkspaceID: ws, ProjectID: p, IssueIds: pageIDs})
		if e != nil {
			return e
		}
		if len(rows) != len(pageIDs) {
			return errors.New("health page membership differs within snapshot")
		}
		workspace, e := q.GetWorkspace(r.Context(), ws)
		if e != nil {
			return e
		}
		out = ProjectRiskPage{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(p), Signal: signal, Items: []IssueResponse{}, Total: int64(len(ids)), SnapshotVersion: current, Refreshed: refreshed, Overview: overview}
		for _, row := range rows {
			item := issueToResponse(row, workspace.IssuePrefix)
			item.StatusCategory = collection.Categories[row.Status]
			item.StatusName = collection.StatusNames[row.Status]
			out.Items = append(out.Items, item)
		}
		if end < len(ids) {
			encoded, e := json.Marshal(projectRiskCursor{WorkspaceID: out.WorkspaceID, ProjectID: out.ProjectID, Signal: signal, Version: current, LastID: ids[end-1]})
			if e != nil {
				return e
			}
			next := base64.RawURLEncoding.EncodeToString(encoded)
			out.NextCursor = &next
		}
		return nil
	})
	if err != nil {
		var input *projecthealth.UnavailableError
		if errors.As(err, &input) {
			err = &projectAPIError{Status: 503, Code: "project_health_unavailable", Message: "project statistics are incomplete", Current: out.Overview}
		}
		writeProjectHealthError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
