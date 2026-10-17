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
	QueueState      string      `json:"queue_state"`
	ControlVersion  int64       `json:"control_version,string"`
	GroupGeneration int64       `json:"group_generation,string"`
	GroupID         pgtype.UUID `json:"group_id"`
	GroupSettled    bool        `json:"group_settled"`
	StoppedRuns     int         `json:"stopped_runs"`
	UnsettledRuns   int         `json:"unsettled_runs"`
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
		if target.GroupGeneration != observed.ObservedGroupGeneration ||
			(target.ActiveGroupID.Valid && target.ActiveGroupID != observed.ObservedGroupID) {
			return "stale_execution", nil
		}
		if !target.ActiveGroupID.Valid && observed.ObservedGroupID.Valid {
			previous, err := t.q.GetCollaborationExecutionGroup(ctx, db.GetCollaborationExecutionGroupParams{SessionID: t.session.ID, ID: observed.ObservedGroupID})
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (previous.TargetID != target.ID || !previous.SettledAt.Valid || previous.Generation != observed.ObservedGroupGeneration)) {
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
		r, err := t.stopTargets(ctx, selected, p.Scope == collaboration.ControlIssue, "stopped", "participant_stop", true)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if p.Scope == collaboration.ControlTarget {
			target, _ := t.target(p.Targets[0].TargetID)
			r.Scope, r.TargetID, r.QueueState, r.ControlVersion = collaboration.ControlTarget, target.ID, target.QueueState, target.ControlVersion
			if len(r.Targets) == 1 {
				r.GroupID, r.GroupSettled = r.Targets[0].GroupID, r.Targets[0].GroupSettled
			}
		}
		return r, t.event(ctx, "queue_stop_requested", r)
	})
}

// ResumeQueue persists refusal only. It has no active-state writer until the
// real resource/installation/supervisor protocol can prove safe admission. A
// denial never becomes a stored request to resume automatically after repair.
func (s *CollaborationService) ResumeQueue(ctx context.Context, p CollaborationResumeQueueParams) (CollaborationQueueReceipt, error) {
	if p.ExpectedExecutionEpoch < 1 || !normalizeCollaborationControl(&p.CollaborationStopQueueParams) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resume")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "resume_queue", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		result := func(code string, status int) (CollaborationQueueReceipt, error) {
			return t.controlReceipt(code, status, p.CollaborationStopQueueParams), nil
		}
		if p.Scope == collaboration.ControlTarget && t.session.QueueState != "active" {
			return result("issue_paused", http.StatusConflict)
		}
		if !t.access.Capabilities.SubmitCommand {
			return result("command_unavailable", http.StatusConflict)
		}
		groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		selected := make(map[pgtype.UUID]bool, len(p.Targets))
		for _, target := range p.Targets {
			selected[target.TargetID] = true
		}
		// Unresolved old work cannot be classified as independent of any new
		// target. Its historical pointer or any retained run remains a barrier.
		if t.session.ActiveRunID.Valid {
			return result("stop_unconfirmed", http.StatusConflict)
		}
		for _, run := range runs {
			if run.LegacyUnresolved || !run.TargetID.Valid || p.Scope == collaboration.ControlIssue || selected[run.TargetID] {
				return result("stop_unconfirmed", http.StatusConflict)
			}
		}
		for _, group := range groups {
			if p.Scope == collaboration.ControlIssue || selected[group.TargetID] {
				return result("stop_unconfirmed", http.StatusConflict)
			}
		}
		for _, target := range t.targets {
			if (p.Scope == collaboration.ControlIssue || selected[target.ID]) && target.ActiveGroupID.Valid {
				return result("stop_unconfirmed", http.StatusConflict)
			}
		}
		if code, err := t.checkControl(ctx, p.CollaborationStopQueueParams); err != nil || code != "" {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return result(code, http.StatusConflict)
		}
		if t.gate.Epoch != p.ExpectedExecutionEpoch {
			return result("stale_admission", http.StatusConflict)
		}
		for _, target := range t.targets {
			if !selected[target.ID] {
				continue
			}
			if target.RevokedAt.Valid {
				return result("grant_required", http.StatusConflict)
			}
			if code, err := t.authorizeTargetInput(ctx, target); err != nil || code != "" {
				if err != nil {
					return CollaborationQueueReceipt{}, err
				}
				return result(code, http.StatusConflict)
			}
		}
		return result("collaboration_execution_unavailable", http.StatusServiceUnavailable)
	})
}

func (t *collaborationQueueTx) pauseTarget(ctx context.Context, target db.CollaborationTarget, reason string) (db.CollaborationTarget, error) {
	updated, err := t.q.PauseCollaborationTarget(ctx, db.PauseCollaborationTargetParams{SessionID: t.session.ID, TargetID: target.ID, Reason: reason})
	if err == nil {
		t.replaceTarget(updated)
	}
	return updated, err
}

func (t *collaborationQueueTx) stopTargets(ctx context.Context, selected map[pgtype.UUID]string, includeLegacy bool, disposition, reason string, advanceControl bool) (CollaborationQueueReceipt, error) {
	groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	for id := range selected {
		if _, exists := t.target(id); !exists {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("selected target missing")
		}
	}
	// Unclassifiable typed work still belongs to this issue. Whole-issue
	// control and legacy invalidation must not silently omit its stop intent.
	if includeLegacy {
		if t.session.ActiveRunID.Valid {
			found := false
			for _, run := range runs {
				if run.ID == t.session.ActiveRunID && run.LegacyUnresolved {
					found = true
					break
				}
			}
			if !found {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("legacy active run missing")
			}
		}
		for _, group := range groups {
			if _, exists := t.target(group.TargetID); !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled group target missing")
			}
		}
		for _, run := range runs {
			if _, exists := t.target(run.TargetID); !run.LegacyUnresolved && !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled run target missing")
			}
		}
	}
	r := t.receipt("already_ended", http.StatusOK)
	for _, target := range t.targets {
		if selected[target.ID] == "" {
			continue
		}
		current := target
		if advanceControl || target.QueueState != "paused" {
			current, err = t.pauseTarget(ctx, target, reason)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		result := CollaborationTargetControlResult{TargetID: target.ID, QueueState: current.QueueState, ControlVersion: current.ControlVersion, GroupGeneration: current.GroupGeneration}
		found := false
		for _, group := range groups {
			if group.TargetID != target.ID {
				continue
			}
			if found || target.ActiveGroupID != group.ID {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("target group pointer mismatch")
			}
			found = true
			stopped, err := t.stopGroup(ctx, target, group, selected[target.ID], reason)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			result.GroupID, result.GroupSettled = group.ID, stopped.GroupSettled
			result.StoppedRuns, result.UnsettledRuns = stopped.StoppedRuns, stopped.UnsettledRuns
			r.Code = "stop_requested"
		}
		if !found && target.ActiveGroupID.Valid {
			return CollaborationQueueReceipt{}, invalidCollaborationLedger("active group missing")
		}
		r.Targets = append(r.Targets, result)
	}
	for _, run := range runs {
		if !run.LegacyUnresolved {
			if selected[run.TargetID] != "" {
				found := false
				for _, group := range groups {
					if group.ID == run.GroupID && group.TargetID == run.TargetID {
						found = true
						break
					}
				}
				if !found {
					return CollaborationQueueReceipt{}, invalidCollaborationLedger("unsettled run group missing")
				}
			}
			continue
		}
		if includeLegacy {
			legacy, err := t.stopRun(ctx, run, disposition, reason)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			r.RunID, r.RunSettled, r.Code = legacy.RunID, legacy.RunSettled, "stop_requested"
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
	if run.LegacyUnresolved {
		command, err := t.q.FinishCollaborationCommand(ctx, db.FinishCollaborationCommandParams{SessionID: t.session.ID, CommandID: run.CommandID,
			State: run.ForcedDisposition.String, Reason: run.DispositionReason.String})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if t.session.ActiveRunID == run.ID {
			t.session, err = t.q.ClearCollaborationActiveRun(ctx, db.ClearCollaborationActiveRunParams{SessionID: t.session.ID, RunID: run.ID})
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		r.CommandID, r.CommandSequence = command.ID, command.Seq
	}
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
