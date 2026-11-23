package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// Member binding for the parent-task budget root.
//
// The three functions below are called from the issue write paths, inside the
// caller's transaction and under the iteration:<workspace> fence the caller
// already holds. They write member and observation rows only. They read the
// root's mode without locking or updating it: an issue writer has blocking statements
// ahead of it, and a root lock taken here could not be the last lock of its
// transaction.
//
// A member binding is never rewritten once it exists. A failure here fails the
// business write it belongs to; there is no branch that swallows it, because a
// child that exists without its member row is the state this code prevents.
//
// Two rules hold for every transaction that binds (design sections 5 and
// 10.10):
//
//   - It is READ COMMITTED. The fence and the binding lock close only for a
//     statement that sees what committed while the transaction waited for
//     them. The create and reparent functions check the level once they have
//     found a member row and fail the write otherwise; a tree without a root
//     is never asked.
//   - Giving an existing issue its first member row (enabling a root, moving
//     an unbound issue under a member) takes the workspace binding lock,
//     exclusive, before the first member row is written. A claim reads its
//     member row under the shared side of that lock, so no claim can read
//     "unbound" and commit unseen while a binding is on its way. Creating a
//     child does not take it: no daemon sees a task of an issue whose
//     creating transaction has not committed, and by then the member row is
//     there.

// issueBudgetBindingVersion is the version of the binding rules in this file,
// stored on every member row they write.
const issueBudgetBindingVersion int32 = 1

// Sources of a member row.
const (
	issueBudgetBindRoot       = "root"
	issueBudgetBindActivation = "activation"
	issueBudgetBindCreate     = "create"
	issueBudgetBindReparent   = "reparent"
)

// Observation kinds and the reasons this file and the management service use.
const (
	issueBudgetObservationWouldRefuse           = "would_refuse"
	issueBudgetObservationCoverageGap           = "coverage_gap"
	issueBudgetObservationAttributionDifference = "attribution_difference"

	issueBudgetReasonRootConflict            = "root_conflict"
	issueBudgetReasonParentCycle             = "parent_cycle"
	issueBudgetReasonInFlightLegacy          = "in_flight_legacy"
	issueBudgetReasonInFlightAtBind          = "in_flight_at_bind"
	issueBudgetReasonObservationOff          = "observation_off"
	issueBudgetReasonQuickCreateRootMismatch = "quick_create_root_mismatch"
)

// ErrIssueBudgetIsolation reports a budget seam reached from a transaction that
// is not READ COMMITTED. The write it belongs to fails.
var ErrIssueBudgetIsolation = errors.New("budget seam reached from a transaction that is not READ COMMITTED")

// issueBudgetReadCommitted is the one level a transaction at a budget seam may
// have, as current_setting('transaction_isolation') spells it.
const issueBudgetReadCommitted = "read committed"

// requireIssueBudgetReadCommitted fails a write that reached a budget seam from
// a transaction whose snapshot was fixed before its locks were granted: under
// REPEATABLE READ the member rows and the claims that committed while it
// waited stay invisible to it, and it would bind from a stale picture. Callers
// ask only after they found a member row, so the question costs a tree without
// a root nothing, and code that arrives from the wrong level fails on the
// first tree that has a root, not only inside a race.
func requireIssueBudgetReadCommitted(ctx context.Context, q *db.Queries) error {
	level, err := q.GetIssueBudgetTransactionIsolation(ctx)
	if err != nil {
		return fmt.Errorf("read transaction isolation level: %w", err)
	}
	if level != issueBudgetReadCommitted {
		return fmt.Errorf("%w: it is %s", ErrIssueBudgetIsolation, level)
	}
	return nil
}

// BindIssueBudgetOnCreate gives a newly inserted issue the root of its parent.
// For an issue without a parent, or under a parent outside every root, it is
// one lookup on the member key and writes nothing.
func BindIssueBudgetOnCreate(ctx context.Context, q *db.Queries, issue db.Issue, verifiedTaskID pgtype.UUID) error {
	var boundRoot pgtype.UUID
	if issue.ParentIssueID.Valid {
		parent, bound, err := issueBudgetMember(ctx, q, issue.WorkspaceID, issue.ParentIssueID)
		if err != nil {
			return err
		}
		if bound {
			if err := requireIssueBudgetReadCommitted(ctx, q); err != nil {
				return err
			}
			member, err := q.InsertIssueBudgetMember(ctx, db.InsertIssueBudgetMemberParams{
				WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, BudgetRootID: parent.BudgetRootID,
				BoundParentIssueID: issue.ParentIssueID, BindSource: issueBudgetBindCreate, BindingVersion: issueBudgetBindingVersion,
			})
			if err != nil {
				return fmt.Errorf("bind created issue to budget root: %w", err)
			}
			boundRoot = member.BudgetRootID
		}
	}
	if !verifiedTaskID.Valid {
		return nil
	}
	attempt, err := q.GetLatestIssueBudgetAttemptForTask(ctx, db.GetLatestIssueBudgetAttemptForTaskParams{
		WorkspaceID: issue.WorkspaceID, TaskID: verifiedTaskID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read creating task's budget attempt: %w", err)
	}
	if attempt.IssueID.Valid || attempt.BudgetRootID == boundRoot {
		return nil
	}
	if err := requireIssueBudgetReadCommitted(ctx, q); err != nil {
		return err
	}
	observing, err := issueBudgetObserving(ctx, q, issue.WorkspaceID, attempt.BudgetRootID)
	if err != nil || !observing {
		return err
	}
	return insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
		WorkspaceID: issue.WorkspaceID, BudgetRootID: attempt.BudgetRootID,
		Kind: issueBudgetObservationAttributionDifference, Reason: issueBudgetReasonQuickCreateRootMismatch,
		IssueID: issue.ID, TaskID: verifiedTaskID, AttemptID: attempt.ID,
		ClaimGeneration: pgtype.Int8{Int64: attempt.ClaimGeneration, Valid: true},
	}, map[string]string{
		"created_issue_budget_root_id": util.UUIDToString(boundRoot),
		"context_parent_issue_id":      util.UUIDToString(attempt.ContextParentIssueID),
	})
}

// BindIssueBudgetOnReparent applies the budget rules to an issue whose parent
// was just changed; issue is the row after that update. Callers invoke it only
// when the parent really changed. It never refuses the move.
//
//   - The issue already has a member row: the row stays as written. Moving it
//     under a member of another root records a would-refuse on its own root.
//   - The issue has none and its new parent has one: the issue and every
//     descendant without a member row join the parent's root.
//   - Neither has one: nothing is written.
func BindIssueBudgetOnReparent(ctx context.Context, q *db.Queries, issue db.Issue) error {
	if !issue.ParentIssueID.Valid {
		// Cleared parent: an existing binding stays, and there is no parent
		// to inherit from.
		return nil
	}
	parent, parentBound, err := issueBudgetMember(ctx, q, issue.WorkspaceID, issue.ParentIssueID)
	if err != nil || !parentBound {
		// A parent outside every root changes nothing on either side.
		return err
	}
	if err := requireIssueBudgetReadCommitted(ctx, q); err != nil {
		return err
	}
	own, ownBound, err := issueBudgetMember(ctx, q, issue.WorkspaceID, issue.ID)
	if err != nil {
		return err
	}
	if ownBound {
		if own.BudgetRootID == parent.BudgetRootID {
			return nil
		}
		observing, err := issueBudgetObserving(ctx, q, issue.WorkspaceID, own.BudgetRootID)
		if err != nil || !observing {
			return err
		}
		return insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
			WorkspaceID:  issue.WorkspaceID,
			BudgetRootID: own.BudgetRootID,
			Kind:         issueBudgetObservationWouldRefuse,
			Reason:       issueBudgetReasonRootConflict,
			IssueID:      issue.ID,
		}, map[string]string{
			"new_parent_issue_id":       util.UUIDToString(issue.ParentIssueID),
			"new_parent_budget_root_id": util.UUIDToString(parent.BudgetRootID),
		})
	}

	descendants, err := q.ListIssueBudgetDescendants(ctx, db.ListIssueBudgetDescendantsParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err != nil {
		return fmt.Errorf("walk descendants of moved issue: %w", err)
	}
	issueIDs := []pgtype.UUID{issue.ID}
	parentIDs := []pgtype.UUID{issue.ParentIssueID}
	var foreign []db.ListIssueBudgetDescendantsRow
	cycle := false
	for _, row := range descendants {
		switch {
		case row.Cycle:
			// The walk came back to the moved issue. It is already in the
			// list; the loop is recorded and the move goes ahead.
			cycle = true
		case !row.MemberBudgetRootID.Valid:
			issueIDs = append(issueIDs, row.IssueID)
			parentIDs = append(parentIDs, row.ParentIssueID)
		case row.MemberBudgetRootID != parent.BudgetRootID:
			foreign = append(foreign, row)
		}
	}
	// From here on existing issues get their first member row. Claims that
	// already read "unbound" under the shared side of the lock commit before
	// it is granted, and the scan at the end finds their tasks; claims that
	// arrive later are turned away until this transaction ends.
	if err := q.LockIssueBudgetBinding(ctx, issue.WorkspaceID); err != nil {
		return fmt.Errorf("take budget binding lock: %w", err)
	}
	// This separate READ COMMITTED statement must follow the binding lock:
	// claims committed while we waited must be covered by this scan or by
	// the observation_off gap of a subsequent switch back to observe.
	observing, err := issueBudgetObserving(ctx, q, issue.WorkspaceID, parent.BudgetRootID)
	if err != nil {
		return err
	}
	if err := insertIssueBudgetMembers(ctx, q, issue.WorkspaceID, parent.BudgetRootID, issueBudgetBindReparent, issueIDs, parentIDs); err != nil {
		return err
	}
	if !observing {
		return nil
	}
	for _, row := range foreign {
		if err := insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
			WorkspaceID:  issue.WorkspaceID,
			BudgetRootID: parent.BudgetRootID,
			Kind:         issueBudgetObservationWouldRefuse,
			Reason:       issueBudgetReasonRootConflict,
			IssueID:      row.IssueID,
		}, map[string]string{
			"moved_issue_id":           util.UUIDToString(issue.ID),
			"new_parent_issue_id":      util.UUIDToString(issue.ParentIssueID),
			"member_of_budget_root_id": util.UUIDToString(row.MemberBudgetRootID),
		}); err != nil {
			return err
		}
	}
	if cycle {
		if err := insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
			WorkspaceID:  issue.WorkspaceID,
			BudgetRootID: parent.BudgetRootID,
			Kind:         issueBudgetObservationCoverageGap,
			Reason:       issueBudgetReasonParentCycle,
			IssueID:      issue.ID,
		}, map[string]string{
			"new_parent_issue_id": util.UUIDToString(issue.ParentIssueID),
		}); err != nil {
			return err
		}
	}
	tasks, err := listIssueBudgetInFlightTasks(ctx, q, issue.WorkspaceID, issueIDs)
	if err != nil {
		return err
	}
	return recordIssueBudgetInFlightGaps(ctx, q, issue.WorkspaceID, parent.BudgetRootID, issueBudgetReasonInFlightAtBind, tasks)
}

// issueBudgetObserving reads the mode without introducing a root lock into an
// issue writer. Missing or invalid roots fail the enclosing business write.
func issueBudgetObserving(ctx context.Context, q *db.Queries, workspaceID, rootID pgtype.UUID) (bool, error) {
	root, err := q.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: workspaceID, ID: rootID})
	if err != nil {
		return false, fmt.Errorf("read budget observation mode: %w", err)
	}
	switch root.Mode {
	case IssueBudgetModeObserve:
		return true, nil
	case IssueBudgetModeOff:
		return false, nil
	default:
		return false, fmt.Errorf("invalid budget observation mode")
	}
}

// TombstoneIssueBudgetMember marks the member row of an issue that is about to
// be deleted. The row, its root and everything recorded under them stay. For
// an issue outside every root it is one update that matches nothing.
func TombstoneIssueBudgetMember(ctx context.Context, q *db.Queries, workspaceID, issueID pgtype.UUID) error {
	if _, err := q.TombstoneIssueBudgetMember(ctx, db.TombstoneIssueBudgetMemberParams{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
	}); err != nil {
		return fmt.Errorf("tombstone budget member: %w", err)
	}
	return nil
}

// issueBudgetMember reads the member row of an issue. bound is false when the
// issue belongs to no root.
func issueBudgetMember(ctx context.Context, q *db.Queries, workspaceID, issueID pgtype.UUID) (member db.IssueBudgetMember, bound bool, err error) {
	member, err = q.GetIssueBudgetMember(ctx, db.GetIssueBudgetMemberParams{WorkspaceID: workspaceID, IssueID: issueID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.IssueBudgetMember{}, false, nil
	}
	if err != nil {
		return db.IssueBudgetMember{}, false, fmt.Errorf("read budget member: %w", err)
	}
	return member, true, nil
}

// insertIssueBudgetMembers binds issueIDs to one root in a single statement.
// boundParentIDs holds, position by position, the parent each binding was
// inherited through.
func insertIssueBudgetMembers(ctx context.Context, q *db.Queries, workspaceID, rootID pgtype.UUID, source string, issueIDs, boundParentIDs []pgtype.UUID) error {
	if len(issueIDs) == 0 {
		return nil
	}
	if len(issueIDs) != len(boundParentIDs) {
		return fmt.Errorf("bind budget members: %d issues with %d parents", len(issueIDs), len(boundParentIDs))
	}
	inserted, err := q.InsertIssueBudgetMembers(ctx, db.InsertIssueBudgetMembersParams{
		WorkspaceID:         workspaceID,
		BudgetRootID:        rootID,
		BindSource:          source,
		BindingVersion:      issueBudgetBindingVersion,
		IssueIds:            issueIDs,
		BoundParentIssueIds: boundParentIDs,
	})
	if err != nil {
		return fmt.Errorf("bind budget members: %w", err)
	}
	if inserted != int64(len(issueIDs)) {
		return fmt.Errorf("bind budget members: wrote %d of %d rows", inserted, len(issueIDs))
	}
	return nil
}

// listIssueBudgetInFlightTasks reads, for the issues a binding covers, every
// task a daemon already holds. Such a task was claimed before its issue was a
// member, so no attempt exists for that claim.
//
// It is only meaningful as a statement sent after the binding lock was
// granted: every claim that read "unbound" has committed by then, so its task
// is in the result.
//
// The query's quick-create branch proposes candidates by the parent saved in
// their context. Whether a candidate is a quick-create task is decided here,
// by the definition every other reader of such a task uses.
func listIssueBudgetInFlightTasks(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, issueIDs []pgtype.UUID) ([]db.ListIssueBudgetInFlightTasksRow, error) {
	rows, err := q.ListIssueBudgetInFlightTasks(ctx, db.ListIssueBudgetInFlightTasksParams{
		WorkspaceID: workspaceID,
		IssueIds:    issueIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("read in-flight tasks: %w", err)
	}
	tasks := make([]db.ListIssueBudgetInFlightTasksRow, 0, len(rows))
	for _, row := range rows {
		if row.QuickCreate {
			// The branch selects tasks without an issue.
			if _, ok := quickCreateTaskContext(pgtype.UUID{}, row.ChatSessionID, row.AutopilotRunID, row.Context); !ok {
				continue
			}
		}
		tasks = append(tasks, row)
	}
	return tasks, nil
}

// recordIssueBudgetInFlightGaps writes one coverage gap for every task the
// scan found. The observation carries the task and the claim generation, and
// counts as covered once an attempt for the same pair turns up.
func recordIssueBudgetInFlightGaps(ctx context.Context, q *db.Queries, workspaceID, rootID pgtype.UUID, reason string, tasks []db.ListIssueBudgetInFlightTasksRow) error {
	for _, task := range tasks {
		detail := map[string]string{"task_status": task.Status}
		if task.QuickCreate {
			// The task has no issue of its own; issue_id is the parent
			// saved in its context.
			detail["attributed_through"] = "quick_create_context_parent"
		}
		if err := insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
			WorkspaceID:     workspaceID,
			BudgetRootID:    rootID,
			Kind:            issueBudgetObservationCoverageGap,
			Reason:          reason,
			IssueID:         task.IssueID,
			TaskID:          task.TaskID,
			ClaimGeneration: pgtype.Int8{Int64: task.ClaimGeneration, Valid: true},
		}, detail); err != nil {
			return err
		}
	}
	return nil
}

// insertIssueBudgetObservation appends one observation. The id is time
// ordered, so observations written by one transaction read back in the order
// they were written.
func insertIssueBudgetObservation(ctx context.Context, q *db.Queries, p db.InsertIssueBudgetObservationParams, detail map[string]string) error {
	if detail == nil {
		detail = map[string]string{}
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode budget observation: %w", err)
	}
	p.ID = dbid.NewV7()
	p.Detail = encoded
	if _, err := q.InsertIssueBudgetObservation(ctx, p); err != nil {
		return fmt.Errorf("record budget observation: %w", err)
	}
	return nil
}
