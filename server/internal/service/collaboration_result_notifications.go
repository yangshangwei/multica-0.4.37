package service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Called only after the guarded group transition returns a newly settled row.
// The source transaction serializes participants and freezes its recipients;
// the outbox later retries delivery without recreating these notifications.
func recordCollaborationGroupResult(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, group db.CollaborationExecutionGroup, runs []db.CollaborationRun) error {
	kind := ""
	switch group.State {
	case "succeeded":
		kind = "ai_completed"
	case "failed":
		kind = "ai_failed"
	case "stopped", "cancelled":
		kind = "ai_stopped"
	default:
		return invalidCollaborationLedger("unsettled group result")
	}
	var representative pgtype.UUID
	var lastTurn int64
	for _, run := range runs {
		if (run.StepKind.String == "agent" || run.StepKind.String == "leader" || run.StepKind.String == "continuation") && run.LeaderTurn.Valid && run.LeaderTurn.Int64 > lastTurn {
			representative, lastTurn = run.ID, run.LeaderTurn.Int64
		}
	}
	if !representative.Valid {
		return invalidCollaborationLedger("group result has no representative run")
	}
	seq, err := q.AdvanceCollaborationEventSequence(ctx, access.Issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		GroupID pgtype.UUID `json:"group_id"`
		RunID   pgtype.UUID `json:"run_id"`
		State   string      `json:"state"`
	}{group.ID, representative, group.State})
	if err != nil {
		return err
	}
	if _, err = q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: access.Issue.ID, EventSeq: seq, Kind: kind, Payload: payload}); err != nil {
		return err
	}
	participants, err := collaborationMentionParticipants(ctx, q, access)
	if err != nil {
		return err
	}
	// The result is a system observation. Neither an input author, resource
	// owner nor a prior Stop operator impersonates its human actor.
	for _, participant := range participants {
		if err = q.CreateCollaborationResultNotification(ctx, db.CreateCollaborationResultNotificationParams{RecipientUserID: participant.UserID, IssueID: access.Issue.ID, EventSeq: seq, Kind: kind, RunID: representative}); err != nil {
			return err
		}
	}
	return nil
}
