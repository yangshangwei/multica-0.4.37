package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
)

// Budget usage protocol v1, daemon half.
//
// For a claimed task that carries a budget attempt id the daemon records
// every provider invocation it launches and what that invocation used, and
// turns the record into one batch per attempt when the run is over. The
// record is kept in a journal directory, one JSON file per attempt, so that
// an attempt survives a daemon restart.

const (
	// budgetUsageProtocolVersion is the protocol_version of every batch.
	budgetUsageProtocolVersion = 1
	// budgetUsageRecordVersion versions the private on-disk form of a record.
	// It is not part of the wire format.
	budgetUsageRecordVersion = 1
	// maxBudgetUsageRecordBytes bounds what the journal reads back as one record.
	maxBudgetUsageRecordBytes = 4 << 20
	// maxBudgetUsageAdapterVersionBytes is the longest adapter_version the
	// server accepts.
	maxBudgetUsageAdapterVersionBytes = 120
)

// How an invocation ended, as the wire format names it.
const (
	budgetUsageEndedReturned    = "returned"
	budgetUsageEndedInterrupted = "interrupted"
	budgetUsageEndedUnknown     = "unknown"
)

var budgetUsageBatchKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

var errBudgetUsageRecordExists = errors.New("a usage report record for this budget attempt already exists")

// budgetUsageRecord is the on-disk form of one attempt's usage report.
type budgetUsageRecord struct {
	Version   int    `json:"version"`
	AttemptID string `json:"attempt_id"`
	// BatchKey is minted once, when the record is created, and never changes.
	BatchKey string `json:"batch_key"`
	// ProducerEpoch is the start identity of the process that created the
	// record. The batch carries the identity of the process that builds it,
	// which is another one after a restart.
	ProducerEpoch string    `json:"producer_epoch"`
	CreatedAt     time.Time `json:"created_at"`
	// ServerURL is the server the attempt was claimed from. The record is
	// reported to that server only.
	ServerURL         string                  `json:"server_url"`
	WorkspaceID       string                  `json:"workspace_id"`
	TaskID            string                  `json:"task_id"`
	Adapter           string                  `json:"adapter"`
	AdapterVersion    string                  `json:"adapter_version,omitempty"`
	NormalizerVersion string                  `json:"normalizer_version"`
	Invocations       []budgetUsageInvocation `json:"invocations"`
	// Batch holds the exact bytes of the batch once the attempt is sealed.
	// What is stored here is what is sent, every time.
	Batch []byte `json:"batch,omitempty"`
}

// budgetUsageInvocation is one provider process launch of an attempt.
type budgetUsageInvocation struct {
	InvocationID string    `json:"invocation_id"`
	Resumed      bool      `json:"resumed"`
	LaunchedAt   time.Time `json:"launched_at"`
	// Ended is empty while only the launch intent is known.
	Ended      string    `json:"ended,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitzero"`
	// Usage is this invocation's own usage, by model.
	Usage map[string]budgetUsageCounts `json:"usage,omitempty"`
}

// budgetUsageCounts is what one invocation used on one model.
type budgetUsageCounts struct {
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
	CostTicks        int64 `json:"cost_ticks,omitempty"`
}

func (r budgetUsageRecord) clone() budgetUsageRecord {
	r.Invocations = slices.Clone(r.Invocations)
	for i := range r.Invocations {
		r.Invocations[i].Usage = maps.Clone(r.Invocations[i].Usage)
	}
	r.Batch = bytes.Clone(r.Batch)
	return r
}

// budgetUsageCountsFrom copies an invocation's usage into the record's form.
func budgetUsageCountsFrom(usage map[string]agent.TokenUsage) map[string]budgetUsageCounts {
	if len(usage) == 0 {
		return nil
	}
	counts := make(map[string]budgetUsageCounts, len(usage))
	for model, u := range usage {
		counts[model] = budgetUsageCounts{
			InputTokens:      u.InputTokens,
			OutputTokens:     u.OutputTokens,
			CacheReadTokens:  u.CacheReadTokens,
			CacheWriteTokens: u.CacheWriteTokens,
			CostTicks:        u.CostUSDTicks,
		}
	}
	return counts
}

// budgetUsageAdapterName is the adapter a batch names: the provider key when
// the task runs the provider's own executable, and custom-<provider key> when
// it runs the command of a custom runtime profile, whose behaviour the
// provider's evidence does not cover.
func budgetUsageAdapterName(provider string, customProfileCommand bool) string {
	if customProfileCommand {
		return "custom-" + provider
	}
	return provider
}

// budgetUsageAdapterVersion is the adapter version a record holds for what an
// executable printed as its version. The value is a note in the envelope, and
// the server refuses a whole batch whose adapter_version it does not accept.
// So the white space around the value is dropped, and a value the server would
// refuse (empty, longer than 120 bytes, not valid UTF-8, or holding a control
// character) counts as no version: the record and the batch leave the field
// out, as they do for the empty value this function returns then.
func budgetUsageAdapterVersion(cliVersion string) string {
	version := strings.TrimSpace(cliVersion)
	if len(version) > maxBudgetUsageAdapterVersionBytes || !utf8.ValidString(version) {
		return ""
	}
	if strings.ContainsFunc(version, unicode.IsControl) {
		return ""
	}
	return version
}

// budgetUsageJournal is the directory of usage report records of one daemon
// process.
type budgetUsageJournal struct {
	directory string
	// epoch is the start identity of this process.
	epoch string
	// serverURL is the server base URL records are created under.
	serverURL string
	now       func() time.Time

	mu sync.Mutex
	// attempts holds the records this process created, by attempt id. An entry
	// stays until its record is deleted.
	attempts map[string]*budgetUsageAttempt
	// inFlight marks the records that are being handled right now.
	inFlight map[string]struct{}
	// settled names the records that are done with (delivered, refused for
	// good or dropped) and whose file could not be removed.
	settled map[string]struct{}
	// cursor is the attempt the last pass of the resend loop ended with.
	cursor string

	// background counts the sends that handleTask started and that have not
	// ended yet.
	background sync.WaitGroup
}

// openBudgetUsageJournal prepares the journal directory and proves that it
// can hold records: a probe file is written the way a record is written
// (temporary file, fsync, rename, directory fsync) and removed again, and the
// directory is listed. Without that proof there is no journal, and the caller
// does not declare the protocol.
func openBudgetUsageJournal(directory, serverURL string) (*budgetUsageJournal, error) {
	// MkdirAll fails when the path exists and is not a directory.
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create budget usage journal directory: %w", err)
	}
	probe := filepath.Join(directory, ".probe-"+uuid.NewString()+".tmp")
	if err := writeManagementJSON(probe, struct{}{}); err != nil {
		// The write can fail after the rename, at the directory sync.
		_ = os.Remove(probe)
		return nil, fmt.Errorf("write budget usage journal probe: %w", err)
	}
	if err := os.Remove(probe); err != nil {
		return nil, fmt.Errorf("remove budget usage journal probe: %w", err)
	}
	if _, err := os.ReadDir(directory); err != nil {
		return nil, fmt.Errorf("list budget usage journal directory: %w", err)
	}
	return &budgetUsageJournal{
		directory: directory,
		epoch:     uuid.NewString(),
		serverURL: serverURL,
		now:       time.Now,
		attempts:  make(map[string]*budgetUsageAttempt),
		inFlight:  make(map[string]struct{}),
		settled:   make(map[string]struct{}),
	}, nil
}

// budgetUsageRecordSuffix ends the name of a record file, after the attempt
// id.
const budgetUsageRecordSuffix = ".json"

func (j *budgetUsageJournal) recordPath(attemptID string) string {
	return filepath.Join(j.directory, attemptID+budgetUsageRecordSuffix)
}

// write replaces the record's file atomically: temporary file, fsync, rename,
// directory fsync.
func (j *budgetUsageJournal) write(record budgetUsageRecord) error {
	return writeManagementJSON(j.recordPath(record.AttemptID), record)
}

// read loads the record of one attempt. A file that is not a record of that
// attempt in this version of the on-disk form is an error.
func (j *budgetUsageJournal) read(attemptID string) (budgetUsageRecord, error) {
	if !managedUUID.MatchString(attemptID) {
		return budgetUsageRecord{}, errors.New("invalid usage report record name")
	}
	path := j.recordPath(attemptID)
	info, err := os.Lstat(path)
	if err != nil {
		return budgetUsageRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxBudgetUsageRecordBytes {
		return budgetUsageRecord{}, errors.New("invalid usage report record file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return budgetUsageRecord{}, err
	}
	var record budgetUsageRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return budgetUsageRecord{}, fmt.Errorf("decode usage report record: %w", err)
	}
	if record.Version != budgetUsageRecordVersion || record.AttemptID != attemptID || !budgetUsageBatchKeyPattern.MatchString(record.BatchKey) {
		return budgetUsageRecord{}, errors.New("invalid usage report record")
	}
	return record, nil
}

// budgetUsageSeed is what a record is created from.
type budgetUsageSeed struct {
	AttemptID         string
	WorkspaceID       string
	TaskID            string
	Adapter           string
	NormalizerVersion string
}

// begin creates the record of an attempt and mints its batch key. It fails
// when the attempt id is not a canonical UUID, when the attempt already has a
// record, or when the record cannot be written; the caller then runs the task
// without the protocol.
func (j *budgetUsageJournal) begin(seed budgetUsageSeed, log *slog.Logger) (*budgetUsageAttempt, error) {
	if !managedUUID.MatchString(seed.AttemptID) {
		return nil, errors.New("budget attempt id is not a canonical lower-case UUID")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	// One attempt never gets two batch keys: an attempt that already has a
	// record, from this process or from an earlier one, keeps that record.
	if _, live := j.attempts[seed.AttemptID]; live {
		return nil, errBudgetUsageRecordExists
	}
	path := j.recordPath(seed.AttemptID)
	if _, err := os.Lstat(path); err == nil {
		return nil, errBudgetUsageRecordExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("check for an existing usage report record: %w", err)
	}
	record := budgetUsageRecord{
		Version:           budgetUsageRecordVersion,
		AttemptID:         seed.AttemptID,
		BatchKey:          uuid.NewString(),
		ProducerEpoch:     j.epoch,
		CreatedAt:         j.now().UTC(),
		ServerURL:         j.serverURL,
		WorkspaceID:       seed.WorkspaceID,
		TaskID:            seed.TaskID,
		Adapter:           seed.Adapter,
		NormalizerVersion: seed.NormalizerVersion,
		Invocations:       []budgetUsageInvocation{},
	}
	if err := j.write(record); err != nil {
		// The write can fail after the rename. A record that stayed behind
		// would later be reported as an attempt that launched nothing, while
		// this run goes on without the protocol.
		_ = os.Remove(path)
		return nil, fmt.Errorf("write usage report record: %w", err)
	}
	attempt := &budgetUsageAttempt{journal: j, log: log, record: record}
	j.attempts[seed.AttemptID] = attempt
	return attempt, nil
}

// budgetUsageAttempt is the live record of one attempt in the process that
// created it. describeLaunch, beginInvocation, endInvocation and seal are safe
// on a nil attempt, which is what a task without the protocol carries.
type budgetUsageAttempt struct {
	journal *budgetUsageJournal
	log     *slog.Logger

	mu     sync.Mutex
	record budgetUsageRecord
}

// persistLocked rewrites the whole record. A journal that cannot be written
// never blocks the task: the failure is logged and the copy in memory stays
// the truth for this process. The file is left as it is: it then lacks a
// launch description, an outcome or the batch bytes, and still holds every
// launch intent. The caller holds a.mu.
func (a *budgetUsageAttempt) persistLocked(change string) {
	if err := a.journal.write(a.record); err != nil {
		a.log.Warn("budget usage: could not write the usage report record, keeping it in memory",
			"attempt_id", a.record.AttemptID, "change", change, "error", err)
	}
}

// sealedLocked reports whether the batch bytes exist. From then on the record
// does not change. The caller holds a.mu.
func (a *budgetUsageAttempt) sealedLocked() bool {
	return len(a.record.Batch) > 0
}

// describeLaunch records which adapter this run launches and the CLI version
// resolved for it, when that version is one a batch can carry.
func (a *budgetUsageAttempt) describeLaunch(provider string, customProfileCommand bool, cliVersion string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sealedLocked() {
		return
	}
	a.record.Adapter = budgetUsageAdapterName(provider, customProfileCommand)
	a.record.AdapterVersion = budgetUsageAdapterVersion(cliVersion)
	a.persistLocked("launch description")
}

// beginInvocation records the intent to launch a provider process and returns
// the invocation's id, minted here, before the launch.
func (a *budgetUsageAttempt) beginInvocation(resumed bool) string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sealedLocked() {
		return ""
	}
	invocation := budgetUsageInvocation{
		InvocationID: uuid.NewString(),
		Resumed:      resumed,
		LaunchedAt:   a.journal.now().UTC(),
	}
	a.record.Invocations = append(a.record.Invocations, invocation)
	a.persistLaunchIntentLocked()
	return invocation.InvocationID
}

// persistLaunchIntentLocked writes the record after a launch intent was added.
// A record file must never be behind its process in launches: a later process
// would report the attempt without an invocation that was started. So when
// this write fails the file is removed, best effort. Only the file goes: the
// attempt stays live, the run goes on, and the next write of the whole record
// that succeeds brings the file back complete. The caller holds a.mu.
func (a *budgetUsageAttempt) persistLaunchIntentLocked() {
	err := a.journal.write(a.record)
	if err == nil {
		return
	}
	removeErr := os.Remove(a.journal.recordPath(a.record.AttemptID))
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		a.log.Warn("budget usage: could not write the usage report record with a launch intent, removed its file and keeping the record in memory",
			"attempt_id", a.record.AttemptID, "error", err)
		return
	}
	a.log.Warn("budget usage: could not write the usage report record with a launch intent and could not remove its file, keeping the record in memory",
		"attempt_id", a.record.AttemptID, "error", err, "remove_error", removeErr)
}

// endInvocation records how an invocation ended and what it used itself. The
// usage is copied.
func (a *budgetUsageAttempt) endInvocation(invocationID, ended string, usage map[string]agent.TokenUsage) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sealedLocked() {
		return
	}
	for i := range a.record.Invocations {
		invocation := &a.record.Invocations[i]
		if invocation.InvocationID != invocationID {
			continue
		}
		invocation.Ended = ended
		invocation.ObservedAt = a.journal.now().UTC()
		invocation.Usage = budgetUsageCountsFrom(usage)
		a.persistLocked("invocation outcome")
		return
	}
}

// seal builds the batch of the attempt once and persists its bytes. Sealing
// again returns the same bytes: a batch that exists is never rebuilt.
func (a *budgetUsageAttempt) seal() []byte {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sealedLocked() {
		return bytes.Clone(a.record.Batch)
	}
	batch, err := buildBudgetUsageBatch(a.record, a.journal.epoch, a.journal.now())
	if err != nil {
		a.log.Warn("budget usage: could not build the usage batch", "attempt_id", a.record.AttemptID, "error", err)
		return nil
	}
	a.record.Batch = batch
	a.persistLocked("sealed batch")
	return bytes.Clone(batch)
}

// snapshot returns a copy of the record as this process holds it.
func (a *budgetUsageAttempt) snapshot() budgetUsageRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.record.clone()
}

type budgetUsageAttemptKey struct{}

// withBudgetUsageAttempt carries the attempt of a task to the code that
// launches its provider processes. Without an attempt the context is returned
// as it is.
func withBudgetUsageAttempt(ctx context.Context, attempt *budgetUsageAttempt) context.Context {
	if attempt == nil {
		return ctx
	}
	return context.WithValue(ctx, budgetUsageAttemptKey{}, attempt)
}

// budgetUsageAttemptFrom returns the attempt the context carries, or nil.
func budgetUsageAttemptFrom(ctx context.Context) *budgetUsageAttempt {
	attempt, _ := ctx.Value(budgetUsageAttemptKey{}).(*budgetUsageAttempt)
	return attempt
}
