package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// KnowledgeConsumerFacts is server-only input assembled by the task/runtime
// owner after authenticating and resolving its real consumer and output scope.
// It is deliberately not an HTTP DTO. Accountable attribution is never a field.
type KnowledgeConsumerFacts struct {
	Supported            bool
	Surface              string
	WorkspaceID          pgtype.UUID
	ProjectID            pgtype.UUID
	OriginatorUserID     pgtype.UUID
	RuntimeOwnerUserID   pgtype.UUID
	AgentID              pgtype.UUID
	RuntimeID            pgtype.UUID
	TaskID               pgtype.UUID
	ChatSessionID        pgtype.UUID
	CollaborationIssueID pgtype.UUID
	InputAuthorUserIDs   []pgtype.UUID
	// Audience is workspace, private_chat, collaboration, or unavailable.
	Audience string
}

type KnowledgeManifestEntry = protocol.TaskKnowledgeManifestEntry
type KnowledgeManifest = protocol.TaskKnowledgeManifest
type KnowledgeTargetRevision = protocol.TaskKnowledgeTargetRevision
type KnowledgeResolution struct {
	Contributions  []protocol.CollaborationKnowledgeContribution
	Manifest       KnowledgeManifest
	ManifestSHA256 string
	Targets        []KnowledgeTargetRevision
	Reason         string
}
type KnowledgeSourceLockHint struct {
	IssueID     pgtype.UUID
	WorkspaceID pgtype.UUID
	Sources     []protocol.CollaborationKnowledgeSource
}
type KnowledgeReadLockHints struct {
	Targets      []KnowledgeTargetRevision
	Sources      []KnowledgeSourceLockHint
	CandidateIDs []pgtype.UUID
}

func knowledgeConsumerTargets(f KnowledgeConsumerFacts) []protocol.CollaborationKnowledgeTarget {
	if !f.WorkspaceID.Valid {
		return nil
	}
	ws := util.UUIDToString(f.WorkspaceID)
	targets := []protocol.CollaborationKnowledgeTarget{{Kind: "workspace", WorkspaceID: ws}}
	if f.ProjectID.Valid {
		targets = append(targets, protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: ws, ProjectID: util.UUIDToString(f.ProjectID)})
	}
	slices.SortFunc(targets, func(a, b protocol.CollaborationKnowledgeTarget) int {
		if a.Kind < b.Kind {
			return -1
		}
		if a.Kind > b.Kind {
			return 1
		}
		return knowledgeCompareUUID(knowledgeTargetID(a), knowledgeTargetID(b))
	})
	return targets
}

func knowledgeTargetApplies(f KnowledgeConsumerFacts, target protocol.CollaborationKnowledgeTarget) bool {
	return slices.Contains(knowledgeConsumerTargets(f), target)
}

// ResolveApprovedKnowledge performs no transaction creation, implicit locking,
// HTTP principal impersonation or stable-context writes. C owns the independent
// authorization transaction and verifies its lock hints before exposing bytes.
func (s *CollaborationService) ResolveApprovedKnowledge(ctx context.Context, q *db.Queries, f KnowledgeConsumerFacts) (KnowledgeResolution, error) {
	out := KnowledgeResolution{Contributions: []protocol.CollaborationKnowledgeContribution{}, Manifest: KnowledgeManifest{ProtocolVersion: 1, Entries: []KnowledgeManifestEntry{}}, Targets: []KnowledgeTargetRevision{}}
	finish := func(reason string) (KnowledgeResolution, error) {
		out.Reason = reason
		raw, _ := json.Marshal(out.Manifest)
		out.ManifestSHA256 = protocol.CollaborationKnowledgeHash(string(raw))
		return out, nil
	}
	reason, err := s.knowledgeConsumer(ctx, q, f)
	if err != nil {
		return out, err
	}
	if reason != "" {
		return finish(reason)
	}
	for _, target := range knowledgeConsumerTargets(f) {
		rows, revision, err := knowledgeTargetRowsForResolution(ctx, q, target)
		if err != nil {
			return out, err
		}
		out.Targets = append(out.Targets, KnowledgeTargetRevision{Target: target, Revision: revision})
		for _, row := range rows {
			eligible, why, err := s.knowledgeEligible(ctx, q, f, row)
			if err != nil {
				return out, err
			}
			if !eligible {
				if out.Reason == "" {
					out.Reason = why
				}
				continue
			}
			item, err := knowledgeContribution(row)
			if err != nil {
				return out, err
			}
			out.Contributions = append(out.Contributions, item)
			out.Manifest.Entries = append(out.Manifest.Entries, knowledgeManifestEntry(row))
		}
	}
	slices.SortFunc(out.Manifest.Entries, func(a, b KnowledgeManifestEntry) int {
		if a.CandidateID < b.CandidateID {
			return -1
		}
		if a.CandidateID > b.CandidateID {
			return 1
		}
		return 0
	})
	if err := protocol.ValidateCollaborationKnowledgeRuntimeBudget(out.Contributions, 0); err != nil {
		return out, knowledgeFailure(503, "knowledge_unavailable")
	}
	return finish(out.Reason)
}

func knowledgeManifestEntry(row db.CollaborationKnowledgeCandidate) KnowledgeManifestEntry {
	return KnowledgeManifestEntry{CandidateID: util.UUIDToString(row.ID), Version: strconv.FormatInt(row.Version, 10), ContentSHA256: row.ContentSha256, SourceIssueID: util.UUIDToString(row.SourceIssueID), Target: knowledgeTarget(row)}
}

// Revalidation checks the exact disclosed identities and never substitutes the
// current target collection. The response contains no candidate body.
func (s *CollaborationService) RevalidateKnowledgeManifest(ctx context.Context, q *db.Queries, f KnowledgeConsumerFacts, manifest KnowledgeManifest) (bool, string, error) {
	if manifest.ProtocolVersion != 1 || manifest.Entries == nil || len(manifest.Entries) > protocol.CollaborationKnowledgeMaxRuntimeItems {
		return false, "source_unavailable", nil
	}
	raw, err := json.Marshal(manifest)
	if err != nil || len(raw) > protocol.CollaborationKnowledgeMaxDeliveryBytes {
		return false, "source_unavailable", nil
	}
	if len(manifest.Entries) == 0 {
		return true, "", nil
	}
	reason, err := s.knowledgeConsumer(ctx, q, f)
	if err != nil || reason != "" {
		return false, reason, err
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Entries {
		id, err := util.ParseUUID(entry.CandidateID)
		if err != nil || seen[entry.CandidateID] {
			return false, "source_unavailable", nil
		}
		seen[entry.CandidateID] = true
		row, err := q.GetKnowledgeCandidate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "source_unavailable", nil
		}
		if err != nil {
			return false, "", err
		}
		if knowledgeManifestEntry(row) != entry {
			return false, "source_unavailable", nil
		}
		ok, reason, err := s.knowledgeEligible(ctx, q, f, row)
		if err != nil || !ok {
			return false, reason, err
		}
	}
	return true, "", nil
}

func knowledgeTargetRowsForResolution(ctx context.Context, q *db.Queries, target protocol.CollaborationKnowledgeTarget) ([]db.CollaborationKnowledgeCandidate, string, error) {
	ws, _ := util.ParseUUID(target.WorkspaceID)
	project, _ := util.ParseUUID(target.ProjectID)
	if _, err := q.GetKnowledgeWorkspaceIdentity(ctx, ws); err != nil {
		return nil, "", err
	}
	if project.Valid {
		if _, err := q.GetKnowledgeProjectIdentity(ctx, db.GetKnowledgeProjectIdentityParams{ID: project, WorkspaceID: ws}); err != nil {
			return nil, "", err
		}
	}
	rows, err := q.ListKnowledgeActiveTargetCandidates(ctx, db.ListKnowledgeActiveTargetCandidatesParams{TargetKind: target.Kind, TargetWorkspaceID: ws, TargetProjectID: project, CheckedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}})
	if err != nil {
		return nil, "", err
	}
	if len(rows) > protocol.CollaborationKnowledgeMaxTargetItems {
		return nil, "", knowledgeFailure(503, "knowledge_unavailable")
	}
	counter, err := q.GetKnowledgeTarget(ctx, db.GetKnowledgeTargetParams{TargetKind: target.Kind, TargetID: knowledgeTargetID(target)})
	if errors.Is(err, pgx.ErrNoRows) {
		// Missing counters with any ever-published history are corruption, even
		// if every historical contribution is now expired/withdrawn.
		head, err := q.GetKnowledgeTargetHead(ctx, db.GetKnowledgeTargetHeadParams{TargetKind: target.Kind, WorkspaceID: ws, ProjectID: project, TargetConfirmer: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return rows, "0", nil
		}
		if err != nil {
			return nil, "", err
		}
		if head.ID.Valid {
			return nil, "", knowledgeFailure(503, "knowledge_unavailable")
		}
	} else if err != nil {
		return nil, "", err
	}
	return rows, strconv.FormatInt(counter.KnowledgeRevision, 10), nil
}

func (s *CollaborationService) knowledgeConsumer(ctx context.Context, q *db.Queries, f KnowledgeConsumerFacts) (string, error) {
	if !f.Supported {
		return "unsupported_client", nil
	}
	if !f.OriginatorUserID.Valid {
		return "missing_originator", nil
	}
	if !f.RuntimeOwnerUserID.Valid || !f.RuntimeID.Valid {
		return "missing_runtime_owner", nil
	}
	if q == nil {
		return "", knowledgeFailure(503, "knowledge_unavailable")
	}
	if !f.WorkspaceID.Valid || !f.AgentID.Valid {
		return "audience_unavailable", nil
	}
	if f.Surface != "collaboration" && !f.TaskID.Valid {
		return "audience_unavailable", nil
	}
	switch f.Surface {
	case "issue", "chat", "quick_create", "autopilot", "collaboration":
	default:
		return "target_not_applicable", nil
	}
	runtime, err := q.GetAgentRuntime(ctx, f.RuntimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "missing_runtime_owner", nil
	}
	if err != nil {
		return "", err
	}
	agent, err := q.GetAgent(ctx, f.AgentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "target_not_applicable", nil
	}
	if err != nil {
		return "", err
	}
	if runtime.OwnerID != f.RuntimeOwnerUserID || agent.RuntimeID != runtime.ID || agent.ArchivedAt.Valid {
		return "missing_runtime_owner", nil
	}
	if f.TaskID.Valid {
		_, err := q.GetChannelTaskDelivery(ctx, f.TaskID)
		if err == nil {
			return "audience_unavailable", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	switch f.Audience {
	case "workspace":
		if f.Surface == "chat" || f.Surface == "collaboration" {
			return "audience_unavailable", nil
		}
	case "private_chat":
		if f.Surface != "chat" || !f.ChatSessionID.Valid || !f.TaskID.Valid {
			return "audience_unavailable", nil
		}
		chat, err := q.GetChatSession(ctx, f.ChatSessionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "audience_unavailable", nil
		}
		if err != nil {
			return "", err
		}
		if chat.CreatorID != f.OriginatorUserID || chat.WorkspaceID != f.WorkspaceID || chat.AgentID != f.AgentID {
			return "audience_unavailable", nil
		}
		allowed, err := knowledgeCanReadAgent(ctx, q, agent, chat.CreatorID)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "audience_unavailable", nil
		}
	case "collaboration":
		if f.Surface != "collaboration" || !f.CollaborationIssueID.Valid {
			return "audience_unavailable", nil
		}
	default:
		return "audience_unavailable", nil
	}
	return "", nil
}

func knowledgeCanReadAgent(ctx context.Context, q *db.Queries, agent db.Agent, user pgtype.UUID) (bool, error) {
	if agent.OwnerID == user {
		return true, nil
	}
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: user, WorkspaceID: agent.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if member.Role == "owner" || member.Role == "admin" {
		return true, nil
	}
	if agent.PermissionMode != "public_to" {
		return false, nil
	}
	targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return false, err
	}
	for _, target := range targets {
		if target.TargetType == "workspace" || target.TargetType == "member" && target.TargetID == user {
			return true, nil
		}
	}
	return false, nil
}

func knowledgeUserReadsSource(ctx context.Context, q *db.Queries, issueID, userID pgtype.UUID) (bool, error) {
	if !userID.Valid {
		return false, nil
	}
	row, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: userID, IssueID: issueID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !row.AccountActive || !row.SourceMember && !row.Participant {
		return false, nil
	}
	return collaborationRecipientEligible(ctx, q, userID, row.Issue.WorkspaceID)
}

func (s *CollaborationService) knowledgeEligible(ctx context.Context, q *db.Queries, f KnowledgeConsumerFacts, row db.CollaborationKnowledgeCandidate) (bool, string, error) {
	issue, err := q.GetIssue(ctx, row.SourceIssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "source_unavailable", nil
	}
	if err != nil {
		return false, "", err
	}
	if issue.WorkspaceID != row.SourceWorkspaceID {
		return false, "source_unavailable", nil
	}
	if row.State != "approved" || !knowledgeTargetApplies(f, knowledgeTarget(row)) {
		return false, "target_not_applicable", nil
	}
	if row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(time.Now()) {
		return false, "expired", nil
	}
	var applicable protocol.CollaborationKnowledgeApplicability
	if json.Unmarshal(row.Applicability, &applicable) != nil {
		return false, "source_unavailable", nil
	}
	if !slices.Contains(applicable.Surfaces, f.Surface) {
		return false, "target_not_applicable", nil
	}
	users := append([]pgtype.UUID{f.OriginatorUserID, f.RuntimeOwnerUserID}, f.InputAuthorUserIDs...)
	if len(users) > 1026 {
		return false, "audience_unavailable", nil
	}
	for _, user := range users {
		allowed, err := knowledgeUserReadsSource(ctx, q, row.SourceIssueID, user)
		if err != nil {
			return false, "", err
		}
		if !allowed {
			return false, "source_access_denied", nil
		}
	}
	switch f.Audience {
	case "workspace":
		if row.SourceWorkspaceID != f.WorkspaceID {
			return false, "audience_unavailable", nil
		}
	case "collaboration":
		if row.SourceIssueID != f.CollaborationIssueID {
			issue, err := q.GetIssue(ctx, f.CollaborationIssueID)
			if errors.Is(err, pgx.ErrNoRows) {
				return false, "audience_unavailable", nil
			}
			if err != nil {
				return false, "", err
			}
			if issue.WorkspaceID != row.SourceWorkspaceID {
				return false, "audience_unavailable", nil
			}
			people, err := q.ListKnowledgeAudienceUsers(ctx, issue.ID)
			if err != nil {
				return false, "", err
			}
			if len(people) > 1024 {
				return false, "audience_unavailable", nil
			}
			for _, user := range people {
				allowed, err := knowledgeUserReadsSource(ctx, q, row.SourceIssueID, user)
				if err != nil {
					return false, "", err
				}
				if !allowed {
					return false, "audience_unavailable", nil
				}
			}
		}
	}
	refs, err := knowledgeCandidateSources(row)
	if err != nil {
		return false, "source_unavailable", nil
	}
	available, err := s.KnowledgeSourceAvailability(ctx, q, row.SourceIssueID, refs)
	if err != nil {
		return false, "", err
	}
	if !available {
		return false, "source_unavailable", nil
	}
	return true, "", nil
}
