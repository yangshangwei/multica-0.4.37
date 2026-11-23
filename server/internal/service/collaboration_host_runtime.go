package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func collaborationRuntimeMessageMode(version int, mode, retained string) bool {
	if retained == protocol.CollaborationContainmentHostProcessSupervision {
		return version == protocol.CollaborationHostRuntimeProtocolVersion && mode == retained
	}
	return retained == protocol.CollaborationContainmentLinuxPIDNamespace && version == 0 && mode == ""
}

type collaborationAcceptedLaunch struct {
	Observation protocol.CollaborationLaunchTermination        `json:"observation"`
	Receipt     protocol.CollaborationLaunchTerminationReceipt `json:"receipt"`
}

// This reads only the originally frozen manifest, so revocation, runtime
// deletion or changed configuration cannot strand minimal termination reports.
func frozenHostPreparation(run db.CollaborationRun, identity protocol.CollaborationRunIdentity) (protocol.CollaborationPreparation, error) {
	var manifest protocol.CollaborationPreparation
	if run.Containment.String != protocol.CollaborationContainmentHostProcessSupervision ||
		decodeCollaborationPreparationJSON(run.PreparationSnapshot, &manifest) != nil || manifest.Version != protocol.CollaborationHostPreparationVersion || manifest.Host == nil {
		return manifest, invalidCollaborationLedger("host preparation identity missing")
	}
	inputs, err := decodeCollaborationRoster[protocol.CollaborationRunInput](run.PromptSnapshot)
	if err != nil || protocol.ValidateCollaborationPreparation(&manifest, identity, inputs) != nil {
		return manifest, invalidCollaborationLedger("host preparation identity mismatch")
	}
	return manifest, nil
}

func decodeAcceptedHostLaunch(attempt db.CollaborationAttempt, identity protocol.CollaborationRunIdentity) (collaborationAcceptedLaunch, error) {
	var saved collaborationAcceptedLaunch
	if decodeCollaborationPreparationJSON(attempt.TerminalObservation, &saved) != nil ||
		protocol.ValidateCollaborationLaunchTermination(saved.Observation) != nil || saved.Observation.Identity != identity ||
		saved.Observation.AttemptSeq != strconv.FormatInt(attempt.AttemptSeq, 10) || saved.Observation.AttemptNonce != attempt.AttemptNonce ||
		attempt.Containment != protocol.CollaborationContainmentHostProcessSupervision ||
		strconv.FormatInt(attempt.ClaimGeneration, 10) != identity.ClaimGeneration {
		return saved, invalidCollaborationLedger("host launch observation unavailable")
	}
	expected := protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
		Identity: identity, LaunchID: attempt.AttemptNonce, EventID: saved.Observation.EventID, Accepted: true, Authorized: attempt.LaunchDescriptor != nil}
	if saved.Receipt != expected || saved.Receipt.Authorized != attempt.AuthorizedAt.Valid ||
		!saved.Receipt.Authorized && saved.Observation.Kind != "not_started" {
		return saved, invalidCollaborationLedger("host launch receipt identity mismatch")
	}
	return saved, nil
}

func validateHostObservationScope(manifest protocol.CollaborationPreparation, observation protocol.CollaborationLaunchTermination) error {
	for _, scope := range manifest.Host.SupervisionScopes {
		if scope == observation.Cleanup.Scope {
			return nil
		}
	}
	return collaborationError(http.StatusConflict, "execution_identity_mismatch")
}

func validateHostAttemptDescriptor(attempt db.CollaborationAttempt, manifest protocol.CollaborationPreparation) error {
	if attempt.LaunchDescriptor == nil && !attempt.AuthorizedAt.Valid {
		return nil
	}
	var descriptor protocol.CollaborationLaunchDescriptor
	if !attempt.AuthorizedAt.Valid || decodeCollaborationPreparationJSON(attempt.LaunchDescriptor, &descriptor) != nil ||
		protocol.ValidateCollaborationLaunchDescriptor(descriptor) != nil || descriptor.CapabilityDigest != manifest.CapabilityDigest ||
		descriptor.ConfigurationDigest != manifest.Host.ConfigurationDigest {
		return invalidCollaborationLedger("host launch descriptor mismatch")
	}
	return nil
}

func (s *CollaborationService) permitHostRuntimeRun(ctx context.Context, runID pgtype.UUID, p protocol.CollaborationRunPermitRequest) (protocol.CollaborationRunPermitResponse, error) {
	seq, valid := collaborationRuntimeVersion(p.AttemptSeq, false)
	nonce, nonceErr := uuid.Parse(p.AttemptNonce)
	if p.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || p.Containment != protocol.CollaborationContainmentHostProcessSupervision ||
		!valid || seq > protocol.CollaborationHostMaxLaunches || nonceErr != nil || nonce == uuid.Nil || nonce.String() != p.AttemptNonce ||
		p.LaunchDescriptor == nil || protocol.ValidateCollaborationLaunchDescriptor(*p.LaunchDescriptor) != nil {
		return protocol.CollaborationRunPermitResponse{}, collaborationError(http.StatusBadRequest, "invalid_runtime_attempt")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunPermitResponse, error) {
		var out protocol.CollaborationRunPermitResponse
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, true, true)
		if err != nil {
			return out, err
		}
		if err = r.currentAdmission(ctx); err != nil {
			return out, err
		}
		if !r.gate.HostExecutionEnabled {
			return out, collaborationError(http.StatusConflict, "execution_mode_unsupported")
		}
		if err = r.requireReservation(ctx); err != nil {
			return out, err
		}
		if p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) || r.run.Containment.String != p.Containment {
			return out, collaborationError(http.StatusConflict, "execution_identity_mismatch")
		}
		manifest, err := s.readRuntimePreparation(ctx, r)
		if err != nil {
			return out, err
		}
		if manifest.Host == nil || manifest.CapabilityDigest != p.LaunchDescriptor.CapabilityDigest || manifest.Host.ConfigurationDigest != p.LaunchDescriptor.ConfigurationDigest {
			return out, collaborationError(http.StatusConflict, "execution_identity_mismatch")
		}
		term, err := r.q.LockCollaborationTermination(ctx, runID)
		if err != nil {
			return out, err
		}
		identity, err := collaborationFrozenIdentity(term)
		if err != nil {
			return out, err
		}
		if identity != p.Identity || term.RevokedAt.Valid || term.ConfirmedAt.Valid || term.StopRequestedAt.Valid || r.run.SealedAttemptSeq.Valid {
			return out, collaborationError(http.StatusConflict, "execution_stopped")
		}
		attempt, err := r.q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		var token string
		if err == nil {
			var descriptor protocol.CollaborationLaunchDescriptor
			if attempt.AttemptNonce != p.AttemptNonce || attempt.ClaimGeneration != r.run.ClaimGeneration || attempt.Containment != p.Containment ||
				decodeCollaborationPreparationJSON(attempt.LaunchDescriptor, &descriptor) != nil || descriptor != *p.LaunchDescriptor {
				return out, collaborationError(http.StatusConflict, "attempt_conflict")
			}
			// Replay returns the original permit; the daemon's consumed identity
			// prevents another spawn. An unissued tombstone has no such permit.
			if !attempt.AuthorizedAt.Valid {
				return out, collaborationError(http.StatusConflict, "attempt_ended")
			}
			capability, err := r.q.GetCollaborationBusinessCapability(ctx, db.GetCollaborationBusinessCapabilityParams{RunID: runID, AttemptSeq: seq})
			if err != nil {
				return out, err
			}
			if capability.RevokedAt.Valid || r.run.AuthorizedAttemptSeq < seq {
				return out, collaborationError(http.StatusConflict, "attempt_ended")
			}
			token, err = openCollaborationCapability(runID, "business", capability.TokenCiphertext)
			if err != nil || !collaborationCapabilityMatches(token, CollaborationBusinessTokenPrefix, capability.TokenHash) {
				return out, invalidCollaborationLedger("host business capability unavailable")
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		} else {
			if seq != r.run.AuthorizedAttemptSeq+1 || r.run.BusinessOutcome.Valid {
				return out, collaborationError(http.StatusConflict, "attempt_conflict")
			}
			previous, err := r.q.ListCollaborationAttempts(ctx, runID)
			if err != nil {
				return out, err
			}
			if int64(len(previous)) != seq-1 {
				return out, invalidCollaborationLedger("host launch history incomplete")
			}
			for i, attempt := range previous {
				if attempt.AttemptSeq != int64(i+1) || attempt.TerminalObservation == nil {
					return out, collaborationError(http.StatusConflict, "prior_launch_unconfirmed")
				}
				accepted, err := decodeAcceptedHostLaunch(attempt, identity)
				if err != nil {
					return out, err
				}
				if !accepted.Receipt.Authorized {
					return out, collaborationError(http.StatusConflict, "attempt_ended")
				}
			}
			descriptor, err := json.Marshal(p.LaunchDescriptor)
			if err != nil {
				return out, err
			}
			if err = r.q.CreateCollaborationHostAttempt(ctx, db.CreateCollaborationHostAttemptParams{RunID: runID, AttemptSeq: seq,
				AttemptNonce: p.AttemptNonce, ClaimGeneration: r.run.ClaimGeneration, LaunchDescriptor: descriptor}); err != nil {
				return out, err
			}
			var hash string
			token, hash, err = newCollaborationCapability(CollaborationBusinessTokenPrefix)
			if err != nil {
				return out, err
			}
			sealed, err := sealCollaborationCapability(runID, "business", token)
			if err != nil {
				return out, err
			}
			if err = r.q.CreateCollaborationBusinessCapability(ctx, db.CreateCollaborationBusinessCapabilityParams{RunID: runID,
				AttemptSeq: seq, TokenHash: hash, TokenCiphertext: sealed}); err != nil {
				return out, err
			}
			if _, err = r.q.AuthorizeCollaborationHostAttempt(ctx, db.AuthorizeCollaborationHostAttemptParams{RunID: runID, AttemptSeq: seq}); err != nil {
				return out, err
			}
			if err = recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, runID, "runtime_permitted"); err != nil {
				return out, err
			}
		}
		return protocol.CollaborationRunPermitResponse{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
			Identity: identity, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce, BusinessToken: token}, nil
	})
}

// AcceptRuntimeLaunchTermination closes one physical launch only. It neither
// seals the run nor releases capacity, and works after authority is withdrawn.
func (s *CollaborationService) AcceptRuntimeLaunchTermination(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationLaunchTermination) (protocol.CollaborationLaunchTerminationReceipt, error) {
	var empty protocol.CollaborationLaunchTerminationReceipt
	if protocol.ValidateCollaborationLaunchTermination(p) != nil || p.Identity.RunID != util.UUIDToString(runID) {
		return empty, collaborationError(http.StatusBadRequest, "invalid_launch_termination")
	}
	seq, _ := collaborationRuntimeVersion(p.AttemptSeq, false)
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationLaunchTerminationReceipt, error) {
		q := s.Queries.WithTx(tx)
		if _, err := authorizeCollaborationTermination(ctx, q, runID, token); err != nil {
			return empty, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, false, false)
		if err != nil {
			return empty, err
		}
		term, err := q.LockCollaborationTermination(ctx, runID)
		if err != nil {
			return empty, err
		}
		if term.RevokedAt.Valid || !collaborationCapabilityMatches(token, CollaborationTerminationTokenPrefix, term.TokenHash) {
			return empty, collaborationError(http.StatusForbidden, "termination_capability_required")
		}
		identity, err := collaborationFrozenIdentity(term)
		if err != nil {
			return empty, err
		}
		if identity != p.Identity || r.run.Containment.String != p.Containment {
			return empty, collaborationError(http.StatusConflict, "execution_identity_mismatch")
		}
		manifest, err := frozenHostPreparation(r.run, identity)
		if err != nil {
			return empty, err
		}
		if err = validateHostObservationScope(manifest, p); err != nil {
			return empty, err
		}
		attempt, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		missing := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !missing {
			return empty, err
		}
		if !missing && (attempt.AttemptNonce != p.AttemptNonce || strconv.FormatInt(attempt.ClaimGeneration, 10) != identity.ClaimGeneration || attempt.Containment != p.Containment) {
			return empty, collaborationError(http.StatusConflict, "execution_identity_mismatch")
		}
		if !missing {
			if err = validateHostAttemptDescriptor(attempt, manifest); err != nil {
				return empty, err
			}
		}
		if !missing && attempt.TerminalObservation != nil {
			prior, err := decodeAcceptedHostLaunch(attempt, identity)
			if err != nil {
				return empty, err
			}
			if !reflect.DeepEqual(prior.Observation, p) {
				return empty, collaborationError(http.StatusConflict, "launch_termination_conflict")
			}
			return prior.Receipt, nil
		}
		if term.ConfirmedAt.Valid || missing && (seq != r.run.AuthorizedAttemptSeq+1 || p.Kind != "not_started") {
			return empty, collaborationError(http.StatusConflict, "stale_attempt_proof")
		}
		receipt := protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
			Identity: identity, LaunchID: p.LaunchID, EventID: p.EventID, Accepted: true, Authorized: !missing}
		raw, err := json.Marshal(collaborationAcceptedLaunch{Observation: p, Receipt: receipt})
		if err != nil {
			return empty, err
		}
		if missing {
			// A request which never obtained a permit gets a durable tombstone.
			// A late permit cannot turn this acknowledged nonlaunch into work.
			generation, _ := collaborationRuntimeVersion(identity.ClaimGeneration, false)
			err = q.CreateCollaborationHostAttempt(ctx, db.CreateCollaborationHostAttemptParams{RunID: runID, AttemptSeq: seq,
				AttemptNonce: p.AttemptNonce, ClaimGeneration: generation, TerminalObservation: raw})
		} else {
			_, err = q.RecordCollaborationLaunchTermination(ctx, db.RecordCollaborationLaunchTerminationParams{RunID: runID, AttemptSeq: seq, Observation: raw})
		}
		if err != nil {
			return empty, err
		}
		if err = recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, runID, "runtime_launch_ended"); err != nil {
			return empty, err
		}
		return receipt, nil
	})
}

func validateHostProofAttempts(ctx context.Context, q *db.Queries, run db.CollaborationRun, p protocol.CollaborationRunProof) error {
	if int64(len(p.Launches)) != run.AuthorizedAttemptSeq {
		return collaborationError(http.StatusConflict, "stale_attempt_proof")
	}
	manifest, err := frozenHostPreparation(run, p.Identity)
	if err != nil {
		return err
	}
	attempts, err := q.ListCollaborationAttempts(ctx, run.ID)
	if err != nil {
		return err
	}
	if len(attempts) < len(p.Launches) || len(attempts) > len(p.Launches)+1 {
		return invalidCollaborationLedger("host attempt coverage incomplete")
	}
	for i, attempt := range attempts {
		if attempt.AttemptSeq != int64(i+1) || attempt.TerminalObservation == nil {
			return collaborationError(http.StatusConflict, "prior_launch_unconfirmed")
		}
		if err = validateHostAttemptDescriptor(attempt, manifest); err != nil {
			return err
		}
		accepted, err := decodeAcceptedHostLaunch(attempt, p.Identity)
		if err != nil {
			return err
		}
		if err = validateHostObservationScope(manifest, accepted.Observation); err != nil {
			return err
		}
		if i == len(p.Launches) {
			if accepted.Receipt.Authorized || accepted.Observation.Kind != "not_started" {
				return collaborationError(http.StatusConflict, "stale_attempt_proof")
			}
			continue
		}
		if !accepted.Receipt.Authorized || !reflect.DeepEqual(p.Launches[i], accepted.Observation) {
			return collaborationError(http.StatusConflict, "stale_attempt_proof")
		}
	}
	return nil
}
