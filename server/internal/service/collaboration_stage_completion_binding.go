package service

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (guard *stageMutationGuard) prepareBinding(ctx context.Context, in DeliveryMutationIntent) error {
	if !reflect.DeepEqual(guard.credential, completionRequestCredential(ctx)) {
		return completionDenied()
	}
	kind, _, err := guard.credential.Actor()
	if err != nil || kind != "human" {
		return completionDenied()
	}
	if err = CheckArtifactCredentialInTx(ctx, guard.tx, guard.credential); err != nil {
		return err
	}
	actor := util.MustParseUUID(guard.credential.Principal.UserID)
	queries := db.New(guard.tx)
	if guard.target.IssueID != util.UUIDToString(guard.before.ID) || !completionIsCollaboration(guard.target.Kind) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if guard.target.Kind == "collaboration_admin" {
		if guard.submit || !in.StageAdminOperationID.Valid || !in.StageAdminOrganizationID.Valid {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		guard.access, err = completionAdminNomination(ctx, guard.tx, guard.before, actor, in.StageAdminOrganizationID, true)
		if err != nil {
			return err
		}
		admin, err := queries.LockCollaborationStageAdminOperation(ctx, db.LockCollaborationStageAdminOperationParams{ID: in.StageAdminOperationID, OrganizationID: in.StageAdminOrganizationID, ActorID: actor, TargetID: guard.before.ID, WorkspaceID: guard.before.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		if err != nil {
			return err
		}
		if admin.Kind != "collaboration.complete" || admin.ActorKind != "user" || admin.State != "applied" || admin.ResultCode != adminCollaborationCode("complete") || guard.credential.PasswordSession == nil || admin.ActorAuthVersion != guard.credential.PasswordSession.Version {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		guard.admin = &admin
	} else {
		if in.StageAdminOperationID.Valid || in.StageAdminOrganizationID.Valid {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		guard.access, err = completionCollaborationAccess(ctx, guard.tx, guard.before, actor)
		if err != nil {
			return err
		}
		if guard.submit {
			if guard.target.Kind != "collaboration_submission" || guard.before.AssigneeType.String != "member" || guard.before.AssigneeID != actor || !guard.access.Capabilities.ControlAI || !collaboration.NonterminalCategory(guard.access.StatusCategory) || guard.access.QueueState != "active" && guard.access.QueueState != "paused" {
				return completionDenied()
			}
		} else if !guard.access.Capabilities.Complete {
			return completionDenied()
		}
	}
	guard.operation, err = queries.LockCollaborationStageOperation(ctx, db.LockCollaborationStageOperationParams{ID: in.StageOperationID, ScopeID: guard.before.ID, WorkspaceID: guard.before.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err != nil {
		return err
	}
	expectedKind := "complete"
	if guard.submit {
		expectedKind = "submit_acceptance"
	} else if guard.admin != nil {
		expectedKind = "admin_complete"
	}
	if guard.operation.ActorUserID != actor || guard.operation.Kind != expectedKind || guard.operation.CompletedAt.Valid || guard.operation.ResultCode.Valid || len(guard.operation.Result) != 0 || guard.operation.ResolvedIssueID.Valid {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if guard.admin != nil && (guard.operation.RequestID != util.UUIDToString(guard.admin.IdempotencyKey) || guard.operation.PayloadHash != guard.admin.PayloadHash) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if !guard.submit && guard.target.Validate() != nil {
		return completionInvalid()
	}
	if guard.target.ExpectedIssueRevision != guard.before.Revision || guard.target.ExpectedStatus != guard.before.Status || guard.target.TargetStatus != guard.targetStatus || guard.target.ExpectedCollaborationRevision != stageVersion(guard.access.Revision) || guard.target.ExpectedLifecycleEpoch != stageVersion(guard.access.LifecycleEpoch) {
		return completionConflict()
	}
	if guard.admin == nil && (guard.target.ExpectedParticipantEpoch != stageVersion(guard.access.ParticipantEpoch.Int64) || guard.target.ExpectedAccountEpoch != stageVersion(guard.access.AccountEpoch.Int64)) {
		return completionConflict()
	}
	return nil
}

func (guard *stageMutationGuard) checkSubmissionOperation(ctx context.Context, row db.CollaborationStageSubmission) error {
	queries := db.New(guard.tx)
	operation, err := queries.LockCollaborationStageOperation(ctx, db.LockCollaborationStageOperationParams{ID: row.OperationID, ScopeID: guard.before.ID, WorkspaceID: guard.before.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err != nil {
		return err
	}
	expectedKind, expectedCode := "submit_acceptance", "acceptance_submitted"
	if !guard.submit && guard.target.Kind == "collaboration_self" {
		expectedKind, expectedCode = "complete", "completed"
	}
	if operation.ActorUserID != row.ActorUserID || operation.Kind != expectedKind || operation.ResolvedIssueID != guard.before.ID || operation.ResultCode.String != expectedCode || (guard.submit || guard.target.Kind == "collaboration_self") && operation.ID != guard.operation.ID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	receipt, err := decodeWorkflowReceipt(operation)
	if err != nil || receipt.Status != 200 || receipt.DeliveryConfirmation == nil || receipt.DeliveryConfirmation.SubmissionID != row.SubmissionID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	handoff, err := queries.LockCollaborationStageSubmissionHandoff(ctx, db.LockCollaborationStageSubmissionHandoffParams{IssueID: guard.before.ID, ID: row.SubmissionID, WorkspaceID: guard.before.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err != nil {
		return err
	}
	if handoff.ActorUserID != row.ActorUserID || handoff.FromUserID != row.ActorUserID || handoff.ToUserID != guard.before.CreatorID || handoff.Purpose != "acceptance" {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	return nil
}

func (guard *stageMutationGuard) checkFinalBinding(ctx context.Context, after db.Issue, result DeliveryWriterResult) error {
	if !reflect.DeepEqual(guard.credential, completionRequestCredential(ctx)) || result.CollaborationOperationID != guard.operation.ID || len(guard.operationResult) == 0 || result != guard.writerResult {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	actor := util.MustParseUUID(guard.credential.Principal.UserID)
	var access CollaborationIssueAccess
	var err error
	queries := db.New(guard.tx)
	if guard.admin != nil {
		if result.AdminOperationID != guard.admin.ID {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		access, err = completionAdminNomination(ctx, guard.tx, after, actor, guard.admin.OrganizationID, true)
	} else {
		if result.AdminOperationID.Valid {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		access, err = completionCollaborationAccess(ctx, guard.tx, after, actor)
	}
	if err != nil {
		return err
	}
	if access.ParticipantEpoch != guard.access.ParticipantEpoch || access.AccountEpoch != guard.access.AccountEpoch || access.SessionID != guard.access.SessionID || access.Revision != result.CollaborationRevision || !reflect.DeepEqual(access.Issue, after) {
		return stageCompletionRefusal("verification", "stage_evidence_changed")
	}
	if access.InformationRequestID.Valid || guard.submit && (access.QueueState != guard.access.QueueState || access.ControlVersion != guard.access.ControlVersion) || !guard.submit && (access.QueueState != "closed" || access.ControlVersion != guard.access.ControlVersion+1) {
		return stageCompletionRefusal("verification", "stage_evidence_changed")
	}
	operation, err := queries.LockCollaborationStageOperation(ctx, db.LockCollaborationStageOperationParams{ID: guard.operation.ID, ScopeID: guard.before.ID, WorkspaceID: guard.before.WorkspaceID})
	if err != nil {
		return err
	}
	header := operation
	header.ResultCode, header.Result, header.ResolvedIssueID, header.CompletedAt = guard.operation.ResultCode, guard.operation.Result, guard.operation.ResolvedIssueID, guard.operation.CompletedAt
	if !reflect.DeepEqual(header, guard.operation) || !operation.CompletedAt.Valid || operation.ResolvedIssueID != after.ID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	var sameResult bool
	if err = guard.tx.QueryRow(ctx, `SELECT $1::jsonb=$2::jsonb`, operation.Result, guard.operationResult).Scan(&sameResult); err != nil {
		return err
	}
	if !sameResult {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if guard.admin != nil {
		admin, err := queries.LockCollaborationStageAdminOperation(ctx, db.LockCollaborationStageAdminOperationParams{ID: guard.admin.ID, OrganizationID: guard.admin.OrganizationID, ActorID: actor, TargetID: after.ID, WorkspaceID: after.WorkspaceID})
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(admin, *guard.admin) {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		receipt, err := loadAdminCollaborationReceipt(ctx, queries, admin)
		if err != nil || receipt.CollaborationRevision != access.Revision || receipt.Status != after.Status || receipt.ControlVersion != access.ControlVersion {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		return nil
	}
	if result.SubmissionID != guard.boundSubmission {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err = guard.checkSubmission(ctx, result.SubmissionID); err != nil {
		return err
	}
	receipt, err := decodeWorkflowReceipt(operation)
	expectedCode := "completed"
	if guard.submit {
		expectedCode = "acceptance_submitted"
	}
	if err != nil || receipt.Status != 200 || receipt.Code != expectedCode || receipt.CollaborationRevision != access.Revision || receipt.QueueState != access.QueueState || receipt.ControlVersion != access.ControlVersion || receipt.DeliveryConfirmation == nil || receipt.DeliveryConfirmation.SubmissionID != result.SubmissionID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	delivery, err := collaborationDeliverySummary(ctx, queries, access)
	if err != nil || delivery.Latest == nil || delivery.Latest.ID != result.SubmissionID || delivery.Latest.EvidenceState != "available" || !reflect.DeepEqual(delivery.Latest.AttachmentIDs, receipt.DeliveryConfirmation.AttachmentIDs) {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if guard.submit {
		if delivery.Current == nil || delivery.Current.ID != result.SubmissionID {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
	} else if delivery.Latest.Decision == nil || delivery.Latest.Decision.Kind != "accepted" || delivery.Latest.Decision.ActorUserID != actor || delivery.Latest.Decision.SubmissionID != result.SubmissionID {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	latest := delivery.Latest
	if guard.submit || guard.target.Kind == "collaboration_self" {
		got := make([]string, 0, len(latest.AttachmentIDs))
		for _, id := range latest.AttachmentIDs {
			got = append(got, util.UUIDToString(id))
		}
		want := slices.Clone(guard.deliveryTarget.AttachmentIDs)
		slices.Sort(got)
		slices.Sort(want)
		if latest.Result == nil || *latest.Result != guard.deliveryTarget.Result || latest.Basis == nil || *latest.Basis != guard.deliveryTarget.Basis || !slices.Equal(got, want) || (guard.deliveryTarget.SourceCommentID != nil) != latest.SourceCommentID.Valid || guard.deliveryTarget.SourceCommentID != nil && util.MustParseUUID(*guard.deliveryTarget.SourceCommentID) != latest.SourceCommentID {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		latest = nil
	}
	if !guard.submit {
		var reason *string
		if guard.target.Kind == "collaboration_submission" && guard.target.AcceptanceNote != "" {
			reason = &guard.target.AcceptanceNote
		}
		if !reflect.DeepEqual(delivery.Latest.Decision.Reason, reason) {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
	}
	digest, err := completionIntentDigest(ctx, guard.tx, guard.deliveryTarget, latest)
	if err != nil {
		return err
	}
	if digest != guard.deliveryDigest {
		return stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	return nil
}

func (guard *stageMutationGuard) freezeDelivery(ctx context.Context, input *collaborationDeliveryInput) error {
	if guard.target.Kind == "collaboration_admin" {
		return nil
	}
	guard.deliveryTarget = guard.target
	var current *CollaborationSubmission
	if guard.submit {
		if input == nil {
			return completionConflict()
		}
		guard.deliveryTarget.Kind = "collaboration_self"
		guard.deliveryTarget.Result, guard.deliveryTarget.Basis = input.note, input.basis
		guard.deliveryTarget.AttachmentIDs = make([]string, 0, len(input.attachments))
		for _, id := range input.attachments {
			guard.deliveryTarget.AttachmentIDs = append(guard.deliveryTarget.AttachmentIDs, util.UUIDToString(id))
		}
		if input.draft != nil {
			id := util.UUIDToString(*input.draft)
			guard.deliveryTarget.DraftID = &id
		}
		if input.sourceComment != nil {
			id := util.UUIDToString(*input.sourceComment)
			guard.deliveryTarget.SourceCommentID = &id
		}
	} else if guard.target.Kind == "collaboration_submission" {
		delivery, err := collaborationDeliverySummary(ctx, db.New(guard.tx), guard.access)
		if err != nil || delivery.Current == nil || delivery.Current.ID != guard.boundSubmission {
			return stageCompletionRefusal("verification", "stage_submission_unbound")
		}
		current = delivery.Current
	}
	var err error
	guard.deliveryDigest, err = completionIntentDigest(ctx, guard.tx, guard.deliveryTarget, current)
	return err
}

// Capture the original server-produced receipt before its database write. This
// is transaction-local data, never a client flag or a durable authorization.
func (guard *stageMutationGuard) freezeResult(tx pgx.Tx, result DeliveryWriterResult, encoded []byte) error {
	if tx == nil || reflect.TypeOf(tx) != reflect.TypeOf(guard.tx) || !reflect.TypeOf(tx).Comparable() || guard.tx != tx || guard.finished || len(guard.operationResult) != 0 || len(encoded) == 0 || result.CollaborationOperationID != guard.operation.ID {
		return completionConflict()
	}
	guard.operationResult, guard.writerResult = slices.Clone(encoded), result
	return nil
}

func (guard *stageMutationGuard) checkFinalM1(ctx context.Context, after db.Issue, result DeliveryWriterResult) (time.Time, error) {
	prepared := guard.delivery
	if prepared == nil {
		return time.Time{}, nil
	}
	queries := db.New(guard.tx)
	authorization, err := queries.LockDeliveryCompletionAuthorization(ctx, prepared.authorization.ID)
	if err != nil {
		return time.Time{}, err
	}
	if !reflect.DeepEqual(authorization, prepared.authorization) || !prepared.finished || !reflect.DeepEqual(prepared.credential, guard.credential) || prepared.webhook != nil {
		return time.Time{}, stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	terminal, err := queries.LockCollaborationStageDeliveryTerminal(ctx, authorization.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if err != nil {
		return time.Time{}, err
	}
	if terminal.Kind != "consumed" || terminal.OperationID != authorization.OperationID || terminal.MutationDigest.String != prepared.mutationDigest || terminal.IssueRevisionBefore.Int64 != guard.before.Revision || terminal.IssueRevisionAfter.Int64 != after.Revision || terminal.StatusAfter.String != after.Status || terminal.CollaborationOperationID != result.CollaborationOperationID || terminal.AdminOperationID != result.AdminOperationID || terminal.ResultSubmissionID != result.SubmissionID || terminal.ResultCollaborationRevision.Int64 != result.CollaborationRevision || terminal.ActorUserID != util.MustParseUUID(guard.credential.Principal.UserID) {
		return time.Time{}, stageCompletionRefusal("verification", "stage_submission_unbound")
	}
	if _, err = completionReceipt(authorization, terminal, false); err != nil {
		return time.Time{}, err
	}
	if err = completionActualConsumer(ctx, guard.tx, authorization, prepared.credential, after, nil, true); err != nil {
		return time.Time{}, err
	}
	project, policy, err := LockDeliveryProjectPolicy(ctx, guard.tx, after.WorkspaceID, after.ProjectID)
	if err != nil {
		return time.Time{}, err
	}
	if project.Revision != authorization.ProjectRevision || policy.Mode != "enforce" || policy.Revision != authorization.PolicyRevision {
		return time.Time{}, stageCompletionRefusal("verification", "stage_evidence_changed")
	}
	if err = completionRevalidateEvidence(ctx, guard.tx, authorization, prepared.request, after, prepared.metrics); err != nil {
		return time.Time{}, err
	}
	if err = completionCollaborationFinished(ctx, guard.tx, authorization, after, result); err != nil {
		return time.Time{}, err
	}
	expires := stageEarlier(authorization.ExpiresAt.Time, authorization.EvidenceExpiresAt.Time)
	issuer := completionIssuerCredential(authorization)
	issuerExpiry, err := completionConsumerExpiry(ctx, guard.tx, issuer)
	if err != nil {
		return time.Time{}, err
	}
	if issuer.Principal.Kind == auth.CredentialPAT {
		pat, err := queries.LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: issuer.Principal.PATTokenHash, UserID: authorization.IssuerUserID})
		if err != nil {
			return time.Time{}, err
		}
		if pat.ID != authorization.IssuerPatID {
			return time.Time{}, stageCompletionRefusal("verification", "stage_signer_changed")
		}
	}
	for _, horizon := range []pgtype.Timestamptz{authorization.IssuerCredentialExpiresAt, issuerExpiry} {
		if horizon.Valid {
			expires = stageEarlier(expires, horizon.Time)
		}
	}
	return expires, nil
}
