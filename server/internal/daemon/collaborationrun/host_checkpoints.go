package collaborationrun

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A single sender owns bounded batching; provider drains never wait on HTTP.
type hostCheckpointSender struct {
	supervisor *hostSupervisor
	record     *hostRecording
	ctx        context.Context
	messages   chan protocol.CollaborationRecordedMessage
	done       chan struct{}
	once       sync.Once
	closed     atomic.Bool
	// failed means recorded content was lost: a batch could not be staged, the
	// ledger failed, or the server refused a chunk. A transient upload error is
	// not a loss; the staged payload is retried later with its original key.
	failed atomic.Bool
	// lost means a local staging failure dropped a batch.
	lost atomic.Bool
}

func startHostCheckpoints(ctx context.Context, s *hostSupervisor, record *hostRecording, version int) (*hostCheckpointSender, error) {
	if version == 0 {
		return nil, nil
	}
	if version != protocol.CollaborationCheckpointVersion {
		return nil, ErrProtocol
	}
	stream := uuid.NewString()
	s.mu.Lock()
	err := s.local.startHostRecording(stream)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	sender := &hostCheckpointSender{supervisor: s, record: record, ctx: ctx, messages: make(chan protocol.CollaborationRecordedMessage, 32), done: make(chan struct{})}
	record.checkpoints = sender
	record.streamID = stream
	go sender.run()
	return sender, nil
}

func (s *hostCheckpointSender) offer(m protocol.CollaborationRecordedMessage) bool {
	if s == nil {
		return true
	}
	if s.closed.Load() {
		return false
	}
	select {
	case s.messages <- m:
		return true
	default:
		s.closed.Store(true)
		return false
	}
}

func (s *hostCheckpointSender) finish() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.messages) })
	<-s.done
	// Only the caller that finished draining Provider output knows the tail.
	// A restarted daemon cannot infer that fact from the last stored chunk.
	reason := ""
	if s.lost.Load() {
		// A batch that failed local staging is missing data, not a limit.
		reason = "incomplete"
	} else if s.closed.Load() {
		reason = "recording_limit"
	} else if s.record.partial {
		reason = "incomplete"
	}
	s.supervisor.mu.Lock()
	err := s.supervisor.local.finishHostRecording(reason)
	s.supervisor.mu.Unlock()
	if err != nil {
		s.failed.Store(true)
	}
}

func (s *hostCheckpointSender) acknowledged() int {
	if s == nil {
		return 0
	}
	s.supervisor.mu.Lock()
	defer s.supervisor.mu.Unlock()
	return hostRecordingAcknowledged(s.supervisor.local.record.Host.Recording)
}

// healthy reports that no recorded content was lost: every observed message
// was staged and no chunk was refused.
func (s *hostCheckpointSender) healthy() bool {
	if s == nil || s.failed.Load() || s.closed.Load() {
		return false
	}
	s.supervisor.mu.Lock()
	defer s.supervisor.mu.Unlock()
	rec := s.supervisor.local.record.Host.Recording
	return rec != nil && !rec.Closed
}

func (s *hostCheckpointSender) run() {
	defer close(s.done)
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	batch := []protocol.CollaborationRecordedMessage{}
	bytes := 0
	// A local staging failure loses this batch. Later queued messages cannot
	// reuse its offsets; HTTP uncertainty still has a durable original payload.
	stagingFailed := false
	stage := func() {
		supervisor := s.supervisor
		supervisor.mu.Lock()
		local := supervisor.local
		rec := local.record.Host.Recording
		offset := 0
		if len(rec.Chunks) > 0 {
			last := rec.Chunks[len(rec.Chunks)-1]
			offset = last.Offset + last.Count
		}
		launch, found := lastAuthorizedRecordingLaunch(local.record.Host)
		err := ErrUnconfirmed
		if found {
			p := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: local.record.Identity, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, StreamID: rec.StreamID, Sequence: len(rec.Chunks) + 1, MessageOffset: offset, Messages: batch}
			err = local.stageHostCheckpoint(p, local.record.Host.BusinessToken)
		}
		supervisor.mu.Unlock()
		batch = []protocol.CollaborationRecordedMessage{}
		bytes = 0
		if err != nil {
			stagingFailed = true
			s.lost.Store(true)
			s.failed.Store(true)
			s.closed.Store(true)
		}
	}
	// Every flush, including the 5-second tick and the final drain, first
	// stages the new batch and then retries the oldest unacknowledged chunk
	// with its original key, so one transient failure cannot end uploads.
	flush := func() {
		if !stagingFailed && len(batch) > 0 {
			stage()
		}
		state, _ := deliverHostCheckpoints(s.ctx, s.supervisor.runner.Client, s.supervisor.local, &s.supervisor.mu, false, time.Second)
		if state == hostDeliveryClosed {
			s.failed.Store(true)
			s.closed.Store(true)
		}
	}
	for {
		select {
		case m, ok := <-s.messages:
			if !ok {
				flush()
				return
			}
			if len(batch) > 0 && (len(batch) >= protocol.CollaborationCheckpointMessageLimit || bytes+len(m.Body) > protocol.CollaborationCheckpointBodyLimit) {
				flush()
			}
			if stagingFailed {
				continue
			}
			batch = append(batch, m)
			bytes += len(m.Body)
			if len(batch) >= protocol.CollaborationCheckpointMessageLimit || bytes >= protocol.CollaborationCheckpointBodyLimit {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}

func recordingDefinitiveRejection(err error) bool {
	var refusal *HTTPError
	return errors.As(err, &refusal) && refusal.Status >= 400 && refusal.Status < 500 && refusal.Status != http.StatusRequestTimeout && refusal.Status != http.StatusTooManyRequests
}

func hostRecordingRecoveryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := time.Second
	if deadline, bounded := ctx.Deadline(); bounded {
		// Leave room for outcome and the independent final proof. This is one
		// budget shared by all chunks and seal retries, never one per request.
		timeout = min(timeout, time.Until(deadline)/4)
	}
	return context.WithTimeout(ctx, timeout)
}

// Pending bodies keep their original issued token/key. A failed recording
// request never prevents independent managed-termination proof recovery.
func (r *Runner) recoverHostRecording(ctx context.Context, local *RunLedger) error {
	if local.record.Host.Recording == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := deliverHostCheckpoints(ctx, r.Client, local, nil, true, 0)
	return err
}

type hostDelivery int

const (
	// hostDeliveryDone: no chunk is waiting to be sent.
	hostDeliveryDone hostDelivery = iota
	// hostDeliveryPending: a chunk stays unacknowledged with its original
	// payload, after a transient failure or behind a pending outcome.
	hostDeliveryPending
	// hostDeliveryClosed: a chunk was refused or the ledger failed.
	hostDeliveryClosed
)

// deliverHostCheckpoints sends unacknowledged chunks oldest first, one request
// at a time. mu guards the ledger while the sender runs; recovery passes nil.
// cascade resolves every chunk after a refused one as refused, which recovery
// needs before sealing; the in-run sender stops at the refused chunk.
func deliverHostCheckpoints(ctx context.Context, client *Client, local *RunLedger, mu *sync.Mutex, cascade bool, requestTimeout time.Duration) (hostDelivery, error) {
	lock := func() {
		if mu != nil {
			mu.Lock()
		}
	}
	unlock := func() {
		if mu != nil {
			mu.Unlock()
		}
	}
	rejected := false
	for i := 0; ; i++ {
		lock()
		if local.failed {
			unlock()
			return hostDeliveryClosed, ErrUnconfirmed
		}
		chunks := local.record.Host.Recording.Chunks
		if i >= len(chunks) {
			unlock()
			if rejected {
				return hostDeliveryClosed, nil
			}
			return hostDeliveryDone, nil
		}
		c := chunks[i]
		unlock()
		if c.Accepted {
			continue
		}
		if c.Rejected {
			if !cascade {
				return hostDeliveryClosed, nil
			}
			rejected = true
			continue
		}
		if err := ctx.Err(); err != nil {
			return hostDeliveryPending, err
		}
		if rejected {
			lock()
			err := local.acknowledgeHostCheckpoint(c.Sequence, true)
			unlock()
			if err != nil {
				return hostDeliveryClosed, err
			}
			continue
		}
		requestCtx, cancel := ctx, context.CancelFunc(func() {})
		if requestTimeout > 0 {
			requestCtx, cancel = context.WithTimeout(ctx, requestTimeout)
		}
		_, err := client.Checkpoint(requestCtx, *c.Payload, c.BusinessToken)
		cancel()
		if err != nil && !recordingDefinitiveRejection(err) {
			return hostDeliveryPending, err
		}
		lock()
		var persistErr error
		switch {
		case err == nil:
			persistErr = local.acknowledgeHostCheckpoint(c.Sequence, false)
		case supersededCheckpoint(err):
			var rebound bool
			rebound, persistErr = local.rebindSupersededCheckpoint(c.Sequence)
			if rebound && persistErr == nil {
				unlock()
				i--
				continue
			}
			if persistErr == nil {
				persistErr = local.rejectHostCheckpoint(c.Sequence, err)
			}
		case outcomeCoversRefusedCheckpoint(local.record.Host, c, err):
			unlock()
			return hostDeliveryPending, nil
		default:
			persistErr = local.rejectHostCheckpoint(c.Sequence, err)
		}
		unlock()
		if persistErr != nil {
			return hostDeliveryClosed, persistErr
		}
		if err != nil {
			if !cascade {
				return hostDeliveryClosed, nil
			}
			rejected = true
		}
	}
}

func refusalCode(err error) string {
	var refusal *HTTPError
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return ""
}

// The service refuses a key that no longer belongs to the authorized attempt
// before it looks for an already committed copy under its run lock, so this
// refusal proves the original key was never committed.
func supersededCheckpoint(err error) bool {
	code := refusalCode(err)
	return code == "business_capability_required" || code == "stale_execution"
}

// After a lost outcome ACK the service already holds the outcome and refuses
// later checkpoints as out of order. If the pending outcome's tail covers the
// chunk, the chunk waits for that outcome receipt instead of being refused.
func outcomeCoversRefusedCheckpoint(h *hostLedgerRecord, c hostCheckpointRecord, err error) bool {
	return refusalCode(err) == "checkpoint_out_of_order" && hostOutcomePending(h) && hostOutcomeCovers(h.Outcome, c)
}

func hostOutcomeCovers(outcome *protocol.CollaborationRunOutcome, c hostCheckpointRecord) bool {
	record := outcome.Record
	return record != nil && record.Version == 2 && c.Offset >= record.MessageOffset && c.Offset+c.Count <= record.MessageOffset+len(record.Messages)
}

// hostSealReason names why the service refused new recorded content.
func hostSealReason(err error) string {
	var refusal *HTTPError
	if errors.As(err, &refusal) {
		switch refusal.Code {
		case "execution_stopped":
			return "stopped"
		case "checkpoint_conflict", "checkpoint_out_of_order", "invalid_recording_checkpoint":
			return "incomplete"
		}
		if refusal.Status == http.StatusBadRequest || refusal.Status == http.StatusRequestEntityTooLarge {
			return "incomplete"
		}
	}
	return "authority_lost"
}

func (r *Runner) sealHostRecording(ctx context.Context, local *RunLedger) error {
	rec := local.record.Host.Recording
	if rec == nil {
		return nil
	}
	if rec.Seal == nil {
		next, err := local.recordingNext()
		if err != nil {
			return err
		}
		rec = next.Host.Recording
		reason := rec.Reason
		observed := 0
		if len(rec.Chunks) > 0 {
			last := rec.Chunks[len(rec.Chunks)-1]
			observed = last.Offset + last.Count
		}
		through := hostRecordingAcknowledged(rec)
		if reason == "" {
			reason = "completed"
			if through != observed {
				reason = "transport_error"
			}
			if next.Host.Outcome == nil || next.Host.OutcomeRejected || next.Host.Outcome.Record != nil && next.Host.Outcome.Record.Completeness == "partial" {
				reason = "incomplete"
			}
		}
		var count *int
		if rec.Drained && !rec.Closed {
			count = &observed
		} else if reason == "completed" {
			reason = "incomplete"
		}
		launch, found := lastAuthorizedRecordingLaunch(next.Host)
		if !found {
			if len(rec.Chunks) != 0 {
				return ErrUnconfirmed
			}
			next.Host.Recording = nil
			return local.persistHostRecording(next)
		}
		rec.Seal = &protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: next.Identity, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, StreamID: rec.StreamID, ObservedThrough: count, AcknowledgedThrough: through, Reason: reason}
		if err := local.persistHostRecording(next); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for {
		rec = local.record.Host.Recording
		if rec.SealAccepted {
			through := hostRecordingAcknowledged(rec)
			// Coalesce receipt progress into one final update. Incrementing for
			// every recovered chunk could exhaust the bounded seal sequence.
			if hostRecordingChunksPending(rec) || rec.Seal.AcknowledgedThrough == through {
				return nil
			}
			next, err := local.recordingNext()
			if err != nil {
				return err
			}
			seal := *rec.Seal
			seal.Sequence++
			seal.AcknowledgedThrough = through
			if protocol.ValidateCollaborationRecordingSealAdvance(rec.Seal, seal) != nil {
				return ErrUnconfirmed
			}
			next.Host.Recording.Seal, next.Host.Recording.SealAccepted = &seal, false
			if err := local.persistHostRecording(next); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// An unacknowledged seal always replays its persisted identity and
		// payload before a new sequence can describe later receipt progress.
		_, err := r.Client.RecordingSeal(ctx, *local.record.Host.Recording.Seal, local.record.TerminationToken)
		if err != nil {
			return err
		}
		next, err := local.recordingNext()
		if err != nil {
			return err
		}
		next.Host.Recording.SealAccepted = true
		if err := local.persistHostRecording(next); err != nil {
			return err
		}
	}
}

func lastAuthorizedRecordingLaunch(h *hostLedgerRecord) (hostLaunchRecord, bool) {
	for i := len(h.Launches) - 1; i >= 0; i-- {
		if h.Launches[i].Authorized != nil && *h.Launches[i].Authorized {
			return h.Launches[i], true
		}
	}
	return hostLaunchRecord{}, false
}
