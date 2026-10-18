package service

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A group generation survives settlement. A delayed stop can pause the ended
// group, but must never stop a later group that now occupies the same target.
type CollaborationTargetControl struct {
	TargetID                pgtype.UUID `json:"target_id"`
	ExpectedTargetEpoch     int64       `json:"expected_target_epoch,string"`
	ExpectedConfigVersion   int64       `json:"expected_config_version,string"`
	ExpectedHoldVersion     int64       `json:"expected_hold_version,string,omitempty"`
	ExpectedControlVersion  int64       `json:"expected_control_version,string"`
	ObservedGroupID         pgtype.UUID `json:"observed_group_id"`
	ObservedGroupGeneration int64       `json:"observed_group_generation,string"`
}

type CollaborationStopQueueParams struct {
	CollaborationQueueRequest
	Scope                    collaboration.ControlScope
	ExpectedControlVersion   int64
	ExpectedTargetSetVersion int64
	Targets                  []CollaborationTargetControl
}

type CollaborationResumeQueueParams struct {
	CollaborationStopQueueParams
	ExpectedExecutionEpoch int64
}

type CollaborationTargetControlResult struct {
	TargetID        pgtype.UUID `json:"target_id"`
	ChannelID       pgtype.UUID `json:"channel_id"`
	Code            string      `json:"code"`
	QueueState      string      `json:"queue_state"`
	ControlVersion  int64       `json:"control_version,string"`
	GroupGeneration int64       `json:"group_generation,string"`
	GroupID         pgtype.UUID `json:"group_id"`
	GroupSettled    bool        `json:"group_settled"`
	StoppedRuns     int         `json:"stopped_runs"`
	UnsettledRuns   int         `json:"unsettled_runs"`
}

type CollaborationChannelControlResult struct {
	ChannelID       pgtype.UUID `json:"channel_id"`
	Code            string      `json:"code"`
	QueueState      string      `json:"queue_state"`
	ControlVersion  int64       `json:"control_version,string"`
	GroupGeneration int64       `json:"group_generation,string"`
	GroupID         pgtype.UUID `json:"group_id"`
	GroupSettled    bool        `json:"group_settled"`
	StoppedRuns     int         `json:"stopped_runs"`
	UnsettledRuns   int         `json:"unsettled_runs"`
}

type CollaborationTargetHoldResult struct {
	TargetID    pgtype.UUID                     `json:"target_id"`
	ChannelID   pgtype.UUID                     `json:"channel_id"`
	Code        string                          `json:"code"`
	HoldReason  *collaboration.TargetHoldReason `json:"hold_reason"`
	HoldVersion int64                           `json:"hold_version,string"`
}

func (r CollaborationTargetControlResult) channelResult() CollaborationChannelControlResult {
	return CollaborationChannelControlResult{ChannelID: r.ChannelID, Code: r.Code, QueueState: r.QueueState,
		ControlVersion: r.ControlVersion, GroupGeneration: r.GroupGeneration, GroupID: r.GroupID,
		GroupSettled: r.GroupSettled, StoppedRuns: r.StoppedRuns, UnsettledRuns: r.UnsettledRuns}
}

func normalizeCollaborationControl(p *CollaborationStopQueueParams) bool {
	if p.Scope != collaboration.ControlIssue && p.Scope != collaboration.ControlTarget {
		return false
	}
	if p.Scope == collaboration.ControlTarget {
		if len(p.Targets) != 1 || p.ExpectedControlVersion != 0 || p.ExpectedTargetSetVersion != 0 {
			return false
		}
	} else if p.ExpectedControlVersion < 1 || p.ExpectedTargetSetVersion < 1 {
		return false
	}
	p.Targets = append([]CollaborationTargetControl(nil), p.Targets...)
	seen := make(map[pgtype.UUID]bool, len(p.Targets))
	for _, target := range p.Targets {
		if !target.TargetID.Valid || target.ExpectedTargetEpoch < 1 || target.ExpectedConfigVersion < 1 ||
			target.ExpectedControlVersion < 1 || target.ObservedGroupGeneration < 0 || seen[target.TargetID] ||
			(target.ObservedGroupID.Valid && target.ObservedGroupGeneration == 0) {
			return false
		}
		seen[target.TargetID] = true
	}
	sort.Slice(p.Targets, func(i, j int) bool {
		return string(p.Targets[i].TargetID.Bytes[:]) < string(p.Targets[j].TargetID.Bytes[:])
	})
	return true
}

func (t *collaborationQueueTx) checkControl(ctx context.Context, p CollaborationStopQueueParams) (string, error) {
	if p.Scope == collaboration.ControlIssue {
		if p.ExpectedTargetSetVersion != t.session.TargetSetVersion || len(p.Targets) != len(t.targets) {
			return "stale_scope", nil
		}
		if p.ExpectedControlVersion != t.session.ControlVersion {
			return "stale_control_version", nil
		}
	}
	for _, observed := range p.Targets {
		target, ok := t.target(observed.TargetID)
		if !ok {
			return "stale_scope", nil
		}
		if channel, exists := t.channel(target.ChannelID); exists {
			target.ActiveGroupID, target.GroupGeneration = channel.ActiveGroupID, channel.GroupGeneration
			target.ControlVersion = channel.ControlVersion
		}
		if target.GroupGeneration != observed.ObservedGroupGeneration ||
			(target.ActiveGroupID.Valid && target.ActiveGroupID != observed.ObservedGroupID) {
			return "stale_execution", nil
		}
		if !target.ActiveGroupID.Valid && observed.ObservedGroupID.Valid {
			previous, err := t.q.GetCollaborationExecutionGroup(ctx, db.GetCollaborationExecutionGroupParams{SessionID: t.session.ID, ID: observed.ObservedGroupID})
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && ((previous.TargetID != target.ID && (!target.ChannelID.Valid || previous.ChannelID != target.ChannelID)) || !previous.SettledAt.Valid || previous.Generation != observed.ObservedGroupGeneration)) {
				return "stale_execution", nil
			}
			if err != nil {
				return "", err
			}
		}
		if target.TargetEpoch != observed.ExpectedTargetEpoch || target.ConfigVersion != observed.ExpectedConfigVersion {
			return "stale_target", nil
		}
		if target.ControlVersion != observed.ExpectedControlVersion {
			return "stale_control_version", nil
		}
	}
	return "", nil
}

func (t *collaborationQueueTx) controlReceipt(code string, status int, p CollaborationStopQueueParams) CollaborationQueueReceipt {
	if p.Scope == collaboration.ControlTarget {
		if target, ok := t.target(p.Targets[0].TargetID); ok {
			return t.targetReceipt(code, status, target)
		}
	}
	return t.receipt(code, status)
}

// StopQueue persists two distinct control scopes. Sealing a group and stopping
// all its descendants share the source/target/group transaction; neither action
// supplies a proof for a run that may have obtained a launch permit.
func (s *CollaborationService) StopQueue(ctx context.Context, p CollaborationStopQueueParams) (CollaborationQueueReceipt, error) {
	if !normalizeCollaborationControl(&p) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_stop")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "stop_queue", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if code, err := t.checkControl(ctx, p); err != nil || code != "" {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return t.controlReceipt(code, http.StatusConflict, p), nil
		}
		selected := make(map[pgtype.UUID]string, len(p.Targets))
		for _, target := range p.Targets {
			selected[target.TargetID] = "stopped"
		}
		if p.Scope == collaboration.ControlIssue {
			var err error
			t.session, err = t.q.PauseCollaborationSession(ctx, db.PauseCollaborationSessionParams{SessionID: t.session.ID, PauseReason: "participant_stop"})
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		r, err := t.stopTargets(ctx, selected, p.Scope == collaboration.ControlIssue, "participant_stop", true)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if p.Scope == collaboration.ControlTarget {
			target, _ := t.target(p.Targets[0].TargetID)
			r.Scope, r.TargetID, r.QueueState, r.ControlVersion = collaboration.ControlTarget, target.ID, target.QueueState, target.ControlVersion
			r.ChannelID = target.ChannelID
			for _, result := range r.Targets {
				if result.TargetID == target.ID {
					r.GroupID, r.GroupSettled = result.GroupID, result.GroupSettled
				}
			}
		}
		return r, t.event(ctx, "queue_stop_requested", r)
	})
}

// ResumeQueue validates an individual target atomically. Issue Resume first
// checks global authority, then resolves channel proof and target holds as
// independent results. Neither path records an intention to resume later.
func (s *CollaborationService) ResumeQueue(ctx context.Context, p CollaborationResumeQueueParams) (CollaborationQueueReceipt, error) {
	if p.ExpectedExecutionEpoch < 1 || !normalizeCollaborationControl(&p.CollaborationStopQueueParams) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resume")
	}
	for _, observed := range p.Targets {
		if observed.ExpectedHoldVersion < 1 {
			return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resume")
		}
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "resume_queue", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		result := func(code string, status int) (CollaborationQueueReceipt, error) {
			return t.controlReceipt(code, status, p.CollaborationStopQueueParams), nil
		}
		if t.session.QueueState == "closed" {
			return result("issue_closed", http.StatusConflict)
		}
		if p.Scope == collaboration.ControlTarget && t.session.QueueState != "active" {
			return result("issue_paused", http.StatusConflict)
		}
		if !t.access.Capabilities.SubmitCommand || (t.access.Issue.AdmissionStatus != "not_required" && t.access.Issue.AdmissionStatus != "accepted") {
			return result("command_unavailable", http.StatusConflict)
		}
		if code, err := t.checkControl(ctx, p.CollaborationStopQueueParams); err != nil || code != "" {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return result(code, http.StatusConflict)
		}
		for _, observed := range p.Targets {
			target, _ := t.target(observed.TargetID)
			if target.HoldVersion != observed.ExpectedHoldVersion {
				return result("stale_hold", http.StatusConflict)
			}
		}
		if t.gate.Epoch != p.ExpectedExecutionEpoch {
			return result("stale_admission", http.StatusConflict)
		}
		if !t.gate.Enabled {
			return result("collaboration_execution_unavailable", http.StatusServiceUnavailable)
		}
		selected := make(map[pgtype.UUID]bool, len(p.Targets))
		selectedChannels := make(map[pgtype.UUID]bool)
		for _, observed := range p.Targets {
			selected[observed.TargetID] = true
			target, _ := t.target(observed.TargetID)
			if target.ChannelID.Valid {
				selectedChannels[target.ChannelID] = true
			}
		}
		if p.Scope == collaboration.ControlIssue {
			for _, channel := range t.channels {
				selectedChannels[channel.ID] = true
			}
		}
		// Only the selected target's authority can refuse a target Resume. An
		// unrelated alias, even the head of a waiting batch, has its own hold.
		reasons := make(map[pgtype.UUID]string, len(selected))
		supportReasons := make(map[pgtype.UUID]string, len(selected))
		for _, target := range t.targets {
			if !selected[target.ID] {
				continue
			}
			code, err := t.targetResumeReason(ctx, target)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			reasons[target.ID] = code
			if p.Scope == collaboration.ControlTarget && code != "" {
				return result(code, http.StatusConflict)
			}
			if code == "" {
				supportReasons[target.ID], err = collaborationRuntimeTargetSupported(ctx, t.q, target)
				if err != nil {
					return CollaborationQueueReceipt{}, err
				}
			}
		}
		barriers, unsettled, err := t.resumeChannelBarriers(ctx, selectedChannels)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if p.Scope == collaboration.ControlTarget {
			target, _ := t.target(p.Targets[0].TargetID)
			if barriers[target.ChannelID] {
				return result("stop_unconfirmed", http.StatusConflict)
			}
			if code := supportReasons[target.ID]; code != "" {
				return result(code, http.StatusConflict)
			}
		}
		for _, target := range t.targets {
			if selected[target.ID] && reasons[target.ID] == "" && !barriers[target.ChannelID] {
				reasons[target.ID] = supportReasons[target.ID]
			}
		}
		changed := false
		if p.Scope == collaboration.ControlIssue && t.session.QueueState != "active" {
			t.session, err = t.q.ResumeCollaborationSession(ctx, t.session.ID)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			changed = true
		}
		r := t.receipt("resumed", http.StatusOK)
		for _, channel := range t.channels {
			if !selectedChannels[channel.ID] {
				continue
			}
			code := "already_active"
			if barriers[channel.ID] {
				code = "stop_unconfirmed"
				r.Code = "partially_resumed"
			} else if channel.QueueState != "active" {
				channel, err = t.q.ResumeCollaborationChannel(ctx, channel.ID)
				if err != nil {
					return CollaborationQueueReceipt{}, err
				}
				if err = t.mirrorChannel(ctx, channel); err != nil {
					return CollaborationQueueReceipt{}, err
				}
				code, changed = "resumed", true
			}
			r.Channels = append(r.Channels, CollaborationChannelControlResult{ChannelID: channel.ID, Code: code,
				QueueState: channel.QueueState, ControlVersion: channel.ControlVersion, GroupGeneration: channel.GroupGeneration, GroupID: channel.ActiveGroupID, UnsettledRuns: unsettled[channel.ID]})
			for _, target := range t.targets {
				if target.ChannelID == channel.ID {
					r.Targets = append(r.Targets, CollaborationTargetControlResult{TargetID: target.ID, ChannelID: channel.ID, Code: code,
						QueueState: channel.QueueState, ControlVersion: channel.ControlVersion, GroupGeneration: channel.GroupGeneration, GroupID: channel.ActiveGroupID, UnsettledRuns: unsettled[channel.ID]})
				}
			}
		}
		for _, target := range t.targets {
			if !selected[target.ID] {
				continue
			}
			code := reasons[target.ID]
			if p.Scope == collaboration.ControlIssue && !target.HoldReason.Valid && code == "" {
				continue
			}
			if code != "" {
				// Global Resume may discover invalid current authority. Persist
				// that target's hold without pausing any otherwise healthy lane.
				hold := collaboration.TargetHoldReason(target.HoldReason.String)
				switch code {
				case "grant_required":
					hold = collaboration.HoldGrantRequired
				case "config_invalid":
					hold = collaboration.HoldConfigInvalid
				default:
					if !target.HoldReason.Valid {
						hold = collaboration.HoldConfigInvalid
					}
				}
				beforeVersion := target.HoldVersion
				target, err = t.setTargetHold(ctx, target, hold, false)
				if err != nil {
					return CollaborationQueueReceipt{}, err
				}
				changed = changed || target.HoldVersion != beforeVersion
				r.Code = "partially_resumed"
			} else if barriers[target.ChannelID] {
				code, r.Code = "stop_unconfirmed", "partially_resumed"
			} else if target.HoldReason.Valid {
				target, err = t.setTargetHold(ctx, target, "", false)
				if err != nil {
					return CollaborationQueueReceipt{}, err
				}
				code, changed = "hold_released", true
			} else {
				code = "already_active"
			}
			r.TargetHolds = append(r.TargetHolds, collaborationTargetHoldResult(target, code))
		}
		if p.Scope == collaboration.ControlTarget {
			target, _ := t.target(p.Targets[0].TargetID)
			r.Scope, r.TargetID, r.ChannelID = p.Scope, target.ID, target.ChannelID
			r.QueueState, r.ControlVersion, r.GroupID = target.QueueState, target.ControlVersion, target.ActiveGroupID
			if !changed {
				r.Code = "already_active"
			}
		}
		if changed {
			return r, t.event(ctx, "queue_resumed", r)
		}
		return r, nil
	})
}

func (t *collaborationQueueTx) targetResumeReason(ctx context.Context, target db.CollaborationTarget) (string, error) {
	channel, exists := t.channel(target.ChannelID)
	if !exists {
		return "config_invalid", nil
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil || !validCollaborationRoster(collaboration.TargetKind(target.Kind), target.ResourceID, roster) {
		return "config_invalid", nil
	}
	root, err := collaborationRootAgent(target)
	if err != nil || root != channel.RootAgentID {
		return "config_invalid", nil
	}
	code, err := t.authorizeTargetInput(ctx, target)
	if code == "target_configuration_changed" {
		code = "config_invalid"
	}
	if code != "" || err != nil {
		return code, err
	}
	return "", nil
}

// A healthy busy active channel does not need termination proof to retain its
// active intent. Paused work, forced stops and contradictory retained pointers
// do. Read and validate the full group ledger without settling any of it here.
func (t *collaborationQueueTx) resumeChannelBarriers(ctx context.Context, selected map[pgtype.UUID]bool) (map[pgtype.UUID]bool, map[pgtype.UUID]int, error) {
	if _, err := t.q.LockCollaborationSessionCommands(ctx, t.session.ID); err != nil {
		return nil, nil, err
	}
	groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return nil, nil, err
	}
	runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return nil, nil, err
	}
	barriers := make(map[pgtype.UUID]bool)
	unsettled := make(map[pgtype.UUID]int)
	byID := make(map[pgtype.UUID]db.CollaborationExecutionGroup, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
		if !selected[group.ChannelID] {
			continue
		}
		channel, exists := t.channel(group.ChannelID)
		target, targetExists := t.target(group.TargetID)
		if !exists || !targetExists || target.ChannelID != group.ChannelID || channel.ActiveGroupID != group.ID || channel.GroupGeneration != group.Generation {
			barriers[group.ChannelID] = true
			continue
		}
		allRuns, err := t.q.LockCollaborationGroupRuns(ctx, group.ID)
		if err != nil {
			return nil, nil, err
		}
		if err = t.validateGroup(ctx, target, group, allRuns); err != nil {
			if !errors.Is(err, errInvalidCollaborationLedger) {
				return nil, nil, err
			}
			barriers[group.ChannelID] = true
		}
		if len(allRuns) == 0 || channel.QueueState != "active" || group.ForcedDisposition.Valid {
			barriers[group.ChannelID] = true
		}
	}
	for _, run := range runs {
		unsettled[run.ChannelID]++
		group, exists := byID[run.GroupID]
		if !exists || group.ChannelID != run.ChannelID || group.BatchID != run.BatchID || group.CommandID != run.CommandID || group.TargetID != run.TargetID || run.ForcedDisposition.Valid {
			barriers[run.ChannelID] = true
			if target, exists := t.target(run.TargetID); exists {
				barriers[target.ChannelID] = true
			}
		}
	}
	for _, channel := range t.channels {
		if !selected[channel.ID] {
			continue
		}
		if channel.ActiveGroupID.Valid {
			group, exists := byID[channel.ActiveGroupID]
			if !exists || group.ChannelID != channel.ID {
				barriers[channel.ID] = true
			}
		}
		for _, target := range t.targets {
			if target.ChannelID == channel.ID && (target.ActiveGroupID != channel.ActiveGroupID || target.GroupGeneration != channel.GroupGeneration || target.QueueState != channel.QueueState || target.ControlVersion != channel.ControlVersion || target.PauseReason != channel.PauseReason) {
				barriers[channel.ID] = true
			}
		}
	}
	return barriers, unsettled, nil
}

func (t *collaborationQueueTx) pauseTarget(ctx context.Context, target db.CollaborationTarget, reason string) (db.CollaborationTarget, error) {
	if target.ChannelID.Valid {
		channel, exists := t.channel(target.ChannelID)
		if !exists {
			return db.CollaborationTarget{}, invalidCollaborationLedger("target channel missing")
		}
		// Lifecycle fanout may visit several aliases of the same channel. One
		// transition advances its control once; explicit Stop advances below.
		if channel.QueueState != "paused" || channel.PauseReason != reason {
			var err error
			channel, err = t.q.PauseCollaborationChannel(ctx, db.PauseCollaborationChannelParams{ChannelID: channel.ID, Reason: reason})
			if err != nil {
				return db.CollaborationTarget{}, err
			}
			if err = t.mirrorChannel(ctx, channel); err != nil {
				return db.CollaborationTarget{}, err
			}
		}
		updated, _ := t.target(target.ID)
		return updated, nil
	}
	updated, err := t.q.PauseCollaborationTarget(ctx, db.PauseCollaborationTargetParams{SessionID: t.session.ID, TargetID: target.ID, Reason: reason})
	if err == nil {
		t.replaceTarget(updated)
	}
	return updated, err
}

func (t *collaborationQueueTx) channel(id pgtype.UUID) (db.CollaborationChannel, bool) {
	if id.Valid {
		for _, channel := range t.channels {
			if channel.ID == id {
				return channel, true
			}
		}
	}
	return db.CollaborationChannel{}, false
}

func (t *collaborationQueueTx) mirrorChannel(ctx context.Context, channel db.CollaborationChannel) error {
	aliases, err := t.q.MirrorCollaborationChannelControl(ctx, channel.ID)
	if err != nil {
		return err
	}
	for i := range t.channels {
		if t.channels[i].ID == channel.ID {
			t.channels[i] = channel
			break
		}
	}
	for _, alias := range aliases {
		t.replaceTarget(alias)
	}
	return nil
}

func (t *collaborationQueueTx) stopTargets(ctx context.Context, selected map[pgtype.UUID]string, wholeIssue bool, reason string, advanceControl bool) (CollaborationQueueReceipt, error) {
	if _, err := t.q.LockCollaborationSessionCommands(ctx, t.session.ID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	selectedChannels := make(map[pgtype.UUID]string)
	for id, disposition := range selected {
		target, exists := t.target(id)
		if !exists {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("selected target missing")
		}
		if target.ChannelID.Valid {
			if _, exists := t.channel(target.ChannelID); !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("selected channel missing")
			}
			if selectedChannels[target.ChannelID] == "" || disposition == "stopped" {
				selectedChannels[target.ChannelID] = disposition
			}
		}
	}
	// Unclassifiable typed work still belongs to this issue. Whole-issue
	// control must not silently omit its stop intent.
	if wholeIssue {
		for _, channel := range t.channels {
			if selectedChannels[channel.ID] == "" {
				selectedChannels[channel.ID] = "stopped"
			}
		}
		for _, group := range groups {
			if _, exists := t.target(group.TargetID); !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled group target missing")
			}
		}
		for _, run := range runs {
			if _, exists := t.target(run.TargetID); !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled run target missing")
			}
		}
	}
	r := t.receipt("already_ended", http.StatusOK)
	results := make(map[pgtype.UUID]CollaborationTargetControlResult, len(selectedChannels))
	for _, channel := range t.channels {
		disposition := selectedChannels[channel.ID]
		if disposition == "" {
			continue
		}
		current := channel
		if advanceControl || channel.QueueState != "paused" {
			current, err = t.q.PauseCollaborationChannel(ctx, db.PauseCollaborationChannelParams{ChannelID: channel.ID, Reason: reason})
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		if err = t.mirrorChannel(ctx, current); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		result := CollaborationTargetControlResult{ChannelID: channel.ID, Code: "already_ended", QueueState: current.QueueState, ControlVersion: current.ControlVersion, GroupGeneration: current.GroupGeneration}
		found := false
		for _, group := range groups {
			if group.ChannelID != channel.ID {
				continue
			}
			if found || channel.ActiveGroupID != group.ID {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("channel group pointer mismatch")
			}
			found = true
			target, exists := t.target(group.TargetID)
			if !exists || target.ChannelID != channel.ID {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("group target channel mismatch")
			}
			stopped, err := t.stopGroup(ctx, target, group, disposition, reason)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			result.Code = "stop_requested"
			result.GroupID, result.GroupSettled = group.ID, stopped.GroupSettled
			result.StoppedRuns, result.UnsettledRuns = stopped.StoppedRuns, stopped.UnsettledRuns
			r.Code = "stop_requested"
		}
		if !found && channel.ActiveGroupID.Valid {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("active group missing")
		}
		results[channel.ID] = result
		r.Channels = append(r.Channels, result.channelResult())
	}
	for _, target := range t.targets {
		if result, exists := results[target.ChannelID]; exists {
			result.TargetID = target.ID
			r.Targets = append(r.Targets, result)
			continue
		}
		if selected[target.ID] == "" {
			continue
		}
		// An unbound configuration has no runnable channel. It remains
		// stoppable, but cannot hide a group that needs termination evidence.
		if target.ActiveGroupID.Valid {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("active target channel missing")
		}
		if advanceControl || target.QueueState != "paused" {
			target, err = t.pauseTarget(ctx, target, reason)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		r.Targets = append(r.Targets, CollaborationTargetControlResult{TargetID: target.ID, Code: "already_ended", QueueState: target.QueueState, ControlVersion: target.ControlVersion, GroupGeneration: target.GroupGeneration})
	}
	for _, group := range groups {
		if selected[group.TargetID] != "" && selectedChannels[group.ChannelID] == "" {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled group channel missing")
		}
	}
	for _, run := range runs {
		if selected[run.TargetID] == "" && selectedChannels[run.ChannelID] == "" {
			continue
		}
		found := false
		for _, group := range groups {
			if group.ID == run.GroupID && group.TargetID == run.TargetID && group.ChannelID == run.ChannelID && group.BatchID == run.BatchID {
				found = true
				break
			}
		}
		if !found {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled run group missing")
		}
	}

	return r, nil
}

// stopRun changes only this run. Typed command/group settlement belongs to the
// group owner; the parent's proof cannot release its descendants' FIFO barrier.
func (t *collaborationQueueTx) stopRun(ctx context.Context, run db.CollaborationRun, disposition, reason string) (CollaborationQueueReceipt, error) {
	if run.SessionID != t.session.ID {
		return CollaborationQueueReceipt{}, invalidCollaborationLedger("run session mismatch")
	}
	neverLaunched := collaborationRunNeverLaunched(run)
	if disposition == "cancelled" && !neverLaunched {
		disposition = "stopped"
	}
	var err error
	switch disposition {
	case "stopped":
		run, err = t.q.RequestCollaborationRunStop(ctx, db.RequestCollaborationRunStopParams{RunID: run.ID, DispositionReason: reason})
	case "cancelled":
		run, err = t.q.RequestCollaborationRunCancellation(ctx, db.RequestCollaborationRunCancellationParams{RunID: run.ID, Reason: reason})
	default:
		return CollaborationQueueReceipt{}, errors.New("invalid collaboration disposition")
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if err = t.q.RequestCollaborationTerminationStop(ctx, run.ID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if err = t.q.RevokeCollaborationRunBusinessCapabilities(ctx, run.ID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	r := t.receipt("stop_requested", http.StatusOK)
	r.RunID = run.ID
	if !neverLaunched {
		return r, nil
	}
	run, err = t.q.SettleNeverLaunchedCollaborationRun(ctx, run.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	r.Code, r.RunSettled = "stopped_before_launch", true

	return r, nil
}

func collaborationRunNeverLaunched(run db.CollaborationRun) bool {
	switch run.ProgressState {
	case "prepared", "dispatched", "preparing":
		return run.AuthorizedAttemptSeq == 0 && !run.StartedAt.Valid && run.ProofState == "unknown" && (!run.BusinessOutcome.Valid || run.BusinessOutcome.String == "failure")
	default:
		return false
	}
}
