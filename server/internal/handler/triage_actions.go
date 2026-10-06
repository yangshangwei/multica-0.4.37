package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type TriageActionInput struct {
	RequestID        string                     `json:"request_id"`
	ExpectedRevision int64                      `json:"expected_revision"`
	Action           string                     `json:"action"`
	Reason           string                     `json:"reason,omitempty"`
	DuplicateIssueID string                     `json:"duplicate_issue_id,omitempty"`
	SnoozedUntil     string                     `json:"snoozed_until,omitempty"`
	ReviewerID       *string                    `json:"reviewer_id,omitempty"`
	Fields           map[string]json.RawMessage `json:"fields,omitempty"`
}
type triageExecutionContext struct {
	AssigneeType string  `json:"assignee_type"`
	AssigneeID   string  `json:"assignee_id"`
	AgentID      string  `json:"agent_id"`
	RuntimeID    string  `json:"runtime_id"`
	ProjectID    *string `json:"project_id"`
	Fingerprint  string  `json:"fingerprint"`
}

func (h *Handler) triageValidateReferences(ctx context.Context, tx pgx.Tx, r *http.Request, ws, project pgtype.UUID, kind pgtype.Text, assignee pgtype.UUID) error {
	if project.Valid {
		if _, err := h.Queries.WithTx(tx).LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: project, WorkspaceID: ws}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return triageErr(400, "project unavailable")
			}
			return err
		}
	}
	if assignee.Valid {
		var id pgtype.UUID
		var err error
		switch kind.String {
		case "agent":
			err = tx.QueryRow(ctx, `SELECT id FROM agent WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, assignee, ws).Scan(&id)
		case "squad":
			err = tx.QueryRow(ctx, `SELECT id FROM squad WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, assignee, ws).Scan(&id)
		case "member":
			err = triageValidateMember(ctx, h.Queries.WithTx(tx), ws, assignee)
		}
		if err != nil {
			return triageReferenceError(err, "assignee unavailable")
		}
	}
	scoped := *h
	scoped.Queries = h.Queries.WithTx(tx)
	scoped.DB = tx
	status, msg := scoped.validateAssigneePair(ctx, r, uuidToString(ws), kind, assignee)
	if status != 0 {
		return triageErr(status, msg)
	}
	return nil
}
func triageValidateAction(in TriageActionInput) error {
	switch in.Action {
	case "accept", "accept_and_execute", "reject", "duplicate", "snooze", "unsnooze", "reopen", "assign_reviewer":
	default:
		return triageErr(400, "unsupported triage action")
	}
	reason := strings.TrimSpace(in.Reason)
	if (in.Action == "reject" || in.Action == "reopen") && reason == "" {
		return triageErr(400, "reason is required")
	}
	if utf8.RuneCountInString(reason) > 2000 {
		return triageErr(400, "reason must contain at most 2000 characters")
	}
	if in.Action == "snooze" {
		at, e := time.Parse(time.RFC3339, in.SnoozedUntil)
		if e != nil || !at.After(time.Now()) || at.After(time.Now().Add(90*24*time.Hour)) {
			return triageErr(400, "snoozed_until must be in the next 90 days")
		}
	}
	if in.Action != "accept" && in.Action != "accept_and_execute" && len(in.Fields) > 0 {
		return triageErr(400, "fields are only supported when accepting")
	}
	return nil
}
func (h *Handler) applyTriageAcceptance(ctx context.Context, tx pgx.Tx, r *http.Request, ws pgtype.UUID, s db.WorkspaceTriageSetting, item TriageItem, in TriageActionInput) (triageExecutionContext, error) {
	var execution triageExecutionContext
	q := h.Queries.WithTx(tx)
	issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: parseUUID(item.Issue.ID), WorkspaceID: ws})
	if err != nil {
		return execution, err
	}
	project := item.CandidateProjectID
	kind := item.CandidateAssigneeType
	assignee := item.CandidateAssigneeID
	priority := issue.Priority
	status := s.AcceptanceStatus
	title := issue.Title
	description := textToPtr(issue.Description)
	start := dateToPtr(issue.StartDate)
	due := dateToPtr(issue.DueDate)
	var labels []string
	labelsSet := false
	for name, raw := range in.Fields {
		if string(raw) == "null" && (name == "priority" || name == "status" || name == "title" || name == "label_ids") {
			return execution, triageErr(400, name+" cannot be null")
		}
		var e error
		switch name {
		case "project_id":
			e = json.Unmarshal(raw, &project)
		case "assignee_type":
			e = json.Unmarshal(raw, &kind)
		case "assignee_id":
			e = json.Unmarshal(raw, &assignee)
		case "priority":
			e = json.Unmarshal(raw, &priority)
		case "status":
			e = json.Unmarshal(raw, &status)
		case "title":
			e = json.Unmarshal(raw, &title)
		case "description":
			e = json.Unmarshal(raw, &description)
		case "start_date":
			e = json.Unmarshal(raw, &start)
		case "due_date":
			e = json.Unmarshal(raw, &due)
		case "label_ids":
			e = json.Unmarshal(raw, &labels)
			labelsSet = true
		default:
			return execution, triageErr(400, "unsupported acceptance field: "+name)
		}
		if e != nil {
			return execution, triageErr(400, "invalid "+name)
		}
	}
	desc := ""
	if description != nil {
		desc = *description
	}
	if err = triageValidateIntake(TriageIntakeInput{Title: title, Description: desc, Priority: priority}); err != nil {
		return execution, err
	}
	if s.RequirePriority && (priority == "none" || priority == "") {
		return execution, triageErr(400, "select an explicit priority before accepting")
	}
	resolved, e := issuestatus.Resolve(ctx, q, ws, status)
	if e != nil || resolved.Category != "backlog" && resolved.Category != "todo" {
		return execution, triageErr(400, "acceptance status must be an active backlog or todo status")
	}
	projectID, err := triageOptionalUUID(project, "project_id")
	if err != nil {
		return execution, err
	}
	assigneeID, err := triageOptionalUUID(assignee, "assignee_id")
	if err != nil {
		return execution, err
	}
	assigneeType := pgtype.Text{}
	if kind != nil {
		assigneeType = triageText(*kind)
	}
	if err = h.triageValidateReferences(ctx, tx, r, ws, projectID, assigneeType, assigneeID); err != nil {
		return execution, err
	}
	startDate, err := triageDate(start, "start_date")
	if err != nil {
		return execution, err
	}
	dueDate, err := triageDate(due, "due_date")
	if err != nil {
		return execution, err
	}
	if startDate.Valid && dueDate.Valid && startDate.Time.After(dueDate.Time) {
		return execution, triageErr(400, "start_date must not be after due_date")
	}
	if labelsSet {
		labelIDs := make([]pgtype.UUID, 0, len(labels))
		for _, v := range labels {
			id, e := triageUUID(v, "label_ids")
			if e != nil {
				return execution, e
			}
			var locked pgtype.UUID
			if e = tx.QueryRow(ctx, `SELECT id FROM issue_label WHERE id=$1 AND workspace_id=$2 AND resource_type='issue' FOR SHARE NOWAIT`, id, ws).Scan(&locked); e != nil {
				return execution, triageReferenceError(e, "label unavailable")
			}
			labelIDs = append(labelIDs, id)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM issue_to_label WHERE issue_id=$1`, issue.ID); err != nil {
			return execution, err
		}
		for _, id := range labelIDs {
			if _, err = tx.Exec(ctx, `INSERT INTO issue_to_label(issue_id,label_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, issue.ID, id); err != nil {
				return execution, err
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE issue SET admission_status='accepted',status=$2,priority=$3,project_id=$4,assignee_type=$5,assignee_id=$6,title=$7,description=$8,start_date=$9,due_date=$10,revision=revision+1,updated_at=now() WHERE id=$1`, issue.ID, status, priority, projectID, assigneeType, assigneeID, title, triageText(desc), startDate, dueDate)
	if err != nil {
		return execution, err
	}
	if _, err = tx.Exec(ctx, `UPDATE issue_triage SET snoozed_until=NULL WHERE issue_id=$1`, issue.ID); err != nil {
		return execution, err
	}
	if in.Action == "accept_and_execute" {
		issue, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issue.ID, WorkspaceID: ws})
		if err != nil {
			return execution, err
		}
		execution, err = h.triageExecutionSnapshot(ctx, tx, r, issue)
		if err != nil {
			return execution, err
		}
	}
	return execution, nil
}
func (h *Handler) triageExecutionSnapshot(ctx context.Context, tx pgx.Tx, r *http.Request, issue db.Issue) (triageExecutionContext, error) {
	var out triageExecutionContext
	q := h.Queries.WithTx(tx)
	if issue.AssigneeType.String != "agent" && issue.AssigneeType.String != "squad" {
		return out, triageErr(400, "accept and execute requires an agent or squad assignee")
	}
	if err := h.triageValidateReferences(ctx, tx, r, issue.WorkspaceID, issue.ProjectID, issue.AssigneeType, issue.AssigneeID); err != nil {
		return out, err
	}
	agentID := issue.AssigneeID
	if issue.AssigneeType.String == "squad" {
		s, e := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
		if e != nil {
			return out, e
		}
		agentID = s.LeaderID
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return out, err
	}
	if !agent.RuntimeID.Valid || agent.ArchivedAt.Valid {
		return out, triageErr(400, "assigned agent runtime unavailable")
	}
	verdict, err := service.AgentReadiness(ctx, service.RuntimeLookup{Queries: q, Source: "issue"}, agent)
	if err != nil {
		return out, err
	}
	if verdict.Blocked() {
		return out, triageErr(400, "assigned agent runtime unavailable")
	}
	// Full project/resource and issue execution inputs remain bound to the
	// original authorization. A changed target requires a fresh explicit run.
	var resources []byte
	err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]') FROM project_resource r WHERE r.project_id=$1`, issue.ProjectID).Scan(&resources)
	if err != nil {
		return out, err
	}
	out = triageExecutionContext{AssigneeType: issue.AssigneeType.String, AssigneeID: uuidToString(issue.AssigneeID), AgentID: uuidToString(agentID), RuntimeID: uuidToString(agent.RuntimeID), ProjectID: uuidToPtr(issue.ProjectID), Fingerprint: triageHash([]any{issue.Title, issue.Description, issue.Status, issue.ProjectID, issue.AssigneeType, issue.AssigneeID, agent.Name, agent.Description, agent.OwnerID, agent.RuntimeMode, agent.RuntimeConfig, agent.McpConfig, agent.CustomEnv, agent.CustomArgs, agent.Instructions, agent.Model, agent.ThinkingLevel, agent.ServiceTier, agent.AutonomyLevel, agent.DisabledRuntimeSkills, agent.ComposioToolkitAllowlist, json.RawMessage(resources)})}
	return out, nil
}
func (h *Handler) actOnTriageItem(r *http.Request, idValue string, in TriageActionInput, preview bool) (TriageActionResult, error) {
	var result TriageActionResult
	var err error
	err = service.RetryProjectAssociationTransaction(r.Context(), func() error {
		result, err = h.actOnTriageItemOnce(r, idValue, in, preview)
		return err
	})
	return result, err
}

func (h *Handler) actOnTriageItemOnce(r *http.Request, idValue string, in TriageActionInput, preview bool) (TriageActionResult, error) {
	var out TriageActionResult
	request, err := triageUUID(in.RequestID, "request_id")
	if err != nil && !preview {
		return out, err
	}
	ctx := r.Context()
	tx, ws, actor, s, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	if !preview {
		var priorID pgtype.UUID
		lookupErr := tx.QueryRow(ctx, `SELECT id FROM triage_action WHERE workspace_id=$1 AND actor_id=$2 AND request_id=$3`, ws, actor, request).Scan(&priorID)
		if lookupErr == nil {
			a, e := q.GetTriageAction(ctx, db.GetTriageActionParams{ID: priorID, WorkspaceID: ws})
			if e != nil {
				return out, e
			}
			var snapshot TriageItem
			if e = json.Unmarshal(a.BeforeSnapshot, &snapshot); e != nil {
				return out, e
			}
			matches := idValue == snapshot.Issue.Identifier
			if parsed, e := triageUUID(idValue, "issue_id"); e == nil {
				matches = parsed == a.IssueID
			}
			hash := triageHash(struct {
				ID    string
				Input TriageActionInput
			}{uuidToString(a.IssueID), in})
			if !matches || a.PayloadHash != hash {
				return out, triageErr(409, "request_id already used for a different action")
			}
			out.Item, e = h.triageItem(ctx, q, ws, a.IssueID)
			if e != nil {
				if !triageMissing(e) {
					return out, e
				}
				return out, triageErr(409, "previous action result was deleted: "+uuidToString(a.IssueID))
			}
			out.Action = triageActionResponse(a)
			if a.Action == "accept_and_execute" && !a.TaskID.Valid && a.ExecutionStatus == "pending" {
				_ = tx.Rollback(ctx)
				if resumed, e := h.retryTriageExecution(r, a.ID); e == nil {
					return resumed, nil
				}
				if latest, e := h.Queries.GetTriageAction(ctx, db.GetTriageActionParams{ID: a.ID, WorkspaceID: ws}); e == nil {
					out.Action = triageActionResponse(latest)
				}
			}
			return out, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return out, lookupErr
		}
	}
	if err := triageValidateAction(in); err != nil {
		return out, err
	}
	id, err := h.triageResolveID(ctx, q, ws, idValue)
	if err != nil {
		return out, err
	}
	hash := triageHash(struct {
		ID    string
		Input TriageActionInput
	}{uuidToString(id), in})
	lockedIssue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: id, WorkspaceID: ws})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, triageErr(404, "triage item unavailable")
		}
		return out, err
	}
	// Reviewable intake is never formally assigned to an iteration. Refuse a
	// damaged association rather than changing its facts without a join decision.
	if lockedIssue.CurrentIterationID.Valid {
		return out, triageErr(409, "triage item has an invalid iteration association")
	}
	before, err := h.triageItem(ctx, q, ws, id)
	if err != nil {
		return out, err
	}
	if before.Issue.Revision != in.ExpectedRevision {
		return out, triageErr(409, "triage item changed; refresh before deciding")
	}
	if in.Action == "reopen" {
		if !s.Enabled {
			return out, triageErr(409, "enable triage before reopening")
		}
		if before.Issue.AdmissionStatus != "rejected" && before.Issue.AdmissionStatus != "duplicate" {
			return out, triageErr(409, "only rejected or duplicate items can reopen")
		}
	} else if before.Issue.AdmissionStatus != "pending" {
		return out, triageErr(409, "item is no longer awaiting triage")
	}
	execution := triageExecutionContext{}
	switch in.Action {
	case "accept", "accept_and_execute":
		execution, err = h.applyTriageAcceptance(ctx, tx, r, ws, s, before, in)
	case "reject", "duplicate":
		target := pgtype.UUID{}
		identifier := pgtype.Text{}
		result := "rejected"
		if in.Action == "duplicate" {
			result = "duplicate"
			target, err = h.triageResolveID(ctx, q, ws, in.DuplicateIssueID)
			if err != nil {
				return out, triageErr(400, "duplicate target unavailable")
			}
			if target == id {
				return out, triageErr(400, "an item cannot duplicate itself")
			}
			var number int32
			err = tx.QueryRow(ctx, `SELECT number FROM issue WHERE id=$1 AND workspace_id=$2 AND admission_status IN ('accepted','not_required') FOR SHARE NOWAIT`, target, ws).Scan(&number)
			if err != nil {
				return out, triageErr(400, "duplicate target must be a formal task in this workspace")
			}
			workspace, e := q.GetWorkspace(ctx, ws)
			if e != nil {
				return out, e
			}
			identifier = triageText(workspace.IssuePrefix + "-" + strconv.Itoa(int(number)))
		}
		_, err = tx.Exec(ctx, `UPDATE issue SET admission_status=$2,status='cancelled',revision=revision+1,updated_at=now() WHERE id=$1`, id, result)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE issue_triage SET duplicate_issue_id=$2,duplicate_identifier=$3,snoozed_until=NULL WHERE issue_id=$1`, id, target, identifier)
		}
	case "snooze", "unsnooze":
		at := pgtype.Timestamptz{}
		if in.Action == "snooze" {
			t, _ := time.Parse(time.RFC3339, in.SnoozedUntil)
			at = pgtype.Timestamptz{Time: t, Valid: true}
		}
		_, err = tx.Exec(ctx, `UPDATE issue_triage SET snoozed_until=$2 WHERE issue_id=$1`, id, at)
	case "assign_reviewer":
		reviewer, e := triageOptionalUUID(in.ReviewerID, "reviewer_id")
		if e != nil {
			return out, e
		}
		if reviewer.Valid {
			if e = triageValidateMember(ctx, q, ws, reviewer); e != nil {
				return out, e
			}
		}
		_, err = tx.Exec(ctx, `UPDATE issue_triage SET reviewer_id=$2 WHERE issue_id=$1`, id, reviewer)
	case "reopen":
		reviewer := pgtype.UUID{}
		if s.ResponsibilityMode == "assign" {
			reviewer = s.ResponsibilityMemberID
			if reviewer.Valid && triageValidateMember(ctx, q, ws, reviewer) != nil {
				reviewer = pgtype.UUID{}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE issue SET admission_status='pending',status='backlog',project_id=NULL,assignee_type=NULL,assignee_id=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE issue_triage SET round=round+1,entered_at=now(),snoozed_until=NULL,duplicate_issue_id=NULL,duplicate_identifier=NULL,reviewer_id=$2 WHERE issue_id=$1`, id, reviewer)
		}
	}
	if err != nil {
		return out, err
	}
	if in.Action == "snooze" || in.Action == "unsnooze" || in.Action == "assign_reviewer" {
		_, err = tx.Exec(ctx, `UPDATE issue SET revision=revision+1,updated_at=now() WHERE id=$1`, id)
		if err != nil {
			return out, err
		}
	}
	out.Item, err = h.triageItem(ctx, q, ws, id)
	if err != nil {
		return out, err
	}
	if preview {
		return out, nil
	}
	actionID := dbid.NewV7()
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(out.Item)
	executionJSON, _ := json.Marshal(execution)
	executionStatus := "not_requested"
	if in.Action == "accept_and_execute" {
		executionStatus = "pending"
	}
	_, err = tx.Exec(ctx, `INSERT INTO triage_action(id,workspace_id,issue_id,actor_id,request_id,payload_hash,action,round,reason,before_snapshot,after_snapshot,execution_status,execution_context) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, actionID, ws, id, actor, request, hash, in.Action, out.Item.Round, triageText(strings.TrimSpace(in.Reason)), beforeJSON, afterJSON, executionStatus, executionJSON)
	if err != nil {
		return out, err
	}
	a, err := q.GetTriageAction(ctx, db.GetTriageActionParams{ID: actionID, WorkspaceID: ws})
	if err != nil {
		return out, err
	}
	out.Action = triageActionResponse(a)
	if err = h.triageActionNotifications(ctx, tx, ws, actor, s, before, out); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	h.broadcastTriage(ws, out.Item.Issue.ID, "", false)
	h.deliverTriageNotifications(ctx, ws)
	if in.Action == "accept_and_execute" {
		if result, e := h.retryTriageExecution(r, actionID); e == nil {
			return result, nil
		}
		a, e := h.Queries.GetTriageAction(ctx, db.GetTriageActionParams{ID: actionID, WorkspaceID: ws})
		if e == nil {
			out.Action = triageActionResponse(a)
		}
	}
	return out, nil
}
func (h *Handler) ActOnTriageItem(w http.ResponseWriter, r *http.Request) {
	var in TriageActionInput
	if err := triageDecode(r, &in); err != nil {
		writeTriageError(w, err)
		return
	}
	out, err := h.actOnTriageItem(r, chi.URLParam(r, "id"), in, false)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) RetryTriageExecution(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if err := triageDecode(r, &body); err != nil {
		writeTriageError(w, err)
		return
	}
	id, err := triageUUID(chi.URLParam(r, "actionId"), "action_id")
	if err != nil {
		writeTriageError(w, err)
		return
	}
	out, err := h.retryTriageExecution(r, id)
	if err != nil {
		writeTriageError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (h *Handler) retryTriageExecution(r *http.Request, actionID pgtype.UUID) (TriageActionResult, error) {
	var out TriageActionResult
	ctx := r.Context()
	ws, err := h.triageReadScope(r)
	if err != nil {
		return out, err
	}
	actor, err := h.triageHumanActor(r, uuidToString(ws))
	if err != nil {
		return out, err
	}
	a, err := h.Queries.GetTriageAction(ctx, db.GetTriageActionParams{ID: actionID, WorkspaceID: ws})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		return out, triageErr(404, "execution action unavailable")
	}
	if a.ActorID != actor || a.Action != "accept_and_execute" {
		return out, triageErr(403, "only the original authorizer may retry execution")
	}
	out.Item, err = h.triageItem(ctx, h.Queries, ws, a.IssueID)
	if err != nil {
		if !triageMissing(err) {
			return out, err
		}
		return out, triageErr(409, "execution result issue was deleted: "+uuidToString(a.IssueID))
	}
	out.Action = triageActionResponse(a)
	if a.TaskID.Valid {
		return out, nil
	}
	fail := func(e error) (TriageActionResult, error) {
		_, _ = h.DB.Exec(ctx, `UPDATE triage_action SET execution_status='failed',execution_error=$2 WHERE id=$1 AND task_id IS NULL`, actionID, e.Error())
		return out, e
	}
	var expected triageExecutionContext
	if err = json.Unmarshal(a.ExecutionContext, &expected); err != nil {
		return fail(err)
	}
	// First transaction checks current authority/context then ends before external
	// preparation. The enqueue transaction repeats those checks under the row lock.
	checkTx, _, _, _, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		return fail(err)
	}
	q := h.Queries.WithTx(checkTx)
	issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: a.IssueID, WorkspaceID: ws})
	if err == nil && issue.AdmissionStatus != "accepted" {
		err = triageErr(409, "issue is no longer accepted")
	}
	var current triageExecutionContext
	if err == nil {
		current, err = h.triageExecutionSnapshot(ctx, checkTx, r, issue)
	}
	_ = checkTx.Rollback(ctx)
	if err != nil {
		return fail(err)
	}
	if !reflect.DeepEqual(expected, current) {
		return fail(triageErr(409, "execution assignment or context changed; make a fresh explicit run decision"))
	}
	attr := attribution.Result{UserID: actor, AccountableUserID: actor, Source: attribution.SourceDirectHuman}
	prepared, err := h.TaskService.PrepareIssueTaskEnqueue(ctx, parseUUID(expected.AgentID), attr)
	if err != nil {
		return fail(err)
	}
	tx, _, _, _, err := h.beginTriageWrite(ctx, r, true)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback(ctx)
	failInTx := func(e error) (TriageActionResult, error) { _ = tx.Rollback(ctx); return fail(e) }
	q = h.Queries.WithTx(tx)
	if _, err = tx.Exec(ctx, `SELECT 1 FROM triage_action WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, actionID, ws); err != nil {
		return failInTx(err)
	}
	a, err = q.GetTriageAction(ctx, db.GetTriageActionParams{ID: actionID, WorkspaceID: ws})
	if err != nil {
		return failInTx(err)
	}
	if a.TaskID.Valid {
		out.Action = triageActionResponse(a)
		return out, nil
	}
	issue, err = q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: a.IssueID, WorkspaceID: ws})
	if err != nil {
		return failInTx(err)
	}
	current, err = h.triageExecutionSnapshot(ctx, tx, r, issue)
	if err != nil {
		_ = tx.Rollback(ctx)
		return failInTx(err)
	}
	if issue.AdmissionStatus != "accepted" || !reflect.DeepEqual(expected, current) {
		_ = tx.Rollback(ctx)
		return failInTx(triageErr(409, "execution assignment or context changed; make a fresh explicit run decision"))
	}
	squad := pgtype.UUID{}
	if expected.AssigneeType == "squad" {
		squad = parseUUID(expected.AssigneeID)
	}
	task, reused, err := h.TaskService.EnqueuePreparedIssueTaskInTx(ctx, tx, issue, parseUUID(expected.AgentID), squad, "", attr, prepared)
	if err != nil {
		_ = tx.Rollback(ctx)
		return failInTx(err)
	}
	_, err = tx.Exec(ctx, `UPDATE triage_action SET task_id=$2,execution_status='queued',execution_error=NULL WHERE id=$1`, actionID, task.ID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return failInTx(err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		// Reconcile a possibly committed operation by its durable identity; never
		// issue another enqueue merely because the commit response was lost.
		current, e := h.Queries.GetTriageAction(ctx, db.GetTriageActionParams{ID: actionID, WorkspaceID: ws})
		if e != nil || !current.TaskID.Valid {
			return out, err
		}
		a = current
	} else {
		a.TaskID = task.ID
		a.ExecutionStatus = "queued"
		a.ExecutionError = pgtype.Text{}
		if !reused {
			h.TaskService.FinalizeIssueTaskEnqueue(ctx, task)
		}
	}
	out.Action = triageActionResponse(a)
	h.broadcastTriage(ws, out.Item.Issue.ID, "", false)
	return out, nil
}

// A candidate is a visible proposal, not invocation authorization. Acceptance
// validates the actual assignment again through the existing invocation gate.
func (h *Handler) triageValidateCandidates(ctx context.Context, tx pgx.Tx, r *http.Request, ws, project pgtype.UUID, kind pgtype.Text, assignee pgtype.UUID) error {
	if kind.Valid != assignee.Valid {
		return triageErr(400, "candidate assignee type and ID must be provided together")
	}
	q := h.Queries.WithTx(tx)
	if project.Valid {
		if _, err := h.Queries.WithTx(tx).LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: project, WorkspaceID: ws}); err != nil {
			return triageReferenceError(err, "candidate project unavailable")
		}
	}
	if !assignee.Valid {
		return nil
	}
	switch kind.String {
	case "member":
		return triageValidateMember(ctx, q, ws, assignee)
	case "agent", "squad":
		agentID := assignee
		var locked pgtype.UUID
		if kind.String == "squad" {
			if err := tx.QueryRow(ctx, `SELECT id FROM squad WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, assignee, ws).Scan(&locked); err != nil {
				return triageReferenceError(err, "candidate squad unavailable")
			}
		}
		if kind.String == "squad" {
			s, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: assignee, WorkspaceID: ws})
			if err != nil || s.ArchivedAt.Valid {
				return triageReferenceError(err, "candidate squad unavailable")
			}
			agentID = s.LeaderID
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM agent WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, agentID, ws).Scan(&locked); err != nil {
			return triageReferenceError(err, "candidate agent unavailable")
		}
		a, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: ws})
		if err != nil || a.ArchivedAt.Valid {
			return triageReferenceError(err, "candidate agent unavailable")
		}
		scoped := *h
		scoped.Queries = q
		scoped.DB = tx
		actorType, actorID := scoped.resolveActor(r, requestUserID(r), uuidToString(ws))
		if !scoped.canAccessPrivateAgent(ctx, a, actorType, actorID, uuidToString(ws)) {
			return triageErr(403, "candidate agent unavailable")
		}
		return nil
	default:
		return triageErr(400, "invalid candidate assignee type")
	}
}
