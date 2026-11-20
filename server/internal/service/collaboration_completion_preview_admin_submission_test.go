package service

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// R38 nomination and source artifact visibility are independently established.
// The synthetic administrator previews an actual pending submission without
// creating an M1 grant or performing an administrative lifecycle operation.
func TestCollaborationCompletionPreviewAdminCurrentSubmission(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("admin submission preview requires an explicitly admitted task-owned DATABASE_URL")
	}
	t.Run("admin_available", func(t *testing.T) {
		f := newStageCompletionFixture(t, stageCompletionIndependentArtifact, stageCompletionIndependentProductRole)
		stageCompletionM1Policy(t, f, "enforce")
		submission := stageCompletionSubmit(t, f)
		admin := stageCompletionAdmin(t, f)
		f.fx.Member(t, f.workspace, util.UUIDToString(admin.admin), "admin")
		stageCompletionDisableCreator(t, f, admin)
		params := admin.params(t, "complete")
		credential := completionRequestCredential(admin.adminCtx)
		if credential.Principal.Kind != auth.CredentialJWT || credential.Principal.UserID != util.UUIDToString(admin.admin) || credential.PasswordSession == nil || credential.PasswordSession.UserID != credential.Principal.UserID || credential.PasswordSession.Kind != "jwt" {
			t.Fatal("R38 control requires the actual administrator JWT/password session")
		}
		for _, table := range []string{"delivery_completion_authorization", "delivery_completion_terminal", "delivery_evidence_evaluation"} {
			if count := f.fx.Count(t, `SELECT count(*) FROM "`+table+`"`); count != 0 {
				t.Fatalf("R38 preview setup created durable %s rows: count=%d", table, count)
			}
		}
		completionCandidatesNoWrites(t, f.stage)
		adminBefore := adminCollaborationStateSnapshot(t, admin)
		adapterState := func() []byte {
			f.source.mu.Lock()
			defer f.source.mu.Unlock()
			state := struct {
				Calls  int                      `json:"calls"`
				Target StageExecutionTarget     `json:"target"`
				Last   StageExecutionInvocation `json:"last"`
			}{f.source.calls, f.source.target, f.source.last}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
		adapterBefore := adapterState()
		t.Cleanup(func() {
			adminAfter := adminCollaborationStateSnapshot(t, admin)
			if adminBefore != adminAfter {
				t.Errorf("R38 preview changed original admin/audit/control rows: before_sha256=%s after_sha256=%s", hexHash([]byte(adminBefore)), hexHash([]byte(adminAfter)))
			}
			adapterAfter := adapterState()
			if !reflect.DeepEqual(adapterBefore, adapterAfter) {
				t.Errorf("R38 preview changed controlled call/target/invocation state: before_sha256=%s after_sha256=%s", hexHash(adapterBefore), hexHash(adapterAfter))
			}
		})

		// Obtain current nomination and delivery through a real borrowed read
		// transaction. Source membership never substitutes for R38 nomination.
		prepare := beginCompletionCandidatesRead(t, f.stage, admin.adminCtx)
		if err := PrepareArtifactTransaction(prepare.ctx, prepare.tx); err != nil {
			t.Fatal(err)
		}
		queries := db.New(prepare.tx)
		issue, err := queries.GetIssue(prepare.ctx, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		access, err := completionAdminNomination(prepare.ctx, prepare.tx, issue, admin.admin, admin.org, true)
		if err != nil {
			completionCandidatesLog(t, "admin_target_nomination", prepare, err)
			t.Fatalf("actual disabled-creator nomination unavailable: %v", err)
		}
		delivery, err := collaborationDeliverySummary(prepare.ctx, queries, access)
		if err != nil || delivery.Current == nil || delivery.Current.ID != submission || delivery.Current.EvidenceState != "available" {
			t.Fatalf("actual R38 nomination did not retain the genuine current submission: current=%+v err=%v", delivery.Current, err)
		}
		bound, err := queries.GetCollaborationStageSubmission(prepare.ctx, db.GetCollaborationStageSubmissionParams{IssueID: issue.ID, SubmissionID: submission})
		if err != nil || bound.SubmissionID != submission || bound.IssueID != issue.ID || bound.WorkspaceID != issue.WorkspaceID || bound.LifecycleEpoch != access.LifecycleEpoch || bound.CandidateID != f.candidate.CandidateID || bound.CandidateVersion != f.candidate.Version || !bound.OperationID.Valid || bound.ActorUserID != util.MustParseUUID(f.handler) {
			t.Fatalf("actual R38 delivery lost its immutable verification binding: bound=%+v err=%v", bound, err)
		}
		writer := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{access: access}}
		request := CollaborationCompletionPreviewRequest{SourceIssueID: f.stage.child, ApprovalID: util.UUIDToString(f.stage.artifact.ApprovalID), Target: completionAdminTarget(writer, params, delivery.Current)}
		if request.Target.Validate() != nil || request.Target.Kind != "collaboration_admin" || request.Target.SubmissionID == nil || *request.Target.SubmissionID != util.UUIDToString(submission) || request.Target.IssueID != util.UUIDToString(issue.ID) || request.Target.ExpectedIssueRevision != issue.Revision || request.Target.ExpectedLifecycleEpoch != stageVersion(access.LifecycleEpoch) || request.Target.ExpectedCollaborationRevision != stageVersion(access.Revision) || issue.Status != "in_review" {
			t.Fatalf("actual R38 target does not name the fresh current submission: %+v", request.Target)
		}
		if err = prepare.finalCredential(); err == nil {
			err = prepare.tx.Commit(prepare.ctx)
		}
		completionCandidatesLog(t, "admin_target_preparation", prepare, err)
		if err != nil {
			prepare.rollback(t)
			t.Fatalf("actual R38 target read did not commit: %v", err)
		}

		read := beginCompletionCandidatesRead(t, f.stage, admin.adminCtx)
		out, err := f.svc.GetCollaborationCompletionPreviewInTx(read.ctx, read.tx, issue.ID, admin.org, completionRequestCredential(read.ctx), request)
		if err == nil {
			err = deliveryCompletionError(read.finalCredential())
		}
		if err == nil {
			err = deliveryCompletionError(read.tx.Commit(read.ctx))
		}
		if err != nil {
			read.rollback(t)
		}
		completionCandidatesLog(t, t.Name(), read, err)
		witness := struct {
			Reason       string                      `json:"reason"`
			Blockers     []CollaborationStageBlocker `json:"stage_blockers"`
			Target       DeliveryCompletionTarget    `json:"target"`
			Source       string                      `json:"source_issue_id"`
			Approval     string                      `json:"approval_id"`
			Admin        string                      `json:"admin_user_id"`
			Organization string                      `json:"organization_id"`
			CheckedAt    time.Time                   `json:"checked_at"`
			MaxExpires   *time.Time                  `json:"max_expires_at"`
		}{out.Reason, out.StageBlockers, request.Target, request.SourceIssueID, request.ApprovalID, credential.Principal.UserID, util.UUIDToString(admin.org), out.CheckedAt, out.MaxExpiresAt}
		raw, marshalErr := json.Marshal(witness)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		t.Logf("M1_ADMIN_SUBMISSION_PREVIEW_WITNESS %s", raw)
		if err != nil {
			t.Fatalf("genuine R38 current-submission preview refused: %v", err)
		}
		completionPreviewSubmissionEnvelope(t, f, request, out, "enforce")
		if out.Reason != "authorization_available" || out.AuthorizationBinding == nil || out.Observation == nil || out.MaxExpiresAt == nil || out.ProjectRevision == nil {
			t.Fatalf("genuine R38 submission has no exact authorization preview: reason=%s blockers=%+v", out.Reason, out.StageBlockers)
		}
		child, err := f.svc.Queries.GetIssue(admin.adminCtx, util.MustParseUUID(f.stage.child))
		if err != nil {
			t.Fatal(err)
		}
		artifact, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(admin.adminCtx, f.stage.artifact.ApprovalID)
		if err != nil {
			t.Fatal(err)
		}
		want := DeliveryCompletionPreviewBinding{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: f.workspace, Target: request.Target,
			Project:  DeliveryCompletionProject{ID: f.stage.project, ExpectedRevision: *out.ProjectRevision, ExpectedPolicyRevision: 1},
			Evidence: DeliveryCompletionEvidence{IssueID: f.stage.child, ExpectedIssueRevision: child.Revision, ApprovalID: request.ApprovalID, ExpectedSourceRevision: artifact.SourceRevision, ExpectedSpecDigest: artifact.SpecDigest, ExpectedArtifactDigest: artifact.OutputArtifactDigest},
			Consumer: DeliveryCompletionConsumer{Kind: "admin", UserID: util.UUIDToString(admin.admin), OrganizationID: util.UUIDToString(admin.org)}}
		if !reflect.DeepEqual(*out.AuthorizationBinding, want) {
			t.Errorf("R38 preview changed exact admin/org/root/submission/source binding: got=%+v want=%+v", *out.AuthorizationBinding, want)
		}
		observation := out.Observation
		if observation.Verdict != "pass" || observation.IssueID != util.UUIDToString(child.ID) || observation.IssueRevision != child.Revision || observation.ApprovalID != request.ApprovalID || observation.ExecutionID != util.UUIDToString(artifact.ExecutionID) || observation.SourceRevision != artifact.SourceRevision || observation.ObservedHeadSHA != f.stage.head || observation.SpecDigest != artifact.SpecDigest || observation.ArtifactDigest != artifact.OutputArtifactDigest || observation.ReceiptDigest != artifact.ReceiptDigest || observation.ValidUntil == nil || observation.ObservedAt.IsZero() || observation.ObservedAt.After(out.CheckedAt) {
			t.Fatalf("R38 preview lost the exact current artifact observation: %+v", observation)
		}
		release, err := f.svc.Queries.GetCollaborationStageReviewHead(admin.adminCtx, db.GetCollaborationStageReviewHeadParams{IssueID: issue.ID, CandidateID: f.candidate.CandidateID, Step: "release"})
		if err != nil || !release.ExpiresAt.Valid || release.DecisionID != bound.ReleaseDecisionID {
			t.Fatalf("R38 preview's original release approval unavailable: %v", err)
		}
		until := release.ExpiresAt.Time
		for _, horizon := range []time.Time{*observation.ValidUntil, credential.Principal.ExpiresAt} {
			if horizon.Before(until) {
				until = horizon
			}
		}
		if !out.MaxExpiresAt.Equal(until) || !out.CheckedAt.Before(until) || out.MaxExpiresAt.Location() != time.UTC {
			t.Errorf("R38 preview renewed or lost saved evidence/credential/release expiry: got=%v want=%v checked=%v", out.MaxExpiresAt, until, out.CheckedAt)
		}
	})
}
