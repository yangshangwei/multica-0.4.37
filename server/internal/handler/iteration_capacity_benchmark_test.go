package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type iterationCapacityFixture struct {
	workspace string
	active    string
	planned   string
	issues    map[string][]string
}

// Bulk inserts use the existing fixture helper and workspace-scoped cleanup.
// All benchmark cases share this population; only their target issues differ.
func newIterationCapacityFixture(b *testing.B) iterationCapacityFixture {
	b.Helper()
	f := iterationCapacityFixture{issues: map[string][]string{}}
	for w := 0; w < 4; w++ {
		workspace := dbfx.Workspace(b, "Iteration capacity", "i1-capacity-"+uuid.NewString())
		dbfx.Member(b, workspace, testUserID, "owner")
		dbfx.InsertNoID(b, "workspace_iteration_settings", testutil.Cols{"workspace_id": workspace, "enabled": true}, "workspace_id=$1", workspace)
		ids := make([]string, 3)
		for n, status := range []string{"active", "planned", "completed"} {
			cols := testutil.Cols{"workspace_id": workspace, "name": "Capacity " + status, "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": status, "created_by": testUserID}
			if status != "planned" {
				cols["started_at"] = "2026-10-01T00:00:00Z"
				cols["started_by"] = testUserID
			}
			if status == "completed" {
				cols["start_date"] = "2026-09-15"
				cols["end_date"] = "2026-09-30"
				cols["started_at"] = "2026-09-15T00:00:00Z"
				cols["logical_ended_at"] = "2026-09-30T00:00:00Z"
				cols["processed_at"] = "2026-09-30T00:00:00Z"
			}
			if status == "planned" {
				cols["start_date"] = "2026-10-07"
				cols["end_date"] = "2026-10-20"
			}
			ids[n] = dbfx.Insert(b, "iteration", cols)
		}
		// Register before insertion so a partial fixture failure is still cleaned.
		dbfx.Cleanup(b, "DELETE FROM issue WHERE workspace_id=$1", workspace)
		dbfx.Cleanup(b, "DELETE FROM iteration_participation WHERE workspace_id=$1", workspace)
		dbfx.Cleanup(b, "DELETE FROM iteration_event WHERE workspace_id=$1", workspace)
		dbfx.Exec(b, `INSERT INTO issue (workspace_id,title,status,priority,creator_type,creator_id,number,position,current_iteration_id)
		 SELECT $1,'Capacity issue '||n,'todo','none','member',$2,n,n,
		 CASE WHEN n<=400 THEN $3::uuid WHEN n<=800 THEN $4::uuid ELSE NULL END
		 FROM generate_series(1,1200) n`, workspace, testUserID, ids[0], ids[1])
		dbfx.Exec(b, `INSERT INTO iteration_participation (workspace_id,iteration_id,issue_id,first_joined_at,current_joined_at,in_original,original_facts)
		 SELECT workspace_id,current_iteration_id,id,'2026-10-01','2026-10-01',number<=400,
		 jsonb_build_object('issue_id',id,'title',title,'status_key',status,'status_category',status,
		 'project_id',NULL,'assignee_type',NULL,'assignee_id',NULL,'rollover_count',0,'has_started',false)
		 FROM issue WHERE workspace_id=$1 AND current_iteration_id IS NOT NULL`, workspace)
		dbfx.Exec(b, `INSERT INTO iteration_participation (workspace_id,iteration_id,issue_id,first_joined_at,last_left_at,in_original,original_facts)
		 SELECT workspace_id,$2,id,'2026-09-15','2026-09-30',true,
		 jsonb_build_object('issue_id',id,'title',title,'status_category','done')
		 FROM issue WHERE workspace_id=$1`, workspace, ids[2])
		for _, id := range ids {
			dbfx.Exec(b, `INSERT INTO iteration_event (workspace_id,iteration_id,sequence,operation_id,issue_id,kind,actor,occurred_at,sampled_at,before_facts,after_facts)
			 SELECT $1,$2,n,gen_random_uuid(),x.id,CASE WHEN i.status='planned' THEN 'planned_activity' ELSE 'issue_changed' END,
			 '{"type":"system","source":"capacity_fixture","id":null,"user_id":null}',
			 CASE WHEN i.status='completed' THEN '2026-09-15' ELSE '2026-10-01' END::timestamptz+n*interval '1 second',
			 CASE WHEN i.status='completed' THEN '2026-09-15' ELSE '2026-10-01' END::timestamptz+n*interval '1 second',
			 jsonb_build_object('issue_id',x.id,'title',repeat('historical fact ',16),'status_category','todo'),
			 jsonb_build_object('issue_id',x.id,'title',repeat('historical edit ',16),'status_category','todo')
			 FROM generate_series(1,10000) n JOIN iteration i ON i.workspace_id=$1 AND i.id=$2
			 JOIN issue x ON x.workspace_id=$1 AND x.number=(n-1)%400+1+CASE WHEN i.status='planned' THEN 400 ELSE 0 END`, workspace, id)
		}
		if w == 0 {
			f.workspace, f.active, f.planned = workspace, ids[0], ids[1]
			rows, err := testPool.Query(context.Background(), `SELECT id::text,CASE WHEN number<=400 THEN 'active' WHEN number<=800 THEN 'planned' ELSE 'unassociated' END FROM issue WHERE workspace_id=$1 ORDER BY number`, workspace)
			if err != nil {
				b.Fatal(err)
			}
			for rows.Next() {
				var id, group string
				if err := rows.Scan(&id, &group); err != nil {
					b.Fatal(err)
				}
				f.issues[group] = append(f.issues[group], id)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				b.Fatal(err)
			}
		}
		if got := dbfx.Count(b, `SELECT count(*) FROM issue WHERE workspace_id=$1`, workspace); got != 1200 {
			b.Fatalf("capacity fixture issue count=%d", got)
		}
		if got := dbfx.Count(b, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1`, workspace); got != 30000 {
			b.Fatalf("capacity fixture history count=%d", got)
		}
	}
	for _, table := range []string{"issue", "iteration", "iteration_participation", "iteration_event"} {
		// Remove dead tuples from previous fixture runs before warming the data.
		dbfx.Exec(b, "VACUUM (ANALYZE) "+pgx.Identifier{table}.Sanitize())
	}
	return f
}

type iterationCapacityTracer struct{ statements atomic.Int64 }

func (t *iterationCapacityTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	t.statements.Add(1)
	return ctx
}
func (*iterationCapacityTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (t *iterationCapacityTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchStartData) context.Context {
	t.statements.Add(int64(data.Batch.Len()))
	return ctx
}
func (*iterationCapacityTracer) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {
}
func (*iterationCapacityTracer) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func iterationCapacityHandler(t testutil.TB, name string, tracer *iterationCapacityTracer) (*Handler, *pgxpool.Pool) {
	t.Helper()
	config := testPool.Config()
	config.ConnConfig.RuntimeParams["application_name"] = name
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("create capacity pool: %v", err)
	}
	t.Cleanup(pool.Close)
	h := *testHandler
	h.Queries, h.DB, h.TxStarter = db.New(pool), pool, pool
	return &h, pool
}

type iterationCapacityWaits struct {
	samples      int
	blocked      int
	blockerLinks int
	blockedTime  time.Duration
	elapsed      time.Duration
	events       map[string]int
	err          error
}

// This deliberately samples PostgreSQL wait state, not client statement time.
// Sampling is approximate: waits shorter than a sampling interval can be missed.
func sampleIterationCapacityWaits(ctx context.Context, conn *pgx.Conn, application string, ready chan<- struct{}) iterationCapacityWaits {
	out := iterationCapacityWaits{events: map[string]int{}}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	previous := time.Now()
	start := previous
	close(ready)
	for {
		select {
		case <-ctx.Done():
			out.elapsed = time.Since(start)
			return out
		case <-ticker.C:
			rows, err := conn.Query(ctx, `SELECT wait_event,cardinality(pg_blocking_pids(pid))
			 FROM pg_stat_activity WHERE datname=current_database()
			 AND application_name=$1 AND wait_event_type='Lock'`, application)
			if err != nil {
				if ctx.Err() == nil {
					out.err = err
				}
				out.elapsed = time.Since(start)
				return out
			}
			now := time.Now()
			interval := now.Sub(previous)
			previous = now
			out.samples++
			for rows.Next() {
				var event string
				var blockers int
				if err := rows.Scan(&event, &blockers); err != nil {
					out.err = err
					break
				}
				out.blocked++
				out.blockerLinks += blockers
				out.events[event]++
				out.blockedTime += interval
			}
			if err := rows.Err(); err != nil && ctx.Err() == nil {
				out.err = err
			}
			rows.Close()
			if out.err != nil {
				return out
			}
		}
	}
}

func capacityTitleRequest(workspace, issue, title string) *http.Request {
	req := withURLParam(newRequest(http.MethodPut, "/api/issues/"+issue, map[string]any{"title": title}), "id", issue)
	req.Header.Set("X-Workspace-ID", workspace)
	return req
}

// BenchmarkIterationPopulatedWrite is intentionally separate from the frozen
// small ordinary-write regression fixture. See s3-capacity-verification.md.
func BenchmarkIterationPopulatedWrite(b *testing.B) {
	if testHandler == nil {
		b.Fatal("a reachable isolated PostgreSQL database is required")
	}
	f := newIterationCapacityFixture(b)
	logIterationCapacityPlans(b, f)
	for _, writers := range []int{1, 4} {
		for _, associated := range []bool{false, true} {
			name := "unassociated"
			if associated {
				name = "active_planned"
			}
			b.Run(fmt.Sprintf("writers_%d/%s", writers, name), func(b *testing.B) {
				tracer := &iterationCapacityTracer{}
				application := "i1-capacity-" + uuid.NewString()
				h, _ := iterationCapacityHandler(b, application, tracer)
				metrics := &iterationBenchmarkMetrics{}
				h.TxStarter = iterationBenchmarkStarter{txStarter: h.TxStarter, metrics: metrics}
				ids := make([][]string, writers)
				for w := range ids {
					ids[w] = []string{f.issues["unassociated"][w*2], f.issues["unassociated"][w*2+1]}
					if associated {
						ids[w] = []string{f.issues["active"][w], f.issues["planned"][w]}
					}
					for n := 0; n < 20; n++ {
						response := httptest.NewRecorder()
						h.UpdateIssue(response, capacityTitleRequest(f.workspace, ids[w][n%2], fmt.Sprintf("%s warmup %d", application, n)))
						if response.Code != http.StatusOK {
							b.Fatalf("warmup: HTTP %d: %s", response.Code, response.Body.String())
						}
					}
				}
				before := dbfx.Count(b, "SELECT count(*) FROM iteration_event WHERE workspace_id=$1", f.workspace)
				beforeScope := dbfx.Count(b, "SELECT sum(scope_revision) FROM iteration WHERE workspace_id=$1", f.workspace)
				monitor, err := pgx.ConnectConfig(context.Background(), testPool.Config().ConnConfig)
				if err != nil {
					b.Fatal(err)
				}
				defer monitor.Close(context.Background())
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ready, done := make(chan struct{}), make(chan iterationCapacityWaits, 1)
				go func() { done <- sampleIterationCapacityWaits(ctx, monitor, application, ready) }()
				<-ready
				metrics.queries.Store(0)
				metrics.lockNS.Store(0)
				tracer.statements.Store(0)
				latencies := make([]time.Duration, b.N)
				var next atomic.Int64
				var failures atomic.Int64
				var workers sync.WaitGroup
				b.ResetTimer()
				start := time.Now()
				for _, targets := range ids {
					workers.Add(1)
					go func(targets []string) {
						defer workers.Done()
						for sequence := 0; ; sequence++ {
							i := int(next.Add(1) - 1)
							if i >= b.N {
								return
							}
							request := capacityTitleRequest(f.workspace, targets[sequence%2], fmt.Sprintf("%s write %d", application, i))
							response := httptest.NewRecorder()
							started := time.Now()
							h.UpdateIssue(response, request)
							latencies[i] = time.Since(started)
							if response.Code != http.StatusOK {
								failures.Add(1)
							}
						}
					}(targets)
				}
				workers.Wait()
				elapsed := time.Since(start)
				b.StopTimer()
				cancel()
				waits := <-done
				if waits.err != nil {
					b.Fatal(waits.err)
				}
				if b.N >= 100 && waits.samples == 0 {
					b.Fatal("no PostgreSQL lock samples collected")
				}
				if failures.Load() != 0 {
					b.Fatalf("%d HTTP updates failed", failures.Load())
				}
				wantEvents := 0
				if associated {
					wantEvents = b.N
				}
				if got := dbfx.Count(b, "SELECT count(*) FROM iteration_event WHERE workspace_id=$1", f.workspace) - before; got != wantEvents {
					b.Fatalf("persisted events=%d, want %d", got, wantEvents)
				}
				if got := dbfx.Count(b, "SELECT sum(scope_revision) FROM iteration WHERE workspace_id=$1", f.workspace) - beforeScope; got != wantEvents {
					b.Fatalf("persisted scope increments=%d, want %d", got, wantEvents)
				}
				if associated && metrics.queries.Load() > int64(12*b.N) {
					b.Fatalf("associated transaction statements=%d exceed frozen 12/write bound", metrics.queries.Load())
				}
				sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
				b.ReportMetric(float64(latencies[(len(latencies)-1)*95/100])/1e6, "p95-ms")
				b.ReportMetric(float64(b.N)/elapsed.Seconds(), "writes/s")
				b.ReportMetric(float64(metrics.queries.Load())/float64(b.N), "tx-queries/write")
				b.ReportMetric(float64(tracer.statements.Load())/float64(b.N), "all-statements/write")
				b.ReportMetric(float64(metrics.lockNS.Load())/float64(b.N)/1e6, "lock-statement-ms/write")
				b.ReportMetric(float64(waits.blockedTime)/float64(b.N)/1e6, "sampled-lock-ms/write")
				b.ReportMetric(float64(waits.samples), "lock-samples")
				b.Logf("server waits: samples=%d mean_interval_ms=%.3f blocked_observations=%d blocker_links=%d events=%v", waits.samples, float64(waits.elapsed)/float64(max(waits.samples, 1))/1e6, waits.blocked, waits.blockerLinks, waits.events)
			})
		}
	}
}

func logIterationCapacityPlans(b *testing.B, f iterationCapacityFixture) {
	b.Helper()
	var version, isolation, buffers, size string
	dbfx.QueryRow(b, "SELECT version(),current_setting('default_transaction_isolation'),current_setting('shared_buffers'),pg_size_pretty(pg_total_relation_size('iteration_event'))").Scan(&version, &isolation, &buffers, &size)
	b.Logf("database: %s isolation=%s shared_buffers=%s iteration_event_size=%s", version, isolation, buffers, size)
	for _, query := range []struct {
		name string
		sql  string
		args []any
	}{
		{"current_iteration", `SELECT i.* FROM iteration i JOIN issue x ON x.workspace_id=i.workspace_id AND x.current_iteration_id=i.id WHERE x.workspace_id=$1 AND x.id=$2 FOR UPDATE OF i`, []any{f.workspace, f.issues["active"][0]}},
		{"participation", `SELECT * FROM iteration_participation WHERE workspace_id=$1 AND iteration_id=$2 AND issue_id=$3 FOR UPDATE`, []any{f.workspace, f.active, f.issues["active"][0]}},
		{"latest_event", `SELECT sequence,occurred_at FROM iteration_event WHERE workspace_id=$1 AND iteration_id=$2 ORDER BY sequence DESC LIMIT 1`, []any{f.workspace, f.active}},
	} {
		rows, err := testPool.Query(context.Background(), "EXPLAIN (ANALYZE, BUFFERS) "+query.sql, query.args...)
		if err != nil {
			b.Fatal(err)
		}
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				b.Fatal(err)
			}
			b.Logf("plan %s: %s", query.name, line)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestIterationWriterServerLockWait(t *testing.T) {
	issue, iterationID := iterationIssueFixture(t)
	application := "i1-lock-proof-" + uuid.NewString()
	h, _ := iterationCapacityHandler(t, application, &iterationCapacityTracer{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blocker, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if err := iteration.LockWorkspace(ctx, blocker, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	monitor, err := pgx.ConnectConfig(ctx, testPool.Config().ConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close(context.Background())
	sampleCtx, stopSampling := context.WithCancel(ctx)
	defer stopSampling()
	ready, sampled := make(chan struct{}), make(chan iterationCapacityWaits, 1)
	go func() { sampled <- sampleIterationCapacityWaits(sampleCtx, monitor, application, ready) }()
	<-ready
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		request := capacityTitleRequest(testWorkspaceID, issue, "After observed server lock")
		h.UpdateIssue(response, withURLParam(request.WithContext(ctx), "id", issue))
		result <- response
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	observed := false
	for !observed {
		select {
		case <-ctx.Done():
			t.Fatal("writer did not expose its real blocking backend in pg_stat_activity")
		case response := <-result:
			t.Fatalf("writer bypassed held fence: HTTP %d", response.Code)
		case <-ticker.C:
			err = testPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			 WHERE datname=current_database() AND application_name=$1
			 AND wait_event_type='Lock' AND wait_event='advisory'
			 AND $2::integer=ANY(pg_blocking_pids(pid)))`, application, int32(blocker.Conn().PgConn().PID())).Scan(&observed)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iterationID); got != 0 {
		t.Fatalf("writer committed while blocked: events=%d", got)
	}
	// Keep the proven blocker held across several nominal sampling intervals.
	select {
	case <-time.After(10 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("deadline while checking server lock sampler")
	}
	stopSampling()
	waits := <-sampled
	if waits.err != nil || waits.blocked == 0 || waits.blockerLinks == 0 {
		t.Fatalf("sampler failed to observe the proven blocker: %+v", waits)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		if response.Code != http.StatusOK {
			t.Fatalf("released writer: HTTP %d: %s", response.Code, response.Body.String())
		}
	case <-ctx.Done():
		t.Fatal("writer did not finish after its observed blocker released")
	}
	if got := dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iterationID); got != 1 {
		t.Fatalf("released writer facts=%d, want one", got)
	}
	t.Logf("Observed PostgreSQL advisory wait and exact blocker PID before release; sampler=%d blocked observations; one committed event after release", waits.blocked)
}
