package daemon

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Checker's file (f3-check-6kd2, step 6 of the F3 budget-root task): the bound
// of one usage report.
//
// One report is bounded by the Timeout of the daemon's HTTP client, which the
// per-call copy made in reportBudgetUsage keeps. No test held an answer open
// until a report gave up by that bound, so a change that builds the per-call
// client afresh would have dropped it without a red test, and a server that
// then holds one request open would stop the resend loop for good: the loop
// is sequential.

const (
	// budgetUsageCheckBound is the bound the tests give the daemon's client in
	// place of the 30 seconds of NewClient.
	budgetUsageCheckBound = 1500 * time.Millisecond
	// budgetUsageCheckGuard is how long a test waits for a report that should
	// have given up after budgetUsageCheckBound before it says that nothing
	// bounds the report.
	budgetUsageCheckGuard = 15 * time.Second
)

// TestBudgetUsageReportGivesUpByTheClientsOwnBound: a server takes a usage
// report and does not answer it, or stops in the middle of its answer. The
// report is sent under a context that never ends, so the one thing that can
// end it is the bound of the client. It ends by that bound, the record is byte
// for byte what it was, nothing is warned, and the mark on the record is
// released: a second handling of the same record sends the same bytes again.
func TestBudgetUsageReportGivesUpByTheClientsOwnBound(t *testing.T) {
	cases := []struct {
		name string
		// stall answers the first report and returns once release is closed.
		stall func(w http.ResponseWriter, release <-chan struct{})
	}{
		{
			name: "no answer at all",
			stall: func(_ http.ResponseWriter, release <-chan struct{}) {
				<-release
			},
		},
		{
			// The status line says 200 and the body never ends: the bound
			// covers the reading of the answer too, and an answer that cannot
			// be read to its end keeps the record whatever its status.
			name: "an answer that stops in the middle of its body",
			stall: func(w http.ResponseWriter, release <-chan struct{}) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", "4096")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, `{"result":"recorded"`)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				<-release
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			release := make(chan struct{})
			var releaseOnce sync.Once
			letGo := func() { releaseOnce.Do(func() { close(release) }) }
			api := budgetUsageTestNewAPI(t, func(nth int, w http.ResponseWriter) {
				if nth == 1 {
					tc.stall(w, release)
					return
				}
				budgetUsageTestRecorded()(nth, w)
			})
			// Registered after the server's own cleanup and so run before it:
			// the held request is let go before the server waits for it.
			t.Cleanup(letGo)

			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			d.client.client.Timeout = budgetUsageCheckBound
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
			fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)

			done := make(chan struct{})
			start := time.Now()
			go func() {
				defer close(done)
				d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)
			}()
			select {
			case <-done:
			case <-time.After(budgetUsageCheckGuard):
				// Let the server answer, so that the send is over before the
				// directory of the journal goes away, and then say what is
				// wrong.
				letGo()
				budgetUsageTestAwait(t, done, "the report ends once the server answers")
				t.Fatalf("a usage report that the server held open had not given up after %s, with the bound of the daemon's client at %s: nothing bounds one report",
					budgetUsageCheckGuard, budgetUsageCheckBound)
			}
			if elapsed := time.Since(start); elapsed < budgetUsageCheckBound {
				t.Fatalf("the report ended after %s, before the bound of %s: something else than the bound ended it", elapsed, budgetUsageCheckBound)
			}

			if reports := api.reports(); len(reports) != 1 {
				t.Fatalf("%d usage reports reached the server before the report gave up, want 1", len(reports))
			}
			if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
				t.Errorf("the record file changed\n before: %s\n after:  %s", fileBefore, got)
			}
			if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
				t.Error("the journal's process lost the record")
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 0 {
				t.Errorf("a report that gave up logged %d warnings:\n%s", got, logs.String())
			}

			// The mark went with the report that gave up: the record is handled
			// again and the same bytes go out.
			letGo()
			d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)
			reports := api.reports()
			if len(reports) != 2 {
				t.Fatalf("%d usage reports after a second handling of the record, want 2: the mark was not released", len(reports))
			}
			for i, report := range reports {
				if !bytes.Equal(report.Body, batch) {
					t.Errorf("report %d does not carry the sealed bytes: %s", i+1, report.Body)
				}
			}
			if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
				t.Errorf("the journal still holds %v after the report was delivered", names)
			}
		})
	}
}

// TestBudgetUsageReportBoundIsThirtySeconds: the number. One report is bounded
// by the Timeout of the client NewClient builds, and a pass of the resend loop
// against a server that holds every request open takes up to
// maxBudgetUsageRecordsPerPass times that. Whoever changes the Timeout of the
// daemon's client changes both, and should see it here.
func TestBudgetUsageReportBoundIsThirtySeconds(t *testing.T) {
	if got := NewClient("http://127.0.0.1:1").client.Timeout; got != 30*time.Second {
		t.Fatalf("the daemon's client gives one request %s, want 30s: it is the bound of one usage report", got)
	}
}
