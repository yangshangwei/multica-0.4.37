package daemon

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The seven-day retention cap applies when the resend pass sees a sealed
// record. Either order is allowed: direct send first, or expiration first.
// Neither order proves completion or releases the server's unknown hold.

// budgetUsageCheckOutlastedRun is an attempt whose one provider run took
// longer than the retention cap and has ended, right before handleTask seals
// it.
func budgetUsageCheckOutlastedRun(t *testing.T) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog, *budgetUsageTestAPI, *budgetUsageAttempt, *budgetUsageTestClock) {
	t.Helper()
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, filepath.Join(t.TempDir(), "journal"), clock)
	attempt := budgetUsageTestBegun(t, journal, budgetUsageTestSeed(budgetUsageTestAttemptA))
	invocationID := attempt.beginInvocation(false)
	clock.advance(budgetUsageRetention + time.Hour)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	return d, journal, logs, api, attempt, clock
}

// budgetUsageCheckRequireDroppedUnsent states what happens today.
func budgetUsageCheckRequireDroppedUnsent(t *testing.T, journal *budgetUsageJournal, logs *budgetUsageTestLog, api *budgetUsageTestAPI) {
	t.Helper()
	// The retention pass won before the direct send.
	if got := len(api.reports()); got != 0 {
		t.Errorf("%d usage reports reached the server; the reproduction expects the batch to be dropped unsent", got)
	}
	// Expiration is explicit in the warning.
	warnings := budgetUsageTestWarningLines(logs.String())
	if len(warnings) != 1 || !strings.Contains(warnings[0], "older than seven days and was not delivered") {
		t.Errorf("warnings = %q; the reproduction expects the one warning of the retention cap", warnings)
	}
	// Expiration removes the local record; it is not delivery evidence.
	budgetUsageTestRequireNoRecordFile(t, journal, budgetUsageTestAttemptA, "after the pass and the send")
	if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) != nil {
		t.Error("the process still holds the record")
	}
}

func TestBudgetUsageExpiredSealedBatchIsDroppedWhenRetentionPassWins(t *testing.T) {
	// finishBudgetUsageAttempt is two steps: the seal, on the goroutine of
	// the task, and the send, on a goroutine of its own. A pass of the resend
	// loop runs every thirty seconds on a third goroutine and can fall
	// between the two.
	t.Run("a pass between the seal and the send", func(t *testing.T) {
		d, journal, logs, api, attempt, _ := budgetUsageCheckOutlastedRun(t)
		ctx := context.Background()

		if len(attempt.seal()) == 0 {
			t.Fatal("the attempt has no batch")
		}
		d.resendBudgetUsage(ctx)
		d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)

		budgetUsageCheckRequireDroppedUnsent(t, journal, logs, api)
	})

	// The window is wider than the gap between two statements: the seal holds
	// the attempt's lock while it writes the record with two fsyncs. A pass
	// that reaches the record during that write takes the mark, waits for the
	// lock, and then sees a sealed record. This subtest goes through
	// finishBudgetUsageAttempt itself and lets the pass arrive while the seal
	// is under way: the seal reads the journal's clock with the attempt
	// locked, and the clock of the test starts the pass at that moment.
	t.Run("a pass while the seal is under way", func(t *testing.T) {
		d, journal, logs, api, attempt, clock := budgetUsageCheckOutlastedRun(t)
		ctx := context.Background()

		var armed atomic.Bool
		passDone := make(chan struct{})
		journal.now = func() time.Time {
			if armed.CompareAndSwap(true, false) {
				go func() {
					defer close(passDone)
					d.resendBudgetUsage(ctx)
				}()
				budgetUsageTestUntil(t, "the pass takes the mark on the record", func() bool {
					journal.mu.Lock()
					defer journal.mu.Unlock()
					_, marked := journal.inFlight[budgetUsageTestAttemptA]
					return marked
				})
			}
			return clock.Now()
		}
		armed.Store(true)

		d.finishBudgetUsageAttempt(ctx, attempt)
		budgetUsageTestAwait(t, passDone, "the pass ends")
		budgetUsageTestWaitForSends(t, journal)

		if armed.Load() {
			t.Fatal("the seal did not read the journal's clock: the reproduction did not run")
		}
		budgetUsageCheckRequireDroppedUnsent(t, journal, logs, api)
	})
}
