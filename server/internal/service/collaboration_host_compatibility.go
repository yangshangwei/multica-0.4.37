package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type collaborationExecutionReadKey struct{}

func WithCollaborationExecutionViewVersion(ctx context.Context, version int) context.Context {
	return context.WithValue(ctx, collaborationExecutionReadKey{}, version)
}

func CollaborationExecutionViewVersion(ctx context.Context) int {
	if version, ok := ctx.Value(collaborationExecutionReadKey{}).(int); ok {
		return version
	}
	return 5
}

func validateCollaborationExecutionRead(ctx context.Context) error {
	version := CollaborationExecutionViewVersion(ctx)
	if version != 5 && version != 6 {
		return collaborationError(http.StatusBadRequest, "unsupported_execution_view_version")
	}
	return nil
}

// A historical execution read does not authorize its retained private roster.
// Inspect only mode references here; the mutation path below still validates
// the complete typed roster and all current authority before any AI operation.
func collaborationProjectionUsesHost(ctx context.Context, q *db.Queries, targets []db.CollaborationTarget, runs []db.CollaborationRun) (bool, error) {
	runtimes := make(map[string]bool)
	for _, run := range runs {
		if run.Containment.String == protocol.CollaborationContainmentHostProcessSupervision {
			return true, nil
		}
		if run.RuntimeID.Valid {
			runtimes[util.UUIDToString(run.RuntimeID)] = true
		}
	}
	for _, target := range targets {
		var resources []struct {
			RuntimeID string `json:"runtime_id"`
		}
		if json.Unmarshal(target.Roster, &resources) != nil {
			return true, nil
		}
		for _, resource := range resources {
			if resource.RuntimeID != "" {
				runtimes[resource.RuntimeID] = true
			}
		}
	}
	for value := range runtimes {
		id, err := util.ParseUUID(value)
		if err != nil {
			return true, nil
		}
		runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		registration, err := collaborationRegisteredCapabilities(runtime)
		if err != nil {
			return true, nil
		}
		if registration != nil && registration.Capabilities.Containment == protocol.CollaborationContainmentHostProcessSupervision {
			return true, nil
		}
	}
	return false, nil
}

func collaborationRosterUsesHost(ctx context.Context, q *db.Queries, roster []collaboration.TargetResource) (bool, error) {
	for _, resource := range roster {
		id, err := util.ParseUUID(resource.RuntimeID)
		if err != nil {
			return false, invalidCollaborationLedger("runtime identity unavailable")
		}
		runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		registration, err := collaborationRegisteredCapabilities(runtime)
		if err != nil {
			return false, err
		}
		if registration != nil && registration.Capabilities.Containment == protocol.CollaborationContainmentHostProcessSupervision {
			return true, nil
		}
	}
	return false, nil
}

// Scope includes current aliases sharing a channel and retained host runs.
// Deleted current resources cannot erase an old host execution's semantics.
func collaborationScopeUsesHost(ctx context.Context, q *db.Queries, session pgtype.UUID, targets []db.CollaborationTarget, selected map[pgtype.UUID]bool) (bool, error) {
	channels := map[pgtype.UUID]bool{}
	for _, target := range targets {
		if selected == nil || selected[target.ID] {
			channels[target.ChannelID] = target.ChannelID.Valid
		}
	}
	scopedTargets := make([]db.CollaborationTarget, 0, len(targets))
	for _, target := range targets {
		if selected == nil || selected[target.ID] || channels[target.ChannelID] {
			scopedTargets = append(scopedTargets, target)
		}
	}
	runs, err := q.ListCollaborationRuns(ctx, session)
	if err != nil {
		return false, err
	}
	scopedRuns := make([]db.CollaborationRun, 0, len(runs))
	for _, run := range runs {
		if selected == nil || selected[run.TargetID] || channels[run.ChannelID] {
			scopedRuns = append(scopedRuns, run)
		}
	}
	// Mode comprehension is separate from resource authorization. Legacy
	// control must retain its original refusal even for opaque old rosters;
	// commands and consent still validate the full roster in their own gates.
	return collaborationProjectionUsesHost(ctx, q, scopedTargets, scopedRuns)
}

func (t *collaborationQueueTx) hostClientRequiredForMutation(ctx context.Context, kind string, payload any) (bool, error) {
	// Revoking authority remains available as a human policy operation; the
	// resulting system Stop does not require a direct AI control capability.
	if kind == "revoke_grant" || kind == "withdraw_resource_offer" {
		return false, nil
	}
	selected := map[pgtype.UUID]bool(nil)
	var roster []collaboration.TargetResource
	switch p := payload.(type) {
	case CollaborationSubmitCommandParams:
		selected = map[pgtype.UUID]bool{p.TargetID: true}
	case CollaborationCancelCommandParams:
		selected = map[pgtype.UUID]bool{p.TargetID: true}
	case CollaborationRegisterTargetParams:
		roster, selected = p.Roster, map[pgtype.UUID]bool{}
	case CollaborationOfferResourceParams:
		roster, selected = p.Roster, map[pgtype.UUID]bool{}
	case CollaborationProposeSwitchParams:
		roster = p.Roster
	}
	if t.controlTarget.Valid {
		selected = map[pgtype.UUID]bool{t.controlTarget: true}
	}
	if host, err := collaborationRosterUsesHost(ctx, t.q, roster); host || err != nil {
		return host, err
	}
	return collaborationScopeUsesHost(ctx, t.q, t.session.ID, t.targets, selected)
}

func blockLegacyHostCommentTargets(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, version int, targets []CollaborationCommentTargetResult) error {
	if version == 6 {
		return nil
	}
	all, err := q.ListCollaborationTargets(ctx, access.SessionID)
	if err != nil {
		return err
	}
	for i := range targets {
		if targets[i].State != "waiting" || !targets[i].TargetID.Valid {
			continue
		}
		host, err := collaborationScopeUsesHost(ctx, q, access.SessionID, all, map[pgtype.UUID]bool{targets[i].TargetID: true})
		if err != nil {
			return err
		}
		if host {
			targets[i].State, targets[i].Code, targets[i].WaitingReason = "blocked", "collaboration_client_upgrade_required", ""
		}
	}
	return nil
}
