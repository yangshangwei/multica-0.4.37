package service

import (
	"context"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Faults apply to one named object read inside its own transaction. All list
// predicates, ownership, grants, capacity and the eventual healthy claim are real.
type runtimeScanFaultStarter struct {
	TxStarter
	query           string
	faults          map[pgtype.UUID]error
	commitQuery     string
	commitError     error
	rejectCommit    bool
	mu              sync.Mutex
	hits            map[pgtype.UUID]int
	rejectedCommits int
	lostCommits     int
}

type runtimeScanFaultTx struct {
	pgx.Tx
	starter     *runtimeScanFaultStarter
	commitFault bool
}

func (s *runtimeScanFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &runtimeScanFaultTx{Tx: tx, starter: s}, nil
}

func (tx *runtimeScanFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.starter.commitQuery != "" && strings.Contains(sql, "-- name: "+tx.starter.commitQuery+" ") {
		tx.commitFault = true
	}
	if strings.Contains(sql, "-- name: "+tx.starter.query+" ") && len(args) > 0 {
		if id, ok := args[0].(pgtype.UUID); ok {
			tx.starter.mu.Lock()
			err := tx.starter.faults[id]
			if err != nil {
				if tx.starter.hits == nil {
					tx.starter.hits = make(map[pgtype.UUID]int)
				}
				tx.starter.hits[id]++
			}
			tx.starter.mu.Unlock()
			if err != nil {
				return queueFailureRow{err: err}
			}
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx *runtimeScanFaultTx) Commit(ctx context.Context) error {
	if tx.commitFault && tx.starter.rejectCommit {
		tx.starter.mu.Lock()
		tx.starter.rejectedCommits++
		tx.starter.mu.Unlock()
		return tx.starter.commitError
	}
	if err := tx.Tx.Commit(ctx); err != nil {
		return err
	}
	if tx.commitFault {
		tx.starter.mu.Lock()
		tx.starter.lostCommits++
		tx.starter.mu.Unlock()
		return tx.starter.commitError
	}
	return nil
}
