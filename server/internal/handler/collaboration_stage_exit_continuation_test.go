package handler

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func stageExitHTTPM1(t *testing.T, s *stageHTTP, mode string) {
	t.Helper()
	s.fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{
		"workspace_id": s.fx.WorkspaceID, "project_id": s.project, "revision": 1,
		"policy_version": service.DeliveryEvidencePolicy, "mode": mode, "stage": service.DeliveryPolicyStage,
		"reason": "Stage-enabled writer compatibility", "created_by": s.actor,
	}, "workspace_id=$1 AND project_id=$2", s.fx.WorkspaceID, s.project)
}

func stageExitHTTPRequest(s *stageHTTP, method, path, id string, body any) *http.Request {
	r := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "X-Workspace-ID", s.fx.WorkspaceID, "X-User-ID", s.actor)
	if id != "" {
		r = testutil.WithURLParams(r, "id", id)
	}
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), stagePrincipal(s.actor)))
}

func stageExitHTTPState(t *testing.T, s *stageHTTP) string {
	t.Helper()
	var state string
	s.fx.QueryRow(t, `SELECT jsonb_build_object(
 'issues',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status,'revision',revision,'project',project_id,'parent',parent_issue_id) ORDER BY id) FROM issue WHERE workspace_id=$1),
 'project',(SELECT jsonb_build_object('id',id,'revision',revision) FROM project WHERE id=$2),
 'tasks',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status,'completed',completed_at,'error',error) ORDER BY id) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)),
 'autopilots',(SELECT jsonb_agg(jsonb_build_object('id',id,'project',project_id,'status',status,'reason',pause_reason) ORDER BY id) FROM autopilot WHERE workspace_id=$1),
 'triggers',(SELECT jsonb_agg(jsonb_build_object('id',id,'enabled',enabled) ORDER BY id) FROM autopilot_trigger WHERE autopilot_id IN (SELECT id FROM autopilot WHERE workspace_id=$1)),
 'runs',(SELECT jsonb_agg(jsonb_build_object('id',id,'issue',issue_id,'status',status,'completed',completed_at) ORDER BY id) FROM autopilot_run WHERE autopilot_id IN (SELECT id FROM autopilot WHERE workspace_id=$1)),
 'tokens',(SELECT jsonb_agg(jsonb_build_object('id',id,'task',task_id) ORDER BY id) FROM task_token WHERE workspace_id=$1),
 'quota',(SELECT jsonb_agg(jsonb_build_object('id',id,'state',state,'finalized',finalized_at) ORDER BY id) FROM autopilot_quota_reservation WHERE workspace_id=$1),
 'periods',(SELECT jsonb_agg(jsonb_build_object('start',period_start,'end',period_end,'used',used_count,'reserved',reserved_count) ORDER BY period_start,period_end) FROM autopilot_quota_period WHERE workspace_id=$1),
 'attachments',(SELECT jsonb_agg(jsonb_build_object('id',id,'issue',issue_id,'comment',comment_id) ORDER BY id) FROM attachment WHERE workspace_id=$1),
 'session',(SELECT jsonb_build_object('queue',queue_state,'control',control_version,'seq',event_seq) FROM collaboration_session WHERE issue_id=$3),
 'events',(SELECT count(*) FROM collaboration_event WHERE issue_id=$3),
 'handoffs',(SELECT count(*) FROM issue_handoff WHERE issue_id=$3)
)::text`, s.fx.WorkspaceID, s.project, s.issue).Scan(&state)
	return state
}

// Forward every real SQL statement; record canonical mutation entry points so a
// preserved final row alone cannot hide an attempted cancellation or deletion.
type stageExitHTTPProbe struct {
	mu             sync.Mutex
	calls          map[string]int
	afterPreflight func()
	preflightOnce  sync.Once
}

func (p *stageExitHTTPProbe) observe(sql string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, name := range []string{"CancelAgentTasksByIssue", "DeleteTaskTokensByTask", "FailAutopilotRunsByIssue", "DeleteIssue", "DetachDirectChildIssues", "ListAttachmentURLsByIssueOrComments", "DeleteAttachmentsBySourceContext", "RecordSourceContextDeletionObjectIntent", "DeleteProject", "DetachProjectIssues", "DisableProjectAutopilotTriggers", "DetachProjectAutopilots"} {
		if strings.Contains(sql, "-- name: "+name+" ") {
			p.calls[name]++
		}
	}
}

func (p *stageExitHTTPProbe) mutations() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]int, len(p.calls))
	for name, count := range p.calls {
		out[name] = count
	}
	return out
}

type stageExitHTTPDB struct {
	db.DBTX
	probe *stageExitHTTPProbe
}

func (d stageExitHTTPDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.probe.observe(sql)
	return d.DBTX.Exec(ctx, sql, args...)
}

func (d stageExitHTTPDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	d.probe.observe(sql)
	return d.DBTX.Query(ctx, sql, args...)
}

func (d stageExitHTTPDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	d.probe.observe(sql)
	row := d.DBTX.QueryRow(ctx, sql, args...)
	if d.probe.afterPreflight != nil && strings.Contains(sql, "-- name: AnyCollaborationChildIssues ") {
		return stageExitHTTPPreflightRow{Row: row, probe: d.probe}
	}
	return row
}

type stageExitHTTPPreflightRow struct {
	pgx.Row
	probe *stageExitHTTPProbe
}

func (r stageExitHTTPPreflightRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	if found, ok := dest[0].(*bool); ok && !*found {
		r.probe.preflightOnce.Do(r.probe.afterPreflight)
	}
	return nil
}

type stageExitHTTPTraceStarter struct {
	inner txStarter
	probe *stageExitHTTPProbe
}

func (s stageExitHTTPTraceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageExitHTTPTraceTx{Tx: tx, probe: s.probe}, nil
}

type stageExitHTTPTraceTx struct {
	pgx.Tx
	probe *stageExitHTTPProbe
}

func (tx *stageExitHTTPTraceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.probe.observe(sql)
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *stageExitHTTPTraceTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.probe.observe(sql)
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *stageExitHTTPTraceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.probe.observe(sql)
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func stageExitHTTPObserved(h *Handler, probe *stageExitHTTPProbe) *Handler {
	copy := *h
	database := stageExitHTTPDB{DBTX: testPool, probe: probe}
	copy.DB, copy.Queries = database, db.New(database)
	copy.TxStarter = stageExitHTTPTraceStarter{inner: h.TxStarter, probe: probe}
	tasks := service.NewTaskService(copy.Queries, copy.TxStarter, h.Hub, h.Bus, h.TaskService.Wakeup)
	tasks.Analytics, tasks.Metrics, tasks.Entitlements = h.TaskService.Analytics, h.TaskService.Metrics, h.TaskService.Entitlements
	tasks.FeatureFlags, tasks.ReclaimCheck = h.TaskService.FeatureFlags, h.TaskService.ReclaimCheck
	autopilots := *h.AutopilotService
	autopilots.Queries, autopilots.TaskSvc = copy.Queries, tasks
	copy.TaskService, copy.AutopilotService = tasks, &autopilots
	return &copy
}

func stageExitHTTPWork(t *testing.T, s *stageHTTP, issue string) {
	t.Helper()
	runtime := s.fx.Runtime(t, "Exit continuation runtime "+issue)
	agent := s.fx.Agent(t, "Exit continuation agent "+issue, runtime)
	task := s.fx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running"})
	s.fx.Insert(t, "task_token", testutil.Cols{"task_id": task, "agent_id": agent, "workspace_id": s.fx.WorkspaceID, "user_id": s.actor,
		"token_hash": strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", ""), "auth_version": int64(1), "expires_at": testutil.Raw("clock_timestamp()+interval '1 hour'")})
	start := time.Now().UTC().Truncate(time.Microsecond)
	end := start.Add(time.Hour)
	s.fx.InsertNoID(t, "autopilot_quota_period", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "period_start": start, "period_end": end, "reserved_count": int64(1)},
		"workspace_id=$1 AND period_start=$2 AND period_end=$3", s.fx.WorkspaceID, start, end)
	reservation := s.fx.Insert(t, "autopilot_quota_reservation", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "period_start": start, "period_end": end,
		"policy_revision": int64(1), "subscription_version": int64(1), "source": "api", "idempotency_key": uuid.NewString()})
	ap := s.fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "project_id": s.project,
		"title": "Retained exit automation", "assignee_id": agent, "created_by_type": "member", "created_by_id": s.actor, "execution_mode": "run_only"})
	trigger := s.fx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": ap, "kind": "api", "enabled": true})
	s.fx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": ap, "trigger_id": trigger, "source": "api", "status": "running", "issue_id": issue, "task_id": task, "quota_reservation_id": reservation})
	s.fx.Insert(t, "attachment", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "issue_id": issue,
		"uploader_type": "member", "uploader_id": s.actor, "filename": "retained-exit.txt", "url": "/retained-exit.txt", "content_type": "text/plain", "size_bytes": 1})
}

func TestStageExitContinuationProjectImpactAndDeleteM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, mode)
			s.revise(t, true)
			ordinary := s.fx.Issue(t, "Retained project work", testutil.Cols{"project_id": s.project})
			stageExitHTTPWork(t, s, ordinary)
			probe := &stageExitHTTPProbe{calls: map[string]int{}}
			h := stageExitHTTPObserved(s.h, probe)
			before := stageExitHTTPState(t, s)
			var impact ProjectDeleteImpact
			testutil.Call(t, h.GetProjectDeleteImpact, stageExitHTTPRequest(s, http.MethodGet, "/api/projects/"+s.project+"/delete-impact", s.project, nil)).Want(http.StatusOK).JSON(&impact)
			if impact.ProjectID != s.project || impact.WorkspaceID != s.fx.WorkspaceID || impact.IssueCount != 2 ||
				!impact.PreservesIssues || !impact.PreservesExecutions || !reflect.DeepEqual(impact.BlockingReasons, []string{"stage_policy_exit_required"}) {
				t.Fatalf("stage-enabled %s delete impact hid its blocker or identities: %+v", mode, impact)
			}
			out := testutil.Call(t, h.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+s.project, s.project, nil)).Want(http.StatusConflict).Map()
			if out["code"] != "stage_policy_exit_required" || out["error"] != "stage_policy_exit_required" {
				t.Fatalf("stage-enabled %s project deletion escaped: %v", mode, out)
			}
			if stageExitHTTPState(t, s) != before || len(probe.mutations()) != 0 {
				t.Fatalf("refusal reached project cleanup, cancellation, queue or attachments: mutations=%v", probe.mutations())
			}
			// Closing stage alone cannot disable the independent M1 or retained
			// collaboration-project rule.
			disable := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "Explicit stage rollback"}
			testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), disable)).Want(http.StatusOK)
			out = testutil.Call(t, h.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+s.project, s.project, nil)).Want(http.StatusConflict).Map()
			want := "project_has_collaboration_issues"
			if mode == "enforce" {
				want = "delivery_project_policy_disable_required"
			}
			if out["code"] != want || stageExitHTTPState(t, s) != before || len(probe.mutations()) != 0 {
				t.Fatalf("explicit stage close weakened %s: body=%v mutations=%v", want, out, probe.mutations())
			}
			empty := s.fx.Project(t, "Unmanaged empty delete control")
			var virtual service.StagePolicy
			testutil.Call(t, s.h.GetStagePolicy, stageExitHTTPRequest(s, http.MethodGet, "/api/projects/"+empty+"/stage-policy", empty, nil)).Want(http.StatusOK).JSON(&virtual)
			if virtual.Enabled || virtual.PolicyRevision != "0" || virtual.Contract != "collaboration-stage-v1" {
				t.Fatalf("unmanaged empty project did not have the explicit virtual-off policy: %+v", virtual)
			}
			tx, err := testPool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			project, err := s.h.Queries.WithTx(tx).GetProjectInWorkspace(t.Context(), db.GetProjectInWorkspaceParams{ID: parseUUID(empty), WorkspaceID: parseUUID(s.fx.WorkspaceID)})
			if err == nil {
				err = service.CheckDeliveryProjectDeleteInTx(t.Context(), tx, project)
			}
			_ = tx.Rollback(t.Context())
			t.Logf("unmanaged empty project actual delete validator: error_type=%T error=%v", err, err)
			testutil.Call(t, s.h.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+empty, empty, nil)).Want(http.StatusNoContent)
			if s.fx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, empty) != 0 {
				t.Fatal("stage policy was incorrectly imposed on an unrelated empty project")
			}
		})
	}
}

func TestStageExitContinuationRootAndParentDeleteBeforeEffects(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, mode)
			s.revise(t, true)
			parent := s.fx.Issue(t, "Ordinary parent of retained collaboration", testutil.Cols{"project_id": s.project})
			companion := s.fx.Issue(t, "Unattempted mixed-delete companion", testutil.Cols{"project_id": s.project})
			testutil.Call(t, s.h.UpdateCollaborationIssue, s.request(http.MethodPatch, "", s.actor, uuid.NewString(),
				s.body(t, s.actor, map[string]any{"parent_issue_id": parent}))).Want(http.StatusOK)
			for _, issue := range []string{s.issue, parent, companion} {
				stageExitHTTPWork(t, s, issue)
			}
			probe := &stageExitHTTPProbe{calls: map[string]int{}}
			h := stageExitHTTPObserved(s.h, probe)
			before := stageExitHTTPState(t, s)
			for _, tc := range []struct {
				name, code, id string
				ids            []string
			}{
				{"root_single", "unsupported_collaboration_action", s.issue, nil},
				{"root_mixed_batch", "unsupported_collaboration_action", "", []string{companion, s.issue}},
				{"parent_single", "issue_has_collaboration_children", parent, nil},
				{"parent_mixed_batch", "issue_has_collaboration_children", "", []string{companion, parent}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					call := h.DeleteIssue
					r := stageExitHTTPRequest(s, http.MethodDelete, "/api/issues/"+tc.id, tc.id, nil)
					if tc.ids != nil {
						call = h.BatchDeleteIssues
						r = stageExitHTTPRequest(s, http.MethodPost, "/api/issues/batch-delete", "", map[string]any{"issue_ids": tc.ids})
					}
					if out := testutil.Call(t, call, r).Want(http.StatusConflict).Map(); out["code"] != tc.code {
						t.Fatalf("%s deletion used the wrong collaboration boundary: %v", tc.name, out)
					}
					if stageExitHTTPState(t, s) != before || len(probe.mutations()) != 0 {
						t.Fatalf("%s cancelled/deleted before rejecting: mutations=%v", tc.name, probe.mutations())
					}
				})
			}
		})
	}
}

func TestStageExitContinuationParentAttachAfterDeletePreflight(t *testing.T) {
	for _, batch := range []bool{false, true} {
		name := "single"
		if batch {
			name = "batch"
		}
		t.Run(name, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, "off")
			s.revise(t, true)
			parent := s.fx.Issue(t, "Parent changes after unlocked preflight", testutil.Cols{"project_id": s.project})
			stageExitHTTPWork(t, s, parent)
			attached := false
			afterAttach := ""
			probe := &stageExitHTTPProbe{calls: map[string]int{}}
			probe.afterPreflight = func() {
				// The original false result is real. A separate owning workflow
				// commits a collaboration child before cancellation/delete begins.
				testutil.Call(t, s.h.UpdateCollaborationIssue, s.request(http.MethodPatch, "", s.actor, uuid.NewString(),
					s.body(t, s.actor, map[string]any{"parent_issue_id": parent}))).Want(http.StatusOK)
				attached = true
				afterAttach = stageExitHTTPState(t, s)
			}
			h := stageExitHTTPObserved(s.h, probe)
			call := h.DeleteIssue
			r := stageExitHTTPRequest(s, http.MethodDelete, "/api/issues/"+parent, parent, nil)
			if batch {
				call = h.BatchDeleteIssues
				r = stageExitHTTPRequest(s, http.MethodPost, "/api/issues/batch-delete", "", map[string]any{"issue_ids": []string{parent}})
			}
			out := testutil.Call(t, call, r).Want(http.StatusConflict).Map()
			if !attached || out["code"] != "issue_has_collaboration_children" {
				t.Fatalf("actual delete did not recheck the separately committed collaboration child: attached=%v out=%v", attached, out)
			}
			if stageExitHTTPState(t, s) != afterAttach || len(probe.mutations()) != 0 {
				t.Fatalf("transactional child refusal happened after cancellation/autopilot effects: mutations=%v", probe.mutations())
			}
		})
	}
}

type stageExitHTTPWriteFailureStarter struct {
	inner txStarter
	step  string
	hit   *bool
}

func (s stageExitHTTPWriteFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageExitHTTPWriteFailureTx{Tx: tx, step: s.step, hit: s.hit}, nil
}

type stageExitHTTPWriteFailureTx struct {
	pgx.Tx
	step string
	hit  *bool
}

func (tx *stageExitHTTPWriteFailureTx) fails(sql string) bool {
	if strings.Contains(sql, "-- name: "+tx.step+" ") {
		*tx.hit = true
		return true
	}
	return false
}

func (tx *stageExitHTTPWriteFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.fails(sql) {
		return pgconn.CommandTag{}, errors.New("injected required issue deletion write failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *stageExitHTTPWriteFailureTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.fails(sql) {
		return nil, errors.New("injected required issue deletion write failure")
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestStageExitContinuationDeleteRequiredWritesRollBackEffects(t *testing.T) {
	for _, step := range []string{"DeleteTaskTokensByTask", "FailAutopilotRunsByIssue", "DeleteIssue"} {
		t.Run(step, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, "off")
			s.revise(t, true)
			ordinary := s.fx.Issue(t, "Independent ordinary deletion rollback", testutil.Cols{"project_id": s.project})
			stageExitHTTPWork(t, s, ordinary)
			probe := &stageExitHTTPProbe{calls: map[string]int{}}
			h := stageExitHTTPObserved(s.h, probe)
			hit := false
			h.TxStarter = stageExitHTTPWriteFailureStarter{inner: h.TxStarter, step: step, hit: &hit}
			bus := events.New()
			h.Bus, h.TaskService.Bus, h.AutopilotService.Bus = bus, bus, bus
			var published atomic.Int32
			for _, kind := range []string{protocol.EventTaskCancelled, protocol.EventIssueDeleted} {
				bus.Subscribe(kind, func(events.Event) { published.Add(1) })
			}
			before := stageExitHTTPState(t, s)
			testutil.Call(t, h.DeleteIssue, stageExitHTTPRequest(s, http.MethodDelete, "/api/issues/"+ordinary, ordinary, nil)).Want(http.StatusInternalServerError)
			if !hit || probe.mutations()["CancelAgentTasksByIssue"] != 1 || stageExitHTTPState(t, s) != before || published.Load() != 0 {
				t.Fatalf("required %s failure did not roll back tasks, tokens, quota, autopilot and attachments before publishing: hit=%v mutations=%v published=%d",
					step, hit, probe.mutations(), published.Load())
			}
		})
	}
}

type stageExitHTTPBarrierStarter struct {
	inner        txStarter
	reached      chan struct{}
	release      <-chan struct{}
	beforeCommit bool
	once         *sync.Once
	pid          *atomic.Uint32
}

func (s stageExitHTTPBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageExitHTTPBarrierTx{Tx: tx, owner: s}, nil
}

type stageExitHTTPBarrierTx struct {
	pgx.Tx
	owner stageExitHTTPBarrierStarter
}

func (tx *stageExitHTTPBarrierTx) pause(ctx context.Context) error {
	tx.owner.once.Do(func() {
		tx.owner.pid.Store(tx.Conn().PgConn().PID())
		close(tx.owner.reached)
	})
	select {
	case <-tx.owner.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (tx *stageExitHTTPBarrierTx) Commit(ctx context.Context) error {
	if tx.owner.beforeCommit {
		if err := tx.pause(ctx); err != nil {
			return err
		}
	}
	return tx.Tx.Commit(ctx)
}

func (tx *stageExitHTTPBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if !tx.owner.beforeCommit && strings.Contains(sql, "-- name: LockProjectForExecutionSquad ") {
		return stageExitHTTPBarrierRow{Row: row, tx: tx, ctx: ctx}
	}
	return row
}

type stageExitHTTPBarrierRow struct {
	pgx.Row
	tx  *stageExitHTTPBarrierTx
	ctx context.Context
}

func (r stageExitHTTPBarrierRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	return r.tx.pause(r.ctx)
}

func stageExitHTTPWaiter(t *testing.T, s *stageHTTP, blocker uint32) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		s.fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND wait_event_type='Lock' AND $1::integer=ANY(pg_blocking_pids(pid)))`, int64(blocker)).Scan(&waiting)
		if waiting {
			t.Logf("actual second backend waits on first writer PID %d before its commit/rollback", blocker)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("second actual handler never waited on the held first transaction")
}

func TestStageExitContinuationProjectDeleteSubmitCommitOrders(t *testing.T) {
	for _, order := range []string{"submit_first", "delete_first"} {
		t.Run(order, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, "off")
			s.revise(t, true)
			ctx := auth.WithAuthenticatedPrincipal(t.Context(), stagePrincipal(s.recipient))
			options, err := s.svc.StatusOptions(ctx, parseUUID(s.issue), "submit_acceptance")
			if err != nil {
				t.Fatal(err)
			}
			key := uuid.NewString()
			body := s.body(t, s.recipient, map[string]any{"note": "Exact pending result", "basis": "Stage is still incomplete",
				"expected_status": options.DefaultStatus, "status_options_version": options.StatusOptionsVersion})
			reached, release := make(chan struct{}), make(chan struct{})
			var once, releaseOnce sync.Once
			pid := &atomic.Uint32{}
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			first := *s.h
			first.TxStarter = stageExitHTTPBarrierStarter{inner: s.h.TxStarter, reached: reached, release: release, beforeCommit: order == "submit_first", once: &once, pid: pid}
			finished := make(chan *testutil.Response, 1)
			second := make(chan *testutil.Response, 1)
			before := stageExitHTTPState(t, s)
			if order == "submit_first" {
				go func() {
					finished <- testutil.Call(t, first.SubmitCollaborationAcceptance, s.request(http.MethodPost, "/acceptance-submissions", s.recipient, key, body))
				}()
			} else {
				go func() {
					finished <- testutil.Call(t, first.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+s.project, s.project, nil))
				}()
			}
			select {
			case <-reached:
			case <-time.After(10 * time.Second):
				t.Fatal("actual submit/delete writer never obtained its fence")
			}
			if order == "submit_first" {
				go func() {
					second <- testutil.Call(t, s.h.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+s.project, s.project, nil))
				}()
			} else {
				go func() {
					second <- testutil.Call(t, s.h.SubmitCollaborationAcceptance, s.request(http.MethodPost, "/acceptance-submissions", s.recipient, key, body))
				}()
			}
			stageExitHTTPWaiter(t, s, pid.Load())
			unblock()
			out := (<-finished).Want(http.StatusConflict).Map()
			want := "stage_policy_exit_required"
			if order == "submit_first" {
				want = "stage_incomplete"
			}
			if out["code"] != want || stageExitHTTPState(t, s) != before {
				t.Fatalf("first commit order escaped stage or changed facts: out=%v", out)
			}
			waiting := (<-second).Want(http.StatusConflict).Map()
			secondWant := "stage_incomplete"
			if order == "submit_first" {
				secondWant = "stage_policy_exit_required"
			}
			if waiting["code"] != secondWant || stageExitHTTPState(t, s) != before {
				t.Fatalf("waiting writer ignored the first writer's committed policy/result: %v", waiting)
			}
			if out := testutil.Call(t, s.h.DeleteProject, stageExitHTTPRequest(s, http.MethodDelete, "/api/projects/"+s.project, s.project, nil)).Want(http.StatusConflict).Map(); out["code"] != "stage_policy_exit_required" {
				t.Fatalf("project exit did not recheck after submit released its fence: %v", out)
			}
			if out := testutil.Call(t, s.h.SubmitCollaborationAcceptance, s.request(http.MethodPost, "/acceptance-submissions", s.recipient, key, body)).Want(http.StatusConflict).Map(); out["code"] != "stage_incomplete" {
				t.Fatalf("submit did not recheck after delete released its fence: %v", out)
			}
		})
	}
}
