package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CreateIterationInput struct {
	RequestID         string  `json:"request_id"`
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	CoordinatorUserID *string `json:"coordinator_user_id"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
	ConfirmedTimezone string  `json:"confirmed_timezone"`
}
type EditIterationInput struct {
	RequestID        string                     `json:"request_id"`
	ExpectedRevision int64                      `json:"expected_revision"`
	Fields           map[string]json.RawMessage `json:"fields"`
	Reason           *string                    `json:"reason"`
}
type ApplyIterationInput struct {
	RequestID   string          `json:"request_id"`
	Draft       iteration.Draft `json:"draft"`
	PreviewHash string          `json:"preview_hash"`
}

func iterationFailure(status int, code, message string) error {
	return &iteration.OperationError{Status: status, Code: code, Message: message}
}
func iterationText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
}
func iterationUUID(value *string) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}
	id, err := util.ParseUUID(*value)
	if err != nil || id.Bytes == [16]byte{} {
		return pgtype.UUID{}, iterationFailure(400, "invalid_request", "A valid UUID is required")
	}
	return id, nil
}
func iterationString(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	value := util.UUIDToString(id)
	return &value
}
func iterationNullable(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
func iterationActor(actor pgtype.UUID) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"type": "member", "id": actor, "user_id": actor})
	return raw
}
func iterationDates(start, end string) (pgtype.Date, pgtype.Date, error) {
	a, err := iteration.ParseDate(start)
	if err != nil {
		return pgtype.Date{}, pgtype.Date{}, iterationFailure(422, "iteration_validation_failed", "Invalid start date")
	}
	b, err := iteration.ParseDate(end)
	if err != nil || b.Before(a) {
		return pgtype.Date{}, pgtype.Date{}, iterationFailure(422, "iteration_validation_failed", "Invalid end date")
	}
	return pgtype.Date{Time: a, Valid: true}, pgtype.Date{Time: b, Valid: true}, nil
}
func (s *IterationService) lifecycleTime(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	if s.Now != nil {
		return s.Now(ctx, tx)
	}
	return iteration.SampleBusinessTime(ctx, tx, nil)
}
func (s *IterationService) lifecycleSettings(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, lock bool) (db.WorkspaceIterationSetting, error) {
	if s.Available == nil || !s.Available(ctx) {
		return db.WorkspaceIterationSetting{}, iterationFailure(422, "iteration_disabled", "Iteration management is unavailable")
	}
	var settings db.WorkspaceIterationSetting
	var err error
	if lock {
		settings, err = db.New(tx).LockIterationSettings(ctx, ws)
	} else {
		settings, err = db.New(tx).GetIterationSettings(ctx, ws)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !settings.Enabled) {
		return settings, iterationFailure(422, "iteration_disabled", "Iterations are disabled")
	}
	return settings, err
}
func (s *IterationService) lifecycleOperation(ctx context.Context, ws, actor pgtype.UUID, requestID, kind string, payload any, authorize func(context.Context, pgx.Tx) error, mutate iteration.OperationMutation) (iteration.WriteResult, error) {
	if s.TxStarter == nil {
		return iteration.WriteResult{}, errors.New("iteration transaction starter is required")
	}
	parsed, err := uuid.Parse(requestID)
	if err != nil || parsed == uuid.Nil {
		return iteration.WriteResult{}, iterationFailure(400, "invalid_request", "A request UUID is required")
	}
	return iteration.RunOperation(ctx, iterationTransactionStarter{s.TxStarter}, iteration.OperationKey{WorkspaceID: util.UUIDToString(ws), ActorUserID: util.UUIDToString(actor), RequestID: parsed.String(), Operation: kind}, payload, authorize, mutate)
}
func (s *IterationService) Create(ctx context.Context, ws, actor pgtype.UUID, input CreateIterationInput, authorize func(context.Context, pgx.Tx) error) (iteration.WriteResult, error) {
	input.Name = iterationText(input.Name)
	input.ConfirmedTimezone = iterationText(input.ConfirmedTimezone)
	if input.Description != nil {
		v := iterationText(*input.Description)
		input.Description = &v
	}
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Name must contain 1 to 200 characters")
	}
	first, last, err := iterationDates(input.StartDate, input.EndDate)
	if err != nil {
		return iteration.WriteResult{}, err
	}
	coordinator, err := iterationUUID(input.CoordinatorUserID)
	if err != nil {
		return iteration.WriteResult{}, err
	}
	if _, err = iteration.ValidateTimezone(input.ConfirmedTimezone); err != nil {
		return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Confirm a valid IANA timezone")
	}
	iterationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	return s.lifecycleOperation(ctx, ws, actor, input.RequestID, "create", input, authorize, func(ctx context.Context, tx pgx.Tx, operationID pgtype.UUID) (iteration.WriteResult, error) {
		settings, err := s.lifecycleSettings(ctx, tx, ws, true)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		q := db.New(tx)
		zone, err := q.LockIterationPlanningTimezone(ctx, ws)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		effective := "UTC"
		if zone.Valid {
			effective = zone.String
		}
		if input.ConfirmedTimezone != effective {
			return iteration.WriteResult{}, iterationFailure(409, "iteration_preview_stale", "Planning timezone changed")
		}
		if coordinator.Valid {
			if _, err = q.LockIterationReferenceMember(ctx, db.LockIterationReferenceMemberParams{WorkspaceID: ws, UserID: coordinator}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					err = iterationFailure(422, "iteration_validation_failed", "Coordinator must be a current member")
				}
				return iteration.WriteResult{}, err
			}
		}
		sampled, err := s.lifecycleTime(ctx, tx)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		row, err := q.CreateIteration(ctx, db.CreateIterationParams{ID: iterationID, WorkspaceID: ws, Name: input.Name, Description: iterationNullable(input.Description), CoordinatorUserID: coordinator, Timezone: effective, StartDate: first, EndDate: last, CreatedBy: actor, BusinessAt: pgtype.Timestamptz{Time: sampled, Valid: true}})
		if err != nil {
			return iteration.WriteResult{}, err
		}
		if err = appendLifecycleEvent(ctx, tx, ws, row.ID, actor, operationID, "create", nil, nil, nil, sampled); err != nil {
			return iteration.WriteResult{}, err
		}
		return iteration.WriteResult{IterationIDs: []string{util.UUIDToString(row.ID)}, Result: iteration.WriteSummary{SettingsRevision: settings.Revision}, CommittedAt: sampled}, nil
	})
}
func appendLifecycleEvent(ctx context.Context, tx pgx.Tx, ws, iid, actor, operationID pgtype.UUID, kind string, before, after any, reason *string, sampled time.Time) error {
	old, err := json.Marshal(before)
	if err != nil {
		return err
	}
	next, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return db.New(tx).AppendIterationLifecycleEvent(ctx, db.AppendIterationLifecycleEventParams{WorkspaceID: ws, IterationID: iid, OperationID: operationID, Kind: kind, Actor: iterationActor(actor), SampledAt: pgtype.Timestamptz{Time: sampled, Valid: true}, BeforeFacts: old, AfterFacts: next, Reason: iterationNullable(reason)})
}
func (s *IterationService) Edit(ctx context.Context, ws, actor, iid pgtype.UUID, input EditIterationInput, authorize func(context.Context, pgx.Tx) error) (iteration.WriteResult, error) {
	if input.ExpectedRevision < 1 || input.ExpectedRevision > 9007199254740991 || len(input.Fields) == 0 {
		return iteration.WriteResult{}, iterationFailure(400, "invalid_request", "Expected revision and fields are required")
	}
	if input.Reason != nil {
		v := iterationText(*input.Reason)
		input.Reason = &v
	}
	for key, raw := range input.Fields {
		switch key {
		case "name", "description", "coordinator_user_id", "start_date", "end_date":
		default:
			return iteration.WriteResult{}, iterationFailure(400, "invalid_request", "Unknown iteration field")
		}
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return iteration.WriteResult{}, iterationFailure(400, "invalid_request", "Iteration fields must be strings or nullable values")
		}
		if value == nil && key != "description" && key != "coordinator_user_id" {
			return iteration.WriteResult{}, iterationFailure(400, "invalid_request", "Field may not be null")
		}
		if value != nil {
			v := iterationText(*value)
			value = &v
		}
		normalized, _ := json.Marshal(value)
		input.Fields[key] = normalized
	}
	payload := struct {
		IterationID pgtype.UUID        `json:"iteration_id"`
		Input       EditIterationInput `json:"input"`
	}{iid, input}
	return s.lifecycleOperation(ctx, ws, actor, input.RequestID, "edit", payload, authorize, func(ctx context.Context, tx pgx.Tx, operationID pgtype.UUID) (iteration.WriteResult, error) {
		settings, err := s.lifecycleSettings(ctx, tx, ws, true)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		q := db.New(tx)
		rows, err := q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: ws, Column2: []pgtype.UUID{iid}})
		if err != nil {
			return iteration.WriteResult{}, err
		}
		if len(rows) != 1 {
			return iteration.WriteResult{}, iterationFailure(404, "iteration_not_found", "Iteration not found")
		}
		before := rows[0]
		after := before
		if before.Revision != input.ExpectedRevision {
			return iteration.WriteResult{}, iterationFailure(409, "iteration_revision_conflict", "Iteration changed")
		}
		for key, raw := range input.Fields {
			var value *string
			_ = json.Unmarshal(raw, &value)
			if (before.Status == "completed" || before.Status == "cancelled") && key != "name" && key != "description" {
				return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Only historical name and description may be corrected")
			}
			switch key {
			case "name":
				after.Name = *value
			case "description":
				after.Description = iterationNullable(value)
			case "coordinator_user_id":
				after.CoordinatorUserID, err = iterationUUID(value)
			case "start_date":
				if before.Status == "active" && *value != before.StartDate.Time.Format(time.DateOnly) {
					return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Active start date is immutable")
				}
				a, e := iteration.ParseDate(*value)
				err = e
				after.StartDate = pgtype.Date{Time: a, Valid: true}
			case "end_date":
				b, e := iteration.ParseDate(*value)
				err = e
				after.EndDate = pgtype.Date{Time: b, Valid: true}
			}
			if err != nil {
				return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Invalid iteration field")
			}
		}
		if utf8.RuneCountInString(after.Name) < 1 || utf8.RuneCountInString(after.Name) > 200 || after.EndDate.Time.Before(after.StartDate.Time) {
			return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "Invalid name or calendar dates")
		}
		dateChanged := after.StartDate != before.StartDate || after.EndDate != before.EndDate
		if before.Status == "active" && dateChanged && (input.Reason == nil || *input.Reason == "") {
			return iteration.WriteResult{}, iterationFailure(422, "iteration_validation_failed", "An end-date change requires a reason")
		}
		if after.CoordinatorUserID.Valid && after.CoordinatorUserID != before.CoordinatorUserID {
			if _, err = q.LockIterationReferenceMember(ctx, db.LockIterationReferenceMemberParams{WorkspaceID: ws, UserID: after.CoordinatorUserID}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					err = iterationFailure(422, "iteration_validation_failed", "Coordinator must be a current member")
				}
				return iteration.WriteResult{}, err
			}
		}
		sampled, err := s.lifecycleTime(ctx, tx)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		if after != before {
			after, err = q.EditIteration(ctx, db.EditIterationParams{WorkspaceID: ws, ID: iid, ExpectedRevision: input.ExpectedRevision, Name: after.Name, Description: after.Description, CoordinatorUserID: after.CoordinatorUserID, StartDate: after.StartDate, EndDate: after.EndDate})
			if err != nil {
				return iteration.WriteResult{}, err
			}
			kind := "edit"
			if dateChanged {
				kind = "date_edit"
			}
			if err = appendLifecycleEvent(ctx, tx, ws, iid, actor, operationID, kind, iteration.IterationFromRow(before), iteration.IterationFromRow(after), input.Reason, sampled); err != nil {
				return iteration.WriteResult{}, err
			}
		}
		return iteration.WriteResult{IterationIDs: []string{util.UUIDToString(iid)}, Result: iteration.WriteSummary{SettingsRevision: settings.Revision}, CommittedAt: sampled}, nil
	})
}
