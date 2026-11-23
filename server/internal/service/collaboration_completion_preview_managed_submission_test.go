package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The real managed run, controlled deployment, human verification advance and
// SubmitAcceptance precede each preview transaction. These synthetic fixtures
// establish the preview contract, not historical HTTP holders or S4 execution.
func TestCollaborationCompletionPreviewManagedSubmission(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("submission preview regressions require an explicitly admitted task-owned DATABASE_URL")
	}
	t.Run("enforce_authorization_available", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "enforce")
		out, err := completionPreviewSubmissionRead(t, f, f.ctx, request)
		if err != nil {
			t.Fatalf("genuine verified submission preview refused: %v", err)
		}
		completionPreviewSubmissionEnvelope(t, f, request, out, "enforce")
		if out.Reason != "authorization_available" || out.AuthorizationBinding == nil || out.Observation == nil || out.MaxExpiresAt == nil || out.ProjectRevision == nil {
			t.Fatalf("genuine verified submission has no exact authorization preview: reason=%s blockers=%+v", out.Reason, out.StageBlockers)
		}
		child, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(f.stage.child))
		if err != nil {
			t.Fatal(err)
		}
		artifact, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(f.ctx, f.stage.artifact.ApprovalID)
		if err != nil {
			t.Fatal(err)
		}
		want := DeliveryCompletionPreviewBinding{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: f.workspace, Target: request.Target,
			Project:  DeliveryCompletionProject{ID: f.stage.project, ExpectedRevision: *out.ProjectRevision, ExpectedPolicyRevision: 1},
			Evidence: DeliveryCompletionEvidence{IssueID: f.stage.child, ExpectedIssueRevision: child.Revision, ApprovalID: util.UUIDToString(artifact.ApprovalID), ExpectedSourceRevision: artifact.SourceRevision, ExpectedSpecDigest: artifact.SpecDigest, ExpectedArtifactDigest: artifact.OutputArtifactDigest},
			Consumer: DeliveryCompletionConsumer{Kind: "human", UserID: f.creator}}
		if !reflect.DeepEqual(*out.AuthorizationBinding, want) {
			t.Errorf("preview changed the exact root/submission/child/approval binding: got=%+v want=%+v", *out.AuthorizationBinding, want)
		}
		observation := out.Observation
		if observation.Verdict != "pass" || observation.IssueID != f.stage.child || observation.IssueRevision != child.Revision || observation.ApprovalID != request.ApprovalID || observation.ExecutionID != util.UUIDToString(artifact.ExecutionID) || observation.SourceRevision != artifact.SourceRevision || observation.ObservedHeadSHA != f.stage.head || observation.SpecDigest != artifact.SpecDigest || observation.ArtifactDigest != artifact.OutputArtifactDigest || observation.ReceiptDigest != artifact.ReceiptDigest || observation.ValidUntil == nil || observation.ObservedAt.IsZero() || observation.ObservedAt.After(out.CheckedAt) {
			t.Fatalf("preview lost the current canonical artifact observation: %+v", observation)
		}
		release, err := f.svc.Queries.GetCollaborationStageReviewHead(f.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "release"})
		if err != nil || !release.ExpiresAt.Valid {
			t.Fatalf("original real release approval unavailable: %v", err)
		}
		// The default fixture's one-minute release is its earliest stage approval.
		// Preserve the exact saved horizon; a preview cannot renew that signature.
		until := release.ExpiresAt.Time
		for _, horizon := range []time.Time{*observation.ValidUntil, completionRequestCredential(f.ctx).Principal.ExpiresAt} {
			if horizon.Before(until) {
				until = horizon
			}
		}
		if !out.MaxExpiresAt.Equal(until) || !out.CheckedAt.Before(until) || out.MaxExpiresAt.Location() != time.UTC {
			t.Errorf("preview did not preserve exact evidence/credential/release expiry: got=%v want=%v checked=%v", out.MaxExpiresAt, until, out.CheckedAt)
		}
	})
	t.Run("policy_off_non_authorizing", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "off")
		out, err := completionPreviewSubmissionRead(t, f, f.ctx, request)
		if err != nil {
			t.Fatalf("genuine verified submission with policy off refused: %v", err)
		}
		completionPreviewSubmissionEnvelope(t, f, request, out, "off")
		if out.Reason != "policy_off" || out.AuthorizationBinding != nil || out.Observation != nil || out.MaxExpiresAt != nil {
			t.Errorf("policy off fabricated a submission blocker or authorization: reason=%s blockers=%+v binding=%+v", out.Reason, out.StageBlockers, out.AuthorizationBinding)
		}
	})
	t.Run("stale_target", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "enforce")
		request.Target.ExpectedIssueRevision++
		out, err := completionPreviewSubmissionRead(t, f, f.ctx, request)
		completionPreviewSubmissionDenied(t, out, err, 409, "delivery_completion_conflict")
	})
	t.Run("wrong_actor", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "enforce")
		actor := f.stage.handlerCtx
		access, err := f.svc.ResolveIssueAccess(actor, nil, f.issue.Issue.ID)
		if err != nil || access.SourceMember || !access.ParticipantEpoch.Valid || access.ParticipantEpoch.Int64 < 1 || completionRequestCredential(actor).Principal.UserID != f.handler {
			t.Fatalf("wrong-actor control is not the actual participating external handler: access=%+v err=%v", access, err)
		}
		out, err := completionPreviewSubmissionRead(t, f, actor, request)
		completionPreviewSubmissionDenied(t, out, err, 403, "delivery_completion_denied")
	})
	t.Run("expired_credential", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "enforce")
		credential := completionRequestCredential(f.ctx)
		if credential.Principal.Kind != auth.CredentialJWT || credential.PasswordSession == nil || credential.Principal.UserID != f.creator {
			t.Fatal("expiry control requires the fixture's actual human JWT/password session")
		}
		principal := credential.Principal
		principal.ExpiresAt = time.Now().UTC().Add(-time.Second)
		actor := auth.WithAuthenticatedPrincipal(f.ctx, principal)
		if got := completionRequestCredential(actor); !reflect.DeepEqual(got.PasswordSession, credential.PasswordSession) {
			t.Fatal("expiry control changed the retained real password session")
		}
		out, err := completionPreviewSubmissionRead(t, f, actor, request)
		completionPreviewSubmissionDenied(t, out, err, 403, "delivery_completion_denied")
	})
}

func TestCollaborationCompletionPreviewRetainedEvidenceIsolation(t *testing.T) {
	retained := r29NewConsumedFixture(t)
	requestKey := uuid.NewString()
	retained.fx.InsertNoID(t, "delivery_evidence_evaluation", testutil.Cols{
		"workspace_id": retained.workspace, "evaluator_user_id": retained.creator,
		"request_key": requestKey, "evaluation_id": uuid.NewString(),
		"issue_id": retained.stage.child, "project_id": retained.stage.project,
		"policy_revision": int64(1), "request_digest": strings.Repeat("a", 64),
		"request_json": []byte(`{}`), "observation": []byte(`{}`),
	}, "workspace_id=$1 AND request_key=$2", retained.workspace, requestKey)
	want := map[string]int{"delivery_completion_authorization": 1, "delivery_completion_terminal": 1, "delivery_evidence_evaluation": 1}
	if got := completionPreviewDurableCounts(t, retained.stageExecutionFixture); !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture-owned durable evidence was not detected: got=%v want=%v", got, want)
	}

	// A different workspace's retained grant, terminal and evaluation are valid
	// history. Preview setup must not require clearing them from the database.
	t.Run("managed_submission", func(t *testing.T) {
		f, request := completionPreviewSubmissionFixture(t, "enforce")
		out, err := completionPreviewSubmissionRead(t, f, f.ctx, request)
		if err != nil || out.Reason != "authorization_available" {
			t.Fatalf("unrelated retained evidence prevented preview: reason=%s err=%v", out.Reason, err)
		}
		completionPreviewSubmissionEnvelope(t, f, request, out, "enforce")
	})
	t.Run("concurrent_read_fixture", func(t *testing.T) {
		f := r28NewPreviewFixture(t)
		r28NoWrites(t, f)
		r28Baseline(t, f, f.ctx)
	})
	t.Run("admin_submission", TestCollaborationCompletionPreviewAdminCurrentSubmission)
}

func completionPreviewDurableCounts(t *testing.T, f *stageExecutionFixture) map[string]int {
	t.Helper()
	// Setup must create no evidence for its own workspace. Other tests retain
	// immutable history; the later full snapshots still guard every row against
	// writes by a preview, including those unrelated retained rows.
	return map[string]int{
		"delivery_completion_authorization": f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE workspace_id=$1`, f.workspace),
		"delivery_completion_terminal": f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal t
			JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.workspace_id=$1`, f.workspace),
		"delivery_evidence_evaluation": f.fx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE workspace_id=$1`, f.workspace),
	}
}

func completionPreviewSubmissionFixture(t *testing.T, mode string) (*stageExecutionFixture, CollaborationCompletionPreviewRequest) {
	t.Helper()
	f := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, f, mode)
	submission := stageCompletionSubmit(t, f)
	params := stageCompletionAcceptParams(t, f, submission)
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	writer := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{actor: util.MustParseUUID(f.creator), access: access}}
	request := CollaborationCompletionPreviewRequest{SourceIssueID: f.stage.child, ApprovalID: util.UUIDToString(f.stage.artifact.ApprovalID), Target: completionCollaborationTargetFromWorkflow(writer, params)}
	bound, err := f.svc.Queries.GetCollaborationStageSubmission(f.ctx, db.GetCollaborationStageSubmissionParams{IssueID: f.issue.Issue.ID, SubmissionID: submission})
	if err != nil || request.Target.Validate() != nil || request.Target.Kind != "collaboration_submission" || request.Target.SubmissionID == nil || *request.Target.SubmissionID != util.UUIDToString(submission) || request.Target.ExpectedIssueRevision != access.Issue.Revision || access.Issue.Status != "in_review" || bound.SubmissionID != submission || bound.IssueID != access.Issue.ID || bound.WorkspaceID != access.Issue.WorkspaceID || bound.LifecycleEpoch != access.LifecycleEpoch || bound.CandidateID != f.candidate.CandidateID || bound.CandidateVersion != f.candidate.Version || !bound.OperationID.Valid || bound.ActorUserID != util.MustParseUUID(f.handler) {
		t.Fatalf("setup did not retain the real current verification submission and exact target: bound=%+v target=%+v err=%v", bound, request.Target, err)
	}
	for table, count := range completionPreviewDurableCounts(t, f) {
		if count != 0 {
			t.Fatalf("preview setup minted durable %s rows: count=%d", table, count)
		}
	}
	completionCandidatesNoWrites(t, f.stage)
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
	before := adapterState()
	t.Cleanup(func() {
		after := adapterState()
		if !reflect.DeepEqual(after, before) {
			t.Errorf("preview changed the controlled adapter's actual call/target/invocation state: before_sha256=%s after_sha256=%s", hexHash(before), hexHash(after))
		}
	})
	return f, request
}

func completionPreviewSubmissionRead(t *testing.T, f *stageExecutionFixture, actor context.Context, request CollaborationCompletionPreviewRequest) (CollaborationCompletionPreview, error) {
	t.Helper()
	read := beginCompletionCandidatesRead(t, f.stage, actor)
	out, err := f.svc.GetCollaborationCompletionPreviewInTx(read.ctx, read.tx, f.issue.Issue.ID, pgtype.UUID{}, completionRequestCredential(read.ctx), request)
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
		Reason     string                      `json:"reason"`
		Blockers   []CollaborationStageBlocker `json:"stage_blockers"`
		Target     DeliveryCompletionTarget    `json:"target"`
		Source     string                      `json:"source_issue_id"`
		Approval   string                      `json:"approval_id"`
		CheckedAt  time.Time                   `json:"checked_at"`
		MaxExpires *time.Time                  `json:"max_expires_at"`
	}{out.Reason, out.StageBlockers, request.Target, request.SourceIssueID, request.ApprovalID, out.CheckedAt, out.MaxExpiresAt}
	raw, marshalErr := json.Marshal(witness)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("M1_SUBMISSION_PREVIEW_WITNESS %s", raw)
	return out, err
}

func completionPreviewSubmissionEnvelope(t *testing.T, f *stageExecutionFixture, request CollaborationCompletionPreviewRequest, out CollaborationCompletionPreview, mode string) {
	t.Helper()
	child, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(f.stage.child))
	if err != nil {
		t.Fatal(err)
	}
	project, err := f.svc.Queries.GetProjectInWorkspace(f.ctx, db.GetProjectInWorkspaceParams{ID: child.ProjectID, WorkspaceID: child.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProtocolVersion != 1 || out.IssueID != request.Target.IssueID || out.IssueRevision != request.Target.ExpectedIssueRevision || out.CurrentStatus != request.Target.ExpectedStatus || out.CurrentCategory == "done" || out.TargetStatus != request.Target.TargetStatus || out.TargetCategory != "done" || out.SourceIssueID != f.stage.child || out.SourceIssueRevision != child.Revision || child.ParentIssueID != f.issue.Issue.ID || out.ProjectID == nil || *out.ProjectID != f.stage.project || out.ProjectRevision == nil || *out.ProjectRevision != project.Revision || out.PolicyMode != mode || out.PolicyRevision != 1 || out.CheckedAt.IsZero() || out.CheckedAt.Location() != time.UTC {
		t.Errorf("preview changed the exact current root/source/project/policy envelope: %+v", out)
	}
	if out.StageBlockers == nil || len(out.StageBlockers) != 0 || out.StageBlockersTruncated {
		t.Errorf("genuine verification submission acquired stage blockers: reason=%s blockers=%+v truncated=%v", out.Reason, out.StageBlockers, out.StageBlockersTruncated)
	}
	if out.Enforced || out.UsableForCompletion || out.SkipSupported {
		t.Errorf("read-only preview conferred completion authority: enforced=%v usable=%v skip=%v", out.Enforced, out.UsableForCompletion, out.SkipSupported)
	}
}

func completionPreviewSubmissionDenied(t *testing.T, out CollaborationCompletionPreview, err error, status int, code string) {
	t.Helper()
	var refusal DeliveryCompletionError
	if !errors.As(err, &refusal) || refusal.Status != status || refusal.Code != code {
		t.Errorf("original exact preview refusal changed: got=%v want=%d/%s", err, status, code)
	}
	// Some service refusals retain public envelope fields. The adapter discards
	// that envelope; no private observation, binding or stage result may survive.
	if out.Observation != nil || out.AuthorizationBinding != nil || out.MaxExpiresAt != nil || len(out.StageBlockers) != 0 || out.StageBlockersTruncated || !out.CheckedAt.IsZero() || out.Enforced || out.UsableForCompletion || out.SkipSupported {
		t.Errorf("refused preview retained a private result or completion authority: %+v", out)
	}
}
