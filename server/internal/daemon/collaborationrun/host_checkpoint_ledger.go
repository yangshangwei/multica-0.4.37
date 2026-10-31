package collaborationrun

import (
	"strconv"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type hostCheckpointRecord struct {
	Sequence      int                                        `json:"sequence"`
	Offset        int                                        `json:"offset"`
	Count         int                                        `json:"count"`
	BodyBytes     int                                        `json:"body_bytes"`
	Payload       *protocol.CollaborationRecordingCheckpoint `json:"payload,omitempty"`
	BusinessToken string                                     `json:"business_token,omitempty"`
	Accepted      bool                                       `json:"accepted,omitempty"`
	Rejected      bool                                       `json:"rejected,omitempty"`
}

type hostRecordingLedger struct {
	StreamID     string                               `json:"stream_id"`
	Chunks       []hostCheckpointRecord               `json:"chunks"`
	Closed       bool                                 `json:"closed,omitempty"`
	Drained      bool                                 `json:"drained,omitempty"`
	Reason       string                               `json:"reason,omitempty"`
	Seal         *protocol.CollaborationRecordingSeal `json:"seal,omitempty"`
	SealAccepted bool                                 `json:"seal_accepted,omitempty"`
}

func validateHostRecordingLedger(r ledgerRecord) error {
	recording := r.Host.Recording
	if recording == nil {
		return nil
	}
	if !validUUID(recording.StreamID) || recording.Chunks == nil || len(recording.Chunks) > protocol.CollaborationCheckpointCountLimit {
		return ErrUnconfirmed
	}
	offset, total := 0, 0
	for i, c := range recording.Chunks {
		if c.Sequence != i+1 || c.Offset != offset || c.Count < 1 || c.Count > protocol.CollaborationCheckpointMessageLimit || c.BodyBytes < 0 || c.BodyBytes > protocol.CollaborationCheckpointBodyLimit || c.Accepted && c.Rejected {
			return ErrUnconfirmed
		}
		if c.Accepted || c.Rejected {
			if c.Payload != nil || c.BusinessToken != "" {
				return ErrUnconfirmed
			}
		} else {
			if c.Payload == nil || protocol.ValidateCollaborationRecordingCheckpoint(*c.Payload) != nil || c.Payload.Identity != r.Identity || c.Payload.StreamID != recording.StreamID || c.Payload.Sequence != c.Sequence || c.Payload.MessageOffset != offset || len(c.Payload.Messages) != c.Count || !strings.HasPrefix(c.BusinessToken, "mcb_") || len(c.BusinessToken) != 68 {
				return ErrUnconfirmed
			}
			attempt, _ := strconv.Atoi(c.Payload.AttemptSeq)
			if attempt < 1 || attempt > len(r.Host.Launches) || r.Host.Launches[attempt-1].AttemptNonce != c.Payload.AttemptNonce || r.Host.Launches[attempt-1].Authorized == nil || !*r.Host.Launches[attempt-1].Authorized {
				return ErrUnconfirmed
			}
			bytes := 0
			for _, m := range c.Payload.Messages {
				bytes += len(m.Body)
			}
			if bytes != c.BodyBytes {
				return ErrUnconfirmed
			}
		}
		offset += c.Count
		total += c.BodyBytes
	}
	if offset > protocol.CollaborationRecordItemLimit || total > protocol.CollaborationRecordingBodyLimit {
		return ErrUnconfirmed
	}
	if recording.SealAccepted && recording.Seal == nil {
		return ErrUnconfirmed
	}
	if seal := recording.Seal; seal != nil {
		if protocol.ValidateCollaborationRecordingSeal(*seal) != nil || seal.Identity != r.Identity || seal.StreamID != recording.StreamID || seal.AcknowledgedThrough > hostRecordingAcknowledged(recording) {
			return ErrUnconfirmed
		}
		attempt, _ := strconv.Atoi(seal.AttemptSeq)
		if attempt < 1 || attempt > len(r.Host.Launches) {
			return ErrUnconfirmed
		}
		launch := r.Host.Launches[attempt-1]
		if launch.AttemptNonce != seal.AttemptNonce || launch.Authorized == nil || !*launch.Authorized {
			return ErrUnconfirmed
		}
	}
	return nil
}

func (r *RunLedger) recordingNext() (ledgerRecord, error) {
	if r.failed || r.lock == nil || r.record == nil || r.record.Version != 2 || r.record.Host == nil {
		return ledgerRecord{}, ErrUnconfirmed
	}
	next := *r.record
	host := *r.record.Host
	next.Host = &host
	if host.Recording != nil {
		recording := *host.Recording
		recording.Chunks = append([]hostCheckpointRecord{}, recording.Chunks...)
		host.Recording = &recording
	}
	return next, nil
}

func hostRecordingPending(h *hostLedgerRecord) bool {
	if h.Recording == nil {
		return false
	}
	if !h.Recording.SealAccepted {
		return true
	}
	return hostRecordingChunksPending(h.Recording) || h.Recording.Seal.AcknowledgedThrough < hostRecordingAcknowledged(h.Recording)
}

func hostRecordingChunksPending(recording *hostRecordingLedger) bool {
	for _, c := range recording.Chunks {
		if !c.Accepted && !c.Rejected {
			return true
		}
	}
	return false
}

func hostRecordingAcknowledged(h *hostRecordingLedger) int {
	if h == nil {
		return 0
	}
	through := 0
	for _, c := range h.Chunks {
		if !c.Accepted {
			break
		}
		through = c.Offset + c.Count
	}
	return through
}

func (r *RunLedger) startHostRecording(stream string) error {
	next, err := r.recordingNext()
	if err != nil {
		return err
	}
	if next.Host.Recording != nil || !validUUID(stream) {
		return ErrUnconfirmed
	}
	next.Host.Recording = &hostRecordingLedger{StreamID: stream, Chunks: []hostCheckpointRecord{}}
	return r.persistHostRecording(next)
}

func (r *RunLedger) stageHostCheckpoint(p protocol.CollaborationRecordingCheckpoint, token string) error {
	next, err := r.recordingNext()
	if err != nil {
		return err
	}
	recording := next.Host.Recording
	if recording == nil || recording.Drained || recording.Seal != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil || p.Identity != next.Identity || p.StreamID != recording.StreamID || p.Sequence != len(recording.Chunks)+1 {
		return ErrUnconfirmed
	}
	offset := 0
	if len(recording.Chunks) > 0 {
		last := recording.Chunks[len(recording.Chunks)-1]
		offset = last.Offset + last.Count
	}
	if p.MessageOffset != offset {
		return ErrUnconfirmed
	}
	bytes := 0
	for _, m := range p.Messages {
		bytes += len(m.Body)
	}
	recording.Chunks = append(recording.Chunks, hostCheckpointRecord{Sequence: p.Sequence, Offset: p.MessageOffset, Count: len(p.Messages), BodyBytes: bytes, Payload: &p, BusinessToken: token})
	return r.persistHostRecording(next)
}

func (r *RunLedger) acknowledgeHostCheckpoint(sequence int, rejected bool) error {
	next, err := r.recordingNext()
	if err != nil {
		return err
	}
	rec := next.Host.Recording
	if rec == nil || sequence < 1 || sequence > len(rec.Chunks) {
		return ErrUnconfirmed
	}
	c := &rec.Chunks[sequence-1]
	if c.Accepted || c.Rejected {
		if c.Rejected == rejected {
			return nil
		}
		return ErrUnconfirmed
	}
	c.Accepted, c.Rejected, c.Payload, c.BusinessToken = !rejected, rejected, nil, ""
	if rejected {
		rec.Closed = true
		rec.Reason = "authority_lost"
	}
	return r.persistHostRecording(next)
}

func (r *RunLedger) finishHostRecording(reason string) error {
	next, err := r.recordingNext()
	if err != nil {
		return err
	}
	if next.Host.Recording == nil {
		return nil
	}
	next.Host.Recording.Drained = true
	if reason != "" {
		next.Host.Recording.Closed = true
		if next.Host.Recording.Reason == "" {
			next.Host.Recording.Reason = reason
		}
	}
	return r.persistHostRecording(next)
}

func (r *RunLedger) persistHostRecording(next ledgerRecord) error {
	if err := validateHostLedgerRecord(next); err != nil {
		return err
	}
	return r.persist(next)
}
