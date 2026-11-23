package service

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The barrier forwards all real SQL. It only holds the reader's acquired
// authority/issue fences while a second connection executes a product request.
type stageProjectionBarrierStarter struct {
	TxStarter
	needle  string
	began   chan<- uint32
	reached chan<- uint32
	resume  <-chan struct{}
}

type stageProjectionBarrierTx struct {
	pgx.Tx
	needle  string
	reached chan<- uint32
	resume  <-chan struct{}
	fired   bool
}

func (s stageProjectionBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if s.began != nil {
		s.began <- tx.Conn().PgConn().PID()
	}
	return &stageProjectionBarrierTx{Tx: tx, needle: s.needle, reached: s.reached, resume: s.resume}, nil
}

func (tx *stageProjectionBarrierTx) before(ctx context.Context, sql string) {
	if tx.fired || tx.needle == "" || !strings.Contains(sql, "-- name: "+tx.needle+" ") {
		return
	}
	tx.fired = true
	tx.reached <- tx.Conn().PgConn().PID()
	select {
	case <-tx.resume:
	case <-ctx.Done():
	}
}

func (tx *stageProjectionBarrierTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.before(ctx, sql)
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *stageProjectionBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.before(ctx, sql)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

type stageProjectionReadResult struct {
	view CollaborationStageCodeEvidenceView
	err  error
}

type stageProjectionHeldRead struct {
	pid     uint32
	release func()
	done    <-chan stageProjectionReadResult
}

func holdStageArtifactR4(t *testing.T, f *stageReviewFixture, candidate CollaborationStageCandidateRef) stageProjectionHeldRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan stageProjectionReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "ListCollaborationCandidateTestReportEvents", reached: reached, resume: resume}
	go func() {
		defer close(done)
		view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, candidate.CandidateID)
		done <- stageProjectionReadResult{view: view, err: err}
	}()
	select {
	case pid := <-reached:
		return stageProjectionHeldRead{pid: pid, release: release, done: done}
	case result := <-done:
		t.Fatalf("reader did not reach its post-authority barrier: %v", result.err)
	case <-ctx.Done():
		t.Fatal("reader did not reach its post-authority barrier before the test deadline")
	}
	return stageProjectionHeldRead{}
}

func stageArtifactConcurrentReadFixture(t *testing.T) (*stageReviewFixture, CollaborationStageCandidateRef) {
	t.Helper()
	f := newStageReviewFixture(t)
	stageArtifactReadFixture(t, f)
	candidate := f.code(t, nil)
	if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, candidate, "code_review")); err != nil {
		t.Fatal(err)
	}
	return f, candidate
}

func stageProjectionWriterRequest(t *testing.T, f *stageReviewFixture) (ArtifactAccess, db.Issue, DeliveryEvidenceRequest) {
	t.Helper()
	child, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(f.child))
	if err != nil {
		t.Fatal(err)
	}
	execution, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(f.ctx, f.artifact.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	req := DeliveryEvidenceRequest{PolicyVersion: DeliveryEvidencePolicy, IssueRevision: child.Revision, ApprovalID: util.UUIDToString(f.artifact.ApprovalID), SourceRevision: f.head, SpecDigest: execution.SpecDigest, ArtifactDigest: execution.OutputArtifactDigest}
	if child.ProjectID.Valid {
		project, err := f.svc.Queries.GetProjectInWorkspace(f.ctx, db.GetProjectInWorkspaceParams{ID: child.ProjectID, WorkspaceID: child.WorkspaceID})
		if err != nil {
			t.Fatal(err)
		}
		id := util.UUIDToString(project.ID)
		req.ProjectID, req.ProjectRevision = &id, &project.Revision
	}
	return ArtifactAccess{Credential: completionRequestCredential(f.ctx), RequestedWorkspaceID: f.workspace}, child, req
}

func TestStageArtifactReadProjectionConcurrentR4(t *testing.T) {
	f, candidate := stageArtifactConcurrentReadFixture(t)
	held := holdStageArtifactR4(t, f, candidate)
	secondPID := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: secondPID}
	view, err := reader.StageCodeEvidence(f.ctx, f.issue.Issue.ID, candidate.CandidateID)
	pid := <-secondPID
	held.release()
	first := <-held.done
	if pid == held.pid || f.creator == f.reviewer {
		t.Fatal("read/read fixture did not use distinct authenticated users and PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("second authorized R4 read contended with a held R4 reader: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if first.err != nil || first.view.Candidate.Baseline == nil || view.Candidate.Baseline == nil || first.view.Candidate.Baseline.Artifact == nil || view.Candidate.Baseline.Artifact == nil || first.view.Candidate.State != "current" || view.Candidate.State != "current" ||
		first.view.Candidate.CandidateID != view.Candidate.CandidateID || first.view.Candidate.Baseline.Artifact.ArtifactDigest != view.Candidate.Baseline.Artifact.ArtifactDigest {
		t.Fatalf("concurrent R4 reads did not preserve the exact authorized artifact candidate: first=%+v second=%+v err=%v", first.view.Candidate, view.Candidate, first.err)
	}
}

func TestStageArtifactReadProjectionConcurrentR4T4(t *testing.T) {
	f, report := stageArtifactReportFixture(t)
	held := holdStageArtifactR4(t, f.stage, f.candidate)
	secondPID := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: secondPID}
	detail, err := reader.TestReport(f.ctx, f.issue.Issue.ID, util.MustParseUUID(report.RunID), util.MustParseUUID(report.ReportID))
	pid := <-secondPID
	held.release()
	first := <-held.done
	if pid == held.pid {
		t.Fatal("R4/T4 fixture did not use two PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("authorized T4 read contended with a held artifact-bound R4 reader: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if first.err != nil || first.view.Candidate.Baseline == nil || detail.ReportID != report.ReportID || len(first.view.TestReports) != 1 || first.view.TestReports[0].ReportID != report.ReportID {
		t.Fatalf("concurrent R4/T4 lost the exact retained report: report=%s first=%+v err=%v", detail.ReportID, first.view.TestReports, first.err)
	}
}

func TestStageArtifactReadProjectionWriterFences(t *testing.T) {
	t.Run("writer_first", func(t *testing.T) {
		f, candidate := stageArtifactConcurrentReadFixture(t)
		access, child, req := stageProjectionWriterRequest(t, f)
		writer, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(f.ctx)
		observation, err := EvaluateDeliveryEvidenceInTx(f.ctx, writer, access, child, req)
		if err != nil || observation.Verdict != "pass" {
			t.Fatalf("actual writer observer did not establish its retained artifact fence: %+v %v", observation, err)
		}
		view, err := f.svc.StageCodeEvidence(f.reviewerCtx, f.issue.Issue.ID, candidate.CandidateID)
		if stagePgCode(err) != "55P03" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
			t.Fatalf("reader crossed the writer's exclusive issue fence or exposed partial evidence: SQLSTATE=%s view=%+v", stagePgCode(err), view.Candidate)
		}
		if err := writer.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
		if fresh, err := f.svc.StageCodeEvidence(f.reviewerCtx, f.issue.Issue.ID, candidate.CandidateID); err != nil || fresh.Candidate.Baseline == nil {
			t.Fatalf("fresh authorized read after writer release failed: %v", err)
		}
	})
	t.Run("reader_first", func(t *testing.T) {
		f, candidate := stageArtifactConcurrentReadFixture(t)
		access, child, req := stageProjectionWriterRequest(t, f)
		held := holdStageArtifactR4(t, f, candidate)
		writer, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(f.ctx)
		afterFence := make(chan uint32, 1)
		resume := make(chan struct{})
		close(resume)
		probe := &stageProjectionBarrierTx{Tx: writer, needle: "LockArtifactUsers", reached: afterFence, resume: resume}
		_, err = EvaluateDeliveryEvidenceInTx(f.ctx, probe, access, child, req)
		if stagePgCode(err) != "55P03" || len(afterFence) != 0 {
			t.Fatalf("writer did not retain its upfront exclusive fence before authority observation: SQLSTATE=%s passed_initial_fence=%t", stagePgCode(err), len(afterFence) != 0)
		}
		if err := writer.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
		held.release()
		if first := <-held.done; first.err != nil || first.view.Candidate.Baseline == nil {
			t.Fatalf("held reader failed after the contending writer rolled back: %v", first.err)
		}
	})
}

func TestStageArtifactReadProjectionRevocation(t *testing.T) {
	t.Run("reader_first", func(t *testing.T) {
		f, report := stageArtifactReportFixture(t)
		held := holdStageArtifactR4(t, f.stage, f.candidate)
		if err := stageDemoteArtifactManager(t, f, f.stage.reviewer); stagePgCode(err) != "55P03" {
			t.Fatalf("current manager demotion crossed the held reader's authority fences: %v", err)
		}
		held.release()
		if first := <-held.done; first.err != nil || first.view.Candidate.Baseline == nil {
			t.Fatalf("held authorized reader failed: %v", first.err)
		}
		if err := stageDemoteArtifactManager(t, f, f.stage.reviewer); err != nil {
			t.Fatal(err)
		}
		view, err := f.svc.StageCodeEvidence(f.stage.reviewerCtx, f.issue.Issue.ID, f.candidate.CandidateID)
		if err != nil || view.Candidate.Baseline != nil || len(view.TestReports) != 0 {
			t.Fatalf("fresh R4 read revived a revoked manager's private evidence: %+v %v", view.Candidate, err)
		}
		if detail, err := f.svc.TestReport(f.stage.reviewerCtx, f.issue.Issue.ID, util.MustParseUUID(report.RunID), util.MustParseUUID(report.ReportID)); !collaborationErrorCode(err, "not_found") || !reflect.DeepEqual(detail, CollaborationTestReportDetail{}) {
			t.Fatalf("fresh T4 read restored private report data after demotion: %v", err)
		}
	})
	t.Run("revocation_before_authority_lock", func(t *testing.T) {
		f, _ := stageArtifactReportFixture(t)
		hits := 0
		reader := *f.svc
		reader.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: "GetCollaborationStageCodeHead", before: func() {
			hits++
			if err := stageDemoteArtifactManager(t, f, f.stage.reviewer); err != nil {
				t.Fatal(err)
			}
		}}
		view, err := reader.StageCodeEvidence(f.stage.reviewerCtx, f.issue.Issue.ID, f.candidate.CandidateID)
		if hits != 1 || stagePgCode(err) != "40001" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
			t.Fatalf("revocation after the RR snapshot was accepted or exposed partial R4 data: hooks=%d SQLSTATE=%s view=%+v", hits, stagePgCode(err), view.Candidate)
		}
		fresh, err := f.svc.StageCodeEvidence(f.stage.reviewerCtx, f.issue.Issue.ID, f.candidate.CandidateID)
		if err != nil || fresh.Candidate.Baseline != nil || len(fresh.TestReports) != 0 {
			t.Fatalf("fresh R4 read after committed revocation exposed private evidence: %v", err)
		}
	})
}
