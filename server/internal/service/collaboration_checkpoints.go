package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type collaborationRecordingState struct {
	stream            string
	sequence          int
	messages          []protocol.CollaborationRecordedMessage
	bytes             int
	checkpointThrough int
	seal              *protocol.CollaborationRecordingSeal
	outcome           bool
}

func (s *collaborationRecordingState) retain(stream string, offset int, messages []protocol.CollaborationRecordedMessage) error {
	invalid := invalidCollaborationLedger("recording stream is not contiguous")
	if s.stream != "" && s.stream != stream || offset > len(s.messages) || offset < 0 {
		return invalid
	}
	s.stream = stream
	for i, message := range messages {
		n := offset + i
		if n < len(s.messages) {
			if !reflect.DeepEqual(s.messages[n], message) {
				return invalid
			}
			continue
		}
		s.bytes += len(message.Body)
		s.messages = append(s.messages, message)
	}
	if s.bytes > protocol.CollaborationRecordingBodyLimit || len(s.messages) > protocol.CollaborationRecordItemLimit {
		return invalid
	}
	return nil
}

func readCollaborationRecording(ctx context.Context, q *db.Queries, issue, run pgtype.UUID, through int64) (collaborationRecordingState, error) {
	state := collaborationRecordingState{}
	rows, err := q.ListCollaborationRecordingEvents(ctx, db.ListCollaborationRecordingEventsParams{IssueID: issue, RunID: run, ThroughSeq: through})
	if err != nil {
		return state, err
	}
	for _, event := range rows {
		switch event.Kind {
		case "record_checkpoint":
			var p protocol.CollaborationRecordingCheckpoint
			if decodeCollaborationPreparationJSON(event.Payload, &p) != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil || p.Identity.RunID != util.UUIDToString(run) || p.Identity.IssueID != util.UUIDToString(issue) || state.seal != nil || state.outcome || p.Sequence != state.sequence+1 || p.MessageOffset != len(state.messages) {
				return state, invalidCollaborationLedger("invalid retained checkpoint")
			}
			if err := state.retain(p.StreamID, p.MessageOffset, p.Messages); err != nil {
				return state, err
			}
			state.sequence = p.Sequence
			state.checkpointThrough = len(state.messages)
		case "recording_seal":
			var p protocol.CollaborationRecordingSeal
			if decodeCollaborationPreparationJSON(event.Payload, &p) != nil || protocol.ValidateCollaborationRecordingSeal(p) != nil || protocol.ValidateCollaborationRecordingSealAdvance(state.seal, p) != nil || p.Identity.RunID != util.UUIDToString(run) || p.Identity.IssueID != util.UUIDToString(issue) || state.stream != "" && state.stream != p.StreamID {
				return state, invalidCollaborationLedger("invalid recording seal")
			}
			state.stream = p.StreamID
			state.seal = &p
		case "outcome":
			var p protocol.CollaborationRunOutcome
			if state.outcome || decodeCollaborationPreparationJSON(event.Payload, &p) != nil || p.Identity.RunID != util.UUIDToString(run) || p.Identity.IssueID != util.UUIDToString(issue) || protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: p.Outcome, Body: p.Body, Record: p.Record}) != nil {
				return state, invalidCollaborationLedger("invalid recording outcome")
			}
			if p.Record != nil && p.Record.Version == 2 {
				if state.seal != nil && p.Record.MessageOffset+len(p.Record.Messages) > len(state.messages) {
					return state, invalidCollaborationLedger("recording suffix follows seal")
				}
				if err := state.retain(p.Record.StreamID, p.Record.MessageOffset, p.Record.Messages); err != nil {
					return state, err
				}
			} else if state.stream != "" {
				return state, invalidCollaborationLedger("legacy outcome conflicts with recording stream")
			}
			state.outcome = true
		}
	}
	return state, nil
}

func (s collaborationRecordingState) retention() *protocol.CollaborationRecordingRetention {
	if s.stream == "" {
		return nil
	}
	out := &protocol.CollaborationRecordingRetention{Version: 1, StreamID: s.stream, RetainedThrough: len(s.messages), Reason: "incomplete"}
	if s.seal != nil {
		out.Sealed = true
		out.Reason = s.seal.Reason
		out.ObservedThrough = s.seal.ObservedThrough
		if out.ObservedThrough != nil && *out.ObservedThrough < out.RetainedThrough {
			observed := out.RetainedThrough
			out.ObservedThrough = &observed
		}
	}
	return out
}

func checkpointKey(p protocol.CollaborationRecordingCheckpoint) string {
	return fmt.Sprintf("checkpoint:%s:%d", p.StreamID, p.Sequence)
}

func validateRecordingAttempt(ctx context.Context, q *db.Queries, run pgtype.UUID, identity protocol.CollaborationRunIdentity, seq, nonce string) error {
	n, _ := strconv.ParseInt(seq, 10, 64)
	attempt, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: run, AttemptSeq: n})
	if err != nil {
		return err
	}
	term, err := q.GetCollaborationTermination(ctx, run)
	if err != nil {
		return err
	}
	frozen, err := collaborationFrozenIdentity(term)
	if err != nil {
		return err
	}
	if frozen != identity || attempt.AttemptNonce != nonce || strconv.FormatInt(attempt.ClaimGeneration, 10) != identity.ClaimGeneration || attempt.Containment != protocol.CollaborationContainmentHostProcessSupervision || !attempt.AuthorizedAt.Valid {
		return collaborationError(http.StatusConflict, "stale_execution")
	}
	return nil
}

func (s *CollaborationService) RecordRuntimeCheckpoint(ctx context.Context, run pgtype.UUID, token string, p protocol.CollaborationRecordingCheckpoint) (protocol.CollaborationCheckpointReceipt, error) {
	empty := protocol.CollaborationCheckpointReceipt{}
	if protocol.ValidateCollaborationRecordingCheckpoint(p) != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_recording_checkpoint")
	}
	p.Messages = append([]protocol.CollaborationRecordedMessage(nil), p.Messages...)
	for i := range p.Messages {
		p.Messages[i].Body = redact.Text(p.Messages[i].Body)
	}
	if protocol.ValidateCollaborationRecordingCheckpoint(p) != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_recording_checkpoint")
	}
	if !strings.HasPrefix(token, CollaborationBusinessTokenPrefix) || len(token) != 68 {
		return empty, collaborationError(http.StatusForbidden, "business_capability_required")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationCheckpointReceipt, error) {
		q := s.Queries.WithTx(tx)
		seq, _ := strconv.ParseInt(p.AttemptSeq, 10, 64)
		cap, err := q.GetCollaborationBusinessCapabilityByHash(ctx, db.GetCollaborationBusinessCapabilityByHashParams{RunID: run, TokenHash: collaborationCapabilityHash(token)})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && cap.AttemptSeq != seq {
			return empty, collaborationError(http.StatusForbidden, "business_capability_required")
		}
		if err != nil {
			return empty, err
		}
		if err := validateRecordingAttempt(ctx, q, run, p.Identity, p.AttemptSeq, p.AttemptNonce); err != nil {
			return empty, err
		}
		receipt := protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true}
		prior, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: checkpointKey(p)})
		if err == nil {
			var saved protocol.CollaborationRecordingCheckpoint
			if prior.Kind != "record_checkpoint" || decodeCollaborationPreparationJSON(prior.Payload, &saved) != nil || !reflect.DeepEqual(saved, p) {
				return empty, collaborationError(http.StatusConflict, "checkpoint_conflict")
			}
			return receipt, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		}
		r, err := s.lockRuntimeBusiness(ctx, tx, run, token)
		if err != nil {
			return empty, err
		}
		if p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) || r.run.AuthorizedAttemptSeq != seq {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		// A concurrent identical request may have committed while this caller
		// waited for the group/run locks. Recheck the original key under lock.
		prior, err = q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: checkpointKey(p)})
		if err == nil {
			var saved protocol.CollaborationRecordingCheckpoint
			if prior.Kind != "record_checkpoint" || decodeCollaborationPreparationJSON(prior.Payload, &saved) != nil || !reflect.DeepEqual(saved, p) {
				return empty, collaborationError(http.StatusConflict, "checkpoint_conflict")
			}
			return receipt, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		}
		state, err := readCollaborationRecording(ctx, q, r.access.Issue.ID, run, math.MaxInt64)
		if err != nil {
			return empty, err
		}
		if state.seal != nil || state.outcome || p.Sequence != state.sequence+1 || p.MessageOffset != len(state.messages) {
			return empty, collaborationError(http.StatusConflict, "checkpoint_out_of_order")
		}
		if err := state.retain(p.StreamID, p.MessageOffset, p.Messages); err != nil {
			return empty, collaborationError(http.StatusConflict, "checkpoint_conflict")
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return empty, err
		}
		if _, err = q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: run, EventID: checkpointKey(p), Kind: "record_checkpoint", Payload: payload}); err != nil {
			return empty, err
		}
		return receipt, recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, run, "runtime_record_checkpoint")
	})
}

func (s *CollaborationService) SealRuntimeRecording(ctx context.Context, run pgtype.UUID, token string, p protocol.CollaborationRecordingSeal) (protocol.CollaborationRecordingRetention, error) {
	empty := protocol.CollaborationRecordingRetention{}
	if protocol.ValidateCollaborationRecordingSeal(p) != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_recording_seal")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRecordingRetention, error) {
		q := s.Queries.WithTx(tx)
		if _, err := authorizeCollaborationTermination(ctx, q, run, token); err != nil {
			return empty, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, run, false, false)
		if err != nil {
			return empty, err
		}
		if err := validateRecordingAttempt(ctx, q, run, p.Identity, p.AttemptSeq, p.AttemptNonce); err != nil {
			return empty, err
		}
		state, err := readCollaborationRecording(ctx, q, r.access.Issue.ID, run, math.MaxInt64)
		if err != nil {
			return empty, err
		}
		if state.stream != "" && state.stream != p.StreamID {
			return empty, collaborationError(http.StatusConflict, "recording_seal_conflict")
		}
		key := fmt.Sprintf("recording-seal:%s:%d", p.StreamID, p.Sequence)
		prior, priorErr := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: key})
		if priorErr == nil {
			var saved protocol.CollaborationRecordingSeal
			if prior.Kind != "recording_seal" || decodeCollaborationPreparationJSON(prior.Payload, &saved) != nil || !reflect.DeepEqual(saved, p) {
				return empty, collaborationError(http.StatusConflict, "recording_seal_conflict")
			}
			return *state.retention(), nil
		}
		if !errors.Is(priorErr, pgx.ErrNoRows) {
			return empty, priorErr
		}
		if protocol.ValidateCollaborationRecordingSealAdvance(state.seal, p) != nil {
			return empty, collaborationError(http.StatusConflict, "recording_seal_conflict")
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return empty, err
		}
		if _, err = q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: run, EventID: key, Kind: "recording_seal", Payload: payload}); err != nil {
			return empty, err
		}
		state.stream = p.StreamID
		state.seal = &p
		return *state.retention(), recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, run, "runtime_recording_sealed")
	})
}

// Called under the existing outcome authority and run/group locks, before insert.
func validateCollaborationCheckpointOutcome(ctx context.Context, q *db.Queries, issue, run pgtype.UUID, p protocol.CollaborationRunOutcome) error {
	state, err := readCollaborationRecording(ctx, q, issue, run, math.MaxInt64)
	if err != nil {
		return err
	}
	if p.Record == nil || p.Record.Version != 2 {
		if state.stream != "" {
			return collaborationError(http.StatusConflict, "recording_stream_required")
		}
		return nil
	}
	if state.seal != nil && p.Record.MessageOffset+len(p.Record.Messages) > len(state.messages) {
		return collaborationError(http.StatusConflict, "recording_stream_sealed")
	}
	if err := state.retain(p.Record.StreamID, p.Record.MessageOffset, p.Record.Messages); err != nil {
		return collaborationError(http.StatusConflict, "recording_stream_conflict")
	}
	return nil
}

func redactCollaborationCheckpointOutcome(p protocol.CollaborationRunOutcome) protocol.CollaborationRunOutcome {
	if p.Record == nil || p.Record.Version != 2 {
		return p
	}
	record := *p.Record
	record.Messages = append([]protocol.CollaborationRecordedMessage{}, record.Messages...)
	for i := range record.Messages {
		record.Messages[i].Body = redact.Text(record.Messages[i].Body)
	}
	p.Record = &record
	p.Body = redact.Text(p.Body)
	return p
}
