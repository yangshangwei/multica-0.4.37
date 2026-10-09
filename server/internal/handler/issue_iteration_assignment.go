package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type issueIterationAssignment struct {
	target         pgtype.UUID
	targetRevision pgtype.Int8
	issueRevision  int64
	reason         *string
	allowCompleted bool
}

func writeIssueIterationOperationError(w http.ResponseWriter, err error) bool {
	var api *iteration.OperationError
	if !errors.As(err, &api) {
		return false
	}
	writeIterationAPIError(w, err)
	return true
}
func parseIssueIterationTarget(raw json.RawMessage) (pgtype.UUID, error) {
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return pgtype.UUID{}, iterationAPIError(400, "invalid_request", "current_iteration_id must be a UUID or null")
	}
	if value == nil {
		return pgtype.UUID{}, nil
	}
	id, err := util.ParseUUID(*value)
	if err != nil || id.Bytes == [16]byte{} {
		return pgtype.UUID{}, iterationAPIError(400, "invalid_request", "current_iteration_id must be a UUID or null")
	}
	return id, nil
}
func parseIssueIterationRevision(raw json.RawMessage, field string) (int64, error) {
	if len(raw) == 0 {
		return 0, iterationAPIError(428, "iteration_confirmation_required", field+" is required")
	}
	var revision int64
	if err := json.Unmarshal(raw, &revision); err != nil || revision < 1 || revision > 9007199254740991 {
		return 0, iterationAPIError(400, "invalid_request", field+" must be a positive safe integer")
	}
	return revision, nil
}
func (h *Handler) parseIssueIterationCreate(r *http.Request, req CreateIssueRequest) (*issueIterationAssignment, error) {
	if len(req.IterationRolloverCount) > 0 {
		return nil, iterationAPIError(428, "iteration_confirmation_required", "iteration_rollover_count is server-owned")
	}
	if len(req.CurrentIterationID) == 0 {
		if len(req.ExpectedIterationRevision) > 0 {
			return nil, iterationAPIError(400, "invalid_request", "expected_iteration_revision requires current_iteration_id")
		}
		if len(req.AllowCompleted) > 0 {
			return nil, iterationAPIError(400, "invalid_request", "allow_completed requires a target iteration")
		}
		return nil, nil
	}
	target, err := parseIssueIterationTarget(req.CurrentIterationID)
	if err != nil {
		return nil, err
	}
	input := &issueIterationAssignment{target: target}
	if target.Valid {
		value, err := parseIssueIterationRevision(req.ExpectedIterationRevision, "expected_iteration_revision")
		if err != nil {
			return nil, err
		}
		input.targetRevision = pgtype.Int8{Int64: value, Valid: true}
	} else if len(req.ExpectedIterationRevision) > 0 {
		return nil, iterationAPIError(400, "invalid_request", "An unassociated create does not accept a target revision")
	}
	if len(req.AllowCompleted) > 0 {
		if !target.Valid {
			return nil, iterationAPIError(400, "invalid_request", "allow_completed requires a target iteration")
		}
		if string(req.AllowCompleted) == "null" || json.Unmarshal(req.AllowCompleted, &input.allowCompleted) != nil {
			return nil, iterationAPIError(400, "invalid_request", "allow_completed must be a boolean")
		}
	}
	return input, nil
}
func (h *Handler) lockIssueIterationSettings(ctx context.Context, tx pgx.Tx, ws pgtype.UUID) error {
	settings, err := db.New(tx).LockIterationSettings(ctx, ws)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !settings.Enabled) {
		return iterationAPIError(422, "iteration_disabled", "Iterations are disabled")
	}
	return err
}

type issueIterationCreateHooks struct {
	before func(context.Context, pgx.Tx, *service.IssueCreateParams) error
	after  func(context.Context, pgx.Tx, *service.IssueCreateResult) error
}

func (h *Handler) issueIterationCreateHooks(r *http.Request, input *issueIterationAssignment, actorType, actorID, userID string) issueIterationCreateHooks {
	if input == nil {
		return issueIterationCreateHooks{}
	}
	var target db.Iteration
	operationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	actor, _ := json.Marshal(map[string]string{"type": actorType, "id": actorID, "user_id": userID})
	return issueIterationCreateHooks{
		before: func(ctx context.Context, tx pgx.Tx, params *service.IssueCreateParams) error {
			target = db.Iteration{}
			if err := h.lockIssueIterationSettings(ctx, tx, params.WorkspaceID); err != nil {
				return err
			}
			if !input.target.Valid {
				return nil
			}
			rows, err := db.New(tx).LockIterations(ctx, db.LockIterationsParams{WorkspaceID: params.WorkspaceID, Column2: []pgtype.UUID{input.target}})
			if err != nil {
				return err
			}
			if len(rows) != 1 {
				return iterationAPIError(404, "iteration_not_found", "Iteration not found")
			}
			target = rows[0]
			if target.Revision != input.targetRevision.Int64 {
				return iterationAPIError(409, "iteration_revision_conflict", "Target iteration changed")
			}
			if target.Status != "planned" && target.Status != "active" {
				return iterationAPIError(409, "iteration_history_move_unsupported", "Historical iteration membership cannot change")
			}
			return nil
		},
		after: func(ctx context.Context, tx pgx.Tx, result *service.IssueCreateResult) error {
			if !input.target.Valid {
				return nil
			}
			change, err := iteration.PrepareMembershipChange(ctx, tx, result.Issue, db.Iteration{}, target, input.allowCompleted)
			if err != nil {
				return err
			}
			sampled, err := iteration.SampleBusinessTime(ctx, tx, nil)
			if err != nil {
				return err
			}
			result.Issue, err = iteration.CommitMembershipChange(ctx, tx, change, actor, operationID, sampled)
			return err
		},
	}
}
func parseIssueIterationUpdateFields(body []byte) (map[string]json.RawMessage, issueIterationAssignment, error) {
	var input issueIterationAssignment
	if _, err := iteration.CanonicalHash(json.RawMessage(body)); err != nil {
		return nil, input, iterationAPIError(400, "invalid_request", "Invalid or duplicate iteration assignment fields")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, input, err
	}
	fields := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		key = strings.ToLower(key)
		if _, seen := fields[key]; seen {
			return nil, input, iterationAPIError(400, "invalid_request", "Duplicate iteration assignment field")
		}
		switch key {
		case "current_iteration_id", "expected_revision", "iteration_reason", "allow_completed":
		default:
			return nil, input, iterationAPIError(400, "invalid_request", "Iteration assignment must be submitted separately from other issue changes")
		}
		fields[key] = value
	}
	var err error
	input.target, err = parseIssueIterationTarget(fields["current_iteration_id"])
	if err != nil {
		return nil, input, err
	}
	input.issueRevision, err = parseIssueIterationRevision(fields["expected_revision"], "expected_revision")
	if err != nil {
		return nil, input, err
	}
	if raw, ok := fields["iteration_reason"]; ok {
		if err = json.Unmarshal(raw, &input.reason); err != nil {
			return nil, input, iterationAPIError(400, "invalid_request", "iteration_reason must be a string or null")
		}
		if input.reason != nil {
			reason := strings.TrimSpace(strings.ReplaceAll(*input.reason, "\r\n", "\n"))
			input.reason = &reason
		}
	}
	if raw, ok := fields["allow_completed"]; ok {
		if string(raw) == "null" || json.Unmarshal(raw, &input.allowCompleted) != nil {
			return nil, input, iterationAPIError(400, "invalid_request", "allow_completed must be a boolean")
		}
	}
	return fields, input, nil
}
func (h *Handler) updateIssueIterationAssignment(w http.ResponseWriter, r *http.Request, before db.Issue, body []byte, req UpdateIssueRequest) {
	if len(req.IterationRolloverCount) > 0 {
		writeIterationAPIError(w, iterationAPIError(428, "iteration_confirmation_required", "iteration_rollover_count is server-owned"))
		return
	}
	fields, input, err := parseIssueIterationUpdateFields(body)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	if !h.requireAgentAutonomy(w, r, uuidToString(before.WorkspaceID), service.AutonomyContributor, "change an issue's iteration") {
		return
	}
	params := db.UpdateIssueParams{ID: before.ID, ExpectedRevision: pgtype.Int8{Int64: input.issueRevision, Valid: true}}
	after, current, _, err := h.updateIssueAtomically(r, before.WorkspaceID, params, fields, nil, nil, nil, "")
	if err != nil {
		if writeIssueUpdateAccessError(w, err) || writeIssueAdmissionError(w, err) || writeIssueProjectAssociationError(w, err) || writeIssueIterationOperationError(w, err) {
			return
		}
		writeIterationAPIError(w, err)
		return
	}
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	if current.Revision != after.Revision {
		actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(after.WorkspaceID))
		h.publish(protocol.EventIssueUpdated, uuidToString(after.WorkspaceID), actorType, actorID, map[string]any{"issue": response, "iteration_changed": true, "suppress_execution": true})
	}
	writeJSON(w, 200, response)
}
func (h *Handler) updateIssueIterationInTx(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, params db.UpdateIssueParams, fields map[string]json.RawMessage, actor json.RawMessage, operationID pgtype.UUID, authorize func(pgx.Tx, db.UpdateIssueParams) error) (db.Issue, db.Issue, error) {
	raw, _ := json.Marshal(fields)
	_, input, err := parseIssueIterationUpdateFields(raw)
	if err != nil {
		return db.Issue{}, db.Issue{}, err
	}
	if err = h.lockIssueIterationSettings(ctx, tx, ws); err != nil {
		return db.Issue{}, db.Issue{}, err
	}
	q := db.New(tx)
	ids, err := q.ListIssueDeleteIterationIDs(ctx, db.ListIssueDeleteIterationIDsParams{WorkspaceID: ws, IssueIds: []pgtype.UUID{params.ID}})
	if err != nil {
		return db.Issue{}, db.Issue{}, err
	}
	if input.target.Valid {
		found := false
		for _, id := range ids {
			found = found || id == input.target
		}
		if !found {
			ids = append(ids, input.target)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return uuidToString(ids[i]) < uuidToString(ids[j]) })
	rows, err := q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: ws, Column2: ids})
	if err != nil {
		return db.Issue{}, db.Issue{}, err
	}
	if len(rows) != len(ids) {
		return db.Issue{}, db.Issue{}, iterationAPIError(404, "iteration_not_found", "Iteration not found")
	}
	byID := map[pgtype.UUID]db.Iteration{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	current, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{WorkspaceID: ws, ID: params.ID})
	if err != nil {
		return db.Issue{}, current, err
	}
	if current.Revision != input.issueRevision {
		return db.Issue{}, current, iterationAPIError(409, "iteration_revision_conflict", "Issue changed")
	}
	if err = authorize(tx, params); err != nil {
		return db.Issue{}, current, err
	}
	if current.CurrentIterationID.Valid && current.CurrentIterationID != input.target && (input.reason == nil || *input.reason == "") {
		return db.Issue{}, current, iterationAPIError(422, "iteration_validation_failed", "Leaving or switching iterations requires a reason")
	}
	change, err := iteration.PrepareMembershipChange(ctx, tx, current, byID[current.CurrentIterationID], byID[input.target], input.allowCompleted)
	if err != nil {
		return db.Issue{}, current, err
	}
	change.Reason = input.reason
	sampled, err := iteration.SampleBusinessTime(ctx, tx, nil)
	if err != nil {
		return db.Issue{}, current, err
	}
	after, err := iteration.CommitMembershipChange(ctx, tx, change, actor, operationID, sampled)
	return after, current, err
}
