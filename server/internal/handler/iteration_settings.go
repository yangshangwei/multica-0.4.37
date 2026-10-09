package handler

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) GetIterationSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	var settings iteration.Settings
	err = h.withIterationRead(r.Context(), ws, actor, func(tx pgx.Tx, _ *db.Queries) error {
		var e error
		settings, e = service.ReadIterationSettings(r.Context(), tx, ws)
		return e
	})
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, settings)
}

func (h *Handler) GetIterationCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	var settings iteration.Settings
	err = h.withIterationRead(r.Context(), ws, actor, func(tx pgx.Tx, _ *db.Queries) error {
		var e error
		settings, e = service.ReadIterationSettings(r.Context(), tx, ws)
		return e
	})
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "schema_version": iteration.SchemaVersion, "supported": true, "enabled": settings.Enabled, "manual": true, "atomic_handoff": true})
}

func (h *Handler) EnableIterationSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	initialType, initialActor := h.resolveActor(r, requestUserID(r), uuidToString(ws))
	if isMachineCredentialActor(r) || initialType != "member" || initialActor != uuidToString(actor) {
		writeIterationAPIError(w, iterationAPIError(403, "forbidden", "Only human workspace administrators may enable iterations"))
		return
	}

	var input service.EnableIterationInput
	if !decodeIterationBody(w, r, &input) {
		return
	}

	authorize := func(ctx context.Context, tx pgx.Tx) error {
		scoped := *h
		scoped.Queries, scoped.DB = h.Queries.WithTx(tx), tx
		actorType, actorID := scoped.resolveActor(r, requestUserID(r), uuidToString(ws))
		if isMachineCredentialActor(r) || actorType != initialType || actorID != initialActor {
			return iterationAPIError(403, "forbidden", "Only human workspace administrators may enable iterations")
		}
		member, e := scoped.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: actor})
		if e != nil {
			return e
		}
		if member.Role != "owner" && member.Role != "admin" {
			return iterationAPIError(403, "forbidden", "Only workspace administrators may enable iterations")
		}
		return nil
	}
	svc := service.IterationService{TxStarter: h.TxStarter}
	result, err := svc.Enable(r.Context(), ws, actor, input, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	h.writeIterationResult(w, r, result, false)
}
