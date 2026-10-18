package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
)

// The credential was verified before this adapter runs. Preserve that actual
// principal and password version; a user ID alone is not human authority.
func (h *Hub) accountConnectionContext(ctx context.Context, token, userID string, agentActor bool) (context.Context, bool, error) {
	kind := auth.CredentialJWT
	if strings.HasPrefix(token, "mul_") {
		kind = auth.CredentialPAT
	}
	ctx = auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: userID, Kind: kind, AgentActor: agentActor})
	if auth.PasswordMode() {
		identity, err := auth.CheckPasswordToken(ctx, h.passwordQueries, token, false)
		if err != nil {
			return ctx, false, err
		}
		ctx = auth.WithPasswordSession(ctx, identity.Session)
	}
	h.mu.RLock()
	authorizer := h.authorizer
	h.mu.RUnlock()
	if authorizer == nil {
		return ctx, false, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	allowed, err := authorizer.AuthorizeScope(bounded, userID, "", ScopeAccount, userID)
	return ctx, allowed, err
}

func (c *Client) authorizeScope(scope, id string) (bool, error) {
	if scope == ScopeAccount && id != c.userID {
		return false, nil
	}
	c.hub.mu.RLock()
	authorizer := c.hub.authorizer
	c.hub.mu.RUnlock()
	if authorizer == nil {
		return false, nil
	}
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return authorizer.AuthorizeScope(ctx, c.userID, c.workspaceID, scope, id)
}

// Only the write pump calls this method. A removed subscriber gets an
// account-local, content-free invalidation even when the issue room must deny
// its queued frame. A temporary lookup failure must not masquerade as revoke.
func (c *Client) outboundScopeCurrent(scope scopeKey) bool {
	if scope.Type != ScopeAccount && scope.Type != ScopeIssue {
		return true
	}
	c.hub.mu.RLock()
	subscribed := c.subscriptions[scope]
	c.hub.mu.RUnlock()
	if !subscribed {
		return false
	}
	allowed, err := c.authorizeScope(scope.Type, scope.ID)
	if err != nil {
		return false
	}
	if allowed {
		return true
	}
	c.hub.unsubscribe(c, scope.Type, scope.ID)
	if scope.Type == ScopeAccount {
		_ = c.conn.Close()
		return false
	}
	frame, _ := json.Marshal(map[string]any{"type": "collaboration:access_changed", "payload": map[string]string{"issue_id": scope.ID}})
	if err := c.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
		_ = c.conn.Close()
	}
	return false
}

// PublishWithID is the single-node outbox publisher. The scope remains attached
// to the queued frame, and durable replay uses the same event ID for dedup.
func (h *Hub) PublishWithID(scopeType, scopeID, exclude string, frame []byte, id string) error {
	frame = injectEventID(frame, id)
	switch scopeType {
	case ScopeUser:
		h.fanoutUser(scopeID, frame, exclude, id)
	case "global":
		h.fanoutAllDedup(frame, exclude, id)
	default:
		h.BroadcastToScopeDedup(scopeType, scopeID, frame, id)
	}
	return nil
}

var _ RelayPublisher = (*Hub)(nil)
