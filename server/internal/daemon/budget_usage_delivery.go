package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/multica-ai/multica/server/internal/cli"
)

// Budget usage protocol v1, daemon half: declaring the protocol and
// delivering the batch of an attempt.

const (
	// budgetUsageJournalDirectoryName is the journal's own directory under the
	// state directory of the daemon's profile.
	budgetUsageJournalDirectoryName = "budget-usage-reports"
	// maxBudgetUsageReplyBytes is how much of an answer is kept to look for
	// its error code. The rest is read and dropped.
	maxBudgetUsageReplyBytes = 64 << 10
)

// budgetUsageErrorCodePattern is the form of an error code that may be
// logged.
var budgetUsageErrorCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// budgetUsageReply is the answer to one usage report. It is not an error and
// never a *requestError, so the reactions the daemon has to a 404 of its
// other routes (register the runtime again, interrupt the agent, drop the
// workspace) cannot see it.
type budgetUsageReply struct {
	// status is the HTTP status of the answer; zero when no answer arrived.
	status int
	// code is the error code the answer names, when it names one.
	code string
	// err says why there is no complete answer.
	err error
}

// budgetUsageVerdict is what an answer means for the record.
type budgetUsageVerdict int

const (
	// budgetUsageKept: the record stays as it is and is sent again later.
	budgetUsageKept budgetUsageVerdict = iota
	// budgetUsageDelivered: the server took the batch.
	budgetUsageDelivered
	// budgetUsageRefused: the server will never take this batch.
	budgetUsageRefused
)

// verdict classifies the answer. Any 2xx means delivered. 400, 404, 409, 413
// and 422 are final. Everything else keeps the record: 401, 403, 408, 425,
// 429, every 5xx, every 3xx (a redirect is not followed), a network error and
// an answer that could not be read to its end.
func (r budgetUsageReply) verdict() budgetUsageVerdict {
	if r.err != nil {
		return budgetUsageKept
	}
	if r.status >= 200 && r.status <= 299 {
		return budgetUsageDelivered
	}
	switch r.status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict,
		http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return budgetUsageRefused
	}
	return budgetUsageKept
}

// budgetUsageErrorCode returns the error code an answer names: its "code",
// and without one its "error". Either is returned only when it has the form
// of a code; a sentence is text of the answer, and no text of an answer is
// logged.
func budgetUsageErrorCode(body []byte) string {
	var answer struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return ""
	}
	code := answer.Code
	if code == "" {
		code = answer.Error
	}
	if !budgetUsageErrorCodePattern.MatchString(code) {
		return ""
	}
	return code
}

// setBudgetUsageReady makes the client declare the budget usage protocol,
// and with it every workspace client of a managed installation.
func (c *Client) setBudgetUsageReady() {
	// The installation's client comes first: a workspace client that is being
	// made right now copies the flag under the registry lock, and one that was
	// made before is reached by the loop below.
	c.budgetUsageReady.Store(true)
	if c.managed == nil {
		return
	}
	c.managed.mu.Lock()
	defer c.managed.mu.Unlock()
	for _, workspace := range c.managed.workspaces {
		workspace.client.budgetUsageReady.Store(true)
	}
}

// budgetUsageTransport returns the client that reports the usage of an
// attempt claimed in the given workspace: the daemon's client, and in a
// managed installation the client of that workspace. A managed installation
// without a valid binding for the workspace has none.
func (c *Client) budgetUsageTransport(workspaceID string) (*Client, error) {
	if c.managed == nil {
		return c, nil
	}
	c.managed.mu.RLock()
	defer c.managed.mu.RUnlock()
	workspace, err := c.managed.workspaceLocked(workspaceID)
	if err != nil {
		return nil, err
	}
	return workspace.client, nil
}

// reportBudgetUsage posts the persisted bytes of a batch to the usage route
// of its attempt, with the credential and the identity headers of the
// client's other requests, and within the time the client gives every request.
// A redirect is not followed: the page it leads to would answer in place of
// the server.
func (c *Client) reportBudgetUsage(ctx context.Context, attemptID string, batch []byte) budgetUsageReply {
	path := "/api/daemon/budget/attempts/" + url.PathEscape(attemptID) + "/usage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(batch))
	if err != nil {
		return budgetUsageReply{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if token := c.Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	c.setIdentityHeaders(req)

	httpClient := *c.client
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := httpClient.Do(req)
	if err != nil {
		return budgetUsageReply{err: err}
	}
	defer resp.Body.Close()
	head, err := io.ReadAll(io.LimitReader(resp.Body, maxBudgetUsageReplyBytes))
	if err == nil {
		_, err = io.Copy(io.Discard, resp.Body)
	}
	if err != nil {
		// An answer that cannot be read to its end is no answer, whatever its
		// status line said.
		return budgetUsageReply{status: resp.StatusCode, err: fmt.Errorf("read the answer: %w", err)}
	}
	return budgetUsageReply{status: resp.StatusCode, code: budgetUsageErrorCode(head)}
}

// claim marks the record of an attempt as being handled. It fails while
// another goroutine holds the mark: the send that follows the seal and the
// resend loop never handle one record at the same time.
func (j *budgetUsageJournal) claim(attemptID string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, busy := j.inFlight[attemptID]; busy {
		return false
	}
	j.inFlight[attemptID] = struct{}{}
	return true
}

func (j *budgetUsageJournal) release(attemptID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.inFlight, attemptID)
}

// budgetUsageHeld is what the process of a journal knows about one attempt.
type budgetUsageHeld struct {
	// attempt is the record this process created, when it holds one.
	attempt *budgetUsageAttempt
	// settled: the record is done with and only its file is left.
	settled bool
}

// load says what there is of an attempt: what the journal's process knows
// about it and, when it knows nothing, what the record file holds. Both are
// looked at under one hold of the lock that begin keeps while it creates a
// record, so a record that this process is creating right now is never taken
// for one that was left behind.
func (j *budgetUsageJournal) load(attemptID string) (budgetUsageHeld, budgetUsageRecord, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, settled := j.settled[attemptID]
	held := budgetUsageHeld{attempt: j.attempts[attemptID], settled: settled}
	if held.attempt != nil || held.settled {
		return held, budgetUsageRecord{}, nil
	}
	record, err := j.read(attemptID)
	return held, record, err
}

// remove deletes the record of an attempt: the entry this process holds for
// it, and its file. When the file cannot be removed the attempt is remembered
// as settled, so that this process neither sends its batch again nor reports
// the failure twice; a later call removes the file once the directory allows
// it.
func (j *budgetUsageJournal) remove(attemptID string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.attempts, attemptID)
	err := os.Remove(j.recordPath(attemptID))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		delete(j.settled, attemptID)
		return nil
	}
	j.settled[attemptID] = struct{}{}
	return err
}

// attemptID returns the id of the attempt the record belongs to.
func (a *budgetUsageAttempt) attemptID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.record.AttemptID
}

// initializeBudgetUsage opens the journal of usage report records in a
// directory of its own under the state directory of the daemon's profile.
// With a usable journal the daemon declares the budget usage protocol.
// Without one it says so once and runs as it did before the protocol: no
// record, no capability.
func (d *Daemon) initializeBudgetUsage() {
	profileDirectory, err := cli.ProfileDir(d.cfg.Profile)
	if err != nil {
		d.logger.Warn("budget usage: the usage report journal is not usable, running without the budget usage protocol", "error", err)
		return
	}
	journal, err := openBudgetUsageJournal(filepath.Join(profileDirectory, budgetUsageJournalDirectoryName), d.cfg.ServerBaseURL)
	if err != nil {
		d.logger.Warn("budget usage: the usage report journal is not usable, running without the budget usage protocol", "error", err)
		return
	}
	d.budgetUsage = journal
	d.client.setBudgetUsageReady()
}

// finishBudgetUsageAttempt seals the record of an attempt once its run is
// over and sends the batch in the background. Sending never holds the task
// up and never changes what the task reports: a batch that is not delivered
// now stays in the journal for the resend loop. ctx is the context the task
// was handed out under, which outlives the run.
func (d *Daemon) finishBudgetUsageAttempt(ctx context.Context, attempt *budgetUsageAttempt) {
	if len(attempt.seal()) == 0 {
		// A task without a record has no attempt and so no batch.
		return
	}
	attemptID := attempt.attemptID()
	attempt.journal.background.Add(1)
	go func() {
		defer attempt.journal.background.Done()
		d.deliverBudgetUsage(ctx, attemptID)
	}()
}

// deliverBudgetUsage is the send that follows the seal. It does not apply
// retention itself. If a resend pass reaches an expired sealed record first,
// that pass may remove it before this send; the server retains the unknown hold.
func (d *Daemon) deliverBudgetUsage(ctx context.Context, attemptID string) {
	d.handleBudgetUsage(ctx, attemptID, false)
}

// handleBudgetUsage handles the record of one attempt. The send that follows
// the seal and the passes of the resend loop (fromPass) both come through
// here, and the mark on the record keeps them apart.
func (d *Daemon) handleBudgetUsage(ctx context.Context, attemptID string, fromPass bool) {
	journal := d.budgetUsage
	if journal == nil {
		return
	}
	if !journal.claim(attemptID) {
		return
	}
	defer journal.release(attemptID)

	held, left, err := journal.load(attemptID)
	switch {
	case held.attempt != nil:
		record := held.attempt.snapshot()
		switch {
		case len(record.Batch) == 0:
			// The attempt is still running. Its record belongs to the run,
			// whatever its age.
		case fromPass && journal.outlived(record.CreatedAt):
			d.dropOutlivedBudgetUsage(journal, record)
		default:
			d.sendBudgetUsage(ctx, journal, record)
		}
	case held.settled:
		// Only the file is left, and its removal failed before. Try again,
		// quietly: the failure was reported when it was new.
		_ = journal.remove(attemptID)
	case !fromPass:
		// The send that follows the seal handles what this process holds and
		// nothing else. What an earlier process left is for the resend loop.
	case err != nil:
		// The file is gone, or it cannot be read as a record.
		d.skipUnreadableBudgetUsage(journal, attemptID)
	default:
		d.recoverBudgetUsage(ctx, journal, left)
	}
}

// sendBudgetUsage sends the batch of a sealed record and acts on the answer.
// The record is one of this server: this process created it, or the resend
// loop checked where it belongs.
func (d *Daemon) sendBudgetUsage(ctx context.Context, journal *budgetUsageJournal, record budgetUsageRecord) {
	client, err := d.client.budgetUsageTransport(record.WorkspaceID)
	if err != nil {
		d.logger.Debug("budget usage: keeping a usage report whose workspace has no valid binding", "attempt_id", record.AttemptID, "error", err)
		return
	}
	reply := client.reportBudgetUsage(ctx, record.AttemptID, record.Batch)
	switch reply.verdict() {
	case budgetUsageDelivered:
		d.logger.Debug("budget usage: usage report delivered", "attempt_id", record.AttemptID, "status", reply.status)
	case budgetUsageRefused:
		// The status and the error code, never the answer itself.
		attrs := []any{"attempt_id", record.AttemptID, "status", reply.status}
		if reply.code != "" {
			attrs = append(attrs, "code", reply.code)
		}
		d.logger.Warn("budget usage: the server refused the usage report for good, dropping it", attrs...)
	default:
		d.logger.Debug("budget usage: usage report not delivered, keeping it for the next send",
			"attempt_id", record.AttemptID, "status", reply.status, "error", reply.err)
		return
	}
	d.removeBudgetUsageRecord(journal, record.AttemptID)
}

// removeBudgetUsageRecord deletes a record that is done with.
func (d *Daemon) removeBudgetUsageRecord(journal *budgetUsageJournal, attemptID string) {
	if err := journal.remove(attemptID); err != nil {
		d.logger.Warn("budget usage: could not remove the file of a usage report that is done with; this process will not send it again",
			"attempt_id", attemptID, "error", err)
	}
}
