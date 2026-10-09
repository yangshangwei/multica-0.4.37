package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Historical projections make multiple queries, so every attempt holds one RR
// snapshot and current membership locks. A revoke committed after the snapshot
// causes a serialization retry; it can never restore the old membership.
func (h *Handler) withIterationHistoryRead(ctx context.Context, ws, user pgtype.UUID, read func(pgx.Tx, *db.Queries) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := h.iterationHistoryReadAttempt(ctx, ws, user, read)
		if err == nil {
			return nil
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40001" && pgErr.Code != "40P01") {
			return err
		}
	}
	return iterationAPIError(503, "iteration_unavailable", "Iteration history changed; retry the read")
}

func (h *Handler) iterationHistoryReadAttempt(ctx context.Context, ws, user pgtype.UUID, read func(pgx.Tx, *db.Queries) error) error {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ WRITE"); err != nil {
		return err
	}
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return iterationAPIError(404, "workspace_not_found", "Workspace not found")
		}
		return err
	}
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: user}); err != nil {
		return err
	}
	if _, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: user}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return iterationAPIError(403, "forbidden", "Workspace membership required")
		}
		return err
	}
	if err = read(tx, q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) readIterationHistory(r *http.Request) (iteration.History, error) {
	return h.readIterationHistoryView(r, false)
}

func (h *Handler) readIterationHistoryView(r *http.Request, audit bool) (iteration.History, error) {
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		return iteration.History{}, err
	}
	id, err := util.ParseUUID(chi.URLParam(r, "iterationID"))
	if err != nil || id.Bytes == [16]byte{} {
		return iteration.History{}, iterationAPIError(400, "invalid_request", "Invalid iteration ID")
	}
	var result iteration.History
	err = h.withIterationHistoryRead(r.Context(), ws, actor, func(tx pgx.Tx, _ *db.Queries) error {
		now, e := iteration.SampleBusinessTime(r.Context(), tx, nil)
		if e != nil {
			return e
		}
		result, e = iteration.LoadHistory(r.Context(), tx, ws, id, now)
		if e == nil && audit && result.Snapshot != nil {
			result.Events, e = iteration.LoadHistoryEvents(r.Context(), tx, ws, id)
		}
		return e
	})
	return result, err
}

func (h *Handler) GetIteration(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	history, err := h.readIterationHistory(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": history.Iteration.WorkspaceID, "iteration": history.Iteration, "statistics": history.Statistics, "snapshot": history.Snapshot})
}

type iterationHistoryCursor struct {
	WorkspaceID   string `json:"workspace_id"`
	IterationID   string `json:"iteration_id"`
	ScopeRevision int64  `json:"scope_revision"`
	Revision      int64  `json:"revision"`
	Filter        string `json:"filter"`
	AfterID       string `json:"after_id,omitempty"`
	AfterSequence int64  `json:"after_sequence,omitempty"`
}

func historyPageLimit(query url.Values) (int, error) {
	limit := 50
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return 0, iterationAPIError(400, "invalid_request", "Limit must be between 1 and 100")
		}
		limit = value
	}
	return limit, nil
}
func historyFilterKey(query url.Values, kind string) string {
	values := url.Values{"kind": {kind}}
	for key, items := range query {
		if key != "cursor" && key != "limit" {
			values[key] = append([]string{}, items...)
		}
	}
	sum := sha256.Sum256([]byte(values.Encode()))
	return hex.EncodeToString(sum[:])
}
func decodeHistoryCursor(raw string, history iteration.History, filter string) (iterationHistoryCursor, error) {
	expected := iterationHistoryCursor{WorkspaceID: history.Iteration.WorkspaceID, IterationID: history.Iteration.ID, ScopeRevision: history.Iteration.ScopeRevision, Revision: history.Iteration.Revision, Filter: filter}
	if raw == "" {
		return expected, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 2048 {
		return expected, iterationAPIError(400, "invalid_request", "Invalid cursor")
	}
	var got iterationHistoryCursor
	if json.Unmarshal(data, &got) != nil {
		return expected, iterationAPIError(400, "invalid_request", "Invalid cursor")
	}
	if got.WorkspaceID != expected.WorkspaceID || got.IterationID != expected.IterationID || got.ScopeRevision != expected.ScopeRevision || got.Revision != expected.Revision || got.Filter != expected.Filter {
		return expected, iterationAPIError(409, "cursor_stale", "Iteration cursor changed; restart from the first page")
	}
	if got.AfterID != "" {
		id, e := util.ParseUUID(got.AfterID)
		if e != nil || id.Bytes == [16]byte{} {
			return expected, iterationAPIError(400, "invalid_request", "Invalid cursor issue ID")
		}
	}
	if got.AfterSequence < 0 || got.AfterSequence > 9007199254740991 {
		return expected, iterationAPIError(400, "invalid_request", "Invalid cursor sequence")
	}
	return got, nil
}
func encodeHistoryCursor(value iterationHistoryCursor) *string {
	raw, _ := json.Marshal(value)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}

func validateHistoryIssueFilters(query url.Values) error {
	for key, values := range query {
		if len(values) != 1 {
			return iterationAPIError(400, "invalid_request", "Repeated iteration filter")
		}
		switch key {
		case "scope", "priority", "label_id", "project_id", "assignee_type", "assignee_id", "status_category", "status", "search", "limit", "cursor":
		default:
			return iterationAPIError(400, "invalid_request", "Unknown iteration issue filter")
		}
	}
	if scope := query.Get("scope"); scope != "" && scope != "current" && scope != "original" {
		return iterationAPIError(400, "invalid_request", "Unknown issue scope")
	}
	if category := query.Get("status_category"); category != "" && !issuestatus.IsCategory(category) {
		return iterationAPIError(400, "invalid_request", "Unknown status category")
	}
	switch query.Get("priority") {
	case "", "urgent", "high", "medium", "low", "none":
	default:
		return iterationAPIError(400, "invalid_request", "Unknown priority")
	}
	if raw, ok := query["label_id"]; ok {
		id, e := util.ParseUUID(raw[0])
		if e != nil || id.Bytes == [16]byte{} {
			return iterationAPIError(400, "invalid_request", "Invalid label filter")
		}
	}
	for _, field := range []string{"project_id", "assignee_id"} {
		if value := query.Get(field); value != "" && value != "null" {
			id, e := util.ParseUUID(value)
			if e != nil || id.Bytes == [16]byte{} {
				return iterationAPIError(400, "invalid_request", "Invalid issue reference filter")
			}
		}
	}
	if kind := query.Get("assignee_type"); kind != "" && kind != "null" && kind != "member" && kind != "agent" && kind != "squad" {
		return iterationAPIError(400, "invalid_request", "Unknown assignee type")
	}
	return nil
}
func matchesHistoricalIssue(item iteration.HistoricalIssue, query url.Values) bool {
	if priority := query.Get("priority"); priority != "" && (item.Priority == nil || *item.Priority != priority) {
		return false
	}
	if label := query.Get("label_id"); label != "" {
		found := false
		for _, value := range item.Labels {
			if strings.EqualFold(value.ID, label) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for key, field := range map[string]*string{"project_id": item.ProjectID, "assignee_type": item.AssigneeType, "assignee_id": item.AssigneeID} {
		filter := query.Get(key)
		if filter == "" {
			continue
		}
		if filter == "null" {
			if field != nil {
				return false
			}
		} else if field == nil || *field != filter {
			return false
		}
	}
	if category := query.Get("status_category"); category != "" && category != item.StatusCategory {
		return false
	}
	if status := query.Get("status"); status != "" && status != item.StatusKey {
		return false
	}
	if search := strings.ToLower(strings.TrimSpace(query.Get("search"))); search != "" && !strings.Contains(strings.ToLower(item.Title+" "+item.Identifier), search) {
		return false
	}
	return true
}

type iterationIssueFilterReference struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

type iterationIssueAssigneeFilter struct {
	Type *string `json:"type"`
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

type iterationIssueFilterOptions struct {
	Statuses  []string                        `json:"statuses"`
	Projects  []iterationIssueFilterReference `json:"projects"`
	Assignees []iterationIssueAssigneeFilter  `json:"assignees"`
	Labels    []iteration.HistoricalLabel     `json:"labels"`
}

func historyReferenceKey(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Choices belong to the full selected historical projection. Using today's
// directories or a filtered task page would omit saved choices or rename them.
func historicalIterationFilterOptions(source []iteration.HistoricalIssue) iterationIssueFilterOptions {
	options := iterationIssueFilterOptions{
		Statuses: []string{}, Projects: []iterationIssueFilterReference{},
		Assignees: []iterationIssueAssigneeFilter{}, Labels: []iteration.HistoricalLabel{},
	}
	ordered := append([]iteration.HistoricalIssue{}, source...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].IssueID < ordered[j].IssueID })
	statuses, projects, labels := map[string]bool{}, map[string]bool{}, map[string]bool{}
	assignees := map[[2]string]bool{}
	for _, item := range ordered {
		if !statuses[item.StatusKey] {
			statuses[item.StatusKey] = true
			options.Statuses = append(options.Statuses, item.StatusKey)
		}
		project := historyReferenceKey(item.ProjectID)
		if !projects[project] {
			projects[project] = true
			options.Projects = append(options.Projects, iterationIssueFilterReference{ID: item.ProjectID, Name: item.ProjectName})
		}
		assignee := [2]string{historyReferenceKey(item.AssigneeType), historyReferenceKey(item.AssigneeID)}
		if !assignees[assignee] {
			assignees[assignee] = true
			options.Assignees = append(options.Assignees, iterationIssueAssigneeFilter{Type: item.AssigneeType, ID: item.AssigneeID, Name: item.AssigneeName})
		}
		for _, label := range item.Labels {
			if !labels[label.ID] {
				labels[label.ID] = true
				options.Labels = append(options.Labels, label)
			}
		}
	}
	sort.Strings(options.Statuses)
	sort.Slice(options.Projects, func(i, j int) bool {
		return historyReferenceKey(options.Projects[i].ID) < historyReferenceKey(options.Projects[j].ID)
	})
	sort.Slice(options.Assignees, func(i, j int) bool {
		iType, jType := historyReferenceKey(options.Assignees[i].Type), historyReferenceKey(options.Assignees[j].Type)
		if iType != jType {
			return iType < jType
		}
		return historyReferenceKey(options.Assignees[i].ID) < historyReferenceKey(options.Assignees[j].ID)
	})
	sort.Slice(options.Labels, func(i, j int) bool { return options.Labels[i].ID < options.Labels[j].ID })
	return options
}

func (h *Handler) ListIterationIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	if err := validateHistoryIssueFilters(query); err != nil {
		writeIterationAPIError(w, err)
		return
	}
	limit, err := historyPageLimit(query)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	history, err := h.readIterationHistory(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	// Labels and priority need not emit scope events. Bind issue cursors to the
	// coherent display projection as well as the persisted iteration revisions.
	projection, _ := json.Marshal(struct{ Original, Scope []iteration.HistoricalIssue }{history.Original, history.Scope})
	digest := sha256.Sum256(projection)
	cursor, err := decodeHistoryCursor(query.Get("cursor"), history, historyFilterKey(query, "issues")+hex.EncodeToString(digest[:]))
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	source := history.Scope
	if query.Get("scope") == "original" {
		source = history.Original
	}
	filterOptions := historicalIterationFilterOptions(source)
	selected := make([]iteration.HistoricalIssue, 0, len(source))
	for _, item := range source {
		if matchesHistoricalIssue(item, query) {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].IssueID < selected[j].IssueID })
	total := len(selected)
	first := sort.Search(len(selected), func(i int) bool { return selected[i].IssueID > cursor.AfterID })
	last := min(first+limit, len(selected))
	var next *string
	if last < len(selected) {
		cursor.AfterID = selected[last-1].IssueID
		next = encodeHistoryCursor(cursor)
	}
	writeJSON(w, 200, map[string]any{"workspace_id": history.Iteration.WorkspaceID, "iteration_id": history.Iteration.ID, "scope_revision": history.Iteration.ScopeRevision, "items": selected[first:last], "total": total, "next_cursor": next, "filter_options": filterOptions})
}

func (h *Handler) ListIterationEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	for key, values := range query {
		if len(values) != 1 || (key != "after_sequence" && key != "limit" && key != "cursor") {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Unknown or repeated event filter"))
			return
		}
	}
	limit, err := historyPageLimit(query)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	after := int64(0)
	if raw := query.Get("after_sequence"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 || after > 9007199254740991 {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid event sequence"))
			return
		}
	}
	history, err := h.readIterationHistoryView(r, true)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	cursor, err := decodeHistoryCursor(query.Get("cursor"), history, historyFilterKey(query, "events"))
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	after = max(after, cursor.AfterSequence)
	first := sort.Search(len(history.Events), func(i int) bool { return history.Events[i].Sequence > after })
	last := min(first+limit, len(history.Events))
	var next *string
	if last < len(history.Events) {
		cursor.AfterSequence = history.Events[last-1].Sequence
		next = encodeHistoryCursor(cursor)
	}
	writeJSON(w, 200, map[string]any{"workspace_id": history.Iteration.WorkspaceID, "iteration_id": history.Iteration.ID, "items": history.Events[first:last], "next_cursor": next})
}
