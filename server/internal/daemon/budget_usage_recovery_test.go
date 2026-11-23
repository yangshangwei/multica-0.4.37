package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Stage E: the resend loop, the recovery after a restart and the retention
// cap.

// budgetUsageTestClock is the clock of the journals of a test.
type budgetUsageTestClock struct {
	mu  sync.Mutex
	now time.Time
}

// budgetUsageTestNewClock starts at a fixed whole second.
func budgetUsageTestNewClock() *budgetUsageTestClock {
	return &budgetUsageTestClock{now: time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)}
}

func (c *budgetUsageTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *budgetUsageTestClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

func (c *budgetUsageTestClock) set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

// budgetUsageTestProcessOf opens a journal directory the way one daemon
// process does. Every call is another process: it has a start identity of its
// own and knows nothing of what an earlier one held in memory.
func budgetUsageTestProcessOf(t *testing.T, directory, serverURL string, clock *budgetUsageTestClock) *budgetUsageJournal {
	t.Helper()
	journal, err := openBudgetUsageJournal(directory, serverURL)
	if err != nil {
		t.Fatalf("open budget usage journal: %v", err)
	}
	journal.now = clock.Now
	t.Cleanup(journal.background.Wait)
	return journal
}

// budgetUsageTestProcess is a daemon process of the server the tests talk to.
func budgetUsageTestProcess(t *testing.T, directory string, clock *budgetUsageTestClock) *budgetUsageJournal {
	t.Helper()
	return budgetUsageTestProcessOf(t, directory, budgetUsageTestServerURL, clock)
}

// budgetUsageTestRestarted is the daemon of a process that starts over a
// journal directory and talks to api.
func budgetUsageTestRestarted(t *testing.T, api *budgetUsageTestAPI, directory string, clock *budgetUsageTestClock) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
	t.Helper()
	logger, logs := budgetUsageTestLogger()
	journal := budgetUsageTestProcess(t, directory, clock)
	d := budgetUsageTestNewDaemon(&budgetUsageTestServer{URL: api.URL}, journal, logger)
	d.client.SetToken(budgetUsageTestToken)
	return d, journal, logs
}

// budgetUsageTestBegun creates the record of an attempt in the process of a
// journal.
func budgetUsageTestBegun(t *testing.T, journal *budgetUsageJournal, seed budgetUsageSeed) *budgetUsageAttempt {
	t.Helper()
	logger, _ := budgetUsageTestLogger()
	attempt, err := journal.begin(seed, logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	return attempt
}

// budgetUsageTestLeftRunning creates the record of an attempt that launched
// one provider process and has no outcome for it, which is what a process
// that dies during a run leaves behind. It returns the record as that process
// held it.
func budgetUsageTestLeftRunning(t *testing.T, journal *budgetUsageJournal, seed budgetUsageSeed) budgetUsageRecord {
	t.Helper()
	attempt := budgetUsageTestBegun(t, journal, seed)
	attempt.describeLaunch("claude", false, "2.1.34")
	if attempt.beginInvocation(false) == "" {
		t.Fatal("the launch intent has no invocation id")
	}
	return budgetUsageTestRequireStored(t, journal, attempt, "after the launch intent")
}

// budgetUsageTestLeftSealed creates the record of an attempt that ran one
// provider process and was sealed, and returns the bytes of its batch.
func budgetUsageTestLeftSealed(t *testing.T, journal *budgetUsageJournal, seed budgetUsageSeed) []byte {
	t.Helper()
	logger, _ := budgetUsageTestLogger()
	return budgetUsageTestSealed(t, journal, logger, seed)
}

// budgetUsageTestStoredRecord is a record as a file holds it, for tests that
// put a record file into the journal directory themselves.
func budgetUsageTestStoredRecord(attemptID, producerEpoch string, createdAt time.Time) budgetUsageRecord {
	return budgetUsageRecord{
		Version:           budgetUsageRecordVersion,
		AttemptID:         attemptID,
		BatchKey:          "stored-key-" + attemptID,
		ProducerEpoch:     producerEpoch,
		CreatedAt:         createdAt.UTC(),
		ServerURL:         budgetUsageTestServerURL,
		WorkspaceID:       "ws-1",
		TaskID:            "task-1",
		Adapter:           "claude",
		NormalizerVersion: "1",
		Invocations:       []budgetUsageInvocation{},
	}
}

// budgetUsageTestStore writes a record file the way the journal writes one.
func budgetUsageTestStore(t *testing.T, journal *budgetUsageJournal, record budgetUsageRecord) {
	t.Helper()
	if err := journal.write(record); err != nil {
		t.Fatalf("write the record of attempt %s: %v", record.AttemptID, err)
	}
}

// budgetUsageTestWireBatchOf decodes a request body the way the server does:
// an unknown field is an error.
func budgetUsageTestWireBatchOf(t *testing.T, body []byte) budgetUsageWireBatch {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var batch budgetUsageWireBatch
	if err := decoder.Decode(&batch); err != nil {
		t.Fatalf("decode the batch %q: %v", body, err)
	}
	return batch
}

// budgetUsageTestReportsFor returns the reports for one attempt, in order.
func budgetUsageTestReportsFor(api *budgetUsageTestAPI, attemptID string) []budgetUsageTestCall {
	var reports []budgetUsageTestCall
	for _, report := range api.reports() {
		if report.Path == budgetUsageTestReportPath(attemptID) {
			reports = append(reports, report)
		}
	}
	return reports
}

// budgetUsageTestRequireNoRecordFile fails when the attempt still has a
// record file.
func budgetUsageTestRequireNoRecordFile(t *testing.T, journal *budgetUsageJournal, attemptID, when string) {
	t.Helper()
	if _, err := os.Lstat(journal.recordPath(attemptID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s: the record file of attempt %s is still there (lstat: %v)", when, attemptID, err)
	}
}

// budgetUsageTestFileWatch remembers what a file held each time the server
// was answering a usage report: the daemon is waiting for the answer then, so
// this is what was on disk before the send was over.
type budgetUsageTestFileWatch struct {
	mu   sync.Mutex
	seen [][]byte
}

// budgetUsageTestWatching wraps an answer so that the file is read before
// each report is answered.
func budgetUsageTestWatching(path string, answer budgetUsageTestAnswer) (budgetUsageTestAnswer, *budgetUsageTestFileWatch) {
	watch := &budgetUsageTestFileWatch{}
	return func(nth int, w http.ResponseWriter) {
		raw, _ := os.ReadFile(path)
		watch.mu.Lock()
		watch.seen = append(watch.seen, raw)
		watch.mu.Unlock()
		answer(nth, w)
	}, watch
}

func (w *budgetUsageTestFileWatch) contents() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([][]byte(nil), w.seen...)
}

// budgetUsageTestUnavailable is the answer of a server that cannot take the
// batch now: the record stays.
func budgetUsageTestUnavailable() budgetUsageTestAnswer {
	return budgetUsageTestRefusing(http.StatusServiceUnavailable, "ledger_unavailable")
}

// TestBudgetUsageRestartResendsThePersistedBatchByteForByte: a record that an
// earlier process sealed is sent by the next process exactly as it is on
// disk, under its key, as often as it takes. It is not rebuilt: the batch
// still names the process that built it. A sealed batch without an invocation
// is such a batch too; it says that the attempt launched nothing.
func TestBudgetUsageRestartResendsThePersistedBatchByteForByte(t *testing.T) {
	for _, tc := range []struct {
		name     string
		launches bool
	}{
		{name: "a batch with an invocation", launches: true},
		{name: "an empty batch", launches: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "journal")
			clock := budgetUsageTestNewClock()
			earlier := budgetUsageTestProcess(t, directory, clock)
			var sealed []byte
			if tc.launches {
				sealed = budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
			} else {
				sealed = budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA)).seal()
				if !bytes.Contains(sealed, []byte(`"invocations":[]`)) {
					t.Fatalf("the batch of an attempt that launched nothing is %s", sealed)
				}
			}
			fileAsLeft := budgetUsageTestFileText(t, earlier, budgetUsageTestAttemptA)
			wantKey := budgetUsageTestBatchKey(t, sealed)
			// The earlier process ends here, before any answer.

			clock.advance(time.Hour)
			api := budgetUsageTestNewAPI(t, budgetUsageTestThen(budgetUsageTestUnavailable(), budgetUsageTestRecorded()))
			d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
			if journal.epoch == earlier.epoch {
				t.Fatal("the two processes have the same start identity")
			}
			ctx := context.Background()

			d.resendBudgetUsage(ctx)
			reports := api.reports()
			if len(reports) != 1 {
				t.Fatalf("%d usage reports after the first pass, want 1", len(reports))
			}
			report := reports[0]
			if report.Method != http.MethodPost || report.Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
				t.Errorf("the report is %s %s", report.Method, report.Path)
			}
			if report.ContentType != "application/json" || report.Authorization != "Bearer "+budgetUsageTestToken {
				t.Errorf("Content-Type = %q, Authorization = %q", report.ContentType, report.Authorization)
			}
			if !bytes.Equal(report.Body, sealed) {
				t.Errorf("the report does not carry the persisted bytes\n sent:      %s\n persisted: %s", report.Body, sealed)
			}
			// Not delivered: the record is as the earlier process left it.
			if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileAsLeft {
				t.Errorf("the record file changed\n now:  %s\n left: %s", got, fileAsLeft)
			}

			clock.advance(time.Hour)
			d.resendBudgetUsage(ctx)
			reports = api.reports()
			if len(reports) != 2 {
				t.Fatalf("%d usage reports after the second pass, want 2", len(reports))
			}
			if !bytes.Equal(reports[1].Body, sealed) {
				t.Errorf("the second report does not carry the persisted bytes: %s", reports[1].Body)
			}
			batch := budgetUsageTestWireBatchOf(t, reports[1].Body)
			if batch.BatchKey != wantKey || batch.ProducerEpoch != earlier.epoch {
				t.Errorf("batch_key = %q, producer_epoch = %q; want the key %q and the process that built the batch %q",
					batch.BatchKey, batch.ProducerEpoch, wantKey, earlier.epoch)
			}
			budgetUsageTestRequireNoRecordFile(t, journal, budgetUsageTestAttemptA, "after the batch was delivered")
			if got := budgetUsageTestLiveCount(journal); got != 0 {
				t.Errorf("the later process holds %d records of its own, want 0", got)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 0 {
				t.Errorf("%d warnings:\n%s", got, logs.String())
			}
		})
	}
}

// TestBudgetUsageRestartReportsAnInvocationWithoutAnOutcomeAsUnknown: a
// record that an earlier process left without batch bytes is built by the
// next process. An invocation that has an outcome keeps it. One that has only
// its launch intent is reported as unknown, with no usage and the time of the
// intent. The batch names the process that builds it and the time it is built
// at, and keeps the key the record was created with. Its bytes are on disk
// before they are sent, and from then on they are what is sent.
func TestBudgetUsageRestartReportsAnInvocationWithoutAnOutcomeAsUnknown(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	attempt := budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	attempt.describeLaunch("claude", false, "2.1.34")
	first := attempt.beginInvocation(false)
	clock.advance(2 * time.Minute)
	attempt.endInvocation(first, budgetUsageEndedReturned, budgetUsageTestUsage())
	clock.advance(time.Minute)
	second := attempt.beginInvocation(true)
	left := budgetUsageTestRequireStored(t, earlier, attempt, "when the earlier process died")
	if len(left.Batch) != 0 || len(left.Invocations) != 2 {
		t.Fatalf("the earlier process left %d batch bytes and %d invocations", len(left.Batch), len(left.Invocations))
	}
	// The earlier process dies here, during its second provider run.

	clock.advance(time.Hour)
	answer, watch := budgetUsageTestWatching(earlier.recordPath(budgetUsageTestAttemptA),
		budgetUsageTestThen(budgetUsageTestUnavailable(), budgetUsageTestRecorded()))
	api := budgetUsageTestNewAPI(t, answer)
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()

	d.resendBudgetUsage(ctx)
	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
		t.Fatalf("usage reports after the first pass: %d, want one for the attempt", len(reports))
	}
	body := reports[0].Body
	batch := budgetUsageTestWireBatchOf(t, body)
	want := budgetUsageWireBatch{
		ProtocolVersion:   1,
		BatchKey:          left.BatchKey,
		ProducerEpoch:     journal.epoch,
		Adapter:           "claude",
		AdapterVersion:    "2.1.34",
		NormalizerVersion: "1",
		ReportedAt:        "2026-11-17T09:03:00Z",
		Invocations: []budgetUsageWireInvocation{
			{
				InvocationID: first,
				Resumed:      false,
				Ended:        "returned",
				ObservedAt:   "2026-11-17T08:02:00Z",
				Usage: []budgetUsageWireUsage{{
					Model:     "model-x",
					Tokens:    budgetUsageWireTokens{Input: "11", Output: "7", CacheRead: "5", CacheWrite: "3"},
					CostTicks: "2",
				}},
			},
			{
				InvocationID: second,
				Resumed:      true,
				Ended:        "unknown",
				// The time of the launch intent: the only time the daemon knows
				// of this invocation.
				ObservedAt: "2026-11-17T08:03:00Z",
				Usage:      []budgetUsageWireUsage{},
			},
		},
	}
	if !reflect.DeepEqual(batch, want) {
		t.Errorf("the batch differs\n got:  %+v\n want: %+v", batch, want)
	}
	if !bytes.Contains(body, []byte(`"ended":"unknown","observed_at":"2026-11-17T08:03:00Z","usage":[]`)) {
		t.Errorf("the invocation without an outcome is not reported with an empty usage array: %s", body)
	}
	if journal.epoch == earlier.epoch {
		t.Fatal("the two processes have the same start identity")
	}

	// The bytes were on disk before they were sent.
	seen := watch.contents()
	if len(seen) != 1 {
		t.Fatalf("the record file was looked at %d times, want 1", len(seen))
	}
	if stored := budgetUsageTestRecordAs(t, seen[0], budgetUsageTestAttemptA); !bytes.Equal(stored.Batch, body) {
		t.Errorf("when the report arrived the record file held the batch %s, want the bytes that were sent", stored.Batch)
	}
	// Not delivered: the record stays, with those bytes and nothing else
	// changed.
	stored, err := journal.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatalf("read the record after the first pass: %v", err)
	}
	wantStored := left.clone()
	wantStored.Batch = body
	if !reflect.DeepEqual(stored, wantStored) {
		t.Errorf("the record on disk differs\n got:  %#v\n want: %#v", stored, wantStored)
	}

	// A batch that has bytes is never rebuilt, however late it is sent.
	clock.advance(3 * time.Hour)
	d.resendBudgetUsage(ctx)
	reports = api.reports()
	if len(reports) != 2 {
		t.Fatalf("%d usage reports after the second pass, want 2", len(reports))
	}
	if !bytes.Equal(reports[1].Body, body) {
		t.Errorf("the second report differs from the first\n first:  %s\n second: %s", body, reports[1].Body)
	}
	budgetUsageTestRequireNoRecordFile(t, journal, budgetUsageTestAttemptA, "after the batch was delivered")
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// budgetUsageTestRequireDropped fails unless the attempt's record is gone
// from disk and from the process, with exactly wantWarnings warnings in the
// log so far, the last of which names the attempt.
func budgetUsageTestRequireDropped(t *testing.T, journal *budgetUsageJournal, logs *budgetUsageTestLog, attemptID string, wantWarnings int) {
	t.Helper()
	budgetUsageTestRequireNoRecordFile(t, journal, attemptID, "after the record was dropped")
	if budgetUsageTestLiveAttempt(journal, attemptID) != nil {
		t.Errorf("the process still holds the record of attempt %s", attemptID)
	}
	warnings := budgetUsageTestWarningLines(logs.String())
	if len(warnings) != wantWarnings {
		t.Fatalf("%d warnings, want %d:\n%s", len(warnings), wantWarnings, logs.String())
	}
	if wantWarnings > 0 {
		budgetUsageTestRequireLogAttr(t, warnings[len(warnings)-1], "attempt_id", attemptID)
	}
}

// TestBudgetUsageRecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation:
// a record file without batch bytes and without a launch intent does not say
// whether a provider process ran. The process that created it may have died
// before the first launch, or the first launch intent may not have reached
// the file, or the run may have gone on without the protocol. An empty batch
// would claim that nothing was launched, so recovery reports nothing: it
// removes the file with one warning, whichever process created it.
func TestBudgetUsageRecoveryNeverReportsARecordWithoutBatchAndWithoutInvocation(t *testing.T) {
	t.Run("left by an earlier process", func(t *testing.T) {
		directory := filepath.Join(t.TempDir(), "journal")
		clock := budgetUsageTestNewClock()
		earlier := budgetUsageTestProcess(t, directory, clock)
		attempt := budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
		// Knowing what would have been launched is not a launch.
		attempt.describeLaunch("claude", false, "2.1.34")
		budgetUsageTestRequireStored(t, earlier, attempt, "when the earlier process died")

		clock.advance(time.Hour)
		api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
		d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
		d.resendBudgetUsage(context.Background())
		budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 1)

		d.resendBudgetUsage(context.Background())
		if got := budgetUsageTestWarnings(logs.String()); got != 1 {
			t.Errorf("%d warnings after a second pass, want the one:\n%s", got, logs.String())
		}
		if reports := api.reports(); len(reports) != 0 {
			t.Errorf("%d usage reports for a record that holds no launch, want none: %s", len(reports), reports[0].Body)
		}
		if names := budgetUsageTestNames(t, directory); len(names) != 0 {
			t.Errorf("the journal holds %v", names)
		}
	})

	t.Run("left by this process", func(t *testing.T) {
		directory := filepath.Join(t.TempDir(), "journal")
		clock := budgetUsageTestNewClock()
		api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
		d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
		// A file of this process that no attempt of it holds: its creation
		// failed after the file was in place, and the file could not be taken
		// away again.
		budgetUsageTestStore(t, journal, budgetUsageTestStoredRecord(budgetUsageTestAttemptA, journal.epoch, clock.Now()))

		clock.advance(time.Minute)
		d.resendBudgetUsage(context.Background())
		budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 1)
		if reports := api.reports(); len(reports) != 0 {
			t.Errorf("%d usage reports for a record that holds no launch, want none: %s", len(reports), reports[0].Body)
		}
	})
}

// TestBudgetUsageRecoveryDropsARecordOfThisProcessThatNoRunningAttemptHolds:
// a record without batch bytes that this process created and that none of
// its attempts holds is a leftover. Nothing of this process will ever finish
// it, and it is not what an earlier process left behind: it is removed with
// one warning and never reported, even when it names a launch.
func TestBudgetUsageRecoveryDropsARecordOfThisProcessThatNoRunningAttemptHolds(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	leftover := budgetUsageTestStoredRecord(budgetUsageTestAttemptA, journal.epoch, clock.Now())
	leftover.Invocations = []budgetUsageInvocation{{
		InvocationID: "0195f3e0-0000-7000-8000-000000000001",
		LaunchedAt:   clock.Now(),
	}}
	budgetUsageTestStore(t, journal, leftover)

	clock.advance(time.Minute)
	d.resendBudgetUsage(context.Background())
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 1)

	d.resendBudgetUsage(context.Background())
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("%d warnings after a second pass, want the one:\n%s", got, logs.String())
	}
	if reports := api.reports(); len(reports) != 0 {
		t.Errorf("%d usage reports for a leftover of this process, want none: %s", len(reports), reports[0].Body)
	}

	// The same file from an earlier process is the record of a run that
	// process did not finish, and that is reported.
	earlierRecord := leftover
	earlierRecord.AttemptID = budgetUsageTestAttemptB
	earlierRecord.BatchKey = "stored-key-" + budgetUsageTestAttemptB
	earlierRecord.ProducerEpoch = "an-earlier-process"
	budgetUsageTestStore(t, journal, earlierRecord)
	d.resendBudgetUsage(context.Background())
	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptB) {
		t.Fatalf("usage reports for the record of an earlier process: %d, want one for that attempt", len(reports))
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("%d warnings after the record of an earlier process was sent, want the one:\n%s", got, logs.String())
	}
}

// TestBudgetUsageRecoveryLeavesARunningAttemptAlone: a record that an
// attempt of this process holds belongs to a run that is under way. A pass
// does not send it, does not remove it and does not change its file, before
// the first launch and during one, however old the record is. When the run
// is over the attempt is sealed and sent as usual.
func TestBudgetUsageRecoveryLeavesARunningAttemptAlone(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()
	attempt := budgetUsageTestBegun(t, journal, budgetUsageTestSeed(budgetUsageTestAttemptA))

	requireUntouched := func(when string) {
		t.Helper()
		fileBefore := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
		d.resendBudgetUsage(ctx)
		if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileBefore {
			t.Errorf("%s: a pass changed the record file\n now:    %s\n before: %s", when, got, fileBefore)
		}
		if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) != attempt {
			t.Errorf("%s: a pass took the record away from its attempt", when)
		}
		if reports := api.reports(); len(reports) != 0 {
			t.Errorf("%s: %d usage reports for a run that is under way", when, len(reports))
		}
		if got := budgetUsageTestWarnings(logs.String()); got != 0 {
			t.Errorf("%s: %d warnings:\n%s", when, got, logs.String())
		}
	}

	requireUntouched("before the first launch")
	attempt.describeLaunch("claude", false, "2.1.34")
	invocationID := attempt.beginInvocation(false)
	requireUntouched("during a provider run")
	clock.advance(budgetUsageRetention + time.Hour)
	requireUntouched("during a provider run that outlasts the retention cap")

	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	d.finishBudgetUsageAttempt(ctx, attempt)
	budgetUsageTestWaitForSends(t, journal)
	reports := api.reports()
	if len(reports) != 1 || !bytes.Equal(reports[0].Body, attempt.seal()) {
		t.Fatalf("usage reports when the run was over: %d, want one with the sealed bytes", len(reports))
	}
	if got := budgetUsageTestBatchInvocations(t, reports[0].Body); len(got) != 1 || got[0].Ended != budgetUsageEndedReturned {
		t.Errorf("the batch reports %+v, want the one invocation as it ended", got)
	}
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 0)
}

// TestBudgetUsageRestartKeepsARecordOfAnotherServer: a record goes only to
// the server it was created under. A process that talks to another server
// leaves it exactly as it is, whatever it holds: it does not send it, does
// not build its batch and does not remove it. The profile may be pointed
// back, and then the process of its own server handles it.
func TestBudgetUsageRestartKeepsARecordOfAnotherServer(t *testing.T) {
	const (
		anotherServer  = "https://another.example.test"
		attemptNothing = "0195f3a0-0000-7000-8000-00000000000c"
	)
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcessOf(t, directory, anotherServer, clock)
	sealed := budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	unsealed := budgetUsageTestLeftRunning(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptB))
	budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(attemptNothing))
	attempts := []string{budgetUsageTestAttemptA, budgetUsageTestAttemptB, attemptNothing}
	filesAsLeft := make(map[string]string)
	for _, attemptID := range attempts {
		filesAsLeft[attemptID] = budgetUsageTestFileText(t, earlier, attemptID)
	}

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(context.Background())
		if calls := api.seen(); len(calls) != 0 {
			t.Fatalf("pass %d: %d requests reached the server, the first to %s", pass, len(calls), calls[0].Path)
		}
		for _, attemptID := range attempts {
			if got := budgetUsageTestFileText(t, journal, attemptID); got != filesAsLeft[attemptID] {
				t.Errorf("pass %d: the record of attempt %s, which belongs to another server, changed\n now:  %s\n left: %s",
					pass, attemptID, got, filesAsLeft[attemptID])
			}
		}
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}

	// The process of the server the records belong to handles all three.
	back := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	ownerLogger, ownerLogs := budgetUsageTestLogger()
	owner := budgetUsageTestProcessOf(t, directory, anotherServer, clock)
	ownerDaemon := budgetUsageTestNewDaemon(&budgetUsageTestServer{URL: back.URL}, owner, ownerLogger)
	ownerDaemon.resendBudgetUsage(context.Background())
	if reports := budgetUsageTestReportsFor(back, budgetUsageTestAttemptA); len(reports) != 1 || !bytes.Equal(reports[0].Body, sealed) {
		t.Errorf("the server of the sealed record got %d reports for it, want one with the persisted bytes", len(reports))
	}
	reports := budgetUsageTestReportsFor(back, budgetUsageTestAttemptB)
	if len(reports) != 1 {
		t.Fatalf("the server of the record without batch bytes got %d reports for it, want one", len(reports))
	}
	if batch := budgetUsageTestWireBatchOf(t, reports[0].Body); batch.BatchKey != unsealed.BatchKey || batch.ProducerEpoch != owner.epoch {
		t.Errorf("batch_key = %q, producer_epoch = %q; want the key of the record %q and the process that built the batch %q",
			batch.BatchKey, batch.ProducerEpoch, unsealed.BatchKey, owner.epoch)
	}
	if reports := budgetUsageTestReportsFor(back, attemptNothing); len(reports) != 0 {
		t.Errorf("%d reports for the record that holds no launch, want none", len(reports))
	}
	if got := len(back.reports()); got != 2 {
		t.Errorf("%d usage reports in all, want 2", got)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal still holds %v", names)
	}
	// One warning: the record that holds no launch was removed.
	if got := budgetUsageTestWarnings(ownerLogs.String()); got != 1 {
		t.Errorf("%d warnings in the process of the records' own server, want 1:\n%s", got, ownerLogs.String())
	}
}

// TestBudgetUsageRecordOlderThanSevenDaysIsDroppedWithAWarning: a record is
// kept for seven days from its creation at most. A pass drops an older one
// with one warning and without sending it, whatever it holds and whichever
// process created it: its file goes, and so does what this process holds of
// it. A record that is exactly seven days old is still handled as before.
func TestBudgetUsageRecordOlderThanSevenDaysIsDroppedWithAWarning(t *testing.T) {
	const sevenDays = 7 * 24 * time.Hour
	if budgetUsageRetention != sevenDays {
		t.Fatalf("the retention cap is %s, want seven days", budgetUsageRetention)
	}
	type fixture struct {
		d       *Daemon
		journal *budgetUsageJournal
		api     *budgetUsageTestAPI
		logs    *budgetUsageTestLog
		clock   *budgetUsageTestClock
		created time.Time
	}
	// leave creates the record of attempt A at the start of the clock and
	// returns the daemon of the process that handles it afterwards.
	for _, tc := range []struct {
		name  string
		leave func(t *testing.T, directory string, clock *budgetUsageTestClock, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog)
		// reportsPerPass is how many reports a pass sends for the record while
		// it is not older than the cap.
		reportsPerPass int
		// reportsBefore is how many reports went out before the first pass.
		reportsBefore int
	}{
		{
			name: "sealed by an earlier process",
			leave: func(t *testing.T, directory string, clock *budgetUsageTestClock, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
				budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
				return budgetUsageTestRestarted(t, api, directory, clock)
			},
			reportsPerPass: 1,
		},
		{
			name: "left without a batch by an earlier process",
			leave: func(t *testing.T, directory string, clock *budgetUsageTestClock, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
				budgetUsageTestLeftRunning(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
				return budgetUsageTestRestarted(t, api, directory, clock)
			},
			reportsPerPass: 1,
		},
		{
			name: "sealed by this process and not delivered",
			leave: func(t *testing.T, directory string, clock *budgetUsageTestClock, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
				d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
				budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
				d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)
				if budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA) == nil {
					t.Fatal("the process does not hold the record that was not delivered")
				}
				return d, journal, logs
			},
			reportsPerPass: 1,
			reportsBefore:  1,
		},
		{
			name: "of another server",
			leave: func(t *testing.T, directory string, clock *budgetUsageTestClock, api *budgetUsageTestAPI) (*Daemon, *budgetUsageJournal, *budgetUsageTestLog) {
				earlier := budgetUsageTestProcessOf(t, directory, "https://another.example.test", clock)
				budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
				return budgetUsageTestRestarted(t, api, directory, clock)
			},
			reportsPerPass: 0,
		},
	} {
		for _, startAtTheCap := range []bool{true, false} {
			name := tc.name + ", first handled when it is older"
			if startAtTheCap {
				name = tc.name + ", first handled when it is seven days old"
			}
			t.Run(name, func(t *testing.T) {
				directory := filepath.Join(t.TempDir(), "journal")
				clock := budgetUsageTestNewClock()
				created := clock.Now()
				api := budgetUsageTestNewAPI(t, budgetUsageTestUnavailable())
				d, journal, logs := tc.leave(t, directory, clock, api)
				f := fixture{d: d, journal: journal, api: api, logs: logs, clock: clock, created: created}
				ctx := context.Background()
				wantReports := tc.reportsBefore

				if startAtTheCap {
					f.clock.set(f.created.Add(sevenDays))
					f.d.resendBudgetUsage(ctx)
					wantReports += tc.reportsPerPass
					if got := len(f.api.reports()); got != wantReports {
						t.Fatalf("%d usage reports when the record is seven days old, want %d", got, wantReports)
					}
					if _, err := os.Lstat(f.journal.recordPath(budgetUsageTestAttemptA)); err != nil {
						t.Fatalf("the record is gone when it is seven days old: %v", err)
					}
					if got := budgetUsageTestWarnings(f.logs.String()); got != 0 {
						t.Fatalf("%d warnings when the record is seven days old:\n%s", got, f.logs.String())
					}
				}

				f.clock.set(f.created.Add(sevenDays + time.Second))
				f.d.resendBudgetUsage(ctx)
				budgetUsageTestRequireDropped(t, f.journal, f.logs, budgetUsageTestAttemptA, 1)
				f.d.resendBudgetUsage(ctx)
				if got := len(f.api.reports()); got != wantReports {
					t.Errorf("%d usage reports in all, want %d: a record that is older than the cap is not sent", got, wantReports)
				}
				if got := budgetUsageTestWarnings(f.logs.String()); got != 1 {
					t.Errorf("%d warnings after another pass, want the one:\n%s", got, f.logs.String())
				}
				if names := budgetUsageTestNames(t, directory); len(names) != 0 {
					t.Errorf("the journal still holds %v", names)
				}
			})
		}
	}
}

// The direct send can try an expired batch when it wins before the retention
// pass. After an unsuccessful send, the next pass drops the record. The reverse
// ordering is covered in budget_usage_retention_order_test.go.
func TestBudgetUsageOutlastedBatchIsSentWhenDirectSendWins(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestUnavailable())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()
	attempt := budgetUsageTestBegun(t, journal, budgetUsageTestSeed(budgetUsageTestAttemptA))
	invocationID := attempt.beginInvocation(false)
	clock.advance(budgetUsageRetention + time.Hour)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())

	d.finishBudgetUsageAttempt(ctx, attempt)
	budgetUsageTestWaitForSends(t, journal)
	reports := api.reports()
	if len(reports) != 1 || !bytes.Equal(reports[0].Body, attempt.seal()) {
		t.Fatalf("usage reports after the seal: %d, want one with the sealed bytes", len(reports))
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Fatalf("%d warnings after the send that follows the seal:\n%s", got, logs.String())
	}
	if _, err := os.Lstat(journal.recordPath(budgetUsageTestAttemptA)); err != nil {
		t.Fatalf("the record that was not delivered is gone: %v", err)
	}

	d.resendBudgetUsage(ctx)
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 1)
	if got := len(api.reports()); got != 1 {
		t.Errorf("%d usage reports in all, want 1", got)
	}
}

// TestBudgetUsageRebuiltBatchAnsweredWithOperationConflictIsDropped: a batch
// that a later process builds from the record file can differ from one the
// earlier process sent from memory under the same key, when that process
// could not write its batch bytes and died before the answer. The server then
// answers 409 operation_conflict. That is a final refusal like every other:
// the record is removed with one warning that names the status and the code,
// and nothing new is minted for the attempt: no other key, no other attempt,
// no other invocation, no second report.
func TestBudgetUsageRebuiltBatchAnsweredWithOperationConflictIsDropped(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	attempt := budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	attempt.describeLaunch("claude", false, "2.1.34")
	invocationID := attempt.beginInvocation(false)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	// The file holds the outcome and no batch bytes: the write of the seal
	// failed. The earlier process sent its batch from memory and died.
	left := budgetUsageTestRequireStored(t, earlier, attempt, "when the earlier process died")

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestReplying(http.StatusConflict, `{"error":"operation_conflict","code":"operation_conflict"}`))
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()
	d.resendBudgetUsage(ctx)

	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) {
		t.Fatalf("usage reports: %d, want one for the attempt", len(reports))
	}
	batch := budgetUsageTestWireBatchOf(t, reports[0].Body)
	if batch.BatchKey != left.BatchKey {
		t.Errorf("batch_key = %q, want the key of the record %q", batch.BatchKey, left.BatchKey)
	}
	if len(batch.Invocations) != 1 || batch.Invocations[0].InvocationID != invocationID || batch.Invocations[0].Ended != budgetUsageEndedReturned {
		t.Errorf("the rebuilt batch reports %+v, want the one invocation of the record as it ended", batch.Invocations)
	}
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 1)
	warning := budgetUsageTestWarningLines(logs.String())[0]
	budgetUsageTestRequireLogAttr(t, warning, "status", "409")
	budgetUsageTestRequireLogAttr(t, warning, "code", "operation_conflict")
	budgetUsageTestRequireNoBodyLogged(t, logs.String(), reports[0].Body)

	// Nothing takes the place of the refused batch.
	for pass := 2; pass <= 3; pass++ {
		clock.advance(time.Minute)
		d.resendBudgetUsage(ctx)
	}
	if got := len(api.seen()); got != 1 {
		t.Errorf("%d requests reached the server in all, want the one refused report", got)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal holds %v after the refusal, want nothing", names)
	}
	if got := budgetUsageTestLiveCount(journal); got != 0 {
		t.Errorf("the process holds %d records, want none", got)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("%d warnings, want the one:\n%s", got, logs.String())
	}
}

// budgetUsageTestPlace puts a file with the given modification time into the
// journal directory.
func budgetUsageTestPlace(t *testing.T, directory, name string, content []byte, modified time.Time) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	return path
}

// budgetUsageTestWithout returns names without the given ones.
func budgetUsageTestWithout(names []string, gone ...string) []string {
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool { return slices.Contains(gone, name) })
}

// TestBudgetUsageRetentionRemovesOldTemporaryFilesAndNoOtherForeignName: a
// write that was interrupted between the creation of its temporary file and
// the rename leaves that file behind, and so does a probe whose removal
// failed. A pass removes such a file (a regular file whose name starts with a
// dot and ends in .tmp) once its modification time is older than the cap. A
// younger one may belong to a write that is under way and stays. No other
// name that is not a record is touched, however old, and the record next to
// them is handled as if they were not there.
func TestBudgetUsageRetentionRemovesOldTemporaryFilesAndNoOtherForeignName(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	sealed := budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, _, logs := budgetUsageTestRestarted(t, api, directory, clock)
	now := clock.Now()
	young := now.Add(-time.Hour)
	atTheCap := now.Add(-budgetUsageRetention)
	old := now.Add(-budgetUsageRetention - time.Second)
	content := []byte("left behind")
	for name, modified := range map[string]time.Time{
		".installation-young.tmp":      young,
		".installation-at-the-cap.tmp": atTheCap,
		".installation-old.tmp":        old,
		".probe-young.tmp":             young,
		".probe-old.tmp":               old,
		// Old, and none of them a temporary file of the journal.
		"notes.txt":     old,
		".hidden":       old,
		"leftover.tmp":  old,
		".keep.tmp.bak": old,
	} {
		budgetUsageTestPlace(t, directory, name, content, modified)
	}
	// Old and named like a temporary file, but not a regular file.
	oldDirectory := filepath.Join(directory, ".directory.tmp")
	if err := os.Mkdir(oldDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldDirectory, old, old); err != nil {
		t.Fatal(err)
	}
	before := budgetUsageTestNames(t, directory)

	d.resendBudgetUsage(context.Background())
	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) || !bytes.Equal(reports[0].Body, sealed) {
		t.Fatalf("usage reports: %d, want one for the record with its persisted bytes", len(reports))
	}
	want := budgetUsageTestWithout(before, budgetUsageTestAttemptA+".json", ".installation-old.tmp", ".probe-old.tmp")
	if got := budgetUsageTestNames(t, directory); !reflect.DeepEqual(got, want) {
		t.Errorf("after the pass the journal holds\n %v\nwant\n %v", got, want)
	}

	// Time passes: what was young is old now.
	clock.advance(budgetUsageRetention)
	d.resendBudgetUsage(context.Background())
	want = budgetUsageTestWithout(want, ".installation-young.tmp", ".installation-at-the-cap.tmp", ".probe-young.tmp")
	if got := budgetUsageTestNames(t, directory); !reflect.DeepEqual(got, want) {
		t.Errorf("seven days later the journal holds\n %v\nwant\n %v", got, want)
	}
	for _, name := range []string{"notes.txt", ".hidden", "leftover.tmp", ".keep.tmp.bak"} {
		if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || !bytes.Equal(got, content) {
			t.Errorf("%s is no longer as it was: %q, %v", name, got, err)
		}
	}
	if got := len(api.reports()); got != 1 {
		t.Errorf("%d usage reports in all, want 1", got)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsageRecoveryActsOnlyOnCanonicalRecordNames: a record file is
// named <canonical lower-case UUID>.json. A pass skips every other name: it
// does not read the file, does not report what it holds and does not count it
// as an error, even when the file holds a complete sealed record.
func TestBudgetUsageRecoveryActsOnlyOnCanonicalRecordNames(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	sealed := budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	record := []byte(budgetUsageTestFileText(t, earlier, budgetUsageTestAttemptA))
	foreign := []string{
		"0195F3A0-0000-7000-8000-0000000000AA.json", // not lower case
		"0195f3a0-0000-7000-8000-0000000000ab.JSON",
		"0195f3a0-0000-7000-8000-0000000000ac.json.bak",
		"0195f3a0-0000-7000-8000-0000000000ad",
		".0195f3a0-0000-7000-8000-0000000000ae.json",
		"0195f3a000007000800000000000000af.json", // no hyphens
		"plain-name.json",
	}
	old := clock.Now().Add(-budgetUsageRetention - time.Hour)
	for _, name := range foreign {
		path := budgetUsageTestPlace(t, directory, name, record, old)
		if runtime.GOOS != "windows" && os.Geteuid() != 0 {
			// A file that cannot be opened: reading it would be an error.
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
		}
	}

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, _, logs := budgetUsageTestRestarted(t, api, directory, clock)
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(context.Background())
		clock.advance(budgetUsageRetention + time.Hour)
	}
	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) || !bytes.Equal(reports[0].Body, sealed) {
		t.Fatalf("usage reports: %d, want one for the record with the canonical name", len(reports))
	}
	if got := len(api.seen()); got != 1 {
		t.Errorf("%d requests reached the server, want 1", got)
	}
	if got, want := budgetUsageTestNames(t, directory), budgetUsageTestSorted(foreign); !reflect.DeepEqual(got, want) {
		t.Errorf("the journal holds\n %v\nwant every file that is not a record\n %v", got, want)
	}
	logged := logs.String()
	if got := budgetUsageTestWarnings(logged); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logged)
	}
	for _, name := range foreign {
		if strings.Contains(logged, strings.TrimSuffix(name, ".json")) {
			t.Errorf("the log names %s, a file that is not a record:\n%s", name, logged)
		}
	}
}

// budgetUsageTestSorted returns names in the order a directory listing has.
func budgetUsageTestSorted(names []string) []string {
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	return sorted
}

// TestBudgetUsageUnreadableRecordIsSkippedUntilTheCapAndThenDropped: a file
// with the name of a record that cannot be read as the record of its attempt
// is not sent and not reported. It stays while it is not older than the cap,
// by its modification time, and is then removed with one warning. A name of
// a record that is not a regular file is never removed. Nothing the file
// holds reaches the log.
func TestBudgetUsageUnreadableRecordIsSkippedUntilTheCapAndThenDropped(t *testing.T) {
	const (
		attemptCutShort    = "0195f3f0-0000-7000-8000-000000000001"
		attemptLaterFormat = "0195f3f0-0000-7000-8000-000000000002"
		attemptMisnamed    = "0195f3f0-0000-7000-8000-000000000003"
		attemptDirectory   = "0195f3f0-0000-7000-8000-000000000004"
		attemptLink        = "0195f3f0-0000-7000-8000-000000000005"
		fileText           = "FILE-TEXT-THAT-MUST-NOT-BE-LOGGED"
	)
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	written := clock.Now()

	sealedRecord := budgetUsageTestStoredRecord(attemptMisnamed, "an-earlier-process", written)
	sealedRecord.Batch = []byte(`{"batch":"` + fileText + `"}`)
	// The record of another attempt under this attempt's name.
	misnamed := sealedRecord
	misnamed.AttemptID = budgetUsageTestAttemptB
	misnamedJSON := budgetUsageTestRecordJSON(t, misnamed)
	laterFormat := sealedRecord
	laterFormat.AttemptID = attemptLaterFormat
	laterFormat.Version = budgetUsageRecordVersion + 1
	laterFormatJSON := budgetUsageTestRecordJSON(t, laterFormat)
	budgetUsageTestPlace(t, directory, attemptCutShort+".json", []byte(`{"version":1,"attempt_id":"`+fileText), written)
	budgetUsageTestPlace(t, directory, attemptLaterFormat+".json", []byte(laterFormatJSON), written)
	budgetUsageTestPlace(t, directory, attemptMisnamed+".json", []byte(misnamedJSON), written)
	notRegular := []string{attemptDirectory + ".json"}
	if err := os.Mkdir(filepath.Join(directory, attemptDirectory+".json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(directory, attemptDirectory+".json"), written, written); err != nil {
		t.Fatal(err)
	}
	// A link to a complete sealed record outside the journal: following it
	// would send that record.
	outside := filepath.Join(t.TempDir(), "outside.json")
	linked := sealedRecord
	linked.AttemptID = attemptLink
	if err := os.WriteFile(outside, []byte(budgetUsageTestRecordJSON(t, linked)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, attemptLink+".json")); err == nil {
		notRegular = append(notRegular, attemptLink+".json")
	} else {
		t.Logf("no symbolic link on this system, leaving that case out: %v", err)
	}
	before := budgetUsageTestNames(t, directory)

	// Not older than the cap: everything stays, twice over.
	clock.set(written.Add(budgetUsageRetention))
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(context.Background())
		if got := budgetUsageTestNames(t, directory); !reflect.DeepEqual(got, before) {
			t.Fatalf("pass %d: the journal holds %v, want %v", pass, got, before)
		}
		if got := budgetUsageTestWarnings(logs.String()); got != 0 {
			t.Fatalf("pass %d: %d warnings:\n%s", pass, got, logs.String())
		}
	}

	// Older than the cap: the regular files go, one warning each.
	clock.set(written.Add(budgetUsageRetention + time.Second))
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(context.Background())
	}
	if got := budgetUsageTestNames(t, directory); !reflect.DeepEqual(got, budgetUsageTestSorted(notRegular)) {
		t.Errorf("the journal holds %v, want only what is not a regular file: %v", got, notRegular)
	}
	warnings := budgetUsageTestWarningLines(logs.String())
	if len(warnings) != 3 {
		t.Fatalf("%d warnings, want one for each of the three files:\n%s", len(warnings), logs.String())
	}
	for _, attemptID := range []string{attemptCutShort, attemptLaterFormat, attemptMisnamed} {
		named := 0
		for _, warning := range warnings {
			if got, _ := budgetUsageTestLogAttr(warning, "attempt_id"); got == attemptID {
				named++
			}
		}
		if named != 1 {
			t.Errorf("%d warnings name attempt %s, want 1:\n%s", named, attemptID, logs.String())
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("the file outside the journal: %v", err)
	}
	if calls := api.seen(); len(calls) != 0 {
		t.Errorf("%d requests reached the server, the first to %s", len(calls), calls[0].Path)
	}
	if got := budgetUsageTestLiveCount(journal); got != 0 {
		t.Errorf("the process holds %d records, want none", got)
	}
	if strings.Contains(logs.String(), fileText) {
		t.Errorf("the log carries text of a file:\n%s", logs.String())
	}
}

// TestBudgetUsagePassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped:
// one pass handles a bounded number of records, in the order of their attempt
// ids, and the next pass goes on after the last one handled, around the end
// of the list. So a journal that holds more than one pass takes does not
// starve the records at its end, and no pass sends a record twice.
func TestBudgetUsagePassHandlesAtMostThirtyTwoRecordsAndTakesUpWhereItStopped(t *testing.T) {
	const (
		perPass = 32
		records = 40
	)
	if maxBudgetUsageRecordsPerPass != perPass {
		t.Fatalf("a pass handles up to %d records, want %d", maxBudgetUsageRecordsPerPass, perPass)
	}
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	var delivering atomic.Bool
	api := budgetUsageTestNewAPI(t, func(nth int, w http.ResponseWriter) {
		if delivering.Load() {
			budgetUsageTestRecorded()(nth, w)
			return
		}
		budgetUsageTestUnavailable()(nth, w)
	})
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ids := make([]string, records)
	batches := make(map[string][]byte, records)
	for n := range ids {
		ids[n] = fmt.Sprintf("0195f3d0-0000-7000-8000-%012d", n)
		record := budgetUsageTestStoredRecord(ids[n], "an-earlier-process", clock.Now())
		record.Batch = []byte(fmt.Sprintf(`{"batch":%d}`, n))
		batches[ids[n]] = record.Batch
		budgetUsageTestStore(t, journal, record)
	}
	ctx := context.Background()
	// pass runs one pass and returns the attempts it sent a report for, in
	// order.
	sent := 0
	pass := func() []string {
		t.Helper()
		d.resendBudgetUsage(ctx)
		reports := api.reports()[sent:]
		sent += len(reports)
		attempts := make([]string, 0, len(reports))
		for _, report := range reports {
			attemptID := strings.TrimSuffix(strings.TrimPrefix(report.Path, "/api/daemon/budget/attempts/"), "/usage")
			if !bytes.Equal(report.Body, batches[attemptID]) {
				t.Errorf("the report for attempt %s carries %s, want the bytes of its record %s", attemptID, report.Body, batches[attemptID])
			}
			attempts = append(attempts, attemptID)
		}
		return attempts
	}
	around := func(from, count int) []string {
		want := make([]string, 0, count)
		for n := 0; n < count; n++ {
			want = append(want, ids[(from+n)%records])
		}
		return want
	}

	// Nothing is delivered: every record stays, and three passes go around
	// the list.
	for n, from := range []int{0, 32, 24} {
		if got, want := pass(), around(from, perPass); !reflect.DeepEqual(got, want) {
			t.Fatalf("pass %d sent reports for\n %v\nwant\n %v", n+1, got, want)
		}
	}
	if got := len(budgetUsageTestNames(t, directory)); got != records {
		t.Fatalf("the journal holds %d records, want all %d", got, records)
	}

	// Now the server takes the batches: 32 go with the next pass, the other
	// eight with the one after.
	delivering.Store(true)
	if got, want := pass(), around(16, perPass); !reflect.DeepEqual(got, want) {
		t.Fatalf("the first delivering pass sent reports for\n %v\nwant\n %v", got, want)
	}
	wantLeft := make([]string, 0, records-perPass)
	for _, attemptID := range around(8, records-perPass) {
		wantLeft = append(wantLeft, attemptID+".json")
	}
	if got := budgetUsageTestNames(t, directory); !reflect.DeepEqual(got, wantLeft) {
		t.Fatalf("after the first delivering pass the journal holds\n %v\nwant\n %v", got, wantLeft)
	}
	if got, want := pass(), around(8, records-perPass); !reflect.DeepEqual(got, want) {
		t.Fatalf("the second delivering pass sent reports for\n %v\nwant\n %v", got, want)
	}
	if got := pass(); len(got) != 0 {
		t.Errorf("a pass over an empty journal sent reports for %v", got)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal still holds %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// budgetUsageTestUntil fails the test when cond does not come true in time.
func budgetUsageTestUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestBudgetUsageLoopPassesAtStartAndThenAtEveryTick: the loop makes its
// first pass at once, which is the recovery after a restart, and another one
// at every tick, until the daemon stops. A daemon without a journal has no
// loop.
func TestBudgetUsageLoopPassesAtStartAndThenAtEveryTick(t *testing.T) {
	// start runs the loop of a process that finds one sealed record, and
	// returns a function that stops the daemon and waits for the loop to end.
	start := func(t *testing.T, api *budgetUsageTestAPI, every time.Duration) (sealed []byte, journal *budgetUsageJournal, logs *budgetUsageTestLog, stop func()) {
		t.Helper()
		directory := filepath.Join(t.TempDir(), "journal")
		clock := budgetUsageTestNewClock()
		sealed = budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
		clock.advance(time.Hour)
		d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
		ctx, cancel := context.WithCancel(context.Background())
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			d.budgetUsageLoop(ctx, every)
		}()
		stop = func() {
			t.Helper()
			cancel()
			budgetUsageTestAwait(t, ended, "the loop ends when the daemon stops")
		}
		t.Cleanup(stop)
		return sealed, journal, logs, stop
	}
	gone := func(journal *budgetUsageJournal) func() bool {
		return func() bool {
			_, err := os.Lstat(journal.recordPath(budgetUsageTestAttemptA))
			return errors.Is(err, os.ErrNotExist)
		}
	}

	t.Run("a pass at start", func(t *testing.T) {
		api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
		// No tick comes while this test runs.
		sealed, journal, logs, stop := start(t, api, time.Hour)
		budgetUsageTestUntil(t, "the record is delivered by the pass at start", gone(journal))
		stop()
		reports := api.reports()
		if len(reports) != 1 || !bytes.Equal(reports[0].Body, sealed) {
			t.Errorf("%d usage reports, want one with the persisted bytes", len(reports))
		}
		if got := budgetUsageTestWarnings(logs.String()); got != 0 {
			t.Errorf("%d warnings:\n%s", got, logs.String())
		}
	})

	t.Run("a pass at every tick", func(t *testing.T) {
		api := budgetUsageTestNewAPI(t, func(nth int, w http.ResponseWriter) {
			if nth <= 2 {
				budgetUsageTestUnavailable()(nth, w)
				return
			}
			budgetUsageTestRecorded()(nth, w)
		})
		sealed, journal, logs, stop := start(t, api, 5*time.Millisecond)
		budgetUsageTestUntil(t, "the record is delivered by the third pass", gone(journal))
		stop()
		reports := api.reports()
		if len(reports) != 3 {
			t.Fatalf("%d usage reports, want 3: one by the pass at start and one by each of two ticks", len(reports))
		}
		for i, report := range reports {
			if !bytes.Equal(report.Body, sealed) {
				t.Errorf("report %d does not carry the persisted bytes: %s", i+1, report.Body)
			}
		}
		if got := budgetUsageTestWarnings(logs.String()); got != 0 {
			t.Errorf("%d warnings:\n%s", got, logs.String())
		}
	})

	t.Run("stops with the daemon", func(t *testing.T) {
		api := budgetUsageTestNewAPI(t, budgetUsageTestUnavailable())
		directory := filepath.Join(t.TempDir(), "journal")
		clock := budgetUsageTestNewClock()
		budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
		clock.advance(time.Hour)
		d, journal, _ := budgetUsageTestRestarted(t, api, directory, clock)
		var started atomic.Int32
		d.client.client.Transport = budgetUsageCountingTransport{inner: d.client.client.Transport, started: &started}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			d.budgetUsageLoop(ctx, 5*time.Millisecond)
		}()
		budgetUsageTestUntil(t, "the loop has made three passes", func() bool { return started.Load() >= 3 })
		cancel()
		budgetUsageTestAwait(t, ended, "the loop ends when the daemon stops")
		// Count at dispatch: a server may finish reading an already cancelled
		// request after the sending loop returns.
		after := started.Load()
		time.Sleep(50 * time.Millisecond)
		if got := started.Load(); got != after {
			t.Errorf("%d usage reports started after the loop ended", got-after)
		}
		if _, err := os.Lstat(journal.recordPath(budgetUsageTestAttemptA)); err != nil {
			t.Errorf("the record that was not delivered: %v", err)
		}
	})

	t.Run("without a journal", func(t *testing.T) {
		logger, _ := budgetUsageTestLogger()
		d := budgetUsageTestNewDaemon(budgetUsageTestNewServer(t), nil, logger)
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			d.budgetUsageLoop(context.Background(), time.Millisecond)
		}()
		budgetUsageTestAwait(t, ended, "the loop of a daemon without a journal returns at once")
		// Neither does a pass do anything, nor a send.
		d.resendBudgetUsage(context.Background())
		d.deliverBudgetUsage(context.Background(), budgetUsageTestAttemptA)
	})
}

// TestBudgetUsagePassDoesNotSendARecordThatIsBeingSent: the send that follows
// the seal and the resend loop share one mark per record. While the first is
// waiting for its answer a pass leaves the record alone, so the server never
// gets the same batch twice at once.
func TestBudgetUsagePassDoesNotSendARecordThatIsBeingSent(t *testing.T) {
	api, hold := budgetUsageTestNewHoldingAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestSendingDaemon(t, api)
	batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
	ctx := context.Background()

	d.finishBudgetUsageAttempt(ctx, budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA))
	budgetUsageTestAwait(t, hold.arrived, "the send that follows the seal reaches the server")

	passed := make(chan struct{})
	go func() {
		defer close(passed)
		d.resendBudgetUsage(ctx)
	}()
	budgetUsageTestAwait(t, passed, "a pass returns while the record is being sent")
	if reports := api.reports(); len(reports) != 1 {
		t.Fatalf("%d usage reports while the first send is under way, want 1", len(reports))
	}
	if _, err := os.Lstat(journal.recordPath(budgetUsageTestAttemptA)); err != nil {
		t.Fatalf("the record that is being sent: %v", err)
	}

	hold.letGo()
	budgetUsageTestWaitForSends(t, journal)
	d.resendBudgetUsage(ctx)
	reports := api.reports()
	if len(reports) != 1 || !bytes.Equal(reports[0].Body, batch) {
		t.Errorf("%d usage reports in all, want the one with the sealed bytes", len(reports))
	}
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 0)
}

// TestBudgetUsageRecoveryDoesNotSendABatchItCouldNotPersist: the bytes of a
// batch that recovery builds must be on disk before they are sent, or a third
// process would build other bytes for the same key. While the journal cannot
// be written the record is not sent and stays as it is; once it can, the
// batch is persisted and sent.
func TestBudgetUsageRecoveryDoesNotSendABatchItCouldNotPersist(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	left := budgetUsageTestLeftRunning(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	fileAsLeft := budgetUsageTestFileText(t, earlier, budgetUsageTestAttemptA)

	clock.advance(time.Hour)
	answer, watch := budgetUsageTestWatching(earlier.recordPath(budgetUsageTestAttemptA), budgetUsageTestRecorded())
	api := budgetUsageTestNewAPI(t, answer)
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()

	// The directory can be read and no file can be created in it.
	budgetUsageTestChmod(t, directory, 0o500)
	for pass := 1; pass <= 2; pass++ {
		d.resendBudgetUsage(ctx)
		if calls := api.seen(); len(calls) != 0 {
			t.Fatalf("pass %d: %d requests reached the server although the batch is not on disk", pass, len(calls))
		}
		if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileAsLeft {
			t.Fatalf("pass %d: the record file changed\n now:  %s\n left: %s", pass, got, fileAsLeft)
		}
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings while the journal cannot be written:\n%s", got, logs.String())
	}

	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	d.resendBudgetUsage(ctx)
	reports := api.reports()
	if len(reports) != 1 || budgetUsageTestBatchKey(t, reports[0].Body) != left.BatchKey {
		t.Fatalf("usage reports once the journal can be written: %d, want one under the key of the record", len(reports))
	}
	seen := watch.contents()
	if len(seen) != 1 {
		t.Fatalf("the record file was looked at %d times, want 1", len(seen))
	}
	if stored := budgetUsageTestRecordAs(t, seen[0], budgetUsageTestAttemptA); !bytes.Equal(stored.Batch, reports[0].Body) {
		t.Errorf("when the report arrived the record file held the batch %s, want the bytes that were sent", stored.Batch)
	}
	budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 0)
}

// TestBudgetUsageKeptBatchOfThisProcessIsResentFromMemory: a batch that the
// send after the seal did not deliver is resent by the loop as this process
// holds it, with the same bytes. The process is the truth for its own
// records: the batch goes out too when its file is not there, and when the
// journal directory cannot be listed. Once delivered, the record is gone from
// disk and from the process.
func TestBudgetUsageKeptBatchOfThisProcessIsResentFromMemory(t *testing.T) {
	for _, tc := range []struct {
		name string
		// spoil changes the journal directory after the first send.
		spoil func(t *testing.T, journal *budgetUsageJournal)
	}{
		{name: "with its record file", spoil: func(*testing.T, *budgetUsageJournal) {}},
		{name: "without a record file", spoil: func(t *testing.T, journal *budgetUsageJournal) {
			if err := os.Remove(journal.recordPath(budgetUsageTestAttemptA)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "when the journal cannot be listed", spoil: func(t *testing.T, journal *budgetUsageJournal) {
			budgetUsageTestRequirePOSIX(t)
			// Files can be created and removed, and no name can be read.
			budgetUsageTestChmod(t, journal.directory, 0o300)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := budgetUsageTestNewAPI(t, budgetUsageTestThen(budgetUsageTestUnavailable(), budgetUsageTestRecorded()))
			d, journal, logs := budgetUsageTestSendingDaemon(t, api)
			batch := budgetUsageTestSealed(t, journal, d.logger, budgetUsageTestSeed(budgetUsageTestAttemptA))
			ctx := context.Background()
			d.deliverBudgetUsage(ctx, budgetUsageTestAttemptA)
			if got := len(api.reports()); got != 1 {
				t.Fatalf("%d usage reports after the send that follows the seal, want 1", got)
			}
			tc.spoil(t, journal)

			d.resendBudgetUsage(ctx)
			reports := api.reports()
			if len(reports) != 2 {
				t.Fatalf("%d usage reports after a pass, want 2", len(reports))
			}
			for i, report := range reports {
				if report.Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) || !bytes.Equal(report.Body, batch) {
					t.Errorf("report %d is %s with the body %s, want the sealed bytes", i+1, report.Path, report.Body)
				}
			}
			if err := os.Chmod(journal.directory, 0o700); err != nil {
				t.Fatal(err)
			}
			budgetUsageTestRequireDropped(t, journal, logs, budgetUsageTestAttemptA, 0)
			d.resendBudgetUsage(ctx)
			if got := len(api.reports()); got != 2 {
				t.Errorf("%d usage reports in all, want 2", got)
			}
		})
	}
}

// TestBudgetUsageRestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace:
// after a restart a managed installation sends each record it finds with the
// credential of the workspace the record names. A record whose workspace has
// no binding stays as it is until there is one.
func TestBudgetUsageRestartInAManagedInstallationSendsWithTheCredentialOfTheRecordsWorkspace(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	seedBound := budgetUsageTestSeed(budgetUsageTestAttemptA)
	seedBound.WorkspaceID = budgetUsageTestWorkspaceA
	seedUnbound := budgetUsageTestSeed(budgetUsageTestAttemptB)
	seedUnbound.WorkspaceID = budgetUsageTestWorkspaceC
	batchBound := budgetUsageTestLeftSealed(t, earlier, seedBound)
	batchUnbound := budgetUsageTestLeftSealed(t, earlier, seedUnbound)
	unboundFile := budgetUsageTestFileText(t, earlier, budgetUsageTestAttemptB)

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	d.client.enableManagedTransport()
	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceA, "mdt_workspace_a")
	ctx := context.Background()

	d.resendBudgetUsage(ctx)
	reports := api.reports()
	if len(reports) != 1 || reports[0].Path != budgetUsageTestReportPath(budgetUsageTestAttemptA) || !bytes.Equal(reports[0].Body, batchBound) {
		t.Fatalf("usage reports after the first pass: %d, want one for the record whose workspace is bound", len(reports))
	}
	if reports[0].Authorization != "Bearer mdt_workspace_a" {
		t.Errorf("Authorization = %q, want the credential of the record's workspace", reports[0].Authorization)
	}
	if got := budgetUsageTestCapabilityCount(reports[0].Capabilities, protocol.DaemonCapabilityManagedInstallationV1); got != 1 {
		t.Errorf("the report was not sent by a workspace client: X-Client-Capabilities = %q", reports[0].Capabilities)
	}
	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptB); got != unboundFile {
		t.Errorf("the record whose workspace has no binding changed")
	}

	budgetUsageTestInstallBinding(t, d.client, budgetUsageTestWorkspaceC, "mdt_workspace_c")
	d.resendBudgetUsage(ctx)
	reports = api.reports()
	if len(reports) != 2 || reports[1].Path != budgetUsageTestReportPath(budgetUsageTestAttemptB) || !bytes.Equal(reports[1].Body, batchUnbound) {
		t.Fatalf("usage reports after the workspace was bound: %d, want a second one for its record", len(reports))
	}
	if reports[1].Authorization != "Bearer mdt_workspace_c" {
		t.Errorf("Authorization = %q, want the credential of the record's workspace", reports[1].Authorization)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal still holds %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsagePassAfterTheDaemonStoppedHandlesNothing: a pass that runs
// when the daemon was told to stop leaves every record as it is. It builds
// nothing that it could not send.
func TestBudgetUsagePassAfterTheDaemonStoppedHandlesNothing(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	left := budgetUsageTestLeftRunning(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	fileAsLeft := budgetUsageTestFileText(t, earlier, budgetUsageTestAttemptA)

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	d.resendBudgetUsage(stopped)
	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileAsLeft {
		t.Errorf("the record file changed\n now:  %s\n left: %s", got, fileAsLeft)
	}
	if calls := api.seen(); len(calls) != 0 {
		t.Errorf("%d requests reached the server", len(calls))
	}

	// The next process, or the next pass of one that goes on, handles it.
	d.resendBudgetUsage(context.Background())
	reports := api.reports()
	if len(reports) != 1 || budgetUsageTestBatchKey(t, reports[0].Body) != left.BatchKey {
		t.Fatalf("usage reports of a pass of a running daemon: %d, want one under the key of the record", len(reports))
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
}

// TestBudgetUsagePassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved:
// the answer of the server stands when the pass that got it cannot delete the
// record file. The file then says what it said before the answer, and the
// passes that follow find it: they do not send its batch again and do not
// repeat the warning, and they remove the file once the directory lets them.
func TestBudgetUsagePassLeavesAnAnsweredRecordAloneWhenItsFileCannotBeRemoved(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	for _, tc := range []struct {
		name   string
		answer budgetUsageTestAnswer
		// warnings is what the answer and the failed removal log together.
		warnings int
	}{
		{"delivered", budgetUsageTestRecorded(), 1},
		{"refused for good", budgetUsageTestRefusing(http.StatusBadRequest, "invalid_request"), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "journal")
			clock := budgetUsageTestNewClock()
			sealed := budgetUsageTestLeftSealed(t, budgetUsageTestProcess(t, directory, clock), budgetUsageTestSeed(budgetUsageTestAttemptA))
			clock.advance(time.Hour)
			api := budgetUsageTestNewAPI(t, tc.answer)
			d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
			fileAsLeft := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			ctx := context.Background()
			// Files of this directory can be read and cannot be removed.
			budgetUsageTestChmod(t, directory, 0o500)

			for pass := 1; pass <= 3; pass++ {
				d.resendBudgetUsage(ctx)
				reports := api.reports()
				if len(reports) != 1 || !bytes.Equal(reports[0].Body, sealed) {
					t.Fatalf("pass %d: %d usage reports, want the persisted batch once", pass, len(reports))
				}
				if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != fileAsLeft {
					t.Fatalf("pass %d: the test wants a record file that could not be removed and is as it was; it holds %s", pass, got)
				}
				if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
					t.Fatalf("pass %d: %d warnings, want %d:\n%s", pass, got, tc.warnings, logs.String())
				}
			}

			// The same holds when the file has outlived the cap in the meantime:
			// its record is done with, and nothing more is said about it.
			clock.advance(budgetUsageRetention + time.Hour)
			d.resendBudgetUsage(ctx)
			if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
				t.Errorf("%d warnings after the cap, want still %d:\n%s", got, tc.warnings, logs.String())
			}

			// The directory lets go.
			if err := os.Chmod(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			d.resendBudgetUsage(ctx)
			if names := budgetUsageTestNames(t, directory); len(names) != 0 {
				t.Errorf("the journal still holds %v", names)
			}
			if got := len(api.reports()); got != 1 {
				t.Errorf("%d usage reports in all, want 1", got)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != tc.warnings {
				t.Errorf("%d warnings at the end, want still %d:\n%s", got, tc.warnings, logs.String())
			}
			budgetUsageTestRequireNoBodyLogged(t, logs.String(), sealed)
		})
	}
}

// TestBudgetUsageSendAfterTheSealHandlesOnlyWhatThisProcessHolds: recovering
// what an earlier process left is the work of the resend loop. The send that
// follows the seal acts on the record its own process holds and on nothing
// else.
func TestBudgetUsageSendAfterTheSealHandlesOnlyWhatThisProcessHolds(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	clock := budgetUsageTestNewClock()
	earlier := budgetUsageTestProcess(t, directory, clock)
	sealed := budgetUsageTestLeftSealed(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptA))
	left := budgetUsageTestLeftRunning(t, earlier, budgetUsageTestSeed(budgetUsageTestAttemptB))
	// A record that names no launch, which a pass removes with a warning.
	const attemptNothing = "0195f3a0-0000-7000-8000-00000000000c"
	budgetUsageTestBegun(t, earlier, budgetUsageTestSeed(attemptNothing))
	filesAsLeft := map[string]string{}
	for _, attemptID := range []string{budgetUsageTestAttemptA, budgetUsageTestAttemptB, attemptNothing} {
		filesAsLeft[attemptID] = budgetUsageTestFileText(t, earlier, attemptID)
	}

	clock.advance(time.Hour)
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, directory, clock)
	ctx := context.Background()
	for attemptID, asLeft := range filesAsLeft {
		d.deliverBudgetUsage(ctx, attemptID)
		if got := budgetUsageTestFileText(t, journal, attemptID); got != asLeft {
			t.Errorf("the record file of attempt %s changed\n now:  %s\n left: %s", attemptID, got, asLeft)
		}
	}
	if calls := api.seen(); len(calls) != 0 {
		t.Fatalf("%d requests reached the server", len(calls))
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Fatalf("%d warnings:\n%s", got, logs.String())
	}

	// The loop does handle them.
	d.resendBudgetUsage(ctx)
	if got := budgetUsageTestReportsFor(api, budgetUsageTestAttemptA); len(got) != 1 || !bytes.Equal(got[0].Body, sealed) {
		t.Errorf("%d usage reports for the sealed record after a pass, want one with the persisted bytes", len(got))
	}
	if got := budgetUsageTestReportsFor(api, budgetUsageTestAttemptB); len(got) != 1 || budgetUsageTestBatchKey(t, got[0].Body) != left.BatchKey {
		t.Errorf("%d usage reports for the record without batch bytes after a pass, want one under its key", len(got))
	}
	if got := len(api.reports()); got != 2 {
		t.Errorf("%d usage reports in all, want 2", got)
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("the journal still holds %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("%d warnings after the pass, want 1 for the record that names no launch:\n%s", got, logs.String())
	}
}

// TestBudgetUsagePassOverARecordThatIsGoneDoesNothing: a record file can go
// between the moment a pass lists the journal and the moment it gets to the
// record. The pass then has nothing to do for that attempt.
func TestBudgetUsagePassOverARecordThatIsGoneDoesNothing(t *testing.T) {
	api := budgetUsageTestNewAPI(t, budgetUsageTestRecorded())
	d, journal, logs := budgetUsageTestRestarted(t, api, filepath.Join(t.TempDir(), "journal"), budgetUsageTestNewClock())

	d.handleBudgetUsage(context.Background(), budgetUsageTestAttemptA, true)

	if calls := api.seen(); len(calls) != 0 {
		t.Errorf("%d requests reached the server", len(calls))
	}
	if names := budgetUsageTestNames(t, journal.directory); len(names) != 0 {
		t.Errorf("the journal holds %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 0 {
		t.Errorf("%d warnings:\n%s", got, logs.String())
	}
	// The attempt is as unknown to the process as before: it can get a record.
	if _, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), d.logger); err != nil {
		t.Errorf("the attempt cannot get a record after the pass: %v", err)
	}
}

// budgetUsageCountingTransport observes dispatch, not delayed server reads.
type budgetUsageCountingTransport struct {
	inner   http.RoundTripper
	started *atomic.Int32
}

func (c budgetUsageCountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.started.Add(1)
	return c.inner.RoundTrip(r)
}
