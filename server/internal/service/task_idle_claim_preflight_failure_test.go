package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Only the new negative probe fails. Credential/runtime checks, promotion and
// every other SQL operation use the original pool and unchanged SQL/arguments.
type r31PreflightFailureDB struct {
	db.DBTX
	err          error
	hits         int
	scans        int
	runtimeIDs   []pgtype.UUID
	recoverySecs float64
	digest       string
	afterProbe   []string
}

func (f *r31PreflightFailureDB) observe(sql string) {
	if f.hits > 0 {
		f.afterProbe = append(f.afterProbe, strings.TrimSpace(strings.SplitN(sql, "\n", 2)[0]))
	}
}

func (f *r31PreflightFailureDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.observe(sql)
	return f.DBTX.Exec(ctx, sql, args...)
}

func (f *r31PreflightFailureDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.observe(sql)
	return f.DBTX.Query(ctx, sql, args...)
}

func (f *r31PreflightFailureDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.observe(sql)
	if !strings.HasPrefix(sql, "-- name: HasPotentialDispatchedReclaimForRuntimes :one\n") {
		return f.DBTX.QueryRow(ctx, sql, args...)
	}
	f.hits++
	f.digest = hexHash([]byte(sql))
	for _, arg := range args {
		switch value := arg.(type) {
		case []pgtype.UUID:
			f.runtimeIDs = append([]pgtype.UUID{}, value...)
		case float64:
			f.recoverySecs = value
		}
	}
	return r31PreflightFailureRow{owner: f}
}

type r31PreflightFailureRow struct{ owner *r31PreflightFailureDB }

func (row r31PreflightFailureRow) Scan(...any) error {
	row.owner.scans++
	return row.owner.err
}

type r31PreflightFailureStarter struct {
	TxStarter
	starts int
}

func (s *r31PreflightFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.starts++
	return s.TxStarter.Begin(ctx)
}

func TestOrdinaryIdlePreflightFailureDoesNotAuthorizeClaim(t *testing.T) {
	f := r31Fixture(t)
	before := r31Snapshot(t, f)
	sentinel := errors.New("R31 controlled potential-reclaim probe failure")
	queries := &r31PreflightFailureDB{DBTX: f.pool, err: sentinel}
	starter := &r31PreflightFailureStarter{TxStarter: f.pool}
	svc := NewTaskService(db.New(queries), starter, nil, events.New())
	if svc.EmptyClaim != nil || svc.ReclaimCheck != nil {
		t.Fatal("preflight failure control did not retain supported nil-cache ordinary behavior")
	}
	ctx, cancel := context.WithTimeout(f.daemon.daemonCtx, 5*time.Second)
	defer cancel()
	// Duplicate input exercises the real runtime-set normalization. It cannot
	// replace the actual current runtime/credential validation before the probe.
	tasks, err := svc.ClaimTasksForRuntimes(ctx, []pgtype.UUID{f.daemon.runtime.ID, f.daemon.runtime.ID}, 1)
	if !errors.Is(err, sentinel) || err.Error() != "check potential dispatched reclaim: "+sentinel.Error() || len(tasks) != 0 {
		t.Fatalf("probe failure became empty success or returned claim work: tasks=%d err=%v", len(tasks), err)
	}
	if queries.hits != 1 || queries.scans != 1 || !reflect.DeepEqual(queries.runtimeIDs, []pgtype.UUID{f.daemon.runtime.ID}) || queries.recoverySecs != claimResponseRecoveryWindow.Seconds() || !DeliveryHex(queries.digest, 64) {
		t.Fatalf("probe control missed its exact runtime/window/query boundary: hits=%d scans=%d runtimes=%v window=%v", queries.hits, queries.scans, queries.runtimeIDs, queries.recoverySecs)
	}
	if starter.starts != 0 || len(queries.afterProbe) != 0 {
		t.Fatalf("failed probe proceeded to account/reclaim/candidate/claim writes: transactions=%d subsequent=%v", starter.starts, queries.afterProbe)
	}
	completionCandidatesCompareState(t, before, r31Snapshot(t, f), "ordinary preflight error")
	t.Logf("ORDINARY_PREFLIGHT_ERROR probe_hits=%d scans=%d query_sha256=%s runtime_count=%d recovery_secs=%v transactions=%d tasks=%d error_type=%T", queries.hits, queries.scans, queries.digest, len(queries.runtimeIDs), queries.recoverySecs, starter.starts, len(tasks), err)
}
