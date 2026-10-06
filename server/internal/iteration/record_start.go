package iteration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// RecordExecutionStart records the first successful running transition in the
// current participation. The caller must hold the task and issue locks, prepare
// the record before starting the task, and commit that transition together with
// these facts. Enqueue, claim and completion are not execution-start evidence.
func RecordExecutionStart(ctx context.Context, tx pgx.Tx, record *IssueRecord, operationID pgtype.UUID) error {
	if record == nil || record.facts.HasStarted {
		return nil
	}
	actor := json.RawMessage(`{"type":"system","id":null,"user_id":null,"source":"task_start"}`)
	if err := validateIssueRecordIdentity(actor, operationID); err != nil {
		return err
	}
	after := record.facts
	after.HasStarted = true
	beforeJSON, err := json.Marshal(record.facts)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	q := db.New(tx)
	issue := record.before
	if err = q.MarkIterationParticipationStarted(ctx, db.MarkIterationParticipationStartedParams{WorkspaceID: issue.WorkspaceID, IterationID: issue.CurrentIterationID, IssueID: issue.ID}); err != nil {
		return err
	}
	kind := "execution_started"
	if record.planned {
		kind = "planned_activity"
	}
	if err = q.AppendIterationIssueEvent(ctx, db.AppendIterationIssueEventParams{
		WorkspaceID: issue.WorkspaceID, IterationID: issue.CurrentIterationID,
		IssueID: issue.ID, OperationID: operationID, Kind: kind,
		Actor:       actor,
		SampledAt:   pgtype.Timestamptz{Time: record.businessAt, Valid: true},
		BeforeFacts: beforeJSON, AfterFacts: afterJSON,
	}); err != nil {
		return fmt.Errorf("append execution start event: %w", err)
	}
	return q.AdvanceIterationScopeRevision(ctx, db.AdvanceIterationScopeRevisionParams{WorkspaceID: issue.WorkspaceID, ID: issue.CurrentIterationID})
}
