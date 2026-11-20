package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type issueBudgetRequestContext struct {
	service   *service.IssueBudgetService
	workspace pgtype.UUID
	actor     pgtype.UUID
	rootID    pgtype.UUID
	issue     db.Issue
	key       string
	enable    issueBudgetEnableBody
	operation issueBudgetSetLimitBody
	mode      string
	page      service.IssueBudgetPageParams
}

type issueBudgetRouteKind int

const (
	budgetEnable issueBudgetRouteKind = iota
	budgetBinding
	budgetEnableOperation
	budgetShow
	budgetManage
	budgetOperation
	budgetObservations
	budgetAttempts
)

func issueBudgetHumanPrincipal(r *http.Request) (auth.AuthenticatedPrincipal, bool) {
	p, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
	hints := middleware.IssueBudgetActorHints(r.Context()) || middleware.IssueBudgetHasActorHeaders(r.Header)
	return p, ok && (p.Kind == auth.CredentialJWT || p.Kind == auth.CredentialPAT) && !p.AgentActor && !p.Restricted && !hints
}

// RequireIssueBudgetHuman precedes TaskCapabilityGuard. A task credential is
// denied here without agent/capability lookups or a generic guard error body.
// The handler repeats this check and then applies current workspace authority.
func (h *Handler) RequireIssueBudgetHuman(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.IsIssueBudgetManagementPath(r.URL.Path) {
			if _, ok := issueBudgetHumanPrincipal(r); !ok {
				w.Header().Set("Cache-Control", "no-store")
				writeIssueBudgetError(w, http.StatusForbidden, service.IssueBudgetForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireIssueBudgetWorkspace delegates the existing workspace middleware for
// every route. Only budget management failures are translated to its private
// code-only contract; successful calls keep the middleware's member context
// and stream directly to the original writer without buffering the response.
func (h *Handler) RequireIssueBudgetWorkspace(next http.Handler) http.Handler {
	member := middleware.RequireWorkspaceMember(h.Queries)
	ordinary := member(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !middleware.IsIssueBudgetManagementPath(r.URL.Path) {
			ordinary.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		passed := false
		capture := &issueBudgetStatusWriter{header: make(http.Header)}
		member(http.HandlerFunc(func(_ http.ResponseWriter, resolved *http.Request) {
			passed = true
			next.ServeHTTP(w, resolved)
		})).ServeHTTP(capture, r)
		if passed {
			return
		}
		switch capture.status {
		case http.StatusForbidden, http.StatusNotFound:
			writeIssueBudgetError(w, http.StatusForbidden, service.IssueBudgetForbidden)
		case http.StatusBadRequest:
			writeIssueBudgetError(w, http.StatusBadRequest, service.IssueBudgetInvalidRequest)
		default:
			slog.Error("budget workspace authorization failed", "status", capture.status)
			writeIssueBudgetError(w, http.StatusInternalServerError, "internal_error")
		}
	})
}

// budgetRequest centralizes the ordering shared by all eight management
// routes: human credential and role, path, strict body/page, issue access, then
// service. No budget table is read for an unauthorized or malformed request.
func (h *Handler) budgetRequest(w http.ResponseWriter, r *http.Request, route issueBudgetRouteKind) (issueBudgetRequestContext, bool) {
	w.Header().Set("Cache-Control", "no-store")
	var out issueBudgetRequestContext
	p, ok := issueBudgetHumanPrincipal(r)
	member, memberOK := ctxMember(r.Context())
	if !ok || !memberOK || !roleAllowed(member.Role, "owner", "admin") {
		writeIssueBudgetError(w, http.StatusForbidden, service.IssueBudgetForbidden)
		return out, false
	}
	var err error
	out.workspace, err = util.ParseUUID(ctxWorkspaceID(r.Context()))
	if err != nil || !out.workspace.Valid || member.WorkspaceID != out.workspace || uuidToString(member.UserID) != p.UserID {
		writeIssueBudgetError(w, http.StatusForbidden, service.IssueBudgetForbidden)
		return out, false
	}
	out.actor = member.UserID
	issueRoute := route <= budgetEnableOperation
	if !issueRoute {
		out.rootID, err = util.ParseUUID(chi.URLParam(r, "rootId"))
		if err != nil || !out.rootID.Valid {
			writeIssueBudgetError(w, http.StatusBadRequest, service.IssueBudgetInvalidRequest)
			return out, false
		}
	}
	if route == budgetEnableOperation || route == budgetOperation {
		out.key = chi.URLParam(r, "key")
		if _, err := delegationKey(out.key); err != nil {
			writeIssueBudgetError(w, http.StatusBadRequest, service.IssueBudgetInvalidRequest)
			return out, false
		}
	}
	switch route {
	case budgetEnable, budgetManage:
		raw, err := readIssueBudgetBody(w, r)
		if err == nil && route == budgetEnable {
			err = decodeIssueBudgetBody(raw, &out.enable, "request_key", "reason", "limit_ticks", "attempt_reserve_ticks")
			if err == nil {
				_, err = delegationKey(out.enable.RequestKey)
			}
		} else if err == nil {
			out.operation, out.mode, err = parseIssueBudgetOperation(raw)
		}
		if err != nil {
			writeIssueBudgetError(w, http.StatusBadRequest, service.IssueBudgetInvalidRequest)
			return out, false
		}
	case budgetObservations, budgetAttempts:
		out.page, err = parseIssueBudgetPage(r.URL.RawQuery)
		if err != nil {
			writeIssueBudgetError(w, http.StatusBadRequest, service.IssueBudgetInvalidRequest)
			return out, false
		}
	}
	// The handler's starter is the pool, never a request-owned transaction.
	out.service = service.NewIssueBudgetService(h.Queries, h.TxStarter)
	if issueRoute {
		out.issue, ok = h.loadBudgetIssue(w, r, chi.URLParam(r, "id"), route == budgetEnable)
		return out, ok
	}
	state, err := out.service.GetRoot(r.Context(), out.workspace, out.rootID)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return out, false
	}
	if !state.TombstonedAt.Valid {
		if _, ok := h.loadBudgetIssue(w, r, uuidToString(state.Root.RootIssueID), false); !ok {
			return out, false
		}
	}
	return out, true
}

// Only the loader's authorization is evaluated using the write method: budget
// reads require the same ordinary-issue edit boundary. No HTTP write occurs.
func (h *Handler) loadBudgetIssue(w http.ResponseWriter, r *http.Request, id string, enable bool) (db.Issue, bool) {
	probe := *r
	probe.Method = http.MethodPost
	capture := &issueBudgetStatusWriter{header: make(http.Header)}
	issue, ok := h.loadIssueForUser(capture, &probe, id)
	if ok {
		return issue, true
	}
	status, code := http.StatusNotFound, service.IssueBudgetNotFound
	if capture.status >= 500 {
		status, code = http.StatusInternalServerError, "internal_error"
	} else if capture.status != http.StatusNotFound && enable {
		status, code = http.StatusUnprocessableEntity, service.IssueBudgetNotEligible
	}
	writeIssueBudgetError(w, status, code)
	return db.Issue{}, false
}

type issueBudgetStatusWriter struct {
	header http.Header
	status int
}

func (w *issueBudgetStatusWriter) Header() http.Header { return w.header }
func (w *issueBudgetStatusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *issueBudgetStatusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(p), nil
}

func writeIssueBudgetError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, issueBudgetErrorBody{Error: code, Code: code})
}

func writeIssueBudgetServiceError(w http.ResponseWriter, err error) {
	code := service.IssueBudgetErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case service.IssueBudgetInvalidRequest:
		status = http.StatusBadRequest
	case service.IssueBudgetForbidden:
		status = http.StatusForbidden
	case service.IssueBudgetNotFound:
		status = http.StatusNotFound
	case service.IssueBudgetNotEligible:
		status = http.StatusUnprocessableEntity
	case service.IssueBudgetAlreadyEnabled, service.IssueBudgetRootConflict, service.IssueBudgetParentCycle,
		service.IssueBudgetOperationConflict, service.IssueBudgetPolicyRevisionConflict, service.IssueBudgetRootDeleted, service.IssueBudgetStateConflict:
		status = http.StatusConflict
	default:
		// Database errors may contain submitted values. Log no raw error or
		// request text; the code and status are sufficient for this boundary.
		code = "internal_error"
		slog.Error("budget management failed", "code", code, "status", status)
	}
	writeIssueBudgetError(w, status, code)
}

func writeIssueBudgetOperation(w http.ResponseWriter, result service.IssueBudgetOperationResult, err error) {
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, service.IssueBudgetWriteResponse{Replayed: result.Replayed, Result: result})
}

func (h *Handler) EnableIssueBudget(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetEnable)
	if !ok {
		return
	}
	result, err := p.service.Enable(r.Context(), service.IssueBudgetEnableParams{
		WorkspaceID: p.workspace, IssueID: p.issue.ID, ActorUserID: p.actor, OperationKey: p.enable.RequestKey,
		Reason: p.enable.Reason, LimitTicks: p.enable.LimitTicks, AttemptReserveTicks: p.enable.AttemptReserveTicks,
	})
	writeIssueBudgetOperation(w, result, err)
}

func (h *Handler) GetIssueBudgetBinding(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetBinding)
	if !ok {
		return
	}
	result, err := p.service.Binding(r.Context(), p.workspace, p.issue.ID)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ShowIssueBudget(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetShow)
	if !ok {
		return
	}
	result, err := p.service.Show(r.Context(), p.workspace, p.rootID)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ManageIssueBudget(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetManage)
	if !ok {
		return
	}
	params := service.IssueBudgetManageParams{
		WorkspaceID: p.workspace, BudgetRootID: p.rootID, OperationKey: p.operation.RequestKey, ActorUserID: p.actor,
		ExpectedPolicyRevision: int64(p.operation.ExpectedPolicyRevision), Reason: p.operation.Reason,
	}
	var result service.IssueBudgetOperationResult
	var err error
	switch p.operation.Kind {
	case service.IssueBudgetOperationSetLimit:
		result, err = p.service.SetLimit(r.Context(), params, p.operation.LimitTicks, p.operation.AttemptReserveTicks)
	case service.IssueBudgetOperationSetMode:
		result, err = p.service.SetMode(r.Context(), params, p.mode)
	case service.IssueBudgetOperationClose:
		result, err = p.service.Close(r.Context(), params)
	case service.IssueBudgetOperationReopen:
		result, err = p.service.Reopen(r.Context(), params)
	}
	writeIssueBudgetOperation(w, result, err)
}

func (h *Handler) GetIssueBudgetEnableOperation(w http.ResponseWriter, r *http.Request) {
	h.getBudgetOperation(w, r, budgetEnableOperation)
}

func (h *Handler) GetIssueBudgetOperation(w http.ResponseWriter, r *http.Request) {
	h.getBudgetOperation(w, r, budgetOperation)
}

func (h *Handler) getBudgetOperation(w http.ResponseWriter, r *http.Request, route issueBudgetRouteKind) {
	p, ok := h.budgetRequest(w, r, route)
	if !ok {
		return
	}
	scope, id, prefix := "root", p.rootID, "/api/issue-budgets/"
	if route == budgetEnableOperation {
		scope, id, prefix = "issue", p.issue.ID, "/api/issues/"
	}
	result, err := p.service.LookupOperation(r.Context(), p.workspace, scope, id, p.key)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	if result == nil {
		path := prefix + uuidToString(id)
		if scope == "issue" {
			path += "/budget"
		}
		writeJSON(w, http.StatusOK, service.IssueBudgetOperationUnknown{State: "unknown", RetryPath: path + "/operations/" + p.key})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListIssueBudgetObservations(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetObservations)
	if !ok {
		return
	}
	result, err := p.service.PageObservations(r.Context(), p.workspace, p.rootID, p.page)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListIssueBudgetAttempts(w http.ResponseWriter, r *http.Request) {
	p, ok := h.budgetRequest(w, r, budgetAttempts)
	if !ok {
		return
	}
	result, err := p.service.PageAttempts(r.Context(), p.workspace, p.rootID, p.page)
	if err != nil {
		writeIssueBudgetServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
