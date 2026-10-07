package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *IterationService) prepareClosure(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, draft iteration.Draft, p *lifecyclePrepared, out *iteration.Preview, facts map[string]iteration.HistoricalIssue, lock bool) error {
	invalid := func(id *string, code string) {
		out.InvalidItems = append(out.InvalidItems, iteration.InvalidItem{IssueID: id, Code: code})
	}
	sourceID, _ := iterationUUID(draft.IterationID)
	source := p.rows[sourceID]
	if draft.Operation != "disable" && source.Status != "active" && !(draft.Operation == "cancel" && source.Status == "planned") {
		invalid(nil, "iteration_history_move_unsupported")
	}
	if source.Status == "planned" && len(draft.Moves) > 0 {
		invalid(nil, "iteration_move_set_invalid")
	}
	moves := map[string]iteration.Move{}
	for _, m := range draft.Moves {
		moves[m.IssueID] = m
	}
	choices := map[string]bool{}
	var next pgtype.UUID
	if draft.Start != nil {
		next, _ = iterationUUID(&draft.Start.TargetID)
		if next == sourceID || p.rows[next].Status != "planned" {
			invalid(nil, "iteration_active_conflict")
		}
		for _, c := range draft.Start.TerminalChoices {
			choices[c.IssueID] = c.Retain
		}
	}
	for _, issue := range p.issues {
		id := util.UUIDToString(issue.ID)
		category := facts[id].StatusCategory
		terminal := category == "done" || category == "cancelled"
		if issue.AdmissionStatus != "accepted" && issue.AdmissionStatus != "not_required" {
			invalid(&id, "triage_review_required")
			continue
		}
		var target db.Iteration
		rollover := false
		if draft.Operation == "handoff" && issue.CurrentIterationID == next {
			retain, chosen := choices[id]
			delete(choices, id)
			if terminal && !chosen {
				invalid(&id, "iteration_terminal_choice_required")
			}
			if !terminal && chosen {
				invalid(&id, "iteration_terminal_choice_invalid")
			}
			if !terminal || retain {
				continue
			}
		} else {
			if draft.Operation != "disable" && issue.CurrentIterationID != sourceID {
				invalid(&id, "iteration_move_set_invalid")
				continue
			}
			m, ok := moves[id]
			delete(moves, id)
			requireMove := source.Status == "active" && draft.Operation != "disable" && !terminal
			if requireMove && !ok {
				invalid(&id, "iteration_move_set_invalid")
				continue
			}
			if ok {
				expected, _ := iterationUUID(m.ExpectedSourceID)
				if !requireMove || m.AllowCompleted {
					invalid(&id, "iteration_move_set_invalid")
					continue
				}
				if issue.Revision != m.ExpectedIssueRevision || expected != sourceID {
					invalid(&id, "iteration_revision_conflict")
					continue
				}
				if m.TargetID != nil {
					tid, _ := iterationUUID(m.TargetID)
					target = p.rows[tid]
					if target.Status != "planned" || tid == sourceID {
						invalid(&id, "iteration_terminal_target_invalid")
						continue
					}
					if issue.IterationRolloverCount == 2147483647 {
						invalid(&id, "iteration_rollover_exhausted")
						continue
					}
					rollover = true
				}
			}
		}
		if lock {
			change, e := iteration.PrepareMembershipChange(ctx, tx, issue, p.rows[issue.CurrentIterationID], target, false)
			if e != nil {
				return e
			}
			change.Reason = iterationClosureReason(draft)
			change.Rollover = rollover
			p.changes = append(p.changes, change)
		}
	}
	for id := range moves {
		id := id
		invalid(&id, "iteration_move_set_invalid")
	}
	for id := range choices {
		id := id
		invalid(&id, "iteration_terminal_choice_invalid")
	}
	return nil
}

// Freeze before any release event changes the live scope. The payload owns all
// historical facts and never depends on future issue or reference mutations.
func (s *IterationService) freezeClosure(ctx context.Context, tx pgx.Tx, ws, actor, operationID pgtype.UUID, draft iteration.Draft, p *lifecyclePrepared) (*string, error) {
	draft.Reason = iterationClosureReason(draft)
	q := db.New(tx)
	var snapshotID *string
	for _, item := range p.preview.Iterations {
		row := p.rows[util.MustParseUUID(item.ID)]
		closeRow := draft.Operation == "disable" || (draft.IterationID != nil && item.ID == *draft.IterationID)
		if !closeRow || row.Status != "active" {
			continue
		}
		logical := p.sampled
		events, e := q.ListAllIterationEvents(ctx, db.ListAllIterationEventsParams{WorkspaceID: ws, IterationID: row.ID})
		if e != nil {
			return nil, e
		}
		for _, event := range events {
			if event.OccurredAt.Time.After(logical) {
				logical = event.OccurredAt.Time
			}
		}
		if row.StartedAt.Time.After(logical) {
			logical = row.StartedAt.Time
		}
		eventKind := "end"
		if draft.Operation == "cancel" {
			eventKind = "cancelled"
		}
		if e = appendLifecycleEvent(ctx, tx, ws, row.ID, actor, operationID, eventKind, nil, nil, draft.Reason, p.sampled); e != nil {
			return nil, e
		}
		history, e := iteration.LoadHistory(ctx, tx, ws, row.ID, logical)
		if e != nil {
			return nil, e
		}
		destinations := []iteration.Destination{}
		moves := map[string]*string{}
		for _, m := range draft.Moves {
			moves[m.IssueID] = m.TargetID
		}
		for _, issue := range history.Scope {
			target := moves[issue.IssueID]
			after := issue.RolloverCount
			if target != nil {
				if after == 2147483647 {
					return nil, errors.New("rollover count exhausted")
				}
				after++
			}
			destinations = append(destinations, iteration.Destination{IssueID: issue.IssueID, TargetIterationID: target, RolloverCountBefore: issue.RolloverCount, RolloverCountAfter: after})
		}
		endType := "completed"
		if draft.Operation == "cancel" {
			endType = "cancelled"
		}
		processed := logical
		snapshot, e := iteration.BuildSnapshot(history, util.UUIDToString(operationID), endType, *draft.Reason, logical, processed, destinations)
		if e != nil {
			return nil, e
		}
		p.snapshots = append(p.snapshots, snapshot)
		// Membership changes still use the already locked active row. Persist the
		// closing status now so handoff can later acquire the unique active slot.
		if _, e = q.CloseActiveIteration(ctx, db.CloseActiveIterationParams{WorkspaceID: ws, ID: row.ID, Status: endType, BusinessAt: pgtype.Timestamptz{Time: logical, Valid: true}, ProcessedAt: pgtype.Timestamptz{Time: processed, Valid: true}, Reason: iterationNullable(draft.Reason)}); e != nil {
			return nil, e
		}
		kind := "end"
		if endType == "cancelled" {
			kind = "cancel"
		}
		if draft.Operation != "disable" {
			if e = EnqueueIterationNotifications(ctx, q, ws, row.ID, operationID, kind, p.preview.Recipients); e != nil {
				return nil, e
			}
		}
		id := item.ID
		snapshotID = &id
	}
	return snapshotID, nil
}

func captureStartedOriginals(ctx context.Context, tx pgx.Tx, ws, iid, actor, operationID pgtype.UUID, sampled time.Time) error {
	q := db.New(tx)
	issues, e := q.ListIterationOperationIssues(ctx, db.ListIterationOperationIssuesParams{WorkspaceID: ws, IterationIds: []pgtype.UUID{iid}, IssueIds: []pgtype.UUID{}})
	if e != nil {
		return e
	}
	refs, e := iteration.CaptureIssueReferences(ctx, tx, ws, issues, true)
	if e != nil {
		return e
	}
	for i, issue := range issues {
		h := refs[i].Issue
		h.WasCompletedAtStart = h.StatusCategory == "done"
		part, e := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID})
		if e != nil {
			return e
		}
		original := iteration.OriginalFacts{HistoricalIssue: h, HasStarted: part.HasStartedCurrentParticipation || h.StatusCategory == "in_progress" || h.StatusCategory == "in_review" || h.StatusCategory == "done"}
		raw, e := json.Marshal(original)
		if e != nil {
			return e
		}
		n, e := q.CaptureIterationOriginal(ctx, db.CaptureIterationOriginalParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID, OriginalFacts: raw, HasStarted: original.HasStarted})
		if e != nil {
			return e
		}
		if n != 1 {
			return errors.New("invalid original commitment")
		}
		if e = q.AppendIterationIssueEvent(ctx, db.AppendIterationIssueEventParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID, OperationID: operationID, Kind: "baseline", Actor: iterationActor(actor), SampledAt: pgtype.Timestamptz{Time: sampled, Valid: true}, BeforeFacts: []byte("null"), AfterFacts: raw}); e != nil {
			return e
		}
	}
	return nil
}

func requireIterationAdministrator(ctx context.Context, tx pgx.Tx, ws, actor pgtype.UUID) error {
	member, e := db.New(tx).GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: ws, UserID: actor})
	if e != nil {
		return e
	}
	if member.Role != "owner" && member.Role != "admin" {
		return iterationFailure(403, "forbidden", "Only workspace administrators may disable iterations")
	}
	return nil
}

// Persist once, after all membership and handoff writes. No UPDATE ever touches
// a stored snapshot; only the owning transaction can see the staged closure.
func (s *IterationService) persistClosureSnapshots(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, p *lifecyclePrepared) error {
	if len(p.snapshots) == 0 {
		return nil
	}
	processed, e := s.lifecycleTime(ctx, tx)
	if e != nil {
		return e
	}
	q := db.New(tx)
	for _, snapshot := range p.snapshots {
		snapshot.ProcessedAt = processed
		if snapshot.ProcessedAt.Before(snapshot.LogicalEndedAt) {
			snapshot.ProcessedAt = snapshot.LogicalEndedAt
		}
		raw, e := json.Marshal(snapshot)
		if e != nil {
			return e
		}
		iid := util.MustParseUUID(snapshot.IterationID)
		sample := pgtype.Timestamptz{Time: snapshot.ProcessedAt, Valid: true}
		if e = q.FinalizeIterationProcessedAt(ctx, db.FinalizeIterationProcessedAtParams{WorkspaceID: ws, ID: iid, ProcessedAt: sample}); e != nil {
			return e
		}
		if e = q.InsertIterationSnapshot(ctx, db.InsertIterationSnapshotParams{WorkspaceID: ws, IterationID: iid, OperationID: util.MustParseUUID(snapshot.OperationID), Body: raw, CreatedAt: sample}); e != nil {
			return e
		}
	}
	return nil
}

func iterationClosureReason(draft iteration.Draft) *string {
	if draft.Operation != "disable" {
		return draft.Reason
	}
	reason := "功能禁用"
	if draft.Reason != nil && *draft.Reason != "" {
		reason += ": " + *draft.Reason
	}
	return &reason
}
