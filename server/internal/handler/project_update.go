package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/projecthealth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ProjectHistoricalMember struct {
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	AvatarURL    *string `json:"avatar_url"`
	Availability string  `json:"availability"`
}
type ProjectEvidenceInput struct {
	Kind string  `json:"kind"`
	ID   *string `json:"id"`
	URL  *string `json:"url"`
}
type ProjectEvidenceVersion struct {
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Revision     int64  `json:"revision,omitempty"`
	StateVersion int64  `json:"state_version,omitempty"`
	ResultDigest string `json:"result_digest,omitempty"`
	URL          string `json:"url,omitempty"`
	Verification string `json:"verification,omitempty"`
}
type ProjectEvidenceView struct {
	Input           ProjectEvidenceInput    `json:"input"`
	ObservedVersion ProjectEvidenceVersion  `json:"observed_version"`
	CollectedAt     time.Time               `json:"collected_at"`
	Availability    string                  `json:"availability"`
	CurrentVersion  *ProjectEvidenceVersion `json:"current_version"`
	Label           *string                 `json:"label"`
	Href            *string                 `json:"href"`
}
type ProjectAcceptanceInput struct {
	Conclusion  string  `json:"conclusion"`
	Scope       string  `json:"scope"`
	Explanation *string `json:"explanation"`
}
type ProjectAcceptance struct {
	Conclusion          string  `json:"conclusion"`
	Scope               string  `json:"scope"`
	Explanation         *string `json:"explanation"`
	DescriptionRevision int64   `json:"description_revision"`
	DescriptionSnapshot string  `json:"description_snapshot"`
}
type ProjectUpdateDraft struct {
	Operation                   string                  `json:"operation"`
	UpdateID                    *string                 `json:"update_id"`
	ExpectedRevision            *int64                  `json:"expected_revision"`
	Kind                        string                  `json:"kind"`
	Body                        string                  `json:"body"`
	HealthJudgment              *string                 `json:"health_judgment"`
	Evidence                    []ProjectEvidenceInput  `json:"evidence"`
	Acceptance                  *ProjectAcceptanceInput `json:"acceptance"`
	ExpectedDescriptionRevision *int64                  `json:"expected_description_revision"`
	IncludeStatistics           bool                    `json:"include_statistics"`
	CorrectionReason            *string                 `json:"correction_reason"`
}
type ProjectUpdatePreview struct {
	WorkspaceID         string                            `json:"workspace_id"`
	ProjectID           string                            `json:"project_id"`
	Draft               ProjectUpdateDraft                `json:"draft"`
	EvidenceVersions    []ProjectEvidenceVersion          `json:"evidence_versions"`
	DescriptionRevision *int64                            `json:"description_revision"`
	Recipients          []ProjectHistoricalMember         `json:"recipients"`
	StatisticsSnapshot  *projecthealth.StatisticsSnapshot `json:"statistics_snapshot"`
	PreviewHash         string                            `json:"preview_hash"`
	PreviewedAt         time.Time                         `json:"previewed_at"`
}
type ProjectUpdateWriteInput struct {
	RequestID        string                   `json:"request_id"`
	Draft            ProjectUpdateDraft       `json:"draft"`
	PreviewHash      string                   `json:"preview_hash"`
	EvidenceVersions []ProjectEvidenceVersion `json:"evidence_versions"`
}
type ProjectUpdateRevision struct {
	WorkspaceID        string                            `json:"workspace_id"`
	ProjectID          string                            `json:"project_id"`
	UpdateID           string                            `json:"update_id"`
	Revision           int64                             `json:"revision"`
	Editor             ProjectHistoricalMember           `json:"editor"`
	CreatedAt          time.Time                         `json:"created_at"`
	Kind               string                            `json:"kind"`
	Body               string                            `json:"body"`
	HealthJudgment     *string                           `json:"health_judgment"`
	CorrectionReason   *string                           `json:"correction_reason"`
	Evidence           []ProjectEvidenceView             `json:"evidence"`
	StatisticsSnapshot *projecthealth.StatisticsSnapshot `json:"statistics_snapshot"`
	Acceptance         *ProjectAcceptance                `json:"acceptance"`
}
type ProjectUpdate struct {
	WorkspaceID     string                  `json:"workspace_id"`
	ProjectID       string                  `json:"project_id"`
	ID              string                  `json:"id"`
	Author          ProjectHistoricalMember `json:"author"`
	PublishedAt     time.Time               `json:"published_at"`
	CurrentRevision int64                   `json:"current_revision"`
	Current         ProjectUpdateRevision   `json:"current"`
}
type ProjectUpdateWriteResult struct {
	WorkspaceID    string                  `json:"workspace_id"`
	ProjectID      string                  `json:"project_id"`
	RequestID      string                  `json:"request_id"`
	UpdateID       string                  `json:"update_id"`
	ResultRevision int64                   `json:"result_revision"`
	Replayed       bool                    `json:"replayed"`
	Result         ProjectUpdateRevision   `json:"result"`
	Author         ProjectHistoricalMember `json:"author"`
	PublishedAt    time.Time               `json:"published_at"`
}

func projectUpdateValidation(field, message string) error {
	return &projectAPIError{Status: 422, Code: "validation_failed", Message: message, FieldErrors: []map[string]string{{"field": field, "message": message}}}
}
func normalizeProjectText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
func normalizeProjectOptional(s *string) *string {
	if s == nil {
		return nil
	}
	v := normalizeProjectText(*s)
	return &v
}
func normalizeProjectUpdateDraft(d ProjectUpdateDraft) (ProjectUpdateDraft, error) {
	d.Body = normalizeProjectText(d.Body)
	d.CorrectionReason = normalizeProjectOptional(d.CorrectionReason)
	if n := utf8.RuneCountInString(d.Body); n < 1 || n > 10000 {
		return d, projectUpdateValidation("body", "body must contain 1 to 10000 characters")
	}
	if d.Kind != "progress" && d.Kind != "risk" && d.Kind != "acceptance" {
		return d, projectUpdateValidation("kind", "invalid update kind")
	}
	if d.HealthJudgment != nil && *d.HealthJudgment != "on_track" && *d.HealthJudgment != "attention" && *d.HealthJudgment != "risk" {
		return d, projectUpdateValidation("health_judgment", "invalid health judgment")
	}
	if d.Operation != "create" && d.Operation != "correct" {
		return d, projectUpdateValidation("operation", "invalid operation")
	}
	if d.Operation == "create" {
		if d.UpdateID != nil || d.ExpectedRevision != nil || d.CorrectionReason != nil {
			return d, projectUpdateValidation("operation", "creation cannot specify correction fields")
		}
	} else {
		if d.UpdateID == nil || d.ExpectedRevision == nil || *d.ExpectedRevision < 1 || *d.ExpectedRevision > 9007199254740991 {
			return d, projectUpdateValidation("expected_revision", "correction requires update id and positive revision")
		}
		id, e := util.ParseUUID(*d.UpdateID)
		if e != nil {
			return d, projectErr(400, "invalid_request", "invalid update id")
		}
		s := uuidToString(id)
		d.UpdateID = &s
		if d.CorrectionReason == nil || utf8.RuneCountInString(*d.CorrectionReason) < 1 || utf8.RuneCountInString(*d.CorrectionReason) > 1000 {
			return d, projectUpdateValidation("correction_reason", "correction reason must contain 1 to 1000 characters")
		}
	}
	if len(d.Evidence) > 50 {
		return d, projectUpdateValidation("evidence", "at most 50 evidence references are allowed")
	}
	ev := make([]ProjectEvidenceInput, 0, len(d.Evidence))
	seen := map[string]bool{}
	for _, v := range d.Evidence {
		key := v.Kind + ":"
		switch v.Kind {
		case "issue", "execution":
			if v.ID == nil || v.URL != nil {
				return d, projectUpdateValidation("evidence", "internal evidence requires only an id")
			}
			id, e := util.ParseUUID(*v.ID)
			if e != nil {
				return d, projectErr(400, "invalid_request", "invalid evidence id")
			}
			s := uuidToString(id)
			v.ID = &s
			key += s
		case "url":
			if v.URL == nil || v.ID != nil {
				return d, projectUpdateValidation("evidence", "external evidence requires only a URL")
			}
			u, e := url.Parse(*v.URL)
			if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return d, projectUpdateValidation("evidence", "evidence URL must be HTTP or HTTPS without credentials")
			}
			s := u.String()
			v.URL = &s
			key += s
		default:
			return d, projectUpdateValidation("evidence", "invalid evidence kind")
		}
		if !seen[key] {
			ev = append(ev, v)
			seen[key] = true
		}
	}
	d.Evidence = ev
	if d.Kind == "acceptance" {
		if d.Acceptance == nil {
			return d, projectUpdateValidation("acceptance", "acceptance details are required")
		}
		a := *d.Acceptance
		a.Scope = normalizeProjectText(a.Scope)
		a.Explanation = normalizeProjectOptional(a.Explanation)
		d.Acceptance = &a
		if a.Conclusion != "passed" && a.Conclusion != "partial" && a.Conclusion != "failed" {
			return d, projectUpdateValidation("acceptance.conclusion", "invalid acceptance conclusion")
		}
		if a.Scope == "" {
			return d, projectUpdateValidation("acceptance.scope", "acceptance scope is required")
		}
		if a.Conclusion != "failed" && len(d.Evidence) == 0 && (a.Explanation == nil || *a.Explanation == "") {
			return d, projectUpdateValidation("acceptance.explanation", "passed or partial acceptance requires evidence or an explanation")
		}
		if d.ExpectedDescriptionRevision == nil || *d.ExpectedDescriptionRevision < 1 || *d.ExpectedDescriptionRevision > 9007199254740991 {
			return d, projectErr(428, "project_description_revision_required", "acceptance requires its description revision")
		}
	} else if d.Acceptance != nil || d.ExpectedDescriptionRevision != nil {
		return d, projectUpdateValidation("acceptance", "only acceptance updates can specify acceptance details")
	}
	return d, nil
}
func projectDigest(value any) (string, error) {
	raw, e := projecthealth.CanonicalJSON(value)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func decodeProjectUpdateRequest(r *http.Request, out any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if e := dec.Decode(out); e != nil {
		return projectErr(400, "invalid_request", "invalid request body")
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return projectErr(400, "invalid_request", "request must contain one JSON object")
	}
	return nil
}
func (h *Handler) projectUpdateScope(r *http.Request, human bool) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	wsText := ctxWorkspaceID(r.Context())
	if wsText == "" {
		wsText = r.Header.Get("X-Workspace-ID")
	}
	ws, e := util.ParseUUID(wsText)
	if e != nil {
		return ws, pgtype.UUID{}, pgtype.UUID{}, projectErr(400, "invalid_request", "invalid workspace id")
	}
	id, e := util.ParseUUID(chi.URLParam(r, "id"))
	if e != nil {
		return ws, id, pgtype.UUID{}, projectErr(400, "invalid_request", "invalid project id")
	}
	var actor pgtype.UUID
	if human {
		actor, e = h.projectHumanActor(r, uuidToString(ws))
	} else {
		actor, e = util.ParseUUID(requestUserID(r))
		if e != nil {
			e = projectErr(401, "unauthenticated", "user not authenticated")
		}
	}
	return ws, id, actor, e
}
func projectUpdateProject(ctx context.Context, q *db.Queries, ws, id pgtype.UUID, write bool) (db.Project, error) {
	var p db.Project
	var e error
	if write {
		p, e = q.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: id, WorkspaceID: ws})
	} else {
		p, e = q.LockProjectForAssociation(ctx, db.LockProjectForAssociationParams{ID: id, WorkspaceID: ws})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		e = projectErr(404, "project_not_found", "project not found")
	}
	return p, e
}
func projectUpdateMembers(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids []pgtype.UUID) (map[pgtype.UUID]ProjectHistoricalMember, error) {
	out := make(map[pgtype.UUID]ProjectHistoricalMember, len(ids))
	for _, id := range ids {
		out[id] = ProjectHistoricalMember{ID: uuidToString(id), Availability: "deleted"}
	}
	users, e := q.GetUsersByIDs(ctx, ids)
	if e != nil {
		return nil, e
	}
	members, e := q.ListProjectUpdateMemberIDs(ctx, db.ListProjectUpdateMemberIDsParams{WorkspaceID: ws, UserIds: ids})
	if e != nil {
		return nil, e
	}
	active := map[pgtype.UUID]bool{}
	for _, id := range members {
		active[id] = true
	}
	for _, u := range users {
		availability := "departed"
		if active[u.ID] {
			availability = "active"
		}
		name := u.Name
		out[u.ID] = ProjectHistoricalMember{ID: uuidToString(u.ID), Name: &name, AvatarURL: textToPtr(u.AvatarUrl), Availability: availability}
	}
	return out, nil
}

// Source reads use this request's transaction; NOWAIT makes changed/deleted
// source authorization restart the complete RR operation instead of passing on
// a snapshot from before a concurrent revocation.
func projectUpdateEvidence(ctx context.Context, q *db.Queries, ws, actor pgtype.UUID, input ProjectEvidenceInput, now time.Time) (ProjectEvidenceView, error) {
	v := ProjectEvidenceView{Input: input, CollectedAt: now, Availability: "available"}
	switch input.Kind {
	case "url":
		v.ObservedVersion = ProjectEvidenceVersion{Kind: "url", URL: *input.URL, Verification: "unverified"}
		v.Availability = "unverified"
		v.Href = input.URL
		v.Label = input.URL
	case "issue":
		issue, e := q.LockProjectUpdateEvidenceIssue(ctx, db.LockProjectUpdateEvidenceIssueParams{ID: parseUUID(*input.ID), WorkspaceID: ws})
		if e != nil {
			return v, e
		}
		v.ObservedVersion = ProjectEvidenceVersion{Kind: "issue", ID: *input.ID, Revision: issue.Revision}
		v.Label = &issue.Title
	case "execution":
		task, e := q.LockProjectUpdateEvidenceExecution(ctx, db.LockProjectUpdateEvidenceExecutionParams{ID: parseUUID(*input.ID), WorkspaceID: ws})
		if e != nil {
			return v, e
		}
		if task.ChatSessionID.Valid {
			chat, e := q.LockProjectUpdateEvidenceChat(ctx, db.LockProjectUpdateEvidenceChatParams{ID: task.ChatSessionID, WorkspaceID: ws})
			if e != nil {
				return v, e
			}
			if chat.CreatorID != actor {
				return v, projectErr(403, "forbidden", "execution evidence is not accessible")
			}
		} else {
			agent, e := q.LockProjectUpdateEvidenceAgent(ctx, db.LockProjectUpdateEvidenceAgentParams{ID: task.AgentID, WorkspaceID: ws})
			if e != nil {
				return v, e
			}
			member, e := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor, WorkspaceID: ws})
			if e != nil {
				return v, e
			}
			targets, e := q.LockProjectUpdateEvidenceTargets(ctx, task.AgentID)
			if e != nil {
				return v, e
			}
			if !memberAllowedToViewAgent(agent, targets, uuidToString(actor), member.Role) {
				return v, projectErr(403, "forbidden", "execution evidence is not accessible")
			}
		}
		var result any
		if len(task.Result) > 0 {
			dec := json.NewDecoder(bytes.NewReader(task.Result))
			dec.UseNumber()
			if e = dec.Decode(&result); e != nil {
				return v, e
			}
		}
		var completed *time.Time
		if task.CompletedAt.Valid {
			t := task.CompletedAt.Time.UTC()
			completed = &t
		}
		digest, e := projectDigest(map[string]any{"status": task.Status, "result": result, "error": textToPtr(task.Error), "completed_at": completed})
		if e != nil {
			return v, e
		}
		v.ObservedVersion = ProjectEvidenceVersion{Kind: "execution", ID: *input.ID, StateVersion: task.StateVersion, ResultDigest: digest}
		label := "Execution " + *input.ID
		v.Label = &label
	default:
		return v, projectUpdateValidation("evidence", "invalid evidence kind")
	}
	version := v.ObservedVersion
	v.CurrentVersion = &version
	return v, nil
}

type projectPreparedUpdate struct {
	Preview    ProjectUpdatePreview
	Evidence   []ProjectEvidenceView
	Acceptance *ProjectAcceptance
	Previous   *db.ProjectUpdateRevision
}

func (h *Handler) prepareProjectUpdate(ctx context.Context, q *db.Queries, p db.Project, actor pgtype.UUID, d ProjectUpdateDraft) (projectPreparedUpdate, error) {
	now := time.Now().UTC()
	out := projectPreparedUpdate{Preview: ProjectUpdatePreview{WorkspaceID: uuidToString(p.WorkspaceID), ProjectID: uuidToString(p.ID), Draft: d, EvidenceVersions: []ProjectEvidenceVersion{}, Recipients: []ProjectHistoricalMember{}, PreviewedAt: now}, Evidence: []ProjectEvidenceView{}}
	if d.Operation == "correct" {
		u, e := q.GetProjectUpdate(ctx, db.GetProjectUpdateParams{ID: parseUUID(*d.UpdateID), WorkspaceID: p.WorkspaceID, ProjectID: p.ID})
		if errors.Is(e, pgx.ErrNoRows) {
			return out, projectErr(404, "project_update_not_found", "project update not found")
		}
		if e != nil {
			return out, e
		}
		if u.CurrentRevision != *d.ExpectedRevision {
			return out, &projectAPIError{Status: 409, Code: "project_update_revision_conflict", Message: "project update has changed", Current: map[string]any{"revision": u.CurrentRevision}}
		}
		old, e := q.GetProjectUpdateRevision(ctx, db.GetProjectUpdateRevisionParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID, UpdateID: u.ID, Revision: u.CurrentRevision})
		if e != nil {
			return out, e
		}
		out.Previous = &old
		if old.Kind != d.Kind {
			return out, projectUpdateValidation("kind", "corrections cannot change the update kind")
		}
		if len(old.Acceptance) > 0 {
			if e = json.Unmarshal(old.Acceptance, &out.Acceptance); e != nil {
				return out, e
			}
			if out.Acceptance == nil || out.Acceptance.DescriptionRevision != *d.ExpectedDescriptionRevision {
				return out, projectUpdateValidation("expected_description_revision", "corrections must keep the original description revision")
			}
		}
		if !d.IncludeStatistics && len(old.StatisticsSnapshot) > 0 {
			if e = json.Unmarshal(old.StatisticsSnapshot, &out.Preview.StatisticsSnapshot); e != nil {
				return out, e
			}
		}
	}
	if d.Acceptance != nil {
		if out.Acceptance == nil {
			if *d.ExpectedDescriptionRevision != p.DescriptionRevision {
				return out, &projectAPIError{Status: 409, Code: "project_description_conflict", Message: "project description has changed", Current: map[string]any{"description_revision": p.DescriptionRevision, "description": textToPtr(p.Description)}}
			}
			out.Acceptance = &ProjectAcceptance{DescriptionRevision: p.DescriptionRevision, DescriptionSnapshot: p.Description.String}
		}
		out.Acceptance.Conclusion = d.Acceptance.Conclusion
		out.Acceptance.Scope = d.Acceptance.Scope
		out.Acceptance.Explanation = d.Acceptance.Explanation
		r := out.Acceptance.DescriptionRevision
		out.Preview.DescriptionRevision = &r
	}
	// Acquire source locks in a deterministic order, but preserve the user's
	// reference order in the canonical intent and stored evidence.
	order := make([]int, len(d.Evidence))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := d.Evidence[order[i]], d.Evidence[order[j]]
		ak, bk := a.Kind, b.Kind
		if a.ID != nil {
			ak += *a.ID
		}
		if b.ID != nil {
			bk += *b.ID
		}
		return ak < bk
	})
	out.Evidence = make([]ProjectEvidenceView, len(d.Evidence))
	out.Preview.EvidenceVersions = make([]ProjectEvidenceVersion, len(d.Evidence))
	for _, i := range order {
		v, e := projectUpdateEvidence(ctx, q, p.WorkspaceID, actor, d.Evidence[i], now)
		if errors.Is(e, pgx.ErrNoRows) {
			return out, projectUpdateValidation("evidence", "evidence is missing or inaccessible")
		}
		if e != nil {
			return out, e
		}
		out.Evidence[i] = v
		out.Preview.EvidenceVersions[i] = v.ObservedVersion
	}
	ids := []pgtype.UUID{}
	seen := map[pgtype.UUID]bool{}
	for _, mention := range util.ParseMentions(d.Body) {
		if mention.Type != "member" {
			continue
		}
		id, e := util.ParseUUID(mention.ID)
		if e == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	active, e := q.LockProjectUpdateRecipients(ctx, db.LockProjectUpdateRecipientsParams{WorkspaceID: p.WorkspaceID, UserIds: ids})
	if e != nil {
		return out, e
	}
	members, e := projectUpdateMembers(ctx, q, p.WorkspaceID, active)
	if e != nil {
		return out, e
	}
	recipients := make([]string, 0, len(active))
	for _, id := range active {
		out.Preview.Recipients = append(out.Preview.Recipients, members[id])
		recipients = append(recipients, uuidToString(id))
	}
	sort.Strings(recipients)
	var statisticsVersion *string
	if d.IncludeStatistics {
		collection, err := projecthealth.Collect(ctx, q, p, now)
		if err != nil {
			if retryableProjectTransaction(err) {
				return out, err
			}
			return out, projectErr(503, "project_health_unavailable", "project statistics are unavailable")
		}
		if !collection.Statistics.Complete {
			return out, projectErr(503, "project_health_unavailable", "project statistics are incomplete")
		}
		out.Preview.StatisticsSnapshot = &collection.Statistics
		statisticsVersion = &collection.Statistics.SnapshotVersion
	}
	out.Preview.PreviewHash, e = projectDigest(map[string]any{"contract_version": 1, "workspace_id": out.Preview.WorkspaceID, "project_id": out.Preview.ProjectID, "actor_user_id": uuidToString(actor), "draft": d, "evidence_versions": out.Preview.EvidenceVersions, "acceptance_description_revision": out.Preview.DescriptionRevision, "recipient_ids_sorted": recipients, "statistics_version": statisticsVersion})
	return out, e
}

func (h *Handler) PreviewProjectUpdate(w http.ResponseWriter, r *http.Request) {
	ws, pid, actor, e := h.projectUpdateScope(r, true)
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	var d ProjectUpdateDraft
	if e = decodeProjectUpdateRequest(r, &d); e == nil {
		d, e = normalizeProjectUpdateDraft(d)
	}
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	var out projectPreparedUpdate
	e = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		p, e := projectUpdateProject(r.Context(), q, ws, pid, false)
		if e != nil {
			return e
		}
		out, e = h.prepareProjectUpdate(r.Context(), q, p, actor, d)
		return e
	})
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	writeJSON(w, 200, out.Preview)
}
func (h *Handler) CreateProjectUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeProjectUpdate(w, r, "create")
}
func (h *Handler) CorrectProjectUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeProjectUpdate(w, r, "correct")
}
func (h *Handler) writeProjectUpdate(w http.ResponseWriter, r *http.Request, operation string) {
	ws, pid, actor, e := h.projectUpdateScope(r, true)
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	var in ProjectUpdateWriteInput
	if e = decodeProjectUpdateRequest(r, &in); e == nil {
		in.Draft, e = normalizeProjectUpdateDraft(in.Draft)
	}
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	request, e := util.ParseUUID(in.RequestID)
	if e != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid request id"))
		return
	}
	in.RequestID = uuidToString(request)
	if in.Draft.Operation != operation {
		writeProjectAPIError(w, projectUpdateValidation("operation", "operation does not match endpoint"))
		return
	}
	if operation == "correct" {
		id, err := util.ParseUUID(chi.URLParam(r, "updateId"))
		if err != nil || uuidToString(id) != *in.Draft.UpdateID {
			writeProjectAPIError(w, projectErr(400, "invalid_request", "update id does not match endpoint"))
			return
		}
	}
	payloadHash, e := projectDigest(map[string]any{"contract_version": 1, "workspace_id": uuidToString(ws), "project_id": uuidToString(pid), "actor_user_id": uuidToString(actor), "operation": operation, "update_id": in.Draft.UpdateID, "draft": in.Draft, "preview_hash": in.PreviewHash, "evidence_versions": in.EvidenceVersions})
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	var out ProjectUpdateWriteResult
	e = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		p, e := projectUpdateProject(r.Context(), q, ws, pid, true)
		if e != nil {
			return e
		}
		prior, e := q.GetProjectUpdateRequest(r.Context(), db.GetProjectUpdateRequestParams{WorkspaceID: ws, ActorUserID: actor, RequestID: request})
		if e == nil {
			if prior.PayloadHash != payloadHash || prior.ProjectID != pid {
				return projectErr(409, "idempotency_conflict", "request id already used for another intent")
			}
			out, e = projectUpdateResult(r.Context(), q, prior, actor, true)
			return e
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if !h.FeatureFlags.IsEnabled(r.Context(), featureflags.ProjectsP1, true) {
			return projectErr(403, "forbidden", "project updates are currently read-only")
		}
		prepared, e := h.prepareProjectUpdate(r.Context(), q, p, actor, in.Draft)
		if e != nil {
			return e
		}
		versions, e := projecthealth.CanonicalJSON(in.EvidenceVersions)
		if e != nil {
			return e
		}
		actual, e := projecthealth.CanonicalJSON(prepared.Preview.EvidenceVersions)
		if e != nil {
			return e
		}
		if in.PreviewHash != prepared.Preview.PreviewHash || !bytes.Equal(versions, actual) {
			return &projectAPIError{Status: 409, Code: "project_update_preview_stale", Message: "project update preview changed; review the current preview", Current: map[string]any{"changed_fields": []string{"preview"}, "preview": prepared.Preview}}
		}
		var update db.ProjectUpdate
		if operation == "create" {
			update, e = q.CreateProjectUpdate(r.Context(), db.CreateProjectUpdateParams{ID: dbid.NewV7(), WorkspaceID: ws, ProjectID: pid, AuthorUserID: actor})
		} else {
			update, e = q.AdvanceProjectUpdateRevision(r.Context(), db.AdvanceProjectUpdateRevisionParams{ID: parseUUID(*in.Draft.UpdateID), WorkspaceID: ws, ProjectID: pid, ExpectedRevision: *in.Draft.ExpectedRevision})
		}
		if e != nil {
			return e
		}
		evidence, e := json.Marshal(prepared.Evidence)
		if e != nil {
			return e
		}
		var acceptance, snapshot []byte
		if prepared.Acceptance != nil {
			acceptance, e = json.Marshal(prepared.Acceptance)
			if e != nil {
				return e
			}
		}
		if prepared.Preview.StatisticsSnapshot != nil {
			snapshot, e = json.Marshal(prepared.Preview.StatisticsSnapshot)
			if e != nil {
				return e
			}
		}
		_, e = q.CreateProjectUpdateRevision(r.Context(), db.CreateProjectUpdateRevisionParams{ID: dbid.NewV7(), WorkspaceID: ws, ProjectID: pid, UpdateID: update.ID, Revision: update.CurrentRevision, EditorUserID: actor, Kind: in.Draft.Kind, Body: in.Draft.Body, HealthJudgment: projectOptionalText(in.Draft.HealthJudgment), CorrectionReason: projectOptionalText(in.Draft.CorrectionReason), Evidence: evidence, Acceptance: acceptance, StatisticsSnapshot: snapshot})
		if e != nil {
			return e
		}
		e = q.CreateProjectUpdateRequest(r.Context(), db.CreateProjectUpdateRequestParams{WorkspaceID: ws, ProjectID: pid, ActorUserID: actor, RequestID: request, Operation: operation, PayloadHash: payloadHash, UpdateID: update.ID, ResultRevision: update.CurrentRevision})
		if e != nil {
			var pg *pgconn.PgError
			if errors.As(e, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, "request") {
				return &pgconn.PgError{Code: "40001", Message: "concurrent project update request"}
			}
			return e
		}
		for _, recipient := range prepared.Preview.Recipients {
			if e = q.CreateProjectUpdateNotification(r.Context(), db.CreateProjectUpdateNotificationParams{ID: dbid.NewV7(), WorkspaceID: ws, ProjectID: pid, UpdateID: update.ID, RecipientUserID: parseUUID(recipient.ID), SourceRevision: update.CurrentRevision}); e != nil {
				return e
			}
		}
		out, e = projectUpdateResult(r.Context(), q, db.ProjectUpdateRequest{WorkspaceID: ws, ProjectID: pid, RequestID: request, UpdateID: update.ID, ResultRevision: update.CurrentRevision}, actor, false)
		return e
	})
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	status := 200
	if !out.Replayed {
		if operation == "create" {
			status = 201
		}
		event := protocol.EventProjectUpdatePublished
		if operation == "correct" {
			event = protocol.EventProjectUpdateCorrected
		}
		h.publish(event, uuidToString(ws), "member", uuidToString(actor), map[string]any{"workspace_id": uuidToString(ws), "project_id": uuidToString(pid), "update_id": out.UpdateID, "revision": out.ResultRevision})
	}
	writeJSON(w, status, out)
}
func projectOptionalText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}
func projectUpdateResult(ctx context.Context, q *db.Queries, request db.ProjectUpdateRequest, actor pgtype.UUID, replayed bool) (ProjectUpdateWriteResult, error) {
	u, e := q.GetProjectUpdate(ctx, db.GetProjectUpdateParams{ID: request.UpdateID, WorkspaceID: request.WorkspaceID, ProjectID: request.ProjectID})
	if e != nil {
		return ProjectUpdateWriteResult{}, e
	}
	row, e := q.GetProjectUpdateRevision(ctx, db.GetProjectUpdateRevisionParams{WorkspaceID: request.WorkspaceID, ProjectID: request.ProjectID, UpdateID: request.UpdateID, Revision: request.ResultRevision})
	if e != nil {
		return ProjectUpdateWriteResult{}, e
	}
	members, e := projectUpdateMembers(ctx, q, request.WorkspaceID, []pgtype.UUID{u.AuthorUserID, row.EditorUserID})
	if e != nil {
		return ProjectUpdateWriteResult{}, e
	}
	revision, e := projectUpdateRevisionView(ctx, q, row, actor, members)
	if e != nil {
		return ProjectUpdateWriteResult{}, e
	}
	return ProjectUpdateWriteResult{WorkspaceID: uuidToString(request.WorkspaceID), ProjectID: uuidToString(request.ProjectID), RequestID: uuidToString(request.RequestID), UpdateID: uuidToString(u.ID), ResultRevision: request.ResultRevision, Replayed: replayed, Result: revision, Author: members[u.AuthorUserID], PublishedAt: u.PublishedAt.Time.UTC()}, nil
}
func projectUpdateRevisionView(ctx context.Context, q *db.Queries, row db.ProjectUpdateRevision, actor pgtype.UUID, members map[pgtype.UUID]ProjectHistoricalMember) (ProjectUpdateRevision, error) {
	out := ProjectUpdateRevision{WorkspaceID: uuidToString(row.WorkspaceID), ProjectID: uuidToString(row.ProjectID), UpdateID: uuidToString(row.UpdateID), Revision: row.Revision, Editor: members[row.EditorUserID], CreatedAt: row.CreatedAt.Time.UTC(), Kind: row.Kind, Body: row.Body, HealthJudgment: textToPtr(row.HealthJudgment), CorrectionReason: textToPtr(row.CorrectionReason), Evidence: []ProjectEvidenceView{}}
	if e := json.Unmarshal(row.Evidence, &out.Evidence); e != nil {
		return out, e
	}
	if len(row.Acceptance) > 0 {
		if e := json.Unmarshal(row.Acceptance, &out.Acceptance); e != nil {
			return out, e
		}
	}
	if len(row.StatisticsSnapshot) > 0 {
		if e := json.Unmarshal(row.StatisticsSnapshot, &out.StatisticsSnapshot); e != nil {
			return out, e
		}
	}
	for i, old := range out.Evidence {
		current, e := projectUpdateEvidence(ctx, q, row.WorkspaceID, actor, old.Input, time.Now().UTC())
		if e != nil {
			var api *projectAPIError
			if errors.Is(e, pgx.ErrNoRows) {
				old.Availability = "deleted"
			} else if errors.As(e, &api) && api.Status == 403 {
				old.Availability = "inaccessible"
			} else {
				return out, e
			}
			old.Label = nil
			old.Href = nil
			old.CurrentVersion = nil
			out.Evidence[i] = old
			continue
		}
		old.Label = current.Label
		old.Href = current.Href
		old.CurrentVersion = current.CurrentVersion
		old.Availability = current.Availability
		if old.Input.Kind != "url" && old.ObservedVersion != current.ObservedVersion {
			old.Availability = "changed"
		}
		out.Evidence[i] = old
	}
	return out, nil
}

type projectUpdateCursor struct {
	WorkspaceID string     `json:"workspace_id"`
	ProjectID   string     `json:"project_id"`
	UpdateID    string     `json:"update_id,omitempty"`
	Time        *time.Time `json:"time,omitempty"`
	ID          string     `json:"id,omitempty"`
	Revision    int64      `json:"revision,omitempty"`
}

func projectUpdatePage(r *http.Request, ws, pid pgtype.UUID, update string) (int32, projectUpdateCursor, error) {
	limit := int64(20)
	if s := r.URL.Query().Get("limit"); s != "" {
		v, e := strconv.ParseInt(s, 10, 32)
		if e != nil || v < 1 || v > 100 {
			return 0, projectUpdateCursor{}, projectErr(400, "invalid_request", "limit must be 1 to 100")
		}
		limit = v
	}
	c := projectUpdateCursor{}
	if s := r.URL.Query().Get("cursor"); s != "" {
		raw, e := base64.RawURLEncoding.DecodeString(s)
		if e != nil || json.Unmarshal(raw, &c) != nil || c.WorkspaceID != uuidToString(ws) || c.ProjectID != uuidToString(pid) || c.UpdateID != update {
			return 0, c, projectErr(400, "invalid_request", "invalid cursor")
		}
		if update != "" {
			if c.Revision < 1 {
				return 0, c, projectErr(400, "invalid_request", "invalid revision cursor")
			}
		} else {
			if _, e = util.ParseUUID(c.ID); e != nil || c.Time == nil {
				return 0, c, projectErr(400, "invalid_request", "invalid timeline cursor")
			}
		}
	}
	return int32(limit), c, nil
}
func encodeProjectUpdateCursor(c projectUpdateCursor) *string {
	raw, _ := json.Marshal(c)
	s := base64.RawURLEncoding.EncodeToString(raw)
	return &s
}
func (h *Handler) ListProjectUpdates(w http.ResponseWriter, r *http.Request) {
	ws, pid, actor, e := h.projectUpdateScope(r, false)
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	limit, c, e := projectUpdatePage(r, ws, pid, "")
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	items := []ProjectUpdate{}
	var next *string
	e = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		if _, e := projectUpdateProject(r.Context(), q, ws, pid, false); e != nil {
			return e
		}
		args := db.ListProjectUpdatesParams{WorkspaceID: ws, ProjectID: pid, RowLimit: limit + 1}
		if c.Time != nil {
			args.BeforeTime = pgtype.Timestamptz{Time: *c.Time, Valid: true}
			args.BeforeID = parseUUID(c.ID)
		}
		rows, e := q.ListProjectUpdates(r.Context(), args)
		if e != nil {
			return e
		}
		next = nil
		if len(rows) > int(limit) {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			t := last.PublishedAt.Time.UTC()
			next = encodeProjectUpdateCursor(projectUpdateCursor{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(pid), Time: &t, ID: uuidToString(last.ID)})
		}
		revisions := make([]db.ProjectUpdateRevision, len(rows))
		ids := make([]pgtype.UUID, 0, len(rows)*2)
		for i, u := range rows {
			row, e := q.GetProjectUpdateRevision(r.Context(), db.GetProjectUpdateRevisionParams{WorkspaceID: ws, ProjectID: pid, UpdateID: u.ID, Revision: u.CurrentRevision})
			if e != nil {
				return e
			}
			revisions[i] = row
			ids = append(ids, u.AuthorUserID, row.EditorUserID)
		}
		members, e := projectUpdateMembers(r.Context(), q, ws, ids)
		if e != nil {
			return e
		}
		items = make([]ProjectUpdate, 0, len(rows))
		for i, u := range rows {
			revision, e := projectUpdateRevisionView(r.Context(), q, revisions[i], actor, members)
			if e != nil {
				return e
			}
			items = append(items, ProjectUpdate{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(pid), ID: uuidToString(u.ID), Author: members[u.AuthorUserID], PublishedAt: u.PublishedAt.Time.UTC(), CurrentRevision: u.CurrentRevision, Current: revision})
		}
		return nil
	})
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "project_id": uuidToString(pid), "items": items, "next_cursor": next})
}
func (h *Handler) ListProjectUpdateRevisions(w http.ResponseWriter, r *http.Request) {
	ws, pid, actor, e := h.projectUpdateScope(r, false)
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	id, e := util.ParseUUID(chi.URLParam(r, "updateId"))
	if e != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid update id"))
		return
	}
	limit, c, e := projectUpdatePage(r, ws, pid, uuidToString(id))
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	items := []ProjectUpdateRevision{}
	var next *string
	e = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		if _, e := projectUpdateProject(r.Context(), q, ws, pid, false); e != nil {
			return e
		}
		if _, e := q.GetProjectUpdate(r.Context(), db.GetProjectUpdateParams{ID: id, WorkspaceID: ws, ProjectID: pid}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_update_not_found", "project update not found")
			}
			return e
		}
		args := db.ListProjectUpdateRevisionsParams{WorkspaceID: ws, ProjectID: pid, UpdateID: id, RowLimit: limit + 1}
		if c.Revision > 0 {
			args.BeforeRevision = pgtype.Int8{Int64: c.Revision, Valid: true}
		}
		rows, e := q.ListProjectUpdateRevisions(r.Context(), args)
		if e != nil {
			return e
		}
		next = nil
		if len(rows) > int(limit) {
			rows = rows[:limit]
			next = encodeProjectUpdateCursor(projectUpdateCursor{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(pid), UpdateID: uuidToString(id), Revision: rows[len(rows)-1].Revision})
		}
		ids := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.EditorUserID)
		}
		members, e := projectUpdateMembers(r.Context(), q, ws, ids)
		if e != nil {
			return e
		}
		items = make([]ProjectUpdateRevision, 0, len(rows))
		for _, row := range rows {
			out, e := projectUpdateRevisionView(r.Context(), q, row, actor, members)
			if e != nil {
				return e
			}
			items = append(items, out)
		}
		return nil
	})
	if e != nil {
		writeProjectAPIError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "project_id": uuidToString(pid), "update_id": uuidToString(id), "items": items, "next_cursor": next})
}

// ProjectUpdateExecutionEvidence supplies a real execution to the existing
// transcript dialog. Internal evidence navigation is a typed identity, not a
// fabricated task page or an unscoped browser URL.
type ProjectUpdateExecutionEvidence struct {
	WorkspaceID string                        `json:"workspace_id"`
	ProjectID   string                        `json:"project_id"`
	UpdateID    string                        `json:"update_id"`
	Revision    int64                         `json:"revision"`
	Task        AgentTaskResponse             `json:"task"`
	Messages    []protocol.TaskMessagePayload `json:"messages"`
}

func (h *Handler) GetProjectUpdateExecutionEvidence(w http.ResponseWriter, r *http.Request) {
	ws, pid, actor, err := h.projectUpdateScope(r, false)
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	updateID, err := util.ParseUUID(chi.URLParam(r, "updateId"))
	if err != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid update id"))
		return
	}
	taskID, err := util.ParseUUID(chi.URLParam(r, "taskId"))
	if err != nil {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid execution id"))
		return
	}
	revision, err := strconv.ParseInt(chi.URLParam(r, "revision"), 10, 64)
	if err != nil || revision < 1 || revision > 9007199254740991 {
		writeProjectAPIError(w, projectErr(400, "invalid_request", "invalid update revision"))
		return
	}
	out := ProjectUpdateExecutionEvidence{WorkspaceID: uuidToString(ws), ProjectID: uuidToString(pid), UpdateID: uuidToString(updateID), Revision: revision, Messages: []protocol.TaskMessagePayload{}}
	err = h.runProjectTransaction(r.Context(), ws, actor, func(_ pgx.Tx, q *db.Queries) error {
		if _, e := projectUpdateProject(r.Context(), q, ws, pid, false); e != nil {
			return e
		}
		if _, e := q.GetProjectUpdate(r.Context(), db.GetProjectUpdateParams{ID: updateID, WorkspaceID: ws, ProjectID: pid}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_update_not_found", "project update not found")
			}
			return e
		}
		row, e := q.GetProjectUpdateRevision(r.Context(), db.GetProjectUpdateRevisionParams{WorkspaceID: ws, ProjectID: pid, UpdateID: updateID, Revision: revision})
		if e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_update_not_found", "project update revision not found")
			}
			return e
		}
		var evidence []ProjectEvidenceView
		if e = json.Unmarshal(row.Evidence, &evidence); e != nil {
			return e
		}
		var source *ProjectEvidenceInput
		for _, view := range evidence {
			if view.Input.Kind == "execution" && view.Input.ID != nil && *view.Input.ID == uuidToString(taskID) {
				input := view.Input
				source = &input
				break
			}
		}
		if source == nil {
			return projectErr(404, "project_update_not_found", "execution is not referenced by this project update revision")
		}
		if _, e = projectUpdateEvidence(r.Context(), q, ws, actor, *source, time.Now().UTC()); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return projectErr(404, "project_update_not_found", "execution evidence is no longer available")
			}
			return e
		}
		task, e := q.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: ws})
		if e != nil {
			return e
		}
		messages, e := q.ListTaskMessages(r.Context(), taskID)
		if e != nil {
			return e
		}
		out.Task = taskToResponse(task, uuidToString(ws))
		out.Messages = make([]protocol.TaskMessagePayload, len(messages))
		for i, message := range messages {
			out.Messages[i] = taskMessageToPayload(message, uuidToString(taskID), uuidToString(task.IssueID))
		}
		return nil
	})
	if err != nil {
		writeProjectAPIError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
