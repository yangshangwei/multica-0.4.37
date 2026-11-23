package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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
	source, _, err := s.authorizeRecipientContext(ctx, p, actor)
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
	currentSource, _, err := s.authorizeRecipientContext(ctx, p, actor)
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

// authorizeRecipientContext also reports whether the actor is currently a
// source member. A creation context requires that membership.
func (s *CollaborationService) authorizeRecipientContext(ctx context.Context, p CollaborationRecipientQuery, actor pgtype.UUID) (pgtype.UUID, bool, error) {
	if p.IssueID.Valid {
		access, err := s.ResolveIssueAccess(ctx, s.Queries, p.IssueID)
		if err != nil {
			return pgtype.UUID{}, false, err
		}
		if !access.Capabilities.Invite && !access.Capabilities.Transfer {
			return pgtype.UUID{}, false, collaborationError(http.StatusForbidden, "forbidden")
		}
		return access.Issue.WorkspaceID, access.SourceMember, nil
	}
	user, err := s.Queries.GetUser(ctx, actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, false, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return pgtype.UUID{}, false, err
	}
	if err = authorizeWorkflowAccount(ctx, s.Queries, actor, user); err != nil {
		return pgtype.UUID{}, false, err
	}
	source, err := s.Queries.GetCollaborationRecipientSource(ctx, db.GetCollaborationRecipientSourceParams{ActorID: actor, SourceWorkspaceID: p.SourceWorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, false, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return pgtype.UUID{}, false, err
	}
	// Check the switch only after source authority, and never for an existing
	// issue. Maintenance cannot become a global account-discovery context.
	gate, err := s.Queries.GetCollaborationGate(ctx)
	if err != nil {
		return pgtype.UUID{}, false, err
	}
	if !gate.CreationEnabled {
		return pgtype.UUID{}, false, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	return source, true, nil
}

const (
	collaborationRecipientSearchDefaultLimit = 30
	collaborationRecipientSearchMaxLimit     = 50
	collaborationRecipientSearchMaxQuery     = 64
	collaborationRecipientCursorMaxName      = 1024
	collaborationRecipientRecentLimit        = 5
)

type CollaborationRecipientSearchQuery struct {
	IssueID           pgtype.UUID
	SourceWorkspaceID pgtype.UUID
	Query             string
	Cursor            string
	// Limit nil means the default page size.
	Limit       *int
	MembersOnly bool
	// Directory is the deployment switch, resolved by the caller. False lists
	// only accounts related to the actor.
	Directory bool
}

type CollaborationRecipientCandidate struct {
	UserID    pgtype.UUID
	Name      string
	AvatarURL string
	// External is nil when the actor is not a source member.
	External *bool
}

type CollaborationRecipientSearchResult struct {
	IssueID           pgtype.UUID
	SourceWorkspaceID pgtype.UUID
	Scope             string
	SourceMember      bool
	Recent            []CollaborationRecipientCandidate
	Recipients        []CollaborationRecipientCandidate
	NextCursor        string
}

type collaborationRecipientCursor struct {
	Name *string `json:"n"`
	ID   *string `json:"i"`
}

type collaborationRecipientSearchInput struct {
	pattern   string
	limit     int
	afterName string
	afterID   pgtype.UUID
	recent    bool
}

// SearchRecipients lists accounts for the picker. Like ResolveRecipient it only
// identifies accounts: invitation, transfer and fields recheck the selection.
func (s *CollaborationService) SearchRecipients(ctx context.Context, p CollaborationRecipientSearchQuery) (CollaborationRecipientSearchResult, error) {
	var out CollaborationRecipientSearchResult
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
	input, err := normalizeCollaborationRecipientSearch(p)
	if err != nil {
		return out, err
	}
	if s.Queries == nil {
		return out, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	recipientContext := CollaborationRecipientQuery{IssueID: p.IssueID, SourceWorkspaceID: p.SourceWorkspaceID}
	source, sourceMember, err := s.authorizeRecipientContext(ctx, recipientContext, actor)
	if err != nil {
		return out, err
	}
	var recent []db.ListRecentCollaborationRecipientsRow
	var page []db.SearchCollaborationRecipientsRow
	// A non-member asking for source members gets an empty 200: the issue
	// client treats 403 as lost access to the whole issue.
	if !p.MembersOnly || sourceMember {
		disabledIDs, disabledEmails := auth.TemporarilyDisabledUsers()
		if input.recent {
			recent, err = s.Queries.ListRecentCollaborationRecipients(ctx, db.ListRecentCollaborationRecipientsParams{
				SourceWorkspaceID: source, ActorID: actor, RequirePasswordCredential: auth.PasswordMode(),
				TemporarilyDisabledUserIds: disabledIDs, TrimChars: collaborationRecipientTrimChars, TemporarilyDisabledUserEmails: disabledEmails,
				MembersOnly: p.MembersOnly, RecentLimit: collaborationRecipientRecentLimit,
			})
			if err != nil {
				return out, err
			}
		}
		page, err = s.Queries.SearchCollaborationRecipients(ctx, db.SearchCollaborationRecipientsParams{
			SourceWorkspaceID: source, ActorID: actor, RequirePasswordCredential: auth.PasswordMode(),
			TemporarilyDisabledUserIds: disabledIDs, TrimChars: collaborationRecipientTrimChars, TemporarilyDisabledUserEmails: disabledEmails,
			Directory: p.Directory, MembersOnly: p.MembersOnly, NamePattern: input.pattern,
			AfterID: input.afterID, AfterName: input.afterName, PageLimit: int32(input.limit + 1),
		})
		if err != nil {
			return out, err
		}
	}
	// Same rule as ResolveRecipient: a slow read must not return accounts under
	// a role or source membership revoked while it was in progress.
	currentSource, currentMember, err := s.authorizeRecipientContext(ctx, recipientContext, actor)
	if err != nil {
		return out, err
	}
	if currentSource != source {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	sourceMember = sourceMember && currentMember
	if p.MembersOnly && !sourceMember {
		recent, page = nil, nil
	}
	out = CollaborationRecipientSearchResult{IssueID: p.IssueID, SourceWorkspaceID: source, Scope: "related", SourceMember: sourceMember,
		Recent: make([]CollaborationRecipientCandidate, 0, len(recent)), Recipients: make([]CollaborationRecipientCandidate, 0, len(page))}
	if p.Directory {
		out.Scope = "directory"
	}
	for _, row := range recent {
		out.Recent = append(out.Recent, collaborationRecipientCandidate(row.UserID, row.Name, row.AvatarUrl, row.External, sourceMember))
	}
	if len(page) > input.limit {
		page = page[:input.limit]
		last := page[len(page)-1]
		if out.NextCursor, err = encodeCollaborationRecipientCursor(last.SortName, last.UserID); err != nil {
			return CollaborationRecipientSearchResult{}, err
		}
	}
	for _, row := range page {
		out.Recipients = append(out.Recipients, collaborationRecipientCandidate(row.UserID, row.Name, row.AvatarUrl, row.External, sourceMember))
	}
	return out, nil
}

func collaborationRecipientCandidate(id pgtype.UUID, name string, avatar pgtype.Text, external, sourceMember bool) CollaborationRecipientCandidate {
	candidate := CollaborationRecipientCandidate{UserID: id, Name: name, AvatarURL: avatar.String}
	// Only a source member may learn who else is one.
	if sourceMember {
		candidate.External = &external
	}
	return candidate
}

func normalizeCollaborationRecipientSearch(p CollaborationRecipientSearchQuery) (collaborationRecipientSearchInput, error) {
	var out collaborationRecipientSearchInput
	invalid := collaborationError(http.StatusBadRequest, "invalid_recipient_query")
	query := strings.TrimSpace(p.Query)
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) || utf8.RuneCountInString(query) > collaborationRecipientSearchMaxQuery {
		return out, invalid
	}
	out.limit = collaborationRecipientSearchDefaultLimit
	if p.Limit != nil {
		if *p.Limit < 1 || *p.Limit > collaborationRecipientSearchMaxLimit {
			return out, invalid
		}
		out.limit = *p.Limit
	}
	out.pattern = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	if p.Cursor != "" {
		name, id, err := decodeCollaborationRecipientCursor(p.Cursor)
		if err != nil {
			return out, invalid
		}
		out.afterName, out.afterID = name, id
	}
	out.recent = query == "" && p.Cursor == ""
	return out, nil
}

func encodeCollaborationRecipientCursor(name string, id pgtype.UUID) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	idText := util.UUIDToString(id)
	if err := encoder.Encode(collaborationRecipientCursor{Name: &name, ID: &idText}); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))), nil
}

func decodeCollaborationRecipientCursor(raw string) (string, pgtype.UUID, error) {
	invalid := errors.New("invalid recipient cursor")
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || !utf8.Valid(decoded) {
		return "", pgtype.UUID{}, invalid
	}
	var cursor collaborationRecipientCursor
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cursor) != nil || cursor.Name == nil || cursor.ID == nil {
		return "", pgtype.UUID{}, invalid
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return "", pgtype.UUID{}, invalid
	}
	if len(*cursor.Name) > collaborationRecipientCursorMaxName || strings.ContainsRune(*cursor.Name, 0) {
		return "", pgtype.UUID{}, invalid
	}
	id, err := util.ParseUUID(*cursor.ID)
	if err != nil {
		return "", pgtype.UUID{}, invalid
	}
	return *cursor.Name, id, nil
}
