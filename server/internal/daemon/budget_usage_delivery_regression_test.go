package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Checker's file (f3-check-6kd2, step 6 of the F3 budget-root task): pins for
// lines of the sending and resending code that no test could tell from a
// changed one. Each test names the change it fails on.

func budgetUsageCheckVerdictName(verdict budgetUsageVerdict) string {
	switch verdict {
	case budgetUsageKept:
		return "kept"
	case budgetUsageDelivered:
		return "delivered"
	case budgetUsageRefused:
		return "refused for good"
	default:
		return fmt.Sprintf("verdict %d", int(verdict))
	}
}

// TestBudgetUsageVerdictOfEveryStatus: the classes of design 11.10, checked
// for every status a status line can carry and not only for the ones a table
// of cases lists. Any 2xx is delivered; 400, 404, 409, 413 and 422 are final;
// everything else is kept, and so is every status whose answer could not be
// read to its end.
//
// Fails on: the lower bound of the 2xx range moved to 100 (a 101, which Go's
// client hands over as a final answer, becomes delivered), and any status
// outside the lists of the case table moved to another class (412 final, 306
// delivered).
func TestBudgetUsageVerdictOfEveryStatus(t *testing.T) {
	final := map[int]bool{
		http.StatusBadRequest:            true,
		http.StatusNotFound:              true,
		http.StatusConflict:              true,
		http.StatusRequestEntityTooLarge: true,
		http.StatusUnprocessableEntity:   true,
	}
	unread := errors.New("the answer ended before its declared length")
	for status := 0; status <= 999; status++ {
		want := budgetUsageKept
		switch {
		case status >= 200 && status <= 299:
			want = budgetUsageDelivered
		case final[status]:
			want = budgetUsageRefused
		}
		if got := (budgetUsageReply{status: status}).verdict(); got != want {
			t.Errorf("status %d is %s, want %s", status, budgetUsageCheckVerdictName(got), budgetUsageCheckVerdictName(want))
		}
		if got := (budgetUsageReply{status: status, err: unread}).verdict(); got != budgetUsageKept {
			t.Errorf("status %d with an answer that could not be read to its end is %s, want kept", status, budgetUsageCheckVerdictName(got))
		}
	}
	if got := (budgetUsageReply{err: errors.New("no connection")}).verdict(); got != budgetUsageKept {
		t.Errorf("no answer at all is %s, want kept", budgetUsageCheckVerdictName(got))
	}
}

// budgetUsageCheckRequireKeptLine fails unless the log says that a report
// with the given status was kept for the next send.
func budgetUsageCheckRequireKeptLine(t *testing.T, logged string, status int) {
	t.Helper()
	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, "keeping it for the next send") && strings.Contains(line, "status="+strconv.Itoa(status)+" ") {
			return
		}
	}
	t.Errorf("no log line says that a report answered with status %d was kept:\n%s", status, logged)
}

// TestBudgetUsageSwitchingProtocolsAnswerKeepsTheRecord: a 1xx answer is not
// a delivery. Go's client consumes the other 1xx answers itself and hands a
// 101 to its caller as the final answer, with an empty body, so this is the
// one status below 200 that reaches the verdict over the wire.
//
// Fails on: the lower bound of the 2xx range moved to 100.
func TestBudgetUsageSwitchingProtocolsAnswerKeepsTheRecord(t *testing.T) {
	api := budgetUsageTestNewAPI(t, budgetUsageTestThen(
		func(_ int, w http.ResponseWriter) { w.WriteHeader(http.StatusSwitchingProtocols) },
		budgetUsageTestRecorded()))
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
	ctx := context.Background()

	d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)

	if reports := api.reports(); len(reports) != 1 {
		t.Fatalf("%d usage reports, want 1", len(reports))
	}
	// The answer reached the verdict as a 101, not as a failed request.
	budgetUsageCheckRequireKeptLine(t, logs.String(), http.StatusSwitchingProtocols)
	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
		t.Errorf("the record file changed\n before: %s\n after:  %s", fileBefore, got)
	}
	if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
		t.Fatal("the journal's process lost the record: a 101 was taken for a delivery")
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}

	d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	reports := api.reports()
	if len(reports) != 2 {
		t.Fatalf("%d usage reports after the second send, want 2", len(reports))
	}
	for i, report := range reports {
		if !bytes.Equal(report.Body, batch) {
			t.Errorf("report %d does not carry the sealed bytes: %s", i+1, report.Body)
		}
	}
	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Errorf("the journal still holds %v after the report was delivered", names)
	}
}

// budgetUsageCheckLongAnswerCutShort answers a report with a body that is
// longer than what the daemon keeps of an answer and that ends before the
// length it declared, after the part the daemon keeps.
func budgetUsageCheckLongAnswerCutShort(status int) budgetUsageTestAnswer {
	return func(_ int, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(4*maxBudgetUsageReplyBytes))
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"result":"recorded","code":"invalid_request","padding":"`)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 2*maxBudgetUsageReplyBytes))
	}
}

// TestBudgetUsageLongAnswerThatEndsEarlyKeepsTheRecord: of an answer the
// first 64 KiB are kept and the rest is read and dropped. The rest is still
// part of the answer: when it ends before its declared length the answer was
// not read to its end, and the record is kept whatever the status line said.
// The case table has this only for answers shorter than what is kept.
//
// Fails on: the copy of the rest of an answer removed (the answer is then
// judged by its status line after the first 64 KiB).
func TestBudgetUsageLongAnswerThatEndsEarlyKeepsTheRecord(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestThen(budgetUsageCheckLongAnswerCutShort(status), budgetUsageTestRecorded()))
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
			fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			ctx := context.Background()

			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)

			if reports := api.reports(); len(reports) != 1 {
				t.Fatalf("%d usage reports, want 1", len(reports))
			}
			if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
				t.Fatalf("the journal's process lost the record: an answer that ended early was judged by its status line\nlog:\n%s", logs.String())
			}
			if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
				t.Errorf("the record file changed\n before: %s\n after:  %s", fileBefore, got)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 0 {
				t.Errorf("%d warnings:\n%s", got, logs.String())
			}

			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
			reports := api.reports()
			if len(reports) != 2 {
				t.Fatalf("%d usage reports after the second send, want 2", len(reports))
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

// TestBudgetUsageRestartKeepsARecordWhoseServerAddressIsSpelledDifferently:
// the server a record belongs to is told by an exact comparison of the address
// it was created under with the address this process runs under (the accepted
// choice: an address rewritten in another spelling strands its records until
// the cap, and none goes to a server it was not created under). The restart
// test of this rule uses another host only, so a comparison that forgives a
// difference would pass it.
//
// Fails on: the comparison ignoring case, or ignoring a trailing slash.
func TestBudgetUsageRestartKeepsARecordWhoseServerAddressIsSpelledDifferently(t *testing.T) {
	spellings := []string{
		"https://MULTICA.example.test",     // upper case in the host
		"HTTPS://multica.example.test",     // upper case in the scheme
		"https://multica.example.test/",    // a trailing slash
		"https://multica.example.test:443", // the default port, spelled out
		"https://multica.example.test.",    // a trailing dot
		"https://multica.example.test/api", // a path
		"http://multica.example.test",      // another scheme
	}
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	var attemptIDs []string
	for i, spelling := range spellings {
		if spelling == budgetUsageTestServerURL {
			t.Fatalf("spelling %q is the address the test process runs under", spelling)
		}
		earlier := budgetUsageTestProcessOf(t, directory, spelling, clock)
		sealedID := fmt.Sprintf("0195f3a0-0000-7000-8000-0000000000%02x", 0xc0+2*i)
		runningID := fmt.Sprintf("0195f3a0-0000-7000-8000-0000000000%02x", 0xc1+2*i)
		// One record with batch bytes, which would be sent as it is, and one
		// with a launch intent, which would be built, written and sent.
		budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(sealedID))
		budgetUsageTestLeftRunning(t, earlier, budgetUsageTestSeed(runningID))
		attemptIDs = append(attemptIDs, sealedID, runningID)
	}
	clock.advance(time.Hour)

	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	if journal.serverURL != budgetUsageTestServerURL {
		t.Fatalf("the test process runs under %q, want %q", journal.serverURL, budgetUsageTestServerURL)
	}
	namesBefore := budgetUsageTestNames(t, directory)
	if len(namesBefore) != len(attemptIDs) {
		t.Fatalf("the journal holds %d files, want one record per attempt (%d): %v", len(namesBefore), len(attemptIDs), namesBefore)
	}
	filesBefore := make(map[string]string, len(attemptIDs))
	for _, attemptID := range attemptIDs {
		filesBefore[attemptID] = budgetUsageTestFileText(t, journal, attemptID)
	}

	ctx := context.Background()
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(ctx)
		if calls := api.seen(); len(calls) != 0 {
			t.Fatalf("pass %d: the server received %d requests for records created under another spelling of its address, the first to %s", pass, len(calls), calls[0].Path)
		}
		if names := budgetUsageTestNames(t, directory); !reflect.DeepEqual(names, namesBefore) {
			t.Fatalf("pass %d: the journal holds %v, want %v", pass, names, namesBefore)
		}
		for i, attemptID := range attemptIDs {
			if got := budgetUsageTestFileText(t, journal, attemptID); got != filesBefore[attemptID] {
				t.Errorf("pass %d: the record created under %q changed\n before: %s\n after:  %s", pass, spellings[i/2], filesBefore[attemptID], got)
			}
		}
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageReportLeavesTheDaemonsOtherRequestsFollowingRedirects: a
// usage report does not follow a redirect. That is a property of the report
// and of nothing else: the daemon's other requests go through the same client
// and, in an installation that is not managed, follow a redirect as they did
// before the protocol existed.
//
// Fails on: following switched off on the shared client instead of on the
// per-call copy, in reportBudgetUsage or where the protocol is declared.
func TestBudgetUsageReportLeavesTheDaemonsOtherRequestsFollowingRedirects(t *testing.T) {
	var reports, redirected, landed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/daemon/budget/"):
			reports.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"result":"recorded"}`)
		case strings.HasSuffix(r.URL.Path, "/status"):
			redirected.Add(1)
			http.Redirect(w, r, "/moved/task-status", http.StatusFound)
		case r.URL.Path == "/moved/task-status":
			landed.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"running"}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)

	d, logs, _ := budgetUsageTestProfileDaemon(t, &budgetUsageTestAPI{URL: server.URL})
	d.client.SetToken(budgetUsageTestToken)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	requireFollowed := func(when string, want int32) {
		t.Helper()
		state, err := d.client.GetTaskState(ctx, budgetUsageTestTaskID)
		if err != nil {
			t.Fatalf("%s: a task status request that the server redirects failed: %v", when, err)
		}
		if state.Status != "running" {
			t.Fatalf("%s: task status = %q, want the answer of the page the server redirected to", when, state.Status)
		}
		if got := landed.Load(); got != want {
			t.Fatalf("%s: %d requests arrived where the server redirected to, want %d", when, got, want)
		}
	}

	// The control: this client follows a redirect.
	requireFollowed("before the protocol is declared", 1)

	d.initializeBudgetUsage()
	journal := d.budgetUsage
	if journal == nil {
		t.Fatalf("the daemon has no journal\nlog:\n%s", logs.String())
	}
	t.Cleanup(journal.background.Wait)
	requireFollowed("after the protocol is declared", 2)

	budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
	if got := reports.Load(); got != 1 {
		t.Fatalf("%d usage reports, want 1", got)
	}
	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Fatalf("the journal still holds %v after the report was delivered", names)
	}
	requireFollowed("after a usage report", 3)
	if got := redirected.Load(); got != 3 {
		t.Errorf("the server redirected %d task status requests, want 3", got)
	}
}

// TestBudgetUsagePassHandlesARecordThisProcessHoldsOnce: a pass takes the
// records this process holds and the record files of the directory, and a
// record this process holds has a file as well. It is one record and is
// handled once in a pass, also when another record sorts before it.
//
// Fails on: the sort of the record ids removed (the two mentions of the held
// record are then not next to each other, the duplicate stays, and a server
// that answers 503 gets the batch twice in every pass).
func TestBudgetUsagePassHandlesARecordThisProcessHoldsOnce(t *testing.T) {
	if budgetUsageTestAttemptA >= budgetUsageTestAttemptB {
		t.Fatal("the test wants attempt A to sort before attempt B")
	}
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	// An earlier process left the record that sorts first.
	left := budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
	clock.advance(time.Hour)

	api := budgetUsageTestNewAPI(t, budgetUsageTestUnavailable())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	// This process holds the record that sorts second.
	held := budgetUsageTestLeftSealed(t, journal, budgetUsageTestSeed(budgetUsageTestAttemptB))
	if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptB) == nil {
		t.Fatal("the test wants a record this process holds")
	}

	ctx := context.Background()
	for pass := 1; pass <= 3; pass++ {
		d.resendBudgetUsage(ctx)
		for _, want := range []struct {
			attemptID string
			batch     []byte
		}{{budgetUsageTestAttemptA, left}, {budgetUsageTestAttemptB, held}} {
			reports := budgetUsageTestReportsFor(api, want.attemptID)
			if len(reports) != pass {
				t.Fatalf("after pass %d the server received %d reports for attempt %s, want %d: one per pass",
					pass, len(reports), want.attemptID, pass)
			}
			for i, report := range reports {
				if !bytes.Equal(report.Body, want.batch) {
					t.Errorf("report %d for attempt %s does not carry its batch: %s", i+1, want.attemptID, report.Body)
				}
			}
		}
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageResendIntervalIsThirtySeconds: the number. A server that
// keeps answering 503 gets every kept batch once per pass, so this interval is
// what bounds the load a daemon puts on it: 120 requests per record and hour,
// and with the cap of 32 records per pass at most 64 requests a minute. The
// loop test passes its own interval, so nothing else looks at this one.
//
// Fails on: the interval changed (30 ms would be a thousand requests a
// second).
func TestBudgetUsageResendIntervalIsThirtySeconds(t *testing.T) {
	if budgetUsageResendInterval != 30*time.Second {
		t.Fatalf("the resend loop passes every %s, want every 30s", budgetUsageResendInterval)
	}
}

func budgetUsageCheckSettled(journal *budgetUsageJournal) []string {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	settled := make([]string, 0, len(journal.settled))
	for attemptID := range journal.settled {
		settled = append(settled, attemptID)
	}
	return settled
}

// TestBudgetUsageSettledAttemptIsForgottenOnceItsFileIsRemoved: an attempt
// whose record is done with and whose file could not be removed is remembered
// as settled, so that this process neither sends its batch again nor warns
// again. Once a later pass has removed the file there is nothing left to
// remember.
//
// Fails on: the removal that succeeds later leaving the entry behind (the set
// then grows by one entry for every such attempt until the process ends).
func TestBudgetUsageSettledAttemptIsForgottenOnceItsFileIsRemoved(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, _ := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()

	// Files of this directory can be read and cannot be removed.
	budgetUsageTestChmod(t, directory, 0o500)
	d.resendBudgetUsage(ctx)
	if got := len(api.reports()); got != 1 {
		t.Fatalf("%d usage reports, want 1", got)
	}
	if settled := budgetUsageCheckSettled(journal); !reflect.DeepEqual(settled, []string{budgetUsageTestAttemptA}) {
		t.Fatalf("the test wants an answered record whose file could not be removed; remembered as settled: %v", settled)
	}

	// The directory lets go.
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	d.resendBudgetUsage(ctx)
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Fatalf("the journal still holds %v", names)
	}
	if settled := budgetUsageCheckSettled(journal); len(settled) != 0 {
		t.Errorf("still remembered as settled after the file was removed: %v", settled)
	}
	if got := len(api.reports()); got != 1 {
		t.Errorf("%d usage reports in all, want 1", got)
	}
}
