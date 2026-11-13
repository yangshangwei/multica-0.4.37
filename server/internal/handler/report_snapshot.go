package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func canonicalReportSnapshotUUID(value string) bool {
	return service.CanonicalReportSnapshotUUID(value)
}
func reportSnapshotFailure(status int, code string) error {
	return service.ReportSnapshotError{Status: status, Code: code}
}
func writeReportSnapshotError(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	status, code := 503, "report_snapshot_unavailable"
	var failure service.ReportSnapshotError
	var artifact service.ArtifactError
	var oldArtifact artifactFailure
	var project *projectAPIError
	var pgerr *pgconn.PgError
	switch {
	case errors.As(err, &failure):
		status, code = failure.Status, failure.Code
	case errors.As(err, &artifact):
		if artifact.Status == 403 {
			status, code = 403, "report_snapshot_denied"
		}
	case errors.As(err, &oldArtifact):
		if oldArtifact.status == 403 {
			status, code = 403, "report_snapshot_denied"
		}
	case errors.As(err, &project):
		if project.Status == 403 || project.Status == 404 {
			status, code = 403, "report_snapshot_denied"
		}
	case errors.Is(err, auth.ErrTaskCapability), errors.Is(err, auth.ErrPasswordSession):
		status, code = 403, "report_snapshot_denied"
	case errors.Is(err, pgx.ErrNoRows):
		status, code = 404, "report_snapshot_not_found"
	case errors.As(err, &pgerr): // SQL details and source identity never leave this boundary.
	}
	writeJSON(w, status, map[string]string{"error": code, "code": code})
}

func reportSnapshotPath(r *http.Request, create bool) bool {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || (r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/issues/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/api/issues/") || len(parts) < 2 || parts[1] != "report-snapshots" || !canonicalReportSnapshotUUID(parts[0]) {
		return false
	}
	if create {
		return r.Method == http.MethodPost && len(parts) == 2
	}
	return r.Method == http.MethodGet && len(parts) == 3 && (parts[2] == "current" || canonicalReportSnapshotUUID(parts[2]))
}

func reportSnapshotOriginalScope(ctx context.Context, r *http.Request, q *db.Queries, issue db.Issue) error {
	original, ok := middleware.ReportSnapshotOriginalHeaders(ctx)
	if !ok {
		return reportSnapshotFailure(403, "report_snapshot_denied")
	}
	principal, ok := auth.AuthenticatedPrincipalFromContext(ctx)
	if !ok {
		return reportSnapshotFailure(401, "report_snapshot_unauthenticated")
	}
	expected := map[string]string{"X-User-ID": principal.UserID, "X-Workspace-ID": issue.WorkspaceID.String(), "X-Agent-ID": "", "X-Task-ID": "", "X-Actor-Source": ""}
	if token, ok := auth.VerifiedTaskTokenFromContext(ctx); ok {
		expected["X-Agent-ID"] = token.AgentID.String()
		expected["X-Task-ID"] = token.TaskID.String()
		expected["X-Actor-Source"] = "task_token"
	}
	for name, values := range original {
		if len(values) != 1 {
			return reportSnapshotFailure(403, "report_snapshot_denied")
		}
		if name == "X-Workspace-Slug" {
			continue
		}
		if value, exists := expected[name]; !exists || values[0] != value {
			return reportSnapshotFailure(403, "report_snapshot_denied")
		}
	}
	if values, exists := original["X-Workspace-Slug"]; exists {
		workspace, err := q.GetWorkspace(ctx, issue.WorkspaceID)
		if err != nil {
			return err
		}
		if values[0] != workspace.Slug {
			return reportSnapshotFailure(403, "report_snapshot_denied")
		}
	}
	return nil
}

func (h *Handler) CreateReportSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !reportSnapshotPath(r, true) {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	principal, kind, actor, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeReportSnapshotError(w, reportSnapshotFailure(403, "report_snapshot_denied"))
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !canonicalReportSnapshotUUID(keys[0]) {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	request, err := service.ParseReportSnapshotRequest(raw)
	if err != nil {
		writeReportSnapshotError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	var out service.ReportSnapshotResponse
	err = h.reportSnapshotHumanTransaction(r, principal, actor, true, func(tx pgx.Tx, q *db.Queries, issue db.Issue, project *db.Project) error {
		var e error
		out, e = service.CaptureReportSnapshotInTx(ctx, tx, issue, project, actor, keys[0], request)
		return e
	})
	if err != nil {
		writeReportSnapshotError(w, err)
		return
	}
	status := http.StatusCreated
	if out.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, out)
}

func (h *Handler) GetReportSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !reportSnapshotPath(r, false) {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	if token, ok := auth.VerifiedTaskTokenFromContext(r.Context()); ok {
		capability, err := auth.ParseTaskCapability(token.ApiCapability)
		if err != nil || capability.Profile != auth.TaskCapabilityReporter || capability.IssueID != chi.URLParam(r, "id") {
			writeReportSnapshotError(w, reportSnapshotFailure(403, "report_snapshot_denied"))
			return
		}
		if h.taskCapabilityRead(w, r, func(scoped *Handler, out http.ResponseWriter, request *http.Request) {
			scoped.GetReportSnapshot(out, request)
		}) {
			return
		}
		issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: parseUUID(capability.IssueID), WorkspaceID: token.WorkspaceID})
		if err == nil && !capability.AllowsIssue(issue, false) {
			err = auth.ErrTaskCapability
		}
		if err == nil {
			err = reportSnapshotEligible(r.Context(), h.Queries, issue)
		}
		if err == nil {
			err = reportSnapshotOriginalScope(r.Context(), r, h.Queries, issue)
		}
		if err == nil && issue.ProjectID.Valid {
			var project db.Project
			project, err = h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
			if err == nil && project.WorkspaceID != issue.WorkspaceID {
				err = auth.ErrTaskCapability
			}
		}
		if err != nil {
			writeReportSnapshotError(w, err)
			return
		}
		out, err := readReportSnapshotRequest(r, h.Queries, issue)
		if err != nil {
			writeReportSnapshotError(w, err)
			return
		}
		writeJSON(w, 200, out)
		return
	}
	principal, kind, actor, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeReportSnapshotError(w, reportSnapshotFailure(403, "report_snapshot_denied"))
		return
	}
	var out service.ReportSnapshotResponse
	err = h.reportSnapshotHumanTransaction(r, principal, actor, false, func(_ pgx.Tx, q *db.Queries, issue db.Issue, _ *db.Project) error {
		var e error
		out, e = readReportSnapshotRequest(r, q, issue)
		return e
	})
	if err != nil {
		writeReportSnapshotError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) WithdrawReportSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	reportID := chi.URLParam(r, "id")
	if r.Method != http.MethodPost || !canonicalReportSnapshotUUID(reportID) || r.URL.Path != "/api/issues/"+reportID+"/report-snapshots/withdrawals" || r.URL.RawQuery != "" || r.URL.ForceQuery || (r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path) {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	principal, kind, actor, err := artifactPrincipal(r)
	if err != nil || kind != "human" {
		writeReportSnapshotError(w, reportSnapshotFailure(403, "report_snapshot_denied"))
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !canonicalReportSnapshotUUID(keys[0]) {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
	if err != nil {
		writeReportSnapshotError(w, reportSnapshotFailure(400, "invalid_report_snapshot_request"))
		return
	}
	request, err := service.ParseReportSnapshotWithdrawalRequest(raw)
	if err != nil {
		writeReportSnapshotError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	var out service.ReportSnapshotWithdrawalResponse
	err = h.reportSnapshotHumanTransaction(r, principal, actor, true, func(tx pgx.Tx, _ *db.Queries, issue db.Issue, _ *db.Project) error {
		var e error
		out, e = service.WithdrawReportSnapshotsInTx(ctx, tx, issue, actor, keys[0], request)
		return e
	})
	if err != nil {
		writeReportSnapshotError(w, err)
		return
	}
	status := http.StatusCreated
	if out.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, out)
}

func (h *Handler) reportSnapshotHumanTransaction(r *http.Request, principal auth.AuthenticatedPrincipal, actor string, capture bool, fn func(pgx.Tx, *db.Queries, db.Issue, *db.Project) error) error {
	if h.TxStarter == nil {
		return reportSnapshotFailure(503, "report_snapshot_unavailable")
	}
	initial, err := h.Queries.GetIssue(r.Context(), parseUUID(chi.URLParam(r, "id")))
	if err != nil {
		return err
	}
	return h.runProjectTransaction(r.Context(), initial.WorkspaceID, parseUUID(actor), func(tx pgx.Tx, q *db.Queries) error {
		if err := service.CheckArtifactCredentialInTx(r.Context(), tx, artifactCredentialSnapshot(r.Context(), principal)); err != nil {
			return err
		}
		if _, err := service.LockArtifactUsers(r.Context(), q, initial.WorkspaceID, actor); err != nil {
			return err
		}
		var issue db.Issue
		var err error
		if capture {
			issue, err = q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
		} else {
			issue, err = q.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
		}
		if err != nil {
			return err
		}
		if err = reportSnapshotEligible(r.Context(), q, issue); err != nil {
			return err
		}
		if err = reportSnapshotOriginalScope(r.Context(), r, q, issue); err != nil {
			return err
		}
		var project *db.Project
		if issue.ProjectID.Valid {
			p, e := q.LockProjectForAssociationNowait(r.Context(), db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
			if e != nil {
				return e
			}
			project = &p
		}
		if err = fn(tx, q, issue, project); err != nil {
			return err
		}
		return service.CheckArtifactCredentialInTx(r.Context(), tx, artifactCredentialSnapshot(r.Context(), principal))
	})
}

func reportSnapshotEligible(ctx context.Context, q *db.Queries, issue db.Issue) error {
	if issue.AdmissionStatus != "not_required" && issue.AdmissionStatus != "accepted" {
		return reportSnapshotFailure(403, "report_snapshot_denied")
	}
	shared, err := q.IsCollaborationIssue(ctx, issue.ID)
	if err != nil {
		return err
	}
	if shared {
		return reportSnapshotFailure(403, "report_snapshot_denied")
	}
	return nil
}

func readReportSnapshotRequest(r *http.Request, q *db.Queries, issue db.Issue) (service.ReportSnapshotResponse, error) {
	out := service.ReportSnapshotResponse{ProtocolVersion: 1}
	control, err := q.GetReportSnapshotControl(r.Context(), db.GetReportSnapshotControlParams{WorkspaceID: issue.WorkspaceID, ReportIssueID: issue.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, existingErr := q.GetCurrentReportSnapshot(r.Context(), db.GetCurrentReportSnapshotParams{WorkspaceID: issue.WorkspaceID, ReportIssueID: issue.ID}); existingErr == nil {
				return out, reportSnapshotFailure(503, "report_snapshot_unavailable")
			} else if !errors.Is(existingErr, pgx.ErrNoRows) {
				return out, existingErr
			}
		}
		return out, err
	}
	if control.WithdrawnAt.Valid || !control.CurrentSnapshotID.Valid {
		return out, reportSnapshotFailure(404, "report_snapshot_not_found")
	}
	var row db.ReportSnapshot
	id := chi.URLParam(r, "snapshotId")
	if id == "" {
		row, err = q.GetReportSnapshot(r.Context(), db.GetReportSnapshotParams{WorkspaceID: issue.WorkspaceID, ReportIssueID: issue.ID, ID: control.CurrentSnapshotID})
		if errors.Is(err, pgx.ErrNoRows) {
			err = reportSnapshotFailure(503, "report_snapshot_unavailable")
		}
		if err == nil && row.Sequence != control.CurrentSequence {
			err = reportSnapshotFailure(503, "report_snapshot_unavailable")
		}
	} else {
		row, err = q.GetReportSnapshot(r.Context(), db.GetReportSnapshotParams{WorkspaceID: issue.WorkspaceID, ReportIssueID: issue.ID, ID: parseUUID(id)})
	}
	if err != nil {
		return out, err
	}
	out.Snapshot, err = service.ReadReportSnapshot(row, issue)
	return out, err
}
