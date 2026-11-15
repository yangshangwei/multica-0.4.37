package service

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageExecutionReady(ctx context.Context, q *db.Queries) error {
	ready, err := q.StageExecutionSchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready.Valid || !ready.Bool {
		return collaborationError(503, "stage_schema_unavailable")
	}
	return nil
}
func stageExecutionTargets(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, workspace pgtype.UUID) ([]StageExecutionTarget, error) {
	if sources == nil {
		return nil, collaborationError(409, "stage_execution_unavailable")
	}
	targets, err := sources.Targets(ctx, tx, workspace)
	if err != nil {
		return nil, err
	}
	if len(targets) > 50 {
		return nil, collaborationError(409, "stage_execution_unavailable")
	}
	seen := map[string]bool{}
	for _, target := range targets {
		digest, err := StageExecutionTargetDigest(target)
		if err != nil || target.WorkspaceID != util.UUIDToString(workspace) || seen[digest] {
			return nil, collaborationError(409, "stage_execution_unavailable")
		}
		seen[digest] = true
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Environment != targets[j].Environment {
			return targets[i].Environment < targets[j].Environment
		}
		a, _ := StageExecutionTargetDigest(targets[i])
		b, _ := StageExecutionTargetDigest(targets[j])
		return a < b
	})
	return targets, nil
}
func stageResolveExecutionTarget(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, workspace pgtype.UUID, digest string) (StageExecutionTarget, error) {
	targets, err := stageExecutionTargets(ctx, tx, sources, workspace)
	if err != nil {
		return StageExecutionTarget{}, err
	}
	for _, target := range targets {
		current, _ := StageExecutionTargetDigest(target)
		if current == digest {
			return target, nil
		}
	}
	return StageExecutionTarget{}, collaborationError(409, "stage_execution_target_changed")
}
func stageTargetStateEqual(a, b StageExecutionTarget) bool {
	return a.ConditionalVersion == b.ConditionalVersion && reflect.DeepEqual(a.CurrentArtifactDigest, b.CurrentArtifactDigest)
}
func lockStageExecutionPhysical(ctx context.Context, q *db.Queries, key string) error {
	ok, err := q.TryLockCollaborationAuthorityNamespace(ctx, "collaboration-stage-execution:"+key)
	if err != nil {
		return err
	}
	if !ok {
		return collaborationError(409, "stage_execution_pending")
	}
	return nil
}
func stageExecutionCode(err error) string {
	var e *CollaborationError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
