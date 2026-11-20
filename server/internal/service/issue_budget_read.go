package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const issueBudgetReadIsolationSQL = "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"

// readSnapshot owns a pool-backed transaction. It takes no row or advisory
// lock and never publishes a replayed ledger to the writer's cache.
func (s *IssueBudgetService) readSnapshot(ctx context.Context, read func(*db.Queries) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin budget read: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, issueBudgetReadIsolationSQL); err != nil {
		return fmt.Errorf("declare budget read snapshot: %w", err)
	}
	if err := read(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("finish budget read: %w", err)
	}
	return nil
}

// IssueBudgetTime is the timestamp spelling shared by every management view.
func IssueBudgetTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func issueBudgetTicks(n pgtype.Int8) *budget.Ticks {
	if !n.Valid {
		return nil
	}
	v := budget.Ticks(n.Int64)
	return &v
}

func issueBudgetCount(n pgtype.Int8) *budget.Count {
	if !n.Valid {
		return nil
	}
	v := budget.Count(n.Int64)
	return &v
}

func issueBudgetUUID(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	s := util.UUIDToString(id)
	return &s
}

func (s *IssueBudgetService) Binding(ctx context.Context, workspaceID, issueID pgtype.UUID) (IssueBudgetBinding, error) {
	m, err := s.GetMember(ctx, workspaceID, issueID)
	if err != nil {
		return IssueBudgetBinding{}, err
	}
	return IssueBudgetBinding{
		IssueID: util.UUIDToString(m.IssueID), BudgetRootID: util.UUIDToString(m.BudgetRootID),
		IsRootIssue: m.BindSource == "root", BindSource: m.BindSource, BoundAt: IssueBudgetTime(m.BoundAt.Time),
	}, nil
}

// LookupOperation has only two outcomes: a committed receipt or nil (unknown).
// Attempt-scoped receipts are deliberately unreachable from this method.
func (s *IssueBudgetService) LookupOperation(ctx context.Context, workspaceID pgtype.UUID, scopeKind string, scopeID pgtype.UUID, key string) (*IssueBudgetOperationLookup, error) {
	if scopeKind != issueBudgetScopeIssue && scopeKind != issueBudgetScopeRoot {
		return nil, issueBudgetError(IssueBudgetInvalidRequest)
	}
	var out *IssueBudgetOperationLookup
	err := s.readSnapshot(ctx, func(q *db.Queries) error {
		op, err := q.GetIssueBudgetOperationByKey(ctx, db.GetIssueBudgetOperationByKeyParams{
			WorkspaceID: workspaceID, ScopeKind: scopeKind, ScopeID: scopeID, OperationKey: key,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read budget operation: %w", err)
		}
		var result IssueBudgetOperationResult
		if err := json.Unmarshal(op.Result, &result); err != nil {
			return fmt.Errorf("decode budget operation result: %w", err)
		}
		root, err := q.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: workspaceID, ID: op.BudgetRootID})
		if err != nil {
			return fmt.Errorf("read committed operation root: %w", err)
		}
		out = &IssueBudgetOperationLookup{State: "committed", Stale: root.PolicyRevision > int64(result.PolicyRevision), Result: result}
		return nil
	})
	return out, err
}

func (s *IssueBudgetService) Show(ctx context.Context, workspaceID, rootID pgtype.UUID) (IssueBudgetShow, error) {
	var out IssueBudgetShow
	err := s.readSnapshot(ctx, func(q *db.Queries) error {
		reader := NewIssueBudgetService(q, nil)
		state, err := reader.GetRoot(ctx, workspaceID, rootID)
		if err != nil {
			return err
		}
		ledger, err := issueBudgetLedgers.loadWithoutRootLock(ctx, q, state.Root)
		if err != nil && !errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
			return err
		}
		gaps, err := q.CountIssueBudgetUncoveredGaps(ctx, db.CountIssueBudgetUncoveredGapsParams{WorkspaceID: workspaceID, BudgetRootID: rootID})
		if err != nil {
			return fmt.Errorf("read budget coverage gaps: %w", err)
		}
		var projection *budget.Projection
		if ledger != nil {
			p := ledger.Projection()
			projection = &p
		}
		out = issueBudgetShowProjection(state, projection, gaps)
		return nil
	})
	return out, err
}

func issueBudgetShowProjection(state IssueBudgetRootState, p *budget.Projection, gaps []db.CountIssueBudgetUncoveredGapsRow) IssueBudgetShow {
	r := state.Root
	out := IssueBudgetShow{
		BudgetRootID: util.UUIDToString(r.ID), RootIssueID: util.UUIDToString(r.RootIssueID),
		AdmissionState: state.Admission(), Mode: r.Mode,
		PolicyRevision: budget.Count(r.PolicyRevision), PolicyEpoch: budget.Count(r.PolicyEpoch), LedgerRevision: budget.Count(r.LedgerRevision),
		ActivatedAt: IssueBudgetTime(r.ActivatedAt.Time), LimitTicks: issueBudgetTicks(r.LimitTicks), AttemptReserveTicks: issueBudgetTicks(r.AttemptReserveTicks),
		LedgerState: "unavailable", Unknown: IssueBudgetUnknown{ByReason: []IssueBudgetUnknownCount{}},
		Coverage: IssueBudgetReadCoverage{NotApplicable: []string{"task_without_issue", "collaboration_host_run", "plugin_external_call"}},
	}
	counts := map[string]int64{}
	if p != nil {
		out.LedgerState = "available"
		out.Amounts = &IssueBudgetAmounts{p.ActualKnown, p.Estimated, p.Reserved, p.UnknownHold, p.Overrun}
		included := p.Coverage.IncludedAttempts
		out.Coverage.IncludedAttempts = &included
		for reason, count := range p.UnknownCounts() {
			if reason == budget.ReasonPreActivationUnknown {
				out.Unknown.PreActivation += int64(count)
			} else {
				counts[string(reason)] += int64(count)
			}
		}
	}
	for _, gap := range gaps {
		if gap.Reason == "in_flight_legacy" {
			out.Unknown.PreActivation += gap.GapCount
		} else {
			counts[gap.Reason] += gap.GapCount
		}
	}
	for reason, count := range counts {
		if count > 0 {
			out.Unknown.ByReason = append(out.Unknown.ByReason, IssueBudgetUnknownCount{Reason: reason, Count: count})
		}
	}
	sort.Slice(out.Unknown.ByReason, func(i, j int) bool { return out.Unknown.ByReason[i].Reason < out.Unknown.ByReason[j].Reason })
	out.TotalKnown = p != nil && len(out.Unknown.ByReason) == 0 && out.Unknown.PreActivation == 0
	if out.TotalKnown && out.LimitTicks != nil {
		out.RemainingTicks = p.Remaining(*out.LimitTicks)
	}
	return out
}

// IssueBudgetPageParams bounds every management list to at most 100 rows.
type IssueBudgetPageParams struct {
	Limit  int
	Cursor string
}

func (p IssueBudgetPageParams) Validate() error {
	_, _, err := p.boundary()
	return err
}

func (p IssueBudgetPageParams) boundary() (pgtype.Timestamptz, pgtype.UUID, error) {
	if p.Limit < 1 || p.Limit > 100 {
		return pgtype.Timestamptz{}, pgtype.UUID{}, issueBudgetError(IssueBudgetInvalidRequest)
	}
	if p.Cursor == "" {
		return pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}, pgtype.UUID{Valid: true}, nil
	}
	return decodeIssueBudgetCursor(p.Cursor)
}

func encodeIssueBudgetCursor(t time.Time, id pgtype.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(IssueBudgetTime(t) + "," + util.UUIDToString(id)))
}

func decodeIssueBudgetCursor(cursor string) (pgtype.Timestamptz, pgtype.UUID, error) {
	invalid := func() (pgtype.Timestamptz, pgtype.UUID, error) {
		return pgtype.Timestamptz{}, pgtype.UUID{}, issueBudgetError(IssueBudgetInvalidRequest)
	}
	if cursor == "" || len(cursor) > 128 {
		return invalid()
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
	if err != nil {
		return invalid()
	}
	ts, ids, ok := strings.Cut(string(data), ",")
	if !ok {
		return invalid()
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil || t.Nanosecond()%1000 != 0 {
		return invalid()
	}
	id, err := util.ParseUUID(ids)
	if err != nil || !id.Valid || encodeIssueBudgetCursor(t, id) != cursor {
		return invalid()
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, id, nil
}

func (s *IssueBudgetService) PageObservations(ctx context.Context, workspaceID, rootID pgtype.UUID, p IssueBudgetPageParams) (IssueBudgetObservationPage, error) {
	out := IssueBudgetObservationPage{Observations: []IssueBudgetObservationView{}}
	before, id, err := p.boundary()
	if err != nil {
		return out, err
	}
	err = s.readSnapshot(ctx, func(q *db.Queries) error {
		if _, err := NewIssueBudgetService(q, nil).GetRoot(ctx, workspaceID, rootID); err != nil {
			return err
		}
		rows, err := q.ListIssueBudgetObservationPage(ctx, db.ListIssueBudgetObservationPageParams{
			WorkspaceID: workspaceID, BudgetRootID: rootID, BeforeCreatedAt: before, BeforeID: id, PageSize: int32(p.Limit + 1),
		})
		if err != nil {
			return fmt.Errorf("read budget observation page: %w", err)
		}
		if len(rows) > p.Limit {
			last := rows[p.Limit-1].IssueBudgetObservation
			cursor := encodeIssueBudgetCursor(last.CreatedAt.Time, last.ID)
			out.NextCursor = &cursor
			rows = rows[:p.Limit]
		}
		for _, row := range rows {
			o := row.IssueBudgetObservation
			out.Observations = append(out.Observations, IssueBudgetObservationView{
				ID: util.UUIDToString(o.ID), Kind: o.Kind, Reason: o.Reason, IssueID: issueBudgetUUID(o.IssueID), TaskID: issueBudgetUUID(o.TaskID),
				ClaimGeneration: issueBudgetCount(o.ClaimGeneration), AttemptID: issueBudgetUUID(o.AttemptID), Covered: row.Covered,
				Detail: json.RawMessage(o.Detail), CreatedAt: IssueBudgetTime(o.CreatedAt.Time),
			})
		}
		return nil
	})
	return out, err
}

func (s *IssueBudgetService) PageAttempts(ctx context.Context, workspaceID, rootID pgtype.UUID, p IssueBudgetPageParams) (IssueBudgetAttemptPage, error) {
	out := IssueBudgetAttemptPage{LedgerState: "available", Attempts: []IssueBudgetAttemptView{}}
	before, id, err := p.boundary()
	if err != nil {
		return out, err
	}
	err = s.readSnapshot(ctx, func(q *db.Queries) error {
		state, err := NewIssueBudgetService(q, nil).GetRoot(ctx, workspaceID, rootID)
		if err != nil {
			return err
		}
		rows, err := q.ListIssueBudgetAttemptPage(ctx, db.ListIssueBudgetAttemptPageParams{
			WorkspaceID: workspaceID, BudgetRootID: rootID, BeforeCreatedAt: before, BeforeID: id, PageSize: int32(p.Limit + 1),
		})
		if err != nil {
			return fmt.Errorf("read budget attempt page: %w", err)
		}
		ledger, err := issueBudgetLedgers.loadWithoutRootLock(ctx, q, state.Root)
		if errors.Is(err, ErrIssueBudgetLedgerUnavailable) {
			out.LedgerState = "unavailable"
		} else if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			last := rows[p.Limit-1]
			cursor := encodeIssueBudgetCursor(last.CreatedAt.Time, last.ID)
			out.NextCursor = &cursor
			rows = rows[:p.Limit]
		}
		for _, row := range rows {
			v := IssueBudgetAttemptView{
				AttemptID: util.UUIDToString(row.ID), TaskID: util.UUIDToString(row.TaskID), ClaimGeneration: budget.Count(row.ClaimGeneration),
				IssueID: issueBudgetUUID(row.IssueID), ContextParentIssueID: issueBudgetUUID(row.ContextParentIssueID),
				AgentID: util.UUIDToString(row.AgentID), RuntimeID: util.UUIDToString(row.RuntimeID), State: row.State,
				ReservedTicks: issueBudgetTicks(row.ReservedTicks), CreatedAt: IssueBudgetTime(row.CreatedAt.Time),
			}
			if row.ProtocolVersion.Valid {
				version := row.ProtocolVersion.Int32
				v.ProtocolVersion = &version
			}
			if ledger != nil {
				if reservation, ok := ledger.Reservation(v.AttemptID); ok {
					v.Reservation = &reservation
				}
			}
			out.Attempts = append(out.Attempts, v)
		}
		return nil
	})
	return out, err
}
