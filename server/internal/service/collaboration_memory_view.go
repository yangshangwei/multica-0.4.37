package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CollaborationMemoryAgent names the agent and run that wrote an AI version.
type CollaborationMemoryAgent struct {
	AgentID pgtype.UUID `json:"agent_id"`
	Name    string      `json:"name"`
	RunID   pgtype.UUID `json:"run_id"`
}

// CollaborationMemoryAuthor reuses the comment author projection: accounts are
// the minimal user summary, external accounts are flagged, never email/username.
type CollaborationMemoryAuthor struct {
	Kind          string                       `json:"kind"`
	Member        *CollaborationContentAccount `json:"member"`
	Agent         *CollaborationMemoryAgent    `json:"agent"`
	ResourceOwner *CollaborationContentAccount `json:"resource_owner"`
}

type CollaborationMemoryView struct {
	IssueID             pgtype.UUID                `json:"issue_id"`
	Version             int64                      `json:"version,string"`
	Content             string                     `json:"content"`
	Bytes               int                        `json:"bytes"`
	MaxBytes            int                        `json:"max_bytes"`
	Author              *CollaborationMemoryAuthor `json:"author"`
	CreatedAt           *time.Time                 `json:"created_at"`
	RestoredFromVersion *int64                     `json:"restored_from_version,string"`
	CanEdit             bool                       `json:"can_edit"`
	CanPurge            bool                       `json:"can_purge"`
}

// CollaborationMemoryVersionView is history metadata. Purge reasons stay in the
// version row and administration audit; they are not shown to readers.
type CollaborationMemoryVersionView struct {
	Version             int64                        `json:"version,string"`
	Bytes               int                          `json:"bytes"`
	Author              *CollaborationMemoryAuthor   `json:"author"`
	CreatedAt           time.Time                    `json:"created_at"`
	RestoredFromVersion *int64                       `json:"restored_from_version,string"`
	Purged              bool                         `json:"purged"`
	PurgedAt            *time.Time                   `json:"purged_at"`
	PurgedBy            *CollaborationContentAccount `json:"purged_by"`
	PurgedByAdmin       bool                         `json:"purged_by_admin"`
}

type CollaborationMemoryVersionsPage struct {
	IssueID        pgtype.UUID                      `json:"issue_id"`
	CurrentVersion int64                            `json:"current_version,string"`
	MaxBytes       int                              `json:"max_bytes"`
	CanPurge       bool                             `json:"can_purge"`
	Items          []CollaborationMemoryVersionView `json:"items"`
	NextCursor     *int64                           `json:"next_cursor,string"`
}

type CollaborationMemoryVersionDetail struct {
	CollaborationMemoryVersionView
	IssueID        pgtype.UUID `json:"issue_id"`
	CurrentVersion int64       `json:"current_version,string"`
	MaxBytes       int         `json:"max_bytes"`
	Content        *string     `json:"content"`
}

type CollaborationMemoryVersionsQuery struct {
	Limit         int
	BeforeVersion pgtype.Int8
}

func collaborationMemoryViewer(ctx context.Context) pgtype.UUID {
	principal, _ := collaboration.PrincipalFromContext(ctx)
	viewer, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return pgtype.UUID{}
	}
	return viewer
}

func (s *CollaborationService) GetMemory(ctx context.Context, issueID pgtype.UUID) (CollaborationMemoryView, error) {
	out := CollaborationMemoryView{IssueID: issueID, MaxBytes: s.MemoryLimit()}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		out.CanEdit = collaborationMemoryEditable(access)
		out.CanPurge = collaborationMemoryInitiator(access, collaborationMemoryViewer(ctx))
		row, err := q.GetCollaborationMemoryCurrent(ctx, issueID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		authors, err := collaborationMemoryAuthors(ctx, q, access.Issue.WorkspaceID, []collaborationMemoryAuthorRef{memoryAuthorRef(row)})
		if err != nil {
			return err
		}
		created := row.CreatedAt.Time
		out.Version, out.Content, out.Bytes, out.Author, out.CreatedAt = row.Version, row.Content, int(row.Bytes), authors[0], &created
		out.RestoredFromVersion = util.Int8ToPtr(row.RestoredFromVersion)
		return nil
	})
	return out, err
}

func (s *CollaborationService) ListMemoryVersions(ctx context.Context, issueID pgtype.UUID, p CollaborationMemoryVersionsQuery) (CollaborationMemoryVersionsPage, error) {
	out := CollaborationMemoryVersionsPage{IssueID: issueID, MaxBytes: s.MemoryLimit(), Items: make([]CollaborationMemoryVersionView, 0)}
	if p.Limit < 1 || p.Limit > 100 || p.BeforeVersion.Valid && p.BeforeVersion.Int64 < 1 {
		return out, collaborationError(http.StatusBadRequest, "invalid_memory_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		out.CanPurge = collaborationMemoryInitiator(access, collaborationMemoryViewer(ctx))
		var err error
		out.Items, out.CurrentVersion, out.NextCursor, err = listCollaborationMemoryVersions(ctx, q, access.Issue.WorkspaceID, issueID, p)
		return err
	})
	return out, err
}

func listCollaborationMemoryVersions(ctx context.Context, q *db.Queries, workspace, issueID pgtype.UUID, p CollaborationMemoryVersionsQuery) ([]CollaborationMemoryVersionView, int64, *int64, error) {
	current, err := q.GetCollaborationMemoryMaxVersion(ctx, issueID)
	if err != nil {
		return nil, 0, nil, err
	}
	rows, err := q.ListCollaborationMemoryVersions(ctx, db.ListCollaborationMemoryVersionsParams{IssueID: issueID, BeforeVersion: p.BeforeVersion, RowLimit: int32(p.Limit + 1)})
	if err != nil {
		return nil, 0, nil, err
	}
	var next *int64
	if len(rows) > p.Limit {
		rows = rows[:p.Limit]
		cursor := rows[len(rows)-1].Version
		next = &cursor
	}
	refs := make([]collaborationMemoryAuthorRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, collaborationMemoryAuthorRef{kind: row.AuthorKind, user: row.AuthorUserID, agent: row.AgentID, run: row.RunID,
			owner: row.ResourceOwnerUserID, purgedBy: row.PurgedByUserID, purgedByAdmin: row.PurgedByAdmin})
	}
	authors, err := collaborationMemoryAuthors(ctx, q, workspace, refs)
	if err != nil {
		return nil, 0, nil, err
	}
	accounts, err := collaborationMemoryPurgers(ctx, q, workspace, refs)
	if err != nil {
		return nil, 0, nil, err
	}
	items := make([]CollaborationMemoryVersionView, 0, len(rows))
	for i, row := range rows {
		items = append(items, collaborationMemoryVersionView(row.Version, row.Bytes, authors[i], row.CreatedAt, row.RestoredFromVersion,
			row.PurgedAt, row.PurgedByAdmin, accounts[i]))
	}
	return items, current, next, nil
}

func (s *CollaborationService) GetMemoryVersion(ctx context.Context, issueID pgtype.UUID, version int64) (CollaborationMemoryVersionDetail, error) {
	var out CollaborationMemoryVersionDetail
	if version < 1 {
		return out, collaborationError(http.StatusBadRequest, "invalid_memory_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		current, err := q.GetCollaborationMemoryMaxVersion(ctx, issueID)
		if err != nil {
			return err
		}
		row, err := q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: issueID, Version: version})
		if errors.Is(err, pgx.ErrNoRows) {
			// Distinct from issue denial: a missing version keeps the issue readable.
			return collaborationError(http.StatusNotFound, "memory_version_not_found")
		}
		if err != nil {
			return err
		}
		refs := []collaborationMemoryAuthorRef{memoryAuthorRef(row)}
		authors, err := collaborationMemoryAuthors(ctx, q, access.Issue.WorkspaceID, refs)
		if err != nil {
			return err
		}
		purgers, err := collaborationMemoryPurgers(ctx, q, access.Issue.WorkspaceID, refs)
		if err != nil {
			return err
		}
		out = CollaborationMemoryVersionDetail{IssueID: issueID, CurrentVersion: current, MaxBytes: s.MemoryLimit(),
			CollaborationMemoryVersionView: collaborationMemoryVersionView(row.Version, row.Bytes, authors[0], row.CreatedAt,
				row.RestoredFromVersion, row.PurgedAt, row.PurgedByAdmin, purgers[0])}
		if !row.PurgedAt.Valid {
			content := row.Content
			out.Content = &content
		}
		return nil
	})
	return out, err
}

func collaborationMemoryVersionView(version int64, bytes int32, author *CollaborationMemoryAuthor, created pgtype.Timestamptz,
	restored pgtype.Int8, purgedAt pgtype.Timestamptz, purgedByAdmin bool, purgedBy *CollaborationContentAccount) CollaborationMemoryVersionView {
	return CollaborationMemoryVersionView{Version: version, Bytes: int(bytes), Author: author, CreatedAt: created.Time,
		RestoredFromVersion: util.Int8ToPtr(restored), Purged: purgedAt.Valid, PurgedAt: collaborationContentTime(purgedAt),
		PurgedBy: purgedBy, PurgedByAdmin: purgedByAdmin}
}

type collaborationMemoryAuthorRef struct {
	kind                    string
	user, agent, run, owner pgtype.UUID
	purgedBy                pgtype.UUID
	purgedByAdmin           bool
}

func memoryAuthorRef(row db.CollaborationMemoryVersion) collaborationMemoryAuthorRef {
	return collaborationMemoryAuthorRef{kind: row.AuthorKind, user: row.AuthorUserID, agent: row.AgentID, run: row.RunID,
		owner: row.ResourceOwnerUserID, purgedBy: row.PurgedByUserID, purgedByAdmin: row.PurgedByAdmin}
}

func collaborationMemoryAuthors(ctx context.Context, q *db.Queries, workspace pgtype.UUID, refs []collaborationMemoryAuthorRef) ([]*CollaborationMemoryAuthor, error) {
	users, agents := make([]pgtype.UUID, 0, len(refs)), make([]pgtype.UUID, 0)
	for _, ref := range refs {
		users = append(users, ref.user, ref.owner)
		if ref.agent.Valid {
			agents = append(agents, ref.agent)
		}
	}
	accounts, err := collaborationContentAccounts(ctx, q, workspace, users)
	if err != nil {
		return nil, err
	}
	names := make(map[pgtype.UUID]string, len(agents))
	if len(agents) > 0 {
		rows, err := q.ListCollaborationMemoryAgentNames(ctx, agents)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			names[row.ID] = row.Name
		}
	}
	out := make([]*CollaborationMemoryAuthor, 0, len(refs))
	for _, ref := range refs {
		author := &CollaborationMemoryAuthor{Kind: ref.kind}
		if ref.kind == "agent" {
			author.Agent = &CollaborationMemoryAgent{AgentID: ref.agent, Name: names[ref.agent], RunID: ref.run}
			author.ResourceOwner = accounts[ref.owner]
		} else {
			author.Member = accounts[ref.user]
		}
		out = append(out, author)
	}
	return out, nil
}

// An administrator purge names no account to readers; the real administrator
// is retained in the version row and the administration audit.
func collaborationMemoryPurgers(ctx context.Context, q *db.Queries, workspace pgtype.UUID, refs []collaborationMemoryAuthorRef) ([]*CollaborationContentAccount, error) {
	ids := make([]pgtype.UUID, 0, len(refs))
	for _, ref := range refs {
		if ref.purgedBy.Valid && !ref.purgedByAdmin {
			ids = append(ids, ref.purgedBy)
		}
	}
	out := make([]*CollaborationContentAccount, len(refs))
	if len(ids) == 0 {
		return out, nil
	}
	accounts, err := collaborationContentAccounts(ctx, q, workspace, ids)
	if err != nil {
		return nil, err
	}
	for i, ref := range refs {
		if ref.purgedBy.Valid && !ref.purgedByAdmin {
			out[i] = accounts[ref.purgedBy]
		}
	}
	return out, nil
}
