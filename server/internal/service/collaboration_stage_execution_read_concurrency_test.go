package service

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/util"
)

type stageExecutionReadAccessProbe struct {
	TxStarter
	collaboration, session *atomic.Int32
}

type stageExecutionReadAccessProbeTx struct {
	pgx.Tx
	collaboration, session *atomic.Int32
}

func (s stageExecutionReadAccessProbe) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageExecutionReadAccessProbeTx{Tx: tx, collaboration: s.collaboration, session: s.session}, nil
}

func (tx *stageExecutionReadAccessProbeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	result, err := tx.Tx.Exec(ctx, sql, args...)
	if err == nil && result.RowsAffected() == 1 {
		if strings.Contains(sql, "FROM issue_collaboration WHERE issue_id=$1 FOR ") {
			tx.collaboration.Add(1)
		}
		if strings.Contains(sql, "FROM collaboration_session WHERE issue_id=$1 FOR ") {
			tx.session.Add(1)
		}
	}
	return result, err
}

func stageStandingExecutionFixture(t *testing.T) (*stageExecutionFixture, string) {
	t.Helper()
	f := newStageExecutionFixture(t)
	f.source.behavior = "unknown"
	request, release := f.release(t)
	receipt, err := f.svc.PrepareStageExecution(context.Background(), request, f.source.consumer(request))
	if err != nil || receipt.State != "unknown" || receipt.FactID != nil || f.source.calls != 1 {
		t.Fatalf("standing release fixture did not retain one actual unknown intent: %+v %v", receipt, err)
	}
	return f, release
}

func holdStandingExecutionR4(t *testing.T, f *stageExecutionFixture) stageProjectionHeldRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.stage.releaseCtx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan stageProjectionReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	var collaboration, session atomic.Int32
	reader := *f.svc
	reader.TxStarter = stageExecutionReadAccessProbe{TxStarter: stageProjectionBarrierStarter{TxStarter: f.pool, needle: "ListStageExecutionIntents", reached: reached, resume: resume}, collaboration: &collaboration, session: &session}
	go func() {
		defer close(done)
		view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
		done <- stageProjectionReadResult{view: view, err: err}
	}()
	select {
	case pid := <-reached:
		if collaboration.Load() < 1 || session.Load() < 1 {
			t.Fatalf("R4 did not execute the retained QA issuer's collaboration/session access branch: collaboration=%d session=%d", collaboration.Load(), session.Load())
		}
		return stageProjectionHeldRead{pid: pid, release: release, done: done}
	case first := <-done:
		t.Fatalf("standing execution R4 did not reach its post-approval-chain barrier: %v", first.err)
	case <-ctx.Done():
		t.Fatal("standing execution R4 did not reach its barrier before the unchanged test deadline")
	}
	return stageProjectionHeldRead{}
}

func TestStageExecutionReadStandingApproval(t *testing.T) {
	for _, other := range []string{"r4", "t4"} {
		t.Run(other, func(t *testing.T) {
			f, release := stageStandingExecutionFixture(t)
			prior := f.stage.evidence(t, f.ctx, f.candidate)
			if len(prior.TestReports) != 1 {
				t.Fatal("standing QA fixture did not retain its exact report")
			}
			held := holdStandingExecutionR4(t, f)
			began := make(chan uint32, 1)
			reader := *f.svc
			reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
			var err error
			if other == "r4" {
				var view CollaborationStageCodeEvidenceView
				view, err = reader.StageCodeEvidence(f.ctx, f.issue.Issue.ID, f.candidate.CandidateID)
				if err == nil && (view.Candidate.CandidateID != util.UUIDToString(f.candidate.CandidateID) || len(view.ExecutionOperations) != 1 || view.ExecutionOperations[0].State != "unknown") {
					t.Errorf("second authorized R4 lost the exact candidate/unknown operation: %+v", view.ExecutionOperations)
				}
			} else {
				report := prior.TestReports[0]
				var detail CollaborationTestReportDetail
				detail, err = reader.TestReport(f.ctx, f.issue.Issue.ID, util.MustParseUUID(report.RunID), util.MustParseUUID(report.ReportID))
				if err == nil && detail.ReportID != report.ReportID {
					t.Errorf("concurrent T4 lost the exact standing QA report: %s", detail.ReportID)
				}
			}
			pid := <-began
			held.release()
			first := <-held.done
			if pid == held.pid || f.creator == f.stage.release {
				t.Fatal("standing read fixture did not use distinct users and PostgreSQL connections")
			}
			if err != nil {
				t.Fatalf("authorized %s reader contended with the held R4 standing approval chain: SQLSTATE=%s err=%v", other, stagePgCode(err), err)
			}
			validRelease := false
			for _, decision := range first.view.Decisions {
				if decision.DecisionID == release && decision.State == "valid" {
					validRelease = true
				}
			}
			if first.err != nil || !validRelease || len(first.view.ExecutionOperations) != 1 || first.view.ExecutionOperations[0].State != "unknown" {
				t.Fatalf("held R4 lost its original issuer authority or unknown intent: release_valid=%t err=%v", validRelease, first.err)
			}
		})
	}
}

func TestStageExecutionReadWriterAccessFences(t *testing.T) {
	for _, relation := range []string{"issue_collaboration", "collaboration_session"} {
		for _, marked := range []bool{false, true} {
			name := relation + "/plain"
			if marked {
				name = relation + "/read_marker"
			}
			t.Run(name, func(t *testing.T) {
				f := newStageReviewFixture(t)
				holder, err := f.pool.Begin(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer holder.Rollback(f.ctx)
				statement := `SELECT issue_id FROM issue_collaboration WHERE issue_id=$1 FOR SHARE NOWAIT`
				if relation == "collaboration_session" {
					statement = `SELECT id FROM collaboration_session WHERE issue_id=$1 FOR SHARE NOWAIT`
				}
				tag, err := holder.Exec(f.ctx, statement, f.issue.Issue.ID)
				if err != nil || tag.RowsAffected() != 1 {
					t.Fatalf("writer control did not hold its exact relation: %v", err)
				}
				writer, err := f.pool.Begin(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer writer.Rollback(f.ctx)
				var tx pgx.Tx = writer
				if marked {
					tx = collaborationStageEvidenceReadTx{Tx: writer}
				}
				access, err := completionCollaborationAccess(f.ctx, tx, f.issue.Issue, util.MustParseUUID(f.creator))
				if stagePgCode(err) != "55P03" || !reflect.DeepEqual(access, CollaborationIssueAccess{}) {
					t.Fatalf("original writer lost its %s exclusive fence with read_marker=%t: SQLSTATE=%s", relation, marked, stagePgCode(err))
				}
			})
		}
	}
}

func TestStageExecutionReadRetainedIssuerFences(t *testing.T) {
	t.Run("reader_first", func(t *testing.T) {
		f, _ := stageStandingExecutionFixture(t)
		held := holdStandingExecutionR4(t, f)
		if err := stageDemoteArtifactManager(t, f.stageRuntimeFixture, f.stage.qa); stagePgCode(err) != "55P03" {
			t.Fatalf("QA issuer demotion crossed the held standing approval chain: %v", err)
		}
		held.release()
		if first := <-held.done; first.err != nil {
			t.Fatal(first.err)
		}
		if err := stageDemoteArtifactManager(t, f.stageRuntimeFixture, f.stage.qa); err != nil {
			t.Fatal(err)
		}
		view := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
		for _, decision := range view.Decisions {
			if decision.Step == "release" && decision.State == "valid" {
				t.Fatal("fresh R4 revived a release after its original QA issuer lost source authority")
			}
		}
	})
	t.Run("revocation_before_authority_lock", func(t *testing.T) {
		f, _ := stageStandingExecutionFixture(t)
		hits := 0
		reader := *f.svc
		reader.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: "GetCollaborationStageCodeHead", before: func() {
			hits++
			if err := stageDemoteArtifactManager(t, f.stageRuntimeFixture, f.stage.qa); err != nil {
				t.Fatal(err)
			}
		}}
		view, err := reader.StageCodeEvidence(f.stage.releaseCtx, f.issue.Issue.ID, f.candidate.CandidateID)
		if hits != 1 || stagePgCode(err) != "40001" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
			t.Fatalf("changed retained QA issuer escaped the RR source fence: hooks=%d SQLSTATE=%s", hits, stagePgCode(err))
		}
	})
}
