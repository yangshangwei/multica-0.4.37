package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func emptyPreparationJSON(raw []byte, array bool) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return true
	}
	if array {
		var values []json.RawMessage
		return json.Unmarshal(raw, &values) == nil && len(values) == 0
	}
	var values map[string]json.RawMessage
	return json.Unmarshal(raw, &values) == nil && len(values) == 0
}

func emptyPreparationMCP(raw []byte) bool {
	if emptyPreparationJSON(raw, false) {
		return true
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) != 1 {
		return false
	}
	servers, exists := values["mcpServers"]
	return exists && emptyPreparationJSON(servers, false)
}

// Stable public refusal codes disclose the unsupported category, never a
// credential, argument, host path or private configuration value.
func unsupportedPreparationConfigurationCode(code string) bool {
	switch code {
	case "unsupported_provider_custom_environment", "unsupported_provider_custom_arguments", "unsupported_provider_mcp",
		"unsupported_provider_connected_apps", "unsupported_provider_service_tier", "unsupported_provider_skills",
		"unsupported_provider_runtime_configuration", "unsupported_provider_skill_exclusions", "unsupported_provider_autonomy",
		"unsupported_provider_invocation", "unsupported_provider_squad_instructions":
		return true
	default:
		return false
	}
}

func preparationInvocationRefusal(actor db.Agent) string {
	switch {
	case !emptyPreparationJSON(actor.CustomEnv, false):
		return "unsupported_provider_custom_environment"
	case !emptyPreparationJSON(actor.CustomArgs, true):
		return "unsupported_provider_custom_arguments"
	case !emptyPreparationMCP(actor.McpConfig):
		return "unsupported_provider_mcp"
	case len(actor.ComposioToolkitAllowlist) != 0:
		return "unsupported_provider_connected_apps"
	case actor.ServiceTier.String != "":
		return "unsupported_provider_service_tier"
	case !emptyPreparationJSON(actor.RuntimeConfig, false):
		return "unsupported_provider_runtime_configuration"
	case !emptyPreparationJSON(actor.DisabledRuntimeSkills, true):
		return "unsupported_provider_skill_exclusions"
	case actor.AutonomyLevel != "":
		return "unsupported_provider_autonomy"
	default:
		return ""
	}
}

func preparationInvocation(ctx context.Context, q *db.Queries, agentID pgtype.UUID, provider string) (protocol.CollaborationPreparationInvocation, error) {
	var empty protocol.CollaborationPreparationInvocation
	rows, err := q.TryLockCollaborationAuthorityAgents(ctx, []pgtype.UUID{agentID})
	if err != nil {
		return empty, err
	}
	if len(rows) != 1 || rows[0].ID != agentID {
		return empty, invalidCollaborationLedger("preparation agent unavailable")
	}
	actor := rows[0]
	if code := preparationInvocationRefusal(actor); code != "" {
		return empty, collaborationError(http.StatusConflict, code)
	}
	hasSkills, err := q.CollaborationAgentHasPreparationSkills(ctx, agentID)
	if err != nil {
		return empty, err
	}
	if hasSkills {
		return empty, collaborationError(http.StatusConflict, "unsupported_provider_skills")
	}
	invocation := protocol.CollaborationPreparationInvocation{Instructions: actor.Instructions, Model: actor.Model.String, ThinkingLevel: actor.ThinkingLevel.String}
	if protocol.ValidateCollaborationPreparationInvocation(invocation) != nil || !agent.IsKnownThinkingValue(provider, invocation.ThinkingLevel) {
		return empty, collaborationError(http.StatusConflict, "unsupported_provider_invocation")
	}
	return invocation, nil
}

func checkPreparationSquadInstructions(ctx context.Context, q *db.Queries, target db.CollaborationTarget) error {
	if target.Kind != "squad" {
		return nil
	}
	squad, err := q.TryLockCollaborationAuthoritySquad(ctx, target.ResourceID)
	if err != nil {
		return err
	}
	if squad.Instructions != "" {
		return collaborationError(http.StatusConflict, "unsupported_provider_squad_instructions")
	}
	return nil
}
