package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These tests use the existing real package/authority fixture. They establish
// the candidate GET's own read contract, not the holder of a historical HTTP409.
// The retained fixture does not expose its archive path; database digests below
// are not a fresh filesystem-byte audit.
type completionCandidatesPGFailure struct {
	Query      string    `json:"query"`
	SQLSHA256  string    `json:"sql_sha256"`
	UUIDArgs   []string  `json:"uuid_args"`
	Phase      string    `json:"phase"`
	BackendPID uint32    `json:"backend_pid"`
	SQLState   string    `json:"sqlstate"`
	OuterType  string    `json:"outer_type"`
	Message    string    `json:"message"`
	Detail     string    `json:"detail"`
	Where      string    `json:"where"`
	Table      string    `json:"table"`
	Constraint string    `json:"constraint"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Forward canonical SQL, arguments, rows and Scan destinations unchanged. The
// domain wrapper erases a PgError's SQLSTATE, so retain the first real error at
// Query/Scan/Rows.Err/Exec/Commit before that wrapper runs. No result is injected.
type completionCandidatesTraceTx struct {
	pgx.Tx
	mu      sync.Mutex
	firstPG *completionCandidatesPGFailure
}

func (tx *completionCandidatesTraceTx) capture(sql, phase string, args []any, err error) {
	var p *pgconn.PgError
	if !errors.As(err, &p) {
		return
	}
	query := strings.TrimSpace(strings.SplitN(sql, "\n", 2)[0])
	if fields := strings.Fields(query); len(fields) >= 3 && fields[0] == "--" && fields[1] == "name:" {
		query = fields[2]
	}
	ids := []string{}
	for _, arg := range args {
		switch value := arg.(type) {
		case pgtype.UUID:
			if value.Valid {
				ids = append(ids, util.UUIDToString(value))
			}
		case string:
			if completionUUID(value) {
				ids = append(ids, value)
			}
		}
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.firstPG == nil {
		tx.firstPG = &completionCandidatesPGFailure{Query: query, SQLSHA256: hexHash([]byte(sql)), UUIDArgs: ids,
			Phase: phase, BackendPID: tx.Conn().PgConn().PID(), SQLState: p.Code, OuterType: fmt.Sprintf("%T", err),
			Message: p.Message, Detail: p.Detail, Where: p.Where, Table: p.TableName, Constraint: p.ConstraintName, RecordedAt: time.Now().UTC()}
	}
}

func (tx *completionCandidatesTraceTx) failure() *completionCandidatesPGFailure {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.firstPG == nil {
		return nil
	}
	copy := *tx.firstPG
	copy.UUIDArgs = append([]string{}, copy.UUIDArgs...)
	return &copy
}

func (tx *completionCandidatesTraceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	tx.capture(sql, "exec", args, err)
	return tag, err
}

func (tx *completionCandidatesTraceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return completionCandidatesTraceRow{row: tx.Tx.QueryRow(ctx, sql, args...), tx: tx, sql: sql, args: args}
}

type completionCandidatesTraceRow struct {
	row  pgx.Row
	tx   *completionCandidatesTraceTx
	sql  string
	args []any
}

func (row completionCandidatesTraceRow) Scan(dest ...any) error {
	err := row.row.Scan(dest...)
	row.tx.capture(row.sql, "row_scan", row.args, err)
	return err
}

func (tx *completionCandidatesTraceTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	tx.capture(sql, "query", args, err)
	if err != nil {
		return rows, err
	}
	return &completionCandidatesTraceRows{Rows: rows, tx: tx, sql: sql, args: args}, nil
}

type completionCandidatesTraceRows struct {
	pgx.Rows
	tx   *completionCandidatesTraceTx
	sql  string
	args []any
}

func (rows *completionCandidatesTraceRows) Scan(dest ...any) error {
	err := rows.Rows.Scan(dest...)
	rows.tx.capture(rows.sql, "rows_scan", rows.args, err)
	return err
}

func (rows *completionCandidatesTraceRows) Err() error {
	err := rows.Rows.Err()
	rows.tx.capture(rows.sql, "rows_err", rows.args, err)
	return err
}

func (tx *completionCandidatesTraceTx) Commit(ctx context.Context) error {
	err := tx.Tx.Commit(ctx)
	tx.capture("COMMIT", "commit", nil, err)
	return err
}

type completionCandidatesRead struct {
	ctx context.Context
	tx  *completionCandidatesTraceTx
}

func beginCompletionCandidatesRead(t *testing.T, f *stageReviewFixture, actor context.Context) completionCandidatesRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(actor, 5*time.Second)
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		cancel()
		t.Fatalf("begin actual candidate transaction: %v", err)
	}
	trace := &completionCandidatesTraceTx{Tx: tx}
	t.Cleanup(func() {
		cancel()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback candidate/writer transaction during cleanup: %v", err)
		}
	})
	return completionCandidatesRead{ctx: ctx, tx: trace}
}

func (read completionCandidatesRead) candidates(f *stageReviewFixture, source pgtype.UUID) (CollaborationDeliveryCandidates, error) {
	return f.svc.ListCollaborationDeliveryCandidatesInTx(read.ctx, read.tx, f.issue.Issue.ID, source,
		pgtype.UUID{}, completionRequestCredential(read.ctx), 20, "")
}

func (read completionCandidatesRead) finalCredential() error {
	return CheckArtifactCredentialInTx(read.ctx, read.tx, completionRequestCredential(read.ctx))
}

func (read completionCandidatesRead) rollback(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := read.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Errorf("rollback actual transaction: %v", err)
	}
}

func completionCandidatesLog(t *testing.T, label string, read completionCandidatesRead, err error) {
	t.Helper()
	status, code := 0, ""
	var completion DeliveryCompletionError
	var collaboration *CollaborationError
	if errors.As(err, &completion) {
		status, code = completion.Status, completion.Code
	} else if errors.As(err, &collaboration) {
		status, code = collaboration.Status, collaboration.Code
	}
	witness := struct {
		Label      string                         `json:"label"`
		Actor      string                         `json:"actor"`
		BackendPID uint32                         `json:"backend_pid"`
		ErrorType  string                         `json:"error_type"`
		Status     int                            `json:"status"`
		Code       string                         `json:"code"`
		FirstPG    *completionCandidatesPGFailure `json:"first_pg_error"`
	}{label, completionRequestCredential(read.ctx).Principal.UserID, read.tx.Conn().PgConn().PID(), fmt.Sprintf("%T", err), status, code, read.tx.failure()}
	raw, marshalErr := json.Marshal(witness)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("M1_CANDIDATES_READ_WITNESS %s", raw)
}

func completionCandidatesCommittedRead(t *testing.T, f *stageReviewFixture, actor context.Context, source pgtype.UUID, label string) (CollaborationDeliveryCandidates, error) {
	t.Helper()
	read := beginCompletionCandidatesRead(t, f, actor)
	page, err := read.candidates(f, source)
	if err == nil {
		err = read.finalCredential()
	}
	if err == nil {
		err = read.tx.Commit(read.ctx)
	} else {
		read.rollback(t)
	}
	completionCandidatesLog(t, label, read, err)
	return page, err
}

// The admitted database is exclusive to this serial package selection. Capture
// complete rows and counts, including currently empty effect tables, rather
// than checking only status or a single zero count. These pool queries are
// verification outside product calls; product calls always use their actual Tx.
type completionCandidatesDatabaseState struct {
	Exit   string            `json:"stage_exit"`
	Tables map[string]string `json:"tables"`
}

func completionCandidatesSnapshot(t *testing.T, f *stageReviewFixture) completionCandidatesDatabaseState {
	t.Helper()
	state := completionCandidatesDatabaseState{Exit: stageExitServiceSnapshot(t, f.collaborationStageFixture), Tables: map[string]string{}}
	for _, table := range []string{
		"workspace", "user", "member", "personal_access_token", "user_password_credential", "project", "issue", "issue_status", "issue_participant", "issue_collaboration",
		"project_stage_policy", "project_delivery_policy", "collaboration_stage_task", "collaboration_stage_role", "collaboration_stage_child", "collaboration_stage_candidate",
		"collaboration_stage_decision", "collaboration_stage_revocation", "collaboration_stage_submission", "collaboration_stage_execution_intent", "collaboration_stage_execution_observation", "collaboration_stage_rollback_observation",
		"delivery_evidence_evaluation", "delivery_project_change", "delivery_completion_authorization", "delivery_completion_terminal",
		"collaboration_session", "collaboration_target", "collaboration_channel", "collaboration_batch", "collaboration_command", "collaboration_group_input", "collaboration_execution_group", "collaboration_delegation", "collaboration_group_dependency", "collaboration_grant",
		"collaboration_run", "collaboration_run_event", "collaboration_termination", "collaboration_attempt", "collaboration_test_subject", "collaboration_business_capability", "collaboration_runtime_reservation", "collaboration_runtime_enrollment", "collaboration_execution_gate",
		"collaboration_account_fence", "collaboration_account_invalidation", "collaboration_operation", "collaboration_event", "collaboration_notification", "collaboration_attachment", "collaboration_attachment_reference", "issue_handoff", "comment", "activity_log", "inbox_item", "iteration_event", "iteration_notification",
		"agent", "agent_runtime", "agent_task_queue", "agent_approval_request", "agent_artifact_execution", "task_usage", "task_usage_hourly",
		"github_installation", "github_pull_request", "github_pull_request_check_run", "issue_pull_request", "vcs_connection", "vcs_pull_request", "issue_vcs_pull_request",
	} {
		var rows string
		f.fx.QueryRow(t, `SELECT jsonb_build_object('count',count(*),'rows',COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb))::text FROM "`+table+`" r`).Scan(&rows)
		state.Tables[table] = rows
	}
	return state
}

func completionCandidatesCompareState(t *testing.T, before, after completionCandidatesDatabaseState, label string) {
	t.Helper()
	if before.Exit != after.Exit {
		t.Errorf("%s changed the existing stage-exit snapshot: before_sha256=%s after_sha256=%s", label, hexHash([]byte(before.Exit)), hexHash([]byte(after.Exit)))
	}
	counts := map[string]int64{}
	for table, rows := range before.Tables {
		if rows != after.Tables[table] {
			t.Errorf("%s changed complete %s rows/count: before_sha256=%s after_sha256=%s", label, table, hexHash([]byte(rows)), hexHash([]byte(after.Tables[table])))
		}
		var count struct {
			Count int64 `json:"count"`
		}
		if err := json.Unmarshal([]byte(rows), &count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count.Count
	}
	beforeRaw, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	afterRaw, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	countRaw, err := json.Marshal(counts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("M1_CANDIDATES_NO_WRITES label=%s before_sha256=%s after_sha256=%s counts=%s", label, hexHash(beforeRaw), hexHash(afterRaw), countRaw)
}

func completionCandidatesNoWrites(t *testing.T, f *stageReviewFixture) {
	t.Helper()
	before := completionCandidatesSnapshot(t, f)
	// Registered before concurrent handles, so their cancellation, release,
	// draining and rollback run before the final snapshot, even after Fatal.
	t.Cleanup(func() {
		completionCandidatesCompareState(t, before, completionCandidatesSnapshot(t, f), "read/control")
	})
}

func completionCandidatesFixture(t *testing.T) (*stageReviewFixture, CollaborationStageCandidateRef, CollaborationDeliveryCandidates, CollaborationStageCodeEvidenceView) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("candidate regressions require an explicitly admitted task-owned DATABASE_URL")
	}
	f, candidate := stageArtifactConcurrentReadFixture(t)
	stageExitServiceM1(t, f.collaborationStageFixture, "enforce")
	before := completionCandidatesSnapshot(t, f)
	page, err := completionCandidatesCommittedRead(t, f, f.ctx, util.MustParseUUID(f.child), "unheld_baseline")
	if err != nil {
		completionCandidatesCompareState(t, before, completionCandidatesSnapshot(t, f), "failed_baseline")
		t.Fatalf("real unheld candidate baseline failed before any concurrency assertion: %v", err)
	}
	root, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(f.child))
	if err != nil {
		t.Fatal(err)
	}
	execution, err := f.svc.Queries.GetAgentArtifactExecutionByApproval(f.ctx, f.artifact.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := f.svc.Queries.GetAgentApprovalRequest(f.ctx, f.artifact.ApprovalID)
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentIssueID != root.ID || child.WorkspaceID != root.WorkspaceID || child.ProjectID != root.ProjectID || root.WorkspaceID != util.MustParseUUID(f.workspace) || root.ProjectID != util.MustParseUUID(f.project) || child.ID == root.ID || execution.ResourceID != child.ID || execution.WorkspaceID != child.WorkspaceID ||
		page.ProtocolVersion != 1 || page.IssueID != util.UUIDToString(root.ID) || page.IssueRevision != root.Revision || page.SourceIssueID != f.child || page.SourceIssueRevision != child.Revision || page.IssueRevision < 0 || page.SourceIssueRevision < 0 || page.ProjectID == nil || *page.ProjectID != f.project || page.TargetContext != nil || page.NextCursor != nil || page.HasMore || len(page.Items) != 1 || page.CheckedAt.IsZero() ||
		f.fx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE resource_id=$1 AND workspace_id=$2`, f.child, f.workspace) != 1 {
		t.Fatalf("baseline did not return the exact current root/source/project and single real artifact: %+v", page)
	}
	item := page.Items[0]
	if item.ApprovalID != util.UUIDToString(execution.ApprovalID) || item.ApprovalID != util.UUIDToString(f.artifact.ApprovalID) || item.ExecutionID != util.UUIDToString(execution.ExecutionID) || item.SourceRevision != f.head || item.SourceRevision != execution.SourceRevision || item.SpecDigest != execution.SpecDigest || !DeliveryHex(item.SpecDigest, 64) ||
		item.ArtifactDigest == nil || *item.ArtifactDigest != execution.OutputArtifactDigest || !DeliveryHex(*item.ArtifactDigest, 64) || item.ReceiptDigest == nil || *item.ReceiptDigest != execution.ReceiptDigest || !DeliveryHex(*item.ReceiptDigest, 64) ||
		!item.CreatedAt.Equal(execution.CreatedAt.Time) || !item.ExpiresAt.Equal(execution.ExpiresAt.Time) || item.Summary != approval.Summary || item.Status != "succeeded" || item.ResultCode == nil || *item.ResultCode != execution.ResultCode || !item.Selectable || !page.CheckedAt.Before(item.ExpiresAt) || !page.CheckedAt.Before(completionRequestCredential(f.ctx).Principal.ExpiresAt) {
		t.Fatalf("baseline did not retain exact selectable canonical package/receipt metadata: %+v", item)
	}
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	defer cancel()
	r4, err := f.svc.StageCodeEvidence(ctx, root.ID, candidate.CandidateID)
	completionCandidatesCompareState(t, before, completionCandidatesSnapshot(t, f), "unheld_baselines")
	if err != nil || r4.Candidate.Baseline == nil || r4.Candidate.Baseline.Artifact == nil || r4.Candidate.State != "current" || r4.Candidate.CandidateID != util.UUIDToString(candidate.CandidateID) || r4.Candidate.Baseline.Artifact.ApprovalID != item.ApprovalID || r4.Candidate.Baseline.Artifact.ArtifactDigest != *item.ArtifactDigest || r4.Observed.CI.State != "passed" {
		t.Fatalf("real unheld R4 baseline is not the exact current artifact candidate: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	return f, candidate, page, r4
}

func completionCandidatesComparePage(t *testing.T, got, want CollaborationDeliveryCandidates) {
	t.Helper()
	if got.CheckedAt.IsZero() {
		t.Error("successful candidate page has no actual final database clock")
	}
	for _, item := range got.Items {
		if !got.CheckedAt.Before(item.ExpiresAt) || !item.Selectable {
			t.Errorf("exact candidate stopped being selectable within the reader deadline: approval=%s", item.ApprovalID)
		}
	}
	got.CheckedAt, want.CheckedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidate read changed the exact authorized page: got=%+v want=%+v", got, want)
	}
}

func completionCandidatesCompareR4(t *testing.T, got, want CollaborationStageCodeEvidenceView, err error) {
	t.Helper()
	if err != nil {
		if !reflect.DeepEqual(got, CollaborationStageCodeEvidenceView{}) {
			t.Error("failed concurrent R4 exposed a partial private projection")
		}
		t.Errorf("authorized R4 contended with a candidate reader: SQLSTATE=%s err=%v", stagePgCode(err), err)
		return
	}
	if got.CheckedAt.IsZero() || got.ProtocolVersion != want.ProtocolVersion || got.IssueID != want.IssueID || got.BlockedEvidence != want.BlockedEvidence ||
		!reflect.DeepEqual(got.Candidate, want.Candidate) || !reflect.DeepEqual(got.Observed, want.Observed) || !reflect.DeepEqual(got.Decisions, want.Decisions) || !reflect.DeepEqual(got.TestReports, want.TestReports) ||
		!reflect.DeepEqual(got.ExecutionFacts, want.ExecutionFacts) || !reflect.DeepEqual(got.ExecutionTargets, want.ExecutionTargets) || !reflect.DeepEqual(got.ExecutionOperations, want.ExecutionOperations) || !reflect.DeepEqual(got.RollbackObservations, want.RollbackObservations) || !reflect.DeepEqual(got.Signable, want.Signable) {
		t.Errorf("R4 lost exact current candidate/artifact/decision identities: candidate=%s", got.Candidate.CandidateID)
	}
}

func completionCandidatesDistinctPIDs(t *testing.T, first, second uint32) {
	t.Helper()
	if first == 0 || second == 0 || first == second {
		t.Errorf("concurrent product calls require distinct real PostgreSQL backends: first=%d second=%d", first, second)
	}
	t.Logf("M1_CANDIDATES_BACKENDS first=%d second=%d", first, second)
}

func TestCollaborationCompletionCandidatesR4Coexistence(t *testing.T) {
	t.Run("r4_first", func(t *testing.T) {
		f, candidate, baseline, r4Baseline := completionCandidatesFixture(t)
		completionCandidatesNoWrites(t, f)
		held := holdStageArtifactR4(t, f, candidate)
		second := beginCompletionCandidatesRead(t, f, f.ctx)
		completionCandidatesDistinctPIDs(t, held.pid, second.tx.Conn().PgConn().PID())
		page, err := second.candidates(f, util.MustParseUUID(f.child))
		if err == nil {
			err = second.finalCredential()
		}
		if err == nil {
			err = second.tx.Commit(second.ctx)
		} else {
			second.rollback(t)
		}
		// The second reader has returned and committed/rolled back before release.
		completionCandidatesLog(t, "r4_first_second_terminal_before_release", second, err)
		held.release()
		first := <-held.done
		completionCandidatesCompareR4(t, first.view, r4Baseline, first.err)
		if err != nil {
			if !reflect.DeepEqual(page, CollaborationDeliveryCandidates{}) {
				t.Error("failed candidate read exposed a partial page")
			}
			t.Errorf("candidate GET contended with a real held R4: first_pg=%+v domain=%v", second.tx.failure(), err)
			return
		}
		completionCandidatesComparePage(t, page, baseline)
	})
	t.Run("candidates_first", func(t *testing.T) {
		f, candidate, baseline, r4Baseline := completionCandidatesFixture(t)
		completionCandidatesNoWrites(t, f)
		first := beginCompletionCandidatesRead(t, f, f.ctx)
		page, err := first.candidates(f, util.MustParseUUID(f.child))
		if err == nil {
			err = first.finalCredential()
		}
		completionCandidatesLog(t, "candidates_first_held_return", first, err)
		if err != nil {
			t.Fatalf("unheld candidate read failed before the R4 control: %v", err)
		}
		completionCandidatesComparePage(t, page, baseline)
		ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
		defer cancel()
		began := make(chan uint32, 1)
		reader := *f.svc
		reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
		view, secondErr := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, candidate.CandidateID)
		select {
		case pid := <-began:
			completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), pid)
		default:
			t.Error("second R4 did not begin an actual transaction")
		}
		// StageCodeEvidence's actual terminal precedes the held candidate commit.
		if err = first.tx.Commit(first.ctx); err != nil {
			t.Errorf("commit held candidate transaction after R4 terminal: %v", err)
		}
		completionCandidatesLog(t, "candidates_first_commit_after_r4_terminal", first, err)
		completionCandidatesCompareR4(t, view, r4Baseline, secondErr)
	})
}

func completionCandidatesSiblingReads(t *testing.T, differentManager bool) {
	t.Helper()
	f, _, baseline, _ := completionCandidatesFixture(t)
	actor := f.ctx
	if differentManager {
		actor = f.reviewerCtx
		if f.creator == f.reviewer {
			t.Fatal("different-manager control requires two actual authorized humans")
		}
		prior, err := completionCandidatesCommittedRead(t, f, actor, util.MustParseUUID(f.child), "unheld_reviewer_baseline")
		if err != nil {
			t.Fatalf("actual current reviewer/manager candidate baseline: %v", err)
		}
		completionCandidatesComparePage(t, prior, baseline)
	}
	completionCandidatesNoWrites(t, f)
	first := beginCompletionCandidatesRead(t, f, f.ctx)
	firstPage, err := first.candidates(f, util.MustParseUUID(f.child))
	if err == nil {
		err = first.finalCredential()
	}
	completionCandidatesLog(t, "sibling_first_held_return", first, err)
	if err != nil {
		t.Fatalf("first unheld sibling candidate read: %v", err)
	}
	completionCandidatesComparePage(t, firstPage, baseline)
	second := beginCompletionCandidatesRead(t, f, actor)
	completionCandidatesDistinctPIDs(t, first.tx.Conn().PgConn().PID(), second.tx.Conn().PgConn().PID())
	page, secondErr := second.candidates(f, util.MustParseUUID(f.child))
	if secondErr == nil {
		secondErr = second.finalCredential()
	}
	if secondErr == nil {
		secondErr = second.tx.Commit(second.ctx)
	} else {
		second.rollback(t)
	}
	completionCandidatesLog(t, "sibling_second_terminal_before_first_commit", second, secondErr)
	if err = first.tx.Commit(first.ctx); err != nil {
		t.Errorf("commit first sibling after second terminal: %v", err)
	}
	if secondErr != nil {
		if !reflect.DeepEqual(page, CollaborationDeliveryCandidates{}) {
			t.Error("failed sibling read exposed a partial page")
		}
		t.Errorf("two real candidate readers contended: first_pg=%+v domain=%v", second.tx.failure(), secondErr)
		return
	}
	completionCandidatesComparePage(t, page, baseline)
}

func TestCollaborationCompletionCandidatesSiblingReads(t *testing.T) {
	t.Run("same_creator", func(t *testing.T) { completionCandidatesSiblingReads(t, false) })
	t.Run("different_current_managers", func(t *testing.T) { completionCandidatesSiblingReads(t, true) })
}

func TestCollaborationCompletionCandidatesWriterFences(t *testing.T) {
	t.Run("writer_first", func(t *testing.T) {
		f, _, baseline, _ := completionCandidatesFixture(t)
		completionCandidatesNoWrites(t, f)
		access, child, request := stageProjectionWriterRequest(t, f)
		writer := beginCompletionCandidatesRead(t, f, f.ctx)
		observation, err := EvaluateDeliveryEvidenceInTx(writer.ctx, writer.tx, access, child, request)
		completionCandidatesLog(t, "writer_first_actual_observer", writer, err)
		if err != nil || observation.Verdict != "pass" || observation.ApprovalID != baseline.Items[0].ApprovalID || observation.ExecutionID != baseline.Items[0].ExecutionID || observation.ArtifactDigest != *baseline.Items[0].ArtifactDigest || observation.ValidUntil == nil {
			t.Fatalf("unchanged real observer did not establish the writer fence: verdict=%s err=%v", observation.Verdict, err)
		}
		reader := beginCompletionCandidatesRead(t, f, f.reviewerCtx)
		completionCandidatesDistinctPIDs(t, writer.tx.Conn().PgConn().PID(), reader.tx.Conn().PgConn().PID())
		page, readErr := reader.candidates(f, util.MustParseUUID(f.child))
		completionCandidatesLog(t, "writer_first_reader_terminal_before_release", reader, readErr)
		reader.rollback(t)
		failure := reader.tx.failure()
		var refusal DeliveryCompletionError
		if failure == nil || failure.SQLState != "55P03" || !errors.As(readErr, &refusal) || refusal.Status != 409 || refusal.Code != "delivery_completion_conflict" || !reflect.DeepEqual(page, CollaborationDeliveryCandidates{}) {
			t.Errorf("candidate read crossed the real writer or exposed a partial page: first_pg=%+v domain=%v", failure, readErr)
		}
		writer.rollback(t)
		fresh, err := completionCandidatesCommittedRead(t, f, f.ctx, util.MustParseUUID(f.child), "writer_first_fresh_after_rollback")
		if err != nil {
			t.Fatalf("fresh read after actual writer rollback: %v", err)
		}
		completionCandidatesComparePage(t, fresh, baseline)
	})
	t.Run("reader_first", func(t *testing.T) {
		f, _, baseline, _ := completionCandidatesFixture(t)
		completionCandidatesNoWrites(t, f)
		access, child, request := stageProjectionWriterRequest(t, f)
		reader := beginCompletionCandidatesRead(t, f, f.ctx)
		page, err := reader.candidates(f, util.MustParseUUID(f.child))
		if err == nil {
			err = reader.finalCredential()
		}
		completionCandidatesLog(t, "reader_first_held_return", reader, err)
		if err != nil {
			t.Fatalf("first real candidate read before writer: %v", err)
		}
		completionCandidatesComparePage(t, page, baseline)
		writer := beginCompletionCandidatesRead(t, f, f.ctx)
		completionCandidatesDistinctPIDs(t, reader.tx.Conn().PgConn().PID(), writer.tx.Conn().PgConn().PID())
		afterFence, resume := make(chan uint32, 1), make(chan struct{})
		close(resume)
		probe := &stageProjectionBarrierTx{Tx: writer.tx, needle: "LockArtifactUsers", reached: afterFence, resume: resume}
		observation, writeErr := EvaluateDeliveryEvidenceInTx(writer.ctx, probe, access, child, request)
		completionCandidatesLog(t, "reader_first_writer_terminal_before_reader_commit", writer, writeErr)
		failure := writer.tx.failure()
		if stagePgCode(writeErr) != "55P03" || failure == nil || failure.Query != "LockLifecycleIssue" || len(afterFence) != 0 || observation.Verdict == "pass" || observation.IssueID != "" {
			t.Errorf("writer lost its upfront exclusive lifecycle fence: first_pg=%+v crossed_authority=%t verdict=%s err=%v", failure, len(afterFence) != 0, observation.Verdict, writeErr)
		}
		writer.rollback(t)
		if err = reader.tx.Commit(reader.ctx); err != nil {
			t.Errorf("commit candidate after refused writer rollback: %v", err)
		}
		fresh, err := completionCandidatesCommittedRead(t, f, f.ctx, util.MustParseUUID(f.child), "reader_first_fresh_after_commit")
		if err != nil {
			t.Fatalf("fresh candidate read after reader commit: %v", err)
		}
		completionCandidatesComparePage(t, fresh, baseline)
	})
}

func completionCandidatesWantRefusal(t *testing.T, page CollaborationDeliveryCandidates, err error, status int, code string, collaborationDomain bool) {
	t.Helper()
	if !reflect.DeepEqual(page, CollaborationDeliveryCandidates{}) {
		t.Error("current authority refusal exposed private candidate metadata/counts")
	}
	if collaborationDomain {
		var refusal *CollaborationError
		if !errors.As(err, &refusal) || refusal.Status != status || refusal.Code != code {
			t.Errorf("want current collaboration authority %d/%s, got %T %v", status, code, err, err)
		}
		return
	}
	var refusal DeliveryCompletionError
	if !errors.As(err, &refusal) || refusal.Status != status || refusal.Code != code {
		t.Errorf("want current delivery authority %d/%s, got %T %v", status, code, err, err)
	}
}

func TestCollaborationCompletionCandidatesCurrentAuthority(t *testing.T) {
	t.Run("wrong_workspace_child", func(t *testing.T) {
		f, _, _, _ := completionCandidatesFixture(t)
		foreign := f.fx.Workspace(t, "Other candidate source", "candidate-foreign-"+uuid.NewString())
		child := f.fx.Issue(t, "Other workspace ordinary child", testutil.Cols{"workspace_id": foreign})
		completionCandidatesNoWrites(t, f)
		page, err := completionCandidatesCommittedRead(t, f, f.ctx, util.MustParseUUID(child), "wrong_workspace_child")
		completionCandidatesWantRefusal(t, page, err, 404, "delivery_completion_not_found", false)
	})
	t.Run("outside_participant", func(t *testing.T) {
		f, _, _, _ := completionCandidatesFixture(t)
		access, err := f.svc.ResolveIssueAccess(f.handlerCtx, nil, f.issue.Issue.ID)
		if err != nil || !access.Capabilities.Read || access.SourceMember || !access.ParticipantEpoch.Valid || f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
			t.Fatalf("external participant must have real root participation and no source membership: %v", err)
		}
		completionCandidatesNoWrites(t, f)
		page, err := completionCandidatesCommittedRead(t, f, f.handlerCtx, util.MustParseUUID(f.child), "outside_participant")
		completionCandidatesWantRefusal(t, page, err, 403, "delivery_completion_denied", false)
	})
	t.Run("disabled_account", func(t *testing.T) {
		f, _, _, _ := completionCandidatesFixture(t)
		f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.creator)
		if f.fx.Count(t, `SELECT count(*) FROM "user" WHERE id=$1 AND disabled_at IS NOT NULL`, f.creator) != 1 {
			t.Fatal("disabled-account control did not update the real current account")
		}
		completionCandidatesNoWrites(t, f)
		page, err := completionCandidatesCommittedRead(t, f, f.ctx, util.MustParseUUID(f.child), "disabled_account")
		completionCandidatesWantRefusal(t, page, err, 404, "not_found", true)
	})
	t.Run("expired_credential", func(t *testing.T) {
		f, _, _, _ := completionCandidatesFixture(t)
		principal, ok := auth.AuthenticatedPrincipalFromContext(f.ctx)
		if !ok || principal.UserID != f.creator || principal.Kind != auth.CredentialJWT {
			t.Fatal("expiry control requires the actual fixture creator's verified JWT identity")
		}
		principal.ExpiresAt = time.Now().UTC().Add(-time.Minute)
		actor := auth.WithAuthenticatedPrincipal(f.ctx, principal)
		completionCandidatesNoWrites(t, f)
		page, err := completionCandidatesCommittedRead(t, f, actor, util.MustParseUUID(f.child), "expired_credential")
		completionCandidatesWantRefusal(t, page, err, 403, "delivery_completion_denied", false)
	})
	t.Run("revoked_artifact_manager", func(t *testing.T) {
		f, _, baseline, _ := completionCandidatesFixture(t)
		prior, err := completionCandidatesCommittedRead(t, f, f.reviewerCtx, util.MustParseUUID(f.child), "manager_before_demotion")
		if err != nil {
			t.Fatalf("current manager's real positive candidate baseline: %v", err)
		}
		completionCandidatesComparePage(t, prior, baseline)
		member, err := f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(f.reviewer)})
		if err != nil || member.Role != "admin" {
			t.Fatalf("manager control lacks actual current admin membership: %v", err)
		}
		mutation := beginCompletionCandidatesRead(t, f, f.ctx)
		changed, err := db.New(mutation.tx).UpdateMemberRole(mutation.ctx, db.UpdateMemberRoleParams{ID: member.ID, Role: "member"})
		if err != nil || changed.Role != "member" {
			t.Fatalf("actual manager revocation: %v", err)
		}
		if err = mutation.tx.Commit(mutation.ctx); err != nil {
			t.Fatal(err)
		}
		// The deliberate setup mutation belongs to this control's own baseline.
		completionCandidatesNoWrites(t, f)
		page, err := completionCandidatesCommittedRead(t, f, f.reviewerCtx, util.MustParseUUID(f.child), "manager_after_demotion")
		if err != nil {
			t.Fatalf("authorized root/source read after manager demotion: %v", err)
		}
		want := baseline
		want.Items = []DeliveryEvidenceCandidate{}
		completionCandidatesComparePage(t, page, want)
	})
}
