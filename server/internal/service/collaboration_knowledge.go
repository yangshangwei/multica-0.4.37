package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeWrite struct {
	actor           pgtype.UUID
	principal       collaboration.Principal
	q               *db.Queries
	issue           db.Issue
	read            bool
	sourceMember    bool
	participant     bool
	target          protocol.CollaborationKnowledgeTarget
	targetAuthority collaboration.KnowledgeTargetAuthority
	candidate       db.CollaborationKnowledgeCandidate
	approved        db.CollaborationKnowledgeCandidate
	counter         *db.CollaborationKnowledgeTarget
	operation       db.CollaborationOperation
}

func (s *CollaborationService) knowledgeNow() time.Time {
	if s.knowledgeClock != nil {
		return s.knowledgeClock()
	}
	return time.Now()
}

func knowledgeFailure(status int, code string) error { return collaborationError(status, code) }
func knowledgePublicError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return knowledgeFailure(404, "knowledge_candidate_not_found")
	}
	var domain *CollaborationError
	if errors.As(err, &domain) {
		if strings.HasPrefix(domain.Code, "knowledge_") || domain.Code == "invalid_knowledge_request" || domain.Code == "idempotency_conflict" || domain.Code == "operation_not_found" {
			return err
		}
		if domain.Code == "collaboration_busy" {
			return knowledgeFailure(503, "knowledge_busy")
		}
		if domain.Status == 401 || domain.Status == 403 {
			return knowledgeFailure(403, "knowledge_permission_denied")
		}
		if domain.Status == 404 {
			return knowledgeFailure(404, "knowledge_candidate_not_found")
		}
	}
	return knowledgeFailure(503, "knowledge_unavailable")
}

func knowledgeTarget(row db.CollaborationKnowledgeCandidate) protocol.CollaborationKnowledgeTarget {
	return protocol.CollaborationKnowledgeTarget{Kind: row.TargetKind, WorkspaceID: util.UUIDToString(row.TargetWorkspaceID), ProjectID: knowledgeUUIDString(row.TargetProjectID)}
}
func knowledgeUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return util.UUIDToString(id)
}
func knowledgeTargetID(target protocol.CollaborationKnowledgeTarget) pgtype.UUID {
	value := target.WorkspaceID
	if target.Kind == "project" {
		value = target.ProjectID
	}
	id, _ := util.ParseUUID(value)
	return id
}
func knowledgeManager(issue db.Issue, actor pgtype.UUID, read bool) bool {
	return read && (issue.CreatorType == "member" && issue.CreatorID == actor || issue.AssigneeType.String == "member" && issue.AssigneeID == actor)
}
func knowledgeReceipt(row db.CollaborationKnowledgeCandidate, request, action, code string, counter *db.CollaborationKnowledgeTarget) protocol.CollaborationKnowledgeReceipt {
	r := protocol.CollaborationKnowledgeReceipt{ProtocolVersion: 1, RequestID: request, Action: action, Code: code, CandidateID: util.UUIDToString(row.ID), Version: strconv.FormatInt(row.Version, 10), StateRevision: strconv.FormatInt(row.StateRevision, 10), State: row.State}
	if counter != nil {
		v := strconv.FormatInt(counter.KnowledgeRevision, 10)
		r.TargetKnowledgeRevision = &v
	}
	return r
}
func knowledgeReceiptStatus(r protocol.CollaborationKnowledgeReceipt) int {
	switch r.Code {
	case "knowledge_proposed", "knowledge_approved", "knowledge_rejected", "knowledge_withdrawn":
		return http.StatusOK
	default:
		return http.StatusConflict
	}
}

func (s *CollaborationService) ProposeKnowledge(ctx context.Context, issueID pgtype.UUID, p protocol.CollaborationKnowledgeProposal) (protocol.CollaborationKnowledgeReceipt, error) {
	if !issueID.Valid {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if err := protocol.ValidateCollaborationKnowledgeProposal(p, util.UUIDToString(issueID)); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeProtocolError(err)
	}
	id, _ := util.ParseUUID(p.CandidateID)
	return s.mutateKnowledge(ctx, issueID, id, p.RequestID, "propose", p, &p, nil)
}
func (s *CollaborationService) ApproveKnowledge(ctx context.Context, id pgtype.UUID, p protocol.CollaborationKnowledgeApproveRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	var parsed protocol.CollaborationKnowledgeApproveRequest
	raw, _ := json.Marshal(p)
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeProtocolError(err)
	}
	return s.mutateKnowledge(ctx, pgtype.UUID{}, id, p.RequestID, "approve", p, nil, &p)
}
func (s *CollaborationService) RejectKnowledge(ctx context.Context, id pgtype.UUID, p protocol.CollaborationKnowledgeDecisionRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	return s.decideKnowledge(ctx, id, "reject", p)
}
func (s *CollaborationService) WithdrawKnowledge(ctx context.Context, id pgtype.UUID, p protocol.CollaborationKnowledgeDecisionRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	return s.decideKnowledge(ctx, id, "withdraw", p)
}
func (s *CollaborationService) decideKnowledge(ctx context.Context, id pgtype.UUID, action string, p protocol.CollaborationKnowledgeDecisionRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	var parsed protocol.CollaborationKnowledgeDecisionRequest
	raw, _ := json.Marshal(p)
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeProtocolError(err)
	}
	return s.mutateKnowledge(ctx, pgtype.UUID{}, id, p.RequestID, action, p, nil, nil)
}
func knowledgeProtocolError(err error) error {
	if errors.Is(err, protocol.ErrCollaborationKnowledgeRevisionRequired) {
		return knowledgeFailure(428, "knowledge_revision_required")
	}
	if errors.Is(err, protocol.ErrCollaborationKnowledgeTooLarge) {
		return knowledgeFailure(413, "knowledge_candidate_too_large")
	}
	return knowledgeFailure(400, "invalid_knowledge_request")
}

func (s *CollaborationService) mutateKnowledge(ctx context.Context, issueID, candidateID pgtype.UUID, request, action string, payload any, proposal *protocol.CollaborationKnowledgeProposal, approval *protocol.CollaborationKnowledgeApproveRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(403, "knowledge_permission_denied")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !candidateID.Valid {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	raw, err := json.Marshal(struct {
		CandidateID string `json:"candidate_id"`
		Action      string `json:"action"`
		Payload     any    `json:"payload"`
	}{util.UUIDToString(candidateID), action, payload})
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeProtocolError(err)
	}
	hash := protocol.CollaborationKnowledgeHash(string(raw))
	r, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationKnowledgeReceipt, error) {
		w := &knowledgeWrite{q: s.Queries.WithTx(tx), actor: actor, principal: principal}
		if proposal != nil {
			w.target = proposal.Target
		} else {
			row, err := w.q.GetKnowledgeCandidate(ctx, candidateID)
			if errors.Is(err, pgx.ErrNoRows) {
				return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "knowledge_candidate_not_found")
			}
			if err != nil {
				return protocol.CollaborationKnowledgeReceipt{}, err
			}
			w.candidate = row
			issueID = row.SourceIssueID
			w.target = knowledgeTarget(row)
		}
		if err := s.lockKnowledgeWrite(ctx, w, issueID, action); err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		if _, err := knowledgePAT(ctx, w.q, actor, false); err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		previous, err := w.q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: request})
		if err == nil {
			if previous.Kind != "knowledge_"+action || previous.PayloadHash != hash {
				return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(409, "idempotency_conflict")
			}
			return decodeKnowledgeReceipt(previous, candidateID, action, request)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		w.operation, err = w.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: request, Kind: "knowledge_" + action, PayloadHash: hash})
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		var receipt protocol.CollaborationKnowledgeReceipt
		if proposal != nil {
			receipt, err = s.createKnowledgeCandidate(ctx, w, *proposal)
		} else {
			receipt, err = s.changeKnowledgeCandidate(ctx, w, action, payload, approval)
		}
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		if _, err = knowledgePAT(ctx, w.q, actor, false); err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		_, err = w.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: w.operation.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: issueID})
		return receipt, err
	})
	if err != nil {
		return r, knowledgePublicError(err)
	}
	if status := knowledgeReceiptStatus(r); status >= 400 {
		return r, knowledgeFailure(status, r.Code)
	}
	return r, nil
}

func (s *CollaborationService) lockKnowledgeWrite(ctx context.Context, w *knowledgeWrite, issueID pgtype.UUID, action string) error {
	users, err := w.q.LockCollaborationUsers(ctx, []pgtype.UUID{w.actor})
	if err != nil {
		return err
	}
	var user db.User
	if len(users) == 1 {
		user = users[0]
	}
	if err = authorizeWorkflowAccount(ctx, w.q, w.actor, user); err != nil {
		return err
	}
	if _, err = knowledgePAT(ctx, w.q, w.actor, true); err != nil {
		return err
	}
	// A caller who cannot propose from this source must not use destination
	// errors to distinguish hidden source issues or enumerate target identities.
	if action == "propose" {
		access, err := s.ResolveIssueAccess(ctx, w.q, issueID)
		if err != nil {
			return err
		}
		if !knowledgeManager(access.Issue, w.actor, access.Capabilities.Read) {
			return knowledgeFailure(403, "knowledge_permission_denied")
		}
	}
	issue, err := w.q.GetIssue(ctx, issueID)
	if err != nil && !(action == "withdraw" && errors.Is(err, pgx.ErrNoRows)) {
		return err
	}
	w.issue = issue
	sourceWorkspace := issue.WorkspaceID
	if !sourceWorkspace.Valid {
		sourceWorkspace = w.candidate.SourceWorkspaceID
	}
	targetWorkspace, _ := util.ParseUUID(w.target.WorkspaceID)
	workspaces := []pgtype.UUID{sourceWorkspace, targetWorkspace}
	slices.SortFunc(workspaces, knowledgeCompareUUID)
	workspaces = slices.Compact(workspaces)
	for _, ws := range workspaces {
		_, err = w.q.LockKnowledgeWorkspace(ctx, ws)
		if errors.Is(err, pgx.ErrNoRows) && action == "propose" && ws == targetWorkspace {
			return knowledgeFailure(404, "knowledge_target_not_found")
		}
		if err != nil && !(action == "withdraw" && errors.Is(err, pgx.ErrNoRows)) {
			return err
		}
		if err = w.q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: w.actor}); err != nil {
			return err
		}
		if _, err = w.q.LockKnowledgeMember(ctx, db.LockKnowledgeMemberParams{WorkspaceID: ws, UserID: w.actor}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if issue.ID.Valid {
		if err = w.q.LockIssueStatusCatalogShared(ctx, sourceWorkspace); err != nil {
			return err
		}
	}
	member, err := w.q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: w.actor, WorkspaceID: targetWorkspace})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	targetExists := true
	usedSourceWithdrawalAuthority := false
	w.targetAuthority = collaboration.KnowledgeTargetAuthority{Kind: w.target.Kind, Member: member.ID.Valid, MemberRole: member.Role, SourceWorkspace: w.target.Kind == "workspace" && sourceWorkspace == targetWorkspace, CurrentSourceProject: w.target.Kind == "project" && issue.ProjectID == knowledgeTargetID(w.target), SourceProjectVisible: issue.ProjectID.Valid, NormallyVisible: member.ID.Valid}
	if w.target.Kind == "project" {
		p, err := w.q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: knowledgeTargetID(w.target), WorkspaceID: targetWorkspace})
		if errors.Is(err, pgx.ErrNoRows) {
			targetExists = false
		} else if err != nil {
			return err
		}
		needsLead := p.ID.Valid && member.ID.Valid && member.Role != "owner" && member.Role != "admin" && action != "propose"
		if needsLead && action == "withdraw" {
			access, err := s.ResolveIssueAccess(ctx, w.q, issueID)
			if err != nil && !knowledgeErrorCode(err, "not_found") {
				return err
			}
			if err == nil && knowledgeManager(access.Issue, w.actor, access.Capabilities.Read) {
				needsLead = false
				usedSourceWithdrawalAuthority = true
			}
		}
		if needsLead {
			lead, err := w.q.LockKnowledgeProjectLead(ctx, db.LockKnowledgeProjectLeadParams{ID: p.ID, WorkspaceID: targetWorkspace})
			if err != nil {
				return err
			}
			w.targetAuthority.LeadType = lead.LeadType.String
			w.targetAuthority.LeadIsActor = lead.LeadID == w.actor
		}
	} else {
		_, err := w.q.GetKnowledgeWorkspaceIdentity(ctx, targetWorkspace)
		if errors.Is(err, pgx.ErrNoRows) {
			targetExists = false
		} else if err != nil {
			return err
		}
	}
	w.targetAuthority.Exists = targetExists
	// A proposal never initializes or locks the effective target collection.
	collectionChange := action == "approve" || action == "withdraw" && w.candidate.State == "approved"
	if collectionChange && targetExists {
		counter, err := lockKnowledgeCounter(ctx, w.q, w.target, false)
		if err != nil {
			return err
		}
		w.counter = &counter
	}
	if issue.ID.Valid {
		w.issue, err = w.q.LockCollaborationIssue(ctx, issueID)
		if err != nil {
			return err
		}
		if w.issue.WorkspaceID != sourceWorkspace {
			return knowledgeFailure(409, "knowledge_source_unavailable")
		}
		if _, err = w.q.LockIssueCollaboration(ctx, issueID); err != nil {
			return err
		}
		if _, err = w.q.LockCollaborationSession(ctx, issueID); err != nil {
			return err
		}
		access, err := s.ResolveIssueAccess(ctx, w.q, issueID)
		if err == nil {
			w.read = access.Capabilities.Read
			w.sourceMember = access.SourceMember
			w.participant = access.ParticipantEpoch.Valid
		} else if !knowledgeErrorCode(err, "not_found") {
			return err
		}
	}
	w.targetAuthority.CurrentSourceProject = w.target.Kind == "project" && w.issue.ProjectID == knowledgeTargetID(w.target)
	if usedSourceWithdrawalAuthority && !knowledgeManager(w.issue, w.actor, w.read) {
		return &pgconn.PgError{Code: "40001", Message: "knowledge authority lock hints changed"}
	}
	caps := knowledgeWriteCapabilities(w)
	allowed := false
	switch action {
	case "propose":
		allowed = caps.Propose
	case "approve", "reject":
		allowed = w.read && knowledgeTargetConfirmer(w.targetAuthority)
	case "withdraw":
		allowed = knowledgeManager(w.issue, w.actor, w.read) || knowledgeTargetConfirmer(w.targetAuthority)
	}
	if !allowed {
		if action == "propose" && !w.read {
			return knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		if action == "propose" && knowledgeManager(w.issue, w.actor, w.read) {
			return knowledgeFailure(404, "knowledge_target_not_found")
		}
		if !w.read && !knowledgeTargetConfirmer(w.targetAuthority) {
			return knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		return knowledgeFailure(403, "knowledge_permission_denied")
	}
	if w.candidate.ID.Valid {
		w.approved, err = w.q.GetKnowledgeApprovedLineage(ctx, w.candidate.LineageID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		ids := []pgtype.UUID{w.candidate.ID, w.candidate.LineageID}
		if w.approved.ID.Valid {
			ids = append(ids, w.approved.ID)
		}
		if _, err = w.q.LockKnowledgeCandidates(ctx, ids); err != nil {
			return err
		}
		w.candidate, err = w.q.GetKnowledgeCandidate(ctx, w.candidate.ID)
		if err != nil {
			return err
		}
		if action == "withdraw" && w.candidate.State == "approved" && targetExists && w.counter == nil {
			return &pgconn.PgError{Code: "40001", Message: "knowledge lock hints changed"}
		}
	}
	return nil
}

func knowledgeTargetConfirmer(t collaboration.KnowledgeTargetAuthority) bool {
	return t.Exists && t.Member && (t.MemberRole == "owner" || t.MemberRole == "admin" || t.Kind == "project" && t.LeadType == "member" && t.LeadIsActor)
}
func knowledgeWriteCapabilities(w *knowledgeWrite) collaboration.KnowledgeCapabilities {
	return collaboration.KnowledgeCapabilitiesFor(w.principal, collaboration.AccessState{AccountActive: true, SourceMember: w.sourceMember, Participant: w.participant, Creator: w.issue.CreatorType == "member" && w.issue.CreatorID == w.actor, Handler: w.issue.AssigneeType.String == "member" && w.issue.AssigneeID == w.actor}, w.targetAuthority, w.candidate.State, true, false)
}

func knowledgeCompareUUID(a, b pgtype.UUID) int {
	return strings.Compare(knowledgeUUIDString(a), knowledgeUUIDString(b))
}
func knowledgeErrorCode(err error, code string) bool {
	var e *CollaborationError
	return errors.As(err, &e) && e.Code == code
}

func lockKnowledgeCounter(ctx context.Context, q *db.Queries, target protocol.CollaborationKnowledgeTarget, read bool) (db.CollaborationKnowledgeTarget, error) {
	key := db.GetKnowledgeTargetParams{TargetKind: target.Kind, TargetID: knowledgeTargetID(target)}
	_, err := q.GetKnowledgeTarget(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		previous, err := q.GetKnowledgeLockTimeout(ctx)
		if err != nil {
			return db.CollaborationKnowledgeTarget{}, err
		}
		if _, err = q.SetKnowledgeLockTimeout(ctx, "25ms"); err != nil {
			return db.CollaborationKnowledgeTarget{}, err
		}
		ws, _ := util.ParseUUID(target.WorkspaceID)
		if _, err = q.InitializeKnowledgeTarget(ctx, db.InitializeKnowledgeTargetParams{TargetKind: target.Kind, TargetID: key.TargetID, WorkspaceID: ws}); err != nil {
			return db.CollaborationKnowledgeTarget{}, err
		}
		if _, err = q.SetKnowledgeLockTimeout(ctx, previous); err != nil {
			return db.CollaborationKnowledgeTarget{}, err
		}
	} else if err != nil {
		return db.CollaborationKnowledgeTarget{}, err
	}
	var row db.CollaborationKnowledgeTarget
	if read {
		row, err = q.LockKnowledgeTargetForRead(ctx, db.LockKnowledgeTargetForReadParams(key))
	} else {
		row, err = q.LockKnowledgeTargetForWrite(ctx, db.LockKnowledgeTargetForWriteParams(key))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return row, knowledgeFailure(503, "knowledge_unavailable")
	}
	return row, err
}

func (s *CollaborationService) createKnowledgeCandidate(ctx context.Context, w *knowledgeWrite, p protocol.CollaborationKnowledgeProposal) (protocol.CollaborationKnowledgeReceipt, error) {
	if err := protocol.ValidateCollaborationKnowledgeProposalAt(p, util.UUIDToString(w.issue.ID), s.knowledgeNow().UTC()); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeProtocolError(err)
	}
	available, err := s.KnowledgeSourceAvailability(ctx, w.q, w.issue.ID, p.Sources)
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, err
	}
	if !available {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(409, "knowledge_source_unavailable")
	}
	var prior, derived pgtype.UUID
	var expected int64
	if p.SupersedesCandidateID != nil {
		prior, _ = util.ParseUUID(*p.SupersedesCandidateID)
		row, err := w.q.GetKnowledgeCandidate(ctx, prior)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		if row.SourceIssueID != w.issue.ID || knowledgeTarget(row) != p.Target {
			return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(409, "knowledge_state_conflict")
		}
		if _, err = w.q.LockKnowledgeCandidates(ctx, []pgtype.UUID{row.LineageID, row.ID}); err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		expected = row.Version
	}
	if p.DerivedFromCandidateID != nil {
		derived, _ = util.ParseUUID(*p.DerivedFromCandidateID)
		row, err := w.q.GetKnowledgeCandidate(ctx, derived)
		if err != nil || row.SourceIssueID != w.issue.ID || row.State != "approved" {
			return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "knowledge_candidate_not_found")
		}
		refs, err := knowledgeCandidateSources(row)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		available, err := s.KnowledgeSourceAvailability(ctx, w.q, row.SourceIssueID, refs)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		if !available {
			return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(409, "knowledge_source_unavailable")
		}
	}
	sources, _ := json.Marshal(p.Sources)
	evidence, _ := json.Marshal(p.Evidence)
	applicability, _ := json.Marshal(p.Applicability)
	ws, _ := util.ParseUUID(p.Target.WorkspaceID)
	project, _ := util.ParseUUID(p.Target.ProjectID)
	id, _ := util.ParseUUID(p.CandidateID)
	expires := pgtype.Timestamptz{}
	if p.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: *p.ExpiresAt, Valid: true}
	}
	row, err := w.q.CreateKnowledgeCandidate(ctx, db.CreateKnowledgeCandidateParams{ID: id, SourceWorkspaceID: w.issue.WorkspaceID, SourceIssueID: w.issue.ID, TargetKind: p.Target.Kind, TargetWorkspaceID: ws, TargetProjectID: project, Content: p.Content, Summary: p.Summary, ContentSha256: protocol.CollaborationKnowledgeHash(p.Content), Sources: sources, Evidence: evidence, Applicability: applicability, ProposedByUserID: w.actor, ExpiresAt: expires, SupersedesCandidateID: prior, DerivedFromCandidateID: derived, ExpectedPreviousVersion: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(409, "knowledge_state_conflict")
	}
	if knowledgeExpiryCheckViolation(err) {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, err
	}
	w.candidate = row
	if err = w.recordKnowledgeChange(ctx, "propose"); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, err
	}
	return knowledgeReceipt(row, p.RequestID, "propose", "knowledge_proposed", nil), nil
}

// Migration 747 declares CHECK (expires_at IS NULL OR expires_at > proposed_at)
// without a name; PostgreSQL generated this one. The L2 service test pins the
// name to that definition.
const knowledgeExpiryCheckConstraint = "collaboration_knowledge_candidate_check5"

// The proposal validates expires_at on the application clock, but PostgreSQL
// fills proposed_at with clock_timestamp(). Only this CHECK is a request error;
// any other 23514 is a server defect and stays knowledge_unavailable.
func knowledgeExpiryCheckViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23514" && pg.TableName == "collaboration_knowledge_candidate" && pg.ConstraintName == knowledgeExpiryCheckConstraint
}

func (s *CollaborationService) changeKnowledgeCandidate(ctx context.Context, w *knowledgeWrite, action string, payload any, approval *protocol.CollaborationKnowledgeApproveRequest) (protocol.CollaborationKnowledgeReceipt, error) {
	request := w.operation.RequestID
	refuse := func(code string) (protocol.CollaborationKnowledgeReceipt, error) {
		return knowledgeReceipt(w.candidate, request, action, code, w.counter), nil
	}
	var revision int64
	var reason string
	if approval != nil {
		revision, _ = strconv.ParseInt(approval.ExpectedStateRevision, 10, 64)
	} else {
		p := payload.(protocol.CollaborationKnowledgeDecisionRequest)
		revision, _ = strconv.ParseInt(p.ExpectedStateRevision, 10, 64)
		reason = p.Reason
	}
	if revision != w.candidate.StateRevision {
		return refuse("knowledge_revision_conflict")
	}
	if action == "approve" || action == "reject" {
		if w.candidate.State != "pending" {
			return refuse("knowledge_state_conflict")
		}
	} else if w.candidate.State != "pending" && w.candidate.State != "approved" {
		return refuse("knowledge_state_conflict")
	}
	if approval != nil {
		if w.approved.ID.Valid && w.approved.Version >= w.candidate.Version {
			return refuse("knowledge_state_conflict")
		}
		version, _ := strconv.ParseInt(approval.ExpectedVersion, 10, 64)
		if version != w.candidate.Version || approval.ExpectedContentSHA256 != w.candidate.ContentSha256 {
			return refuse("knowledge_content_changed")
		}
		// ApproveKnowledgeCandidate compares expires_at with clock_timestamp().
		// Decide expiry on that clock before any target or correction write.
		if w.candidate.ExpiresAt.Valid {
			checked, err := w.q.GetKnowledgeDatabaseTime(ctx)
			if err != nil {
				return protocol.CollaborationKnowledgeReceipt{}, err
			}
			if !w.candidate.ExpiresAt.Time.After(checked.Time) {
				return refuse("knowledge_expired")
			}
		}
		refs, err := knowledgeCandidateSources(w.candidate)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		available, err := s.KnowledgeSourceAvailability(ctx, w.q, w.candidate.SourceIssueID, refs)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		if !available {
			return refuse("knowledge_source_unavailable")
		}
		rows, err := w.q.ListKnowledgeActiveTargetCandidates(ctx, db.ListKnowledgeActiveTargetCandidatesParams{TargetKind: w.target.Kind, TargetWorkspaceID: w.candidate.TargetWorkspaceID, TargetProjectID: w.candidate.TargetProjectID, CheckedAt: pgtype.Timestamptz{Time: s.knowledgeNow().UTC(), Valid: true}})
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		items := make([]protocol.CollaborationKnowledgeContribution, 0, len(rows)+1)
		for _, row := range rows {
			if row.LineageID == w.candidate.LineageID {
				continue
			}
			item, err := knowledgeContribution(row)
			if err != nil {
				return protocol.CollaborationKnowledgeReceipt{}, err
			}
			items = append(items, item)
		}
		item, err := knowledgeContribution(w.candidate)
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		items = append(items, item)
		if err = protocol.ValidateCollaborationKnowledgeTargetBudget(items); err != nil {
			return refuse("knowledge_target_budget_exceeded")
		}
	}
	collectionChange := action == "approve" || action == "withdraw" && w.candidate.State == "approved"
	if collectionChange && w.counter != nil {
		counter, err := w.q.AdvanceKnowledgeTarget(ctx, db.AdvanceKnowledgeTargetParams{TargetKind: w.counter.TargetKind, TargetID: w.counter.TargetID, KnowledgeRevision: w.counter.KnowledgeRevision})
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
		w.counter = &counter
	}
	var row db.CollaborationKnowledgeCandidate
	var err error
	code := "knowledge_" + action + "ed"
	switch action {
	case "approve":
		if w.approved.ID.Valid {
			if _, err = w.q.SupersedeKnowledgeCandidate(ctx, db.SupersedeKnowledgeCandidateParams{ID: w.approved.ID, ActorID: w.actor, ExpectedStateRevision: w.approved.StateRevision}); err != nil {
				return protocol.CollaborationKnowledgeReceipt{}, err
			}
		}
		row, err = w.q.ApproveKnowledgeCandidate(ctx, db.ApproveKnowledgeCandidateParams{ID: w.candidate.ID, ActorID: w.actor, TargetRevision: pgtype.Int8{Int64: w.counter.KnowledgeRevision, Valid: true}, ExpectedStateRevision: revision, ExpectedVersion: w.candidate.Version, ExpectedContentSha256: w.candidate.ContentSha256})
		if errors.Is(err, pgx.ErrNoRows) && w.candidate.ExpiresAt.Valid {
			checked, clockErr := w.q.GetKnowledgeDatabaseTime(ctx)
			if clockErr != nil {
				return protocol.CollaborationKnowledgeReceipt{}, clockErr
			}
			if !w.candidate.ExpiresAt.Time.After(checked.Time) {
				// Roll back target/correction writes only when the database
				// clock proves expiry. Other missing rows retain their404.
				return protocol.CollaborationKnowledgeReceipt{}, &pgconn.PgError{Code: "40001", Message: "knowledge candidate expired during approval"}
			}
		}
		code = "knowledge_approved"
	case "reject":
		row, err = w.q.RejectKnowledgeCandidate(ctx, db.RejectKnowledgeCandidateParams{ID: w.candidate.ID, ActorID: w.actor, Reason: pgtype.Text{String: reason, Valid: true}, ExpectedStateRevision: revision})
		code = "knowledge_rejected"
	case "withdraw":
		row, err = w.q.WithdrawKnowledgeCandidate(ctx, db.WithdrawKnowledgeCandidateParams{ID: w.candidate.ID, ActorID: w.actor, Reason: pgtype.Text{String: reason, Valid: true}, ExpectedStateRevision: revision})
		code = "knowledge_withdrawn"
	}
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, err
	}
	w.candidate = row
	if collectionChange && w.counter != nil {
		_, err = w.q.EnqueueKnowledgeTargetHint(ctx, db.EnqueueKnowledgeTargetHintParams{ID: util.MustParseUUID(uuid.NewString()), CauseID: w.operation.ID, TargetKind: pgtype.Text{String: w.counter.TargetKind, Valid: true}, TargetID: w.counter.TargetID, TargetWorkspaceID: w.counter.WorkspaceID, KnowledgeRevision: pgtype.Int8{Int64: w.counter.KnowledgeRevision, Valid: true}})
		if err != nil {
			return protocol.CollaborationKnowledgeReceipt{}, err
		}
	}
	if err = w.recordKnowledgeChange(ctx, action); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, err
	}
	return knowledgeReceipt(row, request, action, code, w.counter), nil
}

func (w *knowledgeWrite) recordKnowledgeChange(ctx context.Context, action string) error {
	if !w.issue.ID.Valid {
		return nil
	}
	seq, err := w.q.AdvanceCollaborationEventSequence(ctx, w.issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CandidateID   string `json:"candidate_id"`
		Action        string `json:"action"`
		Version       string `json:"version"`
		StateRevision string `json:"state_revision"`
	}{util.UUIDToString(w.candidate.ID), action, strconv.FormatInt(w.candidate.Version, 10), strconv.FormatInt(w.candidate.StateRevision, 10)})
	if err != nil {
		return err
	}
	_, err = w.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: w.issue.ID, EventSeq: seq, ActorUserID: w.actor, Kind: "knowledge_changed", Payload: payload})
	return err
}

func decodeKnowledgeReceipt(op db.CollaborationOperation, candidateID pgtype.UUID, action, request string) (protocol.CollaborationKnowledgeReceipt, error) {
	var r protocol.CollaborationKnowledgeReceipt
	if !op.CompletedAt.Valid || op.Kind != "knowledge_"+action || op.RequestID != request || json.Unmarshal(op.Result, &r) != nil || r.Action != action || r.RequestID != request || r.CandidateID != util.UUIDToString(candidateID) || r.Code != op.ResultCode.String {
		return r, knowledgeFailure(503, "knowledge_unavailable")
	}
	return r, nil
}

func (s *CollaborationService) GetKnowledgeOperation(ctx context.Context, candidateID pgtype.UUID, request string) (protocol.CollaborationKnowledgeReceipt, error) {
	viewer, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgePublicError(err)
	}
	if _, err = knowledgePAT(ctx, s.Queries, viewer, false); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgePublicError(err)
	}
	if _, err = util.ParseUUID(request); err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(400, "invalid_knowledge_request")
	}
	row, err := s.Queries.GetKnowledgeCandidateRouting(ctx, candidateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "operation_not_found")
	}
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgePublicError(err)
	}
	op, err := s.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: row.SourceIssueID, ActorUserID: viewer, RequestID: request})
	if errors.Is(err, pgx.ErrNoRows) {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "operation_not_found")
	}
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgePublicError(err)
	}
	action := strings.TrimPrefix(op.Kind, "knowledge_")
	switch action {
	case "propose", "approve", "reject", "withdraw":
	default:
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "operation_not_found")
	}
	r, err := decodeKnowledgeReceipt(op, candidateID, action, request)
	if err != nil {
		return protocol.CollaborationKnowledgeReceipt{}, knowledgeFailure(404, "operation_not_found")
	}
	return r, nil
}
