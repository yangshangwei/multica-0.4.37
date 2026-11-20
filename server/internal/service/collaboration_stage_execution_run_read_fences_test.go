package service

import (
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageExecutionStandingReportRun(t *testing.T, f *stageExecutionFixture) (pgtype.UUID, string) {
	t.Helper()
	view := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
	if len(view.TestReports) != 1 {
		t.Fatal("standing execution fixture did not retain its one actual QA report")
	}
	return util.MustParseUUID(view.TestReports[0].RunID), view.TestReports[0].ReportID
}

func TestStageExecutionRunReadFences(t *testing.T) {
	for _, marked := range []bool{false, true} {
		name := "plain"
		if marked {
			name = "read_marker"
		}
		t.Run(name, func(t *testing.T) {
			f, _ := stageStandingExecutionFixture(t)
			runID, reportID := stageExecutionStandingReportRun(t, f)
			holder, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(f.ctx)
			tag, err := holder.Exec(f.ctx, `SELECT id FROM collaboration_run WHERE id=$1 FOR SHARE NOWAIT`, runID)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("writer control did not hold its exact run shared fence: %v", err)
			}
			writer, err := f.pool.Begin(f.stage.qaCtx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback(f.stage.qaCtx)
			var tx pgx.Tx = writer
			if marked {
				tx = collaborationStageEvidenceReadTx{Tx: writer}
			}
			q := db.New(tx)
			access, err := f.svc.ResolveIssueAccess(f.stage.qaCtx, q, f.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := q.GetCollaborationStageCodeCandidate(f.stage.qaCtx, db.GetCollaborationStageCodeCandidateParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID})
			if err != nil {
				t.Fatal(err)
			}
			_, code, err := stageQAReportAs(f.stage.qaCtx, tx, access, candidate, reportID, true, completionRequestCredential(f.stage.qaCtx))
			if stagePgCode(err) != "55P03" || code != "" || writer.Conn().PgConn().PID() == holder.Conn().PgConn().PID() {
				t.Fatalf("original QA report writer lost its exclusive run fence with read_marker=%t: code=%s SQLSTATE=%s", marked, code, stagePgCode(err))
			}
		})
	}
	t.Run("writer_first", func(t *testing.T) {
		f, _ := stageStandingExecutionFixture(t)
		runID, _ := stageExecutionStandingReportRun(t, f)
		writer, err := f.pool.Begin(f.stage.qaCtx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(f.stage.qaCtx)
		row, err := db.New(writer).LockCollaborationStageTestRun(f.stage.qaCtx, runID)
		if err != nil || row.ID != runID {
			t.Fatalf("writer did not hold its exact original run update fence: %v", err)
		}
		began := make(chan uint32, 1)
		reader := *f.svc
		reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
		view, err := reader.StageCodeEvidence(f.stage.releaseCtx, f.issue.Issue.ID, f.candidate.CandidateID)
		if stagePgCode(err) != "55P03" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
			t.Fatalf("R4 crossed the original run writer fence or returned partial evidence: SQLSTATE=%s", stagePgCode(err))
		}
		select {
		case pid := <-began:
			if pid == writer.Conn().PgConn().PID() {
				t.Fatal("run writer control did not use distinct PostgreSQL connections")
			}
		default:
			t.Fatal("R4 did not begin an actual transaction")
		}
	})
	t.Run("reader_first", func(t *testing.T) {
		f, _ := stageStandingExecutionFixture(t)
		runID, _ := stageExecutionStandingReportRun(t, f)
		held := holdStandingExecutionR4(t, f)
		writer, err := f.pool.Begin(f.stage.qaCtx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(f.stage.qaCtx)
		if writer.Conn().PgConn().PID() == held.pid {
			t.Fatal("run reader control did not use distinct PostgreSQL connections")
		}
		_, err = db.New(writer).LockCollaborationStageTestRun(f.stage.qaCtx, runID)
		rollbackErr := writer.Rollback(f.stage.qaCtx)
		if stagePgCode(err) != "55P03" || rollbackErr != nil {
			t.Fatalf("original run writer crossed the held R4 fence: SQLSTATE=%s rollback=%v", stagePgCode(err), rollbackErr)
		}
		held.release()
		if first := <-held.done; first.err != nil {
			t.Fatal(first.err)
		}
		writer, err = f.pool.Begin(f.stage.qaCtx)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Rollback(f.stage.qaCtx)
		row, err := db.New(writer).LockCollaborationStageTestRun(f.stage.qaCtx, runID)
		if err != nil || row.ID != runID {
			t.Fatalf("original run writer could not lock after the reader committed: %v", err)
		}
	})
}
