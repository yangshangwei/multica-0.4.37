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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These tests retain the accepted binding/UTC fixture and exercise the actual
// preview's remaining read fences. They do not identify historical HTTP holders
// or prove fresh archive bytes, external execution or a complete R38 read path.
type r28PreviewFixture struct {
	*stageExecutionFixture
	submission pgtype.UUID
	request    CollaborationCompletionPreviewRequest
}

// The old completionPreviewSubmissionFixture registers its equality cleanup
// before returning. This small builder preserves the same genuine setup while
// allowing intentional negative mutations before each new equality baseline.
func r28NewPreviewFixture(t *testing.T) *r28PreviewFixture {
	t.Helper()
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("R28 preview regressions require an explicitly admitted task-owned DATABASE_URL")
	}
	f := newStageCompletionFixture(t)
	stageCompletionM1Policy(t, f, "enforce")
	submission := stageCompletionSubmit(t, f)
	out := &r28PreviewFixture{stageExecutionFixture: f, submission: submission}
	out.request = r28PreviewTarget(t, out)
	bound, err := f.svc.Queries.GetCollaborationStageSubmission(f.ctx, db.GetCollaborationStageSubmissionParams{IssueID: f.issue.Issue.ID, SubmissionID: submission})
	if err != nil || bound.SubmissionID != submission || bound.IssueID != f.issue.Issue.ID || bound.WorkspaceID != util.MustParseUUID(f.workspace) || bound.CandidateID != f.candidate.CandidateID || bound.CandidateVersion != f.candidate.Version || !bound.OperationID.Valid || bound.ActorUserID != util.MustParseUUID(f.handler) {
		t.Fatalf("R28 setup lost the genuine immutable verification submission: bound=%+v err=%v", bound, err)
	}
	for _, table := range []string{"delivery_completion_authorization", "delivery_completion_terminal", "delivery_evidence_evaluation"} {
		if count := f.fx.Count(t, `SELECT count(*) FROM "`+table+`"`); count != 0 {
			t.Fatalf("R28 fixture created durable %s rows before preview: count=%d", table, count)
		}
	}
	return out
}

func r28PreviewTarget(t *testing.T, f *r28PreviewFixture) CollaborationCompletionPreviewRequest {
	t.Helper()
	params := stageCompletionAcceptParams(t, f.stageExecutionFixture, f.submission)
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	writer := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{actor: util.MustParseUUID(f.creator), access: access}}
	request := CollaborationCompletionPreviewRequest{SourceIssueID: f.stage.child, ApprovalID: util.UUIDToString(f.stage.artifact.ApprovalID), Target: completionCollaborationTargetFromWorkflow(writer, params)}
	if request.Target.Validate() != nil || request.Target.Kind != "collaboration_submission" || request.Target.SubmissionID == nil || *request.Target.SubmissionID != util.UUIDToString(f.submission) || request.Target.ExpectedIssueRevision != access.Issue.Revision || request.Target.ExpectedLifecycleEpoch != stageVersion(access.LifecycleEpoch) || request.Target.ExpectedCollaborationRevision != stageVersion(access.Revision) || access.Issue.Status != "in_review" {
		t.Fatalf("R28 target does not name the exact current retained submission: %+v", request.Target)
	}
	return request
}

func r28AdapterState(t *testing.T, f *r28PreviewFixture) []byte {
	t.Helper()
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

func r28NoWrites(t *testing.T, f *r28PreviewFixture) {
	t.Helper()
	completionCandidatesNoWrites(t, f.stage)
	// The snapshot helper reads only the genuine base fixture. This adapter
	// creates no administrator, credential, role or administrative operation.
	adminSnapshot := &adminCollaborationFixture{collaborationQueueFixture: f.collaborationQueueFixture}
	adminBefore := adminCollaborationStateSnapshot(t, adminSnapshot)
	adapterBefore := r28AdapterState(t, f)
	t.Cleanup(func() {
		adminAfter := adminCollaborationStateSnapshot(t, adminSnapshot)
		if adminBefore != adminAfter {
			t.Errorf("R28 read/control changed root admin/audit/control rows: before_sha256=%s after_sha256=%s", hexHash([]byte(adminBefore)), hexHash([]byte(adminAfter)))
		}
		adapterAfter := r28AdapterState(t, f)
		if !reflect.DeepEqual(adapterBefore, adapterAfter) {
			t.Errorf("R28 read/control changed actual adapter call/target/invocation: before_sha256=%s after_sha256=%s", hexHash(adapterBefore), hexHash(adapterAfter))
		}
	})
}

func r28PreviewCall(t *testing.T, f *r28PreviewFixture, read completionCandidatesRead, label string, commit bool) (CollaborationCompletionPreview, error) {
	t.Helper()
	out, err := f.svc.GetCollaborationCompletionPreviewInTx(read.ctx, read.tx, f.issue.Issue.ID, pgtype.UUID{}, completionRequestCredential(read.ctx), f.request)
	if err == nil {
		var isolation string
		if err = read.tx.QueryRow(read.ctx, "SHOW transaction_isolation").Scan(&isolation); err == nil && isolation != "read committed" {
			t.Errorf("preview changed its default transaction isolation: %s", isolation)
		}
	}
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
	witness := struct {
		Label      string                      `json:"label"`
		Reason     string                      `json:"reason"`
		Blockers   []CollaborationStageBlocker `json:"stage_blockers"`
		Target     DeliveryCompletionTarget    `json:"target"`
		Source     string                      `json:"source_issue_id"`
		Approval   string                      `json:"approval_id"`
		CheckedAt  time.Time                   `json:"checked_at"`
		MaxExpires *time.Time                  `json:"max_expires_at"`
	}{label, out.Reason, out.StageBlockers, f.request.Target, f.request.SourceIssueID, f.request.ApprovalID, out.CheckedAt, out.MaxExpiresAt}
	raw, marshalErr := json.Marshal(witness)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("M1_PREVIEW_READ_WITNESS %s", raw)
	return out, err
}

func r28PreviewAvailable(t *testing.T, f *r28PreviewFixture, actor context.Context, out CollaborationCompletionPreview, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("authorized exact managed-submission preview failed: %T %v", err, err)
		if out.AuthorizationBinding != nil || out.Observation != nil || out.MaxExpiresAt != nil {
			t.Error("failed preview retained private authorization/evidence data")
		}
		return
	}
	completionPreviewSubmissionEnvelope(t, f.stageExecutionFixture, f.request, out, "enforce")
	if out.Reason != "authorization_available" || out.AuthorizationBinding == nil || out.Observation == nil || out.MaxExpiresAt == nil || out.ProjectRevision == nil {
		t.Errorf("genuine verified submission is not authorizable: reason=%s blockers=%+v", out.Reason, out.StageBlockers)
		return
	}
	child, err := f.svc.Queries.GetIssue(actor, util.MustParseUUID(f.stage.child))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(actor, f.stage.artifact.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	want := DeliveryCompletionPreviewBinding{ContractVersion: DeliveryCompletionContract, SourceWorkspaceID: f.workspace, Target: f.request.Target,
		Project:  DeliveryCompletionProject{ID: f.stage.project, ExpectedRevision: *out.ProjectRevision, ExpectedPolicyRevision: 1},
		Evidence: DeliveryCompletionEvidence{IssueID: f.stage.child, ExpectedIssueRevision: child.Revision, ApprovalID: util.UUIDToString(artifact.ApprovalID), ExpectedSourceRevision: artifact.SourceRevision, ExpectedSpecDigest: artifact.SpecDigest, ExpectedArtifactDigest: artifact.OutputArtifactDigest},
		Consumer: DeliveryCompletionConsumer{Kind: "human", UserID: f.creator}}
	if !reflect.DeepEqual(*out.AuthorizationBinding, want) {
		t.Errorf("R28 preview changed exact current binding: got=%+v want=%+v", *out.AuthorizationBinding, want)
	}
	observation := out.Observation
	if observation.Verdict != "pass" || observation.PolicyVersion != DeliveryEvidencePolicy || observation.IssueID != f.stage.child || observation.IssueRevision != child.Revision || observation.ApprovalID != f.request.ApprovalID || observation.ExecutionID != util.UUIDToString(artifact.ExecutionID) || observation.SourceRevision != artifact.SourceRevision || observation.ObservedHeadSHA != f.stage.head || observation.SpecDigest != artifact.SpecDigest || observation.ArtifactDigest != artifact.OutputArtifactDigest || observation.ReceiptDigest != artifact.ReceiptDigest || observation.ProducerAgentID != util.UUIDToString(artifact.AgentID) || observation.ProducerTaskID != util.UUIDToString(artifact.TaskID) || observation.ProducerTaskGeneration != artifact.TaskGeneration || observation.ValidUntil == nil || observation.ObservedAt.IsZero() || observation.ObservedAt.After(out.CheckedAt) {
		t.Errorf("R28 preview lost exact canonical observation/producer identity: %+v", observation)
		return
	}
	release, err := f.svc.Queries.GetCollaborationStageReviewHead(actor, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "release"})
	if err != nil || !release.ExpiresAt.Valid {
		t.Fatalf("R28 original release approval is unavailable: %v", err)
	}
	until := release.ExpiresAt.Time
	for _, horizon := range []time.Time{*observation.ValidUntil, completionRequestCredential(actor).Principal.ExpiresAt} {
		if horizon.Before(until) {
			until = horizon
		}
	}
	if !out.MaxExpiresAt.Equal(until) || !out.CheckedAt.Before(until) || out.MaxExpiresAt.Location() != time.UTC || out.MaxExpiresAt.Nanosecond()%1000 != 0 {
		t.Errorf("R28 preview changed or renewed the saved release/evidence/actual-reader horizon: got=%v want=%v checked=%v", out.MaxExpiresAt, until, out.CheckedAt)
	}
}

func r28Baseline(t *testing.T, f *r28PreviewFixture, actor context.Context) CollaborationCompletionPreview {
	t.Helper()
	before := completionCandidatesSnapshot(t, f.stage)
	adapterBefore := r28AdapterState(t, f)
	out, err := completionPreviewSubmissionRead(t, f.stageExecutionFixture, actor, f.request)
	completionCandidatesCompareState(t, before, completionCandidatesSnapshot(t, f.stage), "R28_unheld_baseline")
	if !reflect.DeepEqual(adapterBefore, r28AdapterState(t, f)) {
		t.Error("unheld preview changed the actual adapter before any control")
	}
	r28PreviewAvailable(t, f, actor, out, err)
	if err != nil || out.Reason != "authorization_available" || out.AuthorizationBinding == nil || out.Observation == nil || out.MaxExpiresAt == nil || out.ProjectRevision == nil {
		t.Fatal("genuine unheld preview failed before concurrency/negative setup")
	}
	return out
}

func r28ComparePreview(t *testing.T, got, want CollaborationCompletionPreview) {
	t.Helper()
	got.CheckedAt, want.CheckedAt = time.Time{}, time.Time{}
	if got.Observation != nil && want.Observation != nil {
		actual, baseline := *got.Observation, *want.Observation
		actual.ObservedAt, baseline.ObservedAt = time.Time{}, time.Time{}
		got.Observation, want.Observation = &actual, &baseline
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("concurrent preview changed exact binding/evidence/expiry identities: got=%+v want=%+v", got, want)
	}
}

func r28CommitHeldPreview(t *testing.T, read completionCandidatesRead, label string) {
	t.Helper()
	err := read.finalCredential()
	if err == nil {
		err = read.tx.Commit(read.ctx)
	}
	completionCandidatesLog(t, label, read, err)
	if err != nil {
		read.rollback(t)
		t.Errorf("commit held genuine preview after the second terminal: %v", err)
	}
}

func r28PreviewStillHeld(t *testing.T, read completionCandidatesRead) {
	t.Helper()
	if read.ctx.Err() != nil || read.tx.Conn().PgConn().TxStatus() != 'T' {
		t.Errorf("first actual preview released or exceeded its unchanged deadline before the second terminal: context=%v tx_status=%c", read.ctx.Err(), read.tx.Conn().PgConn().TxStatus())
	}
}

// This forwarding trace records the production R4 isolation statement. The
// existing StageCodeEvidence implementation alone constructs its read marker.
type r28R4TraceTx struct {
	*completionCandidatesTraceTx
	repeatableRead bool
}

func (tx *r28R4TraceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.completionCandidatesTraceTx.Exec(ctx, sql, args...)
	if err == nil && strings.TrimSpace(sql) == "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ" {
		tx.repeatableRead = true
	}
	return tag, err
}

type r28R4TraceStarter struct {
	TxStarter
	started chan<- *r28R4TraceTx
}

func (starter r28R4TraceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	trace := &r28R4TraceTx{completionCandidatesTraceTx: &completionCandidatesTraceTx{Tx: tx}}
	starter.started <- trace
	return trace, nil
}

func r28R4(t *testing.T, f *r28PreviewFixture, label string) (CollaborationStageCodeEvidenceView, error, uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.stage.releaseCtx, 5*time.Second)
	defer cancel()
	began, started := make(chan uint32, 1), make(chan *r28R4TraceTx, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: r28R4TraceStarter{TxStarter: f.pool, started: started}, began: began}
	view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
	var pid uint32
	select {
	case pid = <-began:
	default:
		t.Errorf("actual R4 did not begin its PostgreSQL transaction: %v", err)
	}
	select {
	case trace := <-started:
		completionCandidatesLog(t, label, completionCandidatesRead{ctx: ctx, tx: trace.completionCandidatesTraceTx}, err)
		if !trace.repeatableRead {
			t.Error("actual R4 did not retain production repeatable-read isolation")
		}
	default:
		t.Errorf("actual R4 did not expose its real forwarding trace: %v", err)
	}
	return view, err, pid
}

func r28R4Baseline(t *testing.T, f *r28PreviewFixture) CollaborationStageCodeEvidenceView {
	t.Helper()
	view, err, _ := r28R4(t, f, "R28_R4_unheld_baseline")
	if err != nil || view.Candidate.CandidateID != util.UUIDToString(f.candidate.CandidateID) || view.Candidate.State != "current" || view.Candidate.Baseline == nil || view.Candidate.Baseline.Artifact == nil || view.Candidate.Baseline.Artifact.ApprovalID != f.request.ApprovalID || view.Observed.CI.State != "passed" || len(view.TestReports) != 1 || len(view.ExecutionOperations) != 1 || len(view.ExecutionFacts) != 1 {
		t.Fatalf("R4 baseline did not contain the genuine standing QA/release/run/artifact chain: err=%v candidate=%+v", err, view.Candidate)
	}
	return view
}

func r28R4Coexistence(t *testing.T, r4First bool) {
	t.Helper()
	f := r28NewPreviewFixture(t)
	r28NoWrites(t, f)
	baseline := r28Baseline(t, f, f.ctx)
	r4Baseline := r28R4Baseline(t, f)
	if r4First {
		holdStarted := time.Now()
		held := holdStandingExecutionR4(t, f.stageExecutionFixture)
		second := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, held.pid, second.tx.Conn().PgConn().PID())
		out, secondErr := r28PreviewCall(t, f, second, "R28_preview_terminal_before_standing_R4_release", true)
		if time.Since(holdStarted) >= 5*time.Second {
			t.Error("second preview did not finish before the held R4's unchanged five-second deadline")
		}
		var first stageProjectionReadResult
		select {
		case first = <-held.done:
			t.Error("standing R4 terminated before the second actual preview terminal")
			held.release()
		default:
			held.release()
			first = <-held.done
		}
		completionCandidatesCompareR4(t, first.view, r4Baseline, first.err)
		r28PreviewAvailable(t, f, f.ctx, out, secondErr)
		if secondErr == nil {
			r28ComparePreview(t, out, baseline)
		}
		return
	}
	first := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	out, err := r28PreviewCall(t, f, first, "R28_first_preview_held_uncommitted", false)
	if err != nil {
		t.Fatalf("first real preview failed before the second R4: %v", err)
	}
	r28PreviewAvailable(t, f, f.ctx, out, err)
	r28ComparePreview(t, out, baseline)
	view, secondErr, pid := r28R4(t, f, "R28_R4_terminal_before_first_preview_commit")
	completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), pid)
	r28PreviewStillHeld(t, first)
	r28CommitHeldPreview(t, first, "R28_first_preview_commit_after_R4_terminal")
	completionCandidatesCompareR4(t, view, r4Baseline, secondErr)
}

func TestCollaborationCompletionPreviewManagedSubmissionR4Coexistence(t *testing.T) {
	t.Run("r4_first", func(t *testing.T) { r28R4Coexistence(t, true) })
	t.Run("preview_first", func(t *testing.T) { r28R4Coexistence(t, false) })
}

func TestCollaborationCompletionPreviewManagedSubmissionSiblingReads(t *testing.T) {
	t.Run("same_creator", func(t *testing.T) {
		f := r28NewPreviewFixture(t)
		r28NoWrites(t, f)
		baseline := r28Baseline(t, f, f.ctx)
		first := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		out, err := r28PreviewCall(t, f, first, "R28_sibling_first_held_uncommitted", false)
		if err != nil {
			t.Fatalf("first genuine sibling preview failed: %v", err)
		}
		r28PreviewAvailable(t, f, f.ctx, out, err)
		r28ComparePreview(t, out, baseline)
		second := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), second.tx.Conn().PgConn().PID())
		other, secondErr := r28PreviewCall(t, f, second, "R28_sibling_second_terminal_before_first_commit", true)
		r28PreviewStillHeld(t, first)
		r28CommitHeldPreview(t, first, "R28_sibling_first_commit_after_second_terminal")
		r28PreviewAvailable(t, f, f.ctx, other, secondErr)
		if secondErr == nil {
			r28ComparePreview(t, other, baseline)
		}
	})
}

func r28WriterFence(t *testing.T, writerFirst bool) {
	t.Helper()
	f := r28NewPreviewFixture(t)
	r28NoWrites(t, f)
	baseline := r28Baseline(t, f, f.ctx)
	access, child, request := stageProjectionWriterRequest(t, f.stage)
	if writerFirst {
		writer := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		observation, err := EvaluateDeliveryEvidenceInTx(writer.ctx, writer.tx, access, child, request)
		completionCandidatesLog(t, "R28_real_writer_observer_held", writer, err)
		if err != nil || observation.Verdict != "pass" || observation.ApprovalID != f.request.ApprovalID || observation.ExecutionID != baseline.Observation.ExecutionID || observation.ArtifactDigest != baseline.Observation.ArtifactDigest {
			t.Fatalf("original actual observer did not establish the writer fence: %+v %v", observation, err)
		}
		reader := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, writer.tx.Conn().PgConn().PID(), reader.tx.Conn().PgConn().PID())
		out, readErr := r28PreviewCall(t, f, reader, "R28_writer_first_reader_terminal_before_writer_release", true)
		failure := reader.tx.failure()
		completionPreviewSubmissionDenied(t, out, readErr, 409, "delivery_completion_conflict")
		if failure == nil || failure.SQLState != "55P03" || failure.Query != "LockDeliveryEvidenceIssueRead" {
			t.Errorf("reader crossed or missed the real original writer's source fence: first_pg=%+v err=%v", failure, readErr)
		}
		writer.rollback(t)
	} else {
		reader := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		out, err := r28PreviewCall(t, f, reader, "R28_preview_held_before_original_writer", false)
		if err != nil {
			t.Fatalf("first real preview failed before original writer: %v", err)
		}
		r28PreviewAvailable(t, f, f.ctx, out, err)
		r28ComparePreview(t, out, baseline)
		writer := beginCompletionCandidatesRead(t, f.stage, f.ctx)
		completionCandidatesDistinctPIDs(t, reader.tx.Conn().PgConn().PID(), writer.tx.Conn().PgConn().PID())
		afterFence, resume := make(chan uint32, 1), make(chan struct{})
		close(resume)
		probe := &stageProjectionBarrierTx{Tx: writer.tx, needle: "LockArtifactUsers", reached: afterFence, resume: resume}
		observation, writeErr := EvaluateDeliveryEvidenceInTx(writer.ctx, probe, access, child, request)
		completionCandidatesLog(t, "R28_writer_terminal_before_preview_commit", writer, writeErr)
		failure := writer.tx.failure()
		if stagePgCode(writeErr) != "55P03" || failure == nil || failure.Query != "LockLifecycleIssue" || len(afterFence) != 0 || observation.Verdict == "pass" || observation.IssueID != "" {
			t.Errorf("original observer lost its upfront fence before LockArtifactUsers: first_pg=%+v crossed_authority=%t observation=%+v err=%v", failure, len(afterFence) != 0, observation, writeErr)
		}
		writer.rollback(t)
		r28PreviewStillHeld(t, reader)
		r28CommitHeldPreview(t, reader, "R28_preview_commit_after_original_writer_refusal")
	}
	fresh := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	out, err := r28PreviewCall(t, f, fresh, "R28_fresh_preview_after_writer_control_release", true)
	r28PreviewAvailable(t, f, f.ctx, out, err)
	if err == nil {
		r28ComparePreview(t, out, baseline)
	}
}

func r28StageStateFence(t *testing.T, marked bool) {
	t.Helper()
	f := r28NewPreviewFixture(t)
	r28NoWrites(t, f)
	r28Baseline(t, f, f.ctx)
	issue, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, issue.ID)
	if err != nil || access.Issue.ID != issue.ID || access.LifecycleEpoch < 1 {
		t.Fatalf("stage-state writer lacks the current actual issue/lifecycle: %v", err)
	}
	holder := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	if err = PrepareArtifactTransaction(holder.ctx, holder.tx); err != nil {
		t.Fatal(err)
	}
	var id pgtype.UUID
	if err = holder.tx.QueryRow(holder.ctx, `SELECT issue_id FROM collaboration_stage_task WHERE issue_id=$1 FOR SHARE NOWAIT`, issue.ID).Scan(&id); err != nil || id != issue.ID {
		t.Fatalf("isolated stage-task holder did not acquire the actual row: id=%+v err=%v", id, err)
	}
	writer := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	completionCandidatesDistinctPIDs(t, holder.tx.Conn().PgConn().PID(), writer.tx.Conn().PgConn().PID())
	var tx pgx.Tx = writer.tx
	if marked {
		// Only this negative writer control supplies the existing read marker.
		tx = collaborationStageEvidenceReadTx{Tx: writer.tx}
	}
	if err = PrepareArtifactTransaction(writer.ctx, tx); err != nil {
		t.Fatal(err)
	}
	state, writeErr := stageCompletionState(writer.ctx, tx, issue, access.LifecycleEpoch)
	completionCandidatesLog(t, "R28_original_stage_state_terminal_before_SHARE_release", writer, writeErr)
	failure := writer.tx.failure()
	if stagePgCode(writeErr) != "55P03" || failure == nil || failure.Query != "LockCollaborationStageTask" || state != nil {
		t.Errorf("original stage-state writer changed its UPDATE fence with marker=%t: first_pg=%+v state=%+v err=%v", marked, failure, state, writeErr)
	}
	writer.rollback(t)
	holder.rollback(t)
	control := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	if err = PrepareArtifactTransaction(control.ctx, control.tx); err != nil {
		t.Fatal(err)
	}
	state, err = stageCompletionState(control.ctx, control.tx, issue, access.LifecycleEpoch)
	completionCandidatesLog(t, "R28_original_stage_state_after_SHARE_release", control, err)
	if err != nil || state == nil || state.task.Stage != "verification" {
		t.Errorf("original state writer did not recover after real holder rollback: state=%+v err=%v", state, err)
	}
	control.rollback(t)
}

func TestCollaborationCompletionPreviewManagedSubmissionWriterFences(t *testing.T) {
	t.Run("writer_first", func(t *testing.T) { r28WriterFence(t, true) })
	t.Run("reader_first", func(t *testing.T) { r28WriterFence(t, false) })
	t.Run("stage_state_plain", func(t *testing.T) { r28StageStateFence(t, false) })
	t.Run("stage_state_read_marker", func(t *testing.T) { r28StageStateFence(t, true) })
}

func r28StageRefusal(t *testing.T, f *r28PreviewFixture, out CollaborationCompletionPreview, err error, stage, code string) {
	t.Helper()
	if err != nil || out.Reason != "stage_incomplete" || !reflect.DeepEqual(out.StageBlockers, []CollaborationStageBlocker{{Stage: stage, Code: code}}) || out.StageBlockersTruncated || out.AuthorizationBinding != nil || out.Observation != nil || out.MaxExpiresAt != nil || out.Enforced || out.UsableForCompletion || out.SkipSupported || out.CheckedAt.IsZero() || out.CheckedAt.Location() != time.UTC || out.IssueID != f.request.Target.IssueID || out.IssueRevision != f.request.Target.ExpectedIssueRevision || out.SourceIssueID != f.request.SourceIssueID {
		t.Errorf("exact non-authorizing stage refusal changed: reason=%s blockers=%+v err=%v want=%s/%s", out.Reason, out.StageBlockers, err, stage, code)
	}
}

func r28AuthorityControl(t *testing.T, kind string) {
	t.Helper()
	f := r28NewPreviewFixture(t)
	actor := f.ctx
	if kind == "revoked_artifact_manager" {
		actor = f.stage.reviewerCtx
		if completionRequestCredential(actor).Principal.UserID != f.stage.reviewer {
			t.Fatal("reviewer control must retain its own actual request credentials")
		}
	}
	r28Baseline(t, f, actor)
	switch kind {
	case "disabled_account":
		f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.creator)
		if f.fx.Count(t, `SELECT count(*) FROM "user" WHERE id=$1 AND disabled_at IS NOT NULL`, f.creator) != 1 {
			t.Fatal("account control did not disable the real current user")
		}
	case "changed_password":
		current, err := f.svc.Queries.GetPasswordCredential(f.ctx, util.MustParseUUID(f.creator))
		if err != nil || completionRequestCredential(actor).PasswordSession == nil || completionRequestCredential(actor).PasswordSession.Version != current.SessionVersion {
			t.Fatalf("password control lacks its real original password session: %v", err)
		}
		hash, err := auth.HashPassword(f.ctx, "R28 synthetic replacement password for original-session revocation")
		if err != nil {
			t.Fatal(err)
		}
		changed, err := f.svc.Queries.ChangePasswordCredential(f.ctx, db.ChangePasswordCredentialParams{UserID: current.UserID, PasswordHash: hash, MustChangePassword: false, SessionVersion: current.SessionVersion})
		if err != nil || changed.SessionVersion != current.SessionVersion+1 || changed.PasswordHash != hash || completionRequestCredential(actor).PasswordSession.Version != current.SessionVersion {
			t.Fatalf("canonical password CAS did not revoke the original unchanged session: %v", err)
		}
	case "revoked_artifact_manager", "revoked_qa_issuer":
		user := f.stage.reviewer
		if kind == "revoked_qa_issuer" {
			user = f.stage.qa
		}
		member, err := f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(user)})
		if err != nil || member.Role != "admin" {
			t.Fatalf("manager/issuer control lacks its actual current admin member: %v", err)
		}
		if err = stageDemoteArtifactManager(t, f.stageRuntimeFixture, user); err != nil {
			t.Fatalf("canonical current manager/issuer demotion: %v", err)
		}
		member, err = f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(user)})
		if err != nil || member.Role != "member" {
			t.Fatalf("actual manager/issuer demotion was not committed: %v", err)
		}
	default:
		t.Fatalf("unknown authority control: %s", kind)
	}
	// The deliberate mutation belongs to the control's new baseline. Neither
	// the reader identity nor any original credential/target is refreshed.
	r28NoWrites(t, f)
	read := beginCompletionCandidatesRead(t, f.stage, actor)
	out, err := r28PreviewCall(t, f, read, "R28_current_authority_"+kind, true)
	if read.tx.failure() != nil {
		t.Errorf("sequential authority control failed through an unexpected PostgreSQL error: %+v", read.tx.failure())
	}
	switch kind {
	case "disabled_account", "changed_password":
		var refusal *CollaborationError
		if !errors.As(err, &refusal) || refusal.Status != 404 || refusal.Code != "not_found" || out.AuthorizationBinding != nil || out.Observation != nil || out.MaxExpiresAt != nil || len(out.StageBlockers) != 0 || out.StageBlockersTruncated || !out.CheckedAt.IsZero() || out.Enforced || out.UsableForCompletion || out.SkipSupported {
			t.Errorf("original current-account refusal changed: got=%T %v out=%+v want=404/not_found", err, err, out)
		}
	case "revoked_artifact_manager":
		completionPreviewSubmissionDenied(t, out, err, 403, "delivery_completion_denied")
	case "revoked_qa_issuer":
		r28StageRefusal(t, f, out, err, "verification", "stage_evidence_unavailable")
	}
}

func TestCollaborationCompletionPreviewManagedSubmissionCurrentAuthority(t *testing.T) {
	t.Run("disabled_account", func(t *testing.T) { r28AuthorityControl(t, "disabled_account") })
	t.Run("changed_password", func(t *testing.T) { r28AuthorityControl(t, "changed_password") })
	t.Run("revoked_artifact_manager", func(t *testing.T) { r28AuthorityControl(t, "revoked_artifact_manager") })
	t.Run("revoked_qa_issuer", func(t *testing.T) { r28AuthorityControl(t, "revoked_qa_issuer") })
}

func r28BindingRefusal(t *testing.T, stale bool) {
	t.Helper()
	f := r28NewPreviewFixture(t)
	r28Baseline(t, f, f.ctx)
	stage, code := "verification", "stage_submission_unbound"
	if stale {
		stageCompletionRework(t, f.stageExecutionFixture)
		f.request = r28PreviewTarget(t, f)
		stage, code = "development", "stage_not_reached"
	} else {
		f.fx.Exec(t, `DELETE FROM collaboration_stage_submission WHERE issue_id=$1 AND submission_id=$2`, f.issue.Issue.ID, f.submission)
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1 AND submission_id=$2`, f.issue.Issue.ID, f.submission) != 0 {
			t.Fatal("binding control did not delete only its exact fresh stage binding")
		}
	}
	r28NoWrites(t, f)
	read := beginCompletionCandidatesRead(t, f.stage, f.ctx)
	out, err := r28PreviewCall(t, f, read, "R28_exact_binding_or_stage_refusal", true)
	if read.tx.failure() != nil {
		t.Errorf("binding/stage refusal came from an unexpected PostgreSQL error: %+v", read.tx.failure())
	}
	r28StageRefusal(t, f, out, err, stage, code)
}

func TestCollaborationCompletionPreviewManagedSubmissionBindingRefusals(t *testing.T) {
	t.Run("missing_stage_binding", func(t *testing.T) { r28BindingRefusal(t, false) })
	t.Run("stale_stage", func(t *testing.T) { r28BindingRefusal(t, true) })
}
