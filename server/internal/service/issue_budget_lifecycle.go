package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

// The lifecycle reconciliation of a budget root (design 10.3).
//
// A claim opens an attempt and reserves for it. Nothing tells the root when
// the task of that attempt ends: the paths that end a task do not know about
// budgets and take none of its locks. So the root is reconciled afterwards.
// One reconciliation locks one root, looks at each attempt that is still open
// and, when the attempt's task has reached a final state or its row is gone,
// turns what the attempt reserved into a hold and closes the attempt with the
// reason of the hold.
//
// It never settles and never releases. That a task ended, that its queue row
// is gone, that a request was aborted or that nothing is running any more says
// nothing about what the agent spent, so the amount stays in the root as
// something it does not know until a usage report settles it.
//
// The transaction takes the root lock and no other lock. It reads the task
// rows without locking them, so it never waits for a task, an issue or the
// binding fence, and nothing that holds one of those can wait for it in a
// cycle. A final task status does not change again and a deleted row does not
// come back, so what the unlocked read finds stays true.

// Why a reconciliation left a root as it was.
const (
	// IssueBudgetLifecycleSkippedOff: the root's switch is off.
	IssueBudgetLifecycleSkippedOff = "off"
	// IssueBudgetLifecycleSkippedLedger: the root's ledger could not be
	// loaded. The attempts stay open and the next run tries again.
	IssueBudgetLifecycleSkippedLedger = "ledger_unavailable"
)

// IssueBudgetLifecycleClosed is one attempt a reconciliation closed.
type IssueBudgetLifecycleClosed struct {
	AttemptID pgtype.UUID
	TaskID    pgtype.UUID
	// State is what the attempt left 'open' for. It is also the reason of the
	// hold.
	State string
	// Held is false when the ledger had no reservation of the attempt (it was
	// claimed while the ledger was unavailable): only its state changed.
	Held bool
}

// IssueBudgetLifecycleResult is what one reconciliation of one root did.
type IssueBudgetLifecycleResult struct {
	// Skipped is empty when the run looked at the root's open attempts, and
	// one of the IssueBudgetLifecycleSkipped values when it did not.
	Skipped string
	// Closed are the attempts the run closed, in the order it closed them.
	Closed []IssueBudgetLifecycleClosed
}

// The task states nothing leaves again.
const (
	issueBudgetTaskCompleted = "completed"
	issueBudgetTaskFailed    = "failed"
	issueBudgetTaskCancelled = "cancelled"
)

// issueBudgetAttemptEnd is how an attempt ends when its task did: the reason
// of the hold entry and the state of the attempt row. The two are the same
// word.
type issueBudgetAttemptEnd struct {
	hold  budget.HoldReason
	state string
}

var (
	issueBudgetEndCancelled          = issueBudgetAttemptEnd{budget.HoldCancelled, issueBudgetAttemptCancelled}
	issueBudgetEndTimeout            = issueBudgetAttemptEnd{budget.HoldTimeout, issueBudgetAttemptTimeout}
	issueBudgetEndProcessExit        = issueBudgetAttemptEnd{budget.HoldProcessExit, issueBudgetAttemptProcessExit}
	issueBudgetEndLostContact        = issueBudgetAttemptEnd{budget.HoldLostContact, issueBudgetAttemptLostContact}
	issueBudgetEndSupersededUnproven = issueBudgetAttemptEnd{budget.HoldSupersededUnproven, issueBudgetAttemptSupersededUnproven}
)

// issueBudgetStaleTaskError is the error text FailStaleTasks writes beside the
// failure reason 'timeout'. That reason has two writers: the sweeper, which
// fails a task whose daemon no longer proves it is alive, and a daemon that
// reports an agent or prepare timeout it saw. Only the error text tells the
// two apart. A row that carries exactly this text is the sweeper's and ends
// as lost contact; every other 'timeout' is a timeout. A daemon that happens
// to report the same text gets the weaker of the two, which is the safe side.
const issueBudgetStaleTaskError = "task timed out"

// issueBudgetFailureEnds says how the attempt of a failed task ends, by the
// failure reason on the task row. Every reason the server writes itself and
// every reason of the taskfailure taxonomy is listed, also the ones that end
// as the default does, so that a new reason has to be placed here. A string
// that is not listed (a daemon can report any text) ends as lost contact:
// the weakest claim, not a finding that the runtime was lost.
var issueBudgetFailureEnds = map[string]issueBudgetAttemptEnd{
	// A daemon reports a cancelled run as a failure with this reason.
	"cancelled": issueBudgetEndCancelled,

	// A timeout the daemon saw and reported itself. The bare timeout is what a
	// daemon reports for an agent or prepare timeout; the one row of it the
	// server writes is taken out before this map is asked, see
	// issueBudgetStaleTaskError.
	string(taskfailure.ReasonAgentTimeout):      issueBudgetEndTimeout,
	string(taskfailure.ReasonRuntimeCLITimeout): issueBudgetEndTimeout,
	string(taskfailure.ReasonTimeout):           issueBudgetEndTimeout,
	"idle_watchdog":                             issueBudgetEndTimeout,
	"codex_semantic_inactivity":                 issueBudgetEndTimeout,

	// The server gave the task up without hearing from its runtime.
	string(taskfailure.ReasonRuntimeOffline):          issueBudgetEndLostContact,
	string(taskfailure.ReasonRuntimeRecovery):         issueBudgetEndLostContact,
	string(taskfailure.ReasonRuntimeReconnectTimeout): issueBudgetEndLostContact,
	string(taskfailure.ReasonQueuedExpired):           issueBudgetEndLostContact,

	// A failure somebody in contact with the run reported.
	"agent_error": issueBudgetEndProcessExit, // the default of FailAgentTask
	string(taskfailure.ReasonAgentProviderAuthOrAccess):        issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentProviderQuotaLimit):          issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentProviderCapacityOrRateLimit): issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentProviderServerError):         issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentProviderNetwork):             issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentProcessFailure):              issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentEmptyOrUnparseableOutput):    issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentContextOverflow):             issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentMissingConfig):               issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentModelNotFoundOrUnavailable):  issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentRuntimeVersionUnsupported):   issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentRuntimeMissingExecutable):    issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentUnknown):                     issueBudgetEndProcessExit,
	string(taskfailure.ReasonIterationLimit):                   issueBudgetEndProcessExit,
	string(taskfailure.ReasonAgentBlocked):                     issueBudgetEndProcessExit,
	string(taskfailure.ReasonAPIInvalidRequest):                issueBudgetEndProcessExit,
	string(taskfailure.ReasonSkillBundleUnavailable):           issueBudgetEndProcessExit,
	string(taskfailure.ReasonInvalidTaskIdentity):              issueBudgetEndProcessExit,
	string(taskfailure.ReasonKnowledgeContextChanged):          issueBudgetEndProcessExit,
	string(taskfailure.ReasonKnowledgeAuthorizationDenied):     issueBudgetEndProcessExit,
	string(taskfailure.ReasonKnowledgeUnavailable):             issueBudgetEndProcessExit,
	string(taskfailure.ReasonKnowledgeBusy):                    issueBudgetEndProcessExit,
	string(taskfailure.ReasonKnowledgeInvocationUnsupported):   issueBudgetEndProcessExit,
	"local_directory_error":                                    issueBudgetEndProcessExit,
	"agent_fallback_message":                                   issueBudgetEndProcessExit,
	"codex_resume_oversized":                                   issueBudgetEndProcessExit,
}

// issueBudgetAttemptEndOf says whether an open attempt has ended, and how, by
// what its task row says now. ended is false while the task can still run.
func issueBudgetAttemptEndOf(attempt db.ListIssueBudgetOpenAttemptsOfRootRow) (end issueBudgetAttemptEnd, ended bool) {
	if !attempt.TaskExists {
		// Nothing is left to say how the task ended.
		return issueBudgetEndLostContact, true
	}
	switch attempt.TaskStatus {
	case issueBudgetTaskCompleted, issueBudgetTaskFailed, issueBudgetTaskCancelled:
	default:
		return issueBudgetAttemptEnd{}, false
	}
	if attempt.ClaimGeneration < attempt.TaskClaimGeneration {
		// The final status speaks about the task's last claim, and this
		// attempt belongs to an earlier one: a later claim was committed that
		// wrote no attempt of its own, or the claim that would have superseded
		// this attempt could not load the ledger. It was superseded, which is
		// what the next claim would have written.
		return issueBudgetEndSupersededUnproven, true
	}
	switch attempt.TaskStatus {
	case issueBudgetTaskCompleted:
		return issueBudgetEndProcessExit, true
	case issueBudgetTaskCancelled:
		// Whatever reason text the caller of the cancellation left.
		return issueBudgetEndCancelled, true
	}
	if attempt.TaskFailureReason == string(taskfailure.ReasonTimeout) && attempt.TaskError == issueBudgetStaleTaskError {
		// The sweeper gave the task up; no daemon reported this timeout.
		return issueBudgetEndLostContact, true
	}
	if end, ok := issueBudgetFailureEnds[attempt.TaskFailureReason]; ok {
		return end, true
	}
	return issueBudgetEndLostContact, true
}

// ReconcileLifecycle turns what the ended attempts of one root reserved into
// holds, in a transaction of its own. An attempt has ended when its task
// reached a final state or its task row is gone. Running it again writes
// nothing: an attempt that is not open any more is not looked at.
//
// A root whose switch is off and a root whose ledger cannot be loaded are left
// exactly as they are; the result says which. Both are reconciled by a later
// run, once the switch is on again or the ledger loads.
func (s *IssueBudgetService) ReconcileLifecycle(ctx context.Context, workspaceID, rootID pgtype.UUID) (IssueBudgetLifecycleResult, error) {
	var (
		result   IssueBudgetLifecycleResult
		appended issueBudgetLedgerAppend
	)
	// transact declares READ COMMITTED as the first statement: the open
	// attempts are listed after the root lock is granted and have to be seen
	// as they are then.
	_, err := s.transact(ctx, func(_ pgx.Tx, q *db.Queries) (IssueBudgetOperationResult, error) {
		r, a, err := issueBudgetLedgers.reconcileLifecycle(ctx, q, workspaceID, rootID)
		result, appended = r, a
		return IssueBudgetOperationResult{}, err
	})
	if err != nil {
		return IssueBudgetLifecycleResult{}, err
	}
	// transact returned without an error, so the commit is confirmed.
	appended.Committed()
	return result, nil
}

// reconcileLifecycle is the body of ReconcileLifecycle. The caller commits
// the transaction of q and calls Committed on what is handed back only after
// the commit is confirmed.
func (c *issueBudgetLedgerCache) reconcileLifecycle(ctx context.Context, q *db.Queries, workspaceID, rootID pgtype.UUID) (IssueBudgetLifecycleResult, issueBudgetLedgerAppend, error) {
	root, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: workspaceID, ID: rootID})
	if errors.Is(err, pgx.ErrNoRows) {
		return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, issueBudgetError(IssueBudgetNotFound)
	}
	if err != nil {
		return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, fmt.Errorf("lock budget root: %w", err)
	}
	if root.Mode == IssueBudgetModeOff {
		return IssueBudgetLifecycleResult{Skipped: IssueBudgetLifecycleSkippedOff}, issueBudgetLedgerAppend{}, nil
	}

	ledger, err := c.loadUnderRootLock(ctx, q, root)
	if errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
		// Whether an attempt has a reservation to hold cannot be asked, and
		// closing it without the hold would lose what it reserved. Nothing is
		// changed, so the run after the repair finds the same attempts.
		c.logger().Error("budget ledger unavailable at the lifecycle reconciliation: no attempt is closed",
			"budget_root_id", util.UUIDToString(root.ID),
			"error", err.Error(),
		)
		return IssueBudgetLifecycleResult{Skipped: IssueBudgetLifecycleSkippedLedger}, issueBudgetLedgerAppend{}, nil
	}
	if err != nil {
		return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, err
	}

	// Listed after the lock: a claim that superseded an attempt, or an
	// earlier reconciliation, committed before this statement's snapshot.
	open, err := q.ListIssueBudgetOpenAttemptsOfRoot(ctx, db.ListIssueBudgetOpenAttemptsOfRootParams{
		WorkspaceID:  root.WorkspaceID,
		BudgetRootID: root.ID,
	})
	if err != nil {
		return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, fmt.Errorf("list open budget attempts: %w", err)
	}

	var result IssueBudgetLifecycleResult
	entries := make([]issueBudgetLedgerEntry, 0, len(open))
	for _, attempt := range open {
		end, ended := issueBudgetAttemptEndOf(attempt)
		if !ended {
			continue
		}
		attemptText := util.UUIDToString(attempt.ID)
		// An attempt written while the ledger was unavailable has no
		// reservation to hold; it only leaves 'open'.
		held := ledger.HasReservation(attemptText)
		if held {
			entries = append(entries, issueBudgetLedgerEntry{
				Entry: budget.Entry{
					ID:            "hold/" + attemptText + "/" + string(end.hold),
					Kind:          budget.KindHold,
					ReservationID: attemptText,
					HoldReason:    end.hold,
				},
				Source: issueBudgetSourceLifecycle,
			})
		}
		closed, err := q.CloseIssueBudgetAttempt(ctx, db.CloseIssueBudgetAttemptParams{
			State:       end.state,
			WorkspaceID: attempt.WorkspaceID,
			ID:          attempt.ID,
		})
		if err != nil {
			return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, fmt.Errorf("close budget attempt: %w", err)
		}
		if closed != 1 {
			return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, fmt.Errorf("close budget attempt %s: it is not open any more under the root lock", attemptText)
		}
		result.Closed = append(result.Closed, IssueBudgetLifecycleClosed{
			AttemptID: attempt.ID,
			TaskID:    attempt.TaskID,
			State:     end.state,
			Held:      held,
		})
	}
	if len(entries) == 0 {
		return result, issueBudgetLedgerAppend{}, nil
	}
	// Every hold of the run in one batch: one projection, one root update.
	appended, err := c.appendEntries(ctx, q, root, ledger, entries)
	if err != nil {
		return IssueBudgetLifecycleResult{}, issueBudgetLedgerAppend{}, err
	}
	return result, appended, nil
}

// IssueBudgetLifecycleRound is what one round over the roots with ended
// attempts did.
type IssueBudgetLifecycleRound struct {
	// Roots is how many roots the round found work for.
	Roots int
	// Closed is how many attempts it closed, over all of them.
	Closed int
	// Skipped is how many it left exactly as they were: the switch went off
	// after the root was found, or its ledger could not be loaded.
	Skipped int
	// Failed is how many it could not reconcile: the call returned an error or
	// ran out of its time. Nothing of such a root is changed.
	Failed int
}

// ReconcileLifecycles is one round of the lifecycle reconciliation over every
// workspace. One read finds at most maxRoots roots that have an ended attempt
// (see ListIssueBudgetRootsWithEndedAttempts). When it finds none the round is
// that one statement: no transaction, no lock, no write. Every root found is
// then reconciled in a transaction of its own, which may take each at most, so
// that a root that fails, or whose lock somebody else holds, does not keep the
// round from the roots after it. The read may take the same time.
//
// There is no cursor. A root that was reconciled is not found again, so the
// next round returns the roots after it; a root that failed or was skipped is
// found again and tried again.
//
// The error is the read's. A root that could not be reconciled is counted and
// logged with its id.
func (s *IssueBudgetService) ReconcileLifecycles(ctx context.Context, maxRoots int32, each time.Duration) (IssueBudgetLifecycleRound, error) {
	findCtx, cancel := context.WithTimeout(ctx, each)
	roots, err := s.Queries.ListIssueBudgetRootsWithEndedAttempts(findCtx, maxRoots)
	cancel()
	if err != nil {
		return IssueBudgetLifecycleRound{}, fmt.Errorf("list budget roots with ended attempts: %w", err)
	}
	return s.reconcileLifecycleRoots(ctx, roots, each), nil
}

// reconcileLifecycleRoots reconciles the roots given one after the other,
// each under its own deadline. It stops between two roots when ctx is done.
func (s *IssueBudgetService) reconcileLifecycleRoots(ctx context.Context, roots []db.ListIssueBudgetRootsWithEndedAttemptsRow, each time.Duration) IssueBudgetLifecycleRound {
	round := IssueBudgetLifecycleRound{Roots: len(roots)}
	for _, root := range roots {
		if ctx.Err() != nil {
			// The process is stopping. What is left is found again.
			break
		}
		rootCtx, cancel := context.WithTimeout(ctx, each)
		result, err := s.ReconcileLifecycle(rootCtx, root.WorkspaceID, root.ID)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				// The round was stopped, which says nothing about this root.
				break
			}
			round.Failed++
			issueBudgetLedgers.logger().Warn("budget lifecycle reconciliation of a root failed: its attempts stay open",
				"budget_root_id", util.UUIDToString(root.ID),
				"error", err.Error(),
			)
			continue
		}
		if result.Skipped != "" {
			round.Skipped++
			continue
		}
		round.Closed += len(result.Closed)
	}
	return round
}
