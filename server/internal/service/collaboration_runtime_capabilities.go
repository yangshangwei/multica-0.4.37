package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func collaborationRuntimeVersion(value string, zero bool) (int64, bool) {
	if zero && value == "0" {
		return 0, true
	}
	if len(value) == 0 || len(value) > 19 || value[0] < '1' || value[0] > '9' {
		return 0, false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil
}

func collaborationCapabilityAEAD() (cipher.AEAD, error) {
	key := sha256.Sum256(append([]byte("collaboration-runtime-result-v1\x00"), auth.JWTSecret()...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func sealCollaborationCapability(runID pgtype.UUID, purpose, token string) ([]byte, error) {
	aead, err := collaborationCapabilityAEAD()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	associated := append(append([]byte(nil), runID.Bytes[:]...), []byte(purpose)...)
	return aead.Seal(nonce, nonce, []byte(token), associated), nil
}

func openCollaborationCapability(runID pgtype.UUID, purpose string, sealed []byte) (string, error) {
	aead, err := collaborationCapabilityAEAD()
	if err != nil {
		return "", err
	}
	if len(sealed) < aead.NonceSize() {
		return "", collaborationError(http.StatusServiceUnavailable, "capability_recovery_unavailable")
	}
	associated := append(append([]byte(nil), runID.Bytes[:]...), []byte(purpose)...)
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], associated)
	if err != nil {
		return "", collaborationError(http.StatusServiceUnavailable, "capability_recovery_unavailable")
	}
	return string(plain), nil
}

func newCollaborationCapability(prefix string) (string, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	token := prefix + hex.EncodeToString(secret)
	return token, collaborationCapabilityHash(token), nil
}

func collaborationCapabilityHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func collaborationCapabilityMatches(token, prefix, expectedHash string) bool {
	if len(token) != len(prefix)+64 || !strings.HasPrefix(token, prefix) {
		return false
	}
	for _, c := range token[len(prefix):] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return subtle.ConstantTimeCompare([]byte(collaborationCapabilityHash(token)), []byte(expectedHash)) == 1
}

func collaborationFrozenIdentity(term db.CollaborationTermination) (protocol.CollaborationRunIdentity, error) {
	var identity protocol.CollaborationRunIdentity
	decoder := json.NewDecoder(bytes.NewReader(term.RuntimeIdentity))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&identity) != nil || decoder.Decode(new(any)) != io.EOF {
		return identity, invalidCollaborationLedger("termination identity unavailable")
	}
	if identity.RunID != util.UUIDToString(term.RunID) || identity.IssueID != util.UUIDToString(term.IssueID) || identity.InstallationID != util.UUIDToString(term.InstallationID) ||
		identity.DeploymentID != util.UUIDToString(term.DeploymentID) || identity.LaunchNonce != util.UUIDToString(term.LaunchNonce) || identity.Fence != strconv.FormatInt(term.Fence, 10) {
		return identity, invalidCollaborationLedger("termination identity mismatch")
	}
	if _, ok := collaborationRuntimeVersion(identity.ClaimGeneration, false); !ok {
		return identity, invalidCollaborationLedger("termination delivery identity unavailable")
	}
	return identity, nil
}

func (s *CollaborationService) ClaimRuntimeRun(ctx context.Context, runID pgtype.UUID, p protocol.CollaborationRunClaimRequest) (protocol.CollaborationRunClaimResponse, error) {
	if p.ProtocolVersion != protocol.CollaborationRuntimeProtocolVersion || p.Containment != protocol.CollaborationContainmentLinuxPIDNamespace || !runID.Valid {
		return protocol.CollaborationRunClaimResponse{}, collaborationError(http.StatusBadRequest, "unsupported_runtime_protocol")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunClaimResponse, error) {
		var out protocol.CollaborationRunClaimResponse
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, true, true)
		if err != nil {
			return out, err
		}
		if p.DeploymentID != util.UUIDToString(r.run.DeploymentID) || p.InstallationID != util.UUIDToString(r.run.InstallationID) || p.RuntimeID != util.UUIDToString(r.run.RuntimeID) ||
			p.BindingID != util.UUIDToString(r.run.BindingID) || p.BindingEpoch != strconv.FormatInt(r.run.BindingEpoch, 10) {
			return out, collaborationError(http.StatusConflict, "stale_runtime_binding")
		}
		if err = r.currentAdmission(ctx); err != nil {
			return out, err
		}
		if err = r.reserveRun(ctx); err != nil {
			return out, err
		}
		if r.run.Containment.Valid && r.run.Containment.String != p.Containment {
			return out, collaborationError(http.StatusConflict, "runtime_capability_changed")
		}
		identity := collaborationRunIdentity(r.run, r.access.Issue.ID)
		term, err := r.q.LockCollaborationTermination(ctx, runID)
		var token string
		if errors.Is(err, pgx.ErrNoRows) {
			var hash string
			token, hash, err = newCollaborationCapability(CollaborationTerminationTokenPrefix)
			if err != nil {
				return out, err
			}
			sealed, err := sealCollaborationCapability(runID, "termination", token)
			if err != nil {
				return out, err
			}
			encoded, err := json.Marshal(identity)
			if err != nil {
				return out, err
			}
			installation, err := r.q.GetManagedInstallation(ctx, r.run.InstallationID)
			if err != nil {
				return out, err
			}
			term, err = r.q.CreateCollaborationTerminationCapability(ctx, db.CreateCollaborationTerminationCapabilityParams{IssueID: r.access.Issue.ID, RunID: runID,
				Fence: r.run.Fence, LaunchNonce: r.run.LaunchNonce, DeploymentID: r.run.DeploymentID, InstallationID: r.run.InstallationID, PublicKey: installation.PublicKey,
				TokenHash: hash, RuntimeIdentity: encoded, TokenCiphertext: sealed})
			if err != nil {
				return out, err
			}
		} else if err != nil {
			return out, err
		} else {
			frozen, err := collaborationFrozenIdentity(term)
			if err != nil {
				return out, err
			}
			if frozen != identity || term.RevokedAt.Valid || term.ConfirmedAt.Valid {
				return out, collaborationError(http.StatusConflict, "stale_execution")
			}
			token, err = openCollaborationCapability(runID, "termination", term.TokenCiphertext)
			if err != nil {
				return out, err
			}
		}
		if !collaborationCapabilityMatches(token, CollaborationTerminationTokenPrefix, term.TokenHash) {
			return out, invalidCollaborationLedger("termination recovery hash mismatch")
		}
		newClaim := r.run.ProgressState == "prepared"
		if _, err = r.q.ClaimCollaborationRuntimeRun(ctx, db.ClaimCollaborationRuntimeRunParams{RunID: runID, Containment: p.Containment}); err != nil {
			return out, err
		}
		inputs, err := r.claimPromptSnapshot(ctx)
		if err != nil {
			return out, err
		}
		if newClaim {
			if err = recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, runID, "runtime_claimed"); err != nil {
				return out, err
			}
		}
		out = protocol.CollaborationRunClaimResponse{Identity: identity, Inputs: inputs, AgentID: util.UUIDToString(r.run.AgentID), ResourceOwnerID: util.UUIDToString(r.run.ResourceOwnerUserID), ExecutionWorkspaceID: util.UUIDToString(r.run.ExecutionWorkspaceID), TerminationToken: token}
		return out, nil
	})
}

func (r *collaborationRuntimeState) claimPromptSnapshot(ctx context.Context) ([]protocol.CollaborationRunInput, error) {
	if r.run.PromptSnapshot == nil {
		if err := r.consumeJoinedInputs(ctx); err != nil {
			return nil, err
		}
		commands := []db.CollaborationCommand{}
		for _, input := range r.inputs {
			if input.State == "in_flight" && input.GroupID == r.group.ID {
				commands = append(commands, input)
			}
		}
		inputs, err := collaborationRunInputs(commands)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(inputs)
		if err != nil {
			return nil, err
		}
		r.run, err = r.q.FreezeCollaborationRunPrompt(ctx, db.FreezeCollaborationRunPromptParams{RunID: r.run.ID, PromptSnapshot: encoded})
		if err != nil {
			return nil, err
		}
	}
	inputs, err := decodeCollaborationRoster[protocol.CollaborationRunInput](r.run.PromptSnapshot)
	if err != nil {
		return nil, err
	}
	return inputs, nil
}

func (s *CollaborationService) PermitRuntimeRun(ctx context.Context, runID pgtype.UUID, p protocol.CollaborationRunPermitRequest) (protocol.CollaborationRunPermitResponse, error) {
	seq, ok := collaborationRuntimeVersion(p.AttemptSeq, false)
	if !ok || seq != 1 || len(p.AttemptNonce) < 16 || len(p.AttemptNonce) > 128 || p.Containment != protocol.CollaborationContainmentLinuxPIDNamespace {
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
		if err = r.requireReservation(ctx); err != nil {
			return out, err
		}
		if r.run.Containment.String != p.Containment || p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) {
			return out, collaborationError(http.StatusConflict, "stale_execution")
		}
		term, err := r.q.GetCollaborationTermination(ctx, runID)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, collaborationError(http.StatusConflict, "claim_required")
		}
		if err != nil {
			return out, err
		}
		identity, err := collaborationFrozenIdentity(term)
		if err != nil {
			return out, err
		}
		if identity != p.Identity || term.RevokedAt.Valid || term.ConfirmedAt.Valid {
			return out, collaborationError(http.StatusConflict, "stale_execution")
		}
		attempt, err := r.q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		var token string
		if err == nil {
			if attempt.AttemptNonce != p.AttemptNonce || attempt.ClaimGeneration != r.run.ClaimGeneration || attempt.Containment != p.Containment {
				return out, collaborationError(http.StatusConflict, "attempt_conflict")
			}
			capability, err := r.q.GetCollaborationBusinessCapability(ctx, db.GetCollaborationBusinessCapabilityParams{RunID: runID, AttemptSeq: seq})
			if err != nil {
				return out, err
			}
			if capability.RevokedAt.Valid {
				return out, collaborationError(http.StatusForbidden, "business_capability_revoked")
			}
			token, err = openCollaborationCapability(runID, "business", capability.TokenCiphertext)
			if err != nil {
				return out, err
			}
			if !collaborationCapabilityMatches(token, CollaborationBusinessTokenPrefix, capability.TokenHash) {
				return out, invalidCollaborationLedger("business recovery hash mismatch")
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		} else {
			if r.run.AuthorizedAttemptSeq != 0 {
				return out, collaborationError(http.StatusConflict, "attempt_conflict")
			}
			if err = r.q.CreateCollaborationAttempt(ctx, db.CreateCollaborationAttemptParams{RunID: runID, AttemptSeq: seq, AttemptNonce: p.AttemptNonce, ClaimGeneration: r.run.ClaimGeneration, Containment: p.Containment}); err != nil {
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
			if err = r.q.CreateCollaborationBusinessCapability(ctx, db.CreateCollaborationBusinessCapabilityParams{RunID: runID, AttemptSeq: seq, TokenHash: hash, TokenCiphertext: sealed}); err != nil {
				return out, err
			}
			if _, err = r.q.AuthorizeCollaborationAttempt(ctx, db.AuthorizeCollaborationAttemptParams{RunID: runID, AttemptSeq: seq}); err != nil {
				return out, err
			}
			if err = recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, runID, "runtime_permitted"); err != nil {
				return out, err
			}
		}
		return protocol.CollaborationRunPermitResponse{Identity: identity, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce, BusinessToken: token}, nil
	})
}

func (s *CollaborationService) RecordRuntimeOutcome(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationRunOutcome) error {
	seq, ok := collaborationRuntimeVersion(p.AttemptSeq, false)
	if !ok || len(p.EventID) == 0 || len(p.EventID) > 128 || len(p.Body) > 1<<20 || (p.Outcome != protocol.CollaborationBusinessOutcomeSuccess && p.Outcome != protocol.CollaborationBusinessOutcomeFailure) {
		return collaborationError(http.StatusBadRequest, "invalid_runtime_outcome")
	}
	if !strings.HasPrefix(token, CollaborationBusinessTokenPrefix) || len(token) != 68 {
		return collaborationError(http.StatusForbidden, "business_capability_required")
	}
	_, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (bool, error) {
		q := s.Queries.WithTx(tx)
		capability, err := q.GetCollaborationBusinessCapabilityByHash(ctx, db.GetCollaborationBusinessCapabilityByHashParams{RunID: runID, TokenHash: collaborationCapabilityHash(token)})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (capability.RevokedAt.Valid || capability.AttemptSeq != seq) {
			return false, collaborationError(http.StatusForbidden, "business_capability_required")
		}
		if err != nil {
			return false, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, true, false)
		if err != nil {
			return false, err
		}
		if err = r.currentAdmission(ctx); err != nil {
			return false, err
		}
		if err = r.requireReservation(ctx); err != nil {
			return false, err
		}
		if p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) {
			return false, collaborationError(http.StatusConflict, "stale_execution")
		}
		attempt, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		if err != nil {
			return false, err
		}
		if attempt.AttemptNonce != p.AttemptNonce || attempt.ClaimGeneration != r.run.ClaimGeneration || r.run.AuthorizedAttemptSeq != seq {
			return false, collaborationError(http.StatusConflict, "stale_execution")
		}
		prior, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: p.EventID})
		if err == nil {
			var saved protocol.CollaborationRunOutcome
			if json.Unmarshal(prior.Payload, &saved) != nil || saved != p {
				return false, collaborationError(http.StatusConflict, "outcome_conflict")
			}
			return true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		if r.run.BusinessOutcome.Valid {
			return false, collaborationError(http.StatusConflict, "outcome_already_recorded")
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return false, err
		}
		if _, err = q.CreateCollaborationRuntimeOutcomeEvent(ctx, db.CreateCollaborationRuntimeOutcomeEventParams{IssueID: r.access.Issue.ID, RunID: runID, EventID: p.EventID, Payload: payload}); err != nil {
			return false, err
		}
		if _, err = q.RecordCollaborationRuntimeOutcome(ctx, db.RecordCollaborationRuntimeOutcomeParams{RunID: runID, Outcome: p.Outcome}); err != nil {
			return false, err
		}
		return true, recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, runID, "runtime_outcome_recorded")
	})
	return err
}

func authorizeCollaborationTermination(ctx context.Context, q *db.Queries, runID pgtype.UUID, token string) (db.CollaborationTermination, error) {
	var empty db.CollaborationTermination
	if !strings.HasPrefix(token, CollaborationTerminationTokenPrefix) || len(token) != 68 {
		return empty, collaborationError(http.StatusForbidden, "termination_capability_required")
	}
	term, err := q.GetCollaborationTermination(ctx, runID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (term.RevokedAt.Valid || !collaborationCapabilityMatches(token, CollaborationTerminationTokenPrefix, term.TokenHash)) {
		return empty, collaborationError(http.StatusForbidden, "termination_capability_required")
	}
	return term, err
}

func (s *CollaborationService) RuntimeStopIntent(ctx context.Context, runID pgtype.UUID, token string) (protocol.CollaborationRunStopIntent, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunStopIntent, error) {
		var out protocol.CollaborationRunStopIntent
		q := s.Queries.WithTx(tx)
		term, err := authorizeCollaborationTermination(ctx, q, runID, token)
		if err != nil {
			return out, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, false, false)
		if err != nil {
			return out, err
		}
		identity, err := collaborationFrozenIdentity(term)
		if err != nil {
			return out, err
		}
		return protocol.CollaborationRunStopIntent{Identity: identity, StopRequested: term.StopRequestedAt.Valid || r.run.ForcedDisposition.Valid || r.run.SettledAt.Valid || r.session.QueueState != "active" || r.channel.QueueState != "active"}, nil
	})
}

func (s *CollaborationService) AcceptRuntimeProof(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationRunProof) (protocol.CollaborationRunProofReceipt, error) {
	last, okLast := collaborationRuntimeVersion(p.LastAuthorizedAttemptSeq, true)
	sealed, okSealed := collaborationRuntimeVersion(p.SealedAttemptSeq, true)
	if !okLast || !okSealed || last != sealed || len(p.ReceiptID) == 0 || len(p.ReceiptID) > 128 || p.Containment != protocol.CollaborationContainmentLinuxPIDNamespace ||
		(p.State != "never_launched_proven" && p.State != "exited_proven") || p.ExitCode != nil && (*p.ExitCode < -255 || *p.ExitCode > 255) {
		return protocol.CollaborationRunProofReceipt{}, collaborationError(http.StatusBadRequest, "invalid_runtime_proof")
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return protocol.CollaborationRunProofReceipt{}, err
	}
	digest := collaborationCapabilityHash(string(encoded))
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunProofReceipt, error) {
		var out protocol.CollaborationRunProofReceipt
		q := s.Queries.WithTx(tx)
		if _, err := authorizeCollaborationTermination(ctx, q, runID, token); err != nil {
			return out, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, false, false)
		if err != nil {
			return out, err
		}
		term, err := q.LockCollaborationTermination(ctx, runID)
		if err != nil {
			return out, err
		}
		if term.RevokedAt.Valid || !collaborationCapabilityMatches(token, CollaborationTerminationTokenPrefix, term.TokenHash) {
			return out, collaborationError(http.StatusForbidden, "termination_capability_required")
		}
		identity, err := collaborationFrozenIdentity(term)
		if err != nil {
			return out, err
		}
		if identity != p.Identity {
			return out, collaborationError(http.StatusConflict, "stale_execution")
		}
		expected := collaborationRunIdentity(r.run, r.access.Issue.ID)
		expected.ClaimGeneration = identity.ClaimGeneration
		if expected != identity {
			return out, invalidCollaborationLedger("proof identity no longer matches retained run")
		}
		out = protocol.CollaborationRunProofReceipt{ReceiptID: p.ReceiptID, Identity: identity, Accepted: true}
		if term.ConfirmedAt.Valid {
			if term.ReceiptID.String != p.ReceiptID || term.ReceiptDigest.String != digest {
				return out, collaborationError(http.StatusConflict, "proof_conflict")
			}
			return out, nil
		}
		if last != r.run.AuthorizedAttemptSeq {
			return out, collaborationError(http.StatusConflict, "stale_attempt_proof")
		}
		if last > 0 {
			attempt, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: last})
			if err != nil {
				return out, err
			}
			if attempt.AttemptNonce != p.AttemptNonce || strconv.FormatInt(attempt.ClaimGeneration, 10) != identity.ClaimGeneration || attempt.Containment != p.Containment {
				return out, collaborationError(http.StatusConflict, "stale_attempt_proof")
			}
		} else if p.State != "never_launched_proven" {
			return out, collaborationError(http.StatusConflict, "invalid_runtime_proof")
		}
		if p.State == "never_launched_proven" && (r.run.StartedAt.Valid || r.run.BusinessOutcome.String == protocol.CollaborationBusinessOutcomeSuccess) {
			return out, collaborationError(http.StatusConflict, "contradictory_runtime_proof")
		}
		if r.run.SettledAt.Valid && (r.run.ProofState != p.State || r.run.SealedAttemptSeq.Int64 != sealed) {
			return out, collaborationError(http.StatusConflict, "proof_conflict")
		}
		if !r.run.SettledAt.Valid {
			r.run, err = q.RecordCollaborationRuntimeProof(ctx, db.RecordCollaborationRuntimeProofParams{RunID: runID, ProofState: p.State, SealedAttemptSeq: sealed})
			if err != nil {
				return out, err
			}
		}
		if !collaborationRunProvenEnded(r.run) {
			return out, invalidCollaborationLedger("settled run has contradictory termination evidence")
		}
		if _, err = q.RecordCollaborationTerminationProof(ctx, db.RecordCollaborationTerminationProofParams{RunID: runID, ReceiptID: p.ReceiptID, ReceiptDigest: digest, Proof: encoded}); err != nil {
			return out, err
		}
		if err = q.RevokeCollaborationRunBusinessCapabilities(ctx, runID); err != nil {
			return out, err
		}
		if err = q.ReleaseCollaborationRunReservation(ctx, runID); err != nil {
			return out, err
		}
		if err = q.SettleCollaborationRunDelegations(ctx, runID); err != nil {
			return out, err
		}
		if !r.group.SettledAt.Valid {
			if r.target.Kind == "agent" {
				r.group, err = q.SealCollaborationCompletedGroup(ctx, r.group.ID)
				if err != nil {
					return out, err
				}
			} else if !r.group.ExpansionSealedAt.Valid {
				if r.run.StepKind.String == "member" {
					if err = q.ScheduleCollaborationSquadWakeup(ctx, db.ScheduleCollaborationSquadWakeupParams{ID: r.group.ID, WakeParentRunID: runID}); err != nil {
						return out, err
					}
				} else {
					runs, err := q.LockCollaborationGroupRuns(ctx, r.group.ID)
					if err != nil {
						return out, err
					}
					complete := !r.group.WakePending
					for _, run := range runs {
						if !run.SettledAt.Valid {
							complete = false
						}
					}
					if complete {
						r.group, err = q.SealCollaborationCompletedGroup(ctx, r.group.ID)
						if err != nil {
							return out, err
						}
					}
				}
			}
			if _, err = r.settleProvenGroup(ctx, r.target, r.group); err != nil {
				return out, err
			}
		}
		return out, recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, runID, "runtime_ended")
	})
}
