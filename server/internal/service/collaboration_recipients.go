package service

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationRecipientQuery struct {
	Kind              string
	Value             string
	IssueID           pgtype.UUID
	SourceWorkspaceID pgtype.UUID
}

type CollaborationRecipientSummary struct {
	UserID    pgtype.UUID
	Name      string
	AvatarURL string
	External  bool
}

type CollaborationRecipientResolution struct {
	IssueID           pgtype.UUID
	SourceWorkspaceID pgtype.UUID
	Recipient         CollaborationRecipientSummary
}

// PostgreSQL btrim defaults to ASCII spaces. Historical account values must
// trim the same Unicode White_Space characters as strings.TrimSpace in auth.
const collaborationRecipientTrimChars = "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"

// ResolveRecipient only identifies an account for confirmation. Invitation and
// transfer independently recheck the current actor, target and write context.
func (s *CollaborationService) ResolveRecipient(ctx context.Context, p CollaborationRecipientQuery) (CollaborationRecipientResolution, error) {
	var out CollaborationRecipientResolution
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	if p.IssueID.Valid == p.SourceWorkspaceID.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_recipient_context")
	}
	value, targetID, err := normalizeCollaborationRecipient(p.Kind, p.Value)
	if err != nil {
		return out, err
	}
	if s.Queries == nil {
		return out, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	source, err := s.authorizeRecipientContext(ctx, p, actor)
	if err != nil {
		return out, err
	}
	matches, err := s.Queries.FindCollaborationRecipients(ctx, db.FindCollaborationRecipientsParams{
		Kind: p.Kind, Value: value, UserID: targetID, SourceWorkspaceID: source,
		TrimChars: collaborationRecipientTrimChars, RequirePasswordCredential: auth.PasswordMode(),
	})
	if err != nil {
		return out, err
	}
	// A slow lookup must not return account information using a role or source
	// membership revoked while it was in progress. This remains a read: no
	// operation, account fence, participant or grant is created.
	currentSource, err := s.authorizeRecipientContext(ctx, p, actor)
	if err != nil {
		return out, err
	}
	if currentSource != source {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	if len(matches) > 1 {
		return out, collaborationError(http.StatusConflict, "recipient_ambiguous")
	}
	if len(matches) == 0 || !collaborationRecipientAvailable(matches[0]) {
		return out, collaborationError(http.StatusNotFound, "recipient_not_found")
	}
	match := matches[0]
	return CollaborationRecipientResolution{
		IssueID: p.IssueID, SourceWorkspaceID: source,
		Recipient: CollaborationRecipientSummary{UserID: match.UserID, Name: match.Name, AvatarURL: match.AvatarUrl.String, External: match.External},
	}, nil
}

func normalizeCollaborationRecipient(kind, raw string) (string, pgtype.UUID, error) {
	var id pgtype.UUID
	invalid := collaborationError(http.StatusBadRequest, "invalid_recipient_query")
	if !utf8.ValidString(raw) {
		return "", id, invalid
	}
	switch kind {
	case "username":
		value, err := auth.NormalizeUsername(raw)
		if err != nil {
			return "", id, invalid
		}
		return value, id, nil
	case "email":
		value := strings.ToLower(strings.TrimSpace(raw))
		address, err := mail.ParseAddress(value)
		// Do not interpret a display name, address list or partial value as an
		// account. ParseAddress validates only; auth owns the normalization.
		if err != nil || len(value) > 254 || address.Name != "" || address.Address != value {
			return "", id, invalid
		}
		return value, id, nil
	case "user_id":
		var err error
		id, err = util.ParseUUID(raw)
		if err != nil {
			return "", pgtype.UUID{}, invalid
		}
		return util.UUIDToString(id), id, nil
	default:
		return "", id, invalid
	}
}

func (s *CollaborationService) authorizeRecipientContext(ctx context.Context, p CollaborationRecipientQuery, actor pgtype.UUID) (pgtype.UUID, error) {
	if p.IssueID.Valid {
		access, err := s.ResolveIssueAccess(ctx, s.Queries, p.IssueID)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if !access.Capabilities.Invite && !access.Capabilities.Transfer {
			return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
		}
		return access.Issue.WorkspaceID, nil
	}
	user, err := s.Queries.GetUser(ctx, actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if err = authorizeWorkflowAccount(ctx, s.Queries, actor, user); err != nil {
		return pgtype.UUID{}, err
	}
	source, err := s.Queries.GetCollaborationRecipientSource(ctx, db.GetCollaborationRecipientSourceParams{ActorID: actor, SourceWorkspaceID: p.SourceWorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	// Check the switch only after source authority, and never for an existing
	// issue. Maintenance cannot become a global account-discovery context.
	gate, err := s.Queries.GetCollaborationGate(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}
	if !gate.CreationEnabled {
		return pgtype.UUID{}, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	return source, nil
}
