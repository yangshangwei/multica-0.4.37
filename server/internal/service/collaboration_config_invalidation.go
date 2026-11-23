package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CollaborationConfigReconciler drains the intents emitted in actual resource
// writes. The resource epoch already blocks old admission; source effects and
// the original intent's cursor commit together, independently of feature gates.
type CollaborationConfigReconciler struct {
	Queries   *db.Queries
	TxStarter TxStarter
}

func NewCollaborationConfigReconciler(q *db.Queries, tx TxStarter) *CollaborationConfigReconciler {
	return &CollaborationConfigReconciler{Queries: q, TxStarter: tx}
}

func (s *CollaborationConfigReconciler) Reconcile(ctx context.Context, limit int32) (int, error) {
	if limit < 1 || limit > 128 {
		return 0, errors.New("collaboration configuration reconciliation limit must be between 1 and 128")
	}
	processed := 0
	var excluded []pgtype.UUID
	var failures []error
	for range limit {
		if err := ctx.Err(); err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		worked, intent, err := s.reconcileNext(ctx, excluded)
		if err != nil {
			failures = append(failures, err)
			if !intent.ID.Valid || ctx.Err() != nil {
				return processed, errors.Join(failures...)
			}
			// Later epochs of this resource stay behind this pending intent.
			// Other resources retain a chance to progress in this bounded pass.
			excluded = append(excluded, intent.ID)
			continue
		}
		if !worked {
			break
		}
		processed++
	}
	return processed, errors.Join(failures...)
}

func (s *CollaborationConfigReconciler) Run(ctx context.Context) {
	const batchSize = 32
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		processed, err := s.Reconcile(ctx, batchSize)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			code := "configuration_reconciliation_unavailable"
			var failure *CollaborationError
			var database *pgconn.PgError
			if errors.As(err, &failure) {
				code = failure.Code
			} else if errors.As(err, &database) {
				code = database.Code
			}
			// Database error detail can contain private configuration values.
			slog.WarnContext(ctx, "collaboration configuration reconciliation deferred", "code", code)
		} else if processed == batchSize {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *CollaborationConfigReconciler) reconcileNext(ctx context.Context, excluded []pgtype.UUID) (bool, db.CollaborationConfigInvalidation, error) {
	var intent db.CollaborationConfigInvalidation
	worked := false
	err := WithRuntimeResourceTransaction(ctx, s.TxStarter, s.Queries, func(q *db.Queries) error {
		intent, worked = db.CollaborationConfigInvalidation{}, false
		var err error
		intent, err = q.LockNextCollaborationConfigInvalidation(ctx, excluded)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		worked = true
		return s.reconcileInTx(ctx, q, intent)
	})
	if err != nil && intent.ID.Valid && ctx.Err() == nil {
		// This only rotates scheduling. An uncertain commit is not replayed;
		// the next pass reads the original durable cursor and intent identity.
		err = errors.Join(err, s.Queries.RecordCollaborationConfigInvalidationAttempt(ctx, intent.ID))
	}
	return worked, intent, err
}

func (s *CollaborationConfigReconciler) reconcileInTx(ctx context.Context, q *db.Queries, intent db.CollaborationConfigInvalidation) error {
	if !intent.ResourceID.Valid || intent.ConfigEpoch < 1 ||
		(intent.ResourceKind != "agent" && intent.ResourceKind != "runtime" && intent.ResourceKind != "squad") {
		return invalidCollaborationLedger("invalid configuration invalidation scope")
	}
	next, err := q.NextCollaborationConfigInvalidationIssue(ctx, db.NextCollaborationConfigInvalidationIssueParams{
		ResourceKind: intent.ResourceKind, ResourceID: intent.ResourceID, LastIssueID: intent.LastIssueID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		count, err := q.CompleteCollaborationConfigInvalidation(ctx, intent.ID)
		if err == nil && count != 1 {
			err = invalidCollaborationLedger("configuration invalidation completion lost its intent")
		}
		return err
	}
	if err != nil {
		return err
	}
	source, err := lockCollaborationResourceSource(ctx, q, db.ListCollaborationResourceIssuesRow{IssueID: next.IssueID, WorkspaceID: next.WorkspaceID})
	if err != nil {
		return err
	}
	if err = source.invalidateConfiguration(ctx, intent); err != nil {
		return err
	}
	count, err := q.AdvanceCollaborationConfigInvalidation(ctx, db.AdvanceCollaborationConfigInvalidationParams{IntentID: intent.ID, IssueID: next.IssueID})
	if err == nil && count != 1 {
		err = invalidCollaborationLedger("configuration invalidation cursor did not advance")
	}
	return err
}

type collaborationConfigScope struct {
	kind string
	id   pgtype.UUID
}

func (s collaborationConfigScope) resource(resource collaboration.TargetResource) bool {
	return s.kind == "agent" && resource.AgentID == util.UUIDToString(s.id) ||
		s.kind == "runtime" && resource.RuntimeID == util.UUIDToString(s.id)
}

func (s collaborationConfigScope) target(target db.CollaborationTarget, roster []collaboration.TargetResource) bool {
	if s.kind == "squad" {
		return target.Kind == "squad" && target.ResourceID == s.id
	}
	for _, resource := range roster {
		if s.resource(resource) {
			return true
		}
	}
	return false
}

func (s collaborationConfigScope) grant(grant db.CollaborationGrant, target db.CollaborationTarget) bool {
	return s.kind == "agent" && grant.AgentID == s.id || s.kind == "runtime" && grant.RuntimeID == s.id ||
		s.kind == "squad" && target.Kind == "squad" && target.ResourceID == s.id
}

func (s collaborationResourceSource) invalidateConfiguration(ctx context.Context, intent db.CollaborationConfigInvalidation) error {
	t := s.queue
	scope := collaborationConfigScope{kind: intent.ResourceKind, id: intent.ResourceID}
	hold, reason := collaboration.HoldConfigInvalid, "configuration_changed"
	if intent.Reason == "resource_deleted" {
		hold, reason = collaboration.HoldGrantRequired, "resource_deleted"
	}
	current := make(map[pgtype.UUID]bool)
	valid := make(map[pgtype.UUID]bool)
	changed := false
	for _, target := range t.targets {
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
		if err != nil {
			return err
		}
		if !scope.target(target, roster) {
			continue
		}
		current[target.ID] = true
		// Reading the new fingerprint without fencing its resources could mix
		// old and new configuration, or hold a freshly reconfigured target.
		if err = lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster); err != nil {
			return err
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
		if err != nil {
			return err
		}
		valid[target.ID] = code == "" && target.ConfigurationFingerprint.Valid && configuration.Fingerprint == target.ConfigurationFingerprint.String
		if valid[target.ID] || target.RevokedAt.Valid {
			continue
		}
		updated, err := t.setTargetHold(ctx, target, hold, false)
		if err != nil {
			return err
		}
		changed = changed || updated.HoldVersion != target.HoldVersion
	}

	var obsolete []db.CollaborationGrant
	obsoleteIDs := make(map[pgtype.UUID]bool)
	for _, grant := range s.grants {
		target, exists := t.target(grant.TargetID)
		if !exists {
			return invalidCollaborationLedger("configuration grant target missing")
		}
		if !current[target.ID] && !scope.grant(grant, target) {
			continue
		}
		if valid[target.ID] && grant.ConfigVersion.Valid && grant.ConfigVersion.Int64 == target.ConfigVersion &&
			grant.TargetEpoch.Valid && grant.TargetEpoch.Int64 == target.TargetEpoch &&
			grant.ConfigurationFingerprint.Valid && grant.ConfigurationFingerprint.String == target.ConfigurationFingerprint.String {
			continue
		}
		obsolete, obsoleteIDs[grant.ID] = append(obsolete, grant), true
		if !grant.RevokedAt.Valid {
			if _, err := t.q.RevokeCollaborationGrant(ctx, db.RevokeCollaborationGrantParams{GrantID: grant.ID, IssueID: t.access.Issue.ID, Reason: reason}); err != nil {
				return err
			}
			changed = true
		}
	}
	stopped, err := t.stopInvalidatedWork(ctx, pgtype.UUID{}, 0, obsolete, reason)
	if err != nil {
		return err
	}
	changed = changed || stopped != nil

	// Retained run/roster identities still require stopping when their original
	// grant row was removed. A valid current group from a later configuration
	// is deliberately preserved when an old intent finally reaches this source.
	groups := make(map[pgtype.UUID]bool, len(s.groups))
	for _, group := range s.groups {
		groups[group.ID] = true
		target, exists := t.target(group.TargetID)
		if !exists {
			return invalidCollaborationLedger("configuration group target missing")
		}
		roster, err := decodeCollaborationRoster[collaboration.GroupResource](group.Roster)
		if err != nil {
			return err
		}
		matched := scope.kind == "squad" && target.Kind == "squad" && target.ResourceID == scope.id
		handled := false
		for _, resource := range roster {
			grantID, err := util.ParseUUID(resource.GrantID)
			if err != nil {
				return invalidCollaborationLedger("configuration frozen grant identity malformed")
			}
			matched = matched || scope.resource(resource.TargetResource)
			handled = handled || obsoleteIDs[grantID]
		}
		for _, run := range s.runs {
			if run.GroupID == group.ID {
				matched = matched || scope.kind == "agent" && run.AgentID == scope.id || scope.kind == "runtime" && run.RuntimeID == scope.id
				handled = handled || obsoleteIDs[run.GrantID]
			}
		}
		if !matched || handled || valid[target.ID] && group.ConfigVersion == target.ConfigVersion && group.TargetEpoch == target.TargetEpoch {
			continue
		}
		if _, err = t.stopGroup(ctx, target, group, "stopped", reason); err != nil {
			return err
		}
		changed = true
	}
	for _, run := range s.runs {
		if !run.SettledAt.Valid && (scope.kind == "agent" && run.AgentID == scope.id || scope.kind == "runtime" && run.RuntimeID == scope.id) && !groups[run.GroupID] {
			return invalidCollaborationLedger("configuration run group missing")
		}
	}
	if !changed {
		return nil
	}
	receipt := t.receipt("configuration_invalidated", http.StatusOK)
	if stopped != nil {
		receipt = *stopped
		receipt.Code = "configuration_invalidated"
	}
	return t.event(ctx, "configuration_invalidated", receipt)
}
