package service

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The retired pre-consent mutation is retained only as a synthetic fixture for
// the earlier FIFO/invalidation regressions. Production callers must stage and
// consent a proposal before CommitTargetSwitch; no handler exposes this helper.
type CollaborationReconfigureTargetParams struct {
	CollaborationQueueRequest
	Control CollaborationTargetControl
	Roster  []collaboration.TargetResource
}

// ReconfigureTarget keeps the typed target and immutable input history. It
// requires the old resource's proof barrier to be clear and leaves a switch
// hold for fresh consent. This pending configuration is not a completed Switch.
func (s *CollaborationService) ReconfigureTarget(ctx context.Context, p CollaborationReconfigureTargetParams) (CollaborationQueueReceipt, error) {
	control := CollaborationStopQueueParams{Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{p.Control}}
	if !normalizeCollaborationControl(&control) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_configuration")
	}
	p.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	sort.Slice(p.Roster, func(i, j int) bool { return p.Roster[i].AgentID < p.Roster[j].AgentID })
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "reconfigure_target", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.Invite {
			return t.receipt("forbidden", http.StatusForbidden), nil
		}
		if !t.access.Capabilities.SubmitCommand {
			return t.receipt("issue_closed", http.StatusConflict), nil
		}
		if code, err := t.checkControl(ctx, control); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return t.controlReceipt(code, http.StatusConflict, control), nil
		}
		target, _ := t.target(p.Control.TargetID)
		deny := func(code string) (CollaborationQueueReceipt, error) {
			return t.targetReceipt(code, http.StatusConflict, target), nil
		}
		if !validCollaborationRoster(collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster) {
			return t.receipt("invalid_target_configuration", http.StatusBadRequest), nil
		}
		if target.QueueState != "paused" && !target.HoldReason.Valid {
			return deny("target_pause_required")
		}
		if code, err := t.checkTargetSwitchProof(ctx, target); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code)
		}
		if code, err := t.validateResourceOwners(ctx, p.Roster); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code)
		}
		if _, err := t.q.LockCollaborationTargetGrants(ctx, target.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err := lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return deny(code)
		}
		encoded, err := json.Marshal(p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = t.q.RevokeCollaborationTargetConsents(ctx, db.RevokeCollaborationTargetConsentsParams{TargetID: target.ID, Reason: "configuration_changed"}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		target, err = t.q.ReconfigureCollaborationTarget(ctx, db.ReconfigureCollaborationTargetParams{TargetID: target.ID, SessionID: t.session.ID, Roster: encoded, ConfigurationFingerprint: configuration.Fingerprint})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.replaceTarget(target)
		target, err = t.moveReconfiguredTarget(ctx, target)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := t.targetReceipt("target_reconfigured", http.StatusOK, target)
		return r, t.event(ctx, "target_reconfigured", r)
	})
}
