package iteration

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransactionBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// RunTransaction repeats the whole callback only for database conflicts whose
// transaction has been rolled back. The callback must authorize every attempt,
// acquire locks in the documented order, and keep all external effects out of
// the transaction. An uncertain commit is returned for request-ID reconciliation.
func RunTransaction(ctx context.Context, pool TransactionBeginner, write func(context.Context, pgx.Tx) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err == nil {
			err = runTransactionAttempt(ctx, tx, write)
		}
		if err == nil {
			return nil
		}
		if !retryableTransactionError(err) {
			return err
		}
		if attempt == 2 {
			return &OperationError{Status: 503, Code: "iteration_retry_exhausted", Message: "Iteration write conflicted; retry the original request", Retryable: true}
		}
		delay := []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}[attempt] + time.Duration(rand.IntN(26))*time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	panic("unreachable")
}

func runTransactionAttempt(ctx context.Context, tx pgx.Tx, write func(context.Context, pgx.Tx) error) error {
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err := write(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func retryableTransactionError(err error) bool {
	var collision *operationRequestCollision
	if errors.As(err, &collision) {
		return true
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01", "55P03":
		return true
	default:
		return false
	}
}

// WorkspaceFenceSQL is shared with writers that pipeline their ordered locks.
const WorkspaceFenceSQL = `SELECT pg_advisory_xact_lock(hashtextextended('iteration:' || $1::uuid::text, 0))`

// LockWorkspace is the iteration-specific fence, not an authorization helper.
// Before calling, take workspace KEY SHARE, the flow's existing T1/member
// fences, and the status catalogue fence. Call it before any iteration setting,
// iteration, attachment or issue row lock, including writes to unrelated issues.
func LockWorkspace(ctx context.Context, tx pgx.Tx, workspaceID pgtype.UUID) error {
	if !workspaceID.Valid {
		return fmt.Errorf("iteration fence requires a workspace UUID")
	}
	_, err := tx.Exec(ctx, WorkspaceFenceSQL, workspaceID)
	return err
}

// SampleBusinessTime must run after all required fences and row locks, before
// the first business write. A test clock is explicit; production uses database
// wall time rather than the transaction's start timestamp.
func SampleBusinessTime(ctx context.Context, tx pgx.Tx, clock func() time.Time) (time.Time, error) {
	if clock != nil {
		return clock().UTC(), nil
	}
	var sampled time.Time
	err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled)
	return sampled.UTC(), err
}

// EventTime preserves sequence order through wall-clock corrections. The
// unmodified sample is separately persisted in iteration_event.sampled_at.
func EventTime(sampled, previous time.Time) time.Time {
	if sampled.Before(previous) {
		return previous
	}
	return sampled
}
