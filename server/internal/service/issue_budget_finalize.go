package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// The budget step of a claim's finalization.
//
// A claim that FinalizeTaskClaim commits for a task of a member issue gets an
// attempt row, a shadow reservation in the ledger of the issue's root and the
// receipt of the operation that opened it, all in the finalize transaction.
// Nothing here refuses an execution: what an enforcing root would have refused
// is recorded as an observation, and the claim goes on.
//
// The step is the last thing the finalize transaction does. Its locks follow
// the order of the budget seams:
//
//   - The shared side of the workspace binding lock, tried and never waited
//     for, before the member row is read. A binder holds the exclusive side
//     from before its first member row to its commit, so a claim that got the
//     shared side reads every member row a binder has committed, and a binder
//     that got the exclusive side meets every claim that read "unbound" as a
//     committed task. The member row is read by a statement of its own behind
//     the lock: a statement that took the lock itself would read from the
//     snapshot it had before the lock was granted.
//   - The root row, the last lock of the transaction. Behind it only budget
//     tables are written.
//
// The workspace of all of it is the issue's: the issue row for a task of an
// issue, the workspace the server saved in the task context for a quick-create
// task. It is never the agent's, the runtime's or the task token's, which need
// not be the issue's.

// TaskClaimBudgetProtocol is the version of the budget usage protocol the
// claiming daemon declared. The zero value is a daemon that declared none.
type TaskClaimBudgetProtocol int32

const (
	// TaskClaimBudgetProtocolNone is a daemon that did not declare the budget
	// usage capability. Its claims are recorded as legacy_protocol gaps.
	TaskClaimBudgetProtocolNone TaskClaimBudgetProtocol = 0
	// TaskClaimBudgetProtocolV1 is protocol.DaemonCapabilityBudgetUsageV1.
	TaskClaimBudgetProtocolV1 TaskClaimBudgetProtocol = 1
)

// TaskClaimFinalization is what a committed claim finalization hands back.
type TaskClaimFinalization struct {
	MaterialClaim *protocol.MaterialClaimBinding
	// DeliveredCommentIDs is the comment receipt as the task row holds it.
	DeliveredCommentIDs []pgtype.UUID
	// BudgetAttemptID is valid when the claim got a budget attempt.
	BudgetAttemptID pgtype.UUID
}

// ErrIssueBudgetBindingInProgress is a claim that met a budget binding on its
// way: a transaction that gives existing issues their first member rows holds
// the workspace binding lock, or waits for it. The claim cannot tell whether
// its issue is being bound, so it is returned to the queue and claimed again.
var ErrIssueBudgetBindingInProgress = errors.New("budget binding in progress")

// ErrIssueBudgetClaimNeedsTransaction is a claim for a task of a member issue
// finalized by a service that has no transaction to finalize it in. Without
// one the locks of the budget step end with their statements and its writes
// are not atomic, so nothing is written and the finalization fails.
var ErrIssueBudgetClaimNeedsTransaction = errors.New("budget attempt requires a transaction")

// Scope, kind, source and reasons of what a claim writes.
const (
	issueBudgetScopeAttempt = "attempt"

	// IssueBudgetOperationAttemptOpen is the operation kind of a claim's
	// receipt.
	IssueBudgetOperationAttemptOpen = "attempt_open"

	issueBudgetActorRuntime = "runtime"

	issueBudgetSourceFinalize = "finalize"
	// issueBudgetSourceLifecycle is the source of an entry the lifecycle
	// reconciliation appends.
	issueBudgetSourceLifecycle = "lifecycle"

	// The states an attempt leaves 'open' for when what it reserved becomes a
	// hold. Each is the hold reason of the metering package under the same
	// name.
	issueBudgetAttemptSupersededUnproven = "superseded_unproven"
	issueBudgetAttemptCancelled          = "cancelled"
	issueBudgetAttemptTimeout            = "timeout"
	issueBudgetAttemptProcessExit        = "process_exit"
	issueBudgetAttemptLostContact        = "lost_contact"

	issueBudgetReasonLegacyProtocol      = "legacy_protocol"
	issueBudgetReasonLedgerUnreadable    = "ledger_unreadable"
	issueBudgetReasonReservationOverflow = "reservation_overflow"

	issueBudgetReasonRootClosed       = "root_closed"
	issueBudgetReasonRootDeleted      = "root_deleted"
	issueBudgetReasonUnboundedUnknown = "unbounded_unknown"
	issueBudgetReasonLimit            = "limit"

	// issueBudgetReasonMemberDeleted is an attribution difference, not a
	// reason to refuse.
	issueBudgetReasonMemberDeleted = "member_deleted"
)

// What an attempt reserved, as its operation result says it.
const (
	issueBudgetReservationSized   = "sized"
	issueBudgetReservationUnsized = "unsized"
	issueBudgetReservationNone    = "none"
)

// issueBudgetClaimIntent is what the original key of a claim stands for. It is
// a struct of its own: the management intent always carries its policy keys,
// so the two forms never have the same bytes under the one domain prefix. The
// field order is the digest's canonical form, pinned by a golden vector in the
// tests.
type issueBudgetClaimIntent struct {
	Kind            string       `json:"kind"`
	WorkspaceID     string       `json:"workspace_id"`
	ScopeKind       string       `json:"scope_kind"`
	ScopeID         string       `json:"scope_id"`
	TaskID          string       `json:"task_id"`
	ClaimGeneration budget.Count `json:"claim_generation"`
}

func (i issueBudgetClaimIntent) digest() (string, error) {
	encoded, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("encode budget claim intent: %w", err)
	}
	sum := sha256.Sum256(append([]byte(issueBudgetIntentDomain), encoded...))
	return hex.EncodeToString(sum[:]), nil
}

// issueBudgetAttemptOpenResult is the stored result of an attempt_open
// operation.
type issueBudgetAttemptOpenResult struct {
	OperationID     string       `json:"operation_id"`
	OperationKind   string       `json:"operation_kind"`
	BudgetRootID    string       `json:"budget_root_id"`
	AttemptID       string       `json:"attempt_id"`
	TaskID          string       `json:"task_id"`
	ClaimGeneration budget.Count `json:"claim_generation"`
	// Reservation is "sized" with ReservedTicks set, "unsized" for a root that
	// declares no reserve, and "none" when the ledger was unavailable and no
	// reservation was entered.
	Reservation   string        `json:"reservation"`
	ReservedTicks *budget.Ticks `json:"reserved_ticks"`
	// LedgerUnavailable is true when the ledger of the root could not be
	// loaded: the attempt exists, and nothing was appended for it.
	LedgerUnavailable bool `json:"ledger_unavailable"`
	// SupersededAttemptIDs are the earlier open attempts of the task this
	// claim closed as superseded_unproven, oldest first.
	SupersededAttemptIDs []string `json:"superseded_attempt_ids"`
	// WouldRefuse are the reasons an enforcing root would have refused the
	// claim for, in the order they are judged.
	WouldRefuse []string `json:"would_refuse"`
}

// taskBudgetClaim is what the budget step did for one claim, before the
// finalize transaction has committed.
type taskBudgetClaim struct {
	// AttemptID is valid when the claim got an attempt row.
	AttemptID pgtype.UUID

	appended issueBudgetLedgerAppend
}

// Committed is for the moment the commit of the finalize transaction is
// confirmed, and for no other: it publishes the ledger the claim appended to.
func (c taskBudgetClaim) Committed() { c.appended.Committed() }

// issueBudgetClaimSubject is the issue a claim is attributed through.
type issueBudgetClaimSubject struct {
	// IssueID is the task's own issue, or the context parent of a quick-create
	// task.
	IssueID pgtype.UUID
	// QuickCreate is true when IssueID is a context parent.
	QuickCreate bool
	// WorkspaceID is valid for a quick-create task only: the workspace saved in
	// its context. A task of an issue reads its workspace from the issue row.
	WorkspaceID pgtype.UUID
}

// claimSubject decides whether a task can belong to a root at
// all. ok is false for a task without an issue that is not a quick-create task
// with a context parent (chat, a run-only autopilot, a quick-create without a
// parent): such a claim costs no statement.
func (c *issueBudgetLedgerCache) claimSubject(task db.AgentTaskQueue) (subject issueBudgetClaimSubject, ok bool) {
	if task.IssueID.Valid {
		return issueBudgetClaimSubject{IssueID: task.IssueID}, true
	}
	qc, quickCreate := quickCreateTaskContext(task.IssueID, task.ChatSessionID, task.AutopilotRunID, task.Context)
	if !quickCreate || qc.ParentIssueID == "" {
		return issueBudgetClaimSubject{}, false
	}
	parentID, parentErr := util.ParseUUID(qc.ParentIssueID)
	workspaceID, workspaceErr := util.ParseUUID(qc.WorkspaceID)
	if parentErr != nil || workspaceErr != nil {
		// The server wrote both when it queued the task. A context that does
		// not carry them names no member row.
		c.logger().Warn("quick-create task context names no budget member: the claim is not attributed",
			"task_id", util.UUIDToString(task.ID),
			"parent_issue_id_valid", parentErr == nil,
			"workspace_id_valid", workspaceErr == nil,
		)
		return issueBudgetClaimSubject{}, false
	}
	return issueBudgetClaimSubject{IssueID: parentID, QuickCreate: true, WorkspaceID: workspaceID}, true
}

// finalizeTaskBudget is the budget step of FinalizeTaskClaim. q is the finalize
// transaction, task the row it locked, and every other write of the
// finalization is behind it. inTx is false when the service has no transaction
// and q is the bare pool.
//
// The claim of a task outside every root writes nothing and takes no lock
// beyond the shared try lock. For a member issue the result carries the
// attempt id; the caller calls Committed on it once its commit is confirmed.
// An error fails the finalization: ErrIssueBudgetBindingInProgress when a
// binding is on its way, anything else a write that did not go through.
func (c *issueBudgetLedgerCache) finalizeTaskBudget(ctx context.Context, q *db.Queries, inTx bool, task db.AgentTaskQueue, declared TaskClaimBudgetProtocol) (taskBudgetClaim, error) {
	subject, ok := c.claimSubject(task)
	if !ok {
		return taskBudgetClaim{}, nil
	}
	if !inTx {
		return taskBudgetClaim{}, c.claimWithoutTransaction(ctx, q, task, subject)
	}

	workspaceID := subject.WorkspaceID
	if subject.QuickCreate {
		acquired, err := q.TryLockIssueBudgetBindingShared(ctx, workspaceID)
		if err != nil {
			return taskBudgetClaim{}, fmt.Errorf("try budget binding lock: %w", err)
		}
		if !acquired {
			return taskBudgetClaim{}, ErrIssueBudgetBindingInProgress
		}
	} else {
		row, err := q.TryLockIssueBudgetBindingSharedForIssue(ctx, subject.IssueID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.warnClaimIssueMissing(task)
			return taskBudgetClaim{}, nil
		}
		if err != nil {
			return taskBudgetClaim{}, fmt.Errorf("try budget binding lock: %w", err)
		}
		if !row.Acquired {
			return taskBudgetClaim{}, ErrIssueBudgetBindingInProgress
		}
		workspaceID = row.WorkspaceID
	}

	member, bound, err := issueBudgetMember(ctx, q, workspaceID, subject.IssueID)
	if err != nil {
		return taskBudgetClaim{}, err
	}
	if !bound {
		return taskBudgetClaim{}, nil
	}
	if err := requireIssueBudgetReadCommitted(ctx, q); err != nil {
		return taskBudgetClaim{}, err
	}

	// The last lock of the finalize transaction.
	root, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: member.WorkspaceID, ID: member.BudgetRootID})
	if errors.Is(err, pgx.ErrNoRows) {
		// A member row without its root is damage no code path writes. There
		// is no root to attribute to and no ledger to append to; the
		// execution is not held back for it.
		c.logger().Error("budget member row names a root that does not exist: the claim is not attributed",
			"budget_root_id", util.UUIDToString(member.BudgetRootID),
			"issue_id", util.UUIDToString(member.IssueID),
			"task_id", util.UUIDToString(task.ID),
			"claim_generation", task.ClaimGeneration,
		)
		return taskBudgetClaim{}, nil
	}
	if err != nil {
		return taskBudgetClaim{}, fmt.Errorf("lock budget root: %w", err)
	}
	if root.Mode == IssueBudgetModeOff {
		return taskBudgetClaim{}, nil
	}
	return c.openAttempt(ctx, q, root, member, subject, task, declared)
}

// claimWithoutTransaction is the budget step of a service that has
// no transaction: it takes no lock and writes nothing. It finds the workspace
// and the member row as the step in a transaction does, lets the claim of a
// task outside every root go on as before, and fails the claim of a member
// issue whatever the mode of its root.
func (c *issueBudgetLedgerCache) claimWithoutTransaction(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, subject issueBudgetClaimSubject) error {
	workspaceID := subject.WorkspaceID
	if !subject.QuickCreate {
		issue, err := q.GetIssue(ctx, subject.IssueID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.warnClaimIssueMissing(task)
			return nil
		}
		if err != nil {
			return fmt.Errorf("read the issue of a claim: %w", err)
		}
		workspaceID = issue.WorkspaceID
	}
	_, bound, err := issueBudgetMember(ctx, q, workspaceID, subject.IssueID)
	if err != nil {
		return err
	}
	if !bound {
		return nil
	}
	return ErrIssueBudgetClaimNeedsTransaction
}

func (c *issueBudgetLedgerCache) warnClaimIssueMissing(task db.AgentTaskQueue) {
	c.logger().Warn("the issue of a claimed task does not exist: the claim is not attributed",
		"task_id", util.UUIDToString(task.ID),
		"issue_id", util.UUIDToString(task.IssueID),
	)
}

// openAttempt writes what a claim on an observing root leaves: the holds
// of the claims it supersedes, its own reservation, the attempt row, the
// observations and the operation receipt. The caller holds the root lock and
// root is the row it locked.
func (c *issueBudgetLedgerCache) openAttempt(ctx context.Context, q *db.Queries, root db.IssueBudgetRoot, member db.IssueBudgetMember, subject issueBudgetClaimSubject, task db.AgentTaskQueue, declared TaskClaimBudgetProtocol) (taskBudgetClaim, error) {
	attemptID, operationID := dbid.NewV7(), dbid.NewV7()
	attemptText := util.UUIDToString(attemptID)
	result := issueBudgetAttemptOpenResult{
		OperationID:          util.UUIDToString(operationID),
		OperationKind:        IssueBudgetOperationAttemptOpen,
		BudgetRootID:         util.UUIDToString(root.ID),
		AttemptID:            attemptText,
		TaskID:               util.UUIDToString(task.ID),
		ClaimGeneration:      budget.Count(task.ClaimGeneration),
		Reservation:          issueBudgetReservationNone,
		SupersededAttemptIDs: []string{},
		WouldRefuse:          []string{},
	}
	observation := db.InsertIssueBudgetObservationParams{
		WorkspaceID:     root.WorkspaceID,
		BudgetRootID:    root.ID,
		IssueID:         subject.IssueID,
		TaskID:          task.ID,
		ClaimGeneration: pgtype.Int8{Int64: task.ClaimGeneration, Valid: true},
		AttemptID:       attemptID,
	}
	observe := func(kind, reason string, detail map[string]string) error {
		p := observation
		p.Kind, p.Reason = kind, reason
		return insertIssueBudgetObservation(ctx, q, p, detail)
	}

	ledger, loadErr := c.loadUnderRootLock(ctx, q, root)
	available := loadErr == nil
	if loadErr != nil && !errors.Is(loadErr, ErrIssueBudgetLedgerUnavailable) {
		return taskBudgetClaim{}, loadErr
	}
	claim := taskBudgetClaim{AttemptID: attemptID}
	attempt := db.InsertIssueBudgetAttemptParams{
		ID:              attemptID,
		WorkspaceID:     root.WorkspaceID,
		BudgetRootID:    root.ID,
		TaskID:          task.ID,
		ClaimGeneration: task.ClaimGeneration,
		RuntimeID:       task.RuntimeID,
		AgentID:         task.AgentID,
		ProtocolVersion: pgtype.Int4{Int32: int32(declared), Valid: declared > TaskClaimBudgetProtocolNone},
		PolicyRevision:  pgtype.Int8{Int64: root.PolicyRevision, Valid: true},
		PolicyEpoch:     pgtype.Int8{Int64: root.PolicyEpoch, Valid: true},
	}
	if subject.QuickCreate {
		attempt.ContextParentIssueID = subject.IssueID
	} else {
		attempt.IssueID = subject.IssueID
	}

	var reserved budget.Ticks
	// overflowed is true when the reserve the root declares did not fit in
	// the ledger and the claim reserved without an amount.
	overflowed := false
	if available {
		// The claims this one supersedes. A reservation that was never
		// dispatched cannot be told from one that was, so none is released:
		// what an earlier claim reserved becomes a hold.
		earlier, err := q.ListIssueBudgetOpenAttemptsOfTask(ctx, db.ListIssueBudgetOpenAttemptsOfTaskParams{
			TaskID:          task.ID,
			ClaimGeneration: task.ClaimGeneration,
			WorkspaceID:     root.WorkspaceID,
			BudgetRootID:    root.ID,
		})
		if err != nil {
			return taskBudgetClaim{}, fmt.Errorf("list earlier budget attempts: %w", err)
		}
		entries := make([]issueBudgetLedgerEntry, 0, len(earlier)+1)
		for _, old := range earlier {
			oldText := util.UUIDToString(old.ID)
			// An attempt written while the ledger was unavailable has no
			// reservation to hold.
			if ledger.HasReservation(oldText) {
				entries = append(entries, issueBudgetLedgerEntry{
					Entry: budget.Entry{
						ID:            "hold/" + oldText + "/" + string(budget.HoldSupersededUnproven),
						Kind:          budget.KindHold,
						ReservationID: oldText,
						HoldReason:    budget.HoldSupersededUnproven,
					},
					Source:      issueBudgetSourceFinalize,
					OperationID: operationID,
				})
			}
			closed, err := q.CloseIssueBudgetAttempt(ctx, db.CloseIssueBudgetAttemptParams{
				State:       issueBudgetAttemptSupersededUnproven,
				WorkspaceID: old.WorkspaceID,
				ID:          old.ID,
			})
			if err != nil {
				return taskBudgetClaim{}, fmt.Errorf("supersede budget attempt: %w", err)
			}
			if closed != 1 {
				return taskBudgetClaim{}, fmt.Errorf("supersede budget attempt %s: it is not open any more under the root lock", oldText)
			}
			result.SupersededAttemptIDs = append(result.SupersededAttemptIDs, oldText)
		}

		reserve := budget.Entry{ID: "reserve/" + attemptText, Kind: budget.KindReserve, ReservationID: attemptText}
		if root.AttemptReserveTicks.Valid {
			reserved = budget.Ticks(root.AttemptReserveTicks.Int64)
			reserve.ReservedTicks = reserved
			attempt.ReservedTicks = root.AttemptReserveTicks
			result.Reservation, result.ReservedTicks = issueBudgetReservationSized, &reserved
		} else {
			reserve.Unsized = true
			result.Reservation = issueBudgetReservationUnsized
		}
		entries = append(entries, issueBudgetLedgerEntry{Entry: reserve, Source: issueBudgetSourceFinalize, OperationID: operationID})
		appended, err := c.appendEntries(ctx, q, root, ledger, entries)
		if err != nil && !reserve.Unsized && errors.Is(err, budget.ErrOverflow) {
			// No reservation is released in this slice, so what a root has
			// reserved only grows, and the reserve it declares has no upper
			// bound: a sized reserve can take known spend, reservations and
			// holds out of range. A root that only observes must not hold the
			// execution back for that, so the claim reserves without an amount
			// and the reserve that did not fit is recorded as a gap below.
			//
			// This relies on appendEntries asking the pure package before it
			// sends a statement (ledger.With precedes the first insert): the
			// refused batch left no row and did not move the root, so the same
			// holds can be appended again in front of the unsized reserve. It
			// is tried once. An unsized reserve adds no amount, so a second
			// overflow cannot happen, and it fails the finalization like any
			// other error.
			overflowed = true
			reserve.ReservedTicks, reserve.Unsized = 0, true
			entries[len(entries)-1].Entry = reserve
			reserved = 0
			attempt.ReservedTicks = pgtype.Int8{}
			result.Reservation, result.ReservedTicks = issueBudgetReservationUnsized, nil
			c.logger().Error("budget reserve does not fit in the ledger: the attempt reserves without an amount",
				"budget_root_id", util.UUIDToString(root.ID),
				"attempt_id", attemptText,
				"task_id", util.UUIDToString(task.ID),
				"claim_generation", task.ClaimGeneration,
				"attempt_reserve_ticks", root.AttemptReserveTicks.Int64,
			)
			appended, err = c.appendEntries(ctx, q, root, ledger, entries)
		}
		if err != nil {
			return taskBudgetClaim{}, err
		}
		claim.appended = appended
	} else {
		// The ledger cannot be read, so nothing is appended and the root row
		// keeps its figures. The claim still gets its attempt, without a
		// reservation, and the root has one more thing it does not know.
		// Earlier open attempts of the task stay as they are: whether one has
		// a reservation to hold cannot be asked.
		result.LedgerUnavailable = true
		c.logger().Error("budget ledger unavailable at claim: the attempt is recorded without a reservation",
			"budget_root_id", util.UUIDToString(root.ID),
			"attempt_id", attemptText,
			"task_id", util.UUIDToString(task.ID),
			"claim_generation", task.ClaimGeneration,
			"error", loadErr.Error(),
		)
	}

	if _, err := q.InsertIssueBudgetAttempt(ctx, attempt); err != nil {
		return taskBudgetClaim{}, fmt.Errorf("insert budget attempt: %w", err)
	}
	if !available {
		if err := observe(issueBudgetObservationCoverageGap, issueBudgetReasonLedgerUnreadable, nil); err != nil {
			return taskBudgetClaim{}, err
		}
	}
	if !attempt.ProtocolVersion.Valid {
		// Written before anything is judged: a daemon that reports no usage
		// is itself something the root does not know.
		if err := observe(issueBudgetObservationCoverageGap, issueBudgetReasonLegacyProtocol, nil); err != nil {
			return taskBudgetClaim{}, err
		}
	}
	if overflowed {
		// Written before anything is judged, like the two above, so the
		// unknown set of this claim counts it.
		detail := map[string]string{"attempt_reserve_ticks": strconv.FormatInt(root.AttemptReserveTicks.Int64, 10)}
		if err := observe(issueBudgetObservationCoverageGap, issueBudgetReasonReservationOverflow, detail); err != nil {
			return taskBudgetClaim{}, err
		}
	}

	// What an enforcing root would have refused this claim for. One
	// observation for each reason that holds; the claim goes on.
	refuse := func(reason string, detail map[string]string) error {
		result.WouldRefuse = append(result.WouldRefuse, reason)
		return observe(issueBudgetObservationWouldRefuse, reason, detail)
	}
	if root.AdmissionState == IssueBudgetAdmissionClosed {
		if err := refuse(issueBudgetReasonRootClosed, nil); err != nil {
			return taskBudgetClaim{}, err
		}
	}
	rootMember, rootBound := member, true
	if member.IssueID != root.RootIssueID {
		var err error
		rootMember, rootBound, err = issueBudgetMember(ctx, q, root.WorkspaceID, root.RootIssueID)
		if err != nil {
			return taskBudgetClaim{}, err
		}
	}
	switch {
	case !rootBound:
		c.logger().Error("budget root has no member row for its root issue: whether the root issue was deleted is not judged",
			"budget_root_id", util.UUIDToString(root.ID),
			"root_issue_id", util.UUIDToString(root.RootIssueID),
		)
	case rootMember.TombstonedAt.Valid:
		if err := refuse(issueBudgetReasonRootDeleted, nil); err != nil {
			return taskBudgetClaim{}, err
		}
	}
	if member.TombstonedAt.Valid && member.IssueID != root.RootIssueID {
		// The issue the claim was attributed through was deleted while the
		// task waited; its member row stays, marked. An enforcing root would
		// not refuse the claim for that, so it is recorded as a difference in
		// attribution and is not part of would_refuse. A deleted root issue is
		// root_deleted, judged above for every claim on the root.
		if err := observe(issueBudgetObservationAttributionDifference, issueBudgetReasonMemberDeleted, nil); err != nil {
			return taskBudgetClaim{}, err
		}
	}
	if root.LimitTicks.Valid {
		// A root without a limit has no amount an enforcing root could
		// refuse on, and does not pay for the count.
		var projection budget.Projection
		unknown := map[string]string{}
		if available {
			projection = claim.appended.Ledger.Projection()
			for reason, count := range projection.UnknownCounts() {
				unknown["ledger."+string(reason)] = strconv.Itoa(count)
			}
		} else {
			unknown["ledger"] = "unavailable"
		}
		gaps, err := q.CountIssueBudgetUncoveredGaps(ctx, db.CountIssueBudgetUncoveredGapsParams{WorkspaceID: root.WorkspaceID, BudgetRootID: root.ID})
		if err != nil {
			return taskBudgetClaim{}, fmt.Errorf("count uncovered budget gaps: %w", err)
		}
		for _, gap := range gaps {
			unknown["observation."+gap.Reason] = strconv.FormatInt(gap.GapCount, 10)
		}
		if len(unknown) > 0 {
			if err := refuse(issueBudgetReasonUnboundedUnknown, unknown); err != nil {
				return taskBudgetClaim{}, err
			}
		}
		if available {
			// Judged on what the root has committed to with this
			// reservation in it, and judged whether or not something is
			// unknown.
			if limit := budget.Ticks(root.LimitTicks.Int64); projection.Committed() > limit {
				detail := map[string]string{
					"actual_known_ticks":     strconv.FormatInt(int64(projection.ActualKnown), 10),
					"reserved_ticks":         strconv.FormatInt(int64(projection.Reserved), 10),
					"unknown_hold_ticks":     strconv.FormatInt(int64(projection.UnknownHold), 10),
					"limit_ticks":            strconv.FormatInt(int64(limit), 10),
					"this_reservation":       result.Reservation,
					"this_reservation_ticks": strconv.FormatInt(int64(reserved), 10),
				}
				if err := refuse(issueBudgetReasonLimit, detail); err != nil {
					return taskBudgetClaim{}, err
				}
			}
		}
	}

	// The receipt of the claim, under its original key. The entries above
	// name it; it is written last because it says what was observed.
	intent := issueBudgetClaimIntent{
		Kind:            IssueBudgetOperationAttemptOpen,
		WorkspaceID:     util.UUIDToString(root.WorkspaceID),
		ScopeKind:       issueBudgetScopeAttempt,
		ScopeID:         attemptText,
		TaskID:          result.TaskID,
		ClaimGeneration: result.ClaimGeneration,
	}
	digest, err := intent.digest()
	if err != nil {
		return taskBudgetClaim{}, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return taskBudgetClaim{}, fmt.Errorf("encode budget operation result: %w", err)
	}
	if _, err := q.InsertIssueBudgetOperation(ctx, db.InsertIssueBudgetOperationParams{
		ID:            operationID,
		WorkspaceID:   root.WorkspaceID,
		BudgetRootID:  root.ID,
		ScopeKind:     issueBudgetScopeAttempt,
		ScopeID:       attemptID,
		OperationKey:  "claim:" + result.TaskID + ":" + strconv.FormatInt(task.ClaimGeneration, 10),
		OperationKind: IssueBudgetOperationAttemptOpen,
		IntentDigest:  digest,
		ActorType:     issueBudgetActorRuntime,
		ActorID:       task.RuntimeID,
		Result:        encoded,
	}); err != nil {
		return taskBudgetClaim{}, fmt.Errorf("record budget operation: %w", err)
	}
	return claim, nil
}
