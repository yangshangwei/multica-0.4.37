package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type usageOutcome struct {
	response IssueBudgetUsageResponse
	err      error
}
type usageWriter struct {
	pid  uint32
	done <-chan usageOutcome
	log  *issueBudgetStatementLog
}

func startUsageWriter(t *testing.T, f *issueBudgetFixture, attempt db.IssueBudgetAttempt, raw []byte) usageWriter {
	t.Helper()
	backends := &issueBudgetBackends{TxStarter: f.pool, began: make(chan uint32, 1)}
	log := &issueBudgetStatementLog{TxStarter: backends}
	done := make(chan usageOutcome, 1)
	go func() {
		response, err := usageActualService(f, log).Report(context.Background(), attempt, raw, time.Now())
		done <- usageOutcome{response, err}
	}()
	select {
	case pid := <-backends.began:
		return usageWriter{pid: pid, done: done, log: log}
	case result := <-done:
		t.Fatalf("usage ended before transaction: %+v", result)
	case <-time.After(15 * time.Second):
		t.Fatal("usage never began")
	}
	return usageWriter{}
}
func (w usageWriter) outcome(t *testing.T) usageOutcome {
	t.Helper()
	select {
	case result := <-w.done:
		return result
	case <-time.After(30 * time.Second):
		t.Fatalf("usage did not finish: %v", w.log.names())
	}
	return usageOutcome{}
}

func TestIssueBudgetUsageDBConcurrentDuplicate(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	_, release := f.holdRoot(t, root)
	defer release()
	raw := usageBytes(t, usageOneBatch(t))
	first := startUsageWriter(t, f, attempt, raw)
	f.waitsBehind(t, first.pid, "first usage")
	second := startUsageWriter(t, f, attempt, raw)
	f.waitsBehind(t, second.pid, "second usage")
	for _, writer := range []usageWriter{first, second} {
		if locks := f.advisoryLocks(t, writer.pid); len(locks) != 0 {
			t.Fatalf("usage owns advisory locks: %+v", locks)
		}
	}
	release()
	a := first.outcome(t)
	b := second.outcome(t)
	requireUsageRecorded(t, a.response, a.err, false)
	requireUsageRecorded(t, b.response, b.err, true)
	if len(f.entryRows(t, root)) != 2 || f.rootRow(t, root).ActualKnownTicks != 30 {
		t.Fatal("duplicate charged twice")
	}
	requireUsageProjection(t, f, root)
}

func TestIssueBudgetUsageDBConcurrentFinalize(t *testing.T) {
	f, root, attempt, task := usageScene(t)
	ctx := context.Background()
	next := f.claimedAgain(t, task)
	_, release := f.holdRoot(t, root)
	defer release()
	usage := startUsageWriter(t, f, attempt, usageBytes(t, usageOneBatch(t)))
	f.waitsBehind(t, usage.pid, "usage")
	finalize := f.startFinalize(t, next, TaskClaimBudgetProtocolV1)
	f.waitsBehind(t, finalize.pid, "finalize")
	release()
	out := usage.outcome(t)
	requireUsageRecorded(t, out.response, out.err, false)
	if out := finalize.outcome(t, "finalize after usage"); out.err != nil {
		t.Fatal(out.err)
	}
	requireUsageProjection(t, f, root)
	// The old attempt's known cost reduces its held unspent amount.
	row, err := f.q.GetIssueBudgetRoot(ctx, db.GetIssueBudgetRootParams{WorkspaceID: f.ws, ID: util.MustParseUUID(root)})
	if err != nil {
		t.Fatal(err)
	}
	if row.ActualKnownTicks != 30 || row.UnknownHoldTicks != 70 || row.ReservedTicks != 100 {
		t.Fatalf("after interleaving: %+v", issueBudgetRootFigures(row))
	}
}

func TestIssueBudgetUsageDBConcurrentSwitch(t *testing.T) {
	for _, switchFirst := range []bool{false, true} {
		t.Run(map[bool]string{true: "switch first", false: "usage first"}[switchFirst], func(t *testing.T) {
			f, root, attempt, _ := usageScene(t)
			_, release := f.holdRoot(t, root)
			defer release()
			raw := usageBytes(t, usageOneBatch(t))
			var usage usageWriter
			var change *issueBudgetWriter
			startSwitch := func() *issueBudgetWriter {
				return f.startWriter(t, "switch", func(tx TxStarter) issueBudgetWriterOutcome {
					s := NewIssueBudgetService(f.q, tx)
					result, err := s.SetMode(context.Background(), f.manage(root, 1), "off")
					return issueBudgetWriterOutcome{result: result, err: err}
				})
			}
			if switchFirst {
				change = startSwitch()
				f.waitsBehind(t, change.pid, "switch")
				usage = startUsageWriter(t, f, attempt, raw)
				f.waitsBehind(t, usage.pid, "usage")
			} else {
				usage = startUsageWriter(t, f, attempt, raw)
				f.waitsBehind(t, usage.pid, "usage")
				change = startSwitch()
				f.waitsBehind(t, change.pid, "switch")
			}
			release()
			out := usage.outcome(t)
			if out.err != nil {
				t.Fatal(out.err)
			}
			if c := change.outcome(t, "switch"); c.err != nil {
				t.Fatal(c.err)
			}
			if switchFirst {
				if out.response.Status != "not_recorded" || f.rootRow(t, root).ActualKnownTicks != 0 {
					t.Fatal("off first partially recorded")
				}
			} else {
				requireUsageRecorded(t, out.response, nil, false)
				if f.rootRow(t, root).ActualKnownTicks != 30 {
					t.Fatal("usage first not recorded")
				}
			}
			requireUsageProjection(t, f, root)
		})
	}
}

func TestIssueBudgetUsageDBPendingQueryDoesNotLock(t *testing.T) {
	f, root, attempt, _ := usageScene(t)
	raw := usageBytes(t, usageOneBatch(t))
	ctx := context.Background()
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	log := &issueBudgetStatementLog{TxStarter: f.pool, beforeCommit: func(uint32) { close(reached); <-release }}
	done := make(chan usageOutcome, 1)
	go func() {
		r, err := usageActualService(f, log).Report(ctx, attempt, raw, time.Now())
		done <- usageOutcome{r, err}
	}()
	select {
	case <-reached:
	case <-time.After(15 * time.Second):
		t.Fatal("usage never reached commit")
	}
	before := f.snapshot(t)
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	query, err := NewIssueBudgetUsageService(f.q, f.pool).Query(readCtx, attempt, usageOneBatch(t).BatchKey)
	if err != nil || query.Status != "unknown" {
		t.Fatalf("pending query = %+v %v", query, err)
	}
	f.requireUnchanged(t, before, "pending query")
	unblock()
	select {
	case out := <-done:
		requireUsageRecorded(t, out.response, out.err, false)
	case <-time.After(15 * time.Second):
		t.Fatal("commit did not return")
	}
	query, err = NewIssueBudgetUsageService(f.q, f.pool).Query(ctx, attempt, usageOneBatch(t).BatchKey)
	requireUsageRecorded(t, query, err, true)
	requireUsageProjection(t, f, root)
}
