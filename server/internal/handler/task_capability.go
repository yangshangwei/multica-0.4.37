package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// TaskCapabilityGuard is deliberately closed for every unclassified path.
// The existing handlers retain their authorization; a capability only narrows it.
func (h *Handler) TaskCapabilityGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := auth.VerifiedTaskTokenFromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		agent, err := h.Queries.GetAgent(r.Context(), token.AgentID)
		if err != nil {
			writeTaskCapabilityError(w, err)
			return
		}
		if len(token.ApiCapability) == 0 && auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) == "" {
			next.ServeHTTP(w, r)
			return
		}
		c, err := auth.ParseTaskCapability(token.ApiCapability)
		if err != nil {
			writeTaskCapabilityError(w, auth.ErrTaskCapability)
			return
		}
		if !taskCapabilityRouteAllowed(r, c) {
			writeTaskCapabilityError(w, auth.ErrTaskCapability)
			return
		}
		if _, err = h.validateTaskCapability(r.Context(), token, false); err != nil {
			writeTaskCapabilityError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func taskCapabilityRouteAllowed(r *http.Request, c auth.TaskCapability) bool {
	if r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path {
		return false
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false
	}
	for _, v := range q {
		if len(v) != 1 {
			return false
		}
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/issues" {
		return c.Profile == auth.TaskCapabilityReporter && c.ProjectID != "" && q.Get("project_id") == c.ProjectID
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/issues/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/api/issues/") || len(parts) > 2 || parts[0] == "" {
		return false
	}
	if _, err := util.ParseUUID(parts[0]); err != nil && splitIdentifier(parts[0]) == nil {
		return false
	}
	if r.Method == http.MethodGet {
		return len(parts) == 1 || parts[1] == "comments" || parts[1] == "timeline"
	}
	return r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "comments"
}

func writeTaskCapabilityError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrTaskCapability) || errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusForbidden, "task capability does not authorize this action or resource")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "task capability authorization unavailable")
}

func (h *Handler) validateTaskCapability(ctx context.Context, token db.TaskToken, lock bool) (auth.TaskCapability, error) {
	expected, err := auth.ParseTaskCapability(token.ApiCapability)
	if err != nil {
		return expected, auth.ErrTaskCapability
	}
	// Auth can precede this snapshot. Revocation and expiry are checked again in
	// the same snapshot/transaction that consumes the resource.
	live, err := h.Queries.GetTaskTokenByHash(ctx, token.TokenHash)
	if err != nil {
		return expected, err
	}
	if live.ID != token.ID || !bytes.Equal(live.ApiCapability, token.ApiCapability) {
		return expected, auth.ErrTaskCapability
	}
	if lock {
		if _, err = h.Queries.LockTaskCapabilityToken(ctx, token.ID); err != nil {
			return expected, err
		}
		if _, err = h.Queries.LockTaskCapabilityPrincipals(ctx, token.TaskID); err != nil {
			return expected, err
		}
		if _, err = h.Queries.LockTaskCapabilityInvocationTargets(ctx, token.AgentID); err != nil {
			return expected, err
		}
	}
	task, err := h.Queries.GetAgentTask(ctx, token.TaskID)
	if err != nil {
		return expected, err
	}
	if task.AgentID != token.AgentID {
		return expected, auth.ErrTaskCapability
	}
	agent, err := h.Queries.GetAgent(ctx, token.AgentID)
	if err != nil {
		return expected, err
	}
	current, err := auth.CurrentTaskCapability(ctx, h.Queries, task, agent, uuidToString(token.UserID), uuidToString(token.WorkspaceID))
	if err != nil {
		return expected, err
	}
	if current != expected {
		return expected, auth.ErrTaskCapability
	}
	return current, nil
}

type capabilityReadSnapshotKey struct{}

// Read the actual handler data and its authorization under the same repeatable
// snapshot. A preliminary middleware lookup alone cannot fence a project move.
func (h *Handler) taskCapabilityRead(w http.ResponseWriter, r *http.Request, read func(*Handler, http.ResponseWriter, *http.Request)) bool {
	token, ok := auth.VerifiedTaskTokenFromContext(r.Context())
	if !ok || len(token.ApiCapability) == 0 || r.Context().Value(capabilityReadSnapshotKey{}) != nil {
		return false
	}
	if h.TxStarter == nil {
		writeTaskCapabilityError(w, errors.New("transaction unavailable"))
		return true
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeTaskCapabilityError(w, err)
		return true
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		writeTaskCapabilityError(w, err)
		return true
	}
	scoped := *h
	scoped.Queries = db.New(tx)
	scoped.DB = tx
	if _, err = scoped.validateTaskCapability(r.Context(), token, false); err != nil {
		writeTaskCapabilityError(w, err)
		return true
	}
	// These handlers only write their HTTP response. Buffer it so a transaction
	// failure cannot follow a successful response on the wire.
	out := capabilityResponse{header: make(http.Header)}
	read(&scoped, &out, r.WithContext(context.WithValue(r.Context(), capabilityReadSnapshotKey{}, true)))
	if err = tx.Commit(r.Context()); err != nil {
		writeTaskCapabilityError(w, err)
		return true
	}
	for k, vs := range out.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if out.status == 0 {
		out.status = http.StatusOK
	}
	w.WriteHeader(out.status)
	_, _ = w.Write(out.body.Bytes())
	return true
}

type capabilityResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *capabilityResponse) Header() http.Header { return w.header }
func (w *capabilityResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *capabilityResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(b)
}

func requireTaskCapabilityIssue(w http.ResponseWriter, r *http.Request, issue db.Issue) bool {
	token, ok := auth.VerifiedTaskTokenFromContext(r.Context())
	if !ok || len(token.ApiCapability) == 0 {
		return true
	}
	c, err := auth.ParseTaskCapability(token.ApiCapability)
	if err != nil || issue.WorkspaceID != token.WorkspaceID || !c.AllowsIssue(issue, r.Method != http.MethodGet) {
		writeTaskCapabilityError(w, auth.ErrTaskCapability)
		return false
	}
	return true
}

// createCapabilityComment commits the authoritative scope check and the comment
// together. Event publication stays in CreateComment, after this commit.
func (h *Handler) createCapabilityComment(ctx context.Context, token db.TaskToken, params db.CreateCommentParams) (db.CreateCommentRow, error) {
	if h.TxStarter == nil {
		return db.CreateCommentRow{}, errors.New("transaction unavailable")
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.CreateCommentRow{}, err
	}
	defer tx.Rollback(ctx)
	scoped := *h
	scoped.Queries = db.New(tx)
	scoped.DB = tx
	c, err := scoped.validateTaskCapability(ctx, token, true)
	if err != nil {
		return db.CreateCommentRow{}, err
	}
	issue, err := scoped.Queries.GetIssue(ctx, params.IssueID)
	if err != nil {
		return db.CreateCommentRow{}, err
	}
	if uuidToString(params.IssueID) != c.IssueID || params.WorkspaceID != token.WorkspaceID || !c.AllowsIssue(issue, true) {
		return db.CreateCommentRow{}, auth.ErrTaskCapability
	}
	params.SuppressDispatch.Bool = true
	params.SuppressDispatch.Valid = true
	row, err := scoped.Queries.CreateComment(ctx, params)
	if err != nil {
		return row, err
	}
	if err = tx.Commit(ctx); err != nil {
		return db.CreateCommentRow{}, err
	}
	return row, nil
}

func taskCapabilityHasTools(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte("{}")) {
		return false
	}
	// Empty canonical MCP envelopes carry no capability.
	var envelope map[string]map[string]any
	if json.Unmarshal(raw, &envelope) == nil && len(envelope) == 1 {
		if servers, ok := envelope["mcpServers"]; ok && len(servers) == 0 {
			return false
		}
	}
	return true
}

func taskCapabilityBriefing(c auth.TaskCapability) string {
	scope := "only the current issue " + c.IssueID
	if c.Profile == auth.TaskCapabilityReporter && c.ProjectID != "" {
		scope = "issues in project " + c.ProjectID + "; list them with the exact project_id filter"
	}
	return "## Task API capability (system; overrides general workflow and saved skills)\n" +
		"Read " + scope + " and their comments/timeline. Post your report or review as a comment on the current issue only. " +
		"Do not change issue fields or status, including in_review and done. A human or an existing authorized coordinator advances business status. " +
		"Your comments do not dispatch agents, including replies, mentions and automatic leader wakeups. External MCP/plugin tools are unavailable. " +
		"This limits Multica API authority; it is not a host shell sandbox. Prior provider sessions are not resumed for this scope."
}

func (h *Handler) requireTaskCapabilityTools(w http.ResponseWriter, r *http.Request, task db.AgentTaskQueue) bool {
	restricted, err := service.RestrictedAgentResult(r.Context(), h.Queries, uuidToString(task.AgentID))
	if err != nil {
		writeTaskCapabilityError(w, err)
		return false
	}
	if restricted {
		writeTaskCapabilityError(w, auth.ErrTaskCapability)
		return false
	}
	return true
}
