package iteration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionFixture struct {
	pgx.Tx
	commits, rollbacks int
	commitError        error
}

func (tx *transactionFixture) Commit(context.Context) error   { tx.commits++; return tx.commitError }
func (tx *transactionFixture) Rollback(context.Context) error { tx.rollbacks++; return nil }

type transactionPoolFixture struct {
	txs  []*transactionFixture
	opts []pgx.TxOptions
}

func (p *transactionPoolFixture) BeginTx(_ context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	p.opts = append(p.opts, opts)
	tx := p.txs[len(p.opts)-1]
	return tx, nil
}

func TestRunTransactionRetriesOnlyRolledBackDatabaseConflicts(t *testing.T) {
	pool := &transactionPoolFixture{txs: []*transactionFixture{{}, {}, {}}}
	attempts := 0
	err := RunTransaction(context.Background(), pool, func(context.Context, pgx.Tx) error {
		attempts++
		if attempts < 3 {
			return &pgconn.PgError{Code: "55P03"}
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
	for i, tx := range pool.txs {
		if i < 2 && (tx.rollbacks != 1 || tx.commits != 0) {
			t.Fatalf("retry %d was not rolled back: %+v", i, tx)
		}
		if pool.opts[i].IsoLevel != pgx.ReadCommitted {
			t.Fatalf("write isolation=%v", pool.opts[i])
		}
	}
	if pool.txs[2].commits != 1 {
		t.Fatal("successful attempt did not commit exactly once")
	}
}

func TestRunTransactionDoesNotRetryBusinessConflictsOrUnknownCommits(t *testing.T) {
	for _, unknownCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "business conflict", true: "unknown commit"}[unknownCommit], func(t *testing.T) {
			want := errors.New("response lost")
			tx := &transactionFixture{}
			if unknownCommit {
				tx.commitError = want
			}
			pool := &transactionPoolFixture{txs: []*transactionFixture{tx}}
			err := RunTransaction(context.Background(), pool, func(context.Context, pgx.Tx) error {
				if !unknownCommit {
					return want
				}
				return nil
			})
			if !errors.Is(err, want) || len(pool.opts) != 1 {
				t.Fatalf("must preserve uncertain outcome without replaying: %v", err)
			}
		})
	}
}

func TestRunTransactionExhaustionAndCancellation(t *testing.T) {
	pool := &transactionPoolFixture{txs: []*transactionFixture{{}, {}, {}}}
	err := RunTransaction(context.Background(), pool, func(context.Context, pgx.Tx) error { return &pgconn.PgError{Code: "40001"} })
	var operationError *OperationError
	if !errors.As(err, &operationError) || operationError.Code != "iteration_retry_exhausted" || operationError.Status != 503 || !operationError.Retryable {
		t.Fatalf("unexpected exhaustion: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool = &transactionPoolFixture{txs: []*transactionFixture{{}}}
	err = RunTransaction(ctx, pool, func(context.Context, pgx.Tx) error { cancel(); return &pgconn.PgError{Code: "40P01"} })
	if !errors.Is(err, context.Canceled) || len(pool.opts) != 1 {
		t.Fatalf("cancellation must stop retries: %v", err)
	}
}

func TestEventTimeNeverMovesBackwards(t *testing.T) {
	previous := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	if got := EventTime(previous.Add(-time.Hour), previous); !got.Equal(previous) {
		t.Fatalf("clock rollback reordered event: %v", got)
	}
	if got := EventTime(previous.Add(time.Hour), previous); !got.Equal(previous.Add(time.Hour)) {
		t.Fatalf("forward time lost: %v", got)
	}
}
