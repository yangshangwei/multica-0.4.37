package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type collaborationWakeCause struct {
	SourceKind  string `json:"source_kind"`
	SourceID    string `json:"source_id"`
	SourceRunID string `json:"source_run_id"`
	PendingID   string `json:"pending_id,omitempty"`
}

// The aggregate locks serialize source facts and continuation assignment. A
// proof cause never contains caller-supplied business text.
func (r *collaborationRuntimeState) recordPendingWakeCause(ctx context.Context, kind string, source pgtype.UUID) error {
	_, err := r.writeWakeCause(ctx, r.run.ID, "wakeup_pending", collaborationWakeCause{SourceKind: kind, SourceID: handoffID(source), SourceRunID: handoffID(r.run.ID)})
	return err
}
func (r *collaborationRuntimeState) recordAssignedWakeCause(ctx context.Context, target pgtype.UUID, kind string, source pgtype.UUID) error {
	if !target.Valid {
		return nil
	}
	_, err := r.writeWakeCause(ctx, target, "wakeup_assigned", collaborationWakeCause{SourceKind: kind, SourceID: handoffID(source), SourceRunID: handoffID(r.run.ID)})
	return err
}
func (r *collaborationRuntimeState) writeWakeCause(ctx context.Context, target pgtype.UUID, kind string, cause collaborationWakeCause) (db.CollaborationRunEvent, error) {
	key := kind + ":" + cause.SourceKind + ":" + cause.SourceID
	prior, err := r.q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: target, ProducerEventID: key})
	if err == nil {
		var saved collaborationWakeCause
		if prior.Kind != kind || json.Unmarshal(prior.Payload, &saved) != nil || saved != cause {
			return prior, invalidCollaborationLedger("wakeup source conflict")
		}
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return prior, err
	}
	payload, err := json.Marshal(cause)
	if err != nil {
		return prior, err
	}
	return r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: target, EventID: key, Kind: kind, Payload: payload})
}
func (r *collaborationRuntimeState) assignPendingWakeCauses(ctx context.Context, target pgtype.UUID) error {
	for {
		rows, err := r.q.ListCollaborationPendingWakeCauses(ctx, r.group.ID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			var cause collaborationWakeCause
			if json.Unmarshal(row.Payload, &cause) != nil || cause.SourceRunID != handoffID(row.RunID) {
				return invalidCollaborationLedger("invalid pending wakeup source")
			}
			cause.PendingID = handoffID(row.ID)
			if _, err := r.writeWakeCause(ctx, target, "wakeup_assigned", cause); err != nil {
				return err
			}
		}
	}
}
func projectHandoffCauses(ctx context.Context, q *db.Queries, a CollaborationIssueAccess, run db.CollaborationRun, c collaborationHandoffCursor, out *protocol.CollaborationHandoffEnvelope) error {
	total, err := q.CountCollaborationHandoffCauses(ctx, db.CountCollaborationHandoffCausesParams{IssueID: a.Issue.ID, RunID: run.ID})
	if err != nil {
		return err
	}
	out.WakeupCauses.Total = int(total)
	after := int64(0)
	if c.Kind == "causes" {
		after = int64(c.Offset)
	}
	rows, err := q.ListCollaborationHandoffCauses(ctx, db.ListCollaborationHandoffCausesParams{IssueID: a.Issue.ID, RunID: run.ID, Seq: after, Limit: 33})
	if err != nil {
		return err
	}
	if len(rows) > 32 {
		rows = rows[:32]
		out.WakeupCauses.Truncated = true
		out.WakeupCauses.NextCursor = handoffCursor(collaborationHandoffCursor{Kind: "causes", RunID: out.RunID, Offset: int(rows[len(rows)-1].Seq)})
	}
	for _, row := range rows {
		var cause collaborationWakeCause
		if json.Unmarshal(row.Payload, &cause) != nil {
			return invalidCollaborationLedger("invalid assigned wakeup source")
		}
		out.WakeupCauses.Items = append(out.WakeupCauses.Items, protocol.CollaborationHandoffCause{ID: handoffID(row.ID), SourceKind: cause.SourceKind, SourceID: cause.SourceID, SourceRunID: cause.SourceRunID, Sequence: handoffDecimal(row.Seq)})
	}
	out.WakeupCauses.Returned = len(rows)
	if collaborationWakeCausesRecorded(run) {
		out.WakeupCauses.Completeness = "recorded"
	}
	return nil
}

// collaborationRecordedContinuationPrefix marks continuations queued after
// every wake cause began to be assigned under the group lock. step_key is
// immutable, so the marker cannot be added to an older continuation.
const collaborationRecordedContinuationPrefix = "continuation:recorded:"

// Only continuations carry causes. One queued before cause recording existed
// may hold just the causes assigned after an upgrade, so its inventory stays
// unknown rather than claiming a history it never captured.
func collaborationWakeCausesRecorded(run db.CollaborationRun) bool {
	return run.StepKind.String != "continuation" || strings.HasPrefix(run.StepKey.String, collaborationRecordedContinuationPrefix)
}
