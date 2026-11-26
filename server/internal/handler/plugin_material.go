package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

type pluginMaterialBaseline struct {
	IssueID       string `json:"issue_id"`
	BaselineID    string `json:"baseline_id"`
	Commit        string `json:"commit"`
	ContentSHA256 string `json:"content_sha256"`
	Content       string `json:"content"`
}

func (h *Handler) GetPluginMaterialBaseline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if !h.requirePluginActionV1(w, r) {
		return
	}
	principal, kind, _, err := artifactPrincipal(r)
	if err != nil || kind != "human" || middleware.IsPluginBearerToken(middleware.BearerToken(r)) {
		publicapiv1.WriteProblem(w, r, 403, "forbidden", "this operation requires a human session")
		return
	}
	issueID, err := util.ParseUUID(chi.URLParam(r, "issueID"))
	versionID, versionErr := util.ParseUUID(chi.URLParam(r, "versionID"))
	if err != nil || versionErr != nil {
		publicapiv1.WriteProblem(w, r, 404, "not_found", "material baseline not found")
		return
	}
	out, err := h.pluginMaterialBaseline(r.Context(), artifactCredentialSnapshot(r.Context(), principal), r.Header.Get(pluginInstallationHeader), versionID, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		publicapiv1.WriteProblem(w, r, 404, "not_found", "material baseline not found")
	} else if err != nil {
		var artifactErr service.ArtifactError
		if errors.As(err, &artifactErr) {
			publicapiv1.WriteProblem(w, r, artifactErr.Status, "forbidden", "material baseline unavailable")
		} else {
			writePluginActionError(w, r, err, "material baseline unavailable")
		}
	} else {
		writeJSON(w, http.StatusOK, out)
	}
}

func (h *Handler) pluginMaterialBaseline(ctx context.Context, credential service.ArtifactCredentialSnapshot, installationID string, versionID, issueID pgtype.UUID) (*pluginMaterialBaseline, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return nil, err
	}
	queries := h.Queries.WithTx(tx)
	plugins := service.PluginService{Queries: queries}
	userID, err := util.ParseUUID(credential.Principal.UserID)
	if err != nil {
		return nil, err
	}
	caller, err := plugins.AuthorizePluginAction(ctx, installationID, userID, "")
	if err != nil {
		return nil, err
	}
	if caller.Installation.PackageVersionID != versionID {
		return nil, pgx.ErrNoRows
	}
	if _, err := service.LockArtifactUsers(ctx, queries, caller.WorkspaceID, credential.Principal.UserID); err != nil {
		return nil, err
	}
	if err := service.CheckArtifactCredentialInTx(ctx, tx, credential); err != nil {
		return nil, err
	}
	issue, err := queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: caller.WorkspaceID})
	if err != nil {
		return nil, err
	}
	shared, err := queries.IsCollaborationIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if shared || !issue.ProjectID.Valid {
		return nil, pgx.ErrNoRows
	}
	if !hasGrantedScope(caller.Scopes, plugincontract.ScopeMaterialBaselineReadPrefix+util.UUIDToString(issue.ProjectID)) {
		return nil, &service.PluginError{Kind: service.PluginErrorForbidden, Message: "project material baseline access was not granted"}
	}
	if _, err := queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: caller.WorkspaceID}); err != nil {
		return nil, err
	}
	settings, err := queries.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: issue.ProjectID, WorkspaceID: caller.WorkspaceID})
	if err != nil {
		return nil, err
	}
	var enabled struct{ Enabled bool }
	if err := json.Unmarshal(settings.Settings, &enabled); err != nil || !enabled.Enabled {
		return nil, pgx.ErrNoRows
	}
	plan, err := queries.GetMaterialPlanForIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if plan.ProjectID != issue.ProjectID || plan.WorkspaceID != caller.WorkspaceID {
		return nil, pgx.ErrNoRows
	}
	row, err := queries.GetGitMaterial(ctx, db.GetGitMaterialParams{WorkspaceID: plan.WorkspaceID, ProjectID: plan.ProjectID, ID: plan.MaterialID})
	if err != nil {
		return nil, err
	}
	var baseline material.Baseline
	if err := json.Unmarshal(row.Baseline, &baseline); err != nil || !plan.BaselineID.Valid || baseline.ID != util.UUIDToString(plan.BaselineID) ||
		baseline.RevisionID != baseline.ID || !material.IsSHA(baseline.Commit) || baseline.VerifiedAt.IsZero() ||
		!row.CurrentRevisionID.Valid || (row.CurrentRevisionID != plan.BaselineID && row.CurrentRevisionID != plan.RevisionID) {
		return nil, pgx.ErrNoRows
	}
	revision, err := queries.GetMaterialRevision(ctx, plan.BaselineID)
	if err != nil {
		return nil, err
	}
	var content material.Revision
	if revision.MaterialID != row.ID || revision.ProjectID != plan.ProjectID || revision.WorkspaceID != plan.WorkspaceID || revision.Kind != "requirement" ||
		json.Unmarshal(revision.Payload, &content) != nil || content.ID != baseline.RevisionID || content.Kind != "requirement" ||
		content.Content == "" || len(content.Content) > material.MaxContent || content.ContentSHA256 != baseline.ContentSHA256 || material.Digest(content.Content) != baseline.ContentSHA256 {
		return nil, pgx.ErrNoRows
	}
	return &pluginMaterialBaseline{util.UUIDToString(issueID), baseline.ID, baseline.Commit, baseline.ContentSHA256, content.Content}, tx.Commit(ctx)
}
