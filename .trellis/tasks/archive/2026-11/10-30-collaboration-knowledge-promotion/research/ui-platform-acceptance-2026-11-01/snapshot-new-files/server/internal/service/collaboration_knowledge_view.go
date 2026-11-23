package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type KnowledgeProjection struct {
	ProtocolVersion int        `json:"protocol_version"`
	CheckedAt       time.Time  `json:"projection_checked_at"`
	ValidUntil      time.Time  `json:"valid_until"`
	NextExpiryAt    *time.Time `json:"next_expiry_at"`
}
type KnowledgePageQuery struct {
	Limit  int
	Cursor string
}
type knowledgeCandidateControl struct {
	CandidateID   string `json:"candidate_id"`
	State         string `json:"state"`
	StateRevision string `json:"state_revision"`
	Availability  string `json:"availability"`
}
type knowledgeCandidateMetadata struct {
	LineageID               string                                `json:"lineage_id"`
	Version                 string                                `json:"version"`
	Target                  protocol.CollaborationKnowledgeTarget `json:"target"`
	SourceWorkspaceID       string                                `json:"source_workspace_id"`
	SourceIssueID           string                                `json:"source_issue_id"`
	ProposedByUserID        string                                `json:"proposed_by_user_id"`
	ProposedAt              time.Time                             `json:"proposed_at"`
	ExpiresAt               *time.Time                            `json:"expires_at"`
	SupersedesCandidateID   *string                               `json:"supersedes_candidate_id"`
	DerivedFromCandidateID  *string                               `json:"derived_from_candidate_id"`
	ApprovedByUserID        *string                               `json:"approved_by_user_id"`
	ApprovedAt              *time.Time                            `json:"approved_at"`
	PublishedTargetRevision *string                               `json:"published_target_revision"`
}
type KnowledgeDecision struct {
	ActorUserID string    `json:"actor_user_id"`
	At          time.Time `json:"at"`
	Reason      *string   `json:"reason"`
}
type knowledgeDecisionMetadata struct {
	ActorUserID string    `json:"actor_user_id"`
	At          time.Time `json:"at"`
}
type knowledgeCandidateMaterial struct {
	Summary       string                                       `json:"summary"`
	ContentSHA256 string                                       `json:"content_sha256"`
	Sources       []protocol.CollaborationKnowledgeSource      `json:"sources"`
	Evidence      []protocol.CollaborationKnowledgeEvidence    `json:"evidence"`
	Applicability protocol.CollaborationKnowledgeApplicability `json:"applicability"`
}

// The discriminant owns serialization. Restricted rows cannot accidentally
// expose a zero-valued source/version field or a quoted decision reason.
type KnowledgeCandidateView struct {
	knowledgeCandidateControl
	Metadata     knowledgeCandidateMetadata
	Material     knowledgeCandidateMaterial
	Decision     *KnowledgeDecision
	Capabilities collaboration.KnowledgeCapabilities
	Content      *string
}

func (v KnowledgeCandidateView) MarshalJSON() ([]byte, error) {
	switch v.Availability {
	case "restricted":
		return json.Marshal(struct {
			knowledgeCandidateControl
			Capabilities struct {
				CanWithdraw bool `json:"can_withdraw"`
			} `json:"capabilities"`
		}{v.knowledgeCandidateControl, struct {
			CanWithdraw bool `json:"can_withdraw"`
		}{v.Capabilities.Withdraw}})
	case "source_unavailable":
		var decision *knowledgeDecisionMetadata
		if v.Decision != nil {
			decision = &knowledgeDecisionMetadata{ActorUserID: v.Decision.ActorUserID, At: v.Decision.At}
		}
		return json.Marshal(struct {
			knowledgeCandidateControl
			knowledgeCandidateMetadata
			Decision     *knowledgeDecisionMetadata          `json:"decision"`
			Capabilities collaboration.KnowledgeCapabilities `json:"capabilities"`
		}{v.knowledgeCandidateControl, v.Metadata, decision, v.Capabilities})
	case "available":
		return json.Marshal(struct {
			knowledgeCandidateControl
			knowledgeCandidateMetadata
			knowledgeCandidateMaterial
			Decision     *KnowledgeDecision                  `json:"decision"`
			Capabilities collaboration.KnowledgeCapabilities `json:"capabilities"`
			Content      *string                             `json:"content,omitempty"`
		}{v.knowledgeCandidateControl, v.Metadata, v.Material, v.Decision, v.Capabilities, v.Content})
	default:
		return nil, knowledgeFailure(503, "knowledge_unavailable")
	}
}

type KnowledgeCandidatesPage struct {
	KnowledgeProjection
	IssueID      string                   `json:"issue_id"`
	Items        []KnowledgeCandidateView `json:"items"`
	NextCursor   *string                  `json:"next_cursor"`
	Capabilities struct {
		CanPropose bool `json:"can_propose"`
	} `json:"capabilities"`
}
type KnowledgeCandidateDetail struct {
	KnowledgeProjection
	Candidate        KnowledgeCandidateView   `json:"candidate"`
	History          []KnowledgeCandidateView `json:"history"`
	HistoryTruncated bool                     `json:"history_truncated"`
}
type KnowledgeTargetPage struct {
	KnowledgeProjection
	Target            protocol.CollaborationKnowledgeTarget `json:"target"`
	KnowledgeRevision string                                `json:"knowledge_revision"`
	Items             []KnowledgeCandidateView              `json:"items"`
	NextCursor        *string                               `json:"next_cursor"`
}
type knowledgePageCursor struct {
	ProtocolVersion int        `json:"protocol_version"`
	Scope           string     `json:"scope"`
	ViewerID        string     `json:"viewer_id"`
	CheckedAt       time.Time  `json:"checked_at"`
	ValidUntil      time.Time  `json:"valid_until"`
	NextExpiryAt    *time.Time `json:"next_expiry_at"`
	AfterAt         time.Time  `json:"after_at"`
	AfterID         string     `json:"after_id"`
	ThroughAt       time.Time  `json:"through_at"`
	ThroughID       string     `json:"through_id"`
}

func knowledgeNewProjection() KnowledgeProjection {
	now := time.Now().UTC()
	return KnowledgeProjection{ProtocolVersion: 1, CheckedAt: now, ValidUntil: now.Add(30 * time.Second)}
}
func (p *KnowledgeProjection) expiry(at *time.Time) {
	if at != nil && at.After(p.CheckedAt) && (p.NextExpiryAt == nil || at.Before(*p.NextExpiryAt)) {
		p.NextExpiryAt = at
		if at.Before(p.ValidUntil) {
			p.ValidUntil = *at
		}
	}
}
func knowledgeCursor(p KnowledgeProjection, scope string, viewer pgtype.UUID, after db.CollaborationKnowledgeCandidate, throughAt time.Time, throughID string) *string {
	raw, _ := json.Marshal(knowledgePageCursor{ProtocolVersion: 1, Scope: scope, ViewerID: util.UUIDToString(viewer), CheckedAt: p.CheckedAt, ValidUntil: p.ValidUntil, NextExpiryAt: p.NextExpiryAt, AfterAt: after.ProposedAt.Time.UTC(), AfterID: util.UUIDToString(after.ID), ThroughAt: throughAt, ThroughID: throughID})
	v := base64.RawURLEncoding.EncodeToString(raw)
	return &v
}
func knowledgeParsePage(p KnowledgePageQuery, scope string, viewer pgtype.UUID) (KnowledgePageQuery, knowledgePageCursor, error) {
	if p.Limit == 0 {
		p.Limit = 20
	}
	var cursor knowledgePageCursor
	if p.Limit < 1 || p.Limit > 50 || len(p.Cursor) > 2048 {
		return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if p.Cursor == "" {
		return p, cursor, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err != nil || iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(&cursor) != nil || decoder.Decode(new(any)) != io.EOF || cursor.ProtocolVersion != 1 || cursor.Scope != scope || cursor.ViewerID != util.UUIDToString(viewer) {
		return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 10 {
		return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
	}
	for _, key := range []string{"protocol_version", "scope", "viewer_id", "checked_at", "valid_until", "next_expiry_at", "after_at", "after_id", "through_at", "through_id"} {
		if _, ok := fields[key]; !ok {
			return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
		}
	}
	if _, err = util.ParseUUID(cursor.AfterID); err != nil {
		return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if _, err = util.ParseUUID(cursor.ThroughID); err != nil {
		return p, cursor, knowledgeFailure(400, "invalid_knowledge_request")
	}
	now := time.Now().UTC()
	if cursor.CheckedAt.IsZero() || cursor.CheckedAt.After(now) || !cursor.ValidUntil.After(now) || cursor.ValidUntil.After(cursor.CheckedAt.Add(30*time.Second)) || cursor.AfterAt.IsZero() || cursor.ThroughAt.IsZero() || cursor.AfterAt.After(cursor.ThroughAt) || cursor.NextExpiryAt != nil && cursor.ValidUntil.After(*cursor.NextExpiryAt) {
		return p, cursor, knowledgeFailure(409, "knowledge_context_changed")
	}
	return p, cursor, nil
}
func (c knowledgePageCursor) projection() KnowledgeProjection {
	return KnowledgeProjection{ProtocolVersion: 1, CheckedAt: c.CheckedAt, ValidUntil: c.ValidUntil, NextExpiryAt: c.NextExpiryAt}
}

type knowledgeViewBuilder struct {
	s            *CollaborationService
	q            *db.Queries
	viewer       pgtype.UUID
	principal    collaboration.Principal
	access       map[pgtype.UUID]CollaborationIssueAccess
	availability map[string]bool
	patExpiry    *time.Time
}

func (s *CollaborationService) readKnowledge(ctx context.Context, fn func(*knowledgeViewBuilder) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return knowledgePublicError(err)
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return knowledgePublicError(err)
	}
	q := s.Queries.WithTx(tx)
	viewer, err := collaborationNotificationViewer(ctx, q)
	if err != nil {
		return knowledgePublicError(err)
	}
	p, _ := collaboration.PrincipalFromContext(ctx)
	v := &knowledgeViewBuilder{s: s, q: q, viewer: viewer, principal: p, access: map[pgtype.UUID]CollaborationIssueAccess{}, availability: map[string]bool{}}
	v.patExpiry, err = knowledgePAT(ctx, q, viewer, false)
	if err != nil {
		return knowledgePublicError(err)
	}
	if err = fn(v); err != nil {
		return knowledgePublicError(err)
	}
	return knowledgePublicError(tx.Commit(ctx))
}
func (v *knowledgeViewBuilder) source(ctx context.Context, issue pgtype.UUID) (CollaborationIssueAccess, error) {
	if a, ok := v.access[issue]; ok {
		return a, nil
	}
	a, err := v.s.ResolveIssueAccess(ctx, v.q, issue)
	if err != nil && !knowledgeErrorCode(err, "not_found") {
		return a, err
	}
	v.access[issue] = a
	return a, nil
}
func knowledgeReadTargetAuthority(ctx context.Context, q *db.Queries, viewer pgtype.UUID, target protocol.CollaborationKnowledgeTarget) (collaboration.KnowledgeTargetAuthority, error) {
	t := collaboration.KnowledgeTargetAuthority{Kind: target.Kind}
	ws, err := util.ParseUUID(target.WorkspaceID)
	if err != nil {
		return t, knowledgeFailure(400, "invalid_knowledge_request")
	}
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: viewer, WorkspaceID: ws})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return t, err
	}
	t.Member = member.ID.Valid
	t.MemberRole = member.Role
	t.NormallyVisible = t.Member
	if target.Kind == "workspace" {
		_, err = q.GetKnowledgeWorkspaceIdentity(ctx, ws)
		t.Exists = err == nil
	} else if target.Kind == "project" {
		id, e := util.ParseUUID(target.ProjectID)
		if e != nil {
			return t, knowledgeFailure(400, "invalid_knowledge_request")
		}
		if t.Member {
			var row db.GetKnowledgeProjectAuthorityRow
			row, err = q.GetKnowledgeProjectAuthority(ctx, db.GetKnowledgeProjectAuthorityParams{ID: id, WorkspaceID: ws})
			t.LeadType = row.LeadType.String
			t.LeadIsActor = row.LeadID == viewer
		} else {
			_, err = q.GetKnowledgeProjectIdentity(ctx, db.GetKnowledgeProjectIdentityParams{ID: id, WorkspaceID: ws})
		}
		t.Exists = err == nil
	} else {
		return t, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return t, err
	}
	return t, nil
}

func (v *knowledgeViewBuilder) candidate(ctx context.Context, row db.CollaborationKnowledgeCandidate, detail bool) (KnowledgeCandidateView, error) {
	out := KnowledgeCandidateView{knowledgeCandidateControl: knowledgeCandidateControl{CandidateID: util.UUIDToString(row.ID), State: row.State, StateRevision: strconv.FormatInt(row.StateRevision, 10), Availability: "restricted"}}
	a, err := v.source(ctx, row.SourceIssueID)
	if err != nil {
		return out, err
	}
	target, err := knowledgeReadTargetAuthority(ctx, v.q, v.viewer, knowledgeTarget(row))
	if err != nil {
		return out, err
	}
	if !a.Capabilities.Read {
		if !knowledgeTargetConfirmer(target) {
			return out, knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		out.Capabilities.Withdraw = row.State == "pending" || row.State == "approved"
		return out, nil
	}
	out.Metadata = knowledgeCandidateMetadata{LineageID: util.UUIDToString(row.LineageID), Version: strconv.FormatInt(row.Version, 10), Target: knowledgeTarget(row), SourceWorkspaceID: util.UUIDToString(row.SourceWorkspaceID), SourceIssueID: util.UUIDToString(row.SourceIssueID), ProposedByUserID: util.UUIDToString(row.ProposedByUserID), ProposedAt: row.ProposedAt.Time.UTC(), ExpiresAt: knowledgeTime(row.ExpiresAt), SupersedesCandidateID: knowledgeIDPointer(row.SupersedesCandidateID), DerivedFromCandidateID: knowledgeIDPointer(row.DerivedFromCandidateID), ApprovedByUserID: knowledgeIDPointer(row.ApprovedByUserID), ApprovedAt: knowledgeTime(row.ApprovedAt), PublishedTargetRevision: knowledgeVersionPointer(row.PublishedTargetRevision)}
	if row.LastDecidedAt.Valid {
		out.Decision = &KnowledgeDecision{ActorUserID: util.UUIDToString(row.LastDecidedByUserID), At: row.LastDecidedAt.Time.UTC()}
		if row.LastDecisionReason.Valid {
			reason := row.LastDecisionReason.String
			out.Decision.Reason = &reason
		}
	}
	refs, err := knowledgeCandidateSources(row)
	if err != nil {
		return out, err
	}
	cacheKey := util.UUIDToString(row.SourceIssueID) + ":" + string(row.Sources)
	available, known := v.availability[cacheKey]
	if !known {
		available, err = v.s.KnowledgeSourceAvailability(ctx, v.q, row.SourceIssueID, refs)
		if err != nil {
			return out, err
		}
		v.availability[cacheKey] = available
	}
	target.SourceWorkspace = row.TargetKind == "workspace" && row.TargetWorkspaceID == a.Issue.WorkspaceID
	target.CurrentSourceProject = row.TargetKind == "project" && row.TargetProjectID == a.Issue.ProjectID
	target.SourceProjectVisible = a.Issue.ProjectID.Valid
	out.Capabilities = collaboration.KnowledgeCapabilitiesFor(v.principal, collaboration.AccessState{AccountActive: true, SourceMember: a.SourceMember, Participant: a.ParticipantEpoch.Valid, Creator: a.Issue.CreatorType == "member" && a.Issue.CreatorID == v.viewer, Handler: a.Issue.AssigneeType.String == "member" && a.Issue.AssigneeID == v.viewer}, target, row.State, available, row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(time.Now()))
	out.Availability = "source_unavailable"
	if !available && out.Decision != nil {
		out.Decision.Reason = nil
	}
	if available {
		item, err := knowledgeContribution(row)
		if err != nil {
			return out, err
		}
		out.Availability = "available"
		out.Material = knowledgeCandidateMaterial{Summary: item.Summary, ContentSHA256: item.ContentSHA256, Sources: item.Sources, Evidence: item.Evidence, Applicability: item.Applicability}
		if detail {
			content := item.Content
			out.Content = &content
		}
	}
	return out, nil
}

func (s *CollaborationService) ListKnowledgeCandidates(ctx context.Context, issueID pgtype.UUID, p KnowledgePageQuery) (KnowledgeCandidatesPage, error) {
	out := KnowledgeCandidatesPage{KnowledgeProjection: knowledgeNewProjection(), IssueID: util.UUIDToString(issueID), Items: []KnowledgeCandidateView{}}
	err := s.readKnowledge(ctx, func(v *knowledgeViewBuilder) error {
		out.clampCredential(v.patExpiry)
		a, err := v.source(ctx, issueID)
		if err != nil {
			return err
		}
		if !a.Capabilities.Read {
			return knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		out.Capabilities.CanPropose = knowledgeManager(a.Issue, v.viewer, true)
		scope := "source:" + out.IssueID
		p, cursor, err := knowledgeParsePage(p, scope, v.viewer)
		if err != nil {
			return err
		}
		var throughAt pgtype.Timestamptz
		var throughID pgtype.UUID
		if p.Cursor != "" {
			out.KnowledgeProjection = cursor.projection()
			if v.patExpiry != nil && out.ValidUntil.After(*v.patExpiry) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			throughAt = pgtype.Timestamptz{Time: cursor.ThroughAt, Valid: true}
			throughID, _ = util.ParseUUID(cursor.ThroughID)
		} else {
			head, err := v.q.GetKnowledgeSourceHead(ctx, issueID)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			throughAt, throughID = head.ProposedAt, head.ID
			expiry, err := v.q.GetKnowledgeSourceNextExpiry(ctx, issueID)
			if err != nil {
				return err
			}
			out.expiry(knowledgeTime(expiry))
		}
		afterAt := pgtype.Timestamptz{}
		afterID := pgtype.UUID{}
		if p.Cursor != "" {
			afterAt = pgtype.Timestamptz{Time: cursor.AfterAt, Valid: true}
			afterID, _ = util.ParseUUID(cursor.AfterID)
		}
		rows, err := v.q.ListKnowledgeSourceRows(ctx, db.ListKnowledgeSourceRowsParams{SourceIssueID: issueID, AfterAt: afterAt, AfterID: afterID, ThroughAt: throughAt, ThroughID: throughID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		more := len(rows) > p.Limit
		if more {
			rows = rows[:p.Limit]
		}
		for _, row := range rows {
			item, err := v.candidate(ctx, row, false)
			if err != nil {
				return err
			}
			if err = knowledgePageItemLease(out.KnowledgeProjection, row, item); err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		if more {
			out.NextCursor = knowledgeCursor(out.KnowledgeProjection, scope, v.viewer, rows[len(rows)-1], throughAt.Time.UTC(), util.UUIDToString(throughID))
		}
		return knowledgeProjectionCurrent(out.KnowledgeProjection)
	})
	return out, err
}

func (s *CollaborationService) KnowledgeCandidate(ctx context.Context, id pgtype.UUID) (KnowledgeCandidateDetail, error) {
	out := KnowledgeCandidateDetail{KnowledgeProjection: knowledgeNewProjection(), History: []KnowledgeCandidateView{}}
	err := s.readKnowledge(ctx, func(v *knowledgeViewBuilder) error {
		out.clampCredential(v.patExpiry)
		row, err := v.q.GetKnowledgeCandidate(ctx, id)
		if err != nil {
			return err
		}
		out.Candidate, err = v.candidate(ctx, row, true)
		if err != nil {
			return err
		}
		if out.Candidate.Availability == "restricted" {
			return knowledgeProjectionCurrent(out.KnowledgeProjection)
		}
		expiry, err := v.q.GetKnowledgeSourceNextExpiry(ctx, row.SourceIssueID)
		if err != nil {
			return err
		}
		out.expiry(knowledgeTime(expiry))
		history, err := v.q.ListKnowledgeLineageRows(ctx, db.ListKnowledgeLineageRowsParams{LineageID: row.LineageID, Limit: 51})
		if err != nil {
			return err
		}
		out.HistoryTruncated = len(history) > 50
		if out.HistoryTruncated {
			history = history[:50]
		}
		for _, version := range history {
			item, err := v.candidate(ctx, version, false)
			if err != nil {
				return err
			}
			out.History = append(out.History, item)
		}
		return knowledgeProjectionCurrent(out.KnowledgeProjection)
	})
	return out, err
}

func (s *CollaborationService) ListTargetKnowledge(ctx context.Context, target protocol.CollaborationKnowledgeTarget, p KnowledgePageQuery) (KnowledgeTargetPage, error) {
	out := KnowledgeTargetPage{KnowledgeProjection: knowledgeNewProjection(), Target: target, Items: []KnowledgeCandidateView{}}
	err := s.readKnowledge(ctx, func(v *knowledgeViewBuilder) error {
		out.clampCredential(v.patExpiry)
		auth, err := knowledgeReadTargetAuthority(ctx, v.q, v.viewer, target)
		if err != nil {
			return err
		}
		if !auth.Exists || !auth.Member {
			return knowledgeFailure(404, "knowledge_target_not_found")
		}
		out.KnowledgeRevision, err = knowledgeReadTargetRevision(ctx, v.q, target)
		if err != nil {
			return err
		}
		scope := "target:" + target.Kind + ":" + target.WorkspaceID + ":" + knowledgeUUIDString(knowledgeTargetID(target))
		p, cursor, err := knowledgeParsePage(p, scope, v.viewer)
		if err != nil {
			return err
		}
		ws, _ := util.ParseUUID(target.WorkspaceID)
		project, _ := util.ParseUUID(target.ProjectID)
		confirmer := knowledgeTargetConfirmer(auth)
		var throughAt pgtype.Timestamptz
		var throughID pgtype.UUID
		if p.Cursor != "" {
			out.KnowledgeProjection = cursor.projection()
			if v.patExpiry != nil && out.ValidUntil.After(*v.patExpiry) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			throughAt = pgtype.Timestamptz{Time: cursor.ThroughAt, Valid: true}
			throughID, _ = util.ParseUUID(cursor.ThroughID)
		} else {
			head, err := v.q.GetKnowledgeTargetHead(ctx, db.GetKnowledgeTargetHeadParams{TargetKind: target.Kind, WorkspaceID: ws, ProjectID: project, TargetConfirmer: confirmer, ViewerID: v.viewer})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			throughAt, throughID = head.ProposedAt, head.ID
			expiry, err := v.q.GetKnowledgeTargetNextExpiry(ctx, db.GetKnowledgeTargetNextExpiryParams{TargetKind: target.Kind, WorkspaceID: ws, ProjectID: project, ViewerID: v.viewer})
			if err != nil {
				return err
			}
			out.expiry(knowledgeTime(expiry))
		}
		afterAt := pgtype.Timestamptz{}
		afterID := pgtype.UUID{}
		if p.Cursor != "" {
			afterAt = pgtype.Timestamptz{Time: cursor.AfterAt, Valid: true}
			afterID, _ = util.ParseUUID(cursor.AfterID)
		}
		rows, err := v.q.ListKnowledgeTargetRows(ctx, db.ListKnowledgeTargetRowsParams{TargetKind: target.Kind, WorkspaceID: ws, ProjectID: project, TargetConfirmer: confirmer, ViewerID: v.viewer, AfterAt: afterAt, AfterID: afterID, ThroughAt: throughAt, ThroughID: throughID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		more := len(rows) > p.Limit
		if more {
			rows = rows[:p.Limit]
		}
		for _, row := range rows {
			item, err := v.candidate(ctx, row, false)
			if err != nil {
				return err
			}
			if err = knowledgePageItemLease(out.KnowledgeProjection, row, item); err != nil {
				return err
			}
			out.Items = append(out.Items, item)
		}
		if more {
			out.NextCursor = knowledgeCursor(out.KnowledgeProjection, scope, v.viewer, rows[len(rows)-1], throughAt.Time.UTC(), util.UUIDToString(throughID))
		}
		return knowledgeProjectionCurrent(out.KnowledgeProjection)
	})
	return out, err
}

func knowledgeProjectionCurrent(p KnowledgeProjection) error {
	if !time.Now().Before(p.ValidUntil) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	return nil
}

func (p *KnowledgeProjection) clampCredential(expiry *time.Time) {
	if expiry != nil && expiry.Before(p.ValidUntil) {
		p.ValidUntil = *expiry
	}
}
func knowledgePageItemLease(p KnowledgeProjection, row db.CollaborationKnowledgeCandidate, item KnowledgeCandidateView) error {
	if item.Availability != "restricted" && (row.State == "pending" || row.State == "approved") && row.ExpiresAt.Valid && row.ExpiresAt.Time.After(p.CheckedAt) && row.ExpiresAt.Time.Before(p.ValidUntil) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	return nil
}
