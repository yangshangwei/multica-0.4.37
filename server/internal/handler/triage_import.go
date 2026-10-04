package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/triagecsv"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type TriageImportRow struct {
	RowNumber        int               `json:"row_number"`
	Values           map[string]string `json:"values"`
	Warnings         []string          `json:"warnings"`
	Errors           []string          `json:"errors"`
	Duplicate        bool              `json:"duplicate"`
	DuplicateIssueID *string           `json:"duplicate_issue_id"`
	SimilarIssueIDs  []string          `json:"similar_issue_ids"`
	Status           string            `json:"status"`
	IssueID          *string           `json:"issue_id"`
	Error            *string           `json:"error"`
}
type TriageImportPreview struct {
	BatchID  string            `json:"batch_id"`
	Filename string            `json:"filename"`
	Headers  []string          `json:"headers"`
	Mapping  map[string]string `json:"mapping"`
	Rows     []TriageImportRow `json:"rows"`
	Counts   struct {
		Valid     int `json:"valid"`
		Warning   int `json:"warning"`
		Error     int `json:"error"`
		Duplicate int `json:"duplicate"`
	} `json:"counts"`
	Limits struct {
		MaxRows  int `json:"max_rows"`
		MaxBytes int `json:"max_bytes"`
	} `json:"limits"`
}
type TriageImportResult struct {
	BatchID string            `json:"batch_id"`
	Results []TriageImportRow `json:"results"`
	Created int               `json:"created"`
	Skipped int               `json:"skipped"`
	Failed  int               `json:"failed"`
}

// Source cells are base64-encoded by JSON because PostgreSQL JSONB rejects
// NUL string escapes, including original cells of deliberately invalid rows.
type triageImportPlan struct {
	Input TriageIntakeInput `json:"input"`
	Cells [][]byte          `json:"cells"`
}
type triageImportSelection struct {
	RowNumber       int  `json:"row_number"`
	ImportDuplicate bool `json:"import_duplicate"`
}

// JSON accepts invalid UTF-8 by substituting replacement characters. Validate
// the raw upload before decoding, and reject already-lossy browser decoding.
func decodeTriageImport(r *http.Request, out any) error {
	const maxJSONBytes = triagecsv.MaxBytes*6 + 64*1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBytes+1))
	if err != nil || len(body) > maxJSONBytes {
		return triageErr(400, "import request exceeds the upload limit")
	}
	if !utf8.Valid(body) {
		return triageErr(400, "CSV must be UTF-8 encoded; convert the file to UTF-8 before importing")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return triageErr(400, "invalid import request body")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return triageErr(400, "request must contain one JSON object")
	}
	return nil
}

func (h *Handler) PreviewTriageImport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RequestID string            `json:"request_id"`
		Filename  string            `json:"filename"`
		CSV       string            `json:"csv"`
		Mapping   map[string]string `json:"mapping"`
	}
	if err := decodeTriageImport(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	request, err := triageUUID(in.RequestID, "request_id")
	if err != nil {
		writeTriageError(w, err)
		return
	}
	if strings.TrimSpace(in.Filename) == "" || len(in.Filename) > 1000 || strings.ContainsAny(in.Filename, "\x00\r\n") {
		writeError(w, 400, "filename is required and must contain at most 1000 bytes")
		return
	}
	if strings.ContainsRune(in.CSV, utf8.RuneError) {
		writeError(w, 400, "CSV contains replacement characters; reopen the original file as UTF-8")
		return
	}
	parsed, err := triagecsv.Parse([]byte(in.CSV), in.Mapping)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	for _, header := range parsed.Headers {
		if strings.ContainsRune(header, 0) {
			writeError(w, 400, "CSV headers must not contain NUL characters")
			return
		}
	}
	ctx := r.Context()
	tx, ws, actor, settings, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if !settings.Enabled {
		writeError(w, 409, "enable triage before importing")
		return
	}
	hash := triageHash(in)
	var existing pgtype.UUID
	var existingHash string
	err = tx.QueryRow(ctx, `SELECT id,payload_hash FROM triage_import_batch WHERE workspace_id=$1 AND actor_id=$2 AND request_id=$3`, ws, actor, request).Scan(&existing, &existingHash)
	if err == nil {
		if existingHash != hash {
			writeError(w, 409, "request_id was already used for different input")
			return
		}
		preview, e := loadTriageImport(ctx, tx, ws, actor, existing)
		if e != nil {
			writeTriageError(w, e)
			return
		}
		writeJSON(w, 200, preview)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeTriageError(w, err)
		return
	}
	batch := dbid.NewV7()
	headers, _ := json.Marshal(parsed.Headers)
	mapping, _ := json.Marshal(parsed.Mapping)
	_, err = tx.Exec(ctx, `INSERT INTO triage_import_batch(id,workspace_id,actor_id,request_id,payload_hash,filename,headers,mapping) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, batch, ws, actor, request, hash, in.Filename, headers, mapping)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	resolver := triageImportResolver{h: h, tx: tx, ws: ws, actor: actor, cache: map[string][]string{}}
	for _, row := range parsed.Rows {
		plan, warning, e := resolver.resolve(ctx, row)
		if e != nil {
			writeTriageError(w, e)
			return
		}
		row.Warnings = append(row.Warnings, warning...)
		if len(row.Values["external_id"]) > 1000 {
			row.Errors = append(row.Errors, "External ID exceeds 1000 UTF-8 bytes")
		}
		if e = triageValidateIntake(plan.Input); e != nil && len(row.Errors) == 0 {
			row.Errors = append(row.Errors, e.Error())
		}
		duplicate, e := triageImportExternal(ctx, tx, ws, row.Values["external_id"])
		if e != nil {
			writeTriageError(w, e)
			return
		}
		if duplicate.Valid {
			row.Duplicate = true
			row.Warnings = append(row.Warnings, "External ID was already imported; skipped unless explicitly included")
		}
		similar, e := triageImportSimilar(ctx, tx, ws, row.Values["title"])
		if e != nil {
			writeTriageError(w, e)
			return
		}
		if len(similar) > 0 {
			row.Warnings = append(row.Warnings, "Similar titles exist; review duplicate candidates after importing")
		}
		values, _ := json.Marshal(row.Values)
		normalized, _ := json.Marshal(plan)
		warnings, _ := json.Marshal(row.Warnings)
		errs, _ := json.Marshal(row.Errors)
		similarJSON, _ := json.Marshal(similar)
		_, err = tx.Exec(ctx, `INSERT INTO triage_import_row(batch_id,workspace_id,row_number,values,normalized,warnings,errors,duplicate,duplicate_issue_id,similar_issue_ids,external_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, batch, ws, row.Number, values, normalized, warnings, errs, row.Duplicate, duplicate, similarJSON, triageText(row.Values["external_id"]))
		if err != nil {
			writeTriageError(w, err)
			return
		}
	}
	preview, err := loadTriageImport(ctx, tx, ws, actor, batch)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, preview)
}

type triageImportDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadTriageImport(ctx context.Context, q triageImportDB, ws, actor, batch pgtype.UUID) (TriageImportPreview, error) {
	out := TriageImportPreview{BatchID: uuidToString(batch), Rows: []TriageImportRow{}}
	var headers, mapping []byte
	err := q.QueryRow(ctx, `SELECT filename,headers,mapping FROM triage_import_batch WHERE id=$1 AND workspace_id=$2 AND actor_id=$3`, batch, ws, actor).Scan(&out.Filename, &headers, &mapping)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, triageErr(404, "import batch unavailable")
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(headers, &out.Headers); err != nil {
		return out, err
	}
	if err = json.Unmarshal(mapping, &out.Mapping); err != nil {
		return out, err
	}
	rows, err := q.Query(ctx, `SELECT row_number,values,warnings,errors,duplicate,duplicate_issue_id,similar_issue_ids,status,issue_id,error FROM triage_import_row WHERE batch_id=$1 AND workspace_id=$2 ORDER BY row_number`, batch, ws)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		row, e := scanTriageImportRow(rows)
		if e != nil {
			return out, e
		}
		out.Rows = append(out.Rows, row)
		switch {
		case len(row.Errors) > 0:
			out.Counts.Error++
		case len(row.Warnings) > 0:
			out.Counts.Warning++
		default:
			out.Counts.Valid++
		}
		if row.Duplicate {
			out.Counts.Duplicate++
		}
	}
	out.Limits.MaxRows = triagecsv.MaxRows
	out.Limits.MaxBytes = triagecsv.MaxBytes
	return out, rows.Err()
}
func scanTriageImportRow(scanner interface{ Scan(...any) error }) (TriageImportRow, error) {
	var out TriageImportRow
	var values, warnings, errs, similar []byte
	var duplicate, issue pgtype.UUID
	var failure pgtype.Text
	if err := scanner.Scan(&out.RowNumber, &values, &warnings, &errs, &out.Duplicate, &duplicate, &similar, &out.Status, &issue, &failure); err != nil {
		return out, err
	}
	for _, pair := range []struct {
		data []byte
		out  any
	}{{values, &out.Values}, {warnings, &out.Warnings}, {errs, &out.Errors}, {similar, &out.SimilarIssueIDs}} {
		if err := json.Unmarshal(pair.data, pair.out); err != nil {
			return out, err
		}
	}
	out.DuplicateIssueID = uuidToPtr(duplicate)
	out.IssueID = uuidToPtr(issue)
	out.Error = textToPtr(failure)
	return out, nil
}
func (h *Handler) triageImportRead(r *http.Request) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	ws, err := h.triageReadScope(r)
	if err != nil {
		return ws, pgtype.UUID{}, pgtype.UUID{}, err
	}
	actor, err := h.triageHumanActor(r, uuidToString(ws))
	if err != nil {
		return ws, actor, pgtype.UUID{}, err
	}
	batch, err := triageUUID(chi.URLParam(r, "id"), "batch_id")
	return ws, actor, batch, err
}
func (h *Handler) GetTriageImport(w http.ResponseWriter, r *http.Request) {
	ws, actor, batch, err := h.triageImportRead(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	out, err := loadTriageImport(r.Context(), h.DB, ws, actor, batch)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) CommitTriageImport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rows []triageImportSelection `json:"rows"`
	}
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	if len(in.Rows) == 0 || len(in.Rows) > triagecsv.MaxRows {
		writeError(w, 400, "select between 1 and 1000 rows")
		return
	}
	selected := map[int]bool{}
	for _, row := range in.Rows {
		if row.RowNumber < 1 || row.RowNumber > triagecsv.MaxRows || selected[row.RowNumber] {
			writeError(w, 400, "row numbers must be unique and between 1 and 1000")
			return
		}
		selected[row.RowNumber] = true
	}
	ws, actor, batch, err := h.triageImportRead(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	preview, err := loadTriageImport(r.Context(), h.DB, ws, actor, batch)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	for _, row := range in.Rows {
		if row.RowNumber > len(preview.Rows) {
			writeError(w, 400, "selected row does not exist")
			return
		}
	}
	sort.Slice(in.Rows, func(i, j int) bool { return in.Rows[i].RowNumber < in.Rows[j].RowNumber })
	out := TriageImportResult{BatchID: uuidToString(batch), Results: []TriageImportRow{}}
	for _, selection := range in.Rows {
		row, e := h.commitTriageImportRow(r, batch, selection)
		if e != nil {
			writeTriageError(w, e)
			return
		}
		out.Results = append(out.Results, row)
		switch row.Status {
		case "created":
			out.Created++
		case "skipped":
			out.Skipped++
		case "failed":
			out.Failed++
		}
	}
	h.broadcastTriage(ws, "", uuidToString(batch), false)
	h.deliverTriageNotifications(r.Context(), ws)
	writeJSON(w, 200, out)
}

func (h *Handler) commitTriageImportRow(r *http.Request, batch pgtype.UUID, selection triageImportSelection) (TriageImportRow, error) {
	ctx := r.Context()
	tx, ws, actor, settings, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		return TriageImportRow{}, err
	}
	defer tx.Rollback(ctx)
	var filename string
	err = tx.QueryRow(ctx, `SELECT filename FROM triage_import_batch WHERE id=$1 AND workspace_id=$2 AND actor_id=$3 FOR UPDATE`, batch, ws, actor).Scan(&filename)
	if errors.Is(err, pgx.ErrNoRows) {
		return TriageImportRow{}, triageErr(404, "import batch unavailable")
	}
	if err != nil {
		return TriageImportRow{}, err
	}
	row, err := scanTriageImportRow(tx.QueryRow(ctx, `SELECT row_number,values,warnings,errors,duplicate,duplicate_issue_id,similar_issue_ids,status,issue_id,error FROM triage_import_row WHERE batch_id=$1 AND workspace_id=$2 AND row_number=$3 FOR UPDATE`, batch, ws, selection.RowNumber))
	if err != nil {
		return row, err
	}
	// A consumed row is its own durable request key. It must survive deletion of
	// its issue and never call intake again, even after triage has been disabled.
	if row.Status == "created" {
		return row, nil
	}
	if !settings.Enabled {
		return row, triageErr(409, "enable triage before importing")
	}
	var normalized []byte
	if err = tx.QueryRow(ctx, `SELECT normalized FROM triage_import_row WHERE batch_id=$1 AND row_number=$2`, batch, selection.RowNumber).Scan(&normalized); err != nil {
		return row, err
	}
	var plan triageImportPlan
	if err = json.Unmarshal(normalized, &plan); err != nil {
		return row, err
	}
	external := row.Values["external_id"]
	if external != "" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, uuidToString(ws)+":triage-import:"+external); err != nil {
			return row, err
		}
		duplicate, e := triageImportExternal(ctx, tx, ws, external)
		if e != nil {
			return row, e
		}
		if duplicate.Valid {
			row.Duplicate = true
			row.DuplicateIssueID = uuidToPtr(duplicate)
		}
	}
	row.Error = nil
	switch {
	case len(row.Errors) > 0:
		row.Status = "failed"
		message := strings.Join(row.Errors, "; ")
		row.Error = &message
	case row.Duplicate && !selection.ImportDuplicate:
		row.Status = "skipped"
	default:
		// A failed intake may abort SQL or have already advanced issue counters.
		// Roll back its savepoint before durably recording this row's failure.
		nested, e := tx.Begin(ctx)
		if e != nil {
			return row, e
		}
		item, e := h.createTriageItemInTx(ctx, nested, r, ws, actor, settings, plan.Input, TriageSource{Source: "csv", SourceURL: row.Values["source_url"], ExternalID: external, Filename: filename, BatchID: batch, RowNumber: int32(row.RowNumber)})
		if e != nil {
			if rollbackErr := nested.Rollback(ctx); rollbackErr != nil {
				return row, rollbackErr
			}
			row.Status = "failed"
			message := "Could not create this row; retry the import"
			var typed *triageError
			if errors.As(e, &typed) {
				message = typed.Message
			} else if errors.Is(e, service.ErrIssueLabelNotFound) {
				message = "A selected label is no longer available in this workspace"
			}
			row.Error = &message
		} else {
			if e = nested.Commit(ctx); e != nil {
				return row, e
			}
			row.Status = "created"
			row.IssueID = &item.Issue.ID
		}
	}
	issue := pgtype.UUID{}
	if row.IssueID != nil {
		issue = parseUUID(*row.IssueID)
	}
	duplicate := pgtype.UUID{}
	if row.DuplicateIssueID != nil {
		duplicate = parseUUID(*row.DuplicateIssueID)
	}
	_, err = tx.Exec(ctx, `UPDATE triage_import_row SET status=$3,issue_id=$4,error=$5,duplicate=$6,duplicate_issue_id=$7,import_duplicate=$8,attempted_at=now() WHERE batch_id=$1 AND row_number=$2`, batch, row.RowNumber, row.Status, issue, row.Error, row.Duplicate, duplicate, selection.ImportDuplicate)
	if err == nil {
		err = h.summarizeTriageImport(ctx, tx, ws, actor, batch, filename, settings)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return row, err
}
func (h *Handler) summarizeTriageImport(ctx context.Context, tx pgx.Tx, ws, actor, batch pgtype.UUID, filename string, settings db.WorkspaceTriageSetting) error {
	var created, skipped, failed int
	err := tx.QueryRow(ctx, `UPDATE triage_import_batch SET committed_at=now(),created_count=x.created,skipped_count=x.skipped,failed_count=x.failed FROM (SELECT count(*) FILTER(WHERE status='created') AS created,count(*) FILTER(WHERE status='skipped') AS skipped,count(*) FILTER(WHERE status='failed') AS failed FROM triage_import_row WHERE batch_id=$1) x WHERE id=$1 RETURNING created_count,skipped_count,failed_count`, batch).Scan(&created, &skipped, &failed)
	if err != nil {
		return err
	}
	details := map[string]string{"filename": filename, "created": strconv.Itoa(created), "skipped": strconv.Itoa(skipped), "failed": strconv.Itoa(failed)}
	recipients := []pgtype.UUID{actor}
	if settings.ResponsibilityMode != "none" && settings.ResponsibilityMemberID.Valid && settings.ResponsibilityMemberID != actor {
		recipients = append(recipients, settings.ResponsibilityMemberID)
	}
	for _, recipient := range recipients {
		if err = h.queueTriageNotification(ctx, tx, ws, recipient, pgtype.UUID{}, batch, "import:"+uuidToString(batch), "CSV import: "+filename, details, time.Now()); err != nil {
			return err
		}
	}
	return nil
}
func (h *Handler) DownloadTriageImportFailures(w http.ResponseWriter, r *http.Request) {
	ws, actor, batch, err := h.triageImportRead(r)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	preview, err := loadTriageImport(r.Context(), h.DB, ws, actor, batch)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT row_number,normalized,error,errors FROM triage_import_row WHERE batch_id=$1 AND workspace_id=$2 AND (status='failed' OR jsonb_array_length(errors)>0) ORDER BY row_number`, batch, ws)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	defer rows.Close()
	failures := []triagecsv.Failure{}
	for rows.Next() {
		var number int
		var data, errorsJSON []byte
		var failure pgtype.Text
		if err = rows.Scan(&number, &data, &failure, &errorsJSON); err != nil {
			writeTriageError(w, err)
			return
		}
		var plan triageImportPlan
		var messages []string
		if err = json.Unmarshal(data, &plan); err != nil {
			writeTriageError(w, err)
			return
		}
		if err = json.Unmarshal(errorsJSON, &messages); err != nil {
			writeTriageError(w, err)
			return
		}
		message := failure.String
		if message == "" {
			message = strings.Join(messages, "; ")
		}
		cells := make([]string, len(plan.Cells))
		for i, cell := range plan.Cells {
			cells[i] = string(cell)
		}
		failures = append(failures, triagecsv.Failure{Number: number, Cells: cells, Error: message})
	}
	if err = rows.Err(); err != nil {
		writeTriageError(w, err)
		return
	}
	var output bytes.Buffer
	if err = triagecsv.WriteFailures(&output, preview.Headers, failures); err != nil {
		writeTriageError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "failed-" + preview.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(output.Bytes())
}

func triageImportExternal(ctx context.Context, q triageImportDB, ws pgtype.UUID, external string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if external == "" {
		return id, nil
	}
	err := q.QueryRow(ctx, `SELECT issue_id FROM triage_import_row WHERE workspace_id=$1 AND external_id=$2 AND status='created' AND issue_id IS NOT NULL ORDER BY attempted_at,row_number LIMIT 1`, ws, external).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return id, err
}
func triageImportSimilar(ctx context.Context, q triageImportDB, ws pgtype.UUID, title string) ([]string, error) {
	out := []string{}
	if title == "" {
		return out, nil
	}
	// Exact normalized matches and substring matches are candidates, never gates.
	rows, err := q.Query(ctx, `SELECT id FROM issue WHERE workspace_id=$1 AND (lower(trim(title))=lower(trim($2)) OR (length($2)>=5 AND (strpos(lower(title),lower($2))>0 OR strpos(lower($2),lower(title))>0))) ORDER BY created_at DESC LIMIT 5`, ws, title)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, uuidToString(id))
	}
	return out, rows.Err()
}

type triageImportResolver struct {
	h         *Handler
	tx        pgx.Tx
	ws, actor pgtype.UUID
	cache     map[string][]string
}

func (r *triageImportResolver) candidates(ctx context.Context, kind, name string) ([]string, error) {
	key := kind + ":" + strings.ToLower(name)
	if cached, ok := r.cache[key]; ok {
		return cached, nil
	}
	var query string
	switch kind {
	case "project":
		query = `SELECT id FROM project WHERE workspace_id=$1 AND lower(title)=lower($2)`
	case "labels":
		query = `SELECT id FROM issue_label WHERE workspace_id=$1 AND resource_type='issue' AND lower(name)=lower($2)`
	case "member":
		query = `SELECT u.id FROM "user" u JOIN member m ON m.user_id=u.id WHERE m.workspace_id=$1 AND lower(u.email)=lower($2)`
	case "agent":
		query = `SELECT id FROM agent WHERE workspace_id=$1 AND lower(name)=lower($2) AND archived_at IS NULL AND kind='user'`
	default:
		return nil, fmt.Errorf("unsupported candidate kind")
	}
	rows, err := r.tx.Query(ctx, query, r.ws, name)
	if err != nil {
		return nil, err
	}
	ids := []pgtype.UUID{}
	for rows.Next() {
		var id pgtype.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, id := range ids {
		if kind == "agent" {
			agent, e := r.h.Queries.WithTx(r.tx).GetAgent(ctx, id)
			if e != nil {
				return nil, e
			}
			scoped := *r.h
			scoped.Queries = r.h.Queries.WithTx(r.tx)
			scoped.DB = r.tx
			if !scoped.canAccessPrivateAgent(ctx, agent, "member", uuidToString(r.actor), uuidToString(r.ws)) {
				continue
			}
		}
		out = append(out, uuidToString(id))
	}
	r.cache[key] = out
	return out, nil
}
func (r *triageImportResolver) resolve(ctx context.Context, row triagecsv.Row) (triageImportPlan, []string, error) {
	values := row.Values
	cells := make([][]byte, len(row.Cells))
	for i, cell := range row.Cells {
		cells[i] = []byte(cell)
	}
	plan := triageImportPlan{Cells: cells, Input: TriageIntakeInput{RequestID: uuidToString(dbid.NewV7()), Title: values["title"], Description: values["description"], Priority: values["priority"], SourceURL: values["source_url"], LabelIDs: []string{}}}
	warnings := []string{}
	for _, pair := range []struct {
		key  string
		dest **string
	}{{"start_date", &plan.Input.StartDate}, {"due_date", &plan.Input.DueDate}} {
		if v := values[pair.key]; v != "" {
			*pair.dest = &v
		}
	}
	if name := values["project"]; name != "" {
		ids, e := r.candidates(ctx, "project", name)
		if e != nil {
			return plan, warnings, e
		}
		if len(ids) == 1 {
			plan.Input.CandidateProjectID = &ids[0]
		} else {
			warnings = append(warnings, "Project was not uniquely available; candidate project left empty")
		}
	}
	if name := values["assignee"]; name != "" {
		kind := "agent"
		if strings.Contains(name, "@") {
			kind = "member"
		}
		ids, e := r.candidates(ctx, kind, name)
		if e != nil {
			return plan, warnings, e
		}
		if len(ids) == 1 {
			plan.Input.CandidateAssigneeType = &kind
			plan.Input.CandidateAssigneeID = &ids[0]
		} else {
			warnings = append(warnings, "Assignee was not uniquely available; candidate assignee left empty")
		}
	}
	seen := map[string]bool{}
	for _, label := range strings.FieldsFunc(values["labels"], func(ch rune) bool { return ch == ',' || ch == ';' || ch == '|' }) {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		ids, e := r.candidates(ctx, "labels", label)
		if e != nil {
			return plan, warnings, e
		}
		if len(ids) == 1 {
			if !seen[ids[0]] {
				plan.Input.LabelIDs = append(plan.Input.LabelIDs, ids[0])
				seen[ids[0]] = true
			}
		} else {
			warnings = append(warnings, "Label "+label+" was not uniquely available; label omitted")
		}
	}
	return plan, warnings, nil
}
