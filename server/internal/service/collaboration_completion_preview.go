package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationCompletionPreviewRequest struct {
	SourceIssueID string                   `json:"source_issue_id"`
	ApprovalID    string                   `json:"approval_id"`
	Target        DeliveryCompletionTarget `json:"target"`
}

func ParseCollaborationCompletionPreview(raw []byte) (CollaborationCompletionPreviewRequest, error) {
	var r CollaborationCompletionPreviewRequest
	if _, err := completionObject(raw, []string{"source_issue_id", "approval_id", "target"}, nil); err != nil {
		return r, err
	}
	if json.Unmarshal(raw, &r) != nil || !completionUUID(r.SourceIssueID) || !completionUUID(r.ApprovalID) || !completionIsCollaboration(r.Target.Kind) || r.Target.Validate() != nil {
		return r, completionInvalid()
	}
	return r, nil
}

type CollaborationCompletionTargetContext struct {
	ExpectedLifecycleEpoch string  `json:"expected_lifecycle_epoch"`
	SubmissionID           *string `json:"submission_id"`
}
type CollaborationDeliveryCandidates struct {
	DeliveryEvidenceCandidates
	SourceIssueID       string                                `json:"source_issue_id"`
	SourceIssueRevision int64                                 `json:"source_issue_revision"`
	TargetContext       *CollaborationCompletionTargetContext `json:"target_context,omitempty"`
}
type CollaborationCompletionPreview struct {
	DeliveryCompletionPreview
	SourceIssueID          string                      `json:"source_issue_id"`
	SourceIssueRevision    int64                       `json:"source_issue_revision"`
	StageBlockers          []CollaborationStageBlocker `json:"stage_blockers"`
	StageBlockersTruncated bool                        `json:"stage_blockers_truncated"`
}
type collaborationCompletionCursor struct {
	Root           string `json:"root"`
	RootRevision   int64  `json:"root_revision"`
	Source         string `json:"source"`
	SourceRevision int64  `json:"source_revision"`
	Viewer         string `json:"viewer"`
	Cursor         string `json:"cursor"`
}

func (s *CollaborationService) lockCompletionPreviewRoot(ctx context.Context, tx pgx.Tx, root, source, org pgtype.UUID, credential ArtifactCredentialSnapshot) (CollaborationIssueAccess, db.Issue, error) {
	q := db.New(tx)
	var access CollaborationIssueAccess
	kind, _, err := credential.Actor()
	if err != nil || kind != "human" {
		return access, db.Issue{}, completionDenied()
	}
	if err = PrepareArtifactTransaction(ctx, tx); err != nil {
		return access, db.Issue{}, err
	}
	issue, err := q.GetIssue(ctx, root)
	if err != nil {
		return access, db.Issue{}, err
	}
	if org.Valid {
		access, err = completionAdminNomination(ctx, tx, issue, completionUUIDPG(credential.Principal.UserID), org, true)
	} else {
		access, err = s.ResolveIssueAccess(ctx, q, root)
	}
	if err != nil {
		return access, db.Issue{}, err
	}
	if err = LockArtifactWorkspace(ctx, q, issue.WorkspaceID); err != nil {
		return access, db.Issue{}, err
	}
	if _, err = LockArtifactUsers(ctx, q, issue.WorkspaceID, credential.Principal.UserID); err != nil {
		return access, db.Issue{}, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, credential); err != nil {
		return access, db.Issue{}, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, issue.WorkspaceID); err != nil {
		return access, db.Issue{}, err
	}
	issue, err = q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: root, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return access, db.Issue{}, err
	}
	if _, err = q.LockIssueCollaboration(ctx, root); err != nil {
		return access, db.Issue{}, err
	}
	if org.Valid {
		access, err = completionAdminNomination(ctx, tx, issue, completionUUIDPG(credential.Principal.UserID), org, true)
	} else {
		access, err = s.ResolveIssueAccess(ctx, q, root)
	}
	if err != nil {
		return access, db.Issue{}, err
	}
	child, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: source, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return access, child, err
	}
	if child.ParentIssueID != root || child.ID == root || child.ProjectID != issue.ProjectID {
		return access, child, pgx.ErrNoRows
	}
	if err = completionOrdinary(ctx, q, child); err != nil {
		return access, child, pgx.ErrNoRows
	}
	return access, child, nil
}

func (s *CollaborationService) ListCollaborationDeliveryCandidatesInTx(ctx context.Context, tx pgx.Tx, root, source, org pgtype.UUID, credential ArtifactCredentialSnapshot, limit int32, cursor string) (out CollaborationDeliveryCandidates, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	access, child, err := s.lockCompletionPreviewRoot(ctx, tx, root, source, org, credential)
	if err != nil {
		return out, err
	}
	inner := ""
	if cursor != "" {
		var saved collaborationCompletionCursor
		raw, e := base64.RawURLEncoding.Strict().DecodeString(cursor)
		if len(cursor) > 2048 || e != nil {
			return out, completionInvalid()
		}
		if _, e = completionObject(raw, []string{"root", "root_revision", "source", "source_revision", "viewer", "cursor"}, nil); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &saved) != nil {
			return out, completionInvalid()
		}
		if saved.Root != util.UUIDToString(root) || saved.RootRevision != access.Issue.Revision || saved.Source != util.UUIDToString(source) || saved.SourceRevision != child.Revision || saved.Viewer != credential.Principal.UserID {
			return out, completionConflict()
		}
		inner = saved.Cursor
	}
	page, err := ListDeliveryEvidenceCandidatesInTx(ctx, tx, ArtifactAccess{Credential: credential, RequestedWorkspaceID: util.UUIDToString(child.WorkspaceID)}, child, limit, inner)
	if err != nil {
		return out, err
	}
	out = CollaborationDeliveryCandidates{DeliveryEvidenceCandidates: page, SourceIssueID: util.UUIDToString(source), SourceIssueRevision: child.Revision}
	out.IssueID = util.UUIDToString(root)
	out.IssueRevision = access.Issue.Revision
	if page.NextCursor != nil {
		raw, err := json.Marshal(collaborationCompletionCursor{Root: out.IssueID, RootRevision: out.IssueRevision, Source: out.SourceIssueID, SourceRevision: child.Revision, Viewer: credential.Principal.UserID, Cursor: *page.NextCursor})
		if err != nil {
			return out, err
		}
		value := base64.RawURLEncoding.EncodeToString(raw)
		out.NextCursor = &value
	}
	if org.Valid {
		summary, err := collaborationDeliverySummary(ctx, db.New(tx), access)
		if err != nil {
			return out, err
		}
		out.TargetContext = &CollaborationCompletionTargetContext{ExpectedLifecycleEpoch: strconv.FormatInt(access.LifecycleEpoch, 10)}
		if summary.Current != nil {
			id := util.UUIDToString(summary.Current.ID)
			out.TargetContext.SubmissionID = &id
		}
	}
	return out, nil
}

func (s *CollaborationService) GetCollaborationCompletionPreviewInTx(ctx context.Context, tx pgx.Tx, root, org pgtype.UUID, credential ArtifactCredentialSnapshot, request CollaborationCompletionPreviewRequest) (out CollaborationCompletionPreview, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if request.Target.Validate() != nil || request.Target.IssueID != util.UUIDToString(root) || !completionUUID(request.SourceIssueID) || !completionUUID(request.ApprovalID) || (request.Target.Kind == "collaboration_admin") != org.Valid {
		return out, completionInvalid()
	}
	access, child, err := s.lockCompletionPreviewRoot(ctx, tx, root, completionUUIDPG(request.SourceIssueID), org, credential)
	if err != nil {
		return out, err
	}
	issue := access.Issue
	if issue.Revision != request.Target.ExpectedIssueRevision || issue.Status != request.Target.ExpectedStatus {
		return out, completionConflict()
	}
	q := db.New(tx)
	target, err := issuestatus.Resolve(ctx, q, issue.WorkspaceID, request.Target.TargetStatus)
	if err != nil {
		return out, err
	}
	current, err := IssueStatusCategory(ctx, q, issue)
	if err != nil {
		return out, err
	}
	project, policy, err := LockDeliveryProjectPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return out, err
	}
	consumer := DeliveryCompletionConsumer{Kind: "human", UserID: util.UUIDToString(issue.CreatorID)}
	if org.Valid {
		consumer = DeliveryCompletionConsumer{Kind: "admin", UserID: credential.Principal.UserID, OrganizationID: util.UUIDToString(org)}
	}
	if _, _, _, err = completionCollaborationIntent(ctx, tx, issue, request.Target, consumer, true); err != nil {
		return out, err
	}
	out = CollaborationCompletionPreview{DeliveryCompletionPreview: DeliveryCompletionPreview{ProtocolVersion: 1, IssueID: util.UUIDToString(root), IssueRevision: issue.Revision, CurrentStatus: issue.Status, CurrentCategory: current, TargetStatus: target.Key, TargetCategory: target.Category, PolicyMode: policy.Mode, PolicyRevision: policy.Revision}, SourceIssueID: request.SourceIssueID, SourceIssueRevision: child.Revision, StageBlockers: []CollaborationStageBlocker{}}
	if issue.ProjectID.Valid {
		id := util.UUIDToString(project.ID)
		out.ProjectID = &id
		revision := project.Revision
		out.ProjectRevision = &revision
	}
	// Authorize the chosen source even when policy/stage would otherwise return
	// a non-authorizing branch. Knowing a private approval ID grants no read.
	execution, err := q.GetAgentArtifactExecutionByApproval(ctx, completionUUIDPG(request.ApprovalID))
	if err != nil {
		return out, err
	}
	if execution.ResourceID != child.ID || execution.WorkspaceID != child.WorkspaceID {
		return out, pgx.ErrNoRows
	}
	artifactAccess := ArtifactAccess{Credential: credential, RequestedWorkspaceID: util.UUIDToString(issue.WorkspaceID)}
	if _, _, _, err = lockDeliveryArtifactInTx(ctx, tx, artifactAccess, child, execution); err != nil {
		return out, err
	}
	facts, err := stageCompletionFacts(ctx, tx, s.StageExecutionSources, access)
	var blocked *StageCompletionError
	if errors.As(err, &blocked) {
		out.StageBlockers = blocked.Blockers
		out.Reason = "stage_incomplete"
		err = nil
	} else if err != nil {
		return out, err
	}
	if len(out.StageBlockers) == 0 && facts.state != nil && request.Target.SubmissionID != nil {
		g := &stageMutationGuard{tx: tx, before: issue, access: access, evidence: facts}
		if e := g.checkSubmission(ctx, completionUUIDPG(*request.Target.SubmissionID)); errors.As(e, &blocked) {
			out.StageBlockers = blocked.Blockers
			out.Reason = "stage_incomplete"
		} else if e != nil {
			return out, e
		}
	}
	switch {
	case len(out.StageBlockers) > 0:
	case target.Category != "done":
		out.Reason = "target_not_done"
	case current == "done":
		out.Reason = "already_done"
	case !issue.ProjectID.Valid:
		out.Reason = "no_project"
	case policy.Mode == "off":
		out.Reason = "policy_off"
	case policy.Mode == "observe":
		out.Reason = "policy_observe"
	default:
		if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
			return out, err
		}
		observation, e := EvaluateDeliveryEvidenceInTx(ctx, tx, artifactAccess, child, DeliveryEvidenceRequest{PolicyVersion: DeliveryEvidencePolicy, IssueRevision: child.Revision, ProjectID: out.ProjectID, ProjectRevision: out.ProjectRevision, ApprovalID: request.ApprovalID, SourceRevision: execution.SourceRevision, SpecDigest: execution.SpecDigest, ArtifactDigest: execution.OutputArtifactDigest})
		if e != nil {
			return out, e
		}
		out.Observation = &observation
		out.Reason = observation.Reason
		if observation.Verdict == "pass" && observation.ValidUntil != nil {
			issuer := db.CreateDeliveryCompletionAuthorizationParams{}
			if err = completionFreezeIssuer(ctx, tx, credential, &issuer); err != nil {
				return out, err
			}
			clock, e := q.ArtifactExecutionDatabaseTime(ctx)
			if e != nil {
				return out, e
			}
			expiry := stageEarlier(clock.Time.Add(24*time.Hour), *observation.ValidUntil)
			if issuer.IssuerCredentialExpiresAt.Valid {
				expiry = stageEarlier(expiry, issuer.IssuerCredentialExpiresAt.Time)
			}
			if facts.state != nil {
				expiry = stageEarlier(expiry, facts.expires)
			}
			evidence := DeliveryCompletionEvidence{IssueID: request.SourceIssueID, ExpectedIssueRevision: child.Revision, ApprovalID: request.ApprovalID, ExpectedSourceRevision: observation.SourceRevision, ExpectedSpecDigest: observation.SpecDigest, ExpectedArtifactDigest: observation.ArtifactDigest}
			if facts.state != nil && !stageCompletionMatchesM1(facts, evidence) {
				out.Reason = "stage_incomplete"
				out.StageBlockers = []CollaborationStageBlocker{{Stage: "verification", Code: "stage_evidence_changed"}}
			} else if !expiry.After(clock.Time) {
				out.Reason = "evidence_expired"
			} else {
				out.MaxExpiresAt = &expiry
				out.AuthorizationBinding = &DeliveryCompletionPreviewBinding{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: util.UUIDToString(issue.WorkspaceID), Target: request.Target, Project: DeliveryCompletionProject{ID: util.UUIDToString(project.ID), ExpectedRevision: project.Revision, ExpectedPolicyRevision: policy.Revision}, Evidence: evidence, Consumer: consumer}
				out.Reason = "authorization_available"
			}
		}
	}
	clock, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	out.CheckedAt = clock.Time.UTC()
	if err = CheckArtifactCredentialInTx(ctx, tx, credential); err != nil {
		return out, err
	}
	return out, nil
}

func stageCompletionMatchesM1(facts stageCompletionEvidence, evidence DeliveryCompletionEvidence) bool {
	content, err := decodeStageCode(facts.candidate)
	if err != nil || content.Baseline.Artifact == nil {
		return false
	}
	return content.Baseline.ChildIssueID == evidence.IssueID && content.Baseline.Artifact.ApprovalID == evidence.ApprovalID && content.Baseline.Artifact.ArtifactDigest == evidence.ExpectedArtifactDigest && content.Baseline.PullRequest.HeadSHA == evidence.ExpectedSourceRevision
}
