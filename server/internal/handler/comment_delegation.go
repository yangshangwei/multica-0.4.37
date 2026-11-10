package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DelegationReceipt records admission facts, never execution completion. Unknown
// is retained across restart; neither reads nor repeated writes may take it over.
type DelegationRuntimeObservation struct {
	Status     string `json:"status"`
	ObservedAt string `json:"observed_at"`
}

type DelegationReceipt struct {
	Key       string                  `json:"key"`
	State     string                  `json:"state"`
	CommentID string                  `json:"comment_id"`
	Targets   []CommentTriggerOutcome `json:"targets"`
}

type delegationClaim struct {
	lookup   db.GetCommentDelegationReceiptParams
	receipt  DelegationReceipt
	response CommentResponse
}

func delegationKey(raw string) (pgtype.UUID, error) {
	id, err := util.ParseUUID(raw)
	if err != nil || !id.Valid || uuidToString(id) != raw || raw == "00000000-0000-0000-0000-000000000000" {
		return pgtype.UUID{}, errors.New("invalid idempotency key")
	}
	return id, nil
}

func delegationError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrTaskCapability) {
		writeError(w, 403, "task capability does not authorize delegation receipt access")
		return
	}
	var failure artifactFailure
	if errors.As(err, &failure) {
		writeError(w, failure.status, "delegation authority unavailable")
		return
	}
	if isNotFound(err) {
		writeError(w, 404, "delegation receipt not found")
		return
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40P01") {
		writeError(w, 409, "delegation authority busy")
		return
	}
	writeError(w, 503, "delegation receipt unavailable; do not repost with a new key")
}

func delegationPrincipal(r *http.Request) (auth.AuthenticatedPrincipal, string, string, error) {
	p, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
	if !ok {
		return p, "", "", artifactDeny()
	}
	if p.Kind == auth.CredentialTask {
		tok, ok := auth.VerifiedTaskTokenFromContext(r.Context())
		if !ok || uuidToString(tok.UserID) != p.UserID {
			return p, "", "", artifactDeny()
		}
		return p, "agent", uuidToString(tok.AgentID), nil
	}
	if p.AgentActor || p.Restricted || (p.Kind != auth.CredentialJWT && p.Kind != auth.CredentialPAT) || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
		return p, "", "", artifactDeny()
	}
	return p, "human", p.UserID, nil
}

// Reuse the current-credential and principal fences used by controlled actions.
// Only verified task credentials select an agent; client actor headers cannot.
func (h *Handler) delegationAuthority(r *http.Request, q *db.Queries, tx pgx.Tx, issue db.Issue, key pgtype.UUID) (db.GetCommentDelegationReceiptParams, error) {
	out := db.GetCommentDelegationReceiptParams{WorkspaceID: issue.WorkspaceID, RequestKey: key}
	p, kind, id, err := delegationPrincipal(r)
	if err != nil {
		return out, err
	}
	if err = artifactFence(r.Context(), q, issue.WorkspaceID); err != nil {
		return out, err
	}
	// The target may differ from the token's source issue. Fence it before
	// reading the target project used by a reporter's current scope check.
	if _, err = q.LockDelegationIssue(r.Context(), db.LockDelegationIssueParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID}); err != nil {
		return out, err
	}
	if kind == "agent" {
		token, ok := auth.VerifiedTaskTokenFromContext(r.Context())
		if !ok || token.WorkspaceID != issue.WorkspaceID {
			return out, artifactDeny()
		}
		if _, err = q.LockTaskCapabilityToken(r.Context(), token.ID); err != nil {
			return out, err
		}
		if _, err = q.LockTaskCapabilityPrincipals(r.Context(), token.TaskID); err != nil {
			return out, err
		}
		task, e := q.GetAgentTask(r.Context(), token.TaskID)
		if e != nil {
			return out, e
		}
		if task.AgentID != token.AgentID || (task.Status != "running" && task.Status != "dispatched") || !task.OriginatorUserID.Valid {
			return out, artifactDeny()
		}
		agents, e := q.TryLockCollaborationAuthorityAgents(r.Context(), []pgtype.UUID{token.AgentID})
		if e != nil {
			return out, e
		}
		if len(agents) != 1 || agents[0].WorkspaceID != issue.WorkspaceID || agents[0].ArchivedAt.Valid {
			return out, artifactDeny()
		}
		agent := agents[0]
		runtime, e := (service.RuntimeLookup{Queries: q, Metrics: h.Metrics, Source: obsmetrics.RuntimeLookupSourceTask}).Get(r.Context(), task.RuntimeID)
		if e != nil {
			return out, e
		}
		if runtime.ID != agent.RuntimeID || runtime.WorkspaceID != issue.WorkspaceID {
			return out, artifactDeny()
		}
		if _, e = q.LockTaskCapabilityInvocationTargets(r.Context(), agent.ID); e != nil {
			return out, e
		}
		if _, e = artifactUsers(r.Context(), q, issue.WorkspaceID, p.UserID, uuidToString(task.OriginatorUserID), uuidToString(runtime.OwnerID)); e != nil {
			return out, e
		}
		scoped := *h
		scoped.Queries = q
		scoped.DB = tx
		if !scoped.canInvokeAgent(r.Context(), agent, "member", uuidToString(task.OriginatorUserID), "", uuidToString(issue.WorkspaceID)) {
			return out, artifactDeny()
		}
		if len(token.ApiCapability) > 0 {
			capability, e := scoped.validateTaskCapability(r.Context(), token, true)
			if e != nil {
				return out, e
			}
			currentIssue, e := q.GetIssue(r.Context(), issue.ID)
			if e != nil {
				return out, e
			}
			if !capability.AllowsIssue(currentIssue, r.Method == http.MethodPost) {
				return out, auth.ErrTaskCapability
			}
		}
		out.ActorType = "agent"
		out.ActorID = token.AgentID
		out.OriginatorID = task.OriginatorUserID
	} else {
		if _, err = artifactUsers(r.Context(), q, issue.WorkspaceID, id); err != nil {
			return out, err
		}
		out.ActorType = "member"
		out.ActorID = parseUUID(id)
		out.OriginatorID = out.ActorID
	}
	if err = artifactCredential(r.Context(), q, tx, p); err != nil {
		return out, err
	}
	return out, nil
}

func unknownDelegationTargets(content string) []CommentTriggerOutcome {
	out := []CommentTriggerOutcome{}
	seen := map[string]bool{}
	for _, m := range util.ParseMentions(content) {
		if (m.Type == "agent" || m.Type == "squad") && !seen[m.Type+":"+m.ID] {
			seen[m.Type+":"+m.ID] = true
			out = append(out, CommentTriggerOutcome{TargetType: m.Type, TargetID: m.ID, Status: "unknown", ReasonCode: "dispatch_unconfirmed"})
		}
	}
	return out
}

// Attachments deliberately remain on the legacy path: their separately
// committed binding and expiring URLs cannot be replayed as an atomic snapshot.
func (h *Handler) createDelegationComment(w http.ResponseWriter, r *http.Request, issue db.Issue, req CreateCommentRequest, params db.CreateCommentParams, restricted bool) (db.CreateCommentRow, *delegationClaim, bool) {
	key, err := delegationKey(r.Header.Get("Idempotency-Key"))
	if err != nil || len(r.Header.Values("Idempotency-Key")) != 1 {
		writeError(w, 400, "Idempotency-Key must be one canonical nonzero UUID")
		return db.CreateCommentRow{}, nil, false
	}
	if len(req.AttachmentIDs) > 0 {
		writeError(w, 400, "idempotent delegation comments do not support attachments")
		return db.CreateCommentRow{}, nil, false
	}
	// Arrays with set semantics are canonicalized, while content and parent remain exact.
	normalized := req
	normalized.SuppressAgentIDs = append([]string{}, req.SuppressAgentIDs...)
	sort.Strings(normalized.SuppressAgentIDs)
	raw, _ := json.Marshal(struct {
		Issue   string               `json:"issue"`
		Request CreateCommentRequest `json:"request"`
	}{uuidToString(issue.ID), normalized})
	digest := sha256.Sum256(raw)
	tx, q, err := h.delegationTx(r.Context())
	if err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	defer tx.Rollback(r.Context())
	lookup, err := h.delegationAuthority(r, q, tx, issue, key)
	if err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	// Never inherit legacy header-selected agent impersonation into a durable key.
	if lookup.ActorType != params.AuthorType || lookup.ActorID != params.AuthorID {
		writeError(w, 403, "durable agent delegation requires a verified task credential")
		return db.CreateCommentRow{}, nil, false
	}
	receipt := DelegationReceipt{Key: uuidToString(key), State: "unknown", CommentID: uuidToString(params.ID), Targets: unknownDelegationTargets(params.Content)}
	placeholder := CommentResponse{ID: receipt.CommentID, IssueID: uuidToString(issue.ID), AuthorType: params.AuthorType, AuthorID: uuidToString(params.AuthorID), Content: params.Content, Type: params.Type, ParentID: uuidToPtr(params.ParentID), SourceTaskID: uuidToPtr(params.SourceTaskID), Reactions: []ReactionResponse{}, Attachments: []AttachmentResponse{}}
	targets, _ := json.Marshal(receipt.Targets)
	response, _ := json.Marshal(placeholder)
	claimed, err := q.ClaimCommentDelegationReceipt(r.Context(), db.ClaimCommentDelegationReceiptParams{WorkspaceID: lookup.WorkspaceID, ActorType: lookup.ActorType, ActorID: lookup.ActorID, OriginatorID: lookup.OriginatorID, RequestKey: key, RequestDigest: hex.EncodeToString(digest[:]), IssueID: issue.ID, CommentID: params.ID, SourceTaskID: params.SourceTaskID, Targets: targets, Response: response})
	if err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	if claimed == 0 {
		old, e := q.GetCommentDelegationReceipt(r.Context(), lookup)
		if e != nil {
			delegationError(w, e)
			return db.CreateCommentRow{}, nil, false
		}
		if old.IssueID != issue.ID || old.RequestDigest != hex.EncodeToString(digest[:]) {
			writeError(w, 409, "idempotency key is bound to another request")
			return db.CreateCommentRow{}, nil, false
		}
		out, e := delegationResponse(old)
		if e != nil {
			delegationError(w, e)
			return db.CreateCommentRow{}, nil, false
		}
		if e = tx.Commit(r.Context()); e != nil {
			delegationError(w, e)
			return db.CreateCommentRow{}, nil, false
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, out)
		return db.CreateCommentRow{}, nil, false
	}
	if restricted {
		params.SuppressDispatch = pgtype.Bool{Bool: true, Valid: true}
	}
	created, err := q.CreateComment(r.Context(), params)
	if err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	snapshot := commentToResponse(created.Comment(), nil, nil)
	snapshot.IssueRevision = created.IssueRevision
	response, err = json.Marshal(snapshot)
	if err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	if _, err = q.SnapshotCommentDelegationReceipt(r.Context(), db.SnapshotCommentDelegationReceiptParams{WorkspaceID: lookup.WorkspaceID, ActorType: lookup.ActorType, ActorID: lookup.ActorID, OriginatorID: lookup.OriginatorID, RequestKey: key, Response: response}); err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	if err = tx.Commit(r.Context()); err != nil {
		delegationError(w, err)
		return db.CreateCommentRow{}, nil, false
	}
	return created, &delegationClaim{lookup: lookup, receipt: receipt, response: placeholder}, true
}

func delegationResponse(row db.CommentDelegationReceipt) (CommentResponse, error) {
	var out CommentResponse
	var targets []CommentTriggerOutcome
	if err := json.Unmarshal(row.Response, &out); err != nil {
		return out, err
	}
	if err := json.Unmarshal(row.Targets, &targets); err != nil {
		return out, err
	}
	out.DelegationReceipt = &DelegationReceipt{Key: uuidToString(row.RequestKey), State: row.State, CommentID: uuidToString(row.CommentID), Targets: targets}
	out.TriggerOutcomes = targets
	return out, nil
}

func (h *Handler) finishDelegationComment(r *http.Request, claim *delegationClaim, resp *CommentResponse) {
	// Merge by original target identity. Suppressed/missing targets remain explicitly
	// unconfirmed; a successful HTTP write must not silently drop intended targets.
	targets := append([]CommentTriggerOutcome{}, claim.receipt.Targets...)
	for i := range targets {
		for _, out := range resp.TriggerOutcomes {
			if targets[i].TargetType == out.TargetType && targets[i].TargetID == out.TargetID {
				targets[i] = out
				break
			}
		}
	}
	for i := range targets {
		if (targets[i].Status == DispatchQueued || targets[i].Status == DispatchCoalesced || targets[i].Status == DispatchDeferred) && targets[i].TaskID == "" {
			targets[i].Status = "unknown"
			targets[i].ReasonCode = "task_binding_unconfirmed"
		}
	}
	resp.TriggerOutcomes = targets
	raw, _ := json.Marshal(resp)
	targetRaw, _ := json.Marshal(targets)
	k := claim.lookup
	n, err := h.Queries.RecordCommentDelegationReceipt(r.Context(), db.RecordCommentDelegationReceiptParams{WorkspaceID: k.WorkspaceID, ActorType: k.ActorType, ActorID: k.ActorID, OriginatorID: k.OriginatorID, RequestKey: k.RequestKey, Targets: targetRaw, Response: raw, CommentID: parseUUID(claim.receipt.CommentID)})
	if err != nil || n != 1 {
		resp.TriggerOutcomes = claim.receipt.Targets
		resp.DelegationReceipt = &claim.receipt
		return
	}
	receipt := claim.receipt
	receipt.State = "recorded"
	receipt.Targets = targets
	resp.DelegationReceipt = &receipt
}

func (h *Handler) GetDelegationReceipt(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	key, err := delegationKey(chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, 400, "invalid receipt key")
		return
	}
	tx, q, err := h.delegationTx(r.Context())
	if err != nil {
		delegationError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	lookup, err := h.delegationAuthority(r, q, tx, issue, key)
	if err != nil {
		delegationError(w, err)
		return
	}
	row, err := q.GetCommentDelegationReceipt(r.Context(), lookup)
	if err != nil {
		delegationError(w, err)
		return
	}
	if row.IssueID != issue.ID {
		delegationError(w, pgx.ErrNoRows)
		return
	}
	out, err := delegationResponse(row)
	if err != nil {
		delegationError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		delegationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out.DelegationReceipt)
}

type durableDelegationContextKey struct{}

func captureDelegationTaskID(id pgtype.UUID, into []*pgtype.UUID) {
	if len(into) > 0 && into[0] != nil {
		*into[0] = id
	}
}

func (h *Handler) delegationTx(ctx context.Context) (pgx.Tx, *db.Queries, error) {
	if h.TxStarter == nil {
		return nil, nil, errors.New("transaction unavailable")
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='250ms'"); err != nil {
		tx.Rollback(ctx)
		return nil, nil, err
	}
	q := db.New(tx)
	ready, err := q.DelegationReceiptSchemaReady(ctx)
	if err != nil || !ready {
		tx.Rollback(ctx)
		if err == nil {
			err = errors.New("delegation schema unavailable")
		}
		return nil, nil, err
	}
	return tx, q, nil
}

// A context permit is scoped to one comment and exists only on the original
// committed create winner or an explicit authorized edit. It is never inherited
// by automatic completion/cancellation repair of other comments.
type delegationDispatchContextKey struct{}

const delegationReplayRefused DispatchReasonCode = "delegation_replay_not_admitted"

func (h *Handler) delegationReplayAllowed(ctx context.Context, issue db.Issue, commentID pgtype.UUID, trigger commentAgentTrigger) bool {
	if permitted, ok := ctx.Value(delegationDispatchContextKey{}).(pgtype.UUID); ok && permitted == commentID {
		return true
	}
	if !commentID.Valid {
		return true
	}
	targetType, targetID := "agent", uuidToString(trigger.Agent.ID)
	if trigger.Squad != nil {
		targetType = "squad"
		targetID = uuidToString(trigger.Squad.ID)
	}
	allowed, err := h.Queries.CommentDelegationReplayAllowed(ctx, db.CommentDelegationReplayAllowedParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, CommentID: commentID, ExecutingAgentID: uuidToString(trigger.Agent.ID), TargetType: targetType, TargetID: targetID})
	return err == nil && allowed
}
