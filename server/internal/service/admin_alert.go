package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	AdminAlertInstallationUnreachable = "installation_unreachable"
	AdminAlertQueueTimeout            = "queue_timeout"
	AdminAlertExecutionFailed         = "execution_failed"
	adminAlertBatchSize               = 200
	adminAlertInterval                = 15 * time.Second
	adminAlertQueueAge                = 5 * time.Minute
	adminAlertFailureWindow           = 31 * 24 * time.Hour
)

// AdminAlertLiveness is satisfied by the existing heartbeat store without a
// service-to-handler dependency. Reads happen before database locks are held.
type AdminAlertLiveness interface {
	Available() bool
	IsAliveBatch(context.Context, []string) (map[string]bool, bool)
}
type AdminAlertService struct {
	Queries      *db.Queries
	TxStarter    TxStarter
	DeploymentID string
	Liveness     AdminAlertLiveness
	Now          func() time.Time
	failureHints chan pgtype.UUID
}

func NewAdminAlertService(q *db.Queries, tx TxStarter, deployment string, liveness AdminAlertLiveness) *AdminAlertService {
	return &AdminAlertService{Queries: q, TxStarter: tx, DeploymentID: deployment, Liveness: liveness, Now: time.Now, failureHints: make(chan pgtype.UUID, 256)}
}

type alertObservation struct {
	Rule           string
	SubjectID      pgtype.UUID
	Condition      string // problem, healthy or unknown; never client supplied.
	ObservedAt     time.Time
	ResolutionCode string
}

func alertRuleValid(rule string) bool {
	return rule == AdminAlertInstallationUnreachable || rule == AdminAlertQueueTimeout || rule == AdminAlertExecutionFailed
}
func alertFingerprint(rule string, id pgtype.UUID) string {
	return "v1:" + rule + ":" + util.UUIDToString(id)
}

// AdminAlertResolutionCode is shared by alert DTOs and audit snapshots. Stored
// diagnostic text must never become a management code through a legacy row.
func AdminAlertResolutionCode(value string) string {
	switch value {
	case "handled", "no_action_needed", "retry_succeeded", "queue_left", "queue_below_threshold", "daemon_reachable", "execution_finished", "task_removed":
		return value
	default:
		return "unknown"
	}
}

func (s *AdminAlertService) applyObservations(ctx context.Context, q *db.Queries, org pgtype.UUID, observations []alertObservation) ([]db.AdminAlert, error) {
	rows := make([]db.AdminAlert, 0, len(observations))
	for _, observation := range observations {
		if !org.Valid || !observation.SubjectID.Valid || !alertRuleValid(observation.Rule) || observation.ObservedAt.IsZero() {
			return nil, errors.New("invalid internal alert observation")
		}
		if observation.Condition == "unknown" {
			continue
		}
		if observation.Condition != "problem" && observation.Condition != "healthy" {
			return nil, errors.New("invalid internal alert condition")
		}
		fingerprint := alertFingerprint(observation.Rule, observation.SubjectID)
		if err := q.LockAdminAlertFingerprint(ctx, util.UUIDToString(org)+":"+fingerprint); err != nil {
			return nil, err
		}
		current, err := q.GetLatestAdminAlert(ctx, db.GetLatestAdminAlertParams{OrganizationID: org, Fingerprint: fingerprint})
		found := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		observed := cancellationTimestamp(observation.ObservedAt)
		if found && !observation.ObservedAt.After(current.LastObservedAt.Time) {
			rows = append(rows, current)
			continue
		}
		if observation.Condition == "healthy" {
			if !found || observation.Rule == AdminAlertExecutionFailed {
				continue
			}
			current, err = q.ObserveAdminAlertRecovery(ctx, db.ObserveAdminAlertRecoveryParams{ID: current.ID, ObservedAt: observed, ResolutionCode: observation.ResolutionCode})
		} else if found && current.ConditionActive {
			current, err = q.ObserveAdminAlertCondition(ctx, db.ObserveAdminAlertConditionParams{ID: current.ID, ObservedAt: observed})
		} else {
			subjectKind, severity := "task", "warning"
			if observation.Rule == AdminAlertInstallationUnreachable {
				subjectKind, severity = "installation", "critical"
			}
			current, err = q.CreateAdminAlert(ctx, db.CreateAdminAlertParams{OrganizationID: org, Rule: observation.Rule, SubjectKind: subjectKind, SubjectID: observation.SubjectID, Fingerprint: fingerprint, Severity: severity, FirstSeenAt: observed})
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, current)
	}
	return rows, nil
}

type AdminAlertMutationParams struct {
	OrganizationID, AlertID, IdempotencyKey pgtype.UUID
	Action                                  string
	ExpectedVersion                         int64
	AssigneeID                              pgtype.UUID
	ResolutionCode                          string
	RelatedTaskID                           pgtype.UUID
	Reason, RequestID                       string
}
type AdminAlertMutationResult struct {
	Operation db.AdminOperation
	Target    db.AdminAlert
	Replayed  bool
}

func alertMutationHash(p AdminAlertMutationParams) (string, error) {
	payload, err := json.Marshal(struct {
		AlertID       pgtype.UUID
		Action        string
		Version       int64
		AssigneeID    pgtype.UUID
		Resolution    string
		RelatedTaskID pgtype.UUID
		Reason        string
	}{p.AlertID, p.Action, p.ExpectedVersion, p.AssigneeID, p.ResolutionCode, p.RelatedTaskID, strings.TrimSpace(p.Reason)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
func alertStateJSON(alert db.AdminAlert) []byte {
	var resolution *string
	if alert.ResolutionCode.Valid {
		code := AdminAlertResolutionCode(alert.ResolutionCode.String)
		resolution = &code
	}
	data, _ := json.Marshal(map[string]any{"status": alert.Status, "condition_active": alert.ConditionActive, "assignee_id": util.UUIDToPtr(alert.AssigneeID), "version": strconv.FormatInt(alert.Version, 10), "resolution_code": resolution, "related_task_id": util.UUIDToPtr(alert.RelatedTaskID)})
	return data
}
func (s *AdminAlertService) Mutate(ctx context.Context, p AdminAlertMutationParams) (AdminAlertMutationResult, error) {
	var result AdminAlertMutationResult
	code := map[string]string{"acknowledge": "alert_acknowledged", "assign": "alert_assigned", "close": "alert_closed"}[p.Action]
	if code == "" || !p.OrganizationID.Valid || !p.AlertID.Valid || !p.IdempotencyKey.Valid || p.ExpectedVersion < 1 {
		return result, platformError(400, "invalid_request", "A valid alert, action, version and idempotency key are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	if p.Action != "assign" && p.AssigneeID.Valid || p.Action != "close" && (p.ResolutionCode != "" || p.RelatedTaskID.Valid) {
		return result, platformError(400, "invalid_request", "The action contains unrelated parameters")
	}
	if p.Action == "close" && p.ResolutionCode != "" && p.ResolutionCode != "handled" && p.ResolutionCode != "no_action_needed" && p.ResolutionCode != "retry_succeeded" {
		return result, platformError(400, "invalid_resolution", "The handling conclusion is invalid")
	}
	if p.RelatedTaskID.Valid && p.ResolutionCode != "retry_succeeded" {
		return result, platformError(400, "invalid_resolution", "A related execution is only used to verify a successful retry")
	}
	admin := NewPlatformAdminService(s.Queries, s.TxStarter)
	actor, err := admin.Authorize(ctx, true)
	if err != nil {
		return result, err
	}
	hash, err := alertMutationHash(p)
	if err != nil {
		return result, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if p.Action == "assign" {
		users := []pgtype.UUID{actor.UserID}
		if p.AssigneeID.Valid {
			users = append(users, p.AssigneeID)
		}
		if err = LockPlatformAdminMutation(ctx, tx, users...); err != nil {
			var unavailable *PlatformAdminError
			if !errors.As(err, &unavailable) || unavailable.Code != "user_not_found" {
				return result, err
			}
			// A missing assignee cannot be locked. The common operation boundary
			// still locks/revalidates the actor before recording a failed receipt.
		}
	}
	op, replayed, err := admin.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{OrganizationID: p.OrganizationID, TargetKind: "alert", TargetID: p.AlertID, Kind: "alert." + p.Action, IdempotencyKey: p.IdempotencyKey, PayloadHash: hash, Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: code})
	if err != nil {
		return result, err
	}
	alert, err := q.GetAdminAlertForUpdate(ctx, db.GetAdminAlertForUpdateParams{ID: p.AlertID, OrganizationID: p.OrganizationID})
	if err != nil {
		return result, err
	}
	if replayed {
		return AdminAlertMutationResult{op, alert, true}, nil
	}
	before := alertStateJSON(alert)
	var conflict error
	if alert.Version != p.ExpectedVersion {
		conflict = platformError(409, "alert_version_conflict", "The alert changed; refresh its state")
	}
	if conflict == nil && alert.Status == "closed" {
		conflict = platformError(409, "alert_state_conflict", "The alert is already closed")
	}
	now := cancellationTimestamp(s.Now())
	update := db.UpdateAdminAlertActionParams{ID: alert.ID, OrganizationID: p.OrganizationID, ExpectedVersion: alert.Version, OperationID: op.ID, Status: alert.Status, AssigneeID: alert.AssigneeID, AcknowledgedAt: alert.AcknowledgedAt, ResolvedAt: alert.ResolvedAt, ClosedAt: alert.ClosedAt, ResolutionCode: alert.ResolutionCode, RelatedTaskID: alert.RelatedTaskID}
	if conflict == nil {
		switch p.Action {
		case "acknowledge":
			if alert.Status != "open" {
				conflict = platformError(409, "alert_state_conflict", "Only an open alert can be acknowledged")
			} else {
				update.Status = "acknowledged"
				update.AcknowledgedAt = now
			}
		case "assign":
			if p.AssigneeID.Valid {
				assignee, e := q.GetPlatformAdminAccount(ctx, p.AssigneeID)
				if e != nil && !errors.Is(e, pgx.ErrNoRows) {
					return result, e
				}
				if e != nil || assignee.DisabledAt.Valid || assignee.MustChangePassword || assignee.Role.String != PlatformRoleSuperAdmin || auth.IsTemporarilyDisabledUser(util.UUIDToString(p.AssigneeID), assignee.Email.String) {
					conflict = platformError(409, "alert_assignee_unavailable", "The assignee must be an active super administrator")
				}
			}
			update.AssigneeID = p.AssigneeID
		case "close":
			if alert.Rule != AdminAlertExecutionFailed {
				if alert.Status != "resolved" || alert.ConditionActive {
					conflict = platformError(409, "alert_state_conflict", "This alert requires confirmed recovery before closing")
				}
				if p.ResolutionCode != "" || p.RelatedTaskID.Valid {
					conflict = platformError(409, "alert_resolution_invalid", "Recovery conclusions apply only to execution failure alerts")
				}
			} else {
				if p.ResolutionCode == "" {
					conflict = platformError(409, "alert_resolution_invalid", "A handling conclusion is required")
				}
				if p.ResolutionCode == "retry_succeeded" {
					valid := false
					if p.RelatedTaskID.Valid {
						valid, err = q.IsCompletedAdminAlertRetry(ctx, db.IsCompletedAdminAlertRetryParams{OrganizationID: p.OrganizationID, OriginalTaskID: alert.SubjectID, RelatedTaskID: p.RelatedTaskID})
						if err != nil {
							return result, err
						}
					}
					if !valid {
						conflict = platformError(409, "alert_resolution_invalid", "The completed retry could not be verified")
					}
				}
				update.ResolutionCode = pgtype.Text{String: p.ResolutionCode, Valid: p.ResolutionCode != ""}
				update.RelatedTaskID = p.RelatedTaskID
				if !update.ResolvedAt.Valid {
					update.ResolvedAt = now
				}
			}
			update.Status = "closed"
			update.ClosedAt = now
		}
	}
	state, applied, phase := "succeeded", now, "applied"
	if conflict != nil {
		var typed *PlatformAdminError
		_ = errors.As(conflict, &typed)
		code, state, applied, phase = typed.Code, "failed", pgtype.Timestamptz{}, "failed"
	} else {
		alert, err = q.UpdateAdminAlertAction(ctx, update)
		if err != nil {
			return result, err
		}
	}
	op, err = q.FinishAdminAlertOperation(ctx, db.FinishAdminAlertOperationParams{ID: op.ID, State: state, ResultCode: code, AppliedAt: applied})
	if err != nil {
		return result, err
	}
	phases := []string{"request", phase}
	if conflict == nil {
		phases = append(phases, "succeeded")
	}
	for _, eventPhase := range phases {
		if err = q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{OperationID: op.ID, OrganizationID: op.OrganizationID, ActorKind: op.ActorKind, ActorUserID: op.ActorID, TargetKind: "alert", TargetID: alert.ID, Action: op.Kind, Phase: eventPhase, RequestID: p.RequestID, Reason: op.Reason, BeforeState: before, AfterState: alertStateJSON(alert), ResultCode: code}); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return AdminAlertMutationResult{op, alert, false}, conflict
}

// NotifyTaskFailed never blocks the task event bus. Losing a hint is safe:
// bounded cyclic scans inspect the persistent source rows independently.
func (s *AdminAlertService) NotifyTaskFailed(taskID pgtype.UUID) {
	if !taskID.Valid {
		return
	}
	select {
	case s.failureHints <- taskID:
	default:
	}
}

type AdminAlertDetectorSnapshot struct {
	Rule             string  `json:"rule"`
	SourceState      string  `json:"source_state"`
	ScanComplete     bool    `json:"scan_complete"`
	LastStartedAt    *string `json:"last_started_at"`
	LastSuccessfulAt *string `json:"last_successful_at"`
	LastErrorCode    *string `json:"last_error_code"`
	UnknownCount     int64   `json:"unknown_count"`
}

func (s *AdminAlertService) DetectorHealth(ctx context.Context, org pgtype.UUID) ([]AdminAlertDetectorSnapshot, error) {
	rows, err := s.Queries.ListAdminAlertDetectorHealth(ctx, org)
	if err != nil {
		return nil, err
	}
	byRule := map[string]db.AdminAlertDetectorState{}
	for _, row := range rows {
		byRule[row.Rule] = row
	}
	out := []AdminAlertDetectorSnapshot{}
	for _, rule := range []string{AdminAlertInstallationUnreachable, AdminAlertQueueTimeout, AdminAlertExecutionFailed} {
		state := AdminAlertDetectorSnapshot{Rule: rule, SourceState: "unknown"}
		if row, ok := byRule[rule]; ok {
			state.SourceState = row.SourceState
			state.ScanComplete = row.ScanComplete
			state.LastStartedAt = util.TimestampToPtr(row.LastStartedAt)
			state.LastSuccessfulAt = util.TimestampToPtr(row.LastSuccessfulAt)
			state.LastErrorCode = util.TextToPtr(row.LastErrorCode)
			state.UnknownCount = row.CycleUnknownCount
		}
		out = append(out, state)
	}
	return out, nil
}

type alertScanBatch struct {
	Observations []alertObservation
	CursorTime   pgtype.Timestamptz
	CursorID     pgtype.UUID
	Complete     bool
	Unknown      int64
}

var errAdminAlertLiveness = errors.New("alert liveness source unavailable")
var errAdminAlertTruncated = errors.New("alert installation evidence truncated")
var errAdminAlertDeployment = errors.New("alert deployment identity unavailable")

func (s *AdminAlertService) ScanRule(ctx context.Context, rule string) (resultErr error) {
	if !alertRuleValid(rule) {
		return errors.New("invalid alert rule")
	}
	scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	org, err := s.Queries.GetInternalOrganization(scanCtx)
	if err != nil {
		return err
	}
	if err = s.Queries.EnsureAdminAlertDetector(scanCtx, db.EnsureAdminAlertDetectorParams{OrganizationID: org.ID, Rule: rule}); err != nil {
		return err
	}
	now := s.Now().UTC()
	state, err := s.Queries.LeaseAdminAlertDetector(scanCtx, db.LeaseAdminAlertDetectorParams{OrganizationID: org.ID, Rule: rule, LeaseOwner: pgtype.UUID{Bytes: uuid.New(), Valid: true}, LeaseUntil: cancellationTimestamp(now.Add(30 * time.Second)), ObservedAt: cancellationTimestamp(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		code := "database_unavailable"
		switch {
		case errors.Is(resultErr, errAdminAlertLiveness):
			code = "liveness_unavailable"
		case errors.Is(resultErr, errAdminAlertTruncated):
			code = "evidence_truncated"
		case errors.Is(resultErr, errAdminAlertDeployment):
			code = "deployment_unavailable"
		}
		failureCtx, failureCancel := context.WithTimeout(ctx, 2*time.Second)
		defer failureCancel()
		_ = s.Queries.FailAdminAlertDetectorBatch(failureCtx, db.FailAdminAlertDetectorBatchParams{OrganizationID: org.ID, Rule: rule, LeaseOwner: state.LeaseOwner, ExpectedVersion: state.Version, ErrorCode: code})
	}()
	batch, err := s.readAlertBatch(scanCtx, org.ID, rule, state, now)
	if err != nil {
		return err
	}
	tx, err := s.TxStarter.Begin(scanCtx)
	if err != nil {
		return err
	}
	defer tx.Rollback(scanCtx)
	q := db.New(tx)
	if _, err = q.LockAdminAlertDetectorLease(scanCtx, db.LockAdminAlertDetectorLeaseParams{OrganizationID: org.ID, Rule: rule, LeaseOwner: state.LeaseOwner, Version: state.Version}); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err = s.applyObservations(scanCtx, q, org.ID, batch.Observations); err != nil {
		return err
	}
	unknown := batch.Unknown
	if state.CursorTime.Valid {
		unknown += state.CycleUnknownCount
	}
	sourceState := "healthy"
	errorCode := pgtype.Text{}
	if unknown > 0 {
		sourceState = "unknown"
		errorCode = pgtype.Text{String: "evidence_unknown", Valid: true}
	}
	if batch.Complete {
		batch.CursorTime = pgtype.Timestamptz{}
		batch.CursorID = pgtype.UUID{}
	}
	changed, err := q.CompleteAdminAlertDetectorBatch(scanCtx, db.CompleteAdminAlertDetectorBatchParams{OrganizationID: org.ID, Rule: rule, LeaseOwner: state.LeaseOwner, ExpectedVersion: state.Version, CursorTime: batch.CursorTime, CursorID: batch.CursorID, SourceState: sourceState, LastErrorCode: errorCode, ObservedAt: cancellationTimestamp(now), ScanComplete: batch.Complete, CycleUnknownCount: unknown})
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("alert detector lease was superseded")
	}
	return tx.Commit(scanCtx)
}

func (s *AdminAlertService) readAlertBatch(ctx context.Context, org pgtype.UUID, rule string, state db.AdminAlertDetectorState, now time.Time) (alertScanBatch, error) {
	batch := alertScanBatch{}
	// Keep the population finite even while fresh failures keep arriving.
	// Conditions are evaluated at now, but the cursor walks one fixed horizon.
	horizon := now
	if state.CycleStartedAt.Valid {
		horizon = state.CycleStartedAt.Time
	}
	add := func(id pgtype.UUID, cursor pgtype.Timestamptz, condition, resolution string) {
		batch.CursorID, batch.CursorTime = id, cursor
		batch.Observations = append(batch.Observations, alertObservation{Rule: rule, SubjectID: id, Condition: condition, ObservedAt: now, ResolutionCode: resolution})
		if condition == "unknown" {
			batch.Unknown++
		}
	}
	switch rule {
	case AdminAlertExecutionFailed:
		rows, err := s.Queries.ListAdminAlertFailureCandidates(ctx, db.ListAdminAlertFailureCandidatesParams{OrganizationID: org, WindowFrom: cancellationTimestamp(horizon.Add(-adminAlertFailureWindow)), ObservedAt: cancellationTimestamp(horizon), CursorTime: state.CursorTime, CursorID: state.CursorID, BatchLimit: adminAlertBatchSize})
		if err != nil {
			return batch, err
		}
		for _, row := range rows {
			add(row.SubjectID, row.CursorTime, "problem", "")
		}
		batch.Complete = len(rows) < adminAlertBatchSize
	case AdminAlertQueueTimeout:
		rows, err := s.Queries.ListAdminAlertQueueCandidates(ctx, db.ListAdminAlertQueueCandidatesParams{OrganizationID: org, ObservedAt: cancellationTimestamp(horizon), CursorTime: state.CursorTime, CursorID: state.CursorID, BatchLimit: adminAlertBatchSize})
		if err != nil {
			return batch, err
		}
		for _, row := range rows {
			condition, resolution := "unknown", ""
			if !row.SubjectExists {
				condition, resolution = "healthy", "task_removed"
			} else if row.Status != "" && row.Status != "queued" {
				condition, resolution = "healthy", "queue_left"
			} else if row.Status == "queued" && row.QueuedAt.Valid && (row.QueuedAtSource.String == "transition" || row.QueuedAtSource.String == "observation") && !row.QueuedAt.Time.After(now.Add(time.Minute)) {
				condition, resolution = "healthy", "queue_below_threshold"
				if row.QueuedAt.Time.Before(now.Add(-adminAlertQueueAge)) {
					condition, resolution = "problem", ""
				}
			}
			add(row.SubjectID, row.CursorTime, condition, resolution)
		}
		batch.Complete = len(rows) < adminAlertBatchSize
	case AdminAlertInstallationUnreachable:
		deployment, err := util.ParseUUID(s.DeploymentID)
		if err != nil {
			return batch, errAdminAlertDeployment
		}
		rows, err := s.Queries.ListAdminAlertInstallationCandidates(ctx, db.ListAdminAlertInstallationCandidatesParams{OrganizationID: org, DeploymentID: deployment, ObservedAt: cancellationTimestamp(horizon), CursorTime: state.CursorTime, CursorID: state.CursorID, BatchLimit: adminAlertBatchSize})
		if err != nil {
			return batch, err
		}
		ids := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.SubjectID)
		}
		evidence, err := s.Queries.ListAdminAlertInstallationEvidence(ctx, db.ListAdminAlertInstallationEvidenceParams{OrganizationID: org, DeploymentID: deployment, ObservedAt: cancellationTimestamp(now), InstallationIds: ids})
		if err != nil {
			return batch, err
		}
		if len(evidence) > 2000 {
			return batch, errAdminAlertTruncated
		}
		grouped := map[pgtype.UUID][]db.ListAdminAlertInstallationEvidenceRow{}
		runtimeIDs := []string{}
		for _, row := range evidence {
			grouped[row.InstallationID] = append(grouped[row.InstallationID], row)
			if row.RuntimeID.Valid {
				runtimeIDs = append(runtimeIDs, util.UUIDToString(row.RuntimeID))
			}
		}
		alive := map[string]bool{}
		if s.Liveness != nil && s.Liveness.Available() && len(runtimeIDs) > 0 {
			var ok bool
			alive, ok = s.Liveness.IsAliveBatch(ctx, runtimeIDs)
			if !ok {
				return batch, errAdminAlertLiveness
			}
		}
		for _, row := range rows {
			condition, resolution := "unknown", ""
			if row.Lifecycle == "active" {
				if row.InflightCount == 0 {
					condition, resolution = "healthy", "execution_finished"
				} else {
					eligible, known, reachable := false, false, false
					for _, item := range grouped[row.SubjectID] {
						if !item.Authorized || auth.IsTemporarilyDisabledUser(util.UUIDToString(item.PrincipalUserID), item.PrincipalEmail) {
							continue
						}
						if item.InflightCount > 0 {
							eligible = true
						}
						for _, seen := range []pgtype.Timestamptz{item.BindingSeenAt, item.RuntimeSeenAt} {
							if !seen.Valid || seen.Time.After(now.Add(time.Minute)) {
								continue
							}
							if item.InflightCount > 0 {
								known = true
							}
							if seen.Time.After(now.Add(-time.Duration(RuntimeClaimFreshnessSeconds) * time.Second)) {
								reachable = true
							}
						}
						if item.RuntimeID.Valid && alive[util.UUIDToString(item.RuntimeID)] {
							reachable = true
						}
					}
					if eligible && reachable {
						condition, resolution = "healthy", "daemon_reachable"
					} else if eligible && known {
						condition = "problem"
					}
				}
			}
			add(row.SubjectID, row.CursorTime, condition, resolution)
		}
		batch.Complete = len(rows) < adminAlertBatchSize
	}
	return batch, nil
}

func (s *AdminAlertService) processFailureHints(ctx context.Context, ids []pgtype.UUID) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	org, err := s.Queries.GetInternalOrganization(bounded)
	if err != nil {
		return err
	}
	now := s.Now().UTC()
	rows, err := s.Queries.ListAdminAlertFailureHints(bounded, db.ListAdminAlertFailureHintsParams{OrganizationID: org.ID, TaskIds: ids, WindowFrom: cancellationTimestamp(now.Add(-adminAlertFailureWindow)), ObservedAt: cancellationTimestamp(now)})
	if err != nil {
		return err
	}
	observations := make([]alertObservation, 0, len(rows))
	for _, row := range rows {
		observations = append(observations, alertObservation{Rule: AdminAlertExecutionFailed, SubjectID: row.SubjectID, Condition: "problem", ObservedAt: now})
	}
	tx, err := s.TxStarter.Begin(bounded)
	if err != nil {
		return err
	}
	defer tx.Rollback(bounded)
	if _, err = s.applyObservations(bounded, db.New(tx), org.ID, observations); err != nil {
		return err
	}
	return tx.Commit(bounded)
}

func (s *AdminAlertService) Run(ctx context.Context) {
	ticker := time.NewTicker(adminAlertInterval)
	defer ticker.Stop()
	scan := func() {
		for _, rule := range []string{AdminAlertQueueTimeout, AdminAlertExecutionFailed, AdminAlertInstallationUnreachable} {
			if err := s.ScanRule(ctx, rule); err != nil && ctx.Err() == nil {
				slog.Warn("admin alert detector unavailable", "rule", rule, "error", err)
			}
		}
	}
	scan()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		case first := <-s.failureHints:
			ids := []pgtype.UUID{first}
		collect:
			for len(ids) < adminAlertBatchSize {
				select {
				case id := <-s.failureHints:
					ids = append(ids, id)
				default:
					break collect
				}
			}
			if err := s.processFailureHints(ctx, ids); err != nil && ctx.Err() == nil {
				slog.Warn("admin alert failure hint deferred to scan", "count", len(ids), "error", err)
			}
		}
	}
}
