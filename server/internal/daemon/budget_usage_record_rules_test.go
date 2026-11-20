package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

// Rules for the record that design 11.10 gained on 2026-11-19: the adapter
// version never costs the batch, and a record file is never behind its process
// in launches.

// TestBudgetUsageAdapterVersionNeverCostsTheBatch: adapter_version is a note
// taken from what an executable prints for --version, and a custom profile
// command can print anything. The server refuses a whole batch whose
// adapter_version it does not accept, so a version the server would refuse is
// not recorded: the record on disk and the batch then leave the field out.
func TestBudgetUsageAdapterVersionNeverCostsTheBatch(t *testing.T) {
	const twoByteCharacter = "\xc3\xa9"
	cases := []struct {
		name    string
		version string
		// want is the version the record and the batch carry. Empty means
		// that the field is left out.
		want string
	}{
		{"a plain version", "2.1.34", "2.1.34"},
		{"white space around the version", " \t\r\n2.1.34 (Claude Code)\n ", "2.1.34 (Claude Code)"},
		{"exactly 120 bytes", strings.Repeat("v", 120), strings.Repeat("v", 120)},
		{"121 bytes", strings.Repeat("v", 121), ""},
		{"120 bytes once the white space around them is gone", "  " + strings.Repeat("v", 120) + "\n", strings.Repeat("v", 120)},
		{"exactly 120 bytes in 60 characters", strings.Repeat(twoByteCharacter, 60), strings.Repeat(twoByteCharacter, 60)},
		{"122 bytes in 61 characters", strings.Repeat(twoByteCharacter, 61), ""},
		{"a line break inside", "2.1.34\nbuilt for arm64", ""},
		{"a tab inside", "2.1.34\tbeta", ""},
		{"a terminal escape sequence inside", "\x1b[1m2.1.34\x1b[0m", ""},
		{"a delete character inside", "2.1.34\x7fbeta", ""},
		{"a control character beyond ASCII inside", "2.1.34\xc2\x85beta", ""},
		{"not valid UTF-8", "2.1.34\xff", ""},
		{"empty", "", ""},
		{"white space only", " \t\r\n ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "journal"))
			logger, _ := budgetUsageTestLogger()
			attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
			if err != nil {
				t.Fatalf("create record: %v", err)
			}

			attempt.describeLaunch("claude", false, tc.version)

			if got := attempt.snapshot().AdapterVersion; got != tc.want {
				t.Errorf("the record in memory holds the version %q, want %q", got, tc.want)
			}
			// The record on disk and the batch agree.
			stored := budgetUsageTestRequireStored(t, journal, attempt, "after the launch description")
			if stored.AdapterVersion != tc.want {
				t.Errorf("the record on disk holds the version %q, want %q", stored.AdapterVersion, tc.want)
			}
			if text := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); tc.want == "" && strings.Contains(text, "adapter_version") {
				t.Errorf("the record file names a version that is not recorded: %s", text)
			}

			batch := attempt.seal()
			if len(batch) == 0 {
				t.Fatal("the attempt has no batch")
			}
			if !utf8.Valid(batch) {
				t.Errorf("the batch is not valid UTF-8: %q", batch)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(batch, &envelope); err != nil {
				t.Fatalf("decode the batch %q: %v", batch, err)
			}
			raw, present := envelope["adapter_version"]
			if tc.want == "" {
				// The field is absent. Its bytes are not compared: the two
				// toolchains write a byte that is not UTF-8 differently.
				if present {
					t.Errorf("the batch carries adapter_version %s, want the field left out", raw)
				}
				return
			}
			var got string
			if !present {
				t.Fatalf("the batch carries no adapter_version, want %q: %s", tc.want, batch)
			}
			if err := json.Unmarshal(raw, &got); err != nil || got != tc.want {
				t.Errorf("the batch carries adapter_version %s (%v), want %q", raw, err, tc.want)
			}
		})
	}
}

// budgetUsageTestRequirePOSIX skips a test that makes the journal directory
// unusable through its permission bits.
func budgetUsageTestRequirePOSIX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permission bits and a non-root user")
	}
}

// budgetUsageTestChmod gives the journal directory a mode. The directory is
// usable again when the test ends, so that it can be removed.
func budgetUsageTestChmod(t *testing.T, directory string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(directory, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
}

// budgetUsageTestWarnings counts the warnings in a log.
func budgetUsageTestWarnings(logged string) int {
	return strings.Count(logged, "level=WARN")
}

// budgetUsageTestBatchInvocation is one invocation of a batch as a reader of
// the wire format sees it.
type budgetUsageTestBatchInvocation struct {
	InvocationID string            `json:"invocation_id"`
	Resumed      bool              `json:"resumed"`
	Ended        string            `json:"ended"`
	ObservedAt   string            `json:"observed_at"`
	Usage        []json.RawMessage `json:"usage"`
}

// budgetUsageTestBatchInvocations decodes the invocations of a batch.
func budgetUsageTestBatchInvocations(t *testing.T, batch []byte) []budgetUsageTestBatchInvocation {
	t.Helper()
	var decoded struct {
		Invocations []budgetUsageTestBatchInvocation `json:"invocations"`
	}
	if err := json.Unmarshal(batch, &decoded); err != nil {
		t.Fatalf("decode the batch %q: %v", batch, err)
	}
	return decoded.Invocations
}

// TestBudgetUsageLaunchIntentWriteFailureRemovesTheRecordFile: a record file
// must never be behind its process in launches. When the intent to launch
// cannot be written, the file would say less than what was started, so it is
// removed: a later process then finds no record instead of one that lacks a
// launch. The run goes on, and the next write of the whole record that
// succeeds brings the file back complete.
func TestBudgetUsageLaunchIntentWriteFailureRemovesTheRecordFile(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, logs := budgetUsageTestLogger()
	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	attempt.describeLaunch("claude", false, "2.1.34")
	budgetUsageTestRequireStored(t, journal, attempt, "before the launch")

	// Files in the directory can be created, replaced and removed, and a write
	// still fails: the directory cannot be opened to be synced.
	budgetUsageTestChmod(t, directory, 0o300)
	invocationID := attempt.beginInvocation(true)
	budgetUsageTestChmod(t, directory, 0o700)

	if invocationID == "" {
		t.Fatal("the invocation got no id: the run must go on when its launch intent cannot be written")
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Fatalf("the record file is still there after a launch intent that could not be written: %v", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("a launch intent that could not be written logged %d warnings, want 1:\n%s", got, logs.String())
	}
	if strings.Contains(logs.String(), "remove_error") {
		t.Errorf("the log reports a failed removal of a file that was removed:\n%s", logs.String())
	}
	// Only the file is gone. The attempt stays live and holds the launch.
	if live := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA); live != attempt {
		t.Errorf("the journal's process holds %p for the attempt, want the live attempt %p", live, attempt)
	}
	memory := attempt.snapshot()
	if len(memory.Invocations) != 1 || memory.Invocations[0].InvocationID != invocationID || !memory.Invocations[0].Resumed {
		t.Fatalf("the record in memory does not hold the launch: %+v", memory.Invocations)
	}

	// The outcome is the next write of the whole record. It succeeds, and the
	// file is back with the launch intent and the outcome.
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	stored := budgetUsageTestRequireStored(t, journal, attempt, "after the outcome")
	if len(stored.Invocations) != 1 {
		t.Fatalf("the restored file holds %d invocations, want 1: %+v", len(stored.Invocations), stored.Invocations)
	}
	restored := stored.Invocations[0]
	if restored.InvocationID != invocationID || !restored.Resumed || restored.LaunchedAt.IsZero() ||
		restored.Ended != budgetUsageEndedReturned || !reflect.DeepEqual(restored.Usage, budgetUsageTestCounts()) {
		t.Errorf("the restored file does not hold the launch intent and the outcome: %+v", restored)
	}
	if stored.AdapterVersion != "2.1.34" || stored.BatchKey != memory.BatchKey {
		t.Errorf("the restored file lost what the record held before the launch: %+v", stored)
	}
	if names := budgetUsageTestNames(t, directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
		t.Errorf("journal holds %v, want the one record file", names)
	}
	if got := budgetUsageTestWarnings(logs.String()); got != 1 {
		t.Errorf("%d warnings after the outcome was written, want still 1:\n%s", got, logs.String())
	}
}

// TestBudgetUsageLaunchIntentWriteFailureWhenTheRecordFileCannotBeRemoved: the
// intent to launch cannot be written and the file that is now behind cannot be
// removed either. That is one warning, the run goes on, and the batch the live
// process builds holds the invocation. The file that stays says nothing about
// the launch; recovery never reports such a record.
func TestBudgetUsageLaunchIntentWriteFailureWhenTheRecordFileCannotBeRemoved(t *testing.T) {
	budgetUsageTestRequirePOSIX(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, logs := budgetUsageTestLogger()
	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	before := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)

	// Nothing in the directory can be created or removed any more.
	budgetUsageTestChmod(t, directory, 0o500)
	invocationID := attempt.beginInvocation(false)
	if invocationID == "" {
		t.Fatal("the invocation got no id: the run must go on when its launch intent cannot be written")
	}
	logged := logs.String()
	if got := budgetUsageTestWarnings(logged); got != 1 {
		t.Errorf("a launch intent that could not be written, with a file that could not be removed, logged %d warnings, want 1:\n%s", got, logged)
	}
	if !strings.Contains(logged, "remove_error") {
		t.Errorf("the warning does not say that the record file could not be removed:\n%s", logged)
	}

	// The run goes on.
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
	batch := attempt.seal()
	invocations := budgetUsageTestBatchInvocations(t, batch)
	if len(invocations) != 1 || invocations[0].InvocationID != invocationID || invocations[0].Ended != budgetUsageEndedReturned || len(invocations[0].Usage) != 1 {
		t.Fatalf("the batch of the live process does not hold the invocation: %s", batch)
	}
	if live := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA); live != attempt {
		t.Errorf("the journal's process holds %p for the attempt, want the live attempt %p", live, attempt)
	}

	budgetUsageTestChmod(t, directory, 0o700)
	if after := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); after != before {
		t.Errorf("the record file changed in a directory that could not be written\n before: %s\n after:  %s", before, after)
	}
	stale, err := journal.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatalf("read the file that stayed behind: %v", err)
	}
	if len(stale.Invocations) != 0 || len(stale.Batch) != 0 {
		t.Errorf("the file that stayed behind holds %d invocations and %d batch bytes, want neither", len(stale.Invocations), len(stale.Batch))
	}
}

// TestBudgetUsageOutcomeAndSealWriteFailuresLeaveTheRecordFile: a file that
// lacks an outcome or the batch bytes still holds every launch intent, and what
// it lacks is reported as unknown by the next process. Such a file is not
// removed, also when it could be.
func TestBudgetUsageOutcomeAndSealWriteFailuresLeaveTheRecordFile(t *testing.T) {
	cases := []struct {
		name string
		mode os.FileMode
		// replaced says whether the failing write still replaces the file.
		replaced bool
	}{
		// Nothing in the directory can be created: the file keeps what it held.
		{"the write fails before the file is replaced", 0o500, false},
		// The file is replaced, and could be removed; the write fails when the
		// directory is opened to be synced.
		{"the write fails after the file is replaced", 0o300, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budgetUsageTestRequirePOSIX(t)
			directory := filepath.Join(t.TempDir(), "journal")
			journal := budgetUsageTestOpenJournal(t, directory)
			logger, logs := budgetUsageTestLogger()
			attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
			if err != nil {
				t.Fatalf("create record: %v", err)
			}
			invocationID := attempt.beginInvocation(false)
			withIntent := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			if !strings.Contains(withIntent, invocationID) {
				t.Fatalf("the record file does not hold the launch intent: %s", withIntent)
			}

			budgetUsageTestChmod(t, directory, tc.mode)

			attempt.endInvocation(invocationID, budgetUsageEndedReturned, budgetUsageTestUsage())
			afterOutcome := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			if !strings.Contains(afterOutcome, invocationID) {
				t.Errorf("after the failed write of the outcome the record file does not hold the launch intent: %s", afterOutcome)
			}
			if !tc.replaced && afterOutcome != withIntent {
				t.Errorf("the record file changed in a directory that could not be written: %s", afterOutcome)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 1 {
				t.Errorf("a failed write of the outcome logged %d warnings, want 1:\n%s", got, logs.String())
			}

			batch := attempt.seal()
			if len(budgetUsageTestBatchInvocations(t, batch)) != 1 {
				t.Fatalf("the batch does not hold the invocation: %s", batch)
			}
			afterSeal := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA)
			if !strings.Contains(afterSeal, invocationID) {
				t.Errorf("after the failed write of the batch bytes the record file does not hold the launch intent: %s", afterSeal)
			}
			if !tc.replaced && afterSeal != withIntent {
				t.Errorf("the record file changed in a directory that could not be written: %s", afterSeal)
			}
			if got := budgetUsageTestWarnings(logs.String()); got != 2 {
				t.Errorf("failed writes of the outcome and of the batch bytes logged %d warnings, want 2:\n%s", got, logs.String())
			}
			if strings.Contains(logs.String(), "remove_error") {
				t.Errorf("the log reports a removal; these writes never remove the file:\n%s", logs.String())
			}

			budgetUsageTestChmod(t, directory, 0o700)
			if names := budgetUsageTestNames(t, directory); !reflect.DeepEqual(names, []string{budgetUsageTestAttemptA + ".json"}) {
				t.Errorf("journal holds %v, want the one record file", names)
			}
			if live := budgetUsageTestLiveAttempt(journal, budgetUsageTestAttemptA); live != attempt {
				t.Errorf("the journal's process holds %p for the attempt, want the live attempt %p", live, attempt)
			}
		})
	}
}
