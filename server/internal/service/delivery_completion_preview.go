package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const deliveryCandidateScanLimit = 200

// DeliveryEvidenceCandidate exposes only current-authority artifact metadata.
// Selectable means a complete retained package, not permission to complete.
type DeliveryEvidenceCandidate struct {
	ApprovalID     string    `json:"approval_id"`
	ExecutionID    string    `json:"execution_id"`
	SourceRevision string    `json:"source_revision"`
	SpecDigest     string    `json:"spec_digest"`
	ArtifactDigest *string   `json:"artifact_digest"`
	ReceiptDigest  *string   `json:"receipt_digest"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Summary        string    `json:"summary"`
	Status         string    `json:"status"`
	ResultCode     *string   `json:"result_code"`
	Selectable     bool      `json:"selectable"`
}

type DeliveryEvidenceCandidates struct {
	ProtocolVersion int                         `json:"protocol_version"`
	IssueID         string                      `json:"issue_id"`
	IssueRevision   int64                       `json:"issue_revision"`
	ProjectID       *string                     `json:"project_id"`
	CheckedAt       time.Time                   `json:"checked_at"`
	Items           []DeliveryEvidenceCandidate `json:"items"`
	NextCursor      *string                     `json:"next_cursor"`
	HasMore         bool                        `json:"has_more"`
}

// DeliveryCompletionPreviewBinding is deliberately incomplete as an acceptance
// request: only an explicit human action supplies the operation, reason and expiry.
type DeliveryCompletionPreviewBinding struct {
	ContractVersion   string                     `json:"contract_version"`
	SourceWorkspaceID string                     `json:"source_workspace_id"`
	Target            DeliveryCompletionTarget   `json:"target"`
	Project           DeliveryCompletionProject  `json:"project"`
	Evidence          DeliveryCompletionEvidence `json:"evidence"`
	Consumer          DeliveryCompletionConsumer `json:"consumer"`
}

type DeliveryCompletionPreview struct {
	ProtocolVersion      int                               `json:"protocol_version"`
	IssueID              string                            `json:"issue_id"`
	IssueRevision        int64                             `json:"issue_revision"`
	CurrentStatus        string                            `json:"current_status"`
	CurrentCategory      string                            `json:"current_category"`
	TargetStatus         string                            `json:"target_status"`
	TargetCategory       string                            `json:"target_category"`
	ProjectID            *string                           `json:"project_id"`
	ProjectRevision      *int64                            `json:"project_revision"`
	PolicyMode           string                            `json:"policy_mode"`
	PolicyRevision       int64                             `json:"policy_revision"`
	CheckedAt            time.Time                         `json:"checked_at"`
	Reason               string                            `json:"reason"`
	Observation          *DeliveryObservation              `json:"observation"`
	AuthorizationBinding *DeliveryCompletionPreviewBinding `json:"authorization_binding"`
	MaxExpiresAt         *time.Time                        `json:"max_expires_at"`
	Enforced             bool                              `json:"enforced"`
	UsableForCompletion  bool                              `json:"usable_for_completion"`
	SkipSupported        bool                              `json:"skip_supported"`
}

type DeliveryCompletionPreviewRequest struct {
	ExpectedIssueRevision int64  `json:"expected_issue_revision"`
	TargetStatus          string `json:"target_status"`
	ApprovalID            string `json:"approval_id"`
}

func ParseDeliveryCompletionPreview(raw []byte) (DeliveryCompletionPreviewRequest, error) {
	var req DeliveryCompletionPreviewRequest
	if _, err := completionObject(raw, []string{"expected_issue_revision", "target_status", "approval_id"}, nil); err != nil {
		return req, err
	}
	if json.Unmarshal(raw, &req) != nil {
		return req, completionInvalid()
	}
	req.TargetStatus = strings.ToLower(strings.TrimSpace(req.TargetStatus))
	if req.ExpectedIssueRevision < 0 || !completionUUID(req.ApprovalID) || !completionStatus(req.TargetStatus) {
		return req, completionInvalid()
	}
	return req, nil
}

type deliveryCandidateCursor struct {
	IssueID       string    `json:"issue_id"`
	IssueRevision int64     `json:"issue_revision"`
	WorkspaceID   string    `json:"workspace_id"`
	ViewerID      string    `json:"viewer_id"`
	UpperBound    time.Time `json:"upper_bound"`
	CreatedAt     time.Time `json:"created_at"`
	ApprovalID    string    `json:"approval_id"`
}

func parseDeliveryCandidateCursor(raw string) (deliveryCandidateCursor, error) {
	var c deliveryCandidateCursor
	if len(raw) > 2048 {
		return c, completionInvalid()
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return c, completionInvalid()
	}
	if _, err = completionObject(data, []string{"issue_id", "issue_revision", "workspace_id", "viewer_id", "upper_bound", "created_at", "approval_id"}, nil); err != nil {
		return c, err
	}
	if json.Unmarshal(data, &c) != nil || !completionUUID(c.IssueID) || !completionUUID(c.WorkspaceID) || !completionUUID(c.ViewerID) || !completionUUID(c.ApprovalID) || c.IssueRevision < 0 || c.UpperBound.IsZero() || c.CreatedAt.IsZero() || c.CreatedAt.After(c.UpperBound) || c.UpperBound.Location() != time.UTC || c.CreatedAt.Location() != time.UTC || c.CreatedAt.Nanosecond()%1000 != 0 || c.UpperBound.Nanosecond()%1000 != 0 {
		return c, completionInvalid()
	}
	return c, nil
}

// lockDeliveryPreviewIssue retains the ordinary authority prefix for pure
// projections. SQL READ ONLY cannot be used because these are real row locks.
func lockDeliveryPreviewIssue(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue) (db.Issue, error) {
	if tx == nil {
		return db.Issue{}, completionInvalid()
	}
	kind, _, err := access.Credential.Actor()
	if err != nil || kind != "human" {
		return db.Issue{}, completionDenied()
	}
	if access.RequestedWorkspaceID != "" && access.RequestedWorkspaceID != util.UUIDToString(initial.WorkspaceID) {
		return db.Issue{}, completionDenied()
	}
	if err = PrepareArtifactTransaction(ctx, tx); err != nil {
		return db.Issue{}, err
	}
	q := db.New(tx)
	if err = LockArtifactWorkspace(ctx, q, initial.WorkspaceID); err != nil {
		return db.Issue{}, err
	}
	if _, err = LockArtifactUsers(ctx, q, initial.WorkspaceID, access.Credential.Principal.UserID); err != nil {
		return db.Issue{}, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return db.Issue{}, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, initial.WorkspaceID); err != nil {
		return db.Issue{}, err
	}
	issue, err := q.LockDeliveryEvidenceIssueRead(ctx, db.LockDeliveryEvidenceIssueReadParams{ID: initial.ID, WorkspaceID: initial.WorkspaceID})
	if err == nil {
		err = completionOrdinary(ctx, q, issue)
	}
	return issue, err
}

func ListDeliveryEvidenceCandidatesInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue, limit int32, cursor string) (out DeliveryEvidenceCandidates, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if limit < 1 || limit > 50 {
		return out, completionInvalid()
	}
	var after deliveryCandidateCursor
	if cursor != "" {
		after, err = parseDeliveryCandidateCursor(cursor)
		if err != nil {
			return out, err
		}
	}
	issue, err := lockDeliveryPreviewIssue(ctx, tx, access, initial)
	if err != nil {
		return out, err
	}
	_, policy, err := LockDeliveryProjectPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return out, err
	}
	if policy.Mode == "enforce" {
		if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
			return out, err
		}
	}
	q := db.New(tx)
	clock, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	upper := clock.Time.UTC().Truncate(time.Microsecond)
	params := db.ListDeliveryEvidenceCandidateScanParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, UpperBound: completionTime(upper), ScanLimit: deliveryCandidateScanLimit + 1}
	if cursor != "" {
		if after.IssueID != util.UUIDToString(issue.ID) || after.IssueRevision != issue.Revision || after.WorkspaceID != util.UUIDToString(issue.WorkspaceID) || after.ViewerID != access.Credential.Principal.UserID {
			return out, completionConflict()
		}
		if after.UpperBound.After(upper) {
			return out, completionInvalid()
		}
		upper = after.UpperBound
		params.UpperBound = completionTime(upper)
		params.AfterCreatedAt = completionTime(after.CreatedAt)
		params.AfterApprovalID = completionUUIDPG(after.ApprovalID)
	}
	rows, err := q.ListDeliveryEvidenceCandidateScan(ctx, params)
	if err != nil {
		return out, err
	}
	out = DeliveryEvidenceCandidates{ProtocolVersion: 1, IssueID: util.UUIDToString(issue.ID), IssueRevision: issue.Revision, Items: []DeliveryEvidenceCandidate{}}
	if issue.ProjectID.Valid {
		id := util.UUIDToString(issue.ProjectID)
		out.ProjectID = &id
	}
	for i, e := range rows {
		if i == deliveryCandidateScanLimit {
			return DeliveryEvidenceCandidates{}, DeliveryCompletionError{503, "delivery_completion_candidates_unavailable"}
		}
		live, task, approval, e2 := readDeliveryArtifactInTx(ctx, tx, access, issue, e)
		if e2 != nil {
			var denied ArtifactError
			if errors.As(e2, &denied) && denied.Status == 403 || errors.Is(e2, pgx.ErrNoRows) {
				continue
			}
			return DeliveryEvidenceCandidates{}, e2
		}
		if len(out.Items) == int(limit) {
			out.HasMore = true
			break
		}
		item := DeliveryEvidenceCandidate{ApprovalID: util.UUIDToString(live.ApprovalID), ExecutionID: util.UUIDToString(live.ExecutionID), SourceRevision: live.SourceRevision, SpecDigest: live.SpecDigest, CreatedAt: live.CreatedAt.Time.UTC(), ExpiresAt: live.ExpiresAt.Time.UTC(), Summary: approval.Summary, Status: "unclaimed"}
		if live.ResultStatus.Valid {
			item.Status = live.ResultStatus.String
		}
		if live.ResultCode != "" {
			code := live.ResultCode
			item.ResultCode = &code
		}
		if live.OutputArtifactDigest != "" {
			digest := live.OutputArtifactDigest
			item.ArtifactDigest = &digest
		}
		if live.ReceiptDigest != "" {
			digest := live.ReceiptDigest
			item.ReceiptDigest = &digest
		}
		item.Selectable = approval.Status == "executed" && task.Status != "failed" && task.Status != "cancelled" && deliveryReceiptValid(live) && clock.Time.Before(live.ExpiresAt.Time)
		out.Items = append(out.Items, item)
	}
	if out.HasMore {
		last := out.Items[len(out.Items)-1]
		raw, e := json.Marshal(deliveryCandidateCursor{IssueID: out.IssueID, IssueRevision: out.IssueRevision, WorkspaceID: util.UUIDToString(issue.WorkspaceID), ViewerID: access.Credential.Principal.UserID, UpperBound: upper, CreatedAt: last.CreatedAt, ApprovalID: last.ApprovalID})
		if e != nil {
			return DeliveryEvidenceCandidates{}, e
		}
		next := base64.RawURLEncoding.EncodeToString(raw)
		out.NextCursor = &next
	}
	// The final clock/credential fence covers time spent enumerating candidates.
	clock, err = q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return DeliveryEvidenceCandidates{}, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return DeliveryEvidenceCandidates{}, err
	}
	out.CheckedAt = clock.Time.UTC()
	for i := range out.Items {
		out.Items[i].Selectable = out.Items[i].Selectable && out.CheckedAt.Before(out.Items[i].ExpiresAt)
	}
	return out, nil
}

func GetDeliveryCompletionPreviewInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, initial db.Issue, req DeliveryCompletionPreviewRequest) (out DeliveryCompletionPreview, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if req.ExpectedIssueRevision < 0 || !completionUUID(req.ApprovalID) || !completionStatus(req.TargetStatus) {
		return out, completionInvalid()
	}
	issue, err := lockDeliveryPreviewIssue(ctx, tx, access, initial)
	if err != nil {
		return out, err
	}
	if issue.Revision != req.ExpectedIssueRevision {
		return out, completionConflict()
	}
	q := db.New(tx)
	target, err := issuestatus.Resolve(ctx, q, issue.WorkspaceID, req.TargetStatus)
	if errors.Is(err, issuestatus.ErrUnknownStatus) {
		return out, completionInvalid()
	}
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
	out = DeliveryCompletionPreview{ProtocolVersion: 1, IssueID: util.UUIDToString(issue.ID), IssueRevision: issue.Revision, CurrentStatus: issue.Status, CurrentCategory: current, TargetStatus: target.Key, TargetCategory: target.Category, PolicyMode: policy.Mode, PolicyRevision: policy.Revision}
	if issue.ProjectID.Valid {
		id := util.UUIDToString(project.ID)
		out.ProjectID = &id
		revision := project.Revision
		out.ProjectRevision = &revision
	}
	switch {
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
		e, e2 := q.GetAgentArtifactExecutionByApproval(ctx, completionUUIDPG(req.ApprovalID))
		if e2 != nil {
			return out, e2
		}
		if e.WorkspaceID != issue.WorkspaceID || e.ResourceID != issue.ID {
			return out, pgx.ErrNoRows
		}
		observation, e2 := EvaluateDeliveryEvidenceInTx(ctx, tx, access, issue, DeliveryEvidenceRequest{PolicyVersion: DeliveryEvidencePolicy, IssueRevision: issue.Revision, ProjectID: out.ProjectID, ProjectRevision: out.ProjectRevision, ApprovalID: req.ApprovalID, SourceRevision: e.SourceRevision, SpecDigest: e.SpecDigest, ArtifactDigest: e.OutputArtifactDigest})
		if e2 != nil {
			return out, e2
		}
		out.Observation = &observation
		out.Reason = observation.Reason
		if observation.Verdict == "pass" && observation.ValidUntil != nil {
			// Reuse issuer validation without minting a grant or exposing its snapshot.
			issuer := db.CreateDeliveryCompletionAuthorizationParams{}
			if err = completionFreezeIssuer(ctx, tx, access.Credential, &issuer); err != nil {
				return out, err
			}
			clock, e2 := q.ArtifactExecutionDatabaseTime(ctx)
			if e2 != nil {
				return out, e2
			}
			out.CheckedAt = clock.Time.UTC().Truncate(time.Microsecond)
			expiry := out.CheckedAt.Add(24 * time.Hour)
			if observation.ValidUntil.Before(expiry) {
				expiry = *observation.ValidUntil
			}
			if issuer.IssuerCredentialExpiresAt.Valid && issuer.IssuerCredentialExpiresAt.Time.Before(expiry) {
				expiry = issuer.IssuerCredentialExpiresAt.Time
			}
			expiry = expiry.UTC().Truncate(time.Microsecond)
			if !expiry.After(out.CheckedAt) {
				out.Reason = "evidence_expired"
				return out, nil
			}
			out.MaxExpiresAt = &expiry
			out.AuthorizationBinding = &DeliveryCompletionPreviewBinding{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: util.UUIDToString(issue.WorkspaceID), Target: DeliveryCompletionTarget{Kind: "ordinary", IssueID: out.IssueID, ExpectedIssueRevision: issue.Revision, ExpectedStatus: issue.Status, TargetStatus: target.Key}, Project: DeliveryCompletionProject{ID: util.UUIDToString(project.ID), ExpectedRevision: project.Revision, ExpectedPolicyRevision: policy.Revision}, Evidence: DeliveryCompletionEvidence{IssueID: out.IssueID, ExpectedIssueRevision: issue.Revision, ApprovalID: req.ApprovalID, ExpectedSourceRevision: observation.SourceRevision, ExpectedSpecDigest: observation.SpecDigest, ExpectedArtifactDigest: observation.ArtifactDigest}, Consumer: DeliveryCompletionConsumer{Kind: "human", UserID: access.Credential.Principal.UserID}}
			out.Reason = "authorization_available"
		}
	}
	if out.CheckedAt.IsZero() {
		clock, e2 := q.ArtifactExecutionDatabaseTime(ctx)
		if e2 != nil {
			return out, e2
		}
		out.CheckedAt = clock.Time.UTC()
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return out, err
	}
	return out, nil
}
