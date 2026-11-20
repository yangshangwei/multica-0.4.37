package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// Checker's file (f3-check-6kd2, step 6 of the F3 budget-root task): the pin
// that comes with finding KD2-12 of research/step6-check-daemon-2.md.

// TestBudgetUsageRecordBeingCreatedIsNeverTakenForALeftover: begin writes the
// record file of an attempt and then enters the attempt into the table of the
// records this process holds. A pass of the resend loop that saw the file and
// not yet the entry would read a record of this process that names no provider
// launch, which is what a leftover looks like: it would remove the file with a
// warning and drop the entry. The run would go on with a record that no pass
// knows and every pass removes again, and its batch would never be sent. begin
// keeps the journal's lock from its check for an existing file to the entry in
// the table, and load looks at the table and reads the file under the same
// lock, so a pass sees both or neither.
//
// The test creates records while passes run without a pause on goroutines of
// their own. Every new attempt id sorts before the ones created earlier, so a
// pass reaches the record that is being created first.
//
// Fails on: begin letting go of the journal's lock while it writes the record
// file. There is no seam between the write and the entry, so the test relies
// on a pass falling into that window; how often it does is measured in the
// report (mutant MF4-3-k).
func TestBudgetUsageRecordBeingCreatedIsNeverTakenForALeftover(t *testing.T) {
	const (
		attempts = 24 // fewer than a pass handles, so every pass takes them all
		loops    = 2
	)
	if attempts >= maxBudgetUsageRecordsPerPass {
		t.Fatalf("the test wants fewer attempts than a pass handles (%d)", maxBudgetUsageRecordsPerPass)
	}
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)

	ctx, cancel := context.WithCancel(context.Background())
	var (
		running sync.WaitGroup
		passes  atomic.Int64
	)
	ended := make(chan struct{})
	for range loops {
		running.Add(1)
		go func() {
			defer running.Done()
			for ctx.Err() == nil {
				d.resendBudgetUsage(ctx)
				passes.Add(1)
			}
		}()
	}
	go func() {
		defer close(ended)
		running.Wait()
	}()
	stop := func() {
		cancel()
		budgetUsageTestAwait(t, ended, "the passes end")
	}
	t.Cleanup(stop)

	type begun struct {
		attemptID string
		attempt   *budgetUsageAttempt
	}
	created := make([]begun, 0, attempts)
	for n := range attempts {
		attemptID := fmt.Sprintf("0195f3a0-0000-7000-8000-%012x", attempts-n)
		attempt := budgetUsageTestBegun(t, journal, budgetUsageTestSeed(attemptID))
		created = append(created, begun{attemptID, attempt})
	}
	// One more round of passes over the complete set, then the passes stop.
	seen := passes.Load()
	budgetUsageTestUntil(t, "passes ran after the last record was created", func() bool { return passes.Load() >= seen+2*loops })
	stop()

	for _, c := range created {
		if budgetUsageTestLiveAttempt(journal, c.attemptID) != c.attempt {
			t.Errorf("attempt %s: a pass took the record away from the run that had just created it", c.attemptID)
		}
		if _, err := os.Lstat(journal.recordPath(c.attemptID)); err != nil {
			t.Errorf("attempt %s: the record file of a running attempt: %v", c.attemptID, err)
		}
	}
	if warnings := budgetUsageTestWarningLines(logs.String()); len(warnings) != 0 {
		t.Errorf("%d warnings; a pass took a record that was being created for a leftover:\n%s", len(warnings), warnings[0])
	}
	if reports := api.reports(); len(reports) != 0 {
		t.Errorf("%d usage reports for attempts that are still running", len(reports))
	}
	t.Logf("%d passes while %d records were created", passes.Load(), attempts)
}
