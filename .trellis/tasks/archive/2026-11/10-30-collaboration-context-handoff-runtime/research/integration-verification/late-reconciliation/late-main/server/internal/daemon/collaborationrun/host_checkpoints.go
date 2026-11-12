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
	ctx        context.Context
	messages   chan protocol.CollaborationRecordedMessage
	done       chan struct{}
	once       sync.Once
	closed     atomic.Bool
	failed     atomic.Bool
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
	sender := &hostCheckpointSender{supervisor: s, ctx: ctx, messages: make(chan protocol.CollaborationRecordedMessage, 32), done: make(chan struct{})}
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
}

func (s *hostCheckpointSender) acknowledged() int {
	if s == nil {
		return 0
	}
	s.supervisor.mu.Lock()
	defer s.supervisor.mu.Unlock()
	return hostRecordingAcknowledged(s.supervisor.local.record.Host.Recording)
}

func (s *hostCheckpointSender) run() {
	defer close(s.done)
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	batch := []protocol.CollaborationRecordedMessage{}
	bytes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
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
		if !found {
			supervisor.mu.Unlock()
			s.failed.Store(true)
			s.closed.Store(true)
			batch = nil
			bytes = 0
			return
		}
		p := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: local.record.Identity, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, StreamID: rec.StreamID, Sequence: len(rec.Chunks) + 1, MessageOffset: offset, Messages: batch}
		token := local.record.Host.BusinessToken
		err := local.stageHostCheckpoint(p, token)
		supervisor.mu.Unlock()
		batch = []protocol.CollaborationRecordedMessage{}
		bytes = 0
		if err != nil {
			s.failed.Store(true)
			s.closed.Store(true)
			return
		}
		if s.failed.Load() {
			return
		}
		requestCtx, cancel := context.WithTimeout(s.ctx, time.Second)
		_, err = supervisor.runner.Client.Checkpoint(requestCtx, p, token)
		cancel()
		if err == nil || recordingDefinitiveRejection(err) {
			supervisor.mu.Lock()
			persistErr := local.acknowledgeHostCheckpoint(p.Sequence, err != nil)
			supervisor.mu.Unlock()
			if err != nil || persistErr != nil {
				s.failed.Store(true)
				s.closed.Store(true)
			}
		} else {
			s.failed.Store(true)
		}
	}
	for {
		select {
		case m, ok := <-s.messages:
			if !ok {
				flush()
				if s.closed.Load() {
					s.supervisor.mu.Lock()
					_ = s.supervisor.local.closeHostRecording("recording_limit")
					s.supervisor.mu.Unlock()
				}
				return
			}
			if len(batch) > 0 && (len(batch) >= protocol.CollaborationCheckpointMessageLimit || bytes+len(m.Body) > protocol.CollaborationCheckpointBodyLimit) {
				flush()
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

// Pending bodies keep their original issued token/key. A failed recording
// request never prevents independent managed-termination proof recovery.
func (r *Runner) recoverHostRecording(ctx context.Context, local *RunLedger) error {
	if local.record.Host.Recording == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rejected := false
	for i := 0; i < len(local.record.Host.Recording.Chunks); i++ {
		c := local.record.Host.Recording.Chunks[i]
		if c.Rejected {
			rejected = true
			continue
		}
		if c.Accepted {
			continue
		}
		if rejected {
			if err := local.acknowledgeHostCheckpoint(c.Sequence, true); err != nil {
				return err
			}
			continue
		}
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		_, err := r.Client.Checkpoint(requestCtx, *c.Payload, c.BusinessToken)
		cancel()
		if err != nil && !recordingDefinitiveRejection(err) {
			return err
		}
		if persistErr := local.acknowledgeHostCheckpoint(c.Sequence, err != nil); persistErr != nil {
			return persistErr
		}
		if err != nil {
			rejected = true
		}
	}
	return nil
}

func (r *Runner) sealHostRecording(ctx context.Context, local *RunLedger) error {
	rec := local.record.Host.Recording
	if rec == nil {
		return nil
	}
	if rec.SealAccepted {
		if !hostRecordingSealAdvancePending(rec) {
			return nil
		}
		next, err := local.recordingNext()
		if err != nil {
			return err
		}
		seal := *rec.Seal
		if seal.Sequence >= protocol.CollaborationRecordingSealLimit {
			return ErrUnconfirmed
		}
		seal.Sequence++
		seal.AcknowledgedThrough = hostRecordingAcknowledged(rec)
		next.Host.Recording.Seal = &seal
		next.Host.Recording.SealAccepted = false
		if err := local.persistHostRecording(next); err != nil {
			return err
		}
		rec = local.record.Host.Recording
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
		if !rec.Closed {
			count = &observed
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
	requestCtx, cancel := context.WithTimeout(ctx, time.Second)
	_, err := r.Client.RecordingSeal(requestCtx, *local.record.Host.Recording.Seal, local.record.TerminationToken)
	cancel()
	if err != nil {
		return err
	}
	next, err := local.recordingNext()
	if err != nil {
		return err
	}
	next.Host.Recording.SealAccepted = true
	return local.persistHostRecording(next)
}

func lastAuthorizedRecordingLaunch(h *hostLedgerRecord) (hostLaunchRecord, bool) {
	for i := len(h.Launches) - 1; i >= 0; i-- {
		if h.Launches[i].Authorized != nil && *h.Launches[i].Authorized {
			return h.Launches[i], true
		}
	}
	return hostLaunchRecord{}, false
}
