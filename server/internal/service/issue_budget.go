package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/admission"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// IssueBudgetService manages parent-task budget roots: enabling one, and the
// close, reopen, limit and mode changes on an existing one. Each call owns its
// transaction. Callers authorize the credential and issue access; each write
// rechecks current owner/admin membership before locks or an original-key
// replay. Every query is scoped by the workspace the caller passes.
//
// Member rows written by the issue create, move and delete paths are in
// issue_budget_binding.go.
type IssueBudgetService struct {
	Queries   *db.Queries
	TxStarter TxStarter
}

func NewIssueBudgetService(q *db.Queries, tx TxStarter) *IssueBudgetService {
	return &IssueBudgetService{Queries: q, TxStarter: tx}
}

// Refusal codes of IssueBudgetError.
const (
	IssueBudgetForbidden = "budget_forbidden"
	// IssueBudgetNotFound covers a missing workspace, issue or root, and a
	// root or issue of another workspace: the caller cannot tell them apart.
	IssueBudgetNotFound = "not_found"
	// IssueBudgetInvalidRequest is a malformed key, reason, limit or mode.
	IssueBudgetInvalidRequest = "invalid_request"
	// IssueBudgetNotEligible is an issue that is not an ordinary formal one.
	IssueBudgetNotEligible = "not_eligible"
	// IssueBudgetAlreadyEnabled is an issue that is, or once was, a root's
	// root issue. The slot is taken for life.
	IssueBudgetAlreadyEnabled = "already_enabled"
	// IssueBudgetRootConflict is an issue that belongs to another root, or
	// has a descendant that belongs to one.
	IssueBudgetRootConflict = "root_conflict"
	// IssueBudgetParentCycle is a parent loop through the issue.
	IssueBudgetParentCycle = "parent_cycle"
	// IssueBudgetOperationConflict is an original key reused with a
	// different intent.
	IssueBudgetOperationConflict = "operation_conflict"
	// IssueBudgetPolicyRevisionConflict is a stale expected policy revision.
	IssueBudgetPolicyRevisionConflict = "policy_revision_conflict"
	// IssueBudgetRootDeleted is a reopen of a root whose root issue was
	// deleted.
	IssueBudgetRootDeleted = "root_deleted"
	// IssueBudgetStateConflict is a change that would change nothing: close
	// of a closed root, reopen of an open one, the mode or the limit the root
	// already has. Every committed operation is one new policy revision.
	IssueBudgetStateConflict = "state_conflict"
)

// IssueBudgetError is a refusal decided by the budget rules. Nothing was
// written when one is returned.
type IssueBudgetError struct{ Code string }

func (e *IssueBudgetError) Error() string { return "issue budget: " + e.Code }

func issueBudgetError(code string) error { return &IssueBudgetError{Code: code} }

// IssueBudgetErrorCode returns the refusal code of err, or "" when err is not
// a budget refusal.
func IssueBudgetErrorCode(err error) string {
	var refusal *IssueBudgetError
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return ""
}

// Operation kinds, as stored on the operation row.
const (
	IssueBudgetOperationEnable   = "enable"
	IssueBudgetOperationClose    = "close"
	IssueBudgetOperationReopen   = "reopen"
	IssueBudgetOperationSetLimit = "set_limit"
	IssueBudgetOperationSetMode  = "set_mode"
)

// Root modes and admission states, as stored on the root row.
const (
	IssueBudgetModeOff     = "off"
	IssueBudgetModeObserve = "observe"

	IssueBudgetAdmissionOpen       = "open"
	IssueBudgetAdmissionClosed     = "closed"
	IssueBudgetAdmissionTombstoned = "tombstoned"
)

// IssueBudgetEnableParams enables a root on one issue.
type IssueBudgetEnableParams struct {
	WorkspaceID pgtype.UUID
	IssueID     pgtype.UUID
	// OperationKey is the caller's original key. The same key with the same
	// intent returns the committed result again.
	OperationKey string
	// LimitTicks is nil when no limit is configured.
	LimitTicks *budget.Ticks
	// AttemptReserveTicks is what the manager declares each execution reserves;
	// nil when nothing is declared. It is positive when set.
	AttemptReserveTicks *budget.Ticks
	Reason              string
	// ActorUserID is the already authorized operator.
	ActorUserID pgtype.UUID
}

// IssueBudgetManageParams addresses one existing root for a close, reopen,
// limit or mode change.
type IssueBudgetManageParams struct {
	WorkspaceID  pgtype.UUID
	BudgetRootID pgtype.UUID
	OperationKey string
	// ExpectedPolicyRevision must equal the root's current policy revision.
	// The service never substitutes the latest value for the caller.
	ExpectedPolicyRevision int64
	Reason                 string
	ActorUserID            pgtype.UUID
}

// IssueBudgetOperationResult is the committed outcome of an operation. It is
// stored with the operation and returned unchanged when the original key is
// presented again with the same intent.
type IssueBudgetOperationResult struct {
	OperationID         string        `json:"operation_id"`
	OperationKind       string        `json:"operation_kind"`
	BudgetRootID        string        `json:"budget_root_id"`
	RootIssueID         string        `json:"root_issue_id"`
	AdmissionState      string        `json:"admission_state"`
	Mode                string        `json:"mode"`
	PolicyRevision      budget.Count  `json:"policy_revision"`
	PolicyEpoch         budget.Count  `json:"policy_epoch"`
	LimitTicks          *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks `json:"attempt_reserve_ticks"`
	Reason              string        `json:"reason"`
	// Set by enable only.
	ActivatedAt      string `json:"activated_at,omitempty"`
	MemberCount      int    `json:"member_count,omitempty"`
	InFlightGapCount int    `json:"in_flight_gap_count,omitempty"`
	// Replayed is true when this is the stored result of an earlier commit
	// and the call wrote nothing. It is not part of the stored result.
	Replayed bool `json:"-"`
}

// IssueBudgetRootState is a root row with the one fact that is not on it:
// whether its root issue was deleted, which is read from that issue's own
// member row.
type IssueBudgetRootState struct {
	Root db.IssueBudgetRoot
	// TombstonedAt is valid when the root issue was deleted.
	TombstonedAt pgtype.Timestamptz
}

// Admission is the admission axis as a reader sees it: a deleted root issue
// wins over the stored open or closed state.
func (s IssueBudgetRootState) Admission() string {
	if s.TombstonedAt.Valid {
		return IssueBudgetAdmissionTombstoned
	}
	return s.Root.AdmissionState
}

// IssueBudgetObservation is an observation row with the read-time judgement
// of whether a later attempt covered it.
type IssueBudgetObservation struct {
	Row db.IssueBudgetObservation
	// Covered is true for an in-flight gap whose task and claim generation
	// have an attempt row by now.
	Covered bool
}

// Limits on what a caller may send. The key limit is the column's.
const (
	issueBudgetMaxKeyBytes    = 200
	issueBudgetMaxReasonBytes = 2000
)

// issueBudgetIntentDomain separates the intent digest from every other
// SHA-256 this product stores.
const issueBudgetIntentDomain = "budget-operation-intent-v1\n"

// Scopes an original key is unique within.
const (
	issueBudgetScopeIssue = "issue"
	issueBudgetScopeRoot  = "root"
)

// issueBudgetIntent is what an original key stands for. Two requests with the
// same key are the same request only when every field here is equal. The
// operator is not part of it: a retry by another authorized operator is still
// the same change. The field order is the digest's canonical form, pinned by a
// golden vector in the tests: a field added, renamed or moved here changes what
// every stored key stands for.
type issueBudgetIntent struct {
	Kind                   string        `json:"kind"`
	WorkspaceID            string        `json:"workspace_id"`
	ScopeKind              string        `json:"scope_kind"`
	ScopeID                string        `json:"scope_id"`
	ExpectedPolicyRevision *budget.Count `json:"expected_policy_revision"`
	LimitTicks             *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks    *budget.Ticks `json:"attempt_reserve_ticks"`
	Mode                   string        `json:"mode"`
	Reason                 string        `json:"reason"`
}

func (i issueBudgetIntent) digest() (string, error) {
	encoded, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("encode budget intent: %w", err)
	}
	sum := sha256.Sum256(append([]byte(issueBudgetIntentDomain), encoded...))
	return hex.EncodeToString(sum[:]), nil
}

// issueBudgetRequest checks the parts every operation shares and returns the
// reason as it is stored.
func issueBudgetRequest(workspaceID, targetID pgtype.UUID, key, reason string, actor pgtype.UUID) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case !workspaceID.Valid, !targetID.Valid, !actor.Valid:
	case key == "", len(key) > issueBudgetMaxKeyBytes, !utf8.ValidString(key), strings.ContainsRune(key, 0):
	case reason == "", len(reason) > issueBudgetMaxReasonBytes, !utf8.ValidString(reason), strings.ContainsRune(reason, 0):
	default:
		return reason, nil
	}
	return "", issueBudgetError(IssueBudgetInvalidRequest)
}

// issueBudgetIsolationSQL declares the level every budget operation runs at.
// It is the statement workflowInTx uses.
const issueBudgetIsolationSQL = "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"

// transact runs one operation in its own transaction.
//
// Its first statement declares READ COMMITTED. The fence and the binding lock
// close only because a statement sent after a lock was granted sees what
// committed while the transaction waited for it; a transaction that fixed its
// snapshot earlier would not, and enabling reads its in-flight tasks and
// answers a replay on exactly that. When the starter is an open transaction
// this runs in a savepoint of it: PostgreSQL accepts the statement there when
// the outer transaction already is READ COMMITTED and rejects it otherwise, so
// an operation started from another level fails before it reads anything.
func (s *IssueBudgetService) transact(ctx context.Context, write func(tx pgx.Tx, q *db.Queries) (IssueBudgetOperationResult, error)) (IssueBudgetOperationResult, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("begin budget operation: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, issueBudgetIsolationSQL); err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("declare budget operation isolation level: %w", err)
	}
	result, err := write(tx, s.Queries.WithTx(tx))
	if err != nil {
		return IssueBudgetOperationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("commit budget operation: %w", err)
	}
	return result, nil
}

// transactManagement adds the human membership check without granting the
// system lifecycle writer a human identity or changing its transaction contract.
func (s *IssueBudgetService) transactManagement(ctx context.Context, workspaceID, actorID pgtype.UUID, write func(pgx.Tx, *db.Queries) (IssueBudgetOperationResult, error)) (IssueBudgetOperationResult, error) {
	return s.transact(ctx, func(tx pgx.Tx, q *db.Queries) (IssueBudgetOperationResult, error) {
		member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actorID, WorkspaceID: workspaceID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && member.Role != "owner" && member.Role != "admin") {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetForbidden)
		}
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("recheck budget operator membership: %w", err)
		}
		return write(tx, q)
	})
}

// issueBudgetCommittedOperation looks an original key up in its scope. found
// is false when the key was never committed there. A key committed with the
// same intent returns its stored result; with another intent it is a conflict.
func issueBudgetCommittedOperation(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, scopeKind string, scopeID pgtype.UUID, key, digest string) (result IssueBudgetOperationResult, found bool, err error) {
	operation, err := q.GetIssueBudgetOperationByKey(ctx, db.GetIssueBudgetOperationByKeyParams{
		WorkspaceID: workspaceID, ScopeKind: scopeKind, ScopeID: scopeID, OperationKey: key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return IssueBudgetOperationResult{}, false, nil
	}
	if err != nil {
		return IssueBudgetOperationResult{}, false, fmt.Errorf("read budget operation: %w", err)
	}
	if operation.IntentDigest != digest {
		return IssueBudgetOperationResult{}, true, issueBudgetError(IssueBudgetOperationConflict)
	}
	if err := json.Unmarshal(operation.Result, &result); err != nil {
		return IssueBudgetOperationResult{}, true, fmt.Errorf("decode budget operation result: %w", err)
	}
	result.Replayed = true
	return result, true, nil
}

// recordIssueBudgetOperation stores the committed outcome under its original
// key. It is the last statement of every operation.
func recordIssueBudgetOperation(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, scopeKind string, scopeID pgtype.UUID, key, digest string, actor pgtype.UUID, result IssueBudgetOperationResult) (IssueBudgetOperationResult, error) {
	id := dbid.NewV7()
	result.OperationID = util.UUIDToString(id)
	encoded, err := json.Marshal(result)
	if err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("encode budget operation result: %w", err)
	}
	rootID, err := util.ParseUUID(result.BudgetRootID)
	if err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("budget operation root: %w", err)
	}
	if _, err := q.InsertIssueBudgetOperation(ctx, db.InsertIssueBudgetOperationParams{
		ID:            id,
		WorkspaceID:   workspaceID,
		BudgetRootID:  rootID,
		ScopeKind:     scopeKind,
		ScopeID:       scopeID,
		OperationKey:  key,
		OperationKind: result.OperationKind,
		IntentDigest:  digest,
		ActorType:     "member",
		ActorID:       actor,
		Result:        encoded,
	}); err != nil {
		return IssueBudgetOperationResult{}, fmt.Errorf("record budget operation: %w", err)
	}
	return result, nil
}

func issueBudgetResult(kind string, root db.IssueBudgetRoot, reason string) IssueBudgetOperationResult {
	result := IssueBudgetOperationResult{
		OperationKind:  kind,
		BudgetRootID:   util.UUIDToString(root.ID),
		RootIssueID:    util.UUIDToString(root.RootIssueID),
		AdmissionState: root.AdmissionState,
		Mode:           root.Mode,
		PolicyRevision: budget.Count(root.PolicyRevision),
		PolicyEpoch:    budget.Count(root.PolicyEpoch),
		Reason:         reason,
	}
	if root.LimitTicks.Valid {
		limit := budget.Ticks(root.LimitTicks.Int64)
		result.LimitTicks = &limit
	}
	if root.AttemptReserveTicks.Valid {
		reserve := budget.Ticks(root.AttemptReserveTicks.Int64)
		result.AttemptReserveTicks = &reserve
	}
	return result
}

// issueBudgetLimit is an optional amount as its column stores it: both the
// limit and the declared reserve are NULL when absent.
func issueBudgetLimit(limit *budget.Ticks) pgtype.Int8 {
	if limit == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: int64(*limit), Valid: true}
}

// issueBudgetAmounts checks the two amounts a root is configured with. Zero is
// a limit; a declared reserve is a positive amount, and the way to declare
// nothing is nil.
func issueBudgetAmounts(limit, attemptReserve *budget.Ticks) error {
	if limit != nil && *limit < 0 {
		return issueBudgetError(IssueBudgetInvalidRequest)
	}
	if attemptReserve != nil && *attemptReserve <= 0 {
		return issueBudgetError(IssueBudgetInvalidRequest)
	}
	return nil
}

// Enable makes an ordinary formal issue the root issue of a new budget root
// and binds every descendant it has at that moment.
//
// It takes the workspace fence before it reads anything, so no issue can be
// created, moved or deleted in the workspace, and no other root enabled, until
// it commits: what it reads is the tree it binds. It is the only place a root
// row is inserted. The statements below run in lock order; nothing after the
// fence waits on a row lock.
//
// Once every refusal check has passed it takes the workspace binding lock, and
// only then reads the tasks daemons already hold, samples the activation time
// and writes. A claim reads its member row under the shared side of that lock:
// the claims that read "unbound" have committed when the lock is granted, so
// the scan finds their tasks, and no later claim gets through before this
// commits. The activation time is sampled after the scan, from the statement
// clock: a task that ended between an earlier sample and the scan would lie
// after the activation time with neither an attempt nor a gap. A refused or
// replayed enable takes no binding lock.
func (s *IssueBudgetService) Enable(ctx context.Context, p IssueBudgetEnableParams) (IssueBudgetOperationResult, error) {
	reason, err := issueBudgetRequest(p.WorkspaceID, p.IssueID, p.OperationKey, p.Reason, p.ActorUserID)
	if err != nil {
		return IssueBudgetOperationResult{}, err
	}
	if err := issueBudgetAmounts(p.LimitTicks, p.AttemptReserveTicks); err != nil {
		return IssueBudgetOperationResult{}, err
	}
	digest, err := issueBudgetIntent{
		Kind: IssueBudgetOperationEnable, WorkspaceID: util.UUIDToString(p.WorkspaceID),
		ScopeKind: issueBudgetScopeIssue, ScopeID: util.UUIDToString(p.IssueID),
		LimitTicks: p.LimitTicks, AttemptReserveTicks: p.AttemptReserveTicks, Reason: reason,
	}.digest()
	if err != nil {
		return IssueBudgetOperationResult{}, err
	}
	return s.transactManagement(ctx, p.WorkspaceID, p.ActorUserID, func(tx pgx.Tx, q *db.Queries) (IssueBudgetOperationResult, error) {
		// The fence's own precondition: the workspace row, shared, so a
		// workspace being deleted is waited for and then not found.
		if _, err := q.LockWorkspaceForChatSessionCreate(ctx, p.WorkspaceID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetNotFound)
			}
			return IssueBudgetOperationResult{}, fmt.Errorf("lock workspace: %w", err)
		}
		if err := iteration.LockWorkspace(ctx, tx, p.WorkspaceID); err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("take workspace fence: %w", err)
		}

		// The slot is decided by the root table alone, before the issue is
		// read: it stays taken after the root issue is deleted.
		_, err := q.GetIssueBudgetRootByIssue(ctx, db.GetIssueBudgetRootByIssueParams{WorkspaceID: p.WorkspaceID, RootIssueID: p.IssueID})
		if err == nil {
			result, found, err := issueBudgetCommittedOperation(ctx, q, p.WorkspaceID, issueBudgetScopeIssue, p.IssueID, p.OperationKey, digest)
			if err != nil {
				return IssueBudgetOperationResult{}, err
			}
			if !found {
				return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetAlreadyEnabled)
			}
			return result, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return IssueBudgetOperationResult{}, fmt.Errorf("read budget root: %w", err)
		}

		issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: p.IssueID, WorkspaceID: p.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetNotFound)
		}
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("read issue: %w", err)
		}
		// Ordinary formal work only: past triage, and not a collaboration
		// issue, whose runs are accounted on their own.
		if !admission.Formal(issue.AdmissionStatus) {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetNotEligible)
		}
		shared, err := q.IsCollaborationIssue(ctx, issue.ID)
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("read collaboration state: %w", err)
		}
		if shared {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetNotEligible)
		}
		if _, bound, err := issueBudgetMember(ctx, q, p.WorkspaceID, issue.ID); err != nil {
			return IssueBudgetOperationResult{}, err
		} else if bound {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetRootConflict)
		}

		descendants, err := q.ListIssueBudgetDescendants(ctx, db.ListIssueBudgetDescendantsParams{WorkspaceID: p.WorkspaceID, IssueID: issue.ID})
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("walk descendants: %w", err)
		}
		issueIDs := make([]pgtype.UUID, 0, len(descendants))
		parentIDs := make([]pgtype.UUID, 0, len(descendants))
		conflict := false
		for _, row := range descendants {
			if row.Cycle {
				// A loop through the issue outranks a conflict below it:
				// the tree to bind is not a tree.
				return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetParentCycle)
			}
			conflict = conflict || row.MemberBudgetRootID.Valid
			issueIDs = append(issueIDs, row.IssueID)
			parentIDs = append(parentIDs, row.ParentIssueID)
		}
		if conflict {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetRootConflict)
		}

		// Every refusal is behind us; from here on this transaction binds.
		if err := q.LockIssueBudgetBinding(ctx, p.WorkspaceID); err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("take budget binding lock: %w", err)
		}
		inFlight, err := listIssueBudgetInFlightTasks(ctx, q, p.WorkspaceID, append([]pgtype.UUID{issue.ID}, issueIDs...))
		if err != nil {
			return IssueBudgetOperationResult{}, err
		}
		activatedAt, err := iteration.SampleBusinessTime(ctx, tx, nil)
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("sample activation time: %w", err)
		}
		root, err := q.InsertIssueBudgetRoot(ctx, db.InsertIssueBudgetRootParams{
			ID:                  dbid.NewV7(),
			WorkspaceID:         p.WorkspaceID,
			RootIssueID:         issue.ID,
			LimitTicks:          issueBudgetLimit(p.LimitTicks),
			AttemptReserveTicks: issueBudgetLimit(p.AttemptReserveTicks),
			ActivatedAt:         pgtype.Timestamptz{Time: activatedAt, Valid: true},
		})
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("create budget root: %w", err)
		}
		if _, err := q.InsertIssueBudgetMember(ctx, db.InsertIssueBudgetMemberParams{
			WorkspaceID:    p.WorkspaceID,
			IssueID:        issue.ID,
			BudgetRootID:   root.ID,
			BindSource:     issueBudgetBindRoot,
			BindingVersion: issueBudgetBindingVersion,
		}); err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("bind root issue: %w", err)
		}
		if err := insertIssueBudgetMembers(ctx, q, p.WorkspaceID, root.ID, issueBudgetBindActivation, issueIDs, parentIDs); err != nil {
			return IssueBudgetOperationResult{}, err
		}
		if err := recordIssueBudgetInFlightGaps(ctx, q, p.WorkspaceID, root.ID, issueBudgetReasonInFlightLegacy, inFlight); err != nil {
			return IssueBudgetOperationResult{}, err
		}

		result := issueBudgetResult(IssueBudgetOperationEnable, root, reason)
		result.ActivatedAt = activatedAt.Format(time.RFC3339Nano)
		result.MemberCount = 1 + len(issueIDs)
		result.InFlightGapCount = len(inFlight)
		return recordIssueBudgetOperation(ctx, q, p.WorkspaceID, issueBudgetScopeIssue, p.IssueID, p.OperationKey, digest, p.ActorUserID, result)
	})
}

// Close stops admission on a root. No amount changes.
func (s *IssueBudgetService) Close(ctx context.Context, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
	return s.manage(ctx, p, IssueBudgetOperationClose, nil, nil, "", func(_ *db.Queries, root db.IssueBudgetRoot, change *db.UpdateIssueBudgetRootPolicyParams) error {
		if root.AdmissionState == IssueBudgetAdmissionClosed {
			return issueBudgetError(IssueBudgetStateConflict)
		}
		change.AdmissionState = IssueBudgetAdmissionClosed
		return nil
	})
}

// Reopen opens the same root again: same id, same limit, same amounts. A root
// whose root issue was deleted stays closed for good.
func (s *IssueBudgetService) Reopen(ctx context.Context, p IssueBudgetManageParams) (IssueBudgetOperationResult, error) {
	return s.manage(ctx, p, IssueBudgetOperationReopen, nil, nil, "", func(q *db.Queries, root db.IssueBudgetRoot, change *db.UpdateIssueBudgetRootPolicyParams) error {
		member, bound, err := issueBudgetMember(ctx, q, root.WorkspaceID, root.RootIssueID)
		if err != nil {
			return err
		}
		if !bound {
			return fmt.Errorf("budget root %s has no member row for its root issue", util.UUIDToString(root.ID))
		}
		if member.TombstonedAt.Valid {
			return issueBudgetError(IssueBudgetRootDeleted)
		}
		if root.AdmissionState == IssueBudgetAdmissionOpen {
			return issueBudgetError(IssueBudgetStateConflict)
		}
		change.AdmissionState = IssueBudgetAdmissionOpen
		return nil
	})
}

// SetLimit sets the two amounts a root is configured with: its limit and the
// reserve each execution is declared to take. Both are given on every call and
// replace what the root has as a whole; nil removes the one it stands for.
// There is no way to change one and keep the other without stating it, and the
// request is a no-op only when both already are what the root has.
func (s *IssueBudgetService) SetLimit(ctx context.Context, p IssueBudgetManageParams, limit, attemptReserve *budget.Ticks) (IssueBudgetOperationResult, error) {
	if err := issueBudgetAmounts(limit, attemptReserve); err != nil {
		return IssueBudgetOperationResult{}, err
	}
	return s.manage(ctx, p, IssueBudgetOperationSetLimit, limit, attemptReserve, "", func(_ *db.Queries, root db.IssueBudgetRoot, change *db.UpdateIssueBudgetRootPolicyParams) error {
		nextLimit, nextReserve := issueBudgetLimit(limit), issueBudgetLimit(attemptReserve)
		if nextLimit == root.LimitTicks && nextReserve == root.AttemptReserveTicks {
			return issueBudgetError(IssueBudgetStateConflict)
		}
		change.LimitTicks, change.AttemptReserveTicks = nextLimit, nextReserve
		return nil
	})
}

// SetMode switches a root between off and observe. Every return to observe
// starts a new policy epoch; an epoch is never reused.
func (s *IssueBudgetService) SetMode(ctx context.Context, p IssueBudgetManageParams, mode string) (IssueBudgetOperationResult, error) {
	if mode != IssueBudgetModeOff && mode != IssueBudgetModeObserve {
		return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetInvalidRequest)
	}
	return s.manage(ctx, p, IssueBudgetOperationSetMode, nil, nil, mode, func(q *db.Queries, root db.IssueBudgetRoot, change *db.UpdateIssueBudgetRootPolicyParams) error {
		if root.Mode == mode {
			return issueBudgetError(IssueBudgetStateConflict)
		}
		change.Mode = mode
		if mode == IssueBudgetModeObserve {
			change.PolicyEpoch = root.PolicyEpoch + 1
			return insertIssueBudgetObservation(ctx, q, db.InsertIssueBudgetObservationParams{
				WorkspaceID: root.WorkspaceID, BudgetRootID: root.ID,
				Kind: issueBudgetObservationCoverageGap, Reason: issueBudgetReasonObservationOff,
			}, map[string]string{"policy_epoch": fmt.Sprint(change.PolicyEpoch)})
		}
		return nil
	})
}

// manage is the shape every change to an existing root has: lock the root,
// answer a replay, check the revision the caller decided against, apply, and
// record the operation.
//
// The root lock is the first lock of the transaction, and after it only
// budget tables are read or written: no issue, task, user, agent, workspace
// or runtime row, no fence and no binding lock. decide may read budget tables
// through q; it fills in what changes and returns a refusal to change nothing.
// What decide leaves alone keeps the value the locked row has.
func (s *IssueBudgetService) manage(ctx context.Context, p IssueBudgetManageParams, kind string, limit, attemptReserve *budget.Ticks, mode string,
	decide func(q *db.Queries, root db.IssueBudgetRoot, change *db.UpdateIssueBudgetRootPolicyParams) error) (IssueBudgetOperationResult, error) {
	reason, err := issueBudgetRequest(p.WorkspaceID, p.BudgetRootID, p.OperationKey, p.Reason, p.ActorUserID)
	if err != nil {
		return IssueBudgetOperationResult{}, err
	}
	// A revision is a count: a negative one is malformed, not stale, and the
	// intent digest has no encoding for it.
	if p.ExpectedPolicyRevision < 0 {
		return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetInvalidRequest)
	}
	expected := budget.Count(p.ExpectedPolicyRevision)
	digest, err := issueBudgetIntent{
		Kind: kind, WorkspaceID: util.UUIDToString(p.WorkspaceID),
		ScopeKind: issueBudgetScopeRoot, ScopeID: util.UUIDToString(p.BudgetRootID),
		ExpectedPolicyRevision: &expected, LimitTicks: limit, AttemptReserveTicks: attemptReserve, Mode: mode, Reason: reason,
	}.digest()
	if err != nil {
		return IssueBudgetOperationResult{}, err
	}
	return s.transactManagement(ctx, p.WorkspaceID, p.ActorUserID, func(_ pgx.Tx, q *db.Queries) (IssueBudgetOperationResult, error) {
		root, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: p.WorkspaceID, ID: p.BudgetRootID})
		if errors.Is(err, pgx.ErrNoRows) {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetNotFound)
		}
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("lock budget root: %w", err)
		}
		// A replay is answered before the revision is compared: the root has
		// moved on since the original request, by that request at least.
		if result, found, err := issueBudgetCommittedOperation(ctx, q, p.WorkspaceID, issueBudgetScopeRoot, root.ID, p.OperationKey, digest); err != nil || found {
			return result, err
		}
		if p.ExpectedPolicyRevision != root.PolicyRevision {
			return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetPolicyRevisionConflict)
		}
		change := db.UpdateIssueBudgetRootPolicyParams{
			AdmissionState:      root.AdmissionState,
			Mode:                root.Mode,
			LimitTicks:          root.LimitTicks,
			AttemptReserveTicks: root.AttemptReserveTicks,
			PolicyEpoch:         root.PolicyEpoch,
			WorkspaceID:         root.WorkspaceID,
			ID:                  root.ID,
		}
		if err := decide(q, root, &change); err != nil {
			return IssueBudgetOperationResult{}, err
		}
		updated, err := q.UpdateIssueBudgetRootPolicy(ctx, change)
		if err != nil {
			return IssueBudgetOperationResult{}, fmt.Errorf("update budget root policy: %w", err)
		}
		return recordIssueBudgetOperation(ctx, q, p.WorkspaceID, issueBudgetScopeRoot, root.ID, p.OperationKey, digest, p.ActorUserID,
			issueBudgetResult(kind, updated, reason))
	})
}

// GetRootByIssue reads the root whose root issue is issueID.
func (s *IssueBudgetService) GetRootByIssue(ctx context.Context, workspaceID, issueID pgtype.UUID) (IssueBudgetRootState, error) {
	root, err := s.Queries.GetIssueBudgetRootByIssue(ctx, db.GetIssueBudgetRootByIssueParams{WorkspaceID: workspaceID, RootIssueID: issueID})
	return s.rootState(ctx, root, err)
}

// GetRoot reads a root by its own id.
func (s *IssueBudgetService) GetRoot(ctx context.Context, workspaceID, rootID pgtype.UUID) (IssueBudgetRootState, error) {
	root, err := s.Queries.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: workspaceID, ID: rootID})
	return s.rootState(ctx, root, err)
}

func (s *IssueBudgetService) rootState(ctx context.Context, root db.IssueBudgetRoot, err error) (IssueBudgetRootState, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return IssueBudgetRootState{}, issueBudgetError(IssueBudgetNotFound)
	}
	if err != nil {
		return IssueBudgetRootState{}, fmt.Errorf("read budget root: %w", err)
	}
	member, bound, err := issueBudgetMember(ctx, s.Queries, root.WorkspaceID, root.RootIssueID)
	if err != nil {
		return IssueBudgetRootState{}, err
	}
	if !bound {
		return IssueBudgetRootState{}, fmt.Errorf("budget root %s has no member row for its root issue", util.UUIDToString(root.ID))
	}
	return IssueBudgetRootState{Root: root, TombstonedAt: member.TombstonedAt}, nil
}

// GetMember reads the member row of an issue.
func (s *IssueBudgetService) GetMember(ctx context.Context, workspaceID, issueID pgtype.UUID) (db.IssueBudgetMember, error) {
	member, bound, err := issueBudgetMember(ctx, s.Queries, workspaceID, issueID)
	if err != nil {
		return db.IssueBudgetMember{}, err
	}
	if !bound {
		return db.IssueBudgetMember{}, issueBudgetError(IssueBudgetNotFound)
	}
	return member, nil
}

// ListObservations reads the observations of a root, oldest first.
func (s *IssueBudgetService) ListObservations(ctx context.Context, workspaceID, rootID pgtype.UUID) ([]IssueBudgetObservation, error) {
	if _, err := s.Queries.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: workspaceID, ID: rootID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, issueBudgetError(IssueBudgetNotFound)
		}
		return nil, fmt.Errorf("read budget root: %w", err)
	}
	rows, err := s.Queries.ListIssueBudgetObservations(ctx, db.ListIssueBudgetObservationsParams{WorkspaceID: workspaceID, BudgetRootID: rootID})
	if err != nil {
		return nil, fmt.Errorf("list budget observations: %w", err)
	}
	observations := make([]IssueBudgetObservation, 0, len(rows))
	for _, row := range rows {
		observations = append(observations, IssueBudgetObservation{Row: row.IssueBudgetObservation, Covered: row.Covered})
	}
	return observations, nil
}
