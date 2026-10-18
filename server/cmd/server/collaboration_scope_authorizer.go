package main

import (
	"context"

	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationScopeAuthorizer struct {
	workspace *dbScopeAuthorizer
	service   *service.CollaborationService
}

func (a *collaborationScopeAuthorizer) AuthorizeScope(ctx context.Context, userID, workspaceID, scopeType, scopeID string) (bool, error) {
	if scopeType == realtime.ScopeAccount || scopeType == realtime.ScopeIssue {
		return a.service.AuthorizeRealtimeScope(ctx, userID, scopeType, scopeID)
	}
	return a.workspace.AuthorizeScope(ctx, userID, workspaceID, scopeType, scopeID)
}
