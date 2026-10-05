package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// Date-only upper bounds include the whole selected calendar day. Explicit
// timestamps retain their exact inclusive instant semantics.
func triageDateBound(value string, upper bool) (time.Time, string, error) {
	if date, err := time.Parse("2006-01-02", value); err == nil {
		if upper {
			return date.AddDate(0, 0, 1), "<", nil
		}
		return date, ">=", nil
	}
	at, err := time.Parse(time.RFC3339, value)
	if upper {
		return at, "<=", err
	}
	return at, ">=", err
}
func triagePagination(r *http.Request) (int, int, error) {
	limit := 50
	offset := 0
	var err error
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, triageErr(400, "limit must be between 1 and 100")
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil || offset < 0 {
			return 0, 0, triageErr(400, "offset must be nonnegative")
		}
	}
	return limit, offset, nil
}
func (h *Handler) ListTriageItems(w http.ResponseWriter, r *http.Request) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	limit, offset, err := triagePagination(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	ctx := r.Context()
	h.deliverTriageNotifications(ctx, ws)
	counts := struct {
		Pending int `json:"pending"`
		Ready   int `json:"ready"`
		Snoozed int `json:"snoozed"`
	}{}
	err = h.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE t.snoozed_until IS NULL OR t.snoozed_until<=now()),count(*) FILTER(WHERE t.snoozed_until>now()) FROM issue_triage t JOIN issue i ON i.id=t.issue_id WHERE t.workspace_id=$1 AND i.admission_status='pending'`, ws).Scan(&counts.Pending, &counts.Ready, &counts.Snoozed)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	params := r.URL.Query()
	args := []any{ws}
	where := []string{"t.workspace_id=$1"}
	add := func(expr string, arg any) {
		args = append(args, arg)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	switch params.Get("view") {
	case "", "ready":
		where = append(where, "i.admission_status='pending' AND (t.snoozed_until IS NULL OR t.snoozed_until<=now())")
	case "all":
		where = append(where, "i.admission_status='pending'")
	case "snoozed":
		where = append(where, "i.admission_status='pending' AND t.snoozed_until>now()")
	case "history":
		where = append(where, "i.admission_status IN ('accepted','rejected','duplicate')")
	default:
		writeError(w, 400, "invalid triage view")
		return
	}
	if v := params.Get("q"); v != "" {
		add("(i.title ILIKE $%[1]d OR (w.issue_prefix||'-'||i.number::text) ILIKE $%[1]d)", "%"+v+"%")
	}
	for key, column := range map[string]string{"source": "t.source", "priority": "i.priority", "result": "i.admission_status"} {
		if v := params.Get(key); v != "" {
			add(column+"=$%d", v)
		}
	}
	for key, column := range map[string]string{"project_id": "t.candidate_project_id", "reviewer_id": "t.reviewer_id", "creator_id": "i.creator_id"} {
		if v := params.Get(key); v != "" {
			if v == "none" {
				where = append(where, column+" IS NULL")
				continue
			}
			id, e := triageUUID(v, key)
			if e != nil {
				writeTriageError(w, e)
				return
			}
			add(column+"=$%d", id)
		}
	}
	if v := params.Get("label_id"); v != "" {
		id, e := triageUUID(v, "label_id")
		if e != nil {
			writeTriageError(w, e)
			return
		}
		add("EXISTS(SELECT 1 FROM issue_to_label il WHERE il.issue_id=i.id AND il.label_id=$%d)", id)
	}
	for key, op := range map[string]string{"entered_after": ">=", "entered_before": "<="} {
		if v := params.Get(key); v != "" {
			at, comparator, e := triageDateBound(v, op == "<=")
			op = comparator
			if e != nil {
				writeError(w, 400, "invalid "+key)
				return
			}
			add("t.entered_at"+op+"$%d", at)
		}
	}
	if v := params.Get("processed_by"); v != "" {
		id, e := triageUUID(v, "processed_by")
		if e != nil {
			writeTriageError(w, e)
			return
		}
		add("EXISTS(SELECT 1 FROM triage_action a WHERE a.issue_id=i.id AND a.actor_id=$%d)", id)
	}
	for key, op := range map[string]string{"processed_after": ">=", "processed_before": "<="} {
		if v := params.Get(key); v != "" {
			at, comparator, e := triageDateBound(v, op == "<=")
			op = comparator
			if e != nil {
				writeError(w, 400, "invalid "+key)
				return
			}
			add("EXISTS(SELECT 1 FROM triage_action a WHERE a.issue_id=i.id AND a.created_at"+op+"$%d)", at)
		}
	}
	base := " FROM issue_triage t JOIN issue i ON i.id=t.issue_id JOIN workspace w ON w.id=t.workspace_id WHERE " + strings.Join(where, " AND ")
	var total int
	err = h.DB.QueryRow(ctx, "SELECT count(*)"+base, args...).Scan(&total)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	order := "t.entered_at ASC,i.id ASC"
	switch params.Get("sort") {
	case "", "oldest":
	case "newest":
		order = "t.entered_at DESC,i.id DESC"
	case "priority":
		order = "CASE i.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,t.entered_at,i.id"
	default:
		writeError(w, 400, "invalid sort")
		return
	}
	args = append(args, limit, offset)
	rows, err := h.DB.Query(ctx, "SELECT i.id"+base+" ORDER BY "+order+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	ids := []pgtype.UUID{}
	for rows.Next() {
		var id pgtype.UUID
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	items := make([]TriageItem, 0, len(ids))
	for _, id := range ids {
		item, e := h.triageItem(ctx, h.Queries, ws, id)
		if e != nil {
			writeTriageError(w, e)
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items, "total": total, "counts": counts, "limit": limit, "offset": offset})
}

type TriageHistoryEntry struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	IssueID    *string         `json:"issue_id"`
	Identifier *string         `json:"identifier"`
	Title      string          `json:"title"`
	Action     string          `json:"action"`
	ActorID    string          `json:"actor_id"`
	CreatedAt  string          `json:"created_at"`
	Reason     *string         `json:"reason"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	BatchID    *string         `json:"batch_id"`
	Filename   *string         `json:"filename"`
	Counts     map[string]int  `json:"counts"`
}

func (h *Handler) GetTriageHistory(w http.ResponseWriter, r *http.Request) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	limit, offset, err := triagePagination(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	params := r.URL.Query()
	args := []any{ws}
	where := []string{"true"}
	add := func(expr string, arg any) {
		args = append(args, arg)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if v := params.Get("q"); v != "" {
		add("(title ILIKE $%[1]d OR identifier ILIKE $%[1]d OR filename ILIKE $%[1]d)", "%"+v+"%")
	}
	if v := params.Get("result"); v != "" {
		if v == "accepted" {
			where = append(where, "action IN ('accept','accept_and_execute')")
		} else if v == "rejected" {
			where = append(where, "action='reject'")
		} else {
			add("action=$%d", v)
		}
	}
	if v := params.Get("source"); v != "" {
		add("source=$%d", v)
	}
	if v := params.Get("processed_by"); v != "" {
		id, e := triageUUID(v, "processed_by")
		if e != nil {
			writeTriageError(w, e)
			return
		}
		add("actor_id=$%d", id)
	}
	for key, op := range map[string]string{"processed_after": ">=", "processed_before": "<="} {
		if v := params.Get(key); v != "" {
			at, comparator, e := triageDateBound(v, op == "<=")
			op = comparator
			if e != nil {
				writeError(w, 400, "invalid "+key)
				return
			}
			add("created_at"+op+"$%d", at)
		}
	}
	union := `WITH entries AS (SELECT id,'action'::text kind,issue_id,after_snapshot->'issue'->>'identifier' identifier,COALESCE(after_snapshot->'issue'->>'title','') title,action,actor_id,created_at,reason,before_snapshot,after_snapshot,NULL::uuid batch_id,after_snapshot->>'filename' filename,NULL::jsonb counts,after_snapshot->>'source' source FROM triage_action WHERE workspace_id=$1 UNION ALL SELECT id,'import',NULL,NULL,filename,'import',actor_id,committed_at,NULL,'{}'::jsonb,'{}'::jsonb,id,filename,jsonb_build_object('created',created_count,'skipped',skipped_count,'failed',failed_count),'csv' FROM triage_import_batch WHERE workspace_id=$1 AND committed_at IS NOT NULL) `
	filter := " FROM entries WHERE " + strings.Join(where, " AND ")
	var total int
	if err = h.DB.QueryRow(r.Context(), union+"SELECT count(*)"+filter, args...).Scan(&total); err != nil {
		writeTriageError(w, err)
		return
	}
	args = append(args, limit, offset)
	rows, err := h.DB.Query(r.Context(), union+"SELECT id,kind,issue_id,identifier,title,action,actor_id,created_at,reason,before_snapshot,after_snapshot,batch_id,filename,counts"+filter+fmt.Sprintf(" ORDER BY created_at DESC,id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	defer rows.Close()
	entries := []TriageHistoryEntry{}
	for rows.Next() {
		var e TriageHistoryEntry
		var id, issue, actor, batch pgtype.UUID
		var identifier, reason, filename pgtype.Text
		var at time.Time
		var counts []byte
		if err = rows.Scan(&id, &e.Kind, &issue, &identifier, &e.Title, &e.Action, &actor, &at, &reason, &e.Before, &e.After, &batch, &filename, &counts); err != nil {
			writeTriageError(w, err)
			return
		}
		e.ID = uuidToString(id)
		e.IssueID = uuidToPtr(issue)
		e.Identifier = textToPtr(identifier)
		e.ActorID = uuidToString(actor)
		e.CreatedAt = at.Format(time.RFC3339Nano)
		e.Reason = textToPtr(reason)
		e.BatchID = uuidToPtr(batch)
		e.Filename = textToPtr(filename)
		if len(counts) > 0 {
			if err = json.Unmarshal(counts, &e.Counts); err != nil {
				writeTriageError(w, err)
				return
			}
		}
		entries = append(entries, e)
	}
	if err = rows.Err(); err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"entries": entries, "total": total, "limit": limit, "offset": offset})
}
func triageBatchAllowed(action string) bool {
	return action == "accept" || action == "reject" || action == "snooze" || action == "assign_reviewer"
}
func (h *Handler) PreviewTriageBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []struct {
			IssueID          string `json:"issue_id"`
			ExpectedRevision int64  `json:"expected_revision"`
		} `json:"items"`
		Action       string                     `json:"action"`
		Reason       string                     `json:"reason"`
		SnoozedUntil string                     `json:"snoozed_until"`
		ReviewerID   *string                    `json:"reviewer_id"`
		Fields       map[string]json.RawMessage `json:"fields"`
	}
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	if !triageBatchAllowed(in.Action) || len(in.Items) == 0 || len(in.Items) > 100 {
		writeError(w, 400, "batch accepts 1–100 selected items and accept/reject/snooze/assign_reviewer only")
		return
	}
	type row struct {
		IssueID          string  `json:"issue_id"`
		ExpectedRevision int64   `json:"expected_revision"`
		Valid            bool    `json:"valid"`
		Error            *string `json:"error"`
	}
	results := []row{}
	valid := 0
	seen := map[string]bool{}
	for _, i := range in.Items {
		input := TriageActionInput{RequestID: uuidToString(dbid.NewV7()), ExpectedRevision: i.ExpectedRevision, Action: in.Action, Reason: in.Reason, SnoozedUntil: in.SnoozedUntil, ReviewerID: in.ReviewerID, Fields: in.Fields}
		_, err := h.actOnTriageItem(r, i.IssueID, input, true)
		if seen[i.IssueID] {
			err = triageErr(400, "item appears more than once")
		}
		seen[i.IssueID] = true
		result := row{IssueID: i.IssueID, ExpectedRevision: i.ExpectedRevision, Valid: err == nil}
		if err != nil {
			message := err.Error()
			result.Error = &message
		} else {
			valid++
		}
		results = append(results, result)
	}
	writeJSON(w, 200, map[string]any{"items": results, "valid_count": valid})
}
func (h *Handler) CommitTriageBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []struct {
			IssueID string `json:"issue_id"`
			TriageActionInput
		} `json:"items"`
	}
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	if len(in.Items) == 0 || len(in.Items) > 100 {
		writeError(w, 400, "select 1–100 items")
		return
	}
	ws, scopeErr := h.triageReadScope(r)
	if scopeErr != nil {
		writeTriageError(w, scopeErr)
		return
	}
	selected := map[string]bool{}
	for _, i := range in.Items {
		key := i.IssueID
		if id, e := h.triageResolveID(r.Context(), h.Queries, ws, key); e == nil {
			key = uuidToString(id)
		}
		if selected[key] {
			writeError(w, 400, "an item may appear only once in a batch")
			return
		}
		selected[key] = true
	}
	for _, i := range in.Items {
		if !triageBatchAllowed(i.Action) {
			writeError(w, 400, "batch action is not allowed")
			return
		}
	}
	type row struct {
		IssueID string              `json:"issue_id"`
		Status  string              `json:"status"`
		Result  *TriageActionResult `json:"result,omitempty"`
		Error   string              `json:"error,omitempty"`
	}
	results := []row{}
	success := 0
	for _, i := range in.Items {
		out, err := h.actOnTriageItem(r, i.IssueID, i.TriageActionInput, false)
		result := row{IssueID: i.IssueID, Status: "success"}
		if err == nil {
			success++
			result.Result = &out
		} else {
			result.Status = "failed"
			result.Error = err.Error()
			var e *triageError
			if errors.As(err, &e) {
				switch e.Status {
				case 400, 404:
					result.Status = "invalid"
				case 403:
					result.Status = "forbidden"
				case 409:
					result.Status = "conflict"
				}
			}
		}
		results = append(results, result)
	}
	summaryKeys := make([]string, 0, len(in.Items))
	for _, i := range in.Items {
		summaryKeys = append(summaryKeys, i.RequestID)
	}
	sort.Strings(summaryKeys)
	h.triageBatchSummary(r.Context(), r, triageHash(summaryKeys), success, len(results)-success)
	writeJSON(w, 200, map[string]any{"results": results, "success_count": success})
}
