package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type r31ClaimResult struct {
	tasks []db.AgentTaskQueue
	err   error
}

// Forward the real task transaction. The optional barrier holds returned
// canonical reclaim rows; no SQL, actor, permission or result is substituted.
type r31ClaimProbe struct {
	TxStarter
	mu             sync.Mutex
	tx             *completionCandidatesTraceTx
	pid            uint32
	accountQuery   string
	accountDigest  string
	accountIDs     []string
	accountErr     error
	accountEntered chan uint32
	reclaimReached chan uint32
	resume         <-chan struct{}
	accountOnce    sync.Once
	reclaimOnce    sync.Once
}

type r31ClaimProbeTx struct {
	*completionCandidatesTraceTx
	owner *r31ClaimProbe
}

type r31ClaimProbeRow struct {
	pgx.Row
	owner *r31ClaimProbe
}

func (p *r31ClaimProbe) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := p.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	trace := &completionCandidatesTraceTx{Tx: tx}
	p.mu.Lock()
	p.tx, p.pid = trace, tx.Conn().PgConn().PID()
	p.mu.Unlock()
	return &r31ClaimProbeTx{completionCandidatesTraceTx: trace, owner: p}, nil
}

func (tx *r31ClaimProbeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if !strings.Contains(sql, "-- name: LockPasswordUser ") {
		return tx.completionCandidatesTraceTx.QueryRow(ctx, sql, args...)
	}
	tx.owner.mu.Lock()
	tx.owner.accountQuery, tx.owner.accountDigest = "LockPasswordUser", hexHash([]byte(sql))
	for _, arg := range args {
		if id, ok := arg.(pgtype.UUID); ok && id.Valid {
			tx.owner.accountIDs = []string{util.UUIDToString(id)}
		}
	}
	tx.owner.mu.Unlock()
	tx.owner.accountOnce.Do(func() { tx.owner.accountEntered <- tx.Conn().PgConn().PID() })
	return r31ClaimProbeRow{Row: tx.completionCandidatesTraceTx.QueryRow(ctx, sql, args...), owner: tx.owner}
}

func (row r31ClaimProbeRow) Scan(dest ...any) error {
	err := row.Row.Scan(dest...)
	if err != nil {
		row.owner.mu.Lock()
		row.owner.accountErr = err
		row.owner.mu.Unlock()
	}
	return err
}

func (tx *r31ClaimProbeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.completionCandidatesTraceTx.Query(ctx, sql, args...)
	if err == nil && tx.owner.resume != nil && strings.Contains(sql, "-- name: ReclaimStaleDispatchedTasksForRuntimes ") {
		tx.owner.reclaimOnce.Do(func() {
			tx.owner.reclaimReached <- tx.Conn().PgConn().PID()
			select {
			case <-tx.owner.resume:
			case <-ctx.Done():
			}
		})
	}
	return rows, err
}

type r31ClaimRequest struct {
	probe   *r31ClaimProbe
	done    <-chan struct{}
	result  *r31ClaimResult
	cancel  context.CancelFunc
	release func()
}

func r31StartClaim(t *testing.T, f *stageCapabilitiesReadFixture, hold bool) r31ClaimRequest {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.daemon.daemonCtx, 5*time.Second)
	resume := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	probe := &r31ClaimProbe{TxStarter: f.pool, accountEntered: make(chan uint32, 1), reclaimReached: make(chan uint32, 1)}
	if hold {
		probe.resume = resume
	}
	svc := NewTaskService(db.New(f.pool), probe, nil, events.New())
	if svc.EmptyClaim != nil || svc.ReclaimCheck != nil {
		t.Fatal("ordinary no-Redis claim did not retain its supported nil cache path")
	}
	done, result := make(chan struct{}), &r31ClaimResult{}
	go func() {
		defer close(done)
		result.tasks, result.err = svc.ClaimTasksForRuntimes(ctx, []pgtype.UUID{f.daemon.runtime.ID}, 1)
	}()
	t.Cleanup(func() { release(); cancel(); <-done })
	return r31ClaimRequest{probe: probe, done: done, result: result, cancel: cancel, release: release}
}

func (r r31ClaimRequest) trace() *completionCandidatesTraceTx {
	r.probe.mu.Lock()
	defer r.probe.mu.Unlock()
	return r.probe.tx
}

func r31ClaimWitness(t *testing.T, f *stageCapabilitiesReadFixture, r r31ClaimRequest, label string) {
	t.Helper()
	r.probe.mu.Lock()
	query, digest, ids, accountErr := r.probe.accountQuery, r.probe.accountDigest, append([]string{}, r.probe.accountIDs...), r.probe.accountErr
	pid := r.probe.pid
	r.probe.mu.Unlock()
	var firstPG *completionCandidatesPGFailure
	if trace := r.trace(); trace != nil {
		firstPG = trace.failure()
	}
	witness := map[string]any{"label": label, "actor": f.stage.qa, "backend_pid": pid, "account_query": query, "account_sql_sha256": digest, "account_uuid_args": ids,
		"account_error_type": fmt.Sprintf("%T", accountErr), "account_error": fmt.Sprint(accountErr), "first_pg_error": firstPG, "tasks": len(r.result.tasks), "error_type": fmt.Sprintf("%T", r.result.err), "error": fmt.Sprint(r.result.err)}
	raw, err := json.Marshal(witness)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ORDINARY_IDLE_CLAIM_WITNESS %s", raw)
}

func r31Fixture(t *testing.T) *stageCapabilitiesReadFixture {
	t.Helper()
	f := newStageCapabilitiesReadFixture(t)
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE runtime_id=$1`, f.daemon.runtime.ID) != 0 {
		t.Fatal("selected QA runtime contains ordinary queued/dispatched/deferred/finalization work before the idle baseline")
	}
	session, ok := auth.PasswordSessionFromContext(f.daemon.daemonCtx)
	if !ok || session.UserID != f.stage.qa || f.daemon.runtime.OwnerID != util.MustParseUUID(f.stage.qa) || session.BindingID != util.UUIDToString(f.daemon.binding.ID) || session.BindingEpoch != f.daemon.binding.BindingEpoch {
		t.Fatal("idle claim and artifact R4 do not use the exact current QA managed identity")
	}
	return f
}

// The canonical 72-relation projection is reused, with account-binding/token
// histories added because an ordinary claim must not mint or backfill them.
func r31Snapshot(t *testing.T, f *stageCapabilitiesReadFixture) completionCandidatesDatabaseState {
	t.Helper()
	state := completionCandidatesSnapshot(t, f.stage)
	for _, table := range []string{"installation_daemon_binding", "managed_installation", "task_token", "daemon_token"} {
		var rows string
		f.fx.QueryRow(t, `SELECT jsonb_build_object('count',count(*),'rows',COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb))::text FROM "`+table+`" r`).Scan(&rows)
		state.Tables[table] = rows
	}
	return state
}

func r31NoWrites(t *testing.T, f *stageCapabilitiesReadFixture) {
	t.Helper()
	before := r31Snapshot(t, f)
	t.Cleanup(func() { completionCandidatesCompareState(t, before, r31Snapshot(t, f), "ordinary idle claim/read") })
}

func r31R4(t *testing.T, f *stageCapabilitiesReadFixture) (CollaborationStageCodeEvidenceView, error, uint32, *completionCandidatesPGFailure) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.stage.qaCtx, 5*time.Second)
	defer cancel()
	var trace *completionCandidatesTraceTx
	var pid uint32
	svc := *f.svc
	svc.TxStarter = r31ReadStarter{TxStarter: f.pool, captured: &trace, pid: &pid}
	view, err := svc.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
	if trace == nil {
		t.Fatal("R4 did not open its real borrowed transaction")
	}
	raw, marshalErr := json.Marshal(map[string]any{"label": "R31_current_QA_R4_terminal", "actor": f.stage.qa, "backend_pid": pid, "error_type": fmt.Sprintf("%T", err), "error": fmt.Sprint(err), "first_pg_error": trace.failure()})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Logf("ORDINARY_IDLE_R4_WITNESS %s", raw)
	return view, err, pid, trace.failure()
}

type r31ReadStarter struct {
	TxStarter
	captured **completionCandidatesTraceTx
	pid      *uint32
}

func (s r31ReadStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	*s.pid = tx.Conn().PgConn().PID()
	*s.captured = &completionCandidatesTraceTx{Tx: tx}
	return *s.captured, nil
}

func r31WantR4(t *testing.T, f *stageCapabilitiesReadFixture, got CollaborationStageCodeEvidenceView, err error) {
	t.Helper()
	if err != nil || got.Candidate.CandidateID != f.evidence.Candidate.CandidateID || got.Candidate.State != f.evidence.Candidate.State || !reflect.DeepEqual(got.Candidate.Baseline, f.evidence.Candidate.Baseline) || !reflect.DeepEqual(got.TestReports, f.evidence.TestReports) {
		t.Errorf("same-QA R4 lost its original artifact projection: err=%v candidate=%+v", err, got.Candidate)
	}
}

func r31AwaitHoldOrEmpty(t *testing.T, r r31ClaimRequest, allowEmpty bool) uint32 {
	t.Helper()
	select {
	case pid := <-r.probe.reclaimReached:
		if trace := r.trace(); trace == nil || trace.Conn().PgConn().TxStatus() != 'T' {
			t.Fatal("actual reclaim barrier did not retain its acquired account transaction")
		}
		return pid
	case <-r.done:
		if !allowEmpty || r.result.err != nil || len(r.result.tasks) != 0 {
			t.Fatalf("actual claim terminated before its reclaim barrier: tasks=%d err=%v", len(r.result.tasks), r.result.err)
		}
		return 0 // A future genuine no-work preflight need not enter the old fence.
	case <-time.After(5 * time.Second):
		t.Fatal("claim did not reach its real reclaim barrier or no-work terminal")
	}
	return 0
}

// Observe the canonical blocking FOR UPDATE on the actual second backend.
// Cancellation is test-only; do not relabel it as a NOWAIT SQLSTATE55P03.
func r31CancelBlockedClaim(t *testing.T, f *stageCapabilitiesReadFixture, r r31ClaimRequest, readerPID uint32) bool {
	t.Helper()
	var pid uint32
	select {
	case pid = <-r.probe.accountEntered:
	case <-r.done:
		return false
	case <-time.After(5 * time.Second):
		t.Fatal("claim did not enter its real account query or terminate")
	}
	completionCandidatesDistinctPIDs(t, pid, readerPID)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid=$1 AND wait_event_type='Lock' AND query LIKE '-- name: LockPasswordUser%' AND $2::int=ANY(pg_blocking_pids(pid)))`, int32(pid), int32(readerPID)).Scan(&blocked)
		if err != nil {
			t.Fatalf("observe actual ordinary account wait: %v", err)
		}
		if blocked {
			t.Logf("ORDINARY_IDLE_ACCOUNT_WAIT pid=%d reader_pid=%d query=LockPasswordUser blocked=true", pid, readerPID)
			break
		}
		select {
		case <-r.done:
			return false
		case <-ctx.Done():
			t.Fatal("ordinary claim did not reach the held R4 account fence")
		case <-ticker.C:
		}
	}
	select {
	case <-r.done:
		t.Fatal("blocked ordinary claim completed before diagnostic cancellation")
	case <-ctx.Done():
		r.cancel()
	}
	<-r.done
	if r.result.err == nil || len(r.result.tasks) != 0 {
		t.Fatal("cancelled blocked claim returned a task or success")
	}
	r.probe.mu.Lock()
	accountErr, ids := r.probe.accountErr, append([]string{}, r.probe.accountIDs...)
	r.probe.mu.Unlock()
	if accountErr == nil || !reflect.DeepEqual(ids, []string{f.stage.qa}) {
		t.Fatal("actual cancelled account query lost the singleton QA/error witness")
	}
	return true
}

func r31SeedReclaim(t *testing.T, f *stageCapabilitiesReadFixture) db.AgentTaskQueue {
	t.Helper()
	issue := f.fx.Issue(t, "Actual stale ordinary QA delivery", testutil.Cols{"creator_id": f.stage.qa, "status": "in_progress"})
	id := f.fx.Task(t, util.UUIDToString(f.daemon.grant.AgentID), testutil.Cols{"issue_id": issue, "runtime_id": f.daemon.runtime.ID, "status": "dispatched", "dispatched_at": testutil.Raw("now()-interval '10 minutes'"), "started_at": nil, "prepare_lease_expires_at": nil, "originator_user_id": f.stage.qa, "accountable_user_id": f.stage.qa,
		"execution_binding_id": f.daemon.binding.ID, "execution_binding_epoch": f.daemon.binding.BindingEpoch, "execution_installation_id": f.daemon.binding.InstallationID, "claim_generation": int64(3)})
	task, err := f.svc.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
	if err != nil || task.Status != "dispatched" || task.StartedAt.Valid || task.PrepareLeaseExpiresAt.Valid {
		t.Fatalf("actual reclaim fixture is not stale dispatched work: %v", err)
	}
	if err := ValidateManagedTaskBinding(f.daemon.daemonCtx, task); err != nil {
		t.Fatalf("stale ordinary task lost the actual managed claim credential: %v", err)
	}
	var accepting bool
	f.fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM managed_installation WHERE id=$1 AND lifecycle='active' AND admission='accepting' AND admission_version>0)`, f.daemon.binding.InstallationID).Scan(&accepting)
	if !accepting || task.ExecutionAdmissionVersion.Valid {
		t.Fatal("reclaim control lacks accepting installation authority for its retained unknown admission history")
	}
	return task
}

func r31WantReclaimed(t *testing.T, f *stageCapabilitiesReadFixture, before db.AgentTaskQueue, result r31ClaimResult, snapshot completionCandidatesDatabaseState) {
	t.Helper()
	if result.err != nil || len(result.tasks) != 1 {
		t.Fatalf("actual canonical reclaim did not return one task: tasks=%d err=%v", len(result.tasks), result.err)
	}
	got := result.tasks[0]
	current, err := f.svc.Queries.GetAgentTask(f.ctx, before.ID)
	if err != nil || !reflect.DeepEqual(current, got) || got.ID != before.ID || got.ClaimGeneration != before.ClaimGeneration+1 || !got.PrepareLeaseExpiresAt.Valid || !got.DispatchedAt.Time.Equal(before.DispatchedAt.Time) {
		t.Fatalf("canonical reclaim lost task/generation/lease identity: got=%+v err=%v", got, err)
	}
	now, err := f.svc.Queries.ArtifactExecutionDatabaseTime(f.ctx)
	if err != nil || !got.PrepareLeaseExpiresAt.Time.After(now.Time) || got.PrepareLeaseExpiresAt.Time.After(now.Time.Add(prepareLeaseDuration)) {
		t.Fatalf("canonical reclaim did not retain its actual finite prepare lease: %v", err)
	}
	normalized := got
	normalized.ClaimGeneration, normalized.PrepareLeaseExpiresAt = before.ClaimGeneration, before.PrepareLeaseExpiresAt
	if !reflect.DeepEqual(normalized, before) || got.ExecutionBindingID != f.daemon.binding.ID || got.ExecutionBindingEpoch.Int64 != f.daemon.binding.BindingEpoch || got.ExecutionInstallationID != f.daemon.binding.InstallationID || got.ExecutionAdmissionVersion.Valid != before.ExecutionAdmissionVersion.Valid {
		t.Fatal("real reclaim backfilled or changed original managed delivery history")
	}
	after := r31Snapshot(t, f)
	var priorRows, currentRows struct {
		Count int               `json:"count"`
		Rows  []json.RawMessage `json:"rows"`
	}
	if json.Unmarshal([]byte(snapshot.Tables["agent_task_queue"]), &priorRows) != nil || json.Unmarshal([]byte(after.Tables["agent_task_queue"]), &currentRows) != nil || priorRows.Count != currentRows.Count {
		t.Fatal("real reclaim changed the ordinary task inventory")
	}
	other := func(rows []json.RawMessage) map[string]string {
		out := map[string]string{}
		for _, row := range rows {
			var id struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(row, &id); err != nil {
				t.Fatal(err)
			}
			if id.ID != util.UUIDToString(before.ID) {
				out[id.ID] = string(row)
			}
		}
		return out
	}
	if !reflect.DeepEqual(other(priorRows.Rows), other(currentRows.Rows)) {
		t.Fatal("canonical reclaim changed an unrelated ordinary task")
	}
	delete(snapshot.Tables, "agent_task_queue")
	delete(after.Tables, "agent_task_queue")
	completionCandidatesCompareState(t, snapshot, after, "real reclaim other relations")
}

func TestOrdinaryIdleClaimAndR4Coexistence(t *testing.T) {
	t.Run("idle_first", func(t *testing.T) {
		f := r31Fixture(t)
		r31NoWrites(t, f)
		claim := r31StartClaim(t, f, true)
		pid := r31AwaitHoldOrEmpty(t, claim, true)
		view, err, readerPID, _ := r31R4(t, f)
		if pid != 0 {
			completionCandidatesDistinctPIDs(t, pid, readerPID)
			select {
			case <-claim.done:
				t.Error("idle claim released its real account fence before R4 terminal")
			default:
			}
		}
		claim.release()
		<-claim.done
		r31ClaimWitness(t, f, claim, "idle_first_after_R4_terminal")
		if claim.result.err != nil || len(claim.result.tasks) != 0 {
			t.Errorf("valid no-work ordinary claim did not finish empty: %v", claim.result.err)
		}
		r31WantR4(t, f, view, err)
	})
	t.Run("r4_first", func(t *testing.T) {
		f := r31Fixture(t)
		r31NoWrites(t, f)
		qa := *f.stage
		qa.reviewerCtx = f.stage.qaCtx
		held := holdStageArtifactR4(t, &qa, f.candidate)
		claim := r31StartClaim(t, f, false)
		blocked := r31CancelBlockedClaim(t, f, claim, held.pid)
		if !blocked {
			<-claim.done
		}
		r31ClaimWitness(t, f, claim, "r4_first_claim_terminal_before_R4_release")
		if claim.result.err != nil || len(claim.result.tasks) != 0 {
			t.Errorf("no-work ordinary claim excluded by an authorized R4: %T %v", claim.result.err, claim.result.err)
		}
		held.release()
		first := <-held.done
		r31WantR4(t, f, first.view, first.err)
		fresh := r31StartClaim(t, f, false)
		<-fresh.done
		if fresh.result.err != nil || len(fresh.result.tasks) != 0 {
			t.Errorf("fresh no-work claim after actual R4 release failed: %v", fresh.result.err)
		}
	})
}

func TestOrdinaryReclaimPreservesAuthorityFences(t *testing.T) {
	t.Run("reclaim_first", func(t *testing.T) {
		f := r31Fixture(t)
		before := r31SeedReclaim(t, f)
		snapshot := r31Snapshot(t, f)
		claim := r31StartClaim(t, f, true)
		pid := r31AwaitHoldOrEmpty(t, claim, false)
		view, err, readerPID, failure := r31R4(t, f)
		completionCandidatesDistinctPIDs(t, pid, readerPID)
		if stagePgCode(err) != "55P03" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) || failure == nil || failure.Query != "LockArtifactUsers" || failure.BackendPID != readerPID {
			t.Errorf("R4 crossed the actual reclaim writer account fence: first_pg=%+v err=%v", failure, err)
		}
		completionCandidatesCompareState(t, snapshot, r31Snapshot(t, f), "uncommitted real reclaim")
		claim.release()
		<-claim.done
		r31ClaimWitness(t, f, claim, "reclaim_first_committed")
		r31WantReclaimed(t, f, before, *claim.result, snapshot)
		view, err, _, _ = r31R4(t, f)
		r31WantR4(t, f, view, err)
	})
	t.Run("r4_first", func(t *testing.T) {
		f := r31Fixture(t)
		before := r31SeedReclaim(t, f)
		snapshot := r31Snapshot(t, f)
		qa := *f.stage
		qa.reviewerCtx = f.stage.qaCtx
		held := holdStageArtifactR4(t, &qa, f.candidate)
		claim := r31StartClaim(t, f, false)
		if !r31CancelBlockedClaim(t, f, claim, held.pid) {
			t.Fatal("actual stale-dispatched writer bypassed the held R4 account fence")
		}
		r31ClaimWitness(t, f, claim, "real_reclaim_blocked_before_R4_release")
		completionCandidatesCompareState(t, snapshot, r31Snapshot(t, f), "blocked real reclaim")
		held.release()
		first := <-held.done
		r31WantR4(t, f, first.view, first.err)
		fresh := r31StartClaim(t, f, false)
		<-fresh.done
		r31ClaimWitness(t, f, fresh, "real_reclaim_after_R4_release")
		r31WantReclaimed(t, f, before, *fresh.result, snapshot)
	})
}
