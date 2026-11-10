package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/admission"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DeliveryMutationIntent is assembled from locked server state by the real writer.
// Candidate is never a client-supplied issue snapshot. AuxiliaryChanges covers
// attachment/label/property writes not represented by the issue row.
type DeliveryMutationIntent struct {
	Before            db.Issue
	Candidate         db.Issue
	Completion        *DeliveryCompletionBinding
	Credential        ArtifactCredentialSnapshot
	RequestedPosition *float64
	AuxiliaryChanges  bool
	Webhook           *DeliveryWebhookProof
	Collaboration     *DeliveryCompletionTarget
	Metrics           *obsmetrics.BusinessMetrics
}
type DeliveryPrepareState string

const (
	DeliveryNotApplicable DeliveryPrepareState = "not_applicable"
	DeliveryReplayed      DeliveryPrepareState = "replayed"
	DeliveryPrepared      DeliveryPrepareState = "prepared"
)

type DeliveryPrepareResult struct {
	State    DeliveryPrepareState
	Prepared *DeliveryPreparedMutation
	Receipt  *DeliveryCompletionReceipt
}

// Fields remain private so only a successful preparation in this exact tx can finish.
type DeliveryPreparedMutation struct {
	tx             pgx.Tx
	authorization  db.DeliveryCompletionAuthorization
	request        DeliveryCompletionAuthorizationRequest
	credential     ArtifactCredentialSnapshot
	webhook        *DeliveryWebhookProof
	before         db.Issue
	candidate      db.Issue
	mutationDigest string
	finished       bool
	metrics        *obsmetrics.BusinessMetrics
}
type DeliveryWriterResult struct {
	CollaborationOperationID pgtype.UUID
	AdminOperationID         pgtype.UUID
	SubmissionID             pgtype.UUID
	CollaborationRevision    int64
	ProviderEventID          string
	ProviderEventDigest      string
}
type DeliveryCompletionReceipt struct {
	AuthorizationID       string    `json:"authorization_id"`
	OperationID           string    `json:"operation_id"`
	IssueID               string    `json:"issue_id"`
	CompletedRevision     int64     `json:"completed_revision"`
	TargetStatus          string    `json:"target_status"`
	CompletedAt           time.Time `json:"completed_at"`
	CollaborationRevision *string   `json:"collaboration_revision,omitempty"`
	SubmissionID          *string   `json:"submission_id,omitempty"`
	Replayed              bool      `json:"replayed"`
}

func deliveryCompletionError(err error) error {
	if err == nil {
		return nil
	}
	var p *pgconn.PgError
	if errors.As(err, &p) && (p.Code == "55P03" || p.Code == "40P01" || p.Code == "40001" || p.Code == "23505") {
		return completionConflict()
	}
	var a ArtifactError
	if errors.As(err, &a) {
		if a.Status == 403 {
			return completionDenied()
		}
		if a.Status == 409 {
			return completionConflict()
		}
	}
	if errors.Is(err, auth.ErrPasswordSession) {
		return completionDenied()
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Missing current authority/resource facts are a content-free refusal.
		// Actual database errors remain errors and cannot become successful gates.
		return DeliveryCompletionError{404, "delivery_completion_not_found"}
	}
	return err
}
func CheckDeliveryCompletionSchema(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return DeliveryCompletionError{503, "delivery_completion_unavailable"}
	}
	ready, err := db.New(tx).DeliveryCompletionSchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready.Valid || !ready.Bool {
		return DeliveryCompletionError{503, "delivery_completion_schema_unavailable"}
	}
	return nil
}

// LockDeliveryProjectPolicy reads the current policy under its governing project lock.
// It never treats a missing schema or unrecognized contract as virtual off.
func LockDeliveryProjectPolicy(ctx context.Context, tx pgx.Tx, workspace, project pgtype.UUID) (db.Project, DeliveryPolicy, error) {
	if err := CheckDeliveryPolicySchema(ctx, tx); err != nil {
		return db.Project{}, DeliveryPolicy{}, err
	}
	if !project.Valid {
		return db.Project{}, DeliveryPolicy{Mode: "off", PolicyVersion: DeliveryEvidencePolicy, Stage: DeliveryPolicyStage}, nil
	}
	row, err := db.New(tx).LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: project, WorkspaceID: workspace})
	if err != nil {
		return row, DeliveryPolicy{}, deliveryCompletionError(err)
	}
	policy, err := CurrentDeliveryPolicyInTx(ctx, tx, workspace, project)
	if err != nil {
		return row, policy, err
	}
	if (policy.Mode != "off" && policy.Mode != "observe" && policy.Mode != "enforce") || policy.PolicyVersion != DeliveryEvidencePolicy || policy.Stage != DeliveryPolicyStage {
		return row, policy, DeliveryCompletionError{503, "delivery_policy_unavailable"}
	}
	return row, policy, nil
}
func CheckDeliveryCreateInTx(ctx context.Context, tx pgx.Tx, workspace, project pgtype.UUID, target string) (err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if tx == nil {
		return DeliveryCompletionError{503, "delivery_completion_unavailable"}
	}
	q := db.New(tx)
	if err = q.LockIssueStatusCatalogShared(ctx, workspace); err != nil {
		return err
	}
	status, err := issuestatus.Resolve(ctx, q, workspace, target)
	if err != nil {
		return err
	}
	_, policy, err := LockDeliveryProjectPolicy(ctx, tx, workspace, project)
	if err != nil {
		return err
	}
	if policy.Mode == "enforce" && status.Category == "done" {
		return DeliveryCompletionError{409, "delivery_create_done_forbidden"}
	}
	return nil
}
func CheckDeliveryProjectDeleteInTx(ctx context.Context, tx pgx.Tx, project db.Project) error {
	_, policy, err := LockDeliveryProjectPolicy(ctx, tx, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	if policy.Mode == "enforce" {
		return DeliveryCompletionError{409, "delivery_project_policy_disable_required"}
	}
	return nil
}

func PrepareDeliveryMutationInTx(ctx context.Context, tx pgx.Tx, in DeliveryMutationIntent) (out DeliveryPrepareResult, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if tx == nil || !in.Before.ID.Valid || in.Candidate.ID != in.Before.ID || in.Candidate.WorkspaceID != in.Before.WorkspaceID {
		return out, completionInvalid()
	}
	q := db.New(tx)
	if err = q.LockIssueStatusCatalogShared(ctx, in.Before.WorkspaceID); err != nil {
		return out, err
	}
	current, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: in.Before.ID, WorkspaceID: in.Before.WorkspaceID})
	if err != nil {
		return out, err
	}
	if !reflect.DeepEqual(current, in.Before) {
		return out, completionConflict()
	}
	if in.Completion != nil {
		if err = PrepareArtifactTransaction(ctx, tx); err != nil {
			return out, err
		}
		if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
			return out, err
		}
		if err = in.Completion.Validate(); err != nil {
			return out, err
		}
		id, _ := util.ParseUUID(in.Completion.AuthorizationID)
		row, e := q.LockDeliveryCompletionAuthorization(ctx, id)
		if e != nil {
			return out, e
		}
		request, e := completionStoredRequest(row)
		if e != nil {
			return out, e
		}
		if row.WorkspaceID != current.WorkspaceID || row.IssueID != current.ID || util.UUIDToString(row.OperationID) != in.Completion.OperationID || row.IssueRevision != in.Completion.ExpectedIssueRevision {
			return out, completionConflict()
		}
		if e = completionActualConsumer(ctx, tx, row, in.Credential, current, in.Webhook, false); e != nil {
			return out, e
		}
		digest, e := completionMutationDigest(request, in)
		if e != nil {
			return out, e
		}
		terminal, e := q.GetDeliveryCompletionTerminal(ctx, row.ID)
		if e == nil {
			if terminal.Kind != "consumed" || terminal.OperationID != row.OperationID || terminal.MutationDigest.String != digest {
				return out, completionConflict()
			}
			receipt, e := completionReceipt(row, terminal, true)
			if e != nil {
				return out, e
			}
			return DeliveryPrepareResult{State: DeliveryReplayed, Receipt: &receipt}, nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
			return out, err
		}
		if completionIsCollaboration(row.TargetKind) {
			if in.Collaboration == nil || !completionTargetEqual(request.Target, *in.Collaboration) {
				return out, completionConflict()
			}
			if e = completionRecheckCollaborationIntent(ctx, tx, row, request, current); e != nil {
				return out, e
			}
		} else if row.TargetKind != "ordinary" || in.Collaboration != nil {
			return out, completionUnsupported()
		}
		if !completionContentUnchanged(in.Before, in.Candidate) || in.AuxiliaryChanges {
			return out, DeliveryCompletionError{409, "delivery_completion_content_changed"}
		}
		oldCategory, e := IssueStatusCategory(ctx, q, current)
		if e != nil {
			return out, e
		}
		status, e := issuestatus.Resolve(ctx, q, current.WorkspaceID, in.Candidate.Status)
		if e != nil {
			return out, e
		}
		if oldCategory == "done" || status.Category != "done" || status.Key != row.TargetStatus || current.Status != row.ExpectedStatus || current.Revision != row.IssueRevision {
			return out, completionConflict()
		}
		project, policy, e := LockDeliveryProjectPolicy(ctx, tx, current.WorkspaceID, current.ProjectID)
		if e != nil {
			return out, e
		}
		if policy.Mode != "enforce" || policy.Revision != row.PolicyRevision || project.ID != row.ProjectID || project.Revision != row.ProjectRevision {
			return out, completionConflict()
		}
		if e = completionActualConsumer(ctx, tx, row, in.Credential, current, in.Webhook, true); e != nil {
			return out, e
		}
		if e = completionRevalidateEvidence(ctx, tx, row, request, current, in.Metrics); e != nil {
			return out, e
		}
		prepared := &DeliveryPreparedMutation{tx: tx, authorization: row, request: request, credential: in.Credential, webhook: in.Webhook, before: current, candidate: in.Candidate, mutationDigest: digest, metrics: in.Metrics}
		return DeliveryPrepareResult{State: DeliveryPrepared, Prepared: prepared}, nil
	}
	// The writer owns normal permissions. An unchanged persisted custom key may
	// have been archived; only a new requested key needs active-target resolution.
	oldCategory, err := IssueStatusCategory(ctx, q, current)
	if err != nil {
		return out, err
	}
	targetCategory := oldCategory
	if in.Candidate.Status != current.Status {
		status, e := issuestatus.Resolve(ctx, q, current.WorkspaceID, in.Candidate.Status)
		if e != nil {
			return out, e
		}
		targetCategory = status.Category
	}
	if oldCategory == "done" || targetCategory != "done" {
		return DeliveryPrepareResult{State: DeliveryNotApplicable}, nil
	}
	_, policy, err := LockDeliveryProjectPolicy(ctx, tx, current.WorkspaceID, current.ProjectID)
	if err != nil {
		return out, err
	}
	if policy.Mode == "enforce" {
		return out, DeliveryCompletionError{428, "delivery_completion_authorization_required"}
	}
	return DeliveryPrepareResult{State: DeliveryNotApplicable}, nil
}
func FinishDeliveryMutationInTx(ctx context.Context, tx pgx.Tx, p *DeliveryPreparedMutation, after db.Issue, result DeliveryWriterResult) (out DeliveryCompletionReceipt, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	if p == nil || p.finished || tx == nil || reflect.TypeOf(tx) != reflect.TypeOf(p.tx) || !reflect.TypeOf(tx).Comparable() || p.tx != tx {
		return out, completionConflict()
	}
	row := p.authorization
	if row.TargetKind == "ordinary" {
		if result.CollaborationOperationID.Valid || result.AdminOperationID.Valid || result.SubmissionID.Valid || result.CollaborationRevision != 0 {
			return out, completionInvalid()
		}
	} else if !completionIsCollaboration(row.TargetKind) {
		return out, completionUnsupported()
	}
	if p.webhook == nil && (result.ProviderEventID != "" || result.ProviderEventDigest != "") {
		return out, completionInvalid()
	}
	if p.webhook != nil && (result.ProviderEventID != p.webhook.EventID || result.ProviderEventDigest != p.webhook.EventDigest) {
		return out, completionConflict()
	}
	q := db.New(tx)
	actual, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: after.ID, WorkspaceID: after.WorkspaceID})
	if err != nil {
		return out, err
	}
	if actual.ID != p.before.ID || actual.WorkspaceID != p.before.WorkspaceID || actual.Status != row.TargetStatus || actual.ProjectID != row.ProjectID || actual.Revision <= p.before.Revision || !reflect.DeepEqual(actual, after) || !completionContentUnchanged(p.before, actual) {
		return out, completionConflict()
	}
	if completionIsCollaboration(row.TargetKind) {
		if err = completionCollaborationFinished(ctx, tx, row, actual, result); err != nil {
			return out, err
		}
	}
	if p.request.Target.TargetPosition != nil && actual.Position != *p.request.Target.TargetPosition {
		return out, completionConflict()
	}
	if err = completionActualConsumer(ctx, tx, row, p.credential, actual, p.webhook, true); err != nil {
		return out, err
	}
	project, policy, e := LockDeliveryProjectPolicy(ctx, tx, actual.WorkspaceID, actual.ProjectID)
	if e != nil {
		return out, e
	}
	if project.Revision != row.ProjectRevision || policy.Mode != "enforce" || policy.Revision != row.PolicyRevision {
		return out, completionConflict()
	}
	if err = completionRevalidateEvidence(ctx, tx, row, p.request, actual, p.metrics); err != nil {
		return out, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	if !now.Time.Before(row.ExpiresAt.Time) || !now.Time.Before(row.EvidenceExpiresAt.Time) {
		return out, completionConflict()
	}
	// Evidence reads can take time. Check the actual consumer again immediately
	// before the SQL expiry fence; the retained issuer remains a distinct subject.
	if err = completionActualConsumer(ctx, tx, row, p.credential, actual, p.webhook, true); err != nil {
		return out, err
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, completionIssuerCredential(row)); err != nil {
		return out, err
	}
	params := db.CreateDeliveryCompletionTerminalParams{AuthorizationID: row.ID, Kind: "consumed", OperationID: row.OperationID, MutationDigest: completionText(p.mutationDigest), IssueRevisionBefore: completionInt(row.IssueRevision), IssueRevisionAfter: completionInt(actual.Revision), StatusAfter: completionText(actual.Status), CollaborationOperationID: result.CollaborationOperationID, AdminOperationID: result.AdminOperationID, ResultSubmissionID: result.SubmissionID}
	if result.CollaborationRevision > 0 {
		params.ResultCollaborationRevision = completionInt(result.CollaborationRevision)
	}
	if p.webhook != nil {
		params.ActorKind = p.webhook.Kind
		params.IntegrationID = p.webhook.IntegrationID
		params.ProviderEventID = completionText(result.ProviderEventID)
		params.ProviderEventDigest = completionText(result.ProviderEventDigest)
	} else {
		kind, actor, e := p.credential.Actor()
		if e != nil {
			return out, e
		}
		params.ActorKind = kind
		params.ActorUserID = completionUUIDPG(p.credential.Principal.UserID)
		if kind == "agent" {
			params.ActorKind = "task"
			params.ActorAgentID = completionUUIDPG(actor)
		}
		if row.ConsumerKind == "admin" {
			params.ActorKind = "admin"
		}
	}
	if p.webhook == nil {
		params.ConsumerExpiresAt, err = completionConsumerExpiry(ctx, tx, p.credential)
		if err != nil {
			return out, err
		}
	}
	terminal, err := q.CreateDeliveryCompletionTerminal(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, completionConflict()
	}
	if err != nil {
		return out, err
	}
	p.finished = true
	return completionReceipt(row, terminal, false)
}
func completionContentUnchanged(a, b db.Issue) bool {
	b.Status = a.Status
	b.Position = a.Position
	b.Revision = a.Revision
	b.UpdatedAt = a.UpdatedAt
	b.LastActivityAt = a.LastActivityAt
	return reflect.DeepEqual(a, b)
}
func completionMutationDigest(req DeliveryCompletionAuthorizationRequest, in DeliveryMutationIntent) (string, error) {
	if in.Candidate.Status != req.Target.TargetStatus || !completionSamePosition(req.Target.TargetPosition, in.RequestedPosition) || in.AuxiliaryChanges || !completionContentUnchanged(in.Before, in.Candidate) {
		return "", completionConflict()
	}
	if completionIsCollaboration(req.Target.Kind) {
		if in.Collaboration == nil || !completionTargetEqual(req.Target, *in.Collaboration) {
			return "", completionConflict()
		}
		raw, err := json.Marshal(struct {
			Operation string
			Target    DeliveryCompletionTarget
		}{req.OperationID, *in.Collaboration})
		if err != nil {
			return "", err
		}
		return completionDigest("collaboration_mutation", raw), nil
	}
	if in.Collaboration != nil {
		return "", completionUnsupported()
	}
	raw, err := json.Marshal(struct {
		Operation string   `json:"operation"`
		Issue     string   `json:"issue"`
		Revision  int64    `json:"revision"`
		Status    string   `json:"status"`
		Position  *float64 `json:"position"`
	}{req.OperationID, req.Target.IssueID, req.Target.ExpectedIssueRevision, req.Target.TargetStatus, req.Target.TargetPosition})
	if err != nil {
		return "", err
	}
	return completionDigest("mutation", raw), nil
}
func completionSamePosition(a, b *float64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
func completionText(v string) pgtype.Text { return pgtype.Text{String: v, Valid: true} }
func completionInt(v int64) pgtype.Int8   { return pgtype.Int8{Int64: v, Valid: true} }
func completionTime(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v.UTC(), Valid: true}
}
func completionUUIDPG(v string) pgtype.UUID { id, _ := util.ParseUUID(v); return id }
func completionOrdinary(ctx context.Context, q *db.Queries, issue db.Issue) error {
	if !admission.Formal(issue.AdmissionStatus) {
		return completionDenied()
	}
	shared, err := q.IsCollaborationIssue(ctx, issue.ID)
	if err != nil {
		return err
	}
	if shared {
		return completionUnsupported()
	}
	return nil
}

func completionActualConsumer(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, credential ArtifactCredentialSnapshot, issue db.Issue, proof *DeliveryWebhookProof, newAdmission bool) error {
	if row.ConsumerKind == "github" || row.ConsumerKind == "vcs" {
		if row.TargetKind != "ordinary" {
			return completionDenied()
		}
		return CheckDeliveryWebhookConsumerInTx(ctx, tx, row, issue, proof)
	}
	if proof != nil {
		return completionDenied()
	}
	return completionConsumerAuthority(ctx, tx, row, credential, issue, newAdmission)
}

func completionConsumerExpiry(ctx context.Context, tx pgx.Tx, c ArtifactCredentialSnapshot) (pgtype.Timestamptz, error) {
	switch c.Principal.Kind {
	case auth.CredentialJWT:
		if c.Principal.ExpiresAt.IsZero() {
			return pgtype.Timestamptz{}, completionDenied()
		}
		return completionTime(c.Principal.ExpiresAt), nil
	case auth.CredentialTask:
		if c.TaskToken == nil || !c.TaskToken.ExpiresAt.Valid {
			return pgtype.Timestamptz{}, completionDenied()
		}
		return c.TaskToken.ExpiresAt, nil
	case auth.CredentialPAT:
		row, err := db.New(tx).LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: c.Principal.PATTokenHash, UserID: completionUUIDPG(c.Principal.UserID)})
		if err != nil {
			return pgtype.Timestamptz{}, err
		}
		return row.ExpiresAt, nil
	default:
		return pgtype.Timestamptz{}, completionDenied()
	}
}
