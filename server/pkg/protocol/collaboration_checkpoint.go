package protocol

import (
	"errors"
	"strconv"
)

const (
	CollaborationCheckpointVersion      = 1
	CollaborationCheckpointMessageLimit = 16
	CollaborationCheckpointBodyLimit    = 64 << 10
	CollaborationCheckpointCountLimit   = 256
	CollaborationRecordingSealLimit     = 32
	CollaborationRecordingBodyLimit     = 512 << 10
	CollaborationCheckpointJSONLimit    = 6*CollaborationCheckpointBodyLimit + 64<<10
)

// Recording offsets are zero-based message counts, independent of event sequence.
// They remain continuous across provider retries within one run.
type CollaborationRecordingCheckpoint struct {
	Version       int                            `json:"version"`
	Identity      CollaborationRunIdentity       `json:"identity"`
	AttemptSeq    string                         `json:"attempt_seq"`
	AttemptNonce  string                         `json:"attempt_nonce"`
	StreamID      string                         `json:"stream_id"`
	Sequence      int                            `json:"sequence"`
	MessageOffset int                            `json:"message_offset"`
	Messages      []CollaborationRecordedMessage `json:"messages"`
}

type CollaborationCheckpointReceipt struct {
	Version         int                      `json:"version"`
	Identity        CollaborationRunIdentity `json:"identity"`
	StreamID        string                   `json:"stream_id"`
	Sequence        int                      `json:"sequence"`
	RetainedThrough int                      `json:"retained_through"`
	Accepted        bool                     `json:"accepted"`
}

// A seal is metadata only. ObservedThrough=nil explicitly leaves the tail unknown.
// A sender's acknowledged count never replaces the server's committed prefix.
type CollaborationRecordingSeal struct {
	Sequence            int                      `json:"sequence"`
	Version             int                      `json:"version"`
	Identity            CollaborationRunIdentity `json:"identity"`
	AttemptSeq          string                   `json:"attempt_seq"`
	AttemptNonce        string                   `json:"attempt_nonce"`
	StreamID            string                   `json:"stream_id"`
	ObservedThrough     *int                     `json:"observed_through"`
	AcknowledgedThrough int                      `json:"acknowledged_through"`
	Reason              string                   `json:"reason"`
}

type CollaborationRecordingRetention struct {
	Version         int    `json:"version"`
	StreamID        string `json:"stream_id"`
	RetainedThrough int    `json:"retained_through"`
	ObservedThrough *int   `json:"observed_through"`
	Sealed          bool   `json:"sealed"`
	Reason          string `json:"reason"`
}

func (v *CollaborationRecordingRetention) UnmarshalJSON(raw []byte) error {
	type wire CollaborationRecordingRetention
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "version", "stream_id", "retained_through", "sealed", "reason"); err != nil {
		return err
	}
	if !collaborationRecallHasFields(raw, "observed_through") {
		return errors.New("invalid collaboration recording retention")
	}
	if err := ValidateCollaborationRecordingRetention(CollaborationRecordingRetention(w)); err != nil {
		return err
	}
	*v = CollaborationRecordingRetention(w)
	return nil
}

func ValidateCollaborationRecordingRetention(v CollaborationRecordingRetention) error {
	invalid := errors.New("invalid collaboration recording retention")
	if v.Version != CollaborationCheckpointVersion || !preparationUUID(v.StreamID) || v.RetainedThrough < 0 || v.RetainedThrough > CollaborationRecordItemLimit ||
		v.ObservedThrough != nil && (*v.ObservedThrough < v.RetainedThrough || *v.ObservedThrough > CollaborationRecordItemLimit) ||
		!v.Sealed && (v.ObservedThrough != nil || v.Reason != "incomplete") ||
		!validCollaborationRecordingReason(v.Reason) || v.Reason == "completed" && v.ObservedThrough == nil {
		return invalid
	}
	return nil
}

func validCollaborationRecordingReason(reason string) bool {
	switch reason {
	case "completed", "stopped", "authority_lost", "transport_error", "recording_limit", "incomplete":
		return true
	default:
		return false
	}
}

func validCheckpointAttempt(identity CollaborationRunIdentity, seq, nonce, stream string) bool {
	n, err := strconv.Atoi(seq)
	return preparationIdentity(identity) && err == nil && n > 0 && n <= CollaborationHostMaxLaunches && strconv.Itoa(n) == seq && preparationUUID(nonce) && preparationUUID(stream)
}

func ValidateCollaborationRecordingCheckpoint(p CollaborationRecordingCheckpoint) error {
	invalid := errors.New("invalid collaboration recording checkpoint")
	if p.Version != CollaborationCheckpointVersion || !validCheckpointAttempt(p.Identity, p.AttemptSeq, p.AttemptNonce, p.StreamID) || p.Sequence < 1 || p.Sequence > CollaborationCheckpointCountLimit || p.MessageOffset < 0 || len(p.Messages) < 1 || len(p.Messages) > CollaborationCheckpointMessageLimit || p.MessageOffset+len(p.Messages) > CollaborationRecordItemLimit {
		return invalid
	}
	if ValidateCollaborationRunRecord(&CollaborationRunRecord{Version: 1, Completeness: "partial", Messages: p.Messages}) != nil {
		return invalid
	}
	total := 0
	for _, m := range p.Messages {
		total += len(m.Body)
	}
	if total > CollaborationCheckpointBodyLimit {
		return invalid
	}
	return nil
}

func ValidateCollaborationRecordingSeal(p CollaborationRecordingSeal) error {
	invalid := errors.New("invalid collaboration recording seal")
	if p.Version != CollaborationCheckpointVersion || p.Sequence < 1 || p.Sequence > CollaborationRecordingSealLimit || !validCheckpointAttempt(p.Identity, p.AttemptSeq, p.AttemptNonce, p.StreamID) || p.AcknowledgedThrough < 0 || p.AcknowledgedThrough > CollaborationRecordItemLimit || p.ObservedThrough != nil && (*p.ObservedThrough < p.AcknowledgedThrough || *p.ObservedThrough > CollaborationRecordItemLimit || *p.ObservedThrough < 0) {
		return invalid
	}
	if !validCollaborationRecordingReason(p.Reason) {
		return invalid
	}
	if p.Reason == "completed" && p.ObservedThrough == nil {
		return invalid
	}
	return nil
}

// ValidateCollaborationRecordingSealAdvance validates the next immutable seal.
// Exact original-key replay is checked separately by the caller; a new key may
// only add knowledge about the same run/stream, never remove a known tail bound.
func ValidateCollaborationRecordingSealAdvance(prior *CollaborationRecordingSeal, next CollaborationRecordingSeal) error {
	invalid := errors.New("recording seal metadata must advance monotonically")
	if ValidateCollaborationRecordingSeal(next) != nil {
		return invalid
	}
	if prior == nil {
		if next.Sequence != 1 {
			return invalid
		}
		return nil
	}
	if ValidateCollaborationRecordingSeal(*prior) != nil || next.Sequence != prior.Sequence+1 || next.Identity != prior.Identity || next.StreamID != prior.StreamID || next.Reason != prior.Reason || next.AcknowledgedThrough < prior.AcknowledgedThrough {
		return invalid
	}
	if prior.ObservedThrough != nil && (next.ObservedThrough == nil || *next.ObservedThrough < *prior.ObservedThrough) {
		return invalid
	}
	return nil
}
