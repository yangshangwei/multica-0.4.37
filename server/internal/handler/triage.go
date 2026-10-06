package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type TriageSettings struct {
	Supported              bool    `json:"supported"`
	Enabled                bool    `json:"enabled"`
	AcceptanceStatus       string  `json:"acceptance_status"`
	RequirePriority        bool    `json:"require_priority"`
	ResponsibilityMode     string  `json:"responsibility_mode"`
	ResponsibilityMemberID *string `json:"responsibility_member_id"`
	Revision               int64   `json:"revision"`
}
type TriageItem struct {
	Issue                 IssueResponse `json:"issue"`
	CandidateProjectID    *string       `json:"candidate_project_id"`
	CandidateAssigneeType *string       `json:"candidate_assignee_type"`
	CandidateAssigneeID   *string       `json:"candidate_assignee_id"`
	ReviewerID            *string       `json:"reviewer_id"`
	ReviewerValid         bool          `json:"reviewer_valid"`
	Round                 int32         `json:"round"`
	FirstEnteredAt        string        `json:"first_entered_at"`
	EnteredAt             string        `json:"entered_at"`
	SnoozedUntil          *string       `json:"snoozed_until"`
	DuplicateIssueID      *string       `json:"duplicate_issue_id"`
	DuplicateIdentifier   *string       `json:"duplicate_identifier"`
	Source                string        `json:"source"`
	SourceURL             *string       `json:"source_url"`
	ExternalID            *string       `json:"external_id"`
	BatchID               *string       `json:"batch_id"`
	Filename              *string       `json:"filename"`
	RowNumber             *int32        `json:"row_number"`
}
type TriageAction struct {
	ID              string          `json:"id"`
	IssueID         string          `json:"issue_id"`
	ActorID         string          `json:"actor_id"`
	Action          string          `json:"action"`
	Round           int32           `json:"round"`
	Reason          *string         `json:"reason"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`
	CreatedAt       string          `json:"created_at"`
	ExecutionStatus string          `json:"execution_status"`
	TaskID          *string         `json:"task_id"`
	ExecutionError  *string         `json:"execution_error"`
}
type TriageActionResult struct {
	Item   TriageItem   `json:"item"`
	Action TriageAction `json:"action"`
}
type TriageIntakeInput struct {
	RequestID             string   `json:"request_id"`
	Title                 string   `json:"title"`
	Description           string   `json:"description"`
	Priority              string   `json:"priority"`
	CandidateProjectID    *string  `json:"candidate_project_id"`
	CandidateAssigneeType *string  `json:"candidate_assignee_type"`
	CandidateAssigneeID   *string  `json:"candidate_assignee_id"`
	LabelIDs              []string `json:"label_ids"`
	StartDate             *string  `json:"start_date"`
	DueDate               *string  `json:"due_date"`
	AttachmentIDs         []string `json:"attachment_ids"`
	SourceURL             string   `json:"source_url"`
}
type TriageSource struct {
	Source, SourceURL, ExternalID, Filename string
	BatchID                                 pgtype.UUID
	RowNumber                               int32
}
type triageError struct {
	Status  int
	Message string
}

func (e *triageError) Error() string { return e.Message }
func triageMissing(err error) bool   { var e *triageError; return errors.As(err, &e) && e.Status == 404 }
func triageReferenceError(err error, message string) error {
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return triageErr(400, message)
	}
	var p *pgconn.PgError
	if errors.As(err, &p) && (p.Code == "55P03" || p.Code == "40P01" || p.Code == "40001") {
		return triageErr(409, "resource changed concurrently; retry")
	}
	return err
}
func triageErr(status int, message string) error { return &triageError{status, message} }
func writeTriageError(w http.ResponseWriter, err error) {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40P01" || pgerr.Code == "40001") {
		writeError(w, 409, "triage resource changed concurrently; retry the operation")
		return
	}
	var e *triageError
	if errors.As(err, &e) {
		writeError(w, e.Status, e.Message)
		return
	}
	writeError(w, 500, "triage operation failed")
}
func triageDecode(r *http.Request, out any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 6<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return triageErr(400, "invalid request body")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return triageErr(400, "request must contain one JSON object")
	}
	return nil
}
func triageHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func triageUUID(value string, field string) (pgtype.UUID, error) {
	id, e := util.ParseUUID(value)
	if e != nil {
		return pgtype.UUID{}, triageErr(400, "invalid "+field)
	}
	return id, nil
}
func triageOptionalUUID(value *string, field string) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}
	return triageUUID(*value, field)
}
func triageText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func triageDate(value *string, field string) (pgtype.Date, error) {
	if value == nil || *value == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return pgtype.Date{}, triageErr(400, field+" must be YYYY-MM-DD")
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}
func triageSettingsResponse(s db.WorkspaceTriageSetting) TriageSettings {
	return TriageSettings{true, s.Enabled, s.AcceptanceStatus, s.RequirePriority, s.ResponsibilityMode, uuidToPtr(s.ResponsibilityMemberID), s.Revision}
}
func triageActionResponse(a db.TriageAction) TriageAction {
	return TriageAction{uuidToString(a.ID), uuidToString(a.IssueID), uuidToString(a.ActorID), a.Action, a.Round, textToPtr(a.Reason), a.BeforeSnapshot, a.AfterSnapshot, timestampToString(a.CreatedAt), a.ExecutionStatus, uuidToPtr(a.TaskID), textToPtr(a.ExecutionError)}
}

// Human decisions verify the resolved actor as well as credential class: legacy
// PAT agent requests carry a human credential but remain machine decisions.
func (h *Handler) triageHumanActor(r *http.Request, wsID string) (pgtype.UUID, error) {
	user := requestUserID(r)
	if user == "" {
		return pgtype.UUID{}, triageErr(401, "user not authenticated")
	}
	actor, _ := h.resolveActor(r, user, wsID)
	if isMachineCredentialActor(r) || actor != "member" {
		return pgtype.UUID{}, triageErr(403, "triage review requires a human member")
	}
	return triageUUID(user, "actor_id")
}
func (h *Handler) triageReadScope(r *http.Request) (pgtype.UUID, error) {
	ws := h.resolveWorkspaceID(r)
	id, err := triageUUID(ws, "workspace_id")
	if err != nil {
		return id, err
	}
	user, e := triageUUID(requestUserID(r), "user_id")
	if e != nil {
		return id, triageErr(401, "user not authenticated")
	}
	_, err = h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{WorkspaceID: id, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return id, triageErr(403, "workspace membership required")
	}
	return id, err
}

// Acquire this fence BEFORE import row/action locks. Every pending-producing
// transaction and disable operation takes the same settings row lock.
func (h *Handler) beginTriageWrite(ctx context.Context, r *http.Request, humanOnly bool) (pgx.Tx, pgtype.UUID, pgtype.UUID, db.WorkspaceTriageSetting, error) {
	var empty db.WorkspaceTriageSetting
	ws, err := h.triageReadScope(r)
	if err != nil {
		return nil, ws, pgtype.UUID{}, empty, err
	}
	actor, err := triageUUID(requestUserID(r), "actor_id")
	if humanOnly {
		actor, err = h.triageHumanActor(r, uuidToString(ws))
	}
	if err != nil {
		return nil, ws, actor, empty, err
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, ws, actor, empty, err
	}
	fail := func(e error) (pgx.Tx, pgtype.UUID, pgtype.UUID, db.WorkspaceTriageSetting, error) {
		_ = tx.Rollback(ctx)
		return nil, ws, actor, empty, e
	}
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_triage_settings(workspace_id) VALUES($1) ON CONFLICT(workspace_id) DO NOTHING`, ws); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `SELECT 1 FROM workspace_triage_settings WHERE workspace_id=$1 FOR UPDATE`, ws); err != nil {
		return fail(err)
	}
	// Use revocation's advisory guard before member and owned-agent rows.
	// Otherwise a revoke can hold an owned agent while waiting on our member.
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		return fail(err)
	}
	// Lock the current membership, so revocation cannot commit during a write.
	if _, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: actor}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = triageErr(403, "workspace membership required")
		}
		return fail(err)
	}
	if err = q.LockIssueStatusCatalogShared(ctx, ws); err != nil {
		return fail(err)
	}
	if err = iteration.LockWorkspace(ctx, tx, ws); err != nil {
		return fail(err)
	}
	settings, err := q.GetTriageSettings(ctx, ws)
	if err != nil {
		return fail(err)
	}
	return tx, ws, actor, settings, nil
}
func (h *Handler) GetTriageSettings(w http.ResponseWriter, r *http.Request) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	s, err := h.Queries.GetTriageSettings(r.Context(), ws)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, TriageSettings{Supported: true, AcceptanceStatus: "todo", ResponsibilityMode: "none", Revision: 1})
		return
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, triageSettingsResponse(s))
}
func (h *Handler) UpdateTriageSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled                bool    `json:"enabled"`
		AcceptanceStatus       string  `json:"acceptance_status"`
		RequirePriority        bool    `json:"require_priority"`
		ResponsibilityMode     string  `json:"responsibility_mode"`
		ResponsibilityMemberID *string `json:"responsibility_member_id"`
		ExpectedRevision       int64   `json:"expected_revision"`
	}
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	ctx := r.Context()
	tx, ws, actor, s, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: actor})
	if err != nil {
		writeTriageError(w, err)
		return
	}
	if member.Role != "owner" && member.Role != "admin" {
		writeError(w, 403, "only workspace owner or admin may configure triage")
		return
	}
	if in.ExpectedRevision != s.Revision {
		writeError(w, 409, "triage settings changed; refresh before saving")
		return
	}
	if err = q.LockIssueStatusCatalogShared(ctx, ws); err != nil {
		writeTriageError(w, err)
		return
	}
	status, err := issuestatus.Resolve(ctx, q, ws, in.AcceptanceStatus)
	if err != nil || status.Category != "backlog" && status.Category != "todo" {
		writeError(w, 400, "acceptance_status must be an active backlog or todo status")
		return
	}
	if in.ResponsibilityMode != "none" && in.ResponsibilityMode != "notify" && in.ResponsibilityMode != "assign" {
		writeError(w, 400, "invalid responsibility_mode")
		return
	}
	memberID, err := triageOptionalUUID(in.ResponsibilityMemberID, "responsibility_member_id")
	if err == nil && in.ResponsibilityMode != "none" && !memberID.Valid {
		err = triageErr(400, "select a responsibility member")
	}
	if err == nil && memberID.Valid {
		err = triageValidateMember(ctx, q, ws, memberID)
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	if !in.Enabled {
		var n int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND admission_status='pending'`, ws).Scan(&n)
		if err != nil {
			writeTriageError(w, err)
			return
		}
		if n > 0 {
			writeError(w, 409, fmt.Sprintf("cannot disable triage while %d pending items remain, including snoozed items", n))
			return
		}
	}
	_, err = tx.Exec(ctx, `UPDATE workspace_triage_settings SET enabled=$2,acceptance_status=$3,require_priority=$4,responsibility_mode=$5,responsibility_member_id=$6,revision=revision+1,updated_by=$7,updated_at=now() WHERE workspace_id=$1`, ws, in.Enabled, in.AcceptanceStatus, in.RequirePriority, in.ResponsibilityMode, memberID, actor)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	s, err = q.GetTriageSettings(ctx, ws)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	h.broadcastTriage(ws, "", "", true)
	writeJSON(w, 200, triageSettingsResponse(s))
}
func triageValidateMember(ctx context.Context, q *db.Queries, ws, id pgtype.UUID) error {
	_, err := q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return triageErr(400, "member is no longer in this workspace")
	}
	return err
}
func (h *Handler) broadcastTriage(ws pgtype.UUID, issue, batch string, settings bool) {
	data := map[string]any{"workspace_id": uuidToString(ws)}
	if issue != "" {
		data["issue_id"] = issue
	}
	if batch != "" {
		data["batch_id"] = batch
	}
	if settings {
		data["settings_changed"] = true
	}
	h.publish("triage:updated", uuidToString(ws), "system", "", data)
}

func (h *Handler) triageItem(ctx context.Context, q *db.Queries, ws, id pgtype.UUID) (TriageItem, error) {
	var out TriageItem
	i, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: ws})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, triageErr(404, "triage item unavailable")
	}
	if err != nil {
		return out, err
	}
	t, err := q.GetIssueTriage(ctx, db.GetIssueTriageParams{IssueID: id, WorkspaceID: ws})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, triageErr(404, "triage item unavailable")
	}
	if err != nil {
		return out, err
	}
	workspace, err := q.GetWorkspace(ctx, ws)
	if err != nil {
		return out, err
	}
	out = TriageItem{Issue: issueToResponse(i, workspace.IssuePrefix), CandidateProjectID: uuidToPtr(t.CandidateProjectID), CandidateAssigneeType: textToPtr(t.CandidateAssigneeType), CandidateAssigneeID: uuidToPtr(t.CandidateAssigneeID), ReviewerID: uuidToPtr(t.ReviewerID), Round: t.Round, FirstEnteredAt: timestampToString(t.FirstEnteredAt), EnteredAt: timestampToString(t.EnteredAt), SnoozedUntil: timestampToNanoPtr(t.SnoozedUntil), DuplicateIssueID: uuidToPtr(t.DuplicateIssueID), DuplicateIdentifier: textToPtr(t.DuplicateIdentifier), Source: t.Source, SourceURL: textToPtr(t.SourceUrl), ExternalID: textToPtr(t.ExternalID), BatchID: uuidToPtr(t.BatchID), Filename: textToPtr(t.Filename), RowNumber: int4ToPtr(t.RowNumber)}
	if t.ReviewerID.Valid {
		_, e := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: t.ReviewerID})
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		out.ReviewerValid = e == nil
	}
	labels, err := q.ListLabelsByIssue(ctx, db.ListLabelsByIssueParams{IssueID: i.ID, WorkspaceID: ws})
	if err != nil {
		return out, err
	}
	resolved, statusErr := issuestatus.Resolve(ctx, q, ws, i.Status)
	if statusErr == nil {
		out.Issue.StatusCategory = resolved.Category
		out.Issue.StatusName = resolved.Name
	}
	labelResponses := labelsToResponse(labels)
	out.Issue.Labels = &labelResponses
	return out, nil
}
func (h *Handler) triageResolveID(ctx context.Context, q *db.Queries, ws pgtype.UUID, value string) (pgtype.UUID, error) {
	if id, e := util.ParseUUID(value); e == nil {
		return id, nil
	}
	workspace, err := q.GetWorkspace(ctx, ws)
	if err != nil {
		return pgtype.UUID{}, err
	}
	prefix := workspace.IssuePrefix + "-"
	if !strings.HasPrefix(value, prefix) {
		return pgtype.UUID{}, triageErr(404, "issue unavailable")
	}
	number, e := strconv.ParseInt(strings.TrimPrefix(value, prefix), 10, 32)
	if e != nil || number <= 0 {
		return pgtype.UUID{}, triageErr(404, "issue unavailable")
	}
	i, err := q.GetIssueByNumber(ctx, db.GetIssueByNumberParams{WorkspaceID: ws, Number: int32(number)})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, err
		}
		return pgtype.UUID{}, triageErr(404, "issue unavailable")
	}
	return i.ID, nil
}
func (h *Handler) GetTriageItem(w http.ResponseWriter, r *http.Request) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	id, err := h.triageResolveID(r.Context(), h.Queries, ws, chi.URLParam(r, "id"))
	if err != nil {
		writeTriageError(w, err)
		return
	}
	out, err := h.triageItem(r.Context(), h.Queries, ws, id)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) GetTriageItemHistory(w http.ResponseWriter, r *http.Request) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	id, err := h.triageResolveID(r.Context(), h.Queries, ws, chi.URLParam(r, "id"))
	if err != nil {
		writeTriageError(w, err)
		return
	}
	rows, err := h.Queries.ListTriageIssueActions(r.Context(), db.ListTriageIssueActionsParams{WorkspaceID: ws, IssueID: id})
	if err != nil {
		writeTriageError(w, err)
		return
	}
	out := make([]TriageAction, 0, len(rows))
	for _, a := range rows {
		out = append(out, triageActionResponse(a))
	}
	writeJSON(w, 200, map[string]any{"events": out})
}
func triageValidateIntake(in TriageIntakeInput) error {
	if strings.TrimSpace(in.Title) == "" || utf8.RuneCountInString(in.Title) > 500 || len(in.Description) > 1<<20 || strings.ContainsRune(in.Title+in.Description, 0) {
		return triageErr(400, "title is required (maximum 500 characters); description maximum is 1 MiB")
	}
	if in.Priority != "" && in.Priority != "none" && in.Priority != "low" && in.Priority != "medium" && in.Priority != "high" && in.Priority != "urgent" {
		return triageErr(400, "invalid priority")
	}
	if in.SourceURL != "" {
		u, e := url.Parse(in.SourceURL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return triageErr(400, "source_url must be an HTTP or HTTPS URL")
		}
	}
	return nil
}
func (h *Handler) createTriageItemInTx(ctx context.Context, tx pgx.Tx, r *http.Request, ws, actor pgtype.UUID, s db.WorkspaceTriageSetting, in TriageIntakeInput, source TriageSource) (TriageItem, error) {
	var out TriageItem
	if !s.Enabled {
		return out, triageErr(409, "enable triage before submitting items")
	}
	if err := triageValidateIntake(in); err != nil {
		return out, err
	}
	q := h.Queries.WithTx(tx)
	project, err := triageOptionalUUID(in.CandidateProjectID, "candidate_project_id")
	if err != nil {
		return out, err
	}
	assignee, err := triageOptionalUUID(in.CandidateAssigneeID, "candidate_assignee_id")
	if err != nil {
		return out, err
	}
	kind := pgtype.Text{}
	if in.CandidateAssigneeType != nil {
		kind = triageText(*in.CandidateAssigneeType)
	}
	if err = h.triageValidateCandidates(ctx, tx, r, ws, project, kind, assignee); err != nil {
		return out, err
	}
	start, err := triageDate(in.StartDate, "start_date")
	if err != nil {
		return out, err
	}
	due, err := triageDate(in.DueDate, "due_date")
	if err != nil {
		return out, err
	}
	if start.Valid && due.Valid && start.Time.After(due.Time) {
		return out, triageErr(400, "start_date must not be after due_date")
	}
	labels := make([]pgtype.UUID, 0, len(in.LabelIDs))
	for _, v := range in.LabelIDs {
		id, e := triageUUID(v, "label_ids")
		if e != nil {
			return out, e
		}
		var locked pgtype.UUID
		if e = tx.QueryRow(ctx, `SELECT id FROM issue_label WHERE id=$1 AND workspace_id=$2 AND resource_type='issue' FOR SHARE`, id, ws).Scan(&locked); e != nil {
			return out, triageReferenceError(e, "label unavailable")
		}
		labels = append(labels, id)
	}
	attachments := make([]pgtype.UUID, 0, len(in.AttachmentIDs))
	for _, v := range in.AttachmentIDs {
		id, e := triageUUID(v, "attachment_ids")
		if e != nil {
			return out, e
		}
		attachments = append(attachments, id)
	}
	actorType, creator := h.resolveActor(r, uuidToString(actor), uuidToString(ws))
	if actorType == "agent" {
		creatorUUID, e := triageUUID(creator, "creator_id")
		if e != nil {
			return out, e
		}
		var level string
		if e = tx.QueryRow(ctx, `SELECT autonomy_level FROM agent WHERE id=$1 AND workspace_id=$2 AND archived_at IS NULL FOR SHARE NOWAIT`, creatorUUID, ws).Scan(&level); e != nil {
			return out, triageErr(403, "acting agent unavailable")
		}
		if !service.AutonomyAtLeast(level, service.AutonomyContributor) {
			return out, triageErr(403, autonomyDenialMessage(level, service.AutonomyContributor, "create issues"))
		}
	}
	creatorID, e := triageUUID(creator, "creator_id")
	if e != nil {
		return out, e
	}
	// Lock and validate uploader identity before binding, never claim another
	// member's draft uploads simply because they are in the same workspace.
	for _, id := range attachments {
		var valid bool
		err = tx.QueryRow(ctx, `SELECT uploader_type=$3 AND uploader_id=$4 AND issue_id IS NULL AND comment_id IS NULL AND chat_session_id IS NULL AND chat_message_id IS NULL AND source_context_id IS NULL FROM attachment WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, id, ws, actorType, creatorID).Scan(&valid)
		if err != nil || !valid {
			return out, triageErr(400, "attachment unavailable or already linked")
		}
	}
	priority := in.Priority
	if priority == "" {
		priority = "none"
	}
	created, err := h.IssueService.CreateInTx(ctx, tx, service.IssueCreateParams{WorkspaceID: ws, Title: strings.TrimSpace(in.Title), Description: triageText(in.Description), Status: "backlog", Priority: priority, CreatorType: actorType, CreatorID: creatorID, LabelIDs: labels, StartDate: start, DueDate: due, AllowDuplicate: true}, service.ResolveIssueCountPolicy(ctx, h.IssueService.Entitlements, ws))
	if err != nil {
		return out, err
	}
	id := created.Issue.ID
	if _, err = tx.Exec(ctx, `UPDATE issue SET admission_status='pending' WHERE id=$1`, id); err != nil {
		return out, err
	}
	reviewer := pgtype.UUID{}
	if s.ResponsibilityMode == "assign" {
		reviewer = s.ResponsibilityMemberID
		if reviewer.Valid {
			if e = triageValidateMember(ctx, q, ws, reviewer); e != nil {
				reviewer = pgtype.UUID{}
			}
		}
	}
	if source.Source == "" {
		source.Source = "manual"
	}
	if source.SourceURL == "" {
		source.SourceURL = in.SourceURL
	}
	_, err = tx.Exec(ctx, `INSERT INTO issue_triage(issue_id,workspace_id,reviewer_id,candidate_project_id,candidate_assignee_type,candidate_assignee_id,source,source_url,external_id,batch_id,filename,row_number) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, ws, reviewer, project, kind, assignee, source.Source, triageText(source.SourceURL), triageText(source.ExternalID), source.BatchID, triageText(source.Filename), pgtype.Int4{Int32: source.RowNumber, Valid: source.RowNumber > 0})
	if err != nil {
		return out, err
	}
	if len(attachments) > 0 {
		_, err = q.LinkAttachmentsToIssue(ctx, db.LinkAttachmentsToIssueParams{WorkspaceID: ws, IssueID: id, AttachmentIds: attachments, BumpRevision: false})
		if err != nil {
			return out, err
		}
	}
	// Inert intake skips issue:created dispatch listeners, but the actual human
	// creator still follows later comments through the ordinary subscriber path.
	// Agent creators do not imply an inferred human subscriber.
	if actorType == "member" {
		if _, err = q.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{IssueID: id, UserType: "member", UserID: creatorID, Reason: "creator"}); err != nil {
			return out, err
		}
	}
	out, err = h.triageItem(ctx, q, ws, id)
	if err != nil {
		return out, err
	}
	if source.Source != "csv" && s.ResponsibilityMode != "none" && s.ResponsibilityMemberID.Valid && s.ResponsibilityMemberID != actor {
		err = h.queueTriageNotification(ctx, tx, ws, s.ResponsibilityMemberID, id, pgtype.UUID{}, "entered:"+uuidToString(id)+":1", "New item awaiting triage", nil, time.Now())
	}
	return out, err
}
func (h *Handler) CreateTriageItem(w http.ResponseWriter, r *http.Request) {
	if !h.requireAgentAutonomy(w, r, h.resolveWorkspaceID(r), service.AutonomyContributor, "create issues") {
		return
	}
	var in TriageIntakeInput
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	request, err := triageUUID(in.RequestID, "request_id")
	if err != nil {
		writeTriageError(w, err)
		return
	}
	ctx := r.Context()
	tx, ws, actor, s, err := h.beginTriageWrite(ctx, r, false)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	hash := triageHash(in)
	var priorID pgtype.UUID
	var priorHash string
	err = tx.QueryRow(ctx, `SELECT issue_id,payload_hash FROM triage_intake_request WHERE workspace_id=$1 AND actor_id=$2 AND request_id=$3`, ws, actor, request).Scan(&priorID, &priorHash)
	if err == nil {
		if priorHash != hash {
			writeError(w, 409, "request_id was already used for different input")
			return
		}
		out, e := h.triageItem(ctx, h.Queries.WithTx(tx), ws, priorID)
		if e != nil {
			if !triageMissing(e) {
				writeTriageError(w, e)
				return
			}
			writeError(w, 409, "previously created triage item was deleted: "+uuidToString(priorID))
			return
		}
		writeJSON(w, 201, out)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeTriageError(w, err)
		return
	}
	out, err := h.createTriageItemInTx(ctx, tx, r, ws, actor, s, in, TriageSource{})
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO triage_intake_request(workspace_id,actor_id,request_id,payload_hash,issue_id) VALUES($1,$2,$3,$4,$5)`, ws, actor, request, hash, parseUUID(out.Issue.ID))
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	h.broadcastTriage(ws, out.Issue.ID, "", false)
	h.deliverTriageNotifications(ctx, ws)
	writeJSON(w, 201, out)
}
