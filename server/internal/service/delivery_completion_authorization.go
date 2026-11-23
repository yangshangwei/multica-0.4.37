package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// DeliveryCompletionAuthorizationView intentionally excludes source evidence,
// observations, credential metadata and issuer request contents.
type DeliveryCompletionAuthorizationView struct {
	AuthorizationID   string                     `json:"authorization_id"`
	OperationID       string                     `json:"operation_id"`
	IssueID           string                     `json:"issue_id"`
	IssueRevision     int64                      `json:"issue_revision"`
	TargetStatus      string                     `json:"target_status"`
	ContractVersion   string                     `json:"contract_version"`
	CreatedAt         time.Time                  `json:"created_at"`
	ExpiresAt         time.Time                  `json:"expires_at"`
	State             string                     `json:"state"`
	Applicability     string                     `json:"applicability"`
	CompletionReceipt *DeliveryCompletionReceipt `json:"completion_receipt,omitempty"`
	Replayed          bool                       `json:"replayed"`
}

// CreateDeliveryCompletionAuthorizationInTx borrows a human request transaction.
// Its returned fact is not durable until the transport owner commits that tx.
func CreateDeliveryCompletionAuthorizationInTx(ctx context.Context, tx pgx.Tx, issuer ArtifactAccess, key pgtype.UUID, req DeliveryCompletionAuthorizationRequest) (out DeliveryCompletionAuthorizationView, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if tx == nil || !key.Valid || !completionUUID(util.UUIDToString(key)) || req.Validate() != nil {
		return out, completionInvalid()
	}
	if req.Target.Kind == "collaboration_admin" && !issuer.CollaborationAdministrationEnabled {
		return out, completionDenied()
	}
	kind, _, err := issuer.Credential.Actor()
	if err != nil || kind != "human" {
		return out, completionDenied()
	}
	if err = PrepareArtifactTransaction(ctx, tx); err != nil {
		return out, err
	}
	if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
		return out, err
	}
	q := db.New(tx)
	workspace := completionUUIDPG(req.SourceWorkspaceID)
	if issuer.RequestedWorkspaceID != "" && issuer.RequestedWorkspaceID != req.SourceWorkspaceID {
		return out, completionDenied()
	}
	if err = LockArtifactWorkspace(ctx, q, workspace); err != nil {
		return out, err
	}
	if _, err = LockArtifactUsers(ctx, q, workspace, issuer.Credential.Principal.UserID); err != nil {
		return out, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, issuer.Credential); err != nil {
		return out, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, workspace); err != nil {
		return out, err
	}
	issue, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: completionUUIDPG(req.Target.IssueID), WorkspaceID: workspace})
	if err != nil {
		return out, err
	}
	row, err := q.GetDeliveryCompletionByRequest(ctx, db.GetDeliveryCompletionByRequestParams{WorkspaceID: workspace, IssuerUserID: completionUUIDPG(issuer.Credential.Principal.UserID), RequestKey: key})
	if err == nil {
		if row.RequestDigest != req.Digest() || row.IssueID != issue.ID {
			return out, completionConflict()
		}
		if _, err = completionStoredRequest(row); err != nil {
			return out, err
		}
		if err = completionIssuerRead(ctx, tx, issuer, row, issue); err != nil {
			return out, err
		}
		out, err = completionView(ctx, tx, row)
		out.Replayed = true
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if req.Target.Kind == "ordinary" {
		if err = completionOrdinary(ctx, q, issue); err != nil {
			return out, err
		}
	} else if !completionIsCollaboration(req.Target.Kind) {
		return out, completionUnsupported()
	}
	if issue.Revision != req.Target.ExpectedIssueRevision || issue.Status != req.Target.ExpectedStatus {
		return out, completionConflict()
	}
	currentCategory, err := IssueStatusCategory(ctx, q, issue)
	if err != nil {
		return out, err
	}
	target, err := issuestatus.Resolve(ctx, q, workspace, req.Target.TargetStatus)
	if err != nil {
		return out, err
	}
	if currentCategory == "done" || target.Category != "done" || target.Key != req.Target.TargetStatus {
		return out, completionConflict()
	}
	project, policy, err := LockDeliveryProjectPolicy(ctx, tx, workspace, issue.ProjectID)
	if err != nil {
		return out, err
	}
	if !issue.ProjectID.Valid || util.UUIDToString(project.ID) != req.Project.ID || project.Revision != req.Project.ExpectedRevision || policy.Mode != "enforce" || policy.Revision != req.Project.ExpectedPolicyRevision {
		return out, completionConflict()
	}
	params := db.CreateDeliveryCompletionAuthorizationParams{}
	evidenceIssue := issue
	if completionIsCollaboration(req.Target.Kind) {
		evidenceIssue, err = completionFreezeCollaboration(ctx, tx, issuer, req, issue, &params)
	} else if req.Consumer.Kind == "github" || req.Consumer.Kind == "vcs" {
		err = FreezeDeliveryWebhookConsumerInTx(ctx, tx, req, issue, &params, issuer.GitHubWebhookConfigIdentity)
	} else {
		err = completionFreezeConsumer(ctx, tx, req, issue, &params)
	}
	if err != nil {
		return out, err
	}
	observation, err := EvaluateDeliveryEvidenceInTx(ctx, tx, issuer, evidenceIssue, completionEvidenceRequest(req, evidenceIssue, project))
	if err != nil {
		return out, err
	}
	if observation.Verdict != "pass" || observation.ValidUntil == nil {
		return out, completionConflict()
	}
	if err = completionFreezeIssuer(ctx, tx, issuer.Credential, &params); err != nil {
		return out, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	if !req.ExpiresAt.After(now.Time) || req.ExpiresAt.After(now.Time.Add(24*time.Hour)) || req.ExpiresAt.After(*observation.ValidUntil) || (params.IssuerCredentialExpiresAt.Valid && req.ExpiresAt.After(params.IssuerCredentialExpiresAt.Time)) {
		return out, DeliveryCompletionError{409, "delivery_completion_expiry_invalid"}
	}
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	observedJSON, err := json.Marshal(observation)
	if err != nil {
		return out, err
	}
	params.ID = dbid.NewV7()
	params.WorkspaceID = workspace
	params.IssuerUserID = completionUUIDPG(issuer.Credential.Principal.UserID)
	params.RequestKey = key
	params.OperationID = completionUUIDPG(req.OperationID)
	params.ContractVersion = DeliveryCompletionContract
	params.RequestDigest = req.Digest()
	params.RequestJson = requestJSON
	params.CreatedAt = completionTime(now.Time)
	params.ExpiresAt = completionTime(req.ExpiresAt)
	params.Reason = req.Reason
	params.TargetKind = req.Target.Kind
	params.IssueID = issue.ID
	params.IssueRevision = issue.Revision
	params.ExpectedStatus = issue.Status
	params.TargetStatus = target.Key
	if req.Target.TargetPosition != nil {
		params.TargetPosition = pgtype.Float8{Float64: *req.Target.TargetPosition, Valid: true}
	}
	params.ProjectID = project.ID
	params.ProjectRevision = project.Revision
	params.PolicyRevision = policy.Revision
	params.EvidenceIssueID = evidenceIssue.ID
	params.EvidenceIssueRevision = evidenceIssue.Revision
	params.ApprovalID = completionUUIDPG(observation.ApprovalID)
	params.ExecutionID = completionUUIDPG(observation.ExecutionID)
	params.ProducerAgentID = completionUUIDPG(observation.ProducerAgentID)
	params.ProducerTaskID = completionUUIDPG(observation.ProducerTaskID)
	params.ProducerGeneration = observation.ProducerTaskGeneration
	params.SpecDigest = observation.SpecDigest
	params.ArtifactDigest = observation.ArtifactDigest
	params.ReceiptDigest = observation.ReceiptDigest
	params.SourceRevision = observation.SourceRevision
	params.EvidenceExpiresAt = completionTime(*observation.ValidUntil)
	params.Observation = observedJSON
	row, err = q.CreateDeliveryCompletionAuthorization(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, completionConflict()
	}
	if err != nil {
		return out, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, issuer.Credential); err != nil {
		return out, err
	}
	return completionView(ctx, tx, row)
}

func GetDeliveryCompletionAuthorizationInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, id pgtype.UUID) (out DeliveryCompletionAuthorizationView, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if err = completionReadSetup(ctx, tx, access); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM delivery_completion_authorization WHERE id=$1 FOR SHARE NOWAIT`, id); err != nil {
		return out, err
	}
	row, err := db.New(tx).GetDeliveryCompletionAuthorization(ctx, id)
	if err != nil {
		return out, err
	}
	if err = readCompletionAuthority(ctx, tx, access, row); err != nil {
		return out, err
	}
	return completionView(ctx, tx, row)
}
func GetDeliveryCompletionByRequestInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, workspace, key pgtype.UUID) (out DeliveryCompletionAuthorizationView, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if kind, _, e := access.Credential.Actor(); e != nil || kind != "human" {
		return out, completionDenied()
	}
	if err = completionReadSetup(ctx, tx, access); err != nil {
		return out, err
	}
	row, err := db.New(tx).GetDeliveryCompletionByRequest(ctx, db.GetDeliveryCompletionByRequestParams{WorkspaceID: workspace, IssuerUserID: completionUUIDPG(access.Credential.Principal.UserID), RequestKey: key})
	if err != nil {
		return out, err
	}
	if err = completionReadAuthority(ctx, tx, access, row); err != nil {
		return out, err
	}
	return completionView(ctx, tx, row)
}
func RevokeDeliveryCompletionAuthorizationInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, id, key pgtype.UUID, reason string) (out DeliveryCompletionAuthorizationView, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	reason = strings.TrimSpace(reason)
	if !completionReason(reason) || !key.Valid || !completionUUID(util.UUIDToString(key)) {
		return out, completionInvalid()
	}
	if kind, _, e := access.Credential.Actor(); e != nil || kind != "human" {
		return out, completionDenied()
	}
	if err = completionReadSetup(ctx, tx, access); err != nil {
		return out, err
	}
	q := db.New(tx)
	row, err := q.LockDeliveryCompletionAuthorization(ctx, id)
	if err != nil {
		return out, err
	}
	if _, err = completionStoredRequest(row); err != nil {
		return out, err
	}
	if access.RequestedWorkspaceID != "" && access.RequestedWorkspaceID != util.UUIDToString(row.WorkspaceID) {
		return out, pgx.ErrNoRows
	}
	if err = LockArtifactWorkspace(ctx, q, row.WorkspaceID); err != nil {
		return out, err
	}
	members, err := LockArtifactUsers(ctx, q, row.WorkspaceID, access.Credential.Principal.UserID)
	if err != nil {
		return out, err
	}
	issue, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: row.IssueID, WorkspaceID: row.WorkspaceID})
	if err != nil {
		return out, err
	}
	actor := completionUUIDPG(access.Credential.Principal.UserID)
	if actor == row.IssuerUserID {
		if err = completionIssuerRead(ctx, tx, access, row, issue); err != nil {
			return out, err
		}
	} else {
		m := members[access.Credential.Principal.UserID]
		if m.Role != "owner" && m.Role != "admin" {
			return out, completionDenied()
		}
	}
	raw, _ := json.Marshal(struct{ Authorization, Actor, Key, Reason string }{util.UUIDToString(id), util.UUIDToString(actor), util.UUIDToString(key), reason})
	digest := completionDigest("revocation", raw)
	terminal, err := q.GetDeliveryCompletionTerminal(ctx, row.ID)
	if err == nil {
		if terminal.Kind != "revoked" || terminal.RevokerUserID != actor || terminal.RevocationRequestKey != key || terminal.RevocationDigest.String != digest {
			return out, completionConflict()
		}
		out, err = completionView(ctx, tx, row)
		out.Replayed = true
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	_, err = q.CreateDeliveryCompletionTerminal(ctx, db.CreateDeliveryCompletionTerminalParams{AuthorizationID: row.ID, Kind: "revoked", ActorKind: "human", ActorUserID: actor, RevokerUserID: actor, RevocationRequestKey: key, RevocationReason: completionText(reason), RevocationDigest: completionText(digest)})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, completionConflict()
	}
	if err != nil {
		return out, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return out, err
	}
	return completionView(ctx, tx, row)
}
func completionReadSetup(ctx context.Context, tx pgx.Tx, access ArtifactAccess) error {
	if tx == nil {
		return DeliveryCompletionError{503, "delivery_completion_unavailable"}
	}
	if _, _, err := access.Credential.Actor(); err != nil {
		return err
	}
	if err := PrepareArtifactTransaction(ctx, tx); err != nil {
		return err
	}
	if err := CheckDeliveryCompletionSchema(ctx, tx); err != nil {
		return err
	}
	return CheckArtifactCredentialInTx(ctx, tx, access.Credential)
}
func completionReadAuthority(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization) error {
	return completionReadAuthorityInTx(ctx, tx, access, row, false)
}

func readCompletionAuthority(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization) error {
	return completionReadAuthorityInTx(ctx, tx, access, row, true)
}

func completionReadAuthorityInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization, readProjection bool) error {
	if _, err := completionStoredRequest(row); err != nil {
		return err
	}
	if access.RequestedWorkspaceID != "" && access.RequestedWorkspaceID != util.UUIDToString(row.WorkspaceID) {
		return pgx.ErrNoRows
	}
	q := db.New(tx)
	if err := LockArtifactWorkspace(ctx, q, row.WorkspaceID); err != nil {
		return err
	}
	var issue db.Issue
	var err error
	if readProjection {
		issue, err = q.LockDeliveryEvidenceIssueRead(ctx, db.LockDeliveryEvidenceIssueReadParams{ID: row.IssueID, WorkspaceID: row.WorkspaceID})
	} else {
		issue, err = q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: row.IssueID, WorkspaceID: row.WorkspaceID})
	}
	if err != nil {
		return err
	}
	kind, _, actorErr := access.Credential.Actor()
	if actorErr != nil {
		return actorErr
	}
	if kind == "human" && access.Credential.Principal.UserID == util.UUIDToString(row.IssuerUserID) {
		if readProjection {
			return readCompletionIssuer(ctx, tx, access, row, issue)
		}
		return completionIssuerRead(ctx, tx, access, row, issue)
	}
	if row.TargetKind == "collaboration_admin" && !access.CollaborationAdministrationEnabled {
		return completionDenied()
	}
	return completionConsumerAuthority(ctx, tx, row, access.Credential, issue, false)
}
func completionIssuerRead(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization, issue db.Issue) error {
	return completionIssuerReadInTx(ctx, tx, access, row, issue, false)
}

func readCompletionIssuer(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization, issue db.Issue) error {
	return completionIssuerReadInTx(ctx, tx, access, row, issue, true)
}

func completionIssuerReadInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, row db.DeliveryCompletionAuthorization, issue db.Issue, readProjection bool) error {
	if kind, _, err := access.Credential.Actor(); err != nil || kind != "human" || access.Credential.Principal.UserID != util.UUIDToString(row.IssuerUserID) {
		return completionDenied()
	}
	projectAccess := completionCollaborationAccess
	sourceIssue := completionSourceIssue
	observe := EvaluateDeliveryEvidenceInTx
	if readProjection {
		projectAccess = readCompletionCollaborationAccess
		sourceIssue = readCompletionSourceIssue
		observe = evaluateDeliveryEvidenceReadInTx
	}
	if row.TargetKind == "ordinary" {
		if err := completionOrdinary(ctx, db.New(tx), issue); err != nil {
			return err
		}
	} else if _, err := projectAccess(ctx, tx, issue, row.IssuerUserID); err != nil {
		return err
	}
	req, err := completionStoredRequest(row)
	if err != nil {
		return err
	}
	evidenceIssue, err := sourceIssue(ctx, tx, req, issue, false)
	if err != nil {
		return err
	}
	project := db.Project{}
	if evidenceIssue.ProjectID.Valid {
		var err error
		project, _, err = LockDeliveryProjectPolicy(ctx, tx, evidenceIssue.WorkspaceID, evidenceIssue.ProjectID)
		if err != nil {
			return err
		}
	}
	observed, err := observe(ctx, tx, access, evidenceIssue, completionEvidenceRequest(req, evidenceIssue, project))
	if err != nil {
		return err
	}
	if observed.ExecutionID == "" || observed.ExecutionID != util.UUIDToString(row.ExecutionID) {
		return completionConflict()
	}
	return nil
}
func completionView(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization) (DeliveryCompletionAuthorizationView, error) {
	out := DeliveryCompletionAuthorizationView{AuthorizationID: util.UUIDToString(row.ID), OperationID: util.UUIDToString(row.OperationID), IssueID: util.UUIDToString(row.IssueID), IssueRevision: row.IssueRevision, TargetStatus: row.TargetStatus, ContractVersion: row.ContractVersion, CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: row.ExpiresAt.Time.UTC(), State: "issued", Applicability: "not_checked"}
	terminal, err := db.New(tx).GetDeliveryCompletionTerminal(ctx, row.ID)
	if err == nil {
		out.State = terminal.Kind
		out.Applicability = terminal.Kind
		if terminal.Kind == "consumed" {
			receipt, e := completionReceipt(row, terminal, false)
			if e != nil {
				return out, e
			}
			out.CompletionReceipt = &receipt
		}
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	now, err := db.New(tx).ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	if !now.Time.Before(row.ExpiresAt.Time) {
		out.Applicability = "expired"
	}
	return out, nil
}
func completionReceipt(row db.DeliveryCompletionAuthorization, terminal db.DeliveryCompletionTerminal, replayed bool) (DeliveryCompletionReceipt, error) {
	if terminal.Kind != "consumed" || terminal.AuthorizationID != row.ID || terminal.OperationID != row.OperationID ||
		!terminal.IssueRevisionBefore.Valid || terminal.IssueRevisionBefore.Int64 != row.IssueRevision ||
		!terminal.IssueRevisionAfter.Valid || terminal.IssueRevisionAfter.Int64 <= row.IssueRevision ||
		!terminal.StatusAfter.Valid || terminal.StatusAfter.String != row.TargetStatus || !terminal.RecordedAt.Valid ||
		!terminal.MutationDigest.Valid || !DeliveryHex(terminal.MutationDigest.String, 64) ||
		terminal.RevokerUserID.Valid || terminal.RevocationRequestKey.Valid || terminal.RevocationReason.Valid || terminal.RevocationDigest.Valid {
		return DeliveryCompletionReceipt{}, completionConflict()
	}
	// The terminal is a retained fact, not an arbitrary projection. Keep its real
	// consumer and exact variant separate from the issuer and other workflows.
	switch row.ConsumerKind {
	case "human", "admin":
		if terminal.ActorKind != row.ConsumerKind || !terminal.ActorUserID.Valid || terminal.ActorUserID != row.ConsumerUserID || terminal.ActorAgentID.Valid || terminal.IntegrationID.Valid {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
	case "task":
		if terminal.ActorKind != "task" || !terminal.ActorUserID.Valid || !terminal.ActorAgentID.Valid || terminal.ActorAgentID != row.ConsumerAgentID || terminal.IntegrationID.Valid {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
	case "github", "vcs":
		if terminal.ActorKind != row.ConsumerKind || terminal.ActorUserID.Valid || terminal.ActorAgentID.Valid || !terminal.IntegrationID.Valid || terminal.IntegrationID != row.IntegrationID || !terminal.ProviderEventID.Valid || terminal.ProviderEventID.String == "" || !terminal.ProviderEventDigest.Valid || !DeliveryHex(terminal.ProviderEventDigest.String, 64) {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
	default:
		return DeliveryCompletionReceipt{}, completionConflict()
	}
	if row.ConsumerKind != "github" && row.ConsumerKind != "vcs" && (terminal.ProviderEventID.Valid || terminal.ProviderEventDigest.Valid) {
		return DeliveryCompletionReceipt{}, completionConflict()
	}
	switch row.TargetKind {
	case "ordinary":
		if row.ConsumerKind == "admin" || terminal.CollaborationOperationID.Valid || terminal.AdminOperationID.Valid || terminal.ResultSubmissionID.Valid || terminal.ResultCollaborationRevision.Valid {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
	case "collaboration_self", "collaboration_submission", "collaboration_admin":
		if !row.CollaborationRevision.Valid || !terminal.CollaborationOperationID.Valid || !terminal.ResultCollaborationRevision.Valid || terminal.ResultCollaborationRevision.Int64 <= row.CollaborationRevision.Int64 || terminal.ResultCollaborationRevision.Int64 != row.CollaborationRevision.Int64+1 {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
		if row.TargetKind == "collaboration_admin" {
			if row.ConsumerKind != "admin" || !terminal.AdminOperationID.Valid || terminal.ResultSubmissionID.Valid {
				return DeliveryCompletionReceipt{}, completionConflict()
			}
		} else if row.ConsumerKind != "human" || terminal.AdminOperationID.Valid || !terminal.ResultSubmissionID.Valid || row.TargetKind == "collaboration_submission" && terminal.ResultSubmissionID != row.SubmissionID {
			return DeliveryCompletionReceipt{}, completionConflict()
		}
	default:
		return DeliveryCompletionReceipt{}, completionConflict()
	}
	out := DeliveryCompletionReceipt{AuthorizationID: util.UUIDToString(row.ID), OperationID: util.UUIDToString(row.OperationID), IssueID: util.UUIDToString(row.IssueID), CompletedRevision: terminal.IssueRevisionAfter.Int64, TargetStatus: terminal.StatusAfter.String, CompletedAt: terminal.RecordedAt.Time.UTC(), Replayed: replayed}
	if terminal.ResultCollaborationRevision.Valid {
		x := fmt.Sprint(terminal.ResultCollaborationRevision.Int64)
		out.CollaborationRevision = &x
	}
	if terminal.ResultSubmissionID.Valid {
		x := util.UUIDToString(terminal.ResultSubmissionID)
		out.SubmissionID = &x
	}
	return out, nil
}

func completionStoredRequest(row db.DeliveryCompletionAuthorization) (DeliveryCompletionAuthorizationRequest, error) {
	req, err := ParseDeliveryCompletionAuthorization(row.RequestJson)
	if err != nil {
		return req, completionConflict()
	}
	var saved DeliveryObservation
	if json.Unmarshal(row.Observation, &saved) != nil {
		return req, completionConflict()
	}
	if req.Digest() != row.RequestDigest || req.ContractVersion != row.ContractVersion || req.SourceWorkspaceID != util.UUIDToString(row.WorkspaceID) || req.OperationID != util.UUIDToString(row.OperationID) || req.Target.Kind != row.TargetKind || req.Target.IssueID != util.UUIDToString(row.IssueID) || req.Target.ExpectedIssueRevision != row.IssueRevision || req.Target.ExpectedStatus != row.ExpectedStatus || req.Target.TargetStatus != row.TargetStatus || req.Project.ID != util.UUIDToString(row.ProjectID) || req.Project.ExpectedRevision != row.ProjectRevision || req.Project.ExpectedPolicyRevision != row.PolicyRevision || req.Evidence.IssueID != util.UUIDToString(row.EvidenceIssueID) || req.Evidence.ExpectedIssueRevision != row.EvidenceIssueRevision || req.Evidence.ApprovalID != util.UUIDToString(row.ApprovalID) || req.Evidence.ExpectedSpecDigest != row.SpecDigest || req.Evidence.ExpectedArtifactDigest != row.ArtifactDigest || req.Evidence.ExpectedSourceRevision != row.SourceRevision || !req.ExpiresAt.Equal(row.ExpiresAt.Time) || req.Consumer.Kind != row.ConsumerKind {
		return req, completionConflict()
	}
	if (req.Target.TargetPosition != nil) != row.TargetPosition.Valid || req.Target.TargetPosition != nil && *req.Target.TargetPosition != row.TargetPosition.Float64 {
		return req, completionConflict()
	}
	switch req.Consumer.Kind {
	case "human", "admin":
		if req.Consumer.UserID != util.UUIDToString(row.ConsumerUserID) {
			return req, completionConflict()
		}
	case "github", "vcs":
		integration := req.Consumer.ConnectionID
		if req.Consumer.Kind == "github" {
			integration = req.Consumer.InstallationID
		}
		if completionUUIDPG(integration) != row.IntegrationID || req.Consumer.ExpectedTrustRevision != row.IntegrationTrustRevision.Int64 || completionUUIDPG(req.Consumer.PullRequestID) != row.PullRequestID {
			return req, completionConflict()
		}
	case "task":
		if req.Consumer.AgentID != util.UUIDToString(row.ConsumerAgentID) || req.Consumer.TaskID != util.UUIDToString(row.ConsumerTaskID) || !row.ConsumerTaskGeneration.Valid || req.Consumer.TaskGeneration != row.ConsumerTaskGeneration.Int64 {
			return req, completionConflict()
		}
	}
	if err := completionValidateStoredCollaboration(row, req); err != nil {
		return req, err
	}
	if saved.Verdict != "pass" || saved.PolicyVersion != DeliveryEvidencePolicy || saved.ProjectID == nil || *saved.ProjectID != util.UUIDToString(row.ProjectID) || saved.ProjectRevision == nil || *saved.ProjectRevision != row.ProjectRevision || saved.ObservedAt.IsZero() || !saved.ObservedAt.Before(row.ExpiresAt.Time) || saved.IssueID != util.UUIDToString(row.EvidenceIssueID) || saved.IssueRevision != row.EvidenceIssueRevision || saved.ExecutionID != util.UUIDToString(row.ExecutionID) || saved.ApprovalID != util.UUIDToString(row.ApprovalID) || saved.SpecDigest != row.SpecDigest || saved.ArtifactDigest != row.ArtifactDigest || saved.ReceiptDigest != row.ReceiptDigest || saved.SourceRevision != row.SourceRevision || saved.ObservedHeadSHA != row.SourceRevision || saved.ProducerAgentID != util.UUIDToString(row.ProducerAgentID) || saved.ProducerTaskID != util.UUIDToString(row.ProducerTaskID) || saved.ProducerTaskGeneration != row.ProducerGeneration || saved.ValidUntil == nil || !saved.ValidUntil.Equal(row.EvidenceExpiresAt.Time) {
		return req, completionConflict()
	}
	return req, nil
}
func completionEvidenceRequest(req DeliveryCompletionAuthorizationRequest, issue db.Issue, project db.Project) DeliveryEvidenceRequest {
	out := DeliveryEvidenceRequest{PolicyVersion: DeliveryEvidencePolicy, IssueRevision: issue.Revision, ApprovalID: req.Evidence.ApprovalID, SourceRevision: req.Evidence.ExpectedSourceRevision, SpecDigest: req.Evidence.ExpectedSpecDigest, ArtifactDigest: req.Evidence.ExpectedArtifactDigest}
	if issue.ProjectID.Valid {
		id := util.UUIDToString(project.ID)
		revision := project.Revision
		out.ProjectID = &id
		out.ProjectRevision = &revision
	}
	return out
}
func completionRevalidateEvidence(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, req DeliveryCompletionAuthorizationRequest, issue db.Issue, metrics *obsmetrics.BusinessMetrics) error {
	evidenceIssue, err := completionSourceIssue(ctx, tx, req, issue, true)
	if err != nil {
		return err
	}
	if completionIsCollaboration(row.TargetKind) {
		if _, err = completionCollaborationAccess(ctx, tx, issue, row.IssuerUserID); err != nil {
			return err
		}
	}
	if err := CheckArtifactCredentialInTx(ctx, tx, completionIssuerCredential(row)); err != nil {
		return err
	}
	project, _, err := LockDeliveryProjectPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return err
	}
	access := ArtifactAccess{Credential: completionIssuerCredential(row), RequestedWorkspaceID: util.UUIDToString(row.WorkspaceID), Metrics: metrics}
	live, err := EvaluateDeliveryEvidenceInTx(ctx, tx, access, evidenceIssue, completionEvidenceRequest(req, evidenceIssue, project))
	if err != nil {
		return err
	}
	if live.Verdict != "pass" || live.ExecutionID != util.UUIDToString(row.ExecutionID) || live.SpecDigest != row.SpecDigest || live.ArtifactDigest != row.ArtifactDigest || live.ReceiptDigest != row.ReceiptDigest || live.ProducerTaskID != util.UUIDToString(row.ProducerTaskID) || live.ProducerAgentID != util.UUIDToString(row.ProducerAgentID) || live.ProducerTaskGeneration != row.ProducerGeneration || live.ValidUntil == nil || !live.ValidUntil.Equal(row.EvidenceExpiresAt.Time) || !live.ObservedAt.Before(row.ExpiresAt.Time) {
		return completionConflict()
	}
	return nil
}
func completionFreezeIssuer(ctx context.Context, tx pgx.Tx, c ArtifactCredentialSnapshot, p *db.CreateDeliveryCompletionAuthorizationParams) error {
	if err := CheckArtifactCredentialInTx(ctx, tx, c); err != nil {
		return err
	}
	p.IssuerCredentialKind = string(c.Principal.Kind)
	if c.Principal.Kind == auth.CredentialJWT {
		p.IssuerCredentialExpiresAt = completionTime(c.Principal.ExpiresAt)
	} else if c.Principal.Kind == auth.CredentialPAT {
		token, err := db.New(tx).LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: c.Principal.PATTokenHash, UserID: completionUUIDPG(c.Principal.UserID)})
		if err != nil {
			return err
		}
		p.IssuerPatID = token.ID
		p.IssuerPatHash = completionText(token.TokenHash)
		p.IssuerCredentialExpiresAt = token.ExpiresAt
	} else {
		return completionDenied()
	}
	p.IssuerPasswordMode = c.PasswordSession != nil
	if c.PasswordSession != nil {
		s := c.PasswordSession
		p.IssuerPasswordVersion = completionInt(s.Version)
		p.IssuerPasswordKind = completionText(s.Kind)
		if s.BindingID != "" {
			p.IssuerBindingID = completionUUIDPG(s.BindingID)
			p.IssuerBindingEpoch = completionInt(s.BindingEpoch)
			p.IssuerBindingWorkspaceID = completionUUIDPG(s.WorkspaceID)
			p.IssuerBindingDaemonID = completionText(s.DaemonID)
		}
	}
	return nil
}
func completionIssuerCredential(row db.DeliveryCompletionAuthorization) ArtifactCredentialSnapshot {
	c := ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: util.UUIDToString(row.IssuerUserID), Kind: auth.CredentialKind(row.IssuerCredentialKind)}}
	if row.IssuerCredentialExpiresAt.Valid {
		c.Principal.ExpiresAt = row.IssuerCredentialExpiresAt.Time
	}
	if row.IssuerPatHash.Valid {
		c.Principal.PATTokenHash = row.IssuerPatHash.String
	}
	if row.IssuerPasswordMode {
		s := auth.PasswordSession{UserID: c.Principal.UserID, Version: row.IssuerPasswordVersion.Int64, Kind: row.IssuerPasswordKind.String, ExpiresAt: c.Principal.ExpiresAt}
		if row.IssuerBindingID.Valid {
			s.BindingID = util.UUIDToString(row.IssuerBindingID)
			s.BindingEpoch = row.IssuerBindingEpoch.Int64
			s.WorkspaceID = util.UUIDToString(row.IssuerBindingWorkspaceID)
			s.DaemonID = row.IssuerBindingDaemonID.String
		}
		c.PasswordSession = &s
	}
	return c
}
func completionFreezeConsumer(ctx context.Context, tx pgx.Tx, req DeliveryCompletionAuthorizationRequest, issue db.Issue, p *db.CreateDeliveryCompletionAuthorizationParams) error {
	q := db.New(tx)
	p.ConsumerKind = req.Consumer.Kind
	switch req.Consumer.Kind {
	case "human":
		if _, err := LockArtifactUsers(ctx, q, issue.WorkspaceID, req.Consumer.UserID); err != nil {
			return err
		}
		p.ConsumerUserID = completionUUIDPG(req.Consumer.UserID)
		return nil
	case "task":
		taskID := completionUUIDPG(req.Consumer.TaskID)
		if _, err := q.LockTaskCapabilityPrincipals(ctx, taskID); err != nil {
			return err
		}
		task, err := q.GetAgentTask(ctx, taskID)
		if err != nil {
			return err
		}
		if task.IssueID != issue.ID || task.AgentID != completionUUIDPG(req.Consumer.AgentID) || task.ClaimGeneration != req.Consumer.TaskGeneration || !task.RuntimeID.Valid || !task.OriginatorUserID.Valid || (task.Status != "running" && task.Status != "dispatched") {
			return completionDenied()
		}
		agent, err := q.GetAgent(ctx, task.AgentID)
		if err != nil {
			return err
		}
		if agent.WorkspaceID != issue.WorkspaceID || agent.ArchivedAt.Valid || !AutonomyAtLeast(agent.AutonomyLevel, AutonomyContributor) || auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != "" {
			return completionDenied()
		}
		p.ConsumerAgentID = task.AgentID
		p.ConsumerTaskID = task.ID
		p.ConsumerTaskGeneration = completionInt(task.ClaimGeneration)
		p.ConsumerRuntimeID = task.RuntimeID
		p.ConsumerOriginatorUserID = task.OriginatorUserID
		binding, epoch, e := completionTaskBinding(ctx, tx, task)
		if e != nil {
			return e
		}
		p.ConsumerBindingID = binding
		p.ConsumerBindingEpoch = epoch
		return nil
	default:
		return completionUnsupported()
	}
}
func completionConsumerAuthority(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, c ArtifactCredentialSnapshot, issue db.Issue, newAdmission bool) error {
	if completionIsCollaboration(row.TargetKind) {
		return completionCollaborationConsumer(ctx, tx, row, c, issue, newAdmission)
	}
	if row.TargetKind != "ordinary" {
		return completionUnsupported()
	}
	if err := completionOrdinary(ctx, db.New(tx), issue); err != nil {
		return err
	}
	kind, id, err := c.Actor()
	if err != nil {
		return err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, c); err != nil {
		return err
	}
	q := db.New(tx)
	if row.ConsumerKind == "human" {
		if kind != "human" || completionUUIDPG(id) != row.ConsumerUserID {
			return pgx.ErrNoRows
		}
		_, err = LockArtifactUsers(ctx, q, issue.WorkspaceID, c.Principal.UserID)
		return err
	}
	if row.ConsumerKind != "task" {
		return completionUnsupported()
	}
	if kind != "agent" || c.TaskToken == nil || c.TaskToken.TaskID != row.ConsumerTaskID || c.TaskToken.AgentID != row.ConsumerAgentID || c.TaskToken.WorkspaceID != issue.WorkspaceID {
		return pgx.ErrNoRows
	}
	if _, err = q.LockTaskCapabilityPrincipals(ctx, row.ConsumerTaskID); err != nil {
		return err
	}
	task, err := q.GetAgentTask(ctx, row.ConsumerTaskID)
	if err != nil {
		return err
	}
	if task.IssueID != issue.ID || task.AgentID != row.ConsumerAgentID || task.ClaimGeneration != row.ConsumerTaskGeneration.Int64 || task.RuntimeID != row.ConsumerRuntimeID || task.OriginatorUserID != row.ConsumerOriginatorUserID || (newAdmission && task.Status != "running" && task.Status != "dispatched") {
		return completionDenied()
	}
	binding, epoch, bindingErr := completionTaskBinding(ctx, tx, task)
	if bindingErr != nil {
		return bindingErr
	}
	if binding != row.ConsumerBindingID || epoch != row.ConsumerBindingEpoch || c.TaskToken.InstallationBindingID != binding || c.TaskToken.InstallationBindingEpoch != epoch {
		return completionDenied()
	}
	agent, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return err
	}
	runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
	if err != nil {
		return err
	}
	if agent.WorkspaceID != issue.WorkspaceID || agent.RuntimeID != runtime.ID || runtime.WorkspaceID != issue.WorkspaceID || runtime.OwnerID != completionUUIDPG(c.Principal.UserID) || agent.ArchivedAt.Valid || !AutonomyAtLeast(agent.AutonomyLevel, AutonomyContributor) || auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != "" {
		return completionDenied()
	}
	if _, err = LockArtifactUsers(ctx, q, issue.WorkspaceID, c.Principal.UserID, util.UUIDToString(task.OriginatorUserID)); err != nil {
		return err
	}
	targets, err := q.LockTaskCapabilityInvocationTargets(ctx, agent.ID)
	if err != nil {
		return err
	}
	if !auth.AgentInvocationAllowed(agent, targets, util.UUIDToString(task.OriginatorUserID), true, false) {
		return completionDenied()
	}
	return nil
}

// The execution task owns the immutable managed installation snapshot. The
// runtime's current namespace is locked even for explicit absence, so attaching
// a managed binding after authorization cannot preserve an unbound grant.
func completionTaskBinding(ctx context.Context, tx pgx.Tx, task db.AgentTaskQueue) (pgtype.UUID, pgtype.Int8, error) {
	q := db.New(tx)
	runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
	if err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	if !runtime.DaemonID.Valid || runtime.DaemonID.String == "" {
		if task.ExecutionBindingID.Valid || task.ExecutionBindingEpoch.Valid {
			return pgtype.UUID{}, pgtype.Int8{}, completionDenied()
		}
		return pgtype.UUID{}, pgtype.Int8{}, nil
	}
	acquired, err := q.TryLockCollaborationAuthorityNamespace(ctx, "daemon:"+util.UUIDToString(runtime.WorkspaceID)+":"+runtime.DaemonID.String)
	if err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	if !acquired {
		return pgtype.UUID{}, pgtype.Int8{}, completionConflict()
	}
	current, err := q.GetActiveInstallationBinding(ctx, db.GetActiveInstallationBindingParams{WorkspaceID: runtime.WorkspaceID, DaemonID: runtime.DaemonID.String})
	if errors.Is(err, pgx.ErrNoRows) {
		if task.ExecutionBindingID.Valid || task.ExecutionBindingEpoch.Valid {
			return pgtype.UUID{}, pgtype.Int8{}, completionDenied()
		}
		return pgtype.UUID{}, pgtype.Int8{}, nil
	}
	if err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	if !task.ExecutionBindingID.Valid || !task.ExecutionBindingEpoch.Valid || task.ExecutionBindingID != current.ID || task.ExecutionBindingEpoch.Int64 != current.BindingEpoch || runtime.OwnerID != current.PrincipalUserID {
		return pgtype.UUID{}, pgtype.Int8{}, completionDenied()
	}
	current, err = q.TryLockCollaborationAuthorityBinding(ctx, current.ID)
	if err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	if _, err = q.TryLockCollaborationAuthorityInstallation(ctx, current.InstallationID); err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	if _, err = q.LockArtifactPasswordCredential(ctx, current.PrincipalUserID); err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	var org pgtype.UUID
	if err = tx.QueryRow(ctx, `SELECT ow.organization_id FROM organization_workspace ow JOIN organization o ON o.id=ow.organization_id WHERE ow.workspace_id=$1 FOR SHARE OF ow,o NOWAIT`, current.WorkspaceID).Scan(&org); err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	session := auth.PasswordSession{UserID: util.UUIDToString(current.PrincipalUserID), Version: current.AuthVersion, Kind: "task_token", WorkspaceID: util.UUIDToString(current.WorkspaceID), DaemonID: current.DaemonID, BindingID: util.UUIDToString(current.ID), BindingEpoch: current.BindingEpoch}
	if _, err = auth.CheckPasswordSessionVersion(ctx, q, session); err != nil {
		return pgtype.UUID{}, pgtype.Int8{}, err
	}
	return current.ID, completionInt(current.BindingEpoch), nil
}
