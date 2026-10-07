package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const MaxIterationOperationIssues = 2000

type lifecyclePrepared struct {
	preview    iteration.Preview
	rows       map[pgtype.UUID]db.Iteration
	issues     []db.Issue
	historical []iteration.HistoricalIssue
	changes    []*iteration.MembershipChange
	sampled    time.Time
	snapshots  []iteration.Snapshot
}

func normalizeLifecycleDraft(draft iteration.Draft) (iteration.Draft, error) {
	raw, err := json.Marshal(draft)
	if err != nil {
		return draft, err
	}
	raw, err = iteration.CanonicalDraftJSON(raw)
	if err != nil {
		return draft, iterationFailure(400, "invalid_request", err.Error())
	}
	var normalized iteration.Draft
	if err = json.Unmarshal(raw, &normalized); err != nil {
		return draft, err
	}
	draft = normalized
	switch draft.Operation {
	case "start", "move", "delete", "end", "handoff", "disable", "cancel":
	default:
		return draft, iterationFailure(422, "iteration_validation_failed", "This lifecycle operation is not available")
	}
	if draft.Operation == "move" && (draft.IterationID != nil || draft.ExpectedIterationRevision != nil || draft.ExpectedScopeRevision != nil || len(draft.Moves) == 0) {
		return draft, iterationFailure(400, "invalid_request", "Move requires an explicit issue set and no iteration header")
	}
	if draft.Operation == "start" && draft.Start.TargetID != *draft.IterationID {
		return draft, iterationFailure(400, "invalid_request", "Start target must match iteration")
	}
	return draft, nil
}
func (s *IterationService) Preview(ctx context.Context, ws, actor pgtype.UUID, draft iteration.Draft, authorize func(context.Context, pgx.Tx) error) (iteration.Preview, error) {
	draft, err := normalizeLifecycleDraft(draft)
	if err != nil {
		return iteration.Preview{}, err
	}
	if s.TxStarter == nil || authorize == nil {
		return iteration.Preview{}, errors.New("preview requires transaction and current authorization")
	}
	for attempt := 0; attempt < 3; attempt++ {
		preview, err := s.previewIterationOnce(ctx, ws, actor, draft, authorize)
		if err == nil {
			return preview, nil
		}
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return iteration.Preview{}, err
		}
		if attempt < 2 {
			delay := []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}[attempt] + time.Duration(rand.IntN(26))*time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return iteration.Preview{}, ctx.Err()
			}
		}
	}
	return iteration.Preview{}, &iteration.OperationError{Status: 503, Code: "iteration_unavailable", Message: "Iteration preview conflicted; retry the read", Retryable: true}
}

// A locking read against an older RR snapshot may fail after a member revoke.
// Each fresh attempt repeats current authorization before reading protected facts.
func (s *IterationService) previewIterationOnce(ctx context.Context, ws, actor pgtype.UUID, draft iteration.Draft, authorize func(context.Context, pgx.Tx) error) (iteration.Preview, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return iteration.Preview{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ WRITE"); err != nil {
		return iteration.Preview{}, err
	}
	q := db.New(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
		return iteration.Preview{}, err
	}
	if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		return iteration.Preview{}, err
	}
	if _, err = q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: actor}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = iterationFailure(403, "forbidden", "Workspace membership required")
		}
		return iteration.Preview{}, err
	}
	if err = authorize(ctx, tx); err != nil {
		return iteration.Preview{}, err
	}
	prepared, err := s.prepareLifecycle(ctx, tx, ws, actor, draft, false, "")
	if err != nil {
		return iteration.Preview{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return iteration.Preview{}, err
	}
	return prepared.preview, nil
}
func (s *IterationService) prepareLifecycle(ctx context.Context, tx pgx.Tx, ws, actor pgtype.UUID, draft iteration.Draft, lock bool, expectedHash string) (lifecyclePrepared, error) {
	if draft.Operation == "disable" {
		if e := requireIterationAdministrator(ctx, tx, ws, actor); e != nil {
			return lifecyclePrepared{}, e
		}
	}

	p := lifecyclePrepared{rows: map[pgtype.UUID]db.Iteration{}}
	q := db.New(tx)
	settings, err := s.lifecycleSettings(ctx, tx, ws, lock)
	if err != nil {
		return p, err
	}
	out := iteration.Preview{WorkspaceID: util.UUIDToString(ws), ActorUserID: util.UUIDToString(actor), Draft: draft, Iterations: []iteration.Iteration{}, Issues: []iteration.PreviewIssue{}, Statistics: map[string]iteration.Statistics{}, Recipients: []string{}, InvalidItems: []iteration.InvalidItem{}, Complete: true}
	invalid := func(id *string, code string) {
		out.InvalidItems = append(out.InvalidItems, iteration.InvalidItem{IssueID: id, Code: code})
	}
	if draft.ExpectedSettingsRevision != settings.Revision {
		invalid(nil, "iteration_revision_conflict")
	}
	iterationSet := map[pgtype.UUID]bool{}
	issueIDs := []pgtype.UUID{}
	fullIDs := []pgtype.UUID{}
	addIteration := func(value *string) {
		if value != nil {
			id, _ := iterationUUID(value)
			iterationSet[id] = true
		}
	}
	addIteration(draft.IterationID)
	if draft.Start != nil {
		addIteration(&draft.Start.TargetID)
		if draft.Operation == "handoff" {
			id, _ := iterationUUID(&draft.Start.TargetID)
			fullIDs = append(fullIDs, id)
		}
	}
	if draft.Operation == "disable" {
		all, e := q.ListOpenIterations(ctx, ws)
		if e != nil {
			return p, e
		}
		for _, row := range all {
			iterationSet[row.ID] = true
			fullIDs = append(fullIDs, row.ID)
		}
	}
	if draft.IterationID != nil {
		id, _ := iterationUUID(draft.IterationID)
		fullIDs = append(fullIDs, id)
	}
	for _, move := range draft.Moves {
		id, _ := iterationUUID(&move.IssueID)
		issueIDs = append(issueIDs, id)
		addIteration(move.ExpectedSourceID)
		addIteration(move.TargetID)
	}
	// The owner fence excludes membership changes; preview uses its RR snapshot.
	// Include actual source IDs so a stale request cannot hide a changed source.
	if len(issueIDs) > 0 {
		actual, err := q.ListIssueDeleteIterationIDs(ctx, db.ListIssueDeleteIterationIDsParams{WorkspaceID: ws, IssueIds: issueIDs})
		if err != nil {
			return p, err
		}
		for _, id := range actual {
			iterationSet[id] = true
		}
	}
	ids := make([]pgtype.UUID, 0, len(iterationSet))
	for id := range iterationSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return util.UUIDToString(ids[i]) < util.UUIDToString(ids[j]) })
	rows := []db.Iteration{}
	if lock {
		rows, err = q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: ws, Column2: ids})
		if err != nil {
			return p, err
		}
	} else {
		for _, id := range ids {
			row, e := q.GetIteration(ctx, db.GetIterationParams{WorkspaceID: ws, ID: id})
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			if e != nil {
				return p, e
			}
			rows = append(rows, row)
		}
	}
	if len(rows) != len(ids) {
		return p, iterationFailure(404, "iteration_not_found", "Iteration not found")
	}
	for _, row := range rows {
		p.rows[row.ID] = row
		out.Iterations = append(out.Iterations, iteration.IterationFromRow(row))
	}
	if lock {
		p.issues, err = q.LockIterationOperationIssues(ctx, db.LockIterationOperationIssuesParams{WorkspaceID: ws, IterationIds: fullIDs, IssueIds: issueIDs})
	} else {
		p.issues, err = q.ListIterationOperationIssues(ctx, db.ListIterationOperationIssuesParams{WorkspaceID: ws, IterationIds: fullIDs, IssueIds: issueIDs})
	}
	if err != nil {
		return p, err
	}
	if len(p.issues) > MaxIterationOperationIssues || len(issueIDs) > MaxIterationOperationIssues {
		return p, iterationFailure(413, "iteration_operation_too_large", "Operation exceeds the complete-set limit")
	}
	if len(p.issues) > 0 {
		if s.AuthorizeIssues == nil {
			return p, errors.New("issue authorization callback is required")
		}
		if err = s.AuthorizeIssues(ctx, tx, p.issues); err != nil {
			return p, err
		}
	}
	references, err := iteration.CaptureIssueReferences(ctx, tx, ws, p.issues, lock)
	if err != nil {
		return p, err
	}
	p.historical = make([]iteration.HistoricalIssue, len(references))
	for i, reference := range references {
		p.historical[i] = reference.Issue
	}
	found := map[string]db.Issue{}
	facts := map[string]iteration.HistoricalIssue{}
	recipients := map[string]bool{}
	for _, row := range rows {
		if row.CoordinatorUserID.Valid {
			var member db.Member
			var e error
			if lock {
				member, e = q.LockIterationReferenceMember(ctx, db.LockIterationReferenceMemberParams{UserID: row.CoordinatorUserID, WorkspaceID: ws})
			} else {
				member, e = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: row.CoordinatorUserID, WorkspaceID: ws})
			}
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return p, e
			}
			if e == nil && member.UserID.Valid {
				recipients[util.UUIDToString(member.UserID)] = true
			}
		}
	}
	for i, issue := range p.issues {
		id := util.UUIDToString(issue.ID)
		found[id] = issue
		historical := p.historical[i]
		facts[id] = historical
		running := int64(0)
		tasks, e := q.ListActiveTasksByIssue(ctx, issue.ID)
		if e != nil {
			return p, e
		}
		for _, task := range tasks {
			if task.Status == "running" {
				running++
			}
		}
		projectType := "project"
		out.Issues = append(out.Issues, iteration.PreviewIssue{IssueID: id, Identifier: historical.Identifier, Revision: issue.Revision, SourceID: iterationString(issue.CurrentIterationID), Title: issue.Title, StatusCategory: historical.StatusCategory, Project: iteration.PreviewReference{ID: historical.ProjectID, Name: historical.ProjectName, Type: &projectType, Available: references[i].ProjectAvailable}, Assignee: iteration.PreviewReference{ID: historical.AssigneeID, Name: historical.AssigneeName, Type: historical.AssigneeType, Available: references[i].AssigneeAvailable}, RunningExecutionCount: running, RolloverCount: issue.IterationRolloverCount})
		if historical.AssigneeType != nil && *historical.AssigneeType == "member" && historical.AssigneeID != nil && historical.AssigneeName != nil {
			recipients[*historical.AssigneeID] = true
		}
	}
	for recipient := range recipients {
		out.Recipients = append(out.Recipients, recipient)
	}
	sort.Strings(out.Recipients)
	if draft.IterationID != nil {
		id, _ := iterationUUID(draft.IterationID)
		row := p.rows[id]
		if row.Revision != *draft.ExpectedIterationRevision || row.ScopeRevision != *draft.ExpectedScopeRevision {
			invalid(nil, "iteration_revision_conflict")
		}
		if (draft.Operation == "start" || draft.Operation == "delete") && row.Status != "planned" {
			invalid(nil, "iteration_history_move_unsupported")
		}
	}
	switch draft.Operation {
	case "move":
		for _, move := range draft.Moves {
			issue, ok := found[move.IssueID]
			if !ok {
				invalid(&move.IssueID, "issue_not_found")
				continue
			}
			source, _ := iterationUUID(move.ExpectedSourceID)
			target, _ := iterationUUID(move.TargetID)
			if issue.Revision != move.ExpectedIssueRevision || issue.CurrentIterationID != source {
				invalid(&move.IssueID, "iteration_revision_conflict")
				continue
			}
			if source.Valid && source != target && (draft.Reason == nil || *draft.Reason == "") {
				invalid(&move.IssueID, "iteration_reason_required")
				continue
			}
			if issue.AdmissionStatus != "accepted" && issue.AdmissionStatus != "not_required" {
				invalid(&move.IssueID, "triage_review_required")
				continue
			}
			targetRow := p.rows[target]
			sourceRow := p.rows[source]
			if (source.Valid && sourceRow.Status != "planned" && sourceRow.Status != "active") || (target.Valid && targetRow.Status != "planned" && targetRow.Status != "active") {
				invalid(&move.IssueID, "iteration_history_move_unsupported")
				continue
			}
			category := facts[move.IssueID].StatusCategory
			if target.Valid && target != source && (category == "cancelled" || (category == "done" && (targetRow.Status != "active" || !move.AllowCompleted))) {
				invalid(&move.IssueID, "iteration_terminal_target_invalid")
				continue
			}
			if lock {
				change, e := iteration.PrepareMembershipChange(ctx, tx, issue, sourceRow, targetRow, move.AllowCompleted)
				if e != nil {
					return p, e
				}
				change.Reason = draft.Reason
				p.changes = append(p.changes, change)
			}
		}
	case "start":
		if _, e := q.GetIterationActive(ctx, ws); e == nil {
			invalid(nil, "iteration_active_conflict")
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return p, e
		}
		choices := map[string]bool{}
		for _, choice := range draft.Start.TerminalChoices {
			choices[choice.IssueID] = choice.Retain
		}
		for _, issue := range p.issues {
			id := util.UUIDToString(issue.ID)
			category := facts[id].StatusCategory
			terminal := category == "done" || category == "cancelled"
			retain, chosen := choices[id]
			if terminal && !chosen {
				invalid(&id, "iteration_terminal_choice_required")
			}
			if !terminal && chosen {
				invalid(&id, "iteration_terminal_choice_invalid")
			}
			delete(choices, id)
			if issue.AdmissionStatus != "accepted" && issue.AdmissionStatus != "not_required" {
				invalid(&id, "triage_review_required")
			}
			if lock {
				source := p.rows[issue.CurrentIterationID]
				if terminal && chosen && !retain {
					change, e := iteration.PrepareMembershipChange(ctx, tx, issue, source, db.Iteration{}, false)
					if e != nil {
						return p, e
					}
					change.Reason = draft.Reason
					p.changes = append(p.changes, change)
				} else {
					part, e := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: ws, IterationID: source.ID, IssueID: issue.ID})
					if e != nil {
						return p, e
					}
					if !part.CurrentJoinedAt.Valid || part.InOriginal {
						return p, errors.New("invalid planned participation")
					}
				}
			}
		}
		for id := range choices {
			id := id
			invalid(&id, "iteration_terminal_choice_invalid")
		}
	case "end", "handoff", "disable", "cancel":
		if e := s.prepareClosure(ctx, tx, ws, draft, &p, &out, facts, lock); e != nil {
			return p, e
		}
		if draft.Reason == nil || *draft.Reason == "" {
			invalid(nil, "iteration_reason_required")
		}
	case "delete":
		id, _ := iterationUUID(draft.IterationID)
		parts, e := q.ListIterationParticipations(ctx, db.ListIterationParticipationsParams{WorkspaceID: ws, IterationID: id})
		if e != nil {
			return p, e
		}
		events, e := q.ListAllIterationEvents(ctx, db.ListAllIterationEventsParams{WorkspaceID: ws, IterationID: id})
		if e != nil {
			return p, e
		}
		hasIssueActivity := false
		for _, event := range events {
			hasIssueActivity = hasIssueActivity || event.IssueID.Valid
		}
		if len(parts) > 0 || len(p.issues) > 0 || p.rows[id].StartedAt.Valid || hasIssueActivity {
			invalid(nil, "iteration_delete_requires_unused_plan")
		}
	}
	p.sampled, err = s.lifecycleTime(ctx, tx)
	if err != nil {
		return p, err
	}
	out.PreviewedAt = p.sampled
	if draft.Operation == "start" || draft.Operation == "handoff" {
		id, _ := iterationUUID(&draft.Start.TargetID)
		row := p.rows[id]
		dates, e := iteration.StartDates(row.StartDate.Time.Format(time.DateOnly), row.EndDate.Time.Format(time.DateOnly), row.Timezone, draft.Start.Mode, p.sampled)
		if e != nil {
			invalid(nil, "iteration_start_dates_invalid")
		} else {
			out.StartPreview = &dates
		}
	}

	out.TotalAffected = len(p.issues)
	comparisonIssues := append([]iteration.PreviewIssue{}, out.Issues...)
	for i := range comparisonIssues {
		comparisonIssues[i].RunningExecutionCount = 0
	}
	out.PreviewHash, err = iteration.CanonicalHash(struct {
		Historical       []iteration.HistoricalIssue `json:"historical"`
		Version          int                         `json:"contract_version"`
		Workspace        string                      `json:"workspace_id"`
		Actor            string                      `json:"actor_user_id"`
		Draft            iteration.Draft             `json:"draft"`
		SettingsRevision int64                       `json:"settings_revision"`
		Iterations       []iteration.Iteration       `json:"iterations"`
		Issues           []iteration.PreviewIssue    `json:"issues"`
		Recipients       []string                    `json:"recipients"`
		Start            *iteration.StartDatePreview `json:"start_preview"`
		Invalid          []iteration.InvalidItem     `json:"invalid_items"`
	}{p.historical, iteration.SchemaVersion, out.WorkspaceID, out.ActorUserID, draft, settings.Revision, out.Iterations, comparisonIssues, out.Recipients, out.StartPreview, out.InvalidItems})
	if err != nil {
		return p, err
	}
	if expectedHash != "" && out.PreviewHash != expectedHash {
		return p, iterationFailure(409, "iteration_preview_stale", "Preview facts changed; refresh the complete preview")
	}
	for _, row := range rows {
		history, e := iteration.LoadHistory(ctx, tx, ws, row.ID, p.sampled)
		if e != nil {
			return p, e
		}
		out.Statistics[util.UUIDToString(row.ID)] = history.Statistics
	}
	p.preview = out
	return p, nil
}
