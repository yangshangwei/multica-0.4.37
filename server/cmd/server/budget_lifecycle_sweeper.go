package main

import (
	"context"
	"log/slog"
	"time"

	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
)

const (
	// budgetLifecycleSweepInterval is how often the attempts whose task ended
	// are looked for. It is the cadence of the liveness loop, on a loop of its
	// own.
	budgetLifecycleSweepInterval = 30 * time.Second
	// budgetLifecycleBatchSize bounds the roots one round reconciles, and so
	// the transactions it opens. A root that was reconciled is not found
	// again, so the next round takes the ones after it.
	budgetLifecycleBatchSize = 500
	// budgetLifecycleOperationTimeout bounds the read that finds the roots
	// and the reconciliation of each one, so that one contended root lock or
	// one very long ledger cannot hold up the roots after it.
	budgetLifecycleOperationTimeout = 5 * time.Second
)

// budgetLifecycleReconciler is what the loop needs of the budget service.
type budgetLifecycleReconciler interface {
	ReconcileLifecycles(ctx context.Context, maxRoots int32, each time.Duration) (service.IssueBudgetLifecycleRound, error)
}

// runBudgetLifecycleSweeper turns what the ended attempts of budget roots
// reserved into holds (design 10.3): nothing tells a root when a task of one
// of its attempts ends, so the roots are reconciled afterwards.
//
// It runs on its own goroutine and not inside the runtime sweep tick. A round
// takes a root lock and a transaction for every root it finds, and that tick
// drops its next rounds while one is running: a contended root must never
// delay marking runtimes offline or recovering their tasks. For the same
// reason the loop takes no observer. A round that fails says nothing about
// the health of the task sweeper.
func runBudgetLifecycleSweeper(ctx context.Context, budgets budgetLifecycleReconciler, metrics *obsmetrics.BusinessMetrics) {
	runPeriodicSweep(ctx, budgetLifecycleSweepInterval, func() {
		sweepBudgetLifecycles(ctx, budgets, metrics)
	})
}

// sweepBudgetLifecycles is one round. candidates counts the roots the round
// found work for and changed the attempts it closed. When the round finds no
// root it is one read: no transaction, no lock, no write.
func sweepBudgetLifecycles(ctx context.Context, budgets budgetLifecycleReconciler, metrics *obsmetrics.BusinessMetrics) (stats runtimeSweepStageStats) {
	startedAt := time.Now()
	defer func() {
		observeRuntimeSweepStage(metrics, obsmetrics.RuntimeSweepStageBudgetLifecycle, startedAt, stats)
	}()

	round, err := budgets.ReconcileLifecycles(ctx, budgetLifecycleBatchSize, budgetLifecycleOperationTimeout)
	if err != nil {
		stats.failed = true
		if ctx.Err() == nil {
			slog.Warn("budget lifecycle: failed to list roots with ended attempts", "error", err)
		}
		return
	}
	stats.candidates = round.Roots
	stats.changed = round.Closed
	if round.Closed > 0 {
		slog.Info("budget lifecycle: closed ended attempts",
			"roots", round.Roots, "closed", round.Closed, "skipped", round.Skipped, "failed", round.Failed)
	}
	return
}
