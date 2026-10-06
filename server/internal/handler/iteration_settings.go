package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/featureflags"
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
	available := featureflags.IterationsI1Enabled(r.Context(), h.FeatureFlags)
	// Atomic handoff must not be advertised before its implementation and CG.
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "schema_version": iteration.SchemaVersion, "supported": available, "enabled": available && settings.Enabled, "manual": true, "atomic_handoff": false})
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
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid enable request"))
		return
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Only one request object is allowed"))
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
	svc := service.IterationService{TxStarter: h.TxStarter, Available: func(ctx context.Context) bool { return featureflags.IterationsI1Enabled(ctx, h.FeatureFlags) }}
	result, err := svc.Enable(r.Context(), ws, actor, input, authorize)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
