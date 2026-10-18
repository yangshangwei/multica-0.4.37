package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// InvalidateCollaborationRuntimeInTx borrows the resource writer's transaction.
// The caller must hold the runtime fence and roll back the entire operation on
// error. No source lock may wait behind that fence, including proof-only rows.
// Historical resource and supervisor identities are never rebound or deleted.
func InvalidateCollaborationRuntimeInTx(ctx context.Context, q *db.Queries, runtimeID pgtype.UUID, reason string) error {
	if !runtimeID.Valid || (reason != "runtime_deleted" && reason != "runtime_rebound") {
		return invalidCollaborationLedger("invalid runtime invalidation scope")
	}
	return invalidateCollaborationResourceInTx(ctx, q, collaborationResourceInvalidation{runtimeID: runtimeID, reason: reason})
}

func invalidateCollaborationBindingInTx(ctx context.Context, q *db.Queries, binding db.InstallationDaemonBinding) error {
	if !binding.ID.Valid || binding.BindingEpoch < 1 {
		return invalidCollaborationLedger("invalid binding invalidation scope")
	}
	return invalidateCollaborationResourceInTx(ctx, q, collaborationResourceInvalidation{
		bindingID: binding.ID, bindingEpoch: binding.BindingEpoch, reason: "installation_binding_changed",
	})
}

type collaborationResourceInvalidation struct {
	runtimeID    pgtype.UUID
	bindingID    pgtype.UUID
	bindingEpoch int64
	reason       string
}

func (s collaborationResourceInvalidation) matches(runtimeID, bindingID pgtype.UUID, bindingEpoch int64) bool {
	return s.runtimeID.Valid && runtimeID == s.runtimeID || s.bindingID.Valid && bindingID == s.bindingID && bindingEpoch == s.bindingEpoch
}

type collaborationResourceSource struct {
	queue  *collaborationQueueTx
	grants []db.CollaborationGrant
	groups []db.CollaborationExecutionGroup
	runs   []db.CollaborationRun
}

func invalidateCollaborationResourceInTx(ctx context.Context, q *db.Queries, scope collaborationResourceInvalidation) error {
	params := db.ListCollaborationResourceIssuesParams{RuntimeID: scope.runtimeID, BindingID: scope.bindingID, BindingEpoch: scope.bindingEpoch}
	issues, err := q.ListCollaborationResourceIssues(ctx, params)
	if err != nil {
		return err
	}
	// Fence every source before making any change, not one committed source at
	// a time. A conflicting issue or child row aborts resource deletion as well.
	sources := make([]collaborationResourceSource, 0, len(issues))
	for _, issue := range issues {
		source, err := lockCollaborationResourceSource(ctx, q, issue)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}
	current, err := q.ListCollaborationResourceIssues(ctx, params)
	if err != nil {
		return err
	}
	if len(current) != len(issues) {
		return &pgconn.PgError{Code: "40001", Message: "collaboration resource source set changed"}
	}
	for i := range issues {
		if current[i] != issues[i] {
			return &pgconn.PgError{Code: "40001", Message: "collaboration resource source set changed"}
		}
	}
	for _, source := range sources {
		if err := source.invalidate(ctx, scope); err != nil {
			return err
		}
	}
	return nil
}

func lockCollaborationResourceSource(ctx context.Context, q *db.Queries, hint db.ListCollaborationResourceIssuesRow) (collaborationResourceSource, error) {
	var source collaborationResourceSource
	if !hint.IssueID.Valid || !hint.WorkspaceID.Valid {
		return source, invalidCollaborationLedger("resource invalidation source missing")
	}
	if _, err := q.LockCollaborationReconciliationWorkspace(ctx, hint.WorkspaceID); err != nil {
		return source, err
	}
	issue, err := q.LockCollaborationReconciliationIssue(ctx, hint.IssueID)
	if err != nil {
		return source, err
	}
	if issue.WorkspaceID != hint.WorkspaceID {
		return source, invalidCollaborationLedger("resource invalidation source changed")
	}
	if _, err = q.LockCollaborationReconciliationState(ctx, issue.ID); err != nil {
		return source, err
	}
	session, err := q.LockCollaborationReconciliationSession(ctx, issue.ID)
	if err != nil {
		return source, err
	}
	t := &collaborationQueueTx{q: q, session: session, access: CollaborationIssueAccess{Issue: issue}}
	source.queue = t
	if t.channels, err = q.TryLockCollaborationResourceChannels(ctx, session.ID); err != nil {
		return source, err
	}
	if t.targets, err = q.TryLockCollaborationResourceTargets(ctx, session.ID); err != nil {
		return source, err
	}
	if _, err = q.TryLockCollaborationResourceParticipants(ctx, issue.ID); err != nil {
		return source, err
	}
	if source.grants, err = q.TryLockCollaborationResourceGrants(ctx, issue.ID); err != nil {
		return source, err
	}
	if _, err = q.TryLockCollaborationResourceCommands(ctx, session.ID); err != nil {
		return source, err
	}
	if _, err = q.TryLockCollaborationResourceBatches(ctx, session.ID); err != nil {
		return source, err
	}
	if source.groups, err = q.TryLockCollaborationResourceGroups(ctx, session.ID); err != nil {
		return source, err
	}
	if source.runs, err = q.TryLockCollaborationResourceRuns(ctx, session.ID); err != nil {
		return source, err
	}
	for _, lock := range []func(context.Context, pgtype.UUID) ([]pgtype.UUID, error){
		q.TryLockCollaborationResourceDelegations, q.TryLockCollaborationResourceTerminations,
		q.TryLockCollaborationResourceCapabilities, q.TryLockCollaborationResourceReservations,
	} {
		if _, err = lock(ctx, session.ID); err != nil {
			return source, err
		}
	}
	return source, nil
}

func (s collaborationResourceSource) invalidate(ctx context.Context, scope collaborationResourceInvalidation) error {
	t := s.queue
	grants := make([]db.CollaborationGrant, 0)
	grantIDs := make(map[pgtype.UUID]bool)
	revoked := false
	for _, grant := range s.grants {
		if !scope.matches(grant.RuntimeID, grant.BindingID, grant.BindingEpoch) {
			continue
		}
		grants = append(grants, grant)
		grantIDs[grant.ID] = true
		if grant.RevokedAt.Valid {
			continue
		}
		if _, err := t.q.RevokeCollaborationGrant(ctx, db.RevokeCollaborationGrantParams{GrantID: grant.ID, IssueID: t.access.Issue.ID, Reason: scope.reason}); err != nil {
			return err
		}
		revoked = true
	}

	stopped, err := t.stopInvalidatedWork(ctx, pgtype.UUID{}, 0, grants, scope.reason)
	if err != nil {
		return err
	}
	changed := revoked || stopped != nil
	// A pending target may never have had consent. Runtime loss still leaves a
	// durable hold, but an old binding must not hold a new configuration merely
	// because it uses the same runtime or account.
	if scope.runtimeID.Valid {
		hold := collaboration.HoldGrantRequired
		if scope.reason == "runtime_rebound" {
			hold = collaboration.HoldConfigInvalid
		}
		for _, target := range t.targets {
			if target.RevokedAt.Valid {
				continue
			}
			roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
			if err != nil {
				return err
			}
			for _, resource := range roster {
				if resource.RuntimeID != util.UUIDToString(scope.runtimeID) {
					continue
				}
				updated, err := t.setTargetHold(ctx, target, hold, false)
				if err != nil {
					return err
				}
				changed = changed || updated.HoldVersion != target.HoldVersion
				break
			}
		}
	}

	// Current grants are not required to stop historical runs. Handle an exact
	// retained dependency even when its original grant row is no longer present;
	// never synthesize current consent or rewrite the frozen proof identity.
	for _, group := range s.groups {
		roster, err := decodeCollaborationRoster[collaboration.GroupResource](group.Roster)
		if err != nil {
			return err
		}
		matched, handled := false, false
		for _, resource := range roster {
			grantID, err := util.ParseUUID(resource.GrantID)
			if err != nil {
				return invalidCollaborationLedger("resource invalidation grant identity malformed")
			}
			handled = handled || grantIDs[grantID]
			matched = matched || scope.runtimeID.Valid && resource.RuntimeID == util.UUIDToString(scope.runtimeID)
		}
		for _, run := range s.runs {
			if run.GroupID == group.ID && scope.matches(run.RuntimeID, run.BindingID, run.BindingEpoch) {
				matched = true
				handled = handled || grantIDs[run.GrantID]
			}
		}
		if !matched || handled {
			continue
		}
		target, ok := t.target(group.TargetID)
		if !ok {
			return invalidCollaborationLedger("resource invalidation group target missing")
		}
		if _, err = t.stopGroup(ctx, target, group, "stopped", scope.reason); err != nil {
			return err
		}
		changed = true
	}
	for _, run := range s.runs {
		if run.SettledAt.Valid || !scope.matches(run.RuntimeID, run.BindingID, run.BindingEpoch) {
			continue
		}
		found := false
		for _, group := range s.groups {
			found = found || group.ID == run.GroupID
		}
		if !found {
			return invalidCollaborationLedger("resource invalidation run group missing")
		}
	}
	if !changed {
		return nil
	}
	receipt := t.receipt("resource_invalidated", http.StatusOK)
	if stopped != nil {
		receipt = *stopped
		receipt.Code = "resource_invalidated"
	}
	return t.event(ctx, "resource_invalidated", receipt)
}
