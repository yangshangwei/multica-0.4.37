package service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationCommandCancellationEvent struct {
	CommandID        pgtype.UUID `json:"command_id"`
	TargetID         pgtype.UUID `json:"target_id"`
	ChannelID        pgtype.UUID `json:"channel_id"`
	BatchID          pgtype.UUID `json:"batch_id"`
	GroupID          pgtype.UUID `json:"group_id"`
	CommandSequence  int64       `json:"command_sequence,string"`
	Reason           string      `json:"reason"`
	CancellationKind string      `json:"cancellation_kind"`
	Action           string      `json:"action,omitempty"`
	RequestID        string      `json:"request_id,omitempty"`
	Code             string      `json:"code"`
}

// Call only after the state transition, while the caller still owns its source,
// session and input locks. An event/link failure rolls back that transition too.
func (t *collaborationQueueTx) recordCommandCancellation(ctx context.Context, command db.CollaborationCommand) error {
	if command.SessionID != t.session.ID || command.State != "cancelled" || !command.Reason.Valid || command.CancellationEventID.Valid {
		return invalidCollaborationLedger("cancellation input identity invalid")
	}
	// Runtime authorization can carry an input author in t.actor. Attribution
	// comes only from the trusted request principal, never that proxy identity.
	var actor pgtype.UUID
	if principal, ok := collaboration.PrincipalFromContext(ctx); ok && principal.Human() {
		var err error
		actor, err = util.ParseUUID(principal.UserID)
		if err != nil {
			return invalidCollaborationLedger("cancellation principal invalid")
		}
	}
	kind := "automatic"
	if t.controlAction == "cancel_command" {
		if !actor.Valid || command.Reason.String != "cancelled_by_participant" {
			return invalidCollaborationLedger("manual cancellation principal invalid")
		}
		kind = "manual"
	}
	facts := collaborationCommandCancellationEvent{
		CommandID: command.ID, TargetID: command.TargetID, ChannelID: command.ChannelID, BatchID: command.BatchID,
		GroupID: command.GroupID, CommandSequence: command.Seq, Reason: command.Reason.String,
		CancellationKind: kind, Code: "command_cancelled",
	}
	if kind == "manual" {
		facts.Action, facts.RequestID = t.controlAction, t.requestID
	}
	payload, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	event, err := t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{
		IssueID: t.access.Issue.ID, EventSeq: seq, ActorUserID: actor, Kind: "command_cancelled", Payload: payload,
	})
	if err != nil {
		return err
	}
	_, err = t.q.LinkCollaborationCommandCancellation(ctx, db.LinkCollaborationCommandCancellationParams{
		SessionID: command.SessionID, CommandID: command.ID, EventID: event.ID,
	})
	return err
}

func (t *collaborationQueueTx) recordCommandCancellations(ctx context.Context, commands []db.CollaborationCommand) error {
	for _, command := range commands {
		if command.State == "cancelled" {
			if err := t.recordCommandCancellation(ctx, command); err != nil {
				return err
			}
		}
	}
	return nil
}
