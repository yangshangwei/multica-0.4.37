package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Stage A: the names both halves of the budget usage protocol v1 agree on.

func TestBudgetUsageCapabilityNameIsFrozen(t *testing.T) {
	// The server half adds the same constant in its own tree; the two meet
	// only at this literal.
	if got, want := protocol.DaemonCapabilityBudgetUsageV1, "budget-usage-v1"; got != want {
		t.Fatalf("DaemonCapabilityBudgetUsageV1 = %q, want %q", got, want)
	}
}

func TestBudgetUsageTaskDecodesAttemptID(t *testing.T) {
	const attemptID = "0195f3a0-0000-7000-8000-000000000001"
	var task Task
	raw := []byte(`{"id":"task-1","runtime_id":"rt-1","budget_attempt_id":"` + attemptID + `"}`)
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatalf("decode claim task: %v", err)
	}
	if task.BudgetAttemptID != attemptID {
		t.Fatalf("BudgetAttemptID = %q, want %q", task.BudgetAttemptID, attemptID)
	}

	var without Task
	if err := json.Unmarshal([]byte(`{"id":"task-2","runtime_id":"rt-1"}`), &without); err != nil {
		t.Fatalf("decode claim task without an attempt: %v", err)
	}
	if without.BudgetAttemptID != "" {
		t.Fatalf("BudgetAttemptID = %q for a claim without one, want empty", without.BudgetAttemptID)
	}
	encoded, err := json.Marshal(without)
	if err != nil {
		t.Fatalf("encode task: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode encoded task: %v", err)
	}
	if _, present := fields["budget_attempt_id"]; present {
		t.Fatalf("an empty attempt id must be omitted from the encoded task: %s", encoded)
	}
}

func TestBudgetUsageNormalizerVersionIsOne(t *testing.T) {
	// Every provider starts at "1" in the first slice. The empty and the
	// unknown key are included because the daemon asks with whatever
	// provider key the runtime carries.
	for _, provider := range []string{
		"claude", "codex", "copilot", "opencode", "openclaw", "hermes", "gemini",
		"pi", "cursor", "kimi", "kiro", "omp", "grok", "", "not-a-provider",
	} {
		if got := agent.UsageNormalizerVersion(provider); got != "1" {
			t.Errorf("UsageNormalizerVersion(%q) = %q, want %q", provider, got, "1")
		}
	}
}

// Stage B: the journal, the record and the batch builder.

// budgetUsageTestLog collects what the code under test logs.
type budgetUsageTestLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *budgetUsageTestLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *budgetUsageTestLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func budgetUsageTestLogger() (*slog.Logger, *budgetUsageTestLog) {
	sink := &budgetUsageTestLog{}
	return slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})), sink
}

const (
	budgetUsageTestServerURL = "https://multica.example.test"
	budgetUsageTestAttemptA  = "0195f3a0-0000-7000-8000-00000000000a"
	budgetUsageTestAttemptB  = "0195f3a0-0000-7000-8000-00000000000b"
)

func budgetUsageTestOpenJournal(t *testing.T, directory string) *budgetUsageJournal {
	t.Helper()
	journal, err := openBudgetUsageJournal(directory, budgetUsageTestServerURL)
	if err != nil {
		t.Fatalf("open budget usage journal: %v", err)
	}
	// A send that handleTask started in the background is over before the
	// directory of the journal goes away.
	t.Cleanup(journal.background.Wait)
	return journal
}

func budgetUsageTestSeed(attemptID string) budgetUsageSeed {
	return budgetUsageSeed{
		AttemptID:         attemptID,
		WorkspaceID:       "ws-1",
		TaskID:            "task-1",
		Adapter:           "claude",
		NormalizerVersion: "1",
	}
}

// budgetUsageTestNames lists the journal directory, sorted.
func budgetUsageTestNames(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("list journal directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// budgetUsageTestRecordJSON renders a record for comparison. Two records are
// the same record exactly when this text is the same.
func budgetUsageTestRecordJSON(t *testing.T, record budgetUsageRecord) string {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("encode record: %v", err)
	}
	return string(raw)
}

// budgetUsageTestRequireStored fails unless the record on disk is the one the
// attempt holds in memory: a process that reads the file back holds exactly
// what the process that wrote it holds, value for value.
func budgetUsageTestRequireStored(t *testing.T, journal *budgetUsageJournal, attempt *budgetUsageAttempt, step string) budgetUsageRecord {
	t.Helper()
	memory := attempt.snapshot()
	stored, err := journal.read(memory.AttemptID)
	if err != nil {
		t.Fatalf("%s: read record back: %v", step, err)
	}
	if !reflect.DeepEqual(stored, memory) {
		t.Fatalf("%s: record on disk differs from memory\n disk:   %#v\n memory: %#v", step, stored, memory)
	}
	return stored
}

// budgetUsageTestFileText returns the text of an attempt's record file.
func budgetUsageTestFileText(t *testing.T, journal *budgetUsageJournal, attemptID string) string {
	t.Helper()
	raw, err := os.ReadFile(journal.recordPath(attemptID))
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	return string(raw)
}

func TestBudgetUsageJournalOpen(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "profile", "budget-usage-reports")
	journal, err := openBudgetUsageJournal(directory, budgetUsageTestServerURL)
	if err != nil {
		t.Fatalf("open journal in a directory that does not exist yet: %v", err)
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		t.Fatalf("journal directory was not created: info=%v err=%v", info, err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("journal directory mode = %o, want 700", perm)
		}
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("opening the journal left files behind: %v", names)
	}
	if !managedUUID.MatchString(journal.epoch) {
		t.Errorf("journal epoch %q is not a canonical UUID", journal.epoch)
	}
	if journal.serverURL != budgetUsageTestServerURL {
		t.Errorf("journal server URL = %q, want %q", journal.serverURL, budgetUsageTestServerURL)
	}

	// A later process opens the same directory under its own start identity.
	later, err := openBudgetUsageJournal(directory, budgetUsageTestServerURL)
	if err != nil {
		t.Fatalf("open an existing journal directory: %v", err)
	}
	if later.epoch == journal.epoch {
		t.Errorf("two journal opens share the epoch %q; every process start needs its own", journal.epoch)
	}
}

func TestBudgetUsageJournalOpenFailsOnUnusableDirectory(t *testing.T) {
	// requireNoJournal fails unless the open was refused, and the error names
	// the step that could not be done.
	requireNoJournal := func(t *testing.T, journal *budgetUsageJournal, err error, wantStep string) {
		t.Helper()
		if err == nil || journal != nil {
			t.Fatalf("journal=%v err=%v, want no journal and an error", journal, err)
		}
		if !strings.HasPrefix(err.Error(), wantStep+": ") {
			t.Errorf("open failed with %q, want it to name the step %q", err, wantStep)
		}
	}
	t.Run("the path is a regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "budget-usage-reports")
		if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		journal, err := openBudgetUsageJournal(path, budgetUsageTestServerURL)
		requireNoJournal(t, journal, err, "create budget usage journal directory")
		if raw, err := os.ReadFile(path); err != nil || string(raw) != "not a directory" {
			t.Errorf("the file at the journal path was changed: %q, %v", raw, err)
		}
	})
	t.Run("a parent is a regular file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "profile")
		if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		journal, err := openBudgetUsageJournal(filepath.Join(parent, "budget-usage-reports"), budgetUsageTestServerURL)
		requireNoJournal(t, journal, err, "create budget usage journal directory")
	})
	t.Run("the directory cannot be written", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("needs POSIX permission bits and a non-root user")
		}
		directory := filepath.Join(t.TempDir(), "budget-usage-reports")
		if err := os.Mkdir(directory, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
		journal, err := openBudgetUsageJournal(directory, budgetUsageTestServerURL)
		requireNoJournal(t, journal, err, "write budget usage journal probe")
	})
	t.Run("the directory cannot be listed", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("needs POSIX permission bits and a non-root user")
		}
		directory := filepath.Join(t.TempDir(), "budget-usage-reports")
		if err := os.Mkdir(directory, 0o300); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
		journal, err := openBudgetUsageJournal(directory, budgetUsageTestServerURL)
		// The probe can be created and renamed here. Its write still fails,
		// because the atomic write opens the directory to sync it, which needs
		// the same permission as listing it.
		requireNoJournal(t, journal, err, "write budget usage journal probe")
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if names := budgetUsageTestNames(t, directory); len(names) != 0 {
			t.Errorf("a failed open left files behind: %v", names)
		}
	})
}

func TestBudgetUsageRecordWriteAndReadBack(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	journal := budgetUsageTestOpenJournal(t, directory)
	// The daemon's clock is in a local zone. The record keeps its times in UTC.
	clock := time.Date(2026, 11, 17, 15, 59, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	journal.now = func() time.Time { return clock }
	logger, logs := budgetUsageTestLogger()

	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if got, want := budgetUsageTestNames(t, directory), []string{budgetUsageTestAttemptA + ".json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("journal directory holds %v, want %v", got, want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(directory, budgetUsageTestAttemptA+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("record file mode = %o, want 600", perm)
		}
	}

	created := budgetUsageTestRequireStored(t, journal, attempt, "after create")
	if created.Version != budgetUsageRecordVersion {
		t.Errorf("record version = %d, want %d", created.Version, budgetUsageRecordVersion)
	}
	if created.AttemptID != budgetUsageTestAttemptA {
		t.Errorf("record attempt id = %q", created.AttemptID)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(created.BatchKey) {
		t.Errorf("batch key %q does not match the frozen pattern", created.BatchKey)
	}
	if created.ProducerEpoch != journal.epoch {
		t.Errorf("record epoch = %q, want the creating process's %q", created.ProducerEpoch, journal.epoch)
	}
	if !created.CreatedAt.Equal(clock) {
		t.Errorf("record created at %s, want %s", created.CreatedAt, clock)
	}
	if created.ServerURL != budgetUsageTestServerURL || created.WorkspaceID != "ws-1" || created.TaskID != "task-1" {
		t.Errorf("record scope = server %q workspace %q task %q", created.ServerURL, created.WorkspaceID, created.TaskID)
	}
	if created.Adapter != "claude" || created.AdapterVersion != "" || created.NormalizerVersion != "1" {
		t.Errorf("record adapter = %q version %q normalizer %q", created.Adapter, created.AdapterVersion, created.NormalizerVersion)
	}
	if len(created.Invocations) != 0 || len(created.Batch) != 0 {
		t.Errorf("a new record has %d invocations and %d batch bytes, want none", len(created.Invocations), len(created.Batch))
	}

	// The on-disk form is private to the daemon, but it outlives the process:
	// what one daemon wrote, the next one reads. A change to the text pinned
	// here and at the end of this test needs a new budgetUsageRecordVersion.
	fileHead := `{"version":1,"attempt_id":"` + budgetUsageTestAttemptA + `","batch_key":"` + created.BatchKey +
		`","producer_epoch":"` + journal.epoch + `","created_at":"2026-11-17T07:59:00Z","server_url":"` + budgetUsageTestServerURL +
		`","workspace_id":"ws-1","task_id":"task-1",`
	if got, want := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA), fileHead+`"adapter":"claude","normalizer_version":"1","invocations":[]}`+"\n"; got != want {
		t.Fatalf("record file after create\n got:  %s want: %s", got, want)
	}

	// Every change rewrites the whole record, and none of them touches the
	// batch key the record was created with.
	requireStored := func(step string) budgetUsageRecord {
		t.Helper()
		stored := budgetUsageTestRequireStored(t, journal, attempt, step)
		if stored.BatchKey != created.BatchKey {
			t.Fatalf("%s: batch key = %q, want the one the record was created with %q", step, stored.BatchKey, created.BatchKey)
		}
		return stored
	}
	attempt.describeLaunch("claude", true, "2.1.34")
	described := requireStored("after describeLaunch")
	if described.Adapter != "custom-claude" || described.AdapterVersion != "2.1.34" {
		t.Errorf("after describeLaunch adapter = %q version %q", described.Adapter, described.AdapterVersion)
	}

	clock = clock.Add(10 * time.Second)
	invocationID := attempt.beginInvocation(true)
	if !managedUUID.MatchString(invocationID) {
		t.Fatalf("invocation id %q is not a canonical lower-case UUID", invocationID)
	}
	launched := requireStored("after beginInvocation")
	if len(launched.Invocations) != 1 {
		t.Fatalf("after beginInvocation the record has %d invocations, want 1", len(launched.Invocations))
	}
	intent := launched.Invocations[0]
	if intent.InvocationID != invocationID || !intent.Resumed || !intent.LaunchedAt.Equal(clock) {
		t.Errorf("launch intent = %+v", intent)
	}
	if intent.Ended != "" || !intent.ObservedAt.IsZero() || len(intent.Usage) != 0 {
		t.Errorf("a launch intent must not carry an outcome yet: %+v", intent)
	}

	clock = clock.Add(20 * time.Second)
	usage := map[string]agent.TokenUsage{
		"model-a": {InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4, CostUSDTicks: 5},
	}
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, usage)
	// The record holds a copy: what the caller does with its map afterwards
	// must not reach the record.
	usage["model-a"] = agent.TokenUsage{InputTokens: 999}
	usage["model-b"] = agent.TokenUsage{InputTokens: 999}
	ended := requireStored("after endInvocation")
	outcome := ended.Invocations[0]
	if outcome.Ended != budgetUsageEndedReturned || !outcome.ObservedAt.Equal(clock) {
		t.Errorf("outcome = %+v", outcome)
	}
	wantUsage := map[string]budgetUsageCounts{
		"model-a": {InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4, CostTicks: 5},
	}
	if !reflect.DeepEqual(outcome.Usage, wantUsage) {
		t.Errorf("recorded usage = %+v, want %+v", outcome.Usage, wantUsage)
	}

	// A second launch that was cut short and reported no usage.
	clock = clock.Add(5 * time.Second)
	secondID := attempt.beginInvocation(false)
	if !managedUUID.MatchString(secondID) || secondID == invocationID {
		t.Fatalf("second invocation id %q, want a canonical UUID other than %q", secondID, invocationID)
	}
	clock = clock.Add(5 * time.Second)
	attempt.endInvocation(secondID, budgetUsageEndedInterrupted, map[string]agent.TokenUsage{})
	twice := requireStored("after the second invocation")
	if len(twice.Invocations) != 2 || !reflect.DeepEqual(twice.Invocations[0], outcome) {
		t.Fatalf("the second invocation changed the first one: %+v", twice.Invocations)
	}
	if second := twice.Invocations[1]; second.InvocationID != secondID || second.Resumed || second.Ended != budgetUsageEndedInterrupted || second.Usage != nil {
		t.Errorf("second invocation = %+v", second)
	}

	clock = clock.Add(20 * time.Second)
	batch := attempt.seal()
	sealed := requireStored("after seal")
	if len(batch) == 0 || !bytes.Equal(sealed.Batch, batch) {
		t.Fatalf("sealed bytes were not persisted exactly:\n returned: %s\n stored:   %s", batch, sealed.Batch)
	}
	wantFile := fileHead + `"adapter":"custom-claude","adapter_version":"2.1.34","normalizer_version":"1","invocations":[` +
		`{"invocation_id":"` + invocationID + `","resumed":true,"launched_at":"2026-11-17T07:59:10Z","ended":"returned","observed_at":"2026-11-17T07:59:30Z",` +
		`"usage":{"model-a":{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,"cache_write_tokens":4,"cost_ticks":5}}},` +
		`{"invocation_id":"` + secondID + `","resumed":false,"launched_at":"2026-11-17T07:59:35Z","ended":"interrupted","observed_at":"2026-11-17T07:59:40Z"}` +
		`],"batch":"` + base64.StdEncoding.EncodeToString(batch) + `"}` + "\n"
	if got := budgetUsageTestFileText(t, journal, budgetUsageTestAttemptA); got != wantFile {
		t.Fatalf("record file after seal\n got:  %s want: %s", got, wantFile)
	}

	if got, want := budgetUsageTestNames(t, directory), []string{budgetUsageTestAttemptA + ".json"}; !reflect.DeepEqual(got, want) {
		t.Errorf("journal directory holds %v after the run, want only the record", got)
	}
	if strings.Contains(logs.String(), "level=WARN") || strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("a healthy journal must not warn:\n%s", logs.String())
	}
}

func TestBudgetUsageRecordReadRejectsForeignFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, _ := budgetUsageTestLogger()

	if _, err := journal.read(budgetUsageTestAttemptA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading a record that does not exist: err=%v, want os.ErrNotExist", err)
	}

	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(attempt.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	// asAttempt is the valid record, rewritten to be the record of another
	// attempt id.
	asAttempt := func(attemptID string) string {
		return strings.Replace(string(valid), budgetUsageTestAttemptA, attemptID, 1)
	}
	path := filepath.Join(directory, budgetUsageTestAttemptB+".json")
	for name, content := range map[string]string{
		"not JSON":                    "{",
		"another attempt's record":    string(valid),
		"an unknown record version":   strings.Replace(asAttempt(budgetUsageTestAttemptB), `"version":1`, `"version":2`, 1),
		"a zero record version":       strings.Replace(asAttempt(budgetUsageTestAttemptB), `"version":1`, `"version":0`, 1),
		"a missing record version":    strings.Replace(asAttempt(budgetUsageTestAttemptB), `"version":1,`, ``, 1),
		"a key outside the pattern":   `{"version":1,"attempt_id":"` + budgetUsageTestAttemptB + `","batch_key":"short","producer_epoch":"e","invocations":[]}`,
		"trailing data after the end": asAttempt(budgetUsageTestAttemptB) + "{}",
		// Well-formed and of the right attempt, but larger than any record
		// the daemon writes: it is not read into memory.
		"a file beyond the size bound": asAttempt(budgetUsageTestAttemptB) + strings.Repeat(" ", maxBudgetUsageRecordBytes),
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if record, err := journal.read(budgetUsageTestAttemptB); err == nil {
			t.Errorf("%s was accepted as a record: %+v", name, record)
		}
	}
	// The same bytes under the right attempt id are a record, up to the bound.
	for name, content := range map[string]string{
		"a well-formed record":        asAttempt(budgetUsageTestAttemptB),
		"a record at the size bound":  asAttempt(budgetUsageTestAttemptB) + strings.Repeat(" ", maxBudgetUsageRecordBytes-len(valid)),
		"a record with a final break": asAttempt(budgetUsageTestAttemptB) + "\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := journal.read(budgetUsageTestAttemptB); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// A record is read only under a canonical attempt id. Each of these names
	// leads to a well-formed record that carries the name as its attempt id,
	// so only the name can refuse it.
	for _, name := range []string{
		strings.ToUpper(budgetUsageTestAttemptB),
		"not-an-attempt-id",
		filepath.Join("nested", budgetUsageTestAttemptB),
		filepath.Join("..", budgetUsageTestAttemptB),
	} {
		content, err := json.Marshal(budgetUsageTestRecordAs(t, valid, name))
		if err != nil {
			t.Fatal(err)
		}
		foreign := journal.recordPath(name)
		if err := os.MkdirAll(filepath.Dir(foreign), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(foreign, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if record, err := journal.read(name); err == nil {
			t.Errorf("a record was read under the name %q: %+v", name, record)
		}
		// On a file system that ignores case one of these is the file of the
		// attempt itself.
		if err := os.Remove(foreign); err != nil {
			t.Fatal(err)
		}
	}

	// Something that is not a regular file is not a record, whatever it
	// leads to.
	t.Run("a symbolic link to a record", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "record.json")
		if err := os.WriteFile(target, []byte(asAttempt(budgetUsageTestAttemptB)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("cannot create a symbolic link here: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
		// The link resolves to a well-formed record of this attempt.
		if raw, err := os.ReadFile(path); err != nil || string(raw) != asAttempt(budgetUsageTestAttemptB) {
			t.Fatalf("the link does not lead to the record: %q, %v", raw, err)
		}
		if record, err := journal.read(budgetUsageTestAttemptB); err == nil {
			t.Errorf("a symbolic link was accepted as a record: %+v", record)
		}
	})
	t.Run("a directory", func(t *testing.T) {
		directoryPath := journal.recordPath(budgetUsageTestAttemptB)
		if err := os.Mkdir(directoryPath, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(directoryPath) })
		if record, err := journal.read(budgetUsageTestAttemptB); err == nil {
			t.Errorf("a directory was accepted as a record: %+v", record)
		}
	})
}

// budgetUsageTestRecordAs decodes a record and gives it another attempt id, which
// need not be a canonical one.
func budgetUsageTestRecordAs(t *testing.T, raw []byte, attemptID string) budgetUsageRecord {
	t.Helper()
	var record budgetUsageRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	record.AttemptID = attemptID
	return record
}

func TestBudgetUsageRecordRefusedForSecondAttempt(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, _ := budgetUsageTestLogger()

	first, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	before := budgetUsageTestRecordJSON(t, budgetUsageTestRequireStored(t, journal, first, "first record"))

	// In memory: the same process is asked for the same attempt again.
	second, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err == nil || second != nil {
		t.Fatalf("a second record for a live attempt: attempt=%v err=%v, want none and an error", second, err)
	}
	// On disk: a later process finds the record of an earlier one.
	restarted := budgetUsageTestOpenJournal(t, directory)
	third, err := restarted.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err == nil || third != nil {
		t.Fatalf("a second record for an attempt already on disk: attempt=%v err=%v, want none and an error", third, err)
	}

	after, err := journal.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatal(err)
	}
	if got := budgetUsageTestRecordJSON(t, after); got != before {
		t.Fatalf("a refused second record changed the first one:\n before: %s\n after:  %s", before, got)
	}

	// Another attempt is not affected and gets its own key.
	other, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptB), logger)
	if err != nil {
		t.Fatalf("create a record for another attempt: %v", err)
	}
	if other.snapshot().BatchKey == first.snapshot().BatchKey {
		t.Fatalf("two attempts share the batch key %q", other.snapshot().BatchKey)
	}
}

func TestBudgetUsageRecordRefusedForNonCanonicalAttemptID(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, _ := budgetUsageTestLogger()

	for _, attemptID := range []string{
		"",
		"not-a-uuid",
		"0195F3A0-0000-7000-8000-00000000000A",
		"{0195f3a0-0000-7000-8000-00000000000a}",
		"0195f3a000007000800000000000000a",
		"0195f3a0-0000-7000-8000-00000000000a\n",
		" 0195f3a0-0000-7000-8000-00000000000a",
		"../0195f3a0-0000-7000-8000-00000000000a",
		"urn:uuid:0195f3a0-0000-7000-8000-00000000000a",
	} {
		attempt, err := journal.begin(budgetUsageTestSeed(attemptID), logger)
		if err == nil || attempt != nil {
			t.Errorf("attempt id %q: attempt=%v err=%v, want no record and an error", attemptID, attempt, err)
		}
	}
	if names := budgetUsageTestNames(t, directory); len(names) != 0 {
		t.Errorf("refused attempt ids left files behind: %v", names)
	}
	if names := budgetUsageTestNames(t, filepath.Dir(directory)); len(names) != 1 {
		t.Errorf("refused attempt ids wrote outside the journal directory: %v", names)
	}
}

func TestBudgetUsageRecordWriteFailureKeepsMemory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	journal := budgetUsageTestOpenJournal(t, directory)
	logger, logs := budgetUsageTestLogger()

	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	// The journal stops being writable in the middle of the run.
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	attempt.describeLaunch("claude", false, "2.1.34")
	invocationID := attempt.beginInvocation(false)
	if invocationID == "" {
		t.Fatal("an invocation must still get its id when the journal cannot be written")
	}
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, map[string]agent.TokenUsage{"secret-model-name": {InputTokens: 7}})
	batch := attempt.seal()
	if len(batch) == 0 {
		t.Fatal("an attempt whose record cannot be written must still be sealed from memory")
	}
	if !strings.Contains(string(batch), `"invocation_id":"`+invocationID+`"`) || !strings.Contains(string(batch), `"input":"7"`) {
		t.Fatalf("the batch sealed from memory lost the invocation: %s", batch)
	}
	record := attempt.snapshot()
	if len(record.Invocations) != 1 || record.AdapterVersion != "2.1.34" || !bytes.Equal(record.Batch, batch) {
		t.Fatalf("memory is not the truth after failed writes: %+v", record)
	}

	logged := logs.String()
	if got := strings.Count(logged, "level=WARN"); got != 4 {
		t.Errorf("failed record writes logged %d warnings, want one per change (4):\n%s", got, logged)
	}
	// Rule 13 covers bodies; the record is the batch's source, so its content
	// stays out of the log as well.
	for _, private := range []string{"secret-model-name", `"tokens"`, `"invocations"`} {
		if strings.Contains(logged, private) {
			t.Errorf("the log carries record content %q:\n%s", private, logged)
		}
	}
}

// The bytes of the wire format are pinned in two files under
// pkg/protocol/testdata that the daemon's builder and the server's decoder
// both test against. Each file holds exactly the bytes of one request body.
const (
	budgetUsageTestWireFixtureBatch = "budget_usage_v1_batch.json"
	budgetUsageTestWireFixtureEmpty = "budget_usage_v1_batch_empty.json"
	// budgetUsageTestGoldenBuilderEpoch is the start identity of the process
	// that builds the two fixture batches.
	budgetUsageTestGoldenBuilderEpoch = "0195f39e-0000-7000-8000-00000000b01d"
)

// budgetUsageTestWireFixture reads one of the shared wire fixture files. No
// test writes them: a builder that changes the bytes has to fail.
func budgetUsageTestWireFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "pkg", "protocol", "testdata", name))
	if err != nil {
		t.Fatalf("read the shared wire fixture: %v", err)
	}
	return raw
}

// budgetUsageTestGoldenRecord is the fixed record behind the fixture
// budget_usage_v1_batch.json. It exercises every rule of the wire format at
// once.
func budgetUsageTestGoldenRecord() budgetUsageRecord {
	east := time.FixedZone("UTC+8", 8*60*60)
	return budgetUsageRecord{
		Version:   budgetUsageRecordVersion,
		AttemptID: "0195f3a0-0000-7000-8000-000000000001",
		BatchKey:  "0195f3a2-0000-7000-8000-000000000002",
		// The process that created the record. The batch names the one that
		// builds it.
		ProducerEpoch:     "0195f39e-0000-7000-8000-00000000c0de",
		CreatedAt:         time.Date(2026, 11, 17, 7, 59, 0, 0, time.UTC),
		ServerURL:         budgetUsageTestServerURL,
		WorkspaceID:       "ws-1",
		TaskID:            "task-1",
		Adapter:           "claude",
		AdapterVersion:    "2.1.34 (Claude Code)",
		NormalizerVersion: "1",
		Invocations: []budgetUsageInvocation{
			{
				InvocationID: "0195f3a1-0000-7000-8000-00000000000a",
				Resumed:      false,
				LaunchedAt:   time.Date(2026, 11, 17, 7, 59, 10, 0, time.UTC),
				Ended:        budgetUsageEndedReturned,
				// A local zone and a fraction: the wire carries UTC seconds.
				ObservedAt: time.Date(2026, 11, 17, 15, 59, 40, 987654321, east),
				Usage: map[string]budgetUsageCounts{
					"claude-fable-5": {InputTokens: 1200, OutputTokens: 340},
					// No tokens and a cost: the item is sent.
					"<synthetic>": {CostTicks: 81000000},
					// Says nothing: the item is left out.
					"all-zero": {},
				},
			},
			{
				InvocationID: "0195f3a1-0000-7000-8000-00000000000b",
				Resumed:      true,
				LaunchedAt:   time.Date(2026, 11, 17, 7, 59, 41, 0, time.UTC),
				Ended:        budgetUsageEndedInterrupted,
				ObservedAt:   time.Date(2026, 11, 17, 7, 59, 58, 0, time.UTC),
				Usage: map[string]budgetUsageCounts{
					"claude-opus-5": {InputTokens: 7, OutputTokens: 8, CacheReadTokens: 9, CacheWriteTokens: 10, CostTicks: 11},
				},
			},
			{
				// A launch intent without an outcome.
				InvocationID: "0195f3a1-0000-7000-8000-00000000000c",
				Resumed:      true,
				LaunchedAt:   time.Date(2026, 11, 17, 7, 59, 59, 500000000, time.UTC),
			},
		},
	}
}

// budgetUsageTestGoldenReportedAt is the build time of the fixture
// budget_usage_v1_batch.json, in a local zone and with a fraction.
func budgetUsageTestGoldenReportedAt() time.Time {
	return time.Date(2026, 11, 17, 16, 0, 0, 123456789, time.FixedZone("UTC+8", 8*60*60))
}

// budgetUsageTestEmptyGoldenRecord is the fixed record behind the fixture
// budget_usage_v1_batch_empty.json: an attempt that launched nothing and
// never described a launch, so it has no adapter version.
func budgetUsageTestEmptyGoldenRecord() budgetUsageRecord {
	return budgetUsageRecord{
		Version:           budgetUsageRecordVersion,
		AttemptID:         "0195f3a0-0000-7000-8000-000000000003",
		BatchKey:          "0195f3a2-0000-7000-8000-000000000003",
		ProducerEpoch:     "0195f39e-0000-7000-8000-00000000c0de",
		CreatedAt:         time.Date(2026, 11, 17, 8, 29, 0, 0, time.UTC),
		ServerURL:         budgetUsageTestServerURL,
		WorkspaceID:       "ws-1",
		TaskID:            "task-1",
		Adapter:           "custom-claude",
		NormalizerVersion: "1",
		Invocations:       []budgetUsageInvocation{},
	}
}

// budgetUsageTestEmptyGoldenReportedAt is the build time of the fixture
// budget_usage_v1_batch_empty.json.
func budgetUsageTestEmptyGoldenReportedAt() time.Time {
	return time.Date(2026, 11, 17, 8, 30, 0, 0, time.UTC)
}

// TestBudgetUsageBatchGolden builds the two fixture batches from fixed records
// and requires the bytes of the shared wire fixture files.
func TestBudgetUsageBatchGolden(t *testing.T) {
	cases := []struct {
		name       string
		fixture    string
		record     budgetUsageRecord
		reportedAt time.Time
	}{
		{"three invocations and an adapter version", budgetUsageTestWireFixtureBatch, budgetUsageTestGoldenRecord(), budgetUsageTestGoldenReportedAt()},
		{"no invocation and no adapter version", budgetUsageTestWireFixtureEmpty, budgetUsageTestEmptyGoldenRecord(), budgetUsageTestEmptyGoldenReportedAt()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := budgetUsageTestWireFixture(t, tc.fixture)
			// A fixture is one request body: one line, no line end after the
			// closing brace.
			if !json.Valid(want) || bytes.ContainsAny(want, "\n\r") || want[len(want)-1] != '}' {
				t.Fatalf("the fixture %s is not exactly one request body: %q", tc.fixture, want)
			}
			got, err := buildBudgetUsageBatch(tc.record, budgetUsageTestGoldenBuilderEpoch, tc.reportedAt)
			if err != nil {
				t.Fatalf("build batch: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("batch bytes differ from the shared wire fixture %s\n got:  %s\n want: %s", tc.fixture, got, want)
			}
		})
	}
}

// budgetUsageTestBatch builds the batch of a record with one invocation that
// returned the given usage, and hands back the bytes of its usage array.
func budgetUsageTestUsageBytes(t *testing.T, usage map[string]budgetUsageCounts) string {
	t.Helper()
	record := budgetUsageRecord{
		Version: budgetUsageRecordVersion, AttemptID: budgetUsageTestAttemptA, BatchKey: "0123456789abcdef",
		Adapter: "claude", NormalizerVersion: "1",
		Invocations: []budgetUsageInvocation{{
			InvocationID: "0195f3a1-0000-7000-8000-00000000000a",
			LaunchedAt:   time.Date(2026, 11, 17, 7, 0, 0, 0, time.UTC),
			Ended:        budgetUsageEndedReturned,
			ObservedAt:   time.Date(2026, 11, 17, 7, 0, 1, 0, time.UTC),
			Usage:        usage,
		}},
	}
	batch, err := buildBudgetUsageBatch(record, "epoch", time.Date(2026, 11, 17, 7, 0, 2, 0, time.UTC))
	if err != nil {
		t.Fatalf("build batch: %v", err)
	}
	const marker = `"usage":`
	index := strings.Index(string(batch), marker)
	if index < 0 || !strings.HasSuffix(string(batch), "}]}") {
		t.Fatalf("unexpected batch shape: %s", batch)
	}
	return strings.TrimSuffix(string(batch)[index+len(marker):], "}]}")
}

func TestBudgetUsageBatchUsageItems(t *testing.T) {
	const zeroTokens = `"tokens":{"input":"0","output":"0","cache_read":"0","cache_write":"0"}`
	cases := []struct {
		name  string
		usage map[string]budgetUsageCounts
		want  string
	}{
		{"no usage at all", nil, `[]`},
		{"an empty usage map", map[string]budgetUsageCounts{}, `[]`},
		{"an item with zero tokens and a cost is sent", map[string]budgetUsageCounts{"m": {CostTicks: 5}}, `[{"model":"m",` + zeroTokens + `,"cost_ticks":"5"}]`},
		{"an item with nothing in it is not sent", map[string]budgetUsageCounts{"m": {}}, `[]`},
		{"zero tokens and a negative cost is not sent", map[string]budgetUsageCounts{"m": {CostTicks: -5}}, `[]`},
		{"cost_ticks is absent at zero", map[string]budgetUsageCounts{"m": {OutputTokens: 3}}, `[{"model":"m","tokens":{"input":"0","output":"3","cache_read":"0","cache_write":"0"}}]`},
		{"cost_ticks is absent when negative", map[string]budgetUsageCounts{"m": {CacheReadTokens: 3, CostTicks: -1}}, `[{"model":"m","tokens":{"input":"0","output":"0","cache_read":"3","cache_write":"0"}}]`},
		{"each token field alone keeps the item", map[string]budgetUsageCounts{
			"a": {InputTokens: 1}, "b": {OutputTokens: 1}, "c": {CacheReadTokens: 1}, "d": {CacheWriteTokens: 1},
		}, `[{"model":"a","tokens":{"input":"1","output":"0","cache_read":"0","cache_write":"0"}},` +
			`{"model":"b","tokens":{"input":"0","output":"1","cache_read":"0","cache_write":"0"}},` +
			`{"model":"c","tokens":{"input":"0","output":"0","cache_read":"1","cache_write":"0"}},` +
			`{"model":"d","tokens":{"input":"0","output":"0","cache_read":"0","cache_write":"1"}}]`},
		{"a negative count is sent as it is", map[string]budgetUsageCounts{"m": {InputTokens: -5}}, `[{"model":"m","tokens":{"input":"-5","output":"0","cache_read":"0","cache_write":"0"}}]`},
		{"an empty model name is sent as it is", map[string]budgetUsageCounts{"": {InputTokens: 1}}, `[{"model":"","tokens":{"input":"1","output":"0","cache_read":"0","cache_write":"0"}}]`},
		{"no HTML escaping", map[string]budgetUsageCounts{"<synthetic>&co": {InputTokens: 1}}, `[{"model":"<synthetic>&co","tokens":{"input":"1","output":"0","cache_read":"0","cache_write":"0"}}]`},
		{"the largest count stays a decimal string", map[string]budgetUsageCounts{"m": {InputTokens: 9223372036854775807, CostTicks: 9223372036854775807}}, `[{"model":"m","tokens":{"input":"9223372036854775807","output":"0","cache_read":"0","cache_write":"0"},"cost_ticks":"9223372036854775807"}]`},
		{"items are sorted by model", map[string]budgetUsageCounts{"b": {InputTokens: 2}, "B": {InputTokens: 1}, "a": {InputTokens: 3}}, `[{"model":"B","tokens":{"input":"1","output":"0","cache_read":"0","cache_write":"0"}},` +
			`{"model":"a","tokens":{"input":"3","output":"0","cache_read":"0","cache_write":"0"}},` +
			`{"model":"b","tokens":{"input":"2","output":"0","cache_read":"0","cache_write":"0"}}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := budgetUsageTestUsageBytes(t, tc.usage); got != tc.want {
				t.Fatalf("usage array\n got:  %s\n want: %s", got, tc.want)
			}
		})
	}
}

func TestBudgetUsageBatchEnvelope(t *testing.T) {
	reportedAt := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	base := budgetUsageRecord{
		Version: budgetUsageRecordVersion, AttemptID: budgetUsageTestAttemptA, BatchKey: "0123456789abcdef",
		ProducerEpoch: "creator-epoch", Adapter: "custom-claude", NormalizerVersion: "1",
	}

	t.Run("no invocation gives an empty array and no adapter version", func(t *testing.T) {
		// The frozen bytes of this shape are the shared wire fixture.
		want := budgetUsageTestWireFixture(t, budgetUsageTestWireFixtureEmpty)
		for name, invocations := range map[string][]budgetUsageInvocation{"nil": nil, "empty": {}} {
			record := budgetUsageTestEmptyGoldenRecord()
			record.Invocations = invocations
			got, err := buildBudgetUsageBatch(record, budgetUsageTestGoldenBuilderEpoch, budgetUsageTestEmptyGoldenReportedAt())
			if err != nil {
				t.Fatalf("%s: build batch: %v", name, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s invocations\n got:  %s\n want: %s", name, got, want)
			}
		}
	})

	t.Run("an invocation without an outcome is unknown with no usage", func(t *testing.T) {
		record := base
		record.Invocations = []budgetUsageInvocation{{
			InvocationID: "0195f3a1-0000-7000-8000-00000000000a",
			LaunchedAt:   time.Date(2026, 11, 17, 7, 30, 0, 0, time.UTC),
			// A usage map on an invocation without an outcome cannot come from
			// this code; if a record ever carries one it is still not sent.
			Usage: map[string]budgetUsageCounts{"m": {InputTokens: 9}},
		}}
		got, err := buildBudgetUsageBatch(record, "builder-epoch", reportedAt)
		if err != nil {
			t.Fatalf("build batch: %v", err)
		}
		want := `"invocations":[{"invocation_id":"0195f3a1-0000-7000-8000-00000000000a","resumed":false,"ended":"unknown","observed_at":"2026-11-17T07:30:00Z","usage":[]}]}`
		if !strings.HasSuffix(string(got), want) {
			t.Fatalf("unknown invocation\n got:  %s\n want suffix: %s", got, want)
		}
	})

	t.Run("the bytes are one JSON value without a trailing newline", func(t *testing.T) {
		got, err := buildBudgetUsageBatch(budgetUsageTestGoldenRecord(), "builder-epoch", reportedAt)
		if err != nil {
			t.Fatalf("build batch: %v", err)
		}
		if !json.Valid(got) || bytes.ContainsAny(got, "\n\r") || got[len(got)-1] != '}' {
			t.Fatalf("batch is not a single compact JSON value: %q", got)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(got, &fields); err != nil {
			t.Fatal(err)
		}
		// The server refuses unknown fields: nothing of the private record
		// (attempt id, task, workspace, server URL, creation time) may leak
		// into the batch.
		allowed := map[string]bool{
			"protocol_version": true, "batch_key": true, "producer_epoch": true, "adapter": true,
			"adapter_version": true, "normalizer_version": true, "reported_at": true, "invocations": true,
		}
		for field := range fields {
			if !allowed[field] {
				t.Errorf("batch carries the field %q, which the wire format does not list", field)
			}
		}
		if string(fields["protocol_version"]) != "1" {
			t.Errorf("protocol_version = %s, want the JSON number 1", fields["protocol_version"])
		}
	})
}

func TestBudgetUsageSealBuildsOnce(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	creator := budgetUsageTestOpenJournal(t, directory)
	clock := time.Date(2026, 11, 17, 7, 59, 0, 0, time.UTC)
	creator.now = func() time.Time { return clock }
	logger, _ := budgetUsageTestLogger()

	attempt, err := creator.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	keyAtCreation := attempt.snapshot().BatchKey
	clock = clock.Add(10 * time.Second)
	invocationID := attempt.beginInvocation(false)
	clock = clock.Add(20 * time.Second)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, map[string]agent.TokenUsage{"m": {InputTokens: 5}})
	clock = clock.Add(30 * time.Second)
	sealedAt := clock

	batch := attempt.seal()
	var decoded struct {
		BatchKey      string `json:"batch_key"`
		ProducerEpoch string `json:"producer_epoch"`
		ReportedAt    string `json:"reported_at"`
		Invocations   []struct {
			InvocationID string `json:"invocation_id"`
			ObservedAt   string `json:"observed_at"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(batch, &decoded); err != nil {
		t.Fatalf("decode sealed batch %q: %v", batch, err)
	}
	record := attempt.snapshot()
	if decoded.BatchKey != keyAtCreation || record.BatchKey != keyAtCreation {
		t.Errorf("batch key = %q, record key = %q, want the key the record was created with %q", decoded.BatchKey, record.BatchKey, keyAtCreation)
	}
	if decoded.ProducerEpoch != creator.epoch {
		t.Errorf("producer_epoch = %q, want the building process's %q", decoded.ProducerEpoch, creator.epoch)
	}
	if want := sealedAt.Format(time.RFC3339); decoded.ReportedAt != want {
		t.Errorf("reported_at = %q, want the build time %q", decoded.ReportedAt, want)
	}
	if len(decoded.Invocations) != 1 || decoded.Invocations[0].InvocationID != invocationID {
		t.Fatalf("sealed invocations = %+v", decoded.Invocations)
	}
	if want := sealedAt.Add(-30 * time.Second).Format(time.RFC3339); decoded.Invocations[0].ObservedAt != want {
		t.Errorf("observed_at = %q, want the time the outcome was recorded %q", decoded.Invocations[0].ObservedAt, want)
	}

	// What was persisted is what is sent: sealing again never rebuilds, and
	// nothing that happens after the seal reaches the record.
	before := budgetUsageTestRecordJSON(t, budgetUsageTestRequireStored(t, creator, attempt, "after seal"))
	clock = clock.Add(time.Hour)
	if again := attempt.seal(); !bytes.Equal(again, batch) {
		t.Fatalf("a second seal produced other bytes\n first:  %s\n second: %s", batch, again)
	}
	if late := attempt.beginInvocation(true); late != "" {
		t.Errorf("an invocation was accepted after the seal: %q", late)
	}
	attempt.endInvocation(invocationID, budgetUsageEndedInterrupted, map[string]agent.TokenUsage{"m": {InputTokens: 6}})
	attempt.describeLaunch("codex", true, "9.9.9")
	if after := budgetUsageTestRecordJSON(t, budgetUsageTestRequireStored(t, creator, attempt, "after late changes")); after != before {
		t.Fatalf("the record changed after it was sealed\n before: %s\n after:  %s", before, after)
	}
}

// TestBudgetUsageSealNamesTheBuildingProcess: the batch carries the start
// identity of the process that builds it. The record keeps the identity of
// the process that created it, which is another one once the daemon was
// restarted between the two. The later process is stood in for by an attempt
// built around the record it read from disk.
func TestBudgetUsageSealNamesTheBuildingProcess(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "budget-usage-reports")
	creator := budgetUsageTestOpenJournal(t, directory)
	logger, _ := budgetUsageTestLogger()
	created, err := creator.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	invocationID := created.beginInvocation(false)
	key := created.snapshot().BatchKey

	builder := budgetUsageTestOpenJournal(t, directory)
	if builder.epoch == creator.epoch {
		t.Fatalf("the two processes share the epoch %q", builder.epoch)
	}
	left, err := builder.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatalf("read the record the earlier process left: %v", err)
	}
	adopted := &budgetUsageAttempt{journal: builder, log: logger, record: left}
	batch := adopted.seal()

	var decoded struct {
		BatchKey      string `json:"batch_key"`
		ProducerEpoch string `json:"producer_epoch"`
		Invocations   []struct {
			InvocationID string `json:"invocation_id"`
			Ended        string `json:"ended"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(batch, &decoded); err != nil {
		t.Fatalf("decode sealed batch %q: %v", batch, err)
	}
	if decoded.ProducerEpoch != builder.epoch {
		t.Errorf("producer_epoch = %q, want the building process's %q (the creating process was %q)", decoded.ProducerEpoch, builder.epoch, creator.epoch)
	}
	if decoded.BatchKey != key {
		t.Errorf("batch_key = %q, want the key the record was created with %q", decoded.BatchKey, key)
	}
	// The launch whose outcome the earlier process never recorded.
	if len(decoded.Invocations) != 1 || decoded.Invocations[0].InvocationID != invocationID || decoded.Invocations[0].Ended != budgetUsageEndedUnknown {
		t.Errorf("sealed invocations = %+v, want the one launch intent as unknown", decoded.Invocations)
	}
	stored, err := builder.read(budgetUsageTestAttemptA)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ProducerEpoch != creator.epoch || stored.BatchKey != key {
		t.Errorf("sealed record: epoch %q key %q, want the creating process's epoch %q and the key %q", stored.ProducerEpoch, stored.BatchKey, creator.epoch, key)
	}
	if !bytes.Equal(stored.Batch, batch) {
		t.Errorf("sealed bytes were not persisted exactly:\n returned: %s\n stored:   %s", batch, stored.Batch)
	}
}

// TestBudgetUsageRecordHandsOutCopies: what an attempt hands out is the
// caller's to change. Neither the record nor the bytes of its sealed batch can
// be reached through it.
func TestBudgetUsageRecordHandsOutCopies(t *testing.T) {
	journal := budgetUsageTestOpenJournal(t, filepath.Join(t.TempDir(), "budget-usage-reports"))
	logger, _ := budgetUsageTestLogger()
	attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	invocationID := attempt.beginInvocation(false)
	attempt.endInvocation(invocationID, budgetUsageEndedReturned, map[string]agent.TokenUsage{"m": {InputTokens: 5}})

	snapshot := attempt.snapshot()
	if len(snapshot.Invocations) != 1 || len(snapshot.Invocations[0].Usage) != 1 {
		t.Fatalf("the record does not hold the one invocation and its usage: %+v", snapshot.Invocations)
	}
	snapshot.BatchKey = "changed"
	snapshot.Invocations[0].Ended = "changed"
	snapshot.Invocations[0].Usage["m"] = budgetUsageCounts{InputTokens: 999}
	snapshot.Invocations[0].Usage["other"] = budgetUsageCounts{InputTokens: 999}
	// The file was written before the snapshot was taken, so a live record
	// that still equals it was not reached.
	budgetUsageTestRequireStored(t, journal, attempt, "after changing a snapshot")

	first := attempt.seal()
	want := string(first)
	if want == "" {
		t.Fatal("the attempt was not sealed")
	}
	scribble := func(handedOut []byte) {
		for i := range handedOut {
			handedOut[i] = 'x'
		}
	}
	scribble(first)
	second := attempt.seal()
	if string(second) != want {
		t.Fatalf("changing the bytes of the first seal changed the batch\n got:  %s\n want: %s", second, want)
	}
	scribble(second)
	sealed := attempt.snapshot()
	if string(sealed.Batch) != want {
		t.Fatalf("changing the bytes of a later seal changed the batch\n got:  %s\n want: %s", sealed.Batch, want)
	}
	scribble(sealed.Batch)
	if got := string(attempt.seal()); got != want {
		t.Fatalf("changing the batch of a snapshot changed the batch\n got:  %s\n want: %s", got, want)
	}
	if stored := budgetUsageTestRequireStored(t, journal, attempt, "after changing every copy"); string(stored.Batch) != want {
		t.Fatalf("the stored batch changed\n got:  %s\n want: %s", stored.Batch, want)
	}
}

// TestBudgetUsageRecordCreationFailures: a record that cannot be created is no
// record at all. begin fails and names the step, keeps nothing in the process
// and leaves no file behind, so the attempt is not reported later as one that
// launched nothing.
func TestBudgetUsageRecordCreationFailures(t *testing.T) {
	// withMode gives the journal directory the mode for the time of the
	// attempt to create the record.
	withMode := func(mode os.FileMode) func(t *testing.T, directory string) {
		return func(t *testing.T, directory string) {
			if err := os.Chmod(directory, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
		}
	}
	cases := []struct {
		name string
		// posix cases need POSIX permission bits and a non-root user.
		posix    bool
		arrange  func(t *testing.T, directory string)
		wantStep string
	}{
		{
			// Whether the attempt has a record cannot be told, so no second
			// one is attempted.
			name: "the journal path is no directory any more",
			arrange: func(t *testing.T, directory string) {
				if err := os.RemoveAll(directory); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantStep: "check for an existing usage report record",
		},
		{name: "the directory cannot be searched", posix: true, arrange: withMode(0o600), wantStep: "check for an existing usage report record"},
		{name: "the directory cannot be written", posix: true, arrange: withMode(0o500), wantStep: "write usage report record"},
		// The file can be created and renamed into place; the write fails
		// after that, when the directory is opened to be synced.
		{name: "the directory cannot be synced", posix: true, arrange: withMode(0o300), wantStep: "write usage report record"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.posix && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("needs POSIX permission bits and a non-root user")
			}
			directory := filepath.Join(t.TempDir(), "budget-usage-reports")
			journal := budgetUsageTestOpenJournal(t, directory)
			logger, _ := budgetUsageTestLogger()
			tc.arrange(t, directory)

			attempt, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger)
			if err == nil || attempt != nil {
				t.Fatalf("attempt=%v err=%v, want no record and an error", attempt, err)
			}
			if !strings.HasPrefix(err.Error(), tc.wantStep+": ") {
				t.Errorf("begin failed with %q, want it to name the step %q", err, tc.wantStep)
			}
			if errors.Is(err, errBudgetUsageRecordExists) {
				t.Errorf("begin reported an existing record: %v", err)
			}
			journal.mu.Lock()
			live := len(journal.attempts)
			journal.mu.Unlock()
			if live != 0 {
				t.Errorf("the process keeps %d records of an attempt whose record could not be created", live)
			}
			if !tc.posix {
				if raw, err := os.ReadFile(directory); err != nil || string(raw) != "not a directory" {
					t.Errorf("the unusable journal path was changed: %q, %v", raw, err)
				}
				return
			}
			if err := os.Chmod(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if names := budgetUsageTestNames(t, directory); len(names) != 0 {
				t.Errorf("a record that could not be created left files behind: %v", names)
			}
			// Nothing half-made stands in the way once the directory is usable
			// again.
			if _, err := journal.begin(budgetUsageTestSeed(budgetUsageTestAttemptA), logger); err != nil {
				t.Errorf("create the record after the directory became usable: %v", err)
			}
		})
	}
}

func TestBudgetUsageNilAttemptIsInert(t *testing.T) {
	var attempt *budgetUsageAttempt
	attempt.describeLaunch("claude", false, "2.1.34")
	if id := attempt.beginInvocation(true); id != "" {
		t.Errorf("a nil attempt handed out the invocation id %q", id)
	}
	attempt.endInvocation("anything", budgetUsageEndedReturned, map[string]agent.TokenUsage{"m": {InputTokens: 1}})
	if batch := attempt.seal(); batch != nil {
		t.Errorf("a nil attempt sealed %q", batch)
	}
	ctx := context.Background()
	if got := withBudgetUsageAttempt(ctx, nil); got != ctx {
		t.Error("carrying no attempt must leave the context as it is")
	}
	if got := budgetUsageAttemptFrom(ctx); got != nil {
		t.Errorf("a context without an attempt returned %v", got)
	}
}

func TestBudgetUsageAdapterName(t *testing.T) {
	if got := budgetUsageAdapterName("claude", false); got != "claude" {
		t.Errorf("built-in executable: adapter = %q, want %q", got, "claude")
	}
	if got := budgetUsageAdapterName("omp", true); got != "custom-omp" {
		t.Errorf("custom profile command: adapter = %q, want %q", got, "custom-omp")
	}
}
