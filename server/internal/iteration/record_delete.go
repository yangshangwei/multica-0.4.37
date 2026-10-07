package iteration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// RecordIssueDeletion records one deletion/leave fact before the caller deletes
// the issue. Current participation is released while original facts and prior
// participation identities remain available to historical readers.
func RecordIssueDeletion(ctx context.Context, tx pgx.Tx, record *IssueRecord, actor json.RawMessage, operationID pgtype.UUID) error {
	if record == nil {
		return nil
	}
	if err := validateIssueRecordIdentity(actor, operationID); err != nil {
		return err
	}
	before, err := marshalMembershipFacts(record.facts, record.before.CurrentIterationID, pgtype.UUID{})
	if err != nil {
		return err
	}
	q := db.New(tx)
	issue := record.before
	sampled := pgtype.Timestamptz{Time: record.businessAt, Valid: true}
	if err = q.AppendIterationIssueEvent(ctx, db.AppendIterationIssueEventParams{
		WorkspaceID: issue.WorkspaceID, IterationID: issue.CurrentIterationID,
		IssueID: issue.ID, OperationID: operationID, Kind: "delete", Actor: actor,
		SampledAt: sampled, BeforeFacts: before, AfterFacts: []byte("null"),
	}); err != nil {
		return fmt.Errorf("append issue deletion fact: %w", err)
	}
	changed, err := q.LeaveDeletedIssueIterationParticipation(ctx, db.LeaveDeletedIssueIterationParticipationParams{
		WorkspaceID: issue.WorkspaceID, IterationID: issue.CurrentIterationID,
		IssueID: issue.ID, BusinessAt: sampled,
	})
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("deleted issue participation is not current")
	}
	return q.AdvanceIterationScopeRevision(ctx, db.AdvanceIterationScopeRevisionParams{WorkspaceID: issue.WorkspaceID, ID: issue.CurrentIterationID})
}
