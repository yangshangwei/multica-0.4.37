package daemon

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Budget usage protocol v1, daemon half: resending what the journal holds,
// recovering what an earlier process left behind, and the retention cap.

const (
	// budgetUsageRetention is how long a record is kept at most. A record
	// that is older is dropped, whatever became of its batch.
	budgetUsageRetention = 7 * 24 * time.Hour
	// budgetUsageResendInterval is the time between two passes of the resend
	// loop.
	budgetUsageResendInterval = 30 * time.Second
	// maxBudgetUsageRecordsPerPass bounds the records one pass handles. The
	// next pass goes on after the last of them.
	maxBudgetUsageRecordsPerPass = 32
)

// outlived says whether something that dates from the given time is older
// than the retention cap.
func (j *budgetUsageJournal) outlived(since time.Time) bool {
	return j.now().Sub(since) > budgetUsageRetention
}

// liveIDs lists the attempts whose record this process holds.
func (j *budgetUsageJournal) liveIDs() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Collect(maps.Keys(j.attempts))
}

// takeRound picks the attempts of one pass out of the sorted ids: at most
// limit of them, starting after the one the pass before ended with and going
// around the end of the list.
func (j *budgetUsageJournal) takeRound(ids []string, limit int) []string {
	if len(ids) == 0 {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	start, found := slices.BinarySearch(ids, j.cursor)
	if found {
		start++
	}
	round := make([]string, 0, min(limit, len(ids)))
	for n := range cap(round) {
		round = append(round, ids[(start+n)%len(ids)])
	}
	j.cursor = round[len(round)-1]
	return round
}

// budgetUsageRecordIDs lists the attempts that have a record, sorted: those
// the journal's process holds and those that have a file in the directory.
// Only a file named <canonical UUID>.json is a record. A file of any other
// name is not read and not reported, with one exception: the temporary file
// that a write leaves behind when the process dies in the middle of it is
// removed once it has outlived the retention cap.
func (d *Daemon) budgetUsageRecordIDs(journal *budgetUsageJournal) []string {
	ids := journal.liveIDs()
	entries, err := os.ReadDir(journal.directory)
	if err != nil {
		// The records of this process are handled all the same, with what the
		// directory gave before it failed.
		d.logger.Debug("budget usage: could not list the usage report journal", "error", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if id, named := strings.CutSuffix(name, budgetUsageRecordSuffix); named && managedUUID.MatchString(id) {
			ids = append(ids, id)
			continue
		}
		if !strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".tmp") {
			continue
		}
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() && journal.outlived(info.ModTime()) {
			_ = os.Remove(filepath.Join(journal.directory, name))
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// resendBudgetUsage is one pass of the resend loop over the journal.
func (d *Daemon) resendBudgetUsage(ctx context.Context) {
	journal := d.budgetUsage
	if journal == nil {
		return
	}
	for _, attemptID := range journal.takeRound(d.budgetUsageRecordIDs(journal), maxBudgetUsageRecordsPerPass) {
		if ctx.Err() != nil {
			// The daemon is stopping: what is left stays as it is.
			return
		}
		d.handleBudgetUsage(ctx, attemptID, true)
	}
}

// budgetUsageLoop makes a pass when the daemon starts, which is the recovery
// of what an earlier process left behind, and one at every tick after that,
// until the daemon stops. A daemon without a journal has no loop.
func (d *Daemon) budgetUsageLoop(ctx context.Context, every time.Duration) {
	if d.budgetUsage == nil {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		d.resendBudgetUsage(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// dropOutlivedBudgetUsage deletes a record that is older than the retention
// cap and says so.
func (d *Daemon) dropOutlivedBudgetUsage(journal *budgetUsageJournal, record budgetUsageRecord) {
	d.logger.Warn("budget usage: dropping a usage report that is older than seven days and was not delivered",
		"attempt_id", record.AttemptID, "created_at", record.CreatedAt)
	d.removeBudgetUsageRecord(journal, record.AttemptID)
}

// skipUnreadableBudgetUsage is what a pass does with a file that has the name
// of a record and cannot be read as one: nothing, until the file has outlived
// the retention cap. A record is at least as old as the last change of its
// file, so a regular file that was last changed longer ago than the cap holds
// nothing that could still be sent, whatever is in it. What is not a regular
// file is never removed. The read error stays out of the log: it can quote
// the file.
func (d *Daemon) skipUnreadableBudgetUsage(journal *budgetUsageJournal, attemptID string) {
	info, err := os.Lstat(journal.recordPath(attemptID))
	if err != nil || !info.Mode().IsRegular() || !journal.outlived(info.ModTime()) {
		d.logger.Debug("budget usage: skipping a usage report record that cannot be read", "attempt_id", attemptID)
		return
	}
	d.logger.Warn("budget usage: dropping a usage report record that cannot be read and was last changed more than seven days ago",
		"attempt_id", attemptID)
	d.removeBudgetUsageRecord(journal, attemptID)
}

// recoverBudgetUsage handles a record that the journal's process does not
// hold: one that an earlier process left behind, or a leftover of this one.
func (d *Daemon) recoverBudgetUsage(ctx context.Context, journal *budgetUsageJournal, record budgetUsageRecord) {
	if journal.outlived(record.CreatedAt) {
		d.dropOutlivedBudgetUsage(journal, record)
		return
	}
	// A record goes only to the server it was created under. Under any other
	// server it is left exactly as it is: nothing is built for a server that
	// is not there to take it.
	if record.ServerURL != journal.serverURL {
		d.logger.Debug("budget usage: keeping a usage report that belongs to another server", "attempt_id", record.AttemptID)
		return
	}
	if len(record.Batch) != 0 {
		// What was persisted is what is sent.
		d.sendBudgetUsage(ctx, journal, record)
		return
	}
	if len(record.Invocations) == 0 {
		// The record does not say that a provider was launched. There is
		// nothing to report, whichever process left it.
		d.logger.Warn("budget usage: removing a usage report record that names no provider launch; nothing is reported for it",
			"attempt_id", record.AttemptID)
		d.removeBudgetUsageRecord(journal, record.AttemptID)
		return
	}
	if record.ProducerEpoch == journal.epoch {
		// This process created the record and holds no attempt for it: the
		// run it belonged to went on without the protocol.
		d.logger.Warn("budget usage: removing a usage report record of this process that no running attempt holds; nothing is reported for it",
			"attempt_id", record.AttemptID)
		d.removeBudgetUsageRecord(journal, record.AttemptID)
		return
	}
	// An earlier process launched a provider and did not live to build the
	// batch. This process builds it, under its own epoch: an invocation that
	// has no outcome is reported as unknown. The bytes must be in the journal
	// before they are sent, or the next process would build other bytes under
	// the same key.
	batch, err := buildBudgetUsageBatch(record, journal.epoch, journal.now())
	if err != nil {
		d.logger.Debug("budget usage: could not build the batch of a usage report that an earlier process left; keeping its record as it is",
			"attempt_id", record.AttemptID, "error", err)
		return
	}
	record.Batch = batch
	if err := journal.write(record); err != nil {
		d.logger.Debug("budget usage: could not persist the batch of a usage report that an earlier process left; keeping its record for the next pass",
			"attempt_id", record.AttemptID, "error", err)
		return
	}
	d.sendBudgetUsage(ctx, journal, record)
}
