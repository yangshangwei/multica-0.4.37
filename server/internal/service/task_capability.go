package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Finalization compares the payload's frozen scope with the locked claim's
// current authority. A payload assembled before a project or role change cannot
// publish a credential with a different scope.
func finalizeTaskCapability(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, token *db.CreateTaskTokenParams, daemonTokens int) error {
	agent, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return err
	}
	profile := auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion)
	if profile == "" {
		if len(token.ApiCapability) > 0 {
			return auth.ErrTaskCapability
		}
		return nil
	}
	if daemonTokens != 0 {
		return auth.ErrTaskCapability
	}
	if _, err = q.LockTaskCapabilityPrincipals(ctx, task.ID); err != nil {
		return err
	}
	if _, err = q.LockTaskCapabilityInvocationTargets(ctx, task.AgentID); err != nil {
		return err
	}
	agent, err = q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return err
	}
	expected, err := auth.ParseTaskCapability(token.ApiCapability)
	if err != nil {
		return auth.ErrTaskCapability
	}
	current, err := auth.CurrentTaskCapability(ctx, q, task, agent, util.UUIDToString(token.UserID), util.UUIDToString(token.WorkspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrTaskCapability
	}
	if err != nil {
		return err
	}
	if current != expected {
		return auth.ErrTaskCapability
	}
	token.ApiCapability, err = json.Marshal(current)
	return err
}

// RestrictedAgentResult remains meaningful after task-token cleanup because
// template provenance is immutable and does not depend on mutable autonomy.
func RestrictedAgentResult(ctx context.Context, q *db.Queries, agentID string) (bool, error) {
	id, err := util.ParseUUID(agentID)
	if err != nil {
		return true, err
	}
	agent, err := q.GetAgent(ctx, id)
	if err != nil {
		return true, err
	}
	return auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != "", nil
}
