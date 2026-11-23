package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Every scenario consumes one real same-P grant once before its read baseline.
// Synthetic managed execution and human signatures prove these local contracts,
// not historical lock holders, archive-byte freshness or external deployment.
type r29ConsumedFixture struct {
	*stageExecutionFixture
	submission pgtype.UUID
	params     CollaborationLifecycleParams
	binding    DeliveryCompletionBinding
	grant      db.DeliveryCompletionAuthorization
	terminal   db.DeliveryCompletionTerminal
	after      db.Issue
	expected   DeliveryCompletionAuthorizationView
	operation  CollaborationWorkflowOperationReceipt
}

func r29NewConsumedFixture(t *testing.T) *r29ConsumedFixture {
	t.Helper()
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("consumed-read tests require an explicitly admitted task-owned DATABASE_URL")
	}
	f := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, f, "enforce")
	submission := stageCompletionSubmit(t, f)
	params := stageCompletionAcceptParams(t, f, submission)
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	target := completionCollaborationTargetFromWorkflow(&collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{actor: util.MustParseUUID(f.creator), access: access}}, params)
	binding := stageCompletionM1ForLifecycle(t, f, f.ctx, params)
	params.Completion = &binding
	completed, err := f.svc.ChangeLifecycle(f.ctx, params)
	if err != nil || completed.Status != 200 || completed.Code != "completed" || completed.DeliveryConfirmation == nil || completed.DeliveryConfirmation.SubmissionID != submission {
		t.Fatalf("one actual M1-bound completion did not commit: receipt=%+v err=%v", completed, err)
	}
	out := &r29ConsumedFixture{stageExecutionFixture: f, submission: submission, params: params, binding: binding}
	out.after, err = f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	out.grant, err = f.svc.Queries.LockDeliveryCompletionAuthorization(f.ctx, util.MustParseUUID(binding.AuthorizationID))
	if err != nil {
		t.Fatal(err)
	}
	out.terminal, err = f.svc.Queries.GetDeliveryCompletionTerminal(f.ctx, out.grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := completionStoredRequest(out.grant)
	if err != nil || !completionTargetEqual(stored.Target, target) || stored.OperationID != params.RequestID || stored.SourceWorkspaceID != f.workspace || stored.Consumer.Kind != "human" || stored.Consumer.UserID != f.creator || stored.Evidence.IssueID != f.stage.child || stored.Evidence.ApprovalID != util.UUIDToString(f.stage.artifact.ApprovalID) || stored.Project.ID != f.stage.project || out.grant.RequestDigest != stored.Digest() {
		t.Fatalf("actual grant lost its exact original target/consumer/body: err=%v", err)
	}
	artifact, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(f.ctx, f.stage.artifact.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	if out.grant.IssuerUserID != util.MustParseUUID(f.creator) || out.grant.ConsumerUserID != out.grant.IssuerUserID || out.grant.TargetKind != "collaboration_submission" || out.grant.IssueID != out.after.ID || out.grant.SubmissionID != submission || out.grant.EvidenceIssueID != artifact.ResourceID || out.grant.ExecutionID != artifact.ExecutionID || out.grant.SpecDigest != artifact.SpecDigest || out.grant.ArtifactDigest != artifact.OutputArtifactDigest || out.grant.ReceiptDigest != artifact.ReceiptDigest || out.grant.SourceRevision != artifact.SourceRevision || out.grant.ProducerAgentID != artifact.AgentID || out.grant.ProducerTaskID != artifact.TaskID || out.grant.ProducerGeneration != artifact.TaskGeneration || out.after.Status != params.TargetStatus || out.after.Revision <= target.ExpectedIssueRevision || out.terminal.Kind != "consumed" || out.terminal.AuthorizationID != out.grant.ID || out.terminal.OperationID != out.grant.OperationID || out.terminal.IssueRevisionBefore.Int64 != target.ExpectedIssueRevision || out.terminal.IssueRevisionAfter.Int64 != out.after.Revision || out.terminal.StatusAfter.String != out.after.Status || out.terminal.ActorKind != "human" || out.terminal.ActorUserID != out.grant.ConsumerUserID || out.terminal.ResultSubmissionID != submission || out.terminal.ResultCollaborationRevision.Int64 != completed.CollaborationRevision || !out.terminal.CollaborationOperationID.Valid || !out.terminal.RecordedAt.Valid || !DeliveryHex(out.terminal.MutationDigest.String, 64) {
		t.Fatal("actual consumed terminal/grant/artifact/status identity changed")
	}
	collaborationRevision, submissionID := stageVersion(completed.CollaborationRevision), util.UUIDToString(submission)
	receipt := DeliveryCompletionReceipt{AuthorizationID: binding.AuthorizationID, OperationID: params.RequestID, IssueID: util.UUIDToString(out.after.ID), CompletedRevision: out.after.Revision, TargetStatus: out.after.Status, CompletedAt: out.terminal.RecordedAt.Time.UTC(), CollaborationRevision: &collaborationRevision, SubmissionID: &submissionID}
	out.expected = DeliveryCompletionAuthorizationView{AuthorizationID: binding.AuthorizationID, OperationID: params.RequestID, IssueID: util.UUIDToString(out.after.ID), IssueRevision: target.ExpectedIssueRevision, TargetStatus: target.TargetStatus, ContractVersion: DeliveryCompletionContract, CreatedAt: out.grant.CreatedAt.Time.UTC(), ExpiresAt: out.grant.ExpiresAt.Time.UTC(), State: "consumed", Applicability: "consumed", CompletionReceipt: &receipt}
	out.operation = CollaborationWorkflowOperationReceipt{Status: completed.Status, Code: completed.Code, CollaborationRevision: completed.CollaborationRevision, DeliveryConfirmation: completed.DeliveryConfirmation, StageConfirmation: completed.StageConfirmation, StageBlockers: completed.StageBlockers, StageBlockersTruncated: completed.StageBlockersTruncated}
	read := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	view, err := r29Get(t, out, read, "R29_consumed_setup_unheld_GET", true)
	r29WantConsumed(t, out, view, err)
	if err != nil {
		t.Fatal("unheld consumed GET failed before read-concurrency setup")
	}
	operation, err, _ := r29Operation(t, out, "R29_completed_setup_own_operation")
	r29WantOperation(t, out, operation, err)
	if err != nil {
		t.Fatal("own completed operation failed before read-concurrency setup")
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, out.grant.ID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1 AND submission_id=$2`, out.after.ID, submission) != 1 {
		t.Fatal("real completion did not retain one original terminal and stage submission")
	}
	return out
}

func r29NoWrites(t *testing.T, f *r29ConsumedFixture) {
	t.Helper()
	// r28NoWrites only projects this genuine base fixture. Its adapter does not
	// create a preview, administrator, grant, role or stage fact.
	r28NoWrites(t, &r28PreviewFixture{stageExecutionFixture: f.stageExecutionFixture})
}

func r29Get(t *testing.T, f *r29ConsumedFixture, read completionCandidatesRead, label string, commit bool) (DeliveryCompletionAuthorizationView, error) {
	t.Helper()
	view, err := GetDeliveryCompletionAuthorizationInTx(read.ctx, read.tx, ArtifactAccess{Credential: completionRequestCredential(read.ctx), RequestedWorkspaceID: f.workspace}, f.grant.ID)
	if err == nil {
		err = deliveryCompletionError(read.finalCredential())
	}
	if err == nil && commit {
		err = deliveryCompletionError(read.tx.Commit(read.ctx))
	}
	if err != nil {
		read.rollback(t)
	}
	completionCandidatesLog(t, label, read, err)
	raw, marshalErr := json.Marshal(struct {
		Label string                              `json:"label"`
		View  DeliveryCompletionAuthorizationView `json:"view"`
	}{label, view})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("M1_CONSUMED_READ_WITNESS %s", raw)
	return view, err
}

func r29WantConsumed(t *testing.T, f *r29ConsumedFixture, view DeliveryCompletionAuthorizationView, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("same-P consumed GET did not retain its exact receipt: %T %v", err, err)
		if !reflect.DeepEqual(view, DeliveryCompletionAuthorizationView{}) {
			t.Error("failed consumed GET exposed a partial private receipt")
		}
		return
	}
	if !reflect.DeepEqual(view, f.expected) {
		t.Errorf("consumed GET changed original grant/operation/body/status receipt: got=%+v want=%+v", view, f.expected)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"artifact", "spec_digest", "source_revision", "request_json", "observation", "token_hash", "issuer_password"} {
		if strings.Contains(string(raw), field) {
			t.Errorf("minimal consumed receipt exposed private field %s", field)
		}
	}
}

type r29TraceStarter struct {
	TxStarter
	started chan<- completionCandidatesRead
}

func (starter r29TraceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	read := completionCandidatesRead{ctx: ctx, tx: &completionCandidatesTraceTx{Tx: tx}}
	starter.started <- read
	return read.tx, nil
}

func r29Operation(t *testing.T, f *r29ConsumedFixture, label string) (CollaborationWorkflowOperationReceipt, error, uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	began, started := make(chan uint32, 1), make(chan completionCandidatesRead, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: r29TraceStarter{TxStarter: f.pool, started: started}, began: began}
	receipt, err := reader.GetWorkflowOperation(ctx, f.after.ID, f.params.RequestID)
	var pid uint32
	select {
	case pid = <-began:
	default:
		t.Errorf("own operation did not begin its actual transaction: %v", err)
	}
	select {
	case read := <-started:
		completionCandidatesLog(t, label, read, err)
	default:
		t.Errorf("own operation did not expose its SQL-forwarding trace: %v", err)
	}
	return receipt, err, pid
}

func r29WantOperation(t *testing.T, f *r29ConsumedFixture, receipt CollaborationWorkflowOperationReceipt, err error) {
	t.Helper()
	if err != nil || !reflect.DeepEqual(receipt, f.operation) || receipt.Status != 200 || receipt.Code != "completed" || receipt.DeliveryConfirmation == nil || receipt.DeliveryConfirmation.SubmissionID != f.submission {
		t.Errorf("own operation changed its exact original completed confirmation: receipt=%+v err=%v", receipt, err)
	}
}

func r29R4(t *testing.T, f *r29ConsumedFixture, label string) (CollaborationStageCodeEvidenceView, error, uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	began, started := make(chan uint32, 1), make(chan completionCandidatesRead, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: r29TraceStarter{TxStarter: f.pool, started: started}, began: began}
	view, err := reader.StageCodeEvidence(ctx, f.after.ID, f.candidate.CandidateID)
	var pid uint32
	select {
	case pid = <-began:
	default:
		t.Errorf("post-completion R4 did not begin its actual transaction: %v", err)
	}
	select {
	case read := <-started:
		completionCandidatesLog(t, label, read, err)
	default:
		t.Errorf("post-completion R4 did not expose its actual trace: %v", err)
	}
	return view, err, pid
}

func r29R4Baseline(t *testing.T, f *r29ConsumedFixture) CollaborationStageCodeEvidenceView {
	t.Helper()
	view, err, _ := r29R4(t, f, "R29_post_completion_R4_unheld")
	if err != nil || view.IssueID != util.UUIDToString(f.after.ID) || view.Candidate.CandidateID != util.UUIDToString(f.candidate.CandidateID) || view.Candidate.Baseline == nil || view.Candidate.Baseline.Artifact == nil || view.Candidate.Baseline.Artifact.ApprovalID != util.UUIDToString(f.stage.artifact.ApprovalID) || view.CheckedAt.IsZero() {
		t.Fatalf("post-completion R4 did not retain its actual readable artifact projection: candidate=%+v err=%v", view.Candidate, err)
	}
	// Completion can make the current candidate/QA chain stale. The real
	// unheld projection is the baseline; no approval/epoch is reset here.
	return view
}

func r29HoldR4(t *testing.T, f *r29ConsumedFixture) stageProjectionHeldRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan stageProjectionReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "ListCollaborationCandidateTestReportEvents", reached: reached, resume: resume}
	go func() {
		defer close(done)
		view, err := reader.StageCodeEvidence(ctx, f.after.ID, f.candidate.CandidateID)
		done <- stageProjectionReadResult{view: view, err: err}
	}()
	select {
	case pid := <-reached:
		return stageProjectionHeldRead{pid: pid, release: release, done: done}
	case first := <-done:
		t.Fatalf("same-P post-completion R4 did not reach the real post-artifact barrier: %v", first.err)
	case <-ctx.Done():
		t.Fatal("same-P R4 did not reach its barrier within the original five-second deadline")
	}
	return stageProjectionHeldRead{}
}

func r29HoldOperation(t *testing.T, f *r29ConsumedFixture) workflowHeldReceiptRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan workflowReceiptReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "GetCollaborationOperation", reached: reached, resume: resume}
	go func() {
		defer close(done)
		receipt, err := reader.GetWorkflowOperation(ctx, f.after.ID, f.params.RequestID)
		done <- workflowReceiptReadResult{receipt: receipt, err: err}
	}()
	select {
	case pid := <-reached:
		return workflowHeldReceiptRead{pid: pid, release: release, done: done}
	case first := <-done:
		t.Fatalf("same-P operation did not reach the actual user-SHARE barrier: %v", first.err)
	case <-ctx.Done():
		t.Fatal("same-P operation did not reach its barrier within the original deadline")
	}
	return workflowHeldReceiptRead{}
}

func r29CommitHeld(t *testing.T, read completionCandidatesRead, label string) {
	t.Helper()
	if read.ctx.Err() != nil || read.tx.Conn().PgConn().TxStatus() != 'T' {
		t.Errorf("first real consumed GET released or expired before the second terminal: context=%v status=%c", read.ctx.Err(), read.tx.Conn().PgConn().TxStatus())
	}
	err := read.finalCredential()
	if err == nil {
		err = read.tx.Commit(read.ctx)
	}
	completionCandidatesLog(t, label, read, err)
	if err != nil {
		read.rollback(t)
		t.Errorf("held consumed GET did not commit after the second terminal: %v", err)
	}
}

func TestDeliveryCompletionConsumedGetReadCoexistence(t *testing.T) {
	t.Run("r4_first", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		baseline := r29R4Baseline(t, f)
		started := time.Now()
		held := r29HoldR4(t, f)
		second := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, held.pid, second.tx.Conn().PgConn().PID())
		view, err := r29Get(t, f, second, "R29_GET_terminal_before_R4_release", true)
		if time.Since(started) >= 5*time.Second {
			t.Error("second GET exceeded the held R4's original deadline")
		}
		var first stageProjectionReadResult
		select {
		case first = <-held.done:
			t.Error("held R4 terminated before the second consumed GET terminal")
			held.release()
		default:
			held.release()
			first = <-held.done
		}
		completionCandidatesCompareR4(t, first.view, baseline, first.err)
		r29WantConsumed(t, f, view, err)
	})
	t.Run("authorization_first", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		baseline := r29R4Baseline(t, f)
		first := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		view, err := r29Get(t, f, first, "R29_GET_held_before_R4", false)
		r29WantConsumed(t, f, view, err)
		if err != nil {
			t.Fatal("first consumed GET failed before R4")
		}
		other, readErr, pid := r29R4(t, f, "R29_R4_terminal_before_GET_commit")
		completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), pid)
		r29CommitHeld(t, first, "R29_GET_commit_after_R4_terminal")
		completionCandidatesCompareR4(t, other, baseline, readErr)
	})
	t.Run("same_grant_siblings", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		first := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		view, err := r29Get(t, f, first, "R29_first_same_grant_GET_held", false)
		r29WantConsumed(t, f, view, err)
		if err != nil {
			t.Fatal("first same-grant GET failed")
		}
		second := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), second.tx.Conn().PgConn().PID())
		other, readErr := r29Get(t, f, second, "R29_sibling_terminal_before_first_GET_commit", true)
		r29CommitHeld(t, first, "R29_first_GET_commit_after_sibling_terminal")
		r29WantConsumed(t, f, other, readErr)
	})
	t.Run("operation_first", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		started := time.Now()
		held := r29HoldOperation(t, f)
		second := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, held.pid, second.tx.Conn().PgConn().PID())
		view, err := r29Get(t, f, second, "R29_GET_terminal_before_operation_release", true)
		if time.Since(started) >= 5*time.Second {
			t.Error("second GET exceeded the held operation's original deadline")
		}
		var first workflowReceiptReadResult
		select {
		case first = <-held.done:
			t.Error("held operation terminated before the second GET terminal")
			held.release()
		default:
			held.release()
			first = <-held.done
		}
		r29WantOperation(t, f, first.receipt, first.err)
		r29WantConsumed(t, f, view, err)
	})
	t.Run("operation_after_authorization", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		first := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		view, err := r29Get(t, f, first, "R29_GET_held_before_operation", false)
		r29WantConsumed(t, f, view, err)
		if err != nil {
			t.Fatal("first GET failed before own operation")
		}
		operation, readErr, pid := r29Operation(t, f, "R29_operation_terminal_before_GET_commit")
		completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), pid)
		r29CommitHeld(t, first, "R29_GET_commit_after_operation_terminal")
		r29WantOperation(t, f, operation, readErr)
	})
}

type r29TerminalProbeTx struct {
	*completionCandidatesTraceTx
	terminalScans int
}

type r29TerminalProbeRow struct {
	pgx.Row
	owner *r29TerminalProbeTx
}

func (row r29TerminalProbeRow) Scan(dest ...any) error {
	err := row.Row.Scan(dest...)
	if err == nil {
		row.owner.terminalScans++
	}
	return err
}

func (tx *r29TerminalProbeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.completionCandidatesTraceTx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: GetDeliveryCompletionTerminal ") {
		return r29TerminalProbeRow{Row: row, owner: tx}
	}
	return row
}

func r29Revoke(t *testing.T, f *r29ConsumedFixture, read completionCandidatesRead, label string) (DeliveryCompletionAuthorizationView, error, int) {
	t.Helper()
	probe := &r29TerminalProbeTx{completionCandidatesTraceTx: read.tx}
	view, err := RevokeDeliveryCompletionAuthorizationInTx(read.ctx, probe, ArtifactAccess{Credential: completionRequestCredential(read.ctx), RequestedWorkspaceID: f.workspace}, f.grant.ID, util.MustParseUUID(uuid.NewString()), "Synthetic explicit withdrawal of the already-consumed grant")
	completionCandidatesLog(t, label, read, err)
	t.Logf("M1_CONSUMED_REVOKE terminal_scans=%d", probe.terminalScans)
	return view, err, probe.terminalScans
}

func r29WantConflict(t *testing.T, view DeliveryCompletionAuthorizationView, err error) {
	t.Helper()
	var refusal DeliveryCompletionError
	if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "delivery_completion_conflict" || !reflect.DeepEqual(view, DeliveryCompletionAuthorizationView{}) {
		t.Errorf("original consumed-grant withdrawal refusal changed: view=%+v err=%v", view, err)
	}
}

func TestDeliveryCompletionConsumedGetWriterFences(t *testing.T) {
	t.Run("writer_first", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		writer := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		view, err, hits := r29Revoke(t, f, writer, "R29_real_Revoke_consumed_terminal_then_hold")
		r29WantConflict(t, view, err)
		if hits != 1 || writer.tx.failure() != nil || writer.tx.Conn().PgConn().TxStatus() != 'T' {
			t.Fatalf("Revoke did not actually reach its consumed terminal with original UPDATE retained: scans=%d first_pg=%+v", hits, writer.tx.failure())
		}
		reader := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, writer.tx.Conn().PgConn().PID(), reader.tx.Conn().PgConn().PID())
		other, readErr := r29Get(t, f, reader, "R29_GET_terminal_before_real_Revoke_release", true)
		r29WantConflict(t, other, readErr)
		failure := reader.tx.failure()
		grantLock := false
		if failure != nil {
			query := failure.Query
			statement := strings.ToLower(strings.Join(strings.Fields(query), ""))
			namedGrantLock := strings.Contains(query, "DeliveryCompletionAuthorization") && (strings.HasPrefix(query, "Lock") || strings.HasPrefix(query, "Read") || strings.HasPrefix(query, "Get"))
			literalGrantLock := strings.Contains(statement, "fromdelivery_completion_authorization") && strings.Contains(statement, "whereid=$1") && (strings.Contains(statement, "forsharenowait") || strings.Contains(statement, "forkeysharenowait") || strings.Contains(statement, "forupdatenowait"))
			grantLock = namedGrantLock || literalGrantLock
		}
		if failure == nil || failure.SQLState != "55P03" || failure.BackendPID != reader.tx.Conn().PgConn().PID() || !reflect.DeepEqual(failure.UUIDArgs, []string{util.UUIDToString(f.grant.ID)}) || !DeliveryHex(failure.SQLSHA256, 64) || !grantLock {
			t.Errorf("GET crossed the original Revoke authorization UPDATE fence: first_pg=%+v err=%v", failure, readErr)
		}
		writer.rollback(t)
		fresh := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		other, readErr = r29Get(t, f, fresh, "R29_GET_after_Revoke_rollback", true)
		r29WantConsumed(t, f, other, readErr)
	})
	t.Run("reader_first", func(t *testing.T) {
		f := r29NewConsumedFixture(t)
		r29NoWrites(t, f)
		reader := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		view, err := r29Get(t, f, reader, "R29_consumed_GET_held_before_Revoke", false)
		r29WantConsumed(t, f, view, err)
		if err != nil {
			t.Fatal("first GET failed before Revoke")
		}
		writer := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, reader.tx.Conn().PgConn().PID(), writer.tx.Conn().PgConn().PID())
		other, writeErr, hits := r29Revoke(t, f, writer, "R29_Revoke_terminal_before_GET_commit")
		r29WantConflict(t, other, writeErr)
		failure := writer.tx.failure()
		if hits != 0 || failure == nil || failure.SQLState != "55P03" || failure.Query != "LockDeliveryCompletionAuthorization" {
			t.Errorf("original Revoke lost its upfront UPDATE before terminal read: scans=%d first_pg=%+v err=%v", hits, failure, writeErr)
		}
		writer.rollback(t)
		r29CommitHeld(t, reader, "R29_GET_commit_after_Revoke_refusal")
		fresh := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		other, writeErr, hits = r29Revoke(t, f, fresh, "R29_original_Revoke_refusal_after_GET_commit")
		r29WantConflict(t, other, writeErr)
		if hits != 1 || fresh.tx.failure() != nil {
			t.Errorf("original semantic Revoke refusal did not recover after GET release: scans=%d first_pg=%+v", hits, fresh.tx.failure())
		}
		fresh.rollback(t)
	})
}

func r29Authority(t *testing.T, kind string) {
	t.Helper()
	f := r29NewConsumedFixture(t)
	actor := f.ctx
	status, code := 403, "delivery_completion_denied"
	switch kind {
	case "wrong_actor":
		actor = f.stage.handlerCtx
		access, err := f.svc.ResolveIssueAccess(actor, nil, f.after.ID)
		if err != nil || !access.Capabilities.Read || completionRequestCredential(actor).Principal.UserID != f.handler || f.handler == f.creator {
			t.Fatalf("D is not the real retained different collaboration actor: %v", err)
		}
		status, code = 404, "delivery_completion_not_found"
	case "changed_password":
		current, err := f.svc.Queries.GetPasswordCredential(f.ctx, util.MustParseUUID(f.creator))
		if err != nil || completionRequestCredential(actor).PasswordSession == nil || completionRequestCredential(actor).PasswordSession.Version != current.SessionVersion {
			t.Fatalf("password control lacks the actual original P session: %v", err)
		}
		hash, err := auth.HashPassword(f.ctx, "R29 synthetic replacement password for retained original-session revocation")
		if err != nil {
			t.Fatal(err)
		}
		changed, err := f.svc.Queries.ChangePasswordCredential(f.ctx, db.ChangePasswordCredentialParams{UserID: current.UserID, PasswordHash: hash, MustChangePassword: false, SessionVersion: current.SessionVersion})
		if err != nil || changed.SessionVersion != current.SessionVersion+1 || completionRequestCredential(actor).PasswordSession.Version != current.SessionVersion {
			t.Fatalf("actual password CAS did not revoke the unchanged original session: %v", err)
		}
	case "source_revoked":
		member, err := f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(f.creator)})
		if err != nil {
			t.Fatal(err)
		}
		if err = f.svc.Queries.DeleteMember(f.ctx, member.ID); err != nil {
			t.Fatal(err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM member WHERE id=$1`, member.ID) != 0 {
			t.Fatal("source revocation did not remove the actual source membership")
		}
	default:
		t.Fatalf("unknown consumed-read authority control: %s", kind)
	}
	// Intentional D/session/source changes precede this control's baseline.
	r29NoWrites(t, f)
	read := beginCompletionCandidatesRead(t, f.stage, actor)
	view, err := r29Get(t, f, read, "R29_current_authority_"+kind, true)
	var refusal DeliveryCompletionError
	if !errors.As(err, &refusal) || refusal.Status != status || refusal.Code != code || !reflect.DeepEqual(view, DeliveryCompletionAuthorizationView{}) || read.tx.failure() != nil {
		t.Errorf("original current-authority denial changed: view=%+v err=%T %v first_pg=%+v want=%d/%s", view, err, err, read.tx.failure(), status, code)
	}
	if kind == "source_revoked" {
		operation, err, _ := r29Operation(t, f, "R29_own_operation_after_source_revocation")
		r29WantOperation(t, f, operation, err)
	}
}

func TestDeliveryCompletionConsumedGetCurrentAuthority(t *testing.T) {
	t.Run("wrong_actor", func(t *testing.T) { r29Authority(t, "wrong_actor") })
	t.Run("changed_password", func(t *testing.T) { r29Authority(t, "changed_password") })
	t.Run("source_revoked", func(t *testing.T) { r29Authority(t, "source_revoked") })
}
