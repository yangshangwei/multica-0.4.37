package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (s *CollaborationService) preparationAttachments(ctx context.Context, q *db.Queries, issue, workspace pgtype.UUID, inputs []protocol.CollaborationRunInput) ([]protocol.CollaborationPreparationAttachment, error) {
	rows, err := q.ListCollaborationPreparationAttachments(ctx, issue)
	if err != nil {
		return nil, err
	}
	if len(rows) > collaborationMaxSelectedAttachments {
		return nil, collaborationError(http.StatusConflict, "runtime_preparation_limits")
	}
	files := make([]protocol.CollaborationPreparationAttachment, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.IssueID != issue || row.SourceWorkspaceID != workspace || !row.DeliveredAt.Valid || row.DiscardedAt.Valid {
			return nil, collaborationError(http.StatusServiceUnavailable, "runtime_preparation_unavailable")
		}
		if err = validateCollaborationStoredAttachment(row); err != nil {
			return nil, err
		}
		id, version := util.UUIDToString(row.ID), strconv.FormatInt(row.Version, 10)
		files = append(files, protocol.CollaborationPreparationAttachment{ID: id, Version: version, Name: row.Name,
			ContentType: row.ContentType, SizeBytes: row.SizeBytes, Destination: protocol.CollaborationPreparationAttachmentDestination(id, version)})
		seen[id] = true
	}
	for _, input := range inputs {
		for _, id := range input.AttachmentIDs {
			if !seen[id] {
				return nil, collaborationError(http.StatusConflict, "runtime_preparation_unavailable")
			}
		}
	}
	return files, nil
}

func preparationWithinLimits(files []protocol.CollaborationPreparationAttachment, caps protocol.CollaborationRuntimeCapabilities) error {
	total := int64(0)
	if len(files) > caps.Limits.FileCount {
		return collaborationError(http.StatusConflict, "runtime_preparation_limits")
	}
	for _, file := range files {
		if file.SizeBytes > caps.Limits.PerFileBytes {
			return collaborationError(http.StatusConflict, "runtime_preparation_limits")
		}
		total += file.SizeBytes
	}
	if total > caps.Limits.TotalInputBytes || caps.Git != nil && total+caps.Limits.GitExpandedBytes > caps.Limits.WorkspaceBytes {
		return collaborationError(http.StatusConflict, "runtime_preparation_limits")
	}
	return nil
}

// File-dependent work must not reserve a namespace which an older daemon can
// never consume. This check precedes all batch/group/run/reservation creation.
// Each member needs its own capability; a leader cannot lend file authority.
func (s *CollaborationService) checkPreparationPromotion(ctx context.Context, t *collaborationQueueTx, selection collaborationInputSelection) error {
	inputs, err := collaborationRunInputs(selection.inputs)
	if err != nil {
		return err
	}
	files, err := s.preparationAttachments(ctx, t.q, t.access.Issue.ID, t.access.Issue.WorkspaceID, inputs)
	if err != nil {
		return err
	}
	checked := map[pgtype.UUID]bool{}
	for _, input := range selection.inputs {
		if checked[input.TargetID] {
			continue
		}
		checked[input.TargetID] = true
		target, ok := t.target(input.TargetID)
		if !ok {
			return invalidCollaborationLedger("preparation target unavailable")
		}
		grants, err := t.frozenTargetGrants(ctx, target)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			runtimeID, err := util.ParseUUID(grant.RuntimeID)
			if err != nil {
				return invalidCollaborationLedger("preparation runtime unavailable")
			}
			runtime, err := s.collaborationRuntimeLookup(t.q).Get(ctx, runtimeID)
			if err != nil {
				return err
			}
			saved, err := collaborationRegisteredCapabilities(runtime)
			if err != nil {
				return err
			}
			// Once host execution is opened, every resource in a newly
			// promoted group must use that mode. Namespace support remains
			// only for draining groups already claimed before the cutover.
			if t.gate.HostExecutionEnabled && (saved == nil || saved.Capabilities.Containment != protocol.CollaborationContainmentHostProcessSupervision) {
				return collaborationError(http.StatusConflict, "execution_mode_unsupported")
			}
			if len(files) == 0 && saved == nil {
				continue
			}
			if saved != nil && saved.Capabilities.Host != nil && !t.gate.HostExecutionEnabled {
				return collaborationError(http.StatusConflict, "execution_mode_unsupported")
			}
			installationID, installErr := util.ParseUUID(grant.InstallationID)
			bindingID, bindingErr := util.ParseUUID(grant.BindingID)
			if installErr != nil || bindingErr != nil {
				return collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
			}
			saved, err = currentCollaborationPreparationCapabilities(runtime, installationID, bindingID, grant.BindingEpoch)
			if err != nil {
				return err
			}
			if err = preparationWithinLimits(files, saved.Capabilities); err != nil {
				return err
			}
			if err = checkPreparationSquadInstructions(ctx, t.q, target); err != nil {
				return err
			}
			agentID, err := util.ParseUUID(grant.AgentID)
			if err != nil {
				return invalidCollaborationLedger("preparation agent identity unavailable")
			}
			if _, err = preparationInvocation(ctx, t.q, agentID, saved.Capabilities.Provider.ProtocolFamily); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *CollaborationService) preparationForClaim(ctx context.Context, r *collaborationRuntimeState, p protocol.CollaborationRunClaimRequest) (*protocol.CollaborationRuntimeCapabilityRegistration, error) {
	runtime, err := s.collaborationRuntimeLookup(r.q).Get(ctx, r.run.RuntimeID)
	if err != nil {
		return nil, err
	}
	if p.PreparationVersion == 0 {
		if r.run.PreparationSnapshot != nil {
			return nil, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
		}
		inputs, err := collaborationRunInputs(r.inputs)
		if err != nil {
			return nil, err
		}
		files, err := s.preparationAttachments(ctx, r.q, r.access.Issue.ID, r.access.Issue.WorkspaceID, inputs)
		if err != nil {
			return nil, err
		}
		saved, err := collaborationRegisteredCapabilities(runtime)
		if err != nil {
			return nil, err
		}
		if saved != nil && saved.Capabilities.Host != nil {
			return nil, collaborationError(http.StatusConflict, "execution_mode_unsupported")
		}
		if len(files) > 0 || saved != nil && saved.Capabilities.Git != nil {
			return nil, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
		}
		if saved != nil && r.run.PromptSnapshot == nil {
			invocation, err := preparationInvocation(ctx, r.q, r.run.AgentID, saved.Capabilities.Provider.ProtocolFamily)
			if err != nil {
				return nil, err
			}
			if invocation != (protocol.CollaborationPreparationInvocation{}) {
				return nil, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
			}
		}
		return nil, nil
	}
	if r.run.PromptSnapshot != nil && r.run.PreparationSnapshot == nil {
		return nil, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
	}
	saved, err := currentCollaborationPreparationCapabilities(runtime, r.run.InstallationID, r.run.BindingID, r.run.BindingEpoch)
	if err != nil {
		return nil, err
	}
	if saved.Capabilities.PreparationVersion != p.PreparationVersion || saved.Capabilities.Containment != p.Containment ||
		saved.Capabilities.Provider.BundleDigest != p.BundleDigest || runtime.CollaborationConfigEpoch != r.run.ResourceEpoch {
		return nil, collaborationError(http.StatusConflict, "runtime_capability_changed")
	}
	if saved.Capabilities.Host != nil {
		digest, err := protocol.CollaborationRuntimeCapabilityDigest(saved.Capabilities)
		if err != nil || digest != p.CapabilityDigest {
			return nil, collaborationError(http.StatusConflict, "runtime_capability_changed")
		}
	}
	if r.run.PromptSnapshot == nil {
		if err = checkPreparationSquadInstructions(ctx, r.q, r.target); err != nil {
			return nil, err
		}
		if _, err = preparationInvocation(ctx, r.q, r.run.AgentID, saved.Capabilities.Provider.ProtocolFamily); err != nil {
			return nil, err
		}
		inputs, err := collaborationRunInputs(r.inputs)
		if err != nil {
			return nil, err
		}
		files, err := s.preparationAttachments(ctx, r.q, r.access.Issue.ID, r.access.Issue.WorkspaceID, inputs)
		if err != nil {
			return nil, err
		}
		if err = preparationWithinLimits(files, saved.Capabilities); err != nil {
			return nil, err
		}
	}
	return saved, nil
}

// Delivery can commit after admission but before first claim. A run that has
// never issued any capability or attempt can then be impossible to prepare.
// Reuse the server's affirmative never-permitted proof and input-return path;
// this is not a Stop, a provider outcome or a replacement daemon proof.
func (s *CollaborationService) settleUnclaimedPreparationRefusal(ctx context.Context, r *collaborationRuntimeState, refusal error) (bool, error) {
	var denied *CollaborationError
	if !errors.As(refusal, &denied) || (denied.Code != "runtime_preparation_unsupported" && denied.Code != "runtime_preparation_limits") ||
		r.run.ProgressState != "prepared" || !collaborationRunNeverLaunched(r.run) ||
		r.run.PromptSnapshot != nil || r.run.PreparationSnapshot != nil || r.run.Containment.Valid || r.run.BusinessOutcome.Valid {
		return false, nil
	}
	runtime, err := s.collaborationRuntimeLookup(r.q).Get(ctx, r.run.RuntimeID)
	if err != nil {
		return false, err
	}
	// An old request against a capable runtime is not evidence that the work
	// cannot run. Its actual capable client must retain the same candidate.
	if denied.Code == "runtime_preparation_unsupported" && runtime.CollaborationCapabilities != nil {
		return false, nil
	}
	files, err := r.q.ListCollaborationPreparationAttachments(ctx, r.access.Issue.ID)
	if err != nil || len(files) == 0 {
		return false, err
	}
	runs, err := r.q.LockCollaborationGroupRuns(ctx, r.group.ID)
	if err != nil {
		return false, err
	}
	delegations, err := r.q.LockCollaborationGroupDelegations(ctx, r.group.ID)
	if err != nil {
		return false, err
	}
	if !r.run.ParentRunID.Valid {
		if r.run.StepKey.String != "root" || len(delegations) != 0 || (r.target.Kind != "squad" && len(runs) != 1) {
			return false, nil
		}
		// M11 may queue an unread continuation before the reserved root's
		// first claim. Prove only this root; its untouched continuation keeps
		// its own independent admission/nonlaunch path on the next poll.
		for _, other := range runs {
			if other.ID == r.run.ID {
				continue
			}
			if other.ParentRunID != r.run.ID || other.StepKind.String != "continuation" || other.ProgressState != "prepared" ||
				!collaborationRunNeverLaunched(other) || other.PromptSnapshot != nil || other.PreparationSnapshot != nil || other.Containment.Valid || other.BusinessOutcome.Valid {
				return false, nil
			}
			issued, err := r.q.CollaborationRunHasPreparationStartEvidence(ctx, other.ID)
			if err != nil || issued {
				return false, err
			}
		}
	} else {
		if r.run.StepKind.String != "member" && r.run.StepKind.String != "continuation" {
			return false, nil
		}
		for _, run := range runs {
			if run.ParentRunID == r.run.ID {
				return false, nil
			}
		}
		incoming := 0
		for _, delegation := range delegations {
			if delegation.ParentRunID == r.run.ID {
				return false, nil
			}
			if delegation.RunID == r.run.ID {
				if delegation.State != "waiting" || delegation.ParentRunID != r.run.ParentRunID {
					return false, nil
				}
				incoming++
			}
		}
		if r.run.StepKind.String == "member" && incoming != 1 {
			return false, invalidCollaborationLedger("unclaimed member has no unique delegation")
		}
	}
	evidence, err := r.q.CollaborationRunHasPreparationStartEvidence(ctx, r.run.ID)
	if err != nil || evidence {
		return false, err
	}
	if err = r.validateGroup(ctx, r.target, r.group, runs); err != nil {
		return false, err
	}
	if _, err = r.q.GetCollaborationRuntimeReservation(ctx, r.run.ID); err == nil {
		if err = r.requireReservation(ctx); err != nil {
			return false, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	} else if !r.run.ParentRunID.Valid {
		return false, invalidCollaborationLedger("unclaimed root has no reservation")
	}
	r.run, err = r.q.SettleNeverLaunchedCollaborationRun(ctx, r.run.ID)
	if err != nil {
		return false, err
	}
	if err = completeProvenRuntimeRun(ctx, r); err != nil {
		return false, err
	}
	return true, recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, r.run.ID, "runtime_preparation_refused")
}

func (s *CollaborationService) freezePreparedPrompt(ctx context.Context, r *collaborationRuntimeState, inputs []protocol.CollaborationRunInput, encoded []byte, registration *protocol.CollaborationRuntimeCapabilityRegistration) error {
	files, err := s.preparationAttachments(ctx, r.q, r.access.Issue.ID, r.access.Issue.WorkspaceID, inputs)
	if err != nil {
		return err
	}
	if err = preparationWithinLimits(files, registration.Capabilities); err != nil {
		return err
	}
	invocation, err := preparationInvocation(ctx, r.q, r.run.AgentID, registration.Capabilities.Provider.ProtocolFamily)
	if err != nil {
		return err
	}
	manifest := protocol.CollaborationPreparation{Version: registration.Capabilities.PreparationVersion,
		Identity: collaborationRunIdentity(r.run, r.access.Issue.ID), CommandIDs: make([]string, 0, len(inputs)),
		Resource: protocol.CollaborationPreparationResource{AgentID: util.UUIDToString(r.run.AgentID), RuntimeID: util.UUIDToString(r.run.RuntimeID),
			OwnerUserID: util.UUIDToString(r.run.ResourceOwnerUserID), ExecutionWorkspaceID: util.UUIDToString(r.run.ExecutionWorkspaceID),
			TargetEpoch: strconv.FormatInt(r.run.TargetEpoch.Int64, 10), ConfigVersion: strconv.FormatInt(r.run.ConfigVersion.Int64, 10), RuntimeConfigEpoch: registration.RuntimeConfigEpoch},
		Provider: registration.Capabilities.Provider, Invocation: invocation, Attachments: files, Git: registration.Capabilities.Git, Limits: registration.Capabilities.Limits,
		Host: registration.Capabilities.Host}
	if manifest.Host != nil {
		manifest.CapabilityDigest, err = protocol.CollaborationRuntimeCapabilityDigest(registration.Capabilities)
		if err != nil {
			return err
		}
		actor, err := r.q.GetAgent(ctx, r.run.AgentID)
		if err != nil {
			return err
		}
		manifest.Resource.AgentOwnerUserID = util.UUIDToString(actor.OwnerID)
	}
	for _, input := range inputs {
		manifest.CommandIDs = append(manifest.CommandIDs, input.CommandID)
	}
	if protocol.SealCollaborationPreparation(&manifest) != nil || protocol.ValidateCollaborationPreparation(&manifest, manifest.Identity, inputs) != nil {
		return collaborationError(http.StatusConflict, "runtime_preparation_unavailable")
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	r.run, err = r.q.FreezeCollaborationPreparedRunPrompt(ctx, db.FreezeCollaborationPreparedRunPromptParams{RunID: r.run.ID, PromptSnapshot: encoded, PreparationSnapshot: raw})
	return err
}

func (s *CollaborationService) readRuntimePreparation(ctx context.Context, r *collaborationRuntimeState) (protocol.CollaborationPreparation, error) {
	var out protocol.CollaborationPreparation
	if r.run.PreparationSnapshot == nil || r.run.PromptSnapshot == nil {
		return out, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
	}
	if err := decodeCollaborationPreparationJSON(r.run.PreparationSnapshot, &out); err != nil {
		return out, err
	}
	inputs, err := decodeCollaborationRoster[protocol.CollaborationRunInput](r.run.PromptSnapshot)
	if err != nil {
		return out, err
	}
	if protocol.ValidateCollaborationPreparation(&out, collaborationRunIdentity(r.run, r.access.Issue.ID), inputs) != nil ||
		out.Resource.AgentID != util.UUIDToString(r.run.AgentID) || out.Resource.RuntimeID != util.UUIDToString(r.run.RuntimeID) ||
		out.Resource.OwnerUserID != util.UUIDToString(r.run.ResourceOwnerUserID) || out.Resource.ExecutionWorkspaceID != util.UUIDToString(r.run.ExecutionWorkspaceID) ||
		out.Resource.TargetEpoch != strconv.FormatInt(r.run.TargetEpoch.Int64, 10) || out.Resource.ConfigVersion != strconv.FormatInt(r.run.ConfigVersion.Int64, 10) ||
		out.Resource.RuntimeConfigEpoch != strconv.FormatInt(r.run.ResourceEpoch, 10) {
		return out, collaborationError(http.StatusServiceUnavailable, "runtime_preparation_unavailable")
	}
	runtime, err := s.collaborationRuntimeLookup(r.q).Get(ctx, r.run.RuntimeID)
	if err != nil {
		return out, err
	}
	registration, err := currentCollaborationPreparationCapabilities(runtime, r.run.InstallationID, r.run.BindingID, r.run.BindingEpoch)
	if err != nil {
		return out, err
	}
	if registration.RuntimeConfigEpoch != out.Resource.RuntimeConfigEpoch || registration.Capabilities.Provider != out.Provider ||
		registration.Capabilities.Limits != out.Limits || !reflect.DeepEqual(registration.Capabilities.Git, out.Git) || !reflect.DeepEqual(registration.Capabilities.Host, out.Host) {
		return out, collaborationError(http.StatusConflict, "runtime_capability_changed")
	}
	return out, nil
}

func (s *CollaborationService) RuntimePreparation(ctx context.Context, runID pgtype.UUID, token string) (protocol.CollaborationPreparation, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationPreparation, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return protocol.CollaborationPreparation{}, err
		}
		return s.readRuntimePreparation(ctx, r)
	})
}

// The result is private storage data, never an HTTP projection. Authorization
// belongs to the exact mcb run and manifest, without a human viewer fallback.
func (s *CollaborationService) RuntimeAttachment(ctx context.Context, runID, attachmentID pgtype.UUID, version int64, token string) (db.CollaborationAttachment, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (db.CollaborationAttachment, error) {
		var empty db.CollaborationAttachment
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		manifest, err := s.readRuntimePreparation(ctx, r)
		if err != nil {
			return empty, err
		}
		var expected *protocol.CollaborationPreparationAttachment
		for i := range manifest.Attachments {
			file := &manifest.Attachments[i]
			if file.ID == util.UUIDToString(attachmentID) && file.Version == strconv.FormatInt(version, 10) {
				expected = file
				break
			}
		}
		if expected == nil {
			return empty, collaborationError(http.StatusNotFound, "attachment_not_found")
		}
		row, err := r.q.GetCollaborationAttachment(ctx, attachmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, collaborationError(http.StatusNotFound, "attachment_not_found")
		}
		if err != nil {
			return empty, err
		}
		if row.IssueID != r.access.Issue.ID || row.SourceWorkspaceID != r.access.Issue.WorkspaceID || !row.DeliveredAt.Valid || row.DiscardedAt.Valid ||
			row.Version != version || row.Name != expected.Name || row.ContentType != expected.ContentType || row.SizeBytes != expected.SizeBytes {
			return empty, collaborationError(http.StatusNotFound, "attachment_not_found")
		}
		return row, validateCollaborationStoredAttachment(row)
	})
}

func (s *CollaborationService) RecordRuntimePreparation(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationPreparationObservation) (protocol.CollaborationPreparationObservationReceipt, error) {
	var empty protocol.CollaborationPreparationObservationReceipt
	if protocol.ValidateCollaborationPreparationObservation(p) != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_preparation_observation")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationPreparationObservationReceipt, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		manifest, err := s.readRuntimePreparation(ctx, r)
		if err != nil {
			return empty, err
		}
		if p.Identity != manifest.Identity || p.ManifestDigest != manifest.ManifestDigest || p.Stage == "git" && manifest.Git == nil {
			return empty, collaborationError(http.StatusConflict, "stale_preparation")
		}
		if !collaborationRuntimeMessageMode(p.ProtocolVersion, p.Containment, r.run.Containment.String) {
			return empty, collaborationError(http.StatusConflict, "execution_identity_mismatch")
		}
		seq, _ := collaborationRuntimeVersion(p.AttemptSeq, false)
		attempt, err := r.q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		if err != nil {
			return empty, err
		}
		if attempt.AttemptNonce != p.AttemptNonce || attempt.ClaimGeneration != r.run.ClaimGeneration || r.run.AuthorizedAttemptSeq != seq {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		receipt := protocol.CollaborationPreparationObservationReceipt{Identity: p.Identity, EventID: p.EventID, ManifestDigest: p.ManifestDigest, Recorded: true}
		prior, err := r.q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: p.EventID})
		if err == nil {
			var saved protocol.CollaborationPreparationObservation
			if prior.Kind != "preparation" || decodeCollaborationPreparationJSON(prior.Payload, &saved) != nil || !reflect.DeepEqual(saved, p) {
				return empty, collaborationError(http.StatusConflict, "preparation_observation_conflict")
			}
			return receipt, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return empty, err
		}
		_, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: runID,
			EventID: p.EventID, Kind: "preparation", Payload: raw})
		return receipt, err
	})
}
